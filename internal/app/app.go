package app

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/FelixSeptem/stele/internal/auth"
	"github.com/FelixSeptem/stele/internal/config"
	"github.com/FelixSeptem/stele/internal/embedding"
	"github.com/FelixSeptem/stele/internal/governance"
	"github.com/FelixSeptem/stele/internal/jobs"
	"github.com/FelixSeptem/stele/internal/memory"
	"github.com/FelixSeptem/stele/internal/policy"
	"github.com/FelixSeptem/stele/internal/retrieval"
	"github.com/FelixSeptem/stele/internal/storage/postgres"
	"github.com/FelixSeptem/stele/internal/telemetry"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

type Runner interface {
	Start(ctx context.Context) error
}

type noopRunner struct {
	mode config.Mode
}

type bootstrapper interface {
	Bootstrap(ctx context.Context) error
}

type httpServer interface {
	ListenAndServe() error
	Shutdown(ctx context.Context) error
}

type apiRunner struct {
	bootstrapper bootstrapper
	server       httpServer
	build        func(ctx context.Context) (apiRuntime, error)
}

type apiRuntime struct {
	bootstrapper bootstrapper
	server       httpServer
	cleanup      func()
}

type workerRuntime struct {
	bootstrapper bootstrapper
	worker       backgroundWorker
	cleanup      func()
}

type schedulerRuntime struct {
	bootstrapper bootstrapper
	scheduler    backgroundScheduler
	cleanup      func()
}

type postgresRuntimeStore interface {
	bootstrapDB
	queryRower
	transactionStarter
	Close()
}

type bootstrapDB interface {
	BeginTx(ctx context.Context, txOptions pgx.TxOptions) (pgx.Tx, error)
}

type queryRower interface {
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
	Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error)
}

type transactionStarter interface {
	BeginTx(ctx context.Context, txOptions pgx.TxOptions) (pgx.Tx, error)
}

type apiRuntimeDependencies struct {
	openPool          func(ctx context.Context, dsn string) (postgresRuntimeStore, error)
	bootstrapDatabase func(ctx context.Context, db postgresRuntimeStore) error
	newServer         func(addr string, deps HTTPDependencies) httpServer
	observer          telemetry.Observer
}

type workerRuntimeDependencies struct {
	openPool          func(ctx context.Context, dsn string) (postgresRuntimeStore, error)
	bootstrapDatabase func(ctx context.Context, db postgresRuntimeStore) error
	now               func() time.Time
	observer          telemetry.Observer
}

type backgroundWorker interface {
	Start(ctx context.Context) error
}

type schedulerRuntimeDependencies struct {
	openPool          func(ctx context.Context, dsn string) (postgresRuntimeStore, error)
	bootstrapDatabase func(ctx context.Context, db postgresRuntimeStore) error
	now               func() time.Time
	observer          telemetry.Observer
}

type backgroundScheduler interface {
	Start(ctx context.Context) error
}

const governanceWorkerLeaseDuration = 2 * time.Minute

type lifecycleProcessorAdapter struct {
	processor governance.ForgettingProcessor
}

func (a lifecycleProcessorAdapter) Apply(ctx context.Context, action memory.LifecycleActionRecord) error {
	return a.processor.Apply(ctx, governance.LifecycleAction{
		MemoryID:  action.MemoryID,
		Scope:     action.Scope,
		Action:    action.Action,
		Reason:    action.Reason,
		Actor:     action.Actor,
		RequestID: action.RequestID,
		AppliedAt: action.AppliedAt,
	})
}

func NewRunner(cfg config.Config) (Runner, error) {
	switch cfg.Mode {
	case config.ModeAPI:
		return apiRunner{
			build: func(ctx context.Context) (apiRuntime, error) {
				return buildAPIRuntime(ctx, cfg, defaultAPIRuntimeDependencies())
			},
		}, nil
	case config.ModeWorker:
		return workerRunner{
			build: func(ctx context.Context) (workerRuntime, error) {
				return buildWorkerRuntime(ctx, cfg, defaultWorkerRuntimeDependencies())
			},
		}, nil
	case config.ModeScheduler:
		return schedulerRunner{
			build: func(ctx context.Context) (schedulerRuntime, error) {
				return buildSchedulerRuntime(ctx, cfg, defaultSchedulerRuntimeDependencies())
			},
		}, nil
	default:
		return nil, fmt.Errorf("unsupported mode %q", cfg.Mode)
	}
}

func (r noopRunner) Start(ctx context.Context) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
		return nil
	}
}

type workerRunner struct {
	build func(ctx context.Context) (workerRuntime, error)
}

type schedulerRunner struct {
	build func(ctx context.Context) (schedulerRuntime, error)
}

func (r apiRunner) Start(ctx context.Context) error {
	if r.build == nil {
		if r.bootstrapper == nil {
			return fmt.Errorf("api runtime builder is required")
		}

		return runAPIRuntime(ctx, apiRuntime{
			bootstrapper: r.bootstrapper,
			server:       r.server,
		})
	}

	runtime, err := r.build(ctx)
	if err != nil {
		return err
	}
	return runAPIRuntime(ctx, runtime)
}

func (r workerRunner) Start(ctx context.Context) error {
	if r.build == nil {
		return fmt.Errorf("worker runtime builder is required")
	}

	runtime, err := r.build(ctx)
	if err != nil {
		return err
	}

	if runtime.cleanup != nil {
		defer runtime.cleanup()
	}

	if runtime.bootstrapper == nil {
		return fmt.Errorf("worker bootstrapper is required")
	}

	if runtime.worker == nil {
		return fmt.Errorf("worker runtime is required")
	}

	if err := runtime.bootstrapper.Bootstrap(ctx); err != nil {
		return err
	}

	return runtime.worker.Start(ctx)
}

func (r schedulerRunner) Start(ctx context.Context) error {
	if r.build == nil {
		return fmt.Errorf("scheduler runtime builder is required")
	}

	runtime, err := r.build(ctx)
	if err != nil {
		return err
	}

	if runtime.cleanup != nil {
		defer runtime.cleanup()
	}

	if runtime.bootstrapper == nil {
		return fmt.Errorf("scheduler bootstrapper is required")
	}

	if runtime.scheduler == nil {
		return fmt.Errorf("scheduler runtime is required")
	}

	if err := runtime.bootstrapper.Bootstrap(ctx); err != nil {
		return err
	}

	return runtime.scheduler.Start(ctx)
}

func runAPIRuntime(ctx context.Context, runtime apiRuntime) error {
	if runtime.cleanup != nil {
		defer runtime.cleanup()
	}

	if runtime.bootstrapper == nil {
		return fmt.Errorf("api bootstrapper is required")
	}

	if runtime.server == nil {
		return fmt.Errorf("api server is required")
	}

	if err := runtime.bootstrapper.Bootstrap(ctx); err != nil {
		return err
	}

	if err := runtime.server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		return err
	}

	return nil
}

type noopBootstrapper struct{}

func (noopBootstrapper) Bootstrap(ctx context.Context) error {
	return nil
}

func staticAPIKeysFromConfig(cfg config.Config) auth.StaticAPIKeys {
	return staticAPIKeys(cfg.Auth.APIKeys)
}

func staticAdminAPIKeysFromConfig(cfg config.Config) auth.StaticAPIKeys {
	return staticAPIKeys(cfg.Auth.AdminAPIKeys)
}

func staticAPIKeys(values []string) auth.StaticAPIKeys {
	keys := make(auth.StaticAPIKeys, len(values))
	for _, key := range values {
		keys[key] = struct{}{}
	}

	return keys
}

func httpDependenciesFromConfig(cfg config.Config) HTTPDependencies {
	return HTTPDependencies{
		APIKeys:      staticAPIKeysFromConfig(cfg),
		AdminAPIKeys: staticAdminAPIKeysFromConfig(cfg),
	}
}

func httpDependenciesFromConfigWithIngestor(cfg config.Config, ingestor memory.EventIngestor) HTTPDependencies {
	deps := httpDependenciesFromConfig(cfg)
	deps.EventIngestor = ingestor
	return deps
}

func defaultAPIRuntimeDependencies() apiRuntimeDependencies {
	return apiRuntimeDependencies{
		openPool: func(ctx context.Context, dsn string) (postgresRuntimeStore, error) {
			return postgres.OpenPool(ctx, dsn)
		},
		bootstrapDatabase: func(ctx context.Context, db postgresRuntimeStore) error {
			return postgres.BootstrapDatabase(ctx, db)
		},
		newServer: func(addr string, deps HTTPDependencies) httpServer {
			return NewHTTPServer(addr, deps)
		},
	}
}

func defaultWorkerRuntimeDependencies() workerRuntimeDependencies {
	return workerRuntimeDependencies{
		openPool: func(ctx context.Context, dsn string) (postgresRuntimeStore, error) {
			return postgres.OpenPool(ctx, dsn)
		},
		bootstrapDatabase: func(ctx context.Context, db postgresRuntimeStore) error {
			return postgres.BootstrapDatabase(ctx, db)
		},
		now: time.Now,
	}
}

func defaultSchedulerRuntimeDependencies() schedulerRuntimeDependencies {
	return schedulerRuntimeDependencies{
		openPool: func(ctx context.Context, dsn string) (postgresRuntimeStore, error) {
			return postgres.OpenPool(ctx, dsn)
		},
		bootstrapDatabase: func(ctx context.Context, db postgresRuntimeStore) error {
			return postgres.BootstrapDatabase(ctx, db)
		},
		now: time.Now,
	}
}

func buildAPIRuntime(ctx context.Context, cfg config.Config, deps apiRuntimeDependencies) (apiRuntime, error) {
	if deps.openPool == nil {
		deps.openPool = defaultAPIRuntimeDependencies().openPool
	}
	if deps.bootstrapDatabase == nil {
		deps.bootstrapDatabase = defaultAPIRuntimeDependencies().bootstrapDatabase
	}
	if deps.newServer == nil {
		deps.newServer = defaultAPIRuntimeDependencies().newServer
	}
	if deps.observer == nil {
		deps.observer = telemetry.NoopObserver()
	}

	pool, err := deps.openPool(ctx, cfg.PostgresDSN)
	if err != nil {
		return apiRuntime{}, fmt.Errorf("open postgres pool: %w", err)
	}

	repo := postgres.NewRepositoryWithEmbeddingRouter(pool, embeddingRouterFromConfig(cfg.Embedding))
	ingestor := memory.NewService(repo, time.Now, deps.observer)
	queryService := memory.NewQueryService(repo)
	lifecycleService := memory.LifecycleService{
		Processor: lifecycleProcessorAdapter{processor: governance.ForgettingProcessor{
			Repository: repo,
			Now:        time.Now,
			Observer:   deps.observer,
		}},
		Now: time.Now,
	}
	manualMutationService := memory.ManualMutationService{
		Processor:    repo,
		Now:          time.Now,
		NewMemoryID:  newID,
		NewVersionID: newID,
	}
	retrievalService := retrieval.NewService(retrieval.ServiceDependencies{
		Lexical:   repo,
		Semantic:  repo,
		Relations: repo,
		Citations: repo,
	}, deps.observer)
	httpDeps := httpDependenciesFromConfigWithIngestor(cfg, ingestor)
	httpDeps.Readiness = readinessFunc(func(ctx context.Context) error {
		return nil
	})
	httpDeps.MemoryQuery = queryService
	httpDeps.MemoryLifecycleAction = lifecycleService
	httpDeps.MemoryManualMutation = manualMutationService
	httpDeps.MemorySearcher = retrievalService
	httpDeps.ContextAssembler = retrievalService
	httpDeps.GovernanceStatusRead = observedGovernanceStatusReader{
		reader: governanceStatusReaderFunc(func(ctx context.Context) (GovernanceStatus, error) {
			return repo.ReadGovernanceStatus(ctx, time.Now().UTC())
		}),
		now:      time.Now,
		observer: deps.observer,
	}
	httpDeps.GovernanceAdmin = repo
	httpDeps.MemoryHistoryRead = memoryHistoryReaderFunc(func(ctx context.Context, scope memory.Scope, memoryID string) (memory.MemoryHistory, error) {
		return repo.ReadMemoryHistory(ctx, scope, memoryID, true)
	})
	httpDeps.JobExecutionRead = jobExecutionReaderFunc(func(ctx context.Context, scope memory.Scope, limit int) ([]jobs.JobExecutionRecord, error) {
		return repo.ListRecentJobExecutions(ctx, scope, limit)
	})

	return apiRuntime{
		bootstrapper: bootstrapperFunc(func(ctx context.Context) error {
			return deps.bootstrapDatabase(ctx, pool)
		}),
		server: deps.newServer(cfg.HTTPAddr, httpDeps),
		cleanup: func() {
			pool.Close()
		},
	}, nil
}

func buildWorkerRuntime(ctx context.Context, cfg config.Config, deps workerRuntimeDependencies) (workerRuntime, error) {
	if deps.openPool == nil {
		deps.openPool = defaultWorkerRuntimeDependencies().openPool
	}
	if deps.bootstrapDatabase == nil {
		deps.bootstrapDatabase = defaultWorkerRuntimeDependencies().bootstrapDatabase
	}
	if deps.now == nil {
		deps.now = time.Now
	}
	if deps.observer == nil {
		deps.observer = telemetry.NoopObserver()
	}

	pool, err := deps.openPool(ctx, cfg.PostgresDSN)
	if err != nil {
		return workerRuntime{}, fmt.Errorf("open postgres pool: %w", err)
	}

	repo := postgres.NewRepositoryWithEmbeddingRouter(pool, embeddingRouterFromConfig(cfg.Embedding))
	now := deps.now
	summary := governance.SummaryProcessor{
		Source:         repo,
		Repository:     repo,
		Summarizer:     governance.DeterministicSummarizer{},
		Now:            now,
		NewMemoryID:    newID,
		NewVersionID:   newID,
		MinClusterSize: 2,
		ClusterLimit:   50,
	}
	pipeline := governance.PipelineProcessor{
		Extraction: governance.ExtractionProcessor{
			Extractor:       governance.RuleBasedExtractor{},
			Candidates:      repo,
			RawEvents:       repo,
			Now:             now,
			NewCandidateID:  newID,
			NewProvenanceID: newID,
		},
		Consolidation: governance.ConsolidationProcessor{
			Candidates:      repo,
			Canonicals:      repo,
			Consolidator:    governance.RuleBasedConsolidator{},
			Now:             now,
			NewMemoryID:     newID,
			NewVersionID:    newID,
			NewProvenanceID: newID,
		},
		RawEvents: repo,
		Now:       now,
		Summary:   summary,
	}

	worker := jobs.GovernanceWorker{
		Claimer:            repo,
		Processor:          pipeline,
		FailureRecorder:    repo,
		LeaseRenewer:       repo,
		WorkerID:           "stele-worker",
		BatchSize:          32,
		LeaseDuration:      governanceWorkerLeaseDuration,
		LeaseRenewInterval: cfg.Jobs.GovernanceLeaseRenewPeriod,
		MaxAttempts:        cfg.Jobs.GovernanceMaxAttempts,
		RetryBackoff:       cfg.Jobs.GovernanceRetryBackoff,
		Now:                now,
		Observer:           deps.observer,
	}

	return workerRuntime{
		bootstrapper: bootstrapperFunc(func(ctx context.Context) error {
			return deps.bootstrapDatabase(ctx, pool)
		}),
		worker: jobs.PollingWorker{
			Worker:       worker,
			PollInterval: cfg.Jobs.WorkerPollInterval,
			ErrorBackoff: cfg.Jobs.WorkerErrorBackoff,
		},
		cleanup: func() {
			pool.Close()
		},
	}, nil
}

func buildSchedulerRuntime(ctx context.Context, cfg config.Config, deps schedulerRuntimeDependencies) (schedulerRuntime, error) {
	if deps.openPool == nil {
		deps.openPool = defaultSchedulerRuntimeDependencies().openPool
	}
	if deps.bootstrapDatabase == nil {
		deps.bootstrapDatabase = defaultSchedulerRuntimeDependencies().bootstrapDatabase
	}
	if deps.now == nil {
		deps.now = time.Now
	}
	if deps.observer == nil {
		deps.observer = telemetry.NoopObserver()
	}

	pool, err := deps.openPool(ctx, cfg.PostgresDSN)
	if err != nil {
		return schedulerRuntime{}, fmt.Errorf("open postgres pool: %w", err)
	}

	scope := memory.Scope{
		Tenant:    cfg.Auth.DefaultTenant,
		Project:   cfg.Auth.DefaultProject,
		Namespace: cfg.Auth.DefaultNamespace,
	}.Normalized()
	if err := scope.Validate(); err != nil {
		pool.Close()
		return schedulerRuntime{}, fmt.Errorf("scheduler default scope: %w", err)
	}

	repo := postgres.NewRepositoryWithEmbeddingRouter(pool, embeddingRouterFromConfig(cfg.Embedding))
	now := deps.now
	summary := governance.SummaryProcessor{
		Source:         repo,
		Repository:     repo,
		Summarizer:     governance.DeterministicSummarizer{},
		Now:            now,
		NewMemoryID:    newID,
		NewVersionID:   newID,
		MinClusterSize: 2,
		ClusterLimit:   50,
	}

	retention := governance.RetentionProcessor{
		Policy: policy.DefaultRetentionPolicy(),
		Forgetting: governance.ForgettingProcessor{
			Repository: repo,
			Now:        now,
			Observer:   deps.observer,
		},
		Now: now,
	}

	summaryInterval := firstPositiveDuration(cfg.Jobs.SummaryCompactionInterval, cfg.Jobs.MaintenanceInterval, 15*time.Minute)
	retentionInterval := firstPositiveDuration(cfg.Jobs.RetentionInterval, cfg.Jobs.MaintenanceInterval, 15*time.Minute)
	cleanupInterval := firstPositiveDuration(cfg.Jobs.CleanupInterval, cfg.Jobs.MaintenanceInterval, 15*time.Minute)
	embeddingInterval := firstPositiveDuration(cfg.Jobs.MaintenanceInterval, 15*time.Minute)
	schedulerInterval := minPositiveDuration(summaryInterval, retentionInterval, cleanupInterval, embeddingInterval, cfg.Jobs.MaintenanceInterval, 15*time.Minute)

	embeddingRouter := embeddingRouterFromConfig(cfg.Embedding)
	embeddingProviders := embedding.StaticProviderRegistry{}

	scheduler := jobs.MaintenanceScheduler{
		Jobs: []jobs.MaintenanceJob{
			jobs.ScopeDispatchJob{
				NameValue:       "embedding_rebuild_dispatch",
				ScopeSource:     repo,
				ScopeBatchLimit: cfg.Jobs.MaintenanceScopeBatchLimit,
				FallbackScope:   scope,
				Dispatch: func(scope memory.Scope) jobs.MaintenanceJob {
					return jobs.EmbeddingRebuildJob{
						Scope:          scope,
						Router:         embeddingRouter,
						Providers:      embeddingProviders,
						Store:          repo,
						ExecutionStore: repo,
						Observer:       deps.observer,
						TriggerSource:  "scheduler",
						Cadence:        embeddingInterval,
						Now:            now,
						Limit:          100,
						NewRevisionID:  newID,
					}
				},
			},
			jobs.ScopeDispatchJob{
				NameValue:       "summary_compaction_dispatch",
				ScopeSource:     repo,
				ScopeBatchLimit: cfg.Jobs.MaintenanceScopeBatchLimit,
				FallbackScope:   scope,
				Dispatch: func(scope memory.Scope) jobs.MaintenanceJob {
					return jobs.SummaryCompactionJob{
						Scope:          scope,
						CutoffWindow:   summaryInterval,
						Cadence:        summaryInterval,
						Now:            now,
						Processor:      summary,
						ExecutionStore: repo,
						TriggerSource:  "scheduler",
					}
				},
			},
			jobs.ScopeDispatchJob{
				NameValue:       "retention_sweep_dispatch",
				ScopeSource:     repo,
				ScopeBatchLimit: cfg.Jobs.MaintenanceScopeBatchLimit,
				FallbackScope:   scope,
				Dispatch: func(scope memory.Scope) jobs.MaintenanceJob {
					return jobs.RetentionSweepJob{
						Scope:          scope,
						Cadence:        retentionInterval,
						Now:            now,
						Source:         repo,
						Evaluator:      retention,
						ExecutionStore: repo,
						TriggerSource:  "scheduler",
						Limit:          100,
					}
				},
			},
			jobs.JobExecutionCleanupJob{
				Scope:           scope,
				Cadence:         cleanupInterval,
				RetentionWindow: cfg.Jobs.JobExecutionRetention,
				Now:             now,
				Cleaner:         repo,
				ExecutionStore:  repo,
				TriggerSource:   "scheduler",
			},
		},
		Interval:     schedulerInterval,
		ErrorBackoff: cfg.Jobs.SchedulerErrorBackoff,
	}

	return schedulerRuntime{
		bootstrapper: bootstrapperFunc(func(ctx context.Context) error {
			return deps.bootstrapDatabase(ctx, pool)
		}),
		scheduler: scheduler,
		cleanup: func() {
			pool.Close()
		},
	}, nil
}

func embeddingRouterFromConfig(cfg config.EmbeddingConfig) embedding.Router {
	router := embedding.Router{
		Default: embedding.Target{
			Provider:   cfg.DefaultProvider,
			Model:      cfg.DefaultModel,
			Dimensions: cfg.DefaultDimensions,
		},
	}

	if len(cfg.ClassRoutes) == 0 {
		return router
	}

	router.ByClass = make(map[string]embedding.Target, len(cfg.ClassRoutes))
	for className, route := range cfg.ClassRoutes {
		router.ByClass[className] = embedding.Target{
			Provider:   route.Provider,
			Model:      route.Model,
			Dimensions: route.Dimensions,
		}
	}

	return router
}

func newID() string {
	return fmt.Sprintf("id_%d", time.Now().UnixNano())
}

type bootstrapperFunc func(ctx context.Context) error

func (f bootstrapperFunc) Bootstrap(ctx context.Context) error {
	return f(ctx)
}

type readinessFunc func(ctx context.Context) error

func (f readinessFunc) Ready(ctx context.Context) error {
	return f(ctx)
}

type governanceStatusReaderFunc func(ctx context.Context) (GovernanceStatus, error)

func (f governanceStatusReaderFunc) ReadGovernanceStatus(ctx context.Context) (GovernanceStatus, error) {
	return f(ctx)
}

type memoryHistoryReaderFunc func(ctx context.Context, scope memory.Scope, memoryID string) (memory.MemoryHistory, error)

func (f memoryHistoryReaderFunc) ReadMemoryHistory(ctx context.Context, scope memory.Scope, memoryID string) (memory.MemoryHistory, error) {
	return f(ctx, scope, memoryID)
}

type jobExecutionReaderFunc func(ctx context.Context, scope memory.Scope, limit int) ([]jobs.JobExecutionRecord, error)

func (f jobExecutionReaderFunc) ListRecentJobExecutions(ctx context.Context, scope memory.Scope, limit int) ([]jobs.JobExecutionRecord, error) {
	return f(ctx, scope, limit)
}

type observedGovernanceStatusReader struct {
	reader   GovernanceStatusReader
	now      func() time.Time
	observer telemetry.Observer
}

func (r observedGovernanceStatusReader) ReadGovernanceStatus(ctx context.Context) (GovernanceStatus, error) {
	status, err := r.reader.ReadGovernanceStatus(ctx)
	observedAt := status.ObservedAt
	if observedAt.IsZero() {
		now := time.Now
		if r.now != nil {
			now = r.now
		}
		observedAt = now().UTC()
		status.ObservedAt = observedAt
	}

	if r.observer != nil {
		event := telemetry.BacklogEvent{
			Mode:       "api",
			Component:  "governance_status_reader",
			Queue:      "governance_raw_events",
			ObservedAt: observedAt,
		}
		if err != nil {
			event.Status = "error"
			event.Error = err.Error()
		} else {
			event.Status = "ok"
			event.Pending = status.PendingRawEvents
			event.Leased = status.LeasedRawEvents
			event.Processed = status.ProcessedRawEvents
			if !status.OldestPendingCreatedAt.IsZero() {
				event.OldestAge = observedAt.Sub(status.OldestPendingCreatedAt)
			}
		}
		r.observer.RecordBacklog(ctx, event)
	}

	return status, err
}

func firstPositiveDuration(values ...time.Duration) time.Duration {
	for _, value := range values {
		if value > 0 {
			return value
		}
	}

	return 0
}

func minPositiveDuration(values ...time.Duration) time.Duration {
	var minimum time.Duration
	for _, value := range values {
		if value <= 0 {
			continue
		}
		if minimum == 0 || value < minimum {
			minimum = value
		}
	}

	return minimum
}
