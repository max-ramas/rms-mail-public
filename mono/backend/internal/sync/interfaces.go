package sync

import (
	"context"

	"rmsmail/internal/ai"
	"rmsmail/internal/models"
)

// ── Dependency Inversion interfaces (DIP) ──────────────────────
// Replacing concrete types (*attachment.CASStorage, *ai.Gateway) with
// minimal interfaces enables testing without real file-system or AI.

// CASStore is the subset of attachment.CASStorage used by sync workers.
type CASStore interface {
	Save(ctx context.Context, emailID, accountID, filename string, data []byte, contentID string) (*models.Attachment, error)
}

// AIProvider is the subset of ai.Gateway used by sync workers.
type AIProvider interface {
	Chat(ctx context.Context, providerName, modelName, apiKey string, messages []ai.Message) (string, error)
	GetProvider(name string) (ai.AIProvider, bool)
	Providers() map[string]ai.AIProvider
}
