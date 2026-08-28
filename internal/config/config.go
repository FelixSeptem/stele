package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/FelixSeptem/stele/internal/assurance"
)

type Mode string

const (
	ModeAPI       Mode = "api"
	ModeWorker    Mode = "worker"
	ModeScheduler Mode = "scheduler"
)

type MigrationPolicy string

const (
	MigrationPolicyAuto     MigrationPolicy = "auto"
	MigrationPolicyValidate MigrationPolicy = "validate"
	MigrationPolicyOff      MigrationPolicy = "off"
)

type MigrationConfig struct {
	Policy MigrationPolicy
}

type Config struct {
	Mode        Mode
	HTTPAddr    string
	HTTP        HTTPConfig
	PostgresDSN string
	Migrations  MigrationConfig
	Auth        AuthConfig
	Embedding   EmbeddingConfig
	Jobs        JobConfig
	Assurance   AssuranceConfig
}

type HTTPConfig struct {
	MaxRequestBodyBytes int64
	MaxHeaderBytes      int
	ReadHeaderTimeout   time.Duration
	ReadTimeout         time.Duration
	WriteTimeout        time.Duration
	IdleTimeout         time.Duration
	ShutdownTimeout     time.Duration
}

type AuthConfig struct {
	APIKeys           []string
	AdminAPIKeys      []string
	BootstrapAdminKey string
	DefaultTenant     string
	DefaultProject    string
	DefaultNamespace  string
}

type EmbeddingRouteConfig struct {
	Provider   string
	Model      string
	Dimensions int
}

type OpenAIEmbeddingProviderConfig struct {
	APIKey  string
	BaseURL string
	Timeout time.Duration
}

type EmbeddingConfig struct {
	DefaultProvider   string
	DefaultModel      string
	DefaultDimensions int
	ClassRoutes       map[string]EmbeddingRouteConfig
	OpenAI            OpenAIEmbeddingProviderConfig
}

type JobConfig struct {
	MaintenanceInterval              time.Duration
	WorkerPollInterval               time.Duration
	WorkerErrorBackoff               time.Duration
	SchedulerErrorBackoff            time.Duration
	SummaryCompactionInterval        time.Duration
	RetentionInterval                time.Duration
	CleanupInterval                  time.Duration
	DerivedInsightDerivationInterval time.Duration
	DerivedInsightBatchSize          int
	DerivedInsightMinimumEvidence    int
	JobExecutionRetention            time.Duration
	GovernanceMaxAttempts            int
	GovernanceRetryBackoff           time.Duration
	GovernanceLeaseRenewPeriod       time.Duration
	MaintenanceScopeBatchLimit       int
	WorkflowMaintenanceEnabled       bool
	WorkflowDiagnosticCadence        time.Duration
	WorkflowStaleRunWindow           time.Duration
	WorkflowDiagnosticScanLimit      int
	WorkflowNextActionRefreshLimit   int
	WorkflowHistoryRetention         time.Duration
}

type AssuranceConfig struct {
	Cadence                  time.Duration
	ConformanceCadence       time.Duration
	HistoryRetention         time.Duration
	ConformanceRetention     time.Duration
	IncidentFreshnessWindow  time.Duration
	CapacityMaxBacklog       int
	CapacityMaxWorkerLatency time.Duration
	BackupRestoreFreshness   time.Duration
	Alert                    assurance.AlertDeliveryConfig
	AlertMaxAttempts         int
	AlertRetryBackoff        time.Duration
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

	migrationPolicy := MigrationPolicy(getEnvOrDefault("STELE_DATABASE_MIGRATION_POLICY", string(MigrationPolicyAuto)))
	switch migrationPolicy {
	case MigrationPolicyAuto, MigrationPolicyValidate, MigrationPolicyOff:
	default:
		return Config{}, fmt.Errorf("invalid STELE_DATABASE_MIGRATION_POLICY %q", migrationPolicy)
	}

	maxRequestBodyBytes, err := loadIntWithDefault("STELE_HTTP_MAX_REQUEST_BODY_BYTES", 1<<20)
	if err != nil {
		return Config{}, err
	}
	maxHeaderBytes, err := loadIntWithDefault("STELE_HTTP_MAX_HEADER_BYTES", 1<<20)
	if err != nil {
		return Config{}, err
	}
	readHeaderTimeout, err := loadDurationWithDefault("STELE_HTTP_READ_HEADER_TIMEOUT", 5*time.Second)
	if err != nil {
		return Config{}, err
	}
	readTimeout, err := loadDurationWithDefault("STELE_HTTP_READ_TIMEOUT", 30*time.Second)
	if err != nil {
		return Config{}, err
	}
	writeTimeout, err := loadDurationWithDefault("STELE_HTTP_WRITE_TIMEOUT", 30*time.Second)
	if err != nil {
		return Config{}, err
	}
	idleTimeout, err := loadDurationWithDefault("STELE_HTTP_IDLE_TIMEOUT", 60*time.Second)
	if err != nil {
		return Config{}, err
	}
	shutdownTimeout, err := loadDurationWithDefault("STELE_HTTP_SHUTDOWN_TIMEOUT", 20*time.Second)
	if err != nil {
		return Config{}, err
	}
	if maxRequestBodyBytes <= 0 || maxHeaderBytes <= 0 || readHeaderTimeout <= 0 || readTimeout <= 0 || writeTimeout <= 0 || idleTimeout <= 0 || shutdownTimeout <= 0 {
		return Config{}, fmt.Errorf("HTTP runtime limits must be greater than zero")
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
	derivedInsightDerivationInterval, err := loadDurationWithDefault("STELE_JOBS_DERIVED_INSIGHT_DERIVATION_INTERVAL", maintenanceInterval)
	if err != nil {
		return Config{}, err
	}
	derivedInsightBatchSize, err := loadIntWithDefault("STELE_JOBS_DERIVED_INSIGHT_BATCH_SIZE", 100)
	if err != nil {
		return Config{}, err
	}
	derivedInsightMinimumEvidence, err := loadIntWithDefault("STELE_JOBS_DERIVED_INSIGHT_MINIMUM_EVIDENCE", 2)
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
	workflowDiagnosticCadence, err := loadDurationWithDefault("STELE_WORKFLOW_DIAGNOSTIC_INTERVAL", maintenanceInterval)
	if err != nil {
		return Config{}, err
	}
	workflowStaleRunWindow, err := loadDurationWithDefault("STELE_WORKFLOW_STALE_RUN_WINDOW", 24*time.Hour)
	if err != nil {
		return Config{}, err
	}
	workflowDiagnosticScanLimit, err := loadIntWithDefault("STELE_WORKFLOW_DIAGNOSTIC_SCAN_LIMIT", 100)
	if err != nil {
		return Config{}, err
	}
	workflowNextActionRefreshLimit, err := loadIntWithDefault("STELE_WORKFLOW_NEXT_ACTION_REFRESH_LIMIT", 100)
	if err != nil {
		return Config{}, err
	}
	workflowHistoryRetention, err := loadDurationWithDefault("STELE_WORKFLOW_HISTORY_RETENTION", 30*24*time.Hour)
	if err != nil {
		return Config{}, err
	}
	if workflowDiagnosticCadence <= 0 || workflowStaleRunWindow < time.Minute || workflowDiagnosticScanLimit <= 0 || workflowNextActionRefreshLimit <= 0 || workflowHistoryRetention < time.Hour {
		return Config{}, fmt.Errorf("workflow maintenance settings are invalid")
	}
	assuranceCadence, err := loadDurationWithDefault("STELE_JOBS_ASSURANCE_INTERVAL", maintenanceInterval)
	if err != nil {
		return Config{}, err
	}
	conformanceCadence, err := loadDurationWithDefault("STELE_JOBS_CONFORMANCE_INTERVAL", maintenanceInterval)
	if err != nil {
		return Config{}, err
	}
	assuranceRetention, err := loadDurationWithDefault("STELE_JOBS_ASSURANCE_RETENTION", 7*24*time.Hour)
	if err != nil {
		return Config{}, err
	}
	conformanceRetention, err := loadDurationWithDefault("STELE_JOBS_CONFORMANCE_RETENTION", 14*24*time.Hour)
	if err != nil {
		return Config{}, err
	}
	incidentFreshnessWindow, err := loadDurationWithDefault("STELE_ASSURANCE_INCIDENT_FRESHNESS", 30*time.Minute)
	if err != nil {
		return Config{}, err
	}
	capacityMaxBacklog, err := loadIntWithDefault("STELE_ASSURANCE_CAPACITY_MAX_BACKLOG", 1000)
	if err != nil {
		return Config{}, err
	}
	capacityMaxWorkerLatency, err := loadDurationWithDefault("STELE_ASSURANCE_CAPACITY_MAX_WORKER_LATENCY", 2*time.Minute)
	if err != nil {
		return Config{}, err
	}
	backupRestoreFreshness, err := loadDurationWithDefault("STELE_ASSURANCE_BACKUP_RESTORE_FRESHNESS", 30*24*time.Hour)
	if err != nil {
		return Config{}, err
	}
	alertTimeout, err := loadDurationWithDefault("STELE_ALERT_DELIVERY_TIMEOUT", 10*time.Second)
	if err != nil {
		return Config{}, err
	}
	alertMaxPayloadBytes, err := loadIntWithDefault("STELE_ALERT_MAX_PAYLOAD_BYTES", 64*1024)
	if err != nil {
		return Config{}, err
	}
	alertMaxAttempts, err := loadIntWithDefault("STELE_ALERT_MAX_ATTEMPTS", 5)
	if err != nil {
		return Config{}, err
	}
	alertRetryBackoff, err := loadDurationWithDefault("STELE_ALERT_RETRY_BACKOFF", 30*time.Second)
	if err != nil {
		return Config{}, err
	}
	alertConfig := assurance.AlertDeliveryConfig{
		Mode:               assurance.AlertAdapterKind(getEnvOrDefault("STELE_ALERT_DELIVERY_MODE", string(assurance.AlertAdapterDisabled))),
		WebhookURL:         strings.TrimSpace(os.Getenv("STELE_ALERT_WEBHOOK_URL")),
		WebhookHeaders:     loadHeaderMap("STELE_ALERT_WEBHOOK_HEADERS"),
		AllowInsecureLocal: loadBoolEnv("STELE_ALERT_WEBHOOK_ALLOW_INSECURE_LOCAL"),
		Timeout:            alertTimeout,
		MaxPayloadBytes:    alertMaxPayloadBytes,
	}
	if err := alertConfig.Validate(); err != nil {
		return Config{}, err
	}
	if assuranceCadence <= 0 || conformanceCadence <= 0 {
		return Config{}, fmt.Errorf("assurance and conformance cadence must be greater than zero")
	}
	if assuranceRetention < time.Hour || conformanceRetention < time.Hour {
		return Config{}, fmt.Errorf("assurance and conformance retention must be at least 1h")
	}
	if incidentFreshnessWindow <= 0 || backupRestoreFreshness < time.Hour {
		return Config{}, fmt.Errorf("assurance freshness windows are invalid")
	}
	if capacityMaxBacklog < 0 || capacityMaxWorkerLatency < time.Second {
		return Config{}, fmt.Errorf("assurance capacity thresholds are invalid")
	}
	if alertMaxAttempts <= 0 || alertRetryBackoff < time.Second {
		return Config{}, fmt.Errorf("alert retry settings are invalid")
	}
	defaultEmbeddingDimensions, err := loadIntWithDefault("STELE_EMBEDDING_DEFAULT_DIMENSIONS", 0)
	if err != nil {
		return Config{}, err
	}
	openAIEmbeddingTimeout, err := loadDurationWithDefault("STELE_EMBEDDING_OPENAI_TIMEOUT", 30*time.Second)
	if err != nil {
		return Config{}, err
	}
	classRoutes, err := loadEmbeddingClassRoutes("STELE_EMBEDDING_CLASS_ROUTES")
	if err != nil {
		return Config{}, err
	}
	legacyAPIKeys := splitCSVEnv("STELE_AUTH_API_KEYS")
	legacyAdminAPIKeys := splitCSVEnv("STELE_AUTH_ADMIN_API_KEYS")
	if len(legacyAPIKeys) > 0 || len(legacyAdminAPIKeys) > 0 {
		return Config{}, fmt.Errorf("STELE_AUTH_API_KEYS and STELE_AUTH_ADMIN_API_KEYS are deprecated; configure STELE_AUTH_BOOTSTRAP_ADMIN_KEY to create the first durable principal")
	}
	bootstrapAdminKey := strings.TrimSpace(os.Getenv("STELE_AUTH_BOOTSTRAP_ADMIN_KEY"))
	defaultTenant := strings.TrimSpace(os.Getenv("STELE_AUTH_DEFAULT_TENANT"))
	defaultProject := strings.TrimSpace(os.Getenv("STELE_AUTH_DEFAULT_PROJECT"))
	defaultNamespace := strings.TrimSpace(os.Getenv("STELE_AUTH_DEFAULT_NAMESPACE"))
	if bootstrapAdminKey != "" && (defaultTenant == "" || defaultProject == "" || defaultNamespace == "") {
		return Config{}, fmt.Errorf("STELE_AUTH_BOOTSTRAP_ADMIN_KEY requires STELE_AUTH_DEFAULT_TENANT, STELE_AUTH_DEFAULT_PROJECT, and STELE_AUTH_DEFAULT_NAMESPACE")
	}

	return Config{
		Mode:     mode,
		HTTPAddr: getEnvOrDefault("STELE_HTTP_ADDR", ":8080"),
		HTTP: HTTPConfig{
			MaxRequestBodyBytes: int64(maxRequestBodyBytes),
			MaxHeaderBytes:      maxHeaderBytes,
			ReadHeaderTimeout:   readHeaderTimeout,
			ReadTimeout:         readTimeout,
			WriteTimeout:        writeTimeout,
			IdleTimeout:         idleTimeout,
			ShutdownTimeout:     shutdownTimeout,
		},
		PostgresDSN: postgresDSN,
		Migrations:  MigrationConfig{Policy: migrationPolicy},
		Auth: AuthConfig{
			BootstrapAdminKey: bootstrapAdminKey,
			DefaultTenant:     defaultTenant,
			DefaultProject:    defaultProject,
			DefaultNamespace:  defaultNamespace,
		},
		Embedding: EmbeddingConfig{
			DefaultProvider:   strings.TrimSpace(os.Getenv("STELE_EMBEDDING_DEFAULT_PROVIDER")),
			DefaultModel:      strings.TrimSpace(os.Getenv("STELE_EMBEDDING_DEFAULT_MODEL")),
			DefaultDimensions: defaultEmbeddingDimensions,
			ClassRoutes:       classRoutes,
			OpenAI: OpenAIEmbeddingProviderConfig{
				APIKey:  strings.TrimSpace(os.Getenv("STELE_EMBEDDING_OPENAI_API_KEY")),
				BaseURL: strings.TrimSpace(getEnvOrDefault("STELE_EMBEDDING_OPENAI_BASE_URL", "https://api.openai.com/v1")),
				Timeout: openAIEmbeddingTimeout,
			},
		},
		Jobs: JobConfig{
			MaintenanceInterval:              maintenanceInterval,
			WorkerPollInterval:               workerPollInterval,
			WorkerErrorBackoff:               workerErrorBackoff,
			SchedulerErrorBackoff:            schedulerErrorBackoff,
			SummaryCompactionInterval:        summaryCompactionInterval,
			RetentionInterval:                retentionInterval,
			CleanupInterval:                  cleanupInterval,
			DerivedInsightDerivationInterval: derivedInsightDerivationInterval,
			DerivedInsightBatchSize:          derivedInsightBatchSize,
			DerivedInsightMinimumEvidence:    derivedInsightMinimumEvidence,
			JobExecutionRetention:            jobExecutionRetention,
			GovernanceMaxAttempts:            governanceMaxAttempts,
			GovernanceRetryBackoff:           governanceRetryBackoff,
			GovernanceLeaseRenewPeriod:       governanceLeaseRenewPeriod,
			MaintenanceScopeBatchLimit:       maintenanceScopeBatchLimit,
			WorkflowMaintenanceEnabled:       loadBoolEnv("STELE_WORKFLOW_MAINTENANCE_ENABLED"),
			WorkflowDiagnosticCadence:        workflowDiagnosticCadence,
			WorkflowStaleRunWindow:           workflowStaleRunWindow,
			WorkflowDiagnosticScanLimit:      workflowDiagnosticScanLimit,
			WorkflowNextActionRefreshLimit:   workflowNextActionRefreshLimit,
			WorkflowHistoryRetention:         workflowHistoryRetention,
		},
		Assurance: AssuranceConfig{
			Cadence:                  assuranceCadence,
			ConformanceCadence:       conformanceCadence,
			HistoryRetention:         assuranceRetention,
			ConformanceRetention:     conformanceRetention,
			IncidentFreshnessWindow:  incidentFreshnessWindow,
			CapacityMaxBacklog:       capacityMaxBacklog,
			CapacityMaxWorkerLatency: capacityMaxWorkerLatency,
			BackupRestoreFreshness:   backupRestoreFreshness,
			Alert:                    alertConfig,
			AlertMaxAttempts:         alertMaxAttempts,
			AlertRetryBackoff:        alertRetryBackoff,
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

func loadBoolEnv(key string) bool {
	raw := strings.TrimSpace(strings.ToLower(os.Getenv(key)))
	return raw == "1" || raw == "true" || raw == "yes"
}

func loadHeaderMap(key string) map[string]string {
	raw := strings.TrimSpace(os.Getenv(key))
	if raw == "" {
		return nil
	}
	headers := make(map[string]string)
	for _, entry := range strings.Split(raw, ",") {
		name, value, ok := strings.Cut(entry, "=")
		if !ok {
			continue
		}
		name = strings.TrimSpace(name)
		value = strings.TrimSpace(value)
		if name != "" {
			headers[name] = value
		}
	}
	return headers
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
