package event

import (
	"encoding/json"
	"fmt"
	"log"
	"net/rpc"

	amqp "github.com/rabbitmq/amqp091-go"
)

type Consumer struct {
	conn      *amqp.Connection
	queueName string
}

type Payload struct {
	Action string     `json:"action"`
	Log    LogPayload `json:"log,omitempty"`
}

type LogPayload struct {
	Name string `json:"name"`
	Data string `json:"data"`
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

	return declareExchange(channel)
}

func (c *Consumer) Listen(topics []string) error {
	ch, err := c.conn.Channel()
	if err != nil {
		return err
	}
	defer ch.Close()

	q, err := declareRandomQueue(ch)
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

	channel := make(chan bool)
	go func() {
		for d := range messages {
			log.Println("Message received!")

			var payload Payload
			_ = json.Unmarshal(d.Body, &payload)

			go handlePayload(payload)
		}
	}()

	fmt.Printf("Waiting for messages on [Exchange, Queue] [logs_topic, %s]\n", q.Name)

	<-channel

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
		log.Panic("Invalid action:", p.Action)
	}
}

type RPCLogPayload struct {
	Name string
	Data string
}

func logEvent(p LogPayload) error {
	log.Printf("Logging event %s via RPC", p.Name)

	client, err := rpc.Dial("tcp", "logger:5001")
	if err != nil {
		return err
	}

	payload := RPCLogPayload(p)

	var result string
	err = client.Call("RPCServer.LogInfo", payload, &result)
	if err != nil {
		return err
	}

	return nil
}
