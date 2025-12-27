package main

import (
	"fmt"
	"log"
	"logwolf-toolbox/data"
	"math"
	"net/http"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"
)

const (
	port = "80"
)

type Config struct {
	Rabbit *amqp.Connection
	Models data.Models
}

func main() {

	// RabbitMQ
	conn, err := connectToRabbitMQ()
	if err != nil {
		log.Panic(err)
	}
	defer conn.Close()

	app := Config{
		Rabbit: conn,
	}

	log.Printf("Starting server on port %s\n", port)

	srv := &http.Server{
		Addr:    fmt.Sprintf(":%s", port),
		Handler: app.routes(),
	}

	err = srv.ListenAndServe()
	if err != nil {
		log.Panic(err)
	}
}

func connectToRabbitMQ() (*amqp.Connection, error) {
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
