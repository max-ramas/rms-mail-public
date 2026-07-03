package gc

import (
	"context"
	"log/slog"
	"os"
	"path/filepath"
	"time"

	"rmsmail/internal/api"
)

// RunGarbageCollector scans the storage directory and removes files that are not referenced in the database.
func RunGarbageCollector(ctx context.Context, store api.Store, storageRoot string) error {
	slog.Warn("Starting Storage Garbage Collection...", "storage_root", storageRoot)

	validFiles := make(map[string]struct{})

	// 1. Fetch all legitimate paths from DB
	paths, err := store.GetActiveFilePaths(ctx)
	if err != nil {
		slog.Error("Failed to get active file paths from DB", "error", err)
		return err
	}

	for _, p := range paths {
		if p == "" {
			continue
		}
		// Bring to absolute/clean format for exact comparison
		cleanPath := filepath.Clean(p)
		if !filepath.IsAbs(cleanPath) {
			cleanPath, _ = filepath.Abs(cleanPath)
		}
		validFiles[cleanPath] = struct{}{}
	}

	deletedCount := 0
	freedSpace := int64(0)

	absRoot, err := filepath.Abs(storageRoot)
	if err != nil {
		slog.Error("Failed to get absolute path for storage root", "error", err)
		return err
	}

	// Target specific directories for orphaned file deletion
	targetDirs := []string{
		filepath.Join(absRoot, "emails"),
		filepath.Join(absRoot, "attachments"),
	}

	for _, dir := range targetDirs {
		if _, err := os.Stat(dir); os.IsNotExist(err) {
			continue
		}
		err = filepath.Walk(dir, func(diskPath string, info os.FileInfo, err error) error {
			if err != nil {
				return err
			}
			if info.IsDir() {
				return nil
			}

			cleanDiskPath := filepath.Clean(diskPath)

			// If file is not in validFiles map -> it's an orphan, delete it
			if _, exists := validFiles[cleanDiskPath]; !exists {
				fileSize := info.Size()

				if err := os.Remove(cleanDiskPath); err == nil {
					deletedCount++
					freedSpace += fileSize
					slog.Debug("Deleted orphaned file", "path", cleanDiskPath, "size", fileSize)
				} else {
					slog.Error("Failed to delete orphaned file", "path", cleanDiskPath, "error", err)
				}
			}
			return nil
		})
		if err != nil {
			slog.Error("Failed to walk directory", "dir", dir, "error", err)
		}
	}

	// Clean up camo cache (files older than 30 days OR when total size exceeds 500MB)
	camoDir := filepath.Join(absRoot, "camo")
	if _, err := os.Stat(camoDir); err == nil {
		cutoff := time.Now().Add(-30 * 24 * time.Hour)
		const camoMaxSize int64 = 500 * 1024 * 1024 // 500MB
		var camoTotalSize int64
		var camoFiles []struct {
			path    string
			size    int64
			modTime time.Time
		}
		_ = filepath.Walk(camoDir, func(diskPath string, info os.FileInfo, err error) error {
			if err == nil && !info.IsDir() {
				camoTotalSize += info.Size()
				camoFiles = append(camoFiles, struct {
					path    string
					size    int64
					modTime time.Time
				}{diskPath, info.Size(), info.ModTime()})
			}
			return nil
		})

		// Sort by modTime ascending (oldest first)
		for i := 0; i < len(camoFiles); i++ {
			for j := i + 1; j < len(camoFiles); j++ {
				if camoFiles[j].modTime.Before(camoFiles[i].modTime) {
					camoFiles[i], camoFiles[j] = camoFiles[j], camoFiles[i]
				}
			}
		}

		// Delete files: expired by age OR needed to get under size limit
		for _, cf := range camoFiles {
			shouldDelete := cf.modTime.Before(cutoff) || camoTotalSize > camoMaxSize
			if shouldDelete {
				if err := os.Remove(cf.path); err == nil {
					deletedCount++
					freedSpace += cf.size
					camoTotalSize -= cf.size
				}
			}
		}
	}

	// If there are absolutely zero active files in the DB, we can assume the system is empty
	// We can safely request the user or system to clear the search index, but we cannot safely delete it here
	// because it has active file locks while the server is running.
	if len(validFiles) == 0 {
		slog.Warn("No active files found in DB. You may manually delete storage/emails and storage/attachments if you wish to free up space.")
	}

	slog.Warn("Garbage Collection completed",
		"deleted_files", deletedCount,
		"freed_mb", freedSpace/1024/1024,
	)

	return nil
}
