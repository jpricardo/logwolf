//go:build integration

package integration

import (
	"context"
	"testing"
	"time"

	"github.com/testcontainers/testcontainers-go/modules/rabbitmq"
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
	mongoURI, client := migrationMongo(t)
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
	projectID := project.ID

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

	// Both allowlisted logins own the project, stored lowercase as every
	// membership is — GitHub logins are case-insensitive.
	for _, login := range []string{"alice", "bob"} {
		assertOwner(t, db, project, login)
	}

	if hasIndex(t, db, "logs", "ttl_created_at") {
		t.Error("the legacy global TTL index should be dropped — it would override per-project retention")
	}

	// --- Second boot: idempotent ---

	startLogger(t, mongoURI, "alice,Bob,carol")

	assertCount(t, db, "projects", bson.M{"default": true}, 1)
	assertCount(t, db, "logs", bson.M{"project_id": projectID}, 3)
	assertCount(t, db, "api_keys", bson.M{"project_id": projectID}, 1)
	assertCount(t, db, "settings", bson.M{"project_id": projectID, "key": "retention_days"}, 1)

	// Default already has owners, so the login added to the allowlist since the
	// first boot must not be granted ownership.
	assertCount(t, db, "project_members", bson.M{"project_id": project.ID}, 2)
}

// TestStartupMigration_CleanDatabase verifies a fresh install is left alone:
// no data means no orphans, so no Default project is invented.
func TestStartupMigration_CleanDatabase(t *testing.T) {
	ctx := context.Background()
	mongoURI, client := migrationMongo(t)
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
	mongoURI, client := migrationMongo(t)
	db := client.Database("logs")

	legacy := []interface{}{
		bson.M{"name": "legacy-event-a", "data": "{}", "severity": "INFO", "created_at": time.Now(), "updated_at": time.Now()},
		bson.M{"name": "legacy-event-b", "data": "{}", "severity": "ERROR", "created_at": time.Now(), "updated_at": time.Now()},
	}
	if _, err := db.Collection("logs").InsertMany(ctx, legacy); err != nil {
		t.Fatalf("seed logs: %v", err)
	}

	legacyKey := seedLegacyAPIKey(t, db, "lw_legacykey0000000000000000000000000000000001")

	rabbitC, err := rabbitmq.Run(ctx, rabbitImage)
	if err != nil {
		t.Fatalf("rabbitmq container: %v", err)
	}
	t.Cleanup(func() { rabbitC.Terminate(context.Background()) })
	rabbitURI, _ := rabbitC.AmqpURL(ctx)

	brokerURL := startMigrationStack(t, mongoURI, rabbitURI)

	names := getLogs(t, brokerURL, legacyKey)
	for _, want := range []string{"legacy-event-a", "legacy-event-b"} {
		if !containsName(names, want) {
			t.Errorf("legacy key should read %q after migration, got %v", want, names)
		}
	}
}

// TestStartupMigration_OwnerStepRetried covers an owner step that fails after
// the data has moved. The next start finds no orphans, so it is the owner repair
// — not the adoption — that has to finish the job.
func TestStartupMigration_OwnerStepRetried(t *testing.T) {
	ctx := context.Background()
	mongoURI, client := migrationMongo(t)
	db := client.Database("logs")

	seedOrphanLog(t, db)

	// A validator no membership can satisfy makes every owner upsert fail, while
	// the adoption itself, which never touches project_members, succeeds.
	if err := db.CreateCollection(ctx, "project_members",
		options.CreateCollection().SetValidator(bson.M{"never_present": bson.M{"$exists": true}}),
	); err != nil {
		t.Fatalf("create project_members with validator: %v", err)
	}

	startLogger(t, mongoURI, "alice")

	project := requireDefaultProject(t, db)
	assertCount(t, db, "logs", orphanQuery(), 0)
	assertCount(t, db, "project_members", bson.M{"project_id": project.ID}, 0)

	if err := db.RunCommand(ctx, bson.D{
		{Key: "collMod", Value: "project_members"},
		{Key: "validator", Value: bson.M{}},
	}).Err(); err != nil {
		t.Fatalf("drop project_members validator: %v", err)
	}

	startLogger(t, mongoURI, "alice")

	assertOwner(t, db, project, "alice")
}

// TestStartupMigration_OwnersConfiguredLater covers an upgrade whose first start
// had nobody to make owner. Configuring owners afterwards must be enough — no
// hand-editing MongoDB.
func TestStartupMigration_OwnersConfiguredLater(t *testing.T) {
	ctx := context.Background()
	mongoURI, client := migrationMongo(t)
	db := client.Database("logs")

	seedOrphanLog(t, db)

	startLogger(t, mongoURI, "")

	project := requireDefaultProject(t, db)
	assertCount(t, db, "logs", orphanQuery(), 0)
	assertCount(t, db, "project_members", bson.M{"project_id": project.ID}, 0)

	// A plain member of the still-ownerless project: listing them as an owner
	// must promote them, not leave the existing membership as it is.
	if _, err := db.Collection("project_members").InsertOne(ctx, bson.M{
		"project_id": project.ID, "github_login": "erin", "role": data.RoleMember, "created_at": time.Now(),
	}); err != nil {
		t.Fatalf("seed member: %v", err)
	}

	startLoggerWithOwners(t, mongoURI, "alice", "erin")

	assertOwner(t, db, project, "alice")
	assertOwner(t, db, project, "erin")

	// Once Default has an owner, the allowlist no longer grants ownership: the
	// owners manage membership from the dashboard.
	startLogger(t, mongoURI, "alice,bob")

	assertCount(t, db, "project_members", bson.M{"project_id": project.ID}, 2)
}

// TestStartupMigration_OrgOnlyDeployment covers a deployment that admits users
// through LOGWOLF_ALLOWED_GITHUB_ORGS only. Logger cannot see org membership,
// so LOGWOLF_DEFAULT_PROJECT_OWNERS is how Default gets its owners.
func TestStartupMigration_OrgOnlyDeployment(t *testing.T) {
	mongoURI, client := migrationMongo(t)
	db := client.Database("logs")

	seedOrphanLog(t, db)

	startLoggerWithOwners(t, mongoURI, "", "dave")

	project := requireDefaultProject(t, db)
	assertCount(t, db, "logs", bson.M{"project_id": project.ID}, 1)
	assertOwner(t, db, project, "dave")
	assertCount(t, db, "project_members", bson.M{"project_id": project.ID}, 1)
}

// TestStartupMigration_NormalizesMemberLogins seeds memberships written before
// logins were normalized, including a login stored in two casings on one
// project, and checks the first boot folds them into one lowercase row each,
// keeping the higher role and the older join date.
func TestStartupMigration_NormalizesMemberLogins(t *testing.T) {
	ctx := context.Background()
	mongoURI, client := migrationMongo(t)
	db := client.Database("logs")
	members := db.Collection("project_members")

	p1, p2 := newOID(), newOID()
	for _, p := range []bson.M{
		{"_id": p1, "name": "One", "slug": "one", "created_at": time.Now()},
		{"_id": p2, "name": "Two", "slug": "two", "created_at": time.Now()},
	} {
		if _, err := db.Collection("projects").InsertOne(ctx, p); err != nil {
			t.Fatalf("seed project: %v", err)
		}
	}

	joined := time.Now().Add(-72 * time.Hour).Truncate(time.Millisecond)
	if _, err := members.InsertMany(ctx, []interface{}{
		// A case-only duplicate: the older row is a plain member, the newer one
		// an owner. The merge must keep both the ownership and the join date.
		bson.M{"project_id": p1, "github_login": "JDoe", "role": data.RoleMember, "created_at": joined},
		bson.M{"project_id": p1, "github_login": "jdoe", "role": data.RoleOwner, "created_at": time.Now()},
		// Mixed case, no duplicate: just renamed.
		bson.M{"project_id": p1, "github_login": "Erin", "role": data.RoleMember, "created_at": time.Now()},
		// The same person on another project stays a separate membership.
		bson.M{"project_id": p2, "github_login": "JDOE", "role": data.RoleOwner, "created_at": time.Now()},
		// Already normalized: untouched.
		bson.M{"project_id": p2, "github_login": "frank", "role": data.RoleMember, "created_at": time.Now()},
	}); err != nil {
		t.Fatalf("seed members: %v", err)
	}

	startLogger(t, mongoURI, "")

	assertCount(t, db, "project_members", bson.M{}, 4)
	assertCount(t, db, "project_members", bson.M{"project_id": p1, "github_login": "jdoe"}, 1)

	var merged data.ProjectMember
	if err := members.FindOne(ctx, bson.M{"project_id": p1, "github_login": "jdoe"}).Decode(&merged); err != nil {
		t.Fatalf("merged member: %v", err)
	}
	if merged.Role != data.RoleOwner {
		t.Errorf("merged role = %q, want %q", merged.Role, data.RoleOwner)
	}
	if !merged.CreatedAt.Equal(joined) {
		t.Errorf("merged created_at = %v, want the older %v", merged.CreatedAt, joined)
	}

	assertCount(t, db, "project_members", bson.M{"project_id": p1, "github_login": "erin", "role": data.RoleMember}, 1)
	assertCount(t, db, "project_members", bson.M{"project_id": p2, "github_login": "jdoe", "role": data.RoleOwner}, 1)
	assertCount(t, db, "project_members", bson.M{"project_id": p2, "github_login": "frank"}, 1)

	// Nothing left to do on the next boot.
	startLogger(t, mongoURI, "")
	assertCount(t, db, "project_members", bson.M{}, 4)
}

// TestStartupMigration_ConvertsProjectIDs seeds data the way multi-tenant
// builds stored it before every project_id was an ObjectID — logs, an API key
// and settings under the hex string of their project's id — and checks the
// first boot converts it, and that retention then works off the converted data.
func TestStartupMigration_ConvertsProjectIDs(t *testing.T) {
	ctx := context.Background()
	mongoURI, client := migrationMongo(t)
	db := client.Database("logs")

	// forever is inserted first, so the cleanup pass reaches it before expiring,
	// and once expiring's log is gone, forever has been dealt with as well.
	forever, expiring, both := newOID(), newOID(), newOID()
	for _, p := range []bson.M{
		{"_id": forever, "name": "Forever", "slug": "forever", "created_at": time.Now()},
		{"_id": expiring, "name": "Expiring", "slug": "expiring", "created_at": time.Now()},
		{"_id": both, "name": "Both", "slug": "both", "created_at": time.Now()},
	} {
		if _, err := db.Collection("projects").InsertOne(ctx, p); err != nil {
			t.Fatalf("seed project: %v", err)
		}
	}

	old := func(days int) time.Time { return time.Now().Add(-time.Duration(days) * 24 * time.Hour) }
	logDoc := func(name string, projectID any, created time.Time) bson.M {
		return bson.M{"name": name, "data": "{}", "severity": "info", "project_id": projectID, "created_at": created, "updated_at": created}
	}
	if _, err := db.Collection("logs").InsertMany(ctx, []any{
		logDoc("kept-forever", forever.Hex(), old(365)),
		// Past a 30-day retention but within the 90-day default: only deleted
		// if the project's own setting was converted along with the log.
		logDoc("expired-after-30", expiring.Hex(), old(31)),
		logDoc("fresh", expiring.Hex(), time.Now()),
		// Already an ObjectID: nothing to do.
		logDoc("already-converted", both, time.Now()),
		// Not hex, so not an ObjectID of anything: left as it is.
		logDoc("not-hex", "no-such-project", time.Now()),
	}); err != nil {
		t.Fatalf("seed logs: %v", err)
	}

	if err := insertAPIKey(mongoURI, forever.Hex(), "lw_convertkey0000000000000000000000000000000001"); err != nil {
		t.Fatalf("seed key: %v", err)
	}
	if _, err := db.Collection("api_keys").UpdateOne(ctx, bson.M{"project_id": forever}, bson.M{"$set": bson.M{"project_id": forever.Hex()}}); err != nil {
		t.Fatalf("store the key's project_id as a string: %v", err)
	}

	if _, err := db.Collection("settings").InsertMany(ctx, []any{
		bson.M{"project_id": forever.Hex(), "key": "retention_days", "value": 0},
		bson.M{"project_id": expiring.Hex(), "key": "retention_days", "value": 30},
		// A project that has its retention under both types: the string one is
		// stale, and converting it would collide with the ObjectID one.
		bson.M{"project_id": both.Hex(), "key": "retention_days", "value": 60},
		bson.M{"project_id": both, "key": "retention_days", "value": 180},
	}); err != nil {
		t.Fatalf("seed settings: %v", err)
	}
	if _, err := db.Collection("settings").Indexes().CreateOne(ctx, mongo.IndexModel{
		Keys:    bson.D{{Key: "project_id", Value: 1}, {Key: "key", Value: 1}},
		Options: options.Index().SetUnique(true).SetName("unique_project_key"),
	}); err != nil {
		t.Fatalf("seed settings index: %v", err)
	}

	startLogger(t, mongoURI, "")

	stringIDs := bson.M{"project_id": bson.M{"$type": "string"}}
	assertCount(t, db, "logs", stringIDs, 1)
	assertCount(t, db, "logs", bson.M{"name": "not-hex", "project_id": "no-such-project"}, 1)
	assertCount(t, db, "api_keys", stringIDs, 0)
	assertCount(t, db, "api_keys", bson.M{"project_id": forever}, 1)
	assertCount(t, db, "settings", stringIDs, 0)
	assertCount(t, db, "settings", bson.M{"project_id": both, "value": 180}, 1)
	assertCount(t, db, "settings", bson.M{"project_id": both}, 1)

	// The cleanup pass at startup runs on converted data: expiring's own 30
	// days apply, and forever's log survives the 90-day default.
	logs := db.Collection("logs")
	waitForLogGone(t, logs, "expired-after-30", 15*time.Second)
	for _, name := range []string{"kept-forever", "fresh"} {
		if n := countDocs(t, logs, bson.M{"name": name}); n != 1 {
			t.Errorf("%s: deleted by the cleanup, should have been kept", name)
		}
	}

	// Nothing left to do on the next boot.
	startLogger(t, mongoURI, "")
	assertCount(t, db, "logs", stringIDs, 1)
	assertCount(t, db, "settings", bson.M{}, 3)
}

// TestStartupMigration_MarksDefaultProject boots on a database a build with
// globally unique slugs left behind: the unique slug index, and a Default
// project known only by its slug. The index must go, and the project must be
// found by its flag from then on, even once another project takes its slug.
func TestStartupMigration_MarksDefaultProject(t *testing.T) {
	ctx := context.Background()
	mongoURI, client := migrationMongo(t)
	db := client.Database("logs")
	projects := db.Collection("projects")

	if _, err := projects.Indexes().CreateOne(ctx, mongo.IndexModel{
		Keys:    bson.D{{Key: "slug", Value: 1}},
		Options: options.Index().SetUnique(true).SetName("unique_slug"),
	}); err != nil {
		t.Fatalf("seed unique_slug: %v", err)
	}
	legacyDefault := newOID()
	if _, err := projects.InsertOne(ctx, bson.M{
		"_id": legacyDefault, "name": data.DefaultProjectName, "slug": data.DefaultProjectSlug, "created_at": time.Now(),
	}); err != nil {
		t.Fatalf("seed Default project: %v", err)
	}

	startLogger(t, mongoURI, "alice")

	if hasIndex(t, db, "projects", "unique_slug") {
		t.Error("the unique slug index should be dropped")
	}
	if !hasIndex(t, db, "projects", "unique_default") {
		t.Error("the index that allows one Default project is missing")
	}
	if got := requireDefaultProject(t, db); got.ID != legacyDefault {
		t.Fatalf("Default project = %s, want the one the old build created, %s", got.ID.Hex(), legacyDefault.Hex())
	}
	// Ownerless, so the owner repair found it by its flag.
	assertOwner(t, db, data.Project{ID: legacyDefault}, "alice")

	// A user's project with the same slug is allowed now, and changes nothing.
	if _, err := projects.InsertOne(ctx, bson.M{
		"_id": newOID(), "name": "Default", "slug": data.DefaultProjectSlug, "created_at": time.Now(),
	}); err != nil {
		t.Fatalf("a second project with slug %q: %v", data.DefaultProjectSlug, err)
	}
	seedOrphanLog(t, db)

	startLogger(t, mongoURI, "alice")

	assertCount(t, db, "logs", bson.M{"project_id": legacyDefault}, 1)
	assertCount(t, db, "projects", bson.M{"default": true}, 1)
}

// --- helpers ---

// seedOrphanLog inserts one log in the pre-multi-tenancy shape, enough to make
// the migration create the Default project.
func seedOrphanLog(t *testing.T, db *mongo.Database) {
	t.Helper()

	if _, err := db.Collection("logs").InsertOne(context.Background(), bson.M{
		"name": "old-event", "data": "{}", "severity": "INFO", "created_at": time.Now(), "updated_at": time.Now(),
	}); err != nil {
		t.Fatalf("seed logs: %v", err)
	}
}

// assertOwner fails unless login holds an owner membership on project.
func assertOwner(t *testing.T, db *mongo.Database, project data.Project, login string) {
	t.Helper()

	var member data.ProjectMember
	err := db.Collection("project_members").FindOne(context.Background(), bson.M{"project_id": project.ID, "github_login": login}).Decode(&member)
	if err != nil {
		t.Fatalf("membership for %q: %v", login, err)
	}
	if member.Role != data.RoleOwner {
		t.Errorf("member %q role = %q, want %q", login, member.Role, data.RoleOwner)
	}
}

// migrationMongo gives the test a MongoDB of its own — the migration only runs
// against a database no Logger has booted on yet — plus a client for seeding
// and assertions.
func migrationMongo(t *testing.T) (string, *mongo.Client) {
	t.Helper()

	uri := dedicatedMongo(t)
	return uri, testMongo(t, uri)
}

// startLogger boots Logger and returns its RPC address once the port accepts
// connections, which only happens after the startup migration has run.
func startLogger(t *testing.T, mongoURI, allowedUsers string) string {
	t.Helper()
	return startLoggerWithOwners(t, mongoURI, allowedUsers, "")
}

// startLoggerWithOwners is startLogger with LOGWOLF_DEFAULT_PROJECT_OWNERS set
// as well — the owners an org-only deployment configures for Default.
func startLoggerWithOwners(t *testing.T, mongoURI, allowedUsers, defaultOwners string) string {
	t.Helper()

	rpcAddr := freeAddr(t)
	httpAddr := freeAddr(t)

	startProcess(t, "../logger/cmd/api", map[string]string{
		"MONGO_URL":                      mongoURI,
		"LOGGER_RPC_PORT":                portOf(rpcAddr),
		"LOGGER_HTTP_PORT":               portOf(httpAddr),
		"LOGWOLF_ALLOWED_GITHUB_USERS":   allowedUsers,
		"LOGWOLF_DEFAULT_PROJECT_OWNERS": defaultOwners,
		// Keep the retention loop out of the way of the assertions.
		"CLEANUP_INTERVAL": "24h",
	})

	waitForTCP(t, rpcAddr, 60*time.Second)
	return rpcAddr
}

// startMigrationStack boots Logger, Listener and Broker against mongoURI for
// this test alone and returns the Broker's base URL. The shared stack cannot
// stand in: its Logger has long since run its migration on a different database.
func startMigrationStack(t *testing.T, mongoURI, rabbitURI string) string {
	t.Helper()

	loggerRPCAddr := startLogger(t, mongoURI, "")
	brokerHTTPAddr := freeAddr(t)

	startProcess(t, "../listener/cmd/api", map[string]string{
		"RABBITMQ_URL":    rabbitURI,
		"LOGGER_RPC_ADDR": loggerRPCAddr,
	})
	startProcess(t, "../broker/cmd/api", map[string]string{
		"MONGO_URL":           mongoURI,
		"RABBITMQ_URL":        rabbitURI,
		"LOGGER_RPC_ADDR":     loggerRPCAddr,
		"BROKER_PORT":         portOf(brokerHTTPAddr),
		"INTERNAL_API_SECRET": internalSecret,
	})

	brokerURL := "http://" + brokerHTTPAddr
	if err := waitHTTP(brokerURL+"/ping", 60*time.Second); err != nil {
		t.Fatalf("broker: %v", err)
	}
	return brokerURL
}

func requireDefaultProject(t *testing.T, db *mongo.Database) data.Project {
	t.Helper()

	var project data.Project
	err := db.Collection("projects").FindOne(context.Background(), bson.M{"default": true}).Decode(&project)
	if err != nil {
		t.Fatalf("the migration should have created the %q project: %v", data.DefaultProjectName, err)
	}
	if project.Name != data.DefaultProjectName || project.Slug != data.DefaultProjectSlug {
		t.Errorf("project = %q (%s), want %q (%s)", project.Name, project.Slug, data.DefaultProjectName, data.DefaultProjectSlug)
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
