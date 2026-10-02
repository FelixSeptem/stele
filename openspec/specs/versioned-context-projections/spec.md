# versioned-context-projections Specification

## Purpose
Persist durable, versioned context projections with exact scope and deterministic ordering.

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

### Requirement: Maintenance controls projection freshness eligibility

Projection maintenance SHALL persist bounded rebuild/checkpoint state and
source/projection watermark freshness evidence. A projection with missing,
stale, divergent, foreign, or lifecycle-hidden evidence MUST remain excluded
from ordinary retrieval until a successful exact-scope rebuild revalidates it.

#### Scenario: Exact-scope rebuild revalidates a projection

- **WHEN** maintenance rebuilds a projection from PostgreSQL source records with a matching policy and renderer identity
- **THEN** it records a deterministic completion and the projection becomes eligible only after freshness and lifecycle checks pass

#### Scenario: Rebuild encounters hidden evidence

- **WHEN** a rebuild discovers suppressed, forgotten, deleted, or foreign evidence
- **THEN** it records a fail-closed lifecycle or isolation category and leaves the affected projection ineligible

### Requirement: Projection rebuilds use durable derived work

Context projection rebuilds SHALL be dispatched through the unified derived work
contract with exact scope, policy/renderer identity, source watermark, lease-safe
checkpointing, and idempotent duplicate handling.

#### Scenario: Rebuild is requested twice

- **WHEN** the same scope, kind, policy, renderer, and source watermark are submitted twice
- **THEN** one work item and one deterministic projection result are retained

#### Scenario: Rebuild is interrupted

- **WHEN** a projection worker loses its lease during rebuild
- **THEN** a later worker resumes from durable progress and cannot publish a stale or foreign projection

### Requirement: Projection freshness reports queue degradation

A projection SHALL be ineligible for ordinary retrieval when its rebuild work is
dropped in memory mode, flush-failed, exhausted, stale, divergent, or otherwise
missing required source evidence.

#### Scenario: Lossy queue drops rebuild

- **WHEN** an unflushed projection rebuild is lost
- **THEN** the affected projection remains or becomes ineligible and reports a bounded freshness/SLO category

### Requirement: Projection versions remain append-only

Queue retries and rebuilds SHALL create or resolve versioned projection results
without mutating canonical memory or replacing prior projection history.

#### Scenario: Rebuild produces a newer version

- **WHEN** a durable rebuild completes with a newer source watermark
- **THEN** PostgreSQL stores a new projection version and preserves the prior version as history
