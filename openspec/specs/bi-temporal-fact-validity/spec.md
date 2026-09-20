# bi-temporal-fact-validity Specification

## Purpose
Define deterministic, auditable fact-validity semantics that separate when a
memory version was recorded from the interval in which its factual content was true.

## Requirements

### Requirement: Fact versions expose separate recorded and valid time
The service SHALL represent factual canonical versions with recorded/ingested
time and a valid-time interval independently. Validity SHALL use the half-open
interval `[valid_from, valid_to)`, where a null `valid_to` means open-ended.

#### Scenario: Version has known validity interval
- **WHEN** a factual canonical version is created with valid-time bounds
- **THEN** the stored version preserves recorded time, `valid_from`, optional
  `valid_to`, and stable temporal identity as distinct auditable fields

#### Scenario: Invalid interval is submitted
- **WHEN** `valid_to` is present and is earlier than or equal to `valid_from`
- **THEN** the service rejects the write without creating a canonical version

### Requirement: Temporal corrections are append-only and deterministic
The service MUST preserve a stable temporal fact identity and append a new
version or correction record for every validity correction. It MUST NOT erase or
mutate a prior canonical version or silently create ambiguous overlapping
current intervals for mutually exclusive facts.

#### Scenario: Fact is corrected retroactively
- **WHEN** an authorized correction changes the validity interval or content of
  an existing factual memory
- **THEN** the service appends a successor with correction provenance and keeps
  the prior interval and recorded history available to privileged audit

#### Scenario: Competing current intervals are submitted
- **WHEN** a correction would create overlapping mutually exclusive intervals
  for one temporal fact identity without an explicit conflict disposition
- **THEN** the service rejects it or records a deterministic conflict state and
  excludes the conflict from ordinary current retrieval

### Requirement: Legacy rows remain current-compatible
The service SHALL provide a deterministic compatibility interpretation for
pre-validity rows that preserves existing current retrieval behavior while
marking validity provenance as migrated or inferred. Migration MUST be additive
and replayable within exact scope.

#### Scenario: Legacy active memory is read after migration
- **WHEN** a canonical memory has no explicit valid-time fields from before the
  temporal migration
- **THEN** current retrieval treats it as valid from its recorded creation time
  through open-ended time and reports the compatibility rule in authorized audit

### Requirement: Current retrieval selects valid visible facts
Default search and context assembly MUST require lifecycle visibility and fact
validity at the request's current time. A stale or expired version MUST NOT win
solely because lexical, semantic, relation, or chunk similarity is higher.

#### Scenario: Current fact supersedes stale fact
- **WHEN** current retrieval finds an expired version and a lifecycle-visible
  successor valid now
- **THEN** the successor is eligible and the expired version is excluded from
  ordinary results and context

#### Scenario: Similarity favors an expired fact
- **WHEN** an expired version has the highest lexical or semantic similarity
- **THEN** validity filtering removes it before fusion or final ranking

### Requirement: Historical retrieval requires explicit temporal authorization
Historical `as_of` or interval retrieval MUST be requested through an explicit
temporal plan carrying the authorized scope and deterministic time constraint.
Without that plan, ordinary callers receive current-valid results only.

#### Scenario: Authorized as-of query
- **WHEN** an authorized temporal plan requests a historical instant
- **THEN** retrieval selects versions whose validity interval contains that
  instant and preserves the selected version identity in citations

#### Scenario: Historical selector is missing its plan
- **WHEN** a caller supplies a historical time selector without an explicit
  temporal plan or with an invalid interval
- **THEN** the service rejects the historical constraint or falls back to the
  approved current baseline without exposing history

### Requirement: Temporal identity propagates to derived evidence
Relations, chunks, embeddings, citations, context projections, rebuilds, and
evaluation fixtures MUST retain immutable source-version and validity identity.
Derived artifacts MUST be rebuildable and MUST NOT become a second source of
truth.

#### Scenario: Derived evidence is rebuilt after correction
- **WHEN** a canonical successor changes the valid-time identity of a source
- **THEN** the service creates a new derived version linked to that successor
  while preserving prior derived history for audit

### Requirement: Temporal operations preserve scope and lifecycle safety
Every temporal write, read, correction, migration, rebuild, and evaluation MUST
enforce exact tenant, project, and namespace boundaries and exclude suppressed,
forgotten, deleted, or otherwise hidden versions from ordinary results.

#### Scenario: Historical query crosses a scope boundary
- **WHEN** a temporal selector or derived source would resolve outside the
  caller's exact scope
- **THEN** the service rejects or omits it without disclosing foreign content,
  identifiers, or interval details

### Requirement: Temporal diagnostics are bounded and auditable
Authorized diagnostics SHALL report only logical temporal identities, validity
dispositions, migration/correction categories, bounded counts, and policy
versions. They MUST exclude raw content, query text, hidden identifiers,
foreign scope values, credentials, and provider payloads.

#### Scenario: Temporal evaluation records stale exclusion
- **WHEN** an evaluator excludes an expired or conflicting version
- **THEN** the report records a stable aggregate disposition and policy identity
  without serializing hidden evidence
