## ADDED Requirements

### Requirement: Compaction evidence is watermark-bound and resumable

Compaction SHALL persist source watermark identity, summary and derivation versions,
token pressure estimates, bounded recent-tail references, evidence coverage, and
the resumable work/checkpoint identity used to produce them.

#### Scenario: Compaction input is incomplete

- **WHEN** the source watermark or exact-scope evidence cannot be validated
- **THEN** compaction fails closed and does not activate a summary or mark the work complete

#### Scenario: Compaction resumes after interruption

- **WHEN** a compaction attempt is reclaimed after a lease loss
- **THEN** it resumes from the latest durable checkpoint without deleting prior evidence

### Requirement: Compaction freshness gates projection eligibility

A compaction summary SHALL remain ineligible for default projection or context
until its source watermark, lifecycle visibility, evidence coverage, and freshness
SLO are valid for the exact scope.

#### Scenario: Summary evidence is stale

- **WHEN** source watermark freshness expires or diverges
- **THEN** the summary is marked stale/ineligible and a bounded rebuild or reflection follow-up may be scheduled

### Requirement: Compaction preserves source history

Compaction SHALL never delete or overwrite raw events, canonical versions,
provenance, prior summary evidence, or prior checkpoint history.

#### Scenario: New summary supersedes an old summary

- **WHEN** a newer compaction version is accepted
- **THEN** the prior evidence remains auditable with a superseded disposition
