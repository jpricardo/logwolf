package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	"go.mongodb.org/mongo-driver/bson/primitive"

	"logwolf-toolbox/data"
)

// Project ids have to be real ObjectID hex: the logger parses them, and the
// broker forwards whatever the path carried without touching it.
const (
	projAlpha   = "aaaaaaaaaaaaaaaaaaaaaaa1"
	projBeta    = "bbbbbbbbbbbbbbbbbbbbbbb2"
	projMissing = "ccccccccccccccccccccccc3"

	alphaLogID = "1111111111111111111111a1"
	betaLogID  = "2222222222222222222222b2"

	internalSecret = "test-internal-secret"
)

var projectIDSeq atomic.Int64

func nextProjectID() string {
	return fmt.Sprintf("dddddddddddddddddddd%04d", projectIDSeq.Add(1))
}

func mustObjectID(hex string) primitive.ObjectID {
	id, err := primitive.ObjectIDFromHex(hex)
	if err != nil {
		panic("test fixture: invalid ObjectID hex " + hex + ": " + err.Error())
	}
	return id
}

// newInternalTestServer wires the real router to a fake logger and seeds two
// projects: alpha (owner-a, member-a) and beta (owner-b). Nobody belongs to
// both, so any cross-project read that succeeds is an isolation failure.
func newInternalTestServer(t *testing.T) (http.Handler, *fakeLogger) {
	t.Helper()

	f := newFakeLogger()
	f.addProject(projAlpha, "Alpha", "alpha")
	f.addProject(projBeta, "Beta", "beta")
	f.addMember(projAlpha, "owner-a", data.RoleOwner)
	f.addMember(projAlpha, "member-a", data.RoleMember)
	f.addMember(projBeta, "owner-b", data.RoleOwner)
	f.addLog(projAlpha, alphaLogID, "alpha-event")
	f.addLog(projBeta, betaLogID, "beta-event")

	serveFakeLogger(t, f)
	t.Setenv("INTERNAL_API_SECRET", internalSecret)

	app := &Config{}
	return app.routes(), f
}

// internalRequest builds a request carrying both headers the internal routes
// demand. body may be nil.
func internalRequest(method, target, userLogin string, body any) *http.Request {
	var r *http.Request
	if body == nil {
		r = httptest.NewRequest(method, target, nil)
	} else {
		b, _ := json.Marshal(body)
		r = httptest.NewRequest(method, target, bytes.NewReader(b))
		r.Header.Set("Content-Type", "application/json")
	}
	r.Header.Set("X-Internal-Secret", internalSecret)
	if userLogin != "" {
		r.Header.Set("X-User-Login", userLogin)
	}
	return r
}

func do(h http.Handler, r *http.Request) *httptest.ResponseRecorder {
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	return w
}

// decodeData pulls the "data" member out of the standard response envelope.
func decodeData[T any](t *testing.T, w *httptest.ResponseRecorder) T {
	t.Helper()
	var envelope struct {
		Error   bool            `json:"error"`
		Message string          `json:"message"`
		Data    json.RawMessage `json:"data"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &envelope); err != nil {
		t.Fatalf("decode envelope: %v (body: %s)", err, w.Body.String())
	}
	var out T
	if len(envelope.Data) > 0 {
		if err := json.Unmarshal(envelope.Data, &out); err != nil {
			t.Fatalf("decode data: %v (body: %s)", err, w.Body.String())
		}
	}
	return out
}

// --- requireInternalSecret ---

func TestInternalRoutes_RejectMissingOrWrongSecret(t *testing.T) {
	handler, _ := newInternalTestServer(t)

	routes := []struct{ method, target string }{
		{http.MethodGet, "/projects/" + projAlpha + "/keys"},
		{http.MethodGet, "/projects/" + projAlpha + "/retention"},
		{http.MethodGet, "/projects/" + projAlpha + "/metrics"},
		{http.MethodGet, "/projects"},
		{http.MethodGet, "/projects/" + projAlpha},
		{http.MethodDelete, "/projects/" + projAlpha},
		{http.MethodGet, "/projects/" + projAlpha + "/members"},
		{http.MethodGet, "/projects/" + projAlpha + "/logs"},
		{http.MethodGet, "/projects/" + projAlpha + "/logs/" + alphaLogID},
	}

	for _, route := range routes {
		for _, secret := range []string{"", "wrong-secret"} {
			r := httptest.NewRequest(route.method, route.target, nil)
			if secret != "" {
				r.Header.Set("X-Internal-Secret", secret)
			}
			r.Header.Set("X-User-Login", "owner-a")

			if w := do(handler, r); w.Code != http.StatusUnauthorized {
				t.Errorf("%s %s with secret %q: got %d, want 401", route.method, route.target, secret, w.Code)
			}
		}
	}
}

func TestInternalRoutes_SecretUnsetRejectsEverything(t *testing.T) {
	handler, _ := newInternalTestServer(t)
	// An unset server-side secret must fail closed rather than match the
	// empty header a caller can trivially send.
	t.Setenv("INTERNAL_API_SECRET", "")

	r := httptest.NewRequest(http.MethodGet, "/projects", nil)
	r.Header.Set("X-User-Login", "owner-a")

	if w := do(handler, r); w.Code != http.StatusUnauthorized {
		t.Errorf("unset INTERNAL_API_SECRET: got %d, want 401", w.Code)
	}
}

// --- requireUserLogin ---

func TestInternalRoutes_RejectMissingUserLogin(t *testing.T) {
	handler, _ := newInternalTestServer(t)

	routes := []struct{ method, target string }{
		{http.MethodGet, "/projects/" + projAlpha + "/keys"},
		{http.MethodPost, "/projects/" + projAlpha + "/keys"},
		{http.MethodGet, "/projects/" + projAlpha + "/retention"},
		{http.MethodPatch, "/projects/" + projAlpha + "/retention"},
		{http.MethodGet, "/projects/" + projAlpha + "/metrics"},
		{http.MethodGet, "/projects"},
		{http.MethodPost, "/projects"},
		{http.MethodGet, "/projects/" + projAlpha},
		{http.MethodPatch, "/projects/" + projAlpha},
		{http.MethodDelete, "/projects/" + projAlpha},
		{http.MethodGet, "/projects/" + projAlpha + "/members"},
		{http.MethodPost, "/projects/" + projAlpha + "/members"},
		{http.MethodDelete, "/projects/" + projAlpha + "/members/member-a"},
		{http.MethodGet, "/projects/" + projAlpha + "/logs"},
		{http.MethodPost, "/projects/" + projAlpha + "/logs"},
		{http.MethodGet, "/projects/" + projAlpha + "/logs/" + alphaLogID},
		{http.MethodDelete, "/projects/" + projAlpha + "/logs/" + alphaLogID},
	}

	for _, route := range routes {
		r := internalRequest(route.method, route.target, "", nil)

		if w := do(handler, r); w.Code != http.StatusUnauthorized {
			t.Errorf("%s %s without X-User-Login: got %d, want 401", route.method, route.target, w.Code)
		}
	}
}

// --- membership enforcement ---

func TestProjectRoutes_MembershipEnforced(t *testing.T) {
	handler, _ := newInternalTestServer(t)

	cases := []struct {
		name       string
		method     string
		target     string
		user       string
		body       any
		wantStatus int
	}{
		{"member reads own project", http.MethodGet, "/projects/" + projAlpha, "member-a", nil, http.StatusOK},
		{"owner reads own project", http.MethodGet, "/projects/" + projAlpha, "owner-a", nil, http.StatusOK},
		{"outsider reads project", http.MethodGet, "/projects/" + projAlpha, "stranger", nil, http.StatusForbidden},
		{"other project's owner reads project", http.MethodGet, "/projects/" + projAlpha, "owner-b", nil, http.StatusForbidden},
		{"unknown project is 404 not 403", http.MethodGet, "/projects/" + projMissing, "owner-a", nil, http.StatusNotFound},

		{"member lists members", http.MethodGet, "/projects/" + projAlpha + "/members", "member-a", nil, http.StatusOK},
		{"outsider lists members", http.MethodGet, "/projects/" + projAlpha + "/members", "owner-b", nil, http.StatusForbidden},

		{"member lists logs", http.MethodGet, "/projects/" + projAlpha + "/logs", "member-a", nil, http.StatusOK},
		{"outsider lists logs", http.MethodGet, "/projects/" + projAlpha + "/logs", "owner-b", nil, http.StatusForbidden},
		{"outsider reads one log", http.MethodGet, "/projects/" + projAlpha + "/logs/" + alphaLogID, "owner-b", nil, http.StatusForbidden},
		{"outsider deletes one log", http.MethodDelete, "/projects/" + projAlpha + "/logs/" + alphaLogID, "owner-b", nil, http.StatusForbidden},
		// Rabbit is nil here, so anything past the membership check would panic
		// rather than return 403 — reaching 403 is the assertion.
		{"outsider writes a log", http.MethodPost, "/projects/" + projAlpha + "/logs", "owner-b", map[string]string{"name": "x"}, http.StatusForbidden},

		{"outsider reads retention", http.MethodGet, "/projects/" + projAlpha + "/retention", "owner-b", nil, http.StatusForbidden},
		{"member reads retention", http.MethodGet, "/projects/" + projAlpha + "/retention", "member-a", nil, http.StatusOK},
		{"outsider writes retention", http.MethodPatch, "/projects/" + projAlpha + "/retention", "owner-b",
			map[string]any{"days": 30}, http.StatusForbidden},
		// Raising it from the default 90; lowering is covered in TestUpdateRetention_OnlyOwnersLowerIt.
		{"member writes retention", http.MethodPatch, "/projects/" + projAlpha + "/retention", "member-a",
			map[string]any{"days": 180}, http.StatusOK},

		{"outsider reads metrics", http.MethodGet, "/projects/" + projAlpha + "/metrics", "owner-b", nil, http.StatusForbidden},
		{"member reads metrics", http.MethodGet, "/projects/" + projAlpha + "/metrics", "member-a", nil, http.StatusOK},

		{"outsider lists keys", http.MethodGet, "/projects/" + projAlpha + "/keys", "owner-b", nil, http.StatusForbidden},
		{"outsider creates key", http.MethodPost, "/projects/" + projAlpha + "/keys", "owner-b", map[string]string{}, http.StatusForbidden},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			w := do(handler, internalRequest(tc.method, tc.target, tc.user, tc.body))
			if w.Code != tc.wantStatus {
				t.Errorf("%s %s as %q: got %d, want %d (body: %s)",
					tc.method, tc.target, tc.user, w.Code, tc.wantStatus, w.Body.String())
			}
		})
	}
}

// --- owner-only operations ---

func TestProjectRoutes_OwnerOnly(t *testing.T) {
	cases := []struct {
		name   string
		method string
		target string
		body   any
	}{
		{"rename project", http.MethodPatch, "/projects/" + projAlpha, map[string]string{"name": "Renamed", "slug": "renamed"}},
		{"delete project", http.MethodDelete, "/projects/" + projAlpha, nil},
		{"add member", http.MethodPost, "/projects/" + projAlpha + "/members", map[string]string{"login": "newbie", "role": data.RoleMember}},
		{"remove member", http.MethodDelete, "/projects/" + projAlpha + "/members/member-a", nil},
		{"change member role", http.MethodPatch, "/projects/" + projAlpha + "/members/member-a", map[string]string{"role": data.RoleOwner}},
	}

	for _, tc := range cases {
		t.Run(tc.name+" as member", func(t *testing.T) {
			handler, _ := newInternalTestServer(t)
			w := do(handler, internalRequest(tc.method, tc.target, "member-a", tc.body))
			if w.Code != http.StatusForbidden {
				t.Errorf("%s %s as member: got %d, want 403 (body: %s)", tc.method, tc.target, w.Code, w.Body.String())
			}
		})

		t.Run(tc.name+" as outsider", func(t *testing.T) {
			handler, _ := newInternalTestServer(t)
			w := do(handler, internalRequest(tc.method, tc.target, "owner-b", tc.body))
			if w.Code != http.StatusForbidden {
				t.Errorf("%s %s as outsider: got %d, want 403 (body: %s)", tc.method, tc.target, w.Code, w.Body.String())
			}
		})

		t.Run(tc.name+" as owner", func(t *testing.T) {
			handler, _ := newInternalTestServer(t)
			w := do(handler, internalRequest(tc.method, tc.target, "owner-a", tc.body))
			if w.Code >= http.StatusBadRequest {
				t.Errorf("%s %s as owner: got %d, want success (body: %s)", tc.method, tc.target, w.Code, w.Body.String())
			}
		})
	}
}

// A member's failed write must not reach the logger at all — a 403 that still
// forwarded the call would leave the project modified.
func TestProjectRoutes_OwnerOnlyDeniesBeforeForwarding(t *testing.T) {
	handler, fake := newInternalTestServer(t)

	do(handler, internalRequest(http.MethodDelete, "/projects/"+projAlpha, "member-a", nil))
	do(handler, internalRequest(http.MethodPatch, "/projects/"+projAlpha, "member-a",
		map[string]string{"name": "Renamed", "slug": "renamed"}))
	do(handler, internalRequest(http.MethodPost, "/projects/"+projAlpha+"/members", "member-a",
		map[string]string{"login": "newbie", "role": data.RoleMember}))
	do(handler, internalRequest(http.MethodDelete, "/projects/"+projAlpha+"/members/member-a", "member-a", nil))
	do(handler, internalRequest(http.MethodPatch, "/projects/"+projAlpha+"/members/member-a", "member-a",
		map[string]string{"role": data.RoleOwner}))

	fake.snapshot(func(f *fakeLogger) {
		if len(f.deletedProjects) != 0 {
			t.Errorf("DeleteProject forwarded for a non-owner: %v", f.deletedProjects)
		}
		if len(f.updatedProjects) != 0 {
			t.Errorf("UpdateProject forwarded for a non-owner: %v", f.updatedProjects)
		}
		if len(f.addedMembers) != 0 {
			t.Errorf("AddMember forwarded for a non-owner: %v", f.addedMembers)
		}
		if len(f.removedMembers) != 0 {
			t.Errorf("RemoveMember forwarded for a non-owner: %v", f.removedMembers)
		}
		if len(f.roleChanges) != 0 {
			t.Errorf("UpdateMemberRole forwarded for a non-owner: %v", f.roleChanges)
		}
	})
}

func TestRemoveProjectMember_LastOwnerIsBadRequest(t *testing.T) {
	handler, fake := newInternalTestServer(t)
	fake.lastOwnerLogin = "owner-a"

	w := do(handler, internalRequest(http.MethodDelete, "/projects/"+projAlpha+"/members/owner-a", "owner-a", nil))
	if w.Code != http.StatusBadRequest {
		t.Fatalf("removing the last owner: got %d, want 400 (body: %s)", w.Code, w.Body.String())
	}
}

// --- member roles ---

func TestUpdateProjectMemberRole_PromoteAndDemote(t *testing.T) {
	handler, fake := newInternalTestServer(t)

	w := do(handler, internalRequest(http.MethodPatch, "/projects/"+projAlpha+"/members/member-a", "owner-a",
		map[string]string{"role": data.RoleOwner}))
	if w.Code != http.StatusOK {
		t.Fatalf("promote: got %d, want 200 (body: %s)", w.Code, w.Body.String())
	}

	// Now that member-a is an owner too, owner-a may step down: the two-step
	// ownership transfer the endpoint exists for.
	w = do(handler, internalRequest(http.MethodPatch, "/projects/"+projAlpha+"/members/owner-a", "owner-a",
		map[string]string{"role": data.RoleMember}))
	if w.Code != http.StatusOK {
		t.Fatalf("demote self: got %d, want 200 (body: %s)", w.Code, w.Body.String())
	}

	fake.snapshot(func(f *fakeLogger) {
		want := []data.RPCUpdateMemberRoleArgs{
			{ProjectID: projAlpha, GithubLogin: "member-a", Role: data.RoleOwner},
			{ProjectID: projAlpha, GithubLogin: "owner-a", Role: data.RoleMember},
		}
		if len(f.roleChanges) != len(want) {
			t.Fatalf("UpdateMemberRole calls = %v, want %v", f.roleChanges, want)
		}
		for i := range want {
			if f.roleChanges[i] != want[i] {
				t.Errorf("UpdateMemberRole call %d = %+v, want %+v", i, f.roleChanges[i], want[i])
			}
		}
	})
}

// The login in the path reaches the logger normalized, as it does for removal:
// memberships are stored lowercase, so "Member-A" has to find "member-a".
func TestUpdateProjectMemberRole_NormalizesLogin(t *testing.T) {
	handler, fake := newInternalTestServer(t)

	w := do(handler, internalRequest(http.MethodPatch, "/projects/"+projAlpha+"/members/Member-A", "owner-a",
		map[string]string{"role": data.RoleOwner}))
	if w.Code != http.StatusOK {
		t.Fatalf("promote Member-A: got %d, want 200 (body: %s)", w.Code, w.Body.String())
	}

	fake.snapshot(func(f *fakeLogger) {
		if len(f.roleChanges) != 1 || f.roleChanges[0].GithubLogin != "member-a" {
			t.Errorf("UpdateMemberRole calls = %+v, want one for member-a", f.roleChanges)
		}
	})
}

func TestUpdateProjectMemberRole_LastOwnerIsBadRequest(t *testing.T) {
	handler, fake := newInternalTestServer(t)
	fake.lastOwnerLogin = "owner-a"

	w := do(handler, internalRequest(http.MethodPatch, "/projects/"+projAlpha+"/members/owner-a", "owner-a",
		map[string]string{"role": data.RoleMember}))
	if w.Code != http.StatusBadRequest {
		t.Fatalf("demoting the last owner: got %d, want 400 (body: %s)", w.Code, w.Body.String())
	}
	if !strings.Contains(w.Body.String(), "cannot demote the last owner") {
		t.Errorf("demoting the last owner: body %s lacks the last-owner message", w.Body.String())
	}
}

func TestUpdateProjectMemberRole_InvalidRole(t *testing.T) {
	for _, body := range []any{map[string]string{"role": "admin"}, map[string]string{}} {
		handler, fake := newInternalTestServer(t)

		w := do(handler, internalRequest(http.MethodPatch, "/projects/"+projAlpha+"/members/member-a", "owner-a", body))
		if w.Code != http.StatusBadRequest {
			t.Errorf("role %v: got %d, want 400 (body: %s)", body, w.Code, w.Body.String())
		}
		fake.snapshot(func(f *fakeLogger) {
			if len(f.roleChanges) != 0 {
				t.Errorf("role %v: UpdateMemberRole forwarded: %v", body, f.roleChanges)
			}
		})
	}
}

func TestUpdateProjectMemberRole_UnknownMemberIsNotFound(t *testing.T) {
	handler, _ := newInternalTestServer(t)

	w := do(handler, internalRequest(http.MethodPatch, "/projects/"+projAlpha+"/members/ghost", "owner-a",
		map[string]string{"role": data.RoleOwner}))
	if w.Code != http.StatusNotFound {
		t.Errorf("promote a non-member: got %d, want 404 (body: %s)", w.Code, w.Body.String())
	}
}

// --- project-scoped event access ---

func TestProjectLogs_ScopedToPathProject(t *testing.T) {
	handler, fake := newInternalTestServer(t)

	w := do(handler, internalRequest(http.MethodGet, "/projects/"+projAlpha+"/logs", "member-a", nil))
	if w.Code != http.StatusOK {
		t.Fatalf("list logs: got %d, want 200 (body: %s)", w.Code, w.Body.String())
	}

	entries := decodeData[[]data.LogEntry](t, w)
	for _, e := range entries {
		if e.ProjectID.Hex() != projAlpha {
			t.Errorf("list logs returned an entry from project %s", e.ProjectID.Hex())
		}
		if e.Name == "beta-event" {
			t.Error("list logs leaked project beta's event")
		}
	}

	// The query the broker forwarded must be scoped to the path's project — the
	// dashboard never gets to choose it.
	fake.snapshot(func(f *fakeLogger) {
		if len(f.getLogsParams) != 1 {
			t.Fatalf("GetLogs called %d times, want 1", len(f.getLogsParams))
		}
		if f.getLogsParams[0].ProjectID != projAlpha {
			t.Errorf("GetLogs ProjectID = %q, want %q", f.getLogsParams[0].ProjectID, projAlpha)
		}
	})
}

func TestProjectLogs_CrossProjectLogIDIsNotFound(t *testing.T) {
	handler, fake := newInternalTestServer(t)

	// owner-a knows beta's log id but has no access to it: the id names nothing
	// inside alpha, so the answer is 404, not somebody else's event.
	w := do(handler, internalRequest(http.MethodGet, "/projects/"+projAlpha+"/logs/"+betaLogID, "owner-a", nil))
	if w.Code != http.StatusNotFound {
		t.Errorf("read beta's log through alpha: got %d, want 404 (body: %s)", w.Code, w.Body.String())
	}

	w = do(handler, internalRequest(http.MethodDelete, "/projects/"+projAlpha+"/logs/"+betaLogID, "owner-a", nil))
	if w.Code != http.StatusNotFound {
		t.Errorf("delete beta's log through alpha: got %d, want 404 (body: %s)", w.Code, w.Body.String())
	}

	fake.snapshot(func(f *fakeLogger) {
		if len(f.logs[projBeta]) != 1 {
			t.Errorf("project beta lost %d log(s) to a cross-project delete", 1-len(f.logs[projBeta]))
		}
	})
}

func TestProjectLogs_OwnLogIsReadableAndDeletable(t *testing.T) {
	handler, _ := newInternalTestServer(t)

	w := do(handler, internalRequest(http.MethodGet, "/projects/"+projAlpha+"/logs/"+alphaLogID, "member-a", nil))
	if w.Code != http.StatusOK {
		t.Fatalf("read own log: got %d, want 200 (body: %s)", w.Code, w.Body.String())
	}
	entry := decodeData[data.LogEntry](t, w)
	if entry.ProjectID.Hex() != projAlpha {
		t.Errorf("entry ProjectID = %s, want %s", entry.ProjectID.Hex(), projAlpha)
	}

	w = do(handler, internalRequest(http.MethodDelete, "/projects/"+projAlpha+"/logs/"+alphaLogID, "member-a", nil))
	if w.Code != http.StatusOK {
		t.Fatalf("delete own log: got %d, want 200 (body: %s)", w.Code, w.Body.String())
	}

	// Deleting it twice is a miss, not a second success.
	w = do(handler, internalRequest(http.MethodDelete, "/projects/"+projAlpha+"/logs/"+alphaLogID, "member-a", nil))
	if w.Code != http.StatusNotFound {
		t.Errorf("delete the same log again: got %d, want 404", w.Code)
	}
}

// --- project listing and creation ---

func TestListProjects_ScopedToCaller(t *testing.T) {
	handler, _ := newInternalTestServer(t)

	w := do(handler, internalRequest(http.MethodGet, "/projects", "member-a", nil))
	if w.Code != http.StatusOK {
		t.Fatalf("list projects: got %d, want 200", w.Code)
	}

	projects := decodeData[[]data.UserProject](t, w)
	if len(projects) != 1 {
		t.Fatalf("member-a sees %d project(s), want 1: %+v", len(projects), projects)
	}
	if projects[0].Slug != "alpha" {
		t.Errorf("member-a sees project %q, want alpha", projects[0].Slug)
	}
	if projects[0].Role != data.RoleMember {
		t.Errorf("member-a role = %q, want %q", projects[0].Role, data.RoleMember)
	}

	// A user with no memberships gets an empty list, not everyone's projects.
	w = do(handler, internalRequest(http.MethodGet, "/projects", "stranger", nil))
	if got := decodeData[[]data.UserProject](t, w); len(got) != 0 {
		t.Errorf("stranger sees %d project(s), want 0: %+v", len(got), got)
	}
}

func TestCreateProject_MakesCallerTheOwner(t *testing.T) {
	handler, fake := newInternalTestServer(t)

	w := do(handler, internalRequest(http.MethodPost, "/projects", "newcomer",
		map[string]string{"name": "Fresh", "slug": "fresh"}))
	if w.Code != http.StatusCreated {
		t.Fatalf("create project: got %d, want 201 (body: %s)", w.Code, w.Body.String())
	}
	created := decodeData[data.Project](t, w)

	// The owner travels with the project in one call, which the logger writes
	// in one transaction; the broker attaches no member on its own.
	fake.snapshot(func(f *fakeLogger) {
		if len(f.createdProjects) != 1 {
			t.Fatalf("CreateProject called %d times, want 1", len(f.createdProjects))
		}
		if got := f.createdProjects[0].Owner; got != "newcomer" {
			t.Errorf("owner login = %q, want %q", got, "newcomer")
		}
		if len(f.addedMembers) != 0 {
			t.Errorf("AddMember called %d times, want 0: the owner is created with the project", len(f.addedMembers))
		}
		members := f.members[created.ID.Hex()]
		if len(members) != 1 || members[0].GithubLogin != "newcomer" || members[0].Role != data.RoleOwner {
			t.Errorf("members of the new project = %+v, want newcomer as its only owner", members)
		}
	})
}

func TestCreateProject_RejectsInvalidInput(t *testing.T) {
	handler, _ := newInternalTestServer(t)

	cases := []struct {
		name string
		body map[string]string
	}{
		{"missing name", map[string]string{"slug": "ok-slug"}},
		{"invalid slug", map[string]string{"name": "X", "slug": "Not A Slug"}},
		{"missing slug", map[string]string{"name": "X"}},
	}

	for _, tc := range cases {
		w := do(handler, internalRequest(http.MethodPost, "/projects", "newcomer", tc.body))
		if w.Code != http.StatusBadRequest {
			t.Errorf("%s: got %d, want 400", tc.name, w.Code)
		}
	}
}

// A failed create leaves nothing behind in the logger — its transaction sees to
// that — so the broker has nothing to roll back and only reports the failure.
func TestCreateProject_FailureIsServerError(t *testing.T) {
	handler, fake := newInternalTestServer(t)
	fake.failCreateProject = true

	w := do(handler, internalRequest(http.MethodPost, "/projects", "newcomer",
		map[string]string{"name": "Orphan", "slug": "orphan"}))
	// Nothing the caller sent was wrong, so this is the server's failure.
	if w.Code != http.StatusInternalServerError {
		t.Fatalf("create project with a failing logger: got %d, want 500", w.Code)
	}

	fake.snapshot(func(f *fakeLogger) {
		if len(f.addedMembers) != 0 || len(f.deletedProjects) != 0 {
			t.Errorf("broker made follow-up calls after a failed create: added=%v deleted=%v", f.addedMembers, f.deletedProjects)
		}
	})
}

// --- retention: members may raise it, only owners lower it ---

func TestUpdateRetention_OnlyOwnersLowerIt(t *testing.T) {
	// Each step runs against the retention the previous one left; alpha starts
	// on the default 90 days.
	steps := []struct {
		name       string
		user       string
		days       int
		wantStatus int
		wantDays   int
	}{
		{"member raises 90 to 180", "member-a", 180, http.StatusOK, 180},
		{"member lowers 180 to 30", "member-a", 30, http.StatusForbidden, 180},
		{"member keeps 180", "member-a", 180, http.StatusOK, 180},
		{"member raises 180 to forever", "member-a", 0, http.StatusOK, 0},
		{"member lowers forever to 365", "member-a", 365, http.StatusForbidden, 0},
		{"owner lowers forever to 60", "owner-a", 60, http.StatusOK, 60},
		{"owner lowers 60 to 30", "owner-a", 30, http.StatusOK, 30},
		{"member raises 30 to 60", "member-a", 60, http.StatusOK, 60},
		{"outsider raises 60 to 365", "owner-b", 365, http.StatusForbidden, 60},
	}

	handler, fake := newInternalTestServer(t)
	for _, step := range steps {
		w := do(handler, internalRequest(http.MethodPatch, "/projects/"+projAlpha+"/retention", step.user,
			map[string]any{"days": step.days}))
		if w.Code != step.wantStatus {
			t.Fatalf("%s: got %d, want %d (body: %s)", step.name, w.Code, step.wantStatus, w.Body.String())
		}
		fake.snapshot(func(f *fakeLogger) {
			if got, ok := f.retention[projAlpha]; !ok || got != step.wantDays {
				t.Fatalf("%s: retention is %d (set: %v), want %d", step.name, got, ok, step.wantDays)
			}
		})
	}
}

func TestUpdateRetention_UnknownProjectIsNotFound(t *testing.T) {
	handler, _ := newInternalTestServer(t)

	w := do(handler, internalRequest(http.MethodPatch, "/projects/"+projMissing+"/retention", "member-a",
		map[string]any{"days": 30}))
	if w.Code != http.StatusNotFound {
		t.Errorf("got %d, want 404 (body: %s)", w.Code, w.Body.String())
	}
}

// --- retention and metrics carry the project through ---

func TestRetentionAndMetrics_ForwardTheProjectID(t *testing.T) {
	handler, fake := newInternalTestServer(t)

	if w := do(handler, internalRequest(http.MethodPatch, "/projects/"+projAlpha+"/retention", "owner-a",
		map[string]any{"days": 30})); w.Code != http.StatusOK {
		t.Fatalf("update retention: got %d (body: %s)", w.Code, w.Body.String())
	}
	if w := do(handler, internalRequest(http.MethodGet, "/projects/"+projAlpha+"/metrics", "member-a", nil)); w.Code != http.StatusOK {
		t.Fatalf("get metrics: got %d (body: %s)", w.Code, w.Body.String())
	}

	fake.snapshot(func(f *fakeLogger) {
		for _, args := range f.retentionArgs {
			if args.ProjectID != projAlpha {
				t.Errorf("retention call scoped to %q, want %q", args.ProjectID, projAlpha)
			}
		}
		if len(f.retentionArgs) == 0 || f.retentionArgs[len(f.retentionArgs)-1].Days != 30 {
			t.Errorf("retention days not forwarded: %+v", f.retentionArgs)
		}
		for _, args := range f.metricsArgs {
			if args.ProjectID != projAlpha {
				t.Errorf("metrics call scoped to %q, want %q", args.ProjectID, projAlpha)
			}
		}
	})
}
