//go:build integration

package integration

import (
	"context"
	"encoding/json"
	"net/http"
	"net/rpc"
	"strings"
	"testing"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"

	"logwolf-toolbox/data"
)

// TestDeletedProject_LateEventsAreDropped deletes a project and then delivers
// events for it the two ways that can still happen: through the Broker with a
// key it has cached, and straight to the Logger the way the Listener hands over
// an event that was already queued. Neither may leave a log behind.
func TestDeletedProject_LateEventsAreDropped(t *testing.T) {
	stack := sharedStack(t)
	logs := testMongo(t, stack.mongoURI).Database("logs").Collection("logs")

	const owner = "late-event-owner"

	// --- A project with a key, used once so the Broker caches the key ---

	body := mustInternalCall(t, stack.brokerURL, http.MethodPost, "/projects", owner,
		map[string]string{"name": "Late events", "slug": "late-events"}, http.StatusCreated)
	var project struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(body, &project); err != nil || project.ID == "" {
		t.Fatalf("decode created project: %v (%s)", err, body)
	}

	body = mustInternalCall(t, stack.brokerURL, http.MethodPost, "/projects/"+project.ID+"/keys", owner,
		map[string]string{}, http.StatusCreated)
	var created struct {
		Key string `json:"key"`
	}
	if err := json.Unmarshal(body, &created); err != nil || created.Key == "" {
		t.Fatalf("decode created key: %v (%s)", err, body)
	}

	postLog(t, stack.brokerURL, created.Key, "late-before-delete")
	waitForLog(t, stack.mongoURI, "late-before-delete")

	// --- Delete it ---

	mustInternalCall(t, stack.brokerURL, http.MethodDelete, "/projects/"+project.ID, owner, nil, http.StatusOK)

	// --- Straight to the Logger, as the Listener does ---

	conn, err := rpc.Dial("tcp", stack.loggerRPCAddr)
	if err != nil {
		t.Fatalf("dial logger RPC: %v", err)
	}
	defer conn.Close()

	var reply string
	err = conn.Call("RPCServer.LogInfo", data.RPCLogPayload{
		ProjectID: project.ID,
		Name:      "late-via-rpc",
		Data:      "{}",
		Severity:  "info",
	}, &reply)
	if err == nil || !strings.Contains(err.Error(), "does not exist") {
		t.Errorf("LogInfo for a deleted project: want a \"does not exist\" error, got %v", err)
	}

	// --- Through the Broker, with the key it had cached ---

	req, _ := http.NewRequest(http.MethodPost, stack.brokerURL+"/logs",
		strings.NewReader(`{"name":"late-via-broker","data":"{}","severity":"info","tags":[]}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+created.Key)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("POST /logs with the deleted project's key: %v", err)
	}
	resp.Body.Close()

	// The Broker that deleted the project dropped its keys from the cache, so
	// the key is looked up again, and it went with the project.
	if resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("POST /logs with the deleted project's key: got %d, want 401", resp.StatusCode)
	}

	// --- Nothing of the project is left ---
	//
	// Checked for a couple of seconds rather than once, in case an event queued
	// before the delete is still in flight.
	deadline := time.Now().Add(2 * time.Second)
	for {
		if n := countDocs(t, logs, bson.M{"project_id": oid(t, project.ID)}); n != 0 {
			t.Fatalf("%d log(s) persisted for the deleted project", n)
		}
		if time.Now().After(deadline) {
			break
		}
		time.Sleep(300 * time.Millisecond)
	}
}

// TestDeleteOrphanedLogs covers the sweep that catches events which passed the
// Logger's project check just before their project was deleted.
func TestDeleteOrphanedLogs(t *testing.T) {
	m := setupProjectModels(t)
	logs := testMongo(t, sharedModelsMongo(t)).Database("logs").Collection("logs")
	ctx := context.Background()

	live, err := m.InsertProject(data.Project{Name: "Live", Slug: "live"})
	if err != nil {
		t.Fatalf("InsertProject: %v", err)
	}
	deleted := primitive.NewObjectID()
	old := time.Now().Add(-time.Hour)

	entry := func(name string, fields bson.M) bson.M {
		doc := bson.M{"name": name, "data": "{}", "severity": "info", "created_at": old, "updated_at": old}
		for k, v := range fields {
			doc[k] = v
		}
		return doc
	}
	if _, err := logs.InsertMany(ctx, []any{
		entry("live-log", bson.M{"project_id": live.ID}),
		// Stored the way builds before ConvertProjectIDs did: still its project's,
		// so it is waiting to be converted, not deleted.
		entry("live-unconverted", bson.M{"project_id": live.ID.Hex()}),
		entry("orphan-log-1", bson.M{"project_id": deleted}),
		entry("orphan-log-2", bson.M{"project_id": deleted}),
		entry("orphan-unconverted", bson.M{"project_id": deleted.Hex()}),
		entry("orphan-not-hex", bson.M{"project_id": "gone-project"}),
		// Written a moment ago: within the grace period, left for a later pass.
		entry("orphan-recent", bson.M{"project_id": deleted, "created_at": time.Now()}),
		// Pre-multi-tenancy logs: the startup migration's to adopt, not ours to delete.
		entry("legacy-missing", bson.M{}),
		entry("legacy-empty", bson.M{"project_id": ""}),
	}); err != nil {
		t.Fatalf("seed logs: %v", err)
	}

	got, err := m.DeleteOrphanedLogs(ctx)
	if err != nil {
		t.Fatalf("DeleteOrphanedLogs: %v", err)
	}
	if got[deleted.Hex()] != 3 || got["gone-project"] != 1 || len(got) != 2 {
		t.Errorf("DeleteOrphanedLogs = %v, want {%s: 3, gone-project: 1}", got, deleted.Hex())
	}

	for _, name := range []string{"orphan-log-1", "orphan-log-2", "orphan-unconverted", "orphan-not-hex"} {
		if n := countDocs(t, logs, bson.M{"name": name}); n != 0 {
			t.Errorf("%s: still present after the sweep", name)
		}
	}
	for _, name := range []string{"live-log", "live-unconverted", "orphan-recent", "legacy-missing", "legacy-empty"} {
		if n := countDocs(t, logs, bson.M{"name": name}); n != 1 {
			t.Errorf("%s: deleted by the sweep, should have been kept", name)
		}
	}
}
