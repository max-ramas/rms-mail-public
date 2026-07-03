package api

import (
	"encoding/json"
	"net/http"
	"regexp"

	"rmsmail/internal/api/middleware"
	"rmsmail/internal/models"
)

type OAuthSettingsPayload struct {
	GoogleClientID        string `json:"google_client_id"`
	GoogleClientSecret    string `json:"google_client_secret"`
	MicrosoftClientID     string `json:"microsoft_client_id"`
	MicrosoftClientSecret string `json:"microsoft_client_secret"`
}

const maskedSecret = "********"

func (h *Handler) requireAdmin(w http.ResponseWriter, r *http.Request) bool {
	userID := middleware.GetUserIDFromContext(r.Context())
	if !middleware.IsAdminFromContext(r.Context()) {
		_, _, err := h.Store.GetAdminByEmail(r.Context(), userID)
		if err != nil || userID == "" {
			WriteJSONError(w, http.StatusForbidden, "admin required")
			return false
		}
	}
	return true
}

// GetOAuthSettings returns the current OAuth settings. Client secrets are masked.
func (h *Handler) GetOAuthSettings(w http.ResponseWriter, r *http.Request) {
	if !h.requireAdmin(w, r) {
		return
	}
	ctx := r.Context()

	getSetting := func(key string) string {
		val, _ := h.Store.GetSystemSetting(ctx, key)
		return val
	}

	payload := OAuthSettingsPayload{
		GoogleClientID:    getSetting("oauth_google_client_id"),
		MicrosoftClientID: getSetting("oauth_microsoft_client_id"),
	}

	if getSetting("oauth_google_client_secret") != "" {
		payload.GoogleClientSecret = maskedSecret
	}
	if getSetting("oauth_microsoft_client_secret") != "" {
		payload.MicrosoftClientSecret = maskedSecret
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(payload)
}

// UpdateOAuthSettings updates the OAuth settings in the database.
func (h *Handler) UpdateOAuthSettings(w http.ResponseWriter, r *http.Request) {
	if !h.requireAdmin(w, r) {
		return
	}
	var payload OAuthSettingsPayload
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		WriteJSONError(w, http.StatusBadRequest, "invalid JSON")
		return
	}

	ctx := r.Context()

	// Helper to update only if provided and not masked
	updateSecret := func(key, newValue string) error {
		if newValue == "" || newValue == maskedSecret {
			return nil // ignore empty or unchanged masked secret
		}
		return h.Store.SetSystemSetting(ctx, key, newValue)
	}

	if err := h.Store.SetSystemSetting(ctx, "oauth_google_client_id", payload.GoogleClientID); err != nil {
		WriteInternalError(w, r, err)
		return
	}
	if err := updateSecret("oauth_google_client_secret", payload.GoogleClientSecret); err != nil {
		WriteInternalError(w, r, err)
		return
	}
	if err := h.Store.SetSystemSetting(ctx, "oauth_microsoft_client_id", payload.MicrosoftClientID); err != nil {
		WriteInternalError(w, r, err)
		return
	}
	if err := updateSecret("oauth_microsoft_client_secret", payload.MicrosoftClientSecret); err != nil {
		WriteInternalError(w, r, err)
		return
	}

	w.WriteHeader(http.StatusOK)
	w.Write([]byte(`{"status":"success"}`))
}

// GetAICategories returns the configured AI categorization taxonomy.
func (h *Handler) GetAICategories(w http.ResponseWriter, r *http.Request) {
	if !h.requireAdmin(w, r) {
		return
	}
	raw, _ := h.Store.GetSystemSetting(r.Context(), "ai_categories")
	if raw == "" {
		raw = "[]"
	}
	w.Header().Set("Content-Type", "application/json")
	w.Write([]byte(raw))
}

// UpdateAICategories replaces the AI categorization taxonomy.
func (h *Handler) UpdateAICategories(w http.ResponseWriter, r *http.Request) {
	if !h.requireAdmin(w, r) {
		return
	}
	var categories []models.AICategory
	if err := json.NewDecoder(r.Body).Decode(&categories); err != nil {
		WriteJSONError(w, http.StatusBadRequest, "invalid JSON")
		return
	}

	// Validate categories
	colorRegex := regexp.MustCompile(`^#[0-9a-fA-F]{3,6}$`)
	for _, cat := range categories {
		if cat.Name == "" || len(cat.Name) > 100 {
			WriteJSONError(w, http.StatusBadRequest, "invalid category name: must be 1-100 characters")
			return
		}
		if cat.Color != "" && !colorRegex.MatchString(cat.Color) {
			WriteJSONError(w, http.StatusBadRequest, "invalid category color: must be a valid hex code (e.g. #FF0000)")
			return
		}
		if cat.MoveTo != "" {
			// Check if folder exists
			_, err := h.Store.GetFolderByID(r.Context(), cat.MoveTo)
			if err != nil {
				WriteJSONError(w, http.StatusBadRequest, "invalid move_to folder id: "+cat.MoveTo)
				return
			}
		}
	}

	data, err := json.Marshal(categories)
	if err != nil {
		WriteJSONError(w, http.StatusInternalServerError, "failed to marshal categories")
		return
	}
	if err := h.Store.SetSystemSetting(r.Context(), "ai_categories", string(data)); err != nil {
		WriteInternalError(w, r, err)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}
