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

	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

var client *mongo.Client

type Config struct {
	Models data.Models

	// purges carries the ids of just-deleted projects from the DeleteProject RPC
	// to the cleanup loop, which deletes their logs. See requestPurge.
	purges chan primitive.ObjectID

	// startup is how far the startup tasks have got; see runStartup.
	startup *startupState
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
		Models:  data.New(client),
		purges:  make(chan primitive.ObjectID, purgeQueueSize),
		startup: &startupState{},
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGTERM, syscall.SIGINT)
	defer stop()

	// Indexes and the startup migration, before the RPC server comes up. A
	// failed pass is retried in the background; see runStartup.
	//
	// Retention is looked up by an ObjectID project_id. A setting still stored
	// under the old string one would be missed, and the project's logs expired on
	// the 90-day default however long it had chosen to keep them; so the cleanup
	// starts only once every project_id is converted.
	runStartup(ctx, app.startup, app.runStartupTasks, func() { go app.runCleanup(ctx) })

	app.serve(ctx)
}

func (app *Config) serve(ctx context.Context) {
	err := rpc.Register(&RPCServer{
		models:   app.Models,
		projects: newProjectCache(projectCacheTTL),
		purges:   app.purges,
		startup:  app.startup,
	})
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

	cred, err := mongoCredential()
	if err != nil {
		return nil, err
	}
	if cred != nil {
		clientOptions.SetAuth(*cred)
	}

	c, err := mongo.Connect(context.TODO(), clientOptions)
	if err != nil {
		log.Println("Error connecting to DB:", err)
		return nil, err
	}

	return c, nil
}

// mongoCredential reads MONGO_USERNAME and MONGO_PASSWORD, kept apart from
// MONGO_URL so a password needs no URL escaping. With neither set it returns
// nil, and whatever credentials MONGO_URL carries apply. Setting only one is a
// mistake, and refused, rather than an attempt to log in without a password.
func mongoCredential() (*options.Credential, error) {
	user, pass := os.Getenv("MONGO_USERNAME"), os.Getenv("MONGO_PASSWORD")
	switch {
	case user == "" && pass == "":
		return nil, nil
	case user == "" || pass == "":
		return nil, fmt.Errorf("set both MONGO_USERNAME and MONGO_PASSWORD, or neither")
	}
	return &options.Credential{Username: user, Password: pass}, nil
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
