package main

import (
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"
)

type cacheEntry struct {
	valid     bool
	expiresAt time.Time
}

var (
	keyCache   = make(map[string]cacheEntry)
	keyCacheMu sync.RWMutex
	cacheTTL   = 60 * time.Second
)

func hashKey(plaintext string) string {
	sum := sha256.Sum256([]byte(plaintext))
	return hex.EncodeToString(sum[:])
}

// --- IP rate limiter ---
// Sliding-window counter: tracks failed auth attempts per remote IP.
// After maxFailures within the window, requests are rejected with 429.

const (
	rateLimitWindow = 1 * time.Minute
	maxFailures     = 10
)

type ipEntry struct {
	failures  int
	windowEnd time.Time
}

var (
	ipLimiter   = make(map[string]*ipEntry)
	ipLimiterMu sync.Mutex
)

// recordFailure increments the failure counter for addr and returns true if
// the IP is now rate-limited (i.e. failures >= maxFailures within the window).
func recordFailure(addr string) bool {
	ipLimiterMu.Lock()
	defer ipLimiterMu.Unlock()

	now := time.Now()
	entry, ok := ipLimiter[addr]
	if !ok || now.After(entry.windowEnd) {
		// First failure in this window (or previous window expired).
		ipLimiter[addr] = &ipEntry{failures: 1, windowEnd: now.Add(rateLimitWindow)}
		return false
	}

	entry.failures++
	return entry.failures >= maxFailures
}

// isRateLimited checks whether addr has already hit the limit, without
// incrementing the counter.
func isRateLimited(addr string) bool {
	ipLimiterMu.Lock()
	defer ipLimiterMu.Unlock()

	entry, ok := ipLimiter[addr]
	if !ok {
		return false
	}
	if time.Now().After(entry.windowEnd) {
		delete(ipLimiter, addr)
		return false
	}
	return entry.failures >= maxFailures
}

// remoteIP extracts the IP portion of an addr:port string. Falls back to the
// full string if it cannot be parsed cleanly.
func remoteIP(remoteAddr string) string {
	if idx := strings.LastIndex(remoteAddr, ":"); idx != -1 {
		return remoteAddr[:idx]
	}
	return remoteAddr
}

// --- Middleware ---

func (app *Config) requireAPIKey(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ip := remoteIP(r.RemoteAddr)

		// Pre-check: reject immediately if this IP is already rate-limited.
		if isRateLimited(ip) {
			log.Printf(`{"event":"auth","outcome":"deny","reason":"rate_limited","method":"%s","path":"%s","remote_addr":"%s"}`,
				r.Method, r.URL.Path, r.RemoteAddr)
			app.errorJSON(w, fmt.Errorf("too many failed attempts"), http.StatusTooManyRequests)
			return
		}

		authHeader := r.Header.Get("Authorization")
		if !strings.HasPrefix(authHeader, "Bearer ") {
			recordFailure(ip)
			log.Printf(`{"event":"auth","outcome":"deny","reason":"missing_or_malformed_header","method":"%s","path":"%s","remote_addr":"%s"}`,
				r.Method, r.URL.Path, r.RemoteAddr)
			app.errorJSON(w, fmt.Errorf("missing or malformed Authorization header"), http.StatusUnauthorized)
			return
		}

		plaintext := strings.TrimPrefix(authHeader, "Bearer ")
		keyPrefix := safePrefix(plaintext)
		cacheKey := hashKey(plaintext)

		// Check cache first (keyed on hash, not plaintext).
		keyCacheMu.RLock()
		entry, cached := keyCache[cacheKey]
		keyCacheMu.RUnlock()

		if cached && time.Now().Before(entry.expiresAt) {
			if !entry.valid {
				limited := recordFailure(ip)
				log.Printf(`{"event":"auth","outcome":"deny","reason":"invalid_key","key_prefix":"%s","method":"%s","path":"%s","remote_addr":"%s","source":"cache","rate_limited":%v}`,
					keyPrefix, r.Method, r.URL.Path, r.RemoteAddr, limited)
				app.errorJSON(w, fmt.Errorf("invalid API key"), http.StatusUnauthorized)
				return
			}
			log.Printf(`{"event":"auth","outcome":"allow","key_prefix":"%s","method":"%s","path":"%s","remote_addr":"%s","source":"cache"}`,
				keyPrefix, r.Method, r.URL.Path, r.RemoteAddr)
			next.ServeHTTP(w, r)
			return
		}

		// Cache miss — validate against DB via Logger RPC
		valid, _, err := app.Models.ValidateAPIKey(plaintext)
		if err != nil {
			log.Printf(`{"event":"auth","outcome":"error","reason":"db_error","key_prefix":"%s","method":"%s","path":"%s","remote_addr":"%s","error":"%s"}`,
				keyPrefix, r.Method, r.URL.Path, r.RemoteAddr, err.Error())
			app.errorJSON(w, fmt.Errorf("error validating API key"), http.StatusInternalServerError)
			return
		}

		// Write result to cache (keyed on hash).
		keyCacheMu.Lock()
		keyCache[cacheKey] = cacheEntry{valid: valid, expiresAt: time.Now().Add(cacheTTL)}
		keyCacheMu.Unlock()

		if !valid {
			limited := recordFailure(ip)
			log.Printf(`{"event":"auth","outcome":"deny","reason":"invalid_key","key_prefix":"%s","method":"%s","path":"%s","remote_addr":"%s","source":"db","rate_limited":%v}`,
				keyPrefix, r.Method, r.URL.Path, r.RemoteAddr, limited)
			app.errorJSON(w, fmt.Errorf("invalid API key"), http.StatusUnauthorized)
			return
		}

		log.Printf(`{"event":"auth","outcome":"allow","key_prefix":"%s","method":"%s","path":"%s","remote_addr":"%s","source":"db"}`,
			keyPrefix, r.Method, r.URL.Path, r.RemoteAddr)
		next.ServeHTTP(w, r)
	})
}

// safePrefix returns the first 10 chars of the key ("lw_" + 7 chars) for logging.
// Never logs the full key.
func safePrefix(key string) string {
	if len(key) >= 10 {
		return key[:10]
	}
	return "[invalid]"
}

func (app *Config) requireInternalSecret(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		secret := os.Getenv("INTERNAL_API_SECRET")
		header := r.Header.Get("X-Internal-Secret")
		if secret == "" || (subtle.ConstantTimeCompare([]byte(header), []byte(secret)) == 0) {
			app.errorJSON(w, fmt.Errorf("unauthorized"), http.StatusUnauthorized)
			return
		}
		next.ServeHTTP(w, r)
	})
}
