//go:build integration

package integration

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/exec"
	"sync"
	"testing"
	"time"

	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/rabbitmq"
	"github.com/testcontainers/testcontainers-go/wait"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"golang.org/x/crypto/bcrypt"
)

const (
	testProjectID = "integration"

	mongoUser = "admin"
	mongoPass = "password"

	// internalSecret is what the Broker is started with, so dashboard-style
	// requests in these tests can reach the internal routes.
	internalSecret = "test-secret"
)

// --- package-wide teardown ---

var (
	teardownMu sync.Mutex
	teardowns  []func()
)

func onTeardown(fn func()) {
	teardownMu.Lock()
	defer teardownMu.Unlock()
	teardowns = append(teardowns, fn)
}

// TestMain tears down the fixtures that outlive individual tests. Containers
// and service processes are shared across the package rather than rebuilt per
// test — standing a stack up costs the better part of a minute — so they cannot
// hang off any one test's t.Cleanup.
func TestMain(m *testing.M) {
	code := m.Run()

	teardownMu.Lock()
	fns := teardowns
	teardownMu.Unlock()

	for i := len(fns) - 1; i >= 0; i-- {
		fns[i]()
	}

	os.Exit(code)
}

// --- containers ---

// startMongo launches a MongoDB container with the credentials the services
// expect and returns its connection URI. The container lives until the package
// finishes.
func startMongo() (string, error) {
	ctx := context.Background()

	c, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: testcontainers.ContainerRequest{
			Image:        "mongo:4.2.16-bionic",
			ExposedPorts: []string{"27017/tcp"},
			Env: map[string]string{
				"MONGO_INITDB_ROOT_USERNAME": mongoUser,
				"MONGO_INITDB_ROOT_PASSWORD": mongoPass,
			},
			WaitingFor: wait.ForLog("waiting for connections on port 27017"),
		},
		Started: true,
	})
	if err != nil {
		return "", err
	}
	onTeardown(func() { c.Terminate(context.Background()) })

	host, err := c.Host(ctx)
	if err != nil {
		return "", err
	}
	port, err := c.MappedPort(ctx, "27017")
	if err != nil {
		return "", err
	}

	return fmt.Sprintf("mongodb://%s:%s@%s:%s", mongoUser, mongoPass, host, port.Port()), nil
}

func startRabbit() (string, error) {
	ctx := context.Background()

	c, err := rabbitmq.Run(ctx, "rabbitmq:3.9-alpine")
	if err != nil {
		return "", err
	}
	onTeardown(func() { c.Terminate(context.Background()) })

	return c.AmqpURL(ctx)
}

// dedicatedMongo gives the caller a MongoDB container of its own, for tests
// that cannot share a database with anything else.
func dedicatedMongo(t *testing.T) string {
	t.Helper()

	uri, err := startMongo()
	if err != nil {
		t.Fatalf("mongo container: %v", err)
	}
	return uri
}

// sharedModelsMongo is the MongoDB the data-layer tests share. They clear the
// collections they use on setup, so one container serves all of them.
var (
	modelsMongoOnce sync.Once
	modelsMongoURI  string
	modelsMongoErr  error
)

func sharedModelsMongo(t *testing.T) string {
	t.Helper()

	modelsMongoOnce.Do(func() { modelsMongoURI, modelsMongoErr = startMongo() })
	if modelsMongoErr != nil {
		t.Fatalf("models mongo: %v", modelsMongoErr)
	}
	return modelsMongoURI
}

// --- shared service stack ---

type testStack struct {
	mongoURI  string
	rabbitURI string
	brokerURL string
}

var (
	stackOnce sync.Once
	stackVal  *testStack
	stackErr  error
)

// sharedStack returns the one Logger/Listener/Broker stack the end-to-end tests
// share. They stay out of each other's way by using distinct projects, which is
// the property under test anyway.
func sharedStack(t *testing.T) *testStack {
	t.Helper()

	stackOnce.Do(func() {
		start := time.Now()
		stackVal, stackErr = buildStack()
		if stackErr == nil {
			t.Logf("shared stack ready in %s (broker at %s)", time.Since(start).Round(time.Millisecond), stackVal.brokerURL)
		}
	})
	if stackErr != nil {
		t.Fatalf("shared stack: %v", stackErr)
	}
	return stackVal
}

func buildStack() (*testStack, error) {
	mongoURI, err := startMongo()
	if err != nil {
		return nil, fmt.Errorf("mongo container: %w", err)
	}

	rabbitURI, err := startRabbit()
	if err != nil {
		return nil, fmt.Errorf("rabbitmq container: %w", err)
	}

	loggerRPCAddr, err := freeAddress()
	if err != nil {
		return nil, err
	}
	loggerHTTPAddr, err := freeAddress()
	if err != nil {
		return nil, err
	}
	brokerHTTPAddr, err := freeAddress()
	if err != nil {
		return nil, err
	}

	if err := spawn("../logger/cmd/api", map[string]string{
		"MONGO_URL":        mongoURI,
		"LOGGER_RPC_PORT":  portOf(loggerRPCAddr),
		"LOGGER_HTTP_PORT": portOf(loggerHTTPAddr),
	}); err != nil {
		return nil, fmt.Errorf("logger: %w", err)
	}
	if err := waitTCP(loggerRPCAddr, 60*time.Second); err != nil {
		return nil, fmt.Errorf("logger RPC: %w", err)
	}

	if err := spawn("../listener/cmd/api", map[string]string{
		"RABBITMQ_URL":    rabbitURI,
		"LOGGER_RPC_ADDR": loggerRPCAddr,
	}); err != nil {
		return nil, fmt.Errorf("listener: %w", err)
	}

	if err := spawn("../broker/cmd/api", map[string]string{
		"MONGO_URL":           mongoURI,
		"RABBITMQ_URL":        rabbitURI,
		"LOGGER_RPC_ADDR":     loggerRPCAddr,
		"BROKER_PORT":         portOf(brokerHTTPAddr),
		"INTERNAL_API_SECRET": internalSecret,
	}); err != nil {
		return nil, fmt.Errorf("broker: %w", err)
	}
	if err := waitHTTP("http://"+brokerHTTPAddr+"/ping", 60*time.Second); err != nil {
		return nil, fmt.Errorf("broker HTTP: %w", err)
	}

	if err := waitForPipeline(mongoURI, "http://"+brokerHTTPAddr, 60*time.Second); err != nil {
		return nil, err
	}

	return &testStack{
		mongoURI:  mongoURI,
		rabbitURI: rabbitURI,
		brokerURL: "http://" + brokerHTTPAddr,
	}, nil
}

// --- MongoDB clients ---

func connectMongo(uri string) (*mongo.Client, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	return mongo.Connect(ctx, options.Client().ApplyURI(uri).
		SetAuth(options.Credential{Username: mongoUser, Password: mongoPass}))
}

// testMongo returns a client for uri, disconnected when t finishes.
func testMongo(t *testing.T, uri string) *mongo.Client {
	t.Helper()

	client, err := connectMongo(uri)
	if err != nil {
		t.Fatalf("mongo connect: %v", err)
	}
	t.Cleanup(func() { client.Disconnect(context.Background()) })

	return client
}

// testAPIKey seeds a valid API key directly into MongoDB and returns the plaintext.
// This bypasses the Broker so the integration test doesn't depend on key creation working.
func testAPIKey(t *testing.T, mongoURI string) string {
	t.Helper()

	return seedAPIKey(t, mongoURI, testProjectID, "lw_integrationtestkey0000000001")
}

// seedAPIKey inserts an API key scoped to projectID and returns the plaintext key.
func seedAPIKey(t *testing.T, mongoURI, projectID, plaintext string) string {
	t.Helper()

	if err := insertAPIKey(mongoURI, projectID, plaintext); err != nil {
		t.Fatalf("seedAPIKey: %v", err)
	}
	return plaintext
}

func insertAPIKey(mongoURI, projectID, plaintext string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	hash, err := bcrypt.GenerateFromPassword([]byte(plaintext), bcrypt.DefaultCost)
	if err != nil {
		return fmt.Errorf("bcrypt: %w", err)
	}

	client, err := connectMongo(mongoURI)
	if err != nil {
		return fmt.Errorf("connect: %w", err)
	}
	defer client.Disconnect(context.Background())

	_, err = client.Database("logs").Collection("api_keys").InsertOne(ctx, bson.M{
		"project_id": projectID,
		"prefix":     plaintext[:10],
		"hash":       string(hash),
		"active":     true,
		"created_at": time.Now(),
	})
	if err != nil {
		return fmt.Errorf("insert: %w", err)
	}

	return nil
}

// --- processes ---

// spawn runs a service as a subprocess for the rest of the package's life.
// Output goes nowhere unless LOGWOLF_TEST_VERBOSE is set — a shared stack that
// fails to come up takes every test with it, so the escape hatch is worth having.
func spawn(pkgPath string, env map[string]string) error {
	cmd := exec.Command("go", "run", pkgPath)
	cmd.Env = append(os.Environ(), envSlice(env)...)

	if os.Getenv("LOGWOLF_TEST_VERBOSE") != "" {
		cmd.Stdout = os.Stderr
		cmd.Stderr = os.Stderr
	}

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("start %s: %w", pkgPath, err)
	}

	onTeardown(func() {
		cmd.Process.Kill()
		cmd.Wait()
	})

	return nil
}

// startProcess is spawn scoped to a single test.
func startProcess(t *testing.T, pkgPath string, env map[string]string) {
	t.Helper()

	cmd := exec.Command("go", "run", pkgPath)
	cmd.Env = append(os.Environ(), envSlice(env)...)

	if os.Getenv("LOGWOLF_TEST_VERBOSE") != "" {
		cmd.Stdout = os.Stderr
		cmd.Stderr = os.Stderr
	}

	if err := cmd.Start(); err != nil {
		t.Fatalf("startProcess %s: %v", pkgPath, err)
	}

	t.Cleanup(func() {
		cmd.Process.Kill()
		cmd.Wait()
	})
}

// --- readiness ---

func waitTCP(addr string, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		conn, err := net.DialTimeout("tcp", addr, 500*time.Millisecond)
		if err == nil {
			conn.Close()
			return nil
		}
		time.Sleep(200 * time.Millisecond)
	}
	return fmt.Errorf("%s not ready after %s", addr, timeout)
}

func waitHTTP(url string, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		resp, err := http.Get(url)
		if err == nil {
			resp.Body.Close()
			if resp.StatusCode == http.StatusOK {
				return nil
			}
		}
		time.Sleep(200 * time.Millisecond)
	}
	return fmt.Errorf("%s not ready after %s", url, timeout)
}

// waitForPipeline publishes canary events until one comes back out of MongoDB.
//
// The Listener declares and binds the queue at its own pace after startup, and
// a topic exchange drops what it has no binding for — so a Broker answering
// /ping is not yet proof that an accepted event will be stored anywhere. Every
// test that posts an event depends on this having happened, and there is no
// endpoint that reports it, so the probe drives the real path instead.
func waitForPipeline(mongoURI, brokerURL string, timeout time.Duration) error {
	const (
		canaryProject = "stack-canary"
		canaryKey     = "lw_stackcanary000000001"
		canaryEvent   = "stack-canary-event"
	)

	if err := insertAPIKey(mongoURI, canaryProject, canaryKey); err != nil {
		return fmt.Errorf("pipeline probe: seed key: %w", err)
	}

	client, err := connectMongo(mongoURI)
	if err != nil {
		return fmt.Errorf("pipeline probe: connect: %w", err)
	}
	defer client.Disconnect(context.Background())

	coll := client.Database("logs").Collection("logs")
	body, _ := json.Marshal(map[string]any{
		"name": canaryEvent, "data": "{}", "severity": "info", "tags": []string{},
	})

	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		req, err := http.NewRequest(http.MethodPost, brokerURL+"/logs", bytes.NewReader(body))
		if err != nil {
			return fmt.Errorf("pipeline probe: %w", err)
		}
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+canaryKey)

		if resp, err := http.DefaultClient.Do(req); err == nil {
			resp.Body.Close()
		}

		// Give the event a moment to travel before publishing another.
		for i := 0; i < 6 && time.Now().Before(deadline); i++ {
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			n, err := coll.CountDocuments(ctx, bson.M{"name": canaryEvent})
			cancel()
			if err == nil && n > 0 {
				return nil
			}
			time.Sleep(500 * time.Millisecond)
		}
	}

	return fmt.Errorf("pipeline probe: no canary event reached MongoDB within %s", timeout)
}

func waitForTCP(t *testing.T, addr string, timeout time.Duration) {
	t.Helper()
	if err := waitTCP(addr, timeout); err != nil {
		t.Fatalf("waitForTCP: %v", err)
	}
}

// --- addresses ---

func freeAddress() (string, error) {
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return "", err
	}
	defer l.Close()

	return l.Addr().String(), nil
}

func freeAddr(t *testing.T) string {
	t.Helper()

	addr, err := freeAddress()
	if err != nil {
		t.Fatalf("freeAddr: %v", err)
	}
	return addr
}

func portOf(addr string) string {
	_, port, _ := net.SplitHostPort(addr)
	return port
}

func envSlice(m map[string]string) []string {
	var out []string
	for k, v := range m {
		out = append(out, fmt.Sprintf("%s=%s", k, v))
	}
	return out
}
