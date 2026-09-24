package main

import (
	"errors"
	"strings"
	"testing"
	"time"

	"logwolf-toolbox/data"

	"go.mongodb.org/mongo-driver/bson/primitive"
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
	if !errors.Is(err, data.ErrUnknownProject) {
		t.Errorf("LogInfo for an unknown project: want ErrUnknownProject, got %v", err)
	}
	if reply != "" {
		t.Errorf("reply should stay empty when the event is dropped, got %q", reply)
	}
}

// TestRequestPurge_Queues checks that a deleted project reaches the cleanup
// loop's queue.
func TestRequestPurge_Queues(t *testing.T) {
	purges := make(chan primitive.ObjectID, 1)
	srv := &RPCServer{purges: purges}

	doomed := primitive.NewObjectID()
	srv.requestPurge(doomed)

	select {
	case got := <-purges:
		if got != doomed {
			t.Errorf("queued %s, want %s", got.Hex(), doomed.Hex())
		}
	default:
		t.Fatal("requestPurge queued nothing")
	}
}

// TestRequestPurge_NeverBlocks checks that DeleteProject cannot hang on the
// purge queue: a full queue drops the request (the orphan sweep deletes those
// logs later), and a server with no queue at all asks no one.
func TestRequestPurge_NeverBlocks(t *testing.T) {
	earlier := primitive.NewObjectID()
	full := make(chan primitive.ObjectID, 1)
	full <- earlier

	for name, srv := range map[string]*RPCServer{
		"full queue": {purges: full},
		"no queue":   {},
	} {
		done := make(chan struct{})
		go func() {
			srv.requestPurge(primitive.NewObjectID())
			close(done)
		}()

		select {
		case <-done:
		case <-time.After(time.Second):
			t.Fatalf("%s: requestPurge blocked", name)
		}
	}

	if got := <-full; got != earlier {
		t.Errorf("full queue: now holds %s, want the earlier %s", got.Hex(), earlier.Hex())
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
	err := srv.CreateAPIKey(&data.RPCCreateAPIKeyArgs{ProjectID: primitive.NewObjectID().Hex(), Scopes: []string{"admin"}}, &reply)
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
	if err := srv.RevokeAPIKey(&data.RPCRevokeAPIKeyArgs{ProjectID: primitive.NewObjectID().Hex(), ID: "not-an-id"}, &reply); err == nil {
		t.Error("RevokeAPIKey accepted a malformed id")
	}
}

// TestProjectScopedMethods_MalformedProjectID checks that every method taking a
// project id refuses one that is not an ObjectID before touching the database,
// and says so in the words the broker answers with 404.
func TestProjectScopedMethods_MalformedProjectID(t *testing.T) {
	srv := &RPCServer{}
	const bad = "not-a-project"

	for name, call := range map[string]func() error{
		"GetLogs": func() error {
			var reply []data.LogEntry
			return srv.GetLogs(data.QueryParams{ProjectID: bad}, &reply)
		},
		"GetLog": func() error {
			var reply data.LogEntry
			return srv.GetLog(data.RPCLogEntryFilter{ID: primitive.NewObjectID().Hex(), ProjectID: bad}, &reply)
		},
		"DeleteLog": func() error {
			var reply int64
			return srv.DeleteLog(data.RPCLogEntryFilter{ID: primitive.NewObjectID().Hex(), ProjectID: bad}, &reply)
		},
		"GetRetention": func() error {
			var reply int
			return srv.GetRetention(&data.RetentionArgs{ProjectID: bad}, &reply)
		},
		"UpdateRetention": func() error {
			var reply string
			return srv.UpdateRetention(&data.RetentionArgs{ProjectID: bad, Days: 30}, &reply)
		},
		"GetMetrics": func() error {
			var reply data.Metrics
			return srv.GetMetrics(&data.ProjectArgs{ProjectID: bad}, &reply)
		},
		"ListAPIKeys": func() error {
			var reply []data.APIKey
			return srv.ListAPIKeys(&data.ProjectArgs{ProjectID: bad}, &reply)
		},
		"CreateAPIKey": func() error {
			var reply data.RPCCreateAPIKeyReply
			return srv.CreateAPIKey(&data.RPCCreateAPIKeyArgs{ProjectID: bad}, &reply)
		},
		"RevokeAPIKey": func() error {
			var reply string
			return srv.RevokeAPIKey(&data.RPCRevokeAPIKeyArgs{ProjectID: bad, ID: primitive.NewObjectID().Hex()}, &reply)
		},
	} {
		err := call()
		if err == nil || !strings.Contains(err.Error(), "not a valid ObjectID") {
			t.Errorf("%s: want a \"not a valid ObjectID\" error, got %v", name, err)
		}
	}
}

// TestCreateProject_RequiresOwner checks that no project is created without the
// owner it is created with: one without an owner could never be reached.
func TestCreateProject_RequiresOwner(t *testing.T) {
	srv := &RPCServer{} // zero-value models: reaching MongoDB would panic

	for _, owner := range []string{"", "   "} {
		var reply data.Project
		err := srv.CreateProject(&data.RPCCreateProjectArgs{Name: "App", Slug: "app", Owner: owner}, &reply)
		if err == nil || !strings.Contains(err.Error(), "owner is required") {
			t.Errorf("CreateProject with owner %q: want \"owner is required\", got %v", owner, err)
		}
	}
}

func TestWithoutHash(t *testing.T) {
	key := data.APIKey{Prefix: "lw_abcdefg", Hash: "$2a$10$secret"}
	if got := withoutHash(key); got.Hash != "" || got.Prefix != key.Prefix {
		t.Errorf("withoutHash = %+v, want the key minus its hash", got)
	}
}
