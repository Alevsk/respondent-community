package main

import (
	"testing"
)

// Regression: env-only LLM api_key (no YAML, no default) must populate via Unmarshal.
func TestLoadConfig_ZAIAPIKeyFromEnv(t *testing.T) {
	t.Setenv("RESPONDENT_LLM_ZAI_API_KEY", "zai-secret-xyz")
	// A missing config file is acceptable here; we only exercise env binding.
	_ = InitViper("")
	cfg, err := LoadConfig()
	if err != nil {
		t.Fatalf("LoadConfig: %v", err)
	}
	if cfg.LLM.ZAI.APIKey != "zai-secret-xyz" {
		t.Errorf("ZAI.APIKey = %q, want %q (env-only key not bound through Unmarshal)", cfg.LLM.ZAI.APIKey, "zai-secret-xyz")
	}
}
