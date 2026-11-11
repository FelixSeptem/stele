# EV5 locked longitudinal v3 verification

Verified on 2026-11-11 in worktree `locked-longitudinal-memory-evaluation`.
The change is an internal, opt-in offline CLI/evaluation contract. It adds no
public route, request/response field, authorization role, runtime retrieval
policy, worker, configuration flag, dependency or database migration.

## Owned-stack evidence

The delivered verifier is `scripts/stele-longitudinal-v3-verify.ps1`. The
WorkBuddy PowerShell host used for this session silently blocks native command
invocation (neither `go` nor `docker` produced output or set
`$LASTEXITCODE`), so the script could not be executed end to end inside that
host. It is delivered for normal pwsh terminals and was reviewed statically;
all of its phases were executed equivalently with Bash, the same Go tests and
an owned Docker container (`.tmp/owned-verify-equivalent.sh`, not committed).
This distinction is explicit: the ps1 harness itself was not measured as
passing in this session.

The equivalent run used image digest
`sha256:2ba9ca5f2e7daa0f0e7723cba1ee9167bab54efd3640516a44ac1a928dd67e7a`,
contract `locked-longitudinal-owned-stack-v1` and passed all three suites:

- `contracts_bounds_missing_costs_and_safety`
- `actual_stage_windows_lineage_disclosure_budgets_and_baseline_replay`
- `post_restart_staged_replay_and_verified_row_cleanup`

Bounded report: `.tmp/longitudinal-v3-owned-final.json` (4,864 bytes,
`status=passed`, `restart_replay_verified=true`,
`owned_cleanup_verified=true`). The staged replay test itself runs the full
runner twice, so repeated runs are covered; the two condition reports are
byte-identical apart from measured latency gates. A real container restart
changed `pg_postmaster_start_time()`, replayed with the rebound ephemeral host
port, and preserved `v3.input_digest`, `normalized_checksum` and
`gate_status=passed`. The runner verifies that no owned rows remain in
`canonical_memories`, `raw_events`, `candidate_memories` or
`provenance_links`; the harness then removed its random-owned container and
volumes and confirmed their absence.

The locked synthetic fixture has five conditions, two stages and two immutable
source windows per non-baseline condition, with the correction available only
at the later stage. `comparison_complete=false`, external verdicts stay
inconclusive and `cleanup_verified=true`. This is synthetic service
conformance, not task benefit, causal improvement or rollout authority.

## Implementation review and corrected defects

1. The first staged replay failed with
   `source metadata snapshots are immutable (SQLSTATE P0001)`. The runner had
   tried to `UPDATE` raw/canonical/version `source_metadata` after seeding.
   PostgreSQL derives those immutable snapshots at INSERT. The fixture seeder
   now accepts optional `SourceMetadata` / `SourceVersion` on
   `retrieval.EvaluationSource`, binds them at INSERT in both batch and single
   seed paths, and leaves all existing fixtures on the restrictive default.
   Candidate/version rows inherit the snapshot through existing triggers.
   Fault injection (disabling the INSERT binding) made
   `TestTaskSeriesV3PostgresOwnedSnapshots` fail with
   `v3 physical stage snapshot differs from lock`; restoring the file byte for
   byte made it pass again.
2. Docker Desktop reassigns an ephemeral host port on `docker restart`
   (observed 63506 -> 63508). The verifier kept the pre-restart DSN, so the
   post-restart replay failed to connect. The script now re-resolves
   `docker port` after restart and rebuilds the DSN; the equivalent Bash flow
   does the same and passed.
3. The verifier resolves `docker.exe` explicitly (Docker Desktop also ships an
   extensionless launcher that pwsh rejects inside a pipeline), accepts an
   optional `-GoExe`, writes its own bounded phase log via `-LogPath`, and no
   longer puts native commands in the middle of a PowerShell pipeline.

An independent read-only review found no blocker or major findings. Minor
notes: the immutable-snapshot trigger should stay covered by migration tests
(the replay fails closed if it is missing); v3 top-level `status` remains
non-success while utility is inconclusive even when all hard gates pass, so
consumers must read `v3.gate_status`; and `tasks.md` checkboxes had lagged the
implementation (now reconciled). No top-level status change was made: the
specification requires missing observations to stay inconclusive.

## Verification commands

- `go build ./...`, `go vet ./...`, `go test ./... -count=1 -timeout 15m`
  (26 packages passed), `go test ./docs/... -count=1`, `gofmt -l` on changed
  Go files (clean), `git diff --check`.
- `go test -race ./... -count=1 -timeout 20m` (26 packages passed).
- `openspec validate locked-longitudinal-memory-evaluation --strict --type change`
  (valid) and `openspec validate --all --strict` (96 passed, 0 failed before
  archival).
- CLI coverage: `TestRunBenchmarkTaskSeriesLocksV3AndLegacyFixtures` locks both
  the new v3 fixture and the unchanged v2 fixture; the v2 source/qrel checksums
  are asserted byte-for-byte. The v2 coverage denominator correction is the
  only legacy behavior change.

## Delivery audit

Archived as `083-locked-longitudinal-memory-evaluation`; `INDEX.md` records the
083 entry, `openspec list` reports no active change, and both ADDED
requirements are present in
`openspec/specs/longitudinal-agent-task-memory-evaluation/spec.md`.
Post-archive `openspec validate --all --strict` reports 95 passed, 0 failed.
The archive numbering script was used for sequence/index normalization; the
`openspec archive` step itself ran from Bash because this session's PowerShell
host blocks native commands.

Commit, merge, push and post-merge verification are recorded in the archived
`tasks.md` entry 4.4.
