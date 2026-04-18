package data

import "testing"

func TestLogEntryHasProjectID(t *testing.T) {
	entry := LogEntry{
		ProjectID: "proj-abc",
		Name:      "test-event",
		Severity:  "info",
	}
	if entry.ProjectID != "proj-abc" {
		t.Errorf("LogEntry.ProjectID = %q, want %q", entry.ProjectID, "proj-abc")
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
