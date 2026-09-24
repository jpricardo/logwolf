package main

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"sync"
	"testing"

	"logwolf-toolbox/data"
	"logwolf-toolbox/event"
)

// fakePublisher records what the broker publishes, and answers with err.
type fakePublisher struct {
	mu    sync.Mutex
	calls [][]event.Message
	err   error
}

func (f *fakePublisher) Publish(_ context.Context, msgs ...event.Message) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls = append(f.calls, msgs)
	return f.err
}

func (f *fakePublisher) Check() error { return f.err }

func decodeEvent(t *testing.T, m event.Message) data.JSONLogPayload {
	t.Helper()
	var p event.Payload
	if err := json.Unmarshal(m.Body, &p); err != nil || p.Action != "log" {
		t.Fatalf("message body = %s (%v), want a log payload", m.Body, err)
	}
	return p.Log
}

func TestCreateLog_PublishesUnderItsSeverity(t *testing.T) {
	pub := &fakePublisher{}
	h := (&Config{Events: pub}).routes()
	key := seedKey(t, projAlpha, data.ScopeIngest)

	w := do(h, keyRequest(http.MethodPost, "/logs", key, `{"name":"boom","data":"{}","severity":"error"}`))
	if w.Code != http.StatusAccepted {
		t.Fatalf("POST /logs = %d, want 202 (body: %s)", w.Code, w.Body.String())
	}

	if len(pub.calls) != 1 || len(pub.calls[0]) != 1 {
		t.Fatalf("Publish calls = %v, want one with one message", pub.calls)
	}
	m := pub.calls[0][0]
	if m.RoutingKey != "log.error" {
		t.Errorf("routing key = %q, want log.error", m.RoutingKey)
	}
	if got := decodeEvent(t, m); got.Name != "boom" || got.ProjectID != projAlpha {
		t.Errorf("published %+v, want boom for the key's project %s", got, projAlpha)
	}
}

// TestCreateLogBatch_OnePublishForTheBatch: a batch is published on one
// channel and confirmed as a whole, not one round trip per event.
func TestCreateLogBatch_OnePublishForTheBatch(t *testing.T) {
	pub := &fakePublisher{}
	h := (&Config{Events: pub}).routes()
	key := seedKey(t, projAlpha, data.ScopeIngest)

	body := `[{"name":"a","severity":"info"},{"name":"b","severity":"warning"},{"name":"c","severity":"critical"}]`
	if w := do(h, keyRequest(http.MethodPost, "/logs/batch", key, body)); w.Code != http.StatusAccepted {
		t.Fatalf("POST /logs/batch = %d, want 202 (body: %s)", w.Code, w.Body.String())
	}

	if len(pub.calls) != 1 || len(pub.calls[0]) != 3 {
		t.Fatalf("Publish calls = %v, want one with three messages", pub.calls)
	}
	for i, want := range []string{"log.info", "log.warning", "log.critical"} {
		if got := pub.calls[0][i].RoutingKey; got != want {
			t.Errorf("message %d routing key = %q, want %q", i, got, want)
		}
	}
}

// TestPublishFailure_IsServiceUnavailable: an event RabbitMQ did not confirm is
// a 503 the client can retry, not a 202 for an event that may be gone.
func TestPublishFailure_IsServiceUnavailable(t *testing.T) {
	key := seedKey(t, projAlpha, data.ScopeIngest)

	for name, events := range map[string]publisher{
		"not confirmed": &fakePublisher{err: errors.New("RabbitMQ did not confirm the event")},
		"no queue":      nil,
	} {
		t.Run(name, func(t *testing.T) {
			h := (&Config{Events: events}).routes()
			for _, route := range []struct{ path, body string }{
				{"/logs", `{"name":"x","severity":"info"}`},
				{"/logs/batch", `[{"name":"x","severity":"info"}]`},
			} {
				w := do(h, keyRequest(http.MethodPost, route.path, key, route.body))
				if w.Code != http.StatusServiceUnavailable {
					t.Errorf("POST %s = %d, want 503 (body: %s)", route.path, w.Code, w.Body.String())
				}
			}
		})
	}
}

func TestHealth_ReportsTheEventQueue(t *testing.T) {
	serveFakeLogger(t, newFakeLogger())

	for name, tc := range map[string]struct {
		events publisher
		want   string
	}{
		"reachable":   {&fakePublisher{}, "up"},
		"unreachable": {&fakePublisher{err: errors.New("connection refused")}, "down"},
	} {
		t.Run(name, func(t *testing.T) {
			if got := checkRabbitMQ(&Config{Events: tc.events}); got.Status != tc.want {
				t.Errorf("checkRabbitMQ = %+v, want %s", got, tc.want)
			}
		})
	}
}
