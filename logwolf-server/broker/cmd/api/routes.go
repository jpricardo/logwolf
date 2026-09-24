package main

import (
	"logwolf-toolbox/data"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"
)

func (app *Config) routes() http.Handler {
	mux := chi.NewRouter()

	mux.Use(cors.Handler(cors.Options{
		AllowedOrigins:   []string{"https://*", "http://*"},
		AllowedMethods:   []string{"GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"},
		AllowedHeaders:   []string{"Accept", "Authorization", "Content-Type", "X-CSRF-Token", "X-Internal-Secret", "X-User-Login"},
		ExposedHeaders:   []string{"Link"},
		AllowCredentials: true,
		MaxAge:           300,
	}))

	mux.Use(middleware.Heartbeat("/ping"))

	mux.Get("/health", app.Health)

	// Key management — dashboard only, no API key required
	mux.Group(func(r chi.Router) {
		r.Use(app.requireInternalSecret)
		r.Use(app.requireUserLogin)
		r.Get("/keys", app.ListAPIKeys)
		r.Post("/keys", app.CreateAPIKey)
		r.Delete("/keys/{id}", app.RevokeAPIKey)
		r.Get("/settings/retention", app.GetRetention)
		r.Patch("/settings/retention", app.UpdateRetention)
		r.Get("/metrics", app.GetMetrics)
		r.Get("/projects", app.ListProjects)
		r.Post("/projects", app.CreateProject)

		// Each project route states who may use it; requireProject answers 404
		// or 403 for everyone else, before the handler runs.
		member, owner := app.requireProject(anyMember), app.requireProject(ownerOnly)
		r.With(member).Get("/projects/{id}", app.GetProject)
		r.With(owner).Patch("/projects/{id}", app.UpdateProject)
		r.With(owner).Delete("/projects/{id}", app.DeleteProject)
		r.With(member).Get("/projects/{id}/members", app.ListProjectMembers)
		r.With(owner).Post("/projects/{id}/members", app.AddProjectMember)
		r.With(owner).Patch("/projects/{id}/members/{login}", app.UpdateProjectMemberRole)
		r.With(owner).Delete("/projects/{id}/members/{login}", app.RemoveProjectMember)
		r.With(member).Get("/projects/{id}/logs", app.ListProjectLogs)
		r.With(member).Post("/projects/{id}/logs", app.CreateProjectLog)
		r.With(member).Get("/projects/{id}/logs/{logID}", app.GetProjectLog)
		r.With(member).Delete("/projects/{id}/logs/{logID}", app.DeleteProjectLog)
	})

	// Protected routes — each also needs its scope on the key
	mux.Group(func(r chi.Router) {
		r.Use(app.requireAPIKey)
		r.With(app.requireScope(data.ScopeIngest)).Post("/logs", app.CreateLog)
		r.With(app.requireScope(data.ScopeIngest)).Post("/logs/batch", app.CreateLogBatch)
		r.With(app.requireScope(data.ScopeRead)).Get("/logs", app.GetLogs)
		r.With(app.requireScope(data.ScopeDelete)).Delete("/logs", app.DeleteLog)
	})

	return mux
}
