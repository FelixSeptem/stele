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
	DefaultTenant    string
	DefaultProject   string
	DefaultNamespace string
}

type JobConfig struct {
	MaintenanceInterval time.Duration
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

	return Config{
		Mode:        mode,
		HTTPAddr:    getEnvOrDefault("STELE_HTTP_ADDR", ":8080"),
		PostgresDSN: postgresDSN,
		Auth: AuthConfig{
			APIKeys:          splitCSVEnv("STELE_AUTH_API_KEYS"),
			DefaultTenant:    strings.TrimSpace(os.Getenv("STELE_AUTH_DEFAULT_TENANT")),
			DefaultProject:   strings.TrimSpace(os.Getenv("STELE_AUTH_DEFAULT_PROJECT")),
			DefaultNamespace: strings.TrimSpace(os.Getenv("STELE_AUTH_DEFAULT_NAMESPACE")),
		},
		Jobs: JobConfig{
			MaintenanceInterval: maintenanceInterval,
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
