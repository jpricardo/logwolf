package data

type JSONLogPayload struct {
	ProjectID string   `json:"project_id"`
	Name      string   `json:"name"`
	Data      string   `json:"data"`
	Severity  string   `json:"severity"`
	Tags      []string `json:"tags"`
	Duration  int      `json:"duration"`
}

type RPCLogPayload struct {
	ProjectID string
	Name      string
	Data      string
	Severity  string
	Tags      []string
	Duration  int
}

type RPCLogEntryFilter LogEntryFilter
