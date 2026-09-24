package main

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"logwolf-toolbox/data"
)

func fastStartupRetries(t *testing.T) {
	t.Helper()
	base, max := startupRetryBase, startupRetryMax
	startupRetryBase, startupRetryMax = time.Millisecond, 4*time.Millisecond
	t.Cleanup(func() { startupRetryBase, startupRetryMax = base, max })
}

// scriptedTasks answers each startup pass with the next result, then succeeds.
type scriptedTasks struct {
	mu      sync.Mutex
	results []taskResult
	passes  int
}

type taskResult struct {
	converted bool
	err       error
}

func (s *scriptedTasks) run() (bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.passes++
	if s.passes <= len(s.results) {
		r := s.results[s.passes-1]
		return r.converted, r.err
	}
	return true, nil
}

func waitReady(t *testing.T, state *startupState) data.LoggerStatus {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if st := state.status(); st.Ready {
			return st
		}
		time.Sleep(time.Millisecond)
	}
	t.Fatalf("startup never became ready: %+v", state.status())
	return data.LoggerStatus{}
}

func TestRunStartup_FirstPassSucceeds(t *testing.T) {
	state := &startupState{}
	tasks := &scriptedTasks{}
	var cleanups atomic.Int32

	runStartup(context.Background(), state, tasks.run, func() { cleanups.Add(1) })

	st := state.status()
	if !st.Ready || !st.RetentionCleanup || st.StartupAttempts != 1 || st.StartupError != "" {
		t.Errorf("status = %+v, want ready after one pass", st)
	}
	if n := cleanups.Load(); n != 1 {
		t.Errorf("cleanup started %d times, want 1", n)
	}
}

// TestRunStartup_RetriesUntilAPassSucceeds is the bug: a failed pass used to be
// logged with "will retry on the next start", and nothing ever retried it.
func TestRunStartup_RetriesUntilAPassSucceeds(t *testing.T) {
	fastStartupRetries(t)
	state := &startupState{}
	tasks := &scriptedTasks{results: []taskResult{
		{converted: false, err: errors.New("convert project_id: boom")},
		{converted: true, err: errors.New("adopt pre-multi-tenancy data: boom")},
	}}
	var cleanups atomic.Int32

	runStartup(context.Background(), state, tasks.run, func() { cleanups.Add(1) })

	// The first pass has failed by the time runStartup returns, before serving.
	if st := state.status(); st.Ready || st.RetentionCleanup || st.StartupError == "" {
		t.Errorf("after a failed first pass, status = %+v, want degraded with the error", st)
	}

	st := waitReady(t, state)
	if st.StartupAttempts != 3 || st.StartupError != "" || !st.RetentionCleanup {
		t.Errorf("status = %+v, want ready after 3 passes", st)
	}
	// Converted on the second pass; the cleanup starts once, not per pass.
	if n := cleanups.Load(); n != 1 {
		t.Errorf("cleanup started %d times, want 1", n)
	}
}

func TestRunStartup_StopsRetryingOnShutdown(t *testing.T) {
	fastStartupRetries(t)
	ctx, cancel := context.WithCancel(context.Background())
	state := &startupState{}
	boom := errors.New("boom")
	tasks := &scriptedTasks{results: make([]taskResult, 1_000_000)}
	for i := range tasks.results {
		tasks.results[i].err = boom
	}

	runStartup(ctx, state, tasks.run, func() {})
	time.Sleep(20 * time.Millisecond)
	cancel()
	time.Sleep(20 * time.Millisecond)

	before := state.status().StartupAttempts
	time.Sleep(50 * time.Millisecond)
	if after := state.status().StartupAttempts; after != before {
		t.Errorf("passes went from %d to %d after shutdown, want no more", before, after)
	}
	if before < 2 {
		t.Errorf("only %d pass(es) before shutdown; a failed pass should be retried", before)
	}
}

func TestHealth(t *testing.T) {
	state := &startupState{}
	app := &Config{startup: state}

	get := func() (int, data.LoggerStatus) {
		w := httptest.NewRecorder()
		app.routes().ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/health", nil))
		var st data.LoggerStatus
		if err := json.Unmarshal(w.Body.Bytes(), &st); err != nil {
			t.Fatalf("decode /health: %v (%s)", err, w.Body.String())
		}
		return w.Code, st
	}

	state.record(false, errors.New("projects indexes: boom"))
	if code, st := get(); code != http.StatusServiceUnavailable || st.Ready || st.StartupError != "projects indexes: boom" {
		t.Errorf("degraded /health = %d %+v, want 503 with the error", code, st)
	}

	state.record(true, nil)
	if code, st := get(); code != http.StatusOK || !st.Ready || st.StartupError != "" || st.StartupAttempts != 2 {
		t.Errorf("ready /health = %d %+v, want 200", code, st)
	}
}

func TestStatusRPC(t *testing.T) {
	var st data.LoggerStatus
	if err := (&RPCServer{}).Status("", &st); err != nil || !st.Ready {
		t.Errorf("Status without startup state = %+v, %v; want ready", st, err)
	}

	state := &startupState{}
	state.record(true, errors.New("boom"))
	if err := (&RPCServer{startup: state}).Status("", &st); err != nil || st.Ready || st.StartupError != "boom" {
		t.Errorf("Status while degraded = %+v, %v; want not ready with the error", st, err)
	}
}
