package config

import (
	"fmt"
	"os"
	"strings"
	"time"
)

type Mode string

const (
	ModeAPI       Mode = "api"
	ModeWorker    Mode = "worker"
	ModeScheduler Mode = "scheduler"
)

type Config struct {
	Mode        Mode
	HTTPAddr    string
	PostgresDSN string
	Auth        AuthConfig
	Jobs        JobConfig
}

type AuthConfig struct {
	APIKeys          []string
	AdminAPIKeys     []string
	DefaultTenant    string
	DefaultProject   string
	DefaultNamespace string
}

type JobConfig struct {
	MaintenanceInterval       time.Duration
	WorkerPollInterval        time.Duration
	WorkerErrorBackoff        time.Duration
	SchedulerErrorBackoff     time.Duration
	SummaryCompactionInterval time.Duration
	RetentionInterval         time.Duration
	CleanupInterval           time.Duration
	JobExecutionRetention     time.Duration
}

func LoadFromEnv() (Config, error) {
	mode := Mode(getEnvOrDefault("STELE_MODE", string(ModeAPI)))
	switch mode {
	case ModeAPI, ModeWorker, ModeScheduler:
	default:
		return Config{}, fmt.Errorf("invalid STELE_MODE %q", mode)
	}

	postgresDSN := os.Getenv("STELE_POSTGRES_DSN")
	if postgresDSN == "" {
		return Config{}, fmt.Errorf("STELE_POSTGRES_DSN is required")
	}

	maintenanceInterval, err := loadDurationWithDefault("STELE_JOBS_MAINTENANCE_INTERVAL", 15*time.Minute)
	if err != nil {
		return Config{}, err
	}
	workerPollInterval, err := loadDurationWithDefault("STELE_JOBS_WORKER_POLL_INTERVAL", 5*time.Second)
	if err != nil {
		return Config{}, err
	}
	workerErrorBackoff, err := loadDurationWithDefault("STELE_JOBS_WORKER_ERROR_BACKOFF", 15*time.Second)
	if err != nil {
		return Config{}, err
	}
	schedulerErrorBackoff, err := loadDurationWithDefault("STELE_JOBS_SCHEDULER_ERROR_BACKOFF", 30*time.Second)
	if err != nil {
		return Config{}, err
	}
	summaryCompactionInterval, err := loadDurationWithDefault("STELE_JOBS_SUMMARY_COMPACTION_INTERVAL", maintenanceInterval)
	if err != nil {
		return Config{}, err
	}
	retentionInterval, err := loadDurationWithDefault("STELE_JOBS_RETENTION_INTERVAL", maintenanceInterval)
	if err != nil {
		return Config{}, err
	}
	cleanupInterval, err := loadDurationWithDefault("STELE_JOBS_CLEANUP_INTERVAL", maintenanceInterval)
	if err != nil {
		return Config{}, err
	}
	jobExecutionRetention, err := loadDurationWithDefault("STELE_JOBS_JOB_EXECUTION_RETENTION", 7*24*time.Hour)
	if err != nil {
		return Config{}, err
	}

	return Config{
		Mode:        mode,
		HTTPAddr:    getEnvOrDefault("STELE_HTTP_ADDR", ":8080"),
		PostgresDSN: postgresDSN,
		Auth: AuthConfig{
			APIKeys:          splitCSVEnv("STELE_AUTH_API_KEYS"),
			AdminAPIKeys:     splitCSVEnv("STELE_AUTH_ADMIN_API_KEYS"),
			DefaultTenant:    strings.TrimSpace(os.Getenv("STELE_AUTH_DEFAULT_TENANT")),
			DefaultProject:   strings.TrimSpace(os.Getenv("STELE_AUTH_DEFAULT_PROJECT")),
			DefaultNamespace: strings.TrimSpace(os.Getenv("STELE_AUTH_DEFAULT_NAMESPACE")),
		},
		Jobs: JobConfig{
			MaintenanceInterval:       maintenanceInterval,
			WorkerPollInterval:        workerPollInterval,
			WorkerErrorBackoff:        workerErrorBackoff,
			SchedulerErrorBackoff:     schedulerErrorBackoff,
			SummaryCompactionInterval: summaryCompactionInterval,
			RetentionInterval:         retentionInterval,
			CleanupInterval:           cleanupInterval,
			JobExecutionRetention:     jobExecutionRetention,
		},
	}, nil
}

func getEnvOrDefault(key string, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}

	return fallback
}

func splitCSVEnv(key string) []string {
	raw := os.Getenv(key)
	if raw == "" {
		return nil
	}

	parts := strings.Split(raw, ",")
	values := make([]string, 0, len(parts))
	for _, part := range parts {
		value := strings.TrimSpace(part)
		if value != "" {
			values = append(values, value)
		}
	}

	return values
}

func loadDurationWithDefault(key string, fallback time.Duration) (time.Duration, error) {
	raw := strings.TrimSpace(os.Getenv(key))
	if raw == "" {
		return fallback, nil
	}

	value, err := time.ParseDuration(raw)
	if err != nil {
		return 0, fmt.Errorf("%s is invalid: %w", key, err)
	}

	return value, nil
}
