package core

import (
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/nixmade/orchestrator/response"
)

func (app *App) setTargetVersion(w http.ResponseWriter, r *http.Request) {
	defer r.Body.Close()
	namespace := chi.URLParam(r, "namespace")
	entity := chi.URLParam(r, "entity")

	var targetVersion EntityTargetVersion
	if err := json.NewDecoder(r.Body).Decode(&targetVersion); err != nil {
		response.Error(w, http.StatusBadRequest, err.Error())
		return
	}

	if err := app.e.SetTargetVersion(namespace, entity, targetVersion); err != nil {
		response.Error(w, http.StatusBadRequest, err.Error())
		return
	}
	response.OK(w, "ok")
}
