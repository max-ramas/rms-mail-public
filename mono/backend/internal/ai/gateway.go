package ai

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"strings"
	"time"
)

func debugLog(format string, args ...interface{}) {
	if os.Getenv("DEBUG") == "true" {
		slog.Info(format, args...)
	}
}

// ProviderEnvKey maps a provider name to its environment variable for the API key.
func ProviderEnvKey(providerName string) string {
	switch providerName {
	case "openrouter":
		return "OPENROUTER_API_KEY"
	case "openai":
		return "OPENAI_API_KEY"
	case "anthropic":
		return "ANTHROPIC_API_KEY"
	case "gemini":
		return "GEMINI_API_KEY"
	case "deepseek":
		return "DEEPSEEK_API_KEY"
	case "groq":
		return "GROQ_API_KEY"
	case "xai":
		return "XAI_API_KEY"
	case "qwen":
		return "QWEN_API_KEY"
	case "opencode":
		return "OPENCODE_API_KEY"
	default:
		return ""
	}
}

type ToolCallFunction struct {
	Name      string `json:"name"`
	Arguments string `json:"arguments"`
}

type ToolCall struct {
	ID       string           `json:"id"`
	Type     string           `json:"type"` // "function"
	Function ToolCallFunction `json:"function"`
}

type ToolFunction struct {
	Name        string      `json:"name"`
	Description string      `json:"description"`
	Parameters  interface{} `json:"parameters"`
}

type Tool struct {
	Type     string       `json:"type"` // "function"
	Function ToolFunction `json:"function"`
}

type Message struct {
	Role             string     `json:"role"`
	Content          string     `json:"content"`
	Name             string     `json:"name,omitempty"`
	ToolCallID       string     `json:"tool_call_id,omitempty"`
	ToolCalls        []ToolCall `json:"tool_calls,omitempty"`
	ReasoningContent string     `json:"reasoning_content,omitempty"`
}

type AIProvider interface {
	Chat(ctx context.Context, messages []Message) (string, error)
	ChatWithTools(ctx context.Context, messages []Message, tools []Tool) (Message, error)
	Summarize(ctx context.Context, text string) (string, error)
	GenerateReply(ctx context.Context, contextText, prompt string) (string, error)
	Categorize(ctx context.Context, text string, categories []string) ([]string, error)
	ListModels(ctx context.Context) ([]string, error)
}

type Gateway struct {
	providers map[string]AIProvider
}

func NewGateway() *Gateway {
	return &Gateway{
		providers: make(map[string]AIProvider),
	}
}

func (g *Gateway) RegisterProvider(name string, provider AIProvider) {
	g.providers[name] = provider
}

func (g *Gateway) GetProvider(name string) (AIProvider, bool) {
	p, ok := g.providers[name]
	return p, ok
}

func (g *Gateway) Providers() map[string]AIProvider {
	return g.providers
}

// Chat sends a chat completion request to the specified provider with the given model.
// If modelName is empty, the provider's default model is used.
func (g *Gateway) Chat(ctx context.Context, providerName, modelName, apiKey string, messages []Message) (string, error) {
	provider, ok := g.providers[providerName]
	if !ok {
		// Try default provider
		provider, ok = g.providers["openrouter"]
		if !ok {
			// Try any available provider
			for _, p := range g.providers {
				provider = p
				break
			}
		}
	}
	if provider == nil {
		return "", fmt.Errorf("no AI provider available")
	}

	if modelName != "" {
		if p := OverrideProviderSettings(provider, modelName, apiKey); p != nil {
			provider = p
		}
	}

	return provider.Chat(ctx, messages)
}
func (g *Gateway) ChatWithTools(ctx context.Context, providerName, modelName, apiKey string, messages []Message, tools []Tool) (Message, error) {
	provider, ok := g.providers[providerName]
	if !ok {
		provider, ok = g.providers["openrouter"]
		if !ok {
			for _, p := range g.providers {
				provider = p
				break
			}
		}
	}
	if provider == nil {
		return Message{}, fmt.Errorf("no AI provider available")
	}

	if modelName != "" {
		if p := OverrideProviderSettings(provider, modelName, apiKey); p != nil {
			provider = p
		}
	}

	return provider.ChatWithTools(ctx, messages, tools)
}

// OverrideProviderSettings creates a shallow copy of the provider with the specified model and apiKey.
// Returns nil if the provider type is not recognized.
func OverrideProviderSettings(p AIProvider, model, apiKey string) AIProvider {
	switch v := p.(type) {
	case *OpenRouterProvider:
		cp := *v
		if model != "" {
			cp.Model = model
		}
		if apiKey != "" {
			cp.APIKey = apiKey
		}
		return &cp
	case *OpenAIProvider:
		cp := *v
		if model != "" {
			cp.Model = model
		}
		if apiKey != "" {
			cp.APIKey = apiKey
		}
		return &cp
	case *AnthropicProvider:
		cp := *v
		if model != "" {
			cp.Model = model
		}
		if apiKey != "" {
			cp.APIKey = apiKey
		}
		return &cp
	case *GeminiProvider:
		cp := *v
		if model != "" {
			cp.Model = model
		}
		if apiKey != "" {
			cp.APIKey = apiKey
		}
		return &cp
	case *DeepSeekProvider:
		cp := *v
		if model != "" {
			cp.Model = model
		}
		if apiKey != "" {
			cp.APIKey = apiKey
		}
		return &cp
	case *GroqProvider:
		cp := *v
		if model != "" {
			cp.Model = model
		}
		if apiKey != "" {
			cp.APIKey = apiKey
		}
		return &cp
	case *XAIProvider:
		cp := *v
		if model != "" {
			cp.Model = model
		}
		if apiKey != "" {
			cp.APIKey = apiKey
		}
		return &cp
	case *QwenProvider:
		cp := *v
		if model != "" {
			cp.Model = model
		}
		if apiKey != "" {
			cp.APIKey = apiKey
		}
		return &cp
	case *OpenCodeProvider:
		cp := *v
		if model != "" {
			cp.Model = model
		}
		return &cp
	}
	return nil
}

// --- OpenRouter ---

func truncateText(text string, maxTokens int) string {
	const charsPerToken = 4
	maxChars := maxTokens * charsPerToken
	runes := []rune(text)
	if len(runes) > maxChars {
		return string(runes[:maxChars]) + "..."
	}
	return text
}

type OpenRouterProvider struct {
	APIKey string
	Model  string
}

func (p *OpenRouterProvider) Chat(ctx context.Context, messages []Message) (string, error) {
	return p.callAPI(ctx, messages)
}

func (p *OpenRouterProvider) ChatWithTools(ctx context.Context, messages []Message, tools []Tool) (Message, error) {
	if len(tools) == 0 {
		res, err := p.callAPI(ctx, messages)
		return Message{Role: "assistant", Content: res}, err
	}
	return CallOpenAICompatChatWithTools(ctx, "https://openrouter.ai/api/v1/chat/completions", p.Model, p.APIKey, messages, tools)
}

func (p *OpenRouterProvider) Summarize(ctx context.Context, text string) (string, error) {
	return p.Chat(ctx, []Message{{Role: "system", Content: "Summarize the following email in 2-3 sentences."}, {Role: "user", Content: text}})
}

func (p *OpenRouterProvider) GenerateReply(ctx context.Context, contextText, prompt string) (string, error) {
	return p.Chat(ctx, []Message{{Role: "system", Content: "Generate a professional reply."}, {Role: "user", Content: contextText + "\n\n" + prompt}})
}

func (p *OpenRouterProvider) Categorize(ctx context.Context, text string, categories []string) ([]string, error) {
	systemPrompt := buildCategoriesPrompt(categories)
	reply, err := p.Chat(ctx, []Message{{Role: "system", Content: systemPrompt}, {Role: "user", Content: text}})
	if err != nil {
		return nil, err
	}
	return ParseCategories(reply), nil
}

func (p *OpenRouterProvider) ListModels(ctx context.Context) ([]string, error) {
	return FetchOpenAICompatModels(ctx, "https://openrouter.ai/api/v1/models", p.APIKey)
}

func ParseCategories(text string) []string {
	var categories []string
	parts := splitCategories(text)
	for _, p := range parts {
		cat := trimCategory(p)
		if cat != "" {
			categories = append(categories, cat)
		}
	}
	return categories
}

func splitCategories(text string) []string {
	var result []string
	current := ""
	for _, r := range text {
		if r == ',' || r == '\n' {
			if current != "" {
				result = append(result, current)
			}
			current = ""
		} else {
			current += string(r)
		}
	}
	if current != "" {
		result = append(result, current)
	}
	return result
}

func trimCategory(s string) string {
	for len(s) > 0 && (s[0] == ' ' || s[0] == '-' || s[0] == ':') {
		s = s[1:]
	}
	for len(s) > 0 && s[len(s)-1] == ' ' {
		s = s[:len(s)-1]
	}
	return s
}

func (p *OpenRouterProvider) callAPI(ctx context.Context, messages []Message) (string, error) {
	url := "https://openrouter.ai/api/v1/chat/completions"

	apiMessages := make([]map[string]string, 0, len(messages))
	for _, m := range messages {
		apiMessages = append(apiMessages, map[string]string{"role": m.Role, "content": m.Content})
	}

	payload := map[string]interface{}{
		"model":    p.Model,
		"messages": apiMessages,
	}

	jsonPayload, err := json.Marshal(payload)
	if err != nil {
		return "", fmt.Errorf("marshal error: %w", err)
	}
	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewBuffer(jsonPayload))
	if err != nil {
		return "", err
	}

	req.Header.Set("Authorization", "Bearer "+p.APIKey)
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("AI API error: %s", resp.Status)
	}

	var result struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", err
	}

	if len(result.Choices) > 0 {
		return result.Choices[0].Message.Content, nil
	}

	return "", fmt.Errorf("no response from AI")
}
func CallOpenAICompatChatWithTools(ctx context.Context, url, model, apiKey string, messages []Message, tools []Tool) (Message, error) {
	apiMessages := make([]map[string]interface{}, 0, len(messages))
	for _, m := range messages {
		msg := map[string]interface{}{"role": m.Role, "content": m.Content}
		if m.Name != "" {
			msg["name"] = m.Name
		}
		if m.ToolCallID != "" {
			msg["tool_call_id"] = m.ToolCallID
		}
		if len(m.ToolCalls) > 0 {
			msg["tool_calls"] = m.ToolCalls
		}
		if m.ReasoningContent != "" {
			msg["reasoning_content"] = m.ReasoningContent
		}
		apiMessages = append(apiMessages, msg)
	}

	payload := map[string]interface{}{
		"model":    model,
		"messages": apiMessages,
	}
	if len(tools) > 0 {
		payload["tools"] = tools
	}

	jsonPayload, err := json.Marshal(payload)
	if err != nil {
		return Message{}, fmt.Errorf("marshal error: %w", err)
	}
	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewBuffer(jsonPayload))
	if err != nil {
		return Message{}, err
	}

	if apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+apiKey)
	}
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 90 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return Message{}, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return Message{}, fmt.Errorf("API error %s: %s", resp.Status, string(body))
	}

	var result struct {
		Choices []struct {
			Message Message `json:"message"`
		} `json:"choices"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return Message{}, err
	}

	if len(result.Choices) > 0 {
		m := result.Choices[0].Message
		if m.Role == "" {
			m.Role = "assistant"
		}
		return m, nil
	}

	return Message{}, fmt.Errorf("no response from AI")
}

// --- OpenAI ---

type OpenAIProvider struct {
	APIKey string
	Model  string
}

func (p *OpenAIProvider) Chat(ctx context.Context, messages []Message) (string, error) {
	return p.callAPI(ctx, messages)
}

func (p *OpenAIProvider) ChatWithTools(ctx context.Context, messages []Message, tools []Tool) (Message, error) {
	if len(tools) == 0 {
		res, err := p.callAPI(ctx, messages)
		return Message{Role: "assistant", Content: res}, err
	}
	return CallOpenAICompatChatWithTools(ctx, "https://api.openai.com/v1/chat/completions", p.Model, p.APIKey, messages, tools)
}

func (p *OpenAIProvider) Summarize(ctx context.Context, text string) (string, error) {
	return p.Chat(ctx, []Message{{Role: "system", Content: "Summarize the following email in 2-3 sentences."}, {Role: "user", Content: text}})
}

func (p *OpenAIProvider) GenerateReply(ctx context.Context, contextText, prompt string) (string, error) {
	return p.Chat(ctx, []Message{{Role: "system", Content: "Generate a professional reply."}, {Role: "user", Content: contextText + "\n\n" + prompt}})
}

func (p *OpenAIProvider) Categorize(ctx context.Context, text string, categories []string) ([]string, error) {
	systemPrompt := buildCategoriesPrompt(categories)
	reply, err := p.Chat(ctx, []Message{{Role: "system", Content: systemPrompt}, {Role: "user", Content: text}})
	if err != nil {
		return nil, err
	}
	return ParseCategories(reply), nil
}

func (p *OpenAIProvider) ListModels(ctx context.Context) ([]string, error) {
	return FetchOpenAICompatModels(ctx, "https://api.openai.com/v1/models", p.APIKey)
}

func (p *OpenAIProvider) callAPI(ctx context.Context, messages []Message) (string, error) {
	url := "https://api.openai.com/v1/chat/completions"

	apiMessages := make([]map[string]string, 0, len(messages))
	for _, m := range messages {
		apiMessages = append(apiMessages, map[string]string{"role": m.Role, "content": m.Content})
	}

	payload := map[string]interface{}{
		"model":    p.Model,
		"messages": apiMessages,
	}

	jsonPayload, err := json.Marshal(payload)
	if err != nil {
		return "", fmt.Errorf("marshal error: %w", err)
	}
	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewBuffer(jsonPayload))
	if err != nil {
		return "", err
	}

	req.Header.Set("Authorization", "Bearer "+p.APIKey)
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("OpenAI API error: %s", resp.Status)
	}

	var result struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", err
	}

	if len(result.Choices) > 0 {
		return result.Choices[0].Message.Content, nil
	}

	return "", fmt.Errorf("no response from AI")
}

// --- Anthropic ---

type AnthropicProvider struct {
	APIKey string
	Model  string
}

func (p *AnthropicProvider) ChatWithTools(ctx context.Context, messages []Message, tools []Tool) (Message, error) {
	return Message{}, fmt.Errorf("tools not supported for AnthropicProvider yet")
}

func (p *AnthropicProvider) Chat(ctx context.Context, messages []Message) (string, error) {
	return p.callAPI(ctx, messages)
}

func (p *AnthropicProvider) Summarize(ctx context.Context, text string) (string, error) {
	return p.Chat(ctx, []Message{{Role: "user", Content: "Summarize the following email in 2-3 sentences: " + text}})
}

func (p *AnthropicProvider) GenerateReply(ctx context.Context, contextText, prompt string) (string, error) {
	return p.Chat(ctx, []Message{{Role: "system", Content: "Generate a professional reply."}, {Role: "user", Content: contextText + "\n\n" + prompt}})
}

func (p *AnthropicProvider) Categorize(ctx context.Context, text string, categories []string) ([]string, error) {
	systemPrompt := buildCategoriesPrompt(categories)
	reply, err := p.Chat(ctx, []Message{{Role: "system", Content: systemPrompt}, {Role: "user", Content: text}})
	if err != nil {
		return nil, err
	}
	return ParseCategories(reply), nil
}

func (p *AnthropicProvider) ListModels(ctx context.Context) ([]string, error) {
	return nil, nil
}

func (p *AnthropicProvider) callAPI(ctx context.Context, messages []Message) (string, error) {
	url := "https://api.anthropic.com/v1/messages"

	var systemContent string
	apiMessages := make([]map[string]string, 0, len(messages))
	for _, m := range messages {
		if m.Role == "system" {
			systemContent = m.Content
		} else {
			apiMessages = append(apiMessages, map[string]string{"role": m.Role, "content": m.Content})
		}
	}

	payload := map[string]interface{}{
		"model":      p.Model,
		"messages":   apiMessages,
		"max_tokens": 1024,
	}
	if systemContent != "" {
		payload["system"] = systemContent
	}

	jsonPayload, err := json.Marshal(payload)
	if err != nil {
		return "", fmt.Errorf("marshal error: %w", err)
	}
	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewBuffer(jsonPayload))
	if err != nil {
		return "", err
	}

	req.Header.Set("x-api-key", p.APIKey)
	req.Header.Set("anthropic-version", "2023-06-01")
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("Anthropic API error: %s", resp.Status)
	}

	var result struct {
		Content []struct {
			Text string `json:"text"`
		} `json:"content"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", err
	}

	if len(result.Content) > 0 {
		return result.Content[0].Text, nil
	}

	return "", fmt.Errorf("no response from AI")
}

// --- Google Gemini ---

type GeminiProvider struct {
	APIKey string
	Model  string
}

func (p *GeminiProvider) ChatWithTools(ctx context.Context, messages []Message, tools []Tool) (Message, error) {
	if len(tools) == 0 {
		text, err := p.Chat(ctx, messages)
		if err != nil {
			return Message{}, err
		}
		return Message{Role: "assistant", Content: text}, nil
	}
	return p.callAPIWithTools(ctx, messages, tools)
}

func (p *GeminiProvider) Chat(ctx context.Context, messages []Message) (string, error) {
	return p.callAPI(ctx, messages)
}

func (p *GeminiProvider) Summarize(ctx context.Context, text string) (string, error) {
	return p.Chat(ctx, []Message{{Role: "user", Content: "Summarize the following email in 2-3 sentences: " + text}})
}

func (p *GeminiProvider) GenerateReply(ctx context.Context, contextText, prompt string) (string, error) {
	return p.Chat(ctx, []Message{{Role: "user", Content: "Context: " + contextText + "\n\n" + prompt}})
}

func (p *GeminiProvider) Categorize(ctx context.Context, text string, categories []string) ([]string, error) {
	reply, err := p.Chat(ctx, []Message{{Role: "user", Content: buildCategoriesPrompt(categories) + "\n\nEmail: " + text}})
	if err != nil {
		return nil, err
	}
	return ParseCategories(reply), nil
}

func (p *GeminiProvider) ListModels(ctx context.Context) ([]string, error) {
	if p.APIKey == "" {
		return nil, nil
	}
	return FetchGeminiModels(ctx, p.APIKey)
}

func (p *GeminiProvider) callAPI(ctx context.Context, messages []Message) (string, error) {
	model := p.Model
	if model == "" {
		model = "gemini-1.5-flash"
	}

	keyLen := len(p.APIKey)
	debugLog("[DIAG] GeminiProvider.callAPI: model=%s api_key_len=%d", model, keyLen)

	url := fmt.Sprintf("https://generativelanguage.googleapis.com/v1beta/models/%s:generateContent", model)

	// Build contents (conversation history) + systemInstruction (separate field).
	// Gemini does NOT support role="system" in contents; it must go in systemInstruction.
	var contents []map[string]interface{}
	var systemInstruction string
	for _, m := range messages {
		if m.Role == "system" {
			if systemInstruction != "" {
				systemInstruction += "\n"
			}
			systemInstruction += m.Content
			continue
		}
		role := m.Role
		if role == "assistant" {
			role = "model"
		}
		contents = append(contents, map[string]interface{}{
			"role":  role,
			"parts": []map[string]string{{"text": m.Content}},
		})
	}

	payload := map[string]interface{}{
		"contents": contents,
	}
	if systemInstruction != "" {
		payload["systemInstruction"] = map[string]interface{}{
			"parts": []map[string]string{{"text": systemInstruction}},
		}
	}

	jsonPayload, err := json.Marshal(payload)
	if err != nil {
		return "", fmt.Errorf("marshal error: %w", err)
	}
	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewBuffer(jsonPayload))
	if err != nil {
		return "", err
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-goog-api-key", p.APIKey)

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		debugLog("[DIAG] GeminiProvider.callAPI: HTTP %d, body=%.200s", resp.StatusCode, string(body))
		return "", fmt.Errorf("Gemini API error: %s", resp.Status)
	}

	var result struct {
		Candidates []struct {
			Content struct {
				Parts []struct {
					Text string `json:"text"`
				} `json:"parts"`
			} `json:"content"`
		} `json:"candidates"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", err
	}

	if len(result.Candidates) > 0 && len(result.Candidates[0].Content.Parts) > 0 {
		return result.Candidates[0].Content.Parts[0].Text, nil
	}

	return "", fmt.Errorf("no response from AI")
}

// callAPIWithTools sends a request with function calling (tools) to the Gemini API.
// Converts []Tool → Gemini functionDeclarations format; parses functionCall from response.
func (p *GeminiProvider) callAPIWithTools(ctx context.Context, messages []Message, tools []Tool) (Message, error) {
	model := p.Model
	if model == "" {
		model = "gemini-1.5-flash"
	}

	// Build contents: Gemini expects an array of {role, parts[]} per turn.
	// Unlike callAPI (which squashes everything into one flat text), we preserve
	// the conversation history and include tool responses as functionResponse parts.
	var contents []map[string]interface{}
	var systemInstruction string
	for _, m := range messages {
		if m.Role == "system" {
			if systemInstruction != "" {
				systemInstruction += "\n"
			}
			systemInstruction += m.Content
			continue
		}

		var parts []map[string]interface{}

		if m.Role == "tool" && m.Name != "" {
			// Tool response → functionResponse part
			parts = append(parts, map[string]interface{}{
				"functionResponse": map[string]interface{}{
					"name":     m.Name,
					"response": map[string]interface{}{"content": m.Content},
				},
			})
		} else {
			// Regular text message
			text := m.Content
			if m.Name != "" {
				text = m.Name + ": " + text
			}
			parts = append(parts, map[string]interface{}{"text": text})
		}

		role := m.Role
		if role == "assistant" {
			role = "model"
		} else if role == "tool" {
			role = "user" // Gemini has no tool role; fold into user
		}

		contents = append(contents, map[string]interface{}{
			"role":  role,
			"parts": parts,
		})
	}

	// Convert []Tool → Gemini functionDeclarations
	var funcDecls []map[string]interface{}
	for _, t := range tools {
		decl := map[string]interface{}{
			"name":        t.Function.Name,
			"description": t.Function.Description,
		}
		if t.Function.Parameters != nil {
			decl["parameters"] = t.Function.Parameters
		}
		funcDecls = append(funcDecls, decl)
	}

	payload := map[string]interface{}{
		"contents": contents,
	}
	if systemInstruction != "" {
		payload["systemInstruction"] = map[string]interface{}{
			"parts": []map[string]string{{"text": systemInstruction}},
		}
	}
	payload["tools"] = []map[string]interface{}{
		{"functionDeclarations": funcDecls},
	}

	url := fmt.Sprintf("https://generativelanguage.googleapis.com/v1beta/models/%s:generateContent", model)

	jsonPayload, err := json.Marshal(payload)
	if err != nil {
		return Message{}, fmt.Errorf("marshal error: %w", err)
	}

	debugLog("[DIAG] GeminiProvider.callAPIWithTools: model=%s tools=%d messages=%d key_len=%d",
		model, len(tools), len(messages), len(p.APIKey))

	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewBuffer(jsonPayload))
	if err != nil {
		return Message{}, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-goog-api-key", p.APIKey)

	client := &http.Client{Timeout: 60 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return Message{}, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		debugLog("[DIAG] GeminiProvider.callAPIWithTools: HTTP %d, body=%.200s", resp.StatusCode, string(body))
		return Message{}, fmt.Errorf("Gemini API error: %s", resp.Status)
	}

	var result struct {
		Candidates []struct {
			Content struct {
				Role  string `json:"role"`
				Parts []struct {
					Text         string `json:"text"`
					FunctionCall *struct {
						Name string                 `json:"name"`
						Args map[string]interface{} `json:"args"`
					} `json:"functionCall"`
				} `json:"parts"`
			} `json:"content"`
		} `json:"candidates"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return Message{}, err
	}

	if len(result.Candidates) == 0 || len(result.Candidates[0].Content.Parts) == 0 {
		return Message{}, fmt.Errorf("no response from AI")
	}

	msg := Message{Role: "assistant"}
	for _, part := range result.Candidates[0].Content.Parts {
		if part.FunctionCall != nil {
			argsJSON, _ := json.Marshal(part.FunctionCall.Args)
			msg.ToolCalls = append(msg.ToolCalls, ToolCall{
				ID:   fmt.Sprintf("call_%s_%d", part.FunctionCall.Name, time.Now().UnixNano()),
				Type: "function",
				Function: ToolCallFunction{
					Name:      part.FunctionCall.Name,
					Arguments: string(argsJSON),
				},
			})
		} else if part.Text != "" {
			msg.Content += part.Text
		}
	}

	return msg, nil
}

// --- Ollama (Local) ---

type OllamaProvider struct {
	BaseURL string
	Model   string
}

func NewOllamaProvider(baseURL, model string) *OllamaProvider {
	if baseURL == "" {
		baseURL = "http://localhost:11434"
	}
	if model == "" {
		model = "llama3"
	}
	return &OllamaProvider{BaseURL: baseURL, Model: model}
}

func (p *OllamaProvider) ChatWithTools(ctx context.Context, messages []Message, tools []Tool) (Message, error) {
	return Message{}, fmt.Errorf("tools not supported for OllamaProvider yet")
}

func (p *OllamaProvider) Chat(ctx context.Context, messages []Message) (string, error) {
	return p.callAPI(ctx, messages)
}

func (p *OllamaProvider) Summarize(ctx context.Context, text string) (string, error) {
	return p.Chat(ctx, []Message{{Role: "system", Content: "Summarize the following email in 2-3 sentences."}, {Role: "user", Content: text}})
}

func (p *OllamaProvider) GenerateReply(ctx context.Context, contextText, prompt string) (string, error) {
	return p.Chat(ctx, []Message{{Role: "system", Content: "Generate a professional reply."}, {Role: "user", Content: contextText + "\n\n" + prompt}})
}

func (p *OllamaProvider) Categorize(ctx context.Context, text string, categories []string) ([]string, error) {
	systemPrompt := buildCategoriesPrompt(categories)
	reply, err := p.Chat(ctx, []Message{{Role: "system", Content: systemPrompt}, {Role: "user", Content: text}})
	if err != nil {
		return nil, err
	}
	return ParseCategories(reply), nil
}

func (p *OllamaProvider) ListModels(ctx context.Context) ([]string, error) {
	return FetchOllamaModels(ctx, p.BaseURL)
}

func (p *OllamaProvider) callAPI(ctx context.Context, messages []Message) (string, error) {
	url := p.BaseURL + "/api/chat"

	apiMessages := make([]map[string]string, 0, len(messages))
	for _, m := range messages {
		apiMessages = append(apiMessages, map[string]string{"role": m.Role, "content": m.Content})
	}

	payload := map[string]interface{}{
		"model":    p.Model,
		"messages": apiMessages,
		"stream":   false,
	}

	jsonPayload, err := json.Marshal(payload)
	if err != nil {
		return "", fmt.Errorf("marshal error: %w", err)
	}
	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewBuffer(jsonPayload))
	if err != nil {
		return "", err
	}

	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("Ollama API error: %s", resp.Status)
	}

	var result struct {
		Message struct {
			Content string `json:"content"`
		} `json:"message"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", err
	}

	return result.Message.Content, nil
}

// --- DeepSeek ---

type DeepSeekProvider struct {
	APIKey string
	Model  string
}

func (p *DeepSeekProvider) Chat(ctx context.Context, messages []Message) (string, error) {
	return p.callAPI(ctx, messages)
}

func (p *DeepSeekProvider) ChatWithTools(ctx context.Context, messages []Message, tools []Tool) (Message, error) {
	if len(tools) == 0 {
		res, err := p.callAPI(ctx, messages)
		return Message{Role: "assistant", Content: res}, err
	}
	return CallOpenAICompatChatWithTools(ctx, "https://api.deepseek.com/chat/completions", p.Model, p.APIKey, messages, tools)
}

func (p *DeepSeekProvider) Summarize(ctx context.Context, text string) (string, error) {
	return p.Chat(ctx, []Message{{Role: "system", Content: "Summarize the following email in 2-3 sentences."}, {Role: "user", Content: text}})
}

func (p *DeepSeekProvider) GenerateReply(ctx context.Context, contextText, prompt string) (string, error) {
	return p.Chat(ctx, []Message{{Role: "system", Content: "Generate a professional reply."}, {Role: "user", Content: contextText + "\n\n" + prompt}})
}

func (p *DeepSeekProvider) Categorize(ctx context.Context, text string, categories []string) ([]string, error) {
	systemPrompt := buildCategoriesPrompt(categories)
	reply, err := p.Chat(ctx, []Message{{Role: "system", Content: systemPrompt}, {Role: "user", Content: text}})
	if err != nil {
		return nil, err
	}
	return ParseCategories(reply), nil
}

func (p *DeepSeekProvider) ListModels(ctx context.Context) ([]string, error) {
	return FetchOpenAICompatModels(ctx, "https://api.deepseek.com/v1/models", p.APIKey)
}

func (p *DeepSeekProvider) callAPI(ctx context.Context, messages []Message) (string, error) {
	url := "https://api.deepseek.com/chat/completions"

	model := p.Model
	if model == "" {
		model = "deepseek-v4-flash"
	}

	apiMessages := make([]map[string]string, 0, len(messages))
	for _, m := range messages {
		apiMessages = append(apiMessages, map[string]string{"role": m.Role, "content": m.Content})
	}

	payload := map[string]interface{}{
		"model":    model,
		"messages": apiMessages,
	}

	jsonPayload, err := json.Marshal(payload)
	if err != nil {
		return "", fmt.Errorf("marshal error: %w", err)
	}
	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewBuffer(jsonPayload))
	if err != nil {
		return "", err
	}

	req.Header.Set("Authorization", "Bearer "+p.APIKey)
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("DeepSeek API error: %s", resp.Status)
	}

	var result struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", err
	}

	if len(result.Choices) > 0 {
		return result.Choices[0].Message.Content, nil
	}

	return "", fmt.Errorf("no response from AI")
}

// --- Groq ---

type GroqProvider struct {
	APIKey string
	Model  string
}

func (p *GroqProvider) Chat(ctx context.Context, messages []Message) (string, error) {
	return p.callAPI(ctx, messages)
}

func (p *GroqProvider) ChatWithTools(ctx context.Context, messages []Message, tools []Tool) (Message, error) {
	if len(tools) == 0 {
		res, err := p.callAPI(ctx, messages)
		return Message{Role: "assistant", Content: res}, err
	}
	return CallOpenAICompatChatWithTools(ctx, "https://api.groq.com/openai/v1/chat/completions", p.Model, p.APIKey, messages, tools)
}

func (p *GroqProvider) Summarize(ctx context.Context, text string) (string, error) {
	return p.Chat(ctx, []Message{{Role: "system", Content: "Summarize the following email in 2-3 sentences."}, {Role: "user", Content: text}})
}

func (p *GroqProvider) GenerateReply(ctx context.Context, contextText, prompt string) (string, error) {
	return p.Chat(ctx, []Message{{Role: "system", Content: "Generate a professional reply."}, {Role: "user", Content: contextText + "\n\n" + prompt}})
}

func (p *GroqProvider) Categorize(ctx context.Context, text string, categories []string) ([]string, error) {
	systemPrompt := buildCategoriesPrompt(categories)
	reply, err := p.Chat(ctx, []Message{{Role: "system", Content: systemPrompt}, {Role: "user", Content: text}})
	if err != nil {
		return nil, err
	}
	return ParseCategories(reply), nil
}

func (p *GroqProvider) ListModels(ctx context.Context) ([]string, error) {
	return FetchOpenAICompatModels(ctx, "https://api.groq.com/openai/v1/models", p.APIKey)
}

func (p *GroqProvider) callAPI(ctx context.Context, messages []Message) (string, error) {
	url := "https://api.groq.com/openai/v1/chat/completions"

	model := p.Model
	if model == "" {
		model = "llama3-70b-8192"
	}

	apiMessages := make([]map[string]string, 0, len(messages))
	for _, m := range messages {
		apiMessages = append(apiMessages, map[string]string{"role": m.Role, "content": m.Content})
	}

	payload := map[string]interface{}{
		"model":    model,
		"messages": apiMessages,
	}

	jsonPayload, err := json.Marshal(payload)
	if err != nil {
		return "", fmt.Errorf("marshal error: %w", err)
	}
	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewBuffer(jsonPayload))
	if err != nil {
		return "", err
	}

	req.Header.Set("Authorization", "Bearer "+p.APIKey)
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("Groq API error: %s", resp.Status)
	}

	var result struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", err
	}

	if len(result.Choices) > 0 {
		return result.Choices[0].Message.Content, nil
	}

	return "", fmt.Errorf("no response from AI")
}

// --- xAI (Grok) ---

type XAIProvider struct {
	APIKey string
	Model  string
}

func (p *XAIProvider) Chat(ctx context.Context, messages []Message) (string, error) {
	return p.callAPI(ctx, messages)
}

func (p *XAIProvider) ChatWithTools(ctx context.Context, messages []Message, tools []Tool) (Message, error) {
	if len(tools) == 0 {
		res, err := p.callAPI(ctx, messages)
		return Message{Role: "assistant", Content: res}, err
	}
	return CallOpenAICompatChatWithTools(ctx, "https://api.x.ai/v1/chat/completions", p.Model, p.APIKey, messages, tools)
}

func (p *XAIProvider) Summarize(ctx context.Context, text string) (string, error) {
	return p.Chat(ctx, []Message{{Role: "system", Content: "Summarize the following email in 2-3 sentences."}, {Role: "user", Content: text}})
}

func (p *XAIProvider) GenerateReply(ctx context.Context, contextText, prompt string) (string, error) {
	return p.Chat(ctx, []Message{{Role: "system", Content: "Generate a professional reply."}, {Role: "user", Content: contextText + "\n\n" + prompt}})
}

func (p *XAIProvider) Categorize(ctx context.Context, text string, categories []string) ([]string, error) {
	reply, err := p.Chat(ctx, []Message{
		{Role: "system", Content: buildCategoriesPrompt(categories)},
		{Role: "user", Content: text},
	})
	if err != nil {
		return nil, err
	}
	return ParseCategories(reply), nil
}

func (p *XAIProvider) ListModels(ctx context.Context) ([]string, error) {
	return FetchOpenAICompatModels(ctx, "https://api.x.ai/v1/models", p.APIKey)
}

func (p *XAIProvider) callAPI(ctx context.Context, messages []Message) (string, error) {
	url := "https://api.x.ai/v1/chat/completions"
	model := p.Model
	if model == "" {
		model = "grok-2-latest"
	}
	return CallOpenAICompatChat(ctx, url, model, p.APIKey, messages)
}

// --- OpenCode ---

type OpenCodeProvider struct {
	BaseURL string
	Model   string
}

func NewOpenCodeProvider(baseURL, model string) *OpenCodeProvider {
	if baseURL == "" {
		baseURL = "http://localhost:4312"
	}
	if model == "" {
		model = "codestral-latest"
	}
	return &OpenCodeProvider{BaseURL: baseURL, Model: model}
}

func (p *OpenCodeProvider) Chat(ctx context.Context, messages []Message) (string, error) {
	return p.callAPI(ctx, messages)
}

func (p *OpenCodeProvider) ChatWithTools(ctx context.Context, messages []Message, tools []Tool) (Message, error) {
	if len(tools) == 0 {
		res, err := p.callAPI(ctx, messages)
		return Message{Role: "assistant", Content: res}, err
	}
	return CallOpenAICompatChatWithTools(ctx, p.BaseURL+"/v1/chat/completions", p.Model, "", messages, tools)
}

func (p *OpenCodeProvider) Summarize(ctx context.Context, text string) (string, error) {
	return p.Chat(ctx, []Message{{Role: "system", Content: "Summarize the following email in 2-3 sentences."}, {Role: "user", Content: text}})
}

func (p *OpenCodeProvider) GenerateReply(ctx context.Context, contextText, prompt string) (string, error) {
	return p.Chat(ctx, []Message{{Role: "system", Content: "Generate a professional reply."}, {Role: "user", Content: contextText + "\n\n" + prompt}})
}

func (p *OpenCodeProvider) Categorize(ctx context.Context, text string, categories []string) ([]string, error) {
	reply, err := p.Chat(ctx, []Message{
		{Role: "system", Content: buildCategoriesPrompt(categories)},
		{Role: "user", Content: text},
	})
	if err != nil {
		return nil, err
	}
	return ParseCategories(reply), nil
}

func (p *OpenCodeProvider) ListModels(ctx context.Context) ([]string, error) {
	return FetchOpenAICompatModels(ctx, p.BaseURL+"/v1/models", "")
}

func (p *OpenCodeProvider) callAPI(ctx context.Context, messages []Message) (string, error) {
	url := p.BaseURL + "/v1/chat/completions"
	return CallOpenAICompatChat(ctx, url, p.Model, "", messages)
}

// --- Qwen (DashScope) ---

type QwenProvider struct {
	APIKey string
	Model  string
}

func (p *QwenProvider) Chat(ctx context.Context, messages []Message) (string, error) {
	return p.callAPI(ctx, messages)
}

func (p *QwenProvider) ChatWithTools(ctx context.Context, messages []Message, tools []Tool) (Message, error) {
	if len(tools) == 0 {
		res, err := p.callAPI(ctx, messages)
		return Message{Role: "assistant", Content: res}, err
	}
	return CallOpenAICompatChatWithTools(ctx, "https://dashscope.aliyuncs.com/compatible-mode/v1/chat/completions", p.Model, p.APIKey, messages, tools)
}

func (p *QwenProvider) Summarize(ctx context.Context, text string) (string, error) {
	return p.Chat(ctx, []Message{{Role: "system", Content: "Summarize the following email in 2-3 sentences."}, {Role: "user", Content: text}})
}

func (p *QwenProvider) GenerateReply(ctx context.Context, contextText, prompt string) (string, error) {
	return p.Chat(ctx, []Message{{Role: "system", Content: "Generate a professional reply."}, {Role: "user", Content: contextText + "\n\n" + prompt}})
}

func (p *QwenProvider) Categorize(ctx context.Context, text string, categories []string) ([]string, error) {
	reply, err := p.Chat(ctx, []Message{
		{Role: "system", Content: buildCategoriesPrompt(categories)},
		{Role: "user", Content: text},
	})
	if err != nil {
		return nil, err
	}
	return ParseCategories(reply), nil
}

func (p *QwenProvider) ListModels(ctx context.Context) ([]string, error) {
	return FetchOpenAICompatModels(ctx, "https://dashscope.aliyuncs.com/compatible-mode/v1/models", p.APIKey)
}

func (p *QwenProvider) callAPI(ctx context.Context, messages []Message) (string, error) {
	url := "https://dashscope.aliyuncs.com/compatible-mode/v1/chat/completions"
	model := p.Model
	if model == "" {
		model = "qwen-plus"
	}
	return CallOpenAICompatChat(ctx, url, model, p.APIKey, messages)
}

func CallOpenAICompatChat(ctx context.Context, url, model, apiKey string, messages []Message) (string, error) {
	apiMessages := make([]map[string]string, 0, len(messages))
	for _, m := range messages {
		apiMessages = append(apiMessages, map[string]string{"role": m.Role, "content": m.Content})
	}

	payload := map[string]interface{}{
		"model":    model,
		"messages": apiMessages,
	}

	jsonPayload, err := json.Marshal(payload)
	if err != nil {
		return "", fmt.Errorf("marshal error: %w", err)
	}
	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewBuffer(jsonPayload))
	if err != nil {
		return "", err
	}

	if apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+apiKey)
	}
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 90 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("API error %s: %s", resp.Status, string(body))
	}

	var result struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", err
	}

	if len(result.Choices) > 0 {
		return result.Choices[0].Message.Content, nil
	}

	return "", fmt.Errorf("no response from AI")
}
func FetchOpenAICompatModels(ctx context.Context, url, apiKey string) ([]string, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, err
	}
	if apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+apiKey)
	}

	client := &http.Client{Timeout: 15 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("models API error: %s", resp.Status)
	}

	var result struct {
		Data []struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	var models []string
	for _, m := range result.Data {
		models = append(models, m.ID)
	}
	return models, nil
}

func FetchOllamaModels(ctx context.Context, baseURL string) ([]string, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", baseURL+"/api/tags", nil)
	if err != nil {
		return nil, err
	}

	client := &http.Client{Timeout: 15 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("Ollama models API error %s: %s", resp.Status, string(body))
	}

	var result struct {
		Models []struct {
			Name string `json:"name"`
		} `json:"models"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	var models []string
	for _, m := range result.Models {
		models = append(models, m.Name)
	}
	return models, nil
}

func FetchGeminiModels(ctx context.Context, apiKey string) ([]string, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", "https://generativelanguage.googleapis.com/v1beta/models", nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("x-goog-api-key", apiKey)

	client := &http.Client{Timeout: 15 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("Gemini models API error %s: %s", resp.Status, string(body))
	}

	var result struct {
		Models []struct {
			Name string `json:"name"`
		} `json:"models"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	var models []string
	prefix := "models/"
	for _, m := range result.Models {
		name := m.Name
		if strings.HasPrefix(name, prefix) {
			name = name[len(prefix):]
		}
		// Only include Gemini-family models; skip legacy PaLM (chat-bison, text-bison, embedding, aqa)
		if strings.HasPrefix(name, "gemini-") {
			models = append(models, name)
		}
	}
	return models, nil
}

func FetchAnthropicModels(ctx context.Context, apiKey string) ([]string, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", "https://api.anthropic.com/v1/models?limit=50", nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("x-api-key", apiKey)
	req.Header.Set("anthropic-version", "2023-06-01")

	client := &http.Client{Timeout: 15 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("Anthropic models API error: %s", resp.Status)
	}

	var result struct {
		Data []struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	var models []string
	for _, m := range result.Data {
		models = append(models, m.ID)
	}
	return models, nil
}

func buildCategoriesPrompt(categories []string) string {
	if len(categories) == 0 {
		categories = []string{"Invoice", "Support", "Urgent", "Newsletter", "Personal", "Business", "Official"}
	}
	return "Categorize this email into one or more of these categories: " + strings.Join(categories, ", ") + ". Return only the category names separated by commas."
}
