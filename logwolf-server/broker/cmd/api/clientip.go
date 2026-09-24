package main

import (
	"fmt"
	"net"
	"net/http"
	"net/netip"
	"os"
	"strings"
)

// --- Client IP ---
// The broker sits behind Caddy, so every request from the internet reaches it
// from Caddy's address. Keyed on that, the failed-auth rate limiter would be one
// counter for everybody: ten bad keys from anyone and every SDK client gets 429.
// clientIP looks past the proxies the broker is told to trust.

// parseTrustedProxies reads a comma-separated list of IPs and CIDR ranges. An
// entry that is neither is an error, so a typo stops the broker at start rather
// than silently trusting nothing.
func parseTrustedProxies(list string) ([]netip.Prefix, error) {
	var prefixes []netip.Prefix
	for _, entry := range strings.Split(list, ",") {
		entry = strings.TrimSpace(entry)
		if entry == "" {
			continue
		}
		if strings.Contains(entry, "/") {
			p, err := netip.ParsePrefix(entry)
			if err != nil {
				return nil, fmt.Errorf("TRUSTED_PROXIES: %q is not an IP or CIDR range", entry)
			}
			prefixes = append(prefixes, p.Masked())
			continue
		}
		a, err := netip.ParseAddr(entry)
		if err != nil {
			return nil, fmt.Errorf("TRUSTED_PROXIES: %q is not an IP or CIDR range", entry)
		}
		prefixes = append(prefixes, netip.PrefixFrom(a.Unmap(), a.Unmap().BitLen()))
	}
	return prefixes, nil
}

func trustedProxiesFromEnv() ([]netip.Prefix, error) {
	return parseTrustedProxies(os.Getenv("TRUSTED_PROXIES"))
}

func isTrusted(a netip.Addr, trusted []netip.Prefix) bool {
	a = a.Unmap()
	for _, p := range trusted {
		if p.Contains(a) {
			return true
		}
	}
	return false
}

// clientIP returns the address of the client behind r. A request straight from
// an untrusted peer is that peer. One from a trusted proxy is the right-most
// X-Forwarded-For entry that is not a trusted proxy itself: each proxy appends
// the address it saw, so entries left of the last untrusted one could be
// anything the client chose to send, and are never believed.
//
// An entry that is not an IP ends the walk at the proxy that forwarded it, the
// same as a request with no X-Forwarded-For at all.
func clientIP(r *http.Request, trusted []netip.Prefix) string {
	peer := peerIP(r.RemoteAddr)
	addr, err := netip.ParseAddr(peer)
	if err != nil || !isTrusted(addr, trusted) {
		return peer
	}

	hops := forwardedFor(r.Header)
	for i := len(hops) - 1; i >= 0; i-- {
		hop, err := netip.ParseAddr(hops[i])
		if err != nil {
			return addr.Unmap().String()
		}
		addr = hop
		if !isTrusted(hop, trusted) {
			break
		}
	}
	return addr.Unmap().String()
}

// forwardedFor flattens every X-Forwarded-For header into its entries, in order.
func forwardedFor(h http.Header) []string {
	var hops []string
	for _, v := range h.Values("X-Forwarded-For") {
		for _, hop := range strings.Split(v, ",") {
			hops = append(hops, strings.TrimSpace(hop))
		}
	}
	return hops
}

// peerIP extracts the IP portion of an addr:port string. Falls back to the
// full string if it cannot be parsed cleanly.
func peerIP(remoteAddr string) string {
	if host, _, err := net.SplitHostPort(remoteAddr); err == nil {
		return host
	}
	return remoteAddr
}
