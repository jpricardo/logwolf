package main

import (
	"context"
	"errors"
	"fmt"
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

// runStartupMigration converts project ids to ObjectIDs, normalizes member
// logins and adopts any pre-multi-tenancy data into the Default project, then
// makes sure that project has an owner. It is silent when there is nothing to
// do.
//
// A failed step is logged and the rest still run where they can; the error
// joins every failure. Every step is idempotent, so runStartup runs the whole
// thing again until it succeeds. converted reports whether every project_id is
// an ObjectID now, which the retention cleanup waits for.
func (app *Config) runStartupMigration() (converted bool, err error) {
	ctx, cancel := context.WithTimeout(context.Background(), migrationTimeout)
	defer cancel()

	var errs []error
	fail := func(step string, err error) {
		errs = append(errs, fmt.Errorf("%s: %w", step, err))
	}

	if dropped, err := app.Models.DropLegacyTTLIndex(ctx); err != nil {
		log.Printf("Migration: could not drop the legacy TTL index: %v", err)
		fail("drop legacy TTL index", err)
	} else if dropped {
		log.Println("Migration: dropped the legacy global TTL index on logs — retention is per project now")
	}

	ids, err := app.Models.ConvertProjectIDs(ctx)
	if err != nil {
		log.Printf("Migration: FAILED to convert project_id to ObjectID, will retry: %v", err)
		fail("convert project_id", err)
	}
	if ids.Total() > 0 {
		log.Printf("Migration: converted project_id to ObjectID logs=%d api_keys=%d settings=%d",
			ids.Logs, ids.APIKeys, ids.Settings)
	}
	converted = err == nil

	// Before the Default project steps below: they find it by this flag.
	retired, err := app.Models.MarkDefaultProject(ctx)
	if err != nil {
		log.Printf("Migration: FAILED to mark project %q, will retry: %v", data.DefaultProjectName, err)
		fail("mark Default project", err)
		return converted, errors.Join(errs...)
	}
	if retired {
		log.Println("Migration: dropped the unique index on project slugs — slugs are labels now, and the Default project is marked as such")
	}

	// Before the owner steps below: they look owners up by normalized login, and
	// would otherwise add a second membership next to one stored in another casing.
	logins, err := app.Models.NormalizeMemberLogins(ctx)
	if err != nil {
		log.Printf("Migration: FAILED to normalize member logins, will retry: %v", err)
		fail("normalize member logins", err)
	}
	if logins.Normalized > 0 {
		log.Printf("Migration: normalized member logins to lowercase memberships=%d merged_duplicates=%d",
			logins.Normalized, logins.Merged)
	}

	owners := defaultProjectOwners()

	report, err := app.Models.MigrateOrphansToDefaultProject(ctx, owners)
	if err != nil {
		log.Printf("Migration: FAILED — data without a project stays invisible until this succeeds: %v", err)
		fail("adopt pre-multi-tenancy data", err)
	}
	if report != nil {
		log.Printf("Migration: adopted pre-multi-tenancy data into project %q project_id=%s logs=%d api_keys=%d settings=%d owners=%d",
			data.DefaultProjectName, report.ProjectID, report.Logs, report.APIKeys, report.Settings, report.Owners)
	}

	// Runs whatever the orphan count: it is what finishes an owner step that
	// failed above or on an earlier pass, and what picks up owners configured
	// after the data moved.
	repair, err := app.Models.EnsureDefaultProjectOwners(ctx, owners)
	if err != nil {
		log.Printf("Migration: FAILED to give project %q an owner, will retry: %v", data.DefaultProjectName, err)
		fail("give Default project an owner", err)
	}
	if repair == nil {
		return converted, errors.Join(errs...)
	}

	if repair.Owners > 0 {
		log.Printf("Migration: project %q had no owner; added owners=%d project_id=%s",
			data.DefaultProjectName, repair.Owners, repair.ProjectID)
		return converted, errors.Join(errs...)
	}

	// Not a failure: nothing here can fix it, only configuration can.
	if len(owners) == 0 {
		log.Printf("Migration: WARNING — project %q has no owner, so nobody can see its data. Set LOGWOLF_ALLOWED_GITHUB_USERS or LOGWOLF_DEFAULT_PROJECT_OWNERS on Logger and restart it to add owners",
			data.DefaultProjectName)
	}

	return converted, errors.Join(errs...)
}
