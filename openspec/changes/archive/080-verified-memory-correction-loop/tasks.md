## 1. Verification domain and persistence foundation

- [x] 1.1 Add procedural verification domain enums, immutable record/evidence/run inputs, bounded validation, deterministic fingerprints, and test supported/refuted/inconclusive/revalidation/error cases with `go test ./internal/memory -run ProceduralVerification`.
- [x] 1.2 Add migration `0044` for scoped append-only procedural verification records and lease-owned revalidation runs, including indexes/guards/down migration, and verify migration ordering and SQL contracts with `go test ./internal/storage/postgres -run ProceduralVerificationMigration`.
- [x] 1.3 Implement exact-scope PostgreSQL repositories for verification idempotency, current-target/evidence resolution, history/detail reads, and revalidation run lease/transition methods; verify isolation, lifecycle refusal, source-version drift, idempotency, and restart persistence with `go test ./internal/storage/postgres -run ProceduralVerification`.
- [x] 1.4 Extend history/provenance lookup models with bounded procedural-verification, run, intent, candidate, and successor links; verify authorized lineage and hidden/foreign redaction with `go test ./internal/memory ./internal/storage/postgres -run 'ProceduralVerification.*(History|Provenance)|Provenance.*ProceduralVerification'`.

## 2. Admission and bounded revalidation

- [x] 2.1 Implement the verification service to validate administrator attribution, active current procedural target/version, corroboration policy, source eligibility, and replay-safe append-only recording; verify every allowed disposition and transcript/model-only refusal with `go test ./internal/memory -run 'ProceduralVerificationService|ProceduralVerificationCorroboration'`.
- [x] 2.2 Add the `procedural_revalidation` derived-work kind, bounded dispatcher, per-scope policy/limits, and configuration validation; verify invalid work input, deterministic identity, disabled policy, and bounded enqueue behavior with `go test ./internal/workqueue ./internal/jobs ./internal/config -run ProceduralRevalidation`.
- [x] 2.3 Implement revalidation run request/coalescing and a lease-safe worker that re-resolves recorded target/evidence and appends `needs_revalidation` or `inconclusive` without semantic inference; verify source drift, feedback/manual triggers, retry, exhaustion, duplicate prevention, foreign/hidden refusal, and restart recovery with `go test ./internal/memory ./internal/jobs ./internal/storage/postgres -run ProceduralRevalidation`.
- [x] 2.4 Add bounded telemetry and admin-safe diagnostics for procedural verification/revalidation admission, queue status, outcomes, and failures; verify no high-cardinality evidence/scope content becomes a metric label with `go test ./internal/telemetry ./internal/jobs -run ProceduralVerification`.

## 3. Governed correction handoff and canonical lineage

- [x] 3.1 Extend governed intent contracts and PostgreSQL persistence with immutable verification attribution and idempotency fingerprints while retaining the closed Provider correction association; verify equivalent replay, conflicting reuse, and separate attribution compatibility with `go test ./internal/memory ./internal/storage/postgres -run VerificationAttributedIntent`.
- [x] 3.2 Add verification-correction admission that accepts only a current refuted procedural verification with corrected content and submits the existing governed `update` path; verify stale/foreign/hidden/policy-disabled/target-version refusal before intent or queue creation with `go test ./internal/memory ./internal/app ./internal/storage/postgres -run VerificationCorrectionAdmission`.
- [x] 3.3 Recheck verification attribution in governed worker promotion and persist append-only successor/candidate/provenance lineage; verify stale-before-worker, source drift, target conflict, accepted successor, and no in-place overwrite with `go test ./internal/memory ./internal/jobs ./internal/storage/postgres -run VerificationCorrectionWorker`.
- [x] 3.4 Run an owned PostgreSQL + pgvector fixture for correction/revalidation restart, retry, idempotency, source drift, lineage, and projection invalidation behavior; retain only bounded evidence and verify the fixture through its new repository script.

## 4. Privileged API, OpenAPI, and maintenance documentation

- [x] 4.1 Add administrator-only HTTP service interfaces, handlers, and exact-scope authorization for verification record, revalidation request/list/detail, and correction handoff; verify grant-before-lookup, validation, idempotency, lifecycle-safe not-found, and public Provider denial with `go test ./internal/app -run ProceduralVerification`.
- [x] 4.2 Publish request/response/error schemas and routes in `openapi/spec.go`; add OpenAPI contract tests for admin authorization, bounds, replay/conflict, compatibility, and public-route exclusion with `go test ./openapi ./internal/app -run ProceduralVerification`.
- [x] 4.3 Wire repositories, policy, dispatcher, worker, scheduler/cleanup, telemetry, and admin inspection into `api`, `worker`, and `scheduler` runtime modes; verify disabled default, enabled exact-scope operation, and API/worker restart with `go test ./internal/app ./internal/jobs ./internal/config -run ProceduralVerification`.
- [x] 4.4 Update architecture, components, self-hosting, provider/admin integration, README, and `skills/stele-memory/SKILL.md` with the admin-only verification/revalidation contract, operational limits, rollback, source trust, and correction workflow; verify documentation links and contracts with `pwsh -NoProfile -File scripts/check-documentation.ps1`.

## 5. Task evidence and longitudinal evaluation

- [x] 5.1 Extend task-evaluation evidence models, exact-scope validation, PostgreSQL persistence, summaries, admin reports, and OpenAPI with `procedural_verification`; verify task verdicts cannot mutate verification/canonical state and foreign/hidden links are denied with `go test ./internal/memory ./internal/storage/postgres ./internal/app -run ProceduralVerificationTaskEvidence`.
- [x] 5.2 Add checksum-locked `task-series-v2` / metric-v2 support for optional externally labeled repeated-error and bounded maintenance observations while retaining v1 parsing and report behavior; verify deterministic aggregation, incomplete coverage, redaction, unknown-field refusal, and checksum locks with `go test ./internal/benchmark -run TaskSeriesV2`.
- [x] 5.3 Add a synthetic EV1/EV2 longitudinal fixture comparing no-memory/source retrieval/verified-corrected conditions across an observed error and later stage, then verify only observational metrics are emitted with `go test ./internal/benchmark -run VerifiedMemoryCorrectionTaskSeries`.

## 6. End-to-end verification, archive, and publication

- [x] 6.1 Run focused domain, storage, app, jobs, benchmark, OpenAPI, documentation, and owned-stack verification; record bounded evidence and verify all task checkboxes correspond to passing behavior.
- [x] 6.2 Run `go test ./... -count=1`, `go test -race ./...`, `go vet ./...`, `openspec validate verified-memory-correction-loop --strict`, and `openspec validate --all --strict`; resolve any regression before marking the change complete.
- [x] 6.3 Archive the completed OpenSpec change with `pwsh -File scripts/openspec-archive-seq.ps1 -ChangeName "verified-memory-correction-loop"`; verify the numbered archive, regenerated index, synchronized specs, and no active change remain.
- [x] 6.4 Commit the verified implementation and archive, merge the feature branch into current `main`, push `main`, and verify local/remote commit identity plus clean worktrees before starting the next roadmap proposal.
