package main

import (
	"fmt"
	"log"
	"logwolf-toolbox/data"
)

type RPCServer struct{}

func (r *RPCServer) LogInfo(p data.RPCLogPayload, resp *string) error {
	log.Printf("Logging info: %s", p.Name)

	app := Config{
		Models: data.New(client),
	}

	err := app.Models.LogEntry.Insert(data.LogEntry{
		Name:     p.Name,
		Data:     p.Data,
		Severity: p.Severity,
		Tags:     p.Tags,
		Duration: p.Duration,
	})
	if err != nil {
		log.Println("Error inserting into logs:", err)
		return err
	}

	*resp = fmt.Sprintf("Processed payload via RPC: %s", p.Name)
	return nil
}

func (r *RPCServer) GetLogs(p data.QueryParams, resp *[]data.LogEntry) error {
	log.Printf("Getting logs with params %+v...\n", p)

	app := Config{
		Models: data.New(client),
	}

	result, err := app.Models.LogEntry.All(p)
	if err != nil {
		log.Println("Error getting logs:", err)
		return err
	}

	for _, doc := range result {
		*resp = append(*resp, *doc)
	}

	log.Printf("Logs found via RPC: %d\n", len(*resp))
	return nil
}

func (r *RPCServer) DeleteLog(f data.RPCLogEntryFilter, resp *int64) error {
	log.Printf("Deleting log %+v...\n", f)

	app := Config{
		Models: data.New(client),
	}

	result, err := app.Models.LogEntry.DeleteOne(f.ID)
	if err != nil {
		log.Println("Error deleting document:", err)
		return err
	}

	*resp = result.DeletedCount
	log.Printf("Deleted: %d!", result.DeletedCount)

	return nil
}
