package data

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// ErrLastOwner is returned when an operation would remove the last owner of a project.
var ErrLastOwner = errors.New("cannot remove the last owner of a project")

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

func (m *Models) UpdateProject(id primitive.ObjectID, name, slug string) (*Project, error) {
	if !ValidSlug(slug) {
		return nil, fmt.Errorf("UpdateProject: invalid slug %q", slug)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	sr := m.client.Database("logs").Collection("projects").FindOneAndUpdate(
		ctx,
		bson.M{"_id": id},
		bson.D{{Key: "$set", Value: bson.D{
			{Key: "name", Value: name},
			{Key: "slug", Value: slug},
		}}},
		options.FindOneAndUpdate().SetReturnDocument(options.After),
	)
	if sr.Err() != nil {
		return nil, fmt.Errorf("UpdateProject: %w", sr.Err())
	}
	var p Project
	if err := sr.Decode(&p); err != nil {
		return nil, fmt.Errorf("UpdateProject decode: %w", err)
	}
	return &p, nil
}

// DeleteProject removes a project and all of its associated data (logs, API keys,
// settings, and members) in dependency order.
func (m *Models) DeleteProject(id primitive.ObjectID) error {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	projectIDStr := id.Hex()
	db := m.client.Database("logs")

	if _, err := db.Collection("logs").DeleteMany(ctx, bson.M{"project_id": projectIDStr}); err != nil {
		return fmt.Errorf("DeleteProject logs: %w", err)
	}
	if _, err := db.Collection("api_keys").DeleteMany(ctx, bson.M{"project_id": projectIDStr}); err != nil {
		return fmt.Errorf("DeleteProject api_keys: %w", err)
	}
	if _, err := db.Collection("settings").DeleteMany(ctx, bson.M{"project_id": projectIDStr}); err != nil {
		return fmt.Errorf("DeleteProject settings: %w", err)
	}
	if _, err := db.Collection("project_members").DeleteMany(ctx, bson.M{"project_id": id}); err != nil {
		return fmt.Errorf("DeleteProject project_members: %w", err)
	}
	if _, err := db.Collection("projects").DeleteOne(ctx, bson.M{"_id": id}); err != nil {
		return fmt.Errorf("DeleteProject project: %w", err)
	}
	return nil
}

// RemoveProjectMember removes a member from a project. Returns ErrLastOwner if
// the member is the sole remaining owner.
func (m *Models) RemoveProjectMember(projectID primitive.ObjectID, githubLogin string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	coll := m.client.Database("logs").Collection("project_members")

	var target ProjectMember
	if err := coll.FindOne(ctx, bson.M{"project_id": projectID, "github_login": githubLogin}).Decode(&target); err != nil {
		return fmt.Errorf("RemoveProjectMember: %w", err)
	}

	if target.Role == RoleOwner {
		n, err := coll.CountDocuments(ctx, bson.M{"project_id": projectID, "role": RoleOwner})
		if err != nil {
			return fmt.Errorf("RemoveProjectMember count owners: %w", err)
		}
		if n <= 1 {
			return ErrLastOwner
		}
	}

	result, err := coll.DeleteOne(ctx, bson.M{"project_id": projectID, "github_login": githubLogin})
	if err != nil {
		return fmt.Errorf("RemoveProjectMember delete: %w", err)
	}
	if result.DeletedCount == 0 {
		return fmt.Errorf("RemoveProjectMember: %w", mongo.ErrNoDocuments)
	}
	return nil
}

func (m *Models) IsMember(projectID primitive.ObjectID, githubLogin string) (bool, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	n, err := m.client.Database("logs").Collection("project_members").CountDocuments(ctx, bson.M{
		"project_id":   projectID,
		"github_login": githubLogin,
	})
	if err != nil {
		return false, fmt.Errorf("IsMember: %w", err)
	}
	return n > 0, nil
}

func (m *Models) GetProjectsForUser(githubLogin string) ([]Project, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	memberCursor, err := m.client.Database("logs").Collection("project_members").Find(ctx, bson.M{"github_login": githubLogin})
	if err != nil {
		return nil, fmt.Errorf("GetProjectsForUser members: %w", err)
	}
	defer memberCursor.Close(ctx)

	var members []ProjectMember
	if err := memberCursor.All(ctx, &members); err != nil {
		return nil, fmt.Errorf("GetProjectsForUser members decode: %w", err)
	}

	if len(members) == 0 {
		return []Project{}, nil
	}

	ids := make([]primitive.ObjectID, len(members))
	for i, mb := range members {
		ids[i] = mb.ProjectID
	}

	projectCursor, err := m.client.Database("logs").Collection("projects").Find(ctx, bson.M{"_id": bson.M{"$in": ids}})
	if err != nil {
		return nil, fmt.Errorf("GetProjectsForUser projects: %w", err)
	}
	defer projectCursor.Close(ctx)

	var projects []Project
	if err := projectCursor.All(ctx, &projects); err != nil {
		return nil, fmt.Errorf("GetProjectsForUser projects decode: %w", err)
	}
	return projects, nil
}
