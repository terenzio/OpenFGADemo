package httpapi

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/terenzio/OpenFGADemo/internal/auth"
)

// ErrorResponse is the standard JSON error envelope.
type ErrorResponse struct {
	Error string `json:"error"`
}

// WriteJSON writes v as JSON with the given HTTP status code.
func WriteJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}

// WriteError maps known sentinel errors to HTTP status codes and writes a JSON
// error response.
func WriteError(w http.ResponseWriter, err error) {
	if errors.Is(err, auth.ErrForbidden) {
		WriteJSON(w, http.StatusForbidden, ErrorResponse{Error: "forbidden"})
		return
	}
	WriteJSON(w, http.StatusInternalServerError, ErrorResponse{Error: err.Error()})
}
