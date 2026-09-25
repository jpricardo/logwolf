//go:build integration

package integration

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"testing"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"golang.org/x/crypto/bcrypt"

	"logwolf-toolbox/data"
)

// TestValidateAPIKey_ManyKeysAcrossProjects proves ValidateAPIKey resolves the
// right key, and the right project, among hundreds of active keys spread over
// several projects, without bcrypting its way through them.
//
// Every decoy carries a real DefaultCost hash, so a scan of all active keys
// costs one full bcrypt compare per decoy: far past both the time bound below
// and ValidateAPIKey's own 5s timeout. The decoys share one hash only so that
// seeding them is cheap; comparing against it costs the same either way.
func TestValidateAPIKey_ManyKeysAcrossProjects(t *testing.T) {
	const decoys = 200

	m := setupProjectModels(t)
	if err := m.EnsureAPIKeyIndexes(); err != nil {
		t.Fatalf("EnsureAPIKeyIndexes: %v", err)
	}
	db := testMongo(t, sharedModelsMongo(t)).Database("logs")
	if !hasIndex(t, db, "api_keys", "prefix") {
		t.Fatal("api_keys has no prefix index")
	}

	var projects []primitive.ObjectID
	for _, slug := range []string{"keys-alpha", "keys-beta", "keys-gamma"} {
		p, err := m.InsertProject(data.Project{Name: slug, Slug: slug})
		if err != nil {
			t.Fatalf("InsertProject %s: %v", slug, err)
		}
		projects = append(projects, p.ID)
	}

	decoyHash, err := bcrypt.GenerateFromPassword([]byte("lw_decoy"), bcrypt.DefaultCost)
	if err != nil {
		t.Fatalf("bcrypt: %v", err)
	}

	docs := make([]any, 0, decoys)
	for i := range decoys {
		docs = append(docs, bson.M{
			"project_id": projects[i%len(projects)],
			"prefix":     fmt.Sprintf("lw_d%06d", i),
			"hash":       string(decoyHash),
			"active":     true,
			"created_at": time.Now(),
		})
	}

	plaintext, target, err := data.GenerateAPIKey(projects[1], nil)
	if err != nil {
		t.Fatalf("GenerateAPIKey: %v", err)
	}
	// Two keys that share the target's prefix: an active one in another project,
	// which must be compared and passed over, and a revoked copy of the target
	// itself in a third, which must never be considered.
	docs = append(docs,
		bson.M{
			"project_id": projects[2],
			"prefix":     target.Prefix,
			"hash":       string(decoyHash),
			"active":     true,
			"created_at": time.Now(),
		},
		bson.M{
			"project_id": projects[0],
			"prefix":     target.Prefix,
			"hash":       target.Hash,
			"active":     false,
			"created_at": time.Now(),
		},
	)

	if _, err := db.Collection("api_keys").InsertMany(context.Background(), docs); err != nil {
		t.Fatalf("seed decoys: %v", err)
	}
	if err := m.SaveAPIKey(&target); err != nil {
		t.Fatalf("SaveAPIKey: %v", err)
	}

	start := time.Now()
	ok, key, err := m.ValidateAPIKey(plaintext)
	elapsed := time.Since(start)
	if err != nil {
		t.Fatalf("ValidateAPIKey: %v", err)
	}
	if !ok || key == nil {
		t.Fatal("ValidateAPIKey: valid key rejected")
	}
	if key.ProjectID != projects[1] {
		t.Errorf("resolved project = %s, want %s", key.ProjectID, projects[1])
	}
	if key.Hash != target.Hash {
		t.Error("resolved a different key than the one generated")
	}

	// A handful of compares takes a fraction of this; one per decoy, several times it.
	if elapsed > 2*time.Second {
		t.Errorf("ValidateAPIKey took %v with %d keys; it should not scale with the key count", elapsed, decoys)
	}

	// A well-formed key that matches nothing is refused just as cheaply.
	bogus := plaintext[:len(plaintext)-1] + "!"
	start = time.Now()
	ok, key, err = m.ValidateAPIKey(bogus)
	elapsed = time.Since(start)
	if err != nil {
		t.Fatalf("ValidateAPIKey(bogus): %v", err)
	}
	if ok || key != nil {
		t.Error("ValidateAPIKey accepted a key that was never issued")
	}
	if elapsed > 2*time.Second {
		t.Errorf("rejecting an unknown key took %v with %d keys", elapsed, decoys)
	}
}

// TestRevokeAPIKey_ScopedToProject verifies RevokeAPIKey matches the key's
// project as well as its id: naming the wrong project revokes nothing and is
// ErrKeyNotFound, like an id that never existed.
func TestRevokeAPIKey_ScopedToProject(t *testing.T) {
	m := setupProjectModels(t)

	owner, err := m.InsertProject(data.Project{Name: "Owner", Slug: "revoke-owner"})
	if err != nil {
		t.Fatalf("InsertProject: %v", err)
	}
	other, err := m.InsertProject(data.Project{Name: "Other", Slug: "revoke-other"})
	if err != nil {
		t.Fatalf("InsertProject: %v", err)
	}

	_, key, err := data.GenerateAPIKey(owner.ID, nil)
	if err != nil {
		t.Fatalf("GenerateAPIKey: %v", err)
	}
	if err := m.SaveAPIKey(&key); err != nil {
		t.Fatalf("SaveAPIKey: %v", err)
	}
	if key.ID.IsZero() {
		t.Fatal("SaveAPIKey left the key without the id it was stored under")
	}
	id := key.ID.Hex()

	if err := m.RevokeAPIKey(other.ID, id); !errors.Is(err, data.ErrKeyNotFound) {
		t.Fatalf("RevokeAPIKey through another project: want ErrKeyNotFound, got %v", err)
	}
	if got := findAPIKey(t, m, owner.ID, id); !got.Active {
		t.Fatalf("key after a revoke through another project: %+v; want it still active", got)
	}

	if err := m.RevokeAPIKey(owner.ID, primitive.NewObjectID().Hex()); !errors.Is(err, data.ErrKeyNotFound) {
		t.Errorf("RevokeAPIKey of an unknown id: want ErrKeyNotFound, got %v", err)
	}

	if err := m.RevokeAPIKey(owner.ID, id); err != nil {
		t.Fatalf("RevokeAPIKey: %v", err)
	}
	got := findAPIKey(t, m, owner.ID, id)
	if got.Active || got.RevokedAt == nil {
		t.Errorf("revoked key = active %v, revoked_at %v; want inactive with a time", got.Active, got.RevokedAt)
	}
}

// TestKeyRoutes_ThroughLogger drives the dashboard's key routes against the
// real stack, where the broker has no database and reaches the keys only
// through the logger: a key is minted with the id it is stored under, is used
// to send an event, can be revoked by a member of its project and by no one
// else, and lists as revoked afterwards.
func TestKeyRoutes_ThroughLogger(t *testing.T) {
	stack := sharedStack(t)
	const owner, outsider = "key-routes-owner", "key-routes-outsider"

	createProject := func(login, slug string) string {
		t.Helper()
		body := mustInternalCall(t, stack.brokerURL, http.MethodPost, "/projects", login,
			map[string]string{"name": slug, "slug": slug}, http.StatusCreated)
		var p struct {
			ID string `json:"id"`
		}
		if err := json.Unmarshal(body, &p); err != nil || p.ID == "" {
			t.Fatalf("decode created project: %v (%s)", err, body)
		}
		return p.ID
	}
	projectID := createProject(owner, "key-routes")
	createProject(outsider, "key-routes-other")

	keys := "/projects/" + projectID + "/keys"
	body := mustInternalCall(t, stack.brokerURL, http.MethodPost, keys, owner, map[string]any{}, http.StatusCreated)
	var created struct {
		Key string `json:"key"`
		ID  string `json:"id"`
	}
	if err := json.Unmarshal(body, &created); err != nil || created.Key == "" {
		t.Fatalf("decode created key: %v (%s)", err, body)
	}
	if created.ID == "" || created.ID == primitive.NilObjectID.Hex() {
		t.Fatalf("created key id = %q, want the id it was stored under", created.ID)
	}

	postLog(t, stack.brokerURL, created.Key, "key-routes-event")
	waitForLog(t, stack.mongoURI, "key-routes-event")

	if status, _ := internalCall(t, stack.brokerURL, http.MethodDelete, keys+"/"+created.ID, outsider, nil); status != http.StatusForbidden {
		t.Errorf("DELETE /projects/{id}/keys/{keyID} by a non-member = %d, want 403", status)
	}
	if status, _ := internalCall(t, stack.brokerURL, http.MethodDelete, keys+"/"+primitive.NewObjectID().Hex(), owner, nil); status != http.StatusNotFound {
		t.Errorf("DELETE /projects/{id}/keys/{keyID} of an unknown key = %d, want 404", status)
	}
	mustInternalCall(t, stack.brokerURL, http.MethodDelete, keys+"/"+created.ID, owner, nil, http.StatusOK)

	body = mustInternalCall(t, stack.brokerURL, http.MethodGet, keys, owner, nil, http.StatusOK)
	var listed []struct {
		ID     string `json:"id"`
		Active bool   `json:"active"`
	}
	if err := json.Unmarshal(body, &listed); err != nil {
		t.Fatalf("decode key list: %v (%s)", err, body)
	}
	if len(listed) != 1 || listed[0].ID != created.ID || listed[0].Active {
		t.Errorf("keys after revoke = %+v, want only %s, inactive", listed, created.ID)
	}
}

// findAPIKey reads a key of a project back by id.
func findAPIKey(t *testing.T, m data.Models, projectID primitive.ObjectID, id string) data.APIKey {
	t.Helper()
	keys, err := m.ListAPIKeysByProject(projectID)
	if err != nil {
		t.Fatalf("ListAPIKeysByProject: %v", err)
	}
	for _, k := range keys {
		if k.ID.Hex() == id {
			return k
		}
	}
	t.Fatalf("key %s not found in project %s", id, projectID.Hex())
	return data.APIKey{}
}
