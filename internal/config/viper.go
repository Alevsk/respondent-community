// Package config provides unified configuration handling for all Respondent services.
package config

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/spf13/viper"
)

const envPrefix = "RESPONDENT"

// InitViper initializes the global viper instance with config paths, environment
// variables, and shared defaults. All services use the same config file
// (respondent.yaml) with their own sections.
//
// Environment Variable Naming:
//   - All env vars use RESPONDENT_ prefix
//   - Config keys are converted by replacing dots with underscores
//   - Example: server.http.port -> RESPONDENT_SERVER_HTTP_PORT
//
// Returns viper.ConfigFileNotFoundError if config file is not found (caller can ignore).
func InitViper(cfgFile string) error {
	return InitViperInstance(viper.GetViper(), cfgFile)
}

// InitViperInstance initializes a specific viper instance with config paths,
// environment variables, and shared defaults. Prefer this over InitViper when
// you need isolated configuration (e.g., in tests or multi-service binaries).
func InitViperInstance(v *viper.Viper, cfgFile string) error {
	if cfgFile != "" {
		v.SetConfigFile(cfgFile)
	} else {
		v.SetConfigName("respondent")
		v.SetConfigType("yaml")
		v.AddConfigPath(".")
		v.AddConfigPath("/etc/respondent")
		v.AddConfigPath("$HOME/.config/respondent")
	}

	// Environment variable configuration
	// All variables use RESPONDENT_ prefix to avoid collisions with other software
	// Config key "server.http.port" maps to env var "RESPONDENT_SERVER_HTTP_PORT"
	v.SetEnvPrefix(envPrefix)
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	v.AutomaticEnv()

	// Explicitly bind env-only secret keys. viper.Unmarshal does NOT apply
	// AutomaticEnv to nested keys that have no default and are absent from the
	// config file, so secrets supplied only via env (e.g. RESPONDENT_LLM_ZAI_API_KEY)
	// would unmarshal as empty. BindEnv registers the key so Unmarshal resolves it.
	bindSecretEnvKeys(v)

	// Set shared defaults
	setSharedDefaultsOn(v)

	if err := v.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); ok {
			return err
		}
		return fmt.Errorf("error reading config file: %w", err)
	}

	return nil
}

// bindSecretEnvKeys binds env-only secret config keys so viper.Unmarshal resolves
// them from the environment. These keys have no default (secrets must never be
// defaulted) and are typically absent from the YAML, which is exactly the case
// AutomaticEnv does not cover during Unmarshal.
func bindSecretEnvKeys(v *viper.Viper) {
	for _, key := range []string{
		"llm.openai.api_key",
		"llm.anthropic.api_key",
		"llm.gemini.api_key",
		"llm.xai.api_key",
		"llm.zai.api_key",
		"database.password",
	} {
		// BindEnv with a single arg derives the env name from the prefix + replacer
		// (e.g. llm.zai.api_key -> RESPONDENT_LLM_ZAI_API_KEY).
		_ = v.BindEnv(key)
	}
}

// setSharedDefaultsOn configures shared default values on the given viper instance.
// Password is intentionally not defaulted; it must be explicitly configured via
// config file or environment variable.
func setSharedDefaultsOn(v *viper.Viper) {
	// Database defaults (shared)
	v.SetDefault("database.host", "localhost")
	v.SetDefault("database.port", 5433)
	v.SetDefault("database.name", "respondent")
	v.SetDefault("database.user", "respondent")
	v.SetDefault("database.ssl_mode", "disable")
	v.SetDefault("database.max_conns", 10)
	v.SetDefault("database.min_conns", 2)
	v.SetDefault("database.max_conn_lifetime", "1h")

	// NATS defaults (shared)
	v.SetDefault("nats.url", "nats://localhost:4223")
	v.SetDefault("nats.enabled", false)

	// Logging defaults (shared)
	v.SetDefault("logging.level", "info")
	v.SetDefault("logging.format", "console")

	// Tracing defaults (shared)
	v.SetDefault("tracing.enabled", false)
	v.SetDefault("tracing.otlp_endpoint", "localhost:4317")
	v.SetDefault("tracing.sample_rate", 1.0)

	// LLM defaults (shared)
	v.SetDefault("llm.provider", "")
	v.SetDefault("llm.openai.model", "gpt-4o")
	v.SetDefault("llm.openai.max_tokens", 1024)
	v.SetDefault("llm.openai.base_url", "https://api.openai.com/v1")
	v.SetDefault("llm.anthropic.model", "claude-sonnet-4-20250514")
	v.SetDefault("llm.anthropic.max_tokens", 1024)
	v.SetDefault("llm.anthropic.base_url", "https://api.anthropic.com")
	v.SetDefault("llm.gemini.model", "gemini-2.5-flash")
	v.SetDefault("llm.gemini.base_url", "https://generativelanguage.googleapis.com/v1beta")
	v.SetDefault("llm.xai.model", "grok-3")
	v.SetDefault("llm.xai.max_tokens", 1024)
	v.SetDefault("llm.xai.base_url", "https://api.x.ai/v1")
	v.SetDefault("llm.zai.model", "glm-4.5-flash")
	v.SetDefault("llm.zai.max_tokens", 1024)
	v.SetDefault("llm.zai.base_url", "https://api.z.ai/api/paas/v4")
	v.SetDefault("llm.zai.timeout", "120s")
	v.SetDefault("llm.zai.enable_thinking", false)
	v.SetDefault("llm.ollama.model", "llama3")
	v.SetDefault("llm.ollama.base_url", "http://localhost:11434")
	v.SetDefault("llm.lmstudio.max_tokens", 1024)
	v.SetDefault("llm.lmstudio.base_url", "http://localhost:1234/v1")
}

// LoadDatabaseConfig loads the shared database configuration.
func LoadDatabaseConfig() DatabaseConfig {
	return DatabaseConfig{
		Host:            viper.GetString("database.host"),
		Port:            viper.GetInt("database.port"),
		Name:            viper.GetString("database.name"),
		User:            viper.GetString("database.user"),
		Password:        viper.GetString("database.password"),
		SSLMode:         viper.GetString("database.ssl_mode"),
		MaxConns:        viper.GetInt("database.max_conns"),
		MinConns:        viper.GetInt("database.min_conns"),
		MaxConnLifetime: viper.GetDuration("database.max_conn_lifetime"),
	}
}

// LoadNATSConfig loads the shared NATS configuration.
func LoadNATSConfig() NATSConfig {
	return NATSConfig{
		URL:       viper.GetString("nats.url"),
		ClusterID: viper.GetString("nats.cluster_id"),
		ClientID:  viper.GetString("nats.client_id"),
	}
}

// LoadLoggingConfig loads the shared logging configuration.
func LoadLoggingConfig() LoggingConfig {
	return LoggingConfig{
		Level:  viper.GetString("logging.level"),
		Format: viper.GetString("logging.format"),
	}
}

// LoadTracingConfig loads the shared tracing configuration.
func LoadTracingConfig() TracingConfig {
	return TracingConfig{
		Enabled:      viper.GetBool("tracing.enabled"),
		OTLPEndpoint: viper.GetString("tracing.otlp_endpoint"),
		SampleRate:   viper.GetFloat64("tracing.sample_rate"),
	}
}

// LoadLLMConfig loads the shared LLM configuration.
func LoadLLMConfig() LLMConfig {
	return LLMConfig{
		Provider: viper.GetString("llm.provider"),
		OpenAI: LLMProviderConfig{
			APIKey:    viper.GetString("llm.openai.api_key"),
			Model:     viper.GetString("llm.openai.model"),
			MaxTokens: viper.GetInt("llm.openai.max_tokens"),
			BaseURL:   viper.GetString("llm.openai.base_url"),
		},
		Anthropic: LLMProviderConfig{
			APIKey:    viper.GetString("llm.anthropic.api_key"),
			Model:     viper.GetString("llm.anthropic.model"),
			MaxTokens: viper.GetInt("llm.anthropic.max_tokens"),
			BaseURL:   viper.GetString("llm.anthropic.base_url"),
		},
		Gemini: LLMProviderConfig{
			APIKey:  viper.GetString("llm.gemini.api_key"),
			Model:   viper.GetString("llm.gemini.model"),
			BaseURL: viper.GetString("llm.gemini.base_url"),
		},
		XAI: LLMProviderConfig{
			APIKey:    viper.GetString("llm.xai.api_key"),
			Model:     viper.GetString("llm.xai.model"),
			MaxTokens: viper.GetInt("llm.xai.max_tokens"),
			BaseURL:   viper.GetString("llm.xai.base_url"),
		},
		ZAI: LLMProviderConfig{
			APIKey:    viper.GetString("llm.zai.api_key"),
			Model:     viper.GetString("llm.zai.model"),
			MaxTokens: viper.GetInt("llm.zai.max_tokens"),
			BaseURL:   viper.GetString("llm.zai.base_url"),
		},
		Ollama: LLMProviderConfig{
			Model:   viper.GetString("llm.ollama.model"),
			BaseURL: viper.GetString("llm.ollama.base_url"),
		},
		LMStudio: LLMProviderConfig{
			Model:     viper.GetString("llm.lmstudio.model"),
			MaxTokens: viper.GetInt("llm.lmstudio.max_tokens"),
			BaseURL:   viper.GetString("llm.lmstudio.base_url"),
		},
	}
}

// ParseDuration parses a duration string, returning a default if parsing fails.
func ParseDuration(key string, defaultValue time.Duration) time.Duration {
	str := viper.GetString(key)
	if str == "" {
		return defaultValue
	}
	d, err := time.ParseDuration(str)
	if err != nil {
		return defaultValue
	}
	return d
}

// EnvResolve resolves an environment variable name with the RESPONDENT_ prefix.
// YAML source files use short names (e.g. "NASA_FIRMS_MAP_KEY"), and this function
// looks up the prefixed env var (e.g. "RESPONDENT_NASA_FIRMS_MAP_KEY").
// This follows the same convention as viper's SetEnvPrefix("RESPONDENT").
func EnvResolve(name string) string {
	return os.Getenv(envPrefix + "_" + name)
}
