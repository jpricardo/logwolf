package main

import (
	"context"
	"fmt"
	"log"
	"logwolf-toolbox/data"
	"logwolf-toolbox/rabbitmq"
	"net/http"

	amqp "github.com/rabbitmq/amqp091-go"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

const (
	port = "80"
)

type Config struct {
	Rabbit *amqp.Connection
	Models data.Models
}

func main() {
	mongoClient, err := connectToMongo()
	if err != nil {
		log.Panic(err)
	}

	// RabbitMQ
	conn, err := rabbitmq.ConnectToRabbitMQ("amqp://guest:guest@rabbitmq")
	if err != nil {
		log.Panic(err)
	}
	defer conn.Close()

	app := Config{
		Rabbit: conn,
		Models: data.New(mongoClient),
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

func connectToMongo() (*mongo.Client, error) {
	clientOptions := options.Client().ApplyURI("mongodb://mongo:27017")
	clientOptions.SetAuth(options.Credential{Username: "admin", Password: "password"})
	return mongo.Connect(context.TODO(), clientOptions)
}
