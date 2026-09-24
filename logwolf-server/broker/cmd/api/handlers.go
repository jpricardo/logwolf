package main

import (
	"encoding/json"
	"fmt"
	"logwolf-toolbox/data"
	"logwolf-toolbox/event"
	"net"
	"net/http"
	"net/rpc"
	"net/url"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"
	"go.mongodb.org/mongo-driver/bson/primitive"
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
	client, ok := app.dialLogger(w)
	if !ok {
		return
	}
	defer client.Close()

	var result []data.LogEntry
	err := client.Call("RPCServer.GetLogs", data.QueryParams{
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

	client, ok := app.dialLogger(w)
	if !ok {
		return
	}
	defer client.Close()

	var result int64
	err = client.Call("RPCServer.DeleteLog", data.RPCLogEntryFilter(requestBody), &result)
	if err != nil {
		app.errorJSON(w, err)
		return
	}

	app.writeJSON(w, http.StatusAccepted, jsonResponse{Error: false, Message: "OK!", Data: fmt.Sprintf("Deleted entries: %d", result)})
}

// --- Project keys, retention and metrics ---
//
// Behind requireProject, like the project routes below: the project is the one
// in the path.

func (app *Config) ListAPIKeys(w http.ResponseWriter, r *http.Request) {
	p := projectFromContext(r)

	var keys []data.APIKey
	if err := p.client.Call("RPCServer.ListAPIKeys", &data.ProjectArgs{ProjectID: p.id}, &keys); err != nil {
		app.rpcErrorJSON(w, err, nil)
		return
	}

	if keys == nil {
		keys = []data.APIKey{}
	}

	app.writeJSON(w, http.StatusOK, jsonResponse{Error: false, Message: "OK!", Data: keys})
}

func (app *Config) CreateAPIKey(w http.ResponseWriter, r *http.Request) {
	p := projectFromContext(r)

	var body struct {
		// Scopes left out, or empty, mean data.DefaultScopes: ingest only.
		Scopes []string `json:"scopes"`
	}
	if err := app.readJSON(w, r, &body); err != nil {
		app.errorJSON(w, err)
		return
	}

	scopes, err := data.NormalizeScopes(body.Scopes)
	if err != nil {
		app.errorJSON(w, err, http.StatusBadRequest)
		return
	}

	var created data.RPCCreateAPIKeyReply
	args := data.RPCCreateAPIKeyArgs{ProjectID: p.id, Scopes: scopes}
	if err := p.client.Call("RPCServer.CreateAPIKey", &args, &created); err != nil {
		app.rpcErrorJSON(w, err, nil)
		return
	}

	// Return plaintext only once — it is never stored and cannot be recovered
	app.writeJSON(w, http.StatusCreated, jsonResponse{
		Error:   false,
		Message: "API key created. Copy it now — it will not be shown again.",
		Data: map[string]any{
			"key":    created.Plaintext,
			"prefix": created.Key.Prefix,
			"id":     created.Key.ID.Hex(),
			"scopes": created.Key.Scopes,
		},
	})
}

// RevokeAPIKey revokes a key of the project in the path. The logger matches
// the project as well as the key id, so another project's key is a 404, the
// same as one that does not exist.
func (app *Config) RevokeAPIKey(w http.ResponseWriter, r *http.Request) {
	p := projectFromContext(r)

	keyID, err := primitive.ObjectIDFromHex(chi.URLParam(r, "keyID"))
	if err != nil {
		app.errorJSON(w, fmt.Errorf("key not found"), http.StatusNotFound)
		return
	}

	var reply string
	args := data.RPCRevokeAPIKeyArgs{ProjectID: p.id, ID: keyID.Hex()}
	if err := p.client.Call("RPCServer.RevokeAPIKey", &args, &reply); err != nil {
		app.rpcErrorJSON(w, err, keyNotFound)
		return
	}
	forgetCachedKey(keyID.Hex())
	app.writeJSON(w, http.StatusOK, jsonResponse{Error: false, Message: "Key revoked."})
}

type retentionResponse struct {
	Days int `json:"days"`
}

func (app *Config) GetRetention(w http.ResponseWriter, r *http.Request) {
	p := projectFromContext(r)

	var days int
	if err := p.client.Call("RPCServer.GetRetention", &data.RetentionArgs{ProjectID: p.id}, &days); err != nil {
		app.rpcErrorJSON(w, err, nil)
		return
	}

	app.writeJSON(w, http.StatusOK, jsonResponse{Error: false, Data: retentionResponse{Days: days}})
}

func (app *Config) UpdateRetention(w http.ResponseWriter, r *http.Request) {
	p := projectFromContext(r)

	var payload struct {
		// A pointer, because a missing days would otherwise read as 0: forever.
		Days *int `json:"days"`
	}
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		app.errorJSON(w, err)
		return
	}

	if payload.Days == nil {
		app.errorJSON(w, fmt.Errorf("days is required"), http.StatusBadRequest)
		return
	}
	// The logger refuses these too, but checking here spares the round trip and
	// the string matching to tell its refusal apart from a failure.
	if !data.ValidRetentionDays[*payload.Days] {
		app.errorJSON(w, fmt.Errorf("invalid retention: %d days", *payload.Days), http.StatusBadRequest)
		return
	}

	// Any member may keep logs longer; shortening retention deletes whatever
	// falls outside the new window on the next cleanup pass, so that is an
	// owner's call.
	if p.role != data.RoleOwner {
		var current int
		if err := p.client.Call("RPCServer.GetRetention", &data.RetentionArgs{ProjectID: p.id}, &current); err != nil {
			app.rpcErrorJSON(w, err, nil)
			return
		}
		if data.LowersRetention(current, *payload.Days) {
			app.errorJSON(w, fmt.Errorf("only an owner can lower retention"), http.StatusForbidden)
			return
		}
	}

	args := data.RetentionArgs{ProjectID: p.id, Days: *payload.Days}
	var reply string
	if err := p.client.Call("RPCServer.UpdateRetention", &args, &reply); err != nil {
		app.rpcErrorJSON(w, err, nil)
		return
	}

	app.writeJSON(w, http.StatusOK, jsonResponse{Error: false, Data: retentionResponse{Days: *payload.Days}})
}

func (app *Config) GetMetrics(w http.ResponseWriter, r *http.Request) {
	p := projectFromContext(r)

	var result data.Metrics
	if err := p.client.Call("RPCServer.GetMetrics", &data.ProjectArgs{ProjectID: p.id}, &result); err != nil {
		app.rpcErrorJSON(w, err, nil)
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
	if rabbitmq.Status != "up" || logger.Status != "up" {
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

// loggerHealthTimeout bounds the whole logger check: dial and Status call.
const loggerHealthTimeout = 2 * time.Second

// checkLogger asks the logger for its status. It is "down" if the logger cannot
// be reached or does not answer in time, and "degraded" while its startup tasks
// are failing: it serves then, but its startup migration or indexes are not
// done, and it retries them in the background.
func checkLogger() serviceStatus {
	conn, err := net.DialTimeout("tcp", loggerRPCAddr(), loggerHealthTimeout)
	if err != nil {
		return serviceStatus{Status: "down", Error: err.Error()}
	}
	client := rpc.NewClient(conn)
	defer client.Close()

	var st data.LoggerStatus
	call := client.Go("RPCServer.Status", "", &st, nil)
	timer := time.NewTimer(loggerHealthTimeout)
	defer timer.Stop()
	select {
	case <-call.Done:
		if call.Error != nil {
			return serviceStatus{Status: "down", Error: call.Error.Error()}
		}
	case <-timer.C:
		return serviceStatus{Status: "down", Error: "logger status timed out"}
	}

	if !st.Ready {
		return serviceStatus{Status: "degraded", Error: "startup tasks failing, retrying: " + st.StartupError}
	}
	return serviceStatus{Status: "up"}
}

// --- Project management ---

func (app *Config) ListProjects(w http.ResponseWriter, r *http.Request) {
	client, ok := app.dialLogger(w)
	if !ok {
		return
	}
	defer client.Close()

	args := data.RPCUserProjectsArgs{GithubLogin: userLoginFromContext(r)}
	var projects []data.UserProject
	if err := client.Call("RPCServer.ListUserProjects", &args, &projects); err != nil {
		app.rpcErrorJSON(w, err, nil)
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

	client, ok := app.dialLogger(w)
	if !ok {
		return
	}
	defer client.Close()

	// One call: the logger writes the project and the caller's owner membership
	// in one transaction, so a failure leaves no project nobody can reach.
	// Slugs are not unique, so there is no collision to report.
	var project data.Project
	args := data.RPCCreateProjectArgs{Name: body.Name, Slug: body.Slug, Owner: userLoginFromContext(r)}
	if err := client.Call("RPCServer.CreateProject", &args, &project); err != nil {
		app.rpcErrorJSON(w, err, nil)
		return
	}

	app.writeJSON(w, http.StatusCreated, jsonResponse{Error: false, Message: "Project created.", Data: project})
}

// The handlers from here on sit behind requireProject, which has checked the
// caller's access to the project in the path (routes.go says to which level)
// and hands them the project and an open logger connection.

func (app *Config) GetProject(w http.ResponseWriter, r *http.Request) {
	p := projectFromContext(r)

	var project data.Project
	if err := p.client.Call("RPCServer.GetProject", &data.RPCProjectIDArgs{ID: p.id}, &project); err != nil {
		app.rpcErrorJSON(w, err, projectNotFound)
		return
	}

	app.writeJSON(w, http.StatusOK, jsonResponse{Error: false, Message: "OK!", Data: project})
}

func (app *Config) UpdateProject(w http.ResponseWriter, r *http.Request) {
	p := projectFromContext(r)

	// Only the name changes: the slug is fixed when the project is created.
	var body struct {
		Name string `json:"name"`
	}
	if err := app.readJSON(w, r, &body); err != nil {
		app.errorJSON(w, err)
		return
	}

	if body.Name == "" {
		app.errorJSON(w, fmt.Errorf("name is required"), http.StatusBadRequest)
		return
	}

	var project data.Project
	if err := p.client.Call("RPCServer.UpdateProject", &data.RPCUpdateProjectArgs{ID: p.id, Name: body.Name}, &project); err != nil {
		app.rpcErrorJSON(w, err, projectNotFound)
		return
	}

	app.writeJSON(w, http.StatusOK, jsonResponse{Error: false, Message: "Project updated.", Data: project})
}

func (app *Config) DeleteProject(w http.ResponseWriter, r *http.Request) {
	p := projectFromContext(r)

	var reply string
	if err := p.client.Call("RPCServer.DeleteProject", &data.RPCProjectIDArgs{ID: p.id}, &reply); err != nil {
		app.rpcErrorJSON(w, err, projectNotFound)
		return
	}
	// DeleteProject deleted the project's keys along with it.
	forgetCachedProjectKeys(p.id)

	app.writeJSON(w, http.StatusOK, jsonResponse{Error: false, Message: "Project deleted."})
}

func (app *Config) ListProjectMembers(w http.ResponseWriter, r *http.Request) {
	p := projectFromContext(r)

	var members []data.ProjectMember
	if err := p.client.Call("RPCServer.ListMembers", &data.ProjectArgs{ProjectID: p.id}, &members); err != nil {
		app.rpcErrorJSON(w, err, projectNotFound)
		return
	}

	app.writeJSON(w, http.StatusOK, jsonResponse{Error: false, Message: "OK!", Data: members})
}

func (app *Config) AddProjectMember(w http.ResponseWriter, r *http.Request) {
	p := projectFromContext(r)

	var body struct {
		Login string `json:"login"`
		Role  string `json:"role"`
	}
	if err := app.readJSON(w, r, &body); err != nil {
		app.errorJSON(w, err)
		return
	}

	login := data.NormalizeGithubLogin(body.Login)
	if login == "" {
		app.errorJSON(w, fmt.Errorf("login is required"), http.StatusBadRequest)
		return
	}
	if !data.ValidRole(body.Role) {
		app.errorJSON(w, fmt.Errorf("invalid role"), http.StatusBadRequest)
		return
	}

	var reply string
	if err := p.client.Call("RPCServer.AddMember", &data.RPCAddMemberArgs{
		ProjectID:   p.id,
		GithubLogin: login,
		Role:        body.Role,
	}, &reply); err != nil {
		// A login is unique within a project, so a second add collides.
		app.rpcErrorJSON(w, err, rpcErrorMessages{
			rpcErrDuplicate: fmt.Sprintf("%s is already a member of this project", login),
			rpcErrNotFound:  "project not found",
		})
		return
	}

	app.writeJSON(w, http.StatusCreated, jsonResponse{Error: false, Message: "Member added."})
}

func (app *Config) RemoveProjectMember(w http.ResponseWriter, r *http.Request) {
	p := projectFromContext(r)
	login := data.NormalizeGithubLogin(chi.URLParam(r, "login"))

	var reply string
	if err := p.client.Call("RPCServer.RemoveMember", &data.RPCRemoveMemberArgs{
		ProjectID:   p.id,
		GithubLogin: login,
	}, &reply); err != nil {
		app.rpcErrorJSON(w, err, rpcErrorMessages{
			rpcErrLastOwner: "cannot remove the last owner",
			rpcErrNotFound:  "member not found",
		})
		return
	}

	app.writeJSON(w, http.StatusOK, jsonResponse{Error: false, Message: "Member removed."})
}

// UpdateProjectMemberRole promotes or demotes an existing member. Together with
// a self-demotion it is how an owner hands a project over, so demoting yourself
// is allowed as long as another owner remains.
func (app *Config) UpdateProjectMemberRole(w http.ResponseWriter, r *http.Request) {
	p := projectFromContext(r)
	login := data.NormalizeGithubLogin(chi.URLParam(r, "login"))

	var body struct {
		Role string `json:"role"`
	}
	if err := app.readJSON(w, r, &body); err != nil {
		app.errorJSON(w, err)
		return
	}
	if !data.ValidRole(body.Role) {
		app.errorJSON(w, fmt.Errorf("invalid role"), http.StatusBadRequest)
		return
	}

	var reply string
	if err := p.client.Call("RPCServer.UpdateMemberRole", &data.RPCUpdateMemberRoleArgs{
		ProjectID:   p.id,
		GithubLogin: login,
		Role:        body.Role,
	}, &reply); err != nil {
		app.rpcErrorJSON(w, err, rpcErrorMessages{
			rpcErrLastOwner: "cannot demote the last owner",
			rpcErrNotFound:  "member not found",
		})
		return
	}

	app.writeJSON(w, http.StatusOK, jsonResponse{Error: false, Message: "Role updated."})
}

// --- Project-scoped log access ---

// The dashboard reads and writes events through the routes below instead of the
// public SDK ones. It authenticates as a user rather than as an API key, so the
// project cannot come from a key: it comes from the path, and requireProject
// has checked the caller is a member of it.

func (app *Config) ListProjectLogs(w http.ResponseWriter, r *http.Request) {
	p := projectFromContext(r)

	var result []data.LogEntry
	err := p.client.Call("RPCServer.GetLogs", data.QueryParams{
		ProjectID:  p.id,
		Pagination: paginationFromQuery(r.URL.Query()),
	}, &result)
	if err != nil {
		app.rpcErrorJSON(w, err, nil)
		return
	}

	if result == nil {
		result = []data.LogEntry{}
	}

	app.writeJSON(w, http.StatusOK, jsonResponse{Error: false, Message: "OK!", Data: result})
}

func (app *Config) GetProjectLog(w http.ResponseWriter, r *http.Request) {
	p := projectFromContext(r)
	filter := data.RPCLogEntryFilter{ID: chi.URLParam(r, "logID"), ProjectID: p.id}

	var entry data.LogEntry
	if err := p.client.Call("RPCServer.GetLog", filter, &entry); err != nil {
		app.rpcErrorJSON(w, err, logNotFound)
		return
	}

	app.writeJSON(w, http.StatusOK, jsonResponse{Error: false, Message: "OK!", Data: entry})
}

func (app *Config) CreateProjectLog(w http.ResponseWriter, r *http.Request) {
	p := projectFromContext(r)

	var payload data.JSONLogPayload
	if err := app.readJSON(w, r, &payload); err != nil {
		app.errorJSON(w, err)
		return
	}

	// Whatever project the body named is discarded: the event belongs to the one
	// in the path, which the caller was checked against.
	payload.ProjectID = p.id

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
	p := projectFromContext(r)
	filter := data.RPCLogEntryFilter{ID: chi.URLParam(r, "logID"), ProjectID: p.id}

	var deleted int64
	if err := p.client.Call("RPCServer.DeleteLog", filter, &deleted); err != nil {
		app.rpcErrorJSON(w, err, logNotFound)
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
