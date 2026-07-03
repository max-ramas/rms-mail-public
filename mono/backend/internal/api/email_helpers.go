package api

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"rmsmail/internal/api/middleware"
	"rmsmail/internal/edition"
	"rmsmail/internal/mime"
	"rmsmail/internal/models"
)

const maxBulkEmailIDs = 10000

// requestPublicBaseURL returns the browser-facing origin (scheme + host) for URLs
// returned to clients behind reverse proxies (Next.js rewrites, nginx, aaPanel).
// Priority: MCP_API_URL → FRONTEND_URL → X-Forwarded-* / TLS on the request.
func requestPublicBaseURL(r *http.Request) string {
	if apiURL := strings.TrimSpace(os.Getenv("MCP_API_URL")); apiURL != "" {
		return strings.TrimSuffix(apiURL, "/")
	}
	if front := strings.TrimSpace(os.Getenv("FRONTEND_URL")); front != "" {
		return strings.TrimSuffix(front, "/")
	}
	host := r.Host
	if fwd := r.Header.Get("X-Forwarded-Host"); fwd != "" {
		host = strings.TrimSpace(strings.Split(fwd, ",")[0])
	}
	return requestForwardedScheme(r) + "://" + host
}

func requestForwardedScheme(r *http.Request) string {
	if proto := r.Header.Get("X-Forwarded-Proto"); proto != "" {
		first := strings.ToLower(strings.TrimSpace(strings.Split(proto, ",")[0]))
		if first == "https" || first == "http" {
			return first
		}
	}
	if r.TLS != nil {
		return "https"
	}
	return "http"
}

// safeBodyPath validates that the given path is within the storage/emails/ directory
// to prevent path traversal attacks.

// stripCRLF removes carriage returns and newlines to prevent SMTP header injection.
func stripCRLF(s string) string {
	s = strings.ReplaceAll(s, "\r", "")
	s = strings.ReplaceAll(s, "\n", "")
	return s
}

func (h *Handler) markRepliedEmailAnswered(ctx context.Context, accountID, inReplyTo string) {
	if strings.TrimSpace(inReplyTo) == "" {
		return
	}
	if err := h.Store.MarkEmailAnsweredByMsgID(ctx, accountID, inReplyTo); err != nil {
		slog.Info("mark replied email answered failed", "accountID", accountID, "inReplyTo", inReplyTo, "error", err)
	}
}

func safeBodyPath(bodyPath string) string {
	if bodyPath == "" {
		return ""
	}
	clean := filepath.Clean(bodyPath)
	// Ensure the path starts with storage/emails/
	if !strings.HasPrefix(clean, "storage/emails/") {
		return ""
	}
	return clean
}

// decodeQuotedPrintable decodes a string that may contain Quoted-Printable
// encoded characters (=XX sequences) and soft line breaks (trailing =).
func decodeQuotedPrintable(s string) string {
	return mime.DecodeQuotedPrintable(s)
}

// fixMojibake reverses UTF-8 double-encoding: Latin-1 bytes that were
// re-encoded as UTF-8 (producing mojibake like â€œ for ") are decoded
// back to the original UTF-8 byte stream.
func fixMojibake(s string) string {
	raw := make([]byte, 0, len(s)/2)
	for _, r := range s {
		if r < 256 {
			raw = append(raw, byte(r))
		} else {
			// Non-Latin-1 rune — already valid UTF-8, keep as-is
			raw = append(raw, string(r)...)
		}
	}
	return string(raw)
}

// ensureEmailAccess loads the email and verifies the caller may access its account.
func (h *Handler) ensureEmailAccess(r *http.Request, emailID string) (*models.Email, error) {
	accountIDParam := r.URL.Query().Get("account_id")
	if accountIDParam == "unified" {
		accountIDParam = ""
	}
	email, err := h.Store.GetEmail(r.Context(), emailID, accountIDParam)
	if err != nil || email == nil {
		return nil, errors.New("email not found")
	}
	if err := h.CheckAccountAccess(r.Context(), email.AccountID); err != nil {
		return nil, err
	}
	return email, nil
}

func (h *Handler) verifyBulkEmailAccess(ctx context.Context, ids []string) error {
	if len(ids) == 0 {
		return nil
	}
	emails, err := h.Store.GetEmailsByIDs(ctx, ids)
	if err != nil {
		return err
	}
	checked := make(map[string]bool)
	for _, e := range emails {
		if checked[e.AccountID] {
			continue
		}
		checked[e.AccountID] = true
		if err := h.CheckAccountAccess(ctx, e.AccountID); err != nil {
			return err
		}
	}
	found := make(map[string]struct{}, len(emails))
	for _, e := range emails {
		found[e.ID] = struct{}{}
	}
	for _, id := range ids {
		if _, ok := found[id]; !ok {
			return errors.New("email not found")
		}
	}
	return nil
}

func (h *Handler) folderBelongsToAccount(ctx context.Context, folderID, accountID string) bool {
	if folderID == "" || accountID == "" {
		return false
	}
	f, err := h.Store.GetFolderByID(ctx, folderID)
	if err != nil || f == nil {
		return false
	}
	return f.AccountID == accountID
}

// resolveBulkMoveTarget maps a drop-target folder UUID to the matching folder in accountID.
// When the UUID belongs to another account, the folder is matched by name.
func (h *Handler) resolveBulkMoveTarget(ctx context.Context, accountID, folderID string, folders []models.Folder) (string, string, error) {
	if folderID == "" {
		return "", "", errors.New("folder_id is required")
	}
	if len(folders) == 0 {
		var err error
		folders, err = h.Store.GetFolders(ctx, accountID)
		if err != nil {
			return "", "", err
		}
	}
	for _, f := range folders {
		if f.ID == folderID {
			return f.ID, f.Name, nil
		}
	}
	ref, err := h.Store.GetFolderByID(ctx, folderID)
	if err != nil || ref == nil {
		return "", "", fmt.Errorf("folder not found")
	}
	refName := ref.Name
	for _, f := range folders {
		if strings.EqualFold(f.Name, refName) {
			return f.ID, f.Name, nil
		}
	}
	byName, err := h.Store.GetFolderByName(ctx, accountID, refName)
	if err == nil && byName != nil {
		return byName.ID, byName.Name, nil
	}
	return "", "", fmt.Errorf("folder %q not found for account", refName)
}

// perAccountScopeIDs returns account IDs for group/unified mono aggregation queries.
func (h *Handler) perAccountScopeIDs(ctx context.Context, accountID string) ([]string, error) {
	if strings.HasPrefix(accountID, "group:") {
		return h.Store.GetGroupAccounts(ctx, strings.TrimPrefix(accountID, "group:"))
	}
	if accountID == "unified" {
		return h.monoAccessibleAccountIDs(ctx)
	}
	return nil, nil
}

// usesPerAccountAggregation is true when counts/IDs must be summed per account (Mono unified, project groups).
func (h *Handler) usesPerAccountAggregation(ctx context.Context, accountID string) (bool, error) {
	if strings.HasPrefix(accountID, "group:") {
		return true, nil
	}
	if accountID == "unified" {
		ids, err := h.monoAccessibleAccountIDs(ctx)
		return len(ids) > 0, err
	}
	return false, nil
}

func (h *Handler) isMultiAccountBulkFilter(accountID string) bool {
	return accountID == "unified" || strings.HasPrefix(accountID, "group:")
}

func (h *Handler) bulkFilterAccountIDs(ctx context.Context, accountID, sourceFolderID string) ([]string, error) {
	if strings.HasPrefix(accountID, "group:") {
		return h.Store.GetGroupAccounts(ctx, strings.TrimPrefix(accountID, "group:"))
	}
	if accountID == "unified" {
		monoIDs, err := h.monoAccessibleAccountIDs(ctx)
		if err != nil {
			return nil, err
		}
		if len(monoIDs) > 0 {
			return monoIDs, nil
		}
		return h.Store.GetAccountIDsByFilter(ctx, accountID, sourceFolderID)
	}
	return nil, nil
}

func (h *Handler) runBulkFilterOp(
	ctx context.Context,
	accountID, sourceFolderID, perAccountFolder string,
	op func(context.Context, string, string) (int64, error),
) (int64, error) {
	if h.isMultiAccountBulkFilter(accountID) {
		accIDs, err := h.bulkFilterAccountIDs(ctx, accountID, sourceFolderID)
		if err != nil {
			return 0, err
		}
		var total int64
		for _, accID := range accIDs {
			if chkErr := h.CheckAccountAccess(ctx, accID); chkErr != nil {
				continue
			}
			accFolder := perAccountFolder
			if !h.isMultiAccountBulkFilter(accID) {
				if folders, fErr := h.Store.GetFolders(ctx, accID); fErr == nil {
					for _, fld := range folders {
						if strings.EqualFold(fld.Name, accFolder) {
							accFolder = fld.ID
							break
						}
					}
				}
			}
			n, rErr := op(ctx, accID, accFolder)
			if rErr != nil {
				return 0, rErr
			}
			total += n
		}
		return total, nil
	}
	if chkErr := h.CheckAccountAccess(ctx, accountID); chkErr != nil {
		return 0, chkErr
	}
	return op(ctx, accountID, sourceFolderID)
}

// monoAccessibleAccountIDs returns account IDs owned by the JWT user (Mono / Mono Pro).
func (h *Handler) monoAccessibleAccountIDs(ctx context.Context) ([]string, error) {
	if !edition.IsMono() && !edition.IsMonoPro() {
		return nil, nil
	}
	userEmail := middleware.GetUserIDFromContext(ctx)
	if userEmail == "" {
		return nil, errors.New("unauthorized")
	}
	accounts, err := h.Store.GetAccounts(ctx)
	if err != nil {
		return nil, err
	}
	var ids []string
	for _, acc := range accounts {
		if strings.EqualFold(acc.Email, userEmail) {
			ids = append(ids, acc.ID)
		}
	}
	return ids, nil
}

// filterEmailsByMonoAccess drops emails from other tenants in Mono / Mono Pro editions.
func (h *Handler) filterEmailsByMonoAccess(ctx context.Context, emails []models.Email) ([]models.Email, error) {
	if !edition.IsMono() && !edition.IsMonoPro() || len(emails) == 0 {
		return emails, nil
	}
	allowed, err := h.monoAccessibleAccountIDs(ctx)
	if err != nil {
		return nil, err
	}
	if len(allowed) == 0 {
		return []models.Email{}, nil
	}
	allowedSet := make(map[string]struct{}, len(allowed))
	for _, id := range allowed {
		allowedSet[id] = struct{}{}
	}
	out := make([]models.Email, 0, len(emails))
	for _, e := range emails {
		if _, ok := allowedSet[e.AccountID]; ok {
			out = append(out, e)
		}
	}
	return out, nil
}

// resolveBulkSetFlagged picks star vs unstar for ID-based bulk flag.
func (h *Handler) resolveBulkSetFlagged(ctx context.Context, ids []string, reqSet *bool) (bool, error) {
	if reqSet != nil {
		return *reqSet, nil
	}
	emails, err := h.Store.GetEmailsByIDs(ctx, ids)
	if err != nil {
		return true, err
	}
	if len(emails) == 0 {
		return true, nil
	}
	allFlagged := true
	for _, e := range emails {
		if !e.IsFlagged {
			allFlagged = false
			break
		}
	}
	return !allFlagged, nil
}

// resolveFilterBulkSetFlagged picks star vs unstar for select-all bulk flag in a folder scope.
func (h *Handler) resolveFilterBulkSetFlagged(ctx context.Context, accountID, folderID string, reqSet *bool) (bool, error) {
	if reqSet != nil {
		return *reqSet, nil
	}
	aggregate, err := h.usesPerAccountAggregation(ctx, accountID)
	if err != nil {
		return true, err
	}
	var accIDs []string
	if aggregate || strings.HasPrefix(accountID, "group:") {
		accIDs, err = h.perAccountScopeIDs(ctx, accountID)
		if err != nil {
			return true, err
		}
	} else {
		accIDs = []string{accountID}
	}
	perFolder := folderID
	if perFolder == "" {
		perFolder = "INBOX"
	}
	total := 0
	flagged := 0
	for _, accID := range accIDs {
		if chkErr := h.CheckAccountAccess(ctx, accID); chkErr != nil {
			continue
		}
		t, tErr := h.Store.GetEmailCount(ctx, accID, perFolder, models.EmailCountOpts{})
		if tErr != nil {
			return true, tErr
		}
		f, fErr := h.Store.GetEmailCount(ctx, accID, perFolder, models.EmailCountOpts{Flagged: true})
		if fErr != nil {
			return true, fErr
		}
		total += t
		flagged += f
	}
	if total > 0 && flagged == total {
		return false, nil
	}
	return true, nil
}

// folderNameMap loads folder id→name for one account (avoids N+1 GetFolderByID).
func (h *Handler) folderNameMap(ctx context.Context, accountID string) map[string]string {
	names := make(map[string]string)
	folders, err := h.Store.GetFolders(ctx, accountID)
	if err != nil {
		return names
	}
	for _, f := range folders {
		names[f.ID] = f.Name
	}
	return names
}

func sourceFolderName(folderNames map[string]string, folderID string) string {
	if folderID == "" {
		return "INBOX"
	}
	if name, ok := folderNames[folderID]; ok && name != "" {
		return name
	}
	return "INBOX"
}

const bulkIDChunkSize = 500

func (h *Handler) fetchEmailsByIDsInChunks(ctx context.Context, ids []string) ([]models.Email, error) {
	if len(ids) == 0 {
		return nil, nil
	}
	var all []models.Email
	for i := 0; i < len(ids); i += bulkIDChunkSize {
		end := i + bulkIDChunkSize
		if end > len(ids) {
			end = len(ids)
		}
		chunk, err := h.Store.GetEmailsByIDs(ctx, ids[i:end])
		if err != nil {
			return nil, err
		}
		all = append(all, chunk...)
	}
	return all, nil
}

func (h *Handler) enqueueIMAPMovesFromEmails(ctx context.Context, emails []models.Email, accountID, targetFolderID, targetFolderName string) {
	if len(emails) == 0 {
		return
	}
	folderNames := h.folderNameMap(ctx, accountID)
	for _, email := range emails {
		if email.UID > 0 {
			srcFolder := sourceFolderName(folderNames, email.FolderID)
			h.Store.EnqueueIMAPMove(ctx, email.ID, email.AccountID, targetFolderID, targetFolderName, srcFolder, email.UID)
		}
	}
}

// publishEvent is a helper to broadcast SSE events to all connected clients.
func (h *Handler) publishEvent(ctx context.Context, channel, message string) {
	// Invalidate email-list cache before SSE so clients never read stale pages.
	if channel == "email_updated" || channel == "email_deleted" ||
		channel == "new_email" || channel == "new-email" ||
		channel == "emails_bulk_updated" {
		var payload struct {
			AccountID string `json:"account_id"`
		}
		if err := json.Unmarshal([]byte(message), &payload); err == nil && payload.AccountID != "" {
			h.InvalidateEmailCache(ctx, payload.AccountID)
		}
	}

	if false {
		// Fire-and-forget

	}
	if h.EventBus != nil {
		h.EventBus.Publish(channel, message)
	}
}
