//go:build integration

package integration

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"slices"
	"testing"

	"logwolf-toolbox/data"
)

// TestScopedKeys_IngestOnlyByDefault drives the issue's acceptance criteria
// through the real stack: a key minted without scopes can send events but gets
// 403 reading or deleting them, a key given more scopes can do more, and a key
// from before scopes existed keeps its full access and says so in the list.
func TestScopedKeys_IngestOnlyByDefault(t *testing.T) {
	stack := sharedStack(t)
	const owner = "scoped-key-owner"

	body := mustInternalCall(t, stack.brokerURL, http.MethodPost, "/projects", owner,
		map[string]string{"name": "Scoped keys", "slug": "scoped-keys"}, http.StatusCreated)
	var project struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(body, &project); err != nil || project.ID == "" {
		t.Fatalf("decode created project: %v (%s)", err, body)
	}

	mint := func(scopes []string) (key string, got []string) {
		t.Helper()
		req := map[string]any{"project_id": project.ID}
		if scopes != nil {
			req["scopes"] = scopes
		}
		body := mustInternalCall(t, stack.brokerURL, http.MethodPost, "/keys", owner, req, http.StatusCreated)
		var created struct {
			Key    string   `json:"key"`
			Scopes []string `json:"scopes"`
		}
		if err := json.Unmarshal(body, &created); err != nil || created.Key == "" {
			t.Fatalf("decode created key: %v (%s)", err, body)
		}
		return created.Key, created.Scopes
	}

	// --- No scopes named: ingest only ---

	ingestKey, scopes := mint(nil)
	if !slices.Equal(scopes, []string{data.ScopeIngest}) {
		t.Errorf("key minted without scopes has %v, want [%s]", scopes, data.ScopeIngest)
	}

	postLog(t, stack.brokerURL, ingestKey, "scoped-ingest-event")
	waitForLog(t, stack.mongoURI, "scoped-ingest-event")

	if code := keyCall(t, stack.brokerURL, http.MethodGet, ingestKey, nil); code != http.StatusForbidden {
		t.Errorf("GET /logs with an ingest-only key = %d, want 403", code)
	}
	// An empty filter would delete every log in the project.
	if code := keyCall(t, stack.brokerURL, http.MethodDelete, ingestKey, map[string]string{}); code != http.StatusForbidden {
		t.Errorf("DELETE /logs with an ingest-only key = %d, want 403", code)
	}
	waitForLog(t, stack.mongoURI, "scoped-ingest-event")

	// --- Scopes named: exactly those ---

	readKey, scopes := mint([]string{data.ScopeRead})
	if !slices.Equal(scopes, []string{data.ScopeRead}) {
		t.Errorf("key minted with [read] has %v", scopes)
	}
	if names := getLogs(t, stack.brokerURL, readKey); !containsName(names, "scoped-ingest-event") {
		t.Errorf("read key sees %v, want scoped-ingest-event among them", names)
	}
	if code := keyCall(t, stack.brokerURL, http.MethodPost, readKey,
		map[string]any{"name": "x", "data": "{}", "severity": "info", "tags": []string{}}); code != http.StatusForbidden {
		t.Errorf("POST /logs with a read-only key = %d, want 403", code)
	}

	if status, _ := internalCall(t, stack.brokerURL, http.MethodPost, "/keys", owner,
		map[string]any{"project_id": project.ID, "scopes": []string{"admin"}}); status != http.StatusBadRequest {
		t.Errorf("POST /keys with an unknown scope = %d, want 400", status)
	}

	// --- A key from before scopes: full access, flagged as legacy ---

	legacyKey := seedAPIKey(t, stack.mongoURI, project.ID, "lw_scopedlegacy0000000000000000000000000000001")
	if names := getLogs(t, stack.brokerURL, legacyKey); !containsName(names, "scoped-ingest-event") {
		t.Errorf("legacy key sees %v, want scoped-ingest-event among them", names)
	}

	body = mustInternalCall(t, stack.brokerURL, http.MethodGet, "/keys?project_id="+project.ID, owner, nil, http.StatusOK)
	var listed []struct {
		Prefix string   `json:"prefix"`
		Scopes []string `json:"scopes"`
		Legacy bool     `json:"legacy"`
	}
	if err := json.Unmarshal(body, &listed); err != nil {
		t.Fatalf("decode key list: %v (%s)", err, body)
	}
	byPrefix := map[string]int{}
	for i, k := range listed {
		byPrefix[k.Prefix] = i
	}
	for _, want := range []struct {
		key    string
		scopes []string
		legacy bool
	}{
		{ingestKey, []string{data.ScopeIngest}, false},
		{readKey, []string{data.ScopeRead}, false},
		{legacyKey, data.AllScopes, true},
	} {
		i, ok := byPrefix[want.key[:10]]
		if !ok {
			t.Errorf("key %s missing from the list", want.key[:10])
			continue
		}
		if got := listed[i]; !slices.Equal(got.Scopes, want.scopes) || got.Legacy != want.legacy {
			t.Errorf("listed %s = %v (legacy=%v), want %v (legacy=%v)", got.Prefix, got.Scopes, got.Legacy, want.scopes, want.legacy)
		}
	}
}

// keyCall sends method /logs with apiKey and returns the status. body may be nil.
func keyCall(t *testing.T, brokerURL, method, apiKey string, body any) int {
	t.Helper()

	var reader io.Reader = http.NoBody
	if body != nil {
		b, _ := json.Marshal(body)
		reader = bytes.NewReader(b)
	}
	req, _ := http.NewRequest(method, brokerURL+"/logs", reader)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+apiKey)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("%s /logs: %v", method, err)
	}
	resp.Body.Close()
	return resp.StatusCode
}
