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
	"github.com/FelixSeptem/stele/internal/jobs"
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
	called bool
	err    error
}

func (s *stubAPIServer) ListenAndServe() error {
	s.called = true
	return s.err
}

func (s *stubAPIServer) Shutdown(ctx context.Context) error {
	return nil
}

type stubAppObserver struct {
	operations []telemetry.OperationEvent
	backlogs   []telemetry.BacklogEvent
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

	if gotDeps.Readiness == nil {
		t.Fatal("Readiness = nil, want readiness checker")
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

func TestBuildWorkerRuntimeWiresDurableRetrySettings(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatalf("pgxmock.NewPool() error = %v", err)
	}
	defer mock.Close()

	cfg := config.Config{
		Mode:        config.ModeWorker,
		PostgresDSN: "postgres://runtime",
		Jobs: config.JobConfig{
			WorkerPollInterval:         7 * time.Second,
			WorkerErrorBackoff:         11 * time.Second,
			GovernanceMaxAttempts:      7,
			GovernanceRetryBackoff:     45 * time.Second,
			GovernanceLeaseRenewPeriod: 20 * time.Second,
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

	worker, ok := poller.Worker.(jobs.GovernanceWorker)
	if !ok {
		t.Fatalf("poller.Worker type = %T, want jobs.GovernanceWorker", poller.Worker)
	}

	if worker.MaxAttempts != 7 {
		t.Fatalf("MaxAttempts = %d, want 7", worker.MaxAttempts)
	}

	if worker.RetryBackoff != 45*time.Second {
		t.Fatalf("RetryBackoff = %v, want 45s", worker.RetryBackoff)
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

	if len(scheduler.Jobs) != 3 {
		t.Fatalf("len(scheduler.Jobs) = %d, want 3", len(scheduler.Jobs))
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
			MaintenanceInterval:        30 * time.Second,
			SchedulerErrorBackoff:      time.Minute,
			MaintenanceScopeBatchLimit: 25,
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

	if len(scheduler.Jobs) != 3 {
		t.Fatalf("len(scheduler.Jobs) = %d, want 3", len(scheduler.Jobs))
	}

	dispatchA, ok := scheduler.Jobs[0].(jobs.ScopeDispatchJob)
	if !ok {
		t.Fatalf("scheduler.Jobs[0] type = %T, want jobs.ScopeDispatchJob", scheduler.Jobs[0])
	}

	dispatchB, ok := scheduler.Jobs[1].(jobs.ScopeDispatchJob)
	if !ok {
		t.Fatalf("scheduler.Jobs[1] type = %T, want jobs.ScopeDispatchJob", scheduler.Jobs[1])
	}

	if dispatchA.ScopeBatchLimit != 25 || dispatchB.ScopeBatchLimit != 25 {
		t.Fatalf("scope batch limits = (%d, %d), want 25", dispatchA.ScopeBatchLimit, dispatchB.ScopeBatchLimit)
	}

	if dispatchA.FallbackScope.Namespace != "namespace-a" || dispatchB.FallbackScope.Namespace != "namespace-a" {
		t.Fatalf("fallback scopes = (%+v, %+v), want default namespace", dispatchA.FallbackScope, dispatchB.FallbackScope)
	}

	if _, ok := scheduler.Jobs[2].(jobs.JobExecutionCleanupJob); !ok {
		t.Fatalf("scheduler.Jobs[2] type = %T, want jobs.JobExecutionCleanupJob", scheduler.Jobs[2])
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
