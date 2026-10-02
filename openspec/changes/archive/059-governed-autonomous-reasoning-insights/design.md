## Context

The proposal builds on the existing reasoning-provider boundary, derived
insight substrate, reserved-insight activation policy, durable replay queue,
context authorization, and bounded observability. The new path must preserve
PostgreSQL as the system of record, exact tenant/project/namespace isolation,
append-only provenance, lifecycle visibility, and canonical-memory immutability.
See `proposal.md` and the delta specs for the externally visible contract.

## Goals / Non-Goals

**Goals:**

- Normalize provider reasoning into a small, provider-neutral candidate envelope.
- Make evidence, scope, lifecycle, freshness, redaction, uncertainty, and
  replay identity first-class validation inputs.
- Keep offline/shadow evaluation deterministic and non-authoritative while
  reusing the existing admission and replay governance for any apply path.
- Produce useful operator evidence without exposing prompts, chain-of-thought,
  source content, raw provider payloads, or high-cardinality identifiers.

**Non-Goals:**

- No new persistence system, graph model, public SDK/UI/MCP surface, or default
  retrieval behavior.
- No autonomous activation of reserved insight types and no provider authority
  over canonical or derived lifecycle state.

## Decisions

### 1. Use one normalized candidate envelope

The orchestration layer will resolve scope and policies, select bounded
eligible evidence, and create a deterministic input digest before invoking a
provider. The adapter returns normalized candidate fields: reserved type,
bounded claim/value, evidence references and digest, uncertainty/confidence,
provider/schema identity, refusal/error category, and operation mode. Raw
prompts, chain-of-thought, and provider payloads are discarded or retained only
in an explicitly redacted diagnostic form.

This keeps provider adapters interchangeable and lets replay run from the
normalized envelope. A provider-specific result schema or embedding-provider
coupling was rejected because it would make governance and deterministic replay
depend on one vendor.

### 2. Validate before and after provider invocation

Pre-invocation checks enforce exact scope, lifecycle visibility, redaction,
budget, watermark, and provider capability. Post-invocation checks resolve
citations back to the same authorized evidence set, reject hidden/foreign or
stale sources, enforce type and uncertainty bounds, and reject mutation or
direct-activation instructions. The result is either a quarantined candidate,
a non-authoritative disposition, or an explicit handoff to the existing
reserved-insight policy.

This two-sided validation is preferred over trusting structured output alone,
because a valid JSON response can still cite data the request was not allowed
to see or request an unsafe state change.

### 3. Derive replay identity from normalized inputs

The replay key will cover normalized request fields, exact scope proof, sorted
eligible evidence references plus source watermark, provider/schema identity,
policy versions, and operation mode. Candidate fingerprints additionally cover
normalized type, claim/value, citations, and uncertainty. Replays use these
identities for idempotency and compare compatibility before any apply work.

This reuses existing durable replay and idempotency patterns instead of adding
a second replay system. Nondeterministic provider calls are never required for
offline replay; a missing compatible envelope produces a stale/incomplete
disposition.

### 4. Keep shadow and apply paths separate

Offline and shadow runs write candidate/replay evidence and bounded reports but
cannot create active insights or affect retrieval/context. A bounded admin
apply request enqueues durable work that invokes the existing admission policy,
which alone creates an append-only derived version and audit transition.

The separation makes rollback straightforward and prevents a diagnostic run
from becoming an accidental release mechanism. A direct provider-to-storage
path was rejected because it would bypass policy, idempotency, and audit rules.

### 5. Reuse existing context and observability gates

Reasoning candidates remain outside default context. Future reasoning sections
must pass the same scope, lifecycle, freshness, citation, and budget checks as
other optional insight sections and require explicit authorization. Metrics and
logs use fixed categories and buckets, with diagnostics restricted to authorized
operators and aggregate counts.

## Risks / Trade-offs

- [Provider output may be plausible but unsupported] -> Require source citations,
  watermarks, uncertainty bounds, and policy-specific evidence thresholds; keep
  all initial execution offline/shadow.
- [Source changes make a replay appear different] -> Persist source watermarks
  and evidence digests, mark stale inputs explicitly, and fail closed rather
  than silently refreshing evidence.
- [Candidate volume or provider latency grows without bound] -> Enforce per-run
  evidence, token, candidate, timeout, and queue limits and report exhaustion as
  a bounded category.
- [A future context integration leaks reserved insights] -> Keep the default
  assembler unaware of non-authoritative results and require an explicit,
  versioned section policy for any activation-derived exposure.
- [Telemetry leaks sensitive reasoning data] -> Centralize field allowlists and
  reject or bucket raw scope, identifiers, prompts, payloads, and errors before
  metric or log emission.

## Migration Plan

1. Add the candidate envelope, validation, persistence, replay, and telemetry
   contracts behind offline/shadow-only operation; existing `failure_pattern`
   and `lesson` behavior remains unchanged.
2. Run focused unit/integration tests plus owned PostgreSQL + pgvector shadow
   verification for evidence isolation, deterministic replay, stale fallback,
   canonical immutability, and rollback diagnostics.
3. Enable an exact-scope reserved-insight policy only after its evidence and
   replay report meet the existing activation gate. Activation is independently
   disableable and rollback returns the system to shadow-only behavior.
4. On rollback, disable the policy and stop new admissions; preserve candidate,
   evidence, audit, and prior insight versions for inspection and replay.
