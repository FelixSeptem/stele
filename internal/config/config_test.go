package config

import "testing"

func TestLoadFromEnvReturnsConfigForValidMode(t *testing.T) {
	t.Setenv("STELE_MODE", "api")
	t.Setenv("STELE_HTTP_ADDR", ":8080")
	t.Setenv("STELE_POSTGRES_DSN", "postgres://stele:stele@localhost:5432/stele?sslmode=disable")
	t.Setenv("STELE_AUTH_API_KEYS", " key-a,key-b ")
	t.Setenv("STELE_AUTH_DEFAULT_TENANT", "tenant-a")
	t.Setenv("STELE_AUTH_DEFAULT_PROJECT", "project-a")
	t.Setenv("STELE_AUTH_DEFAULT_NAMESPACE", "namespace-a")
	t.Setenv("STELE_JOBS_MAINTENANCE_INTERVAL", "30m")

	cfg, err := LoadFromEnv()
	if err != nil {
		t.Fatalf("LoadFromEnv() error = %v", err)
	}

	if cfg.Mode != ModeAPI {
		t.Fatalf("Mode = %q, want %q", cfg.Mode, ModeAPI)
	}

	if cfg.HTTPAddr != ":8080" {
		t.Fatalf("HTTPAddr = %q, want %q", cfg.HTTPAddr, ":8080")
	}

	if len(cfg.Auth.APIKeys) != 2 || cfg.Auth.APIKeys[0] != "key-a" || cfg.Auth.APIKeys[1] != "key-b" {
		t.Fatalf("Auth.APIKeys = %#v, want trimmed api keys", cfg.Auth.APIKeys)
	}

	if cfg.Auth.DefaultTenant != "tenant-a" || cfg.Auth.DefaultProject != "project-a" || cfg.Auth.DefaultNamespace != "namespace-a" {
		t.Fatalf("Auth defaults = %+v, want tenant/project/namespace defaults", cfg.Auth)
	}

	if cfg.Jobs.MaintenanceInterval.Minutes() != 30 {
		t.Fatalf("Jobs.MaintenanceInterval = %v, want 30m", cfg.Jobs.MaintenanceInterval)
	}
}

func TestLoadFromEnvRejectsMissingDatabaseDSN(t *testing.T) {
	t.Setenv("STELE_MODE", "worker")

	_, err := LoadFromEnv()
	if err == nil {
		t.Fatal("LoadFromEnv() error = nil, want missing DSN error")
	}
}

func TestLoadFromEnvRejectsUnknownMode(t *testing.T) {
	t.Setenv("STELE_MODE", "unknown")
	t.Setenv("STELE_POSTGRES_DSN", "postgres://stele:stele@localhost:5432/stele?sslmode=disable")

	_, err := LoadFromEnv()
	if err == nil {
		t.Fatal("LoadFromEnv() error = nil, want invalid mode error")
	}
}

func TestLoadFromEnvRejectsInvalidJobInterval(t *testing.T) {
	t.Setenv("STELE_MODE", "api")
	t.Setenv("STELE_POSTGRES_DSN", "postgres://stele:stele@localhost:5432/stele?sslmode=disable")
	t.Setenv("STELE_JOBS_MAINTENANCE_INTERVAL", "not-a-duration")

	_, err := LoadFromEnv()
	if err == nil {
		t.Fatal("LoadFromEnv() error = nil, want invalid duration error")
	}
}
