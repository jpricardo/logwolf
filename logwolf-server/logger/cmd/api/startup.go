package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"logwolf-toolbox/data"
	"sync"
	"time"
)

// Failed startup passes are retried from startupRetryBase, doubling up to
// startupRetryMax. Variables so tests can shorten them.
var (
	startupRetryBase = 30 * time.Second
	startupRetryMax  = 10 * time.Minute
)

// startupState is how far the logger's startup tasks have got. The Status RPC
// and /health report it.
type startupState struct {
	mu        sync.Mutex
	ready     bool
	converted bool
	attempts  int
	lastErr   error
}

func (s *startupState) record(converted bool, err error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.attempts++
	s.converted = s.converted || converted
	s.ready = err == nil
	s.lastErr = err
}

func (s *startupState) status() data.LoggerStatus {
	s.mu.Lock()
	defer s.mu.Unlock()

	st := data.LoggerStatus{
		Ready:            s.ready,
		RetentionCleanup: s.converted,
		StartupAttempts:  s.attempts,
	}
	if s.lastErr != nil {
		st.StartupError = s.lastErr.Error()
	}
	return st
}

// startupTasks is one startup pass. It reports whether every project_id is an
// ObjectID, and joins every failure.
type startupTasks func() (converted bool, err error)

// runStartupTasks is the real startupTasks: the indexes, then the startup
// migration.
func (app *Config) runStartupTasks() (bool, error) {
	indexErr := app.ensureIndexes()
	converted, migrateErr := app.runStartupMigration()
	return converted, errors.Join(indexErr, migrateErr)
}

// ensureIndexes creates every index the logger relies on. Some are unique
// constraints (the Default flag, memberships, settings), so a failure fails the
// pass instead of being a warning.
func (app *Config) ensureIndexes() error {
	var errs []error
	for _, ensure := range []struct {
		name string
		fn   func() error
	}{
		{"settings", app.Models.Settings.EnsureSettingsIndex},
		{"logs", app.Models.EnsureLogsIndexes},
		{"projects", app.Models.EnsureProjectIndexes},
		{"api keys", app.Models.EnsureAPIKeyIndexes},
	} {
		if err := ensure.fn(); err != nil {
			log.Printf("Startup: FAILED to ensure %s indexes, will retry: %v", ensure.name, err)
			errs = append(errs, fmt.Errorf("%s indexes: %w", ensure.name, err))
		}
	}
	return errors.Join(errs...)
}

// runStartup runs the first startup pass before the logger serves, so on the
// happy path no caller ever reads a half-migrated database.
//
// A failed pass does not stop the logger: every task is idempotent, so it is
// retried in the background, with back-off, until one succeeds. The logger
// serves meanwhile, in a degraded state that the Status RPC and /health report.
// Nothing used to retry: the logger kept serving, the log promised a retry "on
// the next start", and without a crash no next start came.
//
// onConverted runs once, as soon as a pass has converted every project_id; it
// starts the retention cleanup.
func runStartup(ctx context.Context, state *startupState, tasks startupTasks, onConverted func()) {
	var once sync.Once
	pass := func() bool {
		converted, err := tasks()
		state.record(converted, err)
		if converted {
			once.Do(onConverted)
		}
		if err != nil {
			log.Printf("Startup: pass %d FAILED; serving in a degraded state until a retry succeeds: %v",
				state.status().StartupAttempts, err)
			return false
		}
		return true
	}

	if pass() {
		return
	}
	if !state.status().RetentionCleanup {
		log.Println("Retention cleanup: DISABLED until every project_id is converted; see the migration error above")
	}

	go func() {
		delay := startupRetryBase
		for {
			timer := time.NewTimer(delay)
			select {
			case <-ctx.Done():
				timer.Stop()
				return
			case <-timer.C:
			}

			if pass() {
				log.Printf("Startup: pass %d succeeded; no longer degraded", state.status().StartupAttempts)
				return
			}
			delay = min(delay*2, startupRetryMax)
		}
	}()
}
