## ADDED Requirements

### Requirement: Resident experiments compare bounded observational conditions

Resident experiments SHALL compare the current profile policy and an
evidence-filtered candidate under identical scope, source snapshot, clock and
independent item/content-byte/token/citation limits. They SHALL report bounded
selection, omission and proposed demotion counts, source/output/policy digests,
external task utility/error/cost observations and tested baseline rollback.
Missing external observations MUST remain inconclusive. Reports MUST exclude
raw text, scope values and source/task identifiers, remain deterministic and
non-activating, and MUST NOT equate whitespace tokens or bytes to model usage.

#### Scenario: Equal-budget resident comparison is repeated
- **WHEN** fixed compatible inputs are replayed with equal limits
- **THEN** selection order, aggregates, digests and omission categories remain
  stable and ordinary context and Provider responses remain unchanged

#### Scenario: Utility differs or observations are absent
- **WHEN** baseline/candidate utility observations differ or are incomplete
- **THEN** the report preserves the observed difference separately from safety,
  coverage and resource gates without inferring missing success or causal effect

#### Scenario: Candidate is disabled or rolled back
- **WHEN** the resident experiment is disabled or requests rollback
- **THEN** the baseline plan is selected without changing canonical data,
  projection history, disclosure grants or public request semantics
