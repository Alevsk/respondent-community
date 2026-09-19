package providers

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/Alevsk/respondent/internal/llm"
)

// ZAIProvider implements llm.Provider using Z.AI's GLM API (OpenAI-compatible).
type ZAIProvider struct {
	apiKey         string
	model          string
	maxTokens      int
	httpClient     HTTPDoer
	baseURL        string
	enableThinking bool
}

// ZAIConfig contains configuration for the Z.AI provider.
type ZAIConfig struct {
	APIKey    string
	Model     string
	MaxTokens int
	BaseURL   string
	// Timeout bounds a single HTTP request. GLM reasoning models can take well
	// over a minute on large enrichment prompts, so this is configurable; it
	// defaults to defaultZAITimeout when zero.
	Timeout time.Duration
	// EnableThinking turns the GLM reasoning phase on. It defaults to off: for
	// schema-constrained structured output (enrichment/analysis) reasoning adds
	// latency and token cost and can consume the max_tokens budget before any
	// content is emitted. Set true only when chain-of-thought is genuinely wanted.
	EnableThinking bool
}

// defaultZAITimeout accommodates GLM reasoning latency on large prompts.
const defaultZAITimeout = 120 * time.Second

// NewZAIProvider creates a new Z.AI provider.
func NewZAIProvider(cfg ZAIConfig) *ZAIProvider {
	if cfg.Model == "" {
		cfg.Model = "glm-4.5-flash"
	}
	if cfg.MaxTokens <= 0 {
		cfg.MaxTokens = 1024
	}
	if cfg.BaseURL == "" {
		cfg.BaseURL = "https://api.z.ai/api/paas/v4"
	}
	if cfg.Timeout <= 0 {
		cfg.Timeout = defaultZAITimeout
	}
	return &ZAIProvider{
		apiKey:         cfg.APIKey,
		model:          cfg.Model,
		maxTokens:      cfg.MaxTokens,
		baseURL:        cfg.BaseURL,
		enableThinking: cfg.EnableThinking,
		httpClient: &http.Client{
			Timeout: cfg.Timeout,
		},
	}
}

// Name returns the provider identifier.
func (p *ZAIProvider) Name() string { return "zai" }

// SupportsProvider returns true for the Z.AI provider type.
func (p *ZAIProvider) SupportsProvider(providerType string) bool {
	return providerType == llm.ProviderTypeZAI
}

// HealthCheck verifies the provider is available.
func (p *ZAIProvider) HealthCheck(ctx context.Context) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, p.baseURL+"/models", nil)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+p.apiKey)
	resp, err := p.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("health check failed: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("health check returned status %d", resp.StatusCode)
	}
	return nil
}

// Complete sends a chat completion request.
func (p *ZAIProvider) Complete(ctx context.Context, req *llm.CompletionRequest) (*llm.CompletionResponse, error) {
	messages := make([]zaiMessage, len(req.Messages))
	for i, m := range req.Messages {
		messages[i] = zaiMessage{Role: m.Role, Content: m.Content}
	}
	maxTokens := p.maxTokens
	if req.MaxTokens > 0 {
		maxTokens = req.MaxTokens
	}
	body := zaiChatRequest{
		Model:     p.model,
		Messages:  messages,
		MaxTokens: maxTokens,
		// Explicitly control GLM reasoning. Disabled by default so structured
		// outputs are emitted directly (no reasoning_tokens burned, no truncation).
		Thinking: &zaiThinking{Type: thinkingType(p.enableThinking)},
	}
	if req.Temperature > 0 {
		body.Temperature = &req.Temperature
	}
	if req.ResponseFormat == llm.ResponseFormatJSON {
		body.ResponseFormat = &zaiResponseFormat{Type: "json_object"}
	}
	bodyBytes, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, p.baseURL+"/chat/completions", bytes.NewReader(bodyBytes))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+p.apiKey)
	resp, err := p.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("API request failed: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()
	respBody, err := io.ReadAll(io.LimitReader(resp.Body, maxLLMResponseBytes))
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}
	if len(respBody) >= maxLLMResponseBytes {
		return nil, fmt.Errorf("response body exceeded %d bytes limit", maxLLMResponseBytes)
	}
	if resp.StatusCode == http.StatusTooManyRequests {
		return nil, llm.ErrRateLimited
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("API returned status %d: %s", resp.StatusCode, string(respBody))
	}
	var chatResp zaiChatResponse
	if err := json.Unmarshal(respBody, &chatResp); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}
	if len(chatResp.Choices) == 0 {
		return nil, llm.ErrInvalidResponse
	}
	return &llm.CompletionResponse{
		Content:      chatResp.Choices[0].Message.Content,
		Model:        chatResp.Model,
		FinishReason: chatResp.Choices[0].FinishReason,
		Usage: llm.Usage{
			PromptTokens:     chatResp.Usage.PromptTokens,
			CompletionTokens: chatResp.Usage.CompletionTokens,
			TotalTokens:      chatResp.Usage.TotalTokens,
		},
	}, nil
}

// Z.AI API types (OpenAI-compatible).
type zaiMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type zaiResponseFormat struct {
	Type string `json:"type"`
}

type zaiChatRequest struct {
	Model          string             `json:"model"`
	Messages       []zaiMessage       `json:"messages"`
	MaxTokens      int                `json:"max_tokens,omitempty"`
	Temperature    *float64           `json:"temperature,omitempty"`
	ResponseFormat *zaiResponseFormat `json:"response_format,omitempty"`
	Thinking       *zaiThinking       `json:"thinking,omitempty"`
}

// zaiThinking controls the GLM reasoning phase: {"type":"enabled"|"disabled"}.
type zaiThinking struct {
	Type string `json:"type"`
}

// thinkingType maps the enable flag to the Z.AI thinking.type value.
func thinkingType(enabled bool) string {
	if enabled {
		return "enabled"
	}
	return "disabled"
}

type zaiChatResponse struct {
	Model   string `json:"model"`
	Choices []struct {
		Message struct {
			Content string `json:"content"`
		} `json:"message"`
		FinishReason string `json:"finish_reason"`
	} `json:"choices"`
	Usage struct {
		PromptTokens     int `json:"prompt_tokens"`
		CompletionTokens int `json:"completion_tokens"`
		TotalTokens      int `json:"total_tokens"`
	} `json:"usage"`
}
