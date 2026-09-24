//go:build integration

package integration

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"testing"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"
	"go.mongodb.org/mongo-driver/bson"
)

// TestSeverityRouting posts one event of each severity and watches the
// logs_topic exchange through a queue of its own, bound to every log key. The
// Broker used to publish everything as log.INFO, so a consumer bound to
// log.error never heard of an error. The Listener binds log.*, so every event
// is still stored, critical ones included, which no binding of its used to cover.
func TestSeverityRouting(t *testing.T) {
	stack := sharedStack(t)
	// A key of its own: testAPIKey's fixed plaintext is another test's.
	key := seedAPIKey(t, stack.mongoURI, seedProject(t, stack.mongoURI, "severity-routing"),
		"lw_severityrouting0000000000000000000000000001")

	conn, err := amqp.Dial(stack.rabbitURI)
	if err != nil {
		t.Fatalf("dial rabbitmq: %v", err)
	}
	defer conn.Close()
	ch, err := conn.Channel()
	if err != nil {
		t.Fatalf("channel: %v", err)
	}
	defer ch.Close()

	// Exclusive and server-named: it disappears with the connection, and takes
	// nothing from the Listener's own queue.
	q, err := ch.QueueDeclare("", false, true, true, false, nil)
	if err != nil {
		t.Fatalf("declare queue: %v", err)
	}
	if err := ch.QueueBind(q.Name, "log.#", "logs_topic", false, nil); err != nil {
		t.Fatalf("bind queue: %v", err)
	}
	deliveries, err := ch.Consume(q.Name, "", true, true, false, false, nil)
	if err != nil {
		t.Fatalf("consume: %v", err)
	}

	cases := map[string]string{ // event name -> routing key wanted
		"severity-info":     "log.info",
		"severity-warning":  "log.warning",
		"severity-error":    "log.error",
		"severity-critical": "log.critical",
		"severity-shouting": "log.error",
		"severity-bogus":    "log.unknown",
	}
	severities := map[string]string{
		"severity-info":     "info",
		"severity-warning":  "warning",
		"severity-error":    "error",
		"severity-critical": "critical",
		"severity-shouting": "ERROR",
		"severity-bogus":    "log.info.forged",
	}

	for name, severity := range severities {
		postSeverity(t, stack.brokerURL, key, name, severity)
	}

	got := map[string]string{}
	timeout := time.After(15 * time.Second)
	for len(got) < len(cases) {
		select {
		case d := <-deliveries:
			var msg struct {
				Log struct {
					Name string `json:"name"`
				} `json:"log"`
			}
			if err := json.Unmarshal(d.Body, &msg); err == nil {
				if _, ours := cases[msg.Log.Name]; ours {
					got[msg.Log.Name] = d.RoutingKey
				}
			}
		case <-timeout:
			t.Fatalf("saw %d of %d events on the exchange: %v", len(got), len(cases), got)
		}
	}

	for name, want := range cases {
		if got[name] != want {
			t.Errorf("%s (severity %q) published as %q, want %q", name, severities[name], got[name], want)
		}
	}

	// The Listener stores every one of them, whatever its key.
	for name := range cases {
		waitForLog(t, stack.mongoURI, name)
	}
	stored := testMongo(t, stack.mongoURI).Database("logs").Collection("logs")
	var critical bson.M
	if err := stored.FindOne(context.Background(), bson.M{"name": "severity-critical"}).Decode(&critical); err != nil {
		t.Fatalf("find critical event: %v", err)
	}
	if critical["severity"] != "critical" {
		t.Errorf("critical event stored with severity %v", critical["severity"])
	}
}

// postSeverity sends one event with the given severity and asserts a 202.
func postSeverity(t *testing.T, brokerURL, apiKey, name, severity string) {
	t.Helper()

	body, _ := json.Marshal(map[string]any{"name": name, "data": "{}", "severity": severity, "tags": []string{}})
	req, _ := http.NewRequest(http.MethodPost, brokerURL+"/logs", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+apiKey)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("post %q: %v", name, err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusAccepted {
		t.Fatalf("post %q: got %d, want 202", name, resp.StatusCode)
	}
}
