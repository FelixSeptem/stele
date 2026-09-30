package config

import (
	"testing"

	"github.com/FelixSeptem/stele/internal/reasoning"
)

func TestLoadFromEnvReasoningIsDisabledByDefault(t *testing.T) {
	t.Setenv("STELE_POSTGRES_DSN", "postgres://runtime/db")
	for _, key := range []string{
		"STELE_REASONING_ENABLED", "STELE_REASONING_MODE", "STELE_REASONING_PROVIDER_VERSION", "STELE_REASONING_SCHEMA_DIGEST",
	} {
		t.Setenv(key, "")
	}
	cfg, err := LoadFromEnv()
	if err != nil {
		t.Fatalf("LoadFromEnv() error = %v", err)
	}
	if cfg.Reasoning.Enabled || cfg.Reasoning.Mode != reasoning.ModeDisabled {
		t.Fatalf("reasoning config = %+v, want disabled by default", cfg.Reasoning)
	}
	if err := cfg.Reasoning.Limits.Validate(); err != nil {
		t.Fatalf("default reasoning limits invalid: %v", err)
	}
}

func TestLoadFromEnvReasoningParsesBoundedOfflineConfiguration(t *testing.T) {
	t.Setenv("STELE_POSTGRES_DSN", "postgres://runtime/db")
	t.Setenv("STELE_REASONING_ENABLED", "true")
	t.Setenv("STELE_REASONING_MODE", "offline")
	t.Setenv("STELE_REASONING_PROVIDER_VERSION", "fixture-provider-v1")
	t.Setenv("STELE_REASONING_SCHEMA_DIGEST", "fixture-schema-v1")
	t.Setenv("STELE_REASONING_MAX_INPUT_BYTES", "1024")
	t.Setenv("STELE_REASONING_MAX_OUTPUT_BYTES", "2048")

	cfg, err := LoadFromEnv()
	if err != nil {
		t.Fatalf("LoadFromEnv() error = %v", err)
	}
	if !cfg.Reasoning.Enabled || cfg.Reasoning.Mode != reasoning.ModeOffline || cfg.Reasoning.ProviderVersion != "fixture-provider-v1" || cfg.Reasoning.SchemaDigest != "fixture-schema-v1" {
		t.Fatalf("reasoning config = %+v, want configured offline capability", cfg.Reasoning)
	}
	if cfg.Reasoning.Limits.MaxInputBytes != 1024 || cfg.Reasoning.Limits.MaxOutputBytes != 2048 {
		t.Fatalf("reasoning limits = %+v, want configured bounds", cfg.Reasoning.Limits)
	}
}

func TestLoadFromEnvRejectsEnabledDisabledReasoningMode(t *testing.T) {
	t.Setenv("STELE_POSTGRES_DSN", "postgres://runtime/db")
	t.Setenv("STELE_REASONING_ENABLED", "true")
	t.Setenv("STELE_REASONING_MODE", "disabled")
	if _, err := LoadFromEnv(); err == nil {
		t.Fatal("LoadFromEnv() error = nil, want enabled/disabled reasoning rejection")
	}
}
