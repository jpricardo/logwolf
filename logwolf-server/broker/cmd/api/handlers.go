package main

import (
	"encoding/json"
	"fmt"
	"logwolf-toolbox/data"
	"logwolf-toolbox/event"
	"net"
	"net/http"
	"net/rpc"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"
)

func (app *Config) CreateLog(w http.ResponseWriter, r *http.Request) {
	var payload data.JSONLogPayload

	err := app.readJSON(w, r, &payload)
	if err != nil {
		app.errorJSON(w, err)
		return
	}

	payload.ProjectID = projectIDFromContext(r)

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

func (app *Config) CreateLogBatch(w http.ResponseWriter, r *http.Request) {
	var payloads []data.JSONLogPayload

	err := app.readJSON(w, r, &payloads)
	if err != nil {
		app.errorJSON(w, err)
		return
	}

	if len(payloads) == 0 {
		app.writeJSON(w, http.StatusAccepted, jsonResponse{Error: false, Message: "OK!"})
		return
	}

	if len(payloads) > 1000 {
		app.errorJSON(w, fmt.Errorf("batch size %d exceeds maximum of 1000", len(payloads)), http.StatusRequestEntityTooLarge)
		return
	}

	projectID := projectIDFromContext(r)
	for i := range payloads {
		payloads[i].ProjectID = projectID
	}

	// Pre-serialize all payloads before emitting any. This ensures a
	// marshaling error doesn't cause a partial write to RabbitMQ.
	messages := make([]string, len(payloads))
	for i, payload := range payloads {
		evp := event.Payload{Action: "log", Log: payload}

		j, err := json.MarshalIndent(&evp, "", "\t")
		if err != nil {
			app.errorJSON(w, err)
			return
		}

		messages[i] = string(j)
	}

	emitter, err := event.NewEmitter(app.Rabbit)
	if err != nil {
		app.errorJSON(w, err)
		return
	}

	for _, msg := range messages {
		if err := emitter.Push(msg, "log.INFO"); err != nil {
			app.errorJSON(w, err)
			return
		}
	}

	app.writeJSON(w, http.StatusAccepted, jsonResponse{Error: false, Message: "OK!"})
}

func (app *Config) GetLogs(w http.ResponseWriter, r *http.Request) {
	client, err := rpc.Dial("tcp", loggerRPCAddr())
	if err != nil {
		app.errorJSON(w, err)
		return
	}

	qp := r.URL.Query()
	page, err := strconv.ParseInt(qp.Get("page"), 10, 0)
	if err != nil {
		page = 0
	}

	if page < 1 {
		page = 1
	}

	pageSize, err := strconv.ParseInt(qp.Get("pageSize"), 10, 0)
	if err != nil {
		pageSize = 0
	}

	if pageSize < 1 {
		pageSize = 20
	}

	var result []data.LogEntry
	err = client.Call("RPCServer.GetLogs", data.QueryParams{
		ProjectID:  projectIDFromContext(r),
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

	requestBody.ProjectID = projectIDFromContext(r)

	client, err := rpc.Dial("tcp", loggerRPCAddr())
	if err != nil {
		app.errorJSON(w, err)
		return
	}

	var result int64
	err = client.Call("RPCServer.DeleteLog", data.RPCLogEntryFilter(requestBody), &result)
	if err != nil {
		app.errorJSON(w, err)
		return
	}

	app.writeJSON(w, http.StatusAccepted, jsonResponse{Error: false, Message: "OK!", Data: fmt.Sprintf("Deleted entries: %d", result)})
}

func (app *Config) ListAPIKeys(w http.ResponseWriter, r *http.Request) {
	projectID := r.URL.Query().Get("project_id")
	if projectID == "" {
		app.errorJSON(w, fmt.Errorf("project_id is required"), http.StatusBadRequest)
		return
	}

	client, err := rpc.Dial("tcp", loggerRPCAddr())
	if err != nil {
		app.errorJSON(w, err)
		return
	}
	defer client.Close()

	userLogin := userLoginFromContext(r)
	isMember, err := app.checkProjectMembership(client, projectID, userLogin)
	if err != nil {
		app.errorJSON(w, err)
		return
	}
	if !isMember {
		app.errorJSON(w, fmt.Errorf("forbidden"), http.StatusForbidden)
		return
	}

	keys, err := app.Models.ListAPIKeysByProject(projectID)
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

	if body.ProjectID == "" {
		app.errorJSON(w, fmt.Errorf("project_id is required"), http.StatusBadRequest)
		return
	}

	client, err := rpc.Dial("tcp", loggerRPCAddr())
	if err != nil {
		app.errorJSON(w, err)
		return
	}
	defer client.Close()

	userLogin := userLoginFromContext(r)
	isMember, err := app.checkProjectMembership(client, body.ProjectID, userLogin)
	if err != nil {
		app.errorJSON(w, err)
		return
	}
	if !isMember {
		app.errorJSON(w, fmt.Errorf("forbidden"), http.StatusForbidden)
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

	key, err := app.Models.GetAPIKeyByID(id)
	if err != nil {
		app.errorJSON(w, err)
		return
	}

	client, err := rpc.Dial("tcp", loggerRPCAddr())
	if err != nil {
		app.errorJSON(w, err)
		return
	}
	defer client.Close()

	userLogin := userLoginFromContext(r)
	isMember, err := app.checkProjectMembership(client, key.ProjectID, userLogin)
	if err != nil {
		app.errorJSON(w, err)
		return
	}
	if !isMember {
		app.errorJSON(w, fmt.Errorf("forbidden"), http.StatusForbidden)
		return
	}

	if err := app.Models.RevokeAPIKey(id); err != nil {
		app.errorJSON(w, err)
		return
	}
	app.writeJSON(w, http.StatusOK, jsonResponse{Error: false, Message: "Key revoked."})
}

type retentionResponse struct {
	Days int `json:"days"`
}

func (app *Config) GetRetention(w http.ResponseWriter, r *http.Request) {
	projectID := r.URL.Query().Get("project_id")
	if projectID == "" {
		app.errorJSON(w, fmt.Errorf("project_id is required"), http.StatusBadRequest)
		return
	}

	client, err := rpc.Dial("tcp", loggerRPCAddr())
	if err != nil {
		app.errorJSON(w, err)
		return
	}
	defer client.Close()

	userLogin := userLoginFromContext(r)
	isMember, err := app.checkProjectMembership(client, projectID, userLogin)
	if err != nil {
		app.errorJSON(w, err)
		return
	}
	if !isMember {
		app.errorJSON(w, fmt.Errorf("forbidden"), http.StatusForbidden)
		return
	}

	var days int
	args := data.RetentionArgs{ProjectID: projectID}
	if err := client.Call("RPCServer.GetRetention", &args, &days); err != nil {
		app.errorJSON(w, err)
		return
	}

	app.writeJSON(w, http.StatusOK, jsonResponse{Error: false, Data: retentionResponse{Days: days}})
}

func (app *Config) UpdateRetention(w http.ResponseWriter, r *http.Request) {
	var payload struct {
		ProjectID string `json:"project_id"`
		Days      int    `json:"days"`
	}

	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		app.errorJSON(w, err)
		return
	}

	if payload.ProjectID == "" {
		app.errorJSON(w, fmt.Errorf("project_id is required"), http.StatusBadRequest)
		return
	}

	client, err := rpc.Dial("tcp", loggerRPCAddr())
	if err != nil {
		app.errorJSON(w, err)
		return
	}
	defer client.Close()

	userLogin := userLoginFromContext(r)
	isMember, err := app.checkProjectMembership(client, payload.ProjectID, userLogin)
	if err != nil {
		app.errorJSON(w, err)
		return
	}
	if !isMember {
		app.errorJSON(w, fmt.Errorf("forbidden"), http.StatusForbidden)
		return
	}

	args := data.RetentionArgs{ProjectID: payload.ProjectID, Days: payload.Days}
	var reply string
	if err := client.Call("RPCServer.UpdateRetention", &args, &reply); err != nil {
		app.errorJSON(w, err)
		return
	}

	app.writeJSON(w, http.StatusOK, jsonResponse{Error: false, Data: retentionResponse{Days: payload.Days}})
}

func (app *Config) GetMetrics(w http.ResponseWriter, r *http.Request) {
	projectID := r.URL.Query().Get("project_id")
	if projectID == "" {
		app.errorJSON(w, fmt.Errorf("project_id is required"), http.StatusBadRequest)
		return
	}

	client, err := rpc.Dial("tcp", loggerRPCAddr())
	if err != nil {
		app.errorJSON(w, err)
		return
	}
	defer client.Close()

	userLogin := userLoginFromContext(r)
	isMember, err := app.checkProjectMembership(client, projectID, userLogin)
	if err != nil {
		app.errorJSON(w, err)
		return
	}
	if !isMember {
		app.errorJSON(w, fmt.Errorf("forbidden"), http.StatusForbidden)
		return
	}

	args := data.ProjectArgs{ProjectID: projectID}
	var result data.Metrics
	if err := client.Call("RPCServer.GetMetrics", &args, &result); err != nil {
		app.errorJSON(w, err)
		return
	}

	app.writeJSON(w, http.StatusOK, jsonResponse{Error: false, Message: "OK!", Data: result})
}

type serviceStatus struct {
	Status string `json:"status"`
	Error  string `json:"error,omitempty"`
}

type healthResponse struct {
	Status   string                   `json:"status"`
	Services map[string]serviceStatus `json:"services"`
}

func (app *Config) Health(w http.ResponseWriter, r *http.Request) {
	rabbitmq := checkRabbitMQ(app)
	logger := checkLogger()

	overall := "healthy"
	if rabbitmq.Status == "down" || logger.Status == "down" {
		overall = "degraded"
	}

	status := http.StatusOK
	if overall == "degraded" {
		status = http.StatusServiceUnavailable
	}

	app.writeJSON(w, status, healthResponse{
		Status: overall,
		Services: map[string]serviceStatus{
			"rabbitmq": rabbitmq,
			"logger":   logger,
		},
	})
}

func checkRabbitMQ(app *Config) serviceStatus {
	if app.Rabbit == nil || app.Rabbit.IsClosed() {
		return serviceStatus{Status: "down", Error: "connection is closed"}
	}

	ch, err := app.Rabbit.Channel()
	if err != nil {
		return serviceStatus{Status: "down", Error: err.Error()}
	}
	ch.Close()

	return serviceStatus{Status: "up"}
}

func checkLogger() serviceStatus {
	conn, err := net.DialTimeout("tcp", loggerRPCAddr(), 2*time.Second)
	if err != nil {
		return serviceStatus{Status: "down", Error: err.Error()}
	}
	conn.Close()

	return serviceStatus{Status: "up"}
}
