package main

import (
	"encoding/json"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"logwolf-toolbox/data"
)

func health(t *testing.T) (int, healthResponse) {
	t.Helper()
	w := httptest.NewRecorder()
	(&Config{}).Health(w, httptest.NewRequest(http.MethodGet, "/health", nil))

	var body healthResponse
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode /health: %v (%s)", err, w.Body.String())
	}
	return w.Code, body
}

func TestHealth_LoggerReady(t *testing.T) {
	serveFakeLogger(t, newFakeLogger())

	_, body := health(t)
	if got := body.Services["logger"]; got.Status != "up" {
		t.Errorf("logger = %+v, want up", got)
	}
}

// TestHealth_LoggerDegraded: a logger whose startup migration keeps failing
// still serves, and used to look healthy. Now the broker's /health says so.
func TestHealth_LoggerDegraded(t *testing.T) {
	f := newFakeLogger()
	f.status = &data.LoggerStatus{StartupAttempts: 3, StartupError: "convert project_id: boom"}
	serveFakeLogger(t, f)

	code, body := health(t)
	got := body.Services["logger"]
	if got.Status != "degraded" || !strings.Contains(got.Error, "convert project_id: boom") {
		t.Errorf("logger = %+v, want degraded with the startup error", got)
	}
	if code != http.StatusServiceUnavailable || body.Status != "degraded" {
		t.Errorf("/health = %d %q, want 503 degraded", code, body.Status)
	}
}

func TestHealth_LoggerDown(t *testing.T) {
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	addr := l.Addr().String()
	l.Close()
	t.Setenv("LOGGER_RPC_ADDR", addr)

	code, body := health(t)
	if got := body.Services["logger"]; got.Status != "down" {
		t.Errorf("logger = %+v, want down", got)
	}
	if code != http.StatusServiceUnavailable {
		t.Errorf("/health = %d, want 503", code)
	}
}
