//go:build integration

package integration

import (
	"context"
	"testing"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"

	"logwolf-toolbox/data"
)

// TestRetentionCleanup verifies that the logger's background cleanup goroutine
// deletes expired log entries while leaving unexpired ones intact, that
// projects configured for infinite retention (days=0) are not touched, and that
// logs of a project that no longer exists are deleted whatever their age.
func TestRetentionCleanup(t *testing.T) {
	ctx := context.Background()

	// A logger with a two-second cleanup interval runs against this database for
	// the length of the test, so it gets a MongoDB of its own rather than
	// deleting entries other tests are still asserting on.
	mongoURI := dedicatedMongo(t)
	client := testMongo(t, mongoURI)

	db := client.Database("logs")

	// --- Seed test data ---

	// Project A: 30-day retention, has an expired log and a fresh log.
	projectA := primitive.NewObjectID()
	projectAStr := projectA.Hex()

	// Project B: infinite retention (days=0), has an old log that must NOT be deleted.
	projectB := primitive.NewObjectID()
	projectBStr := projectB.Hex()

	// Insert projects into the projects collection.
	_, err := db.Collection("projects").InsertMany(ctx, []interface{}{
		bson.M{"_id": projectA, "name": "Project A", "slug": "project-a", "created_at": time.Now()},
		bson.M{"_id": projectB, "name": "Project B", "slug": "project-b", "created_at": time.Now()},
	})
	if err != nil {
		t.Fatalf("insert projects: %v", err)
	}

	// Set retention settings.
	_, err = db.Collection("settings").InsertMany(ctx, []interface{}{
		bson.M{"project_id": projectAStr, "key": "retention_days", "value": 30},
		bson.M{"project_id": projectBStr, "key": "retention_days", "value": 0},
	})
	if err != nil {
		t.Fatalf("insert settings: %v", err)
	}

	// Project A: expired log (31 days old) and a fresh log (now).
	expiredLogName := "expired-log-project-a"
	freshLogName := "fresh-log-project-a"
	oldLogName := "old-log-project-b-forever"
	orphanLogName := "log-of-deleted-project"

	_, err = db.Collection("logs").InsertMany(ctx, []interface{}{
		bson.M{
			"project_id": projectAStr,
			"name":       expiredLogName,
			"data":       "{}",
			"severity":   "info",
			"tags":       bson.A{},
			"created_at": time.Now().Add(-31 * 24 * time.Hour),
			"updated_at": time.Now().Add(-31 * 24 * time.Hour),
		},
		bson.M{
			"project_id": projectAStr,
			"name":       freshLogName,
			"data":       "{}",
			"severity":   "info",
			"tags":       bson.A{},
			"created_at": time.Now(),
			"updated_at": time.Now(),
		},
		bson.M{
			"project_id": projectBStr,
			"name":       oldLogName,
			"data":       "{}",
			"severity":   "info",
			"tags":       bson.A{},
			"created_at": time.Now().Add(-365 * 24 * time.Hour),
			"updated_at": time.Now().Add(-365 * 24 * time.Hour),
		},
		// Its project is in no collection — deleted after this event passed the
		// Logger's check. Not expired by any retention setting; deleted anyway.
		bson.M{
			"project_id": primitive.NewObjectID().Hex(),
			"name":       orphanLogName,
			"data":       "{}",
			"severity":   "info",
			"tags":       bson.A{},
			"created_at": time.Now().Add(-time.Hour),
			"updated_at": time.Now().Add(-time.Hour),
		},
	})
	if err != nil {
		t.Fatalf("insert logs: %v", err)
	}

	// --- Start the logger with a fast cleanup interval ---
	loggerRPCAddr := freeAddr(t)
	loggerHTTPAddr := freeAddr(t)

	startProcess(t, "../logger/cmd/api", map[string]string{
		"MONGO_URL":        mongoURI,
		"LOGGER_RPC_PORT":  portOf(loggerRPCAddr),
		"LOGGER_HTTP_PORT": portOf(loggerHTTPAddr),
		"CLEANUP_INTERVAL": "2s",
	})

	waitForTCP(t, loggerRPCAddr, 30*time.Second)

	// --- Assertions ---

	coll := db.Collection("logs")

	// The cleanup runs immediately at startup; poll until the expired log disappears.
	waitForLogGone(t, coll, expiredLogName, 10*time.Second)
	waitForLogGone(t, coll, orphanLogName, 10*time.Second)

	// The fresh log must remain.
	n, err := coll.CountDocuments(ctx, bson.M{"name": freshLogName})
	if err != nil {
		t.Fatalf("count fresh log: %v", err)
	}
	if n != 1 {
		t.Errorf("expected fresh log %q to be present, but it was deleted", freshLogName)
	}

	// The infinite-retention log must remain.
	n, err = coll.CountDocuments(ctx, bson.M{"name": oldLogName})
	if err != nil {
		t.Fatalf("count infinite-retention log: %v", err)
	}
	if n != 1 {
		t.Errorf("expected infinite-retention log %q to be present, but it was deleted", oldLogName)
	}
}

// TestDeleteExpiredLogs_ManyLogs expires more logs than one batch holds. All of
// them must go, and nothing newer, or of another project.
func TestDeleteExpiredLogs_ManyLogs(t *testing.T) {
	m := setupProjectModels(t)
	logs := testMongo(t, sharedModelsMongo(t)).Database("logs").Collection("logs")

	p, err := m.InsertProject(data.Project{Name: "Old", Slug: "old"})
	if err != nil {
		t.Fatalf("InsertProject: %v", err)
	}
	other, err := m.InsertProject(data.Project{Name: "Neighbour", Slug: "neighbour"})
	if err != nil {
		t.Fatalf("InsertProject: %v", err)
	}
	old := time.Now().Add(-31 * 24 * time.Hour)
	seedLogs(t, logs, p.ID.Hex(), bigProjectLogs, old)
	seedLogs(t, logs, p.ID.Hex(), 2, time.Now())
	seedLogs(t, logs, other.ID.Hex(), 3, old)

	deleted, err := m.DeleteExpiredLogs(context.Background(), p.ID.Hex(), time.Now().Add(-30*24*time.Hour))
	if err != nil {
		t.Fatalf("DeleteExpiredLogs: %v", err)
	}
	if deleted != bigProjectLogs {
		t.Errorf("DeleteExpiredLogs = %d, want %d", deleted, bigProjectLogs)
	}
	if n := countDocs(t, logs, bson.M{"project_id": p.ID.Hex()}); n != 2 {
		t.Errorf("project has %d logs left, want its 2 fresh ones", n)
	}
	if n := countDocs(t, logs, bson.M{"project_id": other.ID.Hex()}); n != 3 {
		t.Errorf("DeleteExpiredLogs reached another project: %d of its 3 logs left", n)
	}
}

// waitForLogGone polls until no document with the given name exists, or the timeout elapses.
func waitForLogGone(t *testing.T, coll *mongo.Collection, name string, timeout time.Duration) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		n, err := coll.CountDocuments(context.Background(), bson.M{"name": name})
		if err == nil && n == 0 {
			return
		}
		time.Sleep(300 * time.Millisecond)
	}
	t.Errorf("log %q still present after %s", name, timeout)
}
