package main

import (
	"logwolf-toolbox/data"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

// --- test doubles ---

type alwaysValidKey struct{}

func (a alwaysValidKey) ValidateAPIKey(string) (bool, *data.APIKey, error) { return true, nil, nil }

type alwaysInvalidKey struct{}

func (a alwaysInvalidKey) ValidateAPIKey(string) (bool, *data.APIKey, error) { return false, nil, nil }

func okHandler(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusOK) }

func newApp() *Config { return &Config{} }

// --- helpers ---

func makeRequest(key string) *http.Request {
	r := httptest.NewRequest(http.MethodGet, "/logs", nil)
	if key != "" {
		r.Header.Set("Authorization", "Bearer "+key)
	}
	return r
}

// --- tests ---

func TestRequireAPIKey_ValidKey(t *testing.T) {
	app := newApp()
	handler := app.requireAPIKeyWith(alwaysValidKey{}, http.HandlerFunc(okHandler))

	w := httptest.NewRecorder()
	handler.ServeHTTP(w, makeRequest("lw_validkey1234567"))

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
}

func TestRequireAPIKey_InvalidKey(t *testing.T) {
	app := newApp()
	handler := app.requireAPIKeyWith(alwaysInvalidKey{}, http.HandlerFunc(okHandler))

	w := httptest.NewRecorder()
	handler.ServeHTTP(w, makeRequest("lw_invalidkey12345"))

	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", w.Code)
	}
}

func TestRequireAPIKey_MissingHeader(t *testing.T) {
	app := newApp()
	handler := app.requireAPIKeyWith(alwaysValidKey{}, http.HandlerFunc(okHandler))

	w := httptest.NewRecorder()
	handler.ServeHTTP(w, makeRequest(""))

	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", w.Code)
	}
}

func TestRequireAPIKey_ExpiredCache(t *testing.T) {
	// Seed the cache with an expired valid entry
	key := "lw_expiredkey12345"
	hash := hashKey(key)
	keyCacheMu.Lock()
	keyCache[hash] = cacheEntry{valid: true, expiresAt: time.Now().Add(-1 * time.Second)}
	keyCacheMu.Unlock()

	// Validator returns invalid — expired cache must not grant access
	app := newApp()
	handler := app.requireAPIKeyWith(alwaysInvalidKey{}, http.HandlerFunc(okHandler))

	w := httptest.NewRecorder()
	handler.ServeHTTP(w, makeRequest(key))

	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected 401 on expired cache, got %d", w.Code)
	}
}

func TestRequireAPIKey_RateLimit(t *testing.T) {
	app := newApp()
	handler := app.requireAPIKeyWith(alwaysInvalidKey{}, http.HandlerFunc(okHandler))

	// Exhaust the rate limit
	for i := 0; i < maxFailures; i++ {
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, makeRequest("lw_badkey000000000"))
	}

	// Next request should be rate limited
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, makeRequest("lw_badkey000000000"))

	if w.Code != http.StatusTooManyRequests {
		t.Errorf("expected 429, got %d", w.Code)
	}
}
