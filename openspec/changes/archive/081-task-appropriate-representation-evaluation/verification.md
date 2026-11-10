# EV3 verification evidence

Verified on 2026-11-10. All representation candidates remain offline/shadow;
ordinary retrieval selects `flat-fusion-v1`. No public route or API field was
added. The fixed source corpus is synthetic; real PostgreSQL execution proves
service conformance, not general task utility or causal benefit.

The final owned-stack verifier passed using image digest
`sha256:2ba9ca5f2e7daa0f0e7723cba1ee9167bab54efd3640516a44ac1a928dd67e7a`.
Its bounded report contract is
`task-appropriate-representation-evaluation-owned-stack-v1` with status
`passed` and no failed phase. All four suites passed:

- `representation_safety_and_bounds`
- `representation_postgres_real_comparison_and_rollback`
- `representation_post_restart_read_and_scoped_cleanup`
- `representation_post_restart_persistence`

The verifier seeds governed source/version/provenance records, activates real
pgvector revisions, materializes existing projections and reads them back,
executes production search for both conditions, repeats candidates, and replays
the flat baseline for rollback. Foreign tenant/project/namespace records and
hidden lifecycle records cannot contribute. Source-version drift is injected
inside a rolled-back transaction. Persisted report and canonical-history
checksums survive an actual container restart, followed by scoped report expiry
cleanup and removal of only the harness-owned container.

Regression tests cover protected literal values, incorrect or mixed citations,
missing baseline, unsafe baseline budgets, stale source/version/watermark/
policy/renderer, temporal validity, hidden/foreign sources, missing search,
combined budgets, missing external observations, evidence-bound identity,
report redaction/tampering and persisted expiry. Observed task failure differs
from success; external deltas remain absent when coverage is incomplete.

Full `go test ./... -count=1 -timeout 15m`,
`go test -race ./... -count=1 -timeout 20m`, and `go vet ./...` passed.
The documentation check and strict validation of the change and all OpenSpec
items passed. Archive and publication are tracked separately in `tasks.md`.
