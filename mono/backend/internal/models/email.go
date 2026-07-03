package models

import (
	"strings"
	"time"
)

type Account struct {
	ID                string     `json:"id"`
	Email             string     `json:"email"`
	Name              string     `json:"name"`
	Provider          string     `json:"provider"`
	IMAPHost          string     `json:"imap_host"`
	IMAPPort          int32      `json:"imap_port"`
	IMAPSSL           bool       `json:"imap_ssl"`
	IMAPEncryption    string     `json:"imap_encryption"`
	SMTPHost          string     `json:"smtp_host"`
	SMTPPort          int32      `json:"smtp_port"`
	SMTPSSL           bool       `json:"smtp_ssl"`
	SMTPEncryption    string     `json:"smtp_encryption"`
	Username          string     `json:"username"`
	PasswordEncrypted string     `json:"-"`
	IsActive          bool       `json:"is_active"`
	LastUID           int32      `json:"last_uid"`
	UIDValidity       int64      `json:"uid_validity"`
	AIProviderConfig  string     `json:"ai_provider_config"`
	SmartCategories   bool       `json:"smart_categories"`
	OAuthAccessToken  string     `json:"-"`
	OAuthRefreshToken string     `json:"-"`
	CreatedAt         time.Time  `json:"created_at"`
	Signature         string     `json:"signature"`
	LastSyncError     string     `json:"last_sync_error"`
	LastSyncAt        time.Time  `json:"last_sync_at"`
	UnreadCount       int        `json:"unread_count"`
	UnreadInbox       int        `json:"unread_inbox"`
	AbsentSince       *time.Time `json:"absent_since,omitempty"`
	SystemDiscovered  bool       `json:"system_discovered"`
	LastSeenAt        time.Time  `json:"last_seen_at"`
	IsLocked          bool       `json:"is_locked"`
	AvatarURL         string     `json:"avatar_url"`
	Color             string     `json:"color"`
	SortOrder         int        `json:"sort_order"`
}

type Email struct {
	ID               string     `json:"id"`
	AccountID        string     `json:"account_id"`
	FolderID         string     `json:"folder_id"`
	MsgID            string     `json:"msg_id"`
	UID              int32      `json:"uid"`
	Subject          string     `json:"subject"`
	SenderName       string     `json:"sender_name"`
	SenderAddress    string     `json:"sender_address"`
	RecipientAddress string     `json:"recipient_address"`
	CcAddress        string     `json:"cc_address"`
	DateSent         time.Time  `json:"date_sent"`
	IsRead           bool       `json:"is_read"`
	IsFlagged        bool       `json:"is_flagged"`
	IsAnswered       bool       `json:"is_answered"`
	HasAttachments   bool       `json:"has_attachments"`
	IsDirtyLocally   bool       `json:"is_dirty_locally"`
	InReplyTo        string     `json:"in_reply_to"`
	ThreadID         string     `json:"thread_id"`
	DraftReply       string     `json:"draft_reply"`
	DraftRemoteUID   int32      `json:"draft_remote_uid"`
	Snippet          string     `json:"snippet"`
	BodyPath         string     `json:"-"`
	AvatarURL        string     `json:"avatar_url"`
	SpfPass          bool       `json:"spf_pass"`
	DkimPass         bool       `json:"dkim_pass"`
	IsPinned         bool       `json:"is_pinned"`
	SnoozeUntil      *time.Time `json:"snooze_until,omitempty"`
	IsMuted          bool       `json:"is_muted"`
	Status           string     `json:"status,omitempty"`
	AssignedTo       string     `json:"assigned_to,omitempty"`
	FirstResponseAt  *time.Time `json:"first_response_at,omitempty"`
	ResolvedAt       *time.Time `json:"resolved_at,omitempty"`
	CreatedAt        time.Time  `json:"created_at"`
}

type AgentStats struct {
	UserID       string `json:"user_id"`
	Name         string `json:"name"`
	Assigned     int    `json:"assigned"`
	Resolved     int    `json:"resolved_today"`
	UnreadByUser int    `json:"unread_by_user"`
}

type Attachment struct {
	ID        string    `json:"id"`
	EmailID   string    `json:"email_id"`
	AccountID string    `json:"account_id,omitempty"`
	Filename  string    `json:"filename"`
	Size      int64     `json:"size"`
	Hash      string    `json:"hash"`
	ContentID string    `json:"content_id,omitempty"`
	Path      string    `json:"-"`
	CreatedAt time.Time `json:"created_at"`
}

type Template struct {
	ID        string    `json:"id"`
	AccountID string    `json:"account_id"`
	Name      string    `json:"name"`
	Subject   string    `json:"subject"`
	Body      string    `json:"body"`
	CreatedAt time.Time `json:"created_at"`
}

type Contact struct {
	ID        string    `json:"id,omitempty"`
	AccountID string    `json:"account_id,omitempty"`
	Address   string    `json:"address"`
	Name      string    `json:"name"`
	Phone     string    `json:"phone,omitempty"`
	Notes     string    `json:"notes,omitempty"`
	Company   string    `json:"company,omitempty"`
	Position  string    `json:"position,omitempty"`
	Tags      string    `json:"tags,omitempty"`
	CreatedAt time.Time `json:"created_at"`
}

type Identity struct {
	ID        string `json:"id"`
	AccountID string `json:"account_id"`
	Email     string `json:"email"`
	Name      string `json:"name"`
}

type Label struct {
	ID        string    `json:"id"`
	AccountID string    `json:"account_id"`
	Name      string    `json:"name"`
	Color     string    `json:"color"`
	CreatedAt time.Time `json:"created_at"`
}

type SyncTask struct {
	ID         int64     `json:"id"`
	AccountID  string    `json:"account_id"`
	FolderName string    `json:"folder_name"`
	UID        uint32    `json:"uid"`
	Priority   int       `json:"priority"`
	Status     string    `json:"status"`
	Attempts   int       `json:"attempts"`
	CreatedAt  time.Time `json:"created_at"`
	UpdatedAt  time.Time `json:"updated_at"`
}

type FilterRule struct {
	ID                string `json:"id"`
	AccountID         string `json:"account_id"`
	Name              string `json:"name"`
	Enabled           bool   `json:"enabled"`
	ConditionField    string `json:"condition_field"`
	ConditionOperator string `json:"condition_operator"`
	ConditionValue    string `json:"condition_value"`
	ActionType        string `json:"action_type"`
	ActionValue       string `json:"action_value"`
	Channel           string `json:"channel,omitempty"`
	Priority          int    `json:"priority"`
	AIProvider        string `json:"ai_provider,omitempty"`
	AIModel           string `json:"ai_model,omitempty"`
	WebhookSecret     string `json:"webhook_secret,omitempty"`
}

type ProjectGroup struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Color     string `json:"color"`
	SortOrder int    `json:"sort_order"`
	IsLocked  bool   `json:"is_locked"`
}

type ProjectGroupWithCount struct {
	ProjectGroup
	UnreadCount   int `json:"unread_count"`
	AccountsCount int `json:"accounts_count"`
}

// Cursor is used for keyset pagination (replaces OFFSET for large mailboxes).
// Format: "true|2026-06-13T18:19:00Z|uuid"
type Cursor struct {
	IsPinned bool
	DateSent time.Time
	ID       string
}

// EmailCountOpts selects which emails to count in a folder scope.
type EmailCountOpts struct {
	Unread         bool
	Flagged        bool
	HasAttachments bool
}

// EmailFilterOpts holds optional filters for email list queries.
// Shared by both PostgreSQL and SQLite storage backends.
type EmailFilterOpts struct {
	Unread      bool
	Flagged     bool
	Attachments bool
	Search      string // searched against subject, sender_name, snippet
	LabelID     string // filters by email_labels junction table
	Tag         string // filters by email_tags junction table
}

// FormatCursor serializes a cursor for the X-Next-Cursor header.
func (c Cursor) Format() string {
	pinned := "false"
	if c.IsPinned {
		pinned = "true"
	}
	return pinned + "|" + c.DateSent.Format(time.RFC3339Nano) + "|" + c.ID
}

// ParseCursor parses a cursor string from the X-Next-Cursor header or query param.
func ParseCursor(raw string) *Cursor {
	if raw == "" {
		return nil
	}
	parts := strings.SplitN(raw, "|", 3)
	if len(parts) != 3 {
		return nil
	}
	isPinned := parts[0] == "true"
	t, err := time.Parse(time.RFC3339Nano, parts[1])
	if err != nil {
		return nil
	}
	return &Cursor{IsPinned: isPinned, DateSent: t, ID: parts[2]}
}

type User struct {
	ID           string    `json:"id"`
	Email        string    `json:"email"`
	Name         string    `json:"name"`
	Role         string    `json:"role"`
	PasswordHash string    `json:"-"`
	AvatarURL    string    `json:"avatar_url"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

type Folder struct {
	ID           string    `json:"id"`
	AccountID    string    `json:"account_id"`
	Name         string    `json:"name"`
	Path         string    `json:"path"`
	IsSubscribed bool      `json:"is_subscribed"`
	LastSyncUID  int       `json:"last_sync_uid"`
	UIDValidity  int64     `json:"uid_validity"`
	UnreadCount  int       `json:"unread_count"`
	CreatedAt    time.Time `json:"created_at"`
}

type EmailComment struct {
	ID        string    `json:"id"`
	EmailID   string    `json:"email_id"`
	AccountID string    `json:"account_id"`
	AuthorID  string    `json:"author_id"`
	Body      string    `json:"body"`
	Internal  bool      `json:"internal"`
	CreatedAt time.Time `json:"created_at"`
}

type AILogEntry struct {
	ID               string    `json:"id"`
	Action           string    `json:"action"`
	Provider         string    `json:"provider"`
	Model            string    `json:"model"`
	PromptTokens     int       `json:"prompt_tokens"`
	CompletionTokens int       `json:"completion_tokens"`
	TotalTokens      int       `json:"total_tokens"`
	DurationMs       int       `json:"duration_ms"`
	Status           string    `json:"status"`
	CreatedAt        time.Time `json:"created_at"`
}

type AILogStats struct {
	TotalActions int            `json:"total_actions"`
	TotalTokens  int            `json:"total_tokens"`
	TotalCostUSD float64        `json:"total_cost_usd"`
	ByAction     map[string]int `json:"by_action"`
	ByProvider   map[string]int `json:"by_provider"`
}

type MCPKey struct {
	ID           string     `json:"id"`
	Name         string     `json:"name"`
	AccountID    string     `json:"account_id"`
	KeyHash      string     `json:"-"`
	KeyPrefix    string     `json:"key_prefix"`
	KeyEncrypted string     `json:"-"`
	FullKey      string     `json:"full_key,omitempty"`
	CreatedBy    string     `json:"created_by"`
	IsActive     bool       `json:"is_active"`
	LastUsedAt   *time.Time `json:"last_used_at"`
	CreatedAt    time.Time  `json:"created_at"`
}

type AISetting struct {
	ID               string    `json:"id"`
	AccountID        string    `json:"account_id"`
	Preset           string    `json:"preset"`
	Config           string    `json:"config"`  // JSON string per task provider/model
	Prompts          string    `json:"prompts"` // JSON string per task prompt
	APIKeysEncrypted string    `json:"api_keys_encrypted"`
	CreatedAt        time.Time `json:"created_at"`
	UpdatedAt        time.Time `json:"updated_at"`
}

// IMAPMove represents a pending IMAP UID MOVE operation for sync with email providers.
type IMAPMove struct {
	ID               string    `json:"id"`
	EmailID          string    `json:"email_id"`
	AccountID        string    `json:"account_id"`
	TargetFolderID   string    `json:"target_folder_id"`
	TargetFolderName string    `json:"target_folder_name"`
	SourceFolderName string    `json:"source_folder_name"`
	RemoteUID        int32     `json:"remote_uid"`
	RetryCount       int       `json:"retry_count"`
	LastError        string    `json:"last_error,omitempty"`
	CreatedAt        time.Time `json:"created_at"`
}

type IMAPMoveMeta struct {
	EmailID   string `json:"email_id"`
	AccountID string `json:"account_id"`
	UID       int32  `json:"uid"`
	FolderID  string `json:"folder_id"`
}

type Webhook struct {
	ID        string    `json:"id"`
	AccountID string    `json:"account_id"`
	Name      string    `json:"name"`
	URL       string    `json:"url"`
	Secret    string    `json:"-"`
	HasSecret bool      `json:"has_secret"`
	CreatedAt time.Time `json:"created_at"`
}

// WebhookEventPayload is the JSON body POSTed to outbound webhook URLs.
type WebhookEventPayload struct {
	Event string `json:"event"`
	Email Email  `json:"email"`
}

type WebhookRetry struct {
	ID          string `json:"id"`
	URL         string `json:"url"`
	Secret      string `json:"secret"`
	Payload     []byte `json:"payload"`
	Attempt     int    `json:"attempt"`
	NextRetryAt int64  `json:"next_retry_at"` // Unix timestamp
}
