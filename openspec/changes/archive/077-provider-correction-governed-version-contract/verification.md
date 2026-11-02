# Provider Correction Governed Version Verification

Date: 2026-11-02

## Fresh PostgreSQL + pgvector evidence

`pwsh -NoProfile -File scripts/stele-provider-correction-verify.ps1` passed
against an owned, disposable PostgreSQL 18 + pgvector container. The verifier
removed its temporary container and emitted a bounded report containing only
the contract, status, image digest, suite names and statuses. Image digest:
`sha256:2ba9ca5f2e7daa0f0e7723cba1ee9167bab54efd3640516a44ac1a928dd67e7a`.

- `correction_resolver_replay_river_retry_handoff` — passed; includes fresh
  migrations, source-denial matrix, idempotent replay/conflict, River job
  rollback/retry and legacy handoff.
- `projection_feedback_supersession_active_summary` — passed; superseded
  feedback is excluded from active summaries while immutable history remains.
- `correction_governed_successor_restart_stale_worker` — passed; a corrected
  update appends a successor with immutable provenance, post-restart replay
  does not add a job/version, and a superseded source reaches a suppressed
  terminal transition without creating a successor.
- `capability_negotiation_strict_consumer` — passed; missing capability or
  operation fails strict validation while ordinary Provider capability remains
  valid.

## Repository gates

- `go test ./... -count=1` — passed.
- `go test -race ./... -count=1` — passed.
- `go vet ./...` — passed.
- `pwsh -NoProfile -File scripts/check-documentation.ps1` — passed.
- `pwsh -NoProfile -File scripts/check-self-hosting-smoke-docs.ps1` — passed.
- `openspec validate --all --strict` — passed after synchronizing all three
  delta specs into the main specifications (94 items, 0 failed; OpenSpec
  reported the expected already-synchronized delta notices).
- `git diff --check` — passed after the final documentation/spec updates.

No `scripts/check-quality-gate.ps1` exists in this checkout. The release gates
were executed directly rather than recording that absent script as passed.
