package api

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"

	"rmsmail/internal/edition"
	"rmsmail/internal/models"

	"github.com/jhillyerd/enmime"
)

func (h *Handler) GetEmails(w http.ResponseWriter, r *http.Request) {
	AppMetrics.HTTPRequests.Add(1)

	// /api/emails/ids — return all email IDs for select-all.
	if strings.HasPrefix(r.URL.Path, "/api/emails/ids") {
		h.GetEmailIDs(w, r)
		return
	}

	unified := r.URL.Query().Get("unified") == "true"
	accountID := r.URL.Query().Get("account_id")
	if accountID != "" && accountID != "unified" {
		if err := h.CheckAccountAccess(r.Context(), accountID); err != nil {
			WriteAccessError(w, err)
			return
		}
	}
	folderID := r.URL.Query().Get("folder_id")
	folderName := r.URL.Query().Get("folder")
	groupID := r.URL.Query().Get("group_id")

	// Parse cursor for keyset pagination (replaces OFFSET for large mailboxes).
	// Format: "true|2026-06-13T18:19:00Z|uuid"
	cursorRaw := r.URL.Query().Get("cursor")
	cursor := models.ParseCursor(cursorRaw)

	offset := 0
	limit := 50
	if offsetStr := r.URL.Query().Get("offset"); offsetStr != "" {
		if o, err := strconv.Atoi(offsetStr); err == nil {
			offset = o
		}
	}
	if limitStr := r.URL.Query().Get("limit"); limitStr != "" {
		maxLimit := 100
		if unified {
			maxLimit = 500
		}
		if l, err := strconv.Atoi(limitStr); err == nil && l <= maxLimit {
			limit = l
		}
	}

	filterUnread := r.URL.Query().Get("unread") == "true"
	filterFlagged := r.URL.Query().Get("flagged") == "true"
	filterAttachments := r.URL.Query().Get("has_attachments") == "true"
	filterSearch := r.URL.Query().Get("search")
	filterLabelID := r.URL.Query().Get("label_id")
	filterTag := r.URL.Query().Get("tag")

	filter := models.EmailFilterOpts{
		Unread:      filterUnread,
		Flagged:     filterFlagged,
		Attachments: filterAttachments,
		Search:      filterSearch,
		LabelID:     filterLabelID,
		Tag:         filterTag,
	}

	// Generate a unique cache key including all filters
	cacheKeyAcc := accountID
	if unified {
		cacheKeyAcc = "unified"
	}
	if groupID != "" {
		cacheKeyAcc = "grp:" + groupID
	}
	cacheKey := fmt.Sprintf("email_list:acc:%s:fold:%s:foldn:%s:off:%d:lim:%d:unr:%v:flg:%v:att:%v:sch:%s:lbl:%s:tag:%s",
		cacheKeyAcc, folderID, folderName, offset, limit, filterUnread, filterFlagged, filterAttachments, filterSearch, filterLabelID, filterTag)

	if h.tryCache(w, r, cacheKey) {
		return
	}

	if groupID != "" {
		ids, err := h.Store.GetGroupEmailAccountIDs(r.Context(), groupID)
		if err != nil {
			AppMetrics.HTTPErrors.Add(1)
			WriteInternalError(w, r, err)
			return
		}
		if len(ids) == 0 {
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode([]models.Email{})
			return
		}
		var allowedIDs []string
		for _, id := range ids {
			if err := h.CheckAccountAccess(r.Context(), id); err == nil {
				allowedIDs = append(allowedIDs, id)
			}
		}
		if len(allowedIDs) == 0 {
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode([]models.Email{})
			return
		}
		// Pass account IDs to SQL-level filter — avoids pagination skew from client-side filtering.
		emails, err := h.Store.GetEmailsByAccounts(r.Context(), allowedIDs, folderName, offset, limit, filter)
		if err != nil {
			AppMetrics.HTTPErrors.Add(1)
			WriteInternalError(w, r, err)
			return
		}

		// Attach avatars via Camo proxy
		h.Store.AttachAvatars(r.Context(), emails)
		for i := range emails {
			if emails[i].AvatarURL != "" {
				emails[i].AvatarURL = fmt.Sprintf("/api/media/proxy?url=%s&sig=%s",
					url.QueryEscape(emails[i].AvatarURL), camoSign(emails[i].AvatarURL))
			}
		}

		w.Header().Set("Content-Type", "application/json")
		if emails == nil {
			emails = []models.Email{}
		}
		b, err := json.Marshal(emails)
		if err != nil {
			WriteInternalError(w, r, err)
			return
		}
		if false {

		}
		w.Write(b)
		return
	}

	var emails []models.Email
	var nextCursor *models.Cursor
	var err error
	var scopedAccountIDs []string
	if unified {
		scopedAccountIDs, err = h.monoAccessibleAccountIDs(r.Context())
		if err != nil {
			WriteAccessError(w, err)
			return
		}
		if edition.IsMono() && len(scopedAccountIDs) == 0 {
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode([]models.Email{})
			return
		}
	}
	emails, nextCursor, err = h.Store.GetEmailsCursor(r.Context(), unified, accountID, folderID, folderName, limit, filter, cursor, scopedAccountIDs)
	if err != nil {
		AppMetrics.HTTPErrors.Add(1)
		WriteInternalError(w, r, err)
		return
	}

	// Attach avatars via Camo proxy
	h.Store.AttachAvatars(r.Context(), emails)
	for i := range emails {
		if emails[i].AvatarURL != "" {
			emails[i].AvatarURL = fmt.Sprintf("/api/media/proxy?url=%s&sig=%s",
				url.QueryEscape(emails[i].AvatarURL), camoSign(emails[i].AvatarURL))
		}
	}

	w.Header().Set("Content-Type", "application/json")
	if emails == nil {
		emails = []models.Email{}
	}
	b2, err := json.Marshal(emails)
	if err != nil {
		WriteInternalError(w, r, err)
		return
	}
	// cache disabled
	if false {

	}
	if nextCursor != nil {
		w.Header().Set("X-Next-Cursor", nextCursor.Format())
	}
	w.Write(b2)
}

func (h *Handler) HandleEmail(w http.ResponseWriter, r *http.Request) {
	pathParts := strings.Split(strings.Trim(r.URL.Path, "/"), "/")
	if len(pathParts) < 3 {
		WriteJSONError(w, http.StatusBadRequest, "invalid path")
		return
	}
	emailID := pathParts[2]

	// Route /api/emails/ids to GetEmailIDs handler.
	if emailID == "ids" {
		h.GetEmailIDs(w, r)
		return
	}

	switch r.Method {
	case http.MethodGet:
		if len(pathParts) >= 4 && pathParts[3] == "tags" {
			h.getEmailTags(w, r, emailID)
		} else if len(pathParts) >= 4 && pathParts[3] == "raw" {
			h.downloadRawEmail(w, r, emailID)
		} else {
			h.getEmail(w, r, emailID)
		}
	case http.MethodPost:
		if len(pathParts) >= 4 {
			action := pathParts[3]
			if fn, ok := emailActions[action]; ok {
				fn(h, w, r, emailID)
			} else {
				WriteJSONError(w, http.StatusBadRequest, "unknown action")
			}
		} else {
			WriteJSONError(w, http.StatusBadRequest, "missing action")
		}
	case http.MethodDelete:
		h.deleteEmail(w, r, emailID)
	default:
		WriteJSONError(w, http.StatusMethodNotAllowed, "method not allowed")
	}
}

func (h *Handler) getEmail(w http.ResponseWriter, r *http.Request, emailID string) {
	email, err := h.Store.GetEmail(r.Context(), emailID, r.URL.Query().Get("account_id"))
	if err != nil || email == nil {
		WriteJSONError(w, http.StatusNotFound, "email not found")
		return
	}
	if err := h.CheckAccountAccess(r.Context(), email.AccountID); err != nil {
		WriteJSONError(w, http.StatusForbidden, "access denied")
		return
	}
	accountID := email.AccountID

	if h.SyncManager != nil && email.UID > 0 && !email.IsDirtyLocally {
		h.SyncManager.RequestFlagRefresh(accountID, email.ID)
		h.SyncManager.WakeUpAccountNow(accountID)
	}

	bodyPath := safeBodyPath(email.BodyPath)
	if bodyPath == "" {
		WriteJSONError(w, http.StatusForbidden, "invalid body path")
		return
	}

	// Parallel I/O: read body file + fetch attachments + fetch tags
	type bodyRes struct {
		raw []byte
		err error
	}
	bodyCh := make(chan bodyRes, 1)
	go func() {
		raw, err := readEncryptedFile(bodyPath)
		bodyCh <- bodyRes{raw, err}
	}()

	type attRes struct {
		att []models.Attachment
		err error
	}
	attCh := make(chan attRes, 1)
	go func() {
		att, err := h.Store.GetEmailAttachments(r.Context(), emailID, accountID)
		attCh <- attRes{att, err}
	}()

	type tagsRes struct {
		tags []string
		err  error
	}
	tagsCh := make(chan tagsRes, 1)
	go func() {
		tags, err := h.Store.GetEmailTags(r.Context(), emailID, accountID)
		tagsCh <- tagsRes{tags, err}
	}()

	// Collect body
	br := <-bodyCh
	raw := br.raw

	bodyText := ""
	bodyHTML := ""

	// Collect attachments
	ar := <-attCh
	emailAttachments := ar.att
	if ar.err != nil {
		slog.Info("getEmail: GetEmailAttachments error", "emailID", emailID, "error", ar.err)
	}

	if len(raw) > 0 {
		env, err := enmime.ReadEnvelope(bytes.NewReader(raw))
		if err == nil {
			bodyText = env.Text
			bodyHTML = decodeQuotedPrintable(env.HTML)
			if strings.Contains(bodyHTML, "â€") {
				bodyHTML = fixMojibake(bodyHTML)
				slog.Info("getEmail: fixed double-encoded UTF-8", "emailID", emailID)
			}

			// Build cid→hash map from attachments for inline image URL fallback.
			cidMap := make(map[string]string)
			for _, att := range emailAttachments {
				if att.ContentID != "" && att.Hash != "" {
					cid := strings.Trim(att.ContentID, "<>")
					cidMap[cid] = att.Hash
				}
			}

			// Rewrite cid: references → inline base64 for small images,
			// attachment API URLs for large ones.
			for _, part := range env.Inlines {
				if part.ContentID != "" && len(part.Content) > 0 && len(part.Content) < 512*1024 {
					cid := strings.Trim(part.ContentID, "<>")
					ct := part.ContentType
					if ct == "" {
						ct = "image/jpeg"
					}
					b64 := base64.StdEncoding.EncodeToString(part.Content)
					dataURI := fmt.Sprintf("data:%s;base64,%s", ct, b64)
					bodyHTML = strings.ReplaceAll(bodyHTML, "cid:"+cid, dataURI)
				} else if part.ContentID != "" {
					cid := strings.Trim(part.ContentID, "<>")
					if hash, ok := cidMap[cid]; ok {
						bodyHTML = strings.ReplaceAll(bodyHTML, "cid:"+cid, fmt.Sprintf("/api/attachments/%s?inline=true", hash))
					}
				}
			}
			for _, part := range env.Attachments {
				if part.ContentID != "" && len(part.Content) > 0 && len(part.Content) < 512*1024 {
					cid := strings.Trim(part.ContentID, "<>")
					ct := part.ContentType
					if ct == "" {
						ct = "image/jpeg"
					}
					b64 := base64.StdEncoding.EncodeToString(part.Content)
					dataURI := fmt.Sprintf("data:%s;base64,%s", ct, b64)
					bodyHTML = strings.ReplaceAll(bodyHTML, "cid:"+cid, dataURI)
				} else if part.ContentID != "" {
					cid := strings.Trim(part.ContentID, "<>")
					if hash, ok := cidMap[cid]; ok {
						bodyHTML = strings.ReplaceAll(bodyHTML, "cid:"+cid, fmt.Sprintf("/api/attachments/%s?inline=true", hash))
					}
				}
			}

			// Only process if there's actual HTML content.
			if strings.TrimSpace(bodyHTML) != "" {
				// Normalize + sanitize for display: legacy attrs → inline styles,
				// strip scripts/active content. XSS boundary is iframe srcdoc CSP.
				bodyHTML = normalizeEmailHTML(bodyHTML)
				bodyHTML = wrapEmailForIframe(bodyHTML)
			}

			if strings.Contains(env.HTML, "=3D") {
				slog.Info("getEmail: QP artifacts detected in env.HTML, decoded", "emailID", emailID, "htmlLen", len(env.HTML))
			}
		}
	}

	// Use previously fetched attachments (already loaded for inline image mapping)
	attachments := emailAttachments
	if attachments == nil {
		attachments = []models.Attachment{}
	}
	slog.Info("getEmail: returning attachments", "count", len(attachments), "emailID", emailID)

	// Fetch thread emails if ThreadID is present
	var threadEmails []models.Email
	if email.ThreadID != "" {
		var err error
		threadEmails, err = h.Store.GetEmailsByThreadID(r.Context(), email.ThreadID, email.AccountID, 50)
		if err != nil {
			slog.Info("getEmail: GetEmailsByThreadID error", "threadID", email.ThreadID, "error", err)
		}
	}
	if threadEmails == nil {
		threadEmails = []models.Email{}
	}

	// Attach avatars via Camo proxy to thread emails
	h.Store.AttachAvatars(r.Context(), threadEmails)
	for i := range threadEmails {
		if threadEmails[i].AvatarURL != "" {
			threadEmails[i].AvatarURL = fmt.Sprintf("/api/media/proxy?url=%s&sig=%s",
				url.QueryEscape(threadEmails[i].AvatarURL), camoSign(threadEmails[i].AvatarURL))
		}
	}

	// Sort thread emails by date descending (newest first)
	sort.Slice(threadEmails, func(i, j int) bool {
		return threadEmails[i].DateSent.After(threadEmails[j].DateSent)
	})

	// Collect tags
	tr := <-tagsCh
	tags := tr.tags
	if tags == nil {
		tags = []string{}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"email":         email,
		"body":          bodyText,
		"html":          bodyHTML,
		"attachments":   attachments,
		"thread_emails": threadEmails,
		"tags":          tags,
	})
}

func (h *Handler) getEmailTags(w http.ResponseWriter, r *http.Request, emailID string) {
	email, err := h.ensureEmailAccess(r, emailID)
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			WriteJSONError(w, http.StatusNotFound, "email not found")
		} else {
			WriteAccessError(w, err)
		}
		return
	}
	tags, err := h.Store.GetEmailTags(r.Context(), emailID, email.AccountID)
	if err != nil {
		AppMetrics.HTTPErrors.Add(1)
		WriteInternalError(w, r, err)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string][]string{"tags": tags})
}

func (h *Handler) deleteEmail(w http.ResponseWriter, r *http.Request, emailID string) {
	accountID := r.URL.Query().Get("account_id")
	ctx := context.WithoutCancel(r.Context())

	// 1. Get the email to find its account and UID
	email, err := h.Store.GetEmail(ctx, emailID, accountID)
	if err != nil || email == nil {
		slog.Info("deleteEmail: email not found", "error", err)
		WriteJSONError(w, http.StatusNotFound, "email not found")
		return
	}
	if err := h.CheckAccountAccess(r.Context(), email.AccountID); err != nil {
		WriteJSONError(w, http.StatusForbidden, "access denied")
		return
	}
	if accountID == "" {
		accountID = email.AccountID
	}

	// 2. Find the Trash folder for this account
	trashFolder, err := h.Store.GetFolderByName(ctx, accountID, "Trash")
	if err != nil {
		WriteInternalError(w, r, err)
		return
	}
	var trashFolderID string
	if trashFolder != nil {
		trashFolderID = trashFolder.ID
	}
	if trashFolderID == "" {
		// Try to create a Trash folder
		trashFolder, createErr := h.Store.CreateFolder(ctx, accountID, "Trash", "Trash", true)
		if createErr != nil || trashFolder == nil {
			// Still can't — fall back to hard delete
			slog.Info("deleteEmail: cannot create Trash folder, falling back to hard delete", "accountID", accountID, "error", createErr)
			if err := h.Store.DeleteEmail(ctx, emailID, accountID); err != nil {
				WriteInternalError(w, r, err)
				return
			}
			AppMetrics.EmailsDeleted.Add(1)
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]bool{"deleted": true})
			return
		}
		trashFolderID = trashFolder.ID
	}

	// 3. Resolve source folder for IMAP move
	srcFolder := "INBOX"
	if email.FolderID != "" {
		if f, fErr := h.Store.GetFolderByID(ctx, email.FolderID); fErr == nil && f != nil {
			srcFolder = f.Name
		}
	}

	// 4. Already in Trash? Hard delete. Otherwise move to Trash.
	if trashFolderID != "" && email.FolderID == trashFolderID {
		if err := h.Store.DeleteEmail(ctx, emailID, accountID); err != nil {
			WriteInternalError(w, r, err)
			return
		}
		AppMetrics.EmailsDeleted.Add(1)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]bool{"deleted": true})
		return
	}

	// 5. Atomic move + enqueue (rolls back on enqueue failure)
	if email.UID > 0 {
		if err := h.Store.MoveEmailAndEnqueueIMAP(ctx, emailID, accountID, trashFolderID, "Trash", srcFolder, email.UID); err != nil {
			WriteInternalError(w, r, err)
			return
		}
		if h.SyncManager != nil {
			h.SyncManager.WakeUpAccountNow(accountID)
		}
	} else {
		// UID == 0 — draft or locally-only email, move locally only
		if err := h.Store.MoveEmail(ctx, emailID, accountID, trashFolderID); err != nil {
			WriteInternalError(w, r, err)
			return
		}
	}

	AppMetrics.EmailsDeleted.Add(1)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]bool{"deleted": true})
}

func (h *Handler) RestoreFromTrash(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		WriteJSONError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	pathParts := strings.Split(strings.Trim(r.URL.Path, "/"), "/")
	emailID := pathParts[len(pathParts)-1]

	// Find INBOX folder for the account
	email, err := h.Store.GetEmail(r.Context(), emailID, r.URL.Query().Get("account_id"))
	if err != nil || email == nil {
		WriteJSONError(w, http.StatusNotFound, "email not found")
		return
	}
	if err := h.CheckAccountAccess(r.Context(), email.AccountID); err != nil {
		WriteJSONError(w, http.StatusForbidden, "access denied")
		return
	}

	// Find INBOX folder for the account — validate early
	inboxFolder, err := h.Store.GetFolderByName(r.Context(), email.AccountID, "INBOX")
	if err != nil {
		WriteInternalError(w, r, err)
		return
	}
	if inboxFolder == nil {
		WriteJSONError(w, http.StatusInternalServerError, "INBOX folder not found")
		return
	}

	// Resolve source folder (Trash or email's current folder)
	srcFolder := "Trash"
	if email.FolderID != "" {
		if f, fErr := h.Store.GetFolderByID(r.Context(), email.FolderID); fErr == nil && f != nil {
			srcFolder = f.Name
		}
	}

	// Atomic move + enqueue
	if email.UID > 0 {
		if err := h.Store.MoveEmailAndEnqueueIMAP(r.Context(), emailID, email.AccountID, inboxFolder.ID, "INBOX", srcFolder, email.UID); err != nil {
			WriteInternalError(w, r, err)
			return
		}
		if h.SyncManager != nil {
			h.SyncManager.WakeUpAccountNow(email.AccountID)
		}
	} else {
		// UID == 0 — local-only move
		if err := h.Store.MoveEmail(r.Context(), emailID, r.URL.Query().Get("account_id"), inboxFolder.ID); err != nil {
			WriteInternalError(w, r, err)
			return
		}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "restored"})
}

func (h *Handler) Search(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query().Get("q")
	if query == "" {
		WriteJSONError(w, http.StatusBadRequest, "query required")
		return
	}

	accountID := r.URL.Query().Get("account_id")
	if accountID != "" && accountID != "unified" {
		if err := h.CheckAccountAccess(r.Context(), accountID); err != nil {
			WriteAccessError(w, err)
			return
		}
	}
	folderID := r.URL.Query().Get("folder_id")

	cleanQuery := query
	replacer := strings.NewReplacer("@", " ", ".", " ", "<", " ", ">", " ", "-", " ", "_", " ")
	cleanQuery = replacer.Replace(cleanQuery)

	results, err := h.Store.SearchFTS(r.Context(), cleanQuery, accountID, 100)
	if err != nil || len(results) == 0 {
		if err != nil {
			slog.Info("FTS search error, falling back to SQL", "error", err)
		}
		results = nil
	}

	var emails []models.Email
	if len(results) > 0 {
		emails, err = h.Store.GetEmailsByIDs(r.Context(), results)
		if err != nil {
			slog.Info("Search GetEmailsByIDs error", "error", err)
		}
	}

	if len(emails) == 0 {
		emails, err = h.Store.SearchEmails(r.Context(), cleanQuery, accountID, 100)
		if err != nil {
			AppMetrics.HTTPErrors.Add(1)
			WriteInternalError(w, r, err)
			return
		}
	}

	if folderID != "" {
		var filtered []models.Email
		for _, e := range emails {
			if e.FolderID == folderID {
				filtered = append(filtered, e)
			}
		}
		emails = filtered
	}

	emails, err = h.filterEmailsByMonoAccess(r.Context(), emails)
	if err != nil {
		WriteAccessError(w, err)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(emails)
}
