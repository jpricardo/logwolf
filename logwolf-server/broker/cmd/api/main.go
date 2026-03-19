package main

import (
	"context"
	"fmt"
	"log"
	"logwolf-toolbox/data"
	"logwolf-toolbox/rabbitmq"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

const (
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
	conn, err := rabbitmq.ConnectToRabbitMQ(rabbitConnectionString())
	if err != nil {
		log.Panic(err)
	}
	defer conn.Close()

	app := Config{
		Rabbit: conn,
		Models: data.New(mongoClient),
	}

	srv := &http.Server{
		Addr:    fmt.Sprintf(":%s", httpPort()),
		Handler: app.routes(),
	}

	// Listen for SIGTERM/SIGINT in the background. When received, gracefully
	// drain in-flight HTTP requests before closing the RabbitMQ connection.
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGTERM, syscall.SIGINT)
	defer stop()

	go func() {
		log.Printf("Starting server on port %s\n", httpPort())
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
	clientOptions := options.Client().ApplyURI(mongoConnectionString())
	clientOptions.SetAuth(options.Credential{Username: "admin", Password: "password"})
	return mongo.Connect(context.TODO(), clientOptions)
}

func mongoConnectionString() string {
	if u := os.Getenv("MONGO_URL"); u != "" {
		return u
	}
	return "mongodb://mongo:27017"
}

func rabbitConnectionString() string {
	if u := os.Getenv("RABBITMQ_URL"); u != "" {
		return u
	}
	return "amqp://guest:guest@rabbitmq"
}

func httpPort() string {
	if u := os.Getenv("BROKER_PORT"); u != "" {
		return u
	}
	return "80"
}
