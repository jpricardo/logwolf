package main

import (
	"context"
	"fmt"
	"log"
	"logwolf-logger/data"
	"time"
)

type RPCServer struct{}

type RPCPayload struct {
	Name string
	Data string
}

func (r *RPCServer) LogInfo(p RPCPayload, resp *string) error {
	log.Printf("Logging info: {Name: %s, Data: %s}", p.Name, p.Data)

	collection := client.Database("logs").Collection("logs")
	_, err := collection.InsertOne(context.TODO(), data.LogEntry{Name: p.Name, Data: p.Data, CreatedAt: time.Now()})
	if err != nil {
		log.Println("Error inserting into logs:", err)
		return err
	}

	*resp = fmt.Sprintf("Processed payload via RPC: %s", p.Name)
	return nil
}
