package config

import (
	"fmt"
	"os"
	"strconv"
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
	Embedding   EmbeddingConfig
	Jobs        JobConfig
}

type AuthConfig struct {
	APIKeys          []string
	AdminAPIKeys     []string
	DefaultTenant    string
	DefaultProject   string
	DefaultNamespace string
}

type EmbeddingRouteConfig struct {
	Provider   string
	Model      string
	Dimensions int
}

type EmbeddingConfig struct {
	DefaultProvider   string
	DefaultModel      string
	DefaultDimensions int
	ClassRoutes       map[string]EmbeddingRouteConfig
}

type JobConfig struct {
	MaintenanceInterval        time.Duration
	WorkerPollInterval         time.Duration
	WorkerErrorBackoff         time.Duration
	SchedulerErrorBackoff      time.Duration
	SummaryCompactionInterval  time.Duration
	RetentionInterval          time.Duration
	CleanupInterval            time.Duration
	JobExecutionRetention      time.Duration
	GovernanceMaxAttempts      int
	GovernanceRetryBackoff     time.Duration
	GovernanceLeaseRenewPeriod time.Duration
	MaintenanceScopeBatchLimit int
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
	governanceMaxAttempts, err := loadIntWithDefault("STELE_JOBS_GOVERNANCE_MAX_ATTEMPTS", 5)
	if err != nil {
		return Config{}, err
	}
	governanceRetryBackoff, err := loadDurationWithDefault("STELE_JOBS_GOVERNANCE_RETRY_BACKOFF", 30*time.Second)
	if err != nil {
		return Config{}, err
	}
	governanceLeaseRenewPeriod, err := loadDurationWithDefault("STELE_JOBS_GOVERNANCE_LEASE_RENEW_INTERVAL", 30*time.Second)
	if err != nil {
		return Config{}, err
	}
	maintenanceScopeBatchLimit, err := loadIntWithDefault("STELE_JOBS_MAINTENANCE_SCOPE_BATCH_LIMIT", 100)
	if err != nil {
		return Config{}, err
	}
	defaultEmbeddingDimensions, err := loadIntWithDefault("STELE_EMBEDDING_DEFAULT_DIMENSIONS", 0)
	if err != nil {
		return Config{}, err
	}
	classRoutes, err := loadEmbeddingClassRoutes("STELE_EMBEDDING_CLASS_ROUTES")
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
		Embedding: EmbeddingConfig{
			DefaultProvider:   strings.TrimSpace(os.Getenv("STELE_EMBEDDING_DEFAULT_PROVIDER")),
			DefaultModel:      strings.TrimSpace(os.Getenv("STELE_EMBEDDING_DEFAULT_MODEL")),
			DefaultDimensions: defaultEmbeddingDimensions,
			ClassRoutes:       classRoutes,
		},
		Jobs: JobConfig{
			MaintenanceInterval:        maintenanceInterval,
			WorkerPollInterval:         workerPollInterval,
			WorkerErrorBackoff:         workerErrorBackoff,
			SchedulerErrorBackoff:      schedulerErrorBackoff,
			SummaryCompactionInterval:  summaryCompactionInterval,
			RetentionInterval:          retentionInterval,
			CleanupInterval:            cleanupInterval,
			JobExecutionRetention:      jobExecutionRetention,
			GovernanceMaxAttempts:      governanceMaxAttempts,
			GovernanceRetryBackoff:     governanceRetryBackoff,
			GovernanceLeaseRenewPeriod: governanceLeaseRenewPeriod,
			MaintenanceScopeBatchLimit: maintenanceScopeBatchLimit,
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

func loadIntWithDefault(key string, fallback int) (int, error) {
	raw := strings.TrimSpace(os.Getenv(key))
	if raw == "" {
		return fallback, nil
	}

	value, err := strconv.Atoi(raw)
	if err != nil {
		return 0, fmt.Errorf("%s is invalid: %w", key, err)
	}

	return value, nil
}

func loadEmbeddingClassRoutes(key string) (map[string]EmbeddingRouteConfig, error) {
	raw := strings.TrimSpace(os.Getenv(key))
	if raw == "" {
		return nil, nil
	}

	routes := make(map[string]EmbeddingRouteConfig)
	for _, entry := range strings.Split(raw, ",") {
		entry = strings.TrimSpace(entry)
		if entry == "" {
			continue
		}

		className, routeRaw, ok := strings.Cut(entry, "=")
		if !ok {
			return nil, fmt.Errorf("%s is invalid: route %q must use class=provider:model:dimensions format", key, entry)
		}
		parts := strings.Split(strings.TrimSpace(routeRaw), ":")
		if len(parts) != 3 {
			return nil, fmt.Errorf("%s is invalid: route %q must use class=provider:model:dimensions format", key, entry)
		}

		dimensions, err := strconv.Atoi(strings.TrimSpace(parts[2]))
		if err != nil {
			return nil, fmt.Errorf("%s is invalid: route %q has invalid dimensions: %w", key, entry, err)
		}
		if dimensions <= 0 {
			return nil, fmt.Errorf("%s is invalid: route %q must use positive dimensions", key, entry)
		}

		className = strings.TrimSpace(className)
		if className == "" {
			return nil, fmt.Errorf("%s is invalid: route %q is missing class name", key, entry)
		}

		routes[className] = EmbeddingRouteConfig{
			Provider:   strings.TrimSpace(parts[0]),
			Model:      strings.TrimSpace(parts[1]),
			Dimensions: dimensions,
		}
		if routes[className].Provider == "" || routes[className].Model == "" {
			return nil, fmt.Errorf("%s is invalid: route %q requires provider and model", key, entry)
		}
	}

	return routes, nil
}
