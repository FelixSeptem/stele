# Self-Hosting First-Ten-Minutes Smoke Path

## Context

Stele already has two complementary self-hosting checks. The bootstrap smoke
script exercises the public bootstrap, principal, ingestion, retrieval, and
context contracts, but expects an already-running service and caller-managed
credentials. The full product verification script owns a disposable Compose
stack and verifies migration upgrade, restart, worker continuation, backup,
restore, and bootstrap behavior, but is intentionally too broad for a first
ten-minute operator check.

The missing path is a short, repeatable command that a new self-host operator
can run from the repository and use to answer: can this checkout start,
accept a scoped event, process it, retrieve it, assemble context, expose
bounded telemetry, and clean up its own disposable resources?

## Goals

- Provide one PowerShell entry point for the first self-hosting smoke path.
- Own the disposable Compose project, ports, credentials, and cleanup.
- Bound the default run to ten minutes, with a hard maximum of fifteen minutes.
- Verify service discovery, readiness, bootstrap, scoped least-privilege access,
  idempotent ingestion, worker processing, retrieval, context assembly, and
  metrics exposure.
- Emit phase-oriented, redacted output that is useful to operators and CI.
- Reuse existing API contracts and `stele-bootstrap-smoke.ps1` behavior rather
  than adding a second API or persistence path.
- Make local environment absence a controlled skip while making CI absence a
  failure.

## Non-goals

- No new HTTP route, OpenAPI schema, migration, memory class, or authorization
  boundary.
- No backup/restore, migration-upgrade, release-evidence, real-provider, or
  ranking-rollout activation checks in this short path.
- No replacement of the existing bootstrap smoke or full product verification
  scripts.
- No persistence of smoke fixtures or credentials after cleanup.

## Proposed Design

### Entry point and ownership

Add `scripts/stele-first-ten-minutes.ps1`. The script accepts a bounded
Compose file and optional project name, timeout, and `KeepResources` diagnostic
switch. It generates a valid isolated Compose project name when none is
provided, selects free host ports, creates random harness credentials, and
sets only process-local environment variables needed by the Compose stack.

The script invokes the existing Compose services (`postgres`, `api`,
`worker`, and `scheduler`) and calls `stele-bootstrap-smoke.ps1` for the
bootstrap/principal/lifecycle assertions that already define the public
contract. The new script owns the surrounding lifecycle and phase reporting.

### Phases

The output uses stable phase names and bounded result categories:

1. `preflight`: verify Docker CLI, daemon, Compose file, and required local
   tooling.
2. `start`: build and start the isolated stack.
3. `discovery`: verify `/health`, `/version`, and `/openapi.yaml`.
4. `readiness`: wait for `/readyz` within the remaining timeout.
5. `bootstrap`: create the durable administrator and exact runtime grant via
   the existing bootstrap smoke script.
6. `lifecycle`: verify idempotent event replay, conflict rejection, worker
   processing, scoped retrieval, and context assembly.
7. `telemetry`: verify `/metrics` responds and contains only the bounded
   contract signals required by the smoke check.
8. `cleanup`: stop and remove the owned Compose project, volumes, temporary
   credentials, and generated files unless `KeepResources` is set.

The script tracks a single deadline from the start of `start`. Every wait and
child process uses the remaining budget. A timeout is reported as a stable
`timeout` category rather than leaking a raw process error.

### Result and safety behavior

Successful output ends with `PASS` and a compact phase summary. Missing Docker
or an unavailable daemon returns `SKIP` with exit code 2 in local mode. CI mode
is enabled by `STELE_FIRST_TEN_MINUTES_CI=1`; in that mode the same prerequisite
condition returns a failure. Any assertion or cleanup failure returns a
non-zero failure code and names the phase and bounded reason.

The script never prints DSNs, API keys, scope values, memory/event identifiers,
request bodies, or raw provider/error payloads. Temporary credential files are
created only under an owned system temp directory and are removed in `finally`.
The Compose project name and generated port values are treated as harness
metadata, not product evidence.

### Documentation and CI

Add a short "first ten minutes" section to `docs/self-hosting.md` with
prerequisites, the command, expected phase output, controlled skip behavior,
and the boundary between this path and full product verification. Add a
separate quick-smoke step to the product-verification workflow so CI records
the short path independently from the longer verification stages.

Extend `docs/self_hosting_test.go` to assert the script contract, including
the phase names, timeout bounds, cleanup guard, CI/local skip distinction,
credential redaction, and reuse of the existing bootstrap smoke script.

## Failure Handling

- Preflight failures stop before creating persistent resources.
- Start or readiness failures trigger best-effort cleanup and identify the
  failing phase.
- A lifecycle assertion failure preserves only bounded counters/statuses in
  output; the default path removes all owned resources.
- Cleanup errors are reported separately and never turn a successful check
  into an unreviewable raw PowerShell trace; they do make the overall result
  non-zero when resources may remain.
- `KeepResources` is explicit and prints the Compose project name so an
  operator can inspect and remove it deliberately.

## Verification

- PowerShell contract tests cover the script's arguments, phase order,
  timeout maximum, skip/fail behavior, redaction, and cleanup structure.
- Documentation tests verify the command and boundary statements remain in
  sync with the script.
- Existing Go HTTP and storage tests remain the source of API and persistence
  correctness; this change adds no new runtime behavior.
- CI runs the script with `STELE_FIRST_TEN_MINUTES_CI=1` on an isolated Docker
  project and verifies a zero exit code.

## Rollout and Reversibility

The change is additive and disabled unless the new script is invoked. Removing
the script and its documentation/test references restores the prior workflow;
no database or canonical-memory rollback is needed. The existing full product
verification command remains the release-oriented path.
