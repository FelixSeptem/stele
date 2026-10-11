## ADDED Requirements

### Requirement: Disclosure conformance checks serialized mode and alternate-route behavior

Provider contract conformance SHALL validate canonical disclosure inputs and actual serialized public responses against the mode-specific OpenAPI schema. It MUST distinguish authorized nonempty/empty references, authorized bounded content, missing/incompatible result shapes, unsupported labels/modes, profile denial, and dependency failure. It MUST test reference-mode text absence and independently authorized citations across optional sections, alternate native/Provider/MCP/sync routes, write/replay echoes, grant changes using existing bindings, and foreign/hidden/high/restricted sources. Outcomes MUST remain bounded/redacted and separate from ranking quality metrics.

#### Scenario: Text exists in every optional source section

- **WHEN** reference-only fixtures contain marked content, title, summary, lesson, goal, metadata, and diagnostic text
- **THEN** the serialized response passes the closed reference schema and exposes none of the prohibited fields/values

#### Scenario: Consumer assumes reference response contains content

- **WHEN** a content consumer receives reference items or a response lacks required mode/disclosure/category metadata
- **THEN** conformance records an incompatible/failed-contract outcome rather than a successful empty content projection

#### Scenario: Restricted credential tries alternate reads or replay echoes

- **WHEN** fixtures request retrieve/sync/native/MCP content or replay a content-bearing write using a constrained credential
- **THEN** conformance verifies equivalent permitted disclosure or a bounded refusal/safe acknowledgment and records any content or identifier leak as a safety failure

### Requirement: Public disclosure and trust evidence proves durable governance and restart

PC2 live integration evidence SHALL use a pinned service revision, fresh owned PostgreSQL/pgvector, official bootstrap and admin grant provisioning, public labeled ingestion, bounded governance completion observation, and exact/prefix Provider context in the same scope. It MUST verify stored source labels and non-root path at candidate/canonical/version boundaries, replay/conflict behavior, content/reference modes, grant downgrade/revocation, and lifecycle/foreign-source refusal before and after API/worker restart. Required unsupported, skipped, stale, or degraded evidence MUST prevent a passing conclusion. Experimental derivation tests MUST be identified separately when only focused domain/repository integration evidence is available, without claiming public live coverage.

#### Scenario: Fresh profiled runtime passes after restart

- **WHEN** public ingestion completes governance and authorized content/reference reads, replay, grant changes, and safety checks pass before and after restart
- **THEN** evidence records a passing disclosure result with bounded service/image/schema/policy/source-metadata provenance and no stored credentials or raw content

#### Scenario: HTTP succeeds but durable labels are missing

- **WHEN** ingestion/context succeeds but governance completion or required candidate/canonical/version label evidence is missing
- **THEN** the verifier records a nonpassing durable integration result and does not claim PC2 completion

#### Scenario: Mixed-source derivation loses restrictive labels

- **WHEN** a summary, insight, chunk, or projection combines differently labeled sources
- **THEN** focused integration evidence verifies the restrictive trust/classification join and records a contract failure if derivation upgrades trust or weakens classification

### Requirement: Disclosure rollout and maintenance documentation are part of acceptance

PC2 acceptance MUST include matching OpenAPI, README, Provider and admin/bootstrap/self-host integration, source-lineage/governance, best-practice, MCP, and agent-skill documentation for changed semantics. Documentation MUST describe capability negotiation, explicit provisioning, reference-only defaults, mode-specific response consumption, classification/trust defaults, unsupported endpoint denials, external business permission ownership, and safe rollback. Historical PC1 success MUST NOT be treated as PC2 readiness or completion of PC3–PC6.

#### Scenario: Migration consumer reads capabilities

- **WHEN** a consumer prepares to use the revised disclosure contract
- **THEN** maintained examples require `context-disclosure-v1` discovery and valid grants, explain reference/content response validation, and forbid silent retry or fallback to broader disclosure

#### Scenario: Operator rolls back to a binary without enforcement

- **WHEN** an operator follows the documented rollback path
- **THEN** affected constrained credentials are denied or disabled across all public content surfaces before reverting, and durable labels/grants/history are retained
