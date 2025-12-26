package main

import (
	"fmt"
	"log"
	"logwolf-toolbox/event"
	"math"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"
)

func main() {
	conn, err := connect()
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

func connect() (*amqp.Connection, error) {
	var count int64
	var limit int64 = 5
	var backoff = 1 * time.Second
	var connection *amqp.Connection

	for {
		c, err := amqp.Dial("amqp://guest:guest@rabbitmq")
		if err != nil {
			fmt.Println("RabbitMQ is not ready...")
			count++
		} else {
			log.Println("Connected to RabbitMQ!")
			connection = c
			break
		}

		if count > limit {
			fmt.Println(err)
			return nil, err
		}

		backoff = time.Duration(math.Pow(float64(count), 2)) * time.Second
		log.Printf("Backing off for %d seconds", int64(backoff.Seconds()))
		time.Sleep(backoff)
	}

	return connection, nil
}
