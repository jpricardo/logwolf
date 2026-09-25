package data

import (
	"errors"
	"testing"
)

func TestSeverityRoutingKey(t *testing.T) {
	cases := map[string]string{
		"info":            "log.info",
		"warning":         "log.warning",
		"error":           "log.error",
		"critical":        "log.critical",
		"ERROR":           "log.error",
		" Warning ":       "log.warning",
		"":                "log.unknown",
		"debug":           "log.unknown",
		"log.info.forged": "log.unknown",
		"*":               "log.unknown",
	}
	for severity, want := range cases {
		if got := SeverityRoutingKey(severity); got != want {
			t.Errorf("SeverityRoutingKey(%q) = %q, want %q", severity, got, want)
		}
	}
}

func TestNormalizeSeverity(t *testing.T) {
	for in, want := range map[string]string{
		"info": "info", "warning": "warning", "error": "error", "critical": "critical",
		"ERROR": "error", " Warning ": "warning", "Critical\n": "critical",
	} {
		if got, err := NormalizeSeverity(in); err != nil || got != want {
			t.Errorf("NormalizeSeverity(%q) = %q, %v; want %q", in, got, err, want)
		}
	}

	for _, in := range []string{"", "  ", "debug", "warn", "fatal", "log.info.forged", "*"} {
		if got, err := NormalizeSeverity(in); !errors.Is(err, ErrInvalidSeverity) {
			t.Errorf("NormalizeSeverity(%q) = %q, %v; want ErrInvalidSeverity", in, got, err)
		}
	}
}
