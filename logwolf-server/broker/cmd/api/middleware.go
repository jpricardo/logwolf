package main

import (
	"fmt"
	"log"
	"net/http"
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

func (app *Config) requireAPIKey(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		authHeader := r.Header.Get("Authorization")
		if !strings.HasPrefix(authHeader, "Bearer ") {
			log.Printf(`{"event":"auth","outcome":"deny","reason":"missing_or_malformed_header","method":"%s","path":"%s","remote_addr":"%s"}`,
				r.Method, r.URL.Path, r.RemoteAddr)
			app.errorJSON(w, fmt.Errorf("missing or malformed Authorization header"), http.StatusUnauthorized)
			return
		}

		plaintext := strings.TrimPrefix(authHeader, "Bearer ")
		keyPrefix := safePrefix(plaintext)

		// Check cache first
		keyCacheMu.RLock()
		entry, cached := keyCache[plaintext]
		keyCacheMu.RUnlock()

		if cached && time.Now().Before(entry.expiresAt) {
			if !entry.valid {
				log.Printf(`{"event":"auth","outcome":"deny","reason":"invalid_key","key_prefix":"%s","method":"%s","path":"%s","remote_addr":"%s","source":"cache"}`,
					keyPrefix, r.Method, r.URL.Path, r.RemoteAddr)
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

		// Write result to cache
		keyCacheMu.Lock()
		keyCache[plaintext] = cacheEntry{valid: valid, expiresAt: time.Now().Add(cacheTTL)}
		keyCacheMu.Unlock()

		if !valid {
			log.Printf(`{"event":"auth","outcome":"deny","reason":"invalid_key","key_prefix":"%s","method":"%s","path":"%s","remote_addr":"%s","source":"db"}`,
				keyPrefix, r.Method, r.URL.Path, r.RemoteAddr)
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
