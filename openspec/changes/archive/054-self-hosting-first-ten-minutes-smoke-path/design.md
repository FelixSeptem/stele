## Context

See `proposal.md` for the motivation and scope. The repository already owns
`docker-compose.yml`, `scripts/stele-bootstrap-smoke.ps1`, and
`scripts/stele-product-verify.ps1`. The new path must compose those existing
contracts without creating a second service lifecycle implementation or
changing the Go runtime.

## Goals / Non-Goals

**Goals:**

- Keep all disposable resources under one generated Compose project.
- Make the run deadline and phase names deterministic enough for local logs and
  CI summaries.
- Preserve the existing bootstrap smoke assertions as the source of truth for
  public API behavior.
- Keep output redacted and cleanup recoverable when diagnostics are requested.
- Make the script contract testable without requiring Docker in Go unit tests.

**Non-Goals:**

- Do not add a Go command, HTTP endpoint, migration, or runtime configuration
  field.
- Do not duplicate the full product verification's backup, restore, migration
  upgrade, or restart drills.
- Do not make smoke output a release-evidence artifact or authorize any
  retrieval/ranking rollout.

## Decisions

### PowerShell orchestration over a Go command

Use a PowerShell script because the existing Compose and self-hosting
verification assets are PowerShell, and the workflow already runs PowerShell
on Windows and Linux runners. A Go command would add Docker process-control
code to the service binary and duplicate the existing scripts. A documentation
only recipe would leave isolation, deadline, and cleanup behavior to each
operator.

### One deadline with bounded child operations

Expose `-TimeoutSeconds` with a default of `600` and reject values above `900`
or below a small positive bound. Capture the deadline after preflight and pass
the remaining seconds to polling loops and child scripts. Every phase checks
the deadline before starting work. This prevents a slow image pull or stuck
readiness loop from extending the promise beyond fifteen minutes.

### Explicit phase runner and stable result categories

Implement a small local phase wrapper that records `preflight`, `start`,
`discovery`, `readiness`, `bootstrap`, `lifecycle`, `telemetry`, and `cleanup`.
The wrapper emits only `phase`, `result`, and a bounded reason category. Raw
PowerShell exceptions are captured for control flow and mapped to categories
such as `missing_prerequisite`, `start_failed`, `not_ready`, `assertion_failed`,
`timeout`, or `cleanup_failed`.

### Reuse bootstrap smoke with an owned credential directory

Generate the bootstrap key and default scope values inside the script, then
invoke `stele-bootstrap-smoke.ps1` with an owned temporary credential
directory. The parent script reads only the generated credential files needed
for the worker/lifecycle and metrics checks, never prints their contents, and
removes the directory in `finally` unless `-KeepResources` is explicitly set.

### Compose isolation and cleanup

Set `COMPOSE_PROJECT_NAME` to a validated generated or caller-supplied value,
select bounded random host ports, and use the existing environment override
variables supported by `docker-compose.yml`. Cleanup uses the same compose
file and project name with `down --volumes --remove-orphans`. The script must
not run cleanup against an unvalidated project name, and `KeepResources` must
print only the project identifier needed for later operator cleanup.

### Contract tests without Docker

Extend `docs/self_hosting_test.go` to read the script as text and assert the
required phase names, timeout constants, CI environment switch, bootstrap
script invocation, redaction markers, project validation, and cleanup command.
The live Docker path remains in CI workflow execution; unit tests remain
deterministic and runnable without a daemon.

### CI placement

Add one step before the longer product verification checks in
`.github/workflows/product-verification.yml`. Set
`STELE_FIRST_TEN_MINUTES_CI=1` and let the script own its project name and
ports. Keep the existing full verification step unchanged so failures retain
their current diagnostic separation.

## Risks / Trade-offs

- [Docker image pulls exceed the default budget] -> Use the existing mirror
  override variables, report `timeout`, and allow the bounded fifteen-minute
  maximum; do not silently extend the deadline.
- [A failed run leaves containers or volumes] -> Always run best-effort
  `finally` cleanup, report `cleanup_failed` separately, and provide an
  explicit `KeepResources` diagnostic mode rather than implicit retention.
- [PowerShell behavior differs across runner versions] -> Keep to existing
  PowerShell constructs used by the current scripts and cover argument/string
  contracts in Go tests plus a real CI invocation.
- [Reusing bootstrap smoke couples phase output to its messages] -> Treat the
  child script as an assertion contract and have the parent map its exit status
  to a bounded phase category; do not parse arbitrary output as evidence.
- [Smoke fixtures appear in operator logs] -> Avoid printing response bodies,
  credentials, scope values, IDs, and raw exceptions; retain only phase and
  result categories.

## Migration Plan

1. Add the script, documentation section, contract tests, and CI step.
2. Run deterministic docs tests, `git diff --check`, and OpenSpec validation.
3. Run the script locally when Docker is available; verify the controlled skip
   behavior when it is not.
4. CI runs the new quick path before the existing full product verification.

Rollback is deletion or disabling of the additive script, docs section, test,
and workflow step. No database or service rollback is needed.

## Open Questions

None. The timeout, phase model, reuse boundary, and CI behavior are fixed by
the approved design and proposal.
