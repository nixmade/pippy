package response

import (
	"encoding/json"
	"net/http"
)

func Error(w http.ResponseWriter, code int, message string) {
	JSON(w, code, map[string]string{"status": "error", "message": message})
}

func OK(w http.ResponseWriter, message string) {
	JSON(w, http.StatusOK, map[string]string{"status": "success", "message": message})
}

func JSON(w http.ResponseWriter, code int, response interface{}) {
	w.Header().Set("Content-Type", "application/json")
	bytes, err := json.Marshal(response)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	w.WriteHeader(code)
	if _, err := w.Write(bytes); err != nil {
		return
	}
}
