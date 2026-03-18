package main

import (
	"fmt"
	"log"
	"logwolf-toolbox/data"
)

type RPCServer struct {
	models data.Models
}

func (r *RPCServer) LogInfo(p data.RPCLogPayload, resp *string) error {
	log.Printf("Logging info: %s", p.Name)

	err := r.models.Insert(data.LogEntry{
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

	result, err := r.models.AllLogs(p)
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

	result, err := r.models.DeleteLog(f.ID)
	if err != nil {
		log.Println("Error deleting document:", err)
		return err
	}

	*resp = result.DeletedCount
	log.Printf("Deleted: %d!", result.DeletedCount)

	return nil
}

func (r *RPCServer) GetRetention(args *string, reply *int) error {
	days, err := r.models.Settings.GetRetentionDays()
	if err != nil {
		return err
	}
	*reply = days
	return nil
}

func (r *RPCServer) UpdateRetention(days *int, reply *string) error {
	if err := r.models.Settings.SetRetentionDays(*days); err != nil {
		return err
	}
	if err := r.models.Settings.EnsureTTLIndex(*days); err != nil {
		return err
	}
	*reply = "ok"
	return nil
}

func (r *RPCServer) GetMetrics(args *string, reply *data.Metrics) error {
	metrics, err := r.models.GetMetrics()
	if err != nil {
		return err
	}
	*reply = *metrics
	return nil
}
