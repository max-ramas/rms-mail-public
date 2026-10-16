package ports

import (
	"context"

	"rmsmail/internal/models"
)

// JobStore is the subset of persistence used by the Mono DB job worker.
// Defined here so sync does not import the api package (DIP).
type JobStore interface {
	DequeueJobs(ctx context.Context, limit int) ([]models.Job, error)
	CompleteJob(ctx context.Context, jobID string) error
	FailJob(ctx context.Context, jobID string, errMessage string) error

	GetEmail(ctx context.Context, id string, accountID string) (*models.Email, error)
	GetAccount(ctx context.Context, id string) (*models.Account, error)
	GetAccountCredentials(ctx context.Context, id string) (*models.Account, error)
	GetEmailAttachments(ctx context.Context, emailID string, accountID string) ([]models.Attachment, error)
	SaveDraftReply(ctx context.Context, emailID string, accountID string, draftReply string) error
	ClearDraftReply(ctx context.Context, emailID string, accountID string) error
}
