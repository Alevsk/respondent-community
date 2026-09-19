package providers

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/Alevsk/respondent/internal/llm"
)

func TestNewOpenAIProvider(t *testing.T) {
	t.Run("default values", func(t *testing.T) {
		provider := NewOpenAIProvider(OpenAIConfig{APIKey: "test-key"})
		assert.Equal(t, "gpt-4o", provider.model)
		assert.Equal(t, 1024, provider.maxTokens)
		assert.Equal(t, "https://api.openai.com/v1", provider.baseURL)
		assert.Equal(t, "test-key", provider.apiKey)
	})

	t.Run("custom values", func(t *testing.T) {
		provider := NewOpenAIProvider(OpenAIConfig{
			APIKey:    "custom-key",
			Model:     "gpt-4-turbo",
			MaxTokens: 2048,
			BaseURL:   "https://custom.api.com/v1",
		})
		assert.Equal(t, "gpt-4-turbo", provider.model)
		assert.Equal(t, 2048, provider.maxTokens)
		assert.Equal(t, "https://custom.api.com/v1", provider.baseURL)
		assert.Equal(t, "custom-key", provider.apiKey)
	})

	t.Run("zero max tokens uses default", func(t *testing.T) {
		provider := NewOpenAIProvider(OpenAIConfig{APIKey: "key", MaxTokens: 0})
		assert.Equal(t, 1024, provider.maxTokens)
	})

	t.Run("negative max tokens uses default", func(t *testing.T) {
		provider := NewOpenAIProvider(OpenAIConfig{APIKey: "key", MaxTokens: -100})
		assert.Equal(t, 1024, provider.maxTokens)
	})

	t.Run("empty model uses default", func(t *testing.T) {
		provider := NewOpenAIProvider(OpenAIConfig{APIKey: "key"})
		assert.Equal(t, "gpt-4o", provider.model)
	})
}

func TestOpenAIProvider_Name(t *testing.T) {
	provider := NewOpenAIProvider(OpenAIConfig{APIKey: "key"})
	assert.Equal(t, "openai", provider.Name())
}

func TestOpenAIProvider_SupportsProvider(t *testing.T) {
	provider := NewOpenAIProvider(OpenAIConfig{APIKey: "key"})

	assert.True(t, provider.SupportsProvider(llm.ProviderTypeOpenAI))
	assert.False(t, provider.SupportsProvider(llm.ProviderTypeAnthropic))
	assert.False(t, provider.SupportsProvider("invalid"))
	assert.False(t, provider.SupportsProvider(""))
}

func TestOpenAIProvider_HealthCheck(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, "/v1/models", r.URL.Path)
			assert.Equal(t, "Bearer test-key", r.Header.Get("Authorization"))
			w.WriteHeader(http.StatusOK)
		}))
		defer server.Close()

		provider := NewOpenAIProvider(OpenAIConfig{
			APIKey:  "test-key",
			BaseURL: server.URL + "/v1",
		})

		err := provider.HealthCheck(context.Background())
		require.NoError(t, err)
	})

	t.Run("unauthorized", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusUnauthorized)
		}))
		defer server.Close()

		provider := NewOpenAIProvider(OpenAIConfig{
			APIKey:  "test-key",
			BaseURL: server.URL + "/v1",
		})

		err := provider.HealthCheck(context.Background())
		require.Error(t, err)
		assert.Contains(t, err.Error(), "status 401")
	})

	t.Run("network error", func(t *testing.T) {
		provider := NewOpenAIProvider(OpenAIConfig{
			APIKey:  "test-key",
			BaseURL: "http://localhost:99999/v1",
		})

		err := provider.HealthCheck(context.Background())
		require.Error(t, err)
	})
}

func TestOpenAIProvider_Complete(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, "/v1/chat/completions", r.URL.Path)
			assert.Equal(t, "application/json", r.Header.Get("Content-Type"))
			assert.Equal(t, "Bearer test-key", r.Header.Get("Authorization"))

			var req openaiChatRequest
			err := json.NewDecoder(r.Body).Decode(&req)
			require.NoError(t, err)
			assert.Equal(t, "gpt-4o", req.Model)
			assert.Len(t, req.Messages, 2)

			resp := `{"model":"gpt-4o","choices":[{"message":{"content":"Hello! How can I help you?"},"finish_reason":"stop"}],"usage":{"prompt_tokens":10,"completion_tokens":8,"total_tokens":18}}`
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(resp))
		}))
		defer server.Close()

		provider := NewOpenAIProvider(OpenAIConfig{
			APIKey:  "test-key",
			Model:   "gpt-4o",
			BaseURL: server.URL + "/v1",
		})

		resp, err := provider.Complete(context.Background(), &llm.CompletionRequest{
			Messages: []llm.Message{
				{Role: "system", Content: "You are helpful"},
				{Role: "user", Content: "Hello"},
			},
		})

		require.NoError(t, err)
		assert.Equal(t, "Hello! How can I help you?", resp.Content)
		assert.Equal(t, "gpt-4o", resp.Model)
		assert.Equal(t, "stop", resp.FinishReason)
		assert.Equal(t, 10, resp.Usage.PromptTokens)
		assert.Equal(t, 8, resp.Usage.CompletionTokens)
		assert.Equal(t, 18, resp.Usage.TotalTokens)
	})

	t.Run("rate limited", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusTooManyRequests)
		}))
		defer server.Close()

		provider := NewOpenAIProvider(OpenAIConfig{
			APIKey:  "test-key",
			BaseURL: server.URL + "/v1",
		})

		resp, err := provider.Complete(context.Background(), &llm.CompletionRequest{
			Messages: []llm.Message{{Role: "user", Content: "Hi"}},
		})

		require.Error(t, err)
		assert.Nil(t, resp)
		assert.ErrorIs(t, err, llm.ErrRateLimited)
	})

	t.Run("server error", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusInternalServerError)
			_, _ = w.Write([]byte("internal server error"))
		}))
		defer server.Close()

		provider := NewOpenAIProvider(OpenAIConfig{
			APIKey:  "test-key",
			BaseURL: server.URL + "/v1",
		})

		resp, err := provider.Complete(context.Background(), &llm.CompletionRequest{
			Messages: []llm.Message{{Role: "user", Content: "Hi"}},
		})

		require.Error(t, err)
		assert.Nil(t, resp)
		assert.Contains(t, err.Error(), "status 500")
	})

	t.Run("invalid json response", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte("invalid json"))
		}))
		defer server.Close()

		provider := NewOpenAIProvider(OpenAIConfig{
			APIKey:  "test-key",
			BaseURL: server.URL + "/v1",
		})

		resp, err := provider.Complete(context.Background(), &llm.CompletionRequest{
			Messages: []llm.Message{{Role: "user", Content: "Hi"}},
		})

		require.Error(t, err)
		assert.Nil(t, resp)
		assert.Contains(t, err.Error(), "failed to decode response")
	})

	t.Run("empty choices", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			resp := `{"model":"gpt-4o","choices":[],"usage":{"prompt_tokens":0,"completion_tokens":0,"total_tokens":0}}`
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(resp))
		}))
		defer server.Close()

		provider := NewOpenAIProvider(OpenAIConfig{
			APIKey:  "test-key",
			BaseURL: server.URL + "/v1",
		})

		resp, err := provider.Complete(context.Background(), &llm.CompletionRequest{
			Messages: []llm.Message{{Role: "user", Content: "Hi"}},
		})

		require.Error(t, err)
		assert.Nil(t, resp)
		assert.ErrorIs(t, err, llm.ErrInvalidResponse)
	})

	t.Run("with custom max tokens", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			var req openaiChatRequest
			_ = json.NewDecoder(r.Body).Decode(&req)
			assert.Equal(t, 500, req.MaxTokens)

			resp := `{"model":"gpt-4o","choices":[{"message":{"content":"ok"},"finish_reason":"stop"}],"usage":{"prompt_tokens":0,"completion_tokens":0,"total_tokens":0}}`
			_, _ = w.Write([]byte(resp))
		}))
		defer server.Close()

		provider := NewOpenAIProvider(OpenAIConfig{
			APIKey:  "test-key",
			BaseURL: server.URL + "/v1",
		})

		_, err := provider.Complete(context.Background(), &llm.CompletionRequest{
			Messages:  []llm.Message{{Role: "user", Content: "Hi"}},
			MaxTokens: 500,
		})
		require.NoError(t, err)
	})

	t.Run("with temperature", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			var req openaiChatRequest
			_ = json.NewDecoder(r.Body).Decode(&req)
			require.NotNil(t, req.Temperature)
			assert.Equal(t, 0.7, *req.Temperature)

			resp := `{"model":"gpt-4o","choices":[{"message":{"content":"ok"},"finish_reason":"stop"}],"usage":{"prompt_tokens":0,"completion_tokens":0,"total_tokens":0}}`
			_, _ = w.Write([]byte(resp))
		}))
		defer server.Close()

		provider := NewOpenAIProvider(OpenAIConfig{
			APIKey:  "test-key",
			BaseURL: server.URL + "/v1",
		})

		_, err := provider.Complete(context.Background(), &llm.CompletionRequest{
			Messages:    []llm.Message{{Role: "user", Content: "Hi"}},
			Temperature: 0.7,
		})
		require.NoError(t, err)
	})

	t.Run("zero temperature not sent", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			var req openaiChatRequest
			_ = json.NewDecoder(r.Body).Decode(&req)
			assert.Nil(t, req.Temperature)

			resp := `{"model":"gpt-4o","choices":[{"message":{"content":"ok"},"finish_reason":"stop"}],"usage":{"prompt_tokens":0,"completion_tokens":0,"total_tokens":0}}`
			_, _ = w.Write([]byte(resp))
		}))
		defer server.Close()

		provider := NewOpenAIProvider(OpenAIConfig{
			APIKey:  "test-key",
			BaseURL: server.URL + "/v1",
		})

		_, err := provider.Complete(context.Background(), &llm.CompletionRequest{
			Messages:    []llm.Message{{Role: "user", Content: "Hi"}},
			Temperature: 0,
		})
		require.NoError(t, err)
	})

	t.Run("network error", func(t *testing.T) {
		provider := NewOpenAIProvider(OpenAIConfig{
			APIKey:  "test-key",
			BaseURL: "http://localhost:99999/v1",
		})

		resp, err := provider.Complete(context.Background(), &llm.CompletionRequest{
			Messages: []llm.Message{{Role: "user", Content: "Hi"}},
		})

		require.Error(t, err)
		assert.Nil(t, resp)
		assert.Contains(t, err.Error(), "API request failed")
	})

	t.Run("json response format", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			var req openaiChatRequest
			err := json.NewDecoder(r.Body).Decode(&req)
			require.NoError(t, err)
			assert.Equal(t, "gpt-4o", req.Model)
			require.NotNil(t, req.ResponseFormat)
			assert.Equal(t, "json_object", req.ResponseFormat.Type)

			resp := `{"model":"gpt-4o","choices":[{"message":{"content":"{\"key\":\"value\"}"},"finish_reason":"stop"}],"usage":{"prompt_tokens":5,"completion_tokens":5,"total_tokens":10}}`
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(resp))
		}))
		defer server.Close()

		provider := NewOpenAIProvider(OpenAIConfig{
			APIKey:  "test-key",
			Model:   "gpt-4o",
			BaseURL: server.URL + "/v1",
		})

		resp, err := provider.Complete(context.Background(), &llm.CompletionRequest{
			Messages:       []llm.Message{{Role: "user", Content: "return JSON"}},
			ResponseFormat: llm.ResponseFormatJSON,
		})

		require.NoError(t, err)
		assert.Equal(t, `{"key":"value"}`, resp.Content)
	})

	t.Run("text response format omits field", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			var raw map[string]any
			err := json.NewDecoder(r.Body).Decode(&raw)
			require.NoError(t, err)
			_, hasRF := raw["response_format"]
			assert.False(t, hasRF, "response_format should be omitted for text mode")

			resp := `{"model":"gpt-4o","choices":[{"message":{"content":"hello"},"finish_reason":"stop"}],"usage":{"prompt_tokens":5,"completion_tokens":5,"total_tokens":10}}`
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(resp))
		}))
		defer server.Close()

		provider := NewOpenAIProvider(OpenAIConfig{
			APIKey:  "test-key",
			Model:   "gpt-4o",
			BaseURL: server.URL + "/v1",
		})

		resp, err := provider.Complete(context.Background(), &llm.CompletionRequest{
			Messages: []llm.Message{{Role: "user", Content: "hello"}},
		})

		require.NoError(t, err)
		assert.Equal(t, "hello", resp.Content)
	})
}

func TestOpenAIProvider_Complete_BadRequest(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"error": "invalid request"}`))
	}))
	defer server.Close()

	provider := NewOpenAIProvider(OpenAIConfig{
		APIKey:  "test-key",
		BaseURL: server.URL + "/v1",
	})

	resp, err := provider.Complete(context.Background(), &llm.CompletionRequest{
		Messages: []llm.Message{{Role: "user", Content: "Hi"}},
	})

	require.Error(t, err)
	assert.Nil(t, resp)
	assert.Contains(t, err.Error(), "400")
}

func TestOpenAIProvider_HTTPClient(t *testing.T) {
	provider := NewOpenAIProvider(OpenAIConfig{APIKey: "key"})
	assert.NotNil(t, provider.httpClient)
}

func TestAnthropicProvider_New(t *testing.T) {
	t.Run("default values", func(t *testing.T) {
		provider := NewAnthropicProvider(AnthropicConfig{APIKey: "key"})
		assert.Equal(t, "claude-sonnet-4-20250514", provider.model)
		assert.Equal(t, 1024, provider.maxTokens)
		assert.Equal(t, "https://api.anthropic.com", provider.baseURL)
	})

	t.Run("custom values", func(t *testing.T) {
		provider := NewAnthropicProvider(AnthropicConfig{
			APIKey:    "key",
			Model:     "claude-3-opus",
			MaxTokens: 2048,
			BaseURL:   "https://custom.anthropic.com",
		})
		assert.Equal(t, "claude-3-opus", provider.model)
		assert.Equal(t, 2048, provider.maxTokens)
		assert.Equal(t, "https://custom.anthropic.com", provider.baseURL)
	})
}

func TestAnthropicProvider_Name(t *testing.T) {
	provider := NewAnthropicProvider(AnthropicConfig{APIKey: "key"})
	assert.Equal(t, "anthropic", provider.Name())
}

func TestAnthropicProvider_SupportsProvider(t *testing.T) {
	provider := NewAnthropicProvider(AnthropicConfig{APIKey: "key"})
	assert.True(t, provider.SupportsProvider(llm.ProviderTypeAnthropic))
	assert.False(t, provider.SupportsProvider(llm.ProviderTypeOpenAI))
}

func TestAnthropicProvider_HealthCheck(t *testing.T) {
	t.Run("healthy", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, "/v1/messages", r.URL.Path)
			assert.Equal(t, "test-key", r.Header.Get("x-api-key"))
			assert.Equal(t, "2023-06-01", r.Header.Get("anthropic-version"))
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"data":[]}`))
		}))
		defer server.Close()

		provider := NewAnthropicProvider(AnthropicConfig{APIKey: "test-key", BaseURL: server.URL})
		err := provider.HealthCheck(context.Background())
		require.NoError(t, err)
	})

	t.Run("missing api key", func(t *testing.T) {
		provider := NewAnthropicProvider(AnthropicConfig{APIKey: ""})
		err := provider.HealthCheck(context.Background())
		require.Error(t, err)
		assert.Contains(t, err.Error(), "API key not configured")
	})

	t.Run("unauthorized", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusUnauthorized)
		}))
		defer server.Close()

		provider := NewAnthropicProvider(AnthropicConfig{APIKey: "bad-key", BaseURL: server.URL})
		err := provider.HealthCheck(context.Background())
		require.Error(t, err)
		assert.Contains(t, err.Error(), "invalid API key")
	})

	t.Run("rate limited", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusTooManyRequests)
		}))
		defer server.Close()

		provider := NewAnthropicProvider(AnthropicConfig{APIKey: "test-key", BaseURL: server.URL})
		err := provider.HealthCheck(context.Background())
		require.Error(t, err)
		assert.ErrorIs(t, err, llm.ErrRateLimited)
	})
}

func TestAnthropicProvider_Complete(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, "/v1/messages", r.URL.Path)
			assert.Equal(t, "application/json", r.Header.Get("Content-Type"))
			assert.Equal(t, "test-key", r.Header.Get("x-api-key"))
			assert.Equal(t, "2023-06-01", r.Header.Get("anthropic-version"))

			var req anthropicRequest
			err := json.NewDecoder(r.Body).Decode(&req)
			require.NoError(t, err)
			assert.Equal(t, "claude-sonnet-4-20250514", req.Model)
			assert.Equal(t, "You are helpful", req.System)

			resp := `{"model":"claude-sonnet-4-20250514","stop_reason":"end_turn","content":[{"type":"text","text":"Hello! How can I help?"}],"usage":{"input_tokens":15,"output_tokens":10}}`
			_, _ = w.Write([]byte(resp))
		}))
		defer server.Close()

		provider := NewAnthropicProvider(AnthropicConfig{
			APIKey:  "test-key",
			BaseURL: server.URL,
		})

		resp, err := provider.Complete(context.Background(), &llm.CompletionRequest{
			Messages: []llm.Message{
				{Role: "system", Content: "You are helpful"},
				{Role: "user", Content: "Hello"},
			},
		})

		require.NoError(t, err)
		assert.Equal(t, "Hello! How can I help?", resp.Content)
		assert.Equal(t, 25, resp.Usage.TotalTokens)
	})

	t.Run("rate limited", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusTooManyRequests)
		}))
		defer server.Close()

		provider := NewAnthropicProvider(AnthropicConfig{
			APIKey:  "test-key",
			BaseURL: server.URL,
		})

		resp, err := provider.Complete(context.Background(), &llm.CompletionRequest{
			Messages: []llm.Message{{Role: "user", Content: "Hi"}},
		})

		require.Error(t, err)
		assert.ErrorIs(t, err, llm.ErrRateLimited)
		assert.Nil(t, resp)
	})

	t.Run("empty content", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			resp := `{"model":"claude-sonnet-4-20250514","stop_reason":"end_turn","content":[]}`
			_, _ = w.Write([]byte(resp))
		}))
		defer server.Close()

		provider := NewAnthropicProvider(AnthropicConfig{
			APIKey:  "test-key",
			BaseURL: server.URL,
		})

		resp, err := provider.Complete(context.Background(), &llm.CompletionRequest{
			Messages: []llm.Message{{Role: "user", Content: "Hi"}},
		})

		require.Error(t, err)
		assert.ErrorIs(t, err, llm.ErrInvalidResponse)
		assert.Nil(t, resp)
	})

	t.Run("non-text content block", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			resp := `{"model":"claude-sonnet-4-20250514","stop_reason":"end_turn","content":[{"type":"image","text":""},{"type":"text","text":"Here is the text"}]}`
			_, _ = w.Write([]byte(resp))
		}))
		defer server.Close()

		provider := NewAnthropicProvider(AnthropicConfig{
			APIKey:  "test-key",
			BaseURL: server.URL,
		})

		resp, err := provider.Complete(context.Background(), &llm.CompletionRequest{
			Messages: []llm.Message{{Role: "user", Content: "Hi"}},
		})

		require.NoError(t, err)
		assert.Equal(t, "Here is the text", resp.Content)
	})

	t.Run("with temperature", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			var req anthropicRequest
			_ = json.NewDecoder(r.Body).Decode(&req)
			require.NotNil(t, req.Temperature)
			assert.Equal(t, 0.5, *req.Temperature)

			resp := `{"model":"claude-sonnet-4-20250514","stop_reason":"end_turn","content":[{"type":"text","text":"ok"}]}`
			_, _ = w.Write([]byte(resp))
		}))
		defer server.Close()

		provider := NewAnthropicProvider(AnthropicConfig{
			APIKey:  "test-key",
			BaseURL: server.URL,
		})

		_, err := provider.Complete(context.Background(), &llm.CompletionRequest{
			Messages:    []llm.Message{{Role: "user", Content: "Hi"}},
			Temperature: 0.5,
		})
		require.NoError(t, err)
	})

	t.Run("with custom max tokens", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			var req anthropicRequest
			_ = json.NewDecoder(r.Body).Decode(&req)
			assert.Equal(t, 500, req.MaxTokens)

			resp := `{"model":"claude-sonnet-4-20250514","stop_reason":"end_turn","content":[{"type":"text","text":"ok"}]}`
			_, _ = w.Write([]byte(resp))
		}))
		defer server.Close()

		provider := NewAnthropicProvider(AnthropicConfig{
			APIKey:  "test-key",
			BaseURL: server.URL,
		})

		_, err := provider.Complete(context.Background(), &llm.CompletionRequest{
			Messages:  []llm.Message{{Role: "user", Content: "Hi"}},
			MaxTokens: 500,
		})
		require.NoError(t, err)
	})

	t.Run("server error", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusInternalServerError)
			_, _ = w.Write([]byte("error"))
		}))
		defer server.Close()

		provider := NewAnthropicProvider(AnthropicConfig{
			APIKey:  "test-key",
			BaseURL: server.URL,
		})

		resp, err := provider.Complete(context.Background(), &llm.CompletionRequest{
			Messages: []llm.Message{{Role: "user", Content: "Hi"}},
		})

		require.Error(t, err)
		assert.Nil(t, resp)
	})

	t.Run("invalid json response", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_, _ = w.Write([]byte("invalid"))
		}))
		defer server.Close()

		provider := NewAnthropicProvider(AnthropicConfig{
			APIKey:  "test-key",
			BaseURL: server.URL,
		})

		resp, err := provider.Complete(context.Background(), &llm.CompletionRequest{
			Messages: []llm.Message{{Role: "user", Content: "Hi"}},
		})

		require.Error(t, err)
		assert.Nil(t, resp)
	})

	t.Run("network error", func(t *testing.T) {
		provider := NewAnthropicProvider(AnthropicConfig{
			APIKey:  "test-key",
			BaseURL: "http://localhost:99999",
		})

		resp, err := provider.Complete(context.Background(), &llm.CompletionRequest{
			Messages: []llm.Message{{Role: "user", Content: "Hi"}},
		})

		require.Error(t, err)
		assert.Nil(t, resp)
	})
}

func TestGeminiProvider_New(t *testing.T) {
	t.Run("default values", func(t *testing.T) {
		provider := NewGeminiProvider(GeminiConfig{APIKey: "key"})
		assert.Equal(t, "gemini-2.5-flash", provider.model)
		assert.Equal(t, "https://generativelanguage.googleapis.com/v1beta", provider.baseURL)
	})

	t.Run("custom values", func(t *testing.T) {
		provider := NewGeminiProvider(GeminiConfig{
			APIKey:  "key",
			Model:   "gemini-pro",
			BaseURL: "https://custom.gemini.com",
		})
		assert.Equal(t, "gemini-pro", provider.model)
		assert.Equal(t, "https://custom.gemini.com", provider.baseURL)
	})
}

func TestGeminiProvider_Name(t *testing.T) {
	provider := NewGeminiProvider(GeminiConfig{APIKey: "key"})
	assert.Equal(t, "gemini", provider.Name())
}

func TestGeminiProvider_SupportsProvider(t *testing.T) {
	provider := NewGeminiProvider(GeminiConfig{APIKey: "key"})
	assert.True(t, provider.SupportsProvider(llm.ProviderTypeGemini))
	assert.False(t, provider.SupportsProvider(llm.ProviderTypeOpenAI))
}

func TestGeminiProvider_HealthCheck(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.True(t, strings.HasSuffix(r.URL.Path, "/models"))
			assert.Equal(t, "test-key", r.Header.Get("x-goog-api-key"))
			w.WriteHeader(http.StatusOK)
		}))
		defer server.Close()

		provider := NewGeminiProvider(GeminiConfig{
			APIKey:  "test-key",
			BaseURL: server.URL,
		})

		err := provider.HealthCheck(context.Background())
		require.NoError(t, err)
	})

	t.Run("error", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusUnauthorized)
		}))
		defer server.Close()

		provider := NewGeminiProvider(GeminiConfig{
			APIKey:  "test-key",
			BaseURL: server.URL,
		})

		err := provider.HealthCheck(context.Background())
		require.Error(t, err)
	})
}

func TestGeminiProvider_Complete(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Contains(t, r.URL.Path, "generateContent")
			assert.Equal(t, "test-key", r.Header.Get("x-goog-api-key"))

			var req geminiRequest
			err := json.NewDecoder(r.Body).Decode(&req)
			require.NoError(t, err)
			assert.Len(t, req.Contents, 1)

			resp := `{"candidates":[{"content":{"parts":[{"text":"Hello from Gemini"}]},"finishReason":"STOP"}],"usageMetadata":{"promptTokenCount":10,"candidatesTokenCount":5,"totalTokenCount":15}}`
			_, _ = w.Write([]byte(resp))
		}))
		defer server.Close()

		provider := NewGeminiProvider(GeminiConfig{
			APIKey:  "test-key",
			Model:   "gemini-2.5-flash",
			BaseURL: server.URL,
		})

		resp, err := provider.Complete(context.Background(), &llm.CompletionRequest{
			Messages: []llm.Message{
				{Role: "user", Content: "Hello"},
			},
		})

		require.NoError(t, err)
		assert.Equal(t, "Hello from Gemini", resp.Content)
		assert.Equal(t, 15, resp.Usage.TotalTokens)
	})

	t.Run("with system message", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			var req geminiRequest
			_ = json.NewDecoder(r.Body).Decode(&req)
			require.Len(t, req.Contents, 1)
			assert.Contains(t, req.Contents[0].Parts[0].Text, "System instruction")

			resp := `{"candidates":[{"content":{"parts":[{"text":"ok"}]},"finishReason":"STOP"}],"usageMetadata":{"promptTokenCount":0,"candidatesTokenCount":0,"totalTokenCount":0}}`
			_, _ = w.Write([]byte(resp))
		}))
		defer server.Close()

		provider := NewGeminiProvider(GeminiConfig{
			APIKey:  "test-key",
			BaseURL: server.URL,
		})

		_, err := provider.Complete(context.Background(), &llm.CompletionRequest{
			Messages: []llm.Message{
				{Role: "system", Content: "System instruction"},
				{Role: "user", Content: "Hello"},
			},
		})
		require.NoError(t, err)
	})

	t.Run("assistant role converts to model", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			var req geminiRequest
			_ = json.NewDecoder(r.Body).Decode(&req)
			assert.Equal(t, "model", req.Contents[1].Role)

			resp := `{"candidates":[{"content":{"parts":[{"text":"ok"}]},"finishReason":"STOP"}],"usageMetadata":{"promptTokenCount":0,"candidatesTokenCount":0,"totalTokenCount":0}}`
			_, _ = w.Write([]byte(resp))
		}))
		defer server.Close()

		provider := NewGeminiProvider(GeminiConfig{
			APIKey:  "test-key",
			BaseURL: server.URL,
		})

		_, err := provider.Complete(context.Background(), &llm.CompletionRequest{
			Messages: []llm.Message{
				{Role: "user", Content: "Hello"},
				{Role: "assistant", Content: "Hi there"},
				{Role: "user", Content: "How are you?"},
			},
		})
		require.NoError(t, err)
	})

	t.Run("rate limited", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusTooManyRequests)
		}))
		defer server.Close()

		provider := NewGeminiProvider(GeminiConfig{
			APIKey:  "test-key",
			BaseURL: server.URL,
		})

		resp, err := provider.Complete(context.Background(), &llm.CompletionRequest{
			Messages: []llm.Message{{Role: "user", Content: "Hi"}},
		})

		require.Error(t, err)
		assert.ErrorIs(t, err, llm.ErrRateLimited)
		assert.Nil(t, resp)
	})

	t.Run("empty candidates", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			resp := `{"candidates":[],"usageMetadata":{"promptTokenCount":0,"candidatesTokenCount":0,"totalTokenCount":0}}`
			_, _ = w.Write([]byte(resp))
		}))
		defer server.Close()

		provider := NewGeminiProvider(GeminiConfig{
			APIKey:  "test-key",
			BaseURL: server.URL,
		})

		resp, err := provider.Complete(context.Background(), &llm.CompletionRequest{
			Messages: []llm.Message{{Role: "user", Content: "Hi"}},
		})

		require.Error(t, err)
		assert.ErrorIs(t, err, llm.ErrInvalidResponse)
		assert.Nil(t, resp)
	})

	t.Run("empty parts", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			resp := `{"candidates":[{"content":{"parts":[]},"finishReason":"STOP"}],"usageMetadata":{"promptTokenCount":0,"candidatesTokenCount":0,"totalTokenCount":0}}`
			_, _ = w.Write([]byte(resp))
		}))
		defer server.Close()

		provider := NewGeminiProvider(GeminiConfig{
			APIKey:  "test-key",
			BaseURL: server.URL,
		})

		resp, err := provider.Complete(context.Background(), &llm.CompletionRequest{
			Messages: []llm.Message{{Role: "user", Content: "Hi"}},
		})

		require.Error(t, err)
		assert.ErrorIs(t, err, llm.ErrInvalidResponse)
		assert.Nil(t, resp)
	})

	t.Run("server error", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusInternalServerError)
			_, _ = w.Write([]byte("error"))
		}))
		defer server.Close()

		provider := NewGeminiProvider(GeminiConfig{
			APIKey:  "test-key",
			BaseURL: server.URL,
		})

		resp, err := provider.Complete(context.Background(), &llm.CompletionRequest{
			Messages: []llm.Message{{Role: "user", Content: "Hi"}},
		})

		require.Error(t, err)
		assert.Nil(t, resp)
	})

	t.Run("network error", func(t *testing.T) {
		provider := NewGeminiProvider(GeminiConfig{
			APIKey:  "test-key",
			BaseURL: "http://localhost:99999",
		})

		resp, err := provider.Complete(context.Background(), &llm.CompletionRequest{
			Messages: []llm.Message{{Role: "user", Content: "Hi"}},
		})

		require.Error(t, err)
		assert.Nil(t, resp)
	})

	t.Run("json response format", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			var req geminiRequest
			err := json.NewDecoder(r.Body).Decode(&req)
			require.NoError(t, err)
			require.NotNil(t, req.GenerationConfig)
			assert.Equal(t, "application/json", req.GenerationConfig.ResponseMimeType)

			resp := `{"candidates":[{"content":{"parts":[{"text":"{\"ok\":true}"}]},"finishReason":"STOP"}],"usageMetadata":{"promptTokenCount":5,"candidatesTokenCount":3,"totalTokenCount":8}}`
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(resp))
		}))
		defer server.Close()

		provider := NewGeminiProvider(GeminiConfig{
			APIKey:  "test-key",
			BaseURL: server.URL,
		})

		resp, err := provider.Complete(context.Background(), &llm.CompletionRequest{
			Messages:       []llm.Message{{Role: "user", Content: "return JSON"}},
			ResponseFormat: llm.ResponseFormatJSON,
		})

		require.NoError(t, err)
		assert.Equal(t, `{"ok":true}`, resp.Content)
	})

	t.Run("text response format omits generation config", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			var raw map[string]any
			err := json.NewDecoder(r.Body).Decode(&raw)
			require.NoError(t, err)
			_, hasGC := raw["generationConfig"]
			assert.False(t, hasGC, "generationConfig should be omitted for text mode")

			resp := `{"candidates":[{"content":{"parts":[{"text":"hello"}]},"finishReason":"STOP"}],"usageMetadata":{"promptTokenCount":5,"candidatesTokenCount":1,"totalTokenCount":6}}`
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(resp))
		}))
		defer server.Close()

		provider := NewGeminiProvider(GeminiConfig{
			APIKey:  "test-key",
			BaseURL: server.URL,
		})

		resp, err := provider.Complete(context.Background(), &llm.CompletionRequest{
			Messages: []llm.Message{{Role: "user", Content: "hello"}},
		})

		require.NoError(t, err)
		assert.Equal(t, "hello", resp.Content)
	})
}

func TestXAIProvider_New(t *testing.T) {
	t.Run("default values", func(t *testing.T) {
		provider := NewXAIProvider(XAIConfig{APIKey: "key"})
		assert.Equal(t, "grok-3", provider.model)
		assert.Equal(t, 1024, provider.maxTokens)
		assert.Equal(t, "https://api.x.ai/v1", provider.baseURL)
	})

	t.Run("custom values", func(t *testing.T) {
		provider := NewXAIProvider(XAIConfig{
			APIKey:    "key",
			Model:     "grok-2",
			MaxTokens: 2048,
			BaseURL:   "https://custom.x.ai/v1",
		})
		assert.Equal(t, "grok-2", provider.model)
		assert.Equal(t, 2048, provider.maxTokens)
		assert.Equal(t, "https://custom.x.ai/v1", provider.baseURL)
	})
}

func TestXAIProvider_Name(t *testing.T) {
	provider := NewXAIProvider(XAIConfig{APIKey: "key"})
	assert.Equal(t, "xai", provider.Name())
}

func TestXAIProvider_SupportsProvider(t *testing.T) {
	provider := NewXAIProvider(XAIConfig{APIKey: "key"})
	assert.True(t, provider.SupportsProvider(llm.ProviderTypeXAI))
	assert.False(t, provider.SupportsProvider(llm.ProviderTypeOpenAI))
}

func TestXAIProvider_HealthCheck(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, "/v1/models", r.URL.Path)
			assert.Equal(t, "Bearer test-key", r.Header.Get("Authorization"))
			w.WriteHeader(http.StatusOK)
		}))
		defer server.Close()

		provider := NewXAIProvider(XAIConfig{
			APIKey:  "test-key",
			BaseURL: server.URL + "/v1",
		})

		err := provider.HealthCheck(context.Background())
		require.NoError(t, err)
	})

	t.Run("error", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusUnauthorized)
		}))
		defer server.Close()

		provider := NewXAIProvider(XAIConfig{
			APIKey:  "test-key",
			BaseURL: server.URL + "/v1",
		})

		err := provider.HealthCheck(context.Background())
		require.Error(t, err)
	})
}

func TestXAIProvider_Complete(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, "/v1/chat/completions", r.URL.Path)

			resp := `{"model":"grok-3","choices":[{"message":{"content":"Hello from Grok"},"finish_reason":"stop"}],"usage":{"prompt_tokens":10,"completion_tokens":5,"total_tokens":15}}`
			_, _ = w.Write([]byte(resp))
		}))
		defer server.Close()

		provider := NewXAIProvider(XAIConfig{
			APIKey:  "test-key",
			BaseURL: server.URL + "/v1",
		})

		resp, err := provider.Complete(context.Background(), &llm.CompletionRequest{
			Messages: []llm.Message{{Role: "user", Content: "Hello"}},
		})

		require.NoError(t, err)
		assert.Equal(t, "Hello from Grok", resp.Content)
	})

	t.Run("rate limited", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusTooManyRequests)
		}))
		defer server.Close()

		provider := NewXAIProvider(XAIConfig{
			APIKey:  "test-key",
			BaseURL: server.URL + "/v1",
		})

		resp, err := provider.Complete(context.Background(), &llm.CompletionRequest{
			Messages: []llm.Message{{Role: "user", Content: "Hi"}},
		})

		require.Error(t, err)
		assert.ErrorIs(t, err, llm.ErrRateLimited)
		assert.Nil(t, resp)
	})

	t.Run("empty choices", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			resp := `{"model":"grok-3","choices":[],"usage":{"prompt_tokens":0,"completion_tokens":0,"total_tokens":0}}`
			_, _ = w.Write([]byte(resp))
		}))
		defer server.Close()

		provider := NewXAIProvider(XAIConfig{
			APIKey:  "test-key",
			BaseURL: server.URL + "/v1",
		})

		resp, err := provider.Complete(context.Background(), &llm.CompletionRequest{
			Messages: []llm.Message{{Role: "user", Content: "Hi"}},
		})

		require.Error(t, err)
		assert.ErrorIs(t, err, llm.ErrInvalidResponse)
		assert.Nil(t, resp)
	})

	t.Run("with temperature", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			var req xaiChatRequest
			_ = json.NewDecoder(r.Body).Decode(&req)
			require.NotNil(t, req.Temperature)
			assert.Equal(t, 0.8, *req.Temperature)

			resp := `{"model":"grok-3","choices":[{"message":{"content":"ok"},"finish_reason":"stop"}],"usage":{"prompt_tokens":0,"completion_tokens":0,"total_tokens":0}}`
			_, _ = w.Write([]byte(resp))
		}))
		defer server.Close()

		provider := NewXAIProvider(XAIConfig{
			APIKey:  "test-key",
			BaseURL: server.URL + "/v1",
		})

		_, err := provider.Complete(context.Background(), &llm.CompletionRequest{
			Messages:    []llm.Message{{Role: "user", Content: "Hi"}},
			Temperature: 0.8,
		})
		require.NoError(t, err)
	})

	t.Run("server error", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusInternalServerError)
			_, _ = w.Write([]byte("error"))
		}))
		defer server.Close()

		provider := NewXAIProvider(XAIConfig{
			APIKey:  "test-key",
			BaseURL: server.URL + "/v1",
		})

		resp, err := provider.Complete(context.Background(), &llm.CompletionRequest{
			Messages: []llm.Message{{Role: "user", Content: "Hi"}},
		})

		require.Error(t, err)
		assert.Nil(t, resp)
	})

	t.Run("network error", func(t *testing.T) {
		provider := NewXAIProvider(XAIConfig{
			APIKey:  "test-key",
			BaseURL: "http://localhost:99999/v1",
		})

		resp, err := provider.Complete(context.Background(), &llm.CompletionRequest{
			Messages: []llm.Message{{Role: "user", Content: "Hi"}},
		})

		require.Error(t, err)
		assert.Nil(t, resp)
	})

	t.Run("json response format", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			var req xaiChatRequest
			err := json.NewDecoder(r.Body).Decode(&req)
			require.NoError(t, err)
			require.NotNil(t, req.ResponseFormat)
			assert.Equal(t, "json_object", req.ResponseFormat.Type)

			resp := `{"model":"grok-2","choices":[{"message":{"content":"{\"ok\":true}"},"finish_reason":"stop"}],"usage":{"prompt_tokens":5,"completion_tokens":3,"total_tokens":8}}`
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(resp))
		}))
		defer server.Close()

		provider := NewXAIProvider(XAIConfig{
			APIKey:  "test-key",
			BaseURL: server.URL + "/v1",
		})

		resp, err := provider.Complete(context.Background(), &llm.CompletionRequest{
			Messages:       []llm.Message{{Role: "user", Content: "return JSON"}},
			ResponseFormat: llm.ResponseFormatJSON,
		})

		require.NoError(t, err)
		assert.Equal(t, `{"ok":true}`, resp.Content)
	})

	t.Run("text response format omits field", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			var raw map[string]any
			err := json.NewDecoder(r.Body).Decode(&raw)
			require.NoError(t, err)
			_, hasRF := raw["response_format"]
			assert.False(t, hasRF, "response_format should be omitted for text mode")

			resp := `{"model":"grok-2","choices":[{"message":{"content":"hi"},"finish_reason":"stop"}],"usage":{"prompt_tokens":5,"completion_tokens":1,"total_tokens":6}}`
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(resp))
		}))
		defer server.Close()

		provider := NewXAIProvider(XAIConfig{
			APIKey:  "test-key",
			BaseURL: server.URL + "/v1",
		})

		resp, err := provider.Complete(context.Background(), &llm.CompletionRequest{
			Messages: []llm.Message{{Role: "user", Content: "hi"}},
		})

		require.NoError(t, err)
		assert.Equal(t, "hi", resp.Content)
	})
}

func TestZAIProvider_New(t *testing.T) {
	t.Run("default values", func(t *testing.T) {
		provider := NewZAIProvider(ZAIConfig{APIKey: "key"})
		assert.Equal(t, "glm-4.5-flash", provider.model)
		assert.Equal(t, 1024, provider.maxTokens)
		assert.Equal(t, "https://api.z.ai/api/paas/v4", provider.baseURL)
	})

	t.Run("custom values", func(t *testing.T) {
		provider := NewZAIProvider(ZAIConfig{
			APIKey:    "key",
			Model:     "glm-4.5",
			MaxTokens: 2048,
			BaseURL:   "https://custom.z.ai/v4",
		})
		assert.Equal(t, "glm-4.5", provider.model)
		assert.Equal(t, 2048, provider.maxTokens)
		assert.Equal(t, "https://custom.z.ai/v4", provider.baseURL)
	})
}

func TestZAIProvider_Name(t *testing.T) {
	provider := NewZAIProvider(ZAIConfig{APIKey: "key"})
	assert.Equal(t, "zai", provider.Name())
}

func TestZAIProvider_SupportsProvider(t *testing.T) {
	provider := NewZAIProvider(ZAIConfig{APIKey: "key"})
	assert.True(t, provider.SupportsProvider(llm.ProviderTypeZAI))
	assert.False(t, provider.SupportsProvider(llm.ProviderTypeOpenAI))
}

func TestZAIProvider_HealthCheck(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, "/v4/models", r.URL.Path)
			assert.Equal(t, "Bearer test-key", r.Header.Get("Authorization"))
			w.WriteHeader(http.StatusOK)
		}))
		defer server.Close()

		provider := NewZAIProvider(ZAIConfig{
			APIKey:  "test-key",
			BaseURL: server.URL + "/v4",
		})

		err := provider.HealthCheck(context.Background())
		require.NoError(t, err)
	})

	t.Run("error", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusUnauthorized)
		}))
		defer server.Close()

		provider := NewZAIProvider(ZAIConfig{
			APIKey:  "test-key",
			BaseURL: server.URL + "/v4",
		})

		err := provider.HealthCheck(context.Background())
		require.Error(t, err)
	})
}

func TestZAIProvider_Complete(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, "/v4/chat/completions", r.URL.Path)

			resp := `{"model":"glm-4.5-flash","choices":[{"message":{"content":"Hello from GLM"},"finish_reason":"stop"}],"usage":{"prompt_tokens":10,"completion_tokens":5,"total_tokens":15}}`
			_, _ = w.Write([]byte(resp))
		}))
		defer server.Close()

		provider := NewZAIProvider(ZAIConfig{
			APIKey:  "test-key",
			BaseURL: server.URL + "/v4",
		})

		resp, err := provider.Complete(context.Background(), &llm.CompletionRequest{
			Messages: []llm.Message{{Role: "user", Content: "Hello"}},
		})

		require.NoError(t, err)
		assert.Equal(t, "Hello from GLM", resp.Content)
	})

	t.Run("rate limited", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusTooManyRequests)
		}))
		defer server.Close()

		provider := NewZAIProvider(ZAIConfig{
			APIKey:  "test-key",
			BaseURL: server.URL + "/v4",
		})

		resp, err := provider.Complete(context.Background(), &llm.CompletionRequest{
			Messages: []llm.Message{{Role: "user", Content: "Hi"}},
		})

		require.Error(t, err)
		assert.ErrorIs(t, err, llm.ErrRateLimited)
		assert.Nil(t, resp)
	})

	t.Run("empty choices", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			resp := `{"model":"glm-4.5-flash","choices":[],"usage":{"prompt_tokens":0,"completion_tokens":0,"total_tokens":0}}`
			_, _ = w.Write([]byte(resp))
		}))
		defer server.Close()

		provider := NewZAIProvider(ZAIConfig{
			APIKey:  "test-key",
			BaseURL: server.URL + "/v4",
		})

		resp, err := provider.Complete(context.Background(), &llm.CompletionRequest{
			Messages: []llm.Message{{Role: "user", Content: "Hi"}},
		})

		require.Error(t, err)
		assert.ErrorIs(t, err, llm.ErrInvalidResponse)
		assert.Nil(t, resp)
	})

	t.Run("with temperature", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			var req zaiChatRequest
			_ = json.NewDecoder(r.Body).Decode(&req)
			require.NotNil(t, req.Temperature)
			assert.Equal(t, 0.8, *req.Temperature)

			resp := `{"model":"glm-4.5-flash","choices":[{"message":{"content":"ok"},"finish_reason":"stop"}],"usage":{"prompt_tokens":0,"completion_tokens":0,"total_tokens":0}}`
			_, _ = w.Write([]byte(resp))
		}))
		defer server.Close()

		provider := NewZAIProvider(ZAIConfig{
			APIKey:  "test-key",
			BaseURL: server.URL + "/v4",
		})

		_, err := provider.Complete(context.Background(), &llm.CompletionRequest{
			Messages:    []llm.Message{{Role: "user", Content: "Hi"}},
			Temperature: 0.8,
		})
		require.NoError(t, err)
	})

	t.Run("server error", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusInternalServerError)
			_, _ = w.Write([]byte("error"))
		}))
		defer server.Close()

		provider := NewZAIProvider(ZAIConfig{
			APIKey:  "test-key",
			BaseURL: server.URL + "/v4",
		})

		resp, err := provider.Complete(context.Background(), &llm.CompletionRequest{
			Messages: []llm.Message{{Role: "user", Content: "Hi"}},
		})

		require.Error(t, err)
		assert.Nil(t, resp)
	})

	t.Run("network error", func(t *testing.T) {
		provider := NewZAIProvider(ZAIConfig{
			APIKey:  "test-key",
			BaseURL: "http://localhost:99999/api/paas/v4",
		})

		resp, err := provider.Complete(context.Background(), &llm.CompletionRequest{
			Messages: []llm.Message{{Role: "user", Content: "Hi"}},
		})

		require.Error(t, err)
		assert.Nil(t, resp)
	})

	t.Run("json response format", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			var req zaiChatRequest
			err := json.NewDecoder(r.Body).Decode(&req)
			require.NoError(t, err)
			require.NotNil(t, req.ResponseFormat)
			assert.Equal(t, "json_object", req.ResponseFormat.Type)

			resp := `{"model":"glm-4.5","choices":[{"message":{"content":"{\"ok\":true}"},"finish_reason":"stop"}],"usage":{"prompt_tokens":5,"completion_tokens":3,"total_tokens":8}}`
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(resp))
		}))
		defer server.Close()

		provider := NewZAIProvider(ZAIConfig{
			APIKey:  "test-key",
			BaseURL: server.URL + "/v4",
		})

		resp, err := provider.Complete(context.Background(), &llm.CompletionRequest{
			Messages:       []llm.Message{{Role: "user", Content: "return JSON"}},
			ResponseFormat: llm.ResponseFormatJSON,
		})

		require.NoError(t, err)
		assert.Equal(t, `{"ok":true}`, resp.Content)
	})

	t.Run("text response format omits field", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			var raw map[string]any
			err := json.NewDecoder(r.Body).Decode(&raw)
			require.NoError(t, err)
			_, hasRF := raw["response_format"]
			assert.False(t, hasRF, "response_format should be omitted for text mode")

			resp := `{"model":"glm-4.5","choices":[{"message":{"content":"hi"},"finish_reason":"stop"}],"usage":{"prompt_tokens":5,"completion_tokens":1,"total_tokens":6}}`
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(resp))
		}))
		defer server.Close()

		provider := NewZAIProvider(ZAIConfig{
			APIKey:  "test-key",
			BaseURL: server.URL + "/v4",
		})

		resp, err := provider.Complete(context.Background(), &llm.CompletionRequest{
			Messages: []llm.Message{{Role: "user", Content: "hi"}},
		})

		require.NoError(t, err)
		assert.Equal(t, "hi", resp.Content)
	})
}

func TestOllamaProvider_New(t *testing.T) {
	t.Run("default values", func(t *testing.T) {
		provider := NewOllamaProvider(OllamaConfig{})
		assert.Equal(t, "llama3", provider.model)
		assert.Equal(t, "http://localhost:11434", provider.baseURL)
	})

	t.Run("custom values", func(t *testing.T) {
		provider := NewOllamaProvider(OllamaConfig{
			BaseURL: "http://custom:11434",
			Model:   "llama2",
		})
		assert.Equal(t, "llama2", provider.model)
		assert.Equal(t, "http://custom:11434", provider.baseURL)
	})
}

func TestOllamaProvider_Name(t *testing.T) {
	provider := NewOllamaProvider(OllamaConfig{})
	assert.Equal(t, "ollama", provider.Name())
}

func TestOllamaProvider_SupportsProvider(t *testing.T) {
	provider := NewOllamaProvider(OllamaConfig{})
	assert.True(t, provider.SupportsProvider(llm.ProviderTypeOllama))
	assert.False(t, provider.SupportsProvider(llm.ProviderTypeOpenAI))
}

func TestOllamaProvider_HealthCheck(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, "/api/tags", r.URL.Path)
			w.WriteHeader(http.StatusOK)
		}))
		defer server.Close()

		provider := NewOllamaProvider(OllamaConfig{BaseURL: server.URL})

		err := provider.HealthCheck(context.Background())
		require.NoError(t, err)
	})

	t.Run("error", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusServiceUnavailable)
		}))
		defer server.Close()

		provider := NewOllamaProvider(OllamaConfig{BaseURL: server.URL})

		err := provider.HealthCheck(context.Background())
		require.Error(t, err)
	})
}

func TestOllamaProvider_Complete(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, "/api/chat", r.URL.Path)

			var req ollamaChatRequest
			err := json.NewDecoder(r.Body).Decode(&req)
			require.NoError(t, err)
			assert.Equal(t, "llama3", req.Model)
			assert.False(t, req.Stream)

			resp := `{"model":"llama3","done_reason":"stop","message":{"role":"assistant","content":"Hello from Ollama"},"prompt_eval_count":10,"eval_count":5}`
			_, _ = w.Write([]byte(resp))
		}))
		defer server.Close()

		provider := NewOllamaProvider(OllamaConfig{BaseURL: server.URL})

		resp, err := provider.Complete(context.Background(), &llm.CompletionRequest{
			Messages: []llm.Message{{Role: "user", Content: "Hello"}},
		})

		require.NoError(t, err)
		assert.Equal(t, "Hello from Ollama", resp.Content)
		assert.Equal(t, 15, resp.Usage.TotalTokens)
	})

	t.Run("with temperature", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			var req ollamaChatRequest
			_ = json.NewDecoder(r.Body).Decode(&req)
			require.NotNil(t, req.Options)
			assert.Equal(t, 0.7, req.Options.Temperature)

			resp := `{"model":"llama3","message":{"role":"assistant","content":"ok"}}`
			_, _ = w.Write([]byte(resp))
		}))
		defer server.Close()

		provider := NewOllamaProvider(OllamaConfig{BaseURL: server.URL})

		_, err := provider.Complete(context.Background(), &llm.CompletionRequest{
			Messages:    []llm.Message{{Role: "user", Content: "Hi"}},
			Temperature: 0.7,
		})
		require.NoError(t, err)
	})

	t.Run("empty content", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			resp := `{"model":"llama3","message":{"role":"assistant","content":""}}`
			_, _ = w.Write([]byte(resp))
		}))
		defer server.Close()

		provider := NewOllamaProvider(OllamaConfig{BaseURL: server.URL})

		resp, err := provider.Complete(context.Background(), &llm.CompletionRequest{
			Messages: []llm.Message{{Role: "user", Content: "Hi"}},
		})

		require.Error(t, err)
		assert.ErrorIs(t, err, llm.ErrInvalidResponse)
		assert.Nil(t, resp)
	})

	t.Run("server error", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusInternalServerError)
			_, _ = w.Write([]byte("error"))
		}))
		defer server.Close()

		provider := NewOllamaProvider(OllamaConfig{BaseURL: server.URL})

		resp, err := provider.Complete(context.Background(), &llm.CompletionRequest{
			Messages: []llm.Message{{Role: "user", Content: "Hi"}},
		})

		require.Error(t, err)
		assert.Nil(t, resp)
	})

	t.Run("network error", func(t *testing.T) {
		provider := NewOllamaProvider(OllamaConfig{BaseURL: "http://localhost:99999"})

		resp, err := provider.Complete(context.Background(), &llm.CompletionRequest{
			Messages: []llm.Message{{Role: "user", Content: "Hi"}},
		})

		require.Error(t, err)
		assert.Nil(t, resp)
	})

	t.Run("invalid json response", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_, _ = w.Write([]byte("invalid json"))
		}))
		defer server.Close()

		provider := NewOllamaProvider(OllamaConfig{BaseURL: server.URL})

		resp, err := provider.Complete(context.Background(), &llm.CompletionRequest{
			Messages: []llm.Message{{Role: "user", Content: "Hi"}},
		})

		require.Error(t, err)
		assert.Nil(t, resp)
	})

	t.Run("json response format", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			var raw map[string]any
			err := json.NewDecoder(r.Body).Decode(&raw)
			require.NoError(t, err)
			assert.Equal(t, "json", raw["format"])

			resp := `{"model":"llama3","message":{"role":"assistant","content":"{\"ok\":true}"},"done":true,"done_reason":"stop","prompt_eval_count":5,"eval_count":3}`
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(resp))
		}))
		defer server.Close()

		provider := NewOllamaProvider(OllamaConfig{
			BaseURL: server.URL,
		})

		resp, err := provider.Complete(context.Background(), &llm.CompletionRequest{
			Messages:       []llm.Message{{Role: "user", Content: "return JSON"}},
			ResponseFormat: llm.ResponseFormatJSON,
		})

		require.NoError(t, err)
		assert.Equal(t, `{"ok":true}`, resp.Content)
	})

	t.Run("text response format omits format field", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			var raw map[string]any
			err := json.NewDecoder(r.Body).Decode(&raw)
			require.NoError(t, err)
			_, hasFormat := raw["format"]
			assert.False(t, hasFormat, "format should be omitted for text mode")

			resp := `{"model":"llama3","message":{"role":"assistant","content":"hello"},"done":true,"done_reason":"stop","prompt_eval_count":5,"eval_count":1}`
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(resp))
		}))
		defer server.Close()

		provider := NewOllamaProvider(OllamaConfig{
			BaseURL: server.URL,
		})

		resp, err := provider.Complete(context.Background(), &llm.CompletionRequest{
			Messages: []llm.Message{{Role: "user", Content: "hello"}},
		})

		require.NoError(t, err)
		assert.Equal(t, "hello", resp.Content)
	})
}

func TestLMStudioProvider_New(t *testing.T) {
	t.Run("default values", func(t *testing.T) {
		provider := NewLMStudioProvider(LMStudioConfig{})
		assert.Equal(t, "", provider.model)
		assert.Equal(t, 1024, provider.maxTokens)
		assert.Equal(t, "http://localhost:1234/v1", provider.baseURL)
	})

	t.Run("custom values", func(t *testing.T) {
		provider := NewLMStudioProvider(LMStudioConfig{
			BaseURL:   "http://custom:1234/v1",
			Model:     "local-model",
			MaxTokens: 2048,
		})
		assert.Equal(t, "local-model", provider.model)
		assert.Equal(t, 2048, provider.maxTokens)
		assert.Equal(t, "http://custom:1234/v1", provider.baseURL)
	})
}

func TestLMStudioProvider_Name(t *testing.T) {
	provider := NewLMStudioProvider(LMStudioConfig{})
	assert.Equal(t, "lmstudio", provider.Name())
}

func TestLMStudioProvider_SupportsProvider(t *testing.T) {
	provider := NewLMStudioProvider(LMStudioConfig{})
	assert.True(t, provider.SupportsProvider(llm.ProviderTypeLMStudio))
	assert.False(t, provider.SupportsProvider(llm.ProviderTypeOpenAI))
}

func TestLMStudioProvider_HealthCheck(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, "/v1/models", r.URL.Path)
			w.WriteHeader(http.StatusOK)
		}))
		defer server.Close()

		provider := NewLMStudioProvider(LMStudioConfig{BaseURL: server.URL + "/v1"})

		err := provider.HealthCheck(context.Background())
		require.NoError(t, err)
	})

	t.Run("error", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusServiceUnavailable)
		}))
		defer server.Close()

		provider := NewLMStudioProvider(LMStudioConfig{BaseURL: server.URL + "/v1"})

		err := provider.HealthCheck(context.Background())
		require.Error(t, err)
	})
}

func TestLMStudioProvider_Complete(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, "/v1/chat/completions", r.URL.Path)

			var req lmstudioChatRequest
			err := json.NewDecoder(r.Body).Decode(&req)
			require.NoError(t, err)
			assert.Len(t, req.Messages, 1)

			resp := `{"model":"local-model","choices":[{"message":{"content":"Hello from LM Studio"},"finish_reason":"stop"}],"usage":{"prompt_tokens":10,"completion_tokens":5,"total_tokens":15}}`
			_, _ = w.Write([]byte(resp))
		}))
		defer server.Close()

		provider := NewLMStudioProvider(LMStudioConfig{BaseURL: server.URL + "/v1"})

		resp, err := provider.Complete(context.Background(), &llm.CompletionRequest{
			Messages: []llm.Message{{Role: "user", Content: "Hello"}},
		})

		require.NoError(t, err)
		assert.Equal(t, "Hello from LM Studio", resp.Content)
	})

	t.Run("with model", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			var req lmstudioChatRequest
			_ = json.NewDecoder(r.Body).Decode(&req)
			assert.Equal(t, "custom-model", req.Model)

			resp := `{"model":"custom-model","choices":[{"message":{"content":"ok"},"finish_reason":"stop"}],"usage":{"prompt_tokens":0,"completion_tokens":0,"total_tokens":0}}`
			_, _ = w.Write([]byte(resp))
		}))
		defer server.Close()

		provider := NewLMStudioProvider(LMStudioConfig{
			BaseURL: server.URL + "/v1",
			Model:   "custom-model",
		})

		_, err := provider.Complete(context.Background(), &llm.CompletionRequest{
			Messages: []llm.Message{{Role: "user", Content: "Hi"}},
		})
		require.NoError(t, err)
	})

	t.Run("with temperature", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			var req lmstudioChatRequest
			_ = json.NewDecoder(r.Body).Decode(&req)
			require.NotNil(t, req.Temperature)
			assert.Equal(t, 0.9, *req.Temperature)

			resp := `{"model":"local","choices":[{"message":{"content":"ok"},"finish_reason":"stop"}],"usage":{"prompt_tokens":0,"completion_tokens":0,"total_tokens":0}}`
			_, _ = w.Write([]byte(resp))
		}))
		defer server.Close()

		provider := NewLMStudioProvider(LMStudioConfig{BaseURL: server.URL + "/v1"})

		_, err := provider.Complete(context.Background(), &llm.CompletionRequest{
			Messages:    []llm.Message{{Role: "user", Content: "Hi"}},
			Temperature: 0.9,
		})
		require.NoError(t, err)
	})

	t.Run("with custom max tokens", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			var req lmstudioChatRequest
			_ = json.NewDecoder(r.Body).Decode(&req)
			assert.Equal(t, 2000, req.MaxTokens)

			resp := `{"model":"local","choices":[{"message":{"content":"ok"},"finish_reason":"stop"}],"usage":{"prompt_tokens":0,"completion_tokens":0,"total_tokens":0}}`
			_, _ = w.Write([]byte(resp))
		}))
		defer server.Close()

		provider := NewLMStudioProvider(LMStudioConfig{BaseURL: server.URL + "/v1"})

		_, err := provider.Complete(context.Background(), &llm.CompletionRequest{
			Messages:  []llm.Message{{Role: "user", Content: "Hi"}},
			MaxTokens: 2000,
		})
		require.NoError(t, err)
	})

	t.Run("empty choices", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			resp := `{"model":"local","choices":[],"usage":{"prompt_tokens":0,"completion_tokens":0,"total_tokens":0}}`
			_, _ = w.Write([]byte(resp))
		}))
		defer server.Close()

		provider := NewLMStudioProvider(LMStudioConfig{BaseURL: server.URL + "/v1"})

		resp, err := provider.Complete(context.Background(), &llm.CompletionRequest{
			Messages: []llm.Message{{Role: "user", Content: "Hi"}},
		})

		require.Error(t, err)
		assert.ErrorIs(t, err, llm.ErrInvalidResponse)
		assert.Nil(t, resp)
	})

	t.Run("server error", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusInternalServerError)
			_, _ = w.Write([]byte("error"))
		}))
		defer server.Close()

		provider := NewLMStudioProvider(LMStudioConfig{BaseURL: server.URL + "/v1"})

		resp, err := provider.Complete(context.Background(), &llm.CompletionRequest{
			Messages: []llm.Message{{Role: "user", Content: "Hi"}},
		})

		require.Error(t, err)
		assert.Nil(t, resp)
	})

	t.Run("network error", func(t *testing.T) {
		provider := NewLMStudioProvider(LMStudioConfig{BaseURL: "http://localhost:99999/v1"})

		resp, err := provider.Complete(context.Background(), &llm.CompletionRequest{
			Messages: []llm.Message{{Role: "user", Content: "Hi"}},
		})

		require.Error(t, err)
		assert.Nil(t, resp)
	})

	t.Run("invalid json response", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_, _ = w.Write([]byte("not json"))
		}))
		defer server.Close()

		provider := NewLMStudioProvider(LMStudioConfig{BaseURL: server.URL + "/v1"})

		resp, err := provider.Complete(context.Background(), &llm.CompletionRequest{
			Messages: []llm.Message{{Role: "user", Content: "Hi"}},
		})

		require.Error(t, err)
		assert.Nil(t, resp)
	})
}
