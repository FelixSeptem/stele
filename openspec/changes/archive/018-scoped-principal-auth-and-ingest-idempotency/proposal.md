## Why

Protected Stele routes currently accept any configured API key and then trust
the caller-supplied tenant, project, and namespace headers. That makes
scope-isolated repository queries insufficient as an authorization boundary,
and public event retries can create duplicate raw events that amplify later
governance and derived-memory work.

This change establishes the first production-grade caller identity boundary and
makes event ingestion retry-safe before broader external integration adoption.

## What Changes

- Add durable scoped principals with bounded public/admin roles, disabled and
  expiry state, explicit tenant/project/namespace grants, and hash-only API key
  storage.
- Add a bootstrap-operator configuration path for first deployment, while
  requiring lifecycle-managed principals for ordinary protected access.
- Enforce that a requested scope header must be granted to the authenticated
  principal before a handler receives scope context. Return uniform
  authorization failures without disclosing grant, principal, or target scope
  existence.
- Add admin-only scoped principal inspection and bounded lifecycle controls for
  creation, rotation, disablement, expiry, and grant management. Raw API keys
  are returned only once at create or rotate time and are never persisted or
  logged.
- Add `Idempotency-Key` support to `POST /v1/events`, scoped to the resolved
  principal and memory scope. Exact retries return the original durable event
  and admission decision; reuse with a different request fingerprint returns a
  conflict without creating another event.
- Persist bounded authentication, authorization, key-lifecycle, and ingestion
  idempotency audit records without raw credentials, event content, or
  unbounded request metadata.

**BREAKING**: protected callers must use a principal key with a grant for each
requested scope. Existing process-wide `STELE_AUTH_API_KEYS` and
`STELE_AUTH_ADMIN_API_KEYS` remain only as a documented bootstrap-operator
compatibility path and no longer authorize arbitrary public scopes.

## Non-goals

- Do not add end-user accounts, hosted identity, OAuth/OIDC login flows, UI, or
  SDK behavior.
- Do not change memory lifecycle, retrieval ranking, agent execution, model
  invocation, prompt orchestration, or final-answer generation.
- Do not introduce a second system of record, external identity provider, Redis
  cache, or token-introspection network dependency.
- Do not add the later migration-framework, HTTP resource-limit, or durable
  multi-scope worker work described in the roadmap.

## Capabilities

### New Capabilities

- `scoped-principal-access`: Durable API principals, credential lifecycle,
  explicit scope grants, protected-route authorization, and bounded audit.

### Modified Capabilities

- `scoped-api-access`: Replace key-presence-only authentication and
  caller-declared scope trust with principal-bound authorization.
- `event-ingestion`: Make public raw-event retries idempotent and expose stable
  replay/conflict semantics.
- `admin-inspection-surface`: Add admin principal and credential/grant
  inspection and lifecycle controls.
- `service-runtime-foundation`: Define bootstrap-operator compatibility
  configuration and fail-safe startup validation for principal access.

## Impact

- Affected code: `internal/auth`, `internal/config`, `internal/app`,
  `internal/memory`, `internal/storage/postgres`, `internal/telemetry`,
  `openapi`, and self-hosting documentation.
- Affected storage: additive principal, principal-grant, credential lifecycle,
  authentication audit, and event-idempotency records in PostgreSQL.
- Affected APIs: all protected public/admin routes receive principal-bound scope
  authorization; new admin principal endpoints; `POST /v1/events` gains the
  `Idempotency-Key` header and conflict response.
- Artifact references: use `openspec instructions apply --change
  scoped-principal-auth-and-ingest-idempotency --json` to implement and the
  repository's configured OpenSpec archive command when the change is complete.
