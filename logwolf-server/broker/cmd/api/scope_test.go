package main

import (
	"fmt"
	"logwolf-toolbox/data"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

// seededKeyCounter keeps every seeded key distinct, so tests sharing the
// package-level key cache cannot see each other's entries.
var seededKeyCounter atomic.Int64

// seedKey puts a valid key with the given scopes in the key cache and returns
// its plaintext. requireAPIKey reads the cache before the database, so the real
// router accepts the key without a MongoDB behind app.Models.
func seedKey(t *testing.T, projectID string, scopes ...string) string {
	t.Helper()
	plaintext := fmt.Sprintf("lw_scopetest%034d", seededKeyCounter.Add(1))

	keyCacheMu.Lock()
	keyCache[hashKey(plaintext)] = cacheEntry{valid: true, projectID: projectID, scopes: scopes, expiresAt: time.Now().Add(time.Minute)}
	keyCacheMu.Unlock()
	t.Cleanup(func() {
		keyCacheMu.Lock()
		delete(keyCache, hashKey(plaintext))
		keyCacheMu.Unlock()
	})
	return plaintext
}

func keyRequest(method, target, key, body string) *http.Request {
	r := httptest.NewRequest(method, target, strings.NewReader(body))
	r.Header.Set("Authorization", "Bearer "+key)
	r.Header.Set("Content-Type", "application/json")
	return r
}

// publicRoutes pairs each public route with the scope it demands, and a body
// its handler answers without RabbitMQ, which the test server has none of: an
// empty batch, or malformed JSON that POST /logs refuses with 400 — past the
// scope check, before any emitter.
var publicRoutes = []struct {
	method, target, body, scope string
}{
	{http.MethodPost, "/logs", `{`, data.ScopeIngest},
	{http.MethodPost, "/logs/batch", `[]`, data.ScopeIngest},
	{http.MethodGet, "/logs", ``, data.ScopeRead},
	{http.MethodDelete, "/logs", `{}`, data.ScopeDelete},
}

// TestPublicRoutes_IngestOnlyKeyCannotReadOrDelete is the issue's acceptance
// criterion: a key pulled out of a browser bundle can write events, but not
// read or wipe the project's logs.
func TestPublicRoutes_IngestOnlyKeyCannotReadOrDelete(t *testing.T) {
	h, _ := newInternalTestServer(t)
	key := seedKey(t, projAlpha, data.ScopeIngest)

	for _, method := range []string{http.MethodGet, http.MethodDelete} {
		w := do(h, keyRequest(method, "/logs", key, `{}`))
		if w.Code != http.StatusForbidden {
			t.Errorf("%s /logs with an ingest-only key = %d, want 403", method, w.Code)
		}
	}

	w := do(h, keyRequest(http.MethodPost, "/logs/batch", key, `[]`))
	if w.Code != http.StatusAccepted {
		t.Errorf("POST /logs/batch with an ingest-only key = %d, want 202", w.Code)
	}
}

// TestPublicRoutes_EachRouteDemandsItsScope checks the wiring route by route:
// every scope but the right one is refused, and the right one alone gets past
// the check.
func TestPublicRoutes_EachRouteDemandsItsScope(t *testing.T) {
	h, _ := newInternalTestServer(t)

	for _, rt := range publicRoutes {
		others := slices.DeleteFunc(slices.Clone(data.AllScopes), func(s string) bool { return s == rt.scope })

		w := do(h, keyRequest(rt.method, rt.target, seedKey(t, projAlpha, others...), rt.body))
		if w.Code != http.StatusForbidden {
			t.Errorf("%s %s without %q = %d, want 403", rt.method, rt.target, rt.scope, w.Code)
		}

		w = do(h, keyRequest(rt.method, rt.target, seedKey(t, projAlpha, rt.scope), rt.body))
		if w.Code == http.StatusForbidden || w.Code == http.StatusUnauthorized {
			t.Errorf("%s %s with only %q = %d, want the scope check passed", rt.method, rt.target, rt.scope, w.Code)
		}
	}
}

// TestPublicRoutes_FullAccessKeyReachesEveryHandler: full access is what a key
// created before scopes existed is read back with, so it must keep working.
func TestPublicRoutes_FullAccessKeyReachesEveryHandler(t *testing.T) {
	h, _ := newInternalTestServer(t)
	key := seedKey(t, projAlpha, data.AllScopes...)

	for _, rt := range publicRoutes {
		w := do(h, keyRequest(rt.method, rt.target, key, rt.body))
		if w.Code == http.StatusForbidden || w.Code == http.StatusUnauthorized {
			t.Errorf("%s %s with a full-access key = %d, want the scope check passed", rt.method, rt.target, w.Code)
		}
	}
}

// TestRequireScope_CachePathKeepsScopes verifies the scopes survive the key
// cache: a second request, served from the cache, is judged the same way as
// the first, which went to the validator.
func TestRequireScope_CachePathKeepsScopes(t *testing.T) {
	app := newApp()
	v := validKeyWithScopes{projectID: "aaaaaaaaaaaaaaaaaaaa5c0e", scopes: []string{data.ScopeIngest}}
	handler := app.requireAPIKeyWith(v, app.requireScope(data.ScopeRead)(http.HandlerFunc(okHandler)))

	for _, source := range []string{"db", "cache"} {
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, makeRequest("lw_scopecachekey12"))
		if w.Code != http.StatusForbidden {
			t.Errorf("%s path: GET with an ingest-only key = %d, want 403", source, w.Code)
		}
	}
}

// TestRequireScope_NoKeyInContextIsRefused verifies requireScope fails closed
// when it runs without requireAPIKey in front of it.
func TestRequireScope_NoKeyInContextIsRefused(t *testing.T) {
	app := newApp()
	handler := app.requireScope(data.ScopeIngest)(http.HandlerFunc(okHandler))

	w := httptest.NewRecorder()
	handler.ServeHTTP(w, httptest.NewRequest(http.MethodPost, "/logs", nil))
	if w.Code != http.StatusForbidden {
		t.Errorf("no key in context = %d, want 403", w.Code)
	}
}

// TestCreateAPIKey_RejectsUnknownScope verifies the broker refuses a scope it
// would never check, before generating anything.
func TestCreateAPIKey_RejectsUnknownScope(t *testing.T) {
	h, _ := newInternalTestServer(t)

	w := do(h, internalRequest(http.MethodPost, "/keys", "owner-a", map[string]any{
		"project_id": projAlpha,
		"scopes":     []string{data.ScopeIngest, "admin"},
	}))
	if w.Code != http.StatusBadRequest {
		t.Errorf("POST /keys with an unknown scope = %d, want 400", w.Code)
	}
}

type validKeyWithScopes struct {
	projectID string
	scopes    []string
}

func (v validKeyWithScopes) ValidateAPIKey(string) (bool, *data.APIKey, error) {
	return true, &data.APIKey{ProjectID: mustObjectID(v.projectID), Scopes: v.scopes}, nil
}
