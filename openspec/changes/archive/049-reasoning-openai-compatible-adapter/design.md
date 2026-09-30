## Context

The archived `reasoning-provider-boundary` package already validates exact
scope, budgets, evidence citations, candidate output, replay identity, and
stable failure categories. Runtime configuration and capability discovery are
shared by `api`, `worker`, and `scheduler`; the existing embedding and reranker
adapters provide local patterns for bounded HTTP clients and operator-supplied
OpenAI-compatible endpoints. See `proposal.md` for motivation and the delta
spec for externally observable requirements.

## Goals / Non-Goals

**Goals:**

- Add one provider implementation behind the existing `reasoning.Provider`
  interface without changing the provider-independent envelope.
- Make the transport deterministic enough for local conformance tests while
  preserving caller deadlines and bounded request/response sizes.
- Keep registration, capability reporting, and failure fallback identical in
  all runtime modes.
- Keep raw model material transient and make diagnostics safe by construction.

**Non-Goals:**

- No vendor SDK, streaming protocol, tool/function calling, multi-provider
  routing, or automatic retries.
- No operator-authored prompt templates or chain-of-thought capture. The
  adapter owns one fixed, versioned instruction and sends normalized evidence
  only.
- No durable candidate store or autonomous derivation scheduler in this
  change; callers continue to submit results to existing governance paths.

## Decisions

### Use a standard-library OpenAI-compatible HTTP transport

Implement the adapter with `net/http`, `encoding/json`, and an injected
`*http.Client`. The endpoint is an exact operator-configured URL rather than a
base URL plus hidden path convention, which supports local gateways while
avoiding URL concatenation surprises. Requests use bearer authentication only
when the adapter is enabled and never include the credential in error text or
diagnostics.

Alternative: add a vendor SDK. Rejected because it would introduce a provider
dependency, obscure request bounds, and make local fixture testing harder.

### Require deterministic structured output

The adapter sends a fixed server-owned instruction and normalized invocation
envelope as the user content, with temperature-like nondeterminism disabled
where the endpoint supports that field. The response must contain one bounded
JSON candidate envelope; adapter parsing then calls the existing candidate and
scope validators. Unknown fields may be ignored only inside a bounded response
wrapper; unknown evidence, scope, mutation, reserved insight, or tool fields
fail closed.

Alternative: accept free-form text and parse heuristically. Rejected because
heuristic parsing makes evidence completeness, replay equivalence, and safe
fallback unverifiable.

### Bound transport before and after the network call

Validate the invocation before creating an HTTP request. Serialize into a
bounded buffer, cap the response reader below the configured output limit plus
small protocol overhead, and combine the caller deadline with the configured
HTTP timeout. A cancelled context stops transport immediately. The adapter
performs no automatic retry; the existing idempotency metadata remains
unchanged for caller-managed retry.

Alternative: retry 429/5xx in the adapter. Rejected because retry budgets
belong to the reasoning executor and silent adapter retries could violate the
provider boundary's deadline and concurrency accounting.

### Map HTTP and decode failures to existing categories

Use low-cardinality mappings: 401/403 to configuration/compatibility,
429 to rate-limit, 408/504 and context deadline to timeout, cancellation to
cancellation, 5xx/network failures to provider-unavailable, and invalid
structured content to malformed-output. Preserve only bounded status/code
diagnostics; never include response bodies, authorization headers, or request
content in ordinary logs.

Alternative: expose provider status text directly. Rejected because gateways
often echo prompts, identifiers, or credentials in error bodies.

### Register one adapter view for all runtime modes

Extend reasoning configuration with endpoint/model/credential and build the
adapter once per runtime dependency graph. `buildReasoningCapability` receives
the registered provider so live/shadow capability state cannot diverge by mode.
Offline mode deliberately skips construction. If enabled remote mode lacks a
complete adapter, startup fails; if reasoning is disabled, no credential or
endpoint is required.

Alternative: let each runtime parse environment variables independently.
Rejected because mode-specific drift was the problem solved by the previous
boundary change.

### Keep conformance network-contained

Use `httptest.Server` fixtures to assert request shape, bounded body size,
authorization behavior, response validation, deadlines, cancellation, and
redaction. Tests must never require an internet endpoint. A fixture handler
records only safe counters and can intentionally return foreign IDs,
malformed JSON, oversized content, or categorized HTTP errors.

## Risks / Trade-offs

- [Risk] OpenAI-compatible gateways differ in structured-output support. ->
  Mitigation: require the narrow response contract, report compatibility
  failure explicitly, and retain offline/shadow fallback; do not add heuristic
  parsing.
- [Risk] Fixed server-owned instructions may be less useful for future model
  families. -> Mitigation: version the instruction/schema and keep the adapter
  behind the provider-neutral interface so a later adapter can define another
  contract.
- [Risk] Remote endpoint latency can consume operator resources. -> Mitigation:
  enforce caller deadlines, configured timeout, byte/concurrency limits, and no
  adapter retries; default remains disabled.
- [Risk] Debugging provider failures becomes harder after redaction. ->
  Mitigation: expose bounded status category, provider/model version, request
  fingerprint, and retryability without retaining payloads.

## Migration Plan

1. Add disabled-by-default adapter configuration and shared validation.
2. Register the adapter in all runtime builders without changing baseline
   retrieval, context, or worker startup when disabled.
3. Add local HTTP conformance fixtures and expose only bounded capability and
   diagnostic state.
4. Operators can enable `shadow` first, compare bounded disagreement evidence,
   then explicitly enable `live` for a governed caller if desired.
5. Rollback is an environment/configuration change to disabled or offline; no
   database migration or schema downgrade is required, and baseline behavior
   remains authoritative.

## Open Questions

None. Endpoint shape, structured response contract, registration semantics, and
rollback behavior are specified sufficiently for task planning.
