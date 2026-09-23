package main

import (
	"errors"
	"testing"
	"time"
)

// fakeLookup answers from a fixed set of project ids and counts its calls.
type fakeLookup struct {
	projects map[string]bool
	err      error
	calls    int
}

func (f *fakeLookup) lookup(id string) (bool, error) {
	f.calls++
	if f.err != nil {
		return false, f.err
	}
	return f.projects[id], nil
}

func newTestCache(clock *time.Time) *projectCache {
	c := newProjectCache(time.Minute)
	c.now = func() time.Time { return *clock }
	return c
}

func TestProjectCache_RemembersExistingProject(t *testing.T) {
	clock := time.Now()
	c := newTestCache(&clock)
	f := &fakeLookup{projects: map[string]bool{"p1": true}}

	for i := 0; i < 3; i++ {
		found, err := c.exists("p1", f.lookup)
		if err != nil || !found {
			t.Fatalf("exists(p1) = %v, %v; want true, nil", found, err)
		}
	}
	if f.calls != 1 {
		t.Errorf("lookup called %d times, want 1", f.calls)
	}
}

func TestProjectCache_DoesNotRememberMissingProject(t *testing.T) {
	clock := time.Now()
	c := newTestCache(&clock)
	f := &fakeLookup{projects: map[string]bool{}}

	for i := 0; i < 2; i++ {
		if found, _ := c.exists("gone", f.lookup); found {
			t.Fatal("exists(gone) = true, want false")
		}
	}
	if f.calls != 2 {
		t.Errorf("lookup called %d times, want 2 — a missing project must be checked every time", f.calls)
	}

	// Created since: seen at once, not after a negative entry expires.
	f.projects["gone"] = true
	if found, _ := c.exists("gone", f.lookup); !found {
		t.Error("a project created after a miss should be found on the next call")
	}
}

func TestProjectCache_ExpiresAfterTTL(t *testing.T) {
	clock := time.Now()
	c := newTestCache(&clock)
	f := &fakeLookup{projects: map[string]bool{"p1": true}}

	c.exists("p1", f.lookup)

	// Deleted by another Logger instance, which this cache never hears about.
	delete(f.projects, "p1")

	clock = clock.Add(30 * time.Second)
	if found, _ := c.exists("p1", f.lookup); !found {
		t.Error("within the TTL the cached answer should stand")
	}

	clock = clock.Add(time.Minute)
	if found, _ := c.exists("p1", f.lookup); found {
		t.Error("after the TTL the project should be looked up again, and found missing")
	}
	if f.calls != 2 {
		t.Errorf("lookup called %d times, want 2", f.calls)
	}
}

func TestProjectCache_Forget(t *testing.T) {
	clock := time.Now()
	c := newTestCache(&clock)
	f := &fakeLookup{projects: map[string]bool{"p1": true}}

	c.exists("p1", f.lookup)
	delete(f.projects, "p1")
	c.forget("p1")

	if found, _ := c.exists("p1", f.lookup); found {
		t.Error("a forgotten project should be looked up again, and found missing")
	}
}

func TestProjectCache_LookupError(t *testing.T) {
	clock := time.Now()
	c := newTestCache(&clock)
	f := &fakeLookup{err: errors.New("mongo down")}

	found, err := c.exists("p1", f.lookup)
	if err == nil {
		t.Fatal("a lookup error should be returned")
	}
	if found {
		t.Error("found should be false on error")
	}

	f.err = nil
	f.projects = map[string]bool{"p1": true}
	if found, _ := c.exists("p1", f.lookup); !found {
		t.Error("an error must not be remembered")
	}
}

func TestProjectCache_Nil(t *testing.T) {
	var c *projectCache
	f := &fakeLookup{projects: map[string]bool{"p1": true}}

	c.exists("p1", f.lookup)
	c.exists("p1", f.lookup)
	c.forget("p1")

	if f.calls != 2 {
		t.Errorf("a nil cache should ask every time: lookup called %d times, want 2", f.calls)
	}
}
