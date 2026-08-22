package data

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// The project that pre-multi-tenancy data is adopted into on first start.
const (
	DefaultProjectName = "Default"
	DefaultProjectSlug = "default"
)

// legacyTTLIndexName is the global TTL index used before retention became a
// per-project setting enforced by the logger's cleanup loop.
const legacyTTLIndexName = "ttl_created_at"

// MongoDB error codes for dropping an index that was never there: the index is
// missing, or the collection itself has not been created yet.
const (
	indexNotFound     = 27
	namespaceNotFound = 26
)

// OrphanCounts reports how many documents in each project-scoped collection
// were written before project scoping existed.
type OrphanCounts struct {
	Logs     int64
	APIKeys  int64
	Settings int64
}

// Total is the number of orphaned documents across all collections.
func (c OrphanCounts) Total() int64 {
	return c.Logs + c.APIKeys + c.Settings
}

// MigrationReport summarises one run of MigrateOrphansToDefaultProject: the
// project the data was adopted into, and how much of it moved.
type MigrationReport struct {
	ProjectID string
	Logs      int64
	APIKeys   int64
	Settings  int64
	Owners    int64
}

// orphanedFilter matches documents written before project scoping existed. A
// pre-multi-tenancy document has no project_id at all; the null and empty-string
// cases cover data half-written by a build in between.
func orphanedFilter() bson.M {
	return bson.M{"$or": []bson.M{
		{"project_id": bson.M{"$exists": false}},
		{"project_id": nil},
		{"project_id": ""},
	}}
}

// ParseGithubLogins splits a comma-separated allowlist (as used by
// LOGWOLF_ALLOWED_GITHUB_USERS) into logins, dropping blanks and duplicates.
//
// Case is preserved: memberships are matched against the login GitHub returns
// at sign-in, exactly as the dashboard allowlist compares it.
func ParseGithubLogins(raw string) []string {
	var logins []string
	seen := make(map[string]bool)

	for _, part := range strings.Split(raw, ",") {
		login := strings.TrimSpace(part)
		if login == "" || seen[login] {
			continue
		}
		seen[login] = true
		logins = append(logins, login)
	}

	return logins
}

// CountOrphanedDocuments counts the documents in each project-scoped collection
// that carry no project ID.
func (m *Models) CountOrphanedDocuments(ctx context.Context) (OrphanCounts, error) {
	var counts OrphanCounts

	for _, c := range []struct {
		collection string
		into       *int64
	}{
		{"logs", &counts.Logs},
		{"api_keys", &counts.APIKeys},
		{"settings", &counts.Settings},
	} {
		n, err := m.client.Database("logs").Collection(c.collection).CountDocuments(ctx, orphanedFilter())
		if err != nil {
			return counts, fmt.Errorf("CountOrphanedDocuments %s: %w", c.collection, err)
		}
		*c.into = n
	}

	return counts, nil
}

// MigrateOrphansToDefaultProject adopts pre-multi-tenancy logs, API keys, and
// settings into a project named "Default", creating that project and an owner
// membership for each of owners if they do not exist yet.
//
// It is idempotent: it does nothing and returns a nil report once no orphaned
// documents remain, so it is safe to run on every start. A partially completed
// run leaves the remaining orphans behind for the next start to finish.
func (m *Models) MigrateOrphansToDefaultProject(ctx context.Context, owners []string) (*MigrationReport, error) {
	counts, err := m.CountOrphanedDocuments(ctx)
	if err != nil {
		return nil, err
	}
	if counts.Total() == 0 {
		return nil, nil
	}

	project, err := m.ensureDefaultProject()
	if err != nil {
		return nil, err
	}

	report := &MigrationReport{ProjectID: project.ID.Hex()}

	// The report is returned alongside any error: a run that fails partway still
	// moved whatever it reports, and the caller should say so.
	if report.Logs, err = m.adoptOrphans(ctx, "logs", report.ProjectID); err != nil {
		return report, err
	}
	if report.APIKeys, err = m.adoptOrphans(ctx, "api_keys", report.ProjectID); err != nil {
		return report, err
	}
	if report.Settings, err = m.adoptOrphans(ctx, "settings", report.ProjectID); err != nil {
		return report, err
	}

	report.Owners, err = m.ensureOwners(ctx, project.ID, owners)
	if err != nil {
		return report, err
	}

	return report, nil
}

// adoptOrphans stamps every project-less document in collection with projectID.
func (m *Models) adoptOrphans(ctx context.Context, collection, projectID string) (int64, error) {
	result, err := m.client.Database("logs").Collection(collection).UpdateMany(
		ctx,
		orphanedFilter(),
		bson.M{"$set": bson.M{"project_id": projectID}},
	)
	if err != nil {
		return 0, fmt.Errorf("adoptOrphans %s: %w", collection, err)
	}
	return result.ModifiedCount, nil
}

// ensureDefaultProject returns the Default project, creating it if needed.
func (m *Models) ensureDefaultProject() (*Project, error) {
	existing, err := m.GetProjectBySlug(DefaultProjectSlug)
	if err == nil {
		return existing, nil
	}
	if !errors.Is(err, mongo.ErrNoDocuments) {
		return nil, fmt.Errorf("ensureDefaultProject: %w", err)
	}

	created, err := m.InsertProject(Project{Name: DefaultProjectName, Slug: DefaultProjectSlug})
	if err == nil {
		return created, nil
	}
	// Another logger instance created it first — the unique slug index caught it.
	if mongo.IsDuplicateKeyError(err) {
		return m.GetProjectBySlug(DefaultProjectSlug)
	}
	return nil, fmt.Errorf("ensureDefaultProject: %w", err)
}

// ensureOwners gives each login an owner membership on the project, leaving any
// membership that already exists untouched. Returns how many were created.
func (m *Models) ensureOwners(ctx context.Context, projectID primitive.ObjectID, logins []string) (int64, error) {
	collection := m.client.Database("logs").Collection("project_members")
	var created int64

	for _, login := range logins {
		result, err := collection.UpdateOne(
			ctx,
			bson.M{"project_id": projectID, "github_login": login},
			bson.M{"$setOnInsert": bson.M{
				"project_id":   projectID,
				"github_login": login,
				"role":         RoleOwner,
				"created_at":   time.Now(),
			}},
			options.Update().SetUpsert(true),
		)
		if err != nil {
			return created, fmt.Errorf("ensureOwners %s: %w", login, err)
		}
		if result.UpsertedCount > 0 {
			created++
		}
	}

	return created, nil
}

// DropLegacyTTLIndex removes the global TTL index that pre-multi-tenancy builds
// kept on logs.created_at. Retention is per project now, so leaving the old
// index in place would keep expiring logs on the previous global schedule no
// matter what a project has configured.
//
// Reports whether an index was actually dropped; dropping a missing index is
// not an error, so this is safe to call on every start.
func (m *Models) DropLegacyTTLIndex(ctx context.Context) (bool, error) {
	_, err := m.client.Database("logs").Collection("logs").Indexes().DropOne(ctx, legacyTTLIndexName)
	if err != nil {
		var cmdErr mongo.CommandError
		if errors.As(err, &cmdErr) && (cmdErr.Code == indexNotFound || cmdErr.Code == namespaceNotFound) {
			return false, nil
		}
		return false, fmt.Errorf("DropLegacyTTLIndex: %w", err)
	}
	return true, nil
}
