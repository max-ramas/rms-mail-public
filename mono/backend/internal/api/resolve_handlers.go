package api

import (
	"encoding/json"
	"net/http"

	"rmsmail/internal/mail"
)

// ResolveMailServer handles GET /api/mail/resolve
// It manually resolves mail server settings for a given email address and returns them as JSON.
func (h *Handler) ResolveMailServer(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		WriteJSONError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	email := r.URL.Query().Get("email")
	if email == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{
			"error": "email parameter is required",
		})
		return
	}

	settings, err := mail.Resolve(r.Context(), email)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{
			"error": "resolution_failed",
			"code":  "ERROR_RESOLUTION_FAILED",
		})
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(settings)
}
