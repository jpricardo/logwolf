package main

import (
	"errors"
	"testing"

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
