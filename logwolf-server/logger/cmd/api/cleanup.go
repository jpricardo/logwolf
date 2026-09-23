package main

import (
	"context"
	"log"
	"logwolf-toolbox/data"
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

const (
	// projectListTimeout bounds reading the project list at the start of a pass.
	projectListTimeout = 30 * time.Second

	// projectCleanupTimeout bounds the retention cleanup of one project. Each
	// project gets its own, so a project with a huge expired set can only use up
	// its own time, never that of the projects after it. What it has not deleted
	// by then is left for the next pass; DeleteExpiredLogs deletes in batches, so
	// the progress is kept.
	projectCleanupTimeout = 2 * time.Minute
)

// retentionStore is what the retention cleanup needs from the database.
type retentionStore interface {
	GetAllProjects(ctx context.Context) ([]data.Project, error)
	GetRetentionDays(ctx context.Context, projectID string) (int, error)
	DeleteExpiredLogs(ctx context.Context, projectID string, before time.Time) (int64, error)
}

// modelsRetentionStore is the retentionStore of a running Logger. data.Models
// keeps the retention lookup on its Settings field; this puts it alongside the
// rest.
type modelsRetentionStore struct {
	*data.Models
}

func (s modelsRetentionStore) GetRetentionDays(ctx context.Context, projectID string) (int, error) {
	return s.Settings.GetRetentionDays(ctx, projectID)
}

func (app *Config) cleanupExpiredLogs(ctx context.Context) {
	expireLogs(ctx, modelsRetentionStore{&app.Models}, projectCleanupTimeout)
}

// expireLogs deletes the expired logs of every project, giving each one
// projectTimeout of its own. ctx stops the pass on shutdown.
func expireLogs(ctx context.Context, store retentionStore, projectTimeout time.Duration) {
	listCtx, cancel := context.WithTimeout(ctx, projectListTimeout)
	projects, err := store.GetAllProjects(listCtx)
	cancel()
	if err != nil {
		log.Printf("Retention cleanup: error fetching projects: %v", err)
		return
	}

	for _, p := range projects {
		if ctx.Err() != nil {
			return
		}
		expireProjectLogs(ctx, store, p.ID.Hex(), projectTimeout)
	}
}

func expireProjectLogs(ctx context.Context, store retentionStore, projectID string, timeout time.Duration) {
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	days, err := store.GetRetentionDays(ctx, projectID)
	if err != nil {
		log.Printf("Retention cleanup: project %s: error reading retention: %v", projectID, err)
		return
	}

	if days == 0 {
		return
	}

	threshold := time.Now().Add(-time.Duration(days) * 24 * time.Hour)
	deleted, err := store.DeleteExpiredLogs(ctx, projectID, threshold)
	if err != nil {
		log.Printf("Retention cleanup: project %s: error after deleting %d expired logs, the rest are left for the next pass: %v", projectID, deleted, err)
		return
	}

	if deleted > 0 {
		log.Printf("Retention cleanup: project %s: deleted %d expired logs (retention=%dd)", projectID, deleted, days)
	}
}
