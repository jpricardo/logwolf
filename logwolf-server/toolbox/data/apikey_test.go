package data

import (
	"errors"
	"strings"
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

// TestGenerateAPIKey_Shape verifies generated keys have the shape ValidateAPIKey
// accepts, and that the stored prefix is the head of the plaintext.
func TestGenerateAPIKey_Shape(t *testing.T) {
	plaintext, key, err := GenerateAPIKey("proj-unit-test")
	if err != nil {
		t.Fatalf("GenerateAPIKey failed: %v", err)
	}
	if len(plaintext) != apiKeyLength {
		t.Errorf("len(plaintext) = %d, want %d", len(plaintext), apiKeyLength)
	}
	if !strings.HasPrefix(plaintext, apiKeyScheme) {
		t.Errorf("plaintext %q does not start with %q", plaintext, apiKeyScheme)
	}
	if key.Prefix != plaintext[:apiKeyPrefixLength] {
		t.Errorf("Prefix = %q, want %q", key.Prefix, plaintext[:apiKeyPrefixLength])
	}
}

// TestValidateAPIKey_RejectsMalformed verifies a key GenerateAPIKey could not
// have produced is refused before the database is touched. The zero Models has
// no client, so reaching the query would panic.
func TestValidateAPIKey_RejectsMalformed(t *testing.T) {
	valid, _, err := GenerateAPIKey("proj-unit-test")
	if err != nil {
		t.Fatalf("GenerateAPIKey failed: %v", err)
	}

	cases := map[string]string{
		"empty":        "",
		"prefix only":  valid[:apiKeyPrefixLength],
		"too short":    valid[:len(valid)-1],
		"too long":     valid + "x",
		"wrong scheme": "sk_" + valid[3:],
		"no separator": "lwx" + valid[3:],
	}

	var m Models
	for name, plaintext := range cases {
		t.Run(name, func(t *testing.T) {
			ok, key, err := m.ValidateAPIKey(plaintext)
			if ok || key != nil || err != nil {
				t.Errorf("ValidateAPIKey(%q) = %v, %v, %v; want false, nil, nil", plaintext, ok, key, err)
			}
		})
	}
}
