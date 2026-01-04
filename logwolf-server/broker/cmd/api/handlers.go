package main

import (
	"encoding/json"
	"fmt"
	"logwolf-toolbox/data"
	"logwolf-toolbox/event"
	"net/http"
	"net/rpc"
	"strconv"
)

func (app *Config) CreateLog(w http.ResponseWriter, r *http.Request) {
	var payload data.JSONLogPayload

	err := app.readJSON(w, r, &payload)
	if err != nil {
		app.errorJSON(w, err)
		return
	}

	// Push to queue
	evp := event.Payload{Action: "log", Log: data.JSONLogPayload(payload)}

	emitter, err := event.NewEmitter(app.Rabbit)
	if err != nil {
		app.errorJSON(w, err)
		return
	}

	j, err := json.MarshalIndent(&evp, "", "\t")
	if err != nil {
		app.errorJSON(w, err)
		return
	}

	err = emitter.Push(string(j), "log.INFO")
	if err != nil {
		app.errorJSON(w, err)
		return
	}

	app.writeJSON(w, http.StatusAccepted, jsonResponse{Error: false, Message: "OK!"})
}

func (app *Config) GetLogs(w http.ResponseWriter, r *http.Request) {
	client, err := rpc.Dial("tcp", "logger:5001")
	if err != nil {
		app.errorJSON(w, err)
		return
	}

	qp := r.URL.Query()
	page, err := strconv.ParseInt(qp.Get("page"), 10, 0)
	if err != nil {
		page = 0
	}

	pageSize, err := strconv.ParseInt(qp.Get("pageSize"), 10, 0)
	if err != nil {
		pageSize = 0
	}

	var result []data.LogEntry
	err = client.Call("RPCServer.GetLogs", data.QueryParams{
		// TODO - Filters
		Pagination: data.PaginationParams{Page: page, PageSize: pageSize},
	}, &result)
	if err != nil {
		app.errorJSON(w, err)
		return
	}

	if result == nil {
		result = []data.LogEntry{}
	}

	app.writeJSON(w, http.StatusOK, jsonResponse{Error: false, Message: "OK!", Data: result})
}

func (app *Config) DeleteLog(w http.ResponseWriter, r *http.Request) {
	var requestBody data.LogEntryFilter
	err := app.readJSON(w, r, &requestBody)
	if err != nil {
		app.errorJSON(w, err)
		return
	}

	client, err := rpc.Dial("tcp", "logger:5001")
	if err != nil {
		app.errorJSON(w, err)
		return
	}

	var result int64
	err = client.Call("RPCServer.DeleteLog", requestBody, &result)
	if err != nil {
		app.errorJSON(w, err)
		return
	}

	app.writeJSON(w, http.StatusAccepted, jsonResponse{Error: false, Message: "OK!", Data: fmt.Sprintf("Deleted entries: %d", result)})
}
