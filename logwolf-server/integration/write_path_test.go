//go:build integration

package integration

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"testing"
	"time"

	"go.mongodb.org/mongo-driver/bson"
)

// TestWritePathRoundTrip drives an event through the whole write path —
// Broker HTTP, RabbitMQ, Listener, Logger RPC, MongoDB — and checks that it
// lands under the project the API key belongs to.
func TestWritePathRoundTrip(t *testing.T) {
	ctx := context.Background()

	stack := sharedStack(t)
	apiKey := testAPIKey(t, stack.mongoURI)

	payload := map[string]interface{}{
		"name":     "integration-test-event",
		"data":     `{"test":true}`,
		"severity": "info",
		"tags":     []string{"integration"},
	}
	body, _ := json.Marshal(payload)

	req, _ := http.NewRequest(http.MethodPost, stack.brokerURL+"/logs", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+apiKey)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("POST /logs: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusAccepted {
		respBody, _ := io.ReadAll(resp.Body)
		t.Fatalf("POST /logs: expected 202, got %d: %s", resp.StatusCode, respBody)
	}

	collection := testMongo(t, stack.mongoURI).Database("logs").Collection("logs")

	// Delivery is asynchronous, so poll until the entry shows up.
	deadline := time.Now().Add(15 * time.Second)
	for time.Now().Before(deadline) {
		count, err := collection.CountDocuments(ctx, bson.M{"name": "integration-test-event"})
		if err == nil && count > 0 {
			break
		}
		time.Sleep(300 * time.Millisecond)
	}

	var doc struct {
		ProjectID string `bson:"project_id"`
	}
	err = collection.FindOne(ctx, bson.M{"name": "integration-test-event"}).Decode(&doc)
	if err != nil {
		t.Fatalf("log entry never appeared in MongoDB: %v", err)
	}
	if doc.ProjectID != testProjectID {
		t.Errorf("project_id = %q, want %q", doc.ProjectID, testProjectID)
	}
}

// TestWritePathBatch_ScopedToKeysProject covers the batch endpoint and the
// property that makes it safe to expose: the project comes from the API key,
// so a caller cannot file events against a project it has no key for.
func TestWritePathBatch_ScopedToKeysProject(t *testing.T) {
	ctx := context.Background()

	stack := sharedStack(t)
	apiKey := seedAPIKey(t, stack.mongoURI, "batch-project", "lw_batchkey0000000001")

	batch := []map[string]interface{}{
		{"name": "batch-event-1", "data": "{}", "severity": "info", "tags": []string{}},
		{"name": "batch-event-2", "data": "{}", "severity": "warning", "tags": []string{}},
		// This one claims to belong somewhere else. The Broker overwrites the
		// field with the key's project rather than trusting the body.
		{"name": "batch-event-3", "data": "{}", "severity": "error", "tags": []string{}, "project_id": "victim-project"},
	}
	body, _ := json.Marshal(batch)

	req, _ := http.NewRequest(http.MethodPost, stack.brokerURL+"/logs/batch", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+apiKey)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("POST /logs/batch: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusAccepted {
		respBody, _ := io.ReadAll(resp.Body)
		t.Fatalf("POST /logs/batch: expected 202, got %d: %s", resp.StatusCode, respBody)
	}

	coll := testMongo(t, stack.mongoURI).Database("logs").Collection("logs")

	deadline := time.Now().Add(20 * time.Second)
	for time.Now().Before(deadline) {
		count, err := coll.CountDocuments(ctx, bson.M{"project_id": "batch-project"})
		if err == nil && count == int64(len(batch)) {
			break
		}
		time.Sleep(300 * time.Millisecond)
	}

	count, err := coll.CountDocuments(ctx, bson.M{"project_id": "batch-project"})
	if err != nil {
		t.Fatalf("count batch entries: %v", err)
	}
	if count != int64(len(batch)) {
		t.Errorf("batch landed %d entr(ies) under batch-project, want %d", count, len(batch))
	}

	// The forged project_id must not have created anything.
	stray, err := coll.CountDocuments(ctx, bson.M{"project_id": "victim-project"})
	if err != nil {
		t.Fatalf("count forged entries: %v", err)
	}
	if stray != 0 {
		t.Errorf("body-supplied project_id was honoured: %d entr(ies) landed under victim-project", stray)
	}

	// And the key reads back exactly its own events.
	names := getLogs(t, stack.brokerURL, apiKey)
	for _, want := range []string{"batch-event-1", "batch-event-2", "batch-event-3"} {
		if !containsName(names, want) {
			t.Errorf("GET /logs did not return %q: %v", want, names)
		}
	}
}
