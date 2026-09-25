package main

import (
	"context"
	"fmt"
	"logwolf-toolbox/data"
	"net/http"
	"net/rpc"

	"github.com/go-chi/chi/v5"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

// --- Project access ---
//
// Every dashboard route that touches a project names it in the path, as
// /projects/{id}/..., and answers the same way when the caller may not: 404 if
// the project does not exist (or the id cannot name one), 403 if it exists and
// the caller is not a member, or is a member where an owner is needed.
// authorizeProject is that rule; requireProject applies it to the routes.

// accessLevel is who may use a route.
type accessLevel int

const (
	anyMember accessLevel = iota
	ownerOnly
)

// projectContext is what requireProject hands the handler: the project it
// checked, the caller's role in it, and the logger connection it checked over,
// open for the handler's own calls.
type projectContext struct {
	id     string // canonical hex, as ObjectID.Hex writes it
	role   string
	client *rpc.Client
}

const projectContextKey contextKey = "project"

// projectFromContext returns what requireProject stored. Handlers behind
// requireProject can rely on it being there.
func projectFromContext(r *http.Request) *projectContext {
	p, _ := r.Context().Value(projectContextKey).(*projectContext)
	return p
}

// dialLogger opens a connection to the logger, answering the request itself if
// it cannot. The caller closes the client.
func (app *Config) dialLogger(w http.ResponseWriter) (*rpc.Client, bool) {
	client, err := rpc.Dial("tcp", loggerRPCAddr())
	if err != nil {
		app.errorJSON(w, err, http.StatusInternalServerError)
		return nil, false
	}
	return client, true
}

// authorizeProject checks the caller's access to projectID over client, and
// answers the request itself when it falls short of need. It returns the
// caller's role and whether the handler may go on.
//
// One RPC answers both questions, whether the project exists and what the
// caller's role in it is, so a denied caller costs no more than an allowed one.
func (app *Config) authorizeProject(w http.ResponseWriter, r *http.Request, client *rpc.Client, projectID string, need accessLevel) (string, bool) {
	var access data.ProjectAccess
	args := data.RPCProjectAccessArgs{ProjectID: projectID, GithubLogin: userLoginFromContext(r)}
	if err := client.Call("RPCServer.ProjectAccess", &args, &access); err != nil {
		app.rpcErrorJSON(w, err, projectNotFound)
		return "", false
	}

	switch {
	case !access.Exists:
		app.errorJSON(w, fmt.Errorf("project not found"), http.StatusNotFound)
		return "", false
	case access.Role == "":
		app.errorJSON(w, fmt.Errorf("forbidden"), http.StatusForbidden)
		return "", false
	case need == ownerOnly && access.Role != data.RoleOwner:
		app.errorJSON(w, fmt.Errorf("only an owner can do this"), http.StatusForbidden)
		return "", false
	}
	return access.Role, true
}

// requireProject guards a route whose path names the project as {id}. It
// checks the caller's access with authorizeProject, then hands the handler a
// projectContext; the logger connection in it is closed once the handler
// returns.
//
// A path id that is not an ObjectID names no project, so it is a 404 without a
// call to the logger.
func (app *Config) requireProject(need accessLevel) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			id, err := primitive.ObjectIDFromHex(chi.URLParam(r, "id"))
			if err != nil {
				app.errorJSON(w, fmt.Errorf("project not found"), http.StatusNotFound)
				return
			}

			client, ok := app.dialLogger(w)
			if !ok {
				return
			}
			defer client.Close()

			role, ok := app.authorizeProject(w, r, client, id.Hex(), need)
			if !ok {
				return
			}

			p := &projectContext{id: id.Hex(), role: role, client: client}
			next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), projectContextKey, p)))
		})
	}
}
