## MODIFIED Requirements

### Requirement: Agent-ready context assembly endpoint
The service SHALL expose a context assembly capability that returns structured
sections rather than a flat result list. Sections MAY be populated from an
authorized, lifecycle-visible versioned context projection as well as live
retrieval, while preserving the existing response shape and citation contract.

#### Scenario: Context is assembled for an agent request
- **WHEN** a client requests assembled context for a scoped query or interaction
- **THEN** the service returns structured sections including `profile`,
  `recent_session`, `recent_episodes`, `relevant_summaries`,
  `related_entities`, and `citations`

#### Scenario: Projection is available for the request
- **WHEN** an exact-scope verified projection contains eligible always-visible
  or session items
- **THEN** the assembler can include those items with redacted source citations
  and bounded projection diagnostics without changing section names

### Requirement: Budget-aware context shaping
The service MUST support bounded context packing so the assembled response stays
within a caller-provided or service-default budget. Projection-backed and live
retrieval items MUST use the same deterministic budget accounting and MUST fail
closed when an item cannot fit.

#### Scenario: Context budget is constrained
- **WHEN** a client requests context assembly with a limited budget
- **THEN** the service trims and prioritizes sections according to retrieval or
  projection policy ordering and summary preference instead of returning
  unbounded memory

#### Scenario: Projection item exceeds remaining budget
- **WHEN** a lifecycle-visible projection item cannot fit within the remaining
  character/token budget
- **THEN** the item is omitted with a bounded budget reason and the assembler
  does not increase the requested budget or fetch a broader scope

## ADDED Requirements

### Requirement: Context assembly reports projection-safe diagnostics
The service SHALL expose bounded diagnostics for projection inclusion, staleness,
policy omission, lifecycle exclusion, scope mismatch, and budget omission
without exposing hidden content, raw event payloads, or foreign identifiers.

#### Scenario: Projection item is omitted safely
- **WHEN** a projection item is omitted because of lifecycle, policy, scope, or
  budget validation
- **THEN** diagnostics include only a stable reason category and bounded counts
  for the affected section
