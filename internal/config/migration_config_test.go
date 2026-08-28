package config

import "testing"

func TestLoadFromEnvDefaultsMigrationPolicyToAuto(t *testing.T) {
	t.Setenv("STELE_MODE", "api")
	t.Setenv("STELE_POSTGRES_DSN", "postgres://example")

	cfg, err := LoadFromEnv()
	if err != nil {
		t.Fatalf("LoadFromEnv() error = %v", err)
	}
	if cfg.Migrations.Policy != MigrationPolicyAuto {
		t.Fatalf("Migrations.Policy = %q, want %q", cfg.Migrations.Policy, MigrationPolicyAuto)
	}
}

func TestLoadFromEnvParsesMigrationPolicy(t *testing.T) {
	for _, policy := range []MigrationPolicy{MigrationPolicyValidate, MigrationPolicyOff} {
		t.Run(string(policy), func(t *testing.T) {
			t.Setenv("STELE_MODE", "worker")
			t.Setenv("STELE_POSTGRES_DSN", "postgres://example")
			t.Setenv("STELE_DATABASE_MIGRATION_POLICY", string(policy))

			cfg, err := LoadFromEnv()
			if err != nil {
				t.Fatalf("LoadFromEnv() error = %v", err)
			}
			if cfg.Migrations.Policy != policy {
				t.Fatalf("Migrations.Policy = %q, want %q", cfg.Migrations.Policy, policy)
			}
		})
	}
}

func TestLoadFromEnvRejectsUnknownMigrationPolicy(t *testing.T) {
	t.Setenv("STELE_MODE", "scheduler")
	t.Setenv("STELE_POSTGRES_DSN", "postgres://example")
	t.Setenv("STELE_DATABASE_MIGRATION_POLICY", "dangerously-ignore")

	if _, err := LoadFromEnv(); err == nil {
		t.Fatal("LoadFromEnv() error = nil, want invalid migration policy rejection")
	}
}
