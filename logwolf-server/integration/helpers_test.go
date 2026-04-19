//go:build integration

package integration

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/exec"
	"testing"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"golang.org/x/crypto/bcrypt"
)

const testProjectID = "integration"

// startStack launches Logger, Listener, and Broker as subprocesses pointed at
// the test containers. It registers t.Cleanup to kill them on test exit.
// Returns the Broker's base URL (e.g. "http://127.0.0.1:18080").
func startStack(t *testing.T, mongoURI, rabbitURI string) string {
	t.Helper()

	loggerRPCAddr := freeAddr(t)
	loggerHTTPAddr := freeAddr(t)
	brokerHTTPAddr := freeAddr(t)

	t.Logf("loggerRPCAddr=%s loggerHTTPAddr=%s brokerHTTPAddr=%s", loggerRPCAddr, loggerHTTPAddr, brokerHTTPAddr)

	t.Log("starting Logger...")
	startProcess(t, "../logger/cmd/api", map[string]string{
		"MONGO_URL":        mongoURI,
		"LOGGER_RPC_PORT":  portOf(loggerRPCAddr),
		"LOGGER_HTTP_PORT": portOf(loggerHTTPAddr),
	})

	t.Log("waiting for Logger RPC...")
	waitForTCP(t, loggerRPCAddr, 30*time.Second)
	t.Log("Logger RPC ready")

	t.Log("starting Listener...")
	startProcess(t, "../listener/cmd/api", map[string]string{
		"RABBITMQ_URL":    rabbitURI,
		"LOGGER_RPC_ADDR": loggerRPCAddr,
	})

	t.Log("starting Broker...")
	startProcess(t, "../broker/cmd/api", map[string]string{
		"MONGO_URL":           mongoURI,
		"RABBITMQ_URL":        rabbitURI,
		"LOGGER_RPC_ADDR":     loggerRPCAddr,
		"BROKER_PORT":         portOf(brokerHTTPAddr),
		"INTERNAL_API_SECRET": "test-secret",
	})

	t.Log("waiting for Broker HTTP...")
	waitForHTTP(t, "http://"+brokerHTTPAddr+"/ping", 30*time.Second)
	t.Log("Broker HTTP ready")

	return "http://" + brokerHTTPAddr
}

// testAPIKey seeds a valid API key directly into MongoDB and returns the plaintext.
// This bypasses the Broker so the integration test doesn't depend on key creation working.
func testAPIKey(t *testing.T, mongoURI string) string {
	t.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	client, err := mongo.Connect(ctx, options.Client().ApplyURI(mongoURI).
		SetAuth(options.Credential{Username: "admin", Password: "password"}))
	if err != nil {
		t.Fatalf("testAPIKey: connect: %v", err)
	}
	t.Cleanup(func() { client.Disconnect(context.Background()) })

	plaintext := "lw_integrationtestkey0000000001"
	hash, err := bcrypt.GenerateFromPassword([]byte(plaintext), bcrypt.DefaultCost)
	if err != nil {
		t.Fatalf("testAPIKey: bcrypt: %v", err)
	}

	_, err = client.Database("logs").Collection("api_keys").InsertOne(ctx, bson.M{
		"project_id": testProjectID,
		"prefix":     plaintext[:10],
		"hash":       string(hash),
		"active":     true,
		"created_at": time.Now(),
	})
	if err != nil {
		t.Fatalf("testAPIKey: insert: %v", err)
	}

	return plaintext
}

// --- internal helpers ---

func startProcess(t *testing.T, pkgPath string, env map[string]string) {
	t.Helper()

	cmd := exec.Command("go", "run", pkgPath)
	cmd.Env = append(os.Environ(), envSlice(env)...)
	// Leave Stdout and Stderr nil — os/exec will use os.DevNull directly,
	// no pipes created, nothing to drain, cmd.Wait() returns immediately.

	if err := cmd.Start(); err != nil {
		t.Fatalf("startProcess %s: %v", pkgPath, err)
	}

	t.Cleanup(func() {
		cmd.Process.Kill()
		cmd.Wait()
	})
}

func waitForTCP(t *testing.T, addr string, timeout time.Duration) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		conn, err := net.DialTimeout("tcp", addr, 500*time.Millisecond)
		if err == nil {
			conn.Close()
			return
		}
		time.Sleep(200 * time.Millisecond)
	}
	t.Fatalf("waitForTCP: %s not ready after %s", addr, timeout)
}

func waitForHTTP(t *testing.T, url string, timeout time.Duration) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		resp, err := http.Get(url)
		if err == nil && resp.StatusCode == http.StatusOK {
			return
		}
		time.Sleep(200 * time.Millisecond)
	}
	t.Fatalf("waitForHTTP: %s not ready after %s", url, timeout)
}

func freeAddr(t *testing.T) string {
	t.Helper()
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("freeAddr: %v", err)
	}
	addr := l.Addr().String()
	l.Close()
	return addr
}

func portOf(addr string) string {
	_, port, _ := net.SplitHostPort(addr)
	return port
}

func baseEnv() []string {
	return os.Environ()
}

func envSlice(m map[string]string) []string {
	var out []string
	for k, v := range m {
		out = append(out, fmt.Sprintf("%s=%s", k, v))
	}
	return out
}
