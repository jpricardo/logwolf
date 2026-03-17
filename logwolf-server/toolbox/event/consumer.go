package event

import (
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

func (c *Consumer) Listen(topics []string) error {
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
		err := ch.QueueBind(q.Name, t, "logs_topic", false, nil)
		if err != nil {
			return err
		}
	}

	messages, err := ch.Consume(q.Name, "", true, false, false, false, nil)
	if err != nil {
		return err
	}

	done := make(chan bool)
	go func() {
		for d := range messages {
			log.Println("Message received!")

			var payload Payload
			_ = json.Unmarshal(d.Body, &payload)

			go handlePayload(payload)
		}
	}()

	fmt.Printf("Waiting for messages on [Exchange, Queue] [logs_topic, %s]\n", q.Name)

	<-done

	return nil
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
