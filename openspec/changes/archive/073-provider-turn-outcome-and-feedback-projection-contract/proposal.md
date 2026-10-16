## Why

Provider turn, outcome, session, usefulness-feedback, insight, context-digest,
and durable-reference capabilities exist, but they do not yet form one explicit
public contract for a runtime to project its reported outcome and feedback with
safe attribution. The current generic Provider turn/outcome input shapes do not
make correction, pending-reference attribution, or the boundary between runtime
business state and Stele evidence independently interoperable or verifiable.

PC6 closes that final Provider integration gap after PC1–PC5: a runtime needs a
versioned way to record evidence without giving Stele authority over task state,
leases, generation commits, artifact publication, or delivery.

## What Changes

- Add the versioned `turn-outcome-feedback-projection-v1` Provider capability
  and dedicated closed public projection endpoint(s) for runtime-reported turn,
  outcome, reference, context-digest, usefulness, user-feedback, and
  task-completion-context evidence.
- Define exact-scope, runtime-binding-owned attribution; bounded operation and
  effect identity; source outcome revision; occurrence time; provenance; and
  idempotent replay. Equivalent delivery returns the original logical
  projection, while conflicting key reuse fails closed.
- Define append-only correction/supersession behavior that preserves prior
  projection evidence and audit history rather than overwriting it.
- Permit PC5 durable references in accepted/pending, completed, failed, and
  unavailable states as explicitly labeled attribution; pending evidence MUST
  not claim completed governance, current visibility, authorized content, or
  ranking eligibility.
- Reuse existing session, outcome, usefulness-feedback, governed insight and
  feedback-ranking/activation paths where sufficient. Recording feedback is
  separate from any later summary, derivation, ranking, or activation decision.
- Publish strict OpenAPI DTOs, capability negotiation, bounded errors, and
  consumer/conformance behavior. Establish fresh PostgreSQL + pgvector public
  fixture and API/worker restart evidence for replay, corrections, scope and
  lifecycle safety.
- Update maintenance documentation, self-hosting/integration guidance and agent
  skills for the new contract, migration and rollback behavior.

## Non-goals

- Do not create or own Danny task/revision state, schedules, occurrences,
  execution leases, generation commits, artifact publication, or webhook
  delivery.
- Do not turn a runtime-reported outcome into a completion, commit, delivery,
  or authorization receipt.
- Do not create a second canonical business-state store, a new autonomous
  reasoning loop, or automatic ranking/insight activation from one projection.
- Do not expose raw Provider payloads, foreign/hidden reference existence,
  unrestricted feedback history, or historical content through a projection.

## Capabilities

### New Capabilities

- `provider-turn-outcome-feedback-projection-contract`: versioned public
  contract for idempotent, correction-safe, exact-scope runtime evidence
  projection and its durable attribution outcomes.

### Modified Capabilities

- `agent-runtime-provider-adapter`: advertise and enforce the dedicated
  projection capability, strict Provider operation metadata, binding ownership,
  bounded errors and public conformance coverage.
- `runtime-memory-provider-contract`: require strict consumer negotiation and
  validation of projection requests/responses without local fallback or runtime
  business-state interpretation.
- `scope-proof-and-session-loop`: align session/turn/outcome evidence with the
  append-only projection identity, revision and correction semantics.
- `memory-usefulness-feedback`: permit governed runtime-projection attribution
  while retaining feedback idempotency, supersession, scoped access and the
  feedback log as source of truth.
- `governed-experience-insights`: constrain projection-derived feedback use to
  existing asynchronous, versioned, lifecycle-filtered derivation and activation
  gates.

## Impact

- Affected public surfaces: Provider capability/OpenAPI publication; new
  versioned projection route(s); existing Provider turn/outcome and usefulness
  feedback compatibility documentation.
- Likely implementation areas: `internal/app`, `internal/provider`,
  `internal/memory`, `internal/storage/postgres`, `internal/assurance`,
  `openapi`, verification scripts, documentation and `skills/stele-memory`.
- PostgreSQL remains the sole system of record. Reuse current session, outcome,
  feedback, governance, idempotency, provenance, scope and PC5 durable-reference
  infrastructure before adding any storage or dependency.
- Artifact references: use `openspec` proposal/apply/archive workflow; verify
  with `go test ./...`, strict OpenSpec validation, documentation gates and the
  repository's public PostgreSQL + pgvector Provider verifier.

