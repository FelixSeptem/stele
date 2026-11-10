## Context

See `proposal.md` for motivation.  Stele already has versioned context
projections, progressive L0/L1/L2 and parent-first shadow evaluation,
repository-owned retrieval fixtures, bounded longitudinal task-series reports,
and a PostgreSQL-backed progressive experiment report store.  These mechanisms
are deliberately non-authoritative: `flat-fusion-v1` remains selected for
ordinary retrieval, all source data remains in PostgreSQL, and the current
release gate requires fresh compatible evidence and explicit authorization.

The missing layer is a reproducible mapping from task family to an existing
representation candidate, with metrics that distinguish required-fact
preservation from resource cost and externally supplied task observations.

## Goals / Non-Goals

**Goals:**

- Define a compact, checksum-locked representation-evaluation fixture format
  that can express exact-fact, cross-document, profile-preference, and
  procedural-lesson cases using existing corpus aliases and exact scopes.
- Evaluate four identities under equal limits: `flat-fusion-v1`,
  `exact-excerpt-with-search-v1`, `topic-projection-with-search-v1`, and
  `hierarchical-search-v1`.
- Reuse existing progressive level, parent-first, retrieval evaluation, and
  longitudinal aggregate machinery wherever their contracts already fit.
- Persist only exact-scope, redacted aggregate experiment evidence; provide a
  deterministic owned PostgreSQL + pgvector verification path.

**Non-Goals:**

- No new canonical memory class, public HTTP/MCP/Provider API, automatic
  selector, rollout flag, or retrieval-default modification.
- No free-form summary/wiki creation, source-less derived content, model call,
  model-token inference, or causal conclusion from task observations.
- No migration of canonical memory.  Existing context projection versions and
  progressive report retention remain the source of derived-history behavior.

## Decisions

### 1. Extend existing experiment evidence rather than introduce a new store

The runner will add a representation-comparison envelope to the existing
progressive experiment vocabulary and retain it through the current append-only
PostgreSQL report repository.  It will include only fixed representation/task
family enums, content-free checksums/identities, aggregate metrics, bounded
failure categories, and a rollback decision.

This reuses established exact-scope persistence, redaction, expiration, and
release-evidence patterns.  A new wiki, graph, or evaluation database was
rejected because it would duplicate authority and weaken the PostgreSQL-only
source-of-record boundary.  A report that serializes candidate content or IDs
was rejected because diagnostics must remain redacted.

### 2. Select candidates declaratively and preserve source search

Fixtures select exactly one declared task family and compatible candidate
identity.  The runner always evaluates the baseline and candidate on the same
visible fixture source, watermark, policy/renderer identity, and independently
bounded retrieval/representation budgets.  Candidate adapters use existing
exact excerpt, topic projection, or hierarchical/parent-first paths, then
retain ordinary authorized source search as a companion channel.

This makes source retrieval a control rather than allowing a projection to
become an opaque replacement.  A query-time automatic selector was rejected:
the roadmap calls for evidence first and current release gates prohibit silent
activation.

### 3. Treat protected facts and safety as gates, not optimization inputs

The report records critical-fact retention, required evidence/citation/source
coverage, stale/noisy or broken-reference categories, candidate/byte/token
budgets, latency, and externally supplied longitudinal observations.  Foreign,
hidden, stale, watermarked-incompatible, broken, over-budget, nondeterministic,
or incomplete protected-family results result in a stable non-pass and baseline
fallback.  Observed utility/repeated-error/maintenance differences stay
separate from release eligibility.

This follows existing retrieval hard-failure semantics.  A single weighted
score was rejected because it could hide a lost exact number or an isolation
violation behind cross-document recall gains.

### 4. Use repository fixtures for deterministic unit coverage and an owned stack for persistence/replay evidence

Unit tests will validate strict fixture decoding, checksum locking, equal-budget
comparison, each task-family/candidate compatibility rule, redaction, replay
identity, incomplete observations, and safety/bound failures.  PostgreSQL tests
will cover exact-scope persistence, replay/idempotency, foreign reads, expiry,
and append-only history.  The owned-stack script will use a temporary
PostgreSQL + pgvector container, only an explicit test DSN, bounded JSON
evidence, and owned-container cleanup.

This retains fast deterministic tests while proving the storage dependency.
Ambient service DSNs and unbounded test reports were rejected because they
would violate the release-evidence contract.

## Risks / Trade-offs

- [Existing projection interfaces may not expose all representation metadata]
  → add a narrow, evaluation-only adapter; do not bend public retrieval types or
  bypass their visibility predicates.
- [Synthetic fixtures can overfit a representation] → cover all four task
  families, lock source/qrel checksums and equal budgets, and label all results
  observational; real owned-stack evidence remains a separate prerequisite.
- [Aggregate reports can inadvertently leak fixture material] → make report
  serialization deny raw strings/aliases/scopes/IDs and add explicit leakage
  tests.
- [Topic/hierarchical candidates may cost more to build] → record maintenance
  observations separately, enforce bounds, retain cleanup/expiry, and never use
  cost as a truth or promotion signal.
- [A future activation may misread an experimental result] → encode
  shadow-only/default-unchanged behavior and require the existing release gate
  with this evidence as one prerequisite, not authorization.

## Migration Plan

1. Land the fixture, evaluator, bounded persistence/report extensions, tests,
   and documentation with all representation candidates disabled except for
   offline/shadow replay.
2. Run focused tests and the owned PostgreSQL + pgvector verifier.  Publish
   only redacted bounded evidence artifacts.
3. Maintain `flat-fusion-v1` as the selected ordinary strategy.  If an
   evaluation defect is found, disable the new replay admission/command and
   ignore its reports; retained derived evidence and canonical history remain
   intact.
4. Any future activation requires a separate proposal and the existing
   exact-scope release-evidence handoff; this change itself produces no active
   candidate.
