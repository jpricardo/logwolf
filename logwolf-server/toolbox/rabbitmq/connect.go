package rabbitmq

import (
	"fmt"
	"log"
	"math"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"
)

func ConnectToRabbitMQ(url string) (*amqp.Connection, error) {
	var count int64
	var limit int64 = 5
	var backoff = 1 * time.Second
	var connection *amqp.Connection

	for {
		c, err := amqp.Dial(url)
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
