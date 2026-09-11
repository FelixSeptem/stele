## ADDED Requirements

### Requirement: Candidate identity and lineage deduplication
The service SHALL apply deterministic identity and validated-lineage
deduplication to bounded, lifecycle-visible fused candidates before those
candidates can compete for diversity-aware context selection. Equivalent
candidates SHALL be recognized by canonical memory ID, source event ID, or
validated parent-memory lineage, and one representative SHALL be selected using
the stable fused ordering.

#### Scenario: Equivalent candidates share validated source lineage
- **WHEN** two or more lifecycle-visible fused candidates resolve to the same
  canonical memory, source event, or validated parent-memory lineage
- **THEN** the service retains one deterministic representative and preserves
  bounded authorized evidence citations without returning duplicate evidence
  into context packing

#### Scenario: Candidate lineage cannot be proven
- **WHEN** a candidate lacks a valid canonical, source-event, or parent-memory
  lineage in the resolved scope
- **THEN** the service excludes it before deduplication and diversity selection
  and does not use it to suppress another candidate

### Requirement: Versioned semantic similarity clustering
The service SHALL permit a diversity policy to form bounded semantic-similarity
clusters only from already validated candidates with compatible active embedding
revisions. The policy SHALL record a name, version, bounded threshold, and
selection parameters through the authorized rollout/evaluation path.

#### Scenario: Compatible visible candidates are near-identical
- **WHEN** an active or shadow diversity policy evaluates bounded visible
  candidates with compatible active embedding revisions that meet its semantic
  similarity threshold
- **THEN** the policy can treat them as one diversity cluster while retaining a
  deterministic representative and bounded citations

#### Scenario: Semantic similarity is unavailable
- **WHEN** candidate embeddings, compatible active revisions, or the optional
  similarity computation are unavailable
- **THEN** the service retains deterministic identity/lineage deduplication,
  reports only a bounded authorized availability category, and does not fail,
  widen scope, or query additional candidates

### Requirement: Diversity-aware selection is deterministic and scope-safe
The service SHALL apply maximal marginal relevance or an explicitly versioned
equivalent deterministic policy after identity/lineage deduplication and before
final context packing. The policy SHALL balance relevance with bounded coverage
across memory class, validated source session, known entity attribution, and
time slice without changing candidate visibility rules.

#### Scenario: Independent evidence competes with one fact cluster
- **WHEN** multiple deduplicated visible candidates fit a context section but
  several candidates belong to the same semantic or source-evidence cluster
- **THEN** the selector prioritizes relevant independent evidence according to
  the active policy while preserving deterministic tie breaking

#### Scenario: Diversity policy is disabled for the resolved scope
- **WHEN** no approved diversity rollout applies to the exact resolved scope or
  an active rollout is disabled
- **THEN** the service uses deterministic identity/lineage-deduplicated baseline
  ordering without rewriting stored memory, chunks, provenance, or fusion state

#### Scenario: Candidate is hidden or foreign
- **WHEN** a candidate is suppressed, forgotten, expired, deleted, or outside
  the resolved tenant, project, namespace, session, or user scope
- **THEN** it is excluded before clustering and selection and cannot influence
  the representation, score, or omission of a visible candidate

### Requirement: Diversity diagnostics are bounded and authorized
The service SHALL expose diversity policy identity, aggregate selection
dispositions, and duplicate/diversity/budget omission categories only through
authorized evaluation or admin diagnostics. Ordinary retrieval and context
responses MUST NOT expose cluster membership, similarity values, candidate pools,
hidden content, or foreign identifiers.

#### Scenario: Authorized evaluation observes an omission
- **WHEN** an authorized evaluation or admin diagnostic path inspects a visible
  candidate omitted by identity deduplication, diversity selection, or budget
  accounting
- **THEN** it can receive the active policy identity and a bounded omission
  category without raw similarity scores or hidden evidence details

#### Scenario: Ordinary client assembles context
- **WHEN** a non-diagnostic client invokes ordinary retrieval or context assembly
- **THEN** the service preserves the established public response shape and does
  not disclose internal diversity configuration or candidate dispositions
