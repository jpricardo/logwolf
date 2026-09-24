package data

import "testing"

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
