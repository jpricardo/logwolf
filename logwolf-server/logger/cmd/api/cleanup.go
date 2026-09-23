package main

import (
	"context"
	"log"
	"os"
	"time"
)

func cleanupInterval() time.Duration {
	if s := os.Getenv("CLEANUP_INTERVAL"); s != "" {
		d, err := time.ParseDuration(s)
		if err != nil {
			log.Printf("Warning: invalid CLEANUP_INTERVAL %q, using default 1h: %v", s, err)
		} else {
			return d
		}
	}
	return time.Hour
}

func (app *Config) runCleanup(ctx context.Context) {
	interval := cleanupInterval()
	log.Printf("Retention cleanup: starting, interval=%s", interval)

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	app.cleanupPass(ctx)

	for {
		select {
		case <-ctx.Done():
			log.Println("Retention cleanup: shutting down")
			return
		case <-ticker.C:
			app.cleanupPass(ctx)
		case projectID := <-app.purges:
			app.purgeProjectLogs(ctx, projectID)
		}
	}
}

// purgeQueueSize is how many deleted projects can wait for their logs to be
// purged. Deletes are rare; one that finds the queue full is left to the orphan
// sweep.
const purgeQueueSize = 64

// purgeProjectLogs deletes the logs of a project DeleteProject has just removed.
// It runs here rather than in the RPC, so the delete returns as soon as the
// project is gone while its logs, however many, are deleted in the background.
// A purge that fails or is cut short by shutdown is not retried: the logs belong
// to no project now, and the orphan sweep in the next pass deletes them.
func (app *Config) purgeProjectLogs(ctx context.Context, projectID string) {
	deleted, err := app.Models.PurgeProjectLogs(ctx, projectID)
	if err != nil {
		log.Printf("Retention cleanup: deleted project %s: error purging logs after %d: %v", projectID, deleted, err)
		return
	}
	log.Printf("Retention cleanup: deleted project %s: purged %d logs", projectID, deleted)
}

func (app *Config) cleanupPass(ctx context.Context) {
	app.cleanupExpiredLogs(ctx)
	app.cleanupOrphanedLogs(ctx)
}

// cleanupOrphanedLogs deletes logs whose project no longer exists. LogInfo
// refuses new ones, but an event can pass that check just before its project
// is deleted, and those logs are invisible to everyone and outside every
// project's retention. It is also what finishes a purgeProjectLogs that failed.
//
// There is no deadline on the pass as a whole: a deleted project can leave
// millions of logs, which take minutes to delete. DeleteOrphanedLogs times out
// each batch on its own, and ctx stops it on shutdown.
func (app *Config) cleanupOrphanedLogs(ctx context.Context) {
	deleted, err := app.Models.DeleteOrphanedLogs(ctx)
	if err != nil {
		log.Printf("Retention cleanup: error deleting logs of deleted projects: %v", err)
	}
	for projectID, n := range deleted {
		log.Printf("Retention cleanup: project %s no longer exists: deleted %d logs", projectID, n)
	}
}

func (app *Config) cleanupExpiredLogs(ctx context.Context) {
	passCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	projects, err := app.Models.GetAllProjects(passCtx)
	if err != nil {
		log.Printf("Retention cleanup: error fetching projects: %v", err)
		return
	}

	for _, p := range projects {
		projectID := p.ID.Hex()

		days, err := app.Models.Settings.GetRetentionDays(projectID)
		if err != nil {
			log.Printf("Retention cleanup: project %s: error reading retention: %v", projectID, err)
			continue
		}

		if days == 0 {
			continue
		}

		threshold := time.Now().Add(-time.Duration(days) * 24 * time.Hour)
		deleted, err := app.Models.DeleteExpiredLogs(passCtx, projectID, threshold)
		if err != nil {
			log.Printf("Retention cleanup: project %s: error deleting: %v", projectID, err)
			continue
		}

		if deleted > 0 {
			log.Printf("Retention cleanup: project %s: deleted %d expired logs (retention=%dd)", projectID, deleted, days)
		}
	}
}
