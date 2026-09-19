// Package providers contains supplementary tests to cover specific branches
// that are not reached by the primary test suite in openai_test.go.
package providers

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/Alevsk/respondent/internal/llm"
)

// --- Gemini HealthCheck: network error (httpClient.Do fails) ---

func TestGeminiProvider_HealthCheck_NetworkError(t *testing.T) {
	p := NewGeminiProvider(GeminiConfig{
		APIKey:  "key",
		BaseURL: "http://localhost:19988/v1beta",
	})
	err := p.HealthCheck(context.Background())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "gemini health check")
}

// --- Ollama HealthCheck: network error ---

func TestOllamaProvider_HealthCheck_NetworkError(t *testing.T) {
	p := NewOllamaProvider(OllamaConfig{
		BaseURL: "http://localhost:19986",
	})
	err := p.HealthCheck(context.Background())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "health check failed")
}

// --- XAI HealthCheck: network error ---

func TestXAIProvider_HealthCheck_NetworkError(t *testing.T) {
	p := NewXAIProvider(XAIConfig{
		APIKey:  "key",
		BaseURL: "http://localhost:19985/v1",
	})
	err := p.HealthCheck(context.Background())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "health check failed")
}

// --- Gemini Complete: invalid JSON response (json.Unmarshal fails) ---

func TestGeminiProvider_Complete_InvalidJSON(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte("not valid json {{{"))
	}))
	defer server.Close()

	p := NewGeminiProvider(GeminiConfig{
		APIKey:  "key",
		BaseURL: server.URL,
	})

	result, err := p.Complete(context.Background(), &llm.CompletionRequest{
		Messages: []llm.Message{{Role: "user", Content: "Hi"}},
	})

	require.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "failed to decode response")
}

// --- LMStudio HealthCheck: network error ---

func TestLMStudioProvider_HealthCheck_NetworkError(t *testing.T) {
	p := NewLMStudioProvider(LMStudioConfig{
		BaseURL: "http://localhost:19987/v1",
	})
	err := p.HealthCheck(context.Background())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "health check failed")
}

// --- XAI Complete: custom max tokens overrides provider default ---

func TestXAIProvider_Complete_CustomMaxTokens(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req xaiChatRequest
		err := json.NewDecoder(r.Body).Decode(&req)
		require.NoError(t, err)
		assert.Equal(t, 512, req.MaxTokens)

		resp := `{
			"model": "grok-3",
			"choices": [{"message": {"content": "Short answer"}, "finish_reason": "stop"}],
			"usage": {"prompt_tokens": 3, "completion_tokens": 2, "total_tokens": 5}
		}`
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(resp))
	}))
	defer server.Close()

	p := NewXAIProvider(XAIConfig{
		APIKey:  "key",
		BaseURL: server.URL + "/v1",
	})

	result, err := p.Complete(context.Background(), &llm.CompletionRequest{
		Messages:  []llm.Message{{Role: "user", Content: "Hi"}},
		MaxTokens: 512,
	})

	require.NoError(t, err)
	assert.Equal(t, "Short answer", result.Content)
}

// --- XAI Complete: invalid JSON response ---

func TestXAIProvider_Complete_InvalidJSON(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte("not valid json"))
	}))
	defer server.Close()

	p := NewXAIProvider(XAIConfig{
		APIKey:  "key",
		BaseURL: server.URL + "/v1",
	})

	result, err := p.Complete(context.Background(), &llm.CompletionRequest{
		Messages: []llm.Message{{Role: "user", Content: "Hi"}},
	})

	require.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "failed to decode response")
}

// --- OpenAI HealthCheck: request creation error via cancelled context ---

func TestOpenAIProvider_HealthCheck_ContextCancelled(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	p := NewOpenAIProvider(OpenAIConfig{
		APIKey:  "key",
		BaseURL: server.URL + "/v1",
	})

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	err := p.HealthCheck(ctx)
	require.Error(t, err)
}

// --- Anthropic Complete: context cancelled causes network failure ---

func TestAnthropicProvider_Complete_ContextCancelled(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	p := NewAnthropicProvider(AnthropicConfig{
		APIKey:  "key",
		BaseURL: server.URL,
	})

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	result, err := p.Complete(ctx, &llm.CompletionRequest{
		Messages: []llm.Message{{Role: "user", Content: "Hi"}},
	})

	require.Error(t, err)
	assert.Nil(t, result)
}

// --- LMStudio Complete: context cancelled ---

func TestLMStudioProvider_Complete_ContextCancelled(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	p := NewLMStudioProvider(LMStudioConfig{
		BaseURL: server.URL + "/v1",
	})

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	result, err := p.Complete(ctx, &llm.CompletionRequest{
		Messages: []llm.Message{{Role: "user", Content: "Hi"}},
	})

	require.Error(t, err)
	assert.Nil(t, result)
}

// --- Ollama Complete: context cancelled ---

func TestOllamaProvider_Complete_ContextCancelled(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	p := NewOllamaProvider(OllamaConfig{
		BaseURL: server.URL,
	})

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	result, err := p.Complete(ctx, &llm.CompletionRequest{
		Messages: []llm.Message{{Role: "user", Content: "Hi"}},
	})

	require.Error(t, err)
	assert.Nil(t, result)
}

// --- Gemini Complete: context cancelled ---

func TestGeminiProvider_Complete_ContextCancelled(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	p := NewGeminiProvider(GeminiConfig{
		APIKey:  "key",
		BaseURL: server.URL,
	})

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	result, err := p.Complete(ctx, &llm.CompletionRequest{
		Messages: []llm.Message{{Role: "user", Content: "Hi"}},
	})

	require.Error(t, err)
	assert.Nil(t, result)
}

// --- XAI Complete: context cancelled ---

func TestXAIProvider_Complete_ContextCancelled(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	p := NewXAIProvider(XAIConfig{
		APIKey:  "key",
		BaseURL: server.URL + "/v1",
	})

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	result, err := p.Complete(ctx, &llm.CompletionRequest{
		Messages: []llm.Message{{Role: "user", Content: "Hi"}},
	})

	require.Error(t, err)
	assert.Nil(t, result)
}

// --- LMStudio HealthCheck: context cancelled (covers req creation error path) ---

func TestLMStudioProvider_HealthCheck_ContextCancelled(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	p := NewLMStudioProvider(LMStudioConfig{
		BaseURL: server.URL + "/v1",
	})

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	err := p.HealthCheck(ctx)
	require.Error(t, err)
}

// --- Ollama HealthCheck: context cancelled ---

func TestOllamaProvider_HealthCheck_ContextCancelled(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	p := NewOllamaProvider(OllamaConfig{
		BaseURL: server.URL,
	})

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	err := p.HealthCheck(ctx)
	require.Error(t, err)
}

// --- XAI HealthCheck: context cancelled ---

func TestXAIProvider_HealthCheck_ContextCancelled(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	p := NewXAIProvider(XAIConfig{
		APIKey:  "key",
		BaseURL: server.URL + "/v1",
	})

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	err := p.HealthCheck(ctx)
	require.Error(t, err)
}

// --- Gemini HealthCheck: context cancelled ---

func TestGeminiProvider_HealthCheck_ContextCancelled(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	p := NewGeminiProvider(GeminiConfig{
		APIKey:  "key",
		BaseURL: server.URL,
	})

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	err := p.HealthCheck(ctx)
	require.Error(t, err)
}

// --- LMStudio Complete: invalid JSON response ---

func TestLMStudioProvider_Complete_InvalidJSON(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte("not valid json {"))
	}))
	defer server.Close()

	p := NewLMStudioProvider(LMStudioConfig{
		BaseURL: server.URL + "/v1",
	})

	result, err := p.Complete(context.Background(), &llm.CompletionRequest{
		Messages: []llm.Message{{Role: "user", Content: "Hi"}},
	})

	require.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "failed to decode response")
}

// --- Anthropic Complete: non-text content block is skipped, then text found ---
// This hits the loop body where block.Type != "text" (no text block at all,
// already covered), and also tests the path where text block has empty text.

func TestAnthropicProvider_Complete_TextBlockWithEmptyText(t *testing.T) {
	// When the only text block has empty text, ErrInvalidResponse is returned.
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := `{
			"model": "claude-sonnet-4-20250514",
			"stop_reason": "end_turn",
			"content": [{"type": "text", "text": ""}],
			"usage": {"input_tokens": 5, "output_tokens": 0}
		}`
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(resp))
	}))
	defer server.Close()

	p := NewAnthropicProvider(AnthropicConfig{
		APIKey:  "key",
		BaseURL: server.URL,
	})

	result, err := p.Complete(context.Background(), &llm.CompletionRequest{
		Messages: []llm.Message{{Role: "user", Content: "Hi"}},
	})

	require.Error(t, err)
	assert.Nil(t, result)
	assert.ErrorIs(t, err, llm.ErrInvalidResponse)
}

// --- Anthropic Complete: io.ReadAll error via premature connection close ---
// Uses a custom server that writes headers but closes the connection mid-body.

func TestAnthropicProvider_Complete_ReadBodyError(t *testing.T) {
	// Create a server that hijacks and closes the connection after writing
	// just the HTTP status line to cause io.ReadAll to fail.
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hj, ok := w.(http.Hijacker)
		if !ok {
			t.Skip("server does not support hijacking")
			return
		}
		conn, _, _ := hj.Hijack()
		// Write valid HTTP headers followed by Content-Length that doesn't match.
		_, _ = conn.Write([]byte("HTTP/1.1 200 OK\r\nContent-Length: 1000\r\nContent-Type: application/json\r\n\r\n"))
		_ = conn.Close() // Close connection before writing the body.
	}))
	defer server.Close()

	p := NewAnthropicProvider(AnthropicConfig{
		APIKey:  "key",
		BaseURL: server.URL,
	})

	result, err := p.Complete(context.Background(), &llm.CompletionRequest{
		Messages: []llm.Message{{Role: "user", Content: "Hi"}},
	})

	require.Error(t, err)
	assert.Nil(t, result)
}

// --- OpenAI Complete: io.ReadAll error ---

func TestOpenAIProvider_Complete_ReadBodyError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hj, ok := w.(http.Hijacker)
		if !ok {
			t.Skip("server does not support hijacking")
			return
		}
		conn, _, _ := hj.Hijack()
		_, _ = conn.Write([]byte("HTTP/1.1 200 OK\r\nContent-Length: 1000\r\nContent-Type: application/json\r\n\r\n"))
		_ = conn.Close()
	}))
	defer server.Close()

	p := NewOpenAIProvider(OpenAIConfig{
		APIKey:  "key",
		BaseURL: server.URL + "/v1",
	})

	result, err := p.Complete(context.Background(), &llm.CompletionRequest{
		Messages: []llm.Message{{Role: "user", Content: "Hi"}},
	})

	require.Error(t, err)
	assert.Nil(t, result)
}

// --- Gemini Complete: io.ReadAll error ---

func TestGeminiProvider_Complete_ReadBodyError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hj, ok := w.(http.Hijacker)
		if !ok {
			t.Skip("server does not support hijacking")
			return
		}
		conn, _, _ := hj.Hijack()
		_, _ = conn.Write([]byte("HTTP/1.1 200 OK\r\nContent-Length: 1000\r\nContent-Type: application/json\r\n\r\n"))
		_ = conn.Close()
	}))
	defer server.Close()

	p := NewGeminiProvider(GeminiConfig{
		APIKey:  "key",
		BaseURL: server.URL,
	})

	result, err := p.Complete(context.Background(), &llm.CompletionRequest{
		Messages: []llm.Message{{Role: "user", Content: "Hi"}},
	})

	require.Error(t, err)
	assert.Nil(t, result)
}

// --- XAI Complete: io.ReadAll error ---

func TestXAIProvider_Complete_ReadBodyError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hj, ok := w.(http.Hijacker)
		if !ok {
			t.Skip("server does not support hijacking")
			return
		}
		conn, _, _ := hj.Hijack()
		_, _ = conn.Write([]byte("HTTP/1.1 200 OK\r\nContent-Length: 1000\r\nContent-Type: application/json\r\n\r\n"))
		_ = conn.Close()
	}))
	defer server.Close()

	p := NewXAIProvider(XAIConfig{
		APIKey:  "key",
		BaseURL: server.URL + "/v1",
	})

	result, err := p.Complete(context.Background(), &llm.CompletionRequest{
		Messages: []llm.Message{{Role: "user", Content: "Hi"}},
	})

	require.Error(t, err)
	assert.Nil(t, result)
}

// --- LMStudio Complete: io.ReadAll error ---

func TestLMStudioProvider_Complete_ReadBodyError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hj, ok := w.(http.Hijacker)
		if !ok {
			t.Skip("server does not support hijacking")
			return
		}
		conn, _, _ := hj.Hijack()
		_, _ = conn.Write([]byte("HTTP/1.1 200 OK\r\nContent-Length: 1000\r\nContent-Type: application/json\r\n\r\n"))
		_ = conn.Close()
	}))
	defer server.Close()

	p := NewLMStudioProvider(LMStudioConfig{
		BaseURL: server.URL + "/v1",
	})

	result, err := p.Complete(context.Background(), &llm.CompletionRequest{
		Messages: []llm.Message{{Role: "user", Content: "Hi"}},
	})

	require.Error(t, err)
	assert.Nil(t, result)
}

// --- Ollama Complete: io.ReadAll error ---

func TestOllamaProvider_Complete_ReadBodyError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hj, ok := w.(http.Hijacker)
		if !ok {
			t.Skip("server does not support hijacking")
			return
		}
		conn, _, _ := hj.Hijack()
		_, _ = conn.Write([]byte("HTTP/1.1 200 OK\r\nContent-Length: 1000\r\nContent-Type: application/json\r\n\r\n"))
		_ = conn.Close()
	}))
	defer server.Close()

	p := NewOllamaProvider(OllamaConfig{
		BaseURL: server.URL,
	})

	result, err := p.Complete(context.Background(), &llm.CompletionRequest{
		Messages: []llm.Message{{Role: "user", Content: "Hi"}},
	})

	require.Error(t, err)
	assert.Nil(t, result)
}

// --- Invalid URL tests: trigger http.NewRequestWithContext errors ---
// A URL with a space in the host name causes NewRequestWithContext to return
// an error, covering the "failed to create request" branches.

func TestAnthropicProvider_Complete_InvalidURL(t *testing.T) {
	p := NewAnthropicProvider(AnthropicConfig{
		APIKey:  "key",
		BaseURL: "http://inva lid.example.com",
	})

	result, err := p.Complete(context.Background(), &llm.CompletionRequest{
		Messages: []llm.Message{{Role: "user", Content: "Hi"}},
	})

	require.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "failed to create request")
}

func TestOpenAIProvider_HealthCheck_InvalidURL(t *testing.T) {
	p := NewOpenAIProvider(OpenAIConfig{
		APIKey:  "key",
		BaseURL: "http://inva lid.example.com/v1",
	})

	err := p.HealthCheck(context.Background())
	require.Error(t, err)
}

func TestOpenAIProvider_Complete_InvalidURL(t *testing.T) {
	p := NewOpenAIProvider(OpenAIConfig{
		APIKey:  "key",
		BaseURL: "http://inva lid.example.com/v1",
	})

	result, err := p.Complete(context.Background(), &llm.CompletionRequest{
		Messages: []llm.Message{{Role: "user", Content: "Hi"}},
	})

	require.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "failed to create request")
}

func TestGeminiProvider_HealthCheck_InvalidURL(t *testing.T) {
	p := NewGeminiProvider(GeminiConfig{
		APIKey:  "key",
		BaseURL: "http://inva lid.example.com/v1beta",
	})

	err := p.HealthCheck(context.Background())
	require.Error(t, err)
}

func TestGeminiProvider_Complete_InvalidURL(t *testing.T) {
	p := NewGeminiProvider(GeminiConfig{
		APIKey:  "key",
		BaseURL: "http://inva lid.example.com/v1beta",
	})

	result, err := p.Complete(context.Background(), &llm.CompletionRequest{
		Messages: []llm.Message{{Role: "user", Content: "Hi"}},
	})

	require.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "failed to create request")
}

func TestLMStudioProvider_HealthCheck_InvalidURL(t *testing.T) {
	p := NewLMStudioProvider(LMStudioConfig{
		BaseURL: "http://inva lid.example.com/v1",
	})

	err := p.HealthCheck(context.Background())
	require.Error(t, err)
}

func TestLMStudioProvider_Complete_InvalidURL(t *testing.T) {
	p := NewLMStudioProvider(LMStudioConfig{
		BaseURL: "http://inva lid.example.com/v1",
	})

	result, err := p.Complete(context.Background(), &llm.CompletionRequest{
		Messages: []llm.Message{{Role: "user", Content: "Hi"}},
	})

	require.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "failed to create request")
}

func TestOllamaProvider_HealthCheck_InvalidURL(t *testing.T) {
	p := NewOllamaProvider(OllamaConfig{
		BaseURL: "http://inva lid.example.com",
	})

	err := p.HealthCheck(context.Background())
	require.Error(t, err)
}

func TestOllamaProvider_Complete_InvalidURL(t *testing.T) {
	p := NewOllamaProvider(OllamaConfig{
		BaseURL: "http://inva lid.example.com",
	})

	result, err := p.Complete(context.Background(), &llm.CompletionRequest{
		Messages: []llm.Message{{Role: "user", Content: "Hi"}},
	})

	require.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "failed to create request")
}

func TestXAIProvider_HealthCheck_InvalidURL(t *testing.T) {
	p := NewXAIProvider(XAIConfig{
		APIKey:  "key",
		BaseURL: "http://inva lid.example.com/v1",
	})

	err := p.HealthCheck(context.Background())
	require.Error(t, err)
}

func TestXAIProvider_Complete_InvalidURL(t *testing.T) {
	p := NewXAIProvider(XAIConfig{
		APIKey:  "key",
		BaseURL: "http://inva lid.example.com/v1",
	})

	result, err := p.Complete(context.Background(), &llm.CompletionRequest{
		Messages: []llm.Message{{Role: "user", Content: "Hi"}},
	})

	require.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "failed to create request")
}

// --- ZAI HealthCheck: network error ---

func TestZAIProvider_HealthCheck_NetworkError(t *testing.T) {
	p := NewZAIProvider(ZAIConfig{
		APIKey:  "key",
		BaseURL: "http://localhost:19984/api/paas/v4",
	})
	err := p.HealthCheck(context.Background())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "health check failed")
}

// --- ZAI Complete: custom max tokens overrides provider default ---

func TestZAIProvider_Complete_CustomMaxTokens(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req zaiChatRequest
		err := json.NewDecoder(r.Body).Decode(&req)
		require.NoError(t, err)
		assert.Equal(t, 512, req.MaxTokens)

		resp := `{
			"model": "glm-4.5-flash",
			"choices": [{"message": {"content": "Short answer"}, "finish_reason": "stop"}],
			"usage": {"prompt_tokens": 3, "completion_tokens": 2, "total_tokens": 5}
		}`
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(resp))
	}))
	defer server.Close()

	p := NewZAIProvider(ZAIConfig{
		APIKey:  "key",
		BaseURL: server.URL + "/v4",
	})

	result, err := p.Complete(context.Background(), &llm.CompletionRequest{
		Messages:  []llm.Message{{Role: "user", Content: "Hi"}},
		MaxTokens: 512,
	})

	require.NoError(t, err)
	assert.Equal(t, "Short answer", result.Content)
}

// --- ZAI Complete: invalid JSON response ---

func TestZAIProvider_Complete_InvalidJSON(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte("not valid json"))
	}))
	defer server.Close()

	p := NewZAIProvider(ZAIConfig{
		APIKey:  "key",
		BaseURL: server.URL + "/v4",
	})

	result, err := p.Complete(context.Background(), &llm.CompletionRequest{
		Messages: []llm.Message{{Role: "user", Content: "Hi"}},
	})

	require.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "failed to decode response")
}

// --- ZAI Complete: context cancelled ---

func TestZAIProvider_Complete_ContextCancelled(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	p := NewZAIProvider(ZAIConfig{
		APIKey:  "key",
		BaseURL: server.URL + "/v4",
	})

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	result, err := p.Complete(ctx, &llm.CompletionRequest{
		Messages: []llm.Message{{Role: "user", Content: "Hi"}},
	})

	require.Error(t, err)
	assert.Nil(t, result)
}

// --- ZAI Complete: io.ReadAll error ---

func TestZAIProvider_Complete_ReadBodyError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hj, ok := w.(http.Hijacker)
		if !ok {
			t.Skip("server does not support hijacking")
			return
		}
		conn, _, _ := hj.Hijack()
		_, _ = conn.Write([]byte("HTTP/1.1 200 OK\r\nContent-Length: 1000\r\nContent-Type: application/json\r\n\r\n"))
		_ = conn.Close()
	}))
	defer server.Close()

	p := NewZAIProvider(ZAIConfig{
		APIKey:  "key",
		BaseURL: server.URL + "/v4",
	})

	result, err := p.Complete(context.Background(), &llm.CompletionRequest{
		Messages: []llm.Message{{Role: "user", Content: "Hi"}},
	})

	require.Error(t, err)
	assert.Nil(t, result)
}

func TestZAIProvider_HealthCheck_InvalidURL(t *testing.T) {
	p := NewZAIProvider(ZAIConfig{
		APIKey:  "key",
		BaseURL: "http://inva lid.example.com/v4",
	})

	err := p.HealthCheck(context.Background())
	require.Error(t, err)
}

func TestZAIProvider_Complete_InvalidURL(t *testing.T) {
	p := NewZAIProvider(ZAIConfig{
		APIKey:  "key",
		BaseURL: "http://inva lid.example.com/v4",
	})

	result, err := p.Complete(context.Background(), &llm.CompletionRequest{
		Messages: []llm.Message{{Role: "user", Content: "Hi"}},
	})

	require.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "failed to create request")
}

// --- ZAI Complete: thinking field driven by EnableThinking config ---

func TestZAIProvider_Complete_ThinkingControlledByConfig(t *testing.T) {
	for _, tc := range []struct {
		name     string
		enable   bool
		wantType string
	}{
		{"disabled_by_default", false, "disabled"},
		{"enabled_when_configured", true, "enabled"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				var req zaiChatRequest
				require.NoError(t, json.NewDecoder(r.Body).Decode(&req))
				require.NotNil(t, req.Thinking, "thinking must always be set explicitly")
				assert.Equal(t, tc.wantType, req.Thinking.Type)
				w.Header().Set("Content-Type", "application/json")
				_, _ = w.Write([]byte(`{"model":"glm-5.2","choices":[{"message":{"content":"{}"},"finish_reason":"stop"}],"usage":{"prompt_tokens":1,"completion_tokens":1,"total_tokens":2}}`))
			}))
			defer server.Close()

			p := NewZAIProvider(ZAIConfig{APIKey: "key", BaseURL: server.URL + "/v4", EnableThinking: tc.enable})
			_, err := p.Complete(context.Background(), &llm.CompletionRequest{
				Messages: []llm.Message{{Role: "user", Content: "Hi"}},
			})
			require.NoError(t, err)
		})
	}
}
