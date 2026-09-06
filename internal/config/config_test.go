package config

import (
	"testing"
	"time"
)

func TestLoadFromEnvReturnsConfigForValidMode(t *testing.T) {
	t.Setenv("STELE_MODE", "api")
	t.Setenv("STELE_HTTP_ADDR", ":8080")
	t.Setenv("STELE_POSTGRES_DSN", "postgres://stele:stele@localhost:5432/stele?sslmode=disable")
	t.Setenv("STELE_AUTH_BOOTSTRAP_ADMIN_KEY", " bootstrap-admin-key ")
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
	t.Setenv("STELE_JOBS_DERIVED_INSIGHT_DERIVATION_INTERVAL", "70m")
	t.Setenv("STELE_JOBS_DERIVED_INSIGHT_BATCH_SIZE", "75")
	t.Setenv("STELE_JOBS_DERIVED_INSIGHT_MINIMUM_EVIDENCE", "3")
	t.Setenv("STELE_JOBS_JOB_EXECUTION_RETENTION", "72h")
	t.Setenv("STELE_WORKFLOW_MAINTENANCE_ENABLED", "true")
	t.Setenv("STELE_WORKFLOW_DIAGNOSTIC_INTERVAL", "25m")
	t.Setenv("STELE_WORKFLOW_STALE_RUN_WINDOW", "3h")
	t.Setenv("STELE_WORKFLOW_DIAGNOSTIC_SCAN_LIMIT", "45")
	t.Setenv("STELE_WORKFLOW_NEXT_ACTION_REFRESH_LIMIT", "55")
	t.Setenv("STELE_WORKFLOW_HISTORY_RETENTION", "240h")

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

	if cfg.Auth.BootstrapAdminKey != "bootstrap-admin-key" {
		t.Fatalf("Auth.BootstrapAdminKey = %q, want configured bootstrap key", cfg.Auth.BootstrapAdminKey)
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

	if cfg.Jobs.DerivedInsightDerivationInterval != 70*time.Minute {
		t.Fatalf("Jobs.DerivedInsightDerivationInterval = %v, want 70m", cfg.Jobs.DerivedInsightDerivationInterval)
	}

	if cfg.Jobs.DerivedInsightBatchSize != 75 {
		t.Fatalf("Jobs.DerivedInsightBatchSize = %d, want 75", cfg.Jobs.DerivedInsightBatchSize)
	}

	if cfg.Jobs.DerivedInsightMinimumEvidence != 3 {
		t.Fatalf("Jobs.DerivedInsightMinimumEvidence = %d, want 3", cfg.Jobs.DerivedInsightMinimumEvidence)
	}

	if cfg.Jobs.JobExecutionRetention != 72*time.Hour {
		t.Fatalf("Jobs.JobExecutionRetention = %v, want 72h", cfg.Jobs.JobExecutionRetention)
	}
	if !cfg.Jobs.WorkflowMaintenanceEnabled || cfg.Jobs.WorkflowDiagnosticCadence != 25*time.Minute || cfg.Jobs.WorkflowStaleRunWindow != 3*time.Hour || cfg.Jobs.WorkflowDiagnosticScanLimit != 45 || cfg.Jobs.WorkflowNextActionRefreshLimit != 55 || cfg.Jobs.WorkflowHistoryRetention != 240*time.Hour {
		t.Fatalf("workflow maintenance config = %+v, want configured bounded values", cfg.Jobs)
	}
}

func TestLoadFromEnvRejectsLegacyAPIKeyLists(t *testing.T) {
	t.Setenv("STELE_MODE", "api")
	t.Setenv("STELE_POSTGRES_DSN", "postgres://stele:stele@localhost:5432/stele?sslmode=disable")
	t.Setenv("STELE_AUTH_API_KEYS", "legacy-key")

	if _, err := LoadFromEnv(); err == nil {
		t.Fatal("LoadFromEnv() error = nil for deprecated legacy key list")
	}
}

func TestLoadFromEnvRequiresDefaultScopeForBootstrapOperator(t *testing.T) {
	t.Setenv("STELE_MODE", "api")
	t.Setenv("STELE_POSTGRES_DSN", "postgres://stele:stele@localhost:5432/stele?sslmode=disable")
	t.Setenv("STELE_AUTH_BOOTSTRAP_ADMIN_KEY", "bootstrap-admin-key")
	t.Setenv("STELE_AUTH_DEFAULT_TENANT", "tenant-a")

	if _, err := LoadFromEnv(); err == nil {
		t.Fatal("LoadFromEnv() error = nil without complete bootstrap default scope")
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

func TestLoadFromEnvRejectsUnsafeWorkflowMaintenanceSettings(t *testing.T) {
	t.Setenv("STELE_MODE", "scheduler")
	t.Setenv("STELE_POSTGRES_DSN", "postgres://example")
	t.Setenv("STELE_WORKFLOW_STALE_RUN_WINDOW", "30s")

	if _, err := LoadFromEnv(); err == nil {
		t.Fatal("LoadFromEnv() error = nil, want unsafe workflow stale window rejection")
	}
}

func TestLoadFromEnvDefaultsMigrationPolicyToAuto(t *testing.T) {
	t.Setenv("STELE_MODE", "api")
	t.Setenv("STELE_POSTGRES_DSN", "postgres://example")

	cfg, err := LoadFromEnv()
	if err != nil {
		t.Fatalf("LoadFromEnv() error = %v", err)
	}
	if cfg.Migrations.Policy != MigrationPolicyAuto {
		t.Fatalf("migration policy = %q, want %q", cfg.Migrations.Policy, MigrationPolicyAuto)
	}
}

func TestLoadFromEnvParsesMigrationPolicy(t *testing.T) {
	for _, policy := range []string{"auto", "validate", "off"} {
		t.Run(policy, func(t *testing.T) {
			t.Setenv("STELE_MODE", "worker")
			t.Setenv("STELE_POSTGRES_DSN", "postgres://example")
			t.Setenv("STELE_MIGRATION_POLICY", policy)

			cfg, err := LoadFromEnv()
			if err != nil {
				t.Fatalf("LoadFromEnv() error = %v", err)
			}
			if string(cfg.Migrations.Policy) != policy {
				t.Fatalf("migration policy = %q, want %q", cfg.Migrations.Policy, policy)
			}
		})
	}
}

func TestLoadFromEnvRejectsUnknownMigrationPolicy(t *testing.T) {
	t.Setenv("STELE_MODE", "scheduler")
	t.Setenv("STELE_POSTGRES_DSN", "postgres://example")
	t.Setenv("STELE_MIGRATION_POLICY", "downgrade")

	if _, err := LoadFromEnv(); err == nil {
		t.Fatal("LoadFromEnv() error = nil, want invalid migration policy error")
	}
}
