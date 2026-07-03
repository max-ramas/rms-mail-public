package api

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"strings"

	"rmsmail/internal/models"

	"github.com/google/uuid"
)

func (h *Handler) GetEmailIDs(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		WriteJSONError(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}

	accountID := r.URL.Query().Get("account_id")
	if accountID != "" && accountID != "unified" && !strings.HasPrefix(accountID, "group:") {
		if err := h.CheckAccountAccess(r.Context(), accountID); err != nil {
			WriteAccessError(w, err)
			return
		}
	}
	unified := r.URL.Query().Get("unified") == "true" || accountID == "unified"
	folderID := r.URL.Query().Get("folder_id")

	if accountID == "" && !unified {
		WriteJSONError(w, http.StatusBadRequest, "Missing account_id")
		return
	}

	// For unified, ignore stale folder_id from previous view — always use INBOX.
	if unified {
		accountID = "unified"
		folderID = ""
	}

	var ids []string
	var err error
	aggregate, aggErr := h.usesPerAccountAggregation(r.Context(), accountID)
	if aggErr != nil {
		WriteAccessError(w, aggErr)
		return
	}
	if aggregate || strings.HasPrefix(accountID, "group:") {
		scoped, sErr := h.perAccountScopeIDs(r.Context(), accountID)
		if sErr != nil {
			WriteAccessError(w, sErr)
			return
		}
		perFolder := folderID
		if perFolder == "" {
			perFolder = "INBOX"
		}
		for _, accID := range scoped {
			if chkErr := h.CheckAccountAccess(r.Context(), accID); chkErr != nil {
				continue
			}
			part, pErr := h.Store.GetEmailIDsByFilter(r.Context(), accID, perFolder)
			if pErr != nil {
				err = pErr
				break
			}
			ids = append(ids, part...)
		}
	} else if unified {
		ids, err = h.Store.GetEmailIDs(r.Context(), accountID, folderID)
	} else {
		ids, err = h.Store.GetEmailIDs(r.Context(), accountID, folderID)
	}
	if err != nil {
		WriteInternalError(w, r, err)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(ids)
}

func (h *Handler) GetEmailCount(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		WriteJSONError(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}
	accountID := r.URL.Query().Get("account_id")
	if accountID == "" {
		WriteJSONError(w, http.StatusBadRequest, "Missing account_id")
		return
	}
	if accountID != "unified" && !strings.HasPrefix(accountID, "group:") {
		if err := h.CheckAccountAccess(r.Context(), accountID); err != nil {
			WriteAccessError(w, err)
			return
		}
	}
	folderID := r.URL.Query().Get("folder_id")
	opts := models.EmailCountOpts{
		Unread:         r.URL.Query().Get("unread") == "true",
		Flagged:        r.URL.Query().Get("flagged") == "true",
		HasAttachments: r.URL.Query().Get("has_attachments") == "true",
	}

	var count int
	var err error
	aggregate, aggErr := h.usesPerAccountAggregation(r.Context(), accountID)
	if aggErr != nil {
		WriteAccessError(w, aggErr)
		return
	}
	if aggregate || strings.HasPrefix(accountID, "group:") {
		scoped, sErr := h.perAccountScopeIDs(r.Context(), accountID)
		if sErr != nil {
			WriteAccessError(w, sErr)
			return
		}
		perAccountFolder := folderID
		if perAccountFolder == "" {
			perAccountFolder = "INBOX"
		}
		for _, accID := range scoped {
			if chkErr := h.CheckAccountAccess(r.Context(), accID); chkErr != nil {
				continue
			}
			c, cErr := h.Store.GetEmailCount(r.Context(), accID, perAccountFolder, opts)
			if cErr != nil {
				err = cErr
				break
			}
			count += c
		}
	} else {
		count, err = h.Store.GetEmailCount(r.Context(), accountID, folderID, opts)
	}
	if err != nil {
		WriteInternalError(w, r, err)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]int{"count": count})
}

func (h *Handler) SaveStandaloneDraft(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		WriteJSONError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	accountID := r.Header.Get("X-Account-Id")
	if accountID == "" {
		WriteJSONError(w, http.StatusBadRequest, "missing X-Account-Id")
		return
	}

	if err := h.CheckAccountAccess(r.Context(), accountID); err != nil {
		WriteAccessError(w, err)
		return
	}

	var req struct {
		ID         string `json:"id"`
		To         string `json:"to"`
		Cc         string `json:"cc"`
		Bcc        string `json:"bcc"`
		Subject    string `json:"subject"`
		Body       string `json:"body"`
		HTML       string `json:"html"`
		InReplyTo  string `json:"in_reply_to"`
		SyncRemote bool   `json:"sync_remote"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		WriteJSONError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	emailID := req.ID
	if emailID == "" {
		emailID = uuid.New().String()
	}

	draftFolder, err := h.Store.GetDraftsFolder(r.Context(), accountID)
	if err != nil || draftFolder == nil {
		WriteJSONError(w, http.StatusInternalServerError, "drafts folder not found")
		return
	}

	// Payload that will be parsed when editing
	draftPayload, err := json.Marshal(req)
	if err != nil {
		WriteInternalError(w, r, err)
		return
	}

	err = h.Store.SaveStandaloneDraft(
		r.Context(),
		accountID,
		emailID,
		draftFolder.ID,
		req.To,
		req.Cc,
		req.Subject,
		string(draftPayload),
		req.SyncRemote,
	)
	if err != nil {
		slog.Info("SaveStandaloneDraft error", "error", err)
		WriteJSONError(w, http.StatusInternalServerError, "failed to save draft")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"id": emailID})
}
