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

	// Bound to every severity. Queues declared by older builds keep their
	// log.INFO, log.WARNING and log.ERROR bindings as well, which is harmless: a
	// message is delivered to a queue once, however many of its bindings match.
	if err := consumer.Listen(ctx, []string{event.LogBindingKey}); err != nil {
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
