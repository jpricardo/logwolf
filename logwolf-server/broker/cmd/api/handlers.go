package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"logwolf-toolbox/data"
	"logwolf-toolbox/event"
	"net"
	"net/http"
	"net/rpc"
	"net/url"
	"strconv"
	"strings"
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

	var result []data.LogEntry
	err = client.Call("RPCServer.GetLogs", data.QueryParams{
		ProjectID:  projectIDFromContext(r),
		Pagination: paginationFromQuery(r.URL.Query()),
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

// paginationFromQuery reads page/pageSize off a query string, falling back to
// the first page of 20 whenever either is missing or unusable.
func paginationFromQuery(qp url.Values) data.PaginationParams {
	page, err := strconv.ParseInt(qp.Get("page"), 10, 0)
	if err != nil || page < 1 {
		page = 1
	}

	pageSize, err := strconv.ParseInt(qp.Get("pageSize"), 10, 0)
	if err != nil || pageSize < 1 {
		pageSize = 20
	}

	return data.PaginationParams{Page: page, PageSize: pageSize}
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
	isMember, err := checkProjectMembership(client, projectID, userLogin)
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

	if keys == nil {
		keys = []data.APIKey{}
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
	isMember, err := checkProjectMembership(client, body.ProjectID, userLogin)
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
	if errors.Is(err, data.ErrKeyNotFound) {
		app.errorJSON(w, fmt.Errorf("key not found"), http.StatusNotFound)
		return
	}
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
	isMember, err := checkProjectMembership(client, key.ProjectID, userLogin)
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
	isMember, err := checkProjectMembership(client, projectID, userLogin)
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
	isMember, err := checkProjectMembership(client, payload.ProjectID, userLogin)
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
	isMember, err := checkProjectMembership(client, projectID, userLogin)
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

// --- Project management ---

// denyProjectAccess sends a 404 if the project doesn't exist, or 403 if it exists
// but the user has no access. Used when getProjectRole returns an empty role.
func (app *Config) denyProjectAccess(w http.ResponseWriter, client *rpc.Client, id string) {
	var proj data.Project
	err := client.Call("RPCServer.GetProject", &data.RPCProjectIDArgs{ID: id}, &proj)
	if err == nil {
		app.errorJSON(w, fmt.Errorf("forbidden"), http.StatusForbidden)
		return
	}
	if strings.Contains(err.Error(), "no documents in result") {
		app.errorJSON(w, fmt.Errorf("project not found"), http.StatusNotFound)
		return
	}
	app.errorJSON(w, err)
}

func (app *Config) ListProjects(w http.ResponseWriter, r *http.Request) {
	userLogin := userLoginFromContext(r)

	client, err := rpc.Dial("tcp", loggerRPCAddr())
	if err != nil {
		app.errorJSON(w, err)
		return
	}
	defer client.Close()

	args := data.RPCUserProjectsArgs{GithubLogin: userLogin}
	var projects []data.UserProject
	if err := client.Call("RPCServer.ListUserProjects", &args, &projects); err != nil {
		app.errorJSON(w, err)
		return
	}

	if projects == nil {
		projects = []data.UserProject{}
	}

	app.writeJSON(w, http.StatusOK, jsonResponse{Error: false, Message: "OK!", Data: projects})
}

func (app *Config) CreateProject(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Name string `json:"name"`
		Slug string `json:"slug"`
	}
	if err := app.readJSON(w, r, &body); err != nil {
		app.errorJSON(w, err)
		return
	}

	if body.Name == "" {
		app.errorJSON(w, fmt.Errorf("name is required"), http.StatusBadRequest)
		return
	}
	if !data.ValidSlug(body.Slug) {
		app.errorJSON(w, fmt.Errorf("invalid slug"), http.StatusBadRequest)
		return
	}

	userLogin := userLoginFromContext(r)

	client, err := rpc.Dial("tcp", loggerRPCAddr())
	if err != nil {
		app.errorJSON(w, err)
		return
	}
	defer client.Close()

	var project data.Project
	if err := client.Call("RPCServer.CreateProject", &data.RPCCreateProjectArgs{Name: body.Name, Slug: body.Slug}, &project); err != nil {
		// Slugs are globally unique, so a collision is a client mistake, not a
		// server fault. net/rpc flattens errors to strings, so the Mongo
		// duplicate-key code is the only thing left to match on.
		if strings.Contains(err.Error(), "E11000") {
			app.errorJSON(w, fmt.Errorf("a project with that slug already exists"), http.StatusConflict)
			return
		}
		app.errorJSON(w, err)
		return
	}

	var reply string
	if err := client.Call("RPCServer.AddMember", &data.RPCAddMemberArgs{
		ProjectID:   project.ID.Hex(),
		GithubLogin: userLogin,
		Role:        data.RoleOwner,
	}, &reply); err != nil {
		// Best-effort rollback: project must not exist without an owner.
		var rollbackReply string
		_ = client.Call("RPCServer.DeleteProject", &data.RPCProjectIDArgs{ID: project.ID.Hex()}, &rollbackReply)
		app.errorJSON(w, err)
		return
	}

	app.writeJSON(w, http.StatusCreated, jsonResponse{Error: false, Message: "Project created.", Data: project})
}

func (app *Config) GetProject(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	userLogin := userLoginFromContext(r)

	client, err := rpc.Dial("tcp", loggerRPCAddr())
	if err != nil {
		app.errorJSON(w, err)
		return
	}
	defer client.Close()

	role, err := getProjectRole(client, id, userLogin)
	if err != nil {
		app.errorJSON(w, err)
		return
	}
	if role == "" {
		app.denyProjectAccess(w, client, id)
		return
	}

	var project data.Project
	if err := client.Call("RPCServer.GetProject", &data.RPCProjectIDArgs{ID: id}, &project); err != nil {
		app.errorJSON(w, err)
		return
	}

	app.writeJSON(w, http.StatusOK, jsonResponse{Error: false, Message: "OK!", Data: project})
}

func (app *Config) UpdateProject(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	userLogin := userLoginFromContext(r)

	var body struct {
		Name string `json:"name"`
		Slug string `json:"slug"`
	}
	if err := app.readJSON(w, r, &body); err != nil {
		app.errorJSON(w, err)
		return
	}

	if body.Name == "" {
		app.errorJSON(w, fmt.Errorf("name is required"), http.StatusBadRequest)
		return
	}
	if !data.ValidSlug(body.Slug) {
		app.errorJSON(w, fmt.Errorf("invalid slug"), http.StatusBadRequest)
		return
	}

	client, err := rpc.Dial("tcp", loggerRPCAddr())
	if err != nil {
		app.errorJSON(w, err)
		return
	}
	defer client.Close()

	role, err := getProjectRole(client, id, userLogin)
	if err != nil {
		app.errorJSON(w, err)
		return
	}
	if role == "" {
		app.denyProjectAccess(w, client, id)
		return
	}
	if role != data.RoleOwner {
		app.errorJSON(w, fmt.Errorf("forbidden"), http.StatusForbidden)
		return
	}

	var project data.Project
	if err := client.Call("RPCServer.UpdateProject", &data.RPCUpdateProjectArgs{ID: id, Name: body.Name, Slug: body.Slug}, &project); err != nil {
		app.errorJSON(w, err)
		return
	}

	app.writeJSON(w, http.StatusOK, jsonResponse{Error: false, Message: "Project updated.", Data: project})
}

func (app *Config) DeleteProject(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	userLogin := userLoginFromContext(r)

	client, err := rpc.Dial("tcp", loggerRPCAddr())
	if err != nil {
		app.errorJSON(w, err)
		return
	}
	defer client.Close()

	role, err := getProjectRole(client, id, userLogin)
	if err != nil {
		app.errorJSON(w, err)
		return
	}
	if role == "" {
		app.denyProjectAccess(w, client, id)
		return
	}
	if role != data.RoleOwner {
		app.errorJSON(w, fmt.Errorf("forbidden"), http.StatusForbidden)
		return
	}

	var reply string
	if err := client.Call("RPCServer.DeleteProject", &data.RPCProjectIDArgs{ID: id}, &reply); err != nil {
		app.errorJSON(w, err)
		return
	}

	app.writeJSON(w, http.StatusOK, jsonResponse{Error: false, Message: "Project deleted."})
}

func (app *Config) ListProjectMembers(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	userLogin := userLoginFromContext(r)

	client, err := rpc.Dial("tcp", loggerRPCAddr())
	if err != nil {
		app.errorJSON(w, err)
		return
	}
	defer client.Close()

	// Single ListMembers call: reuse the result for both the access check and the response.
	args := data.ProjectArgs{ProjectID: id}
	var members []data.ProjectMember
	if err := client.Call("RPCServer.ListMembers", &args, &members); err != nil {
		app.errorJSON(w, err)
		return
	}

	isMember := false
	for _, m := range members {
		if m.GithubLogin == userLogin {
			isMember = true
			break
		}
	}
	if !isMember {
		app.denyProjectAccess(w, client, id)
		return
	}

	app.writeJSON(w, http.StatusOK, jsonResponse{Error: false, Message: "OK!", Data: members})
}

func (app *Config) AddProjectMember(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	userLogin := userLoginFromContext(r)

	var body struct {
		Login string `json:"login"`
		Role  string `json:"role"`
	}
	if err := app.readJSON(w, r, &body); err != nil {
		app.errorJSON(w, err)
		return
	}

	if body.Login == "" {
		app.errorJSON(w, fmt.Errorf("login is required"), http.StatusBadRequest)
		return
	}
	if !data.ValidRole(body.Role) {
		app.errorJSON(w, fmt.Errorf("invalid role"), http.StatusBadRequest)
		return
	}

	client, err := rpc.Dial("tcp", loggerRPCAddr())
	if err != nil {
		app.errorJSON(w, err)
		return
	}
	defer client.Close()

	role, err := getProjectRole(client, id, userLogin)
	if err != nil {
		app.errorJSON(w, err)
		return
	}
	if role == "" {
		app.denyProjectAccess(w, client, id)
		return
	}
	if role != data.RoleOwner {
		app.errorJSON(w, fmt.Errorf("forbidden"), http.StatusForbidden)
		return
	}

	var reply string
	if err := client.Call("RPCServer.AddMember", &data.RPCAddMemberArgs{
		ProjectID:   id,
		GithubLogin: body.Login,
		Role:        body.Role,
	}, &reply); err != nil {
		app.errorJSON(w, err)
		return
	}

	app.writeJSON(w, http.StatusCreated, jsonResponse{Error: false, Message: "Member added."})
}

func (app *Config) RemoveProjectMember(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	login := chi.URLParam(r, "login")
	userLogin := userLoginFromContext(r)

	client, err := rpc.Dial("tcp", loggerRPCAddr())
	if err != nil {
		app.errorJSON(w, err)
		return
	}
	defer client.Close()

	role, err := getProjectRole(client, id, userLogin)
	if err != nil {
		app.errorJSON(w, err)
		return
	}
	if role == "" {
		app.denyProjectAccess(w, client, id)
		return
	}
	if role != data.RoleOwner {
		app.errorJSON(w, fmt.Errorf("forbidden"), http.StatusForbidden)
		return
	}

	var reply string
	if err := client.Call("RPCServer.RemoveMember", &data.RPCRemoveMemberArgs{
		ProjectID:   id,
		GithubLogin: login,
	}, &reply); err != nil {
		// net/rpc transmits errors as strings, so errors.Is won't work across the
		// wire — string matching is the only way to detect data.ErrLastOwner here.
		if strings.Contains(err.Error(), "last owner") {
			app.errorJSON(w, fmt.Errorf("cannot remove the last owner"), http.StatusBadRequest)
			return
		}
		app.errorJSON(w, err)
		return
	}

	app.writeJSON(w, http.StatusOK, jsonResponse{Error: false, Message: "Member removed."})
}

// --- Project-scoped log access ---

// The dashboard reads and writes events through the routes below instead of the
// public SDK ones. It authenticates as a user rather than as an API key, so the
// project cannot come from a key: it comes from the path, and the caller has to
// be a member of it.

// requireProjectAccess confirms the caller belongs to the project named in the
// path, over the connection the handler already opened. It returns false once a
// response has been written, so the handler only has to return — and the client
// stays the handler's to close.
func (app *Config) requireProjectAccess(w http.ResponseWriter, r *http.Request, client *rpc.Client, projectID string) bool {
	isMember, err := checkProjectMembership(client, projectID, userLoginFromContext(r))
	if err != nil {
		app.errorJSON(w, err)
		return false
	}
	if !isMember {
		app.denyProjectAccess(w, client, projectID)
		return false
	}

	return true
}

// logNotFound reports whether an RPC error means "this project has no such log".
// net/rpc flattens errors into strings, so the cause has to be read back out of
// the message. A malformed id is not found either — it cannot name a document.
func logNotFound(err error) bool {
	msg := err.Error()
	return strings.Contains(msg, "no documents in result") || strings.Contains(msg, "not a valid ObjectID")
}

func (app *Config) ListProjectLogs(w http.ResponseWriter, r *http.Request) {
	projectID := chi.URLParam(r, "id")

	client, err := rpc.Dial("tcp", loggerRPCAddr())
	if err != nil {
		app.errorJSON(w, err)
		return
	}
	defer client.Close()

	if !app.requireProjectAccess(w, r, client, projectID) {
		return
	}

	var result []data.LogEntry
	err = client.Call("RPCServer.GetLogs", data.QueryParams{
		ProjectID:  projectID,
		Pagination: paginationFromQuery(r.URL.Query()),
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

func (app *Config) GetProjectLog(w http.ResponseWriter, r *http.Request) {
	projectID := chi.URLParam(r, "id")

	client, err := rpc.Dial("tcp", loggerRPCAddr())
	if err != nil {
		app.errorJSON(w, err)
		return
	}
	defer client.Close()

	if !app.requireProjectAccess(w, r, client, projectID) {
		return
	}

	filter := data.RPCLogEntryFilter{ID: chi.URLParam(r, "logID"), ProjectID: projectID}

	var entry data.LogEntry
	if err := client.Call("RPCServer.GetLog", filter, &entry); err != nil {
		if logNotFound(err) {
			app.errorJSON(w, fmt.Errorf("log not found"), http.StatusNotFound)
			return
		}
		app.errorJSON(w, err)
		return
	}

	app.writeJSON(w, http.StatusOK, jsonResponse{Error: false, Message: "OK!", Data: entry})
}

func (app *Config) CreateProjectLog(w http.ResponseWriter, r *http.Request) {
	projectID := chi.URLParam(r, "id")

	client, err := rpc.Dial("tcp", loggerRPCAddr())
	if err != nil {
		app.errorJSON(w, err)
		return
	}
	defer client.Close()

	if !app.requireProjectAccess(w, r, client, projectID) {
		return
	}

	var payload data.JSONLogPayload
	if err := app.readJSON(w, r, &payload); err != nil {
		app.errorJSON(w, err)
		return
	}

	// Whatever project the body named is discarded: the event belongs to the one
	// in the path, which the caller was just checked against.
	payload.ProjectID = projectID

	evp := event.Payload{Action: "log", Log: payload}

	emitter, err := event.NewEmitter(app.Rabbit)
	if err != nil {
		app.errorJSON(w, err)
		return
	}

	j, err := json.MarshalIndent(&evp, "", "	")
	if err != nil {
		app.errorJSON(w, err)
		return
	}

	if err := emitter.Push(string(j), "log.INFO"); err != nil {
		app.errorJSON(w, err)
		return
	}

	app.writeJSON(w, http.StatusAccepted, jsonResponse{Error: false, Message: "OK!"})
}

func (app *Config) DeleteProjectLog(w http.ResponseWriter, r *http.Request) {
	projectID := chi.URLParam(r, "id")

	client, err := rpc.Dial("tcp", loggerRPCAddr())
	if err != nil {
		app.errorJSON(w, err)
		return
	}
	defer client.Close()

	if !app.requireProjectAccess(w, r, client, projectID) {
		return
	}

	filter := data.RPCLogEntryFilter{ID: chi.URLParam(r, "logID"), ProjectID: projectID}

	var deleted int64
	if err := client.Call("RPCServer.DeleteLog", filter, &deleted); err != nil {
		if logNotFound(err) {
			app.errorJSON(w, fmt.Errorf("log not found"), http.StatusNotFound)
			return
		}
		app.errorJSON(w, err)
		return
	}

	// A log of another project matches the filter no better than a deleted one,
	// so both land here rather than reporting a successful delete of nothing.
	if deleted == 0 {
		app.errorJSON(w, fmt.Errorf("log not found"), http.StatusNotFound)
		return
	}

	app.writeJSON(w, http.StatusOK, jsonResponse{Error: false, Message: fmt.Sprintf("Deleted entries: %d", deleted)})
}
