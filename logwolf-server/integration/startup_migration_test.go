//go:build integration

package integration

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/rabbitmq"
	"github.com/testcontainers/testcontainers-go/wait"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"golang.org/x/crypto/bcrypt"
	"logwolf-toolbox/data"
)

// TestStartupMigration seeds a database in the shape a pre-multi-tenancy build
// left behind — logs, an API key, and a retention setting with no project_id,
// plus the old global TTL index — then boots the real Logger against it.
//
// Everything is asserted after the RPC port opens, which is the guarantee that
// matters: the migration finishes before any caller can read.
func TestStartupMigration(t *testing.T) {
	ctx := context.Background()
	mongoURI, client := startMongo(t)
	db := client.Database("logs")

	// --- Seed pre-multi-tenancy data ---

	legacyLogs := []interface{}{
		bson.M{"name": "old-event-1", "data": "{}", "severity": "INFO", "tags": []string{"legacy"}, "created_at": time.Now().Add(-48 * time.Hour), "updated_at": time.Now()},
		bson.M{"name": "old-event-2", "data": "{}", "severity": "ERROR", "tags": []string{"legacy"}, "created_at": time.Now().Add(-24 * time.Hour), "updated_at": time.Now()},
		// project_id present but empty — a half-written document from an in-between build.
		bson.M{"name": "old-event-3", "data": "{}", "severity": "WARNING", "project_id": "", "created_at": time.Now(), "updated_at": time.Now()},
	}
	if _, err := db.Collection("logs").InsertMany(ctx, legacyLogs); err != nil {
		t.Fatalf("seed logs: %v", err)
	}

	if _, err := db.Collection("api_keys").InsertOne(ctx, bson.M{
		"prefix": "lw_legacy1", "hash": "$2a$10$notarealhash", "active": true, "created_at": time.Now(),
	}); err != nil {
		t.Fatalf("seed api_keys: %v", err)
	}

	if _, err := db.Collection("settings").InsertOne(ctx, bson.M{
		"key": "retention_days", "value": 30,
	}); err != nil {
		t.Fatalf("seed settings: %v", err)
	}

	// The global TTL index the old build maintained on logs.created_at.
	if _, err := db.Collection("logs").Indexes().CreateOne(ctx, mongo.IndexModel{
		Keys:    bson.D{{Key: "created_at", Value: 1}},
		Options: options.Index().SetExpireAfterSeconds(30 * 24 * 60 * 60).SetName("ttl_created_at"),
	}); err != nil {
		t.Fatalf("seed TTL index: %v", err)
	}

	// --- First boot: the migration runs ---

	startLogger(t, mongoURI, "alice,Bob")

	project := requireDefaultProject(t, db)
	projectID := project.ID.Hex()

	// All three logs are readable in the Default project, including the one that
	// carried an empty project_id.
	assertCount(t, db, "logs", bson.M{"project_id": projectID}, 3)
	assertCount(t, db, "logs", orphanQuery(), 0)

	assertCount(t, db, "api_keys", bson.M{"project_id": projectID}, 1)
	assertCount(t, db, "api_keys", orphanQuery(), 0)

	// The retention value survives the move — it is the deployment's setting,
	// now scoped to the project that inherited the data.
	var retention struct {
		Value int `bson:"value"`
	}
	if err := db.Collection("settings").FindOne(ctx, bson.M{"project_id": projectID, "key": "retention_days"}).Decode(&retention); err != nil {
		t.Fatalf("migrated settings: %v", err)
	}
	if retention.Value != 30 {
		t.Errorf("retention_days = %d, want 30 (the pre-migration value)", retention.Value)
	}

	// Both allowlisted logins own the project, with their case preserved.
	for _, login := range []string{"alice", "Bob"} {
		var member data.ProjectMember
		err := db.Collection("project_members").FindOne(ctx, bson.M{"project_id": project.ID, "github_login": login}).Decode(&member)
		if err != nil {
			t.Fatalf("membership for %q: %v", login, err)
		}
		if member.Role != data.RoleOwner {
			t.Errorf("member %q role = %q, want %q", login, member.Role, data.RoleOwner)
		}
	}

	if hasIndex(t, db, "logs", "ttl_created_at") {
		t.Error("the legacy global TTL index should be dropped — it would override per-project retention")
	}

	// --- Second boot: idempotent ---

	startLogger(t, mongoURI, "alice,Bob,carol")

	assertCount(t, db, "projects", bson.M{"slug": data.DefaultProjectSlug}, 1)
	assertCount(t, db, "logs", bson.M{"project_id": projectID}, 3)
	assertCount(t, db, "api_keys", bson.M{"project_id": projectID}, 1)
	assertCount(t, db, "settings", bson.M{"project_id": projectID, "key": "retention_days"}, 1)

	// With no orphans left there is nothing to migrate, so the login added to the
	// allowlist since the first boot must not be granted ownership.
	assertCount(t, db, "project_members", bson.M{"project_id": project.ID}, 2)
}

// TestStartupMigration_CleanDatabase verifies a fresh install is left alone:
// no data means no orphans, so no Default project is invented.
func TestStartupMigration_CleanDatabase(t *testing.T) {
	ctx := context.Background()
	mongoURI, client := startMongo(t)
	db := client.Database("logs")

	startLogger(t, mongoURI, "alice")

	n, err := db.Collection("projects").CountDocuments(ctx, bson.M{})
	if err != nil {
		t.Fatalf("count projects: %v", err)
	}
	if n != 0 {
		t.Errorf("clean database: got %d projects, want 0 — the migration should skip silently", n)
	}
}

// TestStartupMigration_LegacyDataReadableThroughBroker is the automated form of
// the upgrade check: seed a database as a pre-multi-tenancy build left it, boot
// the whole stack, and read the old events back through the public API using the
// old API key. Nothing in the request mentions a project — the key carries the
// one the migration adopted it into.
func TestStartupMigration_LegacyDataReadableThroughBroker(t *testing.T) {
	ctx := context.Background()
	mongoURI, client := startMongo(t)
	db := client.Database("logs")

	legacy := []interface{}{
		bson.M{"name": "legacy-event-a", "data": "{}", "severity": "INFO", "created_at": time.Now(), "updated_at": time.Now()},
		bson.M{"name": "legacy-event-b", "data": "{}", "severity": "ERROR", "created_at": time.Now(), "updated_at": time.Now()},
	}
	if _, err := db.Collection("logs").InsertMany(ctx, legacy); err != nil {
		t.Fatalf("seed logs: %v", err)
	}

	legacyKey := seedLegacyAPIKey(t, db, "lw_legacykey00000000001")

	rabbitC, err := rabbitmq.Run(ctx, "rabbitmq:3.9-alpine")
	if err != nil {
		t.Fatalf("rabbitmq container: %v", err)
	}
	t.Cleanup(func() { rabbitC.Terminate(context.Background()) })
	rabbitURI, _ := rabbitC.AmqpURL(ctx)

	brokerURL := startStack(t, mongoURI, rabbitURI)

	names := getLogs(t, brokerURL, legacyKey)
	for _, want := range []string{"legacy-event-a", "legacy-event-b"} {
		if !containsName(names, want) {
			t.Errorf("legacy key should read %q after migration, got %v", want, names)
		}
	}
}

// --- helpers ---

// startMongo launches a throwaway MongoDB with the credentials Logger expects
// and returns its URI plus a connected client for seeding and assertions.
func startMongo(t *testing.T) (string, *mongo.Client) {
	t.Helper()
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
	t.Cleanup(func() { mongoC.Terminate(context.Background()) })

	host, _ := mongoC.Host(ctx)
	port, _ := mongoC.MappedPort(ctx, "27017")
	uri := fmt.Sprintf("mongodb://admin:password@%s:%s", host, port.Port())

	client, err := mongo.Connect(ctx, options.Client().ApplyURI(uri).
		SetAuth(options.Credential{Username: "admin", Password: "password"}))
	if err != nil {
		t.Fatalf("mongo connect: %v", err)
	}
	t.Cleanup(func() { client.Disconnect(context.Background()) })

	return uri, client
}

// startLogger boots Logger and returns once its RPC port accepts connections,
// which only happens after the startup migration has run.
func startLogger(t *testing.T, mongoURI, allowedUsers string) {
	t.Helper()

	rpcAddr := freeAddr(t)
	httpAddr := freeAddr(t)

	startProcess(t, "../logger/cmd/api", map[string]string{
		"MONGO_URL":                    mongoURI,
		"LOGGER_RPC_PORT":              portOf(rpcAddr),
		"LOGGER_HTTP_PORT":             portOf(httpAddr),
		"LOGWOLF_ALLOWED_GITHUB_USERS": allowedUsers,
		// Keep the retention loop out of the way of the assertions.
		"CLEANUP_INTERVAL": "24h",
	})

	waitForTCP(t, rpcAddr, 60*time.Second)
}

func requireDefaultProject(t *testing.T, db *mongo.Database) data.Project {
	t.Helper()

	var project data.Project
	err := db.Collection("projects").FindOne(context.Background(), bson.M{"slug": data.DefaultProjectSlug}).Decode(&project)
	if err != nil {
		t.Fatalf("the migration should have created the %q project: %v", data.DefaultProjectSlug, err)
	}
	if project.Name != data.DefaultProjectName {
		t.Errorf("project name = %q, want %q", project.Name, data.DefaultProjectName)
	}
	return project
}

// orphanQuery matches documents the migration should have left none of.
func orphanQuery() bson.M {
	return bson.M{"$or": []bson.M{
		{"project_id": bson.M{"$exists": false}},
		{"project_id": nil},
		{"project_id": ""},
	}}
}

func assertCount(t *testing.T, db *mongo.Database, collection string, filter bson.M, want int64) {
	t.Helper()

	got, err := db.Collection(collection).CountDocuments(context.Background(), filter)
	if err != nil {
		t.Fatalf("count %s: %v", collection, err)
	}
	if got != want {
		t.Errorf("%s matching %v: got %d, want %d", collection, filter, got, want)
	}
}

func hasIndex(t *testing.T, db *mongo.Database, collection, name string) bool {
	t.Helper()
	ctx := context.Background()

	cursor, err := db.Collection(collection).Indexes().List(ctx)
	if err != nil {
		t.Fatalf("list indexes on %s: %v", collection, err)
	}
	defer cursor.Close(ctx)

	for cursor.Next(ctx) {
		var idx bson.M
		if err := cursor.Decode(&idx); err != nil {
			continue
		}
		if idx["name"] == name {
			return true
		}
	}
	return false
}

// seedLegacyAPIKey inserts an API key in the pre-multi-tenancy shape — a usable
// bcrypt hash, no project_id — and returns its plaintext.
func seedLegacyAPIKey(t *testing.T, db *mongo.Database, plaintext string) string {
	t.Helper()

	hash, err := bcrypt.GenerateFromPassword([]byte(plaintext), bcrypt.DefaultCost)
	if err != nil {
		t.Fatalf("seedLegacyAPIKey: bcrypt: %v", err)
	}

	_, err = db.Collection("api_keys").InsertOne(context.Background(), bson.M{
		"prefix":     plaintext[:10],
		"hash":       string(hash),
		"active":     true,
		"created_at": time.Now(),
	})
	if err != nil {
		t.Fatalf("seedLegacyAPIKey: insert: %v", err)
	}

	return plaintext
}
