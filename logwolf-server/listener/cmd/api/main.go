package main

import (
	"context"
	"log"
	"logwolf-toolbox/event"
	"logwolf-toolbox/rabbitmq"
	"os"
	"os/signal"
	"syscall"
)

func main() {
	conn, err := rabbitmq.ConnectToRabbitMQ(rabbitConnectionString())
	if err != nil {
		log.Panic(err)
	}
	defer conn.Close()

	consumer, err := event.NewConsumer(conn)
	if err != nil {
		log.Panic(err)
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGTERM, syscall.SIGINT)
	defer stop()

	log.Println("Listener started.")

	if err := consumer.Listen(ctx, []string{"log.INFO", "log.WARNING", "log.ERROR"}); err != nil {
		log.Printf("Listener stopped: %v", err)
	}

	log.Println("Shutdown complete.")
}

func rabbitConnectionString() string {
	if u := os.Getenv("RABBITMQ_URL"); u != "" {
		return u
	}
	return "amqp://guest:guest@rabbitmq"
}
