//go:build integration

package integration

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"testing"
	"time"

	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/rabbitmq"
	"github.com/testcontainers/testcontainers-go/wait"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

func TestWritePathRoundTrip(t *testing.T) {
	ctx := context.Background()

	// --- Start containers ---
	mongoC, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: testcontainers.ContainerRequest{
			Image:        "mongo:4.2.16-bionic",
			ExposedPorts: []string{"27017/tcp"},
			Env: map[string]string{
				"MONGO_INITDB_ROOT_USERNAME": "admin",
				"MONGO_INITDB_ROOT_PASSWORD": "password",
			},
			WaitingFor: wait.ForLog("waiting for connections on port 27017"),
		},
		Started: true,
	})
	if err != nil {
		t.Fatalf("mongo container: %v", err)
	}
	defer mongoC.Terminate(ctx)

	mongoHost, _ := mongoC.Host(ctx)
	mongoPort, _ := mongoC.MappedPort(ctx, "27017")
	mongoURI := fmt.Sprintf("mongodb://admin:password@%s:%s", mongoHost, mongoPort.Port())

	rabbitC, err := rabbitmq.Run(ctx, "rabbitmq:3.9-alpine")
	if err != nil {
		t.Fatalf("rabbitmq container: %v", err)
	}
	defer rabbitC.Terminate(ctx)

	rabbitURI, _ := rabbitC.AmqpURL(ctx)

	// --- Start Logger, Listener, Broker in-process (using env vars) ---
	// Wire up the stack pointing at test containers, then POST a log entry
	// and poll MongoDB until it appears.
	//
	// In practice the three services are separate binaries, so this test
	// invokes them as subprocesses with environment overrides, or you extract
	// the wiring into testable packages. The simplest approach that matches
	// the existing architecture: run the stack via docker-compose with
	// overridden env vars and drive it via HTTP.
	//
	// Here we use direct Mongo polling as the assertion — no need to go
	// through the Broker GET /logs endpoint for the round-trip assertion.

	brokerURL := startStack(t, mongoURI, rabbitURI)

	// --- POST a log entry via Broker HTTP ---
	payload := map[string]interface{}{
		"name":     "integration-test-event",
		"data":     `{"test":true}`,
		"severity": "info",
		"tags":     []string{"integration"},
	}
	body, _ := json.Marshal(payload)

	req, _ := http.NewRequest(http.MethodPost, brokerURL+"/logs", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+testAPIKey(t, mongoURI))

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("POST /logs: %v", err)
	}
	if resp.StatusCode != http.StatusAccepted {
		t.Fatalf("expected 202, got %d", resp.StatusCode)
	}

	// --- Verify POST response body for early diagnosis ---
	respBody, _ := io.ReadAll(resp.Body)
	t.Logf("POST /logs response: %s", string(respBody))

	// --- Poll MongoDB with progress logging ---
	client, err := mongo.Connect(ctx, options.Client().ApplyURI(mongoURI).
		SetAuth(options.Credential{Username: "admin", Password: "password"}))
	if err != nil {
		t.Fatalf("mongo connect: %v", err)
	}
	defer client.Disconnect(ctx)

	collection := client.Database("logs").Collection("logs")
	deadline := time.Now().Add(10 * time.Second)
	attempt := 0

	for time.Now().Before(deadline) {
		attempt++
		count, err := collection.CountDocuments(ctx, bson.M{"name": "integration-test-event"})
		t.Logf("attempt %d: count=%d err=%v", attempt, count, err)
		if err == nil && count > 0 {
			return // success
		}
		time.Sleep(500 * time.Millisecond)
	}

	t.Fatal("log entry never appeared in MongoDB")
}
