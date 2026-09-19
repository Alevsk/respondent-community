package config

import (
	"testing"

	"github.com/go-playground/validator/v10"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newValidator() *validator.Validate {
	return validator.New()
}

func validOperation() OperationConfig {
	return OperationConfig{
		Name:   "classify_aircraft",
		Prompt: "Classify the entity {{.Entity.Name}} based on metadata: {{.Entity.MetadataJSON}}",
		OutputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"category": map[string]any{"type": "string"},
			},
		},
		OutputTarget: "entity",
		Batch: BatchConfig{
			Size:    10,
			Timeout: "5s",
		},
		Retry: RetryConfig{
			MaxAttempts: 3,
			Backoff:     "exponential",
		},
		MaxTokens:   500,
		Temperature: 0.7,
	}
}

func TestOperationConfig_ValidPasses(t *testing.T) {
	v := newValidator()
	op := validOperation()
	cfg := SourceAIConfig{
		Enabled:    true,
		Operations: []OperationConfig{op},
	}

	err := cfg.Validate(v)
	assert.NoError(t, err)
}

func TestOperationConfig_MissingNameFails(t *testing.T) {
	v := newValidator()
	op := validOperation()
	op.Name = ""
	cfg := SourceAIConfig{
		Enabled:    true,
		Operations: []OperationConfig{op},
	}

	err := cfg.Validate(v)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "Name")
}

func TestOperationConfig_MissingPromptFails(t *testing.T) {
	v := newValidator()
	op := validOperation()
	op.Prompt = ""
	cfg := SourceAIConfig{
		Enabled:    true,
		Operations: []OperationConfig{op},
	}

	err := cfg.Validate(v)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "Prompt")
}

func TestOperationConfig_InvalidOutputTargetFails(t *testing.T) {
	v := newValidator()
	op := validOperation()
	op.OutputTarget = "invalid_target"
	cfg := SourceAIConfig{
		Enabled:    true,
		Operations: []OperationConfig{op},
	}

	err := cfg.Validate(v)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "OutputTarget")
}

func TestOperationConfig_ValidBatchConfigPasses(t *testing.T) {
	v := newValidator()
	op := validOperation()
	op.Batch = BatchConfig{Size: 50, Timeout: "10s"}
	cfg := SourceAIConfig{
		Enabled:    true,
		Operations: []OperationConfig{op},
	}

	err := cfg.Validate(v)
	assert.NoError(t, err)
}

func TestOperationConfig_TemperatureOutOfRangeFails(t *testing.T) {
	v := newValidator()
	op := validOperation()
	op.Temperature = 2.5
	cfg := SourceAIConfig{
		Enabled:    true,
		Operations: []OperationConfig{op},
	}

	err := cfg.Validate(v)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "Temperature")
}

func TestOperationConfig_MaxTokensOutOfRangeFails(t *testing.T) {
	v := newValidator()
	op := validOperation()
	op.MaxTokens = 200000
	cfg := SourceAIConfig{
		Enabled:    true,
		Operations: []OperationConfig{op},
	}

	err := cfg.Validate(v)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "MaxTokens")
}

func TestOperationConfig_BatchSizeOutOfRangeFails(t *testing.T) {
	v := newValidator()
	op := validOperation()
	op.Batch.Size = 200
	cfg := SourceAIConfig{
		Enabled:    true,
		Operations: []OperationConfig{op},
	}

	err := cfg.Validate(v)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "Size")
}

func TestOperationConfig_InvalidRetryBackoffFails(t *testing.T) {
	v := newValidator()
	op := validOperation()
	op.Retry.Backoff = "linear"
	cfg := SourceAIConfig{
		Enabled:    true,
		Operations: []OperationConfig{op},
	}

	err := cfg.Validate(v)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "Backoff")
}

func TestGetOutputTarget_ReturnsEntityWhenEmpty(t *testing.T) {
	op := OperationConfig{}
	assert.Equal(t, "entity", op.GetOutputTarget())
}

func TestGetOutputTarget_ReturnsObservationWhenSet(t *testing.T) {
	op := OperationConfig{OutputTarget: "observation"}
	assert.Equal(t, "observation", op.GetOutputTarget())
}

func TestGetOutputTarget_ReturnsEntityWhenSetExplicitly(t *testing.T) {
	op := OperationConfig{OutputTarget: "entity"}
	assert.Equal(t, "entity", op.GetOutputTarget())
}

func TestValidate_DisabledAlwaysSucceeds(t *testing.T) {
	v := newValidator()
	cfg := SourceAIConfig{
		Enabled:    false,
		Operations: nil,
	}

	err := cfg.Validate(v)
	assert.NoError(t, err)
}

func TestValidate_DisabledWithInvalidOperationsSucceeds(t *testing.T) {
	v := newValidator()
	// Even with invalid operations, disabled config should pass.
	cfg := SourceAIConfig{
		Enabled: false,
		Operations: []OperationConfig{
			{Name: "", Prompt: ""}, // invalid but shouldn't matter
		},
	}

	err := cfg.Validate(v)
	assert.NoError(t, err)
}

func TestValidate_EnabledNoOperationsFails(t *testing.T) {
	v := newValidator()
	cfg := SourceAIConfig{
		Enabled:    true,
		Operations: nil,
	}

	err := cfg.Validate(v)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no operations defined")
}

func TestValidate_EnabledEmptyOperationsFails(t *testing.T) {
	v := newValidator()
	cfg := SourceAIConfig{
		Enabled:    true,
		Operations: []OperationConfig{},
	}

	err := cfg.Validate(v)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no operations defined")
}

func TestOperationConfig_MultipleOperationsValid(t *testing.T) {
	v := newValidator()
	op1 := validOperation()
	op2 := validOperation()
	op2.Name = "enrich_metadata"
	op2.OutputTarget = "observation"
	cfg := SourceAIConfig{
		Enabled:    true,
		Operations: []OperationConfig{op1, op2},
	}

	err := cfg.Validate(v)
	assert.NoError(t, err)
}

func TestOperationConfig_ZeroTemperatureValid(t *testing.T) {
	v := newValidator()
	op := validOperation()
	op.Temperature = 0
	cfg := SourceAIConfig{
		Enabled:    true,
		Operations: []OperationConfig{op},
	}

	err := cfg.Validate(v)
	assert.NoError(t, err)
}

func TestOperationConfig_MaxTemperatureValid(t *testing.T) {
	v := newValidator()
	op := validOperation()
	op.Temperature = 2
	cfg := SourceAIConfig{
		Enabled:    true,
		Operations: []OperationConfig{op},
	}

	err := cfg.Validate(v)
	assert.NoError(t, err)
}

func TestOperationConfig_MinMaxTokensValid(t *testing.T) {
	v := newValidator()
	op := validOperation()
	op.MaxTokens = 1
	cfg := SourceAIConfig{
		Enabled:    true,
		Operations: []OperationConfig{op},
	}

	err := cfg.Validate(v)
	assert.NoError(t, err)
}

func TestOperationConfig_MaxMaxTokensValid(t *testing.T) {
	v := newValidator()
	op := validOperation()
	op.MaxTokens = 128000
	cfg := SourceAIConfig{
		Enabled:    true,
		Operations: []OperationConfig{op},
	}

	err := cfg.Validate(v)
	assert.NoError(t, err)
}

func TestOperationConfig_TagsAndFilterOptional(t *testing.T) {
	v := newValidator()
	op := OperationConfig{
		Name:   "simple_op",
		Prompt: "Do something",
	}
	cfg := SourceAIConfig{
		Enabled:    true,
		Operations: []OperationConfig{op},
	}

	err := cfg.Validate(v)
	assert.NoError(t, err)
}

func TestOperationConfig_RetryMaxAttemptsZeroValid(t *testing.T) {
	v := newValidator()
	op := validOperation()
	op.Retry = RetryConfig{MaxAttempts: 0, Backoff: "fixed"}
	cfg := SourceAIConfig{
		Enabled:    true,
		Operations: []OperationConfig{op},
	}

	err := cfg.Validate(v)
	assert.NoError(t, err)
}

func TestOperationConfig_RetryMaxAttemptsExceedsMaxFails(t *testing.T) {
	v := newValidator()
	op := validOperation()
	op.Retry = RetryConfig{MaxAttempts: 15, Backoff: "exponential"}
	cfg := SourceAIConfig{
		Enabled:    true,
		Operations: []OperationConfig{op},
	}

	err := cfg.Validate(v)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "MaxAttempts")
}
