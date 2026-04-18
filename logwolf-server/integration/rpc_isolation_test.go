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
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"golang.org/x/crypto/bcrypt"
)

// seedAPIKey inserts an API key scoped to projectID and returns the plaintext key.
func seedAPIKey(t *testing.T, mongoURI, projectID, plaintext string) string {
	t.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	client, err := mongo.Connect(ctx, options.Client().ApplyURI(mongoURI).
		SetAuth(options.Credential{Username: "admin", Password: "password"}))
	if err != nil {
		t.Fatalf("seedAPIKey: connect: %v", err)
	}
	t.Cleanup(func() { client.Disconnect(context.Background()) })

	hash, err := bcrypt.GenerateFromPassword([]byte(plaintext), bcrypt.DefaultCost)
	if err != nil {
		t.Fatalf("seedAPIKey: bcrypt: %v", err)
	}

	_, err = client.Database("logs").Collection("api_keys").InsertOne(ctx, bson.M{
		"project_id": projectID,
		"prefix":     plaintext[:10],
		"hash":       string(hash),
		"active":     true,
		"created_at": time.Now(),
	})
	if err != nil {
		t.Fatalf("seedAPIKey: insert: %v", err)
	}

	return plaintext
}

// postLog sends a single log entry to the broker and asserts a 202 response.
func postLog(t *testing.T, brokerURL, apiKey, name string) {
	t.Helper()

	body, _ := json.Marshal(map[string]interface{}{
		"name":     name,
		"data":     `{}`,
		"severity": "info",
		"tags":     []string{},
	})

	req, _ := http.NewRequest(http.MethodPost, brokerURL+"/logs", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+apiKey)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("postLog %q: %v", name, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusAccepted {
		b, _ := io.ReadAll(resp.Body)
		t.Fatalf("postLog %q: expected 202, got %d: %s", name, resp.StatusCode, b)
	}
}

// waitForLog polls MongoDB until a document with the given name appears.
func waitForLog(t *testing.T, mongoURI, name string) {
	t.Helper()

	ctx := context.Background()
	client, err := mongo.Connect(ctx, options.Client().ApplyURI(mongoURI).
		SetAuth(options.Credential{Username: "admin", Password: "password"}))
	if err != nil {
		t.Fatalf("waitForLog: connect: %v", err)
	}
	defer client.Disconnect(ctx)

	coll := client.Database("logs").Collection("logs")
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		count, _ := coll.CountDocuments(ctx, bson.M{"name": name})
		if count > 0 {
			return
		}
		time.Sleep(300 * time.Millisecond)
	}
	t.Fatalf("waitForLog: %q never appeared in MongoDB", name)
}

// getLogs calls GET /logs on the broker with the given API key and decodes the
// log entry names from the response.
func getLogs(t *testing.T, brokerURL, apiKey string) []string {
	t.Helper()

	req, _ := http.NewRequest(http.MethodGet, brokerURL+"/logs", nil)
	req.Header.Set("Authorization", "Bearer "+apiKey)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("getLogs: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resp.Body)
		t.Fatalf("getLogs: expected 200, got %d: %s", resp.StatusCode, b)
	}

	var envelope struct {
		Data []struct {
			Name string `json:"name"`
		} `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&envelope); err != nil {
		t.Fatalf("getLogs: decode: %v", err)
	}

	names := make([]string, 0, len(envelope.Data))
	for _, e := range envelope.Data {
		names = append(names, e.Name)
	}
	return names
}

// TestProjectIsolation_GetLogs verifies that logs written under project A are
// not visible when querying as project B, and vice versa.
func TestProjectIsolation_GetLogs(t *testing.T) {
	ctx := context.Background()

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

	brokerURL := startStack(t, mongoURI, rabbitURI)

	keyA := seedAPIKey(t, mongoURI, "project-alpha", "lw_alphakey0000000001")
	keyB := seedAPIKey(t, mongoURI, "project-beta0", "lw_betakey00000000001")

	postLog(t, brokerURL, keyA, "alpha-event")
	postLog(t, brokerURL, keyB, "beta-event")

	waitForLog(t, mongoURI, "alpha-event")
	waitForLog(t, mongoURI, "beta-event")

	// Project A should see its own log and not project B's.
	logsA := getLogs(t, brokerURL, keyA)
	t.Logf("project-alpha logs: %v", logsA)

	if !containsName(logsA, "alpha-event") {
		t.Error("project-alpha: expected to see alpha-event, but did not")
	}
	if containsName(logsA, "beta-event") {
		t.Error("project-alpha: must not see beta-event from project-beta")
	}

	// Project B should see its own log and not project A's.
	logsB := getLogs(t, brokerURL, keyB)
	t.Logf("project-beta logs: %v", logsB)

	if !containsName(logsB, "beta-event") {
		t.Error("project-beta: expected to see beta-event, but did not")
	}
	if containsName(logsB, "alpha-event") {
		t.Error("project-beta: must not see alpha-event from project-alpha")
	}
}

// TestProjectIsolation_DeleteLog verifies that a DELETE /logs request scoped to
// project A does not remove logs belonging to project B.
func TestProjectIsolation_DeleteLog(t *testing.T) {
	ctx := context.Background()

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

	brokerURL := startStack(t, mongoURI, rabbitURI)

	keyA := seedAPIKey(t, mongoURI, "del-alpha", "lw_delalpha0000000001")
	keyB := seedAPIKey(t, mongoURI, "del-beta00", "lw_delbeta00000000001")

	postLog(t, brokerURL, keyA, "del-alpha-event")
	postLog(t, brokerURL, keyB, "del-beta-event")

	waitForLog(t, mongoURI, "del-alpha-event")
	waitForLog(t, mongoURI, "del-beta-event")

	// Fetch project A's log ID so we can target it for deletion.
	logIDA := fetchLogID(t, mongoURI, "del-alpha-event")

	// Delete the log as project A.
	deleteLog(t, brokerURL, keyA, logIDA)

	// Project A's log must be gone.
	logsA := getLogs(t, brokerURL, keyA)
	if containsName(logsA, "del-alpha-event") {
		t.Error("del-alpha-event should have been deleted but is still present")
	}

	// Project B's log must be unaffected.
	logsB := getLogs(t, brokerURL, keyB)
	if !containsName(logsB, "del-beta-event") {
		t.Error("del-beta-event was deleted but should not have been")
	}
}

// --- helpers ---

func containsName(names []string, target string) bool {
	for _, n := range names {
		if n == target {
			return true
		}
	}
	return false
}

// fetchLogID reads the _id of the first log entry matching name directly from MongoDB.
func fetchLogID(t *testing.T, mongoURI, name string) string {
	t.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	client, err := mongo.Connect(ctx, options.Client().ApplyURI(mongoURI).
		SetAuth(options.Credential{Username: "admin", Password: "password"}))
	if err != nil {
		t.Fatalf("fetchLogID: connect: %v", err)
	}
	defer client.Disconnect(ctx)

	var doc struct {
		ID primitive.ObjectID `bson:"_id"`
	}
	err = client.Database("logs").Collection("logs").
		FindOne(ctx, bson.M{"name": name}).Decode(&doc)
	if err != nil {
		t.Fatalf("fetchLogID: find %q: %v", name, err)
	}

	return doc.ID.Hex()
}

// deleteLog sends DELETE /logs with the given log ID scoped to apiKey's project.
func deleteLog(t *testing.T, brokerURL, apiKey, logID string) {
	t.Helper()

	body, _ := json.Marshal(map[string]string{"id": logID})
	req, _ := http.NewRequest(http.MethodDelete, brokerURL+"/logs", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+apiKey)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("deleteLog: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusAccepted {
		b, _ := io.ReadAll(resp.Body)
		t.Fatalf("deleteLog: expected 202, got %d: %s", resp.StatusCode, b)
	}
}
