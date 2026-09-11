## MODIFIED Requirements

### Requirement: Budget-aware context shaping
The service MUST support bounded context packing so the assembled response stays
within a caller-provided or service-default budget. Projection-backed and live
retrieval items MUST use the same deterministic budget accounting and MUST fail
closed when an item cannot fit. After normal eligibility, summary preference,
and identity/lineage deduplication, applicable existing sections MUST use the
selected deterministic diversity policy before final budget packing; that policy
MUST NOT increase the budget, change section names, broaden scope, or omit
required citations.

#### Scenario: Context budget is constrained
- **WHEN** a client requests context assembly with a limited budget
- **THEN** the service applies the active section-aware diversity policy to
  eligible deduplicated candidates, then trims and prioritizes sections
  according to retrieval or projection policy ordering and summary preference
  instead of returning unbounded memory

#### Scenario: Projection item exceeds remaining budget
- **WHEN** a lifecycle-visible projection item cannot fit within the remaining
  character/token budget
- **THEN** the item is omitted with a bounded budget reason and the assembler
  does not increase the requested budget or fetch a broader scope

#### Scenario: Diversity selection sees incomplete coverage attributes
- **WHEN** a visible eligible candidate lacks a source session, entity, or time
  slice attribute used by the active diversity policy
- **THEN** the assembler uses a bounded unknown category without loading broader
  source data, changing lifecycle visibility, or widening the resolved scope
