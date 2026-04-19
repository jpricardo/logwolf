//go:build integration

package integration

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// TestRetentionCleanup verifies that the logger's background cleanup goroutine
// deletes expired log entries while leaving unexpired ones intact, and that
// projects configured for infinite retention (days=0) are not touched.
func TestRetentionCleanup(t *testing.T) {
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

	// Connect directly to MongoDB for seeding and assertions.
	client, err := mongo.Connect(ctx, options.Client().ApplyURI(mongoURI).
		SetAuth(options.Credential{Username: "admin", Password: "password"}))
	if err != nil {
		t.Fatalf("mongo connect: %v", err)
	}
	t.Cleanup(func() { client.Disconnect(context.Background()) })

	db := client.Database("logs")

	// --- Seed test data ---

	// Project A: 30-day retention, has an expired log and a fresh log.
	projectA := primitive.NewObjectID()
	projectAStr := projectA.Hex()

	// Project B: infinite retention (days=0), has an old log that must NOT be deleted.
	projectB := primitive.NewObjectID()
	projectBStr := projectB.Hex()

	// Insert projects into the projects collection.
	_, err = db.Collection("projects").InsertMany(ctx, []interface{}{
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
