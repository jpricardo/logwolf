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

// defaultProjectIndexName is the partial unique index that allows one project
// with the Default flag.
const defaultProjectIndexName = "unique_default"

// legacySlugIndexName is the unique index slugs had while they were globally
// unique. MarkDefaultProject drops it.
const legacySlugIndexName = "unique_slug"

// hexObjectIDPattern matches the hex form of an ObjectID, which is how
// project_id was stored in logs, api_keys and settings before ConvertProjectIDs.
const hexObjectIDPattern = "^[0-9a-fA-F]{24}$"

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

// ProjectIDConversion summarises one run of ConvertProjectIDs: how many
// documents of each collection had their project_id turned into an ObjectID.
type ProjectIDConversion struct {
	Logs     int64
	APIKeys  int64
	Settings int64
}

// Total is the number of documents converted across all collections.
func (c ProjectIDConversion) Total() int64 {
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

	project, err := m.ensureDefaultProject(ctx)
	if err != nil {
		return nil, err
	}

	report := &MigrationReport{ProjectID: project.ID.Hex()}

	// The report is returned alongside any error: a run that fails partway still
	// moved whatever it reports, and the caller should say so.
	if report.Logs, err = m.adoptOrphans(ctx, "logs", project.ID); err != nil {
		return report, err
	}
	if report.APIKeys, err = m.adoptOrphans(ctx, "api_keys", project.ID); err != nil {
		return report, err
	}
	if report.Settings, err = m.adoptOrphans(ctx, "settings", project.ID); err != nil {
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
	project, err := m.getDefaultProject(ctx)
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
func (m *Models) adoptOrphans(ctx context.Context, collection string, projectID primitive.ObjectID) (int64, error) {
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

// getDefaultProject returns the project with the Default flag, or an error
// wrapping mongo.ErrNoDocuments if there is none.
func (m *Models) getDefaultProject(ctx context.Context) (*Project, error) {
	var p Project
	if err := m.client.Database("logs").Collection("projects").FindOne(ctx, bson.M{"default": true}).Decode(&p); err != nil {
		return nil, fmt.Errorf("getDefaultProject: %w", err)
	}
	return &p, nil
}

// ensureDefaultProject returns the Default project, creating it if needed.
func (m *Models) ensureDefaultProject(ctx context.Context) (*Project, error) {
	existing, err := m.getDefaultProject(ctx)
	if err == nil {
		return existing, nil
	}
	if !errors.Is(err, mongo.ErrNoDocuments) {
		return nil, fmt.Errorf("ensureDefaultProject: %w", err)
	}

	created, err := m.InsertProject(Project{Name: DefaultProjectName, Slug: DefaultProjectSlug, Default: true})
	if err == nil {
		return created, nil
	}
	// Another logger instance created it first — the unique_default index caught it.
	if mongo.IsDuplicateKeyError(err) {
		return m.getDefaultProject(ctx)
	}
	return nil, fmt.Errorf("ensureDefaultProject: %w", err)
}

// MarkDefaultProject gives the Default project created by earlier builds its
// Default flag, then drops the unique index slugs had until now. Those builds
// found the project by its slug, default, which only one project could have
// while that index was there; once it is gone any project may be called that.
//
// Only the presence of the old index triggers it, so it does its work once and
// is a no-op on every start after. It reports whether it retired the index. If
// it fails, the index stays and the next start tries again.
func (m *Models) MarkDefaultProject(ctx context.Context) (bool, error) {
	projects := m.client.Database("logs").Collection("projects")

	// No collection yet means no index either.
	var cmdErr mongo.CommandError
	specs, err := projects.Indexes().ListSpecifications(ctx)
	if errors.As(err, &cmdErr) && cmdErr.Code == namespaceNotFound {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("MarkDefaultProject list indexes: %w", err)
	}
	legacy := false
	for _, spec := range specs {
		if spec.Name == legacySlugIndexName {
			legacy = true
		}
	}
	if !legacy {
		return false, nil
	}

	if _, err := projects.UpdateOne(ctx,
		bson.M{"slug": DefaultProjectSlug},
		bson.M{"$set": bson.M{"default": true}},
	); err != nil {
		return false, fmt.Errorf("MarkDefaultProject mark: %w", err)
	}

	if _, err := projects.Indexes().DropOne(ctx, legacySlugIndexName); err != nil {
		// Another logger instance got there first.
		if errors.As(err, &cmdErr) && cmdErr.Code == indexNotFound {
			return false, nil
		}
		return false, fmt.Errorf("MarkDefaultProject drop %s: %w", legacySlugIndexName, err)
	}
	return true, nil
}

// ConvertProjectIDs rewrites project_id in logs, api_keys and settings from the
// hex string earlier builds stored to the ObjectID that projects._id and
// project_members.project_id have always used. A filter with the wrong type
// matches nothing and says nothing, so every project_id has to share one.
//
// It converts in batches and is idempotent: a run that fails or times out
// part-way keeps what it converted, and the next start carries on. A project_id
// that is not hex is left as it is; it names no project either way, and the
// cleanup loop's orphan sweep deletes such logs.
func (m *Models) ConvertProjectIDs(ctx context.Context) (ProjectIDConversion, error) {
	var report ProjectIDConversion

	for _, c := range []struct {
		collection string
		into       *int64
	}{
		{"logs", &report.Logs},
		{"api_keys", &report.APIKeys},
		{"settings", &report.Settings},
	} {
		n, err := m.convertProjectIDs(ctx, c.collection)
		*c.into = n
		if err != nil {
			return report, err
		}
	}
	return report, nil
}

// convertProjectIDs converts the hex string project_ids of one collection,
// project by project.
func (m *Models) convertProjectIDs(ctx context.Context, collection string) (int64, error) {
	coll := m.client.Database("logs").Collection(collection)

	values, err := coll.Distinct(ctx, "project_id", bson.M{"project_id": bson.M{
		"$type":  "string",
		"$regex": hexObjectIDPattern,
	}})
	if err != nil {
		return 0, fmt.Errorf("ConvertProjectIDs %s: %w", collection, err)
	}

	var total int64
	for _, v := range values {
		hex, _ := v.(string)
		id, err := primitive.ObjectIDFromHex(hex)
		if err != nil {
			continue
		}

		for {
			n, more, err := convertProjectIDBatch(ctx, coll, hex, id)
			total += n
			if err != nil {
				return total, fmt.Errorf("ConvertProjectIDs %s project %s: %w", collection, hex, err)
			}
			if !more {
				break
			}
		}
	}
	return total, nil
}

// convertProjectIDBatch converts up to logDeleteBatch documents filed under hex
// to id, and reports whether there may be more.
//
// settings has a unique (project_id, key) index. A project whose retention was
// set by this build while an older string document was still waiting to be
// converted has both, and converting the old one would collide with the new
// one; the new one is what the project last chose, so the old one is dropped.
func convertProjectIDBatch(ctx context.Context, coll *mongo.Collection, hex string, id primitive.ObjectID) (int64, bool, error) {
	ctx, cancel := context.WithTimeout(ctx, logBatchTimeout)
	defer cancel()

	cursor, err := coll.Find(ctx, bson.M{"project_id": hex}, options.Find().
		SetProjection(bson.M{"_id": 1}).
		SetLimit(logDeleteBatch))
	if err != nil {
		return 0, false, fmt.Errorf("find batch: %w", err)
	}
	var docs []bson.M
	if err := cursor.All(ctx, &docs); err != nil {
		return 0, false, fmt.Errorf("decode batch: %w", err)
	}
	if len(docs) == 0 {
		return 0, false, nil
	}

	ids := make(bson.A, len(docs))
	for i, d := range docs {
		ids[i] = d["_id"]
	}

	result, err := coll.UpdateMany(ctx,
		bson.M{"_id": bson.M{"$in": ids}, "project_id": hex},
		bson.M{"$set": bson.M{"project_id": id}},
	)
	if mongo.IsDuplicateKeyError(err) {
		n, err := convertProjectIDsOneByOne(ctx, coll, ids, hex, id)
		return n, len(docs) == logDeleteBatch, err
	}
	if err != nil {
		return 0, false, fmt.Errorf("update batch: %w", err)
	}
	return result.ModifiedCount, len(docs) == logDeleteBatch, nil
}

// convertProjectIDsOneByOne is convertProjectIDBatch's way through a batch that
// hit a unique index: it converts what it can and deletes the documents a
// converted one already stands for.
func convertProjectIDsOneByOne(ctx context.Context, coll *mongo.Collection, ids bson.A, hex string, id primitive.ObjectID) (int64, error) {
	var converted int64
	for _, docID := range ids {
		result, err := coll.UpdateOne(ctx,
			bson.M{"_id": docID, "project_id": hex},
			bson.M{"$set": bson.M{"project_id": id}},
		)
		if mongo.IsDuplicateKeyError(err) {
			if _, err := coll.DeleteOne(ctx, bson.M{"_id": docID, "project_id": hex}); err != nil {
				return converted, fmt.Errorf("drop superseded document: %w", err)
			}
			continue
		}
		if err != nil {
			return converted, fmt.Errorf("update document: %w", err)
		}
		converted += result.ModifiedCount
	}
	return converted, nil
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
