package core

import (
	"net/http"

	"github.com/go-chi/chi/v5/middleware"
	"github.com/nixmade/orchestrator/server"
)

// Route defines registration of different routes supported by app
type Route struct {
	Method   string
	RouteURI string
}

// NewRouter registers multiple logged routes
func NewRouter(app *App) http.Handler {
	router := server.DefaultRouter()
	router.Mount("/v1/orchestrate", app.Orchestrator())
	router.Mount("/orchestrator/profiler", middleware.Profiler())

	return http.Handler(router)
}
