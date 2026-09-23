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

// The API key methods refuse input that could never name a key before they
// touch the database, so the zero-value server answers these.

func TestValidateAPIKey_MalformedKeyIsInvalid(t *testing.T) {
	srv := &RPCServer{}

	var reply data.RPCValidateAPIKeyReply
	if err := srv.ValidateAPIKey(&data.RPCValidateAPIKeyArgs{Plaintext: "lw_short"}, &reply); err != nil {
		t.Fatalf("ValidateAPIKey: %v", err)
	}
	if reply.Valid {
		t.Error("a malformed key was accepted")
	}
}

func TestCreateAPIKey_UnknownScope(t *testing.T) {
	srv := &RPCServer{}

	var reply data.RPCCreateAPIKeyReply
	err := srv.CreateAPIKey(&data.RPCCreateAPIKeyArgs{ProjectID: "p", Scopes: []string{"admin"}}, &reply)
	if !errors.Is(err, data.ErrInvalidScope) {
		t.Errorf("CreateAPIKey with an unknown scope: want ErrInvalidScope, got %v", err)
	}
	if reply.Plaintext != "" {
		t.Error("a key was handed out despite the error")
	}
}

func TestAPIKeyMethods_MalformedID(t *testing.T) {
	srv := &RPCServer{}

	var key data.APIKey
	if err := srv.GetAPIKey(&data.RPCAPIKeyIDArgs{ID: "not-an-id"}, &key); err == nil {
		t.Error("GetAPIKey accepted a malformed id")
	}
	var reply string
	if err := srv.RevokeAPIKey(&data.RPCRevokeAPIKeyArgs{ProjectID: "p", ID: "not-an-id"}, &reply); err == nil {
		t.Error("RevokeAPIKey accepted a malformed id")
	}
}

func TestWithoutHash(t *testing.T) {
	key := data.APIKey{Prefix: "lw_abcdefg", Hash: "$2a$10$secret"}
	if got := withoutHash(key); got.Hash != "" || got.Prefix != key.Prefix {
		t.Errorf("withoutHash = %+v, want the key minus its hash", got)
	}
}
