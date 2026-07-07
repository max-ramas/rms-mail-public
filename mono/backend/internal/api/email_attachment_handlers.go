package api

import (
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"mime"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"rmsmail/internal/models"

	"github.com/google/uuid"
)

func (h *Handler) UploadAttachment(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusMethodNotAllowed)
		json.NewEncoder(w).Encode(map[string]string{"error": "method not allowed"})
		return
	}

	if h.CAS == nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusServiceUnavailable)
		json.NewEncoder(w).Encode(map[string]string{"error": "attachment storage not available"})
		AppMetrics.HTTPErrors.Add(1)
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, 25<<20) // 25MB max
	if err := r.ParseMultipartForm(25 << 20); err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": "file too large or invalid form"})
		return
	}

	files := r.MultipartForm.File["files"]
	if len(files) == 0 {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": "no files uploaded"})
		return
	}

	var results []map[string]interface{}
	results = make([]map[string]interface{}, 0)
	for _, fh := range files {
		f, err := fh.Open()
		if err != nil {
			slog.Info("attachment upload: failed to open file", "filename", fh.Filename, "error", err)
			continue
		}

		data, err := io.ReadAll(f)
		f.Close()
		if err != nil {
			slog.Info("attachment upload: failed to read file", "filename", fh.Filename, "error", err)
			continue
		}

		hash := h.CAS.Hash(data)
		path := h.CAS.StorePath(hash)

		if _, err := os.Stat(path); os.IsNotExist(err) {
			if err := os.MkdirAll(filepath.Dir(path), 0750); err != nil {
				slog.Info("attachment upload: failed to create storage dir", "path", filepath.Dir(path), "error", err)
				continue
			}
			if err := os.WriteFile(path, data, 0640); err != nil {
				slog.Info("attachment upload: failed to write file", "path", path, "error", err)
				continue
			}
		}

		att := &models.Attachment{
			ID:        uuid.New().String(),
			EmailID:   "00000000-0000-0000-0000-000000000000",
			AccountID: "00000000-0000-0000-0000-000000000000",
			Filename:  fh.Filename,
			Size:      int64(len(data)),
			Hash:      hash,
			Path:      path,
		}
		if err := h.Store.SaveAttachment(r.Context(), att); err != nil {
			slog.Info("attachment upload: failed to save to DB", "filename", fh.Filename, "error", err)
			continue
		}

		results = append(results, map[string]interface{}{
			"id":       att.ID,
			"filename": att.Filename,
			"size":     att.Size,
			"hash":     att.Hash,
		})
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(results)
}

func (h *Handler) GetAttachment(w http.ResponseWriter, r *http.Request) {
	pathParts := strings.Split(strings.Trim(r.URL.Path, "/"), "/")
	if len(pathParts) < 3 {
		WriteJSONError(w, http.StatusBadRequest, "invalid path")
		return
	}
	hash := pathParts[len(pathParts)-1]

	att, err := h.Store.GetAttachmentByHash(r.Context(), hash)
	if err != nil {
		WriteJSONError(w, http.StatusNotFound, "attachment not found")
		return
	}

	if att.AccountID != "" {
		if err := h.CheckAccountAccess(r.Context(), att.AccountID); err != nil {
			WriteJSONError(w, http.StatusForbidden, "access denied")
			return
		}
	} else if att.EmailID != "" {
		email, _ := h.Store.GetEmail(r.Context(), att.EmailID, "")
		if email != nil {
			if err := h.CheckAccountAccess(r.Context(), email.AccountID); err != nil {
				WriteJSONError(w, http.StatusForbidden, "access denied")
				return
			}
		}
	}

	isInline := r.URL.Query().Get("inline") == "true"
	disposition := "attachment"
	if isInline {
		disposition = "inline"
	}

	contentType := mime.TypeByExtension(filepath.Ext(att.Filename))
	if contentType == "" {
		ext := strings.ToLower(filepath.Ext(att.Filename))
		if isInline && (ext == ".png" || ext == ".jpg" || ext == ".jpeg" || ext == ".gif") {
			// Ручной fallback на случай отсутствия системной БД MIME-типов
			if ext == ".png" {
				contentType = "image/png"
			} else if ext == ".gif" {
				contentType = "image/gif"
			} else {
				contentType = "image/jpeg"
			}
		} else {
			contentType = "application/octet-stream"
		}
	}

	if att.Path != "" {
		if _, err := os.Stat(att.Path); err == nil {
			cleanPath := filepath.Clean(att.Path)
			if !strings.HasPrefix(cleanPath, "storage/") {
				WriteJSONError(w, http.StatusForbidden, "access denied")
				return
			}
			w.Header().Set("Content-Disposition", mime.FormatMediaType(disposition, map[string]string{"filename": att.Filename}))
			w.Header().Set("Content-Type", contentType)
			http.ServeFile(w, r, cleanPath)
			return
		}
	}

	w.Header().Set("X-Accel-Redirect", "/internal/attachment/"+hash[:2]+"/"+hash)
	w.Header().Set("Content-Disposition", mime.FormatMediaType(disposition, map[string]string{"filename": att.Filename}))
	w.Header().Set("Content-Type", contentType)
	w.Header().Set("Content-Length", fmt.Sprintf("%d", att.Size))
	w.WriteHeader(http.StatusOK)
}
