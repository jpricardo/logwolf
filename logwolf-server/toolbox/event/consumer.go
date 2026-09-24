package event

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"logwolf-toolbox/data"
	"net/rpc"
	"os"
	"strings"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"
)

const queueName = "logwolf_logs"

type Consumer struct {
	conn      *amqp.Connection
	queueName string
}

type Payload struct {
	Action string              `json:"action"`
	Log    data.JSONLogPayload `json:"log,omitempty"`
}

func NewConsumer(conn *amqp.Connection) (Consumer, error) {
	consumer := Consumer{
		conn: conn,
	}

	err := consumer.setup()
	if err != nil {
		return Consumer{}, err
	}

	return consumer, nil
}

func (c *Consumer) setup() error {
	channel, err := c.conn.Channel()
	if err != nil {
		return err
	}
	defer channel.Close()

	return declareExchange(channel)
}

// prefetchCount caps the deliveries RabbitMQ hands the listener before it has
// acknowledged them. Events are handled one at a time, so a few in hand is
// enough; the rest wait in the durable queue.
const prefetchCount = 10

// Retrying an event backs off from retryBaseDelay, doubling up to retryMaxDelay.
// Variables so tests can shorten them.
var (
	retryBaseDelay = time.Second
	retryMaxDelay  = 30 * time.Second
)

// maxRefusals is how many times an event the logger refuses is tried before it
// is dropped. A refusal is usually the database having a bad moment, but one
// that never passes must not hold up the queue forever.
const maxRefusals = 5

// Listen consumes messages until ctx is cancelled, then stops the consumer and
// returns once the message in hand is settled.
//
// Deliveries are acknowledged by hand: only once the logger has stored the
// event, or it has been dropped for good. While the logger is unreachable the
// event is retried, and the rest wait in the queue, so events published during
// a logger outage are stored once it is back. A message still unacknowledged
// when the listener stops goes back to the queue, so delivery is at least once.
func (c *Consumer) Listen(ctx context.Context, topics []string) error {
	ch, err := c.conn.Channel()
	if err != nil {
		return err
	}
	defer ch.Close()

	q, err := declareQueue(ch, queueName)
	if err != nil {
		return err
	}

	for _, t := range topics {
		if err := ch.QueueBind(q.Name, t, logsExchange, false, nil); err != nil {
			return err
		}
	}

	if err := ch.Qos(prefetchCount, 0, false); err != nil {
		return err
	}

	// Cancel the consumer tag to stop delivery when we're shutting down.
	const consumerTag = "logwolf_listener"
	messages, err := ch.Consume(q.Name, consumerTag, false, false, false, false, nil)
	if err != nil {
		return err
	}

	logger := newLoggerClient(loggerAddress())
	defer logger.Close()

	fmt.Printf("Waiting for messages on [Exchange, Queue] [logs_topic, %s]\n", q.Name)

	for {
		select {
		case d, ok := <-messages:
			if !ok {
				// Channel closed by RabbitMQ — connection dropped.
				return fmt.Errorf("message channel closed unexpectedly")
			}
			// Handled synchronously, so the current message is settled before
			// ctx is checked again on the next loop iteration.
			handleDelivery(ctx, d, logger)

		case <-ctx.Done():
			log.Println("Shutdown signal received — stopping consumer...")
			// Cancel stops RabbitMQ from delivering new messages. Deliveries it
			// had already sent and nobody acknowledged go back to the queue when
			// the channel closes.
			if err := ch.Cancel(consumerTag, false); err != nil {
				log.Printf("Consumer cancel error: %v", err)
			}
			return nil
		}
	}
}

// handleDelivery forwards one message to the logger and settles it: ack once
// stored or deliberately skipped, reject (drop) once it can never be stored, and
// requeue if the listener is stopping before the logger took it.
func handleDelivery(ctx context.Context, d amqp.Delivery, sink logSink) {
	var p Payload
	if err := json.Unmarshal(d.Body, &p); err != nil {
		log.Printf(`{"event":"listener","outcome":"drop","reason":"malformed_message","error":%q}`, err.Error())
		settle(d.Reject(false))
		return
	}

	if p.Action != "log" {
		log.Printf(`{"event":"listener","outcome":"skip","reason":"unknown_action","action":"%s"}`, p.Action)
		settle(d.Ack(false))
		return
	}

	err := forward(ctx, sink, p.Log)
	switch {
	case err == nil:
		settle(d.Ack(false))
	case errors.Is(err, context.Canceled), errors.Is(err, context.DeadlineExceeded):
		log.Printf(`{"event":"listener","outcome":"requeue","reason":"shutdown","name":%q}`, p.Log.Name)
		settle(d.Nack(false, true))
	default:
		log.Printf(`{"event":"listener","outcome":"drop","reason":"refused","name":%q,"project_id":%q,"error":%q}`,
			p.Log.Name, p.Log.ProjectID, err.Error())
		settle(d.Reject(false))
	}
}

// forward sends one event to the logger until it is stored, refused for good,
// or ctx ends, which it reports as ctx's error.
//
// An unreachable logger is retried for as long as it takes. An event for a
// project that does not exist is refused at once: that will not change. Any
// other refusal is retried maxRefusals times.
func forward(ctx context.Context, sink logSink, p data.JSONLogPayload) error {
	for attempt := 1; ; attempt++ {
		err := sink.LogInfo(data.RPCLogPayload(p))
		if err == nil {
			return nil
		}

		var refusal rpc.ServerError
		refused := errors.As(err, &refusal)
		if refused && (strings.Contains(err.Error(), data.ErrUnknownProject.Error()) || attempt >= maxRefusals) {
			return err
		}

		delay := retryDelay(attempt)
		log.Printf(`{"event":"listener","outcome":"retry","name":%q,"attempt":%d,"delay":%q,"error":%q}`,
			p.Name, attempt, delay.String(), err.Error())

		timer := time.NewTimer(delay)
		select {
		case <-ctx.Done():
			timer.Stop()
			return ctx.Err()
		case <-timer.C:
		}
	}
}

// retryDelay is how long to wait after the given failed attempt.
func retryDelay(attempt int) time.Duration {
	delay := retryBaseDelay
	for i := 1; i < attempt && delay < retryMaxDelay; i++ {
		delay *= 2
	}
	return min(delay, retryMaxDelay)
}

// settle logs a failed ack, nack or reject. The channel is gone if one fails,
// and RabbitMQ redelivers the message on its own.
func settle(err error) {
	if err != nil {
		log.Printf(`{"event":"listener","outcome":"error","reason":"settle_failed","error":%q}`, err.Error())
	}
}

func loggerAddress() string {
	if a := os.Getenv("LOGGER_RPC_ADDR"); a != "" {
		return a
	}
	return "logger:5001"
}
