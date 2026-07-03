package api

import "net/http"

// EmailAction is a callable action on a specific email (e.g. read, flag, move).
// Registered in emailActions map during init() — read-only at request time.
type EmailAction func(h *Handler, w http.ResponseWriter, r *http.Request, emailID string)

// emailActions maps URL path segments to their handlers.
// Populated in init(), read-only during HTTP serving (safe for concurrent reads).
var emailActions = map[string]EmailAction{}

func init() {
	emailActions["categorize"] = (*Handler).AICategorizeEmail
	emailActions["summarize"] = (*Handler).summarizeEmail
	emailActions["read"] = (*Handler).markEmailRead
	emailActions["flag"] = (*Handler).toggleFlagEmail
	emailActions["pin"] = (*Handler).togglePinEmail
	emailActions["mute"] = (*Handler).toggleMuteEmail
	emailActions["snooze"] = (*Handler).snoozeEmail
	emailActions["raw"] = (*Handler).downloadRawEmail
	emailActions["draft"] = (*Handler).saveDraftReply
	emailActions["clear-draft"] = (*Handler).clearDraftReply
	emailActions["move"] = func(h *Handler, w http.ResponseWriter, r *http.Request, emailID string) {
		h.moveEmail(w, r)
	}
}
