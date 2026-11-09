# Verified Memory Correction Loop Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add attributable verification and bounded revalidation for active procedural-memory claims, with refuted corrections promoted only through the existing governed update path and measured by a redacted longitudinal fixture.

**Architecture:** A new append-only PostgreSQL verification ledger targets an active current procedural-memory version; separate lease-owned revalidation runs detect that recorded evidence drifted but never infer truth. Administrator assertions can record supported/refuted/inconclusive results. A refuted assertion with corrected content creates a verification-attributed ordinary update intent, and the current intent worker remains the only route to a canonical successor. Task evaluation and task-series v2 link/measure the result without judging external work.

**Tech Stack:** Go, PostgreSQL/pgx, Riverqueue, existing derived work queue, net/http, OpenAPI, repository-owned Go tests, PostgreSQL + pgvector owned-stack verification.

---

## File structure

- `internal/memory/procedural_verification.go`: verification/revalidation domain types, validation, fingerprints, store/service contracts.
- `internal/memory/governed_intent*.go`: verification attribution and correction admission over existing update intents.
- `internal/storage/postgres/procedural_verification.go` plus migration `0044_*`: durable exact-scope ledger/runs, evidence rechecks, and attribution persistence.
- `internal/jobs/procedural_verification*.go`, `internal/workqueue/contract.go`, `internal/config`: bounded dispatch, policy, claiming/retry, runtime registration, and telemetry.
- `internal/app/*`, `openapi/spec.go`: administrator endpoints, wiring, OpenAPI schemas, and auth/contract coverage.
- `internal/memory/task_evaluation.go`, `internal/benchmark/longitudinal.go`: verification evidence links and checksum-locked v2 aggregate metrics.
- `docs/`, `README.md`, `skills/stele-memory/SKILL.md`: administrator operation, limits, compatibility, rollback, and public boundary documentation.

### Task 1: Establish the test-first procedural verification domain

**Files:**
- Create: `internal/memory/procedural_verification.go`
- Create: `internal/memory/procedural_verification_test.go`

- [ ] **Step 1: Write failing domain tests.**

```go
func TestProceduralVerificationRequiresIndependentSupport(t *testing.T) {
    record := validProceduralVerification()
    record.Disposition = ProceduralVerificationSupported
    record.Evidence = []ProceduralVerificationEvidence{{
        Kind: ProceduralVerificationEvidenceTranscript,
        ID: "event-1", SourceVersion: "v1", Corroborates: false,
    }}
    if err := record.Validate(); err == nil || !strings.Contains(err.Error(), "independent corroboration") {
        t.Fatalf("Validate() error = %v", err)
    }
}

func TestProceduralVerificationFingerprintIncludesTargetAndEvidenceVersions(t *testing.T) {
    left := validProceduralVerification()
    right := left
    right.Evidence[0].SourceVersion = "v2"
    leftID, _ := ProceduralVerificationFingerprint(left)
    rightID, _ := ProceduralVerificationFingerprint(right)
    if leftID == rightID { t.Fatal("source-version change must change identity") }
}
```

- [ ] **Step 2: Run the domain tests to verify RED.**

Run: `go test ./internal/memory -run 'TestProceduralVerification' -count=1`

Expected: compilation fails because the procedural verification domain does not exist.

- [ ] **Step 3: Implement minimal closed types and validation.**

```go
type ProceduralVerificationDisposition string
const (
    ProceduralVerificationSupported           ProceduralVerificationDisposition = "supported"
    ProceduralVerificationRefuted             ProceduralVerificationDisposition = "refuted"
    ProceduralVerificationInconclusive        ProceduralVerificationDisposition = "inconclusive"
    ProceduralVerificationNeedsRevalidation   ProceduralVerificationDisposition = "needs_revalidation"
)

type ProceduralMemoryVerification struct {
    ID, MemoryID, ClaimFingerprint, ApplicabilityFingerprint string
    TargetVersion int64
    Scope Scope
    Disposition ProceduralVerificationDisposition
    Evidence []ProceduralVerificationEvidence
    Actor, Reason, PolicyVersion, SourceWatermark string
    CorrectedContent string
    CreatedAt time.Time
}
```

Require active procedural targets in service/repository validation, keep `model_assertion`/`transcript` noncorroborating, bound every field, and hash normalized scope/target/version/claim/applicability/disposition/evidence/correction/policy.

- [ ] **Step 4: Run the focused domain suite to verify GREEN.**

Run: `go test ./internal/memory -run 'TestProceduralVerification' -count=1`

Expected: PASS.

- [ ] **Step 5: Commit the domain layer.**

```powershell
git add internal/memory/procedural_verification.go internal/memory/procedural_verification_test.go
git commit -m "feat(memory): define procedural verification ledger"
```

### Task 2: Persist append-only verification and revalidation records

**Files:**
- Create: `internal/storage/postgres/migrations/0044_procedural_memory_verification.up.sql`
- Create: `internal/storage/postgres/migrations/0044_procedural_memory_verification.down.sql`
- Create: `internal/storage/postgres/procedural_verification.go`
- Create: `internal/storage/postgres/procedural_verification_test.go`
- Create: `internal/storage/postgres/procedural_verification_integration_test.go`

- [ ] **Step 1: Write failing repository/integration tests.**

```go
func TestProceduralVerificationRepositoryIsExactScopeAndAppendOnly(t *testing.T) {
    record := seedProceduralVerification(t, scopeA)
    if _, err := repo.ReadProceduralVerification(ctx, scopeB, record.ID); !errors.Is(err, memory.ErrNotFound) {
        t.Fatalf("foreign read = %v", err)
    }
    requireAppendOnlyTrigger(t, pool, "procedural_memory_verifications", record.ID)
}

func TestProceduralRevalidationRunLeaseReclaimsAfterExpiry(t *testing.T) {
    run := seedRevalidationRun(t, scopeA)
    first := claimRun(t, run, "worker-a", now)
    second := claimRun(t, run, "worker-b", now.Add(lease+time.Second))
    if first.ID != second.ID { t.Fatal("expired work must be reclaimable") }
}
```

- [ ] **Step 2: Run RED.**

Run: `go test ./internal/storage/postgres -run 'TestProcedural(Verification|Revalidation)' -count=1`

Expected: compile/schema failure because migration and repository methods are absent.

- [ ] **Step 3: Add migration and scoped repository.**

```sql
CREATE TABLE procedural_memory_verifications (
  id text PRIMARY KEY, tenant text NOT NULL, project text NOT NULL, namespace text NOT NULL,
  memory_id uuid NOT NULL, target_version integer NOT NULL,
  fingerprint text NOT NULL, disposition text NOT NULL, evidence jsonb NOT NULL,
  source_watermark text NOT NULL, policy_version text NOT NULL, actor text NOT NULL,
  reason text NOT NULL, corrected_content text, created_at timestamptz NOT NULL
);
CREATE UNIQUE INDEX procedural_memory_verifications_scope_fingerprint_idx
  ON procedural_memory_verifications (tenant, project, namespace, fingerprint);
```

Add the append-only trigger, exact-scope indexes, lease/run table, and non-destructive down migration. Repository reads/writes must use tenant/project/namespace predicates and resolve every source version before creating the record.

- [ ] **Step 4: Run GREEN.**

Run: `go test ./internal/storage/postgres -run 'TestProcedural(Verification|Revalidation)' -count=1`

Expected: PASS with exact-scope, source-drift, idempotency, and lease cases.

- [ ] **Step 5: Commit durable persistence.**

```powershell
git add internal/storage/postgres/migrations/0044_procedural_memory_verification.* internal/storage/postgres/procedural_verification*.go
git commit -m "feat(postgres): persist procedural verification evidence"
```

### Task 3: Add bounded revalidation dispatch and worker behavior

**Files:**
- Modify: `internal/workqueue/contract.go`
- Modify: `internal/workqueue/contract_test.go`
- Create: `internal/jobs/procedural_verification_dispatch.go`
- Create: `internal/jobs/procedural_verification_worker.go`
- Create: `internal/jobs/procedural_verification_worker_test.go`
- Modify: `internal/config/config.go`
- Modify: `internal/config/config_test.go`

- [ ] **Step 1: Write failing queue/worker tests.**

```go
func TestProceduralRevalidationDriftAppendsNeedsRevalidation(t *testing.T) {
    worker := fixtureRevalidationWorkerWithDrift()
    if _, err := worker.RunOnce(context.Background()); err != nil { t.Fatal(err) }
    if got := worker.store.lastVerification.Disposition; got != memory.ProceduralVerificationNeedsRevalidation {
        t.Fatalf("disposition = %s", got)
    }
}

func TestProceduralRevalidationNeverPromotesWithoutAdministratorAssertion(t *testing.T) {
    worker := fixtureRevalidationWorkerWithEligibleEvidence()
    _, _ = worker.RunOnce(context.Background())
    if worker.store.correctionSubmissions != 0 { t.Fatal("worker inferred a correction") }
}
```

- [ ] **Step 2: Run RED.**

Run: `go test ./internal/workqueue ./internal/jobs ./internal/config -run ProceduralRevalidation -count=1`

Expected: compile failure for missing work kind/dispatcher/worker/policy.

- [ ] **Step 3: Implement a lease-safe bounded worker.**

```go
const WorkKindProceduralRevalidation WorkKind = "procedural_revalidation"

func (w ProceduralRevalidationWorker) RunOnce(ctx context.Context) (int, error) {
    // Claim durable runs, re-resolve recorded evidence only, append
    // needs_revalidation on drift and inconclusive otherwise; never infer support,
    // generate content, or call governed correction submission.
}
```

Add per-scope enablement, batch, attempt, retention, and lease defaults. Dispatch only run IDs with the derived queue. Classify policy/scope/invalid input as terminal bounded outcomes and preserve retryable storage faults.

- [ ] **Step 4: Run GREEN and commit.**

Run: `go test ./internal/workqueue ./internal/jobs ./internal/config -run ProceduralRevalidation -count=1`

```powershell
git add internal/workqueue internal/jobs/procedural_verification* internal/config
git commit -m "feat(worker): revalidate procedural verification evidence"
```

### Task 4: Route refuted corrections through governed intents

**Files:**
- Modify: `internal/memory/governed_intent_contract.go`
- Modify: `internal/memory/governed_intent.go`
- Modify: `internal/memory/governed_intent_worker.go`
- Modify: `internal/memory/governed_intent*_test.go`
- Modify: `internal/storage/postgres/governed_memory_intent.go`
- Modify: `internal/storage/postgres/governed_memory_intent*_test.go`

- [ ] **Step 1: Write failing correction attribution tests.**

```go
func TestVerificationCorrectionDoesNotCreateUpdateForStaleTarget(t *testing.T) {
    service := fixtureVerificationCorrectionService(staleTargetVerification())
    _, err := service.SubmitCorrection(context.Background(), validCorrectionInput())
    if !errors.Is(err, memory.ErrVerificationCorrectionUnavailable) { t.Fatalf("err = %v", err) }
    if service.intents.created != 0 { t.Fatal("stale target created intent") }
}

func TestVerificationCorrectionReplayReturnsOriginalIntent(t *testing.T) {
    service := fixtureVerificationCorrectionService(refutedVerification())
    first, _ := service.SubmitCorrection(context.Background(), validCorrectionInput())
    second, _ := service.SubmitCorrection(context.Background(), validCorrectionInput())
    if first.ID != second.ID || service.intents.created != 1 { t.Fatal("replay duplicated intent") }
}
```

- [ ] **Step 2: Run RED.**

Run: `go test ./internal/memory ./internal/storage/postgres -run VerificationCorrection -count=1`

Expected: missing verification attribution and correction admission.

- [ ] **Step 3: Add immutable attribution and worker rechecks.**

```go
type VerificationAttribution struct {
    VerificationID, TargetMemoryID, SourceWatermark, PolicyVersion, Fingerprint string
    TargetVersion int64
}

func (a VerificationAttribution) Validate() error {
    for _, value := range []string{a.VerificationID, a.TargetMemoryID, a.SourceWatermark, a.PolicyVersion, a.Fingerprint} {
        if strings.TrimSpace(value) == "" || len(value) > 256 {
            return fmt.Errorf("verification attribution is invalid")
        }
    }
    if a.TargetVersion <= 0 { return fmt.Errorf("verification attribution target version is invalid") }
    return nil
}
```

Extend intent fingerprints/persistence without changing Provider correction semantics. At admission and worker promotion, re-resolve refuted verification, current active target/version, eligible cited sources, and enabled policy. Invalidated work creates the existing nonactive transition and never a candidate/successor.

- [ ] **Step 4: Run GREEN and commit.**

Run: `go test ./internal/memory ./internal/storage/postgres -run VerificationCorrection -count=1`

```powershell
git add internal/memory/governed_intent* internal/storage/postgres/governed_memory_intent*
git commit -m "feat(governance): attribute verified procedural corrections"
```

### Task 5: Publish privileged admin APIs and runtime wiring

**Files:**
- Modify: `internal/app/http.go`
- Modify: `internal/app/app.go`
- Create: `internal/app/procedural_verification_http.go`
- Create: `internal/app/procedural_verification_http_test.go`
- Modify: `openapi/spec.go`
- Create: `openapi/procedural_verification.go`
- Create: `openapi/procedural_verification_test.go`

- [ ] **Step 1: Write failing API authorization/contract tests.**

```go
func TestAdminProceduralVerificationRequiresGrantBeforeLookup(t *testing.T) {
    response := callAdmin(t, foreignAdminKey, http.MethodGet,
        "/v1/admin/procedural-memory-verifications/verification-1", nil)
    if response.Code != http.StatusNotFound { t.Fatalf("status = %d", response.Code) }
    if store.reads != 0 { t.Fatal("handler looked up foreign record") }
}

func TestProviderCredentialCannotCreateProceduralVerification(t *testing.T) {
    response := callProvider(t, http.MethodPost, "/v1/admin/procedural-memory-verifications", validBody())
    if response.Code != http.StatusForbidden { t.Fatalf("status = %d", response.Code) }
}
```

- [ ] **Step 2: Run RED.**

Run: `go test ./internal/app ./openapi -run ProceduralVerification -count=1`

Expected: absent routes, DTOs, and OpenAPI operations.

- [ ] **Step 3: Implement the admin boundary.**

```go
mux.Handle("POST /v1/admin/procedural-memory-verifications", adminProceduralVerification)
mux.Handle("GET /v1/admin/procedural-memory-verifications/{verification_id}", adminProceduralVerification)
mux.Handle("POST /v1/admin/procedural-memory-verifications/{verification_id}:revalidate", adminProceduralVerification)
mux.Handle("POST /v1/admin/procedural-memory-verifications/{verification_id}:correct", adminProceduralVerification)
```

Resolve exact scope/grants before lookup; use bounded DTOs and redacted error categories. Wire disabled-by-default policy, repository, dispatcher, worker, scheduler cleanup, telemetry, and OpenAPI schema/error declarations. Do not register these routes under Provider/MCP public runtime surfaces.

- [ ] **Step 4: Run GREEN and commit.**

Run: `go test ./internal/app ./openapi -run ProceduralVerification -count=1`

```powershell
git add internal/app openapi
git commit -m "feat(api): expose admin procedural verification workflow"
```

### Task 6: Link task evidence and add longitudinal task-series v2

**Files:**
- Modify: `internal/memory/task_evaluation.go`
- Modify: `internal/memory/task_evaluation_test.go`
- Modify: `internal/storage/postgres/task_evaluation_repository.go`
- Modify: `internal/storage/postgres/task_evaluation_repository_test.go`
- Modify: `internal/benchmark/longitudinal.go`
- Modify: `internal/benchmark/longitudinal_test.go`
- Create: `internal/benchmark/testdata/verified-memory-correction-task-series-v2.json`

- [ ] **Step 1: Write failing task evidence/v2 evaluation tests.**

```go
func TestTaskEvaluationAcceptsScopedProceduralVerificationEvidence(t *testing.T) {
    evaluation := validTaskEvaluation()
    evaluation.Evidence = append(evaluation.Evidence, TaskEvidenceLink{
        Kind: TaskEvidenceTargetProceduralVerification, ID: "verification-1",
    })
    if err := service.Record(ctx, evaluation); err != nil { t.Fatal(err) }
}

func TestTaskSeriesV2KeepsMissingRepeatedErrorSeparateFromZero(t *testing.T) {
    report, err := EvaluateTaskSeries(v2ManifestWithMissingRepeatedError(), observations)
    if err != nil { t.Fatal(err) }
    if report.Conditions[0].RepeatedErrorCoverage != 0 { t.Fatal("missing labeled as zero") }
}
```

- [ ] **Step 2: Run RED.**

Run: `go test ./internal/memory ./internal/storage/postgres ./internal/benchmark -run '(ProceduralVerificationTaskEvidence|TaskSeriesV2)' -count=1`

Expected: missing evidence kind and v2 manifest/report fields.

- [ ] **Step 3: Implement additive evidence and versioned metrics.**

```go
const TaskEvidenceTargetProceduralVerification TaskEvidenceTargetKind = "procedural_verification"

// Add these two fields to the existing TaskSeriesStage declaration.
RepeatedError   *bool  `json:"repeated_error,omitempty"`
MaintenanceCost *int64 `json:"maintenance_cost,omitempty"`
```

Validate verification evidence exact scope in the task-evaluation service. Preserve v1 parser/evaluator behavior, add v2 constants and separate aggregate rate/coverage/maintenance fields, lock a synthetic baseline/source/verified-corrected fixture, and omit aliases/cost-per-stage/task text from serialized reports.

- [ ] **Step 4: Run GREEN and commit.**

Run: `go test ./internal/memory ./internal/storage/postgres ./internal/benchmark -run '(ProceduralVerificationTaskEvidence|TaskSeriesV2|VerifiedMemoryCorrectionTaskSeries)' -count=1`

```powershell
git add internal/memory/task_evaluation* internal/storage/postgres/task_evaluation* internal/benchmark
git commit -m "feat(benchmark): measure verified memory corrections longitudinally"
```

### Task 7: Complete documentation, integration proof, and OpenSpec closure

**Files:**
- Modify: `docs/architecture.md`
- Modify: `docs/components.md`
- Modify: `docs/self-hosting.md`
- Modify: `docs/integrations/mcp-agent-skill.md`
- Modify: `docs/agent-runtime-memory-provider.md`
- Modify: `README.md`
- Modify: `skills/stele-memory/SKILL.md`
- Modify: `docs/roadmaps/2026-11-08-evidence-verified-memory-maintenance-roadmap.md`
- Create: `scripts/stele-procedural-verification-verify.ps1`
- Create: `docs/procedural-memory-verification_test.go`
- Modify: `openspec/changes/verified-memory-correction-loop/tasks.md`

- [ ] **Step 1: Write failing documentation and owned-stack contract tests.**

```go
func TestProceduralVerificationDocumentationMentionsAdminOnlyRollback(t *testing.T) {
    for _, item := range []struct{ path, needle string }{
        {"self-hosting.md", "procedural verification"},
        {"self-hosting.md", "rollback"},
        {"integrations/mcp-agent-skill.md", "administrator"},
    } {
        data, err := os.ReadFile(item.path)
        if err != nil || !strings.Contains(strings.ToLower(string(data)), item.needle) {
            t.Fatalf("%s must document %q: %v", item.path, item.needle, err)
        }
    }
}
```

- [ ] **Step 2: Run RED.**

Run: `pwsh -NoProfile -File scripts/check-documentation.ps1`

Expected: documentation contract is incomplete until administrator-only workflow, policy defaults, bounds, source trust, correction handoff, rollback, task-series v2 compatibility, and public-runtime denial are published.

- [ ] **Step 3: Implement documentation and owned-stack verification.**

The verification script must start only its owned temporary PostgreSQL 18 + pgvector dependency, cover API/worker restart, revalidation lease/retry, supported/refuted/inconclusive/drift, exact-scope denial, correction successor, duplicate replay, target conflict, projection invalidation, task evidence, and redacted report shape. It must retain only bounded statuses/hashes and clean its temporary resources.

- [ ] **Step 4: Run complete verification and update OpenSpec tasks.**

Run:

```powershell
go test ./... -count=1
go test -race ./...
go vet ./...
pwsh -NoProfile -File scripts/check-documentation.ps1
pwsh -File scripts/stele-procedural-verification-verify.ps1
openspec validate verified-memory-correction-loop --strict
openspec validate --all --strict
```

Expected: every command exits 0. Mark only verified `tasks.md` checkboxes complete.

- [ ] **Step 5: Commit verified change artifacts.**

```powershell
git add docs README.md skills scripts openspec/changes/verified-memory-correction-loop
git commit -m "docs(memory): document procedural verification operations"
```
