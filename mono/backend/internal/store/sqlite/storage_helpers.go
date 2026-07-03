package sqlite

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"strings"
	"time"
)

const (
	writePriorityHigh = 0 // user-facing API (pin, labels, read)
	writePriorityLow  = 1 // background sync
)

type writeJob struct {
	fn   func() error
	done chan error
}

func isSQLiteContention(err error) bool {
	if err == nil {
		return false
	}
	msg := err.Error()
	return strings.Contains(msg, "database is locked") ||
		strings.Contains(msg, "SQLITE_BUSY") ||
		strings.Contains(msg, "cannot start a transaction within a transaction")
}

// retryBusy retries fn on SQLITE write contention (up to ~20s total wait).
func retryBusy(fn func() error) error {
	var err error
	for attempt := 0; attempt < 15; attempt++ {
		err = fn()
		if err == nil {
			return nil
		}
		if !isSQLiteContention(err) {
			return err
		}
		delay := time.Duration(50*(1<<attempt)) * time.Millisecond
		if delay > 1500*time.Millisecond {
			delay = 1500 * time.Millisecond
		}
		time.Sleep(delay)
	}
	return err
}

func (s *Storage) startWriteWorker() {
	go func() {
		for {
			var job writeJob
			select {
			case job = <-s.highWrites:
			default:
				select {
				case job = <-s.highWrites:
				case job = <-s.lowWrites:
				}
			}
			var err error
			func() {
				defer func() {
					if r := recover(); r != nil {
						slog.Error("SQLite write worker panic", "panic", r)
						err = fmt.Errorf("write worker panic: %v", r)
					}
				}()
				err = retryBusy(job.fn)
			}()
			job.done <- err
		}
	}()
}

func (s *Storage) enqueueWrite(ctx context.Context, priority int, fn func(ctx context.Context) error) error {
	// User-facing writes must finish even if the HTTP client disconnects mid-request.
	wctx := context.WithoutCancel(ctx)

	done := make(chan error, 1)
	job := writeJob{
		fn:   func() error { return fn(wctx) },
		done: done,
	}

	target := s.lowWrites
	if priority == writePriorityHigh {
		target = s.highWrites
	}

	target <- job
	return <-done
}

// withWriteRetry — high-priority user writes (pin, labels, read).
func (s *Storage) withWriteRetry(ctx context.Context, fn func(ctx context.Context) error) error {
	return s.enqueueWrite(ctx, writePriorityHigh, fn)
}

// withWriteRetryLow — background sync writes; yields to user actions.
func (s *Storage) withWriteRetryLow(ctx context.Context, fn func(ctx context.Context) error) error {
	return s.enqueueWrite(ctx, writePriorityLow, fn)
}

func boolToInt(b bool) int {
	if b {
		return 1
	}
	return 0
}

func formatTime(t time.Time) string {
	return t.UTC().Format("2006-01-02T15:04:05.000Z")
}

func parseTime(s sql.NullString) time.Time {
	if !s.Valid || s.String == "" || s.String == "0001-01-01T00:00:00Z" {
		return time.Time{}
	}
	formats := []string{
		"2006-01-02T15:04:05.000Z",
		"2006-01-02T15:04:05Z",
		"2006-01-02T15:04:05.000000Z",
		"2006-01-02 15:04:05",
		"2006-01-02T15:04:05-07:00",
		time.RFC3339,
		time.RFC3339Nano,
	}
	for _, f := range formats {
		if t, err := time.Parse(f, s.String); err == nil {
			return t
		}
	}
	return time.Time{}
}
