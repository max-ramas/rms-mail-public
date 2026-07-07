package sync

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/emersion/go-imap/v2"
	"github.com/emersion/go-imap/v2/imapclient"
)

var syncBatchDelay = syncBatchDelayMs()

func syncBatchDelayMs() time.Duration {
	if v := os.Getenv("SYNC_BATCH_DELAY_MS"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			return time.Duration(n) * time.Millisecond
		}
	}
	return 500 * time.Millisecond
}

// isGmailSkippedFolder returns true for Gmail folders that should not be
// synced. Uses IMAP attributes (locale-independent) with name-based fallback.
func isGmailSkippedFolder(path string, attrs []imap.MailboxAttr) bool {
	for _, attr := range attrs {
		switch attr {
		case imap.MailboxAttrAll, imap.MailboxAttrTrash, imap.MailboxAttrNoSelect:
			return true
		}
	}
	// Fallback for servers that don't return attributes
	lower := strings.ToLower(path)
	return strings.Contains(lower, "[gmail]/all mail") ||
		strings.Contains(lower, "[gmail]/trash") ||
		strings.Contains(lower, "[gmail]/вся почта") ||
		strings.Contains(lower, "[gmail]/корзина")
}

type FolderSync struct {
	Name        string
	Path        string
	LastUID     uint32
	UIDValidity uint64
	FolderID    string
	Attrs       []imap.MailboxAttr
}

// ListFolders returns folders with their LastUID loaded from DB when available
func (w *SyncWorker) ListFolders(ctx context.Context, c *imapclient.Client, f *Fetcher) ([]FolderSync, error) {
	folders := []FolderSync{
		{Name: "INBOX", Path: "INBOX"},
	}

	_list, err := c.List("", "*", nil).Collect()
	if err != nil {
		return folders, err
	}

	for _, item := range _list {
		slog.Info(fmt.Sprintf("[%s] LIST: folder=%q delim=%q attrs=%v", w.Account.Email, item.Mailbox, string(item.Delim), item.Attrs))
		if item.Mailbox == "INBOX" {
			continue
		}

		item := item // capture
		folders = append(folders, FolderSync{
			Name:  item.Mailbox,
			Path:  item.Mailbox,
			Attrs: item.Attrs,
		})
	}

	// Sort folders: INBOX first, then alphabetically, Trash/Junk last.
	sort.SliceStable(folders, func(i, j int) bool {
		// INBOX always first
		if strings.EqualFold(folders[i].Name, "INBOX") {
			return true
		}
		if strings.EqualFold(folders[j].Name, "INBOX") {
			return false
		}
		// Trash and Junk always last
		iTrash := strings.EqualFold(folders[i].Name, "Trash") || strings.EqualFold(folders[i].Name, "Junk")
		jTrash := strings.EqualFold(folders[j].Name, "Trash") || strings.EqualFold(folders[j].Name, "Junk")
		if iTrash && !jTrash {
			return false
		}
		if !iTrash && jTrash {
			return true
		}
		// Alphabetical for the rest
		return folders[i].Name < folders[j].Name
	})

	// Load LastUID from database (safe check)
	if f != nil && f.Store != nil {
		dbFolders, err := f.Store.GetFolders(ctx, w.Account.ID)
		if err == nil {
			for i := range folders {
				for _, df := range dbFolders {
					if df.Path == folders[i].Path {
						folders[i].LastUID = uint32(df.LastSyncUID)
						folders[i].UIDValidity = uint64(df.UIDValidity)
						folders[i].FolderID = df.ID
						break
					}
				}
			}
		}
	}

	return folders, nil
}

func (w *SyncWorker) SyncAllFolders(ctx context.Context, c *imapclient.Client, f *Fetcher) error {
	slog.Info(fmt.Sprintf("[%s] SyncAllFolders started", w.Account.Email))
	folders, err := w.ListFolders(ctx, c, f)
	if err != nil {
		slog.Info(fmt.Sprintf("[%s] Failed to list folders: %v", w.Account.Email, err))
		folders = []FolderSync{{Name: "INBOX", Path: "INBOX"}}
	}

	slog.Info(fmt.Sprintf("[%s] Found %d folders to sync", w.Account.Email, len(folders)))

	var firstErr error
	for _, folder := range folders {
		// Gmail: skip redundant virtual folders
		if w.Account.IsGmail && isGmailSkippedFolder(folder.Path, folder.Attrs) {
			continue
		}
		if err := w.syncFolder(ctx, c, f, folder); err != nil {
			slog.Info(fmt.Sprintf("[%s] Failed to sync folder %s: %v", w.Account.Email, folder.Name, err))
			if firstErr == nil {
				firstErr = fmt.Errorf("sync folder %s: %w", folder.Name, err)
			}
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(syncBatchDelay):
		}
	}

	slog.Info(fmt.Sprintf("[%s] SyncAllFolders finished", w.Account.Email))
	if firstErr != nil {
		return firstErr
	}
	return nil
}

// imapOpTimeout is the per-operation deadline for IMAP commands.
// The go-imap v2 library already has internal 30s read/write timeouts,
// but this provides an extra safety layer against hangs on dead connections.
const imapOpTimeout = 60 * time.Second

// waitWithTimeout runs an IMAP operation with a context-based deadline.
// If the operation doesn't complete within the timeout, cancelFn is called to
// force-close the underlying IMAP connection, unblocking the stuck goroutine.
func waitWithTimeout[T any](ctx context.Context, timeout time.Duration, fn func() (T, error), cancelFn func()) (T, error) {
	type result struct {
		val T
		err error
	}
	ch := make(chan result, 1)
	go func() {
		val, err := fn()
		select {
		case ch <- result{val, err}:
		case <-ctx.Done():
		}
	}()

	opCtx, opCancel := context.WithTimeout(ctx, timeout)
	defer opCancel()

	select {
	case <-opCtx.Done():
		// Timeout: force-close the IMAP connection to unblock the goroutine.
		if cancelFn != nil {
			go cancelFn()
		}
		var zero T
		return zero, fmt.Errorf("IMAP operation timed out after %v: %w", timeout, opCtx.Err())
	case r := <-ch:
		return r.val, r.err
	}
}

func (w *SyncWorker) syncFolder(ctx context.Context, c *imapclient.Client, f *Fetcher, folder FolderSync) error {
	// 56.6 Redis Concurrency Lock (Anti-Deadlock)
	if false {

		// Use context.Background() for defer to guarantee cleanup even if sync ctx is canceled

	}

	slog.Info(fmt.Sprintf("[%s] Syncing folder: %s (lastUID=%d)", w.Account.Email, folder.Path, folder.LastUID))

	// Wrap Select with a timeout to prevent hanging on dead connections
	selectData, err := waitWithTimeout(ctx, imapOpTimeout, func() (*imap.SelectData, error) {
		return c.Select(folder.Path, nil).Wait()
	}, func() { c.Close() })
	if err != nil {
		slog.Info(fmt.Sprintf("[%s] Select folder %s failed: %v", w.Account.Email, folder.Path, err))
		return err
	}

	if selectData.NumMessages == 0 {
		slog.Info(fmt.Sprintf("[%s] Folder %s is empty", w.Account.Email, folder.Path))
		return nil
	}

	// Create folder record if it doesn't exist
	if folder.LastUID == 0 {
		if _, err := f.Store.CreateFolder(ctx, w.Account.ID, folder.Name, folder.Path, true); err != nil {
			slog.Info(fmt.Sprintf("Failed to create folder %s: %v", folder.Name, err))
		}
	}

	return w.syncFolderByUID(ctx, c, f, folder, selectData)
}

func folderUIDValidityMismatch(stored, server uint64) bool {
	return server > 0 && stored != 0 && stored != server
}

func (w *SyncWorker) syncFolderByUID(ctx context.Context, c *imapclient.Client, f *Fetcher, folder FolderSync, selectData *imap.SelectData) error {
	folderPath := folder.Path
	lastUID := folder.LastUID
	folderID := folder.FolderID
	storedValidity := folder.UIDValidity
	forceFullSync := lastUID == 0

	serverValidity := uint64(selectData.UIDValidity)
	if serverValidity > 0 {
		if folderID == "" {
			if dbFolders, _ := f.Store.GetFolders(ctx, w.Account.ID); dbFolders != nil {
				for _, df := range dbFolders {
					if df.Path == folderPath {
						folderID = df.ID
						storedValidity = uint64(df.UIDValidity)
						if df.LastSyncUID > 0 && lastUID == 0 {
							lastUID = uint32(df.LastSyncUID)
							forceFullSync = false
						}
						break
					}
				}
			}
		}

		if folderID != "" && folderUIDValidityMismatch(storedValidity, serverValidity) {
			slog.Warn(fmt.Sprintf("[%s] Folder %s: UIDVALIDITY changed %d -> %d, clearing queue and forcing full resync",
				w.Account.Email, folderPath, storedValidity, serverValidity))
			if err := f.Store.ClearFolderQueue(ctx, w.Account.ID, folderPath); err != nil {
				slog.Warn(fmt.Sprintf("[%s] Failed to clear sync queue for %s: %v", w.Account.Email, folderPath, err))
			}
			if err := f.Store.UpdateFolderLastUID(ctx, folderID, 0); err != nil {
				slog.Warn(fmt.Sprintf("[%s] Failed to reset last_sync_uid for %s: %v", w.Account.Email, folderPath, err))
			}
			lastUID = 0
			forceFullSync = true
		}

		if folderID != "" {
			if err := f.Store.UpdateFolderUIDValidity(ctx, folderID, selectData.UIDValidity); err != nil {
				slog.Info(fmt.Sprintf("[%s] Failed to save folder UIDValidity for %s: %v", w.Account.Email, folderPath, err))
			}
		}
	}

	startUID := imap.UID(lastUID + 1)
	start := startUID
	end := imap.UID(selectData.UIDNext - 1)

	// Guard: ensure folderID is always set so lastUID is never lost.
	if folderID == "" {
		if createdFolder, err := f.Store.CreateFolder(ctx, w.Account.ID, folderPath, folderPath, true); err == nil && createdFolder != nil {
			folderID = createdFolder.ID
		} else {
			slog.Warn(fmt.Sprintf("[%s] cannot find or create folder %q — lastUID will not be saved this cycle", w.Account.Email, folderPath))
		}
	}

	slog.Info(fmt.Sprintf("[%s] Folder %s: NumMessages=%d, UIDNext=%d, fetching UIDs %d..%d to enqueue", w.Account.Email, folderPath, selectData.NumMessages, selectData.UIDNext, start, end))

	if start > end {
		return nil
	}

	// ---- Path 1: Full initial sync (forceFullSync) ----
	// Fetch bodies directly in streaming batches, process inline,
	// and checkpoint every 50 emails for crash recovery.
	if forceFullSync {
		slog.Info(fmt.Sprintf("[%s] Folder %s: Full sync — fetching bodies directly (UID %d..%d)",
			w.Account.Email, folderPath, start, end))

		fetchOptions := &imap.FetchOptions{
			UID:   true,
			Flags: true,
			BodySection: []*imap.FetchItemBodySection{
				{Peek: true},
			},
		}

		var set imap.UIDSet
		set.AddRange(start, end)

		cmd := c.Fetch(set, fetchOptions)
		defer cmd.Close()

		const streamBatchSize = 50
		batchCount := 0
		maxProcessedUID := uint32(0)

		for {
			msg := cmd.Next()
			if msg == nil {
				break
			}

			uid, err := f.ProcessMessageStreamToFolder(ctx, w.Account.ID, folderPath, msg, w.Account.IsGmail)
			if err != nil {
				slog.Warn(fmt.Sprintf("[%s] ProcessMessageStreamToFolder error for UID %d: %v",
					w.Account.Email, uid, err))
				// Continue with next message
			}
			if uid > maxProcessedUID {
				maxProcessedUID = uid
			}
			batchCount++

			// Checkpoint every streamBatchSize messages for crash recovery
			if batchCount >= streamBatchSize && folderID != "" && maxProcessedUID > lastUID {
				f.Store.UpdateFolderLastUID(ctx, folderID, int(maxProcessedUID))
				slog.Debug(fmt.Sprintf("[%s] Folder %s: checkpointed UID %d (%d msgs)",
					w.Account.Email, folderPath, maxProcessedUID, batchCount))
				batchCount = 0
			}
		}

		if err := cmd.Close(); err != nil {
			slog.Info(fmt.Sprintf("[%s] Direct fetch close error: %v", w.Account.Email, err))
		}

		// Final checkpoint
		if folderID != "" && maxProcessedUID > lastUID {
			select {
			case <-ctx.Done():
				return ctx.Err()
			default:
			}
			f.Store.UpdateFolderLastUID(ctx, folderID, int(maxProcessedUID))
		}

		slog.Info(fmt.Sprintf("[%s] Folder %s: Full sync complete (up to UID %d processed)",
			w.Account.Email, folderPath, maxProcessedUID))
		return nil
	}

	// ---- Path 2: Incremental sync — UID fetch + enqueue ----
	fetchOptions := &imap.FetchOptions{
		UID:   true,
		Flags: true,
	}

	var set imap.UIDSet
	set.AddRange(start, end)

	msgs, err := waitWithTimeout(ctx, imapOpTimeout, func() ([]*imapclient.FetchMessageBuffer, error) {
		return c.Fetch(set, fetchOptions).Collect()
	}, func() { c.Close() })
	if err != nil {
		return fmt.Errorf("UID FETCH error: %w", err)
	}

	if len(msgs) == 0 {
		if folderID != "" && end > imap.UID(lastUID) {
			f.Store.UpdateFolderLastUID(ctx, folderID, int(end))
		}
		return nil
	}

	var uids []uint32
	maxUID := uint32(0)
	for _, m := range msgs {
		uids = append(uids, uint32(m.UID))
		if uint32(m.UID) > maxUID {
			maxUID = uint32(m.UID)
		}
	}

	// Enqueue in batches to avoid huge queries
	batchSize := 500
	for i := 0; i < len(uids); i += batchSize {
		batchEnd := i + batchSize
		if batchEnd > len(uids) {
			batchEnd = len(uids)
		}
		// Priority 0 for background sync, INBOX can be slightly higher but Checker uses 10.
		priority := 0
		if folderPath == "INBOX" {
			priority = 5
		}
		if err := f.Store.EnqueueUIDs(ctx, w.Account.ID, folderPath, uids[i:batchEnd], priority); err != nil {
			return fmt.Errorf("failed to enqueue UIDs: %w", err)
		}
		if batchEnd < len(uids) {
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(syncBatchDelay):
			}
		}
	}

	if folderID != "" && maxUID > lastUID {
		select {
		case <-ctx.Done():
			return ctx.Err() // don't write if context canceled (worker killed)
		default:
		}
		f.Store.UpdateFolderLastUID(ctx, folderID, int(maxUID))
	}
	// Wake up consumer
	select {
	case w.wakeUp <- struct{}{}:
	default:
	}

	return nil
}
