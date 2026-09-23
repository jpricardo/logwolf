package main

import (
	"context"
	"log"
	"logwolf-toolbox/data"
	"os"
	"time"
)

// migrationTimeout bounds the whole startup migration. A large logs collection
// takes a while to stamp, but startup must not hang forever.
const migrationTimeout = 5 * time.Minute

// defaultProjectOwners returns who should own data that predates projects: the
// dashboard's users allowlist plus LOGWOLF_DEFAULT_PROJECT_OWNERS. The second
// list exists for deployments that admit users through
// LOGWOLF_ALLOWED_GITHUB_ORGS only — org membership is not visible from Logger,
// so it cannot pick owners out of an org by itself.
func defaultProjectOwners() []string {
	return data.ParseGithubLogins(os.Getenv("LOGWOLF_ALLOWED_GITHUB_USERS") + "," + os.Getenv("LOGWOLF_DEFAULT_PROJECT_OWNERS"))
}

// runStartupMigration normalizes member logins and adopts any pre-multi-tenancy
// data into the Default project before the RPC server starts accepting
// connections, then makes sure that project has an owner. It is silent when
// there is nothing to do.
//
// Failures are logged rather than fatal: the migration is idempotent, so a
// crash-looping logger helps nobody when the next start would retry anyway.
func (app *Config) runStartupMigration() {
	ctx, cancel := context.WithTimeout(context.Background(), migrationTimeout)
	defer cancel()

	if dropped, err := app.Models.DropLegacyTTLIndex(ctx); err != nil {
		log.Printf("Migration: could not drop the legacy TTL index: %v", err)
	} else if dropped {
		log.Println("Migration: dropped the legacy global TTL index on logs — retention is per project now")
	}

	// Before the owner steps below: they look owners up by normalized login, and
	// would otherwise add a second membership next to one stored in another casing.
	logins, err := app.Models.NormalizeMemberLogins(ctx)
	if err != nil {
		log.Printf("Migration: FAILED to normalize member logins, will retry on the next start: %v", err)
	}
	if logins.Normalized > 0 {
		log.Printf("Migration: normalized member logins to lowercase memberships=%d merged_duplicates=%d",
			logins.Normalized, logins.Merged)
	}

	owners := defaultProjectOwners()

	report, err := app.Models.MigrateOrphansToDefaultProject(ctx, owners)
	if err != nil {
		log.Printf("Migration: FAILED — data without a project stays invisible until this succeeds: %v", err)
	}
	if report != nil {
		log.Printf("Migration: adopted pre-multi-tenancy data into project %q project_id=%s logs=%d api_keys=%d settings=%d owners=%d",
			data.DefaultProjectName, report.ProjectID, report.Logs, report.APIKeys, report.Settings, report.Owners)
	}

	// Runs whatever the orphan count: it is what finishes an owner step that
	// failed above or on an earlier start, and what picks up owners configured
	// after the data moved.
	repair, err := app.Models.EnsureDefaultProjectOwners(ctx, owners)
	if err != nil {
		log.Printf("Migration: FAILED to give project %q an owner, will retry on the next start: %v", data.DefaultProjectName, err)
	}
	if repair == nil {
		return
	}

	if repair.Owners > 0 {
		log.Printf("Migration: project %q had no owner; added owners=%d project_id=%s",
			data.DefaultProjectName, repair.Owners, repair.ProjectID)
		return
	}

	if len(owners) == 0 {
		log.Printf("Migration: WARNING — project %q has no owner, so nobody can see its data. Set LOGWOLF_ALLOWED_GITHUB_USERS or LOGWOLF_DEFAULT_PROJECT_OWNERS on Logger and restart it to add owners",
			data.DefaultProjectName)
	}
}
