package main

import (
	"net/http"
	"slices"
	"testing"

	"go.mongodb.org/mongo-driver/bson/primitive"

	"logwolf-toolbox/data"
)

// The broker has no database: every key operation below is a call to the fake
// logger, over the same net/rpc wire the real one is reached by.

func TestAPIKeys_CreateListRevokeThroughLogger(t *testing.T) {
	h, f := newInternalTestServer(t)

	w := do(h, internalRequest(http.MethodPost, "/keys", "member-a", map[string]any{
		"project_id": projAlpha,
		"scopes":     []string{data.ScopeRead, data.ScopeIngest},
	}))
	if w.Code != http.StatusCreated {
		t.Fatalf("POST /keys = %d, want 201 (body: %s)", w.Code, w.Body.String())
	}
	created := decodeData[struct {
		Key    string   `json:"key"`
		Prefix string   `json:"prefix"`
		ID     string   `json:"id"`
		Scopes []string `json:"scopes"`
	}](t, w)
	if created.Key == "" || created.Prefix != created.Key[:10] {
		t.Errorf("created key = %q with prefix %q, want a plaintext and its prefix", created.Key, created.Prefix)
	}
	if created.ID == "" || created.ID == primitive.NilObjectID.Hex() {
		t.Errorf("created id = %q, want the id the logger assigned", created.ID)
	}
	if want := []string{data.ScopeIngest, data.ScopeRead}; !slices.Equal(created.Scopes, want) {
		t.Errorf("created scopes = %v, want %v", created.Scopes, want)
	}

	w = do(h, internalRequest(http.MethodGet, "/keys?project_id="+projAlpha, "member-a", nil))
	if w.Code != http.StatusOK {
		t.Fatalf("GET /keys = %d, want 200 (body: %s)", w.Code, w.Body.String())
	}
	listed := decodeData[[]data.APIKey](t, w)
	if len(listed) != 1 || listed[0].ID.Hex() != created.ID || !listed[0].Active {
		t.Fatalf("GET /keys = %+v, want the one active key just created", listed)
	}

	w = do(h, internalRequest(http.MethodDelete, "/keys/"+created.ID, "member-a", nil))
	if w.Code != http.StatusOK {
		t.Fatalf("DELETE /keys/{id} = %d, want 200 (body: %s)", w.Code, w.Body.String())
	}
	f.snapshot(func(f *fakeLogger) {
		want := data.RPCRevokeAPIKeyArgs{ProjectID: projAlpha, ID: created.ID}
		if !slices.Equal(f.revokedKeys, []data.RPCRevokeAPIKeyArgs{want}) {
			t.Errorf("RevokeAPIKey calls = %+v, want %+v", f.revokedKeys, want)
		}
		if f.keys[created.ID].Active {
			t.Error("key still active after DELETE /keys/{id}")
		}
	})
}

// Revoking checks membership in the key's own project, and names that project
// to the logger, so a key of another project stays untouched.
func TestRevokeAPIKey_OtherProjectsKeyIsForbidden(t *testing.T) {
	h, f := newInternalTestServer(t)
	_, betaKey := f.addKey(projBeta, data.ScopeIngest)

	w := do(h, internalRequest(http.MethodDelete, "/keys/"+betaKey.ID.Hex(), "owner-a", nil))
	if w.Code != http.StatusForbidden {
		t.Errorf("revoke another project's key = %d, want 403 (body: %s)", w.Code, w.Body.String())
	}
	f.snapshot(func(f *fakeLogger) {
		if len(f.revokedKeys) != 0 {
			t.Errorf("RevokeAPIKey forwarded for a non-member: %+v", f.revokedKeys)
		}
		if !f.keys[betaKey.ID.Hex()].Active {
			t.Error("another project's key was revoked")
		}
	})
}

func TestRevokeAPIKey_UnknownKeyIsNotFound(t *testing.T) {
	h, _ := newInternalTestServer(t)

	w := do(h, internalRequest(http.MethodDelete, "/keys/"+primitive.NewObjectID().Hex(), "owner-a", nil))
	if w.Code != http.StatusNotFound {
		t.Errorf("revoke an unknown key = %d, want 404 (body: %s)", w.Code, w.Body.String())
	}
}

// requireAPIKey resolves a key it has not cached through the logger, and
// carries the project and scopes the logger answered into the request.
func TestRequireAPIKey_ValidatesThroughLogger(t *testing.T) {
	h, f := newInternalTestServer(t)
	plaintext, _ := f.addKey(projAlpha, data.ScopeRead)
	t.Cleanup(func() {
		keyCacheMu.Lock()
		delete(keyCache, hashKey(plaintext))
		keyCacheMu.Unlock()
	})

	w := do(h, keyRequest(http.MethodGet, "/logs", plaintext, ""))
	if w.Code != http.StatusOK {
		t.Fatalf("GET /logs with a read key = %d, want 200 (body: %s)", w.Code, w.Body.String())
	}
	f.snapshot(func(f *fakeLogger) {
		if len(f.getLogsParams) != 1 || f.getLogsParams[0].ProjectID != projAlpha {
			t.Errorf("GetLogs calls = %+v, want one for the key's project %s", f.getLogsParams, projAlpha)
		}
	})

	if w := do(h, keyRequest(http.MethodDelete, "/logs", plaintext, "{}")); w.Code != http.StatusForbidden {
		t.Errorf("DELETE /logs with a read-only key = %d, want 403", w.Code)
	}
}

func TestRequireAPIKey_UnknownKeyIsUnauthorized(t *testing.T) {
	h, _ := newInternalTestServer(t)
	const unknown = "lw_neverissuedbythefakelogger00000000000000001"
	t.Cleanup(func() {
		keyCacheMu.Lock()
		delete(keyCache, hashKey(unknown))
		keyCacheMu.Unlock()
		ipLimiterMu.Lock()
		clear(ipLimiter)
		ipLimiterMu.Unlock()
	})

	if w := do(h, keyRequest(http.MethodGet, "/logs", unknown, "")); w.Code != http.StatusUnauthorized {
		t.Errorf("GET /logs with an unknown key = %d, want 401", w.Code)
	}
}
