package main

import (
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
		r.Get("/projects/{id}", app.GetProject)
		r.Patch("/projects/{id}", app.UpdateProject)
		r.Delete("/projects/{id}", app.DeleteProject)
		r.Get("/projects/{id}/members", app.ListProjectMembers)
		r.Post("/projects/{id}/members", app.AddProjectMember)
		r.Delete("/projects/{id}/members/{login}", app.RemoveProjectMember)
	})

	// Protected routes
	mux.Group(func(r chi.Router) {
		r.Use(app.requireAPIKey)
		r.Post("/logs", app.CreateLog)
		r.Post("/logs/batch", app.CreateLogBatch)
		r.Get("/logs", app.GetLogs)
		r.Delete("/logs", app.DeleteLog)
	})

	return mux
}
