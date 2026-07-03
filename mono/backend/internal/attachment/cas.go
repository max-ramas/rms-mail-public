package attachment

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"

	"rmsmail/internal/models"

	"github.com/google/uuid"
)

// AttachmentStore is the minimal store interface needed by CASStorage.
type AttachmentStore interface {
	GetAttachmentByHash(ctx context.Context, hash string) (*models.Attachment, error)
	SaveAttachment(ctx context.Context, att *models.Attachment) error
	GetAllAttachments(ctx context.Context) ([]models.Attachment, error)
}

type CASStorage struct {
	storagePath string
	store       AttachmentStore
}

func NewCASStorage(storagePath string, store AttachmentStore) *CASStorage {
	if err := os.MkdirAll(storagePath, 0750); err != nil {
		panic(fmt.Sprintf("failed to create CAS storage dir %s: %v", storagePath, err))
	}
	return &CASStorage{
		storagePath: storagePath,
		store:       store,
	}
}

func (c *CASStorage) Hash(data []byte) string {
	hash := sha256.Sum256(data)
	return hex.EncodeToString(hash[:])
}

func (c *CASStorage) StorePath(hash string) string {
	return filepath.Join(c.storagePath, hash[:2], hash)
}

func (c *CASStorage) Save(ctx context.Context, emailID, accountID, filename string, data []byte, contentID string) (*models.Attachment, error) {
	hash := c.Hash(data)
	path := c.StorePath(hash)

	slog.Info(fmt.Sprintf("[CAS] saving attachment %q (hash=%s, size=%d, email=%s, account=%s)", filename, hash[:12], len(data), emailID, accountID))

	if _, err := os.Stat(path); os.IsNotExist(err) {
		if err := os.MkdirAll(filepath.Dir(path), 0750); err != nil {
			return nil, fmt.Errorf("mkdir %s: %w", filepath.Dir(path), err)
		}
		if err := os.WriteFile(path, data, 0640); err != nil {
			return nil, fmt.Errorf("write file %s: %w", path, err)
		}
		slog.Info(fmt.Sprintf("[CAS] wrote file to disk: %s", path))
	}

	existing, err := c.store.GetAttachmentByHash(ctx, hash)
	if err != nil {
		slog.Info(fmt.Sprintf("[CAS] GetAttachmentByHash error: %v", err))
	}
	if existing != nil {
		slog.Info(fmt.Sprintf("[CAS] dedup: attachment %s already exists (id=%s)", hash[:12], existing.ID))
		return existing, nil
	}

	attachment := &models.Attachment{
		ID:        uuid.New().String(),
		EmailID:   emailID,
		AccountID: accountID,
		Filename:  filename,
		Size:      int64(len(data)),
		Hash:      hash,
		ContentID: contentID,
		Path:      path,
	}

	if err := c.store.SaveAttachment(ctx, attachment); err != nil {
		slog.Info(fmt.Sprintf("[CAS] SaveAttachment DB error for %q (hash=%s): %v", filename, hash[:12], err))
		return nil, fmt.Errorf("save attachment to db: %w", err)
	}

	slog.Info(fmt.Sprintf("[CAS] saved attachment %q (id=%s, hash=%s) to DB", filename, attachment.ID, hash[:12]))
	return attachment, nil
}

func (c *CASStorage) Get(hash string) ([]byte, error) {
	path := c.StorePath(hash)
	return os.ReadFile(path)
}

func (c *CASStorage) Exists(hash string) bool {
	_, err := os.Stat(c.StorePath(hash))
	return err == nil
}

func (c *CASStorage) DeleteUnused(ctx context.Context) (int, error) {
	attachments, err := c.store.GetAllAttachments(ctx)
	if err != nil {
		return 0, err
	}

	usedHashes := make(map[string]bool)
	for _, a := range attachments {
		usedHashes[a.Hash] = true
	}

	entries, err := os.ReadDir(c.storagePath)
	if err != nil {
		return 0, err
	}

	deleted := 0
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		prefix := entry.Name()
		files, err := os.ReadDir(filepath.Join(c.storagePath, prefix))
		if err != nil {
			continue
		}
		for _, f := range files {
			if f.IsDir() {
				continue
			}
			hash := prefix + f.Name()
			if !usedHashes[hash] {
				if err := os.Remove(filepath.Join(c.storagePath, prefix, f.Name())); err != nil {
					slog.Info(fmt.Sprintf("Failed to remove unused attachment %s: %v", hash, err))
				}
				deleted++
			}
		}
	}

	return deleted, nil
}

func (c *CASStorage) GetOrCopy(ctx context.Context, hash, destPath string) error {
	if _, err := os.Stat(destPath); err == nil {
		return nil
	}

	data, err := c.Get(hash)
	if err != nil {
		return err
	}

	dir := filepath.Dir(destPath)
	if err := os.MkdirAll(dir, 0750); err != nil {
		return err
	}

	return os.WriteFile(destPath, data, 0640)
}

func (c *CASStorage) TotalSize() (int64, error) {
	var total int64
	entries, err := os.ReadDir(c.storagePath)
	if err != nil {
		return 0, err
	}
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		files, err := os.ReadDir(filepath.Join(c.storagePath, entry.Name()))
		if err != nil {
			continue
		}
		for _, f := range files {
			info, err := f.Info()
			if err != nil {
				continue
			}
			total += info.Size()
		}
	}
	return total, nil
}

func (c *CASStorage) GetStats(ctx context.Context) (map[string]interface{}, error) {
	attachments, err := c.store.GetAllAttachments(ctx)
	if err != nil {
		return nil, err
	}

	hashes := make(map[string]int64)
	var totalSize int64
	for _, a := range attachments {
		hashes[a.Hash] = a.Size
		totalSize += a.Size
	}

	deduped := int64(len(hashes))
	raw := totalSize
	var uniqueSize int64
	for _, sz := range hashes {
		uniqueSize += sz
	}
	var saved int64
	if raw > 0 {
		saved = (raw - uniqueSize) * 100 / raw
	}

	return map[string]interface{}{
		"total_attachments": len(attachments),
		"unique_files":      deduped,
		"total_size":        totalSize,
		"saved_percent":     saved,
	}, nil
}

func CopyStream(src io.Reader, dest io.Writer) (int64, error) {
	buf := make([]byte, 32*1024)
	var total int64
	for {
		n, err := src.Read(buf)
		if n > 0 {
			if _, writeErr := dest.Write(buf[:n]); writeErr != nil {
				return total, writeErr
			}
			total += int64(n)
		}
		if err == io.EOF {
			return total, nil
		}
		if err != nil {
			return total, err
		}
	}
}

func (c *CASStorage) SaveStream(ctx context.Context, emailID, accountID, filename string, src io.Reader) (*models.Attachment, error) {
	tmpPath := filepath.Join(c.storagePath, ".tmp", fmt.Sprintf("%s.tmp", uuid.New().String()))
	if err := os.MkdirAll(filepath.Dir(tmpPath), 0750); err != nil {
		return nil, fmt.Errorf("mkdir %s: %w", filepath.Dir(tmpPath), err)
	}

	f, err := os.Create(tmpPath)
	if err != nil {
		return nil, err
	}
	defer os.Remove(tmpPath)

	hasher := sha256.New()
	writer := io.MultiWriter(f, hasher)

	if _, err := CopyStream(src, writer); err != nil {
		f.Close()
		return nil, err
	}
	f.Close()

	data, err := os.ReadFile(tmpPath)
	if err != nil {
		return nil, err
	}

	return c.Save(ctx, emailID, accountID, filename, data, "")
}
