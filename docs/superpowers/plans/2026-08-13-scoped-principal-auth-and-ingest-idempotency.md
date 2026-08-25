# Scoped Principal Access And Idempotent Ingestion Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Bind protected requests to durable scoped principals and make public event writes replay-safe.

**Architecture:** PostgreSQL stores principals, credential digests, exact grants, lifecycle audit, and event idempotency results. HTTP middleware authenticates a principal before accepting a scope header, while the ingestion service atomically owns claim, raw event, provenance, and replay-result persistence through a repository method.

**Tech Stack:** Go, `net/http`, `pgx`, PostgreSQL, OpenAPI 3.1, existing telemetry observer and OpenSpec.

---

### Task 1: Principal Domain And Schema

**Files:**
- Create: `internal/auth/principal.go`
- Create: `internal/auth/principal_test.go`
- Modify: `internal/storage/postgres/migrations/0001_base_schema.up.sql`
- Modify: `internal/storage/postgres/migrations/0001_base_schema.down.sql`
- Modify: `internal/storage/postgres/bootstrap_test.go`

- [ ] **Step 1: Write failing validation tests** for public/admin roles, active/disabled credentials, exact scope grants, bounded labels, and rejected empty IDs.
- [ ] **Step 2: Run `go test ./internal/auth -run Principal -count=1`** and confirm the desired API is absent.
- [ ] **Step 3: Add minimal auth domain types**: `Principal`, `Credential`, `ScopeGrant`, status/role enums, validation, and safe projections that omit digest/secret fields.
- [ ] **Step 4: Run the auth tests** and confirm they pass.
- [ ] **Step 5: Add additive tables and indexes** for principals, credentials, grants, audit records, and event idempotency; include bootstrap migration expectations.
- [ ] **Step 6: Run `go test ./internal/auth ./internal/storage/postgres -run 'Principal|Bootstrap' -count=1`** and confirm schema and types pass.

### Task 2: Principal Repository And Credential Verification

**Files:**
- Create: `internal/storage/postgres/principal_repository.go`
- Create: `internal/storage/postgres/principal_repository_test.go`
- Modify: `internal/storage/postgres/repository.go`

- [ ] **Step 1: Write failing repository tests** for active credential lookup, exact grant authorization, disabled/expired credential rejection, and scope-filtered reads.
- [ ] **Step 2: Run `go test ./internal/storage/postgres -run Principal -count=1`** and confirm repository methods are missing.
- [ ] **Step 3: Implement repository methods** using indexed credential lookup, bcrypt digest comparison in auth/service boundary, exact scope filters, and no raw secret reads.
- [ ] **Step 4: Run repository tests** and confirm they pass.

### Task 3: Principal Authorization Middleware

**Files:**
- Modify: `internal/auth/middleware.go`
- Modify: `internal/auth/middleware_test.go`
- Modify: `internal/app/http.go`
- Modify: `internal/app/http_test.go`

- [ ] **Step 1: Write failing middleware tests** proving an authenticated principal reaches a granted public scope, an ungranted scope never reaches the next handler, and public role cannot invoke an admin route.
- [ ] **Step 2: Run `go test ./internal/auth -run PrincipalMiddleware -count=1`** and confirm the authorization middleware is missing.
- [ ] **Step 3: Implement principal and scope contexts plus role-aware authorization middleware** while retaining existing `ScopeFromContext` handler behavior.
- [ ] **Step 4: Run auth and HTTP authorization tests** and confirm role/scope enforcement passes.

### Task 4: Runtime Configuration And Bootstrap Operator

**Files:**
- Modify: `internal/config/config.go`
- Modify: `internal/config/config_test.go`
- Modify: `internal/app/app.go`
- Modify: `internal/app/app_test.go`

- [ ] **Step 1: Write failing configuration tests** for bootstrap key/default-scope requirements and deprecated legacy key rejection.
- [ ] **Step 2: Run `go test ./internal/config -run Principal -count=1`** and confirm the settings are absent.
- [ ] **Step 3: Add constrained bootstrap configuration and runtime auth resolver wiring** without logging raw keys.
- [ ] **Step 4: Run config and app runtime tests** and confirm bootstrap scope and durable principal behavior pass.

### Task 5: Admin Principal Control Plane

**Files:**
- Create: `internal/auth/service.go`
- Create: `internal/auth/service_test.go`
- Modify: `internal/app/http.go`
- Modify: `internal/app/http_test.go`
- Modify: `openapi/spec.go`
- Modify: `openapi/openapi_test.go`

- [ ] **Step 1: Write failing service/HTTP tests** for principal creation, one-time credential issuance, rotation, disablement, grant revoke, and redaction on subsequent reads.
- [ ] **Step 2: Run targeted auth and app tests** and confirm principal admin endpoints are unavailable.
- [ ] **Step 3: Implement service methods, admin routes, safe DTOs, and OpenAPI schemas** for principal, credential, grant, and audit operations.
- [ ] **Step 4: Run targeted auth, app, and OpenAPI tests** and confirm lifecycle operations and redaction pass.

### Task 6: Idempotent Event Ingestion

**Files:**
- Modify: `internal/memory/types.go`
- Modify: `internal/memory/service.go`
- Modify: `internal/memory/service_test.go`
- Modify: `internal/storage/postgres/repository.go`
- Modify: `internal/storage/postgres/repository_test.go`
- Modify: `internal/app/http.go`
- Modify: `internal/app/http_test.go`
- Modify: `openapi/spec.go`

- [ ] **Step 1: Write failing tests** for exact replay, conflicting payload key reuse, different-principal/scope independence, and no duplicate raw event/provenance.
- [ ] **Step 2: Run targeted memory, repository, and HTTP tests** and confirm replay behavior is absent.
- [ ] **Step 3: Add normalized request fingerprinting and a transactional repository ingest-idempotency method** that claims, writes event/provenance, and completes result atomically.
- [ ] **Step 4: Require `Idempotency-Key` on public event HTTP writes** and map conflict/in-progress outcomes to safe HTTP responses and OpenAPI.
- [ ] **Step 5: Run targeted tests** and confirm replay and conflict behavior passes.

### Task 7: Observability, Docs, And Verification

**Files:**
- Modify: `internal/telemetry/metrics.go`
- Modify: `internal/telemetry/metrics_test.go`
- Modify: `docs/self-hosting.md`
- Modify: `docs/self_hosting_test.go`
- Modify: `openspec/changes/scoped-principal-auth-and-ingest-idempotency/tasks.md`

- [ ] **Step 1: Write failing telemetry/docs tests** for bounded authorization/idempotency signals and absence of secret/scope/value leakage.
- [ ] **Step 2: Implement bounded metrics/logging and operator documentation** for first principal, rotation, grants, and event retries.
- [ ] **Step 3: Run targeted package tests, `go test ./... -count=1`, `openspec validate scoped-principal-auth-and-ingest-idempotency --strict`, and `git diff --check`.**
- [ ] **Step 4: Mark each completed OpenSpec task** only after its associated tests pass.
