# Durable Worker Orchestration And Scope Maintenance Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Harden Stele's background governance runtime with durable raw event retry state, renewable worker leases, and scope-aware maintenance dispatch without changing the public API surface.

**Architecture:** Extend the existing PostgreSQL-backed worker model rather than introducing a new queue. Keep `internal/jobs` as the orchestration layer, teach `internal/storage/postgres` to persist raw event failure and retry state, and wrap the current maintenance jobs with a scope-dispatch layer so compaction and retention run per eligible memory scope while cleanup remains runtime-global.

**Tech Stack:** Go, `net/http`, PostgreSQL, `pgx`, `pgxmock`, existing `internal/jobs`, `internal/governance`, `internal/storage/postgres`, existing self-hosting docs

---

## File Map

- `internal/governance/contracts.go`
  - Add durable raw event failure and lease-renewal contracts that the worker can call without coupling to PostgreSQL specifics.
- `internal/governance/contracts_test.go`
  - Cover validation of the new worker-side retry and lease inputs.
- `internal/jobs/jobs.go`
  - Add durable governance worker orchestration, per-claim failure recording, lease renewal, and scope-aware maintenance dispatch wrappers.
- `internal/jobs/governance_worker_test.go`
  - Cover retry recording, exhausted-event handling, and lease-renew behavior for governance claims.
- `internal/jobs/jobs_test.go`
  - Cover scope dispatch behavior and maintenance idempotency across discovered scopes.
- `internal/storage/postgres/migrations/0001_base_schema.up.sql`
  - Extend `raw_events` with retry bookkeeping fields and any supporting index for claim eligibility.
- `internal/storage/postgres/repository.go`
  - Update raw event claim eligibility query, add failure and renewal mutations, and add maintenance scope discovery.
- `internal/storage/postgres/repository_test.go`
  - Verify retry wait filtering, exhausted filtering, failure persistence, lease renewal, and maintenance scope discovery queries.
- `internal/storage/postgres/bootstrap_test.go`
  - Assert the base schema now includes the new `raw_events` columns and any new index statements.
- `internal/config/config.go`
  - Add durable worker and scope-dispatch configuration parsing.
- `internal/config/config_test.go`
  - Verify new env vars parse and validate correctly.
- `internal/app/app.go`
  - Wire the new worker settings into `GovernanceWorker` and replace static scope-bound maintenance jobs with scope-dispatch jobs.
- `internal/app/app_test.go`
  - Assert runtime assembly uses the durable worker config and the new scheduler composition.
- `docs/self-hosting.md`
  - Document retry budget, exhausted raw event semantics, scope-aware maintenance dispatch, and new job env vars.
- `openspec/changes/durable-worker-orchestration-and-scope-maintenance/tasks.md`
  - Mark tasks complete as implementation lands.

### Task 1: Durable Raw Event Persistence Contract

**Files:**
- Modify: `internal/governance/contracts.go`
- Modify: `internal/governance/contracts_test.go`
- Modify: `internal/storage/postgres/migrations/0001_base_schema.up.sql`
- Modify: `internal/storage/postgres/bootstrap_test.go`
- Modify: `internal/storage/postgres/repository.go`
- Modify: `internal/storage/postgres/repository_test.go`

- [ ] **Step 1: Write the failing governance contract and repository tests**

```go
func TestRecordClaimedRawEventFailureInputValidate(t *testing.T) {
	valid := governance.RecordClaimedRawEventFailureInput{
		RawEventID:    "evt_123",
		WorkerID:      "worker-a",
		FailedAt:      time.Date(2026, 6, 10, 12, 0, 0, 0, time.UTC),
		ErrorMessage:  "candidate extraction failed",
		NextAttemptAt: time.Date(2026, 6, 10, 12, 0, 30, 0, time.UTC),
	}

	if err := valid.Validate(); err != nil {
		t.Fatalf("Validate() error = %v", err)
	}
}

func TestRenewClaimedRawEventLeaseInputValidateRejectsNonIncreasingLease(t *testing.T) {
	input := governance.RenewClaimedRawEventLeaseInput{
		RawEventID:  "evt_123",
		WorkerID:    "worker-a",
		RenewedAt:   time.Date(2026, 6, 10, 12, 0, 0, 0, time.UTC),
		LeaseUntil:  time.Date(2026, 6, 10, 12, 0, 0, 0, time.UTC),
	}

	if err := input.Validate(); err == nil {
		t.Fatal("Validate() error = nil, want lease ordering error")
	}
}

func TestRepositoryClaimPendingRawEventsSkipsRetryWaitAndExhausted(t *testing.T) {
	mock.ExpectQuery("WITH claimed AS \\(").
		WithArgs("worker-a", now, now.Add(2*time.Minute), 8).
		WillReturnRows(pgxmock.NewRows([]string{
			"id", "tenant", "project", "namespace", "event_type", "content", "source_timestamp", "created_at",
			"governance_worker_id", "governance_claimed_at", "governance_lease_until", "governance_attempt",
		}))
}

func TestRepositoryRecordClaimedRawEventFailureSchedulesRetry(t *testing.T) {
	mock.ExpectExec("UPDATE raw_events").
		WithArgs(
			"evt_123",
			"worker-a",
			failedAt,
			"candidate extraction failed",
			nextAttemptAt,
			nil,
		).
		WillReturnResult(pgxmock.NewResult("UPDATE", 1))
}

func TestRepositoryRenewClaimedRawEventLeaseRejectsLostOwnership(t *testing.T) {
	mock.ExpectExec("UPDATE raw_events").
		WithArgs("evt_123", "worker-a", renewedAt, leaseUntil).
		WillReturnResult(pgxmock.NewResult("UPDATE", 0))
}
```

- [ ] **Step 2: Run focused contract and repository tests to verify RED**

Run: `go test ./internal/governance ./internal/storage/postgres -run "RecordClaimedRawEventFailure|RenewClaimedRawEventLease|ClaimPendingRawEventsSkipsRetryWaitAndExhausted" -count=1`

Expected: FAIL with missing input types, missing repository methods, or unmet SQL expectations.

- [ ] **Step 3: Implement the durable raw event contracts and persistence layer**

```go
type RecordClaimedRawEventFailureInput struct {
	RawEventID    string
	WorkerID      string
	FailedAt      time.Time
	ErrorMessage  string
	NextAttemptAt time.Time
	ExhaustedAt   time.Time
}

type RenewClaimedRawEventLeaseInput struct {
	RawEventID string
	WorkerID   string
	RenewedAt  time.Time
	LeaseUntil time.Time
}

type RawEventFailureRecorder interface {
	RecordClaimedRawEventFailure(ctx context.Context, input RecordClaimedRawEventFailureInput) error
}

type RawEventLeaseRenewer interface {
	RenewClaimedRawEventLease(ctx context.Context, input RenewClaimedRawEventLeaseInput) error
}
```

```sql
ALTER TABLE raw_events
    ADD COLUMN IF NOT EXISTS governance_last_failed_at timestamptz,
    ADD COLUMN IF NOT EXISTS governance_last_error text,
    ADD COLUMN IF NOT EXISTS governance_next_attempt_at timestamptz,
    ADD COLUMN IF NOT EXISTS governance_exhausted_at timestamptz;

CREATE INDEX IF NOT EXISTS raw_events_governance_claim_idx
    ON raw_events (
        governance_processed_at,
        governance_exhausted_at,
        governance_next_attempt_at,
        governance_lease_until,
        created_at
    );
```

```go
func (r *Repository) ClaimPendingRawEvents(ctx context.Context, input governance.ClaimPendingRawEventsInput) ([]governance.ClaimedRawEvent, error) {
	const query = `
WITH claimed AS (
	UPDATE raw_events
	SET
		governance_worker_id = $1,
		governance_claimed_at = $2,
		governance_lease_until = $3,
		governance_attempt = governance_attempt + 1
	WHERE id IN (
		SELECT id
		FROM raw_events
		WHERE governance_processed_at IS NULL
			AND governance_exhausted_at IS NULL
			AND (governance_next_attempt_at IS NULL OR governance_next_attempt_at <= $2)
			AND (governance_lease_until IS NULL OR governance_lease_until <= $2)
		ORDER BY created_at ASC
		LIMIT $4
		FOR UPDATE SKIP LOCKED
	)
	RETURNING ...
) SELECT ... FROM claimed
`
	// existing scan path remains unchanged
}

func (r *Repository) RecordClaimedRawEventFailure(ctx context.Context, input governance.RecordClaimedRawEventFailureInput) error {
	const query = `
UPDATE raw_events
SET
	governance_last_failed_at = $3,
	governance_last_error = $4,
	governance_next_attempt_at = $5,
	governance_exhausted_at = $6,
	governance_lease_until = NULL
WHERE id = $1
	AND governance_worker_id = $2
	AND governance_processed_at IS NULL
`
	tag, err := r.db.Exec(ctx, query, input.RawEventID, input.WorkerID, input.FailedAt, input.ErrorMessage, nullTime(input.NextAttemptAt), nullTime(input.ExhaustedAt))
	if err != nil {
		return fmt.Errorf("record claimed raw event failure: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return governance.ErrClaimOwnershipLost
	}
	return nil
}

func (r *Repository) RenewClaimedRawEventLease(ctx context.Context, input governance.RenewClaimedRawEventLeaseInput) error {
	const query = `
UPDATE raw_events
SET
	governance_claimed_at = $3,
	governance_lease_until = $4
WHERE id = $1
	AND governance_worker_id = $2
	AND governance_processed_at IS NULL
	AND governance_exhausted_at IS NULL
`
	// return governance.ErrClaimOwnershipLost on zero rows
}
```

- [ ] **Step 4: Re-run the focused contract and repository tests to verify GREEN**

Run: `go test ./internal/governance ./internal/storage/postgres -run "RecordClaimedRawEventFailure|RenewClaimedRawEventLease|ClaimPendingRawEventsSkipsRetryWaitAndExhausted" -count=1`

Expected: PASS

- [ ] **Step 5: Commit the persistence foundation**

```bash
git add internal/governance/contracts.go internal/governance/contracts_test.go internal/storage/postgres/migrations/0001_base_schema.up.sql internal/storage/postgres/bootstrap_test.go internal/storage/postgres/repository.go internal/storage/postgres/repository_test.go
git commit -m "feat: add durable raw event retry persistence"
```

### Task 2: Governance Worker Retry, Exhaustion, And Lease Renewal

**Files:**
- Modify: `internal/jobs/jobs.go`
- Modify: `internal/jobs/governance_worker_test.go`

- [ ] **Step 1: Write the failing governance worker tests**

```go
func TestGovernanceWorkerRunOnceRecordsRetryableFailureAndContinues(t *testing.T) {
	failureRecorder := &stubRawEventFailureRecorder{}
	processor := &stubRawEventProcessor{err: errors.New("boom")}
	worker := jobs.GovernanceWorker{
		Claimer:         claimerWithClaims(newClaimedRawEvent(t, "evt_fail", now), newClaimedRawEvent(t, "evt_ok", now.Add(time.Second))),
		Processor:       processor,
		FailureRecorder: failureRecorder,
		WorkerID:        "worker-a",
		BatchSize:       2,
		LeaseDuration:   2 * time.Minute,
		RetryBackoff:    30 * time.Second,
		MaxAttempts:     3,
		Now:             func() time.Time { return now },
	}

	processed, err := worker.RunOnce(context.Background())
	if err != nil {
		t.Fatalf("RunOnce() error = %v", err)
	}
	if processed != 1 {
		t.Fatalf("processed = %d, want 1 successful claim", processed)
	}
	if len(failureRecorder.inputs) != 1 {
		t.Fatalf("len(inputs) = %d, want 1 retryable failure", len(failureRecorder.inputs))
	}
}

func TestGovernanceWorkerRunOnceExhaustsRawEventAtMaxAttempts(t *testing.T) {
	claim := newClaimedRawEvent(t, "evt_exhaust", now)
	claim.Attempt = 3

	recorder := &stubRawEventFailureRecorder{}
	worker := jobs.GovernanceWorker{
		Claimer:         claimerWithClaims(claim),
		Processor:       &stubRawEventProcessor{err: errors.New("boom")},
		FailureRecorder: recorder,
		WorkerID:        "worker-a",
		BatchSize:       1,
		LeaseDuration:   2 * time.Minute,
		RetryBackoff:    30 * time.Second,
		MaxAttempts:     3,
		Now:             func() time.Time { return now },
	}

	if _, err := worker.RunOnce(context.Background()); err != nil {
		t.Fatalf("RunOnce() error = %v", err)
	}
	if recorder.inputs[0].ExhaustedAt.IsZero() {
		t.Fatal("ExhaustedAt = zero, want terminal failure timestamp")
	}
}

func TestGovernanceWorkerRunOnceRenewsLeaseDuringLongProcessing(t *testing.T) {
	renewer := &stubRawEventLeaseRenewer{}
	processor := &blockingProcessor{release: make(chan struct{})}
	worker := jobs.GovernanceWorker{
		Claimer:            claimerWithClaims(newClaimedRawEvent(t, "evt_lease", now)),
		Processor:          processor,
		LeaseRenewer:       renewer,
		WorkerID:           "worker-a",
		BatchSize:          1,
		LeaseDuration:      2 * time.Minute,
		LeaseRenewInterval: 5 * time.Second,
		Now:                advancingNow(now, 5*time.Second, 10*time.Second),
	}
}
```

- [ ] **Step 2: Run the focused governance worker tests to verify RED**

Run: `go test ./internal/jobs -run "GovernanceWorkerRunOnceRecordsRetryableFailureAndContinues|GovernanceWorkerRunOnceExhaustsRawEventAtMaxAttempts|GovernanceWorkerRunOnceRenewsLeaseDuringLongProcessing" -count=1`

Expected: FAIL with missing worker fields, missing failure recorder calls, or missing lease-renew behavior.

- [ ] **Step 3: Implement durable governance worker orchestration**

```go
type GovernanceWorker struct {
	Claimer            governance.RawEventClaimer
	Processor          governance.RawEventProcessor
	FailureRecorder    governance.RawEventFailureRecorder
	LeaseRenewer       governance.RawEventLeaseRenewer
	WorkerID           string
	BatchSize          int
	LeaseDuration      time.Duration
	LeaseRenewInterval time.Duration
	MaxAttempts        int
	RetryBackoff       time.Duration
	Now                func() time.Time
	Observer           telemetry.Observer
}
```

```go
func (w GovernanceWorker) RunOnce(ctx context.Context) (processed int, err error) {
	claims, err := w.Claimer.ClaimPendingRawEvents(ctx, input)
	if err != nil {
		return 0, err
	}

	for _, claim := range claims {
		ok, claimErr := w.processClaim(ctx, claim)
		if claimErr != nil {
			return processed, claimErr
		}
		if ok {
			processed++
		}
	}
	return processed, nil
}

func (w GovernanceWorker) processClaim(ctx context.Context, claim governance.ClaimedRawEvent) (bool, error) {
	runCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	renewalErrCh := w.startLeaseRenewal(runCtx, claim)
	err := w.Processor.ProcessClaimedRawEvent(runCtx, claim)
	cancel()

	if renewalErr := waitRenewalErr(renewalErrCh); renewalErr != nil {
		return false, renewalErr
	}
	if err == nil {
		return true, nil
	}
	if recordErr := w.recordClaimFailure(ctx, claim, err); recordErr != nil {
		return false, recordErr
	}
	return false, nil
}

func (w GovernanceWorker) recordClaimFailure(ctx context.Context, claim governance.ClaimedRawEvent, cause error) error {
	failedAt := w.nowUTC()
	input := governance.RecordClaimedRawEventFailureInput{
		RawEventID:    claim.Event.ID,
		WorkerID:      claim.WorkerID,
		FailedAt:      failedAt,
		ErrorMessage:  truncateError(cause.Error(), 512),
		NextAttemptAt: failedAt.Add(w.retryBackoff()),
	}
	if claim.Attempt >= w.maxAttempts() {
		input.NextAttemptAt = time.Time{}
		input.ExhaustedAt = failedAt
	}
	return w.FailureRecorder.RecordClaimedRawEventFailure(ctx, input)
}
```

- [ ] **Step 4: Re-run the focused governance worker tests to verify GREEN**

Run: `go test ./internal/jobs -run "GovernanceWorkerRunOnceRecordsRetryableFailureAndContinues|GovernanceWorkerRunOnceExhaustsRawEventAtMaxAttempts|GovernanceWorkerRunOnceRenewsLeaseDuringLongProcessing" -count=1`

Expected: PASS

- [ ] **Step 5: Commit the durable worker loop**

```bash
git add internal/jobs/jobs.go internal/jobs/governance_worker_test.go
git commit -m "feat: harden governance worker retry and lease flow"
```

### Task 3: Scope-Aware Maintenance Dispatch

**Files:**
- Modify: `internal/jobs/jobs.go`
- Modify: `internal/jobs/jobs_test.go`
- Modify: `internal/storage/postgres/repository.go`
- Modify: `internal/storage/postgres/repository_test.go`

- [ ] **Step 1: Write the failing scope dispatch and discovery tests**

```go
func TestScopeDispatchJobRunsScopedMaintenanceAcrossEligibleScopes(t *testing.T) {
	source := &stubMaintenanceScopeSource{
		scopes: []memory.Scope{
			{Tenant: "tenant-a", Project: "project-a", Namespace: "ns-a"},
			{Tenant: "tenant-b", Project: "project-b", Namespace: "ns-b"},
		},
	}
	job := jobs.ScopeDispatchJob{
		NameValue:       "summary_compaction_dispatch",
		ScopeSource:     source,
		ScopeBatchLimit: 10,
		Dispatch: func(scope memory.Scope) jobs.MaintenanceJob {
			return &stubScopedMaintenanceJob{name: "summary_compaction", scope: scope}
		},
	}

	processed, err := job.Run(context.Background())
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if processed != 2 {
		t.Fatalf("processed = %d, want 2", processed)
	}
}

func TestScopeDispatchJobUsesFallbackScopeWhenDiscoveryReturnsNone(t *testing.T) {
	job := jobs.ScopeDispatchJob{
		NameValue:       "retention_sweep_dispatch",
		ScopeSource:     &stubMaintenanceScopeSource{},
		ScopeBatchLimit: 10,
		FallbackScope:   memory.Scope{Tenant: "tenant-a", Project: "project-a", Namespace: "namespace-a"},
		Dispatch: func(scope memory.Scope) jobs.MaintenanceJob {
			return &stubScopedMaintenanceJob{name: "retention_sweep", scope: scope}
		},
	}
}

func TestRepositoryListMaintenanceScopesReturnsDistinctEligibleScopes(t *testing.T) {
	mock.ExpectQuery("SELECT tenant, project, namespace FROM \\(").
		WithArgs(50).
		WillReturnRows(pgxmock.NewRows([]string{"tenant", "project", "namespace"}).
			AddRow("tenant-a", "project-a", "namespace-a").
			AddRow("tenant-b", "project-b", "namespace-b"))
}
```

- [ ] **Step 2: Run the focused scheduler and discovery tests to verify RED**

Run: `go test ./internal/jobs ./internal/storage/postgres -run "ScopeDispatchJob|ListMaintenanceScopesReturnsDistinctEligibleScopes" -count=1`

Expected: FAIL with missing scope source interfaces, missing dispatch job, or missing repository discovery method.

- [ ] **Step 3: Implement scope-aware maintenance dispatch**

```go
type MaintenanceScopeSource interface {
	ListMaintenanceScopes(ctx context.Context, limit int) ([]memory.Scope, error)
}

type ScopeDispatchJob struct {
	NameValue       string
	ScopeSource     MaintenanceScopeSource
	ScopeBatchLimit int
	FallbackScope   memory.Scope
	Dispatch        func(scope memory.Scope) MaintenanceJob
}

func (j ScopeDispatchJob) Name() string {
	return j.NameValue
}

func (j ScopeDispatchJob) Run(ctx context.Context) (int, error) {
	scopes, err := j.ScopeSource.ListMaintenanceScopes(ctx, positiveOrDefault(j.ScopeBatchLimit, 100))
	if err != nil {
		return 0, err
	}
	if len(scopes) == 0 && j.FallbackScope.Validate() == nil {
		scopes = []memory.Scope{j.FallbackScope}
	}

	total := 0
	for _, scope := range scopes {
		job := j.Dispatch(scope)
		if job == nil {
			continue
		}
		processed, err := job.Run(ctx)
		if err != nil {
			return total, err
		}
		total += processed
	}
	return total, nil
}
```

```go
func (r *Repository) ListMaintenanceScopes(ctx context.Context, limit int) ([]memory.Scope, error) {
	const query = `
SELECT tenant, project, namespace
FROM (
	SELECT DISTINCT tenant, project, namespace
	FROM canonical_memories
	WHERE state = 'active'
	UNION
	SELECT DISTINCT tenant, project, namespace
	FROM candidate_memories
	WHERE status = 'promoted'
) scoped
ORDER BY tenant, project, namespace
LIMIT $1
`
	// scan []memory.Scope and return it
}
```

- [ ] **Step 4: Re-run the focused scheduler and discovery tests to verify GREEN**

Run: `go test ./internal/jobs ./internal/storage/postgres -run "ScopeDispatchJob|ListMaintenanceScopesReturnsDistinctEligibleScopes" -count=1`

Expected: PASS

- [ ] **Step 5: Commit the scope-aware scheduler primitives**

```bash
git add internal/jobs/jobs.go internal/jobs/jobs_test.go internal/storage/postgres/repository.go internal/storage/postgres/repository_test.go
git commit -m "feat: add scope-aware maintenance dispatch"
```

### Task 4: Runtime Configuration And App Wiring

**Files:**
- Modify: `internal/config/config.go`
- Modify: `internal/config/config_test.go`
- Modify: `internal/app/app.go`
- Modify: `internal/app/app_test.go`

- [ ] **Step 1: Write the failing config and runtime assembly tests**

```go
func TestLoadFromEnvParsesDurableWorkerSettings(t *testing.T) {
	t.Setenv("STELE_MODE", "worker")
	t.Setenv("STELE_POSTGRES_DSN", "postgres://example")
	t.Setenv("STELE_JOBS_GOVERNANCE_MAX_ATTEMPTS", "7")
	t.Setenv("STELE_JOBS_GOVERNANCE_RETRY_BACKOFF", "45s")
	t.Setenv("STELE_JOBS_GOVERNANCE_LEASE_RENEW_INTERVAL", "20s")
	t.Setenv("STELE_JOBS_MAINTENANCE_SCOPE_BATCH_LIMIT", "25")

	cfg, err := config.LoadFromEnv()
	if err != nil {
		t.Fatalf("LoadFromEnv() error = %v", err)
	}
	if cfg.Jobs.GovernanceMaxAttempts != 7 {
		t.Fatalf("GovernanceMaxAttempts = %d, want 7", cfg.Jobs.GovernanceMaxAttempts)
	}
}

func TestBuildWorkerRuntimeWiresDurableRetrySettings(t *testing.T) {
	poller := runtime.worker.(jobs.PollingWorker)
	worker, ok := poller.Worker.(jobs.GovernanceWorker)
	if !ok {
		t.Fatalf("poller.Worker type = %T, want jobs.GovernanceWorker", poller.Worker)
	}
	if worker.MaxAttempts != 7 {
		t.Fatalf("MaxAttempts = %d, want 7", worker.MaxAttempts)
	}
}

func TestBuildSchedulerRuntimeAssemblesScopeDispatchJobs(t *testing.T) {
	scheduler := runtime.scheduler.(jobs.MaintenanceScheduler)
	if _, ok := scheduler.Jobs[0].(jobs.ScopeDispatchJob); !ok {
		t.Fatalf("scheduler.Jobs[0] type = %T, want jobs.ScopeDispatchJob", scheduler.Jobs[0])
	}
}
```

- [ ] **Step 2: Run the focused config and app tests to verify RED**

Run: `go test ./internal/config ./internal/app -run "DurableWorkerSettings|BuildWorkerRuntimeWiresDurableRetrySettings|BuildSchedulerRuntimeAssemblesScopeDispatchJobs" -count=1`

Expected: FAIL with missing config fields, missing env parsing, or unchanged runtime composition.

- [ ] **Step 3: Implement durable runtime configuration and wiring**

```go
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
```

```go
func LoadFromEnv() (Config, error) {
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
	// assign onto cfg.Jobs
}
```

```go
const governanceLeaseDuration = 2 * time.Minute

worker := jobs.GovernanceWorker{
	Claimer:            repo,
	Processor:          pipeline,
	FailureRecorder:    repo,
	LeaseRenewer:       repo,
	WorkerID:           "stele-worker",
	BatchSize:          32,
	LeaseDuration:      governanceLeaseDuration,
	LeaseRenewInterval: cfg.Jobs.GovernanceLeaseRenewPeriod,
	MaxAttempts:        cfg.Jobs.GovernanceMaxAttempts,
	RetryBackoff:       cfg.Jobs.GovernanceRetryBackoff,
	Now:                now,
	Observer:           deps.observer,
}

scheduler := jobs.MaintenanceScheduler{
	Jobs: []jobs.MaintenanceJob{
		jobs.ScopeDispatchJob{
			NameValue:       "summary_compaction_dispatch",
			ScopeSource:     repo,
			ScopeBatchLimit: cfg.Jobs.MaintenanceScopeBatchLimit,
			FallbackScope:   scope,
			Dispatch: func(scope memory.Scope) jobs.MaintenanceJob {
				return jobs.SummaryCompactionJob{Scope: scope, CutoffWindow: summaryInterval, Cadence: summaryInterval, Now: now, Processor: summary, ExecutionStore: repo, TriggerSource: "scheduler"}
			},
		},
		jobs.ScopeDispatchJob{
			NameValue:       "retention_sweep_dispatch",
			ScopeSource:     repo,
			ScopeBatchLimit: cfg.Jobs.MaintenanceScopeBatchLimit,
			FallbackScope:   scope,
			Dispatch: func(scope memory.Scope) jobs.MaintenanceJob {
				return jobs.RetentionSweepJob{Scope: scope, Cadence: retentionInterval, Now: now, Source: repo, Evaluator: retention, ExecutionStore: repo, TriggerSource: "scheduler", Limit: 100}
			},
		},
		jobs.JobExecutionCleanupJob{Scope: scope, Cadence: cleanupInterval, RetentionWindow: cfg.Jobs.JobExecutionRetention, Now: now, Cleaner: repo, ExecutionStore: repo, TriggerSource: "scheduler"},
	},
}
```

- [ ] **Step 4: Re-run the focused config and app tests to verify GREEN**

Run: `go test ./internal/config ./internal/app -run "DurableWorkerSettings|BuildWorkerRuntimeWiresDurableRetrySettings|BuildSchedulerRuntimeAssemblesScopeDispatchJobs" -count=1`

Expected: PASS

- [ ] **Step 5: Commit the runtime wiring**

```bash
git add internal/config/config.go internal/config/config_test.go internal/app/app.go internal/app/app_test.go
git commit -m "feat: wire durable worker and scope-aware scheduler runtime"
```

### Task 5: Documentation, OpenSpec Tracking, And Verification

**Files:**
- Modify: `docs/self-hosting.md`
- Modify: `openspec/changes/durable-worker-orchestration-and-scope-maintenance/tasks.md`

- [ ] **Step 1: Write the docs and task-state assertions**

```md
- `STELE_JOBS_GOVERNANCE_MAX_ATTEMPTS`
- `STELE_JOBS_GOVERNANCE_RETRY_BACKOFF`
- `STELE_JOBS_GOVERNANCE_LEASE_RENEW_INTERVAL`
- `STELE_JOBS_MAINTENANCE_SCOPE_BATCH_LIMIT`
- exhausted raw events stop automatic claim until a later explicit recovery surface exists
- scheduler dispatches scope-bound jobs per eligible scope and keeps cleanup runtime-global
```

```md
- [x] 1.1 Define the durable raw event execution fields and derived internal states for retryable failure, retry wait, processed, and exhausted outcomes
- [x] 5.3 Run focused regression verification for `internal/jobs`, `internal/governance`, `internal/storage/postgres`, and `internal/app`
```

- [ ] **Step 2: Update the operator docs and OpenSpec task file**

```md
Job tuning variables:

- `STELE_JOBS_GOVERNANCE_MAX_ATTEMPTS`: automatic retry budget for claimed governance raw events, default `5`
- `STELE_JOBS_GOVERNANCE_RETRY_BACKOFF`: retry wait after a failed governance attempt, default `30s`
- `STELE_JOBS_GOVERNANCE_LEASE_RENEW_INTERVAL`: cadence for renewing an in-flight governance claim lease, default `30s`
- `STELE_JOBS_MAINTENANCE_SCOPE_BATCH_LIMIT`: maximum discovered scopes evaluated per scheduler tick, default `100`

Operational notes:

- worker records retryable raw event failures with bounded retry state instead of relying only on lease expiry
- raw events that hit the retry ceiling are marked exhausted and stop automatic claim
- summary compaction and retention sweep are dispatched per eligible scope
- job execution cleanup remains runtime-global and runs once per cadence window
```

- [ ] **Step 3: Run the focused regression suites**

Run: `go test ./internal/jobs ./internal/governance ./internal/storage/postgres ./internal/config ./internal/app -count=1`

Expected: PASS

- [ ] **Step 4: Run the full repository verification**

Run: `go test ./... -count=1 -timeout 15m`

Expected: PASS

- [ ] **Step 5: Mark completed OpenSpec tasks and commit**

```bash
git add docs/self-hosting.md openspec/changes/durable-worker-orchestration-and-scope-maintenance/tasks.md
git commit -m "docs: finalize durable worker orchestration proposal"
```

## Coverage Check

- Durable raw event retry state:
  - Covered by Task 1 and Task 2.
- Exhausted terminal behavior:
  - Covered by Task 1 repository semantics and Task 2 worker tests.
- Renewable lease:
  - Covered by Task 1 contract and Task 2 orchestration.
- Scope-aware maintenance dispatch:
  - Covered by Task 3 and Task 4.
- Runtime configuration:
  - Covered by Task 4.
- Operator docs and verification:
  - Covered by Task 5.
