## ADDED Requirements

### Requirement: Provider context conformance validates public JSON and result shape

The Provider contract suite SHALL validate canonical context request examples and actual serialized public-handler responses against the authoritative OpenAPI contract. It MUST distinguish a valid nonempty categorized result, a genuine empty result, incompatible/malformed results, unsupported input/schema, and scope/lifecycle denial. Missing required result sections, null/scalar sections, invalid item/citation types, or a `references/content` substitute MUST produce a failed-contract outcome rather than an empty success. Findings and artifact provenance MUST remain bounded and redacted, and context contract outcomes MUST remain separate from retrieval ranking metrics.

#### Scenario: Public route accepts documented options

- **WHEN** the suite submits canonical snake-case path, relation, insight, goal, diagnostic, and feedback-ranking options through the authenticated Provider HTTP route
- **THEN** it verifies their intended existing behavior and the serialized response schema rather than constructing a passing request from internal Go field names

#### Scenario: Valid nonempty result is consumed

- **WHEN** the suite validates a typed result containing authorized selected memory items and citations
- **THEN** it records a context-contract pass with bounded attribution and does not locally rerank or repack memory

#### Scenario: Genuine empty result is consumed

- **WHEN** the response has an empty array for every required categorized section and citation collection
- **THEN** the suite records a valid empty context outcome distinct from an incompatible or malformed response

#### Scenario: Required result shape is missing

- **WHEN** a response lacks `result` or a required section, uses null/scalar sections, has invalid item/citation types, or substitutes `references/content`
- **THEN** the suite records a failed-contract outcome and never treats zero decoded items as proof of a successful empty projection

#### Scenario: Unsupported field or version is refused

- **WHEN** a fixture submits an unsupported policy field, undocumented case alias, invalid request value, or unsupported Provider schema
- **THEN** the suite checks the documented bounded validation/compatibility outcome and records failure if the service silently accepts or rewrites the request

#### Scenario: Foreign or hidden evidence is requested

- **WHEN** a fixture uses a foreign binding/session/scope or targets suppressed, forgotten, deleted, or out-of-scope evidence
- **THEN** it verifies denial or lifecycle-safe omission with no leaked content/identifiers and records any violation as a contract safety failure

#### Scenario: Raw ranking data leaks in an optional section

- **WHEN** serialized context includes raw scores, internal plans, feedback history, hidden identifiers, or unbounded payloads in any section
- **THEN** the suite records a redaction contract failure regardless of whether the primary memory sections passed

### Requirement: Public context integration evidence is fresh and exact scoped

PC1 public integration evidence SHALL use a pinned repaired service revision and a fresh owned PostgreSQL/pgvector fixture, official principal/runtime bootstrap, and ordinary governed public ingestion. The verifier MUST establish existing governance completion, check a non-root exact path and explicit prefix through Provider context, validate nonempty and empty responses, and repeat eligible reads after API/worker restart. Missing, stale, skipped, or degraded dependencies/evidence MUST prevent a passing integration conclusion. Evidence MUST identify the service/OpenAPI/Provider schema provenance using bounded redacted fields and MUST NOT retain credentials, authorization headers, raw payloads, or unbounded content. This contract MUST NOT be interpreted as implementing PC3 budget redesign or PC5's expanded durable reference API.

#### Scenario: Fresh context fixture passes

- **WHEN** public fixture ingestion completes existing governance and canonical evidence is read through documented Provider context JSON in the same exact scope before and after restart
- **THEN** the verifier records a passing context integration result with non-root path selection, typed responses, citations, and bounded version provenance

#### Scenario: Write acceptance lacks governance completion

- **WHEN** public ingestion returns success but required governed evidence is missing or incomplete within the verifier's bounded observation window
- **THEN** the verifier records incomplete/degraded integration evidence and does not claim readiness from write acceptance or a transient retrieval result

#### Scenario: Fixture dependencies or shape are missing

- **WHEN** official runtime bootstrap, fresh PostgreSQL/pgvector, required restart evidence, or valid typed response evidence is unavailable
- **THEN** the verifier preserves a nonpassing integration outcome rather than substituting an in-memory or direct-database fixture
