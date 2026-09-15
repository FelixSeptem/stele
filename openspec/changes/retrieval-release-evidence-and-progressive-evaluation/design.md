## Context

The archived P5 capability already defines release reports, protected quality
gates, redacted trajectories, progressive context levels, parent-first shadow
experiments, retention, and rollback policy. The roadmap still lacks an
explicit run boundary that proves those contracts against an owned PostgreSQL
18 + pgvector environment. See `proposal.md` and the archived
`retrieval-release-gate-and-progressive-context-evaluation` specification for
the existing behavior that this change composes.

## Goals / Non-Goals

**Goals:**

- Create one bounded release-evidence orchestration boundary for real-stack and
  offline/shadow evaluations.
- Make evaluation DSN ownership and prerequisite state explicit and fail-closed.
- Run progressive context and parent-first comparisons against one exact scope
  without changing default retrieval.
- Preserve existing redaction, retention, rebuildability, rollback, and
  canonical-source invariants.
- Keep ordinary contributor tests deterministic and provider-independent.

**Non-Goals:**

- No new canonical persistence model or second database.
- No automatic rollout, default ranking change, or public search response change.
- No mandatory remote model, LLM judge, benchmark download, or P6 maintenance
  closure.
- No raw trajectory, query, prompt, scope, credential, or provider payload
  export.

## Decisions

### 1. Use a dedicated evaluation-run boundary

Introduce a run input containing owned evaluation DSN reference, provider
profile, fixture/representation/fusion/ranking/policy identities, exact scope,
and requested strategies. Validate ownership and compatibility before opening
the evaluation connection. A missing DSN yields the stable skip category and
never consults the service runtime DSN.

Alternatives considered:

- Reuse the service DSN: rejected because it can mutate or inspect production
  state and violates the roadmap's owned-evidence requirement.
- Add a second benchmark product: rejected because the run must compose the
  archived release-gate, projection, and retrieval contracts.

### 2. Separate prerequisite verdict from quality verdict

The evaluator computes prerequisite, safety, resource, quality, progressive,
parent-first, and rollback outcomes independently, then applies a precedence
order: prerequisite failure, scope/lifecycle/freshness failure, rollback
failure, resource violation, and only then protected quality thresholds. A
quality improvement can never compensate for a safety or rollback failure.

### 3. Run experimental strategies through shadow adapters

Progressive levels and parent-first expansion consume the same fixture and
exact-scope derived projections as flat fusion. Their outputs are compared and
reported as derived artifacts only; the ordinary retrieval service remains on
the previously approved strategy. Expansion is bounded by candidate count,
latency, budget, and lifecycle visibility.

### 4. Persist only bounded redacted evidence

The report writer receives allowlisted aggregate fields and redacts before
persistence/export. It stores logical identities, category counts, buckets,
watermarks, freshness, citation coverage, rebuild IDs, and rollback status. It
does not store DSNs, credentials, queries, content, raw scores, identifiers, or
provider payloads. Existing retention/deletion services own derived artifact
cleanup.

### 5. Provide offline CI smoke plus opt-in real-stack execution

Repository fixtures exercise compatibility, missing-DSN, stale/hidden/foreign,
budget, rollback, and deterministic rebuild behavior without PostgreSQL. A
release job may invoke the real evaluator only when its explicit evaluation DSN
and PostgreSQL/pgvector prerequisites are present. Skipped evidence is visible
and non-pass, never silently promoted.

## Risks / Trade-offs

- **Owned DSN misconfiguration** → validate the evaluation DSN source and reject
  service-DSN fallback; redact all connection metadata from reports.
- **Synthetic evidence overconfidence** → label offline/shadow runs as non-pass
  for release and require a fresh owned real-stack run.
- **Projection drift** → compare source watermarks and freshness before quality,
  and fail closed on stale or divergent evidence.
- **Shadow cost** → bound fixture size, expansion, latency, and context budgets;
  retain only aggregate trajectories.
- **Strategy leakage** → keep experimental adapters outside default retrieval and
  test that production responses are unchanged.

## Migration Plan

1. Add run/profile/verdict models and prerequisite validation without changing
   retrieval defaults.
2. Compose existing replay, projection, progressive-context, parent-first,
   trajectory, integrity, retention, and rollback services behind the run.
3. Add deterministic offline tests for skip/degraded/non-pass behavior and exact
   scope/redaction/rebuild guarantees.
4. Add an opt-in PostgreSQL 18 + pgvector release job that requires a dedicated
   evaluation DSN and records bounded evidence.
5. Update operator runbooks and roadmap evidence; enable rollout only through
   an explicit policy decision after a passing run.
6. Roll back by disabling the evaluation/strategy policy; preserve canonical
   records and derived history, then rebuild artifacts when needed.
