## Context

The service already has governed derived insights, append-only canonical memory
and provenance, exact tenant/project/namespace isolation, durable intent and
replay patterns, and separate runtime contracts for memory, embedding, and
reranker providers. See `proposal.md` for motivation. This design adds a
reasoning boundary that composes those contracts without making model execution
part of API, worker, scheduler, retrieval, or context startup.

## Goals / Non-Goals

**Goals:**

- Define one provider-neutral request/response envelope with explicit scope,
  evidence, policy, version, budget, deadline, and replay identity.
- Keep provider output candidate-only and route any accepted result through
  existing governance, provenance, lifecycle, and audit paths.
- Make disabled, offline, shadow, fallback, and stale-dependency states
  observable and deterministic.
- Provide safe conformance evidence that can compare providers without exposing
  prompts, chain-of-thought, credentials, hidden records, or raw payloads.

**Non-Goals:**

- Implementing a concrete vendor or local-model adapter.
- Defining prompt templates, model routing, tokenization, chain-of-thought
  storage, or final-answer generation.
- Enabling autonomous reserved insight types or changing default retrieval/context
  behavior.

## Decisions

### Separate reasoning from memory-provider and retrieval-provider contracts

Reasoning is a derivation dependency, not a memory transport and not a ranking
provider. It receives a bounded evidence packet and returns candidates or
intents; it cannot call arbitrary memory APIs, widen scope, or replace baseline
retrieval. This keeps the existing agent-runtime provider contract model-only
free and prevents reranker/embedding configuration from becoming a reasoning
authority.

Alternative considered: expose reasoning as an operation on the agent-runtime
memory provider. Rejected because it would couple model invocation to memory
transport and weaken the existing provider's explicit no-model boundary.

### Use versioned capability and envelope contracts

Capability discovery publishes provider/schema versions, operation kinds, mode,
and bounded limits. Every request carries normalized exact scope, evidence
references, operation/idempotency identity, policy/version, deadline, and input
and output budgets. Responses carry provider metadata, uncertainty/confidence,
evidence citations, and replay identity. Unknown versions or missing bounds fail
closed before provider execution.

Alternative considered: infer capabilities by attempting a model call. Rejected
because trial calls leak data, consume unbounded resources, and make startup or
readiness dependent on an external provider.

### Treat provider output as an evidence-backed candidate or intent

The boundary validates output shape, scope, evidence, size, and allowed insight
types before handing it to existing governance. Direct canonical writes,
lifecycle activation, grant changes, and configuration changes are impossible at
the contract level. Reserved insight types remain disabled until a later,
separately governed change.

Alternative considered: let the provider return an already-active insight.
Rejected because it would bypass admission, review, provenance, and lifecycle
policy.

### Make offline and shadow execution first-class but non-authoritative

Offline replay uses checksum-locked, redacted fixtures and normalized envelopes;
shadow execution may compare a provider result with an approved baseline. Both
produce diagnostic or candidate-only records and never alter canonical memory or
default context. Freshness and compatibility evidence are required before any
future activation decision.

Alternative considered: require live provider calls for conformance. Rejected
because self-hosted deployments must remain testable without vendor credentials
or network access, and live calls are not deterministic release evidence.

### Preserve baseline on failure

Timeouts, cancellations, provider outages, malformed output, stale inputs, and
budget violations map to low-cardinality categories. A configured fallback keeps
the prior deterministic behavior; retries use idempotency and bounded budgets.
No failure path silently retries forever or expands scope and resource limits.

## Risks / Trade-offs

- [Risk] A generic envelope may be too weak for future model families. ->
  Mitigation: version operation kinds and allow bounded provider-specific metadata
  only inside a redacted, size-limited extension field.
- [Risk] Shadow runs can consume significant cost or latency. -> Mitigation:
  require explicit enablement, concurrency/deadline budgets, and candidate-only
  retention with bounded diagnostics.
- [Risk] Operators may confuse a passing provider conformance run with insight
  quality or production readiness. -> Mitigation: keep conformance status,
  retrieval quality, and insight activation evidence as separate report families.
- [Risk] Model output may contain sensitive text or hidden identifiers. ->
  Mitigation: redact inputs and outputs, validate evidence against the visible
  candidate set, and never persist chain-of-thought or raw provider payloads.

## Migration Plan

No database migration or mandatory rollout is required for the boundary itself.
Implementation should add disabled-by-default configuration and contract types,
then introduce offline fixtures and shadow diagnostics before any live adapter.
Rollback consists of disabling the reasoning mode and retaining candidate or
diagnostic records; baseline retrieval, context assembly, and governed insight
processing continue without a reasoning provider.
