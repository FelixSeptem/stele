## 1. Compatible contracts and source locking

- [x] 1.1 Reproduce v2 inconclusive-stage coverage above one in `internal/benchmark/longitudinal_v3_test.go`, correct the denominator, and verify `go test ./internal/benchmark -run TaskSeriesV2`.
- [x] 1.2 Add opt-in v3 types and validation in `longitudinal_v3.go` and existing manifest dispatch; test exact five roles, shared tasks, experiment identities/bounds, source windows, disclosure, version/content identity, no-memory emptiness and legacy refusal with `go test ./internal/benchmark -run TaskSeriesV3`.
- [x] 1.3 Bind v3 source locks and stage snapshots before SQL, reject JSON ambiguity, future/withdrawn evidence and metadata bypasses; verify loader tampering and snapshot regressions, including unchanged v1/v2 fixture checksums.

## 2. Independent observations and gates

- [x] 2.1 Extend existing aggregation/report validation with nullable tool/model observations and independent coverage, v3 gate counts and private-input digests. Write failing missing/false/zero, redaction, bounds, safety/latency/rollback tests first; verify focused task-series tests.

## 3. PostgreSQL staged replay

- [x] 3.1 Implement `longitudinal_v3_postgres.go` using existing fixture seeder, retrieval service and cleanup. Verify physically stage-visible sources, all four result budgets, exact scope/lifecycle/disclosure, actual citation lineage, measured service latency and later baseline replay in owned PostgreSQL tests.
- [x] 3.2 Add a locked five-condition synthetic fixture and `scripts/stele-longitudinal-v3-verify.ps1`; verify replay, explicit future-correction absence, repeated runs, restart and exact owned cleanup. Keep outcomes inconclusive and report synthetic evidence truthfully.

## 4. Maintenance and delivery

- [x] 4.1 Update README, benchmark/self-hosting guides, agent skill and roadmaps for shapes, validation/errors, authorization, defaults, v2 denominator compatibility and v3 limits; verify documentation tests and CLI legacy/v3 coverage.
- [x] 4.2 Obtain independent review, resolve findings, run focused/full/race/vet/docs and strict OpenSpec checks, and record owned-stack evidence in `verification.md`.
- [x] 4.3 Archive with `scripts/openspec-archive-seq.ps1 -ChangeName locked-longitudinal-memory-evaluation`; verify synchronized specifications, numbered index and no active change.
- [ ] 4.4 Commit, merge into main, test merged state, push, verify remote identity and clean worktrees, and publish the delivery audit.
