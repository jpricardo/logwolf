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

// allowedGithubUsers returns the dashboard allowlist. It is the only signal the
// logger has for who should own data that predates projects.
func allowedGithubUsers() []string {
	return data.ParseGithubLogins(os.Getenv("LOGWOLF_ALLOWED_GITHUB_USERS"))
}

// runStartupMigration adopts any pre-multi-tenancy data into the Default project
// before the RPC server starts accepting connections. It is silent when there is
// nothing to migrate.
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

	owners := allowedGithubUsers()

	report, err := app.Models.MigrateOrphansToDefaultProject(ctx, owners)
	if err != nil {
		log.Printf("Migration: FAILED — data without a project stays invisible until this succeeds: %v", err)
	}
	if report == nil {
		return
	}

	log.Printf("Migration: adopted pre-multi-tenancy data into project %q project_id=%s logs=%d api_keys=%d settings=%d owners=%d",
		data.DefaultProjectName, report.ProjectID, report.Logs, report.APIKeys, report.Settings, report.Owners)

	if len(owners) == 0 {
		log.Printf("Migration: WARNING — LOGWOLF_ALLOWED_GITHUB_USERS is empty, so project %q has no owners; nobody can see the migrated data until a member is added",
			data.DefaultProjectName)
	}
}
