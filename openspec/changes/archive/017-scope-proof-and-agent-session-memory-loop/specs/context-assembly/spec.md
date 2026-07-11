## ADDED Requirements

### Requirement: Context assembly supports proof and session diagnostics
The service SHALL allow proof and memory-session workflows to request context assembly with diagnostic attribution.

#### Scenario: Session turn assembles context
- **WHEN** a memory session starts a turn with a scoped query
- **THEN** context assembly returns agent-ready sections and records the memory ids, citations, and diagnostics used as session turn evidence

#### Scenario: Proof verifies expected context recall
- **WHEN** a proof run checks whether fixture memory appears in context
- **THEN** context assembly can report whether expected evidence was included, omitted by budget, omitted by quality, hidden by lifecycle, or unavailable

#### Scenario: Ordinary context request is unchanged
- **WHEN** a caller uses context assembly without proof or session attribution
- **THEN** the service preserves the existing context assembly behavior and response shape
