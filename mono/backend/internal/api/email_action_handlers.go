package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
)

func (h *Handler) markEmailRead(w http.ResponseWriter, r *http.Request, emailID string) {
	email, err := h.ensureEmailAccess(r, emailID)
	if err != nil {
		WriteAccessError(w, err)
		return
	}
	if err := h.Store.MarkEmailRead(r.Context(), emailID, email.AccountID); err != nil {
		AppMetrics.HTTPErrors.Add(1)
		WriteInternalError(w, r, err)
		return
	}

	if h.SyncManager != nil {
		h.SyncManager.WakeUpAccountNow(email.AccountID)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]bool{"read": true})
	h.publishEvent(r.Context(), "email_updated", fmt.Sprintf(`{"email_id":"%s","account_id":"%s","is_read":true}`, emailID, email.AccountID))
}

// toggleEmail is a helper for toggle-pattern actions (pin, mute, flag).
// toggleFn receives (ctx, emailID, accountID) and returns the new boolean state.
func (h *Handler) toggleEmail(w http.ResponseWriter, r *http.Request, emailID string,
	toggleFn func(ctx context.Context, emailID, accountID string) (bool, error),
	field string,
) {
	email, err := h.ensureEmailAccess(r, emailID)
	if err != nil {
		WriteAccessError(w, err)
		return
	}
	val, err := toggleFn(r.Context(), emailID, email.AccountID)
	if err != nil {
		AppMetrics.HTTPErrors.Add(1)
		WriteInternalError(w, r, err)
		return
	}
	json.NewEncoder(w).Encode(map[string]bool{field: val})
	h.InvalidateEmailCache(r.Context(), email.AccountID)
	h.publishEvent(r.Context(), "email_updated", fmt.Sprintf(`{"email_id":"%s","account_id":"%s","%s":%t}`, emailID, email.AccountID, field, val))
}

func (h *Handler) togglePinEmail(w http.ResponseWriter, r *http.Request, emailID string) {
	h.toggleEmail(w, r, emailID, func(ctx context.Context, id, accID string) (bool, error) {
		return h.Store.TogglePinEmail(ctx, id, accID)
	}, "is_pinned")
}

func (h *Handler) toggleMuteEmail(w http.ResponseWriter, r *http.Request, emailID string) {
	h.toggleEmail(w, r, emailID, func(ctx context.Context, id, accID string) (bool, error) {
		return h.Store.ToggleMuteEmail(ctx, id, accID)
	}, "is_muted")
}

func (h *Handler) snoozeEmail(w http.ResponseWriter, r *http.Request, emailID string) {
	email, err := h.ensureEmailAccess(r, emailID)
	if err != nil {
		WriteAccessError(w, err)
		return
	}
	var req struct {
		Minutes int `json:"minutes"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		WriteJSONError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.Minutes <= 0 {
		req.Minutes = 180
	}
	if err := h.Store.SnoozeEmail(r.Context(), emailID, email.AccountID, req.Minutes); err != nil {
		AppMetrics.HTTPErrors.Add(1)
		WriteInternalError(w, r, err)
		return
	}
	json.NewEncoder(w).Encode(map[string]bool{"ok": true})
	h.InvalidateEmailCache(r.Context(), email.AccountID)
	h.publishEvent(r.Context(), "email_updated", fmt.Sprintf(`{"email_id":"%s","account_id":"%s","snoozed":true}`, emailID, email.AccountID))
}

func (h *Handler) downloadRawEmail(w http.ResponseWriter, r *http.Request, emailID string) {
	email, err := h.ensureEmailAccess(r, emailID)
	if err != nil {
		WriteAccessError(w, err)
		return
	}
	rawPath, err := h.Store.GetEmailBodyPath(r.Context(), emailID, email.AccountID)
	if err != nil || rawPath == "" {
		WriteJSONError(w, http.StatusNotFound, "not found")
		return
	}
	bodyPath := safeBodyPath(rawPath)
	if bodyPath == "" {
		WriteJSONError(w, http.StatusForbidden, "invalid body path")
		return
	}
	data, err := readEncryptedFile(bodyPath)
	if err != nil {
		WriteJSONError(w, http.StatusInternalServerError, "read error")
		return
	}
	w.Header().Set("Content-Type", "message/rfc822")
	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s.eml"`, emailID[:8]))
	w.Write(data)
}

func (h *Handler) toggleFlagEmail(w http.ResponseWriter, r *http.Request, emailID string) {
	h.toggleEmail(w, r, emailID, func(ctx context.Context, id, accID string) (bool, error) {
		return h.Store.ToggleFlagEmail(ctx, id, accID)
	}, "is_flagged")
}

func (h *Handler) saveDraftReply(w http.ResponseWriter, r *http.Request, emailID string) {
	email, err := h.ensureEmailAccess(r, emailID)
	if err != nil {
		WriteAccessError(w, err)
		return
	}
	var req struct {
		Body string `json:"body"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		WriteJSONError(w, http.StatusBadRequest, ClientSafeMessage(err, "bad request"))
		return
	}
	if err := h.Store.SaveDraftReply(r.Context(), emailID, email.AccountID, req.Body); err != nil {
		WriteInternalError(w, r, err)
		return
	}
	json.NewEncoder(w).Encode(map[string]bool{"ok": true})
	h.InvalidateEmailCache(r.Context(), email.AccountID)
	h.publishEvent(r.Context(), "email_updated", fmt.Sprintf(`{"email_id":"%s","account_id":"%s","draft_saved":true}`, emailID, email.AccountID))
}

func (h *Handler) clearDraftReply(w http.ResponseWriter, r *http.Request, emailID string) {
	email, err := h.ensureEmailAccess(r, emailID)
	if err != nil {
		WriteAccessError(w, err)
		return
	}
	if err := h.Store.ClearDraftReply(r.Context(), emailID, email.AccountID); err != nil {
		WriteInternalError(w, r, err)
		return
	}
	json.NewEncoder(w).Encode(map[string]bool{"ok": true})
	h.InvalidateEmailCache(r.Context(), email.AccountID)
	h.publishEvent(r.Context(), "email_updated", fmt.Sprintf(`{"email_id":"%s","account_id":"%s","draft_cleared":true}`, emailID, email.AccountID))
}

func (h *Handler) moveEmail(w http.ResponseWriter, r *http.Request) {
	var req struct {
		FolderID string `json:"folder_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		WriteJSONError(w, http.StatusBadRequest, ClientSafeMessage(err, "bad request"))
		return
	}

	// Extract emailID from the URL path: /api/emails/{emailId}/move
	pathParts := strings.Split(strings.Trim(r.URL.Path, "/"), "/")
	emailID := ""
	if len(pathParts) >= 3 {
		emailID = pathParts[2]
	}
	if emailID == "" {
		WriteJSONError(w, http.StatusBadRequest, "missing email ID in path")
		return
	}

	email, err := h.ensureEmailAccess(r, emailID)
	if err != nil {
		WriteAccessError(w, err)
		return
	}

	// Resolve virtual folder names (__archive__, __trash__) to real folder IDs
	if req.FolderID == "__archive__" || req.FolderID == "__trash__" {
		targetName := "Archive"
		if req.FolderID == "__trash__" {
			targetName = "Trash"
		}
		folder, fErr := h.Store.GetFolderByName(r.Context(), email.AccountID, targetName)
		if fErr == nil && folder != nil {
			req.FolderID = folder.ID
		} else {
			created, createErr := h.Store.CreateFolder(r.Context(), email.AccountID, targetName, targetName, true)
			if createErr == nil && created != nil {
				req.FolderID = created.ID
			}
		}
	}

	if req.FolderID == "" {
		WriteJSONError(w, http.StatusBadRequest, "folder_id is required")
		return
	}
	if !h.folderBelongsToAccount(r.Context(), req.FolderID, email.AccountID) {
		WriteJSONError(w, http.StatusBadRequest, "folder does not belong to account")
		return
	}

	srcFolder := "INBOX"
	if email.FolderID != "" {
		if f, fErr := h.Store.GetFolderByID(r.Context(), email.FolderID); fErr == nil && f != nil {
			srcFolder = f.Name
		}
	}
	folderName := ""
	if email.UID > 0 {
		folder, folderErr := h.Store.GetFolderByID(r.Context(), req.FolderID)
		if folderErr == nil && folder != nil {
			folderName = folder.Name
		}
		if err := h.Store.MoveEmailAndEnqueueIMAP(r.Context(), emailID, email.AccountID, req.FolderID, folderName, srcFolder, email.UID); err != nil {
			AppMetrics.HTTPErrors.Add(1)
			WriteInternalError(w, r, err)
			return
		}
	} else {
		if err := h.Store.MoveEmail(r.Context(), emailID, email.AccountID, req.FolderID); err != nil {
			AppMetrics.HTTPErrors.Add(1)
			WriteInternalError(w, r, err)
			return
		}
	}

	json.NewEncoder(w).Encode(map[string]string{"status": "moved"})
	h.InvalidateEmailCache(r.Context(), email.AccountID)
	h.publishEvent(r.Context(), "email_updated", fmt.Sprintf(`{"email_id":"%s","account_id":"%s","folder_id":"%s"}`, emailID, email.AccountID, req.FolderID))
}
