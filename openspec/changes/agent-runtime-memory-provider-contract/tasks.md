## 1. Contract Models And Capability Discovery

- [ ] 1.1 Define versioned provider capability, limit, canonical-scope, operation-metadata, citation, result, and bounded-error models; verify serialization round-trips and rejects oversized/unknown required fields with focused unit tests.
- [ ] 1.2 Implement provider capability/version discovery backed by existing runtime metadata; verify responses contain supported operations, scope dimensions, limits, schema digest/version, and no secrets or unbounded diagnostics.
- [ ] 1.3 Add configuration for provider enablement, supported schema versions, per-operation limits, and binding lifetime with safe defaults; verify invalid values fail startup validation and disabled mode leaves existing routes unchanged.

## 2. Runtime Identity And Exact Scope Binding

- [ ] 2.1 Implement authenticated runtime initialization that separates agent identity, session, conversation, and provider instance while resolving one exact tenant/project/namespace grant; verify unauthorized and malformed initialization requests fail before repository access.
- [ ] 2.2 Add opaque server-owned runtime binding persistence/lookup or reuse an existing durable session binding without creating a second canonical store; verify grant revocation/expiry invalidates subsequent operations.
- [ ] 2.3 Add middleware that validates binding, session, principal, and requested scope on every provider operation; verify caller-invented, widened, mismatched, and cross-tenant scopes are rejected without existence disclosure.

## 3. Operation Metadata And Replay Safety

- [ ] 3.1 Implement normalization/validation for `request_id`, `operation_id`, `idempotency_key`, `event_seq`, and `schema_version`; verify bounded lengths, character rules, monotonic session sequence handling, and stable duplicate/stale dispositions.
- [ ] 3.2 Thread the common metadata envelope through event ingest and session outcome writes using existing durable idempotency repositories; verify equivalent retries return the original result and conflicting reuse creates no duplicate event, provenance, or feedback record.
- [ ] 3.3 Extend metadata propagation to intents, retrieval/context reads, forgetting/lifecycle requests, and status/report reads; verify operation correlation is preserved while read paths remain side-effect free and scope-safe.

## 4. Provider Operation Adapters And Safe Citations

- [ ] 4.1 Add provider handlers that delegate event ingest and governed memory intents to existing services; verify admission, provenance, lifecycle, and event-to-candidate-to-active governance cannot be bypassed by provider payloads.
- [ ] 4.2 Add provider retrieval and context assembly handlers with session binding, projection freshness, deterministic budgets, and lifecycle-safe defaults; verify hidden, stale, and foreign items never enter ordinary provider results.
- [ ] 4.3 Add provider forgetting/lifecycle request and scoped status/report handlers that route privileged actions through existing admin authorization; verify public provider callers cannot mutate canonical state directly.
- [ ] 4.4 Implement citation/provenance shaping for visible memory, projection, intent, and lifecycle outcomes; verify source kind/reference, version/watermark, and bounded availability are present while raw query, scores, hidden IDs, provider payloads, and credentials are absent.

## 5. OpenAPI Publication And Compatibility Errors

- [ ] 5.1 Publish provider discovery, runtime initialization, and operation routes plus schemas/examples in the authoritative OpenAPI document; verify the live endpoint advertises authentication, exact scope, idempotency, limits, citations, and error categories.
- [ ] 5.2 Add stable machine-readable compatibility, scope, validation, conflict, lifecycle, stale, dependency, and retryable error responses; verify unsupported schema versions fail before dispatch and never reveal hidden-record existence or stack traces.
- [ ] 5.3 Add contract tests that invoke the published OpenAPI document against API mode and confirm cache/version metadata remain consistent with the existing runtime API publication contract.

## 6. Provider Conformance And Assurance Integration

- [ ] 6.1 Define bounded provider conformance profiles and fixture operation manifests for capability, scope, replay, lifecycle, citation, restart/fallback, and freshness checks; verify unsupported evidence kinds and out-of-scope fixtures are rejected.
- [ ] 6.2 Implement a service-side conformance runner over an isolated exact scope using ordinary provider handlers; verify runs never execute an external agent/model and preserve canonical records except governed fixture ingestion.
- [ ] 6.3 Persist conformance outcomes through existing assurance records with bounded counters, verdicts, evidence references, schema provenance, and next actions; verify reruns create linked history and diagnostics do not become metric labels.
- [ ] 6.4 Add readiness/conformance tests for missing or stale dependencies, projection freshness, revoked scope, hidden memory, idempotency conflict, and interrupted durable operations; verify degraded/incomplete results cannot claim provider readiness.

## 7. Documentation, Rollout, And Verification

- [ ] 7.1 Document provider initialization, server-resolved scope, operation metadata, citations, supported errors, enablement, and rollback in OpenAPI/operator docs; verify docs consistency checks pass and no secrets/placeholders are introduced.
- [ ] 7.2 Add deterministic CI coverage for provider models, handlers, OpenAPI contract, isolation/redaction, idempotent replay, and conformance without provider credentials or ambient production DSNs; verify focused and full Go tests pass.
- [ ] 7.3 Run `openspec validate agent-runtime-memory-provider-contract --strict`, `openspec status --change agent-runtime-memory-provider-contract --json`, and `git diff --check`; verify all required artifacts are complete and the change is ready for `/opsx:apply`.
