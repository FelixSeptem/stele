# versioned-context-projections Specification

## Purpose
TBD - created by archiving change versioned-context-projections-and-bounded-assembly. Update Purpose after archive.
## Requirements
### Requirement: Context projections are durable and versioned
The service SHALL persist scoped context projections for the kinds
`always_visible`, `session`, `retrieval`, and `archival_history`. Every
projection version SHALL carry a schema/policy version, exact tenant/project/
namespace scope, source watermark, status, and deterministic item order.

#### Scenario: Projection is materialized for a scope
- **WHEN** an authorized materializer creates a projection for a valid scope
- **THEN** PostgreSQL stores a versioned projection and bounded items in that
  exact scope with a source watermark and policy identity

#### Scenario: Projection is rebuilt
- **WHEN** the same source records are rebuilt with the same policy and renderer
  versions
- **THEN** the service produces deterministic item order/content identity and
  preserves the prior projection version as append-only history

### Requirement: Projection items retain authorized source evidence
Every projection item MUST reference an authorized canonical-memory version or
raw-event evidence record, including source kind, source id/version, observed
lifecycle state, and redacted citation metadata. A projection item MUST NOT
become canonical memory or contain an unbounded raw event payload.

#### Scenario: Canonical version backs a projection item
- **WHEN** a visible canonical-memory version is selected by policy
- **THEN** the item records that exact version and its scoped provenance
  reference without copying mutable canonical state into a new source of truth

#### Scenario: Source evidence is missing or out of scope
- **WHEN** a source reference cannot be resolved in the projection scope or its
  lifecycle state is not authorized
- **THEN** materialization rejects or omits the item with a bounded reason and
  does not widen the query scope

### Requirement: Projection policy is class-aware and lifecycle-safe
The service SHALL apply one versioned policy resolver to projection eligibility.
Profile material MAY enter `always_visible` only when confidence and size gates
pass; summaries MAY enter bounded session context; episodic, procedural, and
relation material SHALL remain on-demand; raw history SHALL remain evidence.
Suppressed, forgotten, expired, and deleted sources MUST be excluded from
ordinary projections.

#### Scenario: Eligible profile enters always-visible context
- **WHEN** an active profile memory satisfies configured confidence and size
  limits for a scope
- **THEN** it can be projected into `always_visible` with source version and
  citation metadata

#### Scenario: Hidden or on-demand class is considered
- **WHEN** a suppressed memory or an episodic, procedural, or relation item is
  considered for always-visible projection
- **THEN** the item is omitted and the policy records a bounded lifecycle or
  class reason without exposing hidden content

### Requirement: Projection reads enforce exact scope and rebuildability
Projection reads and rebuilds SHALL require an exact tenant/project/namespace
scope and SHALL be reproducible from PostgreSQL source records, policy version,
and renderer version. A missing, stale, or divergent source watermark MUST fail
closed for the affected item or projection.

#### Scenario: Authorized projection is read
- **WHEN** a caller requests a projection for the exact scope and compatible
  session/kind
- **THEN** the service returns only lifecycle-visible items from that scope in
  deterministic order

#### Scenario: Foreign or stale projection is requested
- **WHEN** a request's scope differs from the projection scope or source
  validation cannot prove compatibility
- **THEN** the service returns no projection items and a bounded diagnostic
  rather than foreign or stale content

