## 1. Contract baseline and data model

- [x] 1.1 Review existing Provider turn/outcome, session, feedback, insight,
  PC4 digest and PC5 reference contracts plus mature Go/PostgreSQL library
  options; record reuse/dependency decisions and a requirement-to-code/test
  matrix in the verification record.
- [x] 1.2 Define bounded projection domain DTOs, state/disposition unions,
  logical effect identity, source revision, correction/supersession lineage and
  exact reference-attribution states; verify unit tests reject malformed,
  unknown, null, aliased, oversized and business-authority fields.
- [x] 1.3 Design and add the append-only PostgreSQL projection/provenance
  persistence and scope/index constraints, using a migration only if existing
  storage cannot represent the required durable identity; verify a clean
  migration ledger and scoped repository round trips.

## 2. Admission, correction and governed attribution

- [x] 2.1 Implement transactional projection admission with binding-derived
  scope, normalized payload fingerprint and durable idempotency claim; verify
  equivalent replay returns the original identity and conflicting key reuse
  creates no duplicate rows or feedback.
- [x] 2.2 Implement greater-revision correction/supersession and delayed or
  out-of-order disposition without in-place overwrite; verify prior evidence,
  audit lineage and concurrent/restart retry behavior in repository tests.
- [x] 2.3 Resolve supplied PC5 durable references under exact scope and persist
  accepted/pending/completed/failed/unavailable attribution without content
  disclosure; verify foreign, hidden, lifecycle-removed and pending references
  cannot become completed/eligible evidence.
- [x] 2.4 Reuse session/turn/outcome persistence where sufficient and map
  projection usefulness/user feedback only after admission; verify projections
  cannot mutate task, lease, commit, publication, delivery or canonical memory
  state.

## 3. Feedback and insight boundaries

- [x] 3.1 Connect eligible projection feedback to existing append-only
  usefulness-feedback source attribution and supersession behavior; verify
  pending/failed/unavailable reference attribution remains excluded from active
  summaries and feedback/ranking activation.
- [x] 3.2 Ensure only asynchronous, versioned, exact-scope, lifecycle-filtered
  feedback/insight jobs can consume eligible projection evidence; verify a
  projection write alone neither changes retrieval ranking nor activates or
  reactivates an insight.
- [x] 3.3 Add history/inspection projections that return bounded identity,
  revision, attribution and disposition without raw payloads or foreign
  existence; verify current grant and lifecycle checks on ordinary reads.

## 4. Public Provider and OpenAPI contract

- [x] 4.1 Publish the dedicated versioned Provider projection route(s), closed
  OpenAPI request/response schemas, capability metadata, bounds and stable
  validation/compatibility/conflict/stale/scope/lifecycle/dependency errors;
  verify authoritative OpenAPI validation and route coverage tests.
- [x] 4.2 Wire public handlers to validate full runtime binding before any
  dispatch, preserve existing generic turn/outcome compatibility, and return
  explicit runtime-reported-evidence semantics; verify serialized HTTP fixtures
  for accepted, replayed, corrected, conflict, pending, unavailable and foreign
  outcomes.
- [x] 4.3 Extend strict Provider consumer/conformance assurance to negotiate
  `turn-outcome-feedback-projection-v1` and reject malformed/missing closed
  fields, nil identities, state conflation or legacy fallback; verify consumer
  fixtures against the served OpenAPI document.

## 5. Fresh-stack assurance and restart evidence

- [x] 5.1 Extend the official Provider public verifier with a redacted PC6
  report, fresh owned PostgreSQL + pgvector bootstrap and serialized public
  projection requests; verify service/OpenAPI/contract/image/source/migration
  provenance and no secret, scope or raw-content leakage in artifacts.
- [x] 5.2 Prove equivalent and conflicting delivery, corrections, exact-scope
  PC4 digest/PC5 reference attribution, pending-to-completed transitions,
  lifecycle unavailability, feedback nonactivation and privileged history using
  public APIs plus bounded supplemental SQL assertions.
- [x] 5.3 Restart API and worker during/after projection processing and repeat
  status/inspection/conformance checks; verify one logical projection, preserved
  correction history and no duplicate feedback or insight effect after restart.

## 6. Maintenance, gates and lifecycle

- [x] 6.1 Update README, Provider integration, OpenAPI, architecture,
  components, self-hosting/bootstrap, best practices, MCP guidance and
  `skills/stele-memory/SKILL.md`; verify docs state the additive migration,
  capability negotiation, pending semantics, correction/history, rollback and
  Danny business-state boundary.
- [x] 6.2 Synchronize affected OpenSpec requirements and roadmap PC6 status
  only after implementation evidence passes; verify strict delta/main-spec
  synchronization and documentation links.
- [x] 6.3 Run focused and full Go tests, changed-path race checks, vet,
  OpenAPI/strict-consumer checks, documentation gates, strict OpenSpec
  validation, diff checks and the fresh public verifier; record bounded results
  in the verification artifact before archiving with the repository sequencing
  script.
