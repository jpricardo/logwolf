//go:build integration

package integration

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
)

// TestProjectDeleteCascade_EndToEnd builds a project the way the dashboard does
// — create it, mint a key, set retention, send events through the SDK path —
// then deletes it and checks that nothing of it survives in MongoDB.
//
// The data-layer cascade is covered by TestDeleteProject_Cascade; this one
// exists because the delete has to reach that code through the Broker's
// ownership check and the Logger RPC, and because the rows it must remove are
// written here by the running services rather than seeded by the test.
func TestProjectDeleteCascade_EndToEnd(t *testing.T) {
	stack := sharedStack(t)
	client := testMongo(t, stack.mongoURI)
	db := client.Database("logs")

	const owner = "cascade-owner"

	// --- Create the project (the caller becomes its owner) ---

	data := mustInternalCall(t, stack.brokerURL, http.MethodPost, "/projects", owner,
		map[string]string{"name": "Cascade", "slug": "cascade"}, http.StatusCreated)

	var project struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(data, &project); err != nil {
		t.Fatalf("decode created project: %v", err)
	}
	if project.ID == "" {
		t.Fatalf("created project has no id: %s", data)
	}

	// --- Mint an API key for it, able to read back as well as ingest ---

	data = mustInternalCall(t, stack.brokerURL, http.MethodPost, "/projects/"+project.ID+"/keys", owner,
		map[string]any{"scopes": []string{"ingest", "read"}}, http.StatusCreated)

	var created struct {
		Key string `json:"key"`
	}
	if err := json.Unmarshal(data, &created); err != nil {
		t.Fatalf("decode created key: %v", err)
	}
	if created.Key == "" {
		t.Fatalf("created key is empty: %s", data)
	}

	// --- Set retention, so a settings document exists ---

	mustInternalCall(t, stack.brokerURL, http.MethodPatch, "/projects/"+project.ID+"/retention", owner,
		map[string]any{"days": 30}, http.StatusOK)

	// --- Send events with that key ---

	postLog(t, stack.brokerURL, created.Key, "cascade-event-1")
	postLog(t, stack.brokerURL, created.Key, "cascade-event-2")
	waitForLog(t, stack.mongoURI, "cascade-event-1")
	waitForLog(t, stack.mongoURI, "cascade-event-2")

	// --- Everything is in place before the delete ---

	// Every project_id is an ObjectID, so these filters match what the cascade
	// deletes; a hex string here would count nothing and prove nothing.
	projectID := oid(t, project.ID)
	counts := map[string]func() int64{
		"projects":        func() int64 { return countDocs(t, db.Collection("projects"), bson.M{"_id": projectID}) },
		"project_members": func() int64 { return countDocs(t, db.Collection("project_members"), bson.M{"project_id": projectID}) },
		"api_keys":        func() int64 { return countDocs(t, db.Collection("api_keys"), bson.M{"project_id": projectID}) },
		"settings":        func() int64 { return countDocs(t, db.Collection("settings"), bson.M{"project_id": projectID}) },
		"logs":            func() int64 { return countDocs(t, db.Collection("logs"), bson.M{"project_id": projectID}) },
	}

	for name, count := range counts {
		if n := count(); n == 0 {
			t.Fatalf("precondition: %s has no rows for the project, so its removal would prove nothing", name)
		}
	}

	// --- Delete the project ---

	mustInternalCall(t, stack.brokerURL, http.MethodDelete, "/projects/"+project.ID, owner, nil, http.StatusOK)

	// Everything but the logs is deleted before the RPC returns. The logs are
	// purged by the Logger's cleanup loop right after, so poll for them. The
	// hourly orphan sweep never runs in this window, so this also covers the
	// purge the delete hands to the loop.
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		clean := true
		for _, count := range counts {
			if count() != 0 {
				clean = false
				break
			}
		}
		if clean {
			break
		}
		time.Sleep(300 * time.Millisecond)
	}

	for name, count := range counts {
		if n := count(); n != 0 {
			t.Errorf("after project delete: %s still holds %d row(s) for the project", name, n)
		}
	}

	// --- The key it minted no longer authenticates ---
	//
	// The Broker caches key lookups for 60s, but the one that deleted the
	// project dropped the project's keys from its cache.
	req, _ := http.NewRequest(http.MethodGet, stack.brokerURL+"/logs", nil)
	req.Header.Set("Authorization", "Bearer "+created.Key)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("GET /logs with the deleted project's key: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("GET /logs with the deleted project's key: got %d, want 401", resp.StatusCode)
	}

	// --- Deleting it again is a 404, not a second success ---

	status, _ := internalCall(t, stack.brokerURL, http.MethodDelete, "/projects/"+project.ID, owner, nil)
	if status != http.StatusNotFound {
		t.Errorf("deleting an already-deleted project: got %d, want 404", status)
	}
}

func countDocs(t *testing.T, coll *mongo.Collection, filter bson.M) int64 {
	t.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	n, err := coll.CountDocuments(ctx, filter)
	if err != nil {
		t.Fatalf("count %s %v: %v", coll.Name(), filter, err)
	}
	return n
}
