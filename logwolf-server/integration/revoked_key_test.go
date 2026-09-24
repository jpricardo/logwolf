//go:build integration

package integration

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"
)

// TestRevokedKey_RefusedAtOnce revokes a key the Broker has just cached. The
// Broker used to keep accepting it from its cache for up to 60s.
func TestRevokedKey_RefusedAtOnce(t *testing.T) {
	stack := sharedStack(t)
	const owner = "revoke-now-owner"

	body := mustInternalCall(t, stack.brokerURL, http.MethodPost, "/projects", owner,
		map[string]string{"name": "Revoke now", "slug": "revoke-now"}, http.StatusCreated)
	var project struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(body, &project); err != nil || project.ID == "" {
		t.Fatalf("decode created project: %v (%s)", err, body)
	}

	keys := "/projects/" + project.ID + "/keys"
	body = mustInternalCall(t, stack.brokerURL, http.MethodPost, keys, owner, map[string]string{}, http.StatusCreated)
	var created struct {
		ID  string `json:"id"`
		Key string `json:"key"`
	}
	if err := json.Unmarshal(body, &created); err != nil || created.Key == "" || created.ID == "" {
		t.Fatalf("decode created key: %v (%s)", err, body)
	}

	// Accepted, and now in the Broker's cache.
	postLog(t, stack.brokerURL, created.Key, "revoke-now-before")

	mustInternalCall(t, stack.brokerURL, http.MethodDelete, keys+"/"+created.ID, owner, nil, http.StatusOK)

	req, _ := http.NewRequest(http.MethodPost, stack.brokerURL+"/logs",
		strings.NewReader(`{"name":"revoke-now-after","data":"{}","severity":"info","tags":[]}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+created.Key)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("POST /logs with the revoked key: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("POST /logs with the revoked key: got %d, want 401", resp.StatusCode)
	}
}
