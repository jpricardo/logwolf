//go:build integration

package integration

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"

	"logwolf-toolbox/data"
)

// bigProjectLogs is more than two of PurgeProjectLogs' batches (10000), so the
// tests below go through several of them, including a short last one.
const bigProjectLogs = 25000

// seedLogs inserts n logs for projectID, created at createdAt.
func seedLogs(t *testing.T, logs *mongo.Collection, projectID primitive.ObjectID, n int, createdAt time.Time) {
	t.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	const chunk = 5000
	for start := 0; start < n; start += chunk {
		docs := make([]any, 0, chunk)
		for i := start; i < n && i < start+chunk; i++ {
			docs = append(docs, bson.M{
				"project_id": projectID,
				"name":       fmt.Sprintf("bulk-%d", i),
				"data":       "{}",
				"severity":   "info",
				"tags":       bson.A{},
				"created_at": createdAt,
				"updated_at": createdAt,
			})
		}
		if _, err := logs.InsertMany(ctx, docs); err != nil {
			t.Fatalf("seed %d logs for %s: %v", n, projectID.Hex(), err)
		}
	}
}

// TestDeleteProject_ManyLogs deletes a project with more logs than one batch
// holds. The delete itself must not touch them — that is what used to make it
// time out — and PurgeProjectLogs must then remove all of them, and nothing of
// any other project.
func TestDeleteProject_ManyLogs(t *testing.T) {
	m := setupProjectModels(t)
	logs := testMongo(t, sharedModelsMongo(t)).Database("logs").Collection("logs")

	doomed, err := m.InsertProject(data.Project{Name: "Big", Slug: "big"})
	if err != nil {
		t.Fatalf("InsertProject: %v", err)
	}
	other, err := m.InsertProject(data.Project{Name: "Neighbour", Slug: "neighbour"})
	if err != nil {
		t.Fatalf("InsertProject: %v", err)
	}
	seedLogs(t, logs, doomed.ID, bigProjectLogs, time.Now())
	seedLogs(t, logs, other.ID, 3, time.Now())

	if err := m.DeleteProject(doomed.ID); err != nil {
		t.Fatalf("DeleteProject: %v", err)
	}
	if _, err := m.GetProject(doomed.ID); !errors.Is(err, mongo.ErrNoDocuments) {
		t.Fatalf("project still present after delete: %v", err)
	}
	if n := countDocs(t, logs, bson.M{"project_id": doomed.ID}); n != bigProjectLogs {
		t.Fatalf("DeleteProject touched the logs: %d left of %d, want all of them for the purge", n, bigProjectLogs)
	}

	purged, err := m.PurgeProjectLogs(context.Background(), doomed.ID)
	if err != nil {
		t.Fatalf("PurgeProjectLogs: %v", err)
	}
	if purged != bigProjectLogs {
		t.Errorf("PurgeProjectLogs = %d, want %d", purged, bigProjectLogs)
	}
	if n := countDocs(t, logs, bson.M{"project_id": doomed.ID}); n != 0 {
		t.Errorf("%d log(s) of the deleted project left after the purge", n)
	}
	if n := countDocs(t, logs, bson.M{"project_id": other.ID}); n != 3 {
		t.Errorf("purge reached another project: %d of its 3 logs left", n)
	}
}

// TestPurgeProjectLogs_RefusesLiveProject: the purge has no grace period and
// no filter but the project id, so it must never run for a project that exists.
func TestPurgeProjectLogs_RefusesLiveProject(t *testing.T) {
	m := setupProjectModels(t)
	logs := testMongo(t, sharedModelsMongo(t)).Database("logs").Collection("logs")

	p, err := m.InsertProject(data.Project{Name: "Alive", Slug: "alive"})
	if err != nil {
		t.Fatalf("InsertProject: %v", err)
	}
	seedLogs(t, logs, p.ID, 2, time.Now())

	purged, err := m.PurgeProjectLogs(context.Background(), p.ID)
	if !errors.Is(err, data.ErrProjectExists) {
		t.Errorf("PurgeProjectLogs on a live project: want ErrProjectExists, got %v", err)
	}
	if purged != 0 {
		t.Errorf("PurgeProjectLogs on a live project reported %d deleted", purged)
	}
	if n := countDocs(t, logs, bson.M{"project_id": p.ID}); n != 2 {
		t.Errorf("live project's logs: %d of 2 left", n)
	}
}

// TestPurgeProjectLogs_FailureIsFinishedBySweep fails the purge after its first
// batch. The project must stay deleted, and the orphan sweep in the next
// cleanup pass must delete what the purge left.
func TestPurgeProjectLogs_FailureIsFinishedBySweep(t *testing.T) {
	m := setupProjectModels(t)
	client := testMongo(t, sharedModelsMongo(t))
	logs := client.Database("logs").Collection("logs")

	p, err := m.InsertProject(data.Project{Name: "Flaky", Slug: "flaky"})
	if err != nil {
		t.Fatalf("InsertProject: %v", err)
	}
	// Older than the sweep's grace period, as they would be an hour later.
	seedLogs(t, logs, p.ID, bigProjectLogs, time.Now().Add(-time.Hour))

	if err := m.DeleteProject(p.ID); err != nil {
		t.Fatalf("DeleteProject: %v", err)
	}

	// Let the first batch through, then fail the second with a non-transient error.
	setFailPoint(t, client, bson.M{"skip": 1}, bson.M{"failCommands": bson.A{"delete"}, "errorCode": 2})
	purged, err := m.PurgeProjectLogs(context.Background(), p.ID)
	clearFailPoint(t, client)

	if err == nil {
		t.Fatal("PurgeProjectLogs: want the injected failure, got nil")
	}
	if purged == 0 || purged >= bigProjectLogs {
		t.Fatalf("PurgeProjectLogs reported %d deleted before failing, want one batch's worth", purged)
	}
	left := countDocs(t, logs, bson.M{"project_id": p.ID})
	if left != bigProjectLogs-purged {
		t.Errorf("after the failed purge: %d logs left, but it reported %d of %d deleted", left, purged, bigProjectLogs)
	}

	if _, err := m.GetProject(p.ID); !errors.Is(err, mongo.ErrNoDocuments) {
		t.Fatalf("a failed purge brought the project back: %v", err)
	}

	swept, err := m.DeleteOrphanedLogs(context.Background())
	if err != nil {
		t.Fatalf("DeleteOrphanedLogs: %v", err)
	}
	if swept[p.ID.Hex()] != left {
		t.Errorf("DeleteOrphanedLogs deleted %d for the project, want the %d the purge left", swept[p.ID.Hex()], left)
	}
	if n := countDocs(t, logs, bson.M{"project_id": p.ID}); n != 0 {
		t.Errorf("%d log(s) of the deleted project left after the sweep", n)
	}
}
