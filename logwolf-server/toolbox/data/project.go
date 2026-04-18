package data

import (
	"context"
	"fmt"
	"regexp"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

const (
	RoleOwner  = "owner"
	RoleMember = "member"
)

var slugRe = regexp.MustCompile(`^[a-z0-9]+(?:-[a-z0-9]+)*$`)

type Project struct {
	ID        primitive.ObjectID `bson:"_id,omitempty" json:"id,omitempty"`
	Name      string             `bson:"name" json:"name"`
	Slug      string             `bson:"slug" json:"slug"`
	CreatedAt time.Time          `bson:"created_at" json:"created_at"`
}

type ProjectMember struct {
	ID          primitive.ObjectID `bson:"_id,omitempty" json:"id,omitempty"`
	ProjectID   primitive.ObjectID `bson:"project_id" json:"project_id"`
	GithubLogin string             `bson:"github_login" json:"github_login"`
	Role        string             `bson:"role" json:"role"`
	CreatedAt   time.Time          `bson:"created_at" json:"created_at"`
}

// ValidSlug reports whether s is a valid URL-safe slug.
func ValidSlug(s string) bool {
	return slugRe.MatchString(s)
}

// ValidRole reports whether r is a recognised project member role.
func ValidRole(r string) bool {
	return r == RoleOwner || r == RoleMember
}

// EnsureProjectIndexes creates the required indexes for projects and project_members.
// Safe to call on startup — CreateOne is idempotent for identical index definitions.
func (m *Models) EnsureProjectIndexes() error {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	projects := m.client.Database("logs").Collection("projects")
	if _, err := projects.Indexes().CreateOne(ctx, mongo.IndexModel{
		Keys:    bson.D{{Key: "slug", Value: 1}},
		Options: options.Index().SetUnique(true).SetName("unique_slug"),
	}); err != nil {
		return fmt.Errorf("EnsureProjectIndexes projects.slug: %w", err)
	}

	members := m.client.Database("logs").Collection("project_members")
	if _, err := members.Indexes().CreateOne(ctx, mongo.IndexModel{
		Keys:    bson.D{{Key: "project_id", Value: 1}, {Key: "github_login", Value: 1}},
		Options: options.Index().SetUnique(true).SetName("unique_project_member"),
	}); err != nil {
		return fmt.Errorf("EnsureProjectIndexes project_members.(project_id,github_login): %w", err)
	}

	return nil
}

func (m *Models) InsertProject(p Project) (*Project, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	p.ID = primitive.NewObjectID()
	p.CreatedAt = time.Now()

	if _, err := m.client.Database("logs").Collection("projects").InsertOne(ctx, p); err != nil {
		return nil, fmt.Errorf("InsertProject: %w", err)
	}
	return &p, nil
}

func (m *Models) GetProject(id primitive.ObjectID) (*Project, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	var p Project
	err := m.client.Database("logs").Collection("projects").FindOne(ctx, bson.M{"_id": id}).Decode(&p)
	if err != nil {
		return nil, fmt.Errorf("GetProject: %w", err)
	}
	return &p, nil
}

func (m *Models) GetProjectBySlug(slug string) (*Project, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	var p Project
	err := m.client.Database("logs").Collection("projects").FindOne(ctx, bson.M{"slug": slug}).Decode(&p)
	if err != nil {
		return nil, fmt.Errorf("GetProjectBySlug: %w", err)
	}
	return &p, nil
}

func (m *Models) InsertProjectMember(pm ProjectMember) (*ProjectMember, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	pm.ID = primitive.NewObjectID()
	pm.CreatedAt = time.Now()

	if _, err := m.client.Database("logs").Collection("project_members").InsertOne(ctx, pm); err != nil {
		return nil, fmt.Errorf("InsertProjectMember: %w", err)
	}
	return &pm, nil
}

func (m *Models) GetProjectMembers(projectID primitive.ObjectID) ([]ProjectMember, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	cursor, err := m.client.Database("logs").Collection("project_members").Find(ctx, bson.M{"project_id": projectID})
	if err != nil {
		return nil, fmt.Errorf("GetProjectMembers: %w", err)
	}
	defer cursor.Close(ctx)

	var members []ProjectMember
	if err := cursor.All(ctx, &members); err != nil {
		return nil, fmt.Errorf("GetProjectMembers decode: %w", err)
	}
	return members, nil
}
