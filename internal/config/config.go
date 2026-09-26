package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/FelixSeptem/stele/internal/assurance"
	"github.com/FelixSeptem/stele/internal/provider"
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
	Mode                                Mode
	HTTPAddr                            string
	HTTP                                HTTPConfig
	PostgresDSN                         string
	Migrations                          MigrationConfig
	ContextProjectionConsumptionEnabled bool
	Auth                                AuthConfig
	Embedding                           EmbeddingConfig
	Reranker                            RerankerConfig
	Jobs                                JobConfig
	Assurance                           AssuranceConfig
	QueryAnalysis                       QueryAnalysisConfig
	GraphTraversal                      GraphTraversalConfig
	Evaluation                          EvaluationConfig
	Provider                            ProviderConfig
	ContextCalibration                  ContextCalibrationConfig
	MCP                                 MCPConfig
}

// MCPConfig contains the optional API-mode MCP adapter guard and protocol
// safety limits. MCP is deliberately disabled unless explicitly enabled.
type MCPConfig struct {
	Enabled         bool
	Path            string
	MaxQueryBytes   int
	MaxPayloadBytes int
	MaxResults      int
	MaxIDs          int
}

type ContextCalibrationConfig struct {
	Enabled             bool
	MaxSummaryAge       time.Duration
	MinimumEvidence     int
	ConfidenceThreshold float64
	DecayWindow         time.Duration
	ContributionCap     float64
	MaxCandidates       int
	MaxContextItems     int
	MaxElapsed          time.Duration
}

type GraphTraversalConfig struct {
	MaxHops            int
	MaxSeeds           int
	MaxEdgesPerHop     int
	MaxPathsPerSeed    int
	MaxPathsPerRequest int
	MaxCandidates      int
	MaxElapsed         time.Duration
}

type ProviderConfig struct {
	Enabled         bool
	SchemaVersions  []string
	Limits          provider.ProviderLimits
	BindingLifetime time.Duration
}

type EvaluationConfig struct {
	OwnedDSN        string
	ProviderProfile string
}

type QueryAnalysisConfig struct {
	MaxQueryBytes          int
	MaxHints               int
	MaxSignals             int
	MaxSubqueries          int
	MaxTermBytes           int
	MaxSubqueryBytes       int
	MaxAnalysisWork        int
	MaxCandidatesPerSignal int
	MaxAggregateCandidates int
	MaxElapsed             time.Duration
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
	APIKey         string
	BaseURL        string
	Timeout        time.Duration
	OmitDimensions bool
}

type EmbeddingConfig struct {
	DefaultProvider   string
	DefaultModel      string
	DefaultDimensions int
	ClassRoutes       map[string]EmbeddingRouteConfig
	OpenAI            OpenAIEmbeddingProviderConfig
}

type RerankerConfig struct {
	Enabled       bool
	Mode          string
	Provider      string
	Endpoint      string
	APIKey        string
	Model         string
	Timeout       time.Duration
	MaxCandidates int
	MaxTextBytes  int
}

type JobConfig struct {
	DurableMaintenanceEnabled        bool
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
	MaintenanceLeaseDuration         time.Duration
	MaintenanceLeaseRenewInterval    time.Duration
	MaintenanceMaxAttempts           int
	MaintenanceRetryBackoff          time.Duration
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
	contextProjectionConsumptionEnabled := loadBoolEnv("STELE_CONTEXT_PROJECTION_CONSUMPTION_ENABLED")
	queryAnalysis, err := loadQueryAnalysisConfig()
	if err != nil {
		return Config{}, err
	}
	graphTraversal, err := loadGraphTraversalConfig()
	if err != nil {
		return Config{}, err
	}
	contextCalibration, err := loadContextCalibrationConfig()
	if err != nil {
		return Config{}, err
	}
	evaluationConfig, err := loadEvaluationConfig(postgresDSN)
	if err != nil {
		return Config{}, err
	}
	providerConfig, err := loadProviderConfig()
	if err != nil {
		return Config{}, err
	}
	mcpConfig, err := loadMCPConfig(mode)
	if err != nil {
		return Config{}, err
	}

	// Accept the short policy name introduced by the migration contract while
	// retaining the database-qualified name used by the product configuration.
	// The short name takes precedence when both are supplied so callers can
	// explicitly override a deployment-wide database default.
	migrationPolicyRaw := strings.TrimSpace(os.Getenv("STELE_MIGRATION_POLICY"))
	if migrationPolicyRaw == "" {
		migrationPolicyRaw = getEnvOrDefault("STELE_DATABASE_MIGRATION_POLICY", string(MigrationPolicyAuto))
	}
	migrationPolicy := MigrationPolicy(migrationPolicyRaw)
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
	maintenanceLeaseDuration, err := loadDurationWithDefault("STELE_JOBS_MAINTENANCE_LEASE_DURATION", time.Minute)
	if err != nil {
		return Config{}, err
	}
	maintenanceLeaseRenewInterval, err := loadDurationWithDefault("STELE_JOBS_MAINTENANCE_LEASE_RENEW_INTERVAL", 30*time.Second)
	if err != nil {
		return Config{}, err
	}
	maintenanceMaxAttempts, err := loadIntWithDefault("STELE_JOBS_MAINTENANCE_MAX_ATTEMPTS", 5)
	if err != nil {
		return Config{}, err
	}
	maintenanceRetryBackoff, err := loadDurationWithDefault("STELE_JOBS_MAINTENANCE_RETRY_BACKOFF", 30*time.Second)
	if err != nil {
		return Config{}, err
	}
	if maintenanceLeaseDuration < time.Second || maintenanceLeaseRenewInterval <= 0 || maintenanceLeaseRenewInterval >= maintenanceLeaseDuration || maintenanceMaxAttempts < 1 || maintenanceMaxAttempts > 20 || maintenanceRetryBackoff < time.Second {
		return Config{}, fmt.Errorf("maintenance lease and retry settings are invalid")
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
	openAIEmbeddingSendDimensions := loadBoolEnvWithDefault("STELE_EMBEDDING_OPENAI_SEND_DIMENSIONS", true)
	rerankerTimeout, err := loadDurationWithDefault("STELE_RERANK_TIMEOUT", 30*time.Second)
	if err != nil {
		return Config{}, err
	}
	rerankerMaxCandidates, err := loadIntWithDefault("STELE_RERANK_MAX_CANDIDATES", 50)
	if err != nil {
		return Config{}, err
	}
	rerankerMaxTextBytes, err := loadIntWithDefault("STELE_RERANK_MAX_TEXT_BYTES", 8192)
	if err != nil {
		return Config{}, err
	}
	rerankerConfig := RerankerConfig{
		Enabled: loadBoolEnv("STELE_RERANK_ENABLED"), Mode: strings.TrimSpace(os.Getenv("STELE_RERANK_MODE")), Provider: strings.TrimSpace(os.Getenv("STELE_RERANK_PROVIDER")),
		Endpoint: strings.TrimSpace(os.Getenv("STELE_RERANK_ENDPOINT")), APIKey: strings.TrimSpace(os.Getenv("STELE_RERANK_API_KEY")), Model: strings.TrimSpace(os.Getenv("STELE_RERANK_MODEL")),
		Timeout: rerankerTimeout, MaxCandidates: rerankerMaxCandidates, MaxTextBytes: rerankerMaxTextBytes,
	}
	if rerankerConfig.Mode == "" && rerankerConfig.Enabled {
		rerankerConfig.Mode = "shadow"
	}
	if rerankerConfig.Mode != "" && rerankerConfig.Mode != "disabled" && rerankerConfig.Mode != "diagnostics_only" && rerankerConfig.Mode != "shadow" && rerankerConfig.Mode != "active_for_scope" {
		return Config{}, fmt.Errorf("invalid reranker mode %q", rerankerConfig.Mode)
	}
	if rerankerConfig.Enabled {
		if rerankerConfig.Provider == "" || rerankerConfig.Endpoint == "" || rerankerConfig.Model == "" || rerankerConfig.Timeout <= 0 || rerankerConfig.MaxCandidates <= 0 || rerankerConfig.MaxTextBytes <= 0 {
			return Config{}, fmt.Errorf("enabled reranker configuration is incomplete or invalid")
		}
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
		PostgresDSN:                         postgresDSN,
		Migrations:                          MigrationConfig{Policy: migrationPolicy},
		ContextProjectionConsumptionEnabled: contextProjectionConsumptionEnabled,
		QueryAnalysis:                       queryAnalysis,
		GraphTraversal:                      graphTraversal,
		ContextCalibration:                  contextCalibration,
		MCP:                                 mcpConfig,
		Evaluation:                          evaluationConfig,
		Provider:                            providerConfig,
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
				APIKey:         strings.TrimSpace(os.Getenv("STELE_EMBEDDING_OPENAI_API_KEY")),
				BaseURL:        strings.TrimSpace(getEnvOrDefault("STELE_EMBEDDING_OPENAI_BASE_URL", "https://api.openai.com/v1")),
				Timeout:        openAIEmbeddingTimeout,
				OmitDimensions: !openAIEmbeddingSendDimensions,
			},
		},
		Reranker: rerankerConfig,
		Jobs: JobConfig{
			DurableMaintenanceEnabled:        loadBoolEnv("STELE_JOBS_DURABLE_MAINTENANCE_ENABLED"),
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
			MaintenanceLeaseDuration:         maintenanceLeaseDuration,
			MaintenanceLeaseRenewInterval:    maintenanceLeaseRenewInterval,
			MaintenanceMaxAttempts:           maintenanceMaxAttempts,
			MaintenanceRetryBackoff:          maintenanceRetryBackoff,
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

func loadProviderConfig() (ProviderConfig, error) {
	versions := splitCSVEnv("STELE_PROVIDER_SCHEMA_VERSIONS")
	if len(versions) == 0 {
		versions = []string{"provider-v1"}
	}
	if len(versions) > 16 {
		return ProviderConfig{}, fmt.Errorf("provider schema versions exceed limit")
	}
	seen := make(map[string]struct{}, len(versions))
	for _, version := range versions {
		if !safeEvaluationProfile(version) || len(version) > provider.MaxSchemaVersionBytes {
			return ProviderConfig{}, fmt.Errorf("provider schema version %q is invalid", version)
		}
		if _, ok := seen[version]; ok {
			return ProviderConfig{}, fmt.Errorf("provider schema version %q is duplicated", version)
		}
		seen[version] = struct{}{}
	}
	limits := provider.ProviderLimits{}
	var err error
	if limits.MaxEventBytes, err = loadIntWithDefault("STELE_PROVIDER_MAX_EVENT_BYTES", 1<<20); err != nil {
		return ProviderConfig{}, err
	}
	if limits.MaxIntentBytes, err = loadIntWithDefault("STELE_PROVIDER_MAX_INTENT_BYTES", 1<<20); err != nil {
		return ProviderConfig{}, err
	}
	if limits.MaxRetrievalResults, err = loadIntWithDefault("STELE_PROVIDER_MAX_RETRIEVAL_RESULTS", 100); err != nil {
		return ProviderConfig{}, err
	}
	if limits.MaxContextBytes, err = loadIntWithDefault("STELE_PROVIDER_MAX_CONTEXT_BYTES", 1<<20); err != nil {
		return ProviderConfig{}, err
	}
	if limits.MaxCitations, err = loadIntWithDefault("STELE_PROVIDER_MAX_CITATIONS", 100); err != nil {
		return ProviderConfig{}, err
	}
	if limits.MaxMetadataBytes, err = loadIntWithDefault("STELE_PROVIDER_MAX_METADATA_BYTES", 64<<10); err != nil {
		return ProviderConfig{}, err
	}
	if err := limits.Validate(); err != nil {
		return ProviderConfig{}, err
	}
	lifetime, err := loadDurationWithDefault("STELE_PROVIDER_BINDING_LIFETIME", time.Hour)
	if err != nil {
		return ProviderConfig{}, err
	}
	if lifetime < time.Minute || lifetime > 24*time.Hour {
		return ProviderConfig{}, fmt.Errorf("provider binding lifetime must be between 1m and 24h")
	}
	return ProviderConfig{Enabled: loadBoolEnv("STELE_PROVIDER_ENABLED"), SchemaVersions: versions, Limits: limits, BindingLifetime: lifetime}, nil
}

func loadMCPConfig(mode Mode) (MCPConfig, error) {
	path := "/mcp"
	if raw, ok := os.LookupEnv("STELE_MCP_PATH"); ok {
		path = strings.TrimSpace(raw)
	}
	maxQueryBytes, err := loadIntWithDefault("STELE_MCP_MAX_QUERY_BYTES", 4096)
	if err != nil {
		return MCPConfig{}, err
	}
	maxPayloadBytes, err := loadIntWithDefault("STELE_MCP_MAX_PAYLOAD_BYTES", 64<<10)
	if err != nil {
		return MCPConfig{}, err
	}
	maxResults, err := loadIntWithDefault("STELE_MCP_MAX_RESULTS", 50)
	if err != nil {
		return MCPConfig{}, err
	}
	maxIDs, err := loadIntWithDefault("STELE_MCP_MAX_IDS", 100)
	if err != nil {
		return MCPConfig{}, err
	}
	enabled := loadBoolEnv("STELE_MCP_ENABLED")
	if enabled && mode != ModeAPI {
		return MCPConfig{}, fmt.Errorf("STELE_MCP_ENABLED requires STELE_MODE=api")
	}
	if path == "" || len(path) > 128 || !strings.HasPrefix(path, "/") || strings.ContainsAny(path, "?#\r\n") {
		return MCPConfig{}, fmt.Errorf("STELE_MCP_PATH must be a non-empty absolute path without query or fragment")
	}
	if maxQueryBytes < 1 || maxQueryBytes > 16<<10 || maxPayloadBytes < 1 || maxPayloadBytes > 1<<20 || maxResults < 1 || maxResults > 100 || maxIDs < 1 || maxIDs > 1000 {
		return MCPConfig{}, fmt.Errorf("MCP safety limits are invalid")
	}
	return MCPConfig{Enabled: enabled, Path: path, MaxQueryBytes: maxQueryBytes, MaxPayloadBytes: maxPayloadBytes, MaxResults: maxResults, MaxIDs: maxIDs}, nil
}

func loadQueryAnalysisConfig() (QueryAnalysisConfig, error) {
	q := QueryAnalysisConfig{
		MaxQueryBytes: 4096, MaxHints: 4, MaxSignals: 8, MaxSubqueries: 4,
		MaxTermBytes: 256, MaxSubqueryBytes: 1024, MaxAnalysisWork: 7,
		MaxCandidatesPerSignal: 50, MaxAggregateCandidates: 200, MaxElapsed: 250 * time.Millisecond,
	}
	var err error
	if q.MaxQueryBytes, err = loadIntWithDefault("STELE_QUERY_ANALYSIS_MAX_QUERY_BYTES", q.MaxQueryBytes); err != nil {
		return QueryAnalysisConfig{}, err
	}
	if q.MaxHints, err = loadIntWithDefault("STELE_QUERY_ANALYSIS_MAX_HINTS", q.MaxHints); err != nil {
		return QueryAnalysisConfig{}, err
	}
	if q.MaxSignals, err = loadIntWithDefault("STELE_QUERY_ANALYSIS_MAX_SIGNALS", q.MaxSignals); err != nil {
		return QueryAnalysisConfig{}, err
	}
	if q.MaxSubqueries, err = loadIntWithDefault("STELE_QUERY_ANALYSIS_MAX_SUBQUERIES", q.MaxSubqueries); err != nil {
		return QueryAnalysisConfig{}, err
	}
	if q.MaxTermBytes, err = loadIntWithDefault("STELE_QUERY_ANALYSIS_MAX_TERM_BYTES", q.MaxTermBytes); err != nil {
		return QueryAnalysisConfig{}, err
	}
	if q.MaxSubqueryBytes, err = loadIntWithDefault("STELE_QUERY_ANALYSIS_MAX_SUBQUERY_BYTES", q.MaxSubqueryBytes); err != nil {
		return QueryAnalysisConfig{}, err
	}
	if q.MaxAnalysisWork, err = loadIntWithDefault("STELE_QUERY_ANALYSIS_MAX_WORK", q.MaxAnalysisWork); err != nil {
		return QueryAnalysisConfig{}, err
	}
	if q.MaxCandidatesPerSignal, err = loadIntWithDefault("STELE_QUERY_ANALYSIS_MAX_CANDIDATES_PER_SIGNAL", q.MaxCandidatesPerSignal); err != nil {
		return QueryAnalysisConfig{}, err
	}
	if q.MaxAggregateCandidates, err = loadIntWithDefault("STELE_QUERY_ANALYSIS_MAX_AGGREGATE_CANDIDATES", q.MaxAggregateCandidates); err != nil {
		return QueryAnalysisConfig{}, err
	}
	if q.MaxElapsed, err = loadDurationWithDefault("STELE_QUERY_ANALYSIS_MAX_ELAPSED", q.MaxElapsed); err != nil {
		return QueryAnalysisConfig{}, err
	}
	if q.MaxQueryBytes < 1 || q.MaxQueryBytes > 16*1024 || q.MaxHints < 0 || q.MaxHints > 4 || q.MaxSignals < 1 || q.MaxSignals > 16 || q.MaxSubqueries < 0 || q.MaxSubqueries > 8 || q.MaxSubqueries >= q.MaxSignals || q.MaxTermBytes < 1 || q.MaxTermBytes > 1024 || q.MaxSubqueryBytes < 1 || q.MaxSubqueryBytes > 4096 || q.MaxAnalysisWork < 1 || q.MaxAnalysisWork > 7 || q.MaxCandidatesPerSignal < 1 || q.MaxCandidatesPerSignal > 100 || q.MaxAggregateCandidates < q.MaxCandidatesPerSignal || q.MaxAggregateCandidates > 1000 || q.MaxElapsed <= 0 || q.MaxElapsed > 5*time.Second {
		return QueryAnalysisConfig{}, fmt.Errorf("query-analysis settings are invalid")
	}
	return q, nil
}

func loadGraphTraversalConfig() (GraphTraversalConfig, error) {
	g := GraphTraversalConfig{MaxHops: 3, MaxSeeds: 32, MaxEdgesPerHop: 64, MaxPathsPerSeed: 32, MaxPathsPerRequest: 256, MaxCandidates: 100, MaxElapsed: 250 * time.Millisecond}
	var err error
	if g.MaxHops, err = loadIntWithDefault("STELE_GRAPH_MAX_HOPS", g.MaxHops); err != nil {
		return GraphTraversalConfig{}, err
	}
	if g.MaxSeeds, err = loadIntWithDefault("STELE_GRAPH_MAX_SEEDS", g.MaxSeeds); err != nil {
		return GraphTraversalConfig{}, err
	}
	if g.MaxEdgesPerHop, err = loadIntWithDefault("STELE_GRAPH_MAX_EDGES_PER_HOP", g.MaxEdgesPerHop); err != nil {
		return GraphTraversalConfig{}, err
	}
	if g.MaxPathsPerSeed, err = loadIntWithDefault("STELE_GRAPH_MAX_PATHS_PER_SEED", g.MaxPathsPerSeed); err != nil {
		return GraphTraversalConfig{}, err
	}
	if g.MaxPathsPerRequest, err = loadIntWithDefault("STELE_GRAPH_MAX_PATHS_PER_REQUEST", g.MaxPathsPerRequest); err != nil {
		return GraphTraversalConfig{}, err
	}
	if g.MaxCandidates, err = loadIntWithDefault("STELE_GRAPH_MAX_CANDIDATES", g.MaxCandidates); err != nil {
		return GraphTraversalConfig{}, err
	}
	if g.MaxElapsed, err = loadDurationWithDefault("STELE_GRAPH_MAX_ELAPSED", g.MaxElapsed); err != nil {
		return GraphTraversalConfig{}, err
	}
	if g.MaxHops < 1 || g.MaxHops > 3 || g.MaxSeeds < 1 || g.MaxSeeds > 1000 || g.MaxEdgesPerHop < 1 || g.MaxEdgesPerHop > 10000 || g.MaxPathsPerSeed < 1 || g.MaxPathsPerSeed > 1000 || g.MaxPathsPerRequest < 1 || g.MaxPathsPerRequest > 10000 || g.MaxCandidates < 1 || g.MaxCandidates > 5000 || g.MaxElapsed <= 0 || g.MaxElapsed > 30*time.Second {
		return GraphTraversalConfig{}, fmt.Errorf("graph traversal settings are invalid")
	}
	return g, nil
}

func loadContextCalibrationConfig() (ContextCalibrationConfig, error) {
	c := ContextCalibrationConfig{MaxSummaryAge: 24 * time.Hour, MinimumEvidence: 5, ConfidenceThreshold: 0.5, DecayWindow: 30 * 24 * time.Hour, ContributionCap: 0.25, MaxCandidates: 100, MaxContextItems: 100, MaxElapsed: 100 * time.Millisecond}
	c.Enabled = loadBoolEnv("STELE_CONTEXT_CALIBRATION_ENABLED")
	var err error
	if c.MaxSummaryAge, err = loadDurationWithDefault("STELE_CONTEXT_CALIBRATION_MAX_SUMMARY_AGE", c.MaxSummaryAge); err != nil {
		return ContextCalibrationConfig{}, err
	}
	if c.MinimumEvidence, err = loadIntWithDefault("STELE_CONTEXT_CALIBRATION_MINIMUM_EVIDENCE", c.MinimumEvidence); err != nil {
		return ContextCalibrationConfig{}, err
	}
	if c.ConfidenceThreshold, err = loadFloatWithDefault("STELE_CONTEXT_CALIBRATION_CONFIDENCE_THRESHOLD", c.ConfidenceThreshold); err != nil {
		return ContextCalibrationConfig{}, err
	}
	if c.DecayWindow, err = loadDurationWithDefault("STELE_CONTEXT_CALIBRATION_DECAY_WINDOW", c.DecayWindow); err != nil {
		return ContextCalibrationConfig{}, err
	}
	if c.ContributionCap, err = loadFloatWithDefault("STELE_CONTEXT_CALIBRATION_CONTRIBUTION_CAP", c.ContributionCap); err != nil {
		return ContextCalibrationConfig{}, err
	}
	if c.MaxCandidates, err = loadIntWithDefault("STELE_CONTEXT_CALIBRATION_MAX_CANDIDATES", c.MaxCandidates); err != nil {
		return ContextCalibrationConfig{}, err
	}
	if c.MaxContextItems, err = loadIntWithDefault("STELE_CONTEXT_CALIBRATION_MAX_CONTEXT_ITEMS", c.MaxContextItems); err != nil {
		return ContextCalibrationConfig{}, err
	}
	if c.MaxElapsed, err = loadDurationWithDefault("STELE_CONTEXT_CALIBRATION_MAX_ELAPSED", c.MaxElapsed); err != nil {
		return ContextCalibrationConfig{}, err
	}
	if c.MaxSummaryAge <= 0 || c.MaxSummaryAge > 7*24*time.Hour || c.MinimumEvidence < 1 || c.MinimumEvidence > 10000 || c.ConfidenceThreshold < 0 || c.ConfidenceThreshold > 1 || c.DecayWindow <= 0 || c.DecayWindow > 365*24*time.Hour || c.ContributionCap <= 0 || c.ContributionCap > 1 || c.MaxCandidates < 1 || c.MaxCandidates > 1000 || c.MaxContextItems < 1 || c.MaxContextItems > 1000 || c.MaxElapsed <= 0 || c.MaxElapsed > time.Second {
		return ContextCalibrationConfig{}, fmt.Errorf("context calibration settings are invalid")
	}
	return c, nil
}

func getEnvOrDefault(key string, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}

	return fallback
}

func loadEvaluationConfig(runtimeDSN string) (EvaluationConfig, error) {
	owned := strings.TrimSpace(os.Getenv("STELE_TEST_RETRIEVAL_EVALUATION_DSN"))
	profile := strings.TrimSpace(os.Getenv("STELE_RETRIEVAL_EVALUATION_PROVIDER_PROFILE"))
	if owned != "" && owned == strings.TrimSpace(runtimeDSN) {
		return EvaluationConfig{}, fmt.Errorf("STELE_TEST_RETRIEVAL_EVALUATION_DSN must not reuse STELE_POSTGRES_DSN")
	}
	if profile != "" && !safeEvaluationProfile(profile) {
		return EvaluationConfig{}, fmt.Errorf("STELE_RETRIEVAL_EVALUATION_PROVIDER_PROFILE is invalid")
	}
	return EvaluationConfig{OwnedDSN: owned, ProviderProfile: profile}, nil
}
func safeEvaluationProfile(value string) bool {
	if len(value) > 128 || value == "" {
		return false
	}
	for _, r := range value {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '-' || r == '_' || r == '.' || r == ':' {
			continue
		}
		return false
	}
	return true
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

func loadFloatWithDefault(key string, fallback float64) (float64, error) {
	raw := strings.TrimSpace(os.Getenv(key))
	if raw == "" {
		return fallback, nil
	}
	value, err := strconv.ParseFloat(raw, 64)
	if err != nil {
		return 0, fmt.Errorf("%s is invalid: %w", key, err)
	}
	return value, nil
}

func loadBoolEnv(key string) bool {
	raw := strings.TrimSpace(strings.ToLower(os.Getenv(key)))
	return raw == "1" || raw == "true" || raw == "yes"
}

func loadBoolEnvWithDefault(key string, fallback bool) bool {
	if strings.TrimSpace(os.Getenv(key)) == "" {
		return fallback
	}
	return loadBoolEnv(key)
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
