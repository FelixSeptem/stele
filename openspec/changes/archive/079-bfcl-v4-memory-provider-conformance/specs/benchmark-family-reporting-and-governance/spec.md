## ADDED Requirements

### Requirement: BFCL official-input conformance evidence is bounded and auditable
Each BFCL V4 official-input Provider-contract report SHALL retain the dataset
identity, selected subset, source and normalized checksums, source revision,
conversion version, compatibility state, prerequisite/snapshot digest, supported
and unsupported operation counts, conformance outcomes, Stele revision, and
cleanup result. The report SHALL redact credentials, raw source contents,
scope values, memory identifiers, hidden candidates, and unbounded operation
payloads.

#### Scenario: Inspect a completed official-input contract report
- **WHEN** a maintainer reads a completed BFCL V4 Provider-contract report
- **THEN** the maintainer can reproduce its local inputs and evaluate its
  compatibility, coverage, safety, and cleanup result without a mutable remote
  service or disclosure of restricted source/runtime data

#### Scenario: Inspect an incomplete official-input contract report
- **WHEN** a BFCL V4 local source cannot complete because an input, snapshot,
  or required operation is unavailable or unsupported
- **THEN** the report identifies the bounded failure category and coverage gap
  without presenting incomplete execution as passing conformance
