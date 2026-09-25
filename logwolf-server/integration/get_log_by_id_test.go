//go:build integration

package integration

import (
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

// TestGetLogByID reads one event through GET /logs/{id}, which the SDK's getOne
// calls, against the real Logger: the key's own event, another project's (a 404,
// like one that does not exist), and ids the Logger cannot parse.
func TestGetLogByID(t *testing.T) {
	stack := sharedStack(t)

	const ownStem, otherStem = "lw_getbyidown", "lw_getbyidother"
	ownKey := seedAPIKey(t, stack.mongoURI, seedProject(t, stack.mongoURI, "get-by-id"),
		ownStem+strings.Repeat("0", 46-len(ownStem)-1)+"1")
	otherKey := seedAPIKey(t, stack.mongoURI, seedProject(t, stack.mongoURI, "get-by-id-other"),
		otherStem+strings.Repeat("0", 46-len(otherStem)-1)+"1")

	postLog(t, stack.brokerURL, ownKey, "get-by-id-own")
	postLog(t, stack.brokerURL, otherKey, "get-by-id-other")
	waitForLog(t, stack.mongoURI, "get-by-id-own")
	waitForLog(t, stack.mongoURI, "get-by-id-other")
	ownID := fetchLogID(t, stack.mongoURI, "get-by-id-own")
	otherID := fetchLogID(t, stack.mongoURI, "get-by-id-other")

	status, body := getLogByID(t, stack.brokerURL, ownKey, ownID)
	if status != http.StatusOK {
		t.Fatalf("GET /logs/{own id} = %d: %s", status, body)
	}
	var envelope struct {
		Data struct {
			ID   string `json:"id"`
			Name string `json:"name"`
		} `json:"data"`
	}
	if err := json.Unmarshal(body, &envelope); err != nil || envelope.Data.ID != ownID || envelope.Data.Name != "get-by-id-own" {
		t.Errorf("GET /logs/{own id} = %s (%v), want get-by-id-own", body, err)
	}

	for _, id := range []string{otherID, primitive.NewObjectID().Hex(), "not-an-id"} {
		if status, body := getLogByID(t, stack.brokerURL, ownKey, id); status != http.StatusNotFound {
			t.Errorf("GET /logs/%s = %d, want 404: %s", id, status, body)
		}
	}
}

func getLogByID(t *testing.T, brokerURL, apiKey, id string) (int, []byte) {
	t.Helper()

	req, _ := http.NewRequest(http.MethodGet, brokerURL+"/logs/"+id, nil)
	req.Header.Set("Authorization", "Bearer "+apiKey)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("GET /logs/%s: %v", id, err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	return resp.StatusCode, body
}
