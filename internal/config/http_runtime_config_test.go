package config

import (
	"testing"
	"time"
)

func TestLoadFromEnvDefaultsHTTPRuntimeLimits(t *testing.T) {
	t.Setenv("STELE_MODE", "api")
	t.Setenv("STELE_POSTGRES_DSN", "postgres://example")

	cfg, err := LoadFromEnv()
	if err != nil {
		t.Fatalf("LoadFromEnv() error = %v", err)
	}
	if cfg.HTTP.MaxRequestBodyBytes <= 0 || cfg.HTTP.MaxHeaderBytes <= 0 {
		t.Fatalf("HTTP limits = %+v, want positive body and header limits", cfg.HTTP)
	}
	if cfg.HTTP.ReadHeaderTimeout <= 0 || cfg.HTTP.ReadTimeout <= 0 || cfg.HTTP.WriteTimeout <= 0 || cfg.HTTP.IdleTimeout <= 0 || cfg.HTTP.ShutdownTimeout <= 0 {
		t.Fatalf("HTTP timeouts = %+v, want positive values", cfg.HTTP)
	}
}

func TestLoadFromEnvParsesHTTPRuntimeLimits(t *testing.T) {
	t.Setenv("STELE_MODE", "api")
	t.Setenv("STELE_POSTGRES_DSN", "postgres://example")
	t.Setenv("STELE_HTTP_MAX_REQUEST_BODY_BYTES", "12345")
	t.Setenv("STELE_HTTP_MAX_HEADER_BYTES", "23456")
	t.Setenv("STELE_HTTP_READ_HEADER_TIMEOUT", "3s")
	t.Setenv("STELE_HTTP_READ_TIMEOUT", "4s")
	t.Setenv("STELE_HTTP_WRITE_TIMEOUT", "5s")
	t.Setenv("STELE_HTTP_IDLE_TIMEOUT", "6s")
	t.Setenv("STELE_HTTP_SHUTDOWN_TIMEOUT", "7s")

	cfg, err := LoadFromEnv()
	if err != nil {
		t.Fatalf("LoadFromEnv() error = %v", err)
	}
	if cfg.HTTP.MaxRequestBodyBytes != 12345 || cfg.HTTP.MaxHeaderBytes != 23456 {
		t.Fatalf("HTTP limits = %+v, want configured bytes", cfg.HTTP)
	}
	if cfg.HTTP.ReadHeaderTimeout != 3*time.Second || cfg.HTTP.ReadTimeout != 4*time.Second || cfg.HTTP.WriteTimeout != 5*time.Second || cfg.HTTP.IdleTimeout != 6*time.Second || cfg.HTTP.ShutdownTimeout != 7*time.Second {
		t.Fatalf("HTTP timeouts = %+v, want configured values", cfg.HTTP)
	}
}

func TestLoadFromEnvRejectsUnsafeHTTPRuntimeLimits(t *testing.T) {
	for _, setting := range []struct {
		name  string
		key   string
		value string
	}{
		{name: "zero body", key: "STELE_HTTP_MAX_REQUEST_BODY_BYTES", value: "0"},
		{name: "negative headers", key: "STELE_HTTP_MAX_HEADER_BYTES", value: "-1"},
		{name: "zero shutdown", key: "STELE_HTTP_SHUTDOWN_TIMEOUT", value: "0s"},
	} {
		t.Run(setting.name, func(t *testing.T) {
			t.Setenv("STELE_MODE", "api")
			t.Setenv("STELE_POSTGRES_DSN", "postgres://example")
			t.Setenv(setting.key, setting.value)
			if _, err := LoadFromEnv(); err == nil {
				t.Fatalf("LoadFromEnv() error = nil for %s", setting.name)
			}
		})
	}
}
