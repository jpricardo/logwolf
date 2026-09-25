package main

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/netip"
	"testing"
)

func mustTrusted(t *testing.T, list string) []netip.Prefix {
	t.Helper()
	p, err := parseTrustedProxies(list)
	if err != nil {
		t.Fatalf("parseTrustedProxies(%q): %v", list, err)
	}
	return p
}

func TestParseTrustedProxies(t *testing.T) {
	p := mustTrusted(t, " 10.0.0.0/8, 172.18.0.5 ,,fd00::/8")
	if len(p) != 3 {
		t.Fatalf("got %d prefixes, want 3: %v", len(p), p)
	}
	if !isTrusted(netip.MustParseAddr("172.18.0.5"), p) || isTrusted(netip.MustParseAddr("172.18.0.6"), p) {
		t.Errorf("a bare IP should trust that address alone: %v", p)
	}

	if p := mustTrusted(t, ""); len(p) != 0 {
		t.Errorf("empty list = %v, want none", p)
	}

	for _, bad := range []string{"caddy", "10.0.0.0/33", "10.0.0.1:80"} {
		if _, err := parseTrustedProxies(bad); err == nil {
			t.Errorf("parseTrustedProxies(%q) succeeded, want an error", bad)
		}
	}
}

func TestClientIP(t *testing.T) {
	trusted := mustTrusted(t, "172.16.0.0/12")

	tests := []struct {
		name       string
		trusted    []netip.Prefix
		remoteAddr string
		xff        []string
		want       string
	}{
		{
			name:       "no trusted proxies ignores the header",
			remoteAddr: "172.18.0.2:5000",
			xff:        []string{"203.0.113.7"},
			want:       "172.18.0.2",
		},
		{
			name:       "untrusted peer cannot spoof its address",
			trusted:    trusted,
			remoteAddr: "198.51.100.9:5000",
			xff:        []string{"203.0.113.7"},
			want:       "198.51.100.9",
		},
		{
			name:       "trusted proxy forwards the client",
			trusted:    trusted,
			remoteAddr: "172.18.0.2:5000",
			xff:        []string{"203.0.113.7"},
			want:       "203.0.113.7",
		},
		{
			name:       "entries left of the first untrusted hop are not believed",
			trusted:    trusted,
			remoteAddr: "172.18.0.2:5000",
			xff:        []string{"1.2.3.4, 203.0.113.7"},
			want:       "203.0.113.7",
		},
		{
			name:       "chained trusted proxies are skipped",
			trusted:    trusted,
			remoteAddr: "172.18.0.2:5000",
			xff:        []string{"203.0.113.7, 172.20.0.3", "172.19.0.4"},
			want:       "203.0.113.7",
		},
		{
			name:       "trusted peer without the header is the client",
			trusted:    trusted,
			remoteAddr: "172.18.0.2:5000",
			want:       "172.18.0.2",
		},
		{
			name:       "garbage entry stops at the proxy that forwarded it",
			trusted:    trusted,
			remoteAddr: "172.18.0.2:5000",
			xff:        []string{"not-an-ip"},
			want:       "172.18.0.2",
		},
		{
			name:       "IPv6 peer",
			remoteAddr: "[2001:db8::1]:5000",
			want:       "2001:db8::1",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := httptest.NewRequest(http.MethodGet, "/logs", nil)
			r.RemoteAddr = tt.remoteAddr
			for _, v := range tt.xff {
				r.Header.Add("X-Forwarded-For", v)
			}
			if got := clientIP(r, tt.trusted); got != tt.want {
				t.Errorf("clientIP = %q, want %q", got, tt.want)
			}
		})
	}
}

// TestRequireAPIKey_RateLimitIsPerClientBehindProxy is the bug this fixes: with
// every request arriving from Caddy, one client's bad keys locked out everyone.
func TestRequireAPIKey_RateLimitIsPerClientBehindProxy(t *testing.T) {
	resetAuthCaches(t)
	app := &Config{TrustedProxies: mustTrusted(t, "172.16.0.0/12")}

	viaProxy := func(client string, key string) *http.Request {
		r := makeRequest(key)
		r.RemoteAddr = "172.18.0.2:44321"
		r.Header.Set("X-Forwarded-For", client)
		return r
	}

	bad := app.requireAPIKeyWith(alwaysInvalidKey{}, http.HandlerFunc(okHandler))
	for i := range maxFailures + 1 {
		do(bad, viaProxy("203.0.113.7", fmt.Sprintf("lw_bad%040d", i)))
	}
	if w := do(bad, viaProxy("203.0.113.7", "lw_bad_again_0000000000")); w.Code != http.StatusTooManyRequests {
		t.Fatalf("offending client = %d, want 429", w.Code)
	}

	good := app.requireAPIKeyWith(alwaysValidKey{}, http.HandlerFunc(okHandler))
	if w := do(good, viaProxy("198.51.100.20", "lw_good_key_0000000000")); w.Code != http.StatusOK {
		t.Errorf("another client behind the same proxy = %d, want 200", w.Code)
	}
}
