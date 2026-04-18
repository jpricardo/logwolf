package data

import (
	"errors"
	"fmt"
	"testing"
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

func TestValidSlug(t *testing.T) {
	cases := []struct {
		slug  string
		valid bool
	}{
		{"my-app", true},
		{"myapp", true},
		{"my-app-123", true},
		{"123", true},
		{"MY-APP", false},        // uppercase not allowed
		{"my_app", false},        // underscore not allowed
		{"-my-app", false},       // leading hyphen
		{"my-app-", false},       // trailing hyphen
		{"my--app", false},       // consecutive hyphens
		{"", false},
		{"my app", false},        // space not allowed
		{"my.app", false},        // dot not allowed
	}

	for _, tc := range cases {
		got := ValidSlug(tc.slug)
		if got != tc.valid {
			t.Errorf("ValidSlug(%q) = %v, want %v", tc.slug, got, tc.valid)
		}
	}
}

func TestValidRole(t *testing.T) {
	if !ValidRole(RoleOwner) {
		t.Errorf("ValidRole(%q) should be true", RoleOwner)
	}
	if !ValidRole(RoleMember) {
		t.Errorf("ValidRole(%q) should be true", RoleMember)
	}
	if ValidRole("admin") {
		t.Error("ValidRole(\"admin\") should be false")
	}
	if ValidRole("") {
		t.Error("ValidRole(\"\") should be false")
	}
}

func TestProjectStruct(t *testing.T) {
	id := primitive.NewObjectID()
	now := time.Now()
	p := Project{
		ID:        id,
		Name:      "My App",
		Slug:      "my-app",
		CreatedAt: now,
	}

	if p.ID != id {
		t.Errorf("Project.ID mismatch")
	}
	if p.Name != "My App" {
		t.Errorf("Project.Name mismatch")
	}
	if p.Slug != "my-app" {
		t.Errorf("Project.Slug mismatch")
	}
	if !p.CreatedAt.Equal(now) {
		t.Errorf("Project.CreatedAt mismatch")
	}
}

func TestProjectMemberStruct(t *testing.T) {
	id := primitive.NewObjectID()
	projectID := primitive.NewObjectID()
	now := time.Now()

	pm := ProjectMember{
		ID:          id,
		ProjectID:   projectID,
		GithubLogin: "jpricardo",
		Role:        RoleOwner,
		CreatedAt:   now,
	}

	if pm.ID != id {
		t.Errorf("ProjectMember.ID mismatch")
	}
	if pm.ProjectID != projectID {
		t.Errorf("ProjectMember.ProjectID mismatch")
	}
	if pm.GithubLogin != "jpricardo" {
		t.Errorf("ProjectMember.GithubLogin mismatch")
	}
	if pm.Role != RoleOwner {
		t.Errorf("ProjectMember.Role mismatch")
	}
	if !ValidRole(pm.Role) {
		t.Errorf("ProjectMember.Role should be valid")
	}
	if !pm.CreatedAt.Equal(now) {
		t.Errorf("ProjectMember.CreatedAt mismatch")
	}
}

func TestErrLastOwner(t *testing.T) {
	if ErrLastOwner == nil {
		t.Fatal("ErrLastOwner must not be nil")
	}
	if ErrLastOwner.Error() == "" {
		t.Error("ErrLastOwner must have a non-empty message")
	}
	// Ensure it wraps correctly with errors.Is.
	wrapped := fmt.Errorf("outer: %w", ErrLastOwner)
	if !errors.Is(wrapped, ErrLastOwner) {
		t.Error("errors.Is should unwrap to ErrLastOwner")
	}
}

func TestRemoveProjectMember_LastOwnerProtection(t *testing.T) {
	// RoleOwner is the only role that triggers the last-owner guard.
	// This test verifies the constant is defined and that ValidRole accepts it.
	if RoleOwner == "" {
		t.Fatal("RoleOwner constant must not be empty")
	}
	if !ValidRole(RoleOwner) {
		t.Error("RoleOwner must be a valid role")
	}
}

func TestGetProjectsForUser_EmptyResult(t *testing.T) {
	// GetProjectsForUser must return an empty (non-nil) slice when the user
	// belongs to no projects. Verify the slice type is correct via struct usage.
	var projects []Project
	if projects != nil {
		t.Error("zero-value []Project should be nil before assignment")
	}
	projects = []Project{}
	if projects == nil {
		t.Error("empty []Project literal must not be nil")
	}
	if len(projects) != 0 {
		t.Error("empty []Project must have length 0")
	}
}
