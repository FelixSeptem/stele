## 1. Resident evidence and policy evaluation

- [x] 1.1 Add bounded policy/evidence/utility contracts and test-first validation of claim/source versions, attribution, stability, applicability, temporal validity and popularity independence; verify `go test ./internal/retrieval -run Resident`.
- [x] 1.2 Reuse the existing profile materializer for equal-budget baseline/candidate plans, bounded demotion/omission reasons, independent item/UTF-8-byte/token/citation caps and baseline rollback; verify focused resident tests including hidden/foreign/disclosure safety and deterministic Unicode accounting.
- [x] 1.3 Add deterministic redacted reports with actual source/output digests, separate external observation values/coverage and deltas; verify tampering/redaction/missing-outcome tests and ordinary API byte compatibility.

## 2. Integration and maintenance evidence

- [x] 2.1 Add an owned PostgreSQL/pgvector verifier and integration test using existing source/projection persistence; verify actual materialization, source refs, source changes, baseline rollback, restart history and owned-container cleanup.
- [x] 2.2 Update context projection/evaluation, architecture/self-hosting/README, roadmap and agent skill docs with exact offline limits, evidence semantics, default behavior, compatibility and rollback; verify documentation checks.

## 3. Verification and delivery

- [x] 3.1 Review the implementation, run focused/full/race/vet/docs and strict OpenSpec checks, and record bounded owned-stack evidence before completion. Local implementation review and regression verification are recorded; an independent reviewer did not return a final status during this delivery run.
- [x] 3.2 Archive via `scripts/openspec-archive-seq.ps1 -ChangeName evidence-gated-resident-context-evaluation`; verify synchronized specs, numbered index and no active change. Archived as 082 and both delta specifications were synchronized.
- [ ] 3.3 Commit the verified implementation/archive, merge into main, push, verify remote identity and clean worktrees, then record delivery audit.
