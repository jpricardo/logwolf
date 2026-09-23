package main

import (
	"context"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"fmt"
	"log"
	"logwolf-toolbox/data"
	"net/http"
	"net/rpc"
	"os"
	"slices"
	"strings"
	"sync"
	"time"
)

type contextKey string

const projectIDKey contextKey = "projectID"
const userLoginKey contextKey = "userLogin"
const keyScopesKey contextKey = "keyScopes"

func projectIDFromContext(r *http.Request) string {
	if v, ok := r.Context().Value(projectIDKey).(string); ok {
		return v
	}
	return ""
}

func keyScopesFromContext(r *http.Request) []string {
	if v, ok := r.Context().Value(keyScopesKey).([]string); ok {
		return v
	}
	return nil
}

func userLoginFromContext(r *http.Request) string {
	if v, ok := r.Context().Value(userLoginKey).(string); ok {
		return v
	}
	return ""
}

type cacheEntry struct {
	valid     bool
	projectID string
	scopes    []string
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

type keyValidator interface {
	ValidateAPIKey(plaintext string) (bool, *data.APIKey, error)
}

// loggerKeyValidator validates keys through the logger, which owns the
// api_keys collection.
type loggerKeyValidator struct{}

func (loggerKeyValidator) ValidateAPIKey(plaintext string) (bool, *data.APIKey, error) {
	client, err := rpc.Dial("tcp", loggerRPCAddr())
	if err != nil {
		return false, nil, err
	}
	defer client.Close()

	var reply data.RPCValidateAPIKeyReply
	if err := client.Call("RPCServer.ValidateAPIKey", &data.RPCValidateAPIKeyArgs{Plaintext: plaintext}, &reply); err != nil {
		return false, nil, err
	}
	if !reply.Valid {
		return false, nil, nil
	}
	return true, &reply.Key, nil
}

func (app *Config) requireAPIKey(next http.Handler) http.Handler {
	return app.requireAPIKeyWith(loggerKeyValidator{}, next)
}

func (app *Config) requireAPIKeyWith(v keyValidator, next http.Handler) http.Handler {
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
			next.ServeHTTP(w, withKey(r, entry.projectID, entry.scopes))
			return
		}

		// Cache miss — validate against DB via Logger RPC
		valid, key, err := v.ValidateAPIKey(plaintext)
		if err != nil {
			log.Printf(`{"event":"auth","outcome":"error","reason":"db_error","key_prefix":"%s","method":"%s","path":"%s","remote_addr":"%s","error":"%s"}`,
				keyPrefix, r.Method, r.URL.Path, r.RemoteAddr, err.Error())
			app.errorJSON(w, fmt.Errorf("error validating API key"), http.StatusInternalServerError)
			return
		}

		projectID := ""
		var scopes []string
		if key != nil {
			projectID = key.ProjectID
			scopes = key.Scopes
		}

		// Write result to cache (keyed on hash).
		keyCacheMu.Lock()
		keyCache[cacheKey] = cacheEntry{valid: valid, projectID: projectID, scopes: scopes, expiresAt: time.Now().Add(cacheTTL)}
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
		next.ServeHTTP(w, withKey(r, projectID, scopes))
	})
}

// withKey stores what requireAPIKey learned about the key in the request
// context: the project it belongs to and the scopes requireScope checks.
func withKey(r *http.Request, projectID string, scopes []string) *http.Request {
	ctx := context.WithValue(r.Context(), projectIDKey, projectID)
	ctx = context.WithValue(ctx, keyScopesKey, scopes)
	return r.WithContext(ctx)
}

// requireScope refuses with 403 a request whose API key does not grant scope.
// It MUST run after requireAPIKey, which puts the key's scopes in the context;
// without it there are none, and every request is refused.
//
// A failed scope check is not recorded against the IP rate limiter: the key is
// genuine, and the caller learns nothing by retrying.
func (app *Config) requireScope(scope string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if !slices.Contains(keyScopesFromContext(r), scope) {
				log.Printf(`{"event":"auth","outcome":"deny","reason":"missing_scope","scope":"%s","method":"%s","path":"%s","remote_addr":"%s"}`,
					scope, r.Method, r.URL.Path, r.RemoteAddr)
				app.errorJSON(w, fmt.Errorf("API key lacks the %q scope", scope), http.StatusForbidden)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

// safePrefix returns the first 10 chars of the key ("lw_" + 7 chars) for logging.
// Never logs the full key.
func safePrefix(key string) string {
	if len(key) >= 10 {
		return key[:10]
	}
	return "[invalid]"
}

// requireUserLogin extracts the GitHub login from X-User-Login and stores it,
// normalized, in the request context for downstream handlers. Memberships are
// stored normalized, so the handlers' role checks can compare with ==.
//
// X-User-Login is caller-supplied and trusted without further verification.
// This middleware MUST run after requireInternalSecret; without that guard,
// any client could impersonate any user by forging the header.
func (app *Config) requireUserLogin(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		login := data.NormalizeGithubLogin(r.Header.Get("X-User-Login"))
		if login == "" {
			log.Printf(`{"event":"auth","outcome":"deny","reason":"missing_x_user_login","method":"%s","path":"%s","remote_addr":"%s"}`,
				r.Method, r.URL.Path, r.RemoteAddr)
			app.errorJSON(w, fmt.Errorf("missing X-User-Login header"), http.StatusUnauthorized)
			return
		}
		ctx := context.WithValue(r.Context(), userLoginKey, login)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// checkProjectMembership reports whether userLogin is a member of projectID.
// The caller is responsible for dialing the RPC client and closing it.
// Accepting the client lets handlers that also need RPC for data reuse the
// same connection instead of opening a second TCP dial.
//
// This is a package-level function (not a Config method) because it relies
// only on the RPC client passed in and has no dependency on Config state.
func checkProjectMembership(client *rpc.Client, projectID, userLogin string) (bool, error) {
	args := data.RPCCheckMembershipArgs{ProjectID: projectID, GithubLogin: userLogin}
	var isMember bool
	return isMember, client.Call("RPCServer.CheckMembership", &args, &isMember)
}

// getProjectRole returns the role of userLogin in projectID ("owner", "member", or "").
// An empty string means the user is not a member (project may or may not exist).
func getProjectRole(client *rpc.Client, projectID, userLogin string) (string, error) {
	args := data.ProjectArgs{ProjectID: projectID}
	var members []data.ProjectMember
	if err := client.Call("RPCServer.ListMembers", &args, &members); err != nil {
		return "", err
	}
	for _, m := range members {
		if m.GithubLogin == userLogin {
			return m.Role, nil
		}
	}
	return "", nil
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
