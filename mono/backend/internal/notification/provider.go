package notification

import "context"

type Provider interface {
	Send(ctx context.Context, targetID string, text string) error
}
