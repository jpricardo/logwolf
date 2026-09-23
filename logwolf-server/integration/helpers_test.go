//go:build integration

package integration

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/rabbitmq"
	"github.com/testcontainers/testcontainers-go/wait"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"golang.org/x/crypto/bcrypt"
)

const (
	mongoUser  = "admin"
	mongoPass  = "password"
	replicaSet = "rs0"

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
// hang off any one test's t.Cleanup. The binaries built for the run go last,
// once nothing is still executing them.
func TestMain(m *testing.M) {
	code := m.Run()

	teardownMu.Lock()
	fns := teardowns
	teardownMu.Unlock()

	for i := len(fns) - 1; i >= 0; i-- {
		fns[i]()
	}

	if binDir != "" {
		os.RemoveAll(binDir)
	}

	os.Exit(code)
}

// --- containers ---

// startMongo launches a MongoDB container with the credentials the services
// expect and returns its connection URI. The container lives until the package
// finishes.
//
// It runs as a single-member replica set, as it does in docker-compose.yml,
// because the data layer uses transactions and a standalone mongod refuses them.
// The testcontainers mongodb module can do this too, but not for this image:
// its readiness check waits for a log line that MongoDB 4.2 spells in lower
// case, and it may run rs.initiate against the entrypoint's temporary init
// server rather than the real one. Initiating from here instead, over the
// mapped port, only ever reaches the real server.
//
// Test commands are enabled so tests can inject failures with the failCommand
// fail point.
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
			// A replica set with auth needs a keyfile. The only member it
			// authenticates to is itself, so a fresh one per container is fine.
			Entrypoint: []string{"bash", "-c", `head -c 756 /dev/urandom | base64 > /tmp/keyfile &&
				chown mongodb:mongodb /tmp/keyfile && chmod 400 /tmp/keyfile &&
				exec docker-entrypoint.sh "$@"`, "--"},
			Cmd: []string{
				"mongod", "--replSet", replicaSet, "--keyFile", "/tmp/keyfile", "--bind_ip_all",
				"--setParameter", "enableTestCommands=1",
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

	// The member is registered as localhost:27017, which only means something
	// inside the container, so clients must not go looking for it: they talk to
	// the mapped port directly.
	uri := fmt.Sprintf("mongodb://%s:%s@%s:%s/?directConnection=true", mongoUser, mongoPass, host, port.Port())

	if err := initiateReplicaSet(uri, 60*time.Second); err != nil {
		return "", err
	}
	return uri, nil
}

// initiateReplicaSet turns the single mongod at uri into a replica set and waits
// until it has elected itself primary, which is when it starts accepting writes.
func initiateReplicaSet(uri string, timeout time.Duration) error {
	client, err := connectMongo(uri)
	if err != nil {
		return fmt.Errorf("replica set: connect: %w", err)
	}
	defer client.Disconnect(context.Background())

	admin := client.Database("admin")
	config := bson.M{
		"_id":     replicaSet,
		"members": bson.A{bson.M{"_id": 0, "host": "localhost:27017"}},
	}

	var lastErr error
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		lastErr = admin.RunCommand(ctx, bson.D{{Key: "replSetInitiate", Value: config}}).Err()
		cancel()

		var cmdErr mongo.CommandError
		if lastErr == nil || errors.As(lastErr, &cmdErr) && cmdErr.Name == "AlreadyInitialized" {
			break
		}
		time.Sleep(500 * time.Millisecond)
	}

	for time.Now().Before(deadline) {
		var hello struct {
			IsMaster bool `bson:"ismaster"`
		}
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		lastErr = admin.RunCommand(ctx, bson.D{{Key: "isMaster", Value: 1}}).Decode(&hello)
		cancel()

		if lastErr == nil && hello.IsMaster {
			return nil
		}
		time.Sleep(250 * time.Millisecond)
	}

	return fmt.Errorf("replica set: no primary after %s (last error: %v)", timeout, lastErr)
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
	mongoURI      string
	rabbitURI     string
	brokerURL     string
	loggerRPCAddr string
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
		mongoURI:      mongoURI,
		rabbitURI:     rabbitURI,
		brokerURL:     "http://" + brokerHTTPAddr,
		loggerRPCAddr: loggerRPCAddr,
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

// testAPIKey seeds a project and a valid API key for it directly into MongoDB,
// and returns the plaintext key and the project id. This bypasses the Broker so
// the integration test doesn't depend on project or key creation working.
func testAPIKey(t *testing.T, mongoURI string) (key, projectID string) {
	t.Helper()

	projectID = seedProject(t, mongoURI, "integration")
	return seedAPIKey(t, mongoURI, projectID, "lw_integrationtestkey0000000000000000000000001"), projectID
}

// seedProject inserts a project with the given slug and returns its id. Logger
// drops events for a project that does not exist, so every key a test sends
// events with has to belong to one.
func seedProject(t *testing.T, mongoURI, slug string) string {
	t.Helper()

	id, err := insertProject(mongoURI, slug)
	if err != nil {
		t.Fatalf("seedProject: %v", err)
	}
	return id
}

func insertProject(mongoURI, slug string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	client, err := connectMongo(mongoURI)
	if err != nil {
		return "", fmt.Errorf("connect: %w", err)
	}
	defer client.Disconnect(context.Background())

	id := primitive.NewObjectID()
	_, err = client.Database("logs").Collection("projects").InsertOne(ctx, bson.M{
		"_id":        id,
		"name":       slug,
		"slug":       slug,
		"created_at": time.Now(),
	})
	if err != nil {
		return "", fmt.Errorf("insert: %w", err)
	}
	return id.Hex(), nil
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
	bin, err := buildService(pkgPath)
	if err != nil {
		return err
	}

	cmd := exec.Command(bin)
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

var (
	binMu   sync.Mutex
	binDir  string
	binPath = map[string]string{}
)

// buildService compiles a service package once per run and returns the path to
// the binary. Services run from that binary directly rather than through
// `go run`, which is what makes cleanup reliable: `go run` starts the service as
// a child of its own, so killing `go run` leaves the service running. On Windows
// it survives the whole session, and a stray Listener that reconnects to a
// recycled RabbitMQ port will consume the messages a later test is waiting for.
// TestMain deletes the binaries once every test has finished.
func buildService(pkgPath string) (string, error) {
	binMu.Lock()
	defer binMu.Unlock()

	if path, ok := binPath[pkgPath]; ok {
		return path, nil
	}

	if binDir == "" {
		dir, err := os.MkdirTemp("", "logwolf-integration")
		if err != nil {
			return "", fmt.Errorf("build %s: temp dir: %w", pkgPath, err)
		}
		binDir = dir
	}

	out := filepath.Join(binDir, strings.NewReplacer("../", "", "/", "-").Replace(pkgPath))
	if runtime.GOOS == "windows" {
		out += ".exe"
	}

	if output, err := exec.Command("go", "build", "-o", out, pkgPath).CombinedOutput(); err != nil {
		return "", fmt.Errorf("build %s: %w: %s", pkgPath, err, output)
	}

	binPath[pkgPath] = out
	return out, nil
}

// startProcess is spawn scoped to a single test.
func startProcess(t *testing.T, pkgPath string, env map[string]string) {
	t.Helper()

	bin, err := buildService(pkgPath)
	if err != nil {
		t.Fatalf("startProcess: %v", err)
	}

	cmd := exec.Command(bin)
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
		canaryKey     = "lw_stackcanary00000000000000000000000000000001"
		canaryEvent   = "stack-canary-event"
	)

	projectID, err := insertProject(mongoURI, canaryProject)
	if err != nil {
		return fmt.Errorf("pipeline probe: seed project: %w", err)
	}
	if err := insertAPIKey(mongoURI, projectID, canaryKey); err != nil {
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
