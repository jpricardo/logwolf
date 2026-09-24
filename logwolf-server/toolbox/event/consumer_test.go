package event

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"logwolf-toolbox/data"
	"net/rpc"
	"testing"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"
)

// --- test doubles ---

// fakeAck records how a delivery was settled.
type fakeAck struct{ outcome string }

func (a *fakeAck) Ack(uint64, bool) error { a.outcome = "ack"; return nil }
func (a *fakeAck) Nack(_ uint64, _ bool, requeue bool) error {
	a.outcome = fmt.Sprintf("nack requeue=%v", requeue)
	return nil
}
func (a *fakeAck) Reject(_ uint64, requeue bool) error {
	a.outcome = fmt.Sprintf("reject requeue=%v", requeue)
	return nil
}

// scriptedSink answers LogInfo with errs in turn, then succeeds.
type scriptedSink struct {
	errs  []error
	calls int
}

func (s *scriptedSink) LogInfo(data.RPCLogPayload) error {
	s.calls++
	if s.calls <= len(s.errs) {
		return s.errs[s.calls-1]
	}
	return nil
}

// failingSink never stores anything.
type failingSink struct {
	err   error
	calls int
}

func (s *failingSink) LogInfo(data.RPCLogPayload) error { s.calls++; return s.err }

func fastRetries(t *testing.T) {
	t.Helper()
	base, max := retryBaseDelay, retryMaxDelay
	retryBaseDelay, retryMaxDelay = time.Millisecond, 4*time.Millisecond
	t.Cleanup(func() { retryBaseDelay, retryMaxDelay = base, max })
}

func logDelivery(t *testing.T, ack *fakeAck) amqp.Delivery {
	t.Helper()
	body, err := json.Marshal(Payload{Action: "log", Log: data.JSONLogPayload{Name: "evt", ProjectID: "p"}})
	if err != nil {
		t.Fatal(err)
	}
	return amqp.Delivery{Acknowledger: ack, Body: body}
}

var (
	errUnreachable = errors.New("dial tcp logger:5001: connect: connection refused")
	errUnknown     = rpc.ServerError(fmt.Sprintf("LogInfo: %s: %q", data.ErrUnknownProject, "p"))
	errMongo       = rpc.ServerError("server selection error: context deadline exceeded")
)

// --- handleDelivery ---

func TestHandleDelivery(t *testing.T) {
	fastRetries(t)

	tests := []struct {
		name      string
		sink      logSink
		wantCalls int
		want      string
	}{
		{"stored", &scriptedSink{}, 1, "ack"},
		{"logger down, then back", &scriptedSink{errs: []error{errUnreachable, io.EOF, errUnreachable}}, 4, "ack"},
		{"refused once, then stored", &scriptedSink{errs: []error{errMongo}}, 2, "ack"},
		{"unknown project is dropped at once", &failingSink{err: errUnknown}, 1, "reject requeue=false"},
		{"refused every time is dropped", &failingSink{err: errMongo}, maxRefusals, "reject requeue=false"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ack := &fakeAck{}
			handleDelivery(context.Background(), logDelivery(t, ack), tt.sink)

			if ack.outcome != tt.want {
				t.Errorf("settled with %q, want %q", ack.outcome, tt.want)
			}
			var calls int
			switch s := tt.sink.(type) {
			case *scriptedSink:
				calls = s.calls
			case *failingSink:
				calls = s.calls
			}
			if calls != tt.wantCalls {
				t.Errorf("LogInfo called %d times, want %d", calls, tt.wantCalls)
			}
		})
	}
}

// TestHandleDelivery_RequeuesOnShutdown: an event the logger has not taken when
// the listener stops goes back to the queue instead of being lost.
func TestHandleDelivery_RequeuesOnShutdown(t *testing.T) {
	fastRetries(t)
	ctx, cancel := context.WithCancel(context.Background())
	sink := &failingSink{err: errUnreachable}

	done := make(chan struct{})
	ack := &fakeAck{}
	go func() {
		handleDelivery(ctx, logDelivery(t, ack), sink)
		close(done)
	}()

	time.Sleep(20 * time.Millisecond)
	cancel()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("handleDelivery kept retrying after ctx was cancelled")
	}

	if ack.outcome != "nack requeue=true" {
		t.Errorf("settled with %q, want requeue", ack.outcome)
	}
	if sink.calls < 2 {
		t.Errorf("LogInfo called %d times; an unreachable logger should be retried", sink.calls)
	}
}

func TestHandleDelivery_MalformedAndUnknownAction(t *testing.T) {
	ack := &fakeAck{}
	handleDelivery(context.Background(), amqp.Delivery{Acknowledger: ack, Body: []byte("{not json")}, &scriptedSink{})
	if ack.outcome != "reject requeue=false" {
		t.Errorf("malformed message settled with %q, want reject", ack.outcome)
	}

	ack = &fakeAck{}
	sink := &scriptedSink{}
	handleDelivery(context.Background(), amqp.Delivery{Acknowledger: ack, Body: []byte(`{"action":"nope"}`)}, sink)
	if ack.outcome != "ack" || sink.calls != 0 {
		t.Errorf("unknown action: settled %q with %d calls, want ack without calling the logger", ack.outcome, sink.calls)
	}
}

func TestRetryDelay(t *testing.T) {
	want := []time.Duration{time.Second, 2 * time.Second, 4 * time.Second, 8 * time.Second, 16 * time.Second, 30 * time.Second, 30 * time.Second}
	for i, w := range want {
		if got := retryDelay(i + 1); got != w {
			t.Errorf("retryDelay(%d) = %v, want %v", i+1, got, w)
		}
	}
}
