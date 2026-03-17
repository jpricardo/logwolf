package main

import (
	"context"
	"fmt"
	"log"
	"logwolf-toolbox/data"
	"logwolf-toolbox/rabbitmq"
	"net/http"
	"os/signal"
	"syscall"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

const (
	port            = "80"
	shutdownTimeout = 30 * time.Second
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

	srv := &http.Server{
		Addr:    fmt.Sprintf(":%s", port),
		Handler: app.routes(),
	}

	// Listen for SIGTERM/SIGINT in the background. When received, gracefully
	// drain in-flight HTTP requests before closing the RabbitMQ connection.
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGTERM, syscall.SIGINT)
	defer stop()

	go func() {
		log.Printf("Starting server on port %s\n", port)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Panicf("ListenAndServe: %v", err)
		}
	}()

	// Block until a signal is received.
	<-ctx.Done()
	log.Println("Shutdown signal received — draining in-flight requests...")

	shutdownCtx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
	defer cancel()

	if err := srv.Shutdown(shutdownCtx); err != nil {
		log.Printf("HTTP shutdown error: %v", err)
	}

	log.Println("Shutdown complete.")
}

func connectToMongo() (*mongo.Client, error) {
	clientOptions := options.Client().ApplyURI("mongodb://mongo:27017")
	clientOptions.SetAuth(options.Credential{Username: "admin", Password: "password"})
	return mongo.Connect(context.TODO(), clientOptions)
}
