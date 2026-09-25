package main

import (
	"errors"
	"net/http"
	"strings"
	"testing"

	"go.mongodb.org/mongo-driver/bson/primitive"

	"logwolf-toolbox/data"
)

// The messages below are the ones the logger actually sends: its RPC methods and
// the data layer wrap the driver's errors, so the classifier has to find the
// cause anywhere in the string.
func TestClassifyRPCError(t *testing.T) {
	cases := []struct {
		msg  string
		want rpcErrorKind
	}{
		{"InsertProjectMember: E11000 duplicate key error collection: logs.project_members index: unique_project_member", rpcErrDuplicate},
		{"UpdateProject: E11000 duplicate key error collection: logs.projects index: slug_1", rpcErrDuplicate},
		{"GetProject: mongo: no documents in result", rpcErrNotFound},
		{"UpdateProjectMemberRole: mongo: no documents in result", rpcErrNotFound},
		{"ListMembers: invalid project ID: the provided hex string is not a valid ObjectID", rpcErrNotFound},
		{data.ErrKeyNotFound.Error(), rpcErrNotFound},
		{"RemoveProjectMember: " + data.ErrLastOwner.Error(), rpcErrLastOwner},
		{"connection is shut down", rpcErrInternal},
		{"GetMetrics: context deadline exceeded", rpcErrInternal},
	}

	for _, tc := range cases {
		if got := classifyRPCError(errors.New(tc.msg)); got != tc.want {
			t.Errorf("classifyRPCError(%q) = %d, want %d", tc.msg, got, tc.want)
		}
	}
}

func TestAddProjectMember_ExistingMemberIsConflict(t *testing.T) {
	handler, _ := newInternalTestServer(t)

	// "Member-A" is stored as member-a, so it collides like the exact login does.
	for _, login := range []string{"member-a", "Member-A"} {
		w := do(handler, internalRequest(http.MethodPost, "/projects/"+projAlpha+"/members", "owner-a",
			map[string]string{"login": login, "role": data.RoleMember}))
		if w.Code != http.StatusConflict {
			t.Errorf("add %s again: got %d, want 409 (body: %s)", login, w.Code, w.Body.String())
		}
		if !strings.Contains(w.Body.String(), "already a member") {
			t.Errorf("add %s again: body %s lacks the already-a-member message", login, w.Body.String())
		}
	}
}

// The slug is fixed at creation: a rename changes the name only, and a slug in
// the body (older dashboards send one) is not passed on.
func TestUpdateProject_RenamesWithoutTouchingTheSlug(t *testing.T) {
	handler, fake := newInternalTestServer(t)

	w := do(handler, internalRequest(http.MethodPatch, "/projects/"+projAlpha, "owner-a",
		map[string]string{"name": "Alpha renamed", "slug": "beta"}))
	if w.Code != http.StatusOK {
		t.Fatalf("rename: got %d, want 200 (body: %s)", w.Code, w.Body.String())
	}
	got := decodeData[data.Project](t, w)
	if got.Name != "Alpha renamed" || got.Slug != "alpha" {
		t.Errorf("renamed project = %q (%s), want %q (alpha)", got.Name, got.Slug, "Alpha renamed")
	}

	fake.snapshot(func(f *fakeLogger) {
		want := data.RPCUpdateProjectArgs{ID: projAlpha, Name: "Alpha renamed"}
		if len(f.updatedProjects) != 1 || f.updatedProjects[0] != want {
			t.Errorf("UpdateProject forwarded %+v, want [%+v]", f.updatedProjects, want)
		}
	})
}

func TestUpdateRetention_RejectsInvalidDays(t *testing.T) {
	cases := []struct {
		name string
		body map[string]any
	}{
		{"unsupported value", map[string]any{"days": 7}},
		{"negative", map[string]any{"days": -1}},
		// Would otherwise decode as 0 and keep logs forever.
		{"missing", map[string]any{}},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			handler, fake := newInternalTestServer(t)

			w := do(handler, internalRequest(http.MethodPatch, "/projects/"+projAlpha+"/retention", "member-a", tc.body))
			if w.Code != http.StatusBadRequest {
				t.Errorf("got %d, want 400 (body: %s)", w.Code, w.Body.String())
			}
			fake.snapshot(func(f *fakeLogger) {
				if len(f.retentionArgs) != 0 {
					t.Errorf("UpdateRetention forwarded: %+v", f.retentionArgs)
				}
			})
		})
	}

	// 0 is a real choice, forever, not a missing value.
	handler, _ := newInternalTestServer(t)
	w := do(handler, internalRequest(http.MethodPatch, "/projects/"+projAlpha+"/retention", "member-a",
		map[string]any{"days": 0}))
	if w.Code != http.StatusOK {
		t.Errorf("days 0: got %d, want 200 (body: %s)", w.Code, w.Body.String())
	}
}

// A malformed id cannot name a project, so it is as missing as a well-formed
// one that names nothing.
func TestProjectRoutes_MalformedIDIsNotFound(t *testing.T) {
	handler, _ := newInternalTestServer(t)

	const bad = "not-an-object-id"
	cases := []struct {
		method, target string
		body           any
	}{
		{http.MethodGet, "/projects/" + bad, nil},
		{http.MethodPatch, "/projects/" + bad, map[string]string{"name": "X", "slug": "x"}},
		{http.MethodDelete, "/projects/" + bad, nil},
		{http.MethodGet, "/projects/" + bad + "/members", nil},
		{http.MethodPost, "/projects/" + bad + "/members", map[string]string{"login": "someone", "role": data.RoleMember}},
		{http.MethodPatch, "/projects/" + bad + "/members/member-a", map[string]string{"role": data.RoleOwner}},
		{http.MethodDelete, "/projects/" + bad + "/members/member-a", nil},
		{http.MethodGet, "/projects/" + bad + "/logs", nil},
		{http.MethodPost, "/projects/" + bad + "/logs", map[string]string{"name": "e", "data": "{}", "severity": "info"}},
		{http.MethodGet, "/projects/" + bad + "/logs/" + alphaLogID, nil},
		{http.MethodDelete, "/projects/" + bad + "/logs/" + alphaLogID, nil},
		{http.MethodGet, "/projects/" + bad + "/keys", nil},
		{http.MethodPost, "/projects/" + bad + "/keys", map[string]any{}},
		{http.MethodDelete, "/projects/" + bad + "/keys/" + primitive.NewObjectID().Hex(), nil},
		{http.MethodGet, "/projects/" + bad + "/retention", nil},
		{http.MethodPatch, "/projects/" + bad + "/retention", map[string]any{"days": 30}},
		{http.MethodGet, "/projects/" + bad + "/metrics", nil},
		// A malformed key id in a real project names no key either.
		{http.MethodDelete, "/projects/" + projAlpha + "/keys/" + bad, nil},
	}

	for _, tc := range cases {
		w := do(handler, internalRequest(tc.method, tc.target, "owner-a", tc.body))
		if w.Code != http.StatusNotFound {
			t.Errorf("%s %s: got %d, want 404 (body: %s)", tc.method, tc.target, w.Code, w.Body.String())
		}
	}
}

func TestRemoveProjectMember_UnknownMemberIsNotFound(t *testing.T) {
	handler, _ := newInternalTestServer(t)

	w := do(handler, internalRequest(http.MethodDelete, "/projects/"+projAlpha+"/members/ghost", "owner-a", nil))
	if w.Code != http.StatusNotFound {
		t.Errorf("remove a non-member: got %d, want 404 (body: %s)", w.Code, w.Body.String())
	}
}
