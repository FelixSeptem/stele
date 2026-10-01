## Why

Stele has a bootstrap smoke script and a comprehensive product-verification
script, but neither is a concise first-run path for a new self-host operator.
The bootstrap script assumes an already-running service, while full product
verification includes migration, restart, backup, and restore checks that are
too broad for a quick confidence check. A bounded, self-owned ten-minute path
is needed now so local operators and CI can verify the core self-hosting loop
without learning several scripts or managing disposable resources manually.

## What Changes

- Add `scripts/stele-first-ten-minutes.ps1` as an isolated Compose smoke
  orchestrator for preflight, startup, discovery, readiness, bootstrap,
  lifecycle, telemetry, and cleanup phases.
- Give the command a ten-minute default deadline and a hard fifteen-minute
  maximum, with remaining-budget propagation to waits and child commands.
- Own a unique Compose project, random host ports, temporary credentials, and
  cleanup; retain resources only when an explicit diagnostic switch is used.
- Reuse `scripts/stele-bootstrap-smoke.ps1` for the existing public bootstrap,
  principal, idempotent-ingestion, retrieval, context, and scope assertions.
- Verify `/health`, `/readyz`, `/version`, `/openapi.yaml`, `/metrics`, worker
  processing, scoped retrieval, and context assembly without adding routes or
  persistence behavior.
- Emit stable phase/result categories while redacting DSNs, API keys, scope
  values, record identifiers, request bodies, provider payloads, and raw
  process errors.
- Return a controlled local `SKIP` with exit code 2 when Docker prerequisites
  are unavailable, while treating the same condition as a CI failure when
  `STELE_FIRST_TEN_MINUTES_CI=1` is set.
- Document the first-ten-minutes path and its boundary with full product
  verification, add contract assertions in `docs/self_hosting_test.go`, and
  run the short path as a separate product-verification CI step.

### Non-goals

- No new HTTP route, OpenAPI schema, migration, memory class, authorization
  boundary, provider integration, or ranking rollout behavior.
- No backup/restore, migration-upgrade, release-evidence, real-provider, or
  activation checks in this quick path.
- No replacement of the existing bootstrap smoke or full product-verification
  workflows.
- No persistence of smoke fixtures or credentials after the default cleanup.

## Capabilities

### New Capabilities

None. This is a tooling, documentation, and CI workflow change with
`skip_specs: true`; it does not introduce service behavior that belongs in an
OpenSpec capability requirement.

### Modified Capabilities

None.

## Impact

- New PowerShell orchestration under `scripts/`, reusing the existing Compose
  deployment and bootstrap smoke contract.
- Self-hosting guidance under `docs/self-hosting.md` and documentation
  contract tests under `docs/self_hosting_test.go`.
- Product verification workflow under `.github/workflows/product-verification.yml`.
- No Go runtime, PostgreSQL schema, OpenAPI, or public API changes.
- Related workflow references: `openspec validate --all`,
  `scripts/stele-bootstrap-smoke.ps1`, `scripts/stele-product-verify.ps1`, and
  `STELE_FIRST_TEN_MINUTES_CI=1`.
