## 1. Operational Contract And Preflight

- [x] 1.1 Define the shared bounded run-state, prerequisite, timeout, cleanup, freshness, attestation, disablement, and rollback categories used by reports, summaries, and telemetry; verify category serialization is stable with focused unit tests.
- [x] 1.2 Extend the owned evaluation preflight path to reject missing/unowned markers, ambiguous or reused evaluation targets, incompatible fixtures/policies, and unavailable PostgreSQL/pgvector prerequisites without consulting `STELE_POSTGRES_DSN`; verify each outcome with wrapper and Go integration tests.
- [x] 1.3 Enforce the configured evaluation timeout and terminal outcome mapping for cancellation, timeout, skipped, degraded, failed, and completed runs; verify timed-out runs cannot be release-ready.

## 2. Isolated Artifacts And Evidence Handoff

- [x] 2.1 Make each evaluation attempt use an isolated report/fixture directory with idempotent cleanup for failed, skipped, cancelled, timed-out, and incomplete runs; verify no incomplete artifact is activation-consumable after cleanup.
- [x] 2.2 Preserve only redacted completed JSON/TXT evidence with stable run identity and bounded retention metadata while keeping prior reports append-only; verify DSNs, credentials, source content, raw scores, provider payloads, and identifiers are absent.
- [x] 2.3 Add logical attestation fields and validation for exact scope identity, source watermark/freshness, fixture/policy compatibility, integrity summary, and rollback verdict; verify stale, mismatched, or missing handoffs fail closed.

## 3. Release-Gate Closure And Rollback

- [x] 3.1 Require the release gate to revalidate the exact evidence handoff at eligibility review and preserve the separate activation authorization boundary; verify quality-positive but incomplete evidence remains non-pass. The existing ranking-rollout `CanActivateFor` path performs the authorized exact-scope handoff validation before persistence.
- [x] 3.2 Record activation disablement and rollback as redacted append-only evidence linked to the run/policy handoff, and restore the previously approved strategy without rewriting canonical memory or source records; verify disablement and rollback tests cover success and failure.
- [x] 3.3 Keep experimental strategies and default retrieval/context behavior unchanged while integrating the new closure checks; verify existing retrieval and OpenAPI contract tests remain green.

## 4. Bounded Observability

- [x] 4.1 Emit low-cardinality metrics and bounded structured logs for preflight, run lifecycle, timeout, cleanup, freshness/attestation, disablement, and rollback using fixed categories and duration/age buckets; verify sensitive-label rejection or bucketing with telemetry tests.
- [x] 4.2 Produce an authorized redacted operator summary that explains whether a run is consumable and why, without raw scope, query, prompt, source, provider, DSN, or record identifiers; verify summary redaction and authorization behavior. `RenderReleaseEvidenceSummary` and retained `release-evidence.txt` remain behind the owned evaluation/evaluation-admin workflow.
- [x] 4.3 Add focused retention and restart/retry tests proving cleanup is idempotent and repeated compatible runs preserve stable logical identities while retaining append-only history.

## 5. Documentation And Verification

- [x] 5.1 Update `docs/retrieval-release-checklist.md` and `docs/retrieval-release-gate.md` with the operational categories, handoff checks, cleanup/retention expectations, and disablement/rollback evidence; verify documentation consistency scripts pass.
- [x] 5.2 Reconcile `docs/roadmaps/2026-05-28-stele-v1-roadmap.md` so archived change 054 is not listed as active and this change is the single active bounded post-v1 proposal; verify the roadmap/OpenSpec consistency check passes.
- [x] 5.3 Run focused evaluation, retrieval, telemetry, and isolation tests plus `openspec validate --all` and `git diff --check`; verify all commands pass and no public default behavior changed.
