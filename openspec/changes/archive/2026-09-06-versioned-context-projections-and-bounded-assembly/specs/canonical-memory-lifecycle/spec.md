## MODIFIED Requirements

### Requirement: Append-only canonical versioning
Canonical memory updates MUST be append-only and MUST preserve material history.
Derived context projections MUST reference canonical versions but MUST NOT mutate
or replace canonical memory when they are materialized, rebuilt, filtered, or
packed into context.

#### Scenario: Canonical memory receives a material update
- **WHEN** consolidation changes the canonical content or state of a memory
- **THEN** the service writes a new memory version instead of mutating the
  previous version in place

#### Scenario: Projection is rebuilt from canonical memory
- **WHEN** a projection is rebuilt after a canonical version changes
- **THEN** the service writes a new projection version referencing the selected
  canonical version and leaves canonical memory and prior projection history
  unchanged
