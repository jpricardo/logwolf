package main

import (
	"context"
	"fmt"
	"logwolf-toolbox/data"
	"net/http"
	"testing"
	"time"
)

// resetAuthCaches empties keyCache and ipLimiter now and again when t ends, so
// a test that fills them neither sees nor leaves other tests' entries.
func resetAuthCaches(t *testing.T) {
	t.Helper()
	reset := func() {
		keyCacheMu.Lock()
		clear(keyCache)
		keyCacheMu.Unlock()
		ipLimiterMu.Lock()
		clear(ipLimiter)
		ipLimiterMu.Unlock()
	}
	reset()
	t.Cleanup(reset)
}

func authCacheSizes() (keys, ips int) {
	keyCacheMu.RLock()
	keys = len(keyCache)
	keyCacheMu.RUnlock()
	ipLimiterMu.Lock()
	ips = len(ipLimiter)
	ipLimiterMu.Unlock()
	return keys, ips
}

// TestSDKLogRoutes_CloseTheirRPCClient is the first acceptance criterion: the
// SDK's read and delete routes leave no connection to the logger open.
func TestSDKLogRoutes_CloseTheirRPCClient(t *testing.T) {
	h, f := newInternalTestServer(t)
	key := seedKey(t, projAlpha, data.ScopeRead, data.ScopeDelete)

	for _, method := range []string{http.MethodGet, http.MethodDelete} {
		w := do(h, keyRequest(method, "/logs", key, `{}`))
		if w.Code != http.StatusOK && w.Code != http.StatusAccepted {
			t.Fatalf("%s /logs = %d, want success (body: %s)", method, w.Code, w.Body.String())
		}
	}

	// The fake closes its end once it reads EOF, which takes a moment.
	deadline := time.Now().Add(2 * time.Second)
	for f.openConns.Load() != 0 {
		if time.Now().After(deadline) {
			t.Fatalf("%d RPC connection(s) to the logger left open", f.openConns.Load())
		}
		time.Sleep(10 * time.Millisecond)
	}
}

// TestRequireAPIKey_FloodOfInvalidKeysStaysBounded is the second acceptance
// criterion: made-up keys from ever-new addresses cannot grow the key cache or
// the rate limiter past their caps.
func TestRequireAPIKey_FloodOfInvalidKeysStaysBounded(t *testing.T) {
	resetAuthCaches(t)
	defer func(keys, ips int) { maxKeyCacheEntries, maxIPLimiterEntries = keys, ips }(maxKeyCacheEntries, maxIPLimiterEntries)
	maxKeyCacheEntries, maxIPLimiterEntries = 50, 50

	handler := newApp().requireAPIKeyWith(alwaysInvalidKey{}, http.HandlerFunc(okHandler))

	for i := range 500 {
		r := makeRequest(fmt.Sprintf("lw_flood%038d", i))
		r.RemoteAddr = fmt.Sprintf("10.0.%d.%d:1234", i/256, i%256)
		if w := do(handler, r); w.Code != http.StatusUnauthorized {
			t.Fatalf("request %d = %d, want 401", i, w.Code)
		}
	}

	keys, ips := authCacheSizes()
	if keys > maxKeyCacheEntries {
		t.Errorf("keyCache holds %d entries, want at most %d", keys, maxKeyCacheEntries)
	}
	if ips > maxIPLimiterEntries {
		t.Errorf("ipLimiter holds %d entries, want at most %d", ips, maxIPLimiterEntries)
	}
}

// TestSweepAuthCaches_DropsExpiredKeepsLive verifies the sweep deletes what
// reads already ignore, and nothing else.
func TestSweepAuthCaches_DropsExpiredKeepsLive(t *testing.T) {
	resetAuthCaches(t)
	now := time.Now()

	keyCacheMu.Lock()
	keyCache["expired"] = cacheEntry{expiresAt: now.Add(-time.Second)}
	keyCache["live"] = cacheEntry{valid: true, expiresAt: now.Add(time.Minute)}
	keyCacheMu.Unlock()
	ipLimiterMu.Lock()
	ipLimiter["10.0.0.1"] = &ipEntry{failures: 3, windowEnd: now.Add(-time.Second)}
	ipLimiter["10.0.0.2"] = &ipEntry{failures: 3, windowEnd: now.Add(time.Minute)}
	ipLimiterMu.Unlock()

	sweepAuthCaches(now)

	keyCacheMu.RLock()
	_, expired := keyCache["expired"]
	_, live := keyCache["live"]
	keyCacheMu.RUnlock()
	if expired || !live {
		t.Errorf("keyCache after sweep: expired kept = %v, live kept = %v; want false, true", expired, live)
	}

	ipLimiterMu.Lock()
	_, expired = ipLimiter["10.0.0.1"]
	_, live = ipLimiter["10.0.0.2"]
	ipLimiterMu.Unlock()
	if expired || !live {
		t.Errorf("ipLimiter after sweep: expired kept = %v, live kept = %v; want false, true", expired, live)
	}
}

// TestSweepAuthCachesEvery_SweepsUntilCancelled verifies the loop main starts
// actually sweeps, and returns once its context is done.
func TestSweepAuthCachesEvery_SweepsUntilCancelled(t *testing.T) {
	resetAuthCaches(t)

	keyCacheMu.Lock()
	keyCache["expired"] = cacheEntry{expiresAt: time.Now().Add(-time.Second)}
	keyCacheMu.Unlock()

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		sweepAuthCachesEvery(ctx, 5*time.Millisecond)
		close(done)
	}()

	deadline := time.Now().Add(2 * time.Second)
	for keys, _ := authCacheSizes(); keys != 0; keys, _ = authCacheSizes() {
		if time.Now().After(deadline) {
			t.Fatal("expired entry never swept")
		}
		time.Sleep(5 * time.Millisecond)
	}

	cancel()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("sweep loop did not return after cancel")
	}
}
