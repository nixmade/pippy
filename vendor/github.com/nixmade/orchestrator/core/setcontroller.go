package core

import (
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/nixmade/orchestrator/response"
)

func (app *App) setEntityTargetController(w http.ResponseWriter, r *http.Request) {
	defer r.Body.Close()
	namespace := chi.URLParam(r, "namespace")
	entity := chi.URLParam(r, "entity")

	entityController := &EntityWebTargetController{}
	if err := json.NewDecoder(r.Body).Decode(entityController); err != nil {
		response.Error(w, http.StatusBadRequest, err.Error())
		return
	}

	if err := app.e.SetEntityTargetController(namespace, entity, entityController); err != nil {
		response.Error(w, http.StatusBadRequest, err.Error())
		return
	}
	response.OK(w, "ok")
}

func (app *App) setEntityMonitoringController(w http.ResponseWriter, r *http.Request) {
	defer r.Body.Close()
	namespace := chi.URLParam(r, "namespace")
	entity := chi.URLParam(r, "entity")

	entityController := &EntityWebMonitoringController{}
	if err := json.NewDecoder(r.Body).Decode(entityController); err != nil {
		response.Error(w, http.StatusBadRequest, err.Error())
		return
	}

	if err := app.e.SetEntityMonitoringController(namespace, entity, entityController); err != nil {
		response.Error(w, http.StatusBadRequest, err.Error())
		return
	}
	response.OK(w, "ok")
}
