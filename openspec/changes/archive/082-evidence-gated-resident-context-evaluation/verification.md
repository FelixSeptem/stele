# EV4 verification evidence

Verified on 2026-11-11. The resident evaluator is internal and offline/shadow.
It reuses the existing profile materializer and PostgreSQL source/projection
history. It introduces no public route, request or response field, live
placement policy, worker, configuration flag, dependency or migration.

The final independent owned-stack run used
`pwsh -NoProfile -File scripts/stele-resident-evaluation-verify.ps1 -ReportPath .tmp/resident-evaluation-final.json -Diagnostic`
and passed with image digest
`sha256:2ba9ca5f2e7daa0f0e7723cba1ee9167bab54efd3640516a44ac1a928dd67e7a`.
The bounded report contract is
`evidence-gated-resident-evaluation-owned-stack-v1`; all four suites passed:

- `resident_safety_bounds_and_rollback`
- `resident_postgres_materialization_lineage_drift_scope`
- `resident_post_restart_projection_history_and_citations`
- `resident_post_restart_stored_row_checksums`

The harness persists five canonical/immutable/provenance source records and
actual raw support/utility observations in its uniquely owned database. The
two in-scope baseline profiles consume 80 UTF-8 content bytes, 12 whitespace
tokens and two citations. The evidence-filtered candidate retains one profile
(43 bytes, seven tokens, one citation), proposes one promotion and one demotion,
and excludes all three foreign tenant/project/namespace sources. Both conditions
have independent limits of 8 items, 4096 bytes, 1024 whitespace tokens and 16
citations. These counts do not measure Provider wire bytes or model tokens.

Both projections are persisted under the same policy; exact immutable source
references and citations, the superseded baseline rows and source history are
checked. A real appended source version inside a rolled-back transaction
rejects both an unresolved version mismatch and support for the older version.
Disabled selection equals the baseline plan. Complete stored source, version,
raw-event, provenance, projection and item checksums remain equal across an
actual PostgreSQL restart; post-restart repository reads confirm history and
isolation. The run removes its exact owned container and volumes and verifies
their absence. It does not use an operator's PostgreSQL DSN.

This is synthetic service-conformance evidence. No external task outcomes are
supplied by the owned run: `outcome_status` stays `inconclusive`, both outcome
coverage flags are false and deltas remain absent. It demonstrates neither
general task benefit nor causal improvement and authorizes no activation.

Unit regressions cover exact source/claim versions and content, source
disclosure/lifecycle/valid time, all isolation dimensions, qualifying evidence
kinds and attribution, stale/missing/negative/ambiguous observations, held-out
stage/time ordering, duplicate JSON keys, independent budgets and Unicode,
popularity independence, deterministic redacted hashes, tampering, baseline
rollback and missing versus observed false/zero external outcomes. A test-first
regression exposed loss of immutable temporal validity at the shared
materializer boundary; plans now preserve that interval and deep-copy its
optional end time so returned plans cannot mutate rollback state.

Independent review identified and test-first regressions reproduced three
additional evidence-boundary defects before archival. Disclosable untrusted
events could satisfy support/utility/comparison attribution; all now require
existing `attested_data` attribution in addition to disclosure. Case-insensitive
JSON aliases could override fields; all three observation types now whitelist
exact canonical keys before decoding. Unrelated or rejected stage values could
block later outcomes; only compatible validated support/utility associated
with eligible evaluated sources now advances the stage. Tests cover all three
observation types and unrelated/foreign/expired/oversized-stage/untrusted events.
Attestation still establishes attribution only, never truth or instruction
authority.

After these review fixes, the owned-stack verifier was rerun successfully. Full
`go test ./... -count=1 -timeout 15m`,
`go test -race ./... -count=1 -timeout 20m`, `go vet ./...`, the documentation
check and `openspec validate --all --strict` passed (96 items, zero failures
before archival). `git diff --check` passed. Archive and publication remain
separately auditable in `tasks.md`.
The documentation check initially encountered the inaccessible global Go cache;
rerunning with the worktree-local `.tmp/gocache` passed.

At the delivery checkpoint on 2026-11-11, the independent reviewer process
had not returned a final status. The implementation and regressions were
re-read locally, and the full Go suite, strict change validation and diff
whitespace check were rerun successfully. This record does not claim an
independent review sign-off.
