package main

import (
	"sync"
	"time"
)

// projectCacheTTL is how long a project stays known to exist without being
// looked up again. DeleteProject evicts its own project straight away, so this
// only bounds how long another Logger instance keeps accepting events for it;
// the orphan sweep in the cleanup loop removes whatever gets through.
const projectCacheTTL = time.Minute

// projectCache remembers which project ids were recently seen to exist, so
// LogInfo does not query the projects collection for every event. Only a
// positive answer is kept: an id that names no project is looked up each time,
// which is rare and keeps a project visible the moment it is created.
//
// A nil *projectCache caches nothing and always asks lookup.
type projectCache struct {
	ttl time.Duration
	now func() time.Time

	mu      sync.Mutex
	expires map[string]time.Time
}

func newProjectCache(ttl time.Duration) *projectCache {
	return &projectCache{ttl: ttl, now: time.Now, expires: map[string]time.Time{}}
}

// exists reports whether projectID names a project, asking lookup when the
// cache has no fresh answer.
func (c *projectCache) exists(projectID string, lookup func(string) (bool, error)) (bool, error) {
	if c == nil {
		return lookup(projectID)
	}

	c.mu.Lock()
	until, ok := c.expires[projectID]
	c.mu.Unlock()
	if ok && c.now().Before(until) {
		return true, nil
	}

	found, err := lookup(projectID)
	if err != nil {
		return false, err
	}

	c.mu.Lock()
	if found {
		c.expires[projectID] = c.now().Add(c.ttl)
	} else {
		delete(c.expires, projectID)
	}
	c.mu.Unlock()
	return found, nil
}

// forget drops projectID, so the next event for it is checked against the
// database again.
func (c *projectCache) forget(projectID string) {
	if c == nil {
		return
	}

	c.mu.Lock()
	delete(c.expires, projectID)
	c.mu.Unlock()
}
