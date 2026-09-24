package data

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// ErrLastOwner is returned when an operation would remove or demote the last
// owner of a project.
var ErrLastOwner = errors.New("cannot remove the last owner of a project")

// ErrProjectExists is returned by PurgeProjectLogs for a project that has not
// been deleted.
var ErrProjectExists = errors.New("project still exists")

// ErrUnknownProject is what the logger's LogInfo answers for an event whose
// project does not exist, usually one deleted while the event sat in RabbitMQ.
// The listener reads it back out of the RPC error and drops the event rather
// than retrying it.
var ErrUnknownProject = errors.New("project does not exist")

const (
	RoleOwner  = "owner"
	RoleMember = "member"
)

var slugRe = regexp.MustCompile(`^[a-z0-9]+(?:-[a-z0-9]+)*$`)

// Project is a tenant: logs, API keys, settings and members all belong to one.
//
// Its Slug is a label derived from the name when the project is created, for
// display only. Nothing looks a project up by it and it is not unique: projects
// of different users may share one, so creating a project never tells anyone
// that another user's project exists. The Default project that holds
// pre-multi-tenancy data is found by its Default flag instead.
type Project struct {
	ID        primitive.ObjectID `bson:"_id,omitempty" json:"id,omitempty"`
	Name      string             `bson:"name" json:"name"`
	Slug      string             `bson:"slug" json:"slug"`
	CreatedAt time.Time          `bson:"created_at" json:"created_at"`
	// Default marks the project the startup migration adopts project-less data
	// into. At most one project has it; see EnsureProjectIndexes.
	Default bool `bson:"default,omitempty" json:"-"`
}

type ProjectMember struct {
	ID          primitive.ObjectID `bson:"_id,omitempty" json:"id,omitempty"`
	ProjectID   primitive.ObjectID `bson:"project_id" json:"project_id"`
	GithubLogin string             `bson:"github_login" json:"github_login"`
	Role        string             `bson:"role" json:"role"`
	CreatedAt   time.Time          `bson:"created_at" json:"created_at"`
}

// UserProject is a project as seen by one user: the project itself plus the
// role that user holds in it. Callers listing "my projects" need both, and the
// role is never a property of the project on its own.
type UserProject struct {
	Project
	Role string `json:"role"`
}

// RPC argument types for project and member operations.

// RPCCreateProjectArgs is the RPC argument for CreateProject. Owner is the login
// that gets the owner membership, created with the project in one transaction.
type RPCCreateProjectArgs struct {
	Name  string
	Slug  string
	Owner string
}

// RPCProjectIDArgs is the RPC argument for calls that take only a project ID.
type RPCProjectIDArgs struct {
	ID string
}

// RPCUpdateProjectArgs is the RPC argument for UpdateProject. Only the name
// changes: the slug is fixed when the project is created.
type RPCUpdateProjectArgs struct {
	ID   string
	Name string
}

// RPCUserProjectsArgs is the RPC argument for ListUserProjects.
type RPCUserProjectsArgs struct {
	GithubLogin string
}

// RPCAddMemberArgs is the RPC argument for AddMember.
type RPCAddMemberArgs struct {
	ProjectID   string
	GithubLogin string
	Role        string
}

// RPCRemoveMemberArgs is the RPC argument for RemoveMember.
type RPCRemoveMemberArgs struct {
	ProjectID   string
	GithubLogin string
}

// RPCUpdateMemberRoleArgs is the RPC argument for UpdateMemberRole.
type RPCUpdateMemberRoleArgs struct {
	ProjectID   string
	GithubLogin string
	Role        string
}

// RPCCheckMembershipArgs is the RPC argument for CheckMembership.
type RPCCheckMembershipArgs struct {
	ProjectID   string
	GithubLogin string
}

// ValidSlug reports whether s is a valid URL-safe slug.
func ValidSlug(s string) bool {
	return slugRe.MatchString(s)
}

// ValidRole reports whether r is a recognised project member role.
func ValidRole(r string) bool {
	return r == RoleOwner || r == RoleMember
}

// NormalizeGithubLogin returns the form a GitHub login is stored and compared
// in. GitHub logins are case-insensitive, and GitHub hands back its own casing
// at sign-in (JDoe) whatever an owner typed on the settings page (jdoe), so
// every membership read and write goes through this: otherwise the two never
// match, and the unique (project_id, github_login) index lets both in as
// separate members.
func NormalizeGithubLogin(login string) string {
	return strings.ToLower(strings.TrimSpace(login))
}

// EnsureProjectIndexes creates the required indexes for projects and project_members.
// Safe to call on startup — CreateOne is idempotent for identical index definitions.
//
// Slugs used to have a unique index of their own; MarkDefaultProject retires it
// during the startup migration, once it has done its last job.
func (m *Models) EnsureProjectIndexes() error {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	// Unique over the projects that have the flag, and only those: every other
	// project leaves it out, so this admits one Default project and any number
	// of others.
	projects := m.client.Database("logs").Collection("projects")
	if _, err := projects.Indexes().CreateOne(ctx, mongo.IndexModel{
		Keys: bson.D{{Key: "default", Value: 1}},
		Options: options.Index().SetUnique(true).SetName(defaultProjectIndexName).
			SetPartialFilterExpression(bson.M{"default": true}),
	}); err != nil {
		return fmt.Errorf("EnsureProjectIndexes projects.default: %w", err)
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

// CreateProjectWithOwner inserts a project and an owner membership for login in
// one transaction. Only an owner can add members, so a project that exists
// without one is unreachable for good; here either both documents are written or
// neither is.
func (m *Models) CreateProjectWithOwner(p Project, login string) (*Project, error) {
	login = NormalizeGithubLogin(login)
	if login == "" {
		return nil, errors.New("CreateProjectWithOwner: owner login is required")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	session, err := m.client.StartSession()
	if err != nil {
		return nil, fmt.Errorf("CreateProjectWithOwner start session: %w", err)
	}
	defer session.EndSession(ctx)

	p.ID = primitive.NewObjectID()
	p.CreatedAt = time.Now()
	owner := ProjectMember{
		ID:          primitive.NewObjectID(),
		ProjectID:   p.ID,
		GithubLogin: login,
		Role:        RoleOwner,
		CreatedAt:   p.CreatedAt,
	}

	// The ids are fixed before the transaction, so a retry by WithTransaction
	// writes the same two documents again rather than a second project.
	db := m.client.Database("logs")
	_, err = session.WithTransaction(ctx, func(sc mongo.SessionContext) (any, error) {
		if _, err := db.Collection("projects").InsertOne(sc, p); err != nil {
			return nil, fmt.Errorf("CreateProjectWithOwner project: %w", err)
		}
		if _, err := db.Collection("project_members").InsertOne(sc, owner); err != nil {
			return nil, fmt.Errorf("CreateProjectWithOwner owner: %w", err)
		}
		return nil, nil
	})
	if err != nil {
		return nil, err
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

// ProjectExists reports whether a project with the given id exists.
func (m *Models) ProjectExists(ctx context.Context, id primitive.ObjectID) (bool, error) {
	n, err := m.client.Database("logs").Collection("projects").CountDocuments(ctx, bson.M{"_id": id}, options.Count().SetLimit(1))
	if err != nil {
		return false, fmt.Errorf("ProjectExists: %w", err)
	}
	return n > 0, nil
}

func (m *Models) InsertProjectMember(pm ProjectMember) (*ProjectMember, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	pm.ID = primitive.NewObjectID()
	pm.GithubLogin = NormalizeGithubLogin(pm.GithubLogin)
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

// RenameProject changes a project's name. Its slug stays what it was when the
// project was created.
func (m *Models) RenameProject(id primitive.ObjectID, name string) (*Project, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	sr := m.client.Database("logs").Collection("projects").FindOneAndUpdate(
		ctx,
		bson.M{"_id": id},
		bson.D{{Key: "$set", Value: bson.D{
			{Key: "name", Value: name},
		}}},
		options.FindOneAndUpdate().SetReturnDocument(options.After),
	)
	if sr.Err() != nil {
		return nil, fmt.Errorf("RenameProject: %w", sr.Err())
	}
	var p Project
	if err := sr.Decode(&p); err != nil {
		return nil, fmt.Errorf("RenameProject decode: %w", err)
	}
	return &p, nil
}

// DeleteProject removes a project with its API keys, settings, and members in one
// transaction: either every collection loses the project's documents or none
// does, so a failure part-way leaves nothing to clean up by hand. Transactions
// need MongoDB to run as a replica set.
//
// The project's logs are not part of it. There can be millions, and deleting
// them inside the transaction would outlast both the timeout here and MongoDB's
// transaction lifetime limit, so the project could never be deleted. Once this
// returns they belong to no project: nobody can read them, PurgeProjectLogs
// removes them, and DeleteOrphanedLogs catches any it misses.
func (m *Models) DeleteProject(id primitive.ObjectID) error {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	session, err := m.client.StartSession()
	if err != nil {
		return fmt.Errorf("DeleteProject start session: %w", err)
	}
	defer session.EndSession(ctx)

	db := m.client.Database("logs")

	// WithTransaction may run the callback more than once on a transient error;
	// every step is a delete by filter, so a rerun is harmless.
	_, err = session.WithTransaction(ctx, func(sc mongo.SessionContext) (any, error) {
		if _, err := db.Collection("api_keys").DeleteMany(sc, bson.M{"project_id": id}); err != nil {
			return nil, fmt.Errorf("DeleteProject api_keys: %w", err)
		}
		if _, err := db.Collection("settings").DeleteMany(sc, bson.M{"project_id": id}); err != nil {
			return nil, fmt.Errorf("DeleteProject settings: %w", err)
		}
		if _, err := db.Collection("project_members").DeleteMany(sc, bson.M{"project_id": id}); err != nil {
			return nil, fmt.Errorf("DeleteProject project_members: %w", err)
		}
		if _, err := db.Collection("projects").DeleteOne(sc, bson.M{"_id": id}); err != nil {
			return nil, fmt.Errorf("DeleteProject project: %w", err)
		}
		return nil, nil
	})
	return err
}

// RemoveProjectMember removes a member from a project. Returns ErrLastOwner if
// the member is the sole remaining owner.
func (m *Models) RemoveProjectMember(projectID primitive.ObjectID, githubLogin string) error {
	githubLogin = NormalizeGithubLogin(githubLogin)

	return m.changeMembers("RemoveProjectMember", projectID, func(sc mongo.SessionContext, coll *mongo.Collection) error {
		var target ProjectMember
		if err := coll.FindOne(sc, bson.M{"project_id": projectID, "github_login": githubLogin}).Decode(&target); err != nil {
			return fmt.Errorf("RemoveProjectMember: %w", err)
		}

		if target.Role == RoleOwner {
			if err := refuseLastOwner(sc, coll, projectID); err != nil {
				return fmt.Errorf("RemoveProjectMember: %w", err)
			}
		}

		result, err := coll.DeleteOne(sc, bson.M{"project_id": projectID, "github_login": githubLogin})
		if err != nil {
			return fmt.Errorf("RemoveProjectMember delete: %w", err)
		}
		if result.DeletedCount == 0 {
			return fmt.Errorf("RemoveProjectMember: %w", mongo.ErrNoDocuments)
		}
		return nil
	})
}

// UpdateProjectMemberRole gives an existing member a new role. Returns
// ErrLastOwner if that would demote the sole remaining owner, and
// mongo.ErrNoDocuments if the login is not a member. Setting the role a member
// already holds changes nothing.
func (m *Models) UpdateProjectMemberRole(projectID primitive.ObjectID, githubLogin, role string) error {
	if !ValidRole(role) {
		return fmt.Errorf("UpdateProjectMemberRole: invalid role %q", role)
	}
	githubLogin = NormalizeGithubLogin(githubLogin)

	return m.changeMembers("UpdateProjectMemberRole", projectID, func(sc mongo.SessionContext, coll *mongo.Collection) error {
		var target ProjectMember
		if err := coll.FindOne(sc, bson.M{"project_id": projectID, "github_login": githubLogin}).Decode(&target); err != nil {
			return fmt.Errorf("UpdateProjectMemberRole: %w", err)
		}
		if target.Role == role {
			return nil
		}

		if target.Role == RoleOwner {
			if err := refuseLastOwner(sc, coll, projectID); err != nil {
				return fmt.Errorf("UpdateProjectMemberRole: %w", err)
			}
		}

		if _, err := coll.UpdateOne(sc, bson.M{"_id": target.ID}, bson.M{"$set": bson.M{"role": role}}); err != nil {
			return fmt.Errorf("UpdateProjectMemberRole update: %w", err)
		}
		return nil
	})
}

// changeMembers runs fn, a change to a project's members that must never leave
// it without an owner, in one transaction.
//
// A transaction alone is not enough: two requests removing or demoting two
// different owners would each read two owners from their own snapshot, write
// different documents, never conflict, and leave the project with none. So the
// transaction first writes to the project's own document. The second one to get
// there hits a write conflict, WithTransaction retries it, and the retry counts
// the owners after the first change has committed.
func (m *Models) changeMembers(op string, projectID primitive.ObjectID, fn func(sc mongo.SessionContext, coll *mongo.Collection) error) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	session, err := m.client.StartSession()
	if err != nil {
		return fmt.Errorf("%s start session: %w", op, err)
	}
	defer session.EndSession(ctx)

	db := m.client.Database("logs")

	_, err = session.WithTransaction(ctx, func(sc mongo.SessionContext) (any, error) {
		// A membership row can outlive its project; with no project document to
		// write to there is nothing to serialize on, and no owner left to protect.
		if _, err := db.Collection("projects").UpdateOne(sc,
			bson.M{"_id": projectID},
			bson.M{"$currentDate": bson.M{"members_updated_at": true}},
		); err != nil {
			return nil, fmt.Errorf("%s lock project: %w", op, err)
		}
		return nil, fn(sc, db.Collection("project_members"))
	})
	return err
}

// refuseLastOwner returns ErrLastOwner unless the project has an owner besides
// the one about to be removed or demoted. The count is only safe from concurrent
// changes inside changeMembers.
func refuseLastOwner(sc mongo.SessionContext, coll *mongo.Collection, projectID primitive.ObjectID) error {
	n, err := coll.CountDocuments(sc, bson.M{"project_id": projectID, "role": RoleOwner})
	if err != nil {
		return fmt.Errorf("count owners: %w", err)
	}
	if n <= 1 {
		return ErrLastOwner
	}
	return nil
}

func (m *Models) IsMember(projectID primitive.ObjectID, githubLogin string) (bool, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	n, err := m.client.Database("logs").Collection("project_members").CountDocuments(ctx, bson.M{
		"project_id":   projectID,
		"github_login": NormalizeGithubLogin(githubLogin),
	})
	if err != nil {
		return false, fmt.Errorf("IsMember: %w", err)
	}
	return n > 0, nil
}

func (m *Models) GetAllProjects(ctx context.Context) ([]Project, error) {
	cursor, err := m.client.Database("logs").Collection("projects").Find(ctx, bson.M{})
	if err != nil {
		return nil, fmt.Errorf("GetAllProjects: %w", err)
	}
	defer cursor.Close(ctx)

	var projects []Project
	if err := cursor.All(ctx, &projects); err != nil {
		return nil, fmt.Errorf("GetAllProjects decode: %w", err)
	}
	return projects, nil
}

// GetProjectsForUser returns every project the user is a member of, each paired
// with the role they hold in it.
func (m *Models) GetProjectsForUser(githubLogin string) ([]UserProject, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	memberCursor, err := m.client.Database("logs").Collection("project_members").Find(ctx, bson.M{"github_login": NormalizeGithubLogin(githubLogin)})
	if err != nil {
		return nil, fmt.Errorf("GetProjectsForUser members: %w", err)
	}
	defer memberCursor.Close(ctx)

	var members []ProjectMember
	if err := memberCursor.All(ctx, &members); err != nil {
		return nil, fmt.Errorf("GetProjectsForUser members decode: %w", err)
	}

	if len(members) == 0 {
		return []UserProject{}, nil
	}

	ids := make([]primitive.ObjectID, len(members))
	roles := make(map[primitive.ObjectID]string, len(members))
	for i, mb := range members {
		ids[i] = mb.ProjectID
		roles[mb.ProjectID] = mb.Role
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

	// A membership row can outlive its project (the project was deleted while the
	// row lingers); the projects query is what decides which entries survive.
	result := make([]UserProject, 0, len(projects))
	for _, p := range projects {
		result = append(result, UserProject{Project: p, Role: roles[p.ID]})
	}
	return result, nil
}
