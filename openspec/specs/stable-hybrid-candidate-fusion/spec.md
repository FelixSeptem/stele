# stable-hybrid-candidate-fusion Specification

## Purpose
TBD - created by archiving change stable-hybrid-candidate-fusion. Update Purpose after archive.

## Requirements

### Requirement: Fusion strategies are versioned and explicit
The service SHALL identify every candidate-fusion execution by a strategy name,
strategy version, and complete bounded parameters, including the rank constant,
channel set, and channel weights when applicable.

#### Scenario: Valid RRF strategy is selected
- **WHEN** an authorized evaluation or scoped rollout selects an RRF strategy
  with a supported version, positive bounded rank constant, and valid channel
  weights
- **THEN** the service accepts the strategy and records its complete identity
  for replay, comparison, and rollback

#### Scenario: Incomplete strategy definition is supplied
- **WHEN** a strategy omits its version, uses an unsupported name, provides an
  invalid rank constant, or contains unbounded/negative channel parameters
- **THEN** the service rejects the selection before querying or merging candidates

### Requirement: RRF is the default cross-channel fusion strategy
The service SHALL merge bounded lexical, semantic, relation, and authorized
chunk-derived candidate lists using deterministic Reciprocal Rank Fusion for the
default approved strategy. For each canonical candidate, the fused score SHALL
be the sum of the explicitly configured channel weight divided by the configured
rank constant plus that channel's one-based rank.

#### Scenario: Candidate appears in multiple channels
- **WHEN** a lifecycle-visible canonical candidate is returned by lexical and
  semantic channels at different ranks
- **THEN** the service emits one canonical candidate with a fused RRF score that
  includes both channel contributions

#### Scenario: Optional channel is empty
- **WHEN** relation or chunk retrieval returns no candidates for an otherwise
  valid query
- **THEN** the service fuses the available channels without changing the RRF
  formula or failing the overall retrieval request

### Requirement: Candidate pools are bounded before fusion
The service MUST enforce per-channel and total candidate bounds before fusion and
MUST apply query scope, memory-class, lifecycle, and source-lineage validation
before a candidate contributes to a fused score.

#### Scenario: Channel exceeds its configured bound
- **WHEN** a recall channel returns more candidates than its configured pool
  limit
- **THEN** the service truncates that channel deterministically before fusion
  and records only a bounded omission category for authorized diagnostics

#### Scenario: Candidate fails visibility validation
- **WHEN** a candidate is foreign-scope, hidden-lifecycle, invalid, or has
  unproven canonical parent/source lineage
- **THEN** the service omits it before fusion and does not allow it to affect
  another candidate's score or rank

### Requirement: Fusion preserves canonical identity and citations
The service SHALL represent overlapping physical candidates by one canonical
memory identity in public results while preserving bounded source and chunk
citations for authorized evaluation or context assembly paths.

#### Scenario: Chunk and canonical paths identify the same parent
- **WHEN** a visible chunk candidate and its canonical parent are returned for
  the same query
- **THEN** the public result contains one canonical memory hit, and authorized
  evidence retains the validated parent/source citations

#### Scenario: Candidate has excessive citation metadata
- **WHEN** a candidate supplies citations beyond the configured evidence bound
- **THEN** the service truncates or omits the excess metadata without exposing
  raw hidden content or changing scope resolution

### Requirement: Final ordering is deterministic
The service SHALL order fused candidates by fused score descending, then explicit
memory-class policy priority, source timestamp descending, and stable canonical
memory ID, and SHALL produce the same order for the same visible inputs and
strategy version.

#### Scenario: Fused scores tie
- **WHEN** two visible candidates have equal fused scores
- **THEN** the service applies the documented class, timestamp, and stable-ID
  tie-break sequence rather than depending on database or goroutine ordering

#### Scenario: Replay repeats the same inputs
- **WHEN** evaluation replay executes the same fixture, visible candidates, and
  fusion strategy version more than once
- **THEN** it produces identical fused ordering and bounded score diagnostics

### Requirement: Fusion rollout is reversible and scope-aware
The service SHALL reuse the existing scoped ranking rollout controls for fusion
diagnostics, dry runs, activation, disablement, and rollback. Fusion selection
MUST never widen the resolved tenant, project, namespace, session, or user scope.

#### Scenario: Diagnostics-only fusion evaluation
- **WHEN** a fusion strategy is selected in diagnostics-only mode
- **THEN** the service evaluates and reports its bounded impact while ordinary
  public retrieval remains on the currently approved baseline strategy

#### Scenario: Approved scoped fusion is activated
- **WHEN** the existing activation gate is satisfied for an exact scope and the
  fusion strategy is explicitly active for that scope
- **THEN** retrieval uses the selected strategy only for that scope and records
  the strategy version in the authorized rollout evidence

#### Scenario: Fusion policy is rolled back
- **WHEN** an operator disables or rolls back an active fusion policy
- **THEN** subsequent retrieval returns to the prior approved baseline without
  rewriting canonical memory, chunks, or provenance records

### Requirement: Optional channel failures degrade safely
The service SHALL treat unavailable optional relation or chunk channels as bounded
diagnostic statuses and SHALL continue with available validated channels. Strategy,
scope, lifecycle, and lineage invariant violations MUST fail closed.

#### Scenario: Relation provider is unavailable
- **WHEN** relation search fails while lexical and semantic retrieval remain
  available
- **THEN** the service returns a result fused from the available channels and
  records a redacted optional-channel-unavailable category for authorized paths

#### Scenario: Fusion invariant is violated
- **WHEN** the strategy or candidate input violates scope, lifecycle, lineage,
  or parameter validation
- **THEN** the service rejects or omits the invalid input and never falls back to
  hidden, foreign, or unbounded evidence

### Requirement: Approved plans can parameterize bounded fusion
The service SHALL allow a compatible approved retrieval plan to select a subset of existing fusion channels, per-channel candidate limits, an aggregate limit, and an explicit supported fusion strategy with query-family parameters. Planned parameters MUST remain complete, versioned, deterministic, and within fusion and request hard limits.

#### Scenario: Query family selects a channel subset
- **WHEN** an active compatible plan enables lexical and semantic channels but omits optional relation and chunk channels
- **THEN** fusion uses only declared validated candidate lists and does not query or score an omitted channel
