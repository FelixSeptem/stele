# reasoning-provider-boundary Specification

## Purpose
Define an optional provider-independent boundary for bounded model-assisted
reasoning that can produce replayable, evidence-backed candidates without
becoming a second authority, source of record, or default runtime dependency.

## Requirements

### Requirement: Reasoning provider capabilities are discoverable and bounded

The service SHALL represent a reasoning provider through a versioned capability
document that identifies provider identity, contract/schema versions, supported
operation kinds, configured input/output limits, deadline and concurrency bounds,
and whether live, shadow, or offline execution is enabled. Capability discovery
MUST NOT expose credentials, prompts, scope values, raw provider payloads, or
unbounded operational details.

#### Scenario: Reasoning is disabled

- **WHEN** no reasoning provider is configured or policy disables reasoning
- **THEN** capability discovery reports a bounded disabled state and API, worker, scheduler, retrieval, and context assembly remain usable without a reasoning dependency

#### Scenario: Provider capability is discovered

- **WHEN** an authorized operator or governed job requests the configured reasoning capability document
- **THEN** the response includes bounded versions, operation names, limits, and execution modes without secrets or prompt content

### Requirement: Reasoning invocations use bounded, scope-bound envelopes

Every reasoning invocation SHALL carry a resolved exact tenant/project/namespace
scope, operation and schema versions, request and operation identifiers,
idempotency metadata, a deadline, and explicit input/output budgets. The provider
MUST receive only redacted, scope-eligible evidence and MUST reject missing,
mismatched, widened, expired, or over-budget envelopes before execution.

#### Scenario: Valid derivation request is accepted

- **WHEN** a governed job submits a request with a resolved exact scope, bounded evidence, supported schema, and remaining execution budget
- **THEN** the provider receives a normalized request with replay metadata and no authority to access records outside that scope

#### Scenario: Invocation exceeds a bound

- **WHEN** input size, requested output size, deadline, concurrency, or evidence count exceeds the configured contract limit
- **THEN** the service rejects the request with a stable bounded validation or budget category without invoking the provider

#### Scenario: Provider attempts to widen scope

- **WHEN** a provider request or response references a different tenant, project, namespace, hidden record, or unapproved evidence source
- **THEN** the service fails closed and does not disclose foreign-record existence or apply the response

### Requirement: Reasoning outputs remain candidates or governed intents

Reasoning output SHALL be validated as bounded candidate records or governed
memory/insight intents containing scope, evidence citations, provenance,
derivation policy/version, provider metadata, confidence or uncertainty, and
replay identifiers. Provider output MUST NOT directly write canonical memory,
change lifecycle state, grant access, alter server configuration, or activate a
reserved insight type. A validated reserved candidate MAY enter a separate,
explicitly enabled activation policy that performs admission, lifecycle, audit,
and rollback decisions outside the provider boundary.

#### Scenario: Evidence-backed candidate is returned

- **WHEN** a provider returns a structurally valid derivation with allowed evidence and bounded metadata
- **THEN** the service records it as a candidate or intent for ordinary governance and preserves the source evidence and provider provenance

#### Scenario: Reserved candidate enters governed activation

- **WHEN** a provider returns a valid reserved insight candidate and an exact-scope activation policy is enabled for its type
- **THEN** the service passes the candidate to the separate admission policy as non-authoritative input and records no activation until that policy accepts it

#### Scenario: Provider returns an unsupported insight type

- **WHEN** a provider proposes `hypothesis`, `goal`, `contradiction`, or `causal_link` while that type is not enabled by a separate policy
- **THEN** the service rejects or quarantines the proposal and does not create an active insight

#### Scenario: Provider requests direct mutation

- **WHEN** a provider response asks to overwrite canonical memory, delete evidence, or set an active lifecycle state directly
- **THEN** the service rejects the mutation and retains no canonical side effect

### Requirement: Offline replay and shadow execution are deterministic and non-authoritative

The service SHALL support deterministic offline replay and optional shadow
execution for reasoning requests using checksum-locked inputs, normalized
envelopes, redacted outputs, and explicit policy/provider versions. Replay and
shadow results MUST be diagnostic or candidate-only, MUST NOT change canonical
memory or default retrieval/context behavior, and MUST preserve enough bounded
evidence to compare result equivalence and degradation outcomes.

#### Scenario: Offline replay repeats a request

- **WHEN** the same normalized fixture, scope proof, provider contract version, and policy version are replayed
- **THEN** the runner produces a deterministic categorized outcome and does not invoke a remote provider or mutate canonical records

#### Scenario: Shadow provider disagrees with baseline

- **WHEN** shadow output differs from the approved baseline or lacks required evidence
- **THEN** the service records a bounded disagreement or incomplete result while the baseline behavior remains authoritative

#### Scenario: Replay dependency is stale

- **WHEN** fixture provenance, source watermark, provider version, or policy version is missing or expired
- **THEN** the run is marked stale or incomplete and no readiness or activation claim is emitted

### Requirement: Provider failures have stable fallback and audit semantics

Reasoning execution SHALL classify configuration, compatibility, scope, validation,
budget, timeout, cancellation, rate-limit, provider-unavailable, malformed
output, stale-dependency, and retryable interruption outcomes. Failures MUST be
redacted, bounded, auditable, and idempotent. When policy permits fallback, the
service SHALL preserve the non-reasoning baseline and MUST NOT silently broaden
budgets, retry indefinitely, or activate unverified output.

#### Scenario: Provider times out

- **WHEN** a live or shadow invocation exceeds its deadline
- **THEN** the service records a timeout category with bounded diagnostics, returns the configured fallback or incomplete result, and preserves the original request identity for safe retry

#### Scenario: Provider returns malformed output

- **WHEN** a provider response violates the schema, budget, evidence, or scope contract
- **THEN** the service rejects the output, records a validation category, and leaves baseline retrieval, context, and canonical memory unchanged

#### Scenario: Retry follows an interrupted durable claim

- **WHEN** execution is interrupted after a durable request claim but before a result is recorded
- **THEN** an equivalent retry resumes or returns the original categorized outcome without duplicate candidates, intents, audits, or provider side effects

### Requirement: Reasoning conformance evidence is scoped and safe

The service SHALL provide a bounded conformance profile or run contract for
reasoning providers that checks capability compatibility, envelope limits, exact
scope enforcement, evidence citation completeness, deterministic replay,
fallback, redaction, and candidate-only behavior. Conformance reports MUST be
scoped, append-only, and safe to expose on authorized diagnostic surfaces
without including prompts, chain-of-thought, credentials, raw model payloads,
hidden identifiers, or foreign scope values.

#### Scenario: Conformance run passes

- **WHEN** a supported fixture set completes with compatible capability metadata, valid evidence, deterministic replay, and no isolation violations
- **THEN** the service records a passing diagnostic run with bounded counters, versions, and cited evidence references

#### Scenario: Conformance detects an isolation violation

- **WHEN** a fixture or provider response attempts to access an ungranted scope or hidden record
- **THEN** the run records a scope-safety failure, returns no foreign content, and leaves canonical records unchanged

#### Scenario: Conformance is rerun

- **WHEN** an operator reruns a reasoning conformance profile
- **THEN** the service creates a new linked diagnostic record and preserves prior run history and evidence

### Requirement: Insight derivation uses a bounded provider-neutral envelope

The reasoning provider boundary SHALL define a provider-neutral request and
response for insight derivation with exact scope, redaction and lifecycle
constraints, evidence references, output schema/version, uncertainty bounds,
and a non-authoritative operation mode.

#### Scenario: Provider receives bounded evidence

- **WHEN** the service invokes a provider for offline or shadow derivation
- **THEN** the request contains only authorized evidence references/content, bounded by scope, watermark, redaction, and execution limits

#### Scenario: Provider response is normalized

- **WHEN** a provider returns one or more insight candidates
- **THEN** the adapter normalizes type, claims, citations, uncertainty, provenance, and refusal/errors into the shared envelope without preserving raw provider payloads

### Requirement: Provider fallback is safe for reasoning derivation

The boundary MUST fail closed on unsupported capability, timeout, malformed
output, budget exhaustion, or incompatible schema and MUST report a bounded
fallback category without converting provider failure into an active insight.

#### Scenario: Provider times out

- **WHEN** a provider exceeds its invocation or evidence budget
- **THEN** the service returns a bounded incomplete/fallback result and leaves canonical and active derived state unchanged

#### Scenario: Provider emits unrecognized output

- **WHEN** a provider emits an unsupported type, unbound citation, or unsafe mutation request
- **THEN** the adapter quarantines the result and returns a stable validation category to the caller
