package core

import (
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/nixmade/orchestrator/response"
)

func (app *App) orchestrate(w http.ResponseWriter, r *http.Request) {
	defer r.Body.Close()
	namespace := chi.URLParam(r, "namespace")
	entity := chi.URLParam(r, "entity")

	var clientTargets []*ClientState
	var err error

	if err := json.NewDecoder(r.Body).Decode(&clientTargets); err != nil {
		response.Error(w, http.StatusBadRequest, err.Error())
		return
	}

	clientTargets, err = app.e.Orchestrate(namespace, entity, clientTargets)

	if err != nil {
		response.Error(w, http.StatusBadRequest, err.Error())
		return
	}

	response.JSON(w, http.StatusOK, clientTargets)
}

func (app *App) reportCurrentStatus(w http.ResponseWriter, r *http.Request) {
	defer r.Body.Close()
	namespace := chi.URLParam(r, "namespace")
	entity := chi.URLParam(r, "entity")

	var clientTargets []*ClientState

	if err := json.NewDecoder(r.Body).Decode(&clientTargets); err != nil {
		response.Error(w, http.StatusBadRequest, err.Error())
		return
	}

	if err := app.e.OrchestrateAsync(namespace, entity, clientTargets); err != nil {
		response.Error(w, http.StatusBadRequest, err.Error())
		return
	}

	response.OK(w, "ok")
}

func (app *App) getClientState(w http.ResponseWriter, r *http.Request) {
	namespace := chi.URLParam(r, "namespace")
	entity := chi.URLParam(r, "entity")

	clientTargets, err := app.e.GetClientState(namespace, entity)

	if err != nil {
		response.Error(w, http.StatusBadRequest, err.Error())
		return
	}

	response.JSON(w, http.StatusOK, clientTargets)
}

func (app *App) getClientGroupState(w http.ResponseWriter, r *http.Request) {
	namespace := chi.URLParam(r, "namespace")
	entity := chi.URLParam(r, "entity")
	group := chi.URLParam(r, "group")

	clientTargets, err := app.e.GetClientGroupState(namespace, entity, group)

	if err != nil {
		response.Error(w, http.StatusBadRequest, err.Error())
		return
	}

	response.JSON(w, http.StatusOK, clientTargets)
}

func (app *App) getNamespaces(w http.ResponseWriter, r *http.Request) {
	namespaces, err := app.e.GetNamespaces()

	if err != nil {
		response.Error(w, http.StatusBadRequest, err.Error())
		return
	}

	response.JSON(w, http.StatusOK, namespaces)
}

func (app *App) getEntities(w http.ResponseWriter, r *http.Request) {
	namespace := chi.URLParam(r, "namespace")

	entities, err := app.e.GetEntites(namespace)

	if err != nil {
		response.Error(w, http.StatusBadRequest, err.Error())
		return
	}

	response.JSON(w, http.StatusOK, entities)
}

func (app *App) getRolloutInfo(w http.ResponseWriter, r *http.Request) {
	namespace := chi.URLParam(r, "namespace")
	entity := chi.URLParam(r, "entity")

	rollout, err := app.e.GetRolloutInfo(namespace, entity)

	if err != nil {
		response.Error(w, http.StatusBadRequest, err.Error())
		return
	}

	response.JSON(w, http.StatusOK, rollout)
}
