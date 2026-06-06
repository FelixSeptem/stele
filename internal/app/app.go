package app

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/FelixSeptem/stele/internal/auth"
	"github.com/FelixSeptem/stele/internal/config"
	"github.com/FelixSeptem/stele/internal/governance"
	"github.com/FelixSeptem/stele/internal/jobs"
	"github.com/FelixSeptem/stele/internal/memory"
	"github.com/FelixSeptem/stele/internal/retrieval"
	"github.com/FelixSeptem/stele/internal/storage/postgres"
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
}

type workerRuntimeDependencies struct {
	openPool          func(ctx context.Context, dsn string) (postgresRuntimeStore, error)
	bootstrapDatabase func(ctx context.Context, db postgresRuntimeStore) error
	now               func() time.Time
}

type backgroundWorker interface {
	RunOnce(ctx context.Context) (int, error)
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
		return noopRunner{mode: cfg.Mode}, nil
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

	_, err = runtime.worker.RunOnce(ctx)
	return err
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
	keys := make(auth.StaticAPIKeys, len(cfg.Auth.APIKeys))
	for _, key := range cfg.Auth.APIKeys {
		keys[key] = struct{}{}
	}

	return keys
}

func httpDependenciesFromConfig(cfg config.Config) HTTPDependencies {
	return HTTPDependencies{
		APIKeys: staticAPIKeysFromConfig(cfg),
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

	pool, err := deps.openPool(ctx, cfg.PostgresDSN)
	if err != nil {
		return apiRuntime{}, fmt.Errorf("open postgres pool: %w", err)
	}

	repo := postgres.NewRepository(pool)
	ingestor := memory.NewService(repo, time.Now)
	retrievalService := retrieval.NewService(retrieval.ServiceDependencies{
		Lexical:   repo,
		Semantic:  repo,
		Relations: repo,
		Citations: repo,
	})
	httpDeps := httpDependenciesFromConfigWithIngestor(cfg, ingestor)
	httpDeps.Readiness = readinessFunc(func(ctx context.Context) error {
		return nil
	})
	httpDeps.MemorySearcher = retrievalService
	httpDeps.ContextAssembler = retrievalService

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

	pool, err := deps.openPool(ctx, cfg.PostgresDSN)
	if err != nil {
		return workerRuntime{}, fmt.Errorf("open postgres pool: %w", err)
	}

	repo := postgres.NewRepository(pool)
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
		Claimer:       repo,
		Processor:     pipeline,
		WorkerID:      "stele-worker",
		BatchSize:     32,
		LeaseDuration: 2 * time.Minute,
		Now:           now,
	}

	return workerRuntime{
		bootstrapper: bootstrapperFunc(func(ctx context.Context) error {
			return deps.bootstrapDatabase(ctx, pool)
		}),
		worker: worker,
		cleanup: func() {
			pool.Close()
		},
	}, nil
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
