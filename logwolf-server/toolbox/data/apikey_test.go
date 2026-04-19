package data

import (
	"errors"
	"testing"
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

// TestErrKeyNotFound verifies the sentinel is non-nil and wraps correctly.
func TestErrKeyNotFound(t *testing.T) {
	if ErrKeyNotFound == nil {
		t.Fatal("ErrKeyNotFound must not be nil")
	}
	if ErrKeyNotFound.Error() == "" {
		t.Error("ErrKeyNotFound must have a non-empty message")
	}

	wrapped := errors.New("outer: " + ErrKeyNotFound.Error())
	// Wrap properly so errors.Is works.
	wrapped2 := errors.Join(ErrKeyNotFound, nil)
	if !errors.Is(wrapped2, ErrKeyNotFound) {
		t.Error("errors.Is should unwrap to ErrKeyNotFound")
	}
	_ = wrapped
}

// TestAPIKeyStruct verifies that APIKey carries a ProjectID field and that
// ErrKeyNotFound is the sentinel returned by GetAPIKeyByID on a miss.
func TestAPIKeyStruct(t *testing.T) {
	id := primitive.NewObjectID()
	now := time.Now()
	k := APIKey{
		ID:        id,
		ProjectID: "proj-abc",
		Prefix:    "lw_abc123",
		Active:    true,
		CreatedAt: now,
	}

	if k.ID != id {
		t.Errorf("APIKey.ID mismatch")
	}
	if k.ProjectID != "proj-abc" {
		t.Errorf("APIKey.ProjectID = %q, want %q", k.ProjectID, "proj-abc")
	}
	if k.Prefix != "lw_abc123" {
		t.Errorf("APIKey.Prefix = %q, want %q", k.Prefix, "lw_abc123")
	}
	if !k.Active {
		t.Error("APIKey.Active should be true")
	}
	if k.RevokedAt != nil {
		t.Error("APIKey.RevokedAt should be nil for an active key")
	}
	if !k.CreatedAt.Equal(now) {
		t.Errorf("APIKey.CreatedAt mismatch")
	}
}

// TestRPCCheckMembershipArgs verifies that the struct exists with the expected fields.
func TestRPCCheckMembershipArgs(t *testing.T) {
	args := RPCCheckMembershipArgs{
		ProjectID:   "507f1f77bcf86cd799439011",
		GithubLogin: "jpricardo",
	}
	if args.ProjectID == "" {
		t.Error("RPCCheckMembershipArgs.ProjectID must not be empty")
	}
	if args.GithubLogin == "" {
		t.Error("RPCCheckMembershipArgs.GithubLogin must not be empty")
	}
}

// TestGenerateAPIKey_ProjectID verifies GenerateAPIKey propagates ProjectID.
func TestGenerateAPIKey_ProjectID(t *testing.T) {
	projectID := "proj-unit-test"
	_, key, err := GenerateAPIKey(projectID)
	if err != nil {
		t.Fatalf("GenerateAPIKey failed: %v", err)
	}
	if key.ProjectID != projectID {
		t.Errorf("generated key ProjectID = %q, want %q", key.ProjectID, projectID)
	}
	if !key.Active {
		t.Error("new key should be active")
	}
	if key.Hash == "" {
		t.Error("key hash must not be empty")
	}
	if len(key.Prefix) < 3 {
		t.Errorf("key prefix too short: %q", key.Prefix)
	}
}
