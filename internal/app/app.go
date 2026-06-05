package app

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/FelixSeptem/stele/internal/auth"
	"github.com/FelixSeptem/stele/internal/config"
	"github.com/FelixSeptem/stele/internal/memory"
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

func NewRunner(cfg config.Config) (Runner, error) {
	switch cfg.Mode {
	case config.ModeAPI:
		return apiRunner{
			build: func(ctx context.Context) (apiRuntime, error) {
				return buildAPIRuntime(ctx, cfg, defaultAPIRuntimeDependencies())
			},
		}, nil
	case config.ModeWorker, config.ModeScheduler:
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
	httpDeps := httpDependenciesFromConfigWithIngestor(cfg, ingestor)
	httpDeps.Readiness = readinessFunc(func(ctx context.Context) error {
		return nil
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

type bootstrapperFunc func(ctx context.Context) error

func (f bootstrapperFunc) Bootstrap(ctx context.Context) error {
	return f(ctx)
}

type readinessFunc func(ctx context.Context) error

func (f readinessFunc) Ready(ctx context.Context) error {
	return f(ctx)
}
