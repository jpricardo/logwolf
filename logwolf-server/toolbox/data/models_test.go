package data

import "testing"

func TestQueryParamsHasProjectID(t *testing.T) {
	p := QueryParams{
		ProjectID:  "proj-alpha",
		Pagination: PaginationParams{Page: 1, PageSize: 20},
	}
	if p.ProjectID != "proj-alpha" {
		t.Errorf("QueryParams.ProjectID = %q, want %q", p.ProjectID, "proj-alpha")
	}
}

func TestLogEntryFilterHasProjectID(t *testing.T) {
	f := LogEntryFilter{
		ID:        "abc123",
		ProjectID: "proj-beta",
	}
	if f.ProjectID != "proj-beta" {
		t.Errorf("LogEntryFilter.ProjectID = %q, want %q", f.ProjectID, "proj-beta")
	}
}

// TestMultiProjectIsolation_Structs verifies that LogEntry, QueryParams, and
// LogEntryFilter all carry ProjectID so that DB queries can be scoped to a
// single project and entries from different projects can be distinguished.
func TestMultiProjectIsolation_Structs(t *testing.T) {
	projectA := "proj-aaa"
	projectB := "proj-bbb"

	entryA := LogEntry{ProjectID: projectA, Name: "event-a", Severity: "info"}
	entryB := LogEntry{ProjectID: projectB, Name: "event-b", Severity: "error"}

	if entryA.ProjectID == entryB.ProjectID {
		t.Error("entries from different projects must have distinct ProjectIDs")
	}

	filterA := LogEntryFilter{ProjectID: projectA}
	filterB := LogEntryFilter{ProjectID: projectB}

	if filterA.ProjectID == filterB.ProjectID {
		t.Error("filters for different projects must have distinct ProjectIDs")
	}

	queryA := QueryParams{ProjectID: projectA, Pagination: PaginationParams{Page: 1, PageSize: 10}}
	queryB := QueryParams{ProjectID: projectB, Pagination: PaginationParams{Page: 1, PageSize: 10}}

	if queryA.ProjectID == queryB.ProjectID {
		t.Error("query params for different projects must have distinct ProjectIDs")
	}

	// RPCLogEntryFilter is an alias for LogEntryFilter, so ProjectID is preserved.
	rpcFilter := RPCLogEntryFilter(filterA)
	if rpcFilter.ProjectID != projectA {
		t.Errorf("RPCLogEntryFilter.ProjectID = %q, want %q", rpcFilter.ProjectID, projectA)
	}
}

func TestRPCLogEntryFilterHasProjectID(t *testing.T) {
	f := RPCLogEntryFilter{
		ID:        "xyz",
		ProjectID: "proj-gamma",
	}
	if f.ProjectID != "proj-gamma" {
		t.Errorf("RPCLogEntryFilter.ProjectID = %q, want %q", f.ProjectID, "proj-gamma")
	}
}
