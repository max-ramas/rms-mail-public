package sync

import (
	"log/slog"
	"strings"
)

type slogWriter struct{}

func (w *slogWriter) Write(p []byte) (n int, err error) {
	lines := strings.Split(string(p), "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line != "" {
			slog.Debug("IMAP_DEBUG_TRACE: " + line)
		}
	}
	return len(p), nil
}
