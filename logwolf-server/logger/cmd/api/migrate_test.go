package main

import (
	"slices"
	"testing"
)

func TestDefaultProjectOwners(t *testing.T) {
	tests := []struct {
		name          string
		allowedUsers  string
		defaultOwners string
		want          []string
	}{
		{"neither set", "", "", nil},
		{"users allowlist only", "alice,bob", "", []string{"alice", "bob"}},
		// An org-only deployment: the users allowlist is empty.
		{"default owners only", "", "dave", []string{"dave"}},
		{"both, deduplicated", "alice,bob", "bob, dave", []string{"alice", "bob", "dave"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv("LOGWOLF_ALLOWED_GITHUB_USERS", tt.allowedUsers)
			t.Setenv("LOGWOLF_DEFAULT_PROJECT_OWNERS", tt.defaultOwners)

			if got := defaultProjectOwners(); !slices.Equal(got, tt.want) {
				t.Errorf("defaultProjectOwners() = %v, want %v", got, tt.want)
			}
		})
	}
}
