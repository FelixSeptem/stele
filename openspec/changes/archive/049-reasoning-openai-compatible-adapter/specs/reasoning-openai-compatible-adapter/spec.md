## Purpose

Provide one optional, operator-controlled OpenAI-compatible HTTP adapter that
executes the existing reasoning boundary without making a remote model an
authority, source of record, or required service dependency.

## ADDED Requirements

### Requirement: OpenAI-compatible reasoning adapter is opt-in and consistently registered

The service SHALL support an optional OpenAI-compatible reasoning adapter with
an operator-configured endpoint, model identifier, credential, timeout, and
bounded transport limits. The adapter MUST be disabled by default. `api`,
`worker`, and `scheduler` MUST derive the same adapter registration and bounded
capability state from the same effective configuration. An enabled `shadow` or
`live` configuration that is missing required adapter settings MUST fail startup
with an actionable configuration error; disabled and offline modes MUST start
without credentials, an endpoint, or a network dependency.

#### Scenario: Operator enables a complete shadow adapter configuration

- **WHEN** an operator configures a valid OpenAI-compatible endpoint, model,
  credential, timeout, and bounded limits in `shadow` mode
- **THEN** every runtime mode reports the same enabled shadow capability and can
  register the adapter without changing retrieval or context authority

#### Scenario: Operator configures an incomplete enabled adapter

- **WHEN** `shadow` or `live` reasoning is enabled but its endpoint, model,
  credential, timeout, or configured bounds are invalid or missing
- **THEN** startup rejects the configuration before any reasoning invocation is
  attempted and does not silently fall back to an unbounded remote request

#### Scenario: Deployment runs without a remote reasoning provider

- **WHEN** reasoning is disabled or configured for offline replay
- **THEN** API, worker, scheduler, retrieval, and context assembly remain
  usable without an adapter, remote endpoint, or credential

### Requirement: Adapter sends only a normalized bounded derivation request

The adapter SHALL translate a validated reasoning invocation envelope into one
bounded OpenAI-compatible structured completion request. It MUST include the
configured model identity and a server-owned response schema, and MUST send
only scope-eligible redacted evidence and normalized operation metadata. The
adapter MUST enforce the request deadline and input/output bounds locally; it
MUST NOT expose credentials, prompts, raw evidence payloads, hidden record
identifiers, or arbitrary provider tools through capability discovery,
diagnostics, ordinary logs, or persisted records.

#### Scenario: Adapter submits a valid bounded derivation

- **WHEN** a validated in-scope invocation is executed through the enabled
  adapter before its deadline
- **THEN** the adapter sends one bounded structured request with the configured
  model identity and returns no provider authority beyond the original envelope

#### Scenario: Request is invalid before transport

- **WHEN** an invocation is expired, over budget, malformed, or references
  evidence outside its exact scope
- **THEN** the adapter rejects it locally with the existing stable category and
  does not contact the configured endpoint

#### Scenario: Operator inspects adapter diagnostics

- **WHEN** an authorized diagnostic surface reports adapter configuration or
  execution status
- **THEN** it includes only bounded mode, provider/model version, limits, and
  categorized outcomes without credentials, prompts, raw request/response
  bodies, or foreign identifiers

### Requirement: Adapter output is validated as candidate-only structured data

The service SHALL accept an OpenAI-compatible response only when it conforms to
the server-owned structured schema and the original exact scope, evidence set,
versions, and output budget. The adapter MUST treat missing content, malformed
JSON, unknown evidence, scope mismatch, unsupported insight type, direct
mutation instruction, provider tool invocation, and response size excess as a
non-authoritative categorized failure. It MUST discard raw provider output after
deriving the validated bounded candidate or diagnostic category.

#### Scenario: Provider returns a valid candidate

- **WHEN** the provider response matches the structured schema and all
  candidate/evidence checks pass
- **THEN** the adapter returns the candidate with provider/model provenance for
  ordinary governance and performs no canonical-memory or lifecycle mutation

#### Scenario: Provider response tries to exceed its authority

- **WHEN** a response includes an unknown citation, a widened scope, a reserved
  insight type, a direct mutation request, or a provider tool request
- **THEN** the adapter rejects or quarantines the response with no canonical,
  lifecycle, grant, configuration, retrieval, or context side effect

#### Scenario: Provider response is malformed or oversized

- **WHEN** the endpoint returns invalid structured data, an unsupported content
  form, or a response exceeding the configured bound
- **THEN** the adapter records a redacted malformed-output category and preserves
  the non-reasoning baseline

### Requirement: Remote failures preserve offline, shadow, and baseline safety

The service SHALL classify OpenAI-compatible authentication, rate-limit,
availability, timeout, cancellation, compatibility, and malformed-response
failures through the existing bounded reasoning error categories. The adapter
MUST honor cancellation, issue no automatic unbounded retry, and retain the
original idempotency identity for a caller-managed safe retry. Offline replay
MUST never invoke the adapter. Shadow output, including a successful candidate,
MUST remain non-authoritative and MUST NOT alter default retrieval, context,
canonical memory, or automatic activation of reserved insight types.

#### Scenario: Remote endpoint times out or is unavailable

- **WHEN** an adapter request exceeds its deadline or the endpoint cannot
  provide a valid response
- **THEN** the service returns the configured non-reasoning fallback or
  incomplete result with a stable redacted category and no widened retry budget

#### Scenario: Shadow adapter returns a valid but different candidate

- **WHEN** a shadow invocation returns a candidate that differs from the
  approved baseline
- **THEN** the service records only bounded disagreement diagnostics or a
  candidate-only result while baseline retrieval and context remain authoritative

#### Scenario: Offline fixture is replayed while adapter credentials exist

- **WHEN** an operator reruns a checksum-locked offline fixture
- **THEN** replay remains deterministic and makes no HTTP request to the
  configured adapter endpoint

### Requirement: Adapter conformance is network-contained and secret-safe

The service SHALL provide deterministic local HTTP conformance coverage for the
OpenAI-compatible request shape, response validation, deadline/cancellation,
error categorization, redaction, exact-scope isolation, and baseline fallback.
Conformance evidence MUST identify the adapter contract/provider/model version
and bounded counters without storing credentials, prompts, raw payloads,
chain-of-thought, or hidden/foreign record identifiers. A live endpoint MUST
NOT be required for ordinary tests, startup, or release verification.

#### Scenario: Local adapter conformance succeeds

- **WHEN** a local controlled endpoint returns a valid bounded structured
  candidate for a compatible request
- **THEN** conformance records a compatible result with bounded evidence and no
  secret or raw-payload retention

#### Scenario: Local adapter conformance detects redaction or isolation failure

- **WHEN** a controlled endpoint observes an unexpected secret/foreign value or
  returns a scope-violating response
- **THEN** conformance fails closed, records only safe counters/categories, and
  makes no canonical-memory mutation
