package main

import (
	"encoding/json"
	"fmt"
	"logwolf-toolbox/data"
	"logwolf-toolbox/event"
	"net/http"
	"net/rpc"
	"strconv"

	"github.com/go-chi/chi/v5"
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

func (app *Config) ListAPIKeys(w http.ResponseWriter, r *http.Request) {
	keys, err := app.Models.ListAPIKeys()
	if err != nil {
		app.errorJSON(w, err)
		return
	}
	app.writeJSON(w, http.StatusOK, jsonResponse{Error: false, Message: "OK!", Data: keys})
}

func (app *Config) CreateAPIKey(w http.ResponseWriter, r *http.Request) {
	var body struct {
		ProjectID string `json:"project_id"`
	}
	if err := app.readJSON(w, r, &body); err != nil {
		app.errorJSON(w, err)
		return
	}

	plaintext, key, err := data.GenerateAPIKey(body.ProjectID)
	if err != nil {
		app.errorJSON(w, err)
		return
	}

	if err := app.Models.SaveAPIKey(key); err != nil {
		app.errorJSON(w, err)
		return
	}

	// Return plaintext only once — it is never stored and cannot be recovered
	app.writeJSON(w, http.StatusCreated, jsonResponse{
		Error:   false,
		Message: "API key created. Copy it now — it will not be shown again.",
		Data:    map[string]string{"key": plaintext, "prefix": key.Prefix, "id": key.ID.Hex()},
	})
}

func (app *Config) RevokeAPIKey(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if err := app.Models.RevokeAPIKey(id); err != nil {
		app.errorJSON(w, err)
		return
	}
	app.writeJSON(w, http.StatusOK, jsonResponse{Error: false, Message: "Key revoked."})
}
