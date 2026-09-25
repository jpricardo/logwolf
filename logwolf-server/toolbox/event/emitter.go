package event

import (
	"context"
	"errors"
	"fmt"
	"logwolf-toolbox/rabbitmq"
	"sync"

	amqp "github.com/rabbitmq/amqp091-go"
)

// errNotConfirmed is what Publish answers when RabbitMQ did not take a message:
// it refused it, or the channel closed before it said.
var errNotConfirmed = errors.New("RabbitMQ did not confirm the event")

// Message is one event to publish: its JSON body and its routing key, see
// data.SeverityRoutingKey.
type Message struct {
	Body       []byte
	RoutingKey string
}

// Emitter publishes events to the logs_topic exchange, so that an event the
// broker has answered 202 for is one RabbitMQ holds on disk:
//
//   - Messages are persistent, and the queue is durable, so they survive a
//     RabbitMQ restart.
//   - Publish waits for RabbitMQ to confirm every message.
//   - The queue is declared, and bound, here as well as by the listener, so an
//     event is routable from the first publish. The exchange drops what no queue
//     is bound for, which used to be everything until the listener first ran.
//   - The connection is dialed again once it has closed, as it does when
//     RabbitMQ restarts. It used to stay dead until the broker was restarted.
//
// It is safe for concurrent use: requests share the connection, and each
// publishes on a channel of its own.
type Emitter struct {
	url  string
	dial func(url string) (*amqp.Connection, error)

	mu   sync.Mutex
	conn *amqp.Connection
}

// NewEmitter connects to RabbitMQ at url, retrying while it starts up, and
// declares the exchange and the log queue.
func NewEmitter(url string) (*Emitter, error) {
	e := &Emitter{url: url, dial: amqp.Dial}
	conn, err := rabbitmq.ConnectToRabbitMQ(url)
	if err != nil {
		return nil, err
	}
	if err := declareTopology(conn); err != nil {
		conn.Close()
		return nil, err
	}
	e.conn = conn
	return e, nil
}

// connection returns the open connection, dialing a new one if it has closed.
// A dial is a single attempt: the request that needed it fails, and the next
// one tries again.
func (e *Emitter) connection() (*amqp.Connection, error) {
	e.mu.Lock()
	defer e.mu.Unlock()

	if e.conn != nil && !e.conn.IsClosed() {
		return e.conn, nil
	}
	conn, err := e.dial(e.url)
	if err != nil {
		return nil, fmt.Errorf("connect to RabbitMQ: %w", err)
	}
	// A restarted node may have come back without the queue, if its data did
	// not survive; declaring again costs nothing when it did.
	if err := declareTopology(conn); err != nil {
		conn.Close()
		return nil, err
	}
	e.conn = conn
	return conn, nil
}

// Publish sends msgs as persistent messages, on one channel, and returns once
// RabbitMQ has confirmed every one of them, or ctx ends.
//
// An error means some of them may not have been taken. Some may have been, so a
// client that retries can store an event twice: delivery is at least once.
func (e *Emitter) Publish(ctx context.Context, msgs ...Message) error {
	conn, err := e.connection()
	if err != nil {
		return err
	}
	ch, err := conn.Channel()
	if err != nil {
		return fmt.Errorf("open channel: %w", err)
	}
	defer ch.Close()

	if err := ch.Confirm(false); err != nil {
		return fmt.Errorf("confirm mode: %w", err)
	}

	confirms := make([]*amqp.DeferredConfirmation, 0, len(msgs))
	for _, m := range msgs {
		c, err := ch.PublishWithDeferredConfirmWithContext(ctx, logsExchange, m.RoutingKey, false, false, amqp.Publishing{
			ContentType:  "application/json",
			DeliveryMode: amqp.Persistent,
			Body:         m.Body,
		})
		if err != nil {
			return fmt.Errorf("publish: %w", err)
		}
		confirms = append(confirms, c)
	}

	for _, c := range confirms {
		acked, err := c.WaitContext(ctx)
		if err != nil {
			return fmt.Errorf("wait for confirm: %w", err)
		}
		if !acked {
			return errNotConfirmed
		}
	}
	return nil
}

// Check reports whether RabbitMQ can be reached, reconnecting if needed; the
// broker's /health asks it.
func (e *Emitter) Check() error {
	conn, err := e.connection()
	if err != nil {
		return err
	}
	ch, err := conn.Channel()
	if err != nil {
		return err
	}
	return ch.Close()
}

// Close closes the connection.
func (e *Emitter) Close() error {
	e.mu.Lock()
	defer e.mu.Unlock()

	if e.conn == nil {
		return nil
	}
	return e.conn.Close()
}
