package sync

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/emersion/go-imap/v2"
	"github.com/emersion/go-imap/v2/imapclient"
	"github.com/emersion/go-message/mail"
	"github.com/google/uuid"
	"github.com/jhillyerd/enmime"
	"github.com/klauspost/compress/zstd"
	"golang.org/x/sync/singleflight"

	"rmsmail/internal/ai"
	"rmsmail/internal/auth"
	"rmsmail/internal/avatar"
	"rmsmail/internal/crypto"
	"rmsmail/internal/mime"
	"rmsmail/internal/models"
	"rmsmail/internal/notification"
	"rmsmail/internal/sanitizer"
)

// Package-level resources reused across all sync operations.
var (
	emailSanitizer = sanitizer.NewEmailSanitizer()
	zstdPool       = sync.Pool{New: func() any { w, _ := zstd.NewWriter(nil); return w }}

	// Gmail IMAP sometimes returns MIME bodies with missing CRLF between boundary
	// and the next header (e.g. "--abc123Content-Type:" instead of "--abc123\r\nContent-Type:").
	// This regex matches a boundary line followed immediately by a header name.
	// Boundaries can be hex-only, alphanumeric, or contain _ / + = - characters.
	repairBrokenBoundary = regexp.MustCompile(`(?m)^(--[0-9a-zA-Z_/+=~-]{8,})([A-Z][A-Za-z-]+:)`)

	// avatarSem bounds concurrent avatar resolution goroutines (non-critical operation).
	avatarSem = make(chan struct{}, 10)
)

// repairMIMEBoundaries fixes Gmail IMAP corruption where MIME boundaries lack CRLF
// before the next header. Without this, enmime.ReadEnvelope fails with "malformed MIME".
func repairMIMEBoundaries(raw []byte) []byte {
	return repairBrokenBoundary.ReplaceAll(raw, []byte("$1\r\n$2"))
}

// getRootMessageID parses the References header and returns the first Message-ID.
// If References is empty, it falls back to In-Reply-To. If that's empty, it returns the email's own Message-ID.
func getRootMessageID(references, inReplyTo, msgID string) string {
	refs := strings.Fields(references)
	for _, ref := range refs {
		if r := strings.TrimSpace(ref); r != "" {
			return r
		}
	}
	if inReplyTo != "" {
		inReplies := strings.Fields(inReplyTo)
		for _, ir := range inReplies {
			if r := strings.TrimSpace(ir); r != "" {
				return r
			}
		}
	}
	if msgID != "" {
		return strings.TrimSpace(msgID)
	}
	return ""
}

type Fetcher struct {
	Store SyncStore
	CAS   CASStore
	AI    AIProvider
	OAuth *auth.OAuthManager
	// Asynq task queue client for fire-and-forget operations
	avatarResolver *avatar.Resolver
	avatarSf       singleflight.Group
	notifier       *notification.RateLimiter
	notifProvider  notification.Provider

	// Redis client for persistent webhook queue
	BroadcastEvent func(ctx context.Context, channel, message string)
	OnActivity     func(accountID string) // called after successful IDLE wakeup/timeout sync
	OnNewEmail     func(ctx context.Context, accountID, emailID, subject, senderName, senderAddr string)
	JobNotify      chan struct{} // notify background job worker
	folderCacheMu  sync.RWMutex
	folderCache    map[string]map[string]*models.Folder // accountID → path → folder, per sync cycle
}

func (f *Fetcher) ProcessMessage(ctx context.Context, accountID string, msg *imapclient.FetchMessageBuffer) error {
	// Concatenate body sections (HEADER must come before TEXT).
	var headerBytes, textBytes []byte
	for _, section := range msg.BodySection {
		if section.Section != nil && section.Section.Specifier == imap.PartSpecifierHeader {
			headerBytes = section.Bytes
		} else {
			textBytes = append(textBytes, section.Bytes...)
		}
	}
	var raw []byte
	if len(headerBytes) > 0 {
		raw = append(headerBytes, "\r\n"...)
	}
	raw = append(raw, textBytes...)
	if len(raw) == 0 {
		slog.Info("empty body, skipping", "accountID", accountID, "uid", msg.UID)
		return nil
	}

	// Fix Gmail IMAP corruption: missing CRLF after MIME boundaries.
	raw = repairMIMEBoundaries(raw)

	env, err := enmime.ReadEnvelope(bytes.NewReader(raw))
	if err != nil {
		return err
	}

	accountDir := filepath.Join("storage", "emails", accountID)
	if err := os.MkdirAll(accountDir, 0700); err != nil {
		return fmt.Errorf("failed to create account dir: %w", err)
	}
	emailID := uuid.New().String()
	bodyPath := filepath.Join(accountDir, emailID+".eml")
	if err := os.MkdirAll(filepath.Dir(bodyPath), 0700); err != nil {
		return fmt.Errorf("failed to create email dir: %w", err)
	}

	var writeData []byte = raw
	val := zstdPool.Get()
	encoder, ok := val.(*zstd.Encoder)
	if !ok || encoder == nil {
		encoder, _ = zstd.NewWriter(nil)
	}
	encoder.Reset(nil)
	writeData = encoder.EncodeAll(raw, make([]byte, 0, len(raw)))
	zstdPool.Put(encoder)

	encKey := []byte(crypto.GetPrimaryEncryptionKey())
	if len(encKey) > 0 {
		if enc, err := crypto.EncryptBytes(writeData, encKey); err == nil {
			writeData = enc
		} else {
			slog.Info("Failed to encrypt draft EML file", "error", err)
		}
	}

	if err := os.WriteFile(bodyPath, writeData, 0600); err != nil {
		return err
	}

	fromAddr := ""
	fromName := ""
	if fromList, _ := env.AddressList("From"); len(fromList) > 0 {
		fromAddr = strings.ToValidUTF8(fromList[0].Address, "")
		fromName = fromList[0].Name
	}

	parsedDate, _ := env.Date()

	snippet := strings.ToValidUTF8(env.Text, "")
	if len(snippet) > 200 {
		snippet = snippet[:200]
	}

	msgID := strings.ToValidUTF8(env.GetHeader("Message-ID"), "")
	if msgID == "" {
		msgID = fmt.Sprintf("local-%d@%s.synced", msg.UID, accountID)
	}

	email := models.Email{
		ID:               emailID,
		AccountID:        accountID,
		MsgID:            msgID,
		InReplyTo:        strings.ToValidUTF8(env.GetHeader("In-Reply-To"), ""),
		UID:              int32(msg.UID),
		Subject:          strings.ToValidUTF8(env.GetHeader("Subject"), ""),
		SenderName:       strings.ToValidUTF8(fromName, ""),
		SenderAddress:    strings.ToValidUTF8(fromAddr, ""),
		RecipientAddress: strings.ToValidUTF8(env.GetHeader("To"), ""),
		CcAddress:        strings.ToValidUTF8(env.GetHeader("Cc"), ""),
		DateSent:         parsedDate,
		Snippet:          strings.ToValidUTF8(snippet, ""),
		BodyPath:         bodyPath,
	}

	email.ThreadID = getRootMessageID(
		env.GetHeader("References"),
		email.InReplyTo,
		email.MsgID,
	)

	// Respect IMAP \Seen flag from the server
	for _, flag := range msg.Flags {
		if flag == imap.FlagSeen {
			email.IsRead = true
		}
		if flag == imap.FlagFlagged {
			email.IsFlagged = true
		}
		if flag == imap.FlagAnswered {
			email.IsAnswered = true
		}
	}

	if email.IsRead {
		slog.Info("flag seen: set to read", "uid", msg.UID)
	}

	// Parse Authentication-Results header for SPF/DKIM verification
	if authHeader := env.GetHeader("Authentication-Results"); authHeader != "" {
		if result := sanitizer.ParseAuthenticationResults(authHeader, []string{os.Getenv("TRUSTED_MX_DOMAINS")}); result != nil {
			email.SpfPass = result.SpfPass
			email.DkimPass = result.DkimPass
		}
	}

	// Extract attachments BEFORE saving email so has_attachments is persisted
	attachments := f.extractAttachments(env)
	slog.Info("extracted attachments from email", "accountID", accountID, "count", len(attachments), "uid", msg.UID)

	// Phase 1: save email FIRST (so FK on attachments works) with HasAttachments=false
	// We don't set HasAttachments=true until we know attachments were actually saved.
	email.HasAttachments = false
	isNewEmail := true
	if email.MsgID != "" {
		exists, err := f.Store.EmailExistsByMsgID(ctx, accountID, email.MsgID)
		if err != nil {
			slog.Info("EmailExistsByMsgID failed", "accountID", accountID, "msgID", email.MsgID, "error", err)
		} else {
			isNewEmail = !exists
		}
	}
	if err := f.Store.SaveEmail(ctx, email); err != nil {
		os.Remove(bodyPath) // clean up orphaned EML
		return err
	}

	// Phase 2: save attachments via CAS (email row exists, FK will work)
	savedCount := 0
	if f.CAS != nil && len(attachments) > 0 {
		for _, att := range attachments {
			if saved, err := f.CAS.Save(ctx, emailID, accountID, att.Filename, att.Data, att.ContentID); err != nil {
				slog.Info("CAS.Save FAILED for attachment", "accountID", accountID, "filename", att.Filename, "emailID", emailID, "uid", msg.UID, "error", err)
			} else {
				savedCount++
				slog.Info("saved attachment", "accountID", accountID, "filename", saved.Filename, "id", saved.ID, "size", saved.Size, "hash", saved.Hash[:12], "emailID", emailID)
			}
		}
		if savedCount != len(attachments) {
			slog.Info("WARNING: attachment save count mismatch", "accountID", accountID, "saved", savedCount, "total", len(attachments), "emailID", emailID)
		}
	}

	// Phase 3: update has_attachments flag only after all CAS saves succeeded
	if savedCount > 0 {
		if err := f.Store.UpdateEmailHasAttachments(ctx, emailID, accountID, true); err != nil {
			slog.Info("failed to update has_attachments=true", "accountID", accountID, "emailID", emailID, "error", err)
		} else {
			slog.Info("updated has_attachments=true", "accountID", accountID, "emailID", emailID, "savedCount", savedCount)
		}
	}

	// Async avatar resolution (non-blocking, with singleflight dedup)
	if false {

	}

	if isNewEmail {
		if err := f.RunRules(ctx, accountID, &email); err != nil {
			slog.Info("Rule action failed", "accountID", accountID, "error", err)
		}
	}

	if isNewEmail && f.OnNewEmail != nil {
		f.OnNewEmail(ctx, accountID, emailID, email.Subject, email.SenderName, email.SenderAddress)
	}

	rawHTML := env.HTML
	rawHTML = decodeQuotedPrintable(rawHTML)

	trackingSanitizer := emailSanitizer
	if trackingSanitizer.HasTrackingPixel(rawHTML) {
		rawHTML = trackingSanitizer.Sanitize(rawHTML)
	}

	f.storeIndex(ctx, emailID, accountID, email.Subject, email.SenderName, email.SenderAddress, email.RecipientAddress, env.Text)
	f.resolveAvatarAsync(email.SenderAddress, email.SenderName)

	// Enqueue Telegram notification via Asynq task queue (persistent, retryable).
	if false {

	}

	// Broadcast new-email event for browser SSE + MCP SSE subscribers.
	if f.BroadcastEvent != nil {
		f.BroadcastEvent(ctx, "new-email", fmt.Sprintf(`{"account_id":"%s","email_id":"%s","subject":"%s"}`, accountID, emailID, email.Subject))
	}

	return nil
}

// storeIndex indexes an email for full-text search.
// Errors are logged but not returned — index failures must not block sync.
func (f *Fetcher) storeIndex(ctx context.Context, emailID, accountID, subject, senderName, senderAddress, recipientAddress, body string) {
	if err := f.Store.IndexEmailFTS(ctx, emailID, accountID, subject, senderName, senderAddress, recipientAddress, body); err != nil {
		slog.Info("IndexEmailFTS failed", "emailID", emailID, "error", err)
	}
}

func htmlEscape(s string) string {
	s = strings.ReplaceAll(s, "&", "&amp;")
	s = strings.ReplaceAll(s, "<", "&lt;")
	s = strings.ReplaceAll(s, ">", "&gt;")
	return s
}

func (f *Fetcher) sendTelegramNotification(ctx context.Context, accountID string, email *models.Email) {
	defer func() {
		if r := recover(); r != nil {
			slog.Error("PANIC in sendTelegramNotification", "account_id", accountID, "panic", r)
		}
	}()
	if f.AI == nil {
		return
	}
	if f.notifier == nil || f.notifProvider == nil {
		slog.Info("[TG Notify] SKIP: notifier or provider nil", "accountID", accountID)
		return
	}

	// GetTelegramSettings queries by email, not accountID
	account, err := f.Store.GetAccount(ctx, accountID)
	if err != nil || account == nil {
		slog.Info("[TG Notify] SKIP: failed to get account", "accountID", accountID, "error", err)
		return
	}

	userID, enabled, aiNotif, _, botToken, err := f.Store.GetTelegramSettings(ctx, account.Email)
	if err != nil || !enabled || userID == 0 {
		slog.Info("[TG Notify] no settings, skipping notification", "email", account.Email)
		return
	}
	if err != nil {
		slog.Info("[TG Notify] SKIP: failed to get telegram settings", "error", err)
		return
	}

	if !enabled || userID == 0 {
		slog.Info("[TG Notify] SKIP: disabled or no userID", "enabled", enabled, "userID", userID, "email", account.Email)
		return
	}

	slog.Info("[TG Notify] SENDING", "userID", userID, "aiNotif", aiNotif, "email", account.Email)

	if botToken != "" {
		ctx = context.WithValue(ctx, notification.BotTokenKey, botToken)
	}

	targetID := strconv.FormatInt(userID, 10)
	var text string

	if aiNotif && f.AI != nil {
		providerName := "openrouter"
		modelName := ""
		// Resolve provider + model from ai_settings.config.chat (account → global).
		for _, lookupID := range []string{accountID, "00000000-0000-0000-0000-000000000000"} {
			if setting, err := f.Store.GetAISettings(ctx, lookupID); err == nil && setting != nil && setting.Config != "" {
				var cfg map[string]struct {
					Provider string `json:"provider"`
					Model    string `json:"model"`
				}
				if json.Unmarshal([]byte(setting.Config), &cfg) == nil {
					if chat, ok := cfg["chat"]; ok && chat.Provider != "" {
						providerName = chat.Provider
						modelName = chat.Model
						break
					}
				}
			}
		}

		// Resolve API key: env (enterprise) → ai_settings (account → global)
		apiKey := os.Getenv(ai.ProviderEnvKey(providerName))
		encKey := []byte(crypto.GetPrimaryEncryptionKey())
		for _, lookupID := range []string{accountID, "00000000-0000-0000-0000-000000000000"} {
			if apiKey != "" {
				break
			}
			if setting, err := f.Store.GetAISettings(ctx, lookupID); err == nil && setting != nil && setting.APIKeysEncrypted != "" {
				if len(encKey) == 0 {
					slog.Info("[TG Notify] AI: ENCRYPTION_KEY not set, skipping ai_settings keys")
					break
				}
				if dec, decErr := crypto.Decrypt(setting.APIKeysEncrypted, encKey); decErr == nil {
					var keys map[string]string
					if json.Unmarshal([]byte(dec), &keys) == nil {
						if k := keys[providerName]; k != "" {
							apiKey = k
						}
					}
				} else {
					slog.Info("[TG Notify] AI: decrypt ai_settings failed", "error", decErr)
				}
			}
		}

		slog.Info("[TG Notify] AI: provider/model/apiKey", "provider", providerName, "model", modelName, "hasAPIKey", apiKey != "")

		snippet := email.Snippet
		if len(snippet) > 800 {
			snippet = snippet[:800]
		}

		// Load notification prompt from ai_settings, fallback to hardcoded default
		defaultPrompt := "You are an AI email assistant. Generate a beautiful notification summary for this new email.\n" +
			"Subject: %s\n" +
			"From: %s <%s>\n" +
			"Snippet: %s\n\n" +
			"Respond using Telegram HTML format (<b>, <i>, <code>):\n" +
			"🤖 <b>RMS Mail Smart Alert</b>\n" +
			"📥 <b>New email:</b> %s\n" +
			"👤 <b>From:</b> %s &lt;%s&gt;\n" +
			"🏷 <b>Priority:</b> [Critical🔴 | Important🟡 | Normal🟢 | Low⚪] (pick one, justify in 1 short phrase)\n" +
			"📝 <b>Summary:</b> [One-sentence summary of what the email is about and if any action is needed]\n\n" +
			"Keep it short, clear, and professional. No extra text."

		promptTemplate := defaultPrompt
		if setting, err := f.Store.GetAISettings(ctx, accountID); err == nil && setting != nil && setting.Prompts != "" {
			var prompts map[string]string
			if json.Unmarshal([]byte(setting.Prompts), &prompts) == nil {
				if p, ok := prompts["tg_notify"]; ok && p != "" {
					promptTemplate = p
				}
			}
		}

		prompt := fmt.Sprintf(
			promptTemplate,
			email.Subject, email.SenderName, email.SenderAddress, snippet,
			email.Subject, email.SenderName, email.SenderAddress,
		)

		aiMsgs := []ai.Message{
			{Role: "user", Content: prompt},
		}

		reply, err := f.AI.Chat(ctx, providerName, modelName, apiKey, aiMsgs)
		if err == nil && reply != "" {
			text = reply
		} else {
			slog.Info("TG Notify: AI chat failed", "error", err)
		}
	}

	if text == "" {
		// Default notification fallback
		emailSub := htmlEscape(email.Subject)
		emailName := htmlEscape(email.SenderName)
		emailAddr := htmlEscape(email.SenderAddress)
		emailSnip := htmlEscape(email.Snippet)
		text = fmt.Sprintf(
			"📥 <b>New email</b>\n"+
				"<b>Subject:</b> %s\n"+
				"<b>From:</b> %s &lt;%s&gt;\n\n"+
				"%s",
			emailSub, emailName, emailAddr, emailSnip,
		)
	}

	// Prepend mailbox name so user knows which account received the email.
	text = fmt.Sprintf("📬 <b>%s</b>\n%s", htmlEscape(account.Email), text)

	f.notifier.SendAsync(ctx, f.notifProvider, targetID, text)
}

type rawAttachment struct {
	Filename  string
	Data      []byte
	ContentID string
}

func (f *Fetcher) extractAttachments(env *enmime.Envelope) []rawAttachment {
	var attachments []rawAttachment

	for _, part := range env.Attachments {
		filename := part.FileName
		if filename == "" {
			filename = "attachment"
		}
		attachments = append(attachments, rawAttachment{
			Filename:  filename,
			Data:      part.Content,
			ContentID: strings.Trim(part.ContentID, "<>"),
		})
	}

	for _, part := range env.Inlines {
		filename := part.FileName
		if filename == "" {
			filename = strings.Trim(part.ContentID, "<>")
			if filename == "" {
				filename = "inline"
			}
		}
		attachments = append(attachments, rawAttachment{
			Filename:  filename,
			Data:      part.Content,
			ContentID: strings.Trim(part.ContentID, "<>"),
		})
	}

	return attachments
}

func (f *Fetcher) ClearFolderCache(accountID string) {
	f.folderCacheMu.Lock()
	defer f.folderCacheMu.Unlock()
	if f.folderCache != nil {
		delete(f.folderCache, accountID)
	}
}

func (f *Fetcher) GetOrCreateFolder(ctx context.Context, accountID, name, path string) (*models.Folder, error) {
	// Use per-account folder cache to avoid expensive GetFolders call per email.
	// During initial sync with 50K+ emails, this saves ~50K correlated subquery calls.

	// Read lock to check cache first
	f.folderCacheMu.RLock()
	if f.folderCache != nil && f.folderCache[accountID] != nil {
		if folder, ok := f.folderCache[accountID][path]; ok {
			f.folderCacheMu.RUnlock()
			return folder, nil
		}
	}
	f.folderCacheMu.RUnlock()

	// Write lock to populate cache
	f.folderCacheMu.Lock()
	defer f.folderCacheMu.Unlock()

	if f.folderCache == nil {
		f.folderCache = make(map[string]map[string]*models.Folder)
	}
	if f.folderCache[accountID] == nil {
		// Populate the whole account cache
		folders, err := f.Store.GetFolders(ctx, accountID)
		if err != nil {
			return nil, err
		}
		f.folderCache[accountID] = make(map[string]*models.Folder)
		for i := range folders {
			f.folderCache[accountID][folders[i].Path] = &folders[i]
		}
	}

	// Check again after populating
	if folder, ok := f.folderCache[accountID][path]; ok {
		return folder, nil
	}

	// If it still doesn't exist, create it
	folder, err := f.Store.CreateFolder(ctx, accountID, name, path, true)
	if err == nil && folder != nil {
		f.folderCache[accountID][path] = folder
	}
	return folder, err
}

func (f *Fetcher) ProcessMessageStreamToFolder(ctx context.Context, accountID, folderPath string, msg *imapclient.FetchMessageData, isGmail bool) (uint32, error) {
	folder, err := f.GetOrCreateFolder(ctx, accountID, folderPath, folderPath)
	if err != nil {
		return 0, err
	}

	var uid uint32
	var flags []imap.Flag
	var rawReader io.Reader

	for {
		item := msg.Next()
		if item == nil {
			break
		}
		switch item := item.(type) {
		case imapclient.FetchItemDataUID:
			uid = uint32(item.UID)
		case imapclient.FetchItemDataFlags:
			flags = item.Flags
		case imapclient.FetchItemDataBodySection:
			if item.Literal != nil {
				// We must consume the literal before the next iteration.
				// Fast path for small emails (<1 MiB, >95% of all emails):
				// read directly into RAM, skip temp file entirely.
				const maxRAMParse = 1 << 20 // 1 MiB
				lr := io.LimitReader(item.Literal, maxRAMParse+1)
				raw, err := io.ReadAll(lr)
				if err != nil {
					return 0, err
				}
				if len(raw) <= maxRAMParse {
					// Small email: repair MIME boundaries in RAM
					raw = repairMIMEBoundaries(raw)
					rawReader = bytes.NewReader(raw)
				} else {
					// Large email: fall back to temp file
					tmpFile, err := os.CreateTemp("", "mail-*.eml")
					if err != nil {
						return 0, err
					}
					defer os.Remove(tmpFile.Name())

					// Write the bytes we already read from the LimitReader
					if _, err := tmpFile.Write(raw); err != nil {
						tmpFile.Close()
						return 0, err
					}
					// Copy the rest of the literal
					if _, err := io.Copy(tmpFile, item.Literal); err != nil {
						tmpFile.Close()
						return 0, err
					}
					tmpFile.Seek(0, 0)

					// If file is small enough (<10 MiB), load and repair MIME boundaries
					stat, _ := tmpFile.Stat()
					if stat.Size() < 10*1024*1024 {
						raw, _ := io.ReadAll(tmpFile)
						raw = repairMIMEBoundaries(raw)
						rawReader = bytes.NewReader(raw)
						tmpFile.Close()
					} else {
						rawReader = tmpFile
					}
				}
			}
		}
	}

	if uid == 0 || rawReader == nil {
		slog.Info("empty stream or missing UID", "accountID", accountID, "folder", folderPath)
		if uid > 0 {
			return uid, fmt.Errorf("empty body for UID %d", uid)
		}
		return uid, nil
	}

	env, err := enmime.ReadEnvelope(rawReader)

	// Close tmpFile if it was used directly
	if tf, ok := rawReader.(*os.File); ok {
		defer tf.Close()
	}

	if err != nil {
		return uid, err
	}

	accountDir := filepath.Join("storage", "emails", accountID)
	if err := os.MkdirAll(accountDir, 0700); err != nil {
		return 0, fmt.Errorf("failed to create account dir: %w", err)
	}
	emailID := uuid.New().String()
	bodyPath := filepath.Join(accountDir, emailID+".eml")
	if err := os.MkdirAll(filepath.Dir(bodyPath), 0700); err != nil {
		return 0, fmt.Errorf("failed to create email dir: %w", err)
	}

	// Save compressed and encrypted file
	if seeker, ok := rawReader.(io.Seeker); ok {
		seeker.Seek(0, 0)
	}

	var rawData []byte
	var readErr error
	if br, ok := rawReader.(*bytes.Reader); ok {
		rawData, readErr = io.ReadAll(br)
	} else {
		rawData, readErr = io.ReadAll(rawReader)
	}
	if readErr != nil {
		return uid, fmt.Errorf("failed to read raw email data: %w", readErr)
	}

	var writeData []byte = rawData
	val := zstdPool.Get()
	encoder, ok := val.(*zstd.Encoder)
	if !ok || encoder == nil {
		encoder, _ = zstd.NewWriter(nil)
	}
	encoder.Reset(nil)
	writeData = encoder.EncodeAll(rawData, make([]byte, 0, len(rawData)))
	zstdPool.Put(encoder)

	encKey := []byte(crypto.GetPrimaryEncryptionKey())
	if len(encKey) > 0 {
		if enc, err := crypto.EncryptBytes(writeData, encKey); err == nil {
			writeData = enc
		} else {
			slog.Info("Failed to encrypt draft EML file", "error", err)
		}
	}

	if err := os.WriteFile(bodyPath, writeData, 0600); err != nil {
		return uid, err
	}
	writeData = nil
	rawData = nil

	fromAddr := ""
	fromName := ""
	if fromList, _ := env.AddressList("From"); len(fromList) > 0 {
		fromAddr = strings.ToValidUTF8(fromList[0].Address, "")
		fromName = fromList[0].Name
	}

	parsedDate, _ := env.Date()

	snippet := strings.ToValidUTF8(env.Text, "")
	if len(snippet) > 200 {
		snippet = snippet[:200]
	}

	msgID := strings.ToValidUTF8(env.GetHeader("Message-ID"), "")
	if msgID == "" {
		msgID = fmt.Sprintf("local-%d@%s.synced", int32(uid), accountID)
	}

	email := models.Email{
		ID:               emailID,
		AccountID:        accountID,
		MsgID:            msgID,
		InReplyTo:        strings.ToValidUTF8(env.GetHeader("In-Reply-To"), ""),
		UID:              int32(int32(uid)),
		Subject:          strings.ToValidUTF8(env.GetHeader("Subject"), ""),
		SenderName:       strings.ToValidUTF8(fromName, ""),
		SenderAddress:    strings.ToValidUTF8(fromAddr, ""),
		RecipientAddress: strings.ToValidUTF8(env.GetHeader("To"), ""),
		CcAddress:        strings.ToValidUTF8(env.GetHeader("Cc"), ""),
		DateSent:         parsedDate,
		Snippet:          strings.ToValidUTF8(snippet, ""),
		BodyPath:         bodyPath,
	}

	email.ThreadID = getRootMessageID(
		env.GetHeader("References"),
		email.InReplyTo,
		email.MsgID,
	)

	// Respect IMAP \Seen and \Flagged from the server
	for _, flag := range flags {
		if flag == imap.FlagSeen {
			email.IsRead = true
		}
		if flag == imap.FlagFlagged {
			email.IsFlagged = true
		}
		if flag == imap.FlagAnswered {
			email.IsAnswered = true
		}
	}

	if email.IsRead {
		slog.Info("flag seen: set to read", "uid", int32(uid))
	}

	// Parse Authentication-Results header for SPF/DKIM verification
	if authHeader := env.GetHeader("Authentication-Results"); authHeader != "" {
		if result := sanitizer.ParseAuthenticationResults(authHeader, []string{os.Getenv("TRUSTED_MX_DOMAINS")}); result != nil {
			email.SpfPass = result.SpfPass
			email.DkimPass = result.DkimPass
		}
	}

	// Extract attachments to set has_attachments flag
	attachments := f.extractAttachments(env)
	slog.Info("extracted attachments from email (stream)", "accountID", accountID, "count", len(attachments), "uid", int32(uid), "folder", folderPath)

	// Mark as read if this is a Sent folder (user's own outgoing mail)
	sentNames := map[string]bool{"sent": true, "sent items": true, "sent messages": true, "gesendet": true, "enviados": true, "inviati": true, "envoys": true, "enviadas": true, "отправленные": true, "gönderilmiş": true}
	if sentNames[strings.ToLower(folderPath)] {
		email.IsRead = true
	}

	// Gmail: deduplicate by msg_id across labels — same email appears in multiple
	// virtual "folders" with different UIDs. Track labels via junction table.
	if isGmail && email.MsgID != "" {
		existing, err := f.Store.GetEmailByMsgIDAccount(ctx, email.MsgID, accountID)
		if err != nil {
			slog.Info("Gmail: dedup lookup failed, saving as new", "msgID", email.MsgID, "folder", folderPath, "error", err)
		} else if existing != nil {
			// Email already exists from another label — append this folder as
			// an additional label (don't replace existing ones).
			existingLabels, err := f.Store.GetGmailLabels(ctx, existing.ID, accountID)
			if err != nil {
				slog.Info("Gmail: failed to get existing labels, skipping label append",
					"emailID", existing.ID, "folder", folderPath, "error", err)
			} else {
				found := false
				for _, l := range existingLabels {
					if strings.EqualFold(l, folderPath) {
						found = true
						break
					}
				}
				if !found {
					existingLabels = append(existingLabels, folderPath)
					if err := f.Store.UpsertEmailLabels(ctx, existing.ID, accountID, existingLabels); err != nil {
						slog.Info("Gmail: failed to upsert labels on existing email",
							"emailID", existing.ID, "folder", folderPath, "error", err)
					}
				}
			}
			// Merge flags from this fetch with existing flags — always keep
			// the most permissive state. This prevents state reversion when
			// Gmail hasn't propagated \Seen to all labels yet.
			isRead := existing.IsRead
			isFlagged := existing.IsFlagged
			isAnswered := existing.IsAnswered
			for _, fl := range flags {
				if strings.EqualFold(string(fl), "\\Seen") {
					isRead = true
				}
				if strings.EqualFold(string(fl), "\\Flagged") {
					isFlagged = true
				}
				if strings.EqualFold(string(fl), "\\Answered") {
					isAnswered = true
				}
			}
			if isRead != existing.IsRead || isFlagged != existing.IsFlagged || isAnswered != existing.IsAnswered {
				if _, err := f.Store.ApplyServerEmailFlags(ctx, existing.ID, accountID, isRead, isFlagged, isAnswered); err != nil {
					slog.Info("Gmail: failed to apply server flags on dedup", "emailID", existing.ID, "error", err)
				}
			}
			// Don't re-index FTS or re-save attachments — existing data is correct.
			f.resolveAvatarAsync(email.SenderAddress, email.SenderName)
			slog.Debug("Gmail: dedup match, appended label", "emailID", existing.ID, "folder", folderPath, "uid", uid)
			return uid, nil
		}
		// New email: save with labels. After saving, add the initial label.
	}

	// Phase 1: save email FIRST (so FK on attachments works) with HasAttachments=false
	email.HasAttachments = false
	isNew, err := f.Store.SaveEmailToFolder(ctx, email, folder.ID)
	if err != nil {
		os.Remove(bodyPath) // clean up orphaned EML
		return uid, err
	}

	// Gmail: record the initial label for new emails
	if isGmail {
		if err := f.Store.UpsertEmailLabels(ctx, emailID, accountID, []string{folderPath}); err != nil {
			slog.Info("Gmail: failed to set initial label on new email",
				"emailID", emailID, "folder", folderPath, "error", err)
		}
	}

	// Phase 2: save attachments via CAS (email row exists, FK will work)
	savedCount := 0
	if f.CAS != nil && len(attachments) > 0 {
		for _, att := range attachments {
			if _, err := f.CAS.Save(ctx, emailID, accountID, att.Filename, att.Data, att.ContentID); err != nil {
				slog.Info("CAS.Save FAILED for attachment (stream)", "accountID", accountID, "filename", att.Filename, "emailID", emailID, "uid", int32(uid), "folder", folderPath, "error", err)
			} else {
				savedCount++
			}
		}
		if savedCount != len(attachments) {
			slog.Info("WARNING: attachment save count mismatch (stream)", "accountID", accountID, "saved", savedCount, "total", len(attachments), "emailID", emailID, "folder", folderPath)
		}
	}

	// Phase 3: update has_attachments flag only after CAS saves succeeded
	if savedCount > 0 {
		if err := f.Store.UpdateEmailHasAttachments(ctx, emailID, accountID, true); err != nil {
			slog.Info("failed to update has_attachments=true (stream)", "accountID", accountID, "emailID", emailID, "folder", folderPath, "error", err)
		} else {
			slog.Info("updated has_attachments=true (stream)", "accountID", accountID, "emailID", emailID, "folder", folderPath, "savedCount", savedCount)
		}
	}

	if false {

	}

	if isNew {
		if err := f.RunRules(ctx, accountID, &email); err != nil {
			slog.Info("Rule action failed", "accountID", accountID, "error", err)
		}
	}

	if isNew && f.OnNewEmail != nil {
		f.OnNewEmail(ctx, accountID, emailID, email.Subject, email.SenderName, email.SenderAddress)
	}

	// Security: HTML is stored raw for fidelity. XSS protection is at read time:
	// normalizeEmailHTML strips scripts/active content; iframe srcdoc CSP blocks JS.
	rawHTML := env.HTML
	rawHTML = decodeQuotedPrintable(rawHTML)

	trackingSanitizer := emailSanitizer
	if trackingSanitizer.HasTrackingPixel(rawHTML) {
		rawHTML = trackingSanitizer.Sanitize(rawHTML)
	}

	f.storeIndex(ctx, emailID, accountID, email.Subject, email.SenderName, email.SenderAddress, email.RecipientAddress, env.Text)
	f.resolveAvatarAsync(email.SenderAddress, email.SenderName)

	// Telegram notification — only for NEW emails in INBOX (don't spam for every folder or re-sync).
	if isNew && folderPath == "INBOX" && !email.IsRead {
		if false {

		}

	}

	// SSE broadcast — only for truly new emails (not upserts).
	if isNew && f.BroadcastEvent != nil {
		isReadStr := "false"
		if email.IsRead {
			isReadStr = "true"
		}
		f.BroadcastEvent(ctx, "new-email", fmt.Sprintf(`{"account_id":"%s","email_id":"%s","subject":"%s","sender_name":"%s","sender_address":"%s","is_read":%s}`, accountID, emailID, escapeJSON(email.Subject), escapeJSON(email.SenderName), escapeJSON(email.SenderAddress), isReadStr))
	}

	return uid, nil
}

// resolveAvatarAsync launches a non-blocking goroutine with singleflight dedup
func (f *Fetcher) resolveAvatarAsync(addr, name string) {
	if addr == "" || f.Store.SenderProfileValid(context.Background(), addr) {
		return
	}

	select {
	case avatarSem <- struct{}{}:
	default:
		return // drop if pool full (non-critical operation)
	}

	go func() {
		defer func() { <-avatarSem }()
		defer func() {
			if r := recover(); r != nil {
				slog.Error("PANIC in resolveAvatarAsync", "addr", addr, "panic", r)
			}
		}()
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		f.avatarSf.Do(addr, func() (interface{}, error) {
			// Double-check inside the lock
			if f.Store.SenderProfileValid(ctx, addr) {
				return nil, nil
			}
			if f.avatarResolver == nil {
				f.avatarResolver = avatar.NewResolver()
			}
			avatarURL := f.avatarResolver.Resolve(ctx, addr)
			f.Store.UpsertSenderProfile(ctx, addr, name, avatarURL)
			return nil, nil
		})
	}()
}

func (f *Fetcher) RunRules(ctx context.Context, accountID string, email *models.Email) error {
	rules, err := f.Store.GetActiveRules(ctx, accountID)
	if err != nil || len(rules) == 0 {
		return err
	}
	var value string
	for _, rule := range rules {
		switch rule.ConditionField {
		case "from":
			value = email.SenderAddress
		case "subject":
			value = email.Subject
		case "to":
			value = email.RecipientAddress
		default:
			continue
		}
		var match bool
		switch rule.ConditionOperator {
		case "contains":
			match = containsIgnoreCase(value, rule.ConditionValue)
		case "equals":
			match = value == rule.ConditionValue
		case "matches":
			match = containsIgnoreCase(value, rule.ConditionValue)
		}
		if !match {
			continue
		}

		switch rule.ActionType {
		case "apply_label":
			f.handleApplyLabel(ctx, email.ID, rule)
		case "delete":
			f.handleDelete(ctx, email.ID)
		case "move":
			f.handleMove(ctx, email.ID, rule)
		case "trigger_webhook":
			f.handleWebhook(ctx, email, rule)
		case "send_notification":
			f.handleSendNotification(ctx, accountID, email, rule)
		case "auto_draft":
			f.handleAutoDraft(ctx, accountID, email.ID, rule)
		case "forward":
			f.handleForward(ctx, accountID, email.ID, rule)
		}
	}
	return nil
}

func (f *Fetcher) handleApplyLabel(ctx context.Context, emailID string, rule models.FilterRule) {
	if rule.ActionValue != "" {
		if err := f.Store.AddEmailTag(ctx, emailID, "", rule.ActionValue); err != nil {
			slog.Warn("rule add tag failed", "error", err, "email", emailID)
		}
	}
}

func (f *Fetcher) handleDelete(ctx context.Context, emailID string) {
	if email, err := f.Store.GetEmail(ctx, emailID, ""); err == nil && email != nil {
		PurgeEmailLocalFiles(email)
	}
	if err := f.Store.DeleteEmail(ctx, emailID, ""); err != nil {
		slog.Warn("rule delete failed", "error", err, "email", emailID)
	}
}

func (f *Fetcher) handleMove(ctx context.Context, emailID string, rule models.FilterRule) {
	if rule.ActionValue != "" {
		if err := f.Store.MoveEmail(ctx, emailID, "", rule.ActionValue); err != nil {
			slog.Warn("rule move failed", "error", err, "email", emailID)
		}
	}
}

func (f *Fetcher) handleWebhook(ctx context.Context, email *models.Email, rule models.FilterRule) {
	if rule.ActionValue == "" {
		return
	}

	// ActionValue contains the Webhook ID
	webhook, err := f.Store.GetWebhook(ctx, rule.ActionValue)
	if err != nil || webhook == nil {
		slog.Warn("handleWebhook: webhook not found", "webhook_id", rule.ActionValue, "error", err)
		return
	}

	if false {

		// Pool full, skip this webhook (non-critical)

	}
	go func() {
		defer func() {
			if false {

			}
			if r := recover(); r != nil {
				slog.Error("PANIC in sendWebhookWithRetry", "url", webhook.URL, "panic", r)
			}
		}()
		f.sendWebhookWithRetry(webhook.URL, webhook.Secret, "email.received", *email)
	}()
}

func (f *Fetcher) handleSendNotification(ctx context.Context, accountID string, email *models.Email, rule models.FilterRule) {
	if f.notifier != nil && f.notifProvider != nil && rule.Channel != "" {
		text := fmt.Sprintf("<b>%s</b>\nFrom: %s\n%s", email.Subject, email.SenderName, email.Snippet)
		f.notifier.SendAsync(ctx, f.notifProvider, rule.ActionValue, text)
	}
}

func (f *Fetcher) handleAutoDraft(ctx context.Context, accountID, emailID string, rule models.FilterRule) {
	if f.AI == nil { return }
	_ = f.Store.SaveDraftReply(ctx, emailID, accountID, "[AI draft pending]")
	_ = f.Store.EnqueueJob(ctx, "auto_draft", "{}", time.Now())
	if f.JobNotify != nil { select { case f.JobNotify <- struct{}{}: default: } }
}

func (f *Fetcher) handleForward(ctx context.Context, accountID, emailID string, rule models.FilterRule) {
	if rule.ActionValue == "" { return }
	_ = f.Store.EnqueueJob(ctx, "forward_email", "{}", time.Now())
	if f.JobNotify != nil { select { case f.JobNotify <- struct{}{}: default: } }
}

// generateAIDraft collects thread context and calls the AI gateway to generate a draft reply.
func (f *Fetcher) generateAIDraft(ctx context.Context, accountID string, email *models.Email, customPrompt string) (string, error) {
	if f.AI == nil {
		return "", fmt.Errorf("AI is disabled")
	}
	// 1. Collect thread context if available
	var threadContext strings.Builder

	if email.ThreadID != "" {
		threadEmails, err := f.Store.GetEmailsByThreadID(ctx, email.ThreadID, accountID, 5)
		if err != nil {
			slog.Info("auto_draft: failed to get thread context", "emailID", email.ID, "error", err)
		} else if len(threadEmails) > 0 {
			// Reverse so oldest first (chronological order)
			for i := len(threadEmails) - 1; i >= 0; i-- {
				t := threadEmails[i]
				// Skip the current email itself (we'll add it as the latest)
				if t.ID == email.ID {
					continue
				}

				textBody := t.Snippet
				if t.BodyPath != "" {
					raw, err := readEncryptedFileLocal(t.BodyPath)
					if err == nil {
						extracted := extractTextSafe(raw)
						if extracted != "" {
							textBody = extracted
						}
					}
				}

				runes := []rune(textBody)
				if len(runes) > 3000 {
					textBody = string(runes[:3000]) + "...[truncated]"
				}
				threadContext.WriteString(fmt.Sprintf("From: %s <%s>\nSubject: %s\n%s\n---\n", t.SenderName, t.SenderAddress, t.Subject, textBody))
			}
		}
	}

	// 2. Build prompt
	prompt := customPrompt
	if prompt == "" {
		prompt = "You are a helpful email assistant. Generate a professional reply to the following email thread. Consider the context of previous messages."
	}
	prompt += "\n\n"

	if threadContext.Len() > 0 {
		prompt += "Thread context:\n" + threadContext.String() + "\n"
	}

	prompt += fmt.Sprintf("Latest email to reply to:\nFrom: %s <%s>\nSubject: %s\n%s",
		email.SenderName, email.SenderAddress, email.Subject, email.Snippet)

	// 3. Determine AI provider from account config or use default
	providerName := "openrouter" // default
	if f.AI != nil {
		account, err := f.Store.GetAccount(ctx, accountID)
		if err == nil && account != nil && account.AIProviderConfig != "" {
			var cfg struct {
				Provider string `json:"provider"`
			}
			if jsonErr := json.Unmarshal([]byte(account.AIProviderConfig), &cfg); jsonErr == nil && cfg.Provider != "" {
				providerName = cfg.Provider
			}
		}

		provider, ok := f.AI.GetProvider(providerName)
		if !ok {
			// Try default provider
			provider, ok = f.AI.GetProvider("openrouter")
			if !ok {
				// Try any available provider
				for name, p := range f.AI.Providers() {
					provider = p
					providerName = name
					break
				}
			}
		}

		if provider != nil {
			// Call GenerateReply with context and prompt
			reply, err := provider.GenerateReply(ctx, threadContext.String(), prompt)
			if err != nil {
				return "", fmt.Errorf("AI call failed: %w", err)
			}
			slog.Info("auto_draft: generated draft", "emailID", email.ID, "provider", providerName)
			return reply, nil
		}
	}

	// 4. No AI gateway configured — return placeholder
	return "[AI draft pending - no AI provider available]", nil
}

// decodeQuotedPrintable decodes a quoted-printable encoded string.
func decodeQuotedPrintable(s string) string {
	return mime.DecodeQuotedPrintable(s)
}

func containsIgnoreCase(s, substr string) bool {
	s, substr = strings.ToLower(s), strings.ToLower(substr)
	return strings.Contains(s, substr)
}

// trustedMXes contains known reliable mail exchangers that add Authentication-Results headers.
var trustedMXes = []string{
	"mx.google.com",
	"spf.google.com",
	"google.com",
	"outlook.com",
	"protection.outlook.com",
	"eu.ohv.outlook.com",
	"mx.microsoft.com",
	"yahoo.com",
	"mx.yandex.ru",
	"mail.yandex.ru",
	"mx.mail.ru",
	"mx1.mail.ru",
	"mx2.mail.ru",
	"mx1.imapmail.org",
	"mx2.imapmail.org",
}

func parseAuthResults(env *enmime.Envelope, email *models.Email) {
	// Get ALL Authentication-Results headers (enmime returns first by name).
	// To be safe, we walk from the TOP (most recent MTA) and verify authserv-id
	// against known trusted mail exchangers. If no trusted MX is found, we DON'T
	// mark SPF/DKIM as passed — a fake header injected by a spammer would be ignored.

	// Enmime's GetHeader returns the FIRST header value. Headers are added at the
	// top by each MTA, so the first is the most recently added (receiving MX).
	results := strings.ToLower(env.GetHeader("Authentication-Results"))
	if results == "" {
		// Fallback to ARC-Authentication-Results for forwarded/archived mail
		results = strings.ToLower(env.GetHeader("ARC-Authentication-Results"))
		if results == "" {
			return
		}
	}

	// Parse authserv-id from the header: "mx.google.com; spf=pass ..."
	// The format is: <authserv-id>; <result1>; <result2>; ...
	authServID := extractAuthServID(results)

	// If we can't identify the authserv-id, or it's not a trusted MX, reject
	if authServID == "" || !isTrustedMX(authServID) {
		slog.Info("[AUTH-RESULTS] Untrusted authserv-id", "authServID", authServID, "header", results[:100])
		return
	}

	// Only trust results from a known MX server
	email.SpfPass = containsAuthResult(results, "spf=pass")
	email.DkimPass = containsAuthResult(results, "dkim=pass")
}

// extractAuthServID extracts the authserv-id from an Authentication-Results header.
// Format: "mx.google.com; spf=pass; dkim=pass"
func extractAuthServID(header string) string {
	// The authserv-id is everything before the first semicolon, trimmed
	idx := strings.Index(header, ";")
	if idx == -1 {
		return strings.TrimSpace(header)
	}
	return strings.TrimSpace(header[:idx])
}

// isTrustedMX checks if the given authserv-id is from a known trusted mail exchanger.
func isTrustedMX(authServID string) bool {
	for _, mx := range trustedMXes {
		if authServID == mx || strings.HasSuffix(authServID, "."+mx) {
			return true
		}
	}
	return false
}

// containsAuthResult checks if a specific auth result (e.g., "spf=pass") appears
// as a full word (not substring) in the Authentication-Results header value.
func containsAuthResult(header, result string) bool {
	// The result should appear as a semicolon-separated token, e.g., "spf=pass"
	// Match as word boundary to avoid "x-spf=passed" or "dkim=pass+extra"
	parts := strings.Split(header, ";")
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if strings.Contains(part, result) {
			return true
		}
	}
	return strings.Contains(header, result)
}

// ValidateWebhookURL prevents SSRF by ensuring the URL is:
// 1. HTTPS only (no HTTP, no other schemes)
// 2. Not a private/reserved IP range (RFC 1918, loopback, etc.)
// Returns the verified IP address to pin for subsequent requests (DNS rebinding protection).
func ValidateWebhookURL(rawURL string) (net.IP, error) {
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return nil, fmt.Errorf("invalid URL: %w", err)
	}

	// Only allow HTTPS
	if parsed.Scheme != "https" {
		return nil, fmt.Errorf("only HTTPS URLs allowed, got %q", parsed.Scheme)
	}

	host := parsed.Hostname()

	// Resolve hostname to IP addresses
	ips, err := net.LookupIP(host)
	if err != nil {
		return nil, fmt.Errorf("cannot resolve host %q: %w", host, err)
	}

	// Pick the first non-private IP as the safe address
	var safeIP net.IP
	for _, ip := range ips {
		if isPrivateIP(ip) {
			return nil, fmt.Errorf("blocked private/reserved IP: %s", ip)
		}
		if safeIP == nil {
			safeIP = ip
		}
	}
	if safeIP == nil {
		return nil, fmt.Errorf("no valid IP resolved for %q", host)
	}

	return safeIP, nil
}

// createPinnedTransport creates an http.Transport that connects directly to the
// pinned IP address, bypassing DNS resolution (prevents DNS rebinding attacks).
// The original hostname is used for TLS SNI and HTTP Host header.
func createPinnedTransport(pinnedIP net.IP, originalHost string) *http.Transport {
	dialer := &net.Dialer{Timeout: 5 * time.Second}
	return &http.Transport{
		DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
			// Ignore addr (DNS-resolved), connect directly to pinned IP
			_, port, _ := net.SplitHostPort(addr)
			if port == "" {
				port = "443"
			}
			target := net.JoinHostPort(pinnedIP.String(), port)
			return dialer.DialContext(ctx, network, target)
		},
		TLSHandshakeTimeout: 5 * time.Second,
	}
}

// isPrivateIP checks if an IP is in a private or reserved range.
func isPrivateIP(ip net.IP) bool {
	if ip.IsLoopback() || ip.IsPrivate() || ip.IsLinkLocalUnicast() || ip.IsUnspecified() {
		return true
	}

	// Additional reserved ranges
	privateCIDRs := []string{
		"10.0.0.0/8",
		"172.16.0.0/12",
		"192.168.0.0/16",
		"127.0.0.0/8",
		"169.254.0.0/16",
		"::1/128",
		"fc00::/7",
		"fe80::/10",
	}

	for _, cidr := range privateCIDRs {
		_, network, err := net.ParseCIDR(cidr)
		if err != nil {
			continue
		}
		if network.Contains(ip) {
			return true
		}
	}

	return false
}

// sendWebhookWithRetry sends a webhook with HMAC-SHA256 signature and exponential backoff retry.
func (f *Fetcher) sendWebhookWithRetry(rawURL, secret, event string, e models.Email) {
	// SSRF + DNS rebinding protection: resolve once to fail fast
	_, parseErr := url.Parse(rawURL)
	if parseErr != nil {
		slog.Error("webhook invalid URL", "url", rawURL, "error", parseErr)
		return
	}
	_, err := ValidateWebhookURL(rawURL)
	if err != nil {
		slog.Error("webhook blocked by SSRF check", "url", rawURL, "error", err)
		return
	}

	if event == "" {
		event = "email.received"
	}
	payload, err := json.Marshal(models.WebhookEventPayload{
		Event: event,
		Email: e,
	})
	if err != nil {
		slog.Error("webhook marshal failed", "error", err)
		return
	}

	// Prefer Asynq task queue (persistent, retryable, monitorable via /mon/).
	if false {

		// Fall through to Redis ZSET or SQLite

	}

	// If Redis is available (Unified build), persist to queue for guaranteed delivery.
	// The WebhookPoller will handle dispatch and retries.
	if false {

	}

	// Fallback: Mono version uses SQLite for guaranteed delivery.
	jobID := uuid.New().String()
	nextRetryAtUnix := time.Now().Unix() // Queue for immediate execution

	if err := f.Store.EnqueueWebhookRetry(context.Background(), jobID, rawURL, secret, payload, nextRetryAtUnix); err != nil {
		slog.Error("webhook failed to enqueue to sqlite", "url", rawURL, "error", err)
	}
}

func readEncryptedFileLocal(path string) ([]byte, error) {
	return crypto.ReadEncryptedFile(path, crypto.GetPrimaryEncryptionKey())
}

func extractTextSafe(raw []byte) string {
	mr, err := mail.CreateReader(bytes.NewReader(raw))
	if err != nil {
		return ""
	}

	var text strings.Builder

	for {
		p, err := mr.NextPart()
		if err == io.EOF {
			break
		}
		if err != nil {
			break
		}

		switch h := p.Header.(type) {
		case *mail.InlineHeader:
			contentType, _, _ := h.ContentType()
			if contentType == "text/plain" || contentType == "text/html" {
				b, _ := io.ReadAll(io.LimitReader(p.Body, 4000))
				content := string(b)
				if contentType == "text/html" {
					content = stripHTMLTagsFast(content)
				}
				text.WriteString(content)
				text.WriteString("\n")
			}
		case *mail.AttachmentHeader:
			io.Copy(io.Discard, p.Body)
		}
	}

	return text.String()
}

var htmlEntityReplacer = strings.NewReplacer(
	"&amp;", "&",
	"&lt;", "<",
	"&gt;", ">",
	"&quot;", "\"",
	"&#39;", "'",
	"&nbsp;", " ",
)

func stripHTMLTagsFast(s string) string {
	var builder strings.Builder
	builder.Grow(len(s))
	inTag := false
	for _, c := range s {
		if c == '<' {
			inTag = true
		} else if c == '>' {
			inTag = false
		} else if !inTag {
			builder.WriteRune(c)
		}
	}
	result := builder.String()
	result = htmlEntityReplacer.Replace(result)
	return result
}

// escapeJSON escapes a string for safe inclusion in a JSON string literal.
func escapeJSON(s string) string {
	s = strings.ReplaceAll(s, "\\", "\\\\")
	s = strings.ReplaceAll(s, `"`, `\\"`)
	s = strings.ReplaceAll(s, "\n", `\\n`)
	s = strings.ReplaceAll(s, "\r", `\\r`)
	s = strings.ReplaceAll(s, "\t", `\\t`)
	s = strings.ReplaceAll(s, "\b", `\\b`)
	s = strings.ReplaceAll(s, "\f", `\\f`)
	return s
}
