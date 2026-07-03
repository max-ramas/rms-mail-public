package api

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"time"
)

type createNoteRequest struct {
	AccountID string `json:"account_id"`
	Subject   string `json:"subject"`
	Content   string `json:"content"`
}

func (h *Handler) CreateNote(w http.ResponseWriter, r *http.Request) {
	var req createNoteRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		WriteJSONError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if req.AccountID == "" {
		WriteJSONError(w, http.StatusBadRequest, "account_id is required")
		return
	}

	if err := h.CheckAccountAccess(r.Context(), req.AccountID); err != nil {
		WriteAccessError(w, err)
		return
	}

	// Make sure the SyncManager supports AppendToNotes
	type notesAppender interface {
		AppendToNotes(ctx context.Context, accountID string, noteBody string) error
	}

	appender, ok := h.SyncManager.(notesAppender)
	if !ok {
		WriteJSONError(w, http.StatusInternalServerError, "internal error: sync manager does not support Notes")
		return
	}

	// Notes generally use plain text or HTML.
	// To match Apple Notes format:
	dateStr := time.Now().Format(time.RFC1123Z)

	// Create boundary for multipart (not strictly necessary if only one part, but good for HTML notes)
	var sb strings.Builder
	sb.WriteString("From: <draft@local>\r\n")
	sb.WriteString(fmt.Sprintf("Subject: %s\r\n", stripCRLF(req.Subject)))
	sb.WriteString(fmt.Sprintf("Date: %s\r\n", dateStr))
	sb.WriteString("X-Uniform-Type-Identifier: com.apple.mail-note\r\n")
	sb.WriteString("MIME-Version: 1.0\r\n")
	sb.WriteString("Content-Type: text/html; charset=utf-8\r\n\r\n")

	// The Apple Notes usually have HTML content
	contentHTML := req.Content
	if !strings.Contains(contentHTML, "<html>") {
		contentHTML = fmt.Sprintf("<html><body>%s</body></html>", req.Content)
	}
	sb.WriteString(contentHTML)

	if err := appender.AppendToNotes(r.Context(), req.AccountID, sb.String()); err != nil {
		slog.Info(fmt.Sprintf("CreateNote error: %v", err))
		WriteJSONError(w, http.StatusInternalServerError, "failed to create note")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"status":  "success",
		"message": "Note created and appended via IMAP successfully",
	})
}
