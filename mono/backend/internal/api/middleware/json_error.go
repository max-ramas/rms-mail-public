package middleware

import (
	"encoding/json"
	"log/slog"
	"net/http"
)

type apiError struct {
	Error string `json:"error"`
}

// WriteJSONError writes a JSON error envelope for middleware responses.
func WriteJSONError(w http.ResponseWriter, status int, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(apiError{Error: message})
}

// WriteInternalError logs the underlying error and returns a generic message.
func WriteInternalError(w http.ResponseWriter, r *http.Request, err error) {
	if err != nil {
		slog.Error("internal error", "error", err, "path", r.URL.Path)
	}
	WriteJSONError(w, http.StatusInternalServerError, "internal error")
}
