## 1. Smoke Orchestrator

- [x] 1.1 Add `scripts/stele-first-ten-minutes.ps1` argument parsing, validated Compose project naming, bounded timeout (`600` default and `900` maximum), random port/credential setup, and local-vs-CI Docker preflight; verify the script rejects invalid timeout/project values and returns local `SKIP` code 2 when Docker is unavailable.
- [x] 1.2 Implement the phase runner and single-deadline helpers for `preflight`, `start`, `discovery`, `readiness`, `bootstrap`, `lifecycle`, `telemetry`, and `cleanup`; verify output contains only stable phase/result categories and timeout is mapped to a bounded category.
- [x] 1.3 Implement isolated Compose startup and discovery/readiness checks, invoking only owned services and existing endpoints; verify the script waits on `/readyz`, uses remaining timeout budget, and does not print DSNs or raw process errors.
- [x] 1.4 Reuse `stele-bootstrap-smoke.ps1` with an owned temporary credential directory, then add worker/lifecycle and metrics assertions for idempotent ingest, conflict rejection, processed work, scoped retrieval, context assembly, and bounded `/metrics`; verify success ends in `PASS` and failed phases preserve only bounded reasons.
- [x] 1.5 Add `finally` cleanup for Compose volumes/orphans, temporary credentials, and generated files, with explicit `-KeepResources` diagnostics; verify default cleanup runs after preflight/start/lifecycle failures and retained resources print only the validated project name.

## 2. Documentation And Contract Coverage

- [x] 2.1 Add a first-ten-minutes section to `docs/self-hosting.md` covering prerequisites, invocation, phase output, local skip/CI failure behavior, cleanup, and the boundary from full product verification; verify the documented command and environment switch match the script.
- [x] 2.2 Extend `docs/self_hosting_test.go` with text-contract assertions for phase names, timeout bounds, CI switch, bootstrap-script reuse, redaction markers, project validation, cleanup guards, and the no-new-runtime-behavior boundary; verify `go test ./docs -count=1` passes without Docker.
- [x] 2.3 Reconcile `docs/roadmaps/2026-05-28-stele-v1-roadmap.md` and its consistency test to identify this change as the active bounded post-v1 proposal until archive; verify `TestRoadmapTracksCurrentP8Proposal` passes.

## 3. CI And Verification

- [x] 3.1 Add a dedicated quick-smoke step to `.github/workflows/product-verification.yml` with `STELE_FIRST_TEN_MINUTES_CI=1`, preserving the existing full product-verification step and isolated resource ownership; verify workflow YAML includes the quick path before the longer verification.
- [x] 3.2 Run deterministic checks (`go test ./docs -count=1`, `git diff --check`, and `openspec validate --all`) and exercise the script's controlled local skip when Docker is unavailable; record the observed exit category and ensure no credential/temp artifact remains.
- [x] 3.3 When Docker and pgvector are available, run the first-ten-minutes command against an isolated Compose project and verify `PASS`, worker completion, retrieval/context responses, metrics response, and automatic resource cleanup; when unavailable, retain the controlled skip evidence without claiming a live-stack pass.
