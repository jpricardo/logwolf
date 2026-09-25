package data

import (
	"errors"
	"fmt"
	"strings"
)

// The severities an event can have: the SDK's LogwolfEventSeveritySchema.
const (
	SeverityInfo     = "info"
	SeverityWarning  = "warning"
	SeverityError    = "error"
	SeverityCritical = "critical"
)

// ErrInvalidSeverity is what NormalizeSeverity answers for a severity that is
// not one of the four.
var ErrInvalidSeverity = errors.New("invalid severity")

// NormalizeSeverity returns the severity an event is stored under: trimmed and
// lower-cased, so "ERROR" and " error" are the same "error", which is what the
// dashboard's metrics count. Anything that is not info, warning, error or
// critical, the empty string included, is ErrInvalidSeverity; the broker
// refuses such an event rather than store a severity nothing reads.
func NormalizeSeverity(severity string) (string, error) {
	switch s := strings.ToLower(strings.TrimSpace(severity)); s {
	case SeverityInfo, SeverityWarning, SeverityError, SeverityCritical:
		return s, nil
	default:
		return "", fmt.Errorf("%w %q: must be one of %s, %s, %s or %s", ErrInvalidSeverity, severity,
			SeverityInfo, SeverityWarning, SeverityError, SeverityCritical)
	}
}

// SeverityRoutingKey is the RabbitMQ routing key an event of the given
// severity is published under on the logs_topic exchange: "log." and the
// normalized severity, so a consumer can bind to log.error alone.
//
// The broker only publishes normalized severities. Anything else gets
// log.unknown, never a key built from the raw value, which could hold a dot
// and land outside every "log.*" binding.
func SeverityRoutingKey(severity string) string {
	s, err := NormalizeSeverity(severity)
	if err != nil {
		return "log.unknown"
	}
	return "log." + s
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
