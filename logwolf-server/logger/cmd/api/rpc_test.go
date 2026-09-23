package main

import (
	"errors"
	"testing"
	"time"

	"logwolf-toolbox/data"
)

// TestCheckMembership_InvalidProjectID verifies that CheckMembership returns
// an error (not a silent false) when the project ID is not a valid ObjectID hex.
func TestCheckMembership_InvalidProjectID(t *testing.T) {
	srv := &RPCServer{} // zero-value models — no DB connection needed for this path

	args := &data.RPCCheckMembershipArgs{
		ProjectID:   "not-a-valid-object-id",
		GithubLogin: "jpricardo",
	}
	var reply bool
	err := srv.CheckMembership(args, &reply)
	if err == nil {
		t.Error("CheckMembership should return an error for an invalid project ID hex")
	}
	if reply {
		t.Error("reply should remain false on error")
	}
}

// TestCheckMembership_EmptyProjectID verifies that an empty project ID is rejected.
func TestCheckMembership_EmptyProjectID(t *testing.T) {
	srv := &RPCServer{}

	args := &data.RPCCheckMembershipArgs{
		ProjectID:   "",
		GithubLogin: "jpricardo",
	}
	var reply bool
	err := srv.CheckMembership(args, &reply)
	if err == nil {
		t.Error("CheckMembership should return an error for an empty project ID")
	}
}

// TestLogInfo_UnknownProject verifies that an event whose project id names no
// project is refused rather than inserted. An id that is not an ObjectID is
// answered without a database, so the zero-value server is enough here.
func TestLogInfo_UnknownProject(t *testing.T) {
	srv := &RPCServer{}

	var reply string
	err := srv.LogInfo(data.RPCLogPayload{ProjectID: "not-a-project", Name: "stray"}, &reply)
	if !errors.Is(err, errUnknownProject) {
		t.Errorf("LogInfo for an unknown project: want errUnknownProject, got %v", err)
	}
	if reply != "" {
		t.Errorf("reply should stay empty when the event is dropped, got %q", reply)
	}
}

// TestRequestPurge_Queues checks that a deleted project reaches the cleanup
// loop's queue.
func TestRequestPurge_Queues(t *testing.T) {
	purges := make(chan string, 1)
	srv := &RPCServer{purges: purges}

	srv.requestPurge("doomed")

	select {
	case got := <-purges:
		if got != "doomed" {
			t.Errorf("queued %q, want %q", got, "doomed")
		}
	default:
		t.Fatal("requestPurge queued nothing")
	}
}

// TestRequestPurge_NeverBlocks checks that DeleteProject cannot hang on the
// purge queue: a full queue drops the request (the orphan sweep deletes those
// logs later), and a server with no queue at all asks no one.
func TestRequestPurge_NeverBlocks(t *testing.T) {
	full := make(chan string, 1)
	full <- "earlier"

	for name, srv := range map[string]*RPCServer{
		"full queue": {purges: full},
		"no queue":   {},
	} {
		done := make(chan struct{})
		go func() {
			srv.requestPurge("doomed")
			close(done)
		}()

		select {
		case <-done:
		case <-time.After(time.Second):
			t.Fatalf("%s: requestPurge blocked", name)
		}
	}

	if got := <-full; got != "earlier" {
		t.Errorf("full queue: now holds %q, want the earlier %q", got, "earlier")
	}
}
