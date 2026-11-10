## 1. Representation fixture and compatibility contract

- [x] 1.1 Define versioned task-family and representation-candidate identities, strict equality/bound validation, and redacted aggregate report types; verify unsupported family/candidate pairs, invalid limits, missing source/qrel locks, and unsafe identities with focused `go test ./internal/retrieval ./internal/benchmark -run Representation`.
- [x] 1.2 Add checksum-locked repository fixtures for exact terms/numbers, cross-document evidence, profile preferences, and evidence-backed procedural lessons; verify strict decoding, equal-budget enforcement, critical-fact/exclusion validation, and checksum tamper refusal with `go test ./internal/retrieval ./internal/benchmark -run RepresentationFixture`.
- [x] 1.3 Add deterministic baseline/candidate compatibility and replay identity checks that retain authorized source search for every candidate; verify incompatible watermark/policy/renderer/fixture identities and nondeterministic input are stable non-passes with `go test ./internal/retrieval -run RepresentationCompatibility`.

## 2. Offline/shadow comparison evaluator

- [x] 2.1 Implement an evaluation-only adapter over existing exact excerpts, topic projections, and hierarchical/parent-first paths without changing `flat-fusion-v1` selection or public response shapes; verify each task-family candidate remains offline/shadow and source search is retained with `go test ./internal/retrieval -run RepresentationSelection`.
- [x] 2.2 Compute bounded family-level critical-fact retention, source/citation coverage, stale/noisy and broken-reference categories, candidate/representation budget use, latency, and externally supplied task/maintenance observation coverage; verify missing observations remain inconclusive and reports never infer agent/model outcomes with `go test ./internal/retrieval ./internal/benchmark -run RepresentationMetrics`.
- [x] 2.3 Enforce hard non-pass and baseline rollback on scope/lifecycle/freshness loss, broken reference, protected-fact loss, over-budget, or nondeterministic replay; verify aggregate quality/utility cannot offset any failure with `go test ./internal/retrieval -run RepresentationSafety`.
- [x] 2.4 Enforce report redaction and deterministic serialization; verify fixture aliases, task/source/query text, scopes, identifiers, raw scores, provider payloads, credentials, and per-stage costs cannot cross the report boundary with `go test ./internal/retrieval ./internal/benchmark -run RepresentationRedaction`.

## 3. PostgreSQL evidence and release-gate integration

- [x] 3.1 Extend the exact-scope progressive experiment persistence boundary for append-only representation comparison aggregates and retention cleanup; verify replay/idempotency, foreign read refusal, malformed aggregate refusal, expiration cleanup, and prior-history preservation with `go test ./internal/storage/postgres -run RepresentationExperiment`.
- [x] 3.2 Integrate compatible task-family representation evidence into the existing release evaluator as a prerequisite-only, observational input; verify fresh complete evidence remains non-activating, and stale/incomplete/incompatible/unsafe evidence is a stable non-pass with `go test ./internal/retrieval -run RepresentationRelease`.
- [x] 3.3 Wire any needed offline/admin diagnostics through the existing authorization boundary without adding Provider, MCP, or ordinary retrieval routes; verify public/default retrieval remains byte- and strategy-compatible with `go test ./internal/app ./internal/retrieval -run Representation`.

## 4. Documentation and owned-stack verification

- [x] 4.1 Update progressive-context experiment, retrieval release gate, architecture/components, self-hosting, README, and `skills/stele-memory/SKILL.md` documentation with task-family coverage, shadow-only/default behavior, bounds, evidence interpretation, and rollback; verify links/contracts with `pwsh -NoProfile -File scripts/check-documentation.ps1`.
- [x] 4.2 Add an owned PostgreSQL + pgvector representation-evaluation verifier that uses only a random-owned temporary container and bounded redacted evidence; verify replay, each task family, source-search retention, stale/foreign/hidden/broken/over-budget refusal, persistence/restart, cleanup, and rollback with the repository script.

## 5. End-to-end verification, archive, and publication

- [x] 5.1 Run focused retrieval, benchmark, PostgreSQL, app, documentation, and owned-stack checks; record bounded evidence and verify all preceding task checkboxes correspond to passing behavior.
- [x] 5.2 Run `go test ./... -count=1`, `go test -race ./...`, `go vet ./...`, `openspec validate task-appropriate-representation-evaluation --strict`, and `openspec validate --all --strict`; resolve any regression before marking the change complete.
- [x] 5.3 Archive the completed OpenSpec change with `pwsh -File scripts/openspec-archive-seq.ps1 -ChangeName "task-appropriate-representation-evaluation"`; verify the numbered archive, regenerated index, synchronized specs, and no active change remain.
- [ ] 5.4 Commit the verified implementation and archive, merge the feature branch into current `main`, push `main`, and verify local/remote commit identity plus clean worktrees before starting the next roadmap proposal.
