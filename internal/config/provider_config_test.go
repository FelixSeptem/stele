package config

import (
	"testing"
	"time"
)

func TestLoadFromEnvProviderDefaultsAreDisabledAndBounded(t *testing.T) {
	t.Setenv("STELE_POSTGRES_DSN", "postgres://example")
	cfg, err := LoadFromEnv()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Provider.Enabled {
		t.Fatal("provider enabled by default")
	}
	if len(cfg.Provider.SupportedSchemaVersions) != 1 || cfg.Provider.SupportedSchemaVersions[0] != "provider-v1" {
		t.Fatalf("versions = %v", cfg.Provider.SupportedSchemaVersions)
	}
	if cfg.Provider.BindingLifetime != 15*time.Minute || cfg.Provider.Limits.EventBytes <= 0 || cfg.Provider.Limits.ContextItems <= 0 {
		t.Fatalf("provider config = %+v", cfg.Provider)
	}
}

func TestLoadFromEnvParsesProviderConfiguration(t *testing.T) {
	t.Setenv("STELE_POSTGRES_DSN", "postgres://example")
	t.Setenv("STELE_PROVIDER_ENABLED", "true")
	t.Setenv("STELE_PROVIDER_SCHEMA_VERSIONS", "provider-v1")
	t.Setenv("STELE_PROVIDER_BINDING_LIFETIME", "30m")
	t.Setenv("STELE_PROVIDER_MAX_EVENT_BYTES", "2048")
	cfg, err := LoadFromEnv()
	if err != nil {
		t.Fatal(err)
	}
	if !cfg.Provider.Enabled || cfg.Provider.BindingLifetime != 30*time.Minute || cfg.Provider.Limits.EventBytes != 2048 {
		t.Fatalf("provider config = %+v", cfg.Provider)
	}
}

func TestLoadFromEnvRejectsInvalidProviderConfiguration(t *testing.T) {
	for _, tc := range []struct{ name, key, value string }{
		{"enabled", "STELE_PROVIDER_ENABLED", "sometimes"},
		{"version", "STELE_PROVIDER_SCHEMA_VERSIONS", "provider-v2"},
		{"lifetime", "STELE_PROVIDER_BINDING_LIFETIME", "10s"},
		{"event limit", "STELE_PROVIDER_MAX_EVENT_BYTES", "0"},
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
