## ADDED Requirements

### Requirement: Real-stack MCP conformance is reproducible and bounded

The project SHALL provide an opt-in conformance run that exercises the enabled
MCP adapter through its actual Streamable HTTP transport against an owned
PostgreSQL database with pgvector enabled. The run MUST use governed,
scope-isolated fixtures, bounded timeouts and result sizes, exact cleanup, and
redacted evidence categories; it MUST skip clearly when the required test DSN
is not supplied rather than silently claiming a pass.

#### Scenario: Real-stack prerequisites are absent

- **WHEN** the conformance command runs without the documented PostgreSQL test
  DSN
- **THEN** it reports an explicit skipped result, does not claim conformance,
  and does not mutate a database selected by an implicit default

#### Scenario: Real-stack prerequisites are available

- **WHEN** the conformance command receives a dedicated PostgreSQL test DSN
  whose migrations and pgvector extension can be verified
- **THEN** it runs the MCP protocol client against the actual Streamable HTTP
  adapter, uses unique exact scopes for fixtures, and removes only records it
  created before returning

### Requirement: MCP conformance evidence covers the complete governed contract

The real-stack run SHALL exercise and report bounded pass/fail categories for
capability discovery, identity, search, context, browse, remember, forget
preview, forget apply, exact scope isolation, explicit-scope precedence,
read-only grants, temporal and lifecycle visibility, exact path and path-prefix
selection, mutation idempotency, restart/replay/conflict behavior, and response
redaction. Evidence MUST contain stable category names and counts or references
needed for release review without exposing credentials, raw SQL, hidden memory
content, or unbounded protocol payloads.

#### Scenario: Authorized scope passes the matrix

- **WHEN** an enabled adapter is exercised with an authorized exact scope and
  governed fixtures
- **THEN** the report records bounded outcomes for each requested tool and
  policy category, including the visible result and the absence of hidden or
  foreign results

#### Scenario: Foreign or hidden data is probed

- **WHEN** a conformance case requests an ungranted scope, suppressed or
  forgotten memory, or a path outside the selected grant
- **THEN** the adapter returns the existing bounded denial or empty result,
  canonical fixture state remains unchanged, and the report records only the
  isolation or lifecycle category and outcome

#### Scenario: Forget is preview-bound and replay-safe over MCP

- **WHEN** a caller previews semantic forgetting, applies the reviewed fixed
  IDs, retries the same idempotency key after repository recreation, or reuses
  the key with a conflicting payload
- **THEN** the report verifies no preview mutation, exact reviewed-ID
  application, durable replay of the original outcome, and a bounded conflict
  for the divergent request

### Requirement: MCP disablement and OpenAPI non-regression are evidenced

The conformance workflow SHALL verify that disabling the MCP adapter fails
closed at the configured endpoint while ordinary OpenAPI behavior, PostgreSQL
state, and provider/runtime behavior remain available and unchanged. The
workflow MUST keep MCP disabled by default outside the explicitly enabled test
process.

#### Scenario: Adapter is disabled during verification

- **WHEN** the API is started or configured with MCP disabled and the
  conformance probe requests the MCP endpoint
- **THEN** the endpoint is not advertised or accepted, the probe records a
  bounded disabled result, and ordinary OpenAPI routes remain usable

#### Scenario: Adapter is re-enabled for the matrix

- **WHEN** the test process explicitly enables MCP with bounded limits and a
  valid credential
- **THEN** only that process exposes the configured endpoint, and the resulting
  evidence identifies the enabled configuration without exposing the secret or
  internal scope grants
