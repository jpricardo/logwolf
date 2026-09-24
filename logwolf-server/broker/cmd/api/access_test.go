package main

import (
	"net/http"
	"strings"
	"testing"

	"logwolf-toolbox/data"
)

// projectRoute is a dashboard route that acts on one project, with a body its
// handler accepts, so that access is the only thing that can refuse it.
type projectRoute struct {
	method string
	target func(projectID string) string
	body   func(projectID string) any
	owner  bool // only an owner may use it
}

func path(suffix string) func(string) string {
	return func(id string) string { return "/projects/" + id + suffix }
}

func noBody(string) any { return nil }

func constBody(body any) func(string) any { return func(string) any { return body } }

// alphaKeyID is a key id the revoke route is tried with. It names no key: a
// request that got past the access check would be a 404 from the logger.
const alphaKeyID = "dddddddddddddddddddddd04"

// projectRoutes lists every dashboard route that acts on one project; they all
// name it in the path.
var projectRoutes = []projectRoute{
	{http.MethodGet, path("/keys"), noBody, false},
	{http.MethodPost, path("/keys"), constBody(map[string]any{}), false},
	{http.MethodDelete, path("/keys/" + alphaKeyID), noBody, false},
	{http.MethodGet, path("/retention"), noBody, false},
	{http.MethodPatch, path("/retention"), constBody(map[string]any{"days": 180}), false},
	{http.MethodGet, path("/metrics"), noBody, false},
	{http.MethodGet, path(""), noBody, false},
	{http.MethodPatch, path(""), constBody(map[string]string{"name": "Renamed"}), true},
	{http.MethodDelete, path(""), noBody, true},
	{http.MethodGet, path("/members"), noBody, false},
	{http.MethodPost, path("/members"), constBody(map[string]string{"login": "newcomer", "role": data.RoleMember}), true},
	{http.MethodPatch, path("/members/member-a"), constBody(map[string]string{"role": data.RoleOwner}), true},
	{http.MethodDelete, path("/members/member-a"), noBody, true},
	{http.MethodGet, path("/logs"), noBody, false},
	{http.MethodPost, path("/logs"), constBody(map[string]string{"name": "x"}), false},
	{http.MethodGet, path("/logs/" + alphaLogID), noBody, false},
	{http.MethodDelete, path("/logs/" + alphaLogID), noBody, false},
}

// TestProjectAccess_SameAnswerOnEveryRoute: every route that acts on a project
// denies the same way.
func TestProjectAccess_SameAnswerOnEveryRoute(t *testing.T) {
	cases := []struct {
		name      string
		projectID string
		user      string
		want      int
		ownerOnly bool // only check the owner-only routes
	}{
		{"project that does not exist", projMissing, "owner-a", http.StatusNotFound, false},
		{"id that is not an ObjectID", "not-an-id", "owner-a", http.StatusNotFound, false},
		{"outsider", projAlpha, "owner-b", http.StatusForbidden, false},
		{"member on an owner-only route", projAlpha, "member-a", http.StatusForbidden, true},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			// Rabbit is nil, so a request that got past the check would panic
			// on POST .../logs rather than answer; the status is the assertion.
			h, _ := newInternalTestServer(t)
			for _, route := range projectRoutes {
				if tc.ownerOnly && !route.owner {
					continue
				}
				target := route.target(tc.projectID)
				w := do(h, internalRequest(route.method, target, tc.user, route.body(tc.projectID)))
				if w.Code != tc.want {
					t.Errorf("%s %s as %s = %d, want %d (body: %s)", route.method, target, tc.user, w.Code, tc.want, w.Body.String())
				}
			}
		})
	}
}

// TestProjectAccess_OneLookupPerRequest: the access check is one ProjectAccess
// call, allowed or denied. Routes used to fetch the whole member list to find
// one role, and a denied caller cost a second call to tell 403 from 404.
func TestProjectAccess_OneLookupPerRequest(t *testing.T) {
	h, f := newInternalTestServer(t)

	for _, user := range []string{"owner-a", "owner-b"} {
		for _, target := range []string{"/projects/" + projAlpha + "/members", "/projects/" + projAlpha + "/metrics"} {
			f.snapshot(func(f *fakeLogger) { f.accessChecks = 0 })
			do(h, internalRequest(http.MethodGet, target, user, nil))
			f.snapshot(func(f *fakeLogger) {
				if f.accessChecks != 1 {
					t.Errorf("GET %s as %s made %d ProjectAccess calls, want 1", target, user, f.accessChecks)
				}
			})
		}
	}
}

// TestProjectAccess_MalformedPathIDSkipsTheLogger: an id in the path that is
// not an ObjectID cannot name a project, so requireProject answers 404 itself.
func TestProjectAccess_MalformedPathIDSkipsTheLogger(t *testing.T) {
	h, f := newInternalTestServer(t)

	w := do(h, internalRequest(http.MethodGet, "/projects/not-an-id", "owner-a", nil))
	if w.Code != http.StatusNotFound || !strings.Contains(w.Body.String(), "project not found") {
		t.Errorf("GET /projects/not-an-id = %d %s, want 404 project not found", w.Code, w.Body.String())
	}
	f.snapshot(func(f *fakeLogger) {
		if f.accessChecks != 0 {
			t.Errorf("made %d ProjectAccess calls for a malformed id, want 0", f.accessChecks)
		}
	})
}

// TestProjectAccess_UppercaseIDIsTheSameProject: ObjectIDFromHex accepts upper
// case, and requireProject hands handlers the canonical lower-case id, which is
// what the logger, and the key cache, know the project by.
func TestProjectAccess_UppercaseIDIsTheSameProject(t *testing.T) {
	h, _ := newInternalTestServer(t)

	w := do(h, internalRequest(http.MethodGet, "/projects/"+strings.ToUpper(projAlpha), "member-a", nil))
	if w.Code != http.StatusOK {
		t.Errorf("GET /projects/{upper-case id} = %d, want 200 (body: %s)", w.Code, w.Body.String())
	}
}
