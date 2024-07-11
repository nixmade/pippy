package core

import (
	"net/http"

	"github.com/go-chi/chi/v5"
)

// Orchestrator Creates a new orchestrator router
func (app *App) Orchestrator() http.Handler {
	r := chi.NewRouter()

	r.Post("/{namespace}/{entity}", app.orchestrate)
	r.Post("/{namespace}/{entity}/version", app.setTargetVersion)
	r.Post("/{namespace}/{entity}/options", app.setRolloutOptions)
	r.Post("/{namespace}/{entity}/target/controller", app.setEntityTargetController)
	r.Post("/{namespace}/{entity}/monitoring/controller", app.setEntityMonitoringController)
	r.Post("/{namespace}/{entity}/status", app.reportCurrentStatus)
	r.Get("/namespaces", app.getNamespaces)
	r.Get("/{namespace}/entities", app.getEntities)
	r.Get("/{namespace}/{entity}/rollout", app.getRolloutInfo)
	r.Get("/{namespace}/{entity}/targets", app.getClientState)
	r.Get("/{namespace}/{entity}/status", app.getClientState)
	r.Get("/{namespace}/{entity}/{group}/status", app.getClientGroupState)
	return r
}
