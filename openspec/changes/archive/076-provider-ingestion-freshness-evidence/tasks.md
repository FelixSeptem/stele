## 1. Contract and deterministic evaluator

- [x] 1.1 Add the closed `ingestion-freshness-evidence` domain types, version,
  stage/disposition/freshness/SLO enums, exact-scope validation and bounded
  redacted report shape; verify invalid values and prohibited fields are
  rejected by unit tests.
- [x] 1.2 Implement deterministic stage ordering, latency buckets and
  freshness/SLO evaluation with configurable windows; verify repeated identical
  inputs produce byte-stable JSON and invalid/regressing timestamps fail closed.
- [x] 1.3 Implement idempotent report identity and replay/conflict comparison;
  verify equivalent replay returns the same logical result and changed scope,
  policy or stage data returns a bounded conflict.

## 2. Existing service and assurance integration

- [x] 2.1 Add an exact-scope read adapter that correlates raw event governance,
  candidate/canonical lifecycle and authorized public-read observations without
  changing event, retrieval or disclosure semantics; verify foreign and hidden
  records produce unavailable/ineligible stages with no existence disclosure.
- [x] 2.2 Add low-cardinality freshness/SLO telemetry and bounded admin/
  assurance inspection output using existing retention conventions; verify no
  scope, payload, raw ID, path, query or ranking value reaches serialized output
  or metric labels.
- [x] 2.3 Extend assurance contract tests for pending, retrying, exhausted,
  promoted, lifecycle-hidden, source-ineligible and successful public-read
  outcomes; verify a write acknowledgment or canonical promotion alone cannot
  pass the final stage.

## 3. Fresh PostgreSQL and restart evidence

- [x] 3.1 Extend the owned PostgreSQL + pgvector assurance fixture to record
  initial stage evidence for admission, governance, canonical promotion and an
  authorized public retrieval observation; verify the report contains only
  bounded hashes/categories and a passing result only after all stages complete.
- [x] 3.2 Add worker delay/retry/restart and lifecycle/source-disclosure
  negative fixtures; verify reports remain nonpassing and distinguish pending,
  retrying, exhausted, hidden and ineligible outcomes.
- [x] 3.3 Repeat the same fixture after API/worker restart and verify stable
  report identity, replay behavior, exact-scope isolation and no duplicate
  logical evidence.

## 4. Documentation and release gates

- [x] 4.1 Publish the new version/capability in OpenAPI or the existing admin/
  assurance contract, with bounded errors, redaction rules and compatibility
  behavior; verify strict schema tests and OpenSpec validation.
- [x] 4.2 Update architecture, components, self-hosting, Provider/assurance
  guidance, best practices and the Agent Skill to distinguish admission,
  governance, canonical and public-read stages; verify documentation checks.
- [x] 4.3 Run focused/full Go tests, race/vet where applicable, diff checks,
  strict OpenSpec validation and the fresh-stack verifier; retain redacted
  initial/restart evidence and mark every task complete only after the commands
  pass.
- [x] 4.4 Update the roadmap candidate status only after acceptance evidence
  passes, then archive with `scripts/openspec-archive-seq.ps1` and verify the
  archive index and synchronized main spec.
