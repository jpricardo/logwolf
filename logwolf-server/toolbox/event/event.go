package event

import (
	"fmt"

	amqp "github.com/rabbitmq/amqp091-go"
)

// LogBindingKey matches every event the broker publishes, whatever its
// severity: the listener stores them all. One word after "log.", which
// data.SeverityRoutingKey always produces.
const LogBindingKey = "log.*"

// logsExchange is the topic exchange every event is published to.
const logsExchange = "logs_topic"

// declareTopology declares the exchange and the listener's queue, bound to it
// for every log key. The broker declares it too, not only the listener: the
// exchange drops what no queue is bound for, so until the listener had first
// run, every event the broker accepted went nowhere. Declaring is idempotent.
func declareTopology(conn *amqp.Connection) error {
	ch, err := conn.Channel()
	if err != nil {
		return fmt.Errorf("declare topology: %w", err)
	}
	defer ch.Close()

	if err := declareExchange(ch); err != nil {
		return fmt.Errorf("declare exchange: %w", err)
	}
	if _, err := declareQueue(ch, queueName); err != nil {
		return fmt.Errorf("declare queue: %w", err)
	}
	if err := ch.QueueBind(queueName, LogBindingKey, logsExchange, false, nil); err != nil {
		return fmt.Errorf("bind queue: %w", err)
	}
	return nil
}

func declareExchange(ch *amqp.Channel) error {
	return ch.ExchangeDeclare(
		logsExchange,
		"topic",
		true,
		false,
		false,
		false,
		nil,
	)
}

// declareQueue declares a named, durable queue that survives Listener restarts.
// autoDelete: false - queue is not deleted when the consumer disconnects.
// exclusive:  false - queue can be reattached to after a restart.
// durable:    true  - queue survives a RabbitMQ broker restart.
func declareQueue(ch *amqp.Channel, name string) (amqp.Queue, error) {
	return ch.QueueDeclare(
		name,
		true,  // durable
		false, // autoDelete
		false, // exclusive
		false, // noWait
		nil,
	)
}
