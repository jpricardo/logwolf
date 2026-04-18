//go:build integration

package integration

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"logwolf-toolbox/data"
)

// setupProjectModels spins up a throwaway MongoDB and returns a data.Models
// with project indexes already created. Cleanup terminates the container.
func setupProjectModels(t *testing.T) data.Models {
	t.Helper()
	ctx := context.Background()

	mongoC, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: testcontainers.ContainerRequest{
			Image:        "mongo:4.2.16-bionic",
			ExposedPorts: []string{"27017/tcp"},
			WaitingFor:   wait.ForLog("waiting for connections on port 27017"),
		},
		Started: true,
	})
	if err != nil {
		t.Fatalf("setupProjectModels: container start: %v", err)
	}
	t.Cleanup(func() { mongoC.Terminate(ctx) })

	host, _ := mongoC.Host(ctx)
	port, _ := mongoC.MappedPort(ctx, "27017")
	uri := fmt.Sprintf("mongodb://%s:%s", host, port.Port())

	client, err := mongo.Connect(ctx, options.Client().ApplyURI(uri))
	if err != nil {
		t.Fatalf("setupProjectModels: connect: %v", err)
	}
	t.Cleanup(func() { client.Disconnect(ctx) })

	m := data.New(client)
	if err := m.EnsureProjectIndexes(); err != nil {
		t.Fatalf("setupProjectModels: indexes: %v", err)
	}
	return m
}

// --- Project CRUD ---

func TestInsertAndGetProject(t *testing.T) {
	m := setupProjectModels(t)

	created, err := m.InsertProject(data.Project{Name: "Alpha", Slug: "alpha"})
	if err != nil {
		t.Fatalf("InsertProject: %v", err)
	}
	if created.ID.IsZero() {
		t.Fatal("InsertProject: ID should be set")
	}

	got, err := m.GetProject(created.ID)
	if err != nil {
		t.Fatalf("GetProject: %v", err)
	}
	if got.Name != "Alpha" || got.Slug != "alpha" {
		t.Errorf("GetProject: got %+v", got)
	}
}

func TestGetProject_NotFound(t *testing.T) {
	m := setupProjectModels(t)

	_, err := m.GetProject(newOID())
	if !errors.Is(err, mongo.ErrNoDocuments) {
		t.Errorf("GetProject missing: want mongo.ErrNoDocuments, got %v", err)
	}
}

func TestGetProjectBySlug(t *testing.T) {
	m := setupProjectModels(t)

	if _, err := m.InsertProject(data.Project{Name: "Beta", Slug: "beta"}); err != nil {
		t.Fatalf("InsertProject: %v", err)
	}

	got, err := m.GetProjectBySlug("beta")
	if err != nil {
		t.Fatalf("GetProjectBySlug: %v", err)
	}
	if got.Slug != "beta" {
		t.Errorf("GetProjectBySlug: slug = %q", got.Slug)
	}
}

func TestGetProjectBySlug_NotFound(t *testing.T) {
	m := setupProjectModels(t)

	_, err := m.GetProjectBySlug("no-such-slug")
	if !errors.Is(err, mongo.ErrNoDocuments) {
		t.Errorf("GetProjectBySlug missing: want mongo.ErrNoDocuments, got %v", err)
	}
}

func TestUpdateProject(t *testing.T) {
	m := setupProjectModels(t)

	p, err := m.InsertProject(data.Project{Name: "Old", Slug: "old-slug"})
	if err != nil {
		t.Fatalf("InsertProject: %v", err)
	}

	updated, err := m.UpdateProject(p.ID, "New Name", "new-slug")
	if err != nil {
		t.Fatalf("UpdateProject: %v", err)
	}
	if updated.Name != "New Name" || updated.Slug != "new-slug" {
		t.Errorf("UpdateProject: got %+v", updated)
	}

	// Verify persistence via a fresh read.
	got, _ := m.GetProject(p.ID)
	if got.Name != "New Name" || got.Slug != "new-slug" {
		t.Errorf("UpdateProject not persisted: got %+v", got)
	}
}

func TestUpdateProject_NotFound(t *testing.T) {
	m := setupProjectModels(t)

	_, err := m.UpdateProject(newOID(), "X", "x")
	if !errors.Is(err, mongo.ErrNoDocuments) {
		t.Errorf("UpdateProject missing: want mongo.ErrNoDocuments, got %v", err)
	}
}

func TestDeleteProject_Cascade(t *testing.T) {
	m := setupProjectModels(t)

	p, _ := m.InsertProject(data.Project{Name: "Doomed", Slug: "doomed"})

	// Seed related data.
	if err := m.Insert(data.LogEntry{ProjectID: p.ID.Hex(), Name: "e", Data: "{}", Severity: "info", Tags: []string{}}); err != nil {
		t.Fatalf("seed log: %v", err)
	}
	plaintext, key, err := data.GenerateAPIKey(p.ID.Hex())
	if err != nil {
		t.Fatalf("GenerateAPIKey: %v", err)
	}
	_ = plaintext
	if err := m.SaveAPIKey(key); err != nil {
		t.Fatalf("SaveAPIKey: %v", err)
	}
	if err := m.Settings.SetRetentionDays(p.ID.Hex(), 30); err != nil {
		t.Fatalf("SetRetentionDays: %v", err)
	}
	if _, err := m.InsertProjectMember(data.ProjectMember{
		ProjectID: p.ID, GithubLogin: "owner1", Role: data.RoleOwner,
	}); err != nil {
		t.Fatalf("InsertProjectMember: %v", err)
	}

	if err := m.DeleteProject(p.ID); err != nil {
		t.Fatalf("DeleteProject: %v", err)
	}

	// Project itself must be gone.
	if _, err := m.GetProject(p.ID); !errors.Is(err, mongo.ErrNoDocuments) {
		t.Errorf("project still present after delete: %v", err)
	}
	// Members must be gone.
	members, _ := m.GetProjectMembers(p.ID)
	if len(members) != 0 {
		t.Errorf("members still present: %d", len(members))
	}
}

// --- Member helpers ---

func TestInsertProjectMember_Duplicate(t *testing.T) {
	m := setupProjectModels(t)

	p, _ := m.InsertProject(data.Project{Name: "Dup", Slug: "dup"})
	pm := data.ProjectMember{ProjectID: p.ID, GithubLogin: "alice", Role: data.RoleOwner}

	if _, err := m.InsertProjectMember(pm); err != nil {
		t.Fatalf("first InsertProjectMember: %v", err)
	}
	if _, err := m.InsertProjectMember(pm); err == nil {
		t.Error("second InsertProjectMember: expected duplicate key error, got nil")
	}
}

func TestRemoveProjectMember_LastOwner(t *testing.T) {
	m := setupProjectModels(t)

	p, _ := m.InsertProject(data.Project{Name: "Solo", Slug: "solo"})
	m.InsertProjectMember(data.ProjectMember{ProjectID: p.ID, GithubLogin: "only-owner", Role: data.RoleOwner})

	err := m.RemoveProjectMember(p.ID, "only-owner")
	if !errors.Is(err, data.ErrLastOwner) {
		t.Errorf("RemoveProjectMember last owner: want ErrLastOwner, got %v", err)
	}
}

func TestRemoveProjectMember_SecondOwnerAllowed(t *testing.T) {
	m := setupProjectModels(t)

	p, _ := m.InsertProject(data.Project{Name: "Multi", Slug: "multi"})
	m.InsertProjectMember(data.ProjectMember{ProjectID: p.ID, GithubLogin: "owner1", Role: data.RoleOwner})
	m.InsertProjectMember(data.ProjectMember{ProjectID: p.ID, GithubLogin: "owner2", Role: data.RoleOwner})

	if err := m.RemoveProjectMember(p.ID, "owner2"); err != nil {
		t.Errorf("RemoveProjectMember second owner: %v", err)
	}
}

func TestRemoveProjectMember_RegularMember(t *testing.T) {
	m := setupProjectModels(t)

	p, _ := m.InsertProject(data.Project{Name: "Reg", Slug: "reg"})
	m.InsertProjectMember(data.ProjectMember{ProjectID: p.ID, GithubLogin: "owner", Role: data.RoleOwner})
	m.InsertProjectMember(data.ProjectMember{ProjectID: p.ID, GithubLogin: "bob", Role: data.RoleMember})

	if err := m.RemoveProjectMember(p.ID, "bob"); err != nil {
		t.Errorf("RemoveProjectMember member: %v", err)
	}
}

func TestRemoveProjectMember_NotFound(t *testing.T) {
	m := setupProjectModels(t)

	p, _ := m.InsertProject(data.Project{Name: "NF", Slug: "nf"})

	err := m.RemoveProjectMember(p.ID, "ghost")
	if !errors.Is(err, mongo.ErrNoDocuments) {
		t.Errorf("RemoveProjectMember missing: want mongo.ErrNoDocuments, got %v", err)
	}
}

func TestIsMember(t *testing.T) {
	m := setupProjectModels(t)

	p, _ := m.InsertProject(data.Project{Name: "Check", Slug: "check"})
	m.InsertProjectMember(data.ProjectMember{ProjectID: p.ID, GithubLogin: "carol", Role: data.RoleMember})

	ok, err := m.IsMember(p.ID, "carol")
	if err != nil || !ok {
		t.Errorf("IsMember carol: ok=%v err=%v", ok, err)
	}

	ok, err = m.IsMember(p.ID, "stranger")
	if err != nil || ok {
		t.Errorf("IsMember stranger: ok=%v err=%v", ok, err)
	}
}

func TestGetProjectsForUser(t *testing.T) {
	m := setupProjectModels(t)

	p1, _ := m.InsertProject(data.Project{Name: "P1", Slug: "p1"})
	p2, _ := m.InsertProject(data.Project{Name: "P2", Slug: "p2"})
	m.InsertProject(data.Project{Name: "P3", Slug: "p3"}) // dave is NOT a member

	m.InsertProjectMember(data.ProjectMember{ProjectID: p1.ID, GithubLogin: "dave", Role: data.RoleOwner})
	m.InsertProjectMember(data.ProjectMember{ProjectID: p2.ID, GithubLogin: "dave", Role: data.RoleMember})

	projects, err := m.GetProjectsForUser("dave")
	if err != nil {
		t.Fatalf("GetProjectsForUser: %v", err)
	}
	if len(projects) != 2 {
		t.Errorf("GetProjectsForUser: want 2 projects, got %d", len(projects))
	}
}

func TestGetProjectsForUser_NoMemberships(t *testing.T) {
	m := setupProjectModels(t)

	projects, err := m.GetProjectsForUser("nobody")
	if err != nil {
		t.Fatalf("GetProjectsForUser no memberships: %v", err)
	}
	if projects == nil {
		t.Error("GetProjectsForUser: must return non-nil slice for user with no projects")
	}
	if len(projects) != 0 {
		t.Errorf("GetProjectsForUser: want 0, got %d", len(projects))
	}
}

// newOID returns a fresh ObjectID guaranteed not to exist in any collection.
func newOID() primitive.ObjectID { return primitive.NewObjectID() }
