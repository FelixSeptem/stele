## Purpose

Define safe runtime registration for optional reranker services so self-hosted deployments can connect to an operator-supplied model without coupling credentials or provider-specific details to source control.

## ADDED Requirements

### Requirement: Provider registration is consistent across runtime modes
The service SHALL construct the configured reranker resolver once from effective runtime configuration and expose the same logical provider view to `api`, `worker`, and `scheduler` modes.

#### Scenario: Provider is configured
- **WHEN** a valid logical provider, endpoint, model, and bounded timeout are configured
- **THEN** each runtime mode can resolve the same provider identity and startup reports the provider as available for its permitted operations

#### Scenario: Provider configuration is incomplete
- **WHEN** a declared provider lacks a required endpoint, model, or valid bound
- **THEN** startup fails with an actionable configuration error before active reranking can execute

### Requirement: OpenAI-compatible adapter uses bounded, secret-safe requests
The service SHALL support an optional OpenAI-compatible HTTP adapter that sends only bounded query/candidate text and receives a validated ranking response without logging authorization headers or raw payloads.

#### Scenario: Adapter request succeeds
- **WHEN** the provider returns a valid ranking for the requested candidate set within the configured timeout
- **THEN** the adapter returns normalized candidate scores/order tagged with provider and model identities

#### Scenario: Adapter request fails
- **WHEN** the endpoint is unreachable, unauthorized, throttled, malformed, or times out
- **THEN** the adapter returns a categorized error suitable for fail-closed fallback and excludes response bodies and secrets from ordinary logs

### Requirement: Provider output cannot widen retrieval scope
The service MUST validate every provider result against the original visible candidate set and resolved scope before applying it.

#### Scenario: Provider returns an unknown candidate ID
- **WHEN** a response references an ID not present in the validated candidate set
- **THEN** the service ignores or rejects that item and keeps the baseline candidate set unchanged

#### Scenario: Provider returns duplicate or out-of-order items
- **WHEN** a response contains duplicates or omits candidates
- **THEN** the service applies deterministic completion rules and never creates new candidates or widens scope
