package main

import (
	"context"
	"logwolf-toolbox/data"
	"sync"
	"testing"
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

// fakeRetentionStore serves a fixed project list, all with 30-day retention.
// The delete of a slow project runs until its context is done, as a huge
// expired set would; every other delete succeeds, unless its context is already
// done by the time it starts.
type fakeRetentionStore struct {
	projects []data.Project
	slow     map[primitive.ObjectID]bool

	mu       sync.Mutex
	deleted  map[primitive.ObjectID]bool
	attempts map[primitive.ObjectID]int
	noLimit  []primitive.ObjectID
}

func newFakeRetentionStore(n int) *fakeRetentionStore {
	f := &fakeRetentionStore{
		slow:     map[primitive.ObjectID]bool{},
		deleted:  map[primitive.ObjectID]bool{},
		attempts: map[primitive.ObjectID]int{},
	}
	for i := 0; i < n; i++ {
		f.projects = append(f.projects, data.Project{ID: primitive.NewObjectID()})
	}
	return f
}

func (f *fakeRetentionStore) id(i int) primitive.ObjectID { return f.projects[i].ID }

func (f *fakeRetentionStore) GetAllProjects(ctx context.Context) ([]data.Project, error) {
	return f.projects, nil
}

func (f *fakeRetentionStore) GetRetentionDays(ctx context.Context, projectID primitive.ObjectID) (int, error) {
	if _, ok := ctx.Deadline(); !ok {
		f.mu.Lock()
		f.noLimit = append(f.noLimit, projectID)
		f.mu.Unlock()
	}
	if err := ctx.Err(); err != nil {
		return 0, err
	}
	return 30, nil
}

func (f *fakeRetentionStore) DeleteExpiredLogs(ctx context.Context, projectID primitive.ObjectID, before time.Time) (int64, error) {
	f.mu.Lock()
	f.attempts[projectID]++
	f.mu.Unlock()

	if f.slow[projectID] {
		<-ctx.Done()
		return 10000, ctx.Err()
	}
	if err := ctx.Err(); err != nil {
		return 0, err
	}

	f.mu.Lock()
	f.deleted[projectID] = true
	f.mu.Unlock()
	return 1, nil
}

// TestExpireLogs_SlowProjectDoesNotStarveTheRest runs a pass in which the first
// two projects each outlast their timeout. The projects after them must still be
// cleaned: the slow ones may only use up their own time.
func TestExpireLogs_SlowProjectDoesNotStarveTheRest(t *testing.T) {
	f := newFakeRetentionStore(4)
	f.slow[f.id(0)] = true
	f.slow[f.id(1)] = true

	const projectTimeout = 50 * time.Millisecond
	start := time.Now()
	expireLogs(context.Background(), f, projectTimeout)
	elapsed := time.Since(start)

	for i := 0; i < 2; i++ {
		if f.attempts[f.id(i)] != 1 {
			t.Errorf("slow project %d: %d delete attempts, want 1", i, f.attempts[f.id(i)])
		}
	}
	for i := 2; i < 4; i++ {
		if !f.deleted[f.id(i)] {
			t.Errorf("project %d after the slow ones was not cleaned", i)
		}
	}
	// Each slow project ran out its own timeout rather than sharing one.
	if elapsed < 2*projectTimeout {
		t.Errorf("pass took %s, want at least %s: the slow projects did not each get their own timeout", elapsed, 2*projectTimeout)
	}
	if len(f.noLimit) != 0 {
		t.Errorf("GetRetentionDays called without a deadline for %v", f.noLimit)
	}
}

// TestExpireLogs_StopsOnShutdown: once the loop's context is cancelled, the pass
// must not start on another project.
func TestExpireLogs_StopsOnShutdown(t *testing.T) {
	f := newFakeRetentionStore(3)
	f.slow[f.id(0)] = true

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		expireLogs(ctx, f, time.Minute)
		close(done)
	}()

	time.Sleep(20 * time.Millisecond)
	cancel()

	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("pass still running 5s after shutdown")
	}

	f.mu.Lock()
	defer f.mu.Unlock()
	for i := 1; i < 3; i++ {
		if f.attempts[f.id(i)] != 0 {
			t.Errorf("project %d: delete attempted after shutdown", i)
		}
	}
}
