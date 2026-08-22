package data

import (
	"testing"

	"go.mongodb.org/mongo-driver/bson"
)

func TestParseGithubLogins(t *testing.T) {
	tests := []struct {
		name string
		raw  string
		want []string
	}{
		{"empty", "", nil},
		{"only separators", " , ,, ", nil},
		{"single", "jpricardo", []string{"jpricardo"}},
		{"comma separated", "alice,bob", []string{"alice", "bob"}},
		{"trims spaces", " alice , bob ", []string{"alice", "bob"}},
		{"drops blanks", "alice,,bob,", []string{"alice", "bob"}},
		{"drops duplicates", "alice,bob,alice", []string{"alice", "bob"}},
		{"preserves case", "JPRicardo", []string{"JPRicardo"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ParseGithubLogins(tt.raw)
			if len(got) != len(tt.want) {
				t.Fatalf("ParseGithubLogins(%q) = %v, want %v", tt.raw, got, tt.want)
			}
			for i := range got {
				if got[i] != tt.want[i] {
					t.Errorf("ParseGithubLogins(%q)[%d] = %q, want %q", tt.raw, i, got[i], tt.want[i])
				}
			}
		})
	}
}

// TestParseGithubLogins_CaseIsSignificant guards the reason case is preserved:
// project_members lookups match the login GitHub returns at sign-in exactly, so
// folding case here would leave a mixed-case user locked out of the project the
// migration created for them.
func TestParseGithubLogins_CaseIsSignificant(t *testing.T) {
	got := ParseGithubLogins("Alice,alice")
	if len(got) != 2 {
		t.Fatalf("ParseGithubLogins should treat differently-cased logins as distinct, got %v", got)
	}
}

// TestOrphanedFilterMatchesLegacyShapes verifies the filter covers every shape a
// pre-project document can have: no project_id, an explicit null, or an empty
// string — and nothing else.
func TestOrphanedFilterMatchesLegacyShapes(t *testing.T) {
	clauses, ok := orphanedFilter()["$or"].([]bson.M)
	if !ok {
		t.Fatalf("orphanedFilter: $or should be a []bson.M, got %T", orphanedFilter()["$or"])
	}

	if len(clauses) != 3 {
		t.Errorf("orphanedFilter: got %d conditions, want 3 (missing, null, empty)", len(clauses))
	}
	for _, c := range clauses {
		if _, ok := c["project_id"]; !ok {
			t.Errorf("orphanedFilter: every condition should be on project_id, got %v", c)
		}
	}
}

func TestOrphanCountsTotal(t *testing.T) {
	c := OrphanCounts{Logs: 12, APIKeys: 3, Settings: 1}
	if c.Total() != 16 {
		t.Errorf("Total() = %d, want 16", c.Total())
	}
	if (OrphanCounts{}).Total() != 0 {
		t.Error("zero OrphanCounts should total 0")
	}
}
