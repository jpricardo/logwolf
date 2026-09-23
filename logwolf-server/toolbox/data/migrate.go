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

// OwnerRepair summarises one run of EnsureDefaultProjectOwners: the ownerless
// Default project it found, and how many owners it gave it.
type OwnerRepair struct {
	ProjectID string
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
// LOGWOLF_ALLOWED_GITHUB_USERS) into normalized logins, dropping blanks and
// duplicates. Logins that differ only in case are duplicates: GitHub treats
// them as one account.
func ParseGithubLogins(raw string) []string {
	var logins []string
	seen := make(map[string]bool)

	for _, part := range strings.Split(raw, ",") {
		login := NormalizeGithubLogin(part)
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
// run leaves the remaining orphans behind for the next start to finish; owners
// it failed to add are EnsureDefaultProjectOwners' job.
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

	report.Owners, err = m.ensureOwners(ctx, project.ID, owners, false)
	if err != nil {
		return report, err
	}

	return report, nil
}

// EnsureDefaultProjectOwners gives an ownerless Default project an owner
// membership for each of owners. Only an owner can add members, so without this
// a Default project left with no owner — by an owner step that failed after the
// data moved, or by a first start with no owners configured — would stay
// unreachable for good: MigrateOrphansToDefaultProject is a no-op once the
// orphans are gone and never gets another chance to add them.
//
// It runs independently of the orphan count, so it is safe to call on every
// start. It returns a nil report when there is no Default project or it already
// has an owner; a Default project with owners is left alone, so a login removed
// from it through the dashboard is not added back. Members of an ownerless
// project who appear in owners are promoted. A report with zero Owners means the
// project is still ownerless because owners was empty.
func (m *Models) EnsureDefaultProjectOwners(ctx context.Context, owners []string) (*OwnerRepair, error) {
	project, err := m.GetProjectBySlug(DefaultProjectSlug)
	if errors.Is(err, mongo.ErrNoDocuments) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("EnsureDefaultProjectOwners: %w", err)
	}

	n, err := m.client.Database("logs").Collection("project_members").CountDocuments(ctx, bson.M{
		"project_id": project.ID,
		"role":       RoleOwner,
	})
	if err != nil {
		return nil, fmt.Errorf("EnsureDefaultProjectOwners: %w", err)
	}
	if n > 0 {
		return nil, nil
	}

	repair := &OwnerRepair{ProjectID: project.ID.Hex()}
	// Promote rather than skip existing memberships: a project with no owner can
	// still have plain members, and one of them may be on the owners list.
	repair.Owners, err = m.ensureOwners(ctx, project.ID, owners, true)
	return repair, err
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

// ensureOwners gives each login an owner membership on the project. A
// membership that already exists is left untouched unless promote is set, in
// which case a plain member is made an owner. Returns how many logins became
// owners.
func (m *Models) ensureOwners(ctx context.Context, projectID primitive.ObjectID, logins []string, promote bool) (int64, error) {
	collection := m.client.Database("logs").Collection("project_members")
	var changed int64

	for _, login := range logins {
		login = NormalizeGithubLogin(login)
		update := bson.M{"$setOnInsert": bson.M{
			"project_id":   projectID,
			"github_login": login,
			"role":         RoleOwner,
			"created_at":   time.Now(),
		}}
		if promote {
			update = bson.M{
				"$set": bson.M{"role": RoleOwner},
				"$setOnInsert": bson.M{
					"project_id":   projectID,
					"github_login": login,
					"created_at":   time.Now(),
				},
			}
		}

		result, err := collection.UpdateOne(
			ctx,
			bson.M{"project_id": projectID, "github_login": login},
			update,
			options.Update().SetUpsert(true),
		)
		if err != nil {
			return changed, fmt.Errorf("ensureOwners %s: %w", login, err)
		}
		changed += result.UpsertedCount + result.ModifiedCount
	}

	return changed, nil
}

// LoginNormalization summarises one run of NormalizeMemberLogins: how many
// memberships it rewrote to their normalized login, and how many case-only
// duplicates it merged into them.
type LoginNormalization struct {
	Normalized int64
	Merged     int64
}

// NormalizeMemberLogins rewrites memberships stored before logins were
// normalized, so the membership checks, which look logins up normalized, find
// them again. Where a project holds the same login in several casings, the rows
// merge into one that keeps the oldest join date and the highest role, so no one
// loses access they had under either casing.
//
// It is idempotent and does nothing once every login is normalized, so it is
// safe to run on every start. Each login is merged in its own transaction; a run
// that fails partway leaves the rest for the next start.
func (m *Models) NormalizeMemberLogins(ctx context.Context) (LoginNormalization, error) {
	var report LoginNormalization
	coll := m.client.Database("logs").Collection("project_members")

	// The collection holds one row per user per project, so it is small enough
	// to scan, and comparing in Go applies exactly the normalization new writes get.
	cursor, err := coll.Find(ctx, bson.M{}, options.Find().SetProjection(bson.M{"project_id": 1, "github_login": 1}))
	if err != nil {
		return report, fmt.Errorf("NormalizeMemberLogins: %w", err)
	}
	var members []ProjectMember
	if err := cursor.All(ctx, &members); err != nil {
		return report, fmt.Errorf("NormalizeMemberLogins decode: %w", err)
	}

	type memberKey struct {
		projectID primitive.ObjectID
		login     string
	}
	var stale []memberKey
	seen := make(map[memberKey]bool)
	for _, mb := range members {
		k := memberKey{mb.ProjectID, NormalizeGithubLogin(mb.GithubLogin)}
		if mb.GithubLogin != k.login && !seen[k] {
			seen[k] = true
			stale = append(stale, k)
		}
	}
	if len(stale) == 0 {
		return report, nil
	}

	session, err := m.client.StartSession()
	if err != nil {
		return report, fmt.Errorf("NormalizeMemberLogins start session: %w", err)
	}
	defer session.EndSession(ctx)

	for _, k := range stale {
		merged, err := m.mergeMemberLogin(ctx, session, k.projectID, k.login)
		if err != nil {
			return report, err
		}
		report.Normalized++
		report.Merged += merged
	}
	return report, nil
}

// mergeMemberLogin collapses every membership of projectID whose login
// normalizes to login into a single row stored under login. Returns how many
// rows it deleted.
func (m *Models) mergeMemberLogin(ctx context.Context, session mongo.Session, projectID primitive.ObjectID, login string) (int64, error) {
	coll := m.client.Database("logs").Collection("project_members")

	merged, err := session.WithTransaction(ctx, func(sc mongo.SessionContext) (any, error) {
		cursor, err := coll.Find(sc, bson.M{"project_id": projectID})
		if err != nil {
			return int64(0), fmt.Errorf("mergeMemberLogin %s: %w", login, err)
		}
		var members []ProjectMember
		if err := cursor.All(sc, &members); err != nil {
			return int64(0), fmt.Errorf("mergeMemberLogin %s decode: %w", login, err)
		}

		var group []ProjectMember
		for _, mb := range members {
			if NormalizeGithubLogin(mb.GithubLogin) == login {
				group = append(group, mb)
			}
		}
		if len(group) == 0 {
			return int64(0), nil
		}

		keep := group[0]
		role := RoleMember
		for _, mb := range group {
			if mb.CreatedAt.Before(keep.CreatedAt) {
				keep = mb
			}
			if mb.Role == RoleOwner {
				role = RoleOwner
			}
		}

		var drop []primitive.ObjectID
		for _, mb := range group {
			if mb.ID != keep.ID {
				drop = append(drop, mb.ID)
			}
		}

		// The duplicates go first: the unique (project_id, github_login) index
		// would refuse the rename while a row already holds login.
		if len(drop) > 0 {
			if _, err := coll.DeleteMany(sc, bson.M{"_id": bson.M{"$in": drop}}); err != nil {
				return int64(0), fmt.Errorf("mergeMemberLogin %s delete: %w", login, err)
			}
		}
		if _, err := coll.UpdateOne(sc,
			bson.M{"_id": keep.ID},
			bson.M{"$set": bson.M{"github_login": login, "role": role}},
		); err != nil {
			return int64(0), fmt.Errorf("mergeMemberLogin %s update: %w", login, err)
		}
		return int64(len(drop)), nil
	})
	if err != nil {
		return 0, err
	}
	return merged.(int64), nil
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
