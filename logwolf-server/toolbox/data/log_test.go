package data

import (
	"encoding/json"
	"testing"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/bsontype"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

func TestLogEntryHasProjectID(t *testing.T) {
	projectID := primitive.NewObjectID()
	entry := LogEntry{
		ProjectID: projectID,
		Name:      "test-event",
		Severity:  "info",
	}
	if entry.ProjectID != projectID {
		t.Errorf("LogEntry.ProjectID = %s, want %s", entry.ProjectID.Hex(), projectID.Hex())
	}
}

// TestProjectIDIsStoredAsObjectID pins down that every document carrying a
// project_id stores it as an ObjectID, the type projects._id and
// project_members.project_id have. A filter of the other type matches nothing,
// silently.
func TestProjectIDIsStoredAsObjectID(t *testing.T) {
	projectID := primitive.NewObjectID()

	for name, doc := range map[string]any{
		"logs":            LogEntry{ProjectID: projectID},
		"api_keys":        APIKey{ProjectID: projectID},
		"settings":        settingsDoc{ProjectID: projectID},
		"project_members": ProjectMember{ProjectID: projectID},
	} {
		raw, err := bson.Marshal(doc)
		if err != nil {
			t.Fatalf("%s: marshal: %v", name, err)
		}
		v := bson.Raw(raw).Lookup("project_id")
		if v.Type != bsontype.ObjectID {
			t.Errorf("%s: project_id stored as %s, want objectId", name, v.Type)
		}
	}
}

// TestProjectIDIsHexInJSON pins down that the switch to ObjectID changed
// nothing for the broker's JSON responses: project_id is still the hex string.
func TestProjectIDIsHexInJSON(t *testing.T) {
	projectID := primitive.NewObjectID()

	for name, v := range map[string]any{
		"LogEntry": LogEntry{ProjectID: projectID},
		"APIKey":   APIKey{ProjectID: projectID},
	} {
		raw, err := json.Marshal(v)
		if err != nil {
			t.Fatalf("%s: marshal: %v", name, err)
		}
		var out struct {
			ProjectID string `json:"project_id"`
		}
		if err := json.Unmarshal(raw, &out); err != nil {
			t.Fatalf("%s: unmarshal: %v", name, err)
		}
		if out.ProjectID != projectID.Hex() {
			t.Errorf("%s: project_id = %q, want %q", name, out.ProjectID, projectID.Hex())
		}
	}
}

func TestJSONLogPayloadHasProjectID(t *testing.T) {
	p := JSONLogPayload{
		ProjectID: "proj-xyz",
		Name:      "test",
		Severity:  "error",
	}
	if p.ProjectID != "proj-xyz" {
		t.Errorf("JSONLogPayload.ProjectID = %q, want %q", p.ProjectID, "proj-xyz")
	}
}

func TestRPCLogPayloadConversion(t *testing.T) {
	json := JSONLogPayload{
		ProjectID: "proj-123",
		Name:      "conversion-test",
		Data:      "{}",
		Severity:  "warning",
		Tags:      []string{"a", "b"},
		Duration:  42,
	}
	rpc := RPCLogPayload(json)
	if rpc.ProjectID != json.ProjectID {
		t.Errorf("RPCLogPayload.ProjectID = %q, want %q", rpc.ProjectID, json.ProjectID)
	}
	if rpc.Name != json.Name {
		t.Errorf("RPCLogPayload.Name = %q, want %q", rpc.Name, json.Name)
	}
	if rpc.Duration != json.Duration {
		t.Errorf("RPCLogPayload.Duration = %d, want %d", rpc.Duration, json.Duration)
	}
}

func TestSettingsRetentionArgs(t *testing.T) {
	args := RetentionArgs{ProjectID: "proj-abc", Days: 30}
	if args.ProjectID != "proj-abc" {
		t.Errorf("RetentionArgs.ProjectID = %q, want %q", args.ProjectID, "proj-abc")
	}
	if args.Days != 30 {
		t.Errorf("RetentionArgs.Days = %d, want %d", args.Days, 30)
	}
}
