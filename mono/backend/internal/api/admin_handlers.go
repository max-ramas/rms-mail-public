package api

import (
	"encoding/json"
	"net/http"

	"rmsmail/internal/mail"
)

type AdminSettingsPayload struct {
	AllowedDomains string `json:"allowed_domains"`
}

// GetAdminSettings returns system settings like allowed domains for security.
func (h *Handler) GetAdminSettings(w http.ResponseWriter, r *http.Request) {
	if !h.requireAdmin(w, r) {
		return
	}

	ctx := r.Context()
	allowedDomains, _ := h.Store.GetSystemSetting(ctx, "allowed_domains")

	payload := AdminSettingsPayload{
		AllowedDomains: allowedDomains,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(payload)
}

// UpdateAdminSettings updates system settings like allowed domains.
func (h *Handler) UpdateAdminSettings(w http.ResponseWriter, r *http.Request) {
	if !h.requireAdmin(w, r) {
		return
	}

	var payload AdminSettingsPayload
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		WriteJSONError(w, http.StatusBadRequest, "invalid request")
		return
	}

	ctx := r.Context()
	err := h.Store.SetSystemSetting(ctx, "allowed_domains", payload.AllowedDomains)
	if err != nil {
		WriteInternalError(w, r, err)
		return
	}

	// Update in-memory value for the resolver
	mail.MonoProAllowedDomains.Store(payload.AllowedDomains)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "success"})
}

type AdminUser struct {
	ID         string `json:"id"`
	Email      string `json:"email"`
	Name       string `json:"name"`
	Role       string `json:"role"`
	LastSeenAt string `json:"last_seen_at"`
}

// GetAdminUsers returns a simple list of registered users and their last seen times.
func (h *Handler) GetAdminUsers(w http.ResponseWriter, r *http.Request) {
	if !h.requireAdmin(w, r) {
		return
	}

	ctx := r.Context()

	accounts, err := h.Store.GetAccounts(ctx)
	if err != nil {
		WriteInternalError(w, r, err)
		return
	}

	var users []AdminUser
	for _, acc := range accounts {
		role := "user"
		if u, err := h.Store.GetUserByEmail(ctx, acc.Email); err == nil && u != nil && u.Role != "" {
			role = u.Role
		}
		users = append(users, AdminUser{
			ID:         acc.ID,
			Email:      acc.Email,
			Name:       acc.Name,
			Role:       role,
			LastSeenAt: acc.LastSeenAt.Format("2006-01-02T15:04:05Z07:00"),
		})
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(users)
}
