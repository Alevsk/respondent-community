package tasks

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/Alevsk/respondent/internal/ai/schema"
	"github.com/Alevsk/respondent/internal/llm"
)

// mockProvider implements llm.Provider for testing.
type mockProvider struct {
	mu       sync.Mutex
	response *llm.CompletionResponse
	err      error
	// lastReq captures the last completion request for assertions.
	lastReq *llm.CompletionRequest
	// called is signalled after each Complete invocation; tests that use
	// mockProvider from goroutines (e.g. event handler tests) can wait on
	// this channel instead of time.Sleep.
	called chan struct{}
}

func (m *mockProvider) Complete(_ context.Context, req *llm.CompletionRequest) (*llm.CompletionResponse, error) {
	m.mu.Lock()
	m.lastReq = req
	m.mu.Unlock()
	if m.called != nil {
		m.called <- struct{}{}
	}
	return m.response, m.err
}

// getLastReq returns the most recently captured request under the mutex.
func (m *mockProvider) getLastReq() *llm.CompletionRequest {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.lastReq
}

func (m *mockProvider) Name() string                              { return "mock" }
func (m *mockProvider) SupportsProvider(providerType string) bool { return providerType == "mock" }
func (m *mockProvider) HealthCheck(_ context.Context) error       { return nil }

func newTestEngine(provider llm.Provider) (*Engine, *schema.Registry) {
	reg := schema.NewRegistry()
	logger := zerolog.Nop()
	engine := NewEngine(provider, reg, nil, logger)
	return engine, reg
}

// --- ExecuteTask tests ---

func TestEngine_ExecuteTask_Disabled(t *testing.T) {
	engine, _ := newTestEngine(nil)

	def := &TaskDefinition{
		Name:    "disabled_task",
		Enabled: false,
	}

	_, err := engine.ExecuteTask(context.Background(), def, nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "disabled")
}

func TestEngine_ExecuteTask_AIStep(t *testing.T) {
	provider := &mockProvider{
		response: &llm.CompletionResponse{
			Content: `{"impact_level":"major"}`,
		},
	}

	engine, reg := newTestEngine(provider)

	// Register a schema for the AI step.
	err := reg.RegisterFromYAML(SchemaKey("test_task", "analyze"), map[string]any{
		"type":       "object",
		"required":   []any{"impact_level"},
		"properties": map[string]any{"impact_level": map[string]any{"type": "string"}},
	})
	require.NoError(t, err)

	def := &TaskDefinition{
		Name:    "test_task",
		Enabled: true,
		Steps: []StepConfig{
			{
				Name:   "analyze",
				Type:   "ai",
				Prompt: "Analyze earthquake with magnitude {{.Entity.magnitude}}",
				OutputSchema: map[string]any{
					"type":       "object",
					"required":   []any{"impact_level"},
					"properties": map[string]any{"impact_level": map[string]any{"type": "string"}},
				},
			},
		},
	}

	triggerData := map[string]any{
		"magnitude": "7.2",
		"name":      "Test Quake",
	}

	sctx, err := engine.ExecuteTask(context.Background(), def, triggerData)
	require.NoError(t, err)
	require.NotNil(t, sctx)

	result, ok := sctx.Steps["analyze"]
	require.True(t, ok)
	assert.NotNil(t, result.Result)

	resultMap, ok := result.Result.(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "major", resultMap["impact_level"])

	// Verify the prompt was rendered with trigger data.
	require.NotNil(t, provider.lastReq)
	assert.Len(t, provider.lastReq.Messages, 2) // system (schema) + user
	assert.Contains(t, provider.lastReq.Messages[1].Content, "7.2")
}

func TestEngine_ExecuteTask_AIStepNoSchema(t *testing.T) {
	provider := &mockProvider{
		response: &llm.CompletionResponse{
			Content: "Generated report text here",
		},
	}

	engine, _ := newTestEngine(provider)

	def := &TaskDefinition{
		Name:    "no_schema_task",
		Enabled: true,
		Steps: []StepConfig{
			{
				Name:   "generate",
				Type:   "ai",
				Prompt: "Generate a report",
			},
		},
	}

	sctx, err := engine.ExecuteTask(context.Background(), def, nil)
	require.NoError(t, err)

	result := sctx.Steps["generate"]
	assert.Equal(t, "Generated report text here", result.Result)
	assert.Equal(t, "Generated report text here", result.Raw)
}

func TestEngine_ExecuteTask_AIStepWithMaxTokensAndTemperature(t *testing.T) {
	provider := &mockProvider{
		response: &llm.CompletionResponse{Content: "ok"},
	}

	engine, _ := newTestEngine(provider)

	def := &TaskDefinition{
		Name:    "params_task",
		Enabled: true,
		Steps: []StepConfig{
			{
				Name:        "analyze",
				Type:        "ai",
				Prompt:      "Analyze",
				MaxTokens:   2000,
				Temperature: 0.3,
			},
		},
	}

	_, err := engine.ExecuteTask(context.Background(), def, nil)
	require.NoError(t, err)

	require.NotNil(t, provider.lastReq)
	assert.Equal(t, 2000, provider.lastReq.MaxTokens)
	assert.InDelta(t, 0.3, provider.lastReq.Temperature, 0.001)
}

func TestEngine_ExecuteTask_AIStepNoProvider(t *testing.T) {
	engine, _ := newTestEngine(nil) // no provider

	def := &TaskDefinition{
		Name:    "no_provider_task",
		Enabled: true,
		Steps: []StepConfig{
			{
				Name:   "analyze",
				Type:   "ai",
				Prompt: "Analyze",
			},
		},
	}

	_, err := engine.ExecuteTask(context.Background(), def, nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no LLM provider")
}

func TestEngine_ExecuteTask_AIStepLLMError(t *testing.T) {
	provider := &mockProvider{
		err: fmt.Errorf("rate limited"),
	}

	engine, _ := newTestEngine(provider)

	def := &TaskDefinition{
		Name:    "error_task",
		Enabled: true,
		Steps: []StepConfig{
			{
				Name:   "analyze",
				Type:   "ai",
				Prompt: "Analyze",
			},
		},
	}

	_, err := engine.ExecuteTask(context.Background(), def, nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "LLM completion")
}

func TestEngine_ExecuteTask_AIStepSchemaValidationFails(t *testing.T) {
	provider := &mockProvider{
		response: &llm.CompletionResponse{
			Content: `{"wrong_field":"value"}`, // missing required field
		},
	}

	engine, reg := newTestEngine(provider)

	err := reg.RegisterFromYAML(SchemaKey("schema_fail_task", "analyze"), map[string]any{
		"type":       "object",
		"required":   []any{"impact_level"},
		"properties": map[string]any{"impact_level": map[string]any{"type": "string"}},
	})
	require.NoError(t, err)

	def := &TaskDefinition{
		Name:    "schema_fail_task",
		Enabled: true,
		Steps: []StepConfig{
			{
				Name:   "analyze",
				Type:   "ai",
				Prompt: "Analyze",
				OutputSchema: map[string]any{
					"type":       "object",
					"required":   []any{"impact_level"},
					"properties": map[string]any{"impact_level": map[string]any{"type": "string"}},
				},
			},
		},
	}

	_, err = engine.ExecuteTask(context.Background(), def, nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "validate")
}

func TestEngine_ExecuteTask_HTTPStep(t *testing.T) {
	// Set up a test HTTP server.
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(map[string]any{"population": 500000, "city": "TestCity"})
	}))
	defer server.Close()

	engine, _ := newTestEngine(nil)

	def := &TaskDefinition{
		Name:    "http_task",
		Enabled: true,
		Steps: []StepConfig{
			{
				Name: "fetch",
				Type: "http_request",
				URL:  server.URL + "/data",
			},
		},
	}

	sctx, err := engine.ExecuteTask(context.Background(), def, nil)
	require.NoError(t, err)

	result := sctx.Steps["fetch"]
	require.NotNil(t, result)

	// Should be parsed as JSON.
	resultMap, ok := result.Result.(map[string]any)
	require.True(t, ok)
	assert.Equal(t, float64(500000), resultMap["population"])
	assert.Equal(t, "TestCity", resultMap["city"])
}

func TestEngine_ExecuteTask_HTTPStepNonJSONResponse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/plain")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("plain text response"))
	}))
	defer server.Close()

	engine, _ := newTestEngine(nil)

	def := &TaskDefinition{
		Name:    "http_text_task",
		Enabled: true,
		Steps: []StepConfig{
			{
				Name: "fetch",
				Type: "http_request",
				URL:  server.URL + "/text",
			},
		},
	}

	sctx, err := engine.ExecuteTask(context.Background(), def, nil)
	require.NoError(t, err)

	result := sctx.Steps["fetch"]
	assert.Equal(t, "plain text response", result.Result)
}

func TestEngine_ExecuteTask_HTTPStepErrorResponse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte("server error"))
	}))
	defer server.Close()

	engine, _ := newTestEngine(nil)

	def := &TaskDefinition{
		Name:    "http_error_task",
		Enabled: true,
		Steps: []StepConfig{
			{
				Name: "fetch",
				Type: "http_request",
				URL:  server.URL + "/error",
			},
		},
	}

	_, err := engine.ExecuteTask(context.Background(), def, nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "500")
}

func TestEngine_ExecuteTask_HTTPStepURLTemplate(t *testing.T) {
	var receivedPath string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedPath = r.URL.Path + "?" + r.URL.RawQuery
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	defer server.Close()

	engine, _ := newTestEngine(nil)

	def := &TaskDefinition{
		Name:    "url_template_task",
		Enabled: true,
		Steps: []StepConfig{
			{
				Name: "fetch",
				Type: "http_request",
				URL:  server.URL + "/data?lat={{.Entity.lat}}&lon={{.Entity.lon}}",
			},
		},
	}

	triggerData := map[string]any{
		"lat": "35.5",
		"lon": "-120.3",
	}

	_, err := engine.ExecuteTask(context.Background(), def, triggerData)
	require.NoError(t, err)
	assert.Equal(t, "/data?lat=35.5&lon=-120.3", receivedPath)
}

func TestEngine_ExecuteTask_TargetStepPlaceholder(t *testing.T) {
	engine, _ := newTestEngine(nil)

	def := &TaskDefinition{
		Name:    "target_task",
		Enabled: true,
		Steps: []StepConfig{
			{
				Name:    "deliver",
				Type:    "target",
				Targets: []string{"slack_alerts", "email_team"},
			},
		},
	}

	sctx, err := engine.ExecuteTask(context.Background(), def, nil)
	require.NoError(t, err)

	result := sctx.Steps["deliver"]
	require.NotNil(t, result)
	resultMap, ok := result.Result.(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "placeholder", resultMap["status"])
}

func TestEngine_ExecuteTask_SequentialSteps(t *testing.T) {
	// Set up HTTP server for step 1.
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(map[string]any{"population": 1000000})
	}))
	defer server.Close()

	// Set up LLM provider for step 2.
	provider := &mockProvider{
		response: &llm.CompletionResponse{
			Content: `{"impact_level":"major"}`,
		},
	}

	engine, reg := newTestEngine(provider)

	err := reg.RegisterFromYAML(SchemaKey("seq_task", "analyze"), map[string]any{
		"type":       "object",
		"required":   []any{"impact_level"},
		"properties": map[string]any{"impact_level": map[string]any{"type": "string"}},
	})
	require.NoError(t, err)

	def := &TaskDefinition{
		Name:    "seq_task",
		Enabled: true,
		Steps: []StepConfig{
			{
				Name: "fetch",
				Type: "http_request",
				URL:  server.URL + "/population",
			},
			{
				Name:   "analyze",
				Type:   "ai",
				Prompt: "Population data: {{index .Steps \"fetch\" \"Result\"}}",
				OutputSchema: map[string]any{
					"type":       "object",
					"required":   []any{"impact_level"},
					"properties": map[string]any{"impact_level": map[string]any{"type": "string"}},
				},
			},
			{
				Name:    "deliver",
				Type:    "target",
				Targets: []string{"slack_alerts"},
			},
		},
	}

	sctx, err := engine.ExecuteTask(context.Background(), def, map[string]any{"name": "test"})
	require.NoError(t, err)

	// All three steps should have results.
	assert.Len(t, sctx.Steps, 3)
	assert.Contains(t, sctx.Steps, "fetch")
	assert.Contains(t, sctx.Steps, "analyze")
	assert.Contains(t, sctx.Steps, "deliver")

	// Step 2 (AI) should have used step 1 results in its prompt.
	require.NotNil(t, provider.lastReq)
	assert.Contains(t, provider.lastReq.Messages[1].Content, "population")
}

func TestEngine_ExecuteTask_StepFailureStopsExecution(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte("error"))
	}))
	defer server.Close()

	engine, _ := newTestEngine(nil)

	def := &TaskDefinition{
		Name:    "fail_task",
		Enabled: true,
		Steps: []StepConfig{
			{
				Name: "fetch",
				Type: "http_request",
				URL:  server.URL + "/fail",
			},
			{
				Name:    "deliver",
				Type:    "target",
				Targets: []string{"slack"},
			},
		},
	}

	sctx, err := engine.ExecuteTask(context.Background(), def, nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "fetch")

	// Second step should not have been reached.
	_, hasDeliver := sctx.Steps["deliver"]
	assert.False(t, hasDeliver)
}

func TestEngine_ExecuteTask_ContextCancellation(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	defer server.Close()

	engine, _ := newTestEngine(nil)

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // cancel immediately

	def := &TaskDefinition{
		Name:    "cancel_task",
		Enabled: true,
		Steps: []StepConfig{
			{
				Name: "fetch",
				Type: "http_request",
				URL:  server.URL + "/data",
			},
		},
	}

	_, err := engine.ExecuteTask(ctx, def, nil)
	require.Error(t, err)
}

// --- renderStepTemplate tests ---

func TestRenderStepTemplate_EntityData(t *testing.T) {
	sctx := &StepContext{
		Entity: map[string]any{
			"magnitude": "7.2",
			"name":      "Test Quake",
		},
		Steps: make(map[string]*StepResult),
	}

	rendered, err := renderStepTemplate("M{{.Entity.magnitude}} earthquake at {{.Entity.name}}", sctx)
	require.NoError(t, err)
	assert.Equal(t, "M7.2 earthquake at Test Quake", rendered)
}

func TestRenderStepTemplate_PreviousStepResults(t *testing.T) {
	sctx := &StepContext{
		Entity: map[string]any{},
		Steps: map[string]*StepResult{
			"fetch": {
				Name:   "fetch",
				Result: map[string]any{"population": 500000},
				Raw:    `{"population":500000}`,
			},
		},
	}

	rendered, err := renderStepTemplate(`Data: {{index .Steps "fetch" "Result"}}`, sctx)
	require.NoError(t, err)
	assert.Contains(t, rendered, "population")
}

func TestRenderStepTemplate_InvalidTemplate(t *testing.T) {
	sctx := &StepContext{
		Entity: map[string]any{},
		Steps:  make(map[string]*StepResult),
	}

	_, err := renderStepTemplate("{{.Invalid.{{", sctx)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "parse template")
}

// --- buildStepResultMap tests ---

func TestBuildStepResultMap(t *testing.T) {
	steps := map[string]*StepResult{
		"step1": {Name: "step1", Result: "value1", Raw: "raw1"},
		"step2": {Name: "step2", Result: map[string]any{"key": "val"}, Raw: `{"key":"val"}`},
	}

	m := buildStepResultMap(steps)
	assert.Len(t, m, 2)

	s1, ok := m["step1"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "value1", s1["Result"])
	assert.Equal(t, "raw1", s1["Raw"])

	s2, ok := m["step2"].(map[string]any)
	require.True(t, ok)
	resultMap, ok := s2["Result"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "val", resultMap["key"])
}

func TestBuildStepResultMap_Empty(t *testing.T) {
	m := buildStepResultMap(map[string]*StepResult{})
	assert.Empty(t, m)
}

// TestEngine_ExecuteTask_ResponseFormatJSONSetWhenSchemaPresent verifies that
// the tasks engine sets ResponseFormatJSON on the LLM request when a step has
// an output schema registered, mirroring the canonical analysis engine pattern.
func TestEngine_ExecuteTask_ResponseFormatJSONSetWhenSchemaPresent(t *testing.T) {
	provider := &mockProvider{
		response: &llm.CompletionResponse{
			Content: `{"impact_level":"major"}`,
		},
	}

	engine, reg := newTestEngine(provider)

	err := reg.RegisterFromYAML(SchemaKey("rf_task", "rf_step"), map[string]any{
		"type":       "object",
		"required":   []any{"impact_level"},
		"properties": map[string]any{"impact_level": map[string]any{"type": "string"}},
	})
	require.NoError(t, err)

	def := &TaskDefinition{
		Name:    "rf_task",
		Enabled: true,
		Steps: []StepConfig{
			{
				Name:   "rf_step",
				Type:   "ai",
				Prompt: "Analyze the event",
				OutputSchema: map[string]any{
					"type":       "object",
					"required":   []any{"impact_level"},
					"properties": map[string]any{"impact_level": map[string]any{"type": "string"}},
				},
			},
		},
	}

	_, err = engine.ExecuteTask(context.Background(), def, nil)
	require.NoError(t, err)

	req := provider.getLastReq()
	require.NotNil(t, req)
	assert.Equal(t, llm.ResponseFormatJSON, req.ResponseFormat)
	assert.Equal(t, "system", req.Messages[0].Role)
}
