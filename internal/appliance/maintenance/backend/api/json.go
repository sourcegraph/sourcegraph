package api

import (
	"encoding/json"
	"net/http"
)

func sendJson(w http.ResponseWriter, result interface{}) {
	response, err := json.Marshal(result)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.Write(response)
}

func receiveJson(w http.ResponseWriter, r *http.Request, result interface{}) {
	err := json.NewDecoder(r.Body).Decode(result)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
}
