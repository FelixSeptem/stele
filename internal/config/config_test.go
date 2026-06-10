package config

import (
	"testing"
	"time"
)

func TestLoadFromEnvReturnsConfigForValidMode(t *testing.T) {
	t.Setenv("STELE_MODE", "api")
	t.Setenv("STELE_HTTP_ADDR", ":8080")
	t.Setenv("STELE_POSTGRES_DSN", "postgres://stele:stele@localhost:5432/stele?sslmode=disable")
	t.Setenv("STELE_AUTH_API_KEYS", " key-a,key-b ")
	t.Setenv("STELE_AUTH_ADMIN_API_KEYS", " admin-a,admin-b ")
	t.Setenv("STELE_AUTH_DEFAULT_TENANT", "tenant-a")
	t.Setenv("STELE_AUTH_DEFAULT_PROJECT", "project-a")
	t.Setenv("STELE_AUTH_DEFAULT_NAMESPACE", "namespace-a")
	t.Setenv("STELE_JOBS_MAINTENANCE_INTERVAL", "30m")
	t.Setenv("STELE_JOBS_WORKER_POLL_INTERVAL", "5s")
	t.Setenv("STELE_JOBS_WORKER_ERROR_BACKOFF", "20s")
	t.Setenv("STELE_JOBS_SCHEDULER_ERROR_BACKOFF", "45s")
	t.Setenv("STELE_JOBS_SUMMARY_COMPACTION_INTERVAL", "40m")
	t.Setenv("STELE_JOBS_RETENTION_INTERVAL", "50m")
	t.Setenv("STELE_JOBS_CLEANUP_INTERVAL", "60m")
	t.Setenv("STELE_JOBS_JOB_EXECUTION_RETENTION", "72h")

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

	if len(cfg.Auth.AdminAPIKeys) != 2 || cfg.Auth.AdminAPIKeys[0] != "admin-a" || cfg.Auth.AdminAPIKeys[1] != "admin-b" {
		t.Fatalf("Auth.AdminAPIKeys = %#v, want trimmed admin api keys", cfg.Auth.AdminAPIKeys)
	}

	if cfg.Auth.DefaultTenant != "tenant-a" || cfg.Auth.DefaultProject != "project-a" || cfg.Auth.DefaultNamespace != "namespace-a" {
		t.Fatalf("Auth defaults = %+v, want tenant/project/namespace defaults", cfg.Auth)
	}

	if cfg.Jobs.MaintenanceInterval.Minutes() != 30 {
		t.Fatalf("Jobs.MaintenanceInterval = %v, want 30m", cfg.Jobs.MaintenanceInterval)
	}

	if cfg.Jobs.WorkerPollInterval != 5*time.Second {
		t.Fatalf("Jobs.WorkerPollInterval = %v, want 5s", cfg.Jobs.WorkerPollInterval)
	}

	if cfg.Jobs.WorkerErrorBackoff != 20*time.Second {
		t.Fatalf("Jobs.WorkerErrorBackoff = %v, want 20s", cfg.Jobs.WorkerErrorBackoff)
	}

	if cfg.Jobs.SchedulerErrorBackoff != 45*time.Second {
		t.Fatalf("Jobs.SchedulerErrorBackoff = %v, want 45s", cfg.Jobs.SchedulerErrorBackoff)
	}

	if cfg.Jobs.SummaryCompactionInterval != 40*time.Minute {
		t.Fatalf("Jobs.SummaryCompactionInterval = %v, want 40m", cfg.Jobs.SummaryCompactionInterval)
	}

	if cfg.Jobs.RetentionInterval != 50*time.Minute {
		t.Fatalf("Jobs.RetentionInterval = %v, want 50m", cfg.Jobs.RetentionInterval)
	}

	if cfg.Jobs.CleanupInterval != 60*time.Minute {
		t.Fatalf("Jobs.CleanupInterval = %v, want 60m", cfg.Jobs.CleanupInterval)
	}

	if cfg.Jobs.JobExecutionRetention != 72*time.Hour {
		t.Fatalf("Jobs.JobExecutionRetention = %v, want 72h", cfg.Jobs.JobExecutionRetention)
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

func TestLoadFromEnvParsesDurableWorkerSettings(t *testing.T) {
	t.Setenv("STELE_MODE", "worker")
	t.Setenv("STELE_POSTGRES_DSN", "postgres://example")
	t.Setenv("STELE_JOBS_GOVERNANCE_MAX_ATTEMPTS", "7")
	t.Setenv("STELE_JOBS_GOVERNANCE_RETRY_BACKOFF", "45s")
	t.Setenv("STELE_JOBS_GOVERNANCE_LEASE_RENEW_INTERVAL", "20s")
	t.Setenv("STELE_JOBS_MAINTENANCE_SCOPE_BATCH_LIMIT", "25")

	cfg, err := LoadFromEnv()
	if err != nil {
		t.Fatalf("LoadFromEnv() error = %v", err)
	}

	if cfg.Jobs.GovernanceMaxAttempts != 7 {
		t.Fatalf("GovernanceMaxAttempts = %d, want 7", cfg.Jobs.GovernanceMaxAttempts)
	}

	if cfg.Jobs.GovernanceRetryBackoff != 45*time.Second {
		t.Fatalf("GovernanceRetryBackoff = %v, want 45s", cfg.Jobs.GovernanceRetryBackoff)
	}

	if cfg.Jobs.GovernanceLeaseRenewPeriod != 20*time.Second {
		t.Fatalf("GovernanceLeaseRenewPeriod = %v, want 20s", cfg.Jobs.GovernanceLeaseRenewPeriod)
	}

	if cfg.Jobs.MaintenanceScopeBatchLimit != 25 {
		t.Fatalf("MaintenanceScopeBatchLimit = %d, want 25", cfg.Jobs.MaintenanceScopeBatchLimit)
	}
}
