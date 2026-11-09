## Why

Stele can retain provenance, confidence, feedback, task outcomes, and
Provider-attributed corrections, but none of those records establishes whether
a reusable procedural-memory claim was actually checked against attributable
evidence. A repeated failure can support a failure pattern without proving that
its proposed remedy works. This leaves a gap between memory maintenance signals
and a governed, auditable correction of an incorrect procedure.

The evidence-verified memory maintenance roadmap prioritizes a narrow first
loop: record a verification result for one active procedural-memory version,
revalidate it when its evidence becomes unsuitable, and route a refuted claim's
correction through existing append-only governance. The loop must be measurable
in later externally judged tasks without Stele claiming that it caused an
outcome.

## What Changes

- Add an admin-only, exact-scope verification ledger for claims made by an
  active procedural-memory version. Each immutable record identifies the target
  memory/version and claim/applicability fingerprint, an attributable method,
  supporting or contrary evidence references with their observed source
  versions, a bounded disposition, actor, reason, policy version, and time.
- Support the bounded dispositions `supported`, `refuted`, `inconclusive`, and
  `needs_revalidation`. A transcript or model-generated assertion is retained
  as provenance but cannot alone constitute independent corroboration. A later
  incompatible source version appends a `needs_revalidation` result rather than
  changing an earlier verification record.
- Add a bounded worker-maintained revalidation path. Explicit correction,
  source-version drift, and eligible stale/review feedback can enqueue one
  exact-scope verification review. The worker records a terminal verification
  result or a bounded unresolved outcome; it never reads foreign, hidden,
  forgotten, deleted, or non-current evidence.
- Allow a refuted verification with an administrator-supplied corrected
  procedural-memory payload to submit an ordinary governed `update` intent.
  Existing asynchronous governance remains the only route to a candidate and
  append-only successor version. Replays, source/target changes, and policy
  disablement must not create duplicate successors or make an unverified claim
  active.
- Publish lifecycle-safe admin inspection and OpenAPI contracts for verification
  history and revalidation status. Ordinary retrieval and Provider context do
  not gain a verification-history route or broader visibility.
- Extend task evaluations and task-series fixtures with opaque, externally
  judged verification references and a separate repeated-error metric. Reports
  continue to label condition differences as observational, keep utility,
  stale/noisy harm, evidence integrity, repeated-error rate, and maintenance
  cost separate, and contain no raw claims, task content, scopes, or IDs.
- Add PostgreSQL migrations, exact-scope repositories, configuration/resource
  limits, API/worker wiring, admin documentation, and regression/owned-stack
  verification for the loop.

## Non-goals

- Do not add a sixth memory class, alter the canonical lifecycle, or overwrite
  canonical memory in place.
- Do not treat a verification record, feedback, source popularity, transcript,
  model output, or task verdict as direct canonical mutation authority.
- Do not execute an agent, invoke a model, construct prompts, infer task
  success, make a causal claim from task-series results, or make experimental
  retrieval behavior active.
- Do not replace the existing closed Provider correction contract, session
  verification, repair-plan verification, or task-evaluation contracts.
- Do not introduce a second store, file-backed wiki, graph database, SDK, UI,
  or public runtime access to administrative verification evidence.

## Capabilities

### New Capabilities

- `verified-procedural-memory-claims`: exact-scope, append-only verification
  evidence and revalidation disposition for claims made by active procedural
  memory versions, including bounded correction handoff.

### Modified Capabilities

- `canonical-memory-lifecycle`: require a verification-driven correction to
  create a governed successor rather than changing a current version in place.
- `governed-memory-intents`: define verification-attributed update admission,
  worker rechecks, idempotency, and nonactive outcomes alongside the existing
  Provider correction association.
- `memory-history-and-provenance`: expose authorized verification and
  revalidation lineage beside canonical version and correction history.
- `task-success-evaluation`: allow durable external task evidence to reference
  a scoped procedural-memory verification without treating the verdict as
  mutation authority.
- `longitudinal-agent-task-memory-evaluation`: report externally labeled
  repeated-error and maintenance-cost observations independently from utility,
  stale/noisy harm, provenance coverage, and safety.
- `admin-inspection-surface`: add lifecycle-safe admin inspection of
  verification history and revalidation results.
- `runtime-api-contract-publication`: publish the new privileged request and
  response schemas, validation failures, authorization boundary, and
  compatibility behavior in OpenAPI.

## Impact

- **Domain and storage:** `internal/memory`, `internal/storage/postgres`, and
  a new versioned PostgreSQL migration add immutable claim-verification and
  revalidation records, source-version checks, and exact-scope indexes.
- **Async processing:** `internal/jobs`, `internal/workqueue`, configuration,
  API composition, and scheduler/worker observability acquire a bounded,
  disableable verification-review work kind. Existing governed update-intent
  processing remains authoritative for correction promotion.
- **HTTP/OpenAPI/admin:** `internal/app`, `openapi/spec.go`, administrator
  documentation, README/integration guidance, and the Stele agent skill gain
  the privileged inspection/recording contract; public Provider and ordinary
  retrieval behavior remain unchanged.
- **Evaluation:** `internal/evaluation`, `internal/benchmark`, task-series
  fixtures, and maintenance documentation add deterministic, redacted
  verification references and separate aggregate metrics.
- **Verification:** focused domain/repository/API/worker tests, migration and
  restart tests, full Go/race/vet/OpenSpec validation, and an owned PostgreSQL
  + pgvector evidence path will be required before activation claims.

Related workflow commands: `openspec validate --strict`, `go test ./...`,
`go test -race ./...`, `go vet ./...`, and
`pwsh -File scripts/openspec-archive-seq.ps1 -ChangeName
"verified-memory-correction-loop"`.
