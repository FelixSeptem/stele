## ADDED Requirements

### Requirement: OpenAPI publishes privileged procedural-verification contracts
The authoritative OpenAPI document SHALL describe privileged procedural-memory
verification creation, revalidation trigger/history/detail, and
verification-attributed correction request/response schemas. It MUST specify
administrator authorization, exact-scope requirements, bounded evidence and
payload limits, idempotency/replay behavior, lifecycle-safe not-found/denial
behavior, and stable validation/conflict/unavailable error categories.

#### Scenario: Administrator discovers verification API semantics
- **WHEN** an integration reads the authoritative OpenAPI document
- **THEN** it can determine the privileged verification and correction request
shapes, required attribution, response dispositions, and failure categories
without relying on undocumented implementation behavior

#### Scenario: Public credential probes the privileged contract
- **WHEN** a public runtime credential invokes a documented privileged
verification route
- **THEN** the service denies the request before verification lookup and does
not widen ordinary public Provider or retrieval disclosure
