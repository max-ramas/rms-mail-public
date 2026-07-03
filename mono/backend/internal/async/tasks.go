package async

// Task type constants for Asynq task queue.
const (
	TypeSendEmail       = "mail:send_email"
	TypeDispatchWebhook = "mail:dispatch_webhook"
	TypeGenerateAIDraft = "mail:generate_ai_draft"
	TypeSendTelegram    = "mail:send_telegram"
	TypeResolveAvatar   = "mail:resolve_avatar"
)

// PayloadSendEmail carries the data needed to send a scheduled email.
type PayloadSendEmail struct {
	JobID     string `json:"job_id"`
	AccountID string `json:"account_id"`
	EmailID   string `json:"email_id"`
}

// PayloadDispatchWebhook carries the data needed to dispatch a webhook.
type PayloadDispatchWebhook struct {
	RetryID string `json:"retry_id"`
	URL     string `json:"url"`
	Secret  string `json:"secret"`
	Payload []byte `json:"payload"`
}

// PayloadGenerateAIDraft carries the data needed to generate an AI draft reply.
type PayloadGenerateAIDraft struct {
	AccountID string `json:"account_id"`
	EmailID   string `json:"email_id"`
	Prompt    string `json:"prompt"`
}

// PayloadSendTelegram carries the data needed to send a Telegram notification.
type PayloadSendTelegram struct {
	AccountID    string `json:"account_id"`
	EmailID      string `json:"email_id"`
	AccountEmail string `json:"account_email"`
	Subject      string `json:"subject"`
	SenderName   string `json:"sender_name"`
	Snippet      string `json:"snippet"`
}

// PayloadResolveAvatar carries the data needed to resolve a sender avatar.
type PayloadResolveAvatar struct {
	Address string `json:"address"`
	Name    string `json:"name"`
}
