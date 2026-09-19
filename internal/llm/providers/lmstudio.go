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

// LMStudioProvider implements llm.Provider using LM Studio's local API (OpenAI-compatible).
type LMStudioProvider struct {
	baseURL    string
	model      string
	maxTokens  int
	httpClient HTTPDoer
}

// LMStudioConfig contains configuration for the LM Studio provider.
type LMStudioConfig struct {
	BaseURL   string
	Model     string
	MaxTokens int
}

// NewLMStudioProvider creates a new LM Studio provider.
func NewLMStudioProvider(cfg LMStudioConfig) *LMStudioProvider {
	if cfg.BaseURL == "" {
		cfg.BaseURL = "http://localhost:1234/v1"
	}
	if cfg.MaxTokens <= 0 {
		cfg.MaxTokens = 1024
	}

	return &LMStudioProvider{
		baseURL:   cfg.BaseURL,
		model:     cfg.Model, // LM Studio auto-selects if empty.
		maxTokens: cfg.MaxTokens,
		httpClient: &http.Client{
			Timeout: 120 * time.Second, // Local models can be slower.
		},
	}
}

// Name returns the provider identifier.
func (p *LMStudioProvider) Name() string { return "lmstudio" }

// SupportsProvider returns true for LM Studio provider type.
func (p *LMStudioProvider) SupportsProvider(providerType string) bool {
	return providerType == llm.ProviderTypeLMStudio
}

// HealthCheck verifies the provider is available.
func (p *LMStudioProvider) HealthCheck(ctx context.Context) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, p.baseURL+"/models", nil)
	if err != nil {
		return err
	}

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
func (p *LMStudioProvider) Complete(ctx context.Context, req *llm.CompletionRequest) (*llm.CompletionResponse, error) {
	messages := make([]lmstudioMessage, len(req.Messages))
	for i, m := range req.Messages {
		messages[i] = lmstudioMessage{Role: m.Role, Content: m.Content}
	}

	maxTokens := p.maxTokens
	if req.MaxTokens > 0 {
		maxTokens = req.MaxTokens
	}

	body := lmstudioChatRequest{
		Messages:  messages,
		MaxTokens: maxTokens,
	}
	if p.model != "" {
		body.Model = p.model
	}
	if req.Temperature > 0 {
		body.Temperature = &req.Temperature
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

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("API returned status %d: %s", resp.StatusCode, string(respBody))
	}

	var chatResp lmstudioChatResponse
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

// LM Studio API types (OpenAI-compatible).
type lmstudioMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type lmstudioChatRequest struct {
	Model       string            `json:"model,omitempty"`
	Messages    []lmstudioMessage `json:"messages"`
	MaxTokens   int               `json:"max_tokens,omitempty"`
	Temperature *float64          `json:"temperature,omitempty"`
}

type lmstudioChatResponse struct {
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
