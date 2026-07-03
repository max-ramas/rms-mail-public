package api

import (
	"encoding/json"
	"net/http"
	"strings"
)

func (h *Handler) HandleIdentities(w http.ResponseWriter, r *http.Request) {
	accountID := r.URL.Query().Get("account_id")
	if accountID != "" && accountID != "unified" {
		if err := h.CheckAccountAccess(r.Context(), accountID); err != nil {
			WriteAccessError(w, err)
			return
		}
	}
	if r.Method == http.MethodGet {
		cacheKey := "identities:" + accountID
		if h.tryCache(w, r, cacheKey) {
			return
		}
		ids, err := h.Store.GetIdentities(r.Context(), accountID)
		if err != nil {
			WriteInternalError(w, r, err)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		b, err := json.Marshal(ids)
		if err != nil {
			WriteInternalError(w, r, err)
			return
		}
		w.Write(b)
		if false {

		}
		return
	}
	if r.Method == http.MethodPost {
		var req struct {
			AccountID string `json:"account_id"`
			Email     string `json:"email"`
			Name      string `json:"name"`
		}
		json.NewDecoder(r.Body).Decode(&req)
		if err := h.CheckAccountAccess(r.Context(), req.AccountID); err != nil {
			WriteAccessError(w, err)
			return
		}
		id, err := h.Store.CreateIdentity(r.Context(), req.AccountID, req.Email, req.Name)
		if err != nil {
			WriteInternalError(w, r, err)
			return
		}
		w.WriteHeader(201)
		json.NewEncoder(w).Encode(id)
		return
	}
	if r.Method == http.MethodDelete {
		pathParts := strings.Split(strings.Trim(r.URL.Path, "/"), "/")
		if len(pathParts) >= 3 {
			identityID := pathParts[2]
			// Get identity first to verify ownership
			identities, err := h.Store.GetIdentities(r.Context(), "")
			if err == nil {
				for _, ident := range identities {
					if ident.ID == identityID {
						if err := h.CheckAccountAccess(r.Context(), ident.AccountID); err != nil {
							WriteAccessError(w, err)
							return
						}
						break
					}
				}
			}
			h.Store.DeleteIdentity(r.Context(), identityID)
		}
		json.NewEncoder(w).Encode(map[string]bool{"ok": true})
		return
	}
}

func (h *Handler) HandleFolder(w http.ResponseWriter, r *http.Request) {
	pathParts := strings.Split(strings.Trim(r.URL.Path, "/"), "/")
	folderID := pathParts[len(pathParts)-1]
	if r.Method == http.MethodDelete && folderID != "" {
		folder, err := h.Store.GetFolderByID(r.Context(), folderID)
		if err != nil || folder == nil {
			WriteJSONError(w, http.StatusNotFound, "folder not found")
			return
		}
		if err := h.CheckAccountAccess(r.Context(), folder.AccountID); err != nil {
			WriteAccessError(w, err)
			return
		}
		if err := h.Store.DeleteEmailsInFolder(r.Context(), folderID); err != nil {
			AppMetrics.HTTPErrors.Add(1)
			WriteInternalError(w, r, err)
			return
		}
		json.NewEncoder(w).Encode(map[string]bool{"ok": true})
		return
	}
	WriteJSONError(w, http.StatusNotFound, "not found")
}

func (h *Handler) GetFolders(w http.ResponseWriter, r *http.Request) {
	accountID := r.URL.Query().Get("account_id")
	if accountID != "" && accountID != "unified" {
		if err := h.CheckAccountAccess(r.Context(), accountID); err != nil {
			WriteAccessError(w, err)
			return
		}
	}

	folders, err := h.Store.GetFolders(r.Context(), accountID)
	if err != nil {
		AppMetrics.HTTPErrors.Add(1)
		WriteInternalError(w, r, err)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	b, err := json.Marshal(folders)
	if err != nil {
		WriteInternalError(w, r, err)
		return
	}
	w.Write(b)

	if false {

	}
}
