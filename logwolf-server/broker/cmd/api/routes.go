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
		AllowedHeaders:   []string{"Accept", "Authorization", "Content-Type", "X-CSRF-Token"},
		ExposedHeaders:   []string{"Link"},
		AllowCredentials: true,
		MaxAge:           300,
	}))

	mux.Use(middleware.Heartbeat("/ping"))

	mux.Get("/health", app.Health)

	// Key management — dashboard only, no API key required
	mux.Group(func(r chi.Router) {
		r.Use(app.requireInternalSecret)
		r.Get("/keys", app.ListAPIKeys)
		r.Post("/keys", app.CreateAPIKey)
		r.Delete("/keys/{id}", app.RevokeAPIKey)
		r.Get("/settings/retention", app.GetRetention)
		r.Patch("/settings/retention", app.UpdateRetention)
		r.Get("/metrics", app.GetMetrics)
	})

	// Protected routes
	mux.Group(func(r chi.Router) {
		r.Use(app.requireAPIKey)
		r.Post("/logs", app.CreateLog)
		r.Get("/logs", app.GetLogs)
		r.Delete("/logs", app.DeleteLog)
	})

	return mux
}
