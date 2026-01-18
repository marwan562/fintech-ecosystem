package jsonutil

import (
	"encoding/json"
	"log"
	"net/http"
)

// WriteJSON writes a JSON response with the given status code and data.
func WriteJSON(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}

// WriteErrorJSON writes a JSON error response with a standard error format.
func WriteErrorJSON(w http.ResponseWriter, errMsg string) {
	log.Printf("Error: %s", errMsg)
	WriteJSON(w, http.StatusBadRequest, map[string]string{"error": errMsg})
}
