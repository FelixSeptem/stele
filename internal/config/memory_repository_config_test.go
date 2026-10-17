package config

import "testing"

func TestLoadFromEnvDefaultsMemoryRepositoryAdmissionToDisabled(t *testing.T) {
	t.Setenv("STELE_POSTGRES_DSN", "postgres://example")
	t.Setenv("STELE_MEMORY_REPOSITORY_IMPORT_ENABLED", "")

	cfg, err := LoadFromEnv()
	if err != nil {
		t.Fatalf("LoadFromEnv() error = %v", err)
	}
	if cfg.MemoryRepositoryImport.Enabled {
		t.Fatal("memory repository admission must default to disabled")
	}
}

func TestLoadFromEnvEnablesMemoryRepositoryAdmissionExplicitly(t *testing.T) {
	t.Setenv("STELE_POSTGRES_DSN", "postgres://example")
	t.Setenv("STELE_MEMORY_REPOSITORY_IMPORT_ENABLED", "true")

	cfg, err := LoadFromEnv()
	if err != nil {
		t.Fatalf("LoadFromEnv() error = %v", err)
	}
	if !cfg.MemoryRepositoryImport.Enabled {
		t.Fatal("memory repository admission did not honor explicit enablement")
	}
}
