package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"logwolf-toolbox/data"
	"logwolf-toolbox/event"
	"net/http"
	"time"
)

// publisher is where the broker sends events: an *event.Emitter, or a fake in
// tests.
type publisher interface {
	Publish(ctx context.Context, msgs ...event.Message) error
	Check() error
}

// publishTimeout bounds how long a request waits for RabbitMQ to confirm its
// events.
const publishTimeout = 10 * time.Second

// eventMessage encodes one event the way the listener reads it, under its
// severity's routing key.
func eventMessage(p data.JSONLogPayload) (event.Message, error) {
	body, err := json.Marshal(event.Payload{Action: "log", Log: p})
	if err != nil {
		return event.Message{}, err
	}
	return event.Message{Body: body, RoutingKey: data.SeverityRoutingKey(p.Severity)}, nil
}

// publishEvents queues events and answers the request: 202 once RabbitMQ has
// confirmed every one of them, 503 otherwise. A 202 used to mean only that the
// broker had tried.
//
// All of them are encoded before any is sent, so a bad one sends none. A batch
// that fails part-way may have queued some; a client that retries it can store
// those twice.
func (app *Config) publishEvents(w http.ResponseWriter, r *http.Request, payloads ...data.JSONLogPayload) {
	msgs := make([]event.Message, len(payloads))
	for i, p := range payloads {
		m, err := eventMessage(p)
		if err != nil {
			app.errorJSON(w, err, http.StatusBadRequest)
			return
		}
		msgs[i] = m
	}

	if app.Events == nil {
		app.errorJSON(w, fmt.Errorf("event queue unavailable"), http.StatusServiceUnavailable)
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), publishTimeout)
	defer cancel()
	if err := app.Events.Publish(ctx, msgs...); err != nil {
		log.Printf(`{"event":"publish","outcome":"error","count":%d,"error":%q}`, len(msgs), err.Error())
		app.errorJSON(w, fmt.Errorf("could not queue the events, try again"), http.StatusServiceUnavailable)
		return
	}

	app.writeJSON(w, http.StatusAccepted, jsonResponse{Error: false, Message: "OK!"})
}
