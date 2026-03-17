package main

import (
	"fmt"
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
			app.errorJSON(w, fmt.Errorf("missing or malformed Authorization header"), http.StatusUnauthorized)
			return
		}

		plaintext := strings.TrimPrefix(authHeader, "Bearer ")

		// Check cache first
		keyCacheMu.RLock()
		entry, cached := keyCache[plaintext]
		keyCacheMu.RUnlock()

		if cached && time.Now().Before(entry.expiresAt) {
			if !entry.valid {
				app.errorJSON(w, fmt.Errorf("invalid API key"), http.StatusUnauthorized)
				return
			}
			next.ServeHTTP(w, r)
			return
		}

		// Cache miss — validate against DB via Logger RPC
		valid, _, err := app.Models.ValidateAPIKey(plaintext)
		if err != nil {
			app.errorJSON(w, fmt.Errorf("error validating API key"), http.StatusInternalServerError)
			return
		}

		// Write result to cache
		keyCacheMu.Lock()
		keyCache[plaintext] = cacheEntry{valid: valid, expiresAt: time.Now().Add(cacheTTL)}
		keyCacheMu.Unlock()

		if !valid {
			app.errorJSON(w, fmt.Errorf("invalid API key"), http.StatusUnauthorized)
			return
		}

		next.ServeHTTP(w, r)
	})
}
