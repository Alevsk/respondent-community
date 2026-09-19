package tasks

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"text/template"
	"time"

	"github.com/rs/zerolog"

	"github.com/Alevsk/respondent/internal/llm"
)

const (
	// defaultHTTPTimeout is the default timeout for HTTP step requests.
	defaultHTTPTimeout = 30 * time.Second

	// maxHTTPResponseBody is the maximum response body size for HTTP steps (1 MB).
	maxHTTPResponseBody = 1 << 20
)

// HTTPDoer abstracts HTTP request execution. Defined in the consumer
// package (tasks) following DIP so the engine does not depend on a
// concrete *http.Client.
type HTTPDoer interface {
	Do(req *http.Request) (*http.Response, error)
}

// TargetDeliverer routes payloads to named targets. The tasks package
// defines this interface (consumer-side) so it can accept the targets
// registry without creating an import cycle.
type TargetDeliverer interface {
	Deliver(ctx context.Context, targetName string, payload any) error
}

// schemaProvider supplies JSON Schemas by key and validates/extracts LLM output
// against them. *schema.Registry satisfies it; the engine depends on the
// interface, not the concrete registry (DIP), like HTTPDoer and TargetDeliverer.
type schemaProvider interface {
	SchemaJSON(key string) string
	ValidateAndExtract(key, rawResponse string) (map[string]any, error)
}

// Engine executes task pipeline definitions. Each task consists of a
// sequence of steps (ai, http_request, target) executed in order,
// where each step can access results from all prior steps.
type Engine struct {
	llmProvider llm.Provider
	schemas     schemaProvider
	httpClient  HTTPDoer
	deliverer   TargetDeliverer
	logger      zerolog.Logger
}

// NewEngine creates a task execution engine.
// llmProvider may be nil if no AI steps will be executed.
// deliverer may be nil if target delivery is not configured; target steps
// will fall back to placeholder logging in that case.
// httpClient is optional; when nil a default *http.Client with a 30s timeout is used.
func NewEngine(llmProvider llm.Provider, schemas schemaProvider, deliverer TargetDeliverer, logger zerolog.Logger, httpClient ...HTTPDoer) *Engine {
	var hc HTTPDoer
	if len(httpClient) > 0 && httpClient[0] != nil {
		hc = httpClient[0]
	} else {
		hc = &http.Client{
			Timeout: defaultHTTPTimeout,
		}
	}
	return &Engine{
		llmProvider: llmProvider,
		schemas:     schemas,
		deliverer:   deliverer,
		httpClient:  hc,
		logger:      logger,
	}
}

// ExecuteTask runs all steps of a task definition sequentially.
// triggerData provides context about what triggered the task (e.g., entity data).
// Returns the StepContext containing all step results.
func (e *Engine) ExecuteTask(ctx context.Context, def *TaskDefinition, triggerData map[string]any) (*StepContext, error) {
	if !def.Enabled {
		return nil, fmt.Errorf("task %q is disabled", def.Name)
	}

	sctx := &StepContext{
		Entity: triggerData,
		Steps:  make(map[string]*StepResult, len(def.Steps)),
	}

	e.logger.Info().
		Str("task", def.Name).
		Int("steps", len(def.Steps)).
		Msg("executing task pipeline")

	for i, step := range def.Steps {
		e.logger.Debug().
			Str("task", def.Name).
			Str("step", step.Name).
			Str("type", step.Type).
			Int("index", i).
			Msg("executing step")

		result, err := e.executeStep(ctx, def.Name, step, sctx)
		if err != nil {
			e.logger.Error().Err(err).
				Str("task", def.Name).
				Str("step", step.Name).
				Str("type", step.Type).
				Msg("step execution failed")
			return sctx, fmt.Errorf("step %q (index %d): %w", step.Name, i, err)
		}

		sctx.Steps[step.Name] = result

		e.logger.Debug().
			Str("task", def.Name).
			Str("step", step.Name).
			Str("type", step.Type).
			Msg("step completed")
	}

	e.logger.Info().
		Str("task", def.Name).
		Msg("task pipeline completed")

	return sctx, nil
}

// executeStep routes a step to its type-specific handler.
func (e *Engine) executeStep(ctx context.Context, taskName string, step StepConfig, sctx *StepContext) (*StepResult, error) {
	switch step.Type {
	case "ai":
		return e.executeAIStep(ctx, taskName, step, sctx)
	case "http_request":
		return e.executeHTTPStep(ctx, step, sctx)
	case "target":
		return e.executeTargetStep(ctx, step, sctx)
	default:
		return nil, fmt.Errorf("unknown step type: %q", step.Type)
	}
}

// executeAIStep renders the prompt template, calls the LLM, and optionally
// validates the response against the step's output schema.
func (e *Engine) executeAIStep(ctx context.Context, taskName string, step StepConfig, sctx *StepContext) (*StepResult, error) {
	if e.llmProvider == nil {
		return nil, fmt.Errorf("no LLM provider configured for ai step")
	}

	// Render prompt with step context.
	rendered, err := renderStepTemplate(step.Prompt, sctx)
	if err != nil {
		return nil, fmt.Errorf("render prompt template: %w", err)
	}

	// Build completion request.
	req := &llm.CompletionRequest{
		Messages: []llm.Message{
			{Role: "user", Content: rendered},
		},
	}
	if step.MaxTokens > 0 {
		req.MaxTokens = step.MaxTokens
	}
	if step.Temperature > 0 {
		req.Temperature = step.Temperature
	}

	// If output schema is defined, enforce JSON mode and inject schema into a system message.
	schemaKey := SchemaKey(taskName, step.Name)
	if schemaJSON := e.schemas.SchemaJSON(schemaKey); schemaJSON != "" {
		req.ResponseFormat = llm.ResponseFormatJSON
		req.Messages = append([]llm.Message{
			{Role: "system", Content: "Respond with JSON matching this exact schema:\n" + schemaJSON},
		}, req.Messages...)
	}

	// Call LLM.
	resp, err := llm.RetryableComplete(ctx, e.llmProvider, req, llm.DefaultMaxRetries)
	if err != nil {
		return nil, fmt.Errorf("LLM completion: %w", err)
	}

	result := &StepResult{
		Name: step.Name,
		Raw:  resp.Content,
	}

	// If output schema is defined, validate and parse the response.
	if len(step.OutputSchema) > 0 {
		parsed, err := e.schemas.ValidateAndExtract(schemaKey, resp.Content)
		if err != nil {
			return nil, fmt.Errorf("validate LLM response: %w", err)
		}
		result.Result = parsed
	} else {
		result.Result = resp.Content
	}

	return result, nil
}

// executeHTTPStep renders the URL template and makes an HTTP request.
func (e *Engine) executeHTTPStep(ctx context.Context, step StepConfig, sctx *StepContext) (*StepResult, error) {
	// Render URL template.
	renderedURL, err := renderStepTemplate(step.URL, sctx)
	if err != nil {
		return nil, fmt.Errorf("render url template: %w", err)
	}

	method := step.Method
	if method == "" {
		method = http.MethodGet
	}

	req, err := http.NewRequestWithContext(ctx, method, renderedURL, nil)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	for k, v := range step.Headers {
		req.Header.Set(k, v)
	}

	resp, err := e.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("http request: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	body, err := io.ReadAll(io.LimitReader(resp.Body, maxHTTPResponseBody))
	if err != nil {
		return nil, fmt.Errorf("read response body: %w", err)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("http request returned status %d: %s", resp.StatusCode, string(body))
	}

	result := &StepResult{
		Name: step.Name,
		Raw:  string(body),
	}

	// Try to parse as JSON; fall back to raw string.
	var parsed any
	if err := json.Unmarshal(body, &parsed); err == nil {
		result.Result = parsed
	} else {
		result.Result = string(body)
	}

	return result, nil
}

// executeTargetStep delivers payload to named targets via the TargetDeliverer.
// If no deliverer is configured, it falls back to placeholder logging.
func (e *Engine) executeTargetStep(ctx context.Context, step StepConfig, sctx *StepContext) (*StepResult, error) {
	if e.deliverer == nil {
		e.logger.Info().
			Str("step", step.Name).
			Strs("targets", step.Targets).
			Int("prior_steps", len(sctx.Steps)).
			Msg("target step placeholder: no deliverer configured")

		return &StepResult{
			Name:   step.Name,
			Result: map[string]any{"status": "placeholder", "targets": step.Targets},
			Raw:    fmt.Sprintf("placeholder delivery to %v", step.Targets),
		}, nil
	}

	// Build payload from the prompt template (if set) or from the last step result.
	var payload any
	if step.Prompt != "" {
		rendered, err := renderStepTemplate(step.Prompt, sctx)
		if err != nil {
			return nil, fmt.Errorf("render target payload template: %w", err)
		}
		payload = rendered
	} else {
		// Use the last completed step result as the delivery payload.
		payload = buildStepResultMap(sctx.Steps)
	}

	deliveryResults := make(map[string]any, len(step.Targets))
	for _, targetName := range step.Targets {
		if err := e.deliverer.Deliver(ctx, targetName, payload); err != nil {
			e.logger.Error().Err(err).
				Str("step", step.Name).
				Str("target", targetName).
				Msg("target delivery failed")
			deliveryResults[targetName] = map[string]any{"status": "failed", "error": err.Error()}
		} else {
			deliveryResults[targetName] = map[string]any{"status": "delivered"}
			e.logger.Info().
				Str("step", step.Name).
				Str("target", targetName).
				Msg("target delivery succeeded")
		}
	}

	return &StepResult{
		Name:   step.Name,
		Result: deliveryResults,
		Raw:    fmt.Sprintf("delivered to %v", step.Targets),
	}, nil
}

// renderStepTemplate renders a Go template string with the StepContext as data.
// The template has access to .Entity and .Steps (previous step results).
func renderStepTemplate(tmplStr string, sctx *StepContext) (string, error) {
	// Build a template data map that includes helpers for accessing step results.
	data := map[string]any{
		"Entity": sctx.Entity,
		"Steps":  buildStepResultMap(sctx.Steps),
	}

	tmpl, err := template.New("step").Parse(tmplStr)
	if err != nil {
		return "", fmt.Errorf("parse template: %w", err)
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return "", fmt.Errorf("execute template: %w", err)
	}

	return buf.String(), nil
}

// buildStepResultMap converts the StepResult map into a template-friendly map
// where each step is accessible as .Steps.<name>.Result.
func buildStepResultMap(steps map[string]*StepResult) map[string]any {
	m := make(map[string]any, len(steps))
	for name, sr := range steps {
		m[name] = map[string]any{
			"Result": sr.Result,
			"Raw":    sr.Raw,
		}
	}
	return m
}
