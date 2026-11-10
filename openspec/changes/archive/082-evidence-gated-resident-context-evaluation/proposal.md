## Why

EV3 now compares task-appropriate representations, but the resident layer still
has only profile confidence/size eligibility. EV4 needs a bounded experiment
that separates evidence of stability, applicability and later utility from
access popularity before any change to resident placement is authorized.

## What Changes

- Add an offline/shadow resident-placement evaluator against the current
  profile confidence/size policy, reusing the existing projection materializer.
- Require current-version, independently attributed support plus explicit
  stability, applicability and held-out task observations for proposed resident
  promotion. Missing observations remain inconclusive; access frequency is
  recorded only as an input and never confers truth or placement authority.
- Propose demotion of stale, corrected, unsupported or unhelpful resident items
  without changing canonical memory or the live projection.
- Enforce exact scope, active/current-valid sources, source references,
  disclosure lineage, and independent item/content-byte/token/citation bounds.
- Report redacted selection counts, omission reasons, cost and external utility
  comparisons with deterministic source/output identity and baseline rollback.

## Non-goals

No public route, Provider field, MCP capability, autonomous worker, live policy
activation, new verification ledger, model call, or canonical mutation. The
first slice preserves existing class placement: only eligible profiles can
enter always-visible context; procedural/episodic/relation material stays
on-demand. Whitespace token observations are not model usage or Provider limits.

## Capabilities

### New Capabilities

None.

### Modified Capabilities

- `versioned-context-projections`: offline resident promotion/demotion evidence
  with existing class and source constraints preserved.
- `context-efficiency-feedback-calibration`: equal-budget resident comparisons,
  externally supplied observations and redacted, non-activating reports.

## Impact

Internal retrieval evaluation and focused tests; owned PostgreSQL verification
over existing canonical/projection persistence; context/self-hosting/roadmap
and agent skill documentation. No dependency or schema migration is planned.
Use `openspec-apply-change`, `go test`, and the owned verifier, then archive via
`scripts/openspec-archive-seq.ps1` before commit, merge and push.
