package sync

import (
	"log/slog"
	"os"
	"path/filepath"
	"strings"

	"rmsmail/internal/models"
)

func safeEmailBodyPath(bodyPath string) string {
	if bodyPath == "" {
		return ""
	}
	clean := filepath.Clean(bodyPath)
	if !strings.HasPrefix(clean, "storage/emails/") {
		return ""
	}
	return clean
}

// PurgeEmailLocalFiles removes the on-disk encrypted body for a deleted email.
func PurgeEmailLocalFiles(email *models.Email) {
	if email == nil {
		return
	}
	path := safeEmailBodyPath(email.BodyPath)
	if path == "" {
		return
	}
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		slog.Debug("purge email body failed", "path", path, "error", err)
	}
}
