package chatbot

import (
	"context"
	"net/http"
)

type Gateway interface {
	HandleWebhook(w http.ResponseWriter, r *http.Request)
	ProcessMessage(ctx context.Context, sessionKey string, msg string) (string, error)
}
