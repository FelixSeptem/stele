## ADDED Requirements

### Requirement: Runtime admission requires verified migration integrity
API, worker, and scheduler modes SHALL use the same checksummed migration
integrity validation as `stele migrate status` and SHALL fail closed before
protected HTTP traffic or job claims when integrity cannot be proven.

#### Scenario: Auto policy sees verified pending forward migrations
- **WHEN** a mode starts with `auto` policy against a clean, checksummed
  compatible migration prefix below the binary's latest version
- **THEN** it waits for the shared serialized forward transition and begins
  ordinary work only after the resulting integrity state is current

#### Scenario: Any mode sees migration divergence
- **WHEN** API, worker, or scheduler startup finds dirty, divergent,
  incompatible, or unreconciled unsupported migration history
- **THEN** the mode exits before protected traffic or job work and reports only
  bounded migration/readiness diagnostics
