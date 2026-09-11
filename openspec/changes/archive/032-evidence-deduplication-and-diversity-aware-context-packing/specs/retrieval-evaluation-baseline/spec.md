## MODIFIED Requirements

### Requirement: Retrieval quality metrics
The evaluator SHALL calculate recall, ranking, coverage, duplication,
candidate-pool, diversity-selection, and latency metrics from lifecycle-visible
scoped results. For an evaluated diversity policy version, it SHALL compare the
identity/lineage-deduplicated baseline with the policy result and report
protected evidence coverage, duplicate rate, diversity disposition aggregates,
candidate-pool size, and latency without serializing raw candidate payloads.

#### Scenario: Expected evidence is retrieved
- **WHEN** a query returns one or more required evidence aliases within the configured
  cutoff
- **THEN** the report includes the applicable Recall@k, MRR, nDCG@k, final rank, and
  multi-hop evidence coverage contribution

#### Scenario: Similar evidence crowds a result set
- **WHEN** multiple returned hits map to the same fixture fact cluster or source group
- **THEN** the report records duplicate-rate evidence separately from recall and rank
  quality and identifies whether the evaluated policy omitted duplicate or
  diversity-competing evidence through bounded aggregate dispositions

#### Scenario: Diversity policy is compared with its baseline
- **WHEN** an evaluator compares a named diversity-policy version with a
  compatible identity/lineage-deduplicated baseline
- **THEN** the report includes per-metric deltas for protected recall, evidence
  coverage, duplicate rate, candidate-pool size, and latency, and rejects the
  candidate if a hard safety failure occurs

#### Scenario: Replay has bounded execution
- **WHEN** a replay run completes or fails
- **THEN** the report includes bounded candidate-pool and latency measurements without
  serializing raw fixture payloads
