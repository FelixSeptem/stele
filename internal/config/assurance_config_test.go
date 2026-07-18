package config

import (
	"testing"
	"time"
)

func TestLoadFromEnvParsesAssuranceConformanceSettings(t *testing.T) {
	t.Setenv("STELE_MODE", "scheduler")
	t.Setenv("STELE_POSTGRES_DSN", "postgres://example")
	t.Setenv("STELE_JOBS_MAINTENANCE_INTERVAL", "20m")
	t.Setenv("STELE_JOBS_ASSURANCE_INTERVAL", "7m")
	t.Setenv("STELE_JOBS_CONFORMANCE_INTERVAL", "11m")
	t.Setenv("STELE_JOBS_ASSURANCE_RETENTION", "168h")
	t.Setenv("STELE_JOBS_CONFORMANCE_RETENTION", "336h")
	t.Setenv("STELE_ASSURANCE_CAPACITY_MAX_BACKLOG", "250")
	t.Setenv("STELE_ASSURANCE_CAPACITY_MAX_WORKER_LATENCY", "45s")
	t.Setenv("STELE_ASSURANCE_BACKUP_RESTORE_FRESHNESS", "720h")
	t.Setenv("STELE_ALERT_DELIVERY_MODE", "stdout")
	t.Setenv("STELE_ALERT_MAX_ATTEMPTS", "4")
	t.Setenv("STELE_ALERT_RETRY_BACKOFF", "90s")

	cfg, err := LoadFromEnv()
	if err != nil {
		t.Fatalf("LoadFromEnv() error = %v", err)
	}

	if cfg.Assurance.Cadence != 7*time.Minute {
		t.Fatalf("Assurance.Cadence = %v, want 7m", cfg.Assurance.Cadence)
	}
	if cfg.Assurance.ConformanceCadence != 11*time.Minute {
		t.Fatalf("Assurance.ConformanceCadence = %v, want 11m", cfg.Assurance.ConformanceCadence)
	}
	if cfg.Assurance.HistoryRetention != 168*time.Hour || cfg.Assurance.ConformanceRetention != 336*time.Hour {
		t.Fatalf("Assurance retention = %+v, want configured windows", cfg.Assurance)
	}
	if cfg.Assurance.CapacityMaxBacklog != 250 || cfg.Assurance.CapacityMaxWorkerLatency != 45*time.Second {
		t.Fatalf("Capacity config = %+v, want configured thresholds", cfg.Assurance)
	}
	if cfg.Assurance.BackupRestoreFreshness != 720*time.Hour {
		t.Fatalf("BackupRestoreFreshness = %v, want 720h", cfg.Assurance.BackupRestoreFreshness)
	}
	if cfg.Assurance.Alert.Mode != "stdout" {
		t.Fatalf("Alert.Mode = %q, want stdout", cfg.Assurance.Alert.Mode)
	}
	if cfg.Assurance.AlertMaxAttempts != 4 {
		t.Fatalf("AlertMaxAttempts = %d, want 4", cfg.Assurance.AlertMaxAttempts)
	}
	if cfg.Assurance.AlertRetryBackoff != 90*time.Second {
		t.Fatalf("AlertRetryBackoff = %v, want 90s", cfg.Assurance.AlertRetryBackoff)
	}
}

func TestLoadFromEnvDefaultsAssuranceCadenceToMaintenanceInterval(t *testing.T) {
	t.Setenv("STELE_MODE", "scheduler")
	t.Setenv("STELE_POSTGRES_DSN", "postgres://example")
	t.Setenv("STELE_JOBS_MAINTENANCE_INTERVAL", "20m")

	cfg, err := LoadFromEnv()
	if err != nil {
		t.Fatalf("LoadFromEnv() error = %v", err)
	}

	if cfg.Assurance.Cadence != 20*time.Minute {
		t.Fatalf("Assurance.Cadence = %v, want maintenance interval fallback", cfg.Assurance.Cadence)
	}
	if cfg.Assurance.ConformanceCadence != 20*time.Minute {
		t.Fatalf("Assurance.ConformanceCadence = %v, want maintenance interval fallback", cfg.Assurance.ConformanceCadence)
	}
	if cfg.Assurance.Alert.Mode != "disabled" {
		t.Fatalf("Alert.Mode = %q, want disabled default", cfg.Assurance.Alert.Mode)
	}
	if cfg.Assurance.AlertMaxAttempts != 5 {
		t.Fatalf("AlertMaxAttempts = %d, want 5", cfg.Assurance.AlertMaxAttempts)
	}
	if cfg.Assurance.AlertRetryBackoff != 30*time.Second {
		t.Fatalf("AlertRetryBackoff = %v, want 30s", cfg.Assurance.AlertRetryBackoff)
	}
}

func TestLoadFromEnvRejectsUnsafeWebhookSettings(t *testing.T) {
	t.Setenv("STELE_MODE", "worker")
	t.Setenv("STELE_POSTGRES_DSN", "postgres://example")
	t.Setenv("STELE_ALERT_DELIVERY_MODE", "webhook")
	t.Setenv("STELE_ALERT_WEBHOOK_URL", "http://169.254.169.254/latest/meta-data")

	if _, err := LoadFromEnv(); err == nil {
		t.Fatal("LoadFromEnv() error = nil, want unsafe webhook validation error")
	}
}

func TestLoadFromEnvRejectsIncompleteOrUnsafeAlertSettings(t *testing.T) {
	tests := []struct {
		name string
		env  map[string]string
	}{
		{
			name: "webhook missing url",
			env: map[string]string{
				"STELE_ALERT_DELIVERY_MODE": "webhook",
			},
		},
		{
			name: "http webhook without local override",
			env: map[string]string{
				"STELE_ALERT_DELIVERY_MODE": "webhook",
				"STELE_ALERT_WEBHOOK_URL":   "http://localhost:8081/alerts",
			},
		},
		{
			name: "rejected forwarded header",
			env: map[string]string{
				"STELE_ALERT_DELIVERY_MODE":   "webhook",
				"STELE_ALERT_WEBHOOK_URL":     "https://alerts.example.com/hook",
				"STELE_ALERT_WEBHOOK_HEADERS": "X-Forwarded-Host=evil.example",
			},
		},
		{
			name: "short delivery timeout",
			env: map[string]string{
				"STELE_ALERT_DELIVERY_TIMEOUT": "500ms",
			},
		},
		{
			name: "oversized payload limit",
			env: map[string]string{
				"STELE_ALERT_MAX_PAYLOAD_BYTES": "65537",
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv("STELE_MODE", "worker")
			t.Setenv("STELE_POSTGRES_DSN", "postgres://example")
			for key, value := range tt.env {
				t.Setenv(key, value)
			}

			if _, err := LoadFromEnv(); err == nil {
				t.Fatal("LoadFromEnv() error = nil, want alert validation error")
			}
		})
	}
}

func TestLoadFromEnvRejectsUnsafeAssuranceRuntimeBounds(t *testing.T) {
	tests := []struct {
		name string
		key  string
		val  string
	}{
		{name: "short assurance retention", key: "STELE_JOBS_ASSURANCE_RETENTION", val: "30m"},
		{name: "short conformance retention", key: "STELE_JOBS_CONFORMANCE_RETENTION", val: "30m"},
		{name: "short backup freshness", key: "STELE_ASSURANCE_BACKUP_RESTORE_FRESHNESS", val: "30m"},
		{name: "short capacity latency", key: "STELE_ASSURANCE_CAPACITY_MAX_WORKER_LATENCY", val: "500ms"},
		{name: "zero alert attempts", key: "STELE_ALERT_MAX_ATTEMPTS", val: "0"},
		{name: "short alert retry backoff", key: "STELE_ALERT_RETRY_BACKOFF", val: "500ms"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv("STELE_MODE", "scheduler")
			t.Setenv("STELE_POSTGRES_DSN", "postgres://example")
			t.Setenv(tt.key, tt.val)

			if _, err := LoadFromEnv(); err == nil {
				t.Fatalf("LoadFromEnv() error = nil, want validation error for %s", tt.key)
			}
		})
	}
}
