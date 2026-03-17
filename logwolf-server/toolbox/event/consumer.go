package event

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"logwolf-toolbox/data"
	"net/rpc"

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

// Listen consumes messages until ctx is cancelled, then stops the consumer
// and waits for the current message goroutine to drain before returning.
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
		if err := ch.QueueBind(q.Name, t, "logs_topic", false, nil); err != nil {
			return err
		}
	}

	// Cancel the consumer tag to stop delivery when we're shutting down.
	const consumerTag = "logwolf_listener"
	messages, err := ch.Consume(q.Name, consumerTag, true, false, false, false, nil)
	if err != nil {
		return err
	}

	fmt.Printf("Waiting for messages on [Exchange, Queue] [logs_topic, %s]\n", q.Name)

	for {
		select {
		case d, ok := <-messages:
			if !ok {
				// Channel closed by RabbitMQ — connection dropped.
				return fmt.Errorf("message channel closed unexpectedly")
			}
			log.Println("Message received!")
			var payload Payload
			_ = json.Unmarshal(d.Body, &payload)
			// handlePayload is called synchronously so we finish the current
			// message before checking ctx again on the next loop iteration.
			handlePayload(payload)

		case <-ctx.Done():
			log.Println("Shutdown signal received — stopping consumer...")
			// Cancel stops RabbitMQ from delivering new messages.
			// Any message already in handlePayload above will have finished
			// because we process synchronously.
			if err := ch.Cancel(consumerTag, false); err != nil {
				log.Printf("Consumer cancel error: %v", err)
			}
			return nil
		}
	}
}

func handlePayload(p Payload) {
	switch p.Action {
	case "log":
		err := logEvent(p.Log)
		if err != nil {
			log.Println(err)
		}

	default:
		log.Printf(`{"event":"listener","outcome":"skip","reason":"unknown_action","action":"%s"}`, p.Action)
	}
}

func logEvent(p data.JSONLogPayload) error {
	log.Printf("Logging event %s via RPC", p.Name)

	client, err := rpc.Dial("tcp", "logger:5001")
	if err != nil {
		return err
	}

	payload := data.RPCLogPayload(p)

	var result string
	err = client.Call("RPCServer.LogInfo", payload, &result)
	if err != nil {
		return err
	}

	return nil
}
