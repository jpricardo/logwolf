package data

import "strings"

// The severities an event can have: the SDK's LogwolfEventSeveritySchema.
const (
	SeverityInfo     = "info"
	SeverityWarning  = "warning"
	SeverityError    = "error"
	SeverityCritical = "critical"
)

// SeverityRoutingKey is the RabbitMQ routing key an event of the given
// severity is published under on the logs_topic exchange: "log." and the
// severity, lower-cased, so a consumer can bind to log.error alone.
//
// The broker does not refuse a severity it does not know; the event is stored
// as sent. Its key is log.unknown rather than one built from the raw value,
// which could hold a dot and land outside every "log.*" binding.
func SeverityRoutingKey(severity string) string {
	switch s := strings.ToLower(strings.TrimSpace(severity)); s {
	case SeverityInfo, SeverityWarning, SeverityError, SeverityCritical:
		return "log." + s
	default:
		return "log.unknown"
	}
}

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
