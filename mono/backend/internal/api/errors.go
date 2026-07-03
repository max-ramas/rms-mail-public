package api

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"rmsmail/internal/sentry"
	"strings"
)

// APIError is the standard JSON error envelope for REST endpoints.
type APIError struct {
	Error string `json:"error"`
}

// WriteJSONError writes a JSON error response with the given HTTP status.
func WriteJSONError(w http.ResponseWriter, status int, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(APIError{Error: message})
}

// WriteInternalError logs the underlying error and returns a generic message.
func WriteInternalError(w http.ResponseWriter, r *http.Request, err error) {
	if err != nil {
		slog.Error("internal error", "error", err, "path", r.URL.Path)
		sentry.CaptureException(err)
	}
	WriteJSONError(w, http.StatusInternalServerError, "internal error")
}

// clientSafeErrors are error messages safe to return to API clients.
var clientSafeErrors = map[string]bool{
	"unauthorized":           true,
	"account_id is required": true,
	"access to system account is forbidden in Mono edition": true,
}

// ClientSafeMessage returns err.Error() for known-safe messages, otherwise a generic fallback.
func ClientSafeMessage(err error, fallback string) string {
	if err == nil {
		return fallback
	}
	msg := err.Error()
	if clientSafeErrors[msg] || strings.HasPrefix(msg, "access denied") {
		return msg
	}
	return fallback
}

// WriteAccessError writes a forbidden response without leaking internal details.
func WriteAccessError(w http.ResponseWriter, err error) {
	WriteJSONError(w, http.StatusForbidden, ClientSafeMessage(err, "forbidden"))
}

// WriteBadRequestError writes a bad-request response without leaking internal details.
func WriteBadRequestError(w http.ResponseWriter, err error) {
	msg := "bad request"
	if err != nil {
		if errors.Is(err, ErrInvalidInput) {
			msg = err.Error()
		}
	}
	WriteJSONError(w, http.StatusBadRequest, msg)
}

// ErrInvalidInput marks validation errors that are safe to expose to clients.
var ErrInvalidInput = errors.New("invalid input")
