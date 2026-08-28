package app

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/FelixSeptem/stele/internal/config"
	"github.com/FelixSeptem/stele/internal/embedding"
	"github.com/FelixSeptem/stele/internal/jobs"
	"github.com/FelixSeptem/stele/internal/memory"
	"github.com/FelixSeptem/stele/internal/telemetry"
	pgxmock "github.com/pashagolub/pgxmock/v4"
)

type stubBootstrapper struct {
	called bool
	err    error
}

func (s *stubBootstrapper) Bootstrap(ctx context.Context) error {
	s.called = true
	return s.err
}

type stubAPIServer struct {
	called         bool
	err            error
	shutdownCalled bool
	shutdownCh     chan struct{}
}

func (s *stubAPIServer) ListenAndServe() error {
	s.called = true
	if s.shutdownCh != nil {
		<-s.shutdownCh
		return http.ErrServerClosed
	}
	return s.err
}

func (s *stubAPIServer) Shutdown(ctx context.Context) error {
	s.shutdownCalled = true
	if s.shutdownCh != nil {
		close(s.shutdownCh)
	}
	return nil
}

func TestRunAPIRuntimeShutsDownServerWhenContextIsCanceled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	server := &stubAPIServer{shutdownCh: make(chan struct{})}
	runtime := apiRuntime{bootstrapper: &stubBootstrapper{}, server: server, shutdownTimeout: time.Second}

	done := make(chan error, 1)
	go func() { done <- runAPIRuntime(ctx, runtime) }()
	time.Sleep(10 * time.Millisecond)
	cancel()

	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("runAPIRuntime() error = %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("runAPIRuntime() did not stop after context cancellation")
	}
	if !server.shutdownCalled {
		t.Fatal("server shutdown was not called")
	}
}

type stubAppObserver struct {
	operations []telemetry.OperationEvent
	backlogs   []telemetry.BacklogEvent
}

type stubEmbeddingProvider struct{}

func (stubEmbeddingProvider) GenerateEmbedding(ctx context.Context, input embedding.ProviderRequest) (embedding.ProviderResult, error) {
	return embedding.ProviderResult{
		Provider:   input.Target.Provider,
		Model:      input.Target.Model,
		Dimensions: input.Target.Dimensions,
		Embedding:  []float32{0.1, 0.2, 0.3},
	}, nil
}

func (s *stubAppObserver) RecordOperation(ctx context.Context, event telemetry.OperationEvent) {
	s.operations = append(s.operations, event)
}

func (s *stubAppObserver) RecordBacklog(ctx context.Context, event telemetry.BacklogEvent) {
	s.backlogs = append(s.backlogs, event)
}

func TestNewRunnerCreatesAPIRunner(t *testing.T) {
	cfg := config.Config{Mode: config.ModeAPI, PostgresDSN: "postgres://example"}

	runner, err := NewRunner(cfg)
	if err != nil {
		t.Fatalf("NewRunner() error = %v", err)
	}

	if runner == nil {
		t.Fatal("NewRunner() returned nil runner")
	}
}

func TestNewRunnerRejectsUnsupportedMode(t *testing.T) {
	cfg := config.Config{Mode: config.Mode("invalid"), PostgresDSN: "postgres://example"}

	_, err := NewRunner(cfg)
	if err == nil {
		t.Fatal("NewRunner() error = nil, want invalid mode error")
	}
}

func TestNewRunnerCreatesRealSchedulerRunner(t *testing.T) {
	cfg := config.Config{
		Mode:        config.ModeScheduler,
		PostgresDSN: "postgres://example",
	}

	runner, err := NewRunner(cfg)
	if err != nil {
		t.Fatalf("NewRunner() error = %v", err)
	}

	if _, ok := runner.(noopRunner); ok {
		t.Fatal("scheduler runner = noopRunner, want maintenance-backed scheduler runner")
	}
}

func TestNewRunnerCreatesRealWorkerRunner(t *testing.T) {
	cfg := config.Config{
		Mode:        config.ModeWorker,
		PostgresDSN: "postgres://example",
	}

	runner, err := NewRunner(cfg)
	if err != nil {
		t.Fatalf("NewRunner() error = %v", err)
	}

	if _, ok := runner.(noopRunner); ok {
		t.Fatal("worker runner = noopRunner, want governance-backed worker runner")
	}
}

func TestAPIRunnerBootstrapsBeforeServing(t *testing.T) {
	bootstrapper := &stubBootstrapper{}
	server := &stubAPIServer{err: http.ErrServerClosed}

	runner := apiRunner{
		bootstrapper: bootstrapper,
		server:       server,
	}

	if err := runner.Start(context.Background()); err != nil {
		t.Fatalf("Start() error = %v", err)
	}

	if !bootstrapper.called {
		t.Fatal("bootstrapper was not called")
	}

	if !server.called {
		t.Fatal("server was not started")
	}
}

func TestAPIRunnerReturnsBootstrapFailure(t *testing.T) {
	bootstrapper := &stubBootstrapper{err: errors.New("bootstrap failed")}
	server := &stubAPIServer{}

	runner := apiRunner{
		bootstrapper: bootstrapper,
		server:       server,
	}

	if err := runner.Start(context.Background()); err == nil {
		t.Fatal("Start() error = nil, want bootstrap failure")
	}

	if server.called {
		t.Fatal("server should not start when bootstrap fails")
	}
}

func TestHTTPDependenciesFromConfigPassesRuntimeLimits(t *testing.T) {
	cfg := config.Config{HTTP: config.HTTPConfig{
		MaxRequestBodyBytes: 123,
		MaxHeaderBytes:      456,
		ReadHeaderTimeout:   time.Second,
		ReadTimeout:         2 * time.Second,
		WriteTimeout:        3 * time.Second,
		IdleTimeout:         4 * time.Second,
	}}

	deps := httpDependenciesFromConfig(cfg)
	if deps.HTTP.MaxRequestBodyBytes != 123 || deps.HTTP.MaxHeaderBytes != 456 || deps.HTTP.ReadHeaderTimeout != time.Second || deps.HTTP.ReadTimeout != 2*time.Second || deps.HTTP.WriteTimeout != 3*time.Second || deps.HTTP.IdleTimeout != 4*time.Second {
		t.Fatalf("HTTP dependencies = %+v, want config runtime limits", deps.HTTP)
	}
}

func TestNewHTTPHandlerUsesConfiguredAPIKeys(t *testing.T) {
	handler := NewHTTPHandler(HTTPDependencies{
		APIKeys: map[string]struct{}{"test-key": {}},
		Logger:  log.New(&strings.Builder{}, "", 0),
	})

	req := httptest.NewRequest(http.MethodPost, "/v1/events", strings.NewReader(`{"event_type":"conversation.message","content":"hello"}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Stele-Tenant", "tenant-a")
	req.Header.Set("X-Stele-Project", "project-a")
	req.Header.Set("X-Stele-Namespace", "namespace-a")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusUnauthorized)
	}
}

func TestStaticAPIKeysFromConfig(t *testing.T) {
	cfg := config.Config{
		Auth: config.AuthConfig{
			APIKeys: []string{"key-a", "key-b"},
		},
	}

	keys := staticAPIKeysFromConfig(cfg)

	if _, ok := keys["key-a"]; !ok {
		t.Fatal("key-a not found in static api keys")
	}

	if _, ok := keys["key-b"]; !ok {
		t.Fatal("key-b not found in static api keys")
	}
}

func TestHTTPDependenciesFromConfigUsesConfiguredKeys(t *testing.T) {
	cfg := config.Config{
		Auth: config.AuthConfig{
			APIKeys: []string{"key-a", "key-b"},
		},
	}

	deps := httpDependenciesFromConfig(cfg)

	if _, ok := deps.APIKeys["key-a"]; !ok {
		t.Fatal("key-a not found in HTTP dependencies")
	}

	if _, ok := deps.APIKeys["key-b"]; !ok {
		t.Fatal("key-b not found in HTTP dependencies")
	}
}

func TestHTTPDependenciesFromConfigPreservesProvidedIngestor(t *testing.T) {
	cfg := config.Config{
		Auth: config.AuthConfig{
			APIKeys: []string{"key-a"},
		},
	}
	ingestor := &stubEventIngestor{eventID: "evt_123"}

	deps := httpDependenciesFromConfigWithIngestor(cfg, ingestor)

	if deps.EventIngestor != ingestor {
		t.Fatalf("EventIngestor = %T, want provided ingestor", deps.EventIngestor)
	}
}

func TestNewRunnerUsesConfiguredAPIKeysForAPIMode(t *testing.T) {
	cfg := config.Config{
		Mode:        config.ModeAPI,
		HTTPAddr:    ":8080",
		PostgresDSN: "postgres://example",
		Auth: config.AuthConfig{
			APIKeys: []string{"key-a"},
		},
	}

	runner, err := NewRunner(cfg)
	if err != nil {
		t.Fatalf("NewRunner() error = %v", err)
	}

	api, ok := runner.(apiRunner)
	if !ok {
		t.Fatalf("runner type = %T, want apiRunner", runner)
	}

	if api.build == nil {
		t.Fatal("api runtime builder = nil, want configured builder")
	}
}

func TestBuildAPIRuntimeUsesConfiguredDependencies(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatalf("pgxmock.NewPool() error = %v", err)
	}
	defer mock.Close()

	cfg := config.Config{
		HTTPAddr:    ":9090",
		PostgresDSN: "postgres://runtime",
		Auth: config.AuthConfig{
			APIKeys: []string{"key-a"},
		},
	}

	var gotDSN string
	var gotDeps HTTPDependencies
	bootstrapCalled := false

	runtime, err := buildAPIRuntime(context.Background(), cfg, apiRuntimeDependencies{
		openPool: func(ctx context.Context, dsn string) (postgresRuntimeStore, error) {
			gotDSN = dsn
			return mock, nil
		},
		bootstrapDatabase: func(ctx context.Context, db postgresRuntimeStore) error {
			bootstrapCalled = true
			return nil
		},
		newServer: func(addr string, deps HTTPDependencies) httpServer {
			if addr != cfg.HTTPAddr {
				t.Fatalf("addr = %q, want %q", addr, cfg.HTTPAddr)
			}
			gotDeps = deps
			return &stubAPIServer{err: http.ErrServerClosed}
		},
	})
	if err != nil {
		t.Fatalf("buildAPIRuntime() error = %v", err)
	}

	if gotDSN != cfg.PostgresDSN {
		t.Fatalf("dsn = %q, want %q", gotDSN, cfg.PostgresDSN)
	}

	if runtime.bootstrapper == nil {
		t.Fatal("runtime bootstrapper = nil, want bootstrapper")
	}

	if runtime.server == nil {
		t.Fatal("runtime server = nil, want server")
	}

	if gotDeps.EventIngestor == nil {
		t.Fatal("EventIngestor = nil, want configured ingestor")
	}

	if gotDeps.MemoryQuery == nil {
		t.Fatal("MemoryQuery = nil, want configured memory query service")
	}

	if gotDeps.GovernanceAdmin == nil {
		t.Fatal("GovernanceAdmin = nil, want configured governance admin service")
	}

	if gotDeps.DerivedInsightAdmin == nil {
		t.Fatal("DerivedInsightAdmin = nil, want configured derived insight admin service")
	}

	if gotDeps.UsefulnessFeedback == nil {
		t.Fatal("UsefulnessFeedback = nil, want configured usefulness feedback service")
	}

	if gotDeps.AssuranceAdmin == nil {
		t.Fatal("AssuranceAdmin = nil, want configured assurance admin service")
	}

	if gotDeps.Readiness == nil {
		t.Fatal("Readiness = nil, want readiness checker")
	}

	if gotDeps.Metrics == nil {
		t.Fatal("Metrics = nil, want metrics recorder")
	}

	if _, ok := gotDeps.APIKeys["key-a"]; !ok {
		t.Fatal("configured api key not found in runtime dependencies")
	}

	if err := runtime.bootstrapper.Bootstrap(context.Background()); err != nil {
		t.Fatalf("Bootstrap() error = %v", err)
	}

	if !bootstrapCalled {
		t.Fatal("bootstrap function was not called")
	}
}

func TestBuildAPIRuntimeReturnsPoolOpenFailure(t *testing.T) {
	cfg := config.Config{
		HTTPAddr:    ":9090",
		PostgresDSN: "postgres://runtime",
	}

	_, err := buildAPIRuntime(context.Background(), cfg, apiRuntimeDependencies{
		openPool: func(ctx context.Context, dsn string) (postgresRuntimeStore, error) {
			return nil, errors.New("dial tcp 127.0.0.1:5432: connectex: connection refused")
		},
	})
	if err == nil {
		t.Fatal("buildAPIRuntime() error = nil, want pool open failure")
	}

	if !strings.Contains(err.Error(), "open postgres pool") {
		t.Fatalf("error = %q, want wrapped pool open failure", err)
	}
}

func TestRuntimeReadinessCheckerUsesModeSpecificEmbeddingProviderChecks(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatalf("pgxmock.NewPool() error = %v", err)
	}
	defer mock.Close()

	runtime := embeddingRuntime{
		Providers: embedding.StaticProviderRegistry{},
		Status: memory.EmbeddingRuntimeStatus{
			SemanticRebuildEnabled: true,
			RegisteredProviders:    []string{"openai"},
		},
	}

	mock.ExpectQuery("SELECT 1").WillReturnRows(pgxmock.NewRows([]string{"one"}).AddRow(1))
	observer := telemetry.NewMetricsObserver()
	apiReady := runtimeReadinessChecker(config.ModeAPI, mock, runtime, false, observer)
	if err := apiReady.Ready(context.Background()); err != nil {
		t.Fatalf("api readiness error = %v, want nil", err)
	}

	mock.ExpectQuery("SELECT 1").WillReturnRows(pgxmock.NewRows([]string{"one"}).AddRow(1))
	schedulerReady := runtimeReadinessChecker(config.ModeScheduler, mock, runtime, true, observer)
	if err := schedulerReady.Ready(context.Background()); err == nil || !strings.Contains(err.Error(), "embedding provider readiness") {
		t.Fatalf("scheduler readiness error = %v, want embedding provider readiness failure", err)
	}

	metrics := observer.RenderPrometheus()
	if !strings.Contains(metrics, `stele_embedding_provider_probe_total{mode="scheduler",model="unknown",provider="openai",result="failure"} 1`) {
		t.Fatalf("metrics missing provider probe failure:\n%s", metrics)
	}
}

func TestBuildWorkerRuntimeAssemblesGovernanceWorker(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatalf("pgxmock.NewPool() error = %v", err)
	}
	defer mock.Close()

	cfg := config.Config{
		Mode:        config.ModeWorker,
		PostgresDSN: "postgres://runtime",
	}

	runtime, err := buildWorkerRuntime(context.Background(), cfg, workerRuntimeDependencies{
		openPool: func(ctx context.Context, dsn string) (postgresRuntimeStore, error) {
			return mock, nil
		},
		bootstrapDatabase: func(ctx context.Context, db postgresRuntimeStore) error {
			return nil
		},
		now: func() time.Time {
			return time.Date(2026, 6, 2, 10, 0, 0, 0, time.UTC)
		},
	})
	if err != nil {
		t.Fatalf("buildWorkerRuntime() error = %v", err)
	}

	if runtime.bootstrapper == nil {
		t.Fatal("runtime bootstrapper = nil, want bootstrapper")
	}

	if runtime.worker == nil {
		t.Fatal("runtime worker = nil, want governance worker")
	}
}

func TestBuildWorkerAndSchedulerRuntimeWireReadinessChecks(t *testing.T) {
	workerMock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatalf("pgxmock.NewPool() error = %v", err)
	}
	defer workerMock.Close()
	schedulerMock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatalf("pgxmock.NewPool() error = %v", err)
	}
	defer schedulerMock.Close()

	cfg := config.Config{
		PostgresDSN: "postgres://runtime",
		Auth: config.AuthConfig{
			DefaultTenant:    "tenant-a",
			DefaultProject:   "project-a",
			DefaultNamespace: "namespace-a",
		},
		Embedding: config.EmbeddingConfig{
			DefaultProvider:   "openai",
			DefaultModel:      "text-embedding-3-small",
			DefaultDimensions: 1536,
		},
	}
	providers := map[string]embedding.Provider{
		"openai": stubEmbeddingProvider{},
	}

	workerRuntime, err := buildWorkerRuntime(context.Background(), cfg, workerRuntimeDependencies{
		openPool: func(ctx context.Context, dsn string) (postgresRuntimeStore, error) {
			return workerMock, nil
		},
		bootstrapDatabase: func(ctx context.Context, db postgresRuntimeStore) error {
			return nil
		},
		embeddingProviders: providers,
	})
	if err != nil {
		t.Fatalf("buildWorkerRuntime() error = %v", err)
	}
	if workerRuntime.readiness == nil {
		t.Fatal("worker readiness = nil, want configured checker")
	}
	workerMock.ExpectQuery("SELECT 1").WillReturnRows(pgxmock.NewRows([]string{"one"}).AddRow(1))
	if err := workerRuntime.readiness.Ready(context.Background()); err != nil {
		t.Fatalf("worker readiness error = %v, want nil", err)
	}

	schedulerRuntime, err := buildSchedulerRuntime(context.Background(), cfg, schedulerRuntimeDependencies{
		openPool: func(ctx context.Context, dsn string) (postgresRuntimeStore, error) {
			return schedulerMock, nil
		},
		bootstrapDatabase: func(ctx context.Context, db postgresRuntimeStore) error {
			return nil
		},
		embeddingProviders: providers,
	})
	if err != nil {
		t.Fatalf("buildSchedulerRuntime() error = %v", err)
	}
	if schedulerRuntime.readiness == nil {
		t.Fatal("scheduler readiness = nil, want configured checker")
	}
	schedulerMock.ExpectQuery("SELECT 1").WillReturnRows(pgxmock.NewRows([]string{"one"}).AddRow(1))
	if err := schedulerRuntime.readiness.Ready(context.Background()); err != nil {
		t.Fatalf("scheduler readiness error = %v, want nil", err)
	}
}

func TestBuildWorkerRuntimeWiresDurableRetrySettings(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatalf("pgxmock.NewPool() error = %v", err)
	}
	defer mock.Close()

	cfg := config.Config{
		Mode:        config.ModeWorker,
		PostgresDSN: "postgres://runtime",
		Auth: config.AuthConfig{
			DefaultTenant:    "tenant-a",
			DefaultProject:   "project-a",
			DefaultNamespace: "namespace-a",
		},
		Jobs: config.JobConfig{
			WorkerPollInterval:         7 * time.Second,
			WorkerErrorBackoff:         11 * time.Second,
			GovernanceMaxAttempts:      7,
			GovernanceRetryBackoff:     45 * time.Second,
			GovernanceLeaseRenewPeriod: 20 * time.Second,
		},
		Assurance: config.AssuranceConfig{
			AlertMaxAttempts:  4,
			AlertRetryBackoff: 2 * time.Minute,
		},
	}

	runtime, err := buildWorkerRuntime(context.Background(), cfg, workerRuntimeDependencies{
		openPool: func(ctx context.Context, dsn string) (postgresRuntimeStore, error) {
			return mock, nil
		},
		bootstrapDatabase: func(ctx context.Context, db postgresRuntimeStore) error {
			return nil
		},
		now: func() time.Time {
			return time.Date(2026, 6, 10, 10, 0, 0, 0, time.UTC)
		},
	})
	if err != nil {
		t.Fatalf("buildWorkerRuntime() error = %v", err)
	}

	poller, ok := runtime.worker.(jobs.PollingWorker)
	if !ok {
		t.Fatalf("runtime worker type = %T, want jobs.PollingWorker", runtime.worker)
	}

	worker, ok := findGovernanceWorker(poller.Worker)
	if !ok {
		t.Fatalf("poller.Worker type = %T, want jobs.GovernanceWorker in worker graph", poller.Worker)
	}
	alertWorker, ok := findAssuranceAlertDeliveryWorker(poller.Worker)
	if !ok {
		t.Fatalf("poller.Worker type = %T, want jobs.AssuranceAlertDeliveryWorker in worker graph", poller.Worker)
	}

	if worker.MaxAttempts != 7 {
		t.Fatalf("MaxAttempts = %d, want 7", worker.MaxAttempts)
	}
	if alertWorker.MaxAttempts != 4 {
		t.Fatalf("alert MaxAttempts = %d, want 4", alertWorker.MaxAttempts)
	}

	if worker.RetryBackoff != 45*time.Second {
		t.Fatalf("RetryBackoff = %v, want 45s", worker.RetryBackoff)
	}
	if alertWorker.RetryBackoff != 2*time.Minute {
		t.Fatalf("alert RetryBackoff = %v, want 2m", alertWorker.RetryBackoff)
	}

	if worker.LeaseRenewInterval != 20*time.Second {
		t.Fatalf("LeaseRenewInterval = %v, want 20s", worker.LeaseRenewInterval)
	}

	if poller.PollInterval != 7*time.Second {
		t.Fatalf("PollInterval = %v, want 7s", poller.PollInterval)
	}

	if poller.ErrorBackoff != 11*time.Second {
		t.Fatalf("ErrorBackoff = %v, want 11s", poller.ErrorBackoff)
	}
}

func findGovernanceWorker(worker jobs.LoopWorker) (jobs.GovernanceWorker, bool) {
	if governanceWorker, ok := worker.(jobs.GovernanceWorker); ok {
		return governanceWorker, true
	}
	composite, ok := worker.(jobs.CompositeWorker)
	if !ok {
		return jobs.GovernanceWorker{}, false
	}
	for _, child := range composite.Workers {
		if governanceWorker, ok := child.(jobs.GovernanceWorker); ok {
			return governanceWorker, true
		}
	}
	return jobs.GovernanceWorker{}, false
}

func findAssuranceAlertDeliveryWorker(worker jobs.LoopWorker) (jobs.AssuranceAlertDeliveryWorker, bool) {
	if alertWorker, ok := worker.(jobs.AssuranceAlertDeliveryWorker); ok {
		return alertWorker, true
	}
	composite, ok := worker.(jobs.CompositeWorker)
	if !ok {
		return jobs.AssuranceAlertDeliveryWorker{}, false
	}
	for _, child := range composite.Workers {
		if alertWorker, ok := child.(jobs.AssuranceAlertDeliveryWorker); ok {
			return alertWorker, true
		}
	}
	return jobs.AssuranceAlertDeliveryWorker{}, false
}

func TestBuildSchedulerRuntimeAssemblesMaintenanceScheduler(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatalf("pgxmock.NewPool() error = %v", err)
	}
	defer mock.Close()

	cfg := config.Config{
		Mode:        config.ModeScheduler,
		PostgresDSN: "postgres://runtime",
		Auth: config.AuthConfig{
			DefaultTenant:    "tenant-a",
			DefaultProject:   "project-a",
			DefaultNamespace: "namespace-a",
		},
		Jobs: config.JobConfig{
			MaintenanceInterval:   30 * time.Second,
			SchedulerErrorBackoff: time.Minute,
		},
	}

	runtime, err := buildSchedulerRuntime(context.Background(), cfg, schedulerRuntimeDependencies{
		openPool: func(ctx context.Context, dsn string) (postgresRuntimeStore, error) {
			return mock, nil
		},
		bootstrapDatabase: func(ctx context.Context, db postgresRuntimeStore) error {
			return nil
		},
		now: func() time.Time {
			return time.Date(2026, 6, 6, 18, 0, 0, 0, time.UTC)
		},
	})
	if err != nil {
		t.Fatalf("buildSchedulerRuntime() error = %v", err)
	}

	if runtime.bootstrapper == nil {
		t.Fatal("runtime bootstrapper = nil, want bootstrapper")
	}

	if runtime.scheduler == nil {
		t.Fatal("runtime scheduler = nil, want maintenance scheduler")
	}

	scheduler, ok := runtime.scheduler.(jobs.MaintenanceScheduler)
	if !ok {
		t.Fatalf("runtime scheduler type = %T, want jobs.MaintenanceScheduler", runtime.scheduler)
	}

	if len(scheduler.Jobs) != 9 {
		t.Fatalf("len(scheduler.Jobs) = %d, want 9", len(scheduler.Jobs))
	}
}

func TestBuildSchedulerRuntimeAddsWorkflowMaintenanceOnlyWhenEnabled(t *testing.T) {
	newRuntime := func(enabled bool) jobs.MaintenanceScheduler {
		t.Helper()
		mock, err := pgxmock.NewPool()
		if err != nil {
			t.Fatalf("pgxmock.NewPool() error = %v", err)
		}
		t.Cleanup(func() { mock.Close() })
		cfg := config.Config{
			Mode:        config.ModeScheduler,
			PostgresDSN: "postgres://runtime",
			Auth:        config.AuthConfig{DefaultTenant: "tenant-a", DefaultProject: "project-a", DefaultNamespace: "namespace-a"},
			Jobs:        config.JobConfig{MaintenanceInterval: time.Minute, SchedulerErrorBackoff: time.Minute, WorkflowMaintenanceEnabled: enabled, WorkflowDiagnosticCadence: 2 * time.Minute, WorkflowDiagnosticScanLimit: 10, WorkflowHistoryRetention: 24 * time.Hour},
		}
		runtime, err := buildSchedulerRuntime(context.Background(), cfg, schedulerRuntimeDependencies{
			openPool:          func(ctx context.Context, dsn string) (postgresRuntimeStore, error) { return mock, nil },
			bootstrapDatabase: func(ctx context.Context, db postgresRuntimeStore) error { return nil },
			now:               func() time.Time { return time.Date(2026, 7, 18, 19, 0, 0, 0, time.UTC) },
		})
		if err != nil {
			t.Fatalf("buildSchedulerRuntime() error = %v", err)
		}
		scheduler, ok := runtime.scheduler.(jobs.MaintenanceScheduler)
		if !ok {
			t.Fatalf("runtime scheduler type = %T, want jobs.MaintenanceScheduler", runtime.scheduler)
		}
		return scheduler
	}

	disabled := newRuntime(false)
	for _, job := range disabled.Jobs {
		if job.Name() == "workflow_diagnostics_dispatch" || job.Name() == "workflow_retention_dispatch" {
			t.Fatalf("disabled scheduler jobs = %+v, must not include workflow maintenance", disabled.Jobs)
		}
	}
	enabled := newRuntime(true)
	found := map[string]bool{}
	for _, job := range enabled.Jobs {
		found[job.Name()] = true
	}
	for _, name := range []string{"workflow_diagnostics_dispatch", "workflow_retention_dispatch"} {
		if !found[name] {
			t.Fatalf("enabled scheduler jobs missing %q: %+v", name, enabled.Jobs)
		}
	}
}

func TestBuildSchedulerRuntimeAssemblesScopeDispatchJobs(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatalf("pgxmock.NewPool() error = %v", err)
	}
	defer mock.Close()

	cfg := config.Config{
		Mode:        config.ModeScheduler,
		PostgresDSN: "postgres://runtime",
		Auth: config.AuthConfig{
			DefaultTenant:    "tenant-a",
			DefaultProject:   "project-a",
			DefaultNamespace: "namespace-a",
		},
		Jobs: config.JobConfig{
			MaintenanceInterval:              30 * time.Second,
			SchedulerErrorBackoff:            time.Minute,
			MaintenanceScopeBatchLimit:       25,
			DerivedInsightDerivationInterval: 45 * time.Second,
			DerivedInsightBatchSize:          55,
			DerivedInsightMinimumEvidence:    3,
		},
		Assurance: config.AssuranceConfig{
			Cadence:              90 * time.Second,
			ConformanceCadence:   3 * time.Minute,
			HistoryRetention:     7 * 24 * time.Hour,
			ConformanceRetention: 14 * 24 * time.Hour,
		},
	}

	runtime, err := buildSchedulerRuntime(context.Background(), cfg, schedulerRuntimeDependencies{
		openPool: func(ctx context.Context, dsn string) (postgresRuntimeStore, error) {
			return mock, nil
		},
		bootstrapDatabase: func(ctx context.Context, db postgresRuntimeStore) error {
			return nil
		},
		now: func() time.Time {
			return time.Date(2026, 6, 10, 11, 0, 0, 0, time.UTC)
		},
	})
	if err != nil {
		t.Fatalf("buildSchedulerRuntime() error = %v", err)
	}

	scheduler, ok := runtime.scheduler.(jobs.MaintenanceScheduler)
	if !ok {
		t.Fatalf("runtime scheduler type = %T, want jobs.MaintenanceScheduler", runtime.scheduler)
	}

	if len(scheduler.Jobs) != 9 {
		t.Fatalf("len(scheduler.Jobs) = %d, want 9", len(scheduler.Jobs))
	}

	dispatchA, ok := scheduler.Jobs[0].(jobs.ScopeDispatchJob)
	if !ok {
		t.Fatalf("scheduler.Jobs[0] type = %T, want jobs.ScopeDispatchJob", scheduler.Jobs[0])
	}

	dispatchB, ok := scheduler.Jobs[1].(jobs.ScopeDispatchJob)
	if !ok {
		t.Fatalf("scheduler.Jobs[1] type = %T, want jobs.ScopeDispatchJob", scheduler.Jobs[1])
	}

	dispatchC, ok := scheduler.Jobs[2].(jobs.ScopeDispatchJob)
	if !ok {
		t.Fatalf("scheduler.Jobs[2] type = %T, want jobs.ScopeDispatchJob", scheduler.Jobs[2])
	}

	dispatchD, ok := scheduler.Jobs[3].(jobs.ScopeDispatchJob)
	if !ok {
		t.Fatalf("scheduler.Jobs[3] type = %T, want jobs.ScopeDispatchJob", scheduler.Jobs[3])
	}

	dispatchE, ok := scheduler.Jobs[4].(jobs.ScopeDispatchJob)
	if !ok {
		t.Fatalf("scheduler.Jobs[4] type = %T, want jobs.ScopeDispatchJob", scheduler.Jobs[4])
	}

	dispatchF, ok := scheduler.Jobs[5].(jobs.ScopeDispatchJob)
	if !ok {
		t.Fatalf("scheduler.Jobs[5] type = %T, want jobs.ScopeDispatchJob", scheduler.Jobs[5])
	}

	dispatchG, ok := scheduler.Jobs[6].(jobs.ScopeDispatchJob)
	if !ok {
		t.Fatalf("scheduler.Jobs[6] type = %T, want jobs.ScopeDispatchJob", scheduler.Jobs[6])
	}

	dispatchH, ok := scheduler.Jobs[7].(jobs.ScopeDispatchJob)
	if !ok {
		t.Fatalf("scheduler.Jobs[7] type = %T, want jobs.ScopeDispatchJob", scheduler.Jobs[7])
	}

	if dispatchA.ScopeBatchLimit != 25 || dispatchB.ScopeBatchLimit != 25 || dispatchC.ScopeBatchLimit != 25 || dispatchD.ScopeBatchLimit != 25 || dispatchE.ScopeBatchLimit != 25 || dispatchF.ScopeBatchLimit != 25 || dispatchG.ScopeBatchLimit != 25 || dispatchH.ScopeBatchLimit != 25 {
		t.Fatalf("scope batch limits = (%d, %d, %d, %d, %d, %d, %d, %d), want 25", dispatchA.ScopeBatchLimit, dispatchB.ScopeBatchLimit, dispatchC.ScopeBatchLimit, dispatchD.ScopeBatchLimit, dispatchE.ScopeBatchLimit, dispatchF.ScopeBatchLimit, dispatchG.ScopeBatchLimit, dispatchH.ScopeBatchLimit)
	}

	if dispatchA.FallbackScope.Namespace != "namespace-a" || dispatchB.FallbackScope.Namespace != "namespace-a" || dispatchC.FallbackScope.Namespace != "namespace-a" || dispatchD.FallbackScope.Namespace != "namespace-a" || dispatchE.FallbackScope.Namespace != "namespace-a" || dispatchF.FallbackScope.Namespace != "namespace-a" || dispatchG.FallbackScope.Namespace != "namespace-a" || dispatchH.FallbackScope.Namespace != "namespace-a" {
		t.Fatalf("fallback scopes = (%+v, %+v, %+v, %+v, %+v, %+v, %+v, %+v), want default namespace", dispatchA.FallbackScope, dispatchB.FallbackScope, dispatchC.FallbackScope, dispatchD.FallbackScope, dispatchE.FallbackScope, dispatchF.FallbackScope, dispatchG.FallbackScope, dispatchH.FallbackScope)
	}

	if dispatchA.NameValue != "embedding_rebuild_dispatch" {
		t.Fatalf("scheduler.Jobs[0].NameValue = %q, want embedding_rebuild_dispatch", dispatchA.NameValue)
	}

	embeddingJob, ok := dispatchA.Dispatch(dispatchA.FallbackScope).(jobs.EmbeddingRebuildJob)
	if !ok {
		t.Fatalf("dispatchA.Dispatch(...) type = %T, want jobs.EmbeddingRebuildJob", dispatchA.Dispatch(dispatchA.FallbackScope))
	}

	if embeddingJob.Observer == nil {
		t.Fatal("embedding rebuild job observer = nil, want telemetry observer wiring")
	}

	if dispatchD.NameValue != "derived_insight_derivation_dispatch" {
		t.Fatalf("scheduler.Jobs[3].NameValue = %q, want derived_insight_derivation_dispatch", dispatchD.NameValue)
	}

	derivedInsightJob, ok := dispatchD.Dispatch(dispatchD.FallbackScope).(jobs.DerivedInsightDerivationJob)
	if !ok {
		t.Fatalf("dispatchD.Dispatch(...) type = %T, want jobs.DerivedInsightDerivationJob", dispatchD.Dispatch(dispatchD.FallbackScope))
	}

	if derivedInsightJob.Cadence != 45*time.Second || derivedInsightJob.Limit != 55 || derivedInsightJob.MinimumEvidence != 3 {
		t.Fatalf("derived insight job cadence/limit/minimum = %v/%d/%d, want 45s/55/3", derivedInsightJob.Cadence, derivedInsightJob.Limit, derivedInsightJob.MinimumEvidence)
	}

	if dispatchE.NameValue != "derived_insight_replay_execution_dispatch" {
		t.Fatalf("scheduler.Jobs[4].NameValue = %q, want derived_insight_replay_execution_dispatch", dispatchE.NameValue)
	}

	replayJob, ok := dispatchE.Dispatch(dispatchE.FallbackScope).(jobs.DerivedInsightReplayExecutionJob)
	if !ok {
		t.Fatalf("dispatchE.Dispatch(...) type = %T, want jobs.DerivedInsightReplayExecutionJob", dispatchE.Dispatch(dispatchE.FallbackScope))
	}
	if replayJob.Cadence != 45*time.Second || replayJob.Limit != 55 {
		t.Fatalf("replay job cadence/limit = %v/%d, want 45s/55", replayJob.Cadence, replayJob.Limit)
	}

	if dispatchF.NameValue != "assurance_evaluation_dispatch" {
		t.Fatalf("scheduler.Jobs[5].NameValue = %q, want assurance_evaluation_dispatch", dispatchF.NameValue)
	}
	assuranceJob, ok := dispatchF.Dispatch(dispatchF.FallbackScope).(jobs.AssuranceEvaluationJob)
	if !ok {
		t.Fatalf("dispatchF.Dispatch(...) type = %T, want jobs.AssuranceEvaluationJob", dispatchF.Dispatch(dispatchF.FallbackScope))
	}
	if assuranceJob.Cadence != 90*time.Second {
		t.Fatalf("assurance job cadence = %v, want 90s", assuranceJob.Cadence)
	}

	if dispatchG.NameValue != "conformance_run_dispatch" {
		t.Fatalf("scheduler.Jobs[6].NameValue = %q, want conformance_run_dispatch", dispatchG.NameValue)
	}
	conformanceJob, ok := dispatchG.Dispatch(dispatchG.FallbackScope).(jobs.ConformanceRunJob)
	if !ok {
		t.Fatalf("dispatchG.Dispatch(...) type = %T, want jobs.ConformanceRunJob", dispatchG.Dispatch(dispatchG.FallbackScope))
	}
	if conformanceJob.Cadence != 3*time.Minute {
		t.Fatalf("conformance job cadence = %v, want 3m", conformanceJob.Cadence)
	}

	if dispatchH.NameValue != "assurance_retention_dispatch" {
		t.Fatalf("scheduler.Jobs[7].NameValue = %q, want assurance_retention_dispatch", dispatchH.NameValue)
	}
	retentionJob, ok := dispatchH.Dispatch(dispatchH.FallbackScope).(jobs.AssuranceRetentionJob)
	if !ok {
		t.Fatalf("dispatchH.Dispatch(...) type = %T, want jobs.AssuranceRetentionJob", dispatchH.Dispatch(dispatchH.FallbackScope))
	}
	if retentionJob.HistoryRetention != 7*24*time.Hour || retentionJob.ConformanceRetention != 14*24*time.Hour {
		t.Fatalf("assurance retention windows = %v/%v, want 7d/14d", retentionJob.HistoryRetention, retentionJob.ConformanceRetention)
	}

	if _, ok := scheduler.Jobs[8].(jobs.JobExecutionCleanupJob); !ok {
		t.Fatalf("scheduler.Jobs[8] type = %T, want jobs.JobExecutionCleanupJob", scheduler.Jobs[8])
	}
}

func TestBuildSchedulerRuntimeAllowsLexicalOnlyEmbeddingConfiguration(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatalf("pgxmock.NewPool() error = %v", err)
	}
	defer mock.Close()

	cfg := config.Config{
		Mode:        config.ModeScheduler,
		PostgresDSN: "postgres://runtime",
		Auth: config.AuthConfig{
			DefaultTenant:    "tenant-a",
			DefaultProject:   "project-a",
			DefaultNamespace: "namespace-a",
		},
	}

	if _, err := buildSchedulerRuntime(context.Background(), cfg, schedulerRuntimeDependencies{
		openPool: func(ctx context.Context, dsn string) (postgresRuntimeStore, error) {
			return mock, nil
		},
		bootstrapDatabase: func(ctx context.Context, db postgresRuntimeStore) error {
			return nil
		},
		now: func() time.Time {
			return time.Date(2026, 6, 19, 12, 0, 0, 0, time.UTC)
		},
	}); err != nil {
		t.Fatalf("buildSchedulerRuntime() error = %v, want lexical-only startup to succeed", err)
	}
}

func TestBuildSchedulerRuntimeRejectsUnknownConfiguredEmbeddingProvider(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatalf("pgxmock.NewPool() error = %v", err)
	}
	defer mock.Close()

	cfg := config.Config{
		Mode:        config.ModeScheduler,
		PostgresDSN: "postgres://runtime",
		Auth: config.AuthConfig{
			DefaultTenant:    "tenant-a",
			DefaultProject:   "project-a",
			DefaultNamespace: "namespace-a",
		},
		Embedding: config.EmbeddingConfig{
			DefaultProvider:   "openai",
			DefaultModel:      "text-embedding-3-small",
			DefaultDimensions: 1536,
		},
	}

	_, err = buildSchedulerRuntime(context.Background(), cfg, schedulerRuntimeDependencies{
		openPool: func(ctx context.Context, dsn string) (postgresRuntimeStore, error) {
			return mock, nil
		},
		bootstrapDatabase: func(ctx context.Context, db postgresRuntimeStore) error {
			return nil
		},
		now: func() time.Time {
			return time.Date(2026, 6, 19, 12, 0, 0, 0, time.UTC)
		},
	})
	if err == nil {
		t.Fatal("buildSchedulerRuntime() error = nil, want unknown provider validation failure")
	}
}

func TestBuildEmbeddingRuntimeRegistersConfiguredOpenAIProvider(t *testing.T) {
	runtime, err := buildEmbeddingRuntime(config.EmbeddingConfig{
		DefaultProvider:   "openai",
		DefaultModel:      "text-embedding-3-small",
		DefaultDimensions: 1536,
		OpenAI: config.OpenAIEmbeddingProviderConfig{
			APIKey:  "test-openai-key",
			BaseURL: "https://embeddings.example.com/v1",
			Timeout: 30 * time.Second,
		},
	}, nil)
	if err != nil {
		t.Fatalf("buildEmbeddingRuntime() error = %v", err)
	}

	if runtime.Providers == nil {
		t.Fatal("runtime.Providers = nil, want configured resolver")
	}

	if _, err := runtime.Providers.ResolveProvider("openai"); err != nil {
		t.Fatalf("ResolveProvider(openai) error = %v, want configured provider", err)
	}
}

func TestBuildSchedulerRuntimeWiresConfiguredEmbeddingProviderRegistry(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatalf("pgxmock.NewPool() error = %v", err)
	}
	defer mock.Close()

	cfg := config.Config{
		Mode:        config.ModeScheduler,
		PostgresDSN: "postgres://runtime",
		Auth: config.AuthConfig{
			DefaultTenant:    "tenant-a",
			DefaultProject:   "project-a",
			DefaultNamespace: "namespace-a",
		},
		Embedding: config.EmbeddingConfig{
			DefaultProvider:   "openai",
			DefaultModel:      "text-embedding-3-small",
			DefaultDimensions: 1536,
		},
	}

	runtime, err := buildSchedulerRuntime(context.Background(), cfg, schedulerRuntimeDependencies{
		openPool: func(ctx context.Context, dsn string) (postgresRuntimeStore, error) {
			return mock, nil
		},
		bootstrapDatabase: func(ctx context.Context, db postgresRuntimeStore) error {
			return nil
		},
		now: func() time.Time {
			return time.Date(2026, 6, 19, 12, 0, 0, 0, time.UTC)
		},
		embeddingProviders: map[string]embedding.Provider{
			"openai": stubEmbeddingProvider{},
		},
	})
	if err != nil {
		t.Fatalf("buildSchedulerRuntime() error = %v", err)
	}

	scheduler, ok := runtime.scheduler.(jobs.MaintenanceScheduler)
	if !ok {
		t.Fatalf("runtime scheduler type = %T, want jobs.MaintenanceScheduler", runtime.scheduler)
	}

	dispatch, ok := scheduler.Jobs[0].(jobs.ScopeDispatchJob)
	if !ok {
		t.Fatalf("scheduler.Jobs[0] type = %T, want jobs.ScopeDispatchJob", scheduler.Jobs[0])
	}

	job, ok := dispatch.Dispatch(dispatch.FallbackScope).(jobs.EmbeddingRebuildJob)
	if !ok {
		t.Fatalf("dispatch.Dispatch(...) type = %T, want jobs.EmbeddingRebuildJob", dispatch.Dispatch(dispatch.FallbackScope))
	}

	if job.Providers == nil {
		t.Fatal("job.Providers = nil, want configured provider resolver")
	}

	if _, err := job.Providers.ResolveProvider("openai"); err != nil {
		t.Fatalf("ResolveProvider(openai) error = %v, want configured provider", err)
	}
}

func TestObservedGovernanceStatusReaderEmitsBacklogTelemetry(t *testing.T) {
	observer := &stubAppObserver{}
	now := time.Date(2026, 6, 7, 14, 0, 0, 0, time.UTC)
	reader := observedGovernanceStatusReader{
		reader: governanceStatusReaderFunc(func(ctx context.Context) (GovernanceStatus, error) {
			return GovernanceStatus{
				PendingRawEvents:       7,
				LeasedRawEvents:        2,
				ProcessedRawEvents:     19,
				OldestPendingCreatedAt: now.Add(-5 * time.Minute),
				ObservedAt:             now,
			}, nil
		}),
		now:      func() time.Time { return now },
		observer: observer,
	}

	_, err := reader.ReadGovernanceStatus(context.Background())
	if err != nil {
		t.Fatalf("ReadGovernanceStatus() error = %v", err)
	}

	if len(observer.backlogs) != 1 {
		t.Fatalf("len(observer.backlogs) = %d, want 1", len(observer.backlogs))
	}

	if observer.backlogs[0].Queue != "governance_raw_events" || observer.backlogs[0].Pending != 7 || observer.backlogs[0].Status != "ok" {
		t.Fatalf("backlog event = %+v, want governance backlog snapshot", observer.backlogs[0])
	}
}

func TestAPIRunnerClosesRuntimeResourcesWhenBootstrapFails(t *testing.T) {
	closed := false
	runner := apiRunner{
		build: func(ctx context.Context) (apiRuntime, error) {
			return apiRuntime{
				bootstrapper: &stubBootstrapper{err: errors.New("bootstrap failed")},
				server:       &stubAPIServer{},
				cleanup: func() {
					closed = true
				},
			}, nil
		},
	}

	if err := runner.Start(context.Background()); err == nil {
		t.Fatal("Start() error = nil, want bootstrap failure")
	}

	if !closed {
		t.Fatal("runtime cleanup was not called")
	}
}

func TestBuildAPIRuntimeDefaultMigrationPolicyRunsBeforeServing(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatalf("pgxmock.NewPool() error = %v", err)
	}
	defer mock.Close()
	cfg := config.Config{Mode: config.ModeAPI, HTTPAddr: ":9090", PostgresDSN: "postgres://runtime", Migrations: config.MigrationConfig{Policy: config.MigrationPolicyValidate}}
	var gotPolicy string
	runtime, err := buildAPIRuntime(context.Background(), cfg, apiRuntimeDependencies{
		openPool:        func(context.Context, string) (postgresRuntimeStore, error) { return mock, nil },
		migrateDatabase: func(_ context.Context, _ string, policy string) error { gotPolicy = policy; return nil },
		newServer:       func(string, HTTPDependencies) httpServer { return &stubAPIServer{err: http.ErrServerClosed} },
	})
	if err != nil {
		t.Fatalf("buildAPIRuntime() error = %v", err)
	}
	if err := runAPIRuntime(context.Background(), runtime); err != nil {
		t.Fatalf("runAPIRuntime() error = %v", err)
	}
	if gotPolicy != string(cfg.Migrations.Policy) {
		t.Fatalf("migration policy = %q, want %q", gotPolicy, cfg.Migrations.Policy)
	}
}

type stubBackgroundWorker struct{ called bool }

func (s *stubBackgroundWorker) Start(context.Context) error { s.called = true; return nil }

type stubBackgroundScheduler struct{ called bool }

func (s *stubBackgroundScheduler) Start(context.Context) error { s.called = true; return nil }

type cancellationAwareWorker struct{ called chan struct{} }

func (w cancellationAwareWorker) Start(ctx context.Context) error {
	close(w.called)
	<-ctx.Done()
	return ctx.Err()
}

type cancellationAwareScheduler struct{ called chan struct{} }

func (s cancellationAwareScheduler) Start(ctx context.Context) error {
	close(s.called)
	<-ctx.Done()
	return ctx.Err()
}

func TestRuntimeCleanupIsExactlyOnceOnCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	server := &stubAPIServer{shutdownCh: make(chan struct{})}
	cleanupCount := 0
	done := make(chan error, 1)
	go func() {
		done <- runAPIRuntime(ctx, apiRuntime{bootstrapper: &stubBootstrapper{}, server: server, shutdownTimeout: time.Second, cleanup: func() { cleanupCount++ }})
	}()
	time.Sleep(10 * time.Millisecond)
	cancel()
	if err := <-done; err != nil {
		t.Fatalf("api runtime error = %v", err)
	}
	if cleanupCount != 1 {
		t.Fatalf("api cleanup count = %d, want 1", cleanupCount)
	}

	workerCtx, workerCancel := context.WithCancel(context.Background())
	workerCalled := make(chan struct{})
	workerCleanup := 0
	worker := workerRunner{build: func(context.Context) (workerRuntime, error) {
		return workerRuntime{bootstrapper: &stubBootstrapper{}, worker: cancellationAwareWorker{called: workerCalled}, cleanup: func() { workerCleanup++ }}, nil
	}}
	workerDone := make(chan error, 1)
	go func() { workerDone <- worker.Start(workerCtx) }()
	<-workerCalled
	workerCancel()
	if err := <-workerDone; !errors.Is(err, context.Canceled) {
		t.Fatalf("worker runtime error = %v, want context canceled", err)
	}
	if workerCleanup != 1 {
		t.Fatalf("worker cleanup count = %d, want 1", workerCleanup)
	}

	schedulerCtx, schedulerCancel := context.WithCancel(context.Background())
	schedulerCalled := make(chan struct{})
	schedulerCleanup := 0
	scheduler := schedulerRunner{build: func(context.Context) (schedulerRuntime, error) {
		return schedulerRuntime{bootstrapper: &stubBootstrapper{}, scheduler: cancellationAwareScheduler{called: schedulerCalled}, cleanup: func() { schedulerCleanup++ }}, nil
	}}
	schedulerDone := make(chan error, 1)
	go func() { schedulerDone <- scheduler.Start(schedulerCtx) }()
	<-schedulerCalled
	schedulerCancel()
	if err := <-schedulerDone; !errors.Is(err, context.Canceled) {
		t.Fatalf("scheduler runtime error = %v, want context canceled", err)
	}
	if schedulerCleanup != 1 {
		t.Fatalf("scheduler cleanup count = %d, want 1", schedulerCleanup)
	}
}

func TestRuntimeLifecycleTelemetryUsesBoundedLabels(t *testing.T) {
	observer := &stubAppObserver{}
	runtime := apiRuntime{
		bootstrapper: &stubBootstrapper{},
		server:       &stubAPIServer{err: http.ErrServerClosed},
		observer:     observer,
	}
	if err := runAPIRuntime(context.Background(), runtime); err != nil {
		t.Fatalf("runAPIRuntime() error = %v", err)
	}
	if len(observer.operations) < 3 {
		t.Fatalf("lifecycle operation count = %d, want startup and migration events", len(observer.operations))
	}
	for _, event := range observer.operations {
		if event.Mode != string(config.ModeAPI) || event.Component != "runtime" {
			t.Fatalf("lifecycle event = %+v, want bounded api runtime labels", event)
		}
		if event.Error != "" {
			t.Fatalf("lifecycle event leaked raw error: %+v", event)
		}
	}
}

func TestBuildWorkerAndSchedulerRuntimeDefaultMigrationPolicyRunsBeforeWork(t *testing.T) {
	cases := []struct {
		name   string
		policy config.MigrationPolicy
		build  func(config.Config, func(context.Context, string) (postgresRuntimeStore, error), func(context.Context, string, string) error) (Runner, error)
	}{
		{name: "worker", policy: config.MigrationPolicyValidate, build: func(cfg config.Config, open func(context.Context, string) (postgresRuntimeStore, error), migrate func(context.Context, string, string) error) (Runner, error) {
			return workerRunner{build: func(ctx context.Context) (workerRuntime, error) {
				r, err := buildWorkerRuntime(ctx, cfg, workerRuntimeDependencies{openPool: open, migrateDatabase: migrate})
				r.worker = &stubBackgroundWorker{}
				return r, err
			}}, nil
		}},
		{name: "scheduler", policy: config.MigrationPolicyOff, build: func(cfg config.Config, open func(context.Context, string) (postgresRuntimeStore, error), migrate func(context.Context, string, string) error) (Runner, error) {
			return schedulerRunner{build: func(ctx context.Context) (schedulerRuntime, error) {
				r, err := buildSchedulerRuntime(ctx, cfg, schedulerRuntimeDependencies{openPool: open, migrateDatabase: migrate})
				r.scheduler = &stubBackgroundScheduler{}
				return r, err
			}}, nil
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			mock, err := pgxmock.NewPool()
			if err != nil {
				t.Fatalf("pgxmock.NewPool() error = %v", err)
			}
			defer mock.Close()
			cfg := config.Config{PostgresDSN: "postgres://runtime", Migrations: config.MigrationConfig{Policy: tc.policy}, Auth: config.AuthConfig{DefaultTenant: "tenant-a", DefaultProject: "project-a", DefaultNamespace: "namespace-a"}}
			var gotPolicy string
			runner, err := tc.build(cfg, func(context.Context, string) (postgresRuntimeStore, error) { return mock, nil }, func(_ context.Context, _ string, policy string) error { gotPolicy = policy; return nil })
			if err != nil {
				t.Fatalf("build runner error = %v", err)
			}
			if err := runner.Start(context.Background()); err != nil {
				t.Fatalf("runner.Start() error = %v", err)
			}
			if gotPolicy != string(tc.policy) {
				t.Fatalf("migration policy = %q, want %q", gotPolicy, tc.policy)
			}
		})
	}
}

func TestRuntimeStartupRejectsMigrationValidationFailureBeforeWork(t *testing.T) {
	validationErr := errors.New("migration validation failed: status=pending current_version=0 latest_version=1 dirty=false")
	apiServer := &stubAPIServer{err: http.ErrServerClosed}
	apiMock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatalf("pgxmock.NewPool() error = %v", err)
	}
	defer apiMock.Close()
	apiRuntime, err := buildAPIRuntime(context.Background(), config.Config{Mode: config.ModeAPI, HTTPAddr: ":9090", PostgresDSN: "postgres://runtime", Migrations: config.MigrationConfig{Policy: config.MigrationPolicyValidate}}, apiRuntimeDependencies{
		openPool:        func(context.Context, string) (postgresRuntimeStore, error) { return apiMock, nil },
		migrateDatabase: func(context.Context, string, string) error { return validationErr },
		newServer:       func(string, HTTPDependencies) httpServer { return apiServer },
	})
	if err != nil {
		t.Fatalf("buildAPIRuntime() error = %v", err)
	}
	if err := runAPIRuntime(context.Background(), apiRuntime); !errors.Is(err, validationErr) && !strings.Contains(err.Error(), "status=pending") {
		t.Fatalf("runAPIRuntime() error = %v, want pending migration failure", err)
	}
	if apiServer.called {
		t.Fatal("api server started despite migration validation failure")
	}
	dirtyServer := &stubAPIServer{err: http.ErrServerClosed}
	dirtyRuntime, err := buildAPIRuntime(context.Background(), config.Config{Mode: config.ModeAPI, HTTPAddr: ":9090", PostgresDSN: "postgres://runtime", Migrations: config.MigrationConfig{Policy: config.MigrationPolicyValidate}}, apiRuntimeDependencies{
		openPool: func(context.Context, string) (postgresRuntimeStore, error) { return apiMock, nil },
		migrateDatabase: func(context.Context, string, string) error {
			return errors.New("migration validation failed: status=dirty current_version=1 latest_version=1 dirty=true")
		},
		newServer: func(string, HTTPDependencies) httpServer { return dirtyServer },
	})
	if err != nil {
		t.Fatalf("build dirty APIRuntime() error = %v", err)
	}
	if err := runAPIRuntime(context.Background(), dirtyRuntime); err == nil || !strings.Contains(err.Error(), "status=dirty") {
		t.Fatalf("runAPIRuntime() error = %v, want dirty migration failure", err)
	}
	if dirtyServer.called {
		t.Fatal("api server started despite dirty migration validation failure")
	}

	workerMock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatalf("pgxmock.NewPool() error = %v", err)
	}
	defer workerMock.Close()
	workerRunner := workerRunner{build: func(ctx context.Context) (workerRuntime, error) {
		r, err := buildWorkerRuntime(ctx, config.Config{Mode: config.ModeWorker, PostgresDSN: "postgres://runtime", Migrations: config.MigrationConfig{Policy: config.MigrationPolicyValidate}}, workerRuntimeDependencies{
			openPool:        func(context.Context, string) (postgresRuntimeStore, error) { return workerMock, nil },
			migrateDatabase: func(context.Context, string, string) error { return validationErr },
		})
		if err == nil {
			r.worker = &stubBackgroundWorker{}
		}
		return r, err
	}}
	if err := workerRunner.Start(context.Background()); !strings.Contains(err.Error(), "status=pending") {
		t.Fatalf("worker Start() error = %v, want pending migration failure", err)
	}

	schedulerMock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatalf("pgxmock.NewPool() error = %v", err)
	}
	defer schedulerMock.Close()
	schedulerRunner := schedulerRunner{build: func(ctx context.Context) (schedulerRuntime, error) {
		r, err := buildSchedulerRuntime(ctx, config.Config{Mode: config.ModeScheduler, PostgresDSN: "postgres://runtime", Migrations: config.MigrationConfig{Policy: config.MigrationPolicyValidate}, Auth: config.AuthConfig{DefaultTenant: "tenant-a", DefaultProject: "project-a", DefaultNamespace: "namespace-a"}}, schedulerRuntimeDependencies{
			openPool:        func(context.Context, string) (postgresRuntimeStore, error) { return schedulerMock, nil },
			migrateDatabase: func(context.Context, string, string) error { return validationErr },
		})
		if err == nil {
			r.scheduler = &stubBackgroundScheduler{}
		}
		return r, err
	}}
	if err := schedulerRunner.Start(context.Background()); !strings.Contains(err.Error(), "status=pending") {
		t.Fatalf("scheduler Start() error = %v, want pending migration failure", err)
	}
}

func TestBuildAPIRuntimeAssemblesEventIngestionPath(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatalf("pgxmock.NewPool() error = %v", err)
	}
	defer mock.Close()

	cfg := config.Config{
		HTTPAddr:    ":9090",
		PostgresDSN: "postgres://runtime",
		Auth: config.AuthConfig{
			APIKeys: []string{"key-a"},
		},
	}

	runtime, err := buildAPIRuntime(context.Background(), cfg, apiRuntimeDependencies{
		openPool: func(ctx context.Context, dsn string) (postgresRuntimeStore, error) {
			return mock, nil
		},
		bootstrapDatabase: func(ctx context.Context, db postgresRuntimeStore) error {
			return nil
		},
		newServer: func(addr string, deps HTTPDependencies) httpServer {
			return NewHTTPServer(addr, deps)
		},
	})
	if err != nil {
		t.Fatalf("buildAPIRuntime() error = %v", err)
	}

	server, ok := runtime.server.(*http.Server)
	if !ok {
		t.Fatalf("server type = %T, want *http.Server", runtime.server)
	}

	sourceTime := time.Date(2026, 5, 31, 12, 0, 0, 0, time.UTC)
	mock.ExpectQuery("SELECT[\\s\\S]*FROM raw_events").
		WithArgs("tenant-a", "project-a", "namespace-a", pgxmock.AnyArg()).
		WillReturnRows(pgxmock.NewRows([]string{"pending_governance", "leased_governance"}).AddRow(0, 0))
	mock.ExpectBegin()
	mock.ExpectQuery("INSERT INTO raw_events").
		WithArgs("tenant-a", "project-a", "namespace-a", "conversation.message", "hello world", pgxmock.AnyArg(), sourceTime).
		WillReturnRows(pgxmock.NewRows([]string{"id", "tenant", "project", "namespace", "event_type", "content", "source_timestamp", "created_at"}).
			AddRow("evt_123", "tenant-a", "project-a", "namespace-a", "conversation.message", "hello world", sourceTime, sourceTime))
	mock.ExpectExec("INSERT INTO provenance_links").
		WithArgs(
			pgxmock.AnyArg(),
			"evt_123",
			nil,
			nil,
			"tenant-a",
			"project-a",
			"namespace-a",
			"ingest_event",
			nil,
			nil,
			pgxmock.AnyArg(),
			pgxmock.AnyArg(),
		).
		WillReturnResult(pgxmock.NewResult("INSERT", 1))
	mock.ExpectCommit()

	body, err := json.Marshal(map[string]any{
		"event_type":       "conversation.message",
		"content":          "hello world",
		"source_timestamp": sourceTime.Format(time.RFC3339),
		"metadata":         map[string]any{"channel": "chat"},
	})
	if err != nil {
		t.Fatalf("json.Marshal() error = %v", err)
	}

	successReq := httptest.NewRequest(http.MethodPost, "/v1/events", bytes.NewReader(body))
	successReq.Header.Set("Content-Type", "application/json")
	successReq.Header.Set("X-API-Key", "key-a")
	successReq.Header.Set("X-Stele-Tenant", "tenant-a")
	successReq.Header.Set("X-Stele-Project", "project-a")
	successReq.Header.Set("X-Stele-Namespace", "namespace-a")
	successRec := httptest.NewRecorder()

	server.Handler.ServeHTTP(successRec, successReq)

	if successRec.Code != http.StatusCreated {
		t.Fatalf("success status = %d, want %d", successRec.Code, http.StatusCreated)
	}

	var response eventIngestResponse
	if err := json.NewDecoder(successRec.Body).Decode(&response); err != nil {
		t.Fatalf("json.NewDecoder() error = %v", err)
	}

	if response.EventID != "evt_123" {
		t.Fatalf("EventID = %q, want %q", response.EventID, "evt_123")
	}
	if response.Admission == nil || response.Admission.Decision != memory.AdmissionPressureDecisionAccept {
		t.Fatalf("Admission = %+v, want accept metadata", response.Admission)
	}

	invalidPayloadReq := httptest.NewRequest(http.MethodPost, "/v1/events", strings.NewReader(`{"event_type":"","content":""}`))
	invalidPayloadReq.Header.Set("Content-Type", "application/json")
	invalidPayloadReq.Header.Set("X-API-Key", "key-a")
	invalidPayloadReq.Header.Set("X-Stele-Tenant", "tenant-a")
	invalidPayloadReq.Header.Set("X-Stele-Project", "project-a")
	invalidPayloadReq.Header.Set("X-Stele-Namespace", "namespace-a")
	invalidPayloadRec := httptest.NewRecorder()

	server.Handler.ServeHTTP(invalidPayloadRec, invalidPayloadReq)

	if invalidPayloadRec.Code != http.StatusBadRequest {
		t.Fatalf("invalid payload status = %d, want %d", invalidPayloadRec.Code, http.StatusBadRequest)
	}

	missingKeyReq := httptest.NewRequest(http.MethodPost, "/v1/events", strings.NewReader(`{"event_type":"conversation.message","content":"hello"}`))
	missingKeyReq.Header.Set("Content-Type", "application/json")
	missingKeyReq.Header.Set("X-Stele-Tenant", "tenant-a")
	missingKeyReq.Header.Set("X-Stele-Project", "project-a")
	missingKeyReq.Header.Set("X-Stele-Namespace", "namespace-a")
	missingKeyRec := httptest.NewRecorder()

	server.Handler.ServeHTTP(missingKeyRec, missingKeyReq)

	if missingKeyRec.Code != http.StatusUnauthorized {
		t.Fatalf("missing key status = %d, want %d", missingKeyRec.Code, http.StatusUnauthorized)
	}

	invalidScopeReq := httptest.NewRequest(http.MethodPost, "/v1/events", strings.NewReader(`{"event_type":"conversation.message","content":"hello"}`))
	invalidScopeReq.Header.Set("Content-Type", "application/json")
	invalidScopeReq.Header.Set("X-API-Key", "key-a")
	invalidScopeReq.Header.Set("X-Stele-Tenant", "tenant-a")
	invalidScopeReq.Header.Set("X-Stele-Project", "project-a")
	invalidScopeRec := httptest.NewRecorder()

	server.Handler.ServeHTTP(invalidScopeRec, invalidScopeReq)

	if invalidScopeRec.Code != http.StatusBadRequest {
		t.Fatalf("invalid scope status = %d, want %d", invalidScopeRec.Code, http.StatusBadRequest)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("ExpectationsWereMet() error = %v", err)
	}
}
