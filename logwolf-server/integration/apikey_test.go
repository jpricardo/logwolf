//go:build integration

package integration

import (
	"context"
	"fmt"
	"testing"
	"time"

	"go.mongodb.org/mongo-driver/bson"
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

	var projects []string
	for _, slug := range []string{"keys-alpha", "keys-beta", "keys-gamma"} {
		p, err := m.InsertProject(data.Project{Name: slug, Slug: slug})
		if err != nil {
			t.Fatalf("InsertProject %s: %v", slug, err)
		}
		projects = append(projects, p.ID.Hex())
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
	if err := m.SaveAPIKey(target); err != nil {
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
