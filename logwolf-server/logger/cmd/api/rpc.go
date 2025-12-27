package main

import (
	"context"
	"fmt"
	"log"
	"logwolf-toolbox/data"
	"time"
)

type RPCServer struct{}

func (r *RPCServer) LogInfo(p data.RPCLogPayload, resp *string) error {
	log.Printf("Logging info: %+v", p)

	collection := client.Database("logs").Collection("logs")
	_, err := collection.InsertOne(context.TODO(), data.LogEntry{
		Name:      p.Name,
		Data:      p.Data,
		Severity:  p.Severity,
		Tags:      p.Tags,
		CreatedAt: time.Now(),
	})

	if err != nil {
		log.Println("Error inserting into logs:", err)
		return err
	}

	*resp = fmt.Sprintf("Processed payload via RPC: %s", p.Name)
	return nil
}

func (r *RPCServer) GetLogs(f data.RPCLogEntryFilter, resp *[]data.LogEntry) error {
	log.Printf("Getting logs: %+v\n", f)

	collection := client.Database("logs").Collection("logs")
	docs, err := collection.Find(context.TODO(), f)
	if err != nil {
		log.Println("Error getting logs:", err)
		return err
	}

	err = docs.All(context.TODO(), resp)
	if err != nil {
		log.Println("Error getting logs:", err)
		return err
	}

	log.Printf("Logs found via RPC: %d\n", len(*resp))
	return nil
}
