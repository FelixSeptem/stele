## 1. Contract and dependency baseline

- [x] 1.1 Review existing event, intent, governance, path, canonical,
  provenance and lifecycle APIs plus mature Go/PostgreSQL library options;
  record reuse decisions, dependency impact and requirement-to-code/test matrix.
- [x] 1.2 Publish the versioned durable-reference capability, closed write,
  status and exact-read DTOs in OpenAPI with supported event kinds, fields,
  bounds, statuses and bounded error categories.
- [x] 1.3 Add public-handler fixtures for accepted/pending/failed/completed,
  unsupported types, exact path, foreign scope, lifecycle removal and replay
  conflict; ensure fixtures use serialized JSON and strict unknown-field checks.

## 2. Durable write and governance status

- [x] 2.1 Define and enforce the allowlist mapping Provider event/intent types to
  reference promotion semantics; preserve archival behavior for unsupported
  types and keep ordinary governance gates first.
- [x] 2.2 Implement bounded status resolution over durable raw-event, candidate,
  canonical and provenance state with stable categories and no raw SQL/error
  disclosure.
- [x] 2.3 Extend transactional idempotency fingerprints and replay responses to
  include exact scope, principal/binding, event kind, normalized path, bounded
  value fingerprint and attribution; verify equivalent/conflicting reuse.
- [x] 2.4 Add repository tests for interrupted writes, retryable/terminal worker
  failure, UUID-compatible lineage and completion only after all required rows
  commit.

## 3. Path, identity and provenance preservation

- [x] 3.1 Trace raw event → intent → candidate → canonical promotion and retain
  non-root normalized `memory_path` at every boundary, including deduplication
  and summary/consolidation paths.
- [x] 3.2 Ensure event, memory, version and provenance identifiers use existing
  UUID-compatible storage types and return bounded lineage in the reference DTO.
- [x] 3.3 Add exact-path resolution over current eligible canonical versions;
  distinguish deterministic lookup from ranked search and preserve unavailable
  outcomes for missing or hidden records.
- [x] 3.4 Add focused tests for exact path versus descendant path, current
  version replacement, suppression, forgetting, deletion and privileged history.

## 4. Scope, disclosure and compatibility

- [x] 4.1 Apply current exact-scope, principal grant, disclosure profile and
  lifecycle gates before status or exact-read lookup; ensure foreign existence
  and hidden content are not disclosed.
- [x] 4.2 Wire capability/version negotiation and strict consumer assurance;
  reject missing fields, unsupported versions, aliases, nulls and unknown fields
  without local fallback or broader disclosure.
- [x] 4.3 Add bounded limits for values, lineage, response body and polling
  status; document omitted/empty and pending/failed/unavailable semantics.

## 5. Fresh PostgreSQL and restart verification

- [x] 5.1 Extend the public live verifier with a PC5 report identity, official
  bootstrap, fresh PostgreSQL + pgvector, supported task/run and compilation
  writes, governance polling and exact-path reads.
- [x] 5.2 Verify identical replay returns the original event/reference and
  conflicting replay is rejected; verify non-root paths, UUID lineage and
  provenance through direct bounded database assertions.
- [x] 5.3 Restart API and worker, then repeat status and exact-path lookup;
  require durable completion and readable current reference after restart.
- [x] 5.4 Suppress, forget and delete the referenced memory and verify explicit
  unavailable/lifecycle outcomes without historical resurrection or cross-scope
  leakage.

## 6. Documentation and release gates

- [x] 6.1 Update README, Provider integration, OpenAPI, architecture,
  components, self-hosting, best practices, MCP documentation and
  `skills/stele-memory/SKILL.md` for reference write/status/read semantics,
  supported event types, errors, limits, migration and rollback.
- [x] 6.2 Update the runtime-memory-provider and affected memory/event specs;
  state that PC5 does not own Danny task/business state and does not imply PC6.
- [x] 6.3 Run focused and full Go tests, changed-path race checks, vet,
  documentation checks, strict OpenSpec validation, diff checks and the fresh
  public verifier. Record redacted evidence with no credentials, authorization
  headers, scope values or unbounded reference content.
- [x] 6.4 Update roadmap status only after all acceptance evidence passes; archive
  this change using the repository archive sequencing script.
