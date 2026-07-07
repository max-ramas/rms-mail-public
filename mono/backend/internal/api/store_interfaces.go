package api

import (
	"context"
	"time"

	"rmsmail/internal/models"
)

// ── Segregated store interfaces (ISP) ──────────────────────────
// Each handler method depends on 1–3 of these instead of the 99-method Store.
// The composite `Store` interface is kept for backward compatibility in main.go.

// EmailReader — read-only email operations (hottest path).
type EmailReader interface {
	GetEmails(ctx context.Context, unified bool, accountID, folderID, folderName string, offset, limit int, filter models.EmailFilterOpts) ([]models.Email, error)
	GetEmailsCursor(ctx context.Context, unified bool, accountID, folderID, folderName string, limit int, filter models.EmailFilterOpts, cursor *models.Cursor, scopedAccountIDs []string) ([]models.Email, *models.Cursor, error)
	GetEmailsByAccounts(ctx context.Context, accountIDs []string, folderName string, offset, limit int, filter models.EmailFilterOpts) ([]models.Email, error)
	GetEmail(ctx context.Context, emailID string, accountID string) (*models.Email, error)
	GetEmailsByIDs(ctx context.Context, ids []string) ([]models.Email, error)
	GetEmailsByThreadID(ctx context.Context, threadID, accountID string, limit int) ([]models.Email, error)
	SearchEmails(ctx context.Context, q string, accountID string, limit int) ([]models.Email, error)
	GetEmailIDs(ctx context.Context, accountID, folderID string) ([]string, error)
	GetEmailIDsByFilter(ctx context.Context, accountID, folderID string) ([]string, error)
	GetEmailCount(ctx context.Context, accountID, folderID string, opts models.EmailCountOpts) (int, error)
	GetEmailBodyPath(ctx context.Context, emailID string, accountID string) (string, error)
	GetUnreadCountByAccount(ctx context.Context) (map[string]int, error)
	GetUnreadInboxCountByAccount(ctx context.Context) (map[string]int, error)
	GetUnreadCountByFolder(ctx context.Context, accountID string) (map[string]int, error)
	GetUnassignedCount(ctx context.Context) (int, error)
	AttachAvatars(ctx context.Context, emails []models.Email) error
	GetEmailTags(ctx context.Context, emailID string, accountID string) ([]string, error)
	GetBatchEmailTags(ctx context.Context, emailIDs []string) (map[string][]string, error)
	GetBatchEmailLabels(ctx context.Context, emailIDs []string) (map[string][]models.Label, error)
	GetSnoozedEmails(ctx context.Context) ([]models.Email, error)
	GetStatsByAgent(ctx context.Context) ([]models.AgentStats, error)
	GetSLABreaches(ctx context.Context, slaHours int) (int, error)
	EmailExistsByMsgID(ctx context.Context, accountID, msgID string) (bool, error)
	GetDirtyDrafts(ctx context.Context, accountID string) ([]models.Email, error)
	GetDirtyEmails(ctx context.Context, accountID string) ([]models.Email, error)
	GetEmailsForInboundFlagSync(ctx context.Context, accountID string, limit int) ([]models.Email, error)
	GetDraftsFolder(ctx context.Context, accountID string) (*models.Folder, error)
	GetActiveFilePaths(ctx context.Context) ([]string, error)
	RefreshUnreadCounts(ctx context.Context) error
	GetEmailByMsgIDAccount(ctx context.Context, msgID, accountID string) (*models.Email, error)
	GetEmailsByLabel(ctx context.Context, accountID, label string, offset, limit int) ([]models.Email, error)
	GetGmailLabels(ctx context.Context, emailID, accountID string) ([]string, error)
}

// EmailWriter — email mutations.
type EmailWriter interface {
	MarkEmailRead(ctx context.Context, emailID string, accountID string) error
	BulkMarkEmailsRead(ctx context.Context, ids []string) error
	BulkMarkEmailsUnread(ctx context.Context, ids []string) error
	TogglePinEmail(ctx context.Context, emailID string, accountID string) (bool, error)
	ToggleMuteEmail(ctx context.Context, emailID string, accountID string) (bool, error)
	SnoozeEmail(ctx context.Context, emailID string, accountID string, minutes int) error
	UnsnoozeEmail(ctx context.Context, emailID string) error
	MoveEmail(ctx context.Context, emailID, accountID, folderID string) error
	ToggleFlagEmail(ctx context.Context, emailID string, accountID string) (bool, error)
	BulkSetFlagEmails(ctx context.Context, ids []string, flagged bool) error
	BulkReadByFilter(ctx context.Context, accountID, folderID string) (int64, error)
	BulkUnreadByFilter(ctx context.Context, accountID, folderID string) (int64, error)
	BulkFlagByFilter(ctx context.Context, accountID, folderID string) (int64, error)
	BulkSetFlagByFilter(ctx context.Context, accountID, folderID string, flagged bool) (int64, error)
	BulkMoveByFilter(ctx context.Context, accountID, sourceFolderID, targetFolderID string) (int64, error)
	BulkMoveAndEnqueue(ctx context.Context, ids []string, accountID, targetFolderID, targetFolderName string, emails []models.Email) error
	BulkDeleteEmails(ctx context.Context, ids []string) error
	DeleteEmail(ctx context.Context, id string, accountID string) error
	DeleteEmailsInFolder(ctx context.Context, folderID string) error
	MarkEmailAnsweredByMsgID(ctx context.Context, accountID, msgID string) error
	AssignEmail(ctx context.Context, emailID, userID string) error
	UnassignEmail(ctx context.Context, emailID string) error
	UpdateEmailStatus(ctx context.Context, emailID, status string) error
	UpdateEmailFirstResponseAt(ctx context.Context, emailID string, t time.Time) error
	UpdateEmailResolvedAt(ctx context.Context, emailID string, t time.Time) error
	UpdateEmailHasAttachments(ctx context.Context, emailID string, accountID string, has bool) error
	ApplyServerEmailFlags(ctx context.Context, emailID, accountID string, isRead, isFlagged, isAnswered bool) (bool, error)
	ClearDirtyFlag(ctx context.Context, emailID string) error
	SetDraftRemoteUID(ctx context.Context, emailID string, accountID string, uid int) error
	SaveDraftReply(ctx context.Context, emailID string, accountID string, draftReply string) error
	ClearDraftReply(ctx context.Context, emailID string, accountID string) error
	SaveStandaloneDraft(ctx context.Context, accountID, emailID, folderID, to, cc, subject, draftPayload string, isDirty bool) error
	DeleteDraft(ctx context.Context, emailID string) error
	SaveEmail(ctx context.Context, email models.Email) error
	SaveEmailToFolder(ctx context.Context, email models.Email, folderID string) (bool, error)
	MoveEmailAndEnqueueIMAP(ctx context.Context, emailID, accountID, targetFolderID, targetFolderName, sourceFolderName string, remoteUID int32) error
	EnqueueIMAPMove(ctx context.Context, emailID, accountID, targetFolderID, targetFolderName, sourceFolderName string, remoteUID int32) error
	DeleteEmailLabels(ctx context.Context, emailID, accountID string) error
}

// AccountStore — account lifecycle and credentials.
type AccountStore interface {
	GetAccounts(ctx context.Context) ([]models.Account, error)
	GetFirstRegisteredAccountEmail(ctx context.Context) (string, error)
	GetAccount(ctx context.Context, id string) (*models.Account, error)
	GetAccountCredentials(ctx context.Context, id string) (*models.Account, error)
	GetAccountCredentialsByEmail(ctx context.Context, email string) (*models.Account, error)
	UpdateAccountTokens(ctx context.Context, id string, accessToken, refreshToken string) error
	UpdateAccountOAuth(ctx context.Context, id, provider, imapHost string, imapPort int, imapSSL bool, imapEncryption, smtpHost string, smtpPort int, smtpSSL bool, smtpEncryption, username string) error
	CreateAccount(ctx context.Context, email, name, provider, imapHost string, imapPort int, imapSSL bool, imapEncryption, smtpHost string, smtpPort int, smtpSSL bool, smtpEncryption, username, password, aiConfig, signature string) (*models.Account, error)
	UpdateAccount(ctx context.Context, id, email, name, provider, imapHost string, imapPort int, imapSSL bool, imapEncryption, smtpHost string, smtpPort int, smtpSSL bool, smtpEncryption, username, password, aiConfig, signature string) (*models.Account, error)
	DeleteAccount(ctx context.Context, id string) error
	UpdateAccountTimestamp(ctx context.Context, id string, field string) error
	UpdateSmartCategories(ctx context.Context, accountID string, enabled bool) error
	ResetAccountSync(ctx context.Context, accountID string) error
	GetAccountIDsByFilter(ctx context.Context, accountID, folderID string) ([]string, error)
	UpdateAccountSyncError(ctx context.Context, id string, errText string) error
	UpdateAccountLastUID(ctx context.Context, id string, lastUID uint32) error
	UpdateAccountUIDValidity(ctx context.Context, id string, uidValidity uint32) error
	UpdateAccountIsGmail(ctx context.Context, accountID string, isGmail bool) error
}

// FolderStore — folder management.
type FolderStore interface {
	GetFolders(ctx context.Context, accountID string) ([]models.Folder, error)
	GetFolderByName(ctx context.Context, accountID, name string) (*models.Folder, error)
	GetFolderByID(ctx context.Context, id string) (*models.Folder, error)
	CreateFolder(ctx context.Context, accountID, name, path string, subscribed bool) (*models.Folder, error)
	RenameFolder(ctx context.Context, folderID, newName string) error
	DeleteFolder(ctx context.Context, folderID string) error
	UpdateFolderLastUID(ctx context.Context, folderID string, lastUID int) error
	UpdateFolderUIDValidity(ctx context.Context, folderID string, uidValidity uint32) error
}

// EntityStore — CRUD for templates, labels, rules, contacts, identities,
// attachments, tags, project groups, users, comments, profiles.
type EntityStore interface {
	GetTemplates(ctx context.Context, accountID string) ([]models.Template, error)
	GetTemplate(ctx context.Context, id string) (*models.Template, error)
	CreateTemplate(ctx context.Context, accountID, name, subject, body string) (*models.Template, error)
	DeleteTemplate(ctx context.Context, id string) error
	GetLabels(ctx context.Context, accountID string) ([]models.Label, error)
	GetLabel(ctx context.Context, id string) (*models.Label, error)
	CreateLabel(ctx context.Context, accountID, name, color string) (*models.Label, error)
	UpdateLabel(ctx context.Context, id, name, color string) (*models.Label, error)
	DeleteLabel(ctx context.Context, id string) error
	GetEmailLabels(ctx context.Context, emailID string) ([]models.Label, error)
	SetEmailLabels(ctx context.Context, emailID, accountID string, labelIDs []string) error
	GetRules(ctx context.Context, accountID string) ([]models.FilterRule, error)
	GetRule(ctx context.Context, id string) (*models.FilterRule, error)
	CreateRule(ctx context.Context, r models.FilterRule) (*models.FilterRule, error)
	UpdateRule(ctx context.Context, id string, r models.FilterRule) (*models.FilterRule, error)
	DeleteRule(ctx context.Context, id string) error
	GetContacts(ctx context.Context, accountID string) ([]models.Contact, error)
	GetContact(ctx context.Context, id string) (*models.Contact, error)
	CreateContact(ctx context.Context, contact models.Contact) (*models.Contact, error)
	UpdateContact(ctx context.Context, id string, contact models.Contact) (*models.Contact, error)
	DeleteContact(ctx context.Context, id string) error
	GetIdentities(ctx context.Context, accountID string) ([]models.Identity, error)
	CreateIdentity(ctx context.Context, accountID, email, name string) (*models.Identity, error)
	DeleteIdentity(ctx context.Context, id string) error
	GetEmailAttachments(ctx context.Context, emailID string, accountID string) ([]models.Attachment, error)
	GetAttachmentByHash(ctx context.Context, hash string) (*models.Attachment, error)
	SaveAttachment(ctx context.Context, att *models.Attachment) error
	AddEmailTag(ctx context.Context, emailID string, accountID string, tag string) error
	AddEmailTags(ctx context.Context, emailID string, accountID string, tags []string) error
	GetGroups(ctx context.Context) ([]models.ProjectGroupWithCount, error)
	GetGroupAccounts(ctx context.Context, groupID string) ([]string, error)
	GetGroupEmailAccountIDs(ctx context.Context, groupID string) ([]string, error)
	SetGroupAccounts(ctx context.Context, groupID string, accountIDs []string) error
	CreateGroup(ctx context.Context, name, color string, sortOrder int) (*models.ProjectGroup, error)
	UpdateGroup(ctx context.Context, id, name, color string, sortOrder int) (*models.ProjectGroup, error)
	DeleteGroup(ctx context.Context, id string) error
	GetUsers(ctx context.Context) ([]models.User, error)
	CreateUser(ctx context.Context, email, name, role string) (*models.User, error)
	UpsertUser(ctx context.Context, email, name, role string) (*models.User, error)
	GetUserByEmail(ctx context.Context, email string) (*models.User, error)
	DeleteUser(ctx context.Context, id string) error
	GetComments(ctx context.Context, emailID string) ([]models.EmailComment, error)
	GetComment(ctx context.Context, id string) (*models.EmailComment, error)
	CreateComment(ctx context.Context, emailID, authorID, body string, internal bool) (*models.EmailComment, error)
	DeleteComment(ctx context.Context, id string) error
}

// AdminStore — admin auth, AI, webhooks, MCP, Telegram.
type AdminStore interface {
	AdminExists(ctx context.Context) (bool, error)
	CreateAdmin(ctx context.Context, email, passwordHash string) (string, error)
	UpdateAdminPassword(ctx context.Context, email, passwordHash string) error
	GetAdminByEmail(ctx context.Context, email string) (string, string, error)
	GetAISettings(ctx context.Context, accountID string) (*models.AISetting, error)
	UpsertAISetting(ctx context.Context, setting models.AISetting) error
	GetPresetSettings(ctx context.Context, accountID string, presetName string) (provider string, model string, err error)
	GetAIStats(ctx context.Context) (*models.AILogStats, error)
	ResetAIStats(ctx context.Context) error
	LogAI(ctx context.Context, action, provider, model string, promptTokens, completionTokens, durationMs int, status string) error
	GetAILog(ctx context.Context, limit int) ([]models.AILogEntry, error)
	ListMCPKeys(ctx context.Context) ([]models.MCPKey, error)
	GetMCPKey(ctx context.Context, id string) (*models.MCPKey, error)
	GetMCPKeyFull(ctx context.Context, id string) (string, error)
	GetMCPKeyByAPIKey(ctx context.Context, apiKey string) (*models.MCPKey, error)
	CreateMCPKey(ctx context.Context, name, accountID, keyHash, keyPrefix, rawKey string) (*models.MCPKey, error)
	DeleteMCPKey(ctx context.Context, id string) error
	ToggleMCPKey(ctx context.Context, id string) (*models.MCPKey, error)
	GetWebhooks(ctx context.Context, accountID string) ([]models.Webhook, error)
	GetWebhook(ctx context.Context, id string) (*models.Webhook, error)
	CreateWebhook(ctx context.Context, accountID, name, url, secret string) (*models.Webhook, error)
	DeleteWebhook(ctx context.Context, id string) error
	GetTelegramSettings(ctx context.Context, email string) (userID int64, enabled bool, aiNotifications bool, aiChat bool, botToken string, err error)
	UpdateTelegramSettings(ctx context.Context, email string, userID int64, enabled, aiNotifications, aiChat bool, botToken string) error
	GetAnyTelegramSettings(ctx context.Context) (userID int64, enabled bool, aiNotifications bool, aiChat bool, botToken string, err error)
	RekeyAll(ctx context.Context) (int, error)
}

// SystemStore — migrations, FTS, settings, queue management, jobs, sync queue.
type SystemStore interface {
	RunMigrations(ctx context.Context) (int, error)
	Ping(ctx context.Context) error
	IndexEmailFTS(ctx context.Context, emailID, accountID, subject, senderName, senderAddress, recipientAddress, body string) error
	SearchFTS(ctx context.Context, q, accountID string, limit int) ([]string, error)
	ReindexFTS(ctx context.Context) error
	GetSystemSetting(ctx context.Context, key string) (string, error)
	SetSystemSetting(ctx context.Context, key, value string) error
	ProcessQueueRetries(ctx context.Context) (int64, error)
	CleanQueueGarbage(ctx context.Context, retentionCompleted time.Duration, retentionFailed time.Duration) (int64, error)
	EnqueueJob(ctx context.Context, jobType string, payload string, runAt time.Time) error
	DequeueJobs(ctx context.Context, limit int) ([]models.Job, error)
	CompleteJob(ctx context.Context, jobID string) error
	FailJob(ctx context.Context, jobID string, errMessage string) error
	StoreScheduledEmail(ctx context.Context, id, accountID, payload string) error
	GetScheduledEmail(ctx context.Context, id string) (string, string, error)
	DeleteScheduledEmail(ctx context.Context, id string) error
	EnqueueUIDs(ctx context.Context, accountID, folderName string, uids []uint32, priority int) error
	DequeueUIDs(ctx context.Context, accountID string, limit int) ([]models.SyncTask, error)
	CompleteSyncTask(ctx context.Context, taskID int64) error
	CompleteSyncTasks(ctx context.Context, taskIDs []int64) error
	FailSyncTask(ctx context.Context, taskID int64, errReason string) error
	RemoveSyncTaskByUID(ctx context.Context, accountID, folderName string, uid uint32) error
	ClearFolderQueue(ctx context.Context, accountID, folderName string) error
	GetEmailIDByFolderUID(ctx context.Context, accountID, folderPath string, uid uint32) (string, error)
	UpsertEmailLabels(ctx context.Context, emailID, accountID string, labels []string) error
	CleanupGmailDuplicates(ctx context.Context, accountID string) (int, error)
}
