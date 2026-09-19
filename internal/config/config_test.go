package config

import (
	"os"
	"testing"
	"time"

	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
)

func TestObservationRecordMode_Valid(t *testing.T) {
	tests := []struct {
		name     string
		mode     ObservationRecordMode
		expected bool
	}{
		{"append", RecordAppend, true},
		{"upsert", RecordUpsert, true},
		{"dedupe", RecordDedupe, true},
		{"invalid", ObservationRecordMode("invalid"), false},
		{"empty", ObservationRecordMode(""), false},
		{"unknown", ObservationRecordMode("unknown"), false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.mode.Valid()
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestDatabaseConfig_GetDSN(t *testing.T) {
	tests := []struct {
		name     string
		config   DatabaseConfig
		expected string
	}{
		{
			name: "full config",
			config: DatabaseConfig{
				Host:     "localhost",
				Port:     5432,
				Name:     "testdb",
				User:     "testuser",
				Password: "testpass",
				SSLMode:  "disable",
			},
			expected: "host=localhost port=5432 dbname=testdb user=testuser password=testpass sslmode=disable",
		},
		{
			name: "empty values",
			config: DatabaseConfig{
				Host:     "",
				Port:     0,
				Name:     "",
				User:     "",
				Password: "",
				SSLMode:  "",
			},
			expected: "host= port=0 dbname= user= password= sslmode=",
		},
		{
			name: "with ssl mode require",
			config: DatabaseConfig{
				Host:     "prod-db.example.com",
				Port:     5433,
				Name:     "proddb",
				User:     "admin",
				Password: "secretpass",
				SSLMode:  "require",
			},
			expected: "host=prod-db.example.com port=5433 dbname=proddb user=admin password=secretpass sslmode=require",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.config.GetDSN()
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestLoadValkeyConfig(t *testing.T) {
	viper.Reset()
	viper.Set("valkey.addr", "localhost:6379")
	viper.Set("valkey.password", "secret")
	viper.Set("valkey.db", 1)
	viper.Set("valkey.pool_size", 10)
	viper.Set("valkey.min_idle_conns", 5)

	result := LoadValkeyConfig()

	assert.Equal(t, "localhost:6379", result.Addr)
	assert.Equal(t, "secret", result.Password)
	assert.Equal(t, 1, result.DB)
	assert.Equal(t, 10, result.PoolSize)
	assert.Equal(t, 5, result.MinIdleConn)
}

func TestLoadValkeyConfig_Defaults(t *testing.T) {
	viper.Reset()

	result := LoadValkeyConfig()

	assert.Equal(t, "", result.Addr)
	assert.Equal(t, "", result.Password)
	assert.Equal(t, 0, result.DB)
	assert.Equal(t, 0, result.PoolSize)
	assert.Equal(t, 0, result.MinIdleConn)
}

func TestDatabaseConfig_Fields(t *testing.T) {
	config := DatabaseConfig{
		Host:            "db.example.com",
		Port:            5432,
		Name:            "mydb",
		User:            "myuser",
		Password:        "mypass",
		SSLMode:         "require",
		MaxConns:        100,
		MinConns:        10,
		MaxConnLifetime: time.Hour,
	}

	assert.Equal(t, "db.example.com", config.Host)
	assert.Equal(t, 5432, config.Port)
	assert.Equal(t, "mydb", config.Name)
	assert.Equal(t, "myuser", config.User)
	assert.Equal(t, "mypass", config.Password)
	assert.Equal(t, "require", config.SSLMode)
	assert.Equal(t, 100, config.MaxConns)
	assert.Equal(t, 10, config.MinConns)
	assert.Equal(t, time.Hour, config.MaxConnLifetime)
}

func TestNATSConfig_Fields(t *testing.T) {
	config := NATSConfig{
		URL:       "nats://localhost:4222",
		ClusterID: "test-cluster",
		ClientID:  "test-client",
	}

	assert.Equal(t, "nats://localhost:4222", config.URL)
	assert.Equal(t, "test-cluster", config.ClusterID)
	assert.Equal(t, "test-client", config.ClientID)
}

func TestLoggingConfig_Fields(t *testing.T) {
	config := LoggingConfig{
		Level:  "debug",
		Format: "json",
	}

	assert.Equal(t, "debug", config.Level)
	assert.Equal(t, "json", config.Format)
}

func TestTracingConfig_Fields(t *testing.T) {
	config := TracingConfig{
		Enabled:      true,
		OTLPEndpoint: "localhost:4317",
		SampleRate:   0.5,
	}

	assert.True(t, config.Enabled)
	assert.Equal(t, "localhost:4317", config.OTLPEndpoint)
	assert.Equal(t, 0.5, config.SampleRate)
}

func TestValkeyConfig_Fields(t *testing.T) {
	config := ValkeyConfig{
		Addr:        "redis.example.com:6379",
		Password:    "redispass",
		DB:          2,
		PoolSize:    50,
		MinIdleConn: 10,
	}

	assert.Equal(t, "redis.example.com:6379", config.Addr)
	assert.Equal(t, "redispass", config.Password)
	assert.Equal(t, 2, config.DB)
	assert.Equal(t, 50, config.PoolSize)
	assert.Equal(t, 10, config.MinIdleConn)
}

func TestFeederConfig_Fields(t *testing.T) {
	config := FeederConfig{
		BatchSize:    100,
		WorkerCount:  5,
		RetryLimit:   3,
		RetryBackoff: time.Second * 30,
	}

	assert.Equal(t, 100, config.BatchSize)
	assert.Equal(t, 5, config.WorkerCount)
	assert.Equal(t, 3, config.RetryLimit)
	assert.Equal(t, time.Second*30, config.RetryBackoff)
}

func TestIngestConfig_Fields(t *testing.T) {
	config := IngestConfig{
		SourcesDir: "/etc/respondent/sources.d",
		DevMode:    true,
	}

	assert.Equal(t, "/etc/respondent/sources.d", config.SourcesDir)
	assert.True(t, config.DevMode)
}

// TestLLMConfig_Validate_ZAI asserts that zai is a recognized provider in
// internal/config.LLMConfig.Validate(): valid with an API key, rejected without
// one. The per-provider switch lives here, in LLMConfig.Validate().
func TestLLMConfig_Validate_ZAI(t *testing.T) {
	t.Run("valid with api key", func(t *testing.T) {
		cfg := LLMConfig{Provider: "zai", ZAI: LLMProviderConfig{APIKey: "k"}}
		if err := cfg.Validate(); err != nil {
			t.Fatalf("expected no error for zai provider with api key, got: %v", err)
		}
	})

	t.Run("rejected without api key", func(t *testing.T) {
		cfg := LLMConfig{Provider: "zai"}
		if err := cfg.Validate(); err == nil {
			t.Fatal("expected error for zai provider without api key, got nil")
		}
	})
}

// TestLLMConfig_Validate_LMStudio_Valid asserts that lmstudio with a base_url passes validation.
func TestLLMConfig_Validate_LMStudio_Valid(t *testing.T) {
	cfg := LLMConfig{
		Provider: "lmstudio",
		LMStudio: LLMProviderConfig{BaseURL: "http://localhost:1234/v1"},
	}
	err := cfg.Validate()
	if err != nil {
		t.Fatalf("expected no error for lmstudio provider, got: %v", err)
	}
}

func TestLLMConfig_Fields(t *testing.T) {
	config := LLMConfig{
		Provider: "openai",
		OpenAI: LLMProviderConfig{
			APIKey:    "sk-test",
			Model:     "gpt-4",
			MaxTokens: 4096,
			BaseURL:   "https://api.openai.com/v1",
		},
		Anthropic: LLMProviderConfig{
			APIKey:    "anthropic-key",
			Model:     "claude-3",
			MaxTokens: 8192,
			BaseURL:   "https://api.anthropic.com",
		},
		Gemini: LLMProviderConfig{
			APIKey:    "gemini-key",
			Model:     "gemini-pro",
			MaxTokens: 2048,
			BaseURL:   "https://generativelanguage.googleapis.com",
		},
		XAI: LLMProviderConfig{
			APIKey:    "xai-key",
			Model:     "grok-1",
			MaxTokens: 4096,
			BaseURL:   "https://api.x.ai",
		},
		Ollama: LLMProviderConfig{
			APIKey:    "",
			Model:     "llama2",
			MaxTokens: 4096,
			BaseURL:   "http://localhost:11434",
		},
		LMStudio: LLMProviderConfig{
			APIKey:    "",
			Model:     "local-model",
			MaxTokens: 8192,
			BaseURL:   "http://localhost:1234",
		},
	}

	assert.Equal(t, "openai", config.Provider)
	assert.Equal(t, "sk-test", config.OpenAI.APIKey)
	assert.Equal(t, "gpt-4", config.OpenAI.Model)
	assert.Equal(t, 4096, config.OpenAI.MaxTokens)
	assert.Equal(t, "https://api.openai.com/v1", config.OpenAI.BaseURL)
	assert.Equal(t, "anthropic-key", config.Anthropic.APIKey)
	assert.Equal(t, "claude-3", config.Anthropic.Model)
	assert.Equal(t, "gemini-key", config.Gemini.APIKey)
	assert.Equal(t, "xai-key", config.XAI.APIKey)
	assert.Equal(t, "", config.Ollama.APIKey)
	assert.Equal(t, "", config.LMStudio.APIKey)
}

func TestLLMProviderConfig_Fields(t *testing.T) {
	config := LLMProviderConfig{
		APIKey:    "test-key",
		Model:     "test-model",
		MaxTokens: 1000,
		BaseURL:   "https://test.example.com",
	}

	assert.Equal(t, "test-key", config.APIKey)
	assert.Equal(t, "test-model", config.Model)
	assert.Equal(t, 1000, config.MaxTokens)
	assert.Equal(t, "https://test.example.com", config.BaseURL)
}

func TestFilteringMode_Constants(t *testing.T) {
	// Config aliases must match domain constants.
	assert.Equal(t, FilteringMode("viewport"), FilteringViewport)
	assert.Equal(t, FilteringMode("all"), FilteringAll)
}

func TestObservationRecordMode_Constants(t *testing.T) {
	// Config aliases must match domain constants.
	assert.Equal(t, ObservationRecordMode("append"), RecordAppend)
	assert.Equal(t, ObservationRecordMode("upsert"), RecordUpsert)
	assert.Equal(t, ObservationRecordMode("dedupe"), RecordDedupe)
}

func TestInitViper_NoConfigFile(t *testing.T) {
	viper.Reset()
	err := InitViper("")
	assert.Error(t, err)
}

func TestInitViper_WithConfigFile(t *testing.T) {
	tmpFile, err := os.CreateTemp("", "respondent-*.yaml")
	assert.NoError(t, err)
	defer func() { _ = os.Remove(tmpFile.Name()) }()

	_, err = tmpFile.WriteString("database:\n  host: testhost\n")
	assert.NoError(t, err)
	_ = tmpFile.Close()

	viper.Reset()
	err = InitViper(tmpFile.Name())
	assert.NoError(t, err)
	assert.Equal(t, "testhost", viper.GetString("database.host"))
}

func TestInitViper_InvalidConfigFile(t *testing.T) {
	tmpFile, err := os.CreateTemp("", "respondent-*.yaml")
	assert.NoError(t, err)
	defer func() { _ = os.Remove(tmpFile.Name()) }()

	_, err = tmpFile.WriteString("invalid: yaml: content: :\n")
	assert.NoError(t, err)
	_ = tmpFile.Close()

	viper.Reset()
	err = InitViper(tmpFile.Name())
	assert.Error(t, err)
}

func TestInitViper_NonExistentConfigFile(t *testing.T) {
	viper.Reset()
	err := InitViper("/nonexistent/path/config.yaml")
	assert.Error(t, err)
}

func TestSetSharedDefaults(t *testing.T) {
	viper.Reset()
	setSharedDefaultsOn(viper.GetViper())

	assert.Equal(t, "localhost", viper.GetString("database.host"))
	assert.Equal(t, 5433, viper.GetInt("database.port"))
	assert.Equal(t, "respondent", viper.GetString("database.name"))
	assert.Equal(t, "respondent", viper.GetString("database.user"))
	assert.Equal(t, "", viper.GetString("database.password")) // Password intentionally not defaulted
	assert.Equal(t, "disable", viper.GetString("database.ssl_mode"))
	assert.Equal(t, 10, viper.GetInt("database.max_conns"))
	assert.Equal(t, 2, viper.GetInt("database.min_conns"))
	assert.Equal(t, "1h", viper.GetString("database.max_conn_lifetime"))

	assert.Equal(t, "nats://localhost:4223", viper.GetString("nats.url"))
	assert.Equal(t, false, viper.GetBool("nats.enabled"))

	assert.Equal(t, "info", viper.GetString("logging.level"))
	assert.Equal(t, "console", viper.GetString("logging.format"))

	assert.Equal(t, false, viper.GetBool("tracing.enabled"))
	assert.Equal(t, "localhost:4317", viper.GetString("tracing.otlp_endpoint"))
	assert.Equal(t, 1.0, viper.GetFloat64("tracing.sample_rate"))

	assert.Equal(t, "", viper.GetString("llm.provider"))
	assert.Equal(t, "gpt-4o", viper.GetString("llm.openai.model"))
	assert.Equal(t, 1024, viper.GetInt("llm.openai.max_tokens"))
	assert.Equal(t, "https://api.openai.com/v1", viper.GetString("llm.openai.base_url"))
}

func TestLoadDatabaseConfig(t *testing.T) {
	viper.Reset()
	viper.Set("database.host", "dbhost")
	viper.Set("database.port", 5434)
	viper.Set("database.name", "testdb")
	viper.Set("database.user", "testuser")
	viper.Set("database.password", "testpass")
	viper.Set("database.ssl_mode", "require")
	viper.Set("database.max_conns", 20)
	viper.Set("database.min_conns", 5)
	viper.Set("database.max_conn_lifetime", "2h")

	result := LoadDatabaseConfig()

	assert.Equal(t, "dbhost", result.Host)
	assert.Equal(t, 5434, result.Port)
	assert.Equal(t, "testdb", result.Name)
	assert.Equal(t, "testuser", result.User)
	assert.Equal(t, "testpass", result.Password)
	assert.Equal(t, "require", result.SSLMode)
	assert.Equal(t, 20, result.MaxConns)
	assert.Equal(t, 5, result.MinConns)
	assert.Equal(t, 2*time.Hour, result.MaxConnLifetime)
}

func TestLoadNATSConfig(t *testing.T) {
	viper.Reset()
	viper.Set("nats.url", "nats://custom:4222")
	viper.Set("nats.cluster_id", "custom-cluster")
	viper.Set("nats.client_id", "custom-client")

	result := LoadNATSConfig()

	assert.Equal(t, "nats://custom:4222", result.URL)
	assert.Equal(t, "custom-cluster", result.ClusterID)
	assert.Equal(t, "custom-client", result.ClientID)
}

func TestLoadLoggingConfig(t *testing.T) {
	viper.Reset()
	viper.Set("logging.level", "debug")
	viper.Set("logging.format", "json")

	result := LoadLoggingConfig()

	assert.Equal(t, "debug", result.Level)
	assert.Equal(t, "json", result.Format)
}

func TestLoadTracingConfig(t *testing.T) {
	viper.Reset()
	viper.Set("tracing.enabled", true)
	viper.Set("tracing.otlp_endpoint", "custom:4317")
	viper.Set("tracing.sample_rate", 0.5)

	result := LoadTracingConfig()

	assert.True(t, result.Enabled)
	assert.Equal(t, "custom:4317", result.OTLPEndpoint)
	assert.Equal(t, 0.5, result.SampleRate)
}

func TestLoadLLMConfig(t *testing.T) {
	viper.Reset()
	viper.Set("llm.provider", "anthropic")
	viper.Set("llm.openai.api_key", "openai-key")
	viper.Set("llm.openai.model", "gpt-4-turbo")
	viper.Set("llm.openai.max_tokens", 2048)
	viper.Set("llm.openai.base_url", "https://custom.openai.com")
	viper.Set("llm.anthropic.api_key", "anthropic-key")
	viper.Set("llm.anthropic.model", "claude-3-opus")
	viper.Set("llm.anthropic.max_tokens", 4096)
	viper.Set("llm.anthropic.base_url", "https://custom.anthropic.com")
	viper.Set("llm.gemini.api_key", "gemini-key")
	viper.Set("llm.gemini.model", "gemini-ultra")
	viper.Set("llm.gemini.base_url", "https://custom.gemini.com")
	viper.Set("llm.xai.api_key", "xai-key")
	viper.Set("llm.xai.model", "grok-2")
	viper.Set("llm.xai.max_tokens", 8192)
	viper.Set("llm.xai.base_url", "https://custom.x.ai")
	viper.Set("llm.ollama.model", "llama3.1")
	viper.Set("llm.ollama.base_url", "http://custom:11434")
	viper.Set("llm.lmstudio.model", "mistral")
	viper.Set("llm.lmstudio.max_tokens", 4096)
	viper.Set("llm.lmstudio.base_url", "http://custom:1234")

	result := LoadLLMConfig()

	assert.Equal(t, "anthropic", result.Provider)

	assert.Equal(t, "openai-key", result.OpenAI.APIKey)
	assert.Equal(t, "gpt-4-turbo", result.OpenAI.Model)
	assert.Equal(t, 2048, result.OpenAI.MaxTokens)
	assert.Equal(t, "https://custom.openai.com", result.OpenAI.BaseURL)

	assert.Equal(t, "anthropic-key", result.Anthropic.APIKey)
	assert.Equal(t, "claude-3-opus", result.Anthropic.Model)
	assert.Equal(t, 4096, result.Anthropic.MaxTokens)
	assert.Equal(t, "https://custom.anthropic.com", result.Anthropic.BaseURL)

	assert.Equal(t, "gemini-key", result.Gemini.APIKey)
	assert.Equal(t, "gemini-ultra", result.Gemini.Model)
	assert.Equal(t, "https://custom.gemini.com", result.Gemini.BaseURL)

	assert.Equal(t, "xai-key", result.XAI.APIKey)
	assert.Equal(t, "grok-2", result.XAI.Model)
	assert.Equal(t, 8192, result.XAI.MaxTokens)
	assert.Equal(t, "https://custom.x.ai", result.XAI.BaseURL)

	assert.Equal(t, "llama3.1", result.Ollama.Model)
	assert.Equal(t, "http://custom:11434", result.Ollama.BaseURL)

	assert.Equal(t, "mistral", result.LMStudio.Model)
	assert.Equal(t, 4096, result.LMStudio.MaxTokens)
	assert.Equal(t, "http://custom:1234", result.LMStudio.BaseURL)
}

func TestParseDuration(t *testing.T) {
	tests := []struct {
		name         string
		key          string
		value        string
		defaultValue time.Duration
		expected     time.Duration
	}{
		{"valid duration", "test.duration", "30s", time.Minute, 30 * time.Second},
		{"valid duration hours", "test.duration", "2h", time.Minute, 2 * time.Hour},
		{"valid duration complex", "test.duration", "1h30m", time.Minute, 90 * time.Minute},
		{"empty string", "test.duration", "", time.Minute, time.Minute},
		{"invalid duration", "test.duration", "invalid", time.Minute, time.Minute},
		{"not set", "nonexistent.key", "", time.Hour, time.Hour},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			viper.Reset()
			if tt.value != "" {
				viper.Set(tt.key, tt.value)
			}
			result := ParseDuration(tt.key, tt.defaultValue)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestEnvResolve(t *testing.T) {
	_ = os.Setenv("RESPONDENT_TEST_KEY", "test-value")
	defer func() { _ = os.Unsetenv("RESPONDENT_TEST_KEY") }()

	result := EnvResolve("TEST_KEY")
	assert.Equal(t, "test-value", result)
}

func TestEnvResolve_NotSet(t *testing.T) {
	result := EnvResolve("NONEXISTENT_VAR")
	assert.Equal(t, "", result)
}

func TestEnvResolve_Multiple(t *testing.T) {
	_ = os.Setenv("RESPONDENT_API_KEY", "api-secret")
	_ = os.Setenv("RESPONDENT_DB_PASSWORD", "db-secret")
	defer func() {
		_ = os.Unsetenv("RESPONDENT_API_KEY")
		_ = os.Unsetenv("RESPONDENT_DB_PASSWORD")
	}()

	assert.Equal(t, "api-secret", EnvResolve("API_KEY"))
	assert.Equal(t, "db-secret", EnvResolve("DB_PASSWORD"))
}
