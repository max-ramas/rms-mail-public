package chatbot

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"strings"
	"sync"

	"rmsmail/internal/ai"
	"rmsmail/internal/models"
)

type TelegramBot struct {
	Token        string
	store        TGUserStore
	aiGateway    AIGateway
	sessionStore SessionStore
	username     string
}

type TGUserStore interface {
	IsTelegramUserAllowed(ctx context.Context, userID int64) bool
	GetEmailByTelegramUserID(ctx context.Context, userID int64) (string, error)
	GetAccounts(ctx context.Context) ([]models.Account, error)
	GetEmails(ctx context.Context, unified bool, accountID string, folderID string, folderName string, offset int, limit int, filter models.EmailFilterOpts) ([]models.Email, error)
	GetAISettings(ctx context.Context, accountID string) (*models.AISetting, error)
}

type AIGateway interface {
	Chat(ctx context.Context, accountID string, messages []AIMessage) (string, error)
	ChatWithTools(ctx context.Context, accountID string, messages []AIMessage, tools []interface{}) (AIMessage, error)
}

type AIMessage struct {
	Role             string      `json:"role"`
	Content          string      `json:"content"`
	Name             string      `json:"name,omitempty"`
	ToolCallID       string      `json:"tool_call_id,omitempty"`
	ToolCalls        interface{} `json:"tool_calls,omitempty"`
	ReasoningContent string      `json:"reasoning_content,omitempty"`
}

type SessionStore interface {
	Get(ctx context.Context, key string) ([]AIMessage, error)
	Append(ctx context.Context, key string, msg AIMessage) error
	Trim(ctx context.Context, key string, maxTokens int) error
}

func NewTelegramBot(store TGUserStore, ai AIGateway, sessions SessionStore) *TelegramBot {
	return &TelegramBot{
		Token:        os.Getenv("TELEGRAM_BOT_TOKEN"),
		store:        store,
		aiGateway:    ai,
		sessionStore: sessions,
	}
}

func (b *TelegramBot) IsConfigured() bool {
	if b == nil {
		return false
	}
	return b.Token != ""
}

func (b *TelegramBot) GetToken() string {
	if b == nil {
		return ""
	}
	return b.Token
}

func (b *TelegramBot) SetToken(token string) {
	if b != nil {
		b.Token = token
	}
}

func (b *TelegramBot) GetUsername() string {
	if b == nil {
		return ""
	}
	return b.username
}

func (b *TelegramBot) SetUsername(name string) {
	if b == nil {
		return
	}
	b.username = name
}

func (b *TelegramBot) FetchUsername(ctx context.Context) (string, error) {
	if b == nil || b.Token == "" {
		return "", fmt.Errorf("bot token is empty")
	}
	url := fmt.Sprintf("https://api.telegram.org/bot%s/getMe", b.Token)
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return "", err
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	var result struct {
		Ok     bool `json:"ok"`
		Result struct {
			Username string `json:"username"`
		} `json:"result"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", err
	}
	if !result.Ok {
		return "", fmt.Errorf("telegram api error")
	}
	b.username = result.Result.Username
	return b.username, nil
}

type tgUpdate struct {
	Message struct {
		MessageID int `json:"message_id"`
		From      struct {
			ID        int64  `json:"id"`
			FirstName string `json:"first_name"`
		} `json:"from"`
		Chat struct {
			ID int64 `json:"id"`
		} `json:"chat"`
		Text string `json:"text"`
	} `json:"message"`
}

func (b *TelegramBot) HandleWebhook(w http.ResponseWriter, r *http.Request) {
	if b.Token == "" {
		http.Error(w, "bot not configured", http.StatusServiceUnavailable)
		return
	}

	// Verify Telegram secret token to prevent spoofed webhook requests
	expectedSecret := fmt.Sprintf("%x", sha256.Sum256([]byte(b.Token)))
	if r.Header.Get("X-Telegram-Bot-Api-Secret-Token") != expectedSecret {
		slog.Info("TG webhook: invalid or missing secret token")
		w.WriteHeader(http.StatusOK)
		return
	}

	var update tgUpdate
	if err := json.NewDecoder(io.LimitReader(r.Body, 65536)).Decode(&update); err != nil {
		slog.Info(fmt.Sprintf("TG webhook decode error: %v", err))
		w.WriteHeader(http.StatusOK)
		return
	}

	userID := update.Message.From.ID
	if !b.store.IsTelegramUserAllowed(r.Context(), userID) {
		slog.Info(fmt.Sprintf("TG: unauthorized user %d", userID))
		// We still return 200 OK so Telegram doesn't keep retrying
		b.sendMessage(update.Message.Chat.ID, "🔒 <b>Доступ заблокирован</b>\n\nТвой Telegram ID не авторизован в настройках RMS Mail, либо функция управления ИИ-чатом отключена.")
		w.WriteHeader(http.StatusOK)
		return
	}

	sessionKey := fmt.Sprintf("tg_session:%d", userID)
	reply, err := b.ProcessMessage(r.Context(), userID, sessionKey, update.Message.Text)
	if err != nil {
		slog.Info(fmt.Sprintf("TG process error: %v", err))
		reply = "Sorry, something went wrong with the AI assistant."
	}

	b.sendMessage(update.Message.Chat.ID, reply)
	w.WriteHeader(http.StatusOK)
}

// hasToolCalls checks if the AIMessage has non-empty tool calls.
func hasToolCalls(msg AIMessage) bool {
	if msg.ToolCalls == nil {
		return false
	}
	if tcBytes, err := json.Marshal(msg.ToolCalls); err == nil {
		var tcs []interface{}
		if json.Unmarshal(tcBytes, &tcs) == nil {
			return len(tcs) > 0
		}
	}
	return false
}

func (b *TelegramBot) ProcessMessage(ctx context.Context, userID int64, sessionKey string, msg string) (string, error) {
	msg = strings.TrimSpace(msg)

	// 1. Resolve user's primary email address using their Telegram ID
	userEmail, err := b.store.GetEmailByTelegramUserID(ctx, userID)
	if err != nil {
		slog.Info(fmt.Sprintf("TG Bot: failed to resolve email for user %d: %v", userID, err))
	}

	// Resolve accountID (UUID) from email — GetEmails and GetAISettings expect UUID, not email
	accountID := ""
	if userEmail != "" {
		accounts, accErr := b.store.GetAccounts(ctx)
		if accErr == nil {
			for _, a := range accounts {
				if strings.EqualFold(a.Email, userEmail) {
					accountID = a.ID
					break
				}
			}
		}
	}

	// 2. Process commands
	if strings.HasPrefix(msg, "/") {
		cmd := strings.Fields(msg)[0]
		switch cmd {
		case "/start":
			return "👋 <b>Привет! Я твой персональный ИИ-ассистент RMS Mail.</b>\n\n" +
				"Я готов помочь тебе управлять твоей почтой прямо отсюда.\n\n" +
				"📌 <b>Доступные команды:</b>\n" +
				"• /latest — показать 5 последних входящих писем\n" +
				"• /summary — сделать общий ИИ-отчет по последним письмам\n" +
				"• /help — получить подробную справку\n\n" +
				"✍️ Ты можешь общаться со мной в свободной форме. Например:\n" +
				"<i>\"Есть ли новые важные письма?\"</i>\n" +
				"<i>\"О чем пишет Иван?\"</i>\n" +
				"<i>\"Напиши черновик ответа на последнее письмо\"</i>", nil

		case "/help":
			return "📖 <b>Справка по RMS Mail боту</b>\n\n" +
				"🤖 Этот бот интегрирован с почтовым клиентом RMS Mail и использует передовой искусственный интеллект для анализа твоих писем.\n\n" +
				"⚙️ <b>Возможности:</b>\n" +
				"1. <b>Мгновенные оповещения:</b> если включено в настройках, ты будешь получать уведомления о каждом новом письме (включая умные ИИ-выжимки).\n" +
				"2. <b>Интеллектуальный поиск и анализ:</b> я знаю контекст твоих последних писем и могу отвечать на вопросы о них.\n" +
				"3. <b>Команды управления:</b>\n" +
				"• /latest — список последних писем с темой и отправителем.\n" +
				"• /summary — ИИ проанализирует входящие за последнее время и выдаст краткий отчет о главном.\n\n" +
				"Если у тебя есть вопросы, просто напиши мне!", nil

		case "/latest":
			if userEmail == "" {
				return "⚠️ Твой Telegram ID не привязан ни к одному аккаунту в настройках RMS Mail. Пожалуйста, укажи его на странице настроек.", nil
			}
			emails, err := b.store.GetEmails(ctx, false, accountID, "", "INBOX", 0, 5, models.EmailFilterOpts{})
			if err != nil || len(emails) == 0 {
				return "📭 Твой почтовый ящик INBOX пуст или не удалось загрузить письма.", nil
			}
			var sb strings.Builder
			sb.WriteString("📥 <b>Последние 5 входящих писем:</b>\n\n")
			for i, e := range emails {
				sb.WriteString(fmt.Sprintf("%d. <b>%s</b>\n   👤 От: %s\n   📝 %s\n\n", i+1, htmlEscape(e.Subject), htmlEscape(e.SenderName), htmlEscape(e.Snippet)))
			}
			return sb.String(), nil

		case "/summary":
			if userEmail == "" {
				return "⚠️ Твой Telegram ID не привязан ни к одному аккаунту в настройках RMS Mail.", nil
			}
			emails, err := b.store.GetEmails(ctx, false, accountID, "", "INBOX", 0, 5, models.EmailFilterOpts{})
			if err != nil || len(emails) == 0 {
				return "📭 Твой почтовый ящик INBOX пуст.", nil
			}
			var sb strings.Builder
			sb.WriteString("Provide a concise summary for the following 5 recent emails. ")
			sb.WriteString("Highlight anything interesting or requiring attention.\n\n")
			for i, e := range emails {
				sb.WriteString(fmt.Sprintf("Email %d:\nSubject: %s\nFrom: %s\nSnippet: %s\n\n", i+1, e.Subject, e.SenderName, e.Snippet))
			}

			aiMsgs := []AIMessage{
				{Role: "user", Content: sb.String()},
			}
			return b.aiGateway.Chat(ctx, accountID, aiMsgs)
		}
	}

	// 3. Regular chat - Use Tool Calling to let AI access emails
	messages, _ := b.sessionStore.Get(ctx, sessionKey)

	// Load system prompt from ai_settings, fallback to hardcoded default
	defaultPrompt := "You are the RMS Mail AI assistant. You help the user manage their email through this Telegram chat.\n" +
		"Answer the user's questions clearly, concisely and professionally.\n" +
		"You have a tool 'fetch_emails' that lets you query the user's mailbox.\n" +
		"If the user asks about their recent emails or specific emails, YOU MUST use the fetch_emails tool to get the data before answering.\n" +
		"IMPORTANT: Your fetch_emails calls are automatically scoped to the user's mailbox. You do NOT need to specify account_id.\n" +
		"After ONE fetch_emails call, synthesize a response from the results. Do NOT call fetch_emails again unless the user asks a follow-up question.\n" +
		"Always use standard HTML formatting for Telegram (<b>, <i>, <code>) to make messages readable."

	systemPrompt := defaultPrompt
	if setting, err := b.store.GetAISettings(ctx, accountID); err == nil && setting != nil && setting.Prompts != "" {
		var prompts map[string]string
		if json.Unmarshal([]byte(setting.Prompts), &prompts) == nil {
			if p, ok := prompts["tg_bot"]; ok && p != "" {
				systemPrompt = p
			}
		}
	}

	var filtered []AIMessage
	filtered = append(filtered, AIMessage{Role: "system", Content: systemPrompt})
	for _, m := range messages {
		if m.Role != "system" {
			filtered = append(filtered, m)
		}
	}
	messages = filtered

	userMsg := AIMessage{Role: "user", Content: msg}
	messages = append(messages, userMsg)

	// Tools
	tools := make([]interface{}, len(AvailableTools))
	for i, t := range AvailableTools {
		tools[i] = t
	}

	b.sessionStore.Trim(ctx, sessionKey, 32000)

	// Tool execution loop
	const maxToolCalls = 5
	for i := 0; i < maxToolCalls; i++ {
		replyMsg, err := b.aiGateway.ChatWithTools(ctx, accountID, messages, tools)
		if err != nil {
			return "", err
		}

		messages = append(messages, replyMsg)

		// If no tool calls, this is a final text response
		if !hasToolCalls(replyMsg) {
			b.sessionStore.Append(ctx, sessionKey, userMsg)
			b.sessionStore.Append(ctx, sessionKey, replyMsg)
			return replyMsg.Content, nil
		}

		// Handle tool calls
		var toolCalls []ai.ToolCall
		if tcBytes, err := json.Marshal(replyMsg.ToolCalls); err == nil {
			json.Unmarshal(tcBytes, &toolCalls)
		}

		for _, tc := range toolCalls {
			if tc.Function.Name == "fetch_emails" {
				var args struct {
					SearchQuery string `json:"search_query"`
					Limit       int    `json:"limit"`
					AccountID   string `json:"account_id"`
					FolderName  string `json:"folder_name"`
				}
				json.Unmarshal([]byte(tc.Function.Arguments), &args)

				if args.Limit <= 0 || args.Limit > 30 {
					args.Limit = 10
				}
				if args.FolderName == "" {
					args.FolderName = "INBOX"
				}
				// GUARD: always use user-resolved accountID, ignore LLM's "unified"
				queryAID := accountID
				if args.AccountID != "" && args.AccountID != "unified" && strings.EqualFold(args.AccountID, userEmail) {
					queryAID = args.AccountID
				}

				filter := models.EmailFilterOpts{Search: args.SearchQuery}
				emails, err := b.store.GetEmails(ctx, false, queryAID, "", args.FolderName, 0, args.Limit, filter)

				var toolResult string
				if err != nil {
					toolResult = fmt.Sprintf("Error fetching emails: %v", err)
				} else {
					if len(emails) == 0 {
						toolResult = "No emails found matching the criteria."
					} else {
						var sb strings.Builder
						for idx, e := range emails {
							sb.WriteString(fmt.Sprintf("[%d] Subject: %s | From: %s <%s> | Snippet: %s\n", idx+1, e.Subject, e.SenderName, e.SenderAddress, e.Snippet))
						}
						toolResult = sb.String()
					}
				}

				messages = append(messages, AIMessage{
					Role:       "tool",
					Content:    toolResult,
					Name:       tc.Function.Name,
					ToolCallID: tc.ID,
				})
			}
		}
	}

	// Too many tool calls — try to synthesize a response from collected data
	noToolMessages := make([]AIMessage, len(messages))
	copy(noToolMessages, messages)
	noToolMessages = append(noToolMessages, AIMessage{
		Role:    "user",
		Content: "Based on the tool results above, please provide a concise answer to the user's original question. Use Telegram HTML formatting.",
	})
	summary, err := b.aiGateway.Chat(ctx, accountID, noToolMessages)
	if err == nil && summary != "" {
		return summary, nil
	}
	return "I gathered the information but couldn't format a response. Please try again.", nil
}

func (b *TelegramBot) sendMessage(chatID int64, text string) {
	url := fmt.Sprintf("https://api.telegram.org/bot%s/sendMessage", b.Token)
	body := fmt.Sprintf(`{"chat_id":%d,"text":%s,"parse_mode":"HTML"}`, chatID, jsonEscape(text))
	req, _ := http.NewRequest("POST", url, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		slog.Info(fmt.Sprintf("TG send error: %v", err))
		return
	}
	resp.Body.Close()
}

func jsonEscape(s string) string {
	b, _ := json.Marshal(s)
	return string(b)
}

func htmlEscape(s string) string {
	s = strings.ReplaceAll(s, "&", "&amp;")
	s = strings.ReplaceAll(s, "<", "&lt;")
	s = strings.ReplaceAll(s, ">", "&gt;")
	return s
}

func (b *TelegramBot) SetWebhook(ctx context.Context, publicURL string) error {
	secretToken := fmt.Sprintf("%x", sha256.Sum256([]byte(b.Token)))
	url := fmt.Sprintf("https://api.telegram.org/bot%s/setWebhook?url=%s/api/tg/webhook&secret_token=%s",
		b.Token, publicURL, secretToken)
	resp, err := http.Get(url)
	if err != nil {
		return err
	}
	resp.Body.Close()
	return nil
}

// Memory-based session store
type MemSessionStore struct {
	mu   sync.RWMutex
	data map[string][]AIMessage
}

func NewMemSessionStore() *MemSessionStore {
	return &MemSessionStore{data: make(map[string][]AIMessage)}
}

func (s *MemSessionStore) Get(ctx context.Context, key string) ([]AIMessage, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.data[key], nil
}

func (s *MemSessionStore) Append(ctx context.Context, key string, msg AIMessage) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.data[key] = append(s.data[key], msg)
	return nil
}

func (s *MemSessionStore) Trim(ctx context.Context, key string, maxTokens int) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	msgs := s.data[key]
	total := 0
	for i := len(msgs) - 1; i >= 0; i-- {
		total += len(msgs[i].Content)
		if total > maxTokens {
			s.data[key] = msgs[i+1:]
			return nil
		}
	}
	return nil
}
