package api

import (
	"expvar"
	"fmt"
	"net/http"
	"sync/atomic"
)

// Metrics holds application-level counters for Prometheus scraping.
type Metrics struct {
	// Email operations
	EmailsSynced   atomic.Int64
	EmailsSent     atomic.Int64
	EmailsDeleted  atomic.Int64
	EmailsArchived atomic.Int64

	// IMAP operations
	IMAPConnections atomic.Int64
	IMAPErrors      atomic.Int64

	// AI operations
	AIChatRequests      atomic.Int64
	AISummarizeRequests atomic.Int64
	AICategorizeReqs    atomic.Int64
	AIDraftGenerations  atomic.Int64
	AIErrors            atomic.Int64

	// HTTP requests
	HTTPRequests   atomic.Int64
	HTTPErrors     atomic.Int64
	ActiveSSEConns atomic.Int64

	// Auth
	LoginAttempts atomic.Int64
	LoginFailures atomic.Int64
}

// AppMetrics is the global metrics instance.
var AppMetrics Metrics

func init() {
	expvar.Publish("rms_mail_metrics", expvar.Func(func() interface{} {
		return map[string]int64{
			"emails_synced_total":        AppMetrics.EmailsSynced.Load(),
			"emails_sent_total":          AppMetrics.EmailsSent.Load(),
			"emails_deleted_total":       AppMetrics.EmailsDeleted.Load(),
			"emails_archived_total":      AppMetrics.EmailsArchived.Load(),
			"imap_connections_total":     AppMetrics.IMAPConnections.Load(),
			"imap_errors_total":          AppMetrics.IMAPErrors.Load(),
			"ai_chat_requests_total":     AppMetrics.AIChatRequests.Load(),
			"ai_summarize_total":         AppMetrics.AISummarizeRequests.Load(),
			"ai_categorize_total":        AppMetrics.AICategorizeReqs.Load(),
			"ai_draft_generations_total": AppMetrics.AIDraftGenerations.Load(),
			"ai_errors_total":            AppMetrics.AIErrors.Load(),
			"http_requests_total":        AppMetrics.HTTPRequests.Load(),
			"http_errors_total":          AppMetrics.HTTPErrors.Load(),
			"active_sse_connections":     AppMetrics.ActiveSSEConns.Load(),
			"login_attempts_total":       AppMetrics.LoginAttempts.Load(),
			"login_failures_total":       AppMetrics.LoginFailures.Load(),
		}
	}))
}

// MetricsHandler writes Prometheus-format metrics.
func MetricsHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.WriteHeader(http.StatusOK)

	data := map[string]int64{
		"rms_mail_emails_synced_total":        AppMetrics.EmailsSynced.Load(),
		"rms_mail_emails_sent_total":          AppMetrics.EmailsSent.Load(),
		"rms_mail_emails_deleted_total":       AppMetrics.EmailsDeleted.Load(),
		"rms_mail_emails_archived_total":      AppMetrics.EmailsArchived.Load(),
		"rms_mail_imap_connections_total":     AppMetrics.IMAPConnections.Load(),
		"rms_mail_imap_errors_total":          AppMetrics.IMAPErrors.Load(),
		"rms_mail_ai_chat_requests_total":     AppMetrics.AIChatRequests.Load(),
		"rms_mail_ai_summarize_total":         AppMetrics.AISummarizeRequests.Load(),
		"rms_mail_ai_categorize_total":        AppMetrics.AICategorizeReqs.Load(),
		"rms_mail_ai_draft_generations_total": AppMetrics.AIDraftGenerations.Load(),
		"rms_mail_ai_errors_total":            AppMetrics.AIErrors.Load(),
		"rms_mail_http_requests_total":        AppMetrics.HTTPRequests.Load(),
		"rms_mail_http_errors_total":          AppMetrics.HTTPErrors.Load(),
		"rms_mail_active_sse_connections":     AppMetrics.ActiveSSEConns.Load(),
		"rms_mail_login_attempts_total":       AppMetrics.LoginAttempts.Load(),
		"rms_mail_login_failures_total":       AppMetrics.LoginFailures.Load(),
	}

	for name, val := range data {
		fmt.Fprintf(w, "# HELP %s Application metric\n", name)
		fmt.Fprintf(w, "# TYPE %s gauge\n", name)
		fmt.Fprintf(w, "%s %d\n", name, val)
	}
}
