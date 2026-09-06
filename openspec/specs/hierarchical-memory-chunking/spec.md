# hierarchical-memory-chunking Specification

## Purpose
TBD - created by archiving change hierarchical-memory-representation-bounded-chunking. Update Purpose after archive.
## Requirements
### Requirement: Derived chunks are versioned PostgreSQL records
The service SHALL persist source chunks only as derived PostgreSQL records. Each
chunk SHALL identify its exact tenant, project, and namespace; source kind, source
identifier, and immutable source version; chunk ordinal; policy and renderer
versions; bounded content; source range; character and token counts; lifecycle
snapshot; and creation timestamp. A chunk MUST NOT replace raw events, canonical
memory, canonical versions, or provenance as a system of record.

#### Scenario: Chunk is materialized from an authorized source version
- **WHEN** the materializer receives a lifecycle-visible raw-event or canonical-memory
  version in an exact authorized scope
- **THEN** it persists derived chunks with source identity and immutable lineage in
  that same scope

#### Scenario: Same source version is materialized again
- **WHEN** the materializer retries with the same source identity, policy version,
  renderer version, and output boundaries
- **THEN** it converges idempotently without overwriting or duplicating prior chunk
  history

### Requirement: Chunking is deterministic and boundary-aware
The service SHALL split eligible source text deterministically, preferring message,
sentence, paragraph, list, and code boundaries before applying configured maximum
character and token limits. Chunks SHALL retain deterministic ordinal order and
source ranges. An oversized indivisible boundary SHALL be split by a deterministic
hard bound.

#### Scenario: Source has natural boundaries within configured limits
- **WHEN** an eligible source contains message, paragraph, list, sentence, or code
  boundaries that fit the configured bounds
- **THEN** the chunker selects those boundaries before a fixed-width split and emits
  stable ordinals and source ranges

#### Scenario: One atomic source unit exceeds the hard limit
- **WHEN** an otherwise indivisible source unit exceeds the maximum configured bound
- **THEN** the chunker emits deterministically bounded fragments with the same parent
  lineage and does not create an unbounded chunk

### Requirement: Chunk granularity is memory-class aware
The service SHALL resolve chunk boundaries through one versioned policy that is
explicit for every memory class. Profile memory SHALL favor atomic facts; episodic
memory SHALL favor event or message units; procedural memory SHALL preserve bounded
rule or step groups; summaries SHALL use bounded coverage units; and relation memory
SHALL favor atomic relation facts.

#### Scenario: Procedural memory includes multiple ordered steps
- **WHEN** a visible procedural source contains an ordered set of steps within the
  configured policy bounds
- **THEN** the chunker preserves the step group when possible and records the policy
  version used

#### Scenario: A source class is ineligible for chunking
- **WHEN** the policy rejects a source class or source shape
- **THEN** materialization emits no chunk and records only a bounded omission reason
  in authorized diagnostics

### Requirement: Chunk operations enforce source scope and lifecycle
Chunk creation, reads, rebuilds, parent lookup, and adjacency lookup SHALL enforce
exact tenant/project/namespace scope and SHALL NOT cross a source session or user
boundary. Suppressed, forgotten, expired, deleted, missing, stale, or foreign
sources MUST be excluded from ordinary chunk materialization and consumption.

#### Scenario: Foreign-scope source is supplied for materialization
- **WHEN** a requested source does not resolve in the materializer's exact scope
- **THEN** the service rejects or omits it without persisting a chunk or disclosing
  foreign identifiers or content

#### Scenario: Visible chunk's parent becomes suppressed
- **WHEN** a previously materialized chunk is considered after its parent source is
  suppressed, forgotten, expired, or deleted
- **THEN** ordinary retrieval and context consumption exclude the chunk and do not
  expose its bounded content or parent identifier

### Requirement: Chunk rebuilds preserve provenance and history
The service SHALL rebuild chunks only from authorized PostgreSQL source records,
source watermarks, and recorded policy/renderer versions. Rebuild SHALL be
deterministic for identical inputs, append a new derived version when source or
policy identity changes, and preserve prior derived history for audit.

#### Scenario: Identical rebuild is requested
- **WHEN** a rebuild uses the same visible source versions, source watermark, policy
  version, renderer version, and chunker configuration
- **THEN** it produces the same content identities and ordinals without creating a
  divergent derived result

#### Scenario: Source version changes
- **WHEN** a new authorized canonical-memory version changes the materialized source
- **THEN** the service creates a new chunk derivation linked to that version while
  preserving the previous derivation as history

### Requirement: Parent and adjacent evidence are bounded
The service SHALL support retrieving a chunk's parent source and a bounded adjacent
chunk window only after exact scope and lifecycle validation. Parent or adjacent
expansion MUST honor configured count and character/token budgets and MUST fail
closed when lineage or visibility cannot be proven.

#### Scenario: Chunk hit needs local evidence context
- **WHEN** an authorized retrieval or context path requests parent or adjacent
  evidence for a visible chunk
- **THEN** it returns only the validated bounded evidence that fits the remaining
  budget with source citations

#### Scenario: Adjacent chunk belongs to another session or scope
- **WHEN** an adjacent candidate differs in session, user, tenant, project, or
  namespace from the selected chunk
- **THEN** the service omits it and does not broaden the lookup
