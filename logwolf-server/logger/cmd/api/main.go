package main

import (
	"context"
	"fmt"
	"log"
	"logwolf-toolbox/data"
	"net"
	"net/http"
	"net/rpc"
	"os"
	"os/signal"
	"syscall"
	"time"

	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

const (
	grpcPort = "50001"
)

var client *mongo.Client

type Config struct {
	Models data.Models
}

func main() {
	mongoClient, err := connectToMongo()
	if err != nil {
		log.Panic(err)
	}

	client = mongoClient

	defer func() {
		disconnectCtx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		if err = client.Disconnect(disconnectCtx); err != nil {
			panic(err)
		}
	}()

	app := Config{
		Models: data.New(client),
	}

	if err := app.Models.Settings.EnsureSettingsIndex(); err != nil {
		log.Printf("Warning: could not ensure settings index: %v", err)
	}
	if err := app.Models.EnsureLogsIndexes(); err != nil {
		log.Printf("Warning: could not ensure logs indexes: %v", err)
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGTERM, syscall.SIGINT)
	defer stop()

	go app.runCleanup(ctx)

	app.serve(ctx)
}

func (app *Config) serve(ctx context.Context) {
	err := rpc.Register(&RPCServer{models: app.Models})
	if err != nil {
		log.Panic(err)
	}
	go app.rpcListen()

	srv := &http.Server{
		Addr:    fmt.Sprintf(":%s", httpPort()),
		Handler: app.routes(),
	}

	go func() {
		log.Println("Starting HTTP server on port", httpPort())
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Printf("HTTP server error: %v", err)
		}
	}()

	<-ctx.Done()
	log.Println("Shutting down HTTP server...")

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := srv.Shutdown(shutdownCtx); err != nil {
		log.Printf("HTTP server shutdown error: %v", err)
	}
}

func (app *Config) rpcListen() error {
	log.Println("Starting RPC server on port", rpcPort())

	listen, err := net.Listen("tcp", fmt.Sprintf("0.0.0.0:%s", rpcPort()))
	if err != nil {
		return err
	}
	defer listen.Close()

	for {
		conn, err := listen.Accept()
		if err != nil {
			continue
		}

		go rpc.ServeConn(conn)
	}
}

func connectToMongo() (*mongo.Client, error) {
	clientOptions := options.Client().ApplyURI(mongoConnectionString())
	clientOptions.SetAuth(options.Credential{Username: "admin", Password: "password"})

	c, err := mongo.Connect(context.TODO(), clientOptions)
	if err != nil {
		log.Println("Error connecting to DB:", err)
		return nil, err
	}

	return c, nil
}

func mongoConnectionString() string {
	if u := os.Getenv("MONGO_URL"); u != "" {
		return u
	}
	return "mongodb://mongo:27017"
}

func rpcPort() string {
	if u := os.Getenv("LOGGER_RPC_PORT"); u != "" {
		return u
	}
	return "5001"
}

func httpPort() string {
	if p := os.Getenv("LOGGER_HTTP_PORT"); p != "" {
		return p
	}
	return "80"
}
