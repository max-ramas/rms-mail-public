package api

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"net/mail"
	"strings"
	"time"

	"rmsmail/internal/smtp"

	"github.com/google/uuid"
)

func (h *Handler) SendEmail(w http.ResponseWriter, r *http.Request) {
	var req struct {
		AccountID        string   `json:"account_id"`
		To               []string `json:"to"`
		Cc               []string `json:"cc"`
		Subject          string   `json:"subject"`
		Body             string   `json:"body"`
		HTML             string   `json:"html"`
		FromIdentity     string   `json:"from_identity"`
		InReplyTo        string   `json:"in_reply_to"`
		References       string   `json:"references"`
		AttachmentHashes []string `json:"attachment_hashes"`
		DraftID          string   `json:"draft_id"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		WriteJSONError(w, http.StatusBadRequest, ClientSafeMessage(err, "bad request"))
		return
	}

	// Validation: all recipients
	allRecipients := append([]string{}, req.To...)
	allRecipients = append(allRecipients, req.Cc...)

	if len(allRecipients) == 0 {
		WriteJSONError(w, http.StatusBadRequest, "at least one recipient is required")
		return
	}
	if len(allRecipients) > 50 {
		WriteJSONError(w, http.StatusBadRequest, "max 50 recipients allowed")
		return
	}

	for _, addr := range allRecipients {
		if _, err := mail.ParseAddress(addr); err != nil {
			WriteJSONError(w, http.StatusBadRequest, fmt.Sprintf("invalid email address: %s", addr))
			return
		}
	}

	// Validation: body size (max 10MB)
	bodySize := len(req.Body) + len(req.HTML)
	if bodySize > 10*1024*1024 {
		WriteJSONError(w, http.StatusBadRequest, "email body too large (max 10MB)")
		return
	}

	// Validation: subject required
	if strings.TrimSpace(req.Subject) == "" {
		WriteJSONError(w, http.StatusBadRequest, "subject is required")
		return
	}

	if err := h.CheckAccountAccess(r.Context(), req.AccountID); err != nil {
		WriteAccessError(w, err)
		return
	}

	account, err := h.Store.GetAccountCredentials(r.Context(), req.AccountID)
	if err != nil || account == nil {
		WriteJSONError(w, http.StatusNotFound, "account not found")
		return
	}

	if account.IsLocked {
		WriteJSONError(w, http.StatusPaymentRequired, "account is locked due to license limits")
		return
	}

	// Sanitize header values to prevent CRLF injection
	req.Subject = stripCRLF(req.Subject)
	req.InReplyTo = stripCRLF(req.InReplyTo)
	req.References = stripCRLF(req.References)


	if account.SMTPHost == "" || account.SMTPPort == 0 {
		WriteJSONError(w, http.StatusBadRequest, "SMTP not configured for this account")
		return
	}

	var client *smtp.Client

	fromAddr := account.Email
	if account.Name != "" {
		fromAddr = fmt.Sprintf("%s <%s>", account.Name, account.Email)
	}
	if req.FromIdentity != "" {
		// Validate fromIdentity (H1)
		valid := false
		parsed, err := mail.ParseAddress(req.FromIdentity)
		if err == nil {
			if strings.EqualFold(parsed.Address, account.Email) {
				valid = true
			} else {
				ids, _ := h.Store.GetIdentities(r.Context(), account.ID)
				for _, ident := range ids {
					if strings.EqualFold(parsed.Address, ident.Email) {
						valid = true
						break
					}
				}
			}
		}
		if !valid {
			WriteJSONError(w, http.StatusForbidden, "invalid from_identity: not authorized to send from this address")
			return
		}
		fromAddr = req.FromIdentity
	}

	email := &smtp.Email{
		To:         req.To,
		Cc:         req.Cc,
		Subject:    req.Subject,
		Body:       req.Body,
		HTML:       req.HTML,
		From:       fromAddr,
		MessageID:  fmt.Sprintf("<%s@rmsmail>", uuid.New().String()),
		Date:       time.Now(),
		InReplyTo:  req.InReplyTo,
		References: req.References,
	}

	// Resolve attachments by hash from CAS
	for _, hash := range req.AttachmentHashes {
		att, err := h.Store.GetAttachmentByHash(r.Context(), hash)
		if err != nil || att == nil {
			slog.Info("SendEmail: attachment hash not found", "hash", hash, "error", err)
			continue
		}
		email.Attachments = append(email.Attachments, smtp.Attachment{
			Filename: att.Filename,
			Path:     att.Path,
		})
	}

	if false {

		// Save to DB first (so Cancel can find it), then enqueue delayed send.

		// Fall through to Scheduler if Asynq fails

	}

	if false {

	}

	if err := client.Send(email); err != nil {
		AppMetrics.HTTPErrors.Add(1)
		WriteInternalError(w, r, err)
		return
	}

	AppMetrics.EmailsSent.Add(1)
	h.markRepliedEmailAnswered(r.Context(), req.AccountID, req.InReplyTo)

	if req.DraftID != "" {
		h.Store.DeleteDraft(r.Context(), req.DraftID)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "sent"})
	h.publishEvent(r.Context(), "email_updated", fmt.Sprintf(`{"status":"sent","account_id":"%s"}`, req.AccountID))
}

func (h *Handler) CancelSend(w http.ResponseWriter, r *http.Request) {
	pathParts := strings.Split(strings.Trim(r.URL.Path, "/"), "/")
	jobID := pathParts[len(pathParts)-1]


	// Verify job ownership before canceling
	accountID, _, err := h.Store.GetScheduledEmail(r.Context(), jobID)
	if err != nil || accountID == "" {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusNotFound)
		json.NewEncoder(w).Encode(map[string]string{"status": "not_found", "job_id": jobID})
		return
	}
	if err := h.CheckAccountAccess(r.Context(), accountID); err != nil {
		WriteAccessError(w, err)
		return
	}

}
