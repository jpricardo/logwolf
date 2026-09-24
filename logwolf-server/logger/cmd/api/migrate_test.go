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

func TestMongoCredential(t *testing.T) {
	cases := []struct {
		name, user, pass string
		want             bool // a credential is set
		wantErr          bool
	}{
		{"neither: MONGO_URL's credentials apply", "", "", false, false},
		{"both", "logwolf", "s3cret", true, false},
		{"username alone", "logwolf", "", false, true},
		{"password alone", "", "s3cret", false, true},
	}

	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv("MONGO_USERNAME", tt.user)
			t.Setenv("MONGO_PASSWORD", tt.pass)

			cred, err := mongoCredential()
			if (err != nil) != tt.wantErr {
				t.Fatalf("mongoCredential() error = %v, want error %v", err, tt.wantErr)
			}
			if (cred != nil) != tt.want {
				t.Fatalf("mongoCredential() = %+v, want a credential: %v", cred, tt.want)
			}
			if cred != nil && (cred.Username != tt.user || cred.Password != tt.pass) {
				t.Errorf("mongoCredential() = %+v, want %s / %s", cred, tt.user, tt.pass)
			}
		})
	}
}
