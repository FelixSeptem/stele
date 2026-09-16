package config

import (
	"testing"
	"time"
)

func TestLoadFromEnvDefaultsProviderDisabledWithBoundedSettings(t *testing.T) {
	t.Setenv("STELE_POSTGRES_DSN", "postgres://example")
	cfg, err := LoadFromEnv()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Provider.Enabled {
		t.Fatal("provider must be disabled by default")
	}
	if len(cfg.Provider.SchemaVersions) != 1 || cfg.Provider.SchemaVersions[0] != "provider-v1" {
		t.Fatalf("schema versions=%v", cfg.Provider.SchemaVersions)
	}
	if cfg.Provider.BindingLifetime != time.Hour || cfg.Provider.Limits.MaxEventBytes <= 0 || cfg.Provider.Limits.MaxContextBytes <= 0 {
		t.Fatalf("provider=%+v", cfg.Provider)
	}
}

func TestLoadFromEnvParsesProviderConfiguration(t *testing.T) {
	t.Setenv("STELE_POSTGRES_DSN", "postgres://example")
	t.Setenv("STELE_PROVIDER_ENABLED", "true")
	t.Setenv("STELE_PROVIDER_SCHEMA_VERSIONS", "provider-v1,provider-v2")
	t.Setenv("STELE_PROVIDER_MAX_EVENT_BYTES", "4096")
	t.Setenv("STELE_PROVIDER_MAX_INTENT_BYTES", "8192")
	t.Setenv("STELE_PROVIDER_MAX_RETRIEVAL_RESULTS", "25")
	t.Setenv("STELE_PROVIDER_MAX_CONTEXT_BYTES", "16384")
	t.Setenv("STELE_PROVIDER_MAX_CITATIONS", "12")
	t.Setenv("STELE_PROVIDER_MAX_METADATA_BYTES", "2048")
	t.Setenv("STELE_PROVIDER_BINDING_LIFETIME", "30m")
	cfg, err := LoadFromEnv()
	if err != nil {
		t.Fatal(err)
	}
	if !cfg.Provider.Enabled || len(cfg.Provider.SchemaVersions) != 2 || cfg.Provider.Limits.MaxRetrievalResults != 25 || cfg.Provider.BindingLifetime != 30*time.Minute {
		t.Fatalf("provider=%+v", cfg.Provider)
	}
}

func TestLoadFromEnvRejectsInvalidProviderConfiguration(t *testing.T) {
	for _, tc := range []struct{ name, key, value string }{
		{"version", "STELE_PROVIDER_SCHEMA_VERSIONS", "provider-v1,unsafe version"},
		{"event limit", "STELE_PROVIDER_MAX_EVENT_BYTES", "0"},
		{"result limit", "STELE_PROVIDER_MAX_RETRIEVAL_RESULTS", "1001"},
		{"lifetime", "STELE_PROVIDER_BINDING_LIFETIME", "30s"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv("STELE_POSTGRES_DSN", "postgres://example")
			t.Setenv(tc.key, tc.value)
			if _, err := LoadFromEnv(); err == nil {
				t.Fatal("expected provider config error")
			}
		})
	}
}
