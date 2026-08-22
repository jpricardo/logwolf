//go:build integration

package integration

import (
	"testing"
)

// TestProjectIsolation_GetLogs verifies that logs written under project A are
// not visible when querying as project B, and vice versa.
func TestProjectIsolation_GetLogs(t *testing.T) {
	stack := sharedStack(t)

	keyA := seedAPIKey(t, stack.mongoURI, "project-alpha", "lw_alphakey0000000001")
	keyB := seedAPIKey(t, stack.mongoURI, "project-beta0", "lw_betakey00000000001")

	postLog(t, stack.brokerURL, keyA, "alpha-event")
	postLog(t, stack.brokerURL, keyB, "beta-event")

	waitForLog(t, stack.mongoURI, "alpha-event")
	waitForLog(t, stack.mongoURI, "beta-event")

	// Project A should see its own log and not project B's.
	logsA := getLogs(t, stack.brokerURL, keyA)
	t.Logf("project-alpha logs: %v", logsA)

	if !containsName(logsA, "alpha-event") {
		t.Error("project-alpha: expected to see alpha-event, but did not")
	}
	if containsName(logsA, "beta-event") {
		t.Error("project-alpha: must not see beta-event from project-beta")
	}

	// Project B should see its own log and not project A's.
	logsB := getLogs(t, stack.brokerURL, keyB)
	t.Logf("project-beta logs: %v", logsB)

	if !containsName(logsB, "beta-event") {
		t.Error("project-beta: expected to see beta-event, but did not")
	}
	if containsName(logsB, "alpha-event") {
		t.Error("project-beta: must not see alpha-event from project-alpha")
	}
}

// TestProjectIsolation_DeleteLog verifies that a DELETE /logs request scoped to
// project A does not remove logs belonging to project B.
func TestProjectIsolation_DeleteLog(t *testing.T) {
	stack := sharedStack(t)

	keyA := seedAPIKey(t, stack.mongoURI, "del-alpha", "lw_delalpha0000000001")
	keyB := seedAPIKey(t, stack.mongoURI, "del-beta00", "lw_delbeta00000000001")

	postLog(t, stack.brokerURL, keyA, "del-alpha-event")
	postLog(t, stack.brokerURL, keyB, "del-beta-event")

	waitForLog(t, stack.mongoURI, "del-alpha-event")
	waitForLog(t, stack.mongoURI, "del-beta-event")

	// Fetch project A's log ID so we can target it for deletion.
	logIDA := fetchLogID(t, stack.mongoURI, "del-alpha-event")

	// Delete the log as project A.
	deleteLog(t, stack.brokerURL, keyA, logIDA)

	// Project A's log must be gone.
	logsA := getLogs(t, stack.brokerURL, keyA)
	if containsName(logsA, "del-alpha-event") {
		t.Error("del-alpha-event should have been deleted but is still present")
	}

	// Project B's log must be unaffected.
	logsB := getLogs(t, stack.brokerURL, keyB)
	if !containsName(logsB, "del-beta-event") {
		t.Error("del-beta-event was deleted but should not have been")
	}
}

// TestProjectIsolation_CrossDelete verifies that project A cannot delete a log
// that belongs to project B, even when supplying project B's log ID directly.
// The broker sets project_id from the authenticated key, so the DB filter
// (id=B_id AND project_id=A) matches nothing and returns 0 deleted.
func TestProjectIsolation_CrossDelete(t *testing.T) {
	stack := sharedStack(t)

	keyA := seedAPIKey(t, stack.mongoURI, "xdel-alpha", "lw_xdelattempt000001")
	keyB := seedAPIKey(t, stack.mongoURI, "xdel-beta0", "lw_xdelvictim000001")

	postLog(t, stack.brokerURL, keyB, "xdel-victim-event")
	waitForLog(t, stack.mongoURI, "xdel-victim-event")

	logIDB := fetchLogID(t, stack.mongoURI, "xdel-victim-event")

	// Project A attempts to delete project B's log ID. The broker will scope
	// the filter to project A, so the document is never matched.
	n := deleteLogCount(t, stack.brokerURL, keyA, logIDB)
	if n != 0 {
		t.Errorf("cross-project delete: expected 0 deleted, got %d", n)
	}

	// Project B's log must still be present.
	logsB := getLogs(t, stack.brokerURL, keyB)
	if !containsName(logsB, "xdel-victim-event") {
		t.Error("cross-project delete: victim log was removed but should not have been")
	}
}

// TestProjectIsolation_CrossReadByID verifies that a key for project A cannot
// read project B's entry even when it knows the id: GET /logs is scoped to the
// key's project, so B's entry is simply not in the result.
func TestProjectIsolation_CrossReadByID(t *testing.T) {
	stack := sharedStack(t)

	keyA := seedAPIKey(t, stack.mongoURI, "xread-alpha", "lw_xreadalpha000001")
	keyB := seedAPIKey(t, stack.mongoURI, "xread-beta0", "lw_xreadbeta0000001")

	postLog(t, stack.brokerURL, keyB, "xread-private-event")
	waitForLog(t, stack.mongoURI, "xread-private-event")

	if names := getLogs(t, stack.brokerURL, keyA); containsName(names, "xread-private-event") {
		t.Errorf("project xread-alpha can read xread-beta0's entry: %v", names)
	}
}
