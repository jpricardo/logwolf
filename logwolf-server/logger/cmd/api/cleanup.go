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

	app.cleanupExpiredLogs(ctx)

	for {
		select {
		case <-ctx.Done():
			log.Println("Retention cleanup: shutting down")
			return
		case <-ticker.C:
			app.cleanupExpiredLogs(ctx)
		}
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
