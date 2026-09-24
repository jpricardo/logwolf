//go:build integration

package integration

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/go-connections/nat"
	amqp "github.com/rabbitmq/amqp091-go"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
)

// TestEvents_SurviveARabbitMQRestart follows two events that the Broker
// accepted into a RabbitMQ restart. Each step used to lose them:
//
//   - A is posted before any Listener has run. Only the Listener declared the
//     queue, and the exchange drops what no queue is bound for.
//   - RabbitMQ restarts with A queued. A was published non-persistent.
//   - B is posted after the restart. The Broker's one connection had died with
//     RabbitMQ, and it never dialed again.
//
// Then the Listener starts, and both must be stored.
func TestEvents_SurviveARabbitMQRestart(t *testing.T) {
	ctx := context.Background()
	mongoURI := dedicatedMongo(t)

	// A fixed host port, so the Broker's RABBITMQ_URL still points at RabbitMQ
	// after the restart; Docker would otherwise map a new one.
	rabbitAddr := freeAddr(t)
	rabbitC, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: testcontainers.ContainerRequest{
			Image:        rabbitImage,
			Hostname:     "rabbitmq",
			ExposedPorts: []string{"5672/tcp"},
			Env:          map[string]string{"RABBITMQ_DEFAULT_USER": "logwolf", "RABBITMQ_DEFAULT_PASS": "logwolf"},
			HostConfigModifier: func(hc *container.HostConfig) {
				hc.PortBindings = nat.PortMap{"5672/tcp": {{HostIP: "127.0.0.1", HostPort: portOf(rabbitAddr)}}}
			},
			WaitingFor: wait.ForLog("Server startup complete").WithStartupTimeout(2 * time.Minute),
		},
		Started: true,
	})
	if err != nil {
		t.Fatalf("rabbitmq container: %v", err)
	}
	t.Cleanup(func() { rabbitC.Terminate(context.Background()) })
	rabbitURI := "amqp://logwolf:logwolf@" + rabbitAddr + "/"

	projectID := seedProject(t, mongoURI, "durable-events")
	const keyStem = "lw_durableevents"
	key := seedAPIKey(t, mongoURI, projectID, keyStem+strings.Repeat("0", 46-len(keyStem)-1)+"1")

	loggerRPCAddr := freeAddr(t)
	startProcess(t, "../logger/cmd/api", map[string]string{
		"MONGO_URL":        mongoURI,
		"LOGGER_RPC_PORT":  portOf(loggerRPCAddr),
		"LOGGER_HTTP_PORT": portOf(freeAddr(t)),
		"CLEANUP_INTERVAL": "24h",
	})
	waitForTCP(t, loggerRPCAddr, 60*time.Second)

	brokerHTTPAddr := freeAddr(t)
	startProcess(t, "../broker/cmd/api", map[string]string{
		"RABBITMQ_URL":        rabbitURI,
		"LOGGER_RPC_ADDR":     loggerRPCAddr,
		"BROKER_PORT":         portOf(brokerHTTPAddr),
		"INTERNAL_API_SECRET": internalSecret,
	})
	brokerURL := "http://" + brokerHTTPAddr
	if err := waitHTTP(brokerURL+"/ping", 60*time.Second); err != nil {
		t.Fatalf("broker: %v", err)
	}

	// --- A, with no Listener anywhere ---
	postLog(t, brokerURL, key, "durable-before-restart")
	if n := queuedMessages(t, rabbitURI); n != 1 {
		t.Fatalf("logwolf_logs holds %d message(s) before the restart, want 1", n)
	}

	// --- RabbitMQ restarts ---
	stopTimeout := 30 * time.Second
	if err := rabbitC.Stop(ctx, &stopTimeout); err != nil {
		t.Fatalf("stop rabbitmq: %v", err)
	}
	if err := rabbitC.Start(ctx); err != nil {
		t.Fatalf("start rabbitmq: %v", err)
	}
	if n := queuedMessages(t, rabbitURI); n != 1 {
		t.Fatalf("logwolf_logs holds %d message(s) after the restart, want 1", n)
	}

	// --- B, through the Broker that was connected to the old RabbitMQ ---
	postEventually(t, brokerURL, key, "durable-after-restart", 30*time.Second)

	// --- The Listener stores both ---
	startProcess(t, "../listener/cmd/api", map[string]string{
		"RABBITMQ_URL":    rabbitURI,
		"LOGGER_RPC_ADDR": loggerRPCAddr,
	})
	waitForLog(t, mongoURI, "durable-before-restart")
	waitForLog(t, mongoURI, "durable-after-restart")
}

// queuedMessages reads how many messages wait in the Listener's queue.
func queuedMessages(t *testing.T, rabbitURI string) int {
	t.Helper()

	// Right after a restart the port may be open before RabbitMQ serves it.
	var conn *amqp.Connection
	var err error
	for deadline := time.Now().Add(60 * time.Second); ; {
		if conn, err = amqp.Dial(rabbitURI); err == nil || time.Now().After(deadline) {
			break
		}
		time.Sleep(500 * time.Millisecond)
	}
	if err != nil {
		t.Fatalf("dial rabbitmq: %v", err)
	}
	defer conn.Close()
	ch, err := conn.Channel()
	if err != nil {
		t.Fatalf("channel: %v", err)
	}
	defer ch.Close()

	q, err := ch.QueueDeclarePassive("logwolf_logs", true, false, false, false, nil)
	if err != nil {
		t.Fatalf("inspect logwolf_logs: %v", err)
	}
	return q.Messages
}

// postEventually posts an event until the Broker answers 202, for the moments
// right after RabbitMQ comes back.
func postEventually(t *testing.T, brokerURL, apiKey, name string, timeout time.Duration) {
	t.Helper()

	body, _ := json.Marshal(map[string]any{"name": name, "data": "{}", "severity": "info", "tags": []string{}})
	deadline := time.Now().Add(timeout)
	last := 0
	for time.Now().Before(deadline) {
		req, _ := http.NewRequest(http.MethodPost, brokerURL+"/logs", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+apiKey)
		if resp, err := http.DefaultClient.Do(req); err == nil {
			resp.Body.Close()
			last = resp.StatusCode
			if last == http.StatusAccepted {
				return
			}
		}
		time.Sleep(500 * time.Millisecond)
	}
	t.Fatalf("post %q: no 202 within %s (last status %d)", name, timeout, last)
}
