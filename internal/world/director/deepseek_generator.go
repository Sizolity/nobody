package director

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

const (
	deepseekDefaultBaseURL = "https://api.deepseek.com"
	deepseekDefaultModel   = "deepseek-v4-pro"
)

type DeepSeekGeneratorConfig struct {
	APIKey  string
	Model   string // defaults to "deepseek-v4-pro"
	BaseURL string // defaults to "https://api.deepseek.com"

	// HTTPClient overrides the default http.Client. Useful for testing.
	HTTPClient *http.Client
}

type DeepSeekGenerator struct {
	apiKey  string
	model   string
	baseURL string
	client  *http.Client
}

func NewDeepSeekGenerator(cfg DeepSeekGeneratorConfig) *DeepSeekGenerator {
	model := cfg.Model
	if model == "" {
		model = deepseekDefaultModel
	}
	baseURL := cfg.BaseURL
	if baseURL == "" {
		baseURL = deepseekDefaultBaseURL
	}
	client := cfg.HTTPClient
	if client == nil {
		client = http.DefaultClient
	}
	return &DeepSeekGenerator{
		apiKey:  cfg.APIKey,
		model:   model,
		baseURL: baseURL,
		client:  client,
	}
}

func (g *DeepSeekGenerator) Generate(ctx context.Context, systemPrompt, userPrompt string) (string, error) {
	messages := make([]chatMessage, 0, 2)
	if systemPrompt != "" {
		messages = append(messages, chatMessage{Role: "system", Content: systemPrompt})
	}
	messages = append(messages, chatMessage{Role: "user", Content: userPrompt})
	return g.doChat(ctx, messages)
}

// GenerateRepair implements ConversationGenerator. It sends a 4-message
// conversation: system → original user → previous assistant response →
// repair instruction, preserving full context for the LLM to self-correct.
func (g *DeepSeekGenerator) GenerateRepair(ctx context.Context, systemPrompt, originalUser, previousAssistant, repairUser string) (string, error) {
	messages := make([]chatMessage, 0, 4)
	if systemPrompt != "" {
		messages = append(messages, chatMessage{Role: "system", Content: systemPrompt})
	}
	messages = append(messages,
		chatMessage{Role: "user", Content: originalUser},
		chatMessage{Role: "assistant", Content: previousAssistant},
		chatMessage{Role: "user", Content: repairUser},
	)
	return g.doChat(ctx, messages)
}

func (g *DeepSeekGenerator) doChat(ctx context.Context, messages []chatMessage) (string, error) {
	reqBody := chatRequest{
		Model:    g.model,
		Messages: messages,
		Stream:   false,
	}
	body, err := json.Marshal(reqBody)
	if err != nil {
		return "", fmt.Errorf("marshal request: %w", err)
	}

	url := g.baseURL + "/chat/completions"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return "", fmt.Errorf("build request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	if g.apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+g.apiKey)
	}

	resp, err := g.client.Do(req)
	if err != nil {
		return "", fmt.Errorf("http request: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("read response: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("deepseek api %d: %s", resp.StatusCode, truncate(respBody, 512))
	}

	var chatResp chatResponse
	if err := json.Unmarshal(respBody, &chatResp); err != nil {
		return "", fmt.Errorf("unmarshal response: %w", err)
	}
	if len(chatResp.Choices) == 0 {
		return "", fmt.Errorf("deepseek api returned 0 choices")
	}
	return chatResp.Choices[0].Message.Content, nil
}

type chatRequest struct {
	Model    string        `json:"model"`
	Messages []chatMessage `json:"messages"`
	Stream   bool          `json:"stream"`
}

type chatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type chatResponse struct {
	Choices []chatChoice `json:"choices"`
}

type chatChoice struct {
	Message chatMessage `json:"message"`
}

func truncate(b []byte, max int) string {
	if len(b) <= max {
		return string(b)
	}
	return string(b[:max]) + "..."
}
