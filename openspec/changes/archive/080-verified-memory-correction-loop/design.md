## Context

See [proposal.md](proposal.md) for motivation and the new/modified behavior
contracts under `specs/`. Existing session verification proves scope/context
assembly behavior; repair-plan verification proves a repair plan ran; the closed
Provider correction contract validates one Provider projection revision. None is
the durable truth claim for an active procedural-memory version.

Stele already provides exact-scope authorization, PostgreSQL append-only history,
active-lifecycle filtering, governed update intents, derived-work leases,
feedback/quality findings, task evaluations, and a checksum-locked longitudinal
benchmark. The design must preserve these boundaries. In particular, the
service does not execute agents, infer task success, or call an LLM to decide
whether a procedure is true.

## Goals / Non-Goals

**Goals:**

- Persist a narrow, auditable verification ledger for current active
  `procedural` canonical-memory versions.
- Separate corroboration quality from provenance, confidence, popularity,
  feedback, and lifecycle.
- Detect source drift and stale/review signals asynchronously, append a
  revalidation result, and let an administrator correct a refuted procedure
  through the existing governed update path.
- Provide exact-scope admin/OpenAPI inspection and deterministic evaluation
  inputs without widening public Provider/retrieval access.

**Non-Goals:**

- The worker does not semantically judge a procedure, invent a correction, or
  turn a feedback signal into a `supported`/`refuted` result. Only an
  attributable administrator submission may assert those dispositions.
- This change does not generalize to profile, episodic, summary, relation, or
  derived-insight claims; subsequent proposals can reuse the ledger pattern.
- The work does not replace Provider corrections, task evaluation, reflection,
  quality repair, or add active hierarchical retrieval/context placement.

## Decisions

### 1. Make claim verification a new append-only domain, not a memory state

Add `internal/memory/procedural_verification.go` with closed enums for
disposition (`supported`, `refuted`, `inconclusive`, `needs_revalidation`),
method (`tool_observation`, `user_correction`, `external_evaluation`,
`transcript`, `model_assertion`), evidence polarity, trigger, and run status.

`ProceduralMemoryVerification` contains immutable target memory/version,
SHA-256 claim and applicability fingerprints, policy version, actor/reason,
source watermark, evidence, optional bounded corrected content, and a
verification idempotency fingerprint. Evidence records contain a typed source
reference, observed source version, method, polarity, and corroboration flag.
The service derives the fingerprint from normalized target/version, claim,
applicability, evidence source versions, disposition, correction content,
policy, and scope. It never accepts a caller-provided effective fingerprint.

The ledger targets a current active `procedural` canonical memory version only.
`supported` requires at least one same-scope independently corroborating source;
`transcript` and `model_assertion` are attributable evidence but are not
independent corroboration. `refuted` may contain contrary evidence and optional
corrected content. `inconclusive` represents an attributable check whose result
does not settle the claim. `needs_revalidation` records source/evidence drift
and never changes the old result.

This keeps lifecycle meaning stable and avoids a new memory class. Using a
confidence field or a generic insight was rejected because neither can express
which current canonical version and evidence versions were checked, nor can it
enforce source-version revalidation.

### 2. Use two PostgreSQL tables with immutable results and mutable work state

Migration `0044_procedural_memory_verification` creates:

- `procedural_memory_verifications`: append-only terminal verification records,
  scoped target/version references, normalized fingerprints, evidence JSONB,
  corrected payload metadata, policy/actor/reason/watermark, and timestamps.
  It receives a scoped idempotency unique index and a trigger preventing update
  and delete.
- `procedural_memory_revalidation_runs`: exact-scope durable execution records
  keyed by run id, target/version, trigger, watermark, input verification, work
  identity, status, attempt/lease/backoff/failure fields, and terminal
  verification link. This table follows existing reflection/session-run patterns:
  work-state fields are lease-owned and transition monotonically; it is not a
  substitute for the immutable evidence ledger.

Foreign keys link to canonical-memory identity where existing schema permits;
scope predicates remain mandatory in every query to avoid using a raw ID as an
authority. Evidence source references remain JSONB because supported sources
span existing events, canonical versions, usefulness feedback, and external
evaluation records; service validation resolves each typed reference through
its existing exact-scope repository before admission. The normalized target
version and source versions are retained independently so later source drift is
detectable even if payload representation changes.

A fully generic evidence graph was rejected: it would expand the first slice
into cross-class semantics and duplicate provenance storage. New indexed tables
are preferable to overloading session verification or repair-plan tables, whose
lifetimes and target semantics differ.

### 3. Admit assertions synchronously; keep revalidation mechanical and bounded

`ProceduralVerificationService.Record` validates administrator attribution,
exact scope, current active target/version, bounded text/evidence limits,
evidence eligibility, method/disposition compatibility, and idempotency. It
persists one immutable ledger row. It never calls a model or searches arbitrary
transcripts.

`ProceduralRevalidationService.Request` creates one durable run from an
administrator trigger, source-version drift, explicit correction signal, or
eligible `stale`/`needs_review` feedback. The request has a per-scope policy,
max attempts, source watermark, and deterministic idempotency identity. It
enqueues `workqueue.WorkKindProceduralRevalidation` whose payload remains the
run identifier only.

`jobs.ProceduralRevalidationWorker` claims runs with `FOR UPDATE SKIP LOCKED`
through a repository interface. It re-resolves only the recorded evidence and
target. If any source is no longer current/eligible, it appends a
`needs_revalidation` verification and completes the run. If evidence stays
eligible but no administrator assertion exists, it records an explicit
`inconclusive` result. It never upgrades a claim to supported or creates
corrected text. Retryable storage failures retain the run for bounded retry;
invalid/scope/policy conditions finish as bounded terminal results.

This avoids converting an LLM or feedback aggregation into a hidden authority.
It also makes the first loop useful immediately: operators receive durable,
actionable drift evidence and can perform an attributable correction.

### 4. Delegate correction promotion to the existing governed update pipeline

A refuted verification with corrected content calls a narrow
`SubmitVerificationCorrection` adapter. It first rechecks the verification is
current/refuted, the target is current/active at the expected version, all cited
sources remain eligible, and the procedural-verification policy is enabled. It
then constructs an ordinary `update` intent with a new immutable
`VerificationAttribution` (verification id, target version, source watermark,
policy, and fingerprint).

The existing intent repository, River handoff, candidate governance, and
canonical promotion remain the sole authority. Their idempotency identity is
extended to include verification attribution. Worker admission rechecks it in
the same way it already rechecks Provider correction attribution. A stale
verification/target produces a normal nonactive transition; no automatic retry
can attach a newer target without a new verification.

Adding a direct canonical update endpoint was rejected because it would violate
the governed write path and create two correction authorities. Reusing the
Provider correction association was rejected because its binding/projection
preconditions are deliberately closed and do not describe administrator-held
verification evidence.

### 5. Keep access privileged and publish a closed OpenAPI contract

Add admin-only endpoints under `/v1/admin/procedural-memory-verifications` for
recording an assertion, requesting revalidation, listing history by target, and
reading a run/detail. Correction handoff is an explicit admin action against a
refuted verification. Handlers resolve principal and exact scope before record
lookup, use safe not-found/denial behavior, and return bounded identities/status
without raw evidence payloads by default. Detail is limited to authorized
administrators and remains source/lifecycle filtered.

`openapi/spec.go`, admin interfaces, DTOs, error helpers, and contract tests
define the exact request/response shape. README, architecture/components,
self-hosting, integration guide, and `skills/stele-memory/SKILL.md` explain
that this is administrator maintenance, not Provider runtime context.

### 6. Extend evaluation without contaminating task outcomes

Extend `TaskEvidenceKind` with `procedural_verification` after verifying a
same-scope ledger record. Extend the checksum-locked task-series fixture schema
to allow optional external boolean `repeated_error` and a nonnegative bounded
maintenance-cost observation per stage. Version the schema and metric constants
to `task-series-v2` / `task-series-metrics-v2`; v1 fixtures remain parseable
only when their declared version selects v1 behavior, preserving archived
reproducibility.

For v2, reports expose aggregate evaluated/repeated-error coverage and rate,
maintenance-observation coverage and total/average, all separate from existing
utility, stale/noisy, provenance, and safety results. Missing observations are
not zero. Reports continue to omit stage text, evidence aliases, scopes, memory
ids, and raw costs. The benchmark does not call the new admin APIs or mutate a
production scope; fixtures model conditions in isolated benchmark-owned scopes.

## Risks / Trade-offs

- **Administrator evidence can still be wrong** → require attributable method,
  actor, reason, source-version refs, immutable history, and later contrary
  results; never treat support as instruction authority.
- **Evidence validation spans multiple repositories** → define one resolver
  interface owned by `internal/memory`, enforce scope/lifecycle before the
  idempotency store, and test every source kind including foreign/hidden cases.
- **New work can accumulate** → use per-scope enablement, enqueue limits,
  deterministic coalescing, lease expiry, maximum attempts, and bounded
  retention; expose only aggregate operational telemetry.
- **Correction races can promote an obsolete procedure** → persist expected
  target/source versions, recheck at update admission and worker promotion, and
  terminally suppress stale work.
- **Task-series v2 can weaken reproducibility** → retain v1 parser/evaluator,
  checksum the complete v2 source, reject unknown fields, and make missing
  observations explicit rather than silently zero.
- **Admin details may disclose sensitive content** → default list views to
  bounded metadata, resolve grants before lookup, preserve source trust and
  lifecycle filters, and keep public Provider/MCP paths closed.

## Migration Plan

1. Ship migration `0044` with new tables, indexes, scope guards, append-only
   trigger, and compatibility-safe down migration that disables new consumers
   without erasing verification history.
2. Deploy binaries with procedural verification policy disabled by default;
   existing API, worker, scheduler, retrieval, Provider, and task-series v1
   behavior remain unchanged.
3. Enable the policy only for an explicitly provisioned exact scope after admin
   authorization, queue configuration, OpenAPI publication, focused tests, and
   owned PostgreSQL + pgvector verification evidence pass.
4. Roll back by disabling new admission/dispatch and routes where required;
   retain append-only verification, run, intent, candidate, successor, and
   provenance history. Do not delete rows, rewrite canonical versions, or
   requeue completed history as rollback.
5. Retain v1 task-series support for locked fixtures; v2 is opt-in by manifest
   version and requires its own source/qrel locks.
