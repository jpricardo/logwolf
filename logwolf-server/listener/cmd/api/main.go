package main

import (
	"log"
	"logwolf-toolbox/event"
	"logwolf-toolbox/rabbitmq"
)

func main() {
	conn, err := rabbitmq.ConnectToRabbitMQ("amqp://guest:guest@rabbitmq")
	if err != nil {
		log.Panic(err)
	}
	defer conn.Close()

	consumer, err := event.NewConsumer(conn)
	if err != nil {
		log.Panic(err)
	}

	err = consumer.Listen([]string{"log.INFO", "log.WARNING", "log.ERROR"})
	if err != nil {
		log.Panic(err)
	}
}
