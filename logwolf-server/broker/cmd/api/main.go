package main

import (
	"context"
	"fmt"
	"log"
	"logwolf-toolbox/data"
	"math"
	"net/http"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

const (
	port     = "80"
	mongoURL = "mongodb://mongo:27017"
)

var client *mongo.Client

type Config struct {
	Rabbit *amqp.Connection
	Models data.Models
}

func main() {
	// Mongo
	mongoClient, err := connectToMongo()
	client = mongoClient

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	defer func() {
		if err = client.Disconnect(ctx); err != nil {
			panic(err)
		}
	}()

	// RabbitMQ
	conn, err := connectToRabbitMQ()
	if err != nil {
		log.Panic(err)
	}
	defer conn.Close()

	app := Config{
		Rabbit: conn,
		Models: data.New(client),
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

func connectToMongo() (*mongo.Client, error) {
	clientOptions := options.Client().ApplyURI(mongoURL)
	clientOptions.SetAuth(options.Credential{Username: "admin", Password: "password"})

	c, err := mongo.Connect(context.TODO(), clientOptions)
	if err != nil {
		log.Println("Error connecting to DB:", err)
		return nil, err
	}

	return c, nil
}
