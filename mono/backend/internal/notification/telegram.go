package notification

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"
)

type contextKey string

const BotTokenKey contextKey = "tg_bot_token"

var GlobalBotToken string

type TelegramProvider struct {
	BotToken string
	client   *http.Client
}

func NewTelegramProvider() *TelegramProvider {
	token := os.Getenv("TELEGRAM_BOT_TOKEN")
	if token == "" {
		token = GlobalBotToken
	}
	return &TelegramProvider{
		BotToken: token,
		client:   &http.Client{Timeout: 5 * time.Second},
	}
}

func (p *TelegramProvider) Send(ctx context.Context, targetID string, text string) error {
	token := p.BotToken
	if ctxToken, ok := ctx.Value(BotTokenKey).(string); ok && ctxToken != "" {
		token = ctxToken
	}
	if token == "" {
		token = GlobalBotToken
	}
	if token == "" {
		token = os.Getenv("TELEGRAM_BOT_TOKEN")
	}
	if token == "" {
		return fmt.Errorf("TELEGRAM_BOT_TOKEN not set")
	}

	// Telegram parse_mode=HTML expects raw HTML tags (<b>, <i>, <code>).
	// We only escape bare & that aren't part of known entities.
	// < and > are NOT escaped — Telegram safely strips unknown tags.
	// Callers are responsible for escaping user data (e.g. &lt; &gt; &amp;).
	safe := escapeTelegramHTML(text)

	u := fmt.Sprintf("https://api.telegram.org/bot%s/sendMessage", token)
	payload := url.Values{
		"chat_id":    {targetID},
		"text":       {safe},
		"parse_mode": {"HTML"},
	}

	req, err := http.NewRequestWithContext(ctx, "POST", u, strings.NewReader(payload.Encode()))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := p.client.Do(req)
	if err != nil {
		return err
	}
	resp.Body.Close()
	return nil
}

// escapeTelegramHTML performs minimal escaping for Telegram HTML parse_mode.
// Only bare & that aren't part of known entities (&lt; &gt; &amp; &quot;) are escaped.
// < and > are left as-is — Telegram strips unknown tags safely.
// Callers must escape user-provided data (e.g. subject/sender/snippet) themselves.
func escapeTelegramHTML(s string) string {
	// Replace & not followed by lt; gt; amp; quot; or #
	var buf strings.Builder
	buf.Grow(len(s))
	i := 0
	for i < len(s) {
		if s[i] == '&' {
			// Check for known entities
			rest := s[i+1:]
			if strings.HasPrefix(rest, "lt;") || strings.HasPrefix(rest, "gt;") ||
				strings.HasPrefix(rest, "amp;") || strings.HasPrefix(rest, "quot;") ||
				(len(rest) > 0 && rest[0] == '#') {
				buf.WriteByte('&')
				i++
				continue
			}
			buf.WriteString("&amp;")
			i++
		} else {
			buf.WriteByte(s[i])
			i++
		}
	}
	return buf.String()
}
