package main

import (
	"net/http"
	"testing"
	"time"

	"logwolf-toolbox/data"
)

// readLogs is an SDK read with key, through the real router.
func readLogs(h http.Handler, key string) int {
	return do(h, keyRequest(http.MethodGet, "/logs", key, "")).Code
}

// TestRevokedKey_RefusedAtOnce: revoking a key used to leave it working from the
// broker's cache for up to cacheTTL.
func TestRevokedKey_RefusedAtOnce(t *testing.T) {
	resetAuthCaches(t)
	h, f := newInternalTestServer(t)
	plaintext, key := f.addKey(projAlpha, data.ScopeRead)
	otherPlaintext, _ := f.addKey(projAlpha, data.ScopeRead)

	if code := readLogs(h, plaintext); code != http.StatusOK {
		t.Fatalf("GET /logs before revoking = %d, want 200", code)
	}
	if code := readLogs(h, otherPlaintext); code != http.StatusOK {
		t.Fatalf("GET /logs with the other key = %d, want 200", code)
	}

	w := do(h, internalRequest(http.MethodDelete, "/projects/"+projAlpha+"/keys/"+key.ID.Hex(), "member-a", nil))
	if w.Code != http.StatusOK {
		t.Fatalf("DELETE /projects/{id}/keys/{keyID} = %d, want 200 (body: %s)", w.Code, w.Body.String())
	}

	if code := readLogs(h, plaintext); code != http.StatusUnauthorized {
		t.Errorf("GET /logs with the revoked key = %d, want 401", code)
	}
	if code := readLogs(h, otherPlaintext); code != http.StatusOK {
		t.Errorf("GET /logs with a key that was not revoked = %d, want 200", code)
	}
}

// TestDeletedProjectKeys_RefusedAtOnce: DeleteProject deletes the project's keys,
// and the broker drops them from its cache with it.
func TestDeletedProjectKeys_RefusedAtOnce(t *testing.T) {
	resetAuthCaches(t)
	h, f := newInternalTestServer(t)
	alphaPlaintext, _ := f.addKey(projAlpha, data.ScopeRead)
	betaPlaintext, _ := f.addKey(projBeta, data.ScopeRead)

	for _, k := range []string{alphaPlaintext, betaPlaintext} {
		if code := readLogs(h, k); code != http.StatusOK {
			t.Fatalf("GET /logs before deleting = %d, want 200", code)
		}
	}

	w := do(h, internalRequest(http.MethodDelete, "/projects/"+projAlpha, "owner-a", nil))
	if w.Code != http.StatusOK {
		t.Fatalf("DELETE /projects/{id} = %d, want 200 (body: %s)", w.Code, w.Body.String())
	}

	if code := readLogs(h, alphaPlaintext); code != http.StatusUnauthorized {
		t.Errorf("GET /logs with a deleted project's key = %d, want 401", code)
	}
	if code := readLogs(h, betaPlaintext); code != http.StatusOK {
		t.Errorf("GET /logs with another project's key = %d, want 200", code)
	}
}

func TestForgetCachedKeys_EvictsOnlyMatchingValidEntries(t *testing.T) {
	resetAuthCaches(t)
	live := time.Now().Add(time.Minute)

	keyCacheMu.Lock()
	keyCache["a1"] = cacheEntry{valid: true, keyID: "k1", projectID: "pa", expiresAt: live}
	keyCache["a2"] = cacheEntry{valid: true, keyID: "k2", projectID: "pa", expiresAt: live}
	keyCache["b1"] = cacheEntry{valid: true, keyID: "k3", projectID: "pb", expiresAt: live}
	keyCache["bad"] = cacheEntry{valid: false, expiresAt: live}
	keyCacheMu.Unlock()

	forgetCachedKey("k1")
	assertCached(t, "a1", false)
	assertCached(t, "a2", true)

	forgetCachedProjectKeys("pa")
	assertCached(t, "a2", false)
	assertCached(t, "b1", true)
	// Invalid entries name no key or project; an eviction leaves them alone.
	assertCached(t, "bad", true)
}

// TestCacheKeyResult_DropsAResultThatRacedAnEviction: a validation that was in
// flight while a key was revoked may have read the key as still active.
// Caching that would bring the revoked key back for a whole cacheTTL.
func TestCacheKeyResult_DropsAResultThatRacedAnEviction(t *testing.T) {
	resetAuthCaches(t)
	entry := cacheEntry{valid: true, keyID: "k1", projectID: "pa", expiresAt: time.Now().Add(time.Minute)}

	gen := keyCacheGeneration()
	forgetCachedKey("k1")
	cacheKeyResult("raced", entry, gen)
	assertCached(t, "raced", false)

	cacheKeyResult("fresh", entry, keyCacheGeneration())
	assertCached(t, "fresh", true)
}

func assertCached(t *testing.T, cacheKey string, want bool) {
	t.Helper()
	keyCacheMu.RLock()
	_, got := keyCache[cacheKey]
	keyCacheMu.RUnlock()
	if got != want {
		t.Errorf("keyCache[%q] present = %v, want %v", cacheKey, got, want)
	}
}
