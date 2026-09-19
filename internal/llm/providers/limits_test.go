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

func TestOpenAI_ResponseBodyLimit(t *testing.T) {
	// Create a server that returns a response larger than maxLLMResponseBytes.
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		// Write more than maxLLMResponseBytes of data.
		bigBody := strings.Repeat("x", maxLLMResponseBytes+1)
		_, _ = w.Write([]byte(bigBody))
	}))
	defer server.Close()

	provider := NewOpenAIProvider(OpenAIConfig{
		APIKey:  "test",
		BaseURL: server.URL,
	})

	_, err := provider.Complete(context.Background(), &llm.CompletionRequest{
		Messages: []llm.Message{{Role: "user", Content: "test"}},
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "exceeded")
}

func TestOpenAI_NormalResponsePasses(t *testing.T) {
	// Create a server that returns a normal response.
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := map[string]any{
			"model": "gpt-4o",
			"choices": []map[string]any{
				{
					"message":       map[string]string{"content": "hello"},
					"finish_reason": "stop",
				},
			},
			"usage": map[string]int{
				"prompt_tokens":     10,
				"completion_tokens": 5,
				"total_tokens":      15,
			},
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	provider := NewOpenAIProvider(OpenAIConfig{
		APIKey:  "test",
		BaseURL: server.URL,
	})

	resp, err := provider.Complete(context.Background(), &llm.CompletionRequest{
		Messages: []llm.Message{{Role: "user", Content: "test"}},
	})
	require.NoError(t, err)
	assert.Equal(t, "hello", resp.Content)
}

func TestMaxLLMResponseBytes_Value(t *testing.T) {
	// Verify the constant is set to 10MB.
	assert.Equal(t, 10*1024*1024, maxLLMResponseBytes)
}
