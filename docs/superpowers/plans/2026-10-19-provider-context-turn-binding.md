# Provider Context Turn Binding Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add optional, server-verified `turn_id` to Provider Context reads without widening authorization or changing turnless behavior.

**Architecture:** Provider turn creation writes immutable runtime-binding ownership to its session-turn row. Context reads authorize that ownership before all retrieval work. A verified turn flows only into response attribution, v2 digest/canonical bytes, continuity fingerprint/receipt, and bounded audit metadata; no turnless v1 byte or receipt behavior changes.

**Tech Stack:** Go, net/http, PostgreSQL/pgx, OpenAPI, existing Provider PC4 continuity and PC6 projection contracts.

---

## File structure

- `internal/memory/scope_proof_session.go` and `memory_session_service.go`: server-owned per-turn binding attribution, never accepted from JSON.
- `internal/provider/context_turn.go`: exact-turn authorizer and attribution store contracts; `context*.go`, `disclosure_context.go`, and `capability.go`: wire field, response, v2 identity, receipt validation, budget, and discovery.
- `internal/storage/postgres/scope_proof_session_repository.go`, `context_continuity.go`, and new `provider_context_turn.go`: atomic turn ownership, receipt attribution, exact authorization, and audit storage.
- `internal/app/provider_http.go` and `provider_context_continuity.go`: authorize before retrieval and persist successful attribution.
- `openapi/provider_context.go`, `openapi/spec.go`, Provider docs, README, and Agent Skill: externally visible contract and migration operations.

### Task 1: Make Provider-created turns carry immutable trusted binding ownership

**Files:**
- Modify: `internal/memory/scope_proof_session.go`
- Modify: `internal/memory/memory_session_service.go`
- Modify: `internal/provider/operation.go`
- Create: `internal/provider/context_turn.go`
- Test: `internal/memory/memory_session_service_test.go`
- Test: `internal/provider/operation_test.go`
- Test: `internal/provider/context_turn_test.go`

- [ ] **Step 1: Write the failing ownership tests.**

```go
func TestAdapterCreateTurnCarriesOnlyTrustedBinding(t *testing.T) {
    adapter, session, binding := fixtureAdapterWithSession()
    _, err := adapter.CreateTurn(context.Background(), binding, validMetadata(), memory.CreateMemorySessionTurnInput{SessionID: binding.SessionID, Query: "q"})
    if err != nil { t.Fatal(err) }
    if got := session.input.ProviderBinding; got == nil || got.BindingID != binding.BindingID || got.PrincipalID != binding.PrincipalID || got.SessionID != binding.SessionID { t.Fatalf("binding = %+v", got) }
}
```

- [ ] **Step 2: Run red.** Run `go test ./internal/memory ./internal/provider -run TestAdapterCreateTurnCarriesOnlyTrustedBinding -count=1`; expect a compile failure because `ProviderBinding` is absent.

- [ ] **Step 3: Implement the minimum types and propagation.** Add `memory.RuntimeBindingAttribution` with binding/principal/scope/agent/session/conversation/provider-instance values and validation. Add it as `ProviderBinding *RuntimeBindingAttribution \`json:"-"\`` on the create input and created turn. In `Adapter.CreateTurn`, populate it solely from the validated `provider.RuntimeBinding`; in `MemorySessionService.CreateTurn`, copy it to the new immutable turn. Define `provider.ContextTurnAuthorizer`, `ContextTurnAttributionStore`, `ContextTurnAttribution`, and `ErrContextTurnUnavailable`. Keep generic session requests unbound.

- [ ] **Step 4: Run green.** Run `go test ./internal/memory ./internal/provider -count=1`; expect exit 0.

- [ ] **Step 5: Commit.** Run `git add internal/memory/scope_proof_session.go internal/memory/memory_session_service.go internal/memory/memory_session_service_test.go internal/provider/operation.go internal/provider/operation_test.go internal/provider/context_turn.go internal/provider/context_turn_test.go; git commit -m "feat(provider): bind provider turns to runtime identity"`.

### Task 2: Add strict turn input, response echo, v2 digest and fingerprint

**Files:**
- Modify: `internal/provider/context.go`
- Modify: `internal/provider/disclosure_context.go`
- Modify: `internal/provider/context_budget.go`
- Modify: `internal/provider/context_identity.go`
- Modify: `internal/provider/context_continuity.go`
- Modify: `internal/provider/capability.go`
- Test: `internal/provider/context_test.go`
- Test: `internal/provider/context_budget_accounting_test.go`
- Test: `internal/provider/context_identity_test.go`
- Test: `internal/provider/capability_test.go`

- [ ] **Step 1: Write failing wire/identity tests.**

```go
func TestContextTurnIDIsStrict(t *testing.T) {
    for _, body := range []string{`{"turn_id":""}`, `{"turn_id":"a/b"}`, `{"turn_id":null}`} {
        if _, err := DecodeContextInput([]byte(body)); err == nil { t.Fatalf("accepted %s", body) }
    }
}
func TestContextTurnChangesV2DigestAndFingerprint(t *testing.T) {
    legacy := fixtureContextResponse()
    bound := legacy; bound.Result.TurnID = "turn_1"
    digest, _ := ComputeContextDigest(scope, bound, fingerprint)
    if digest.ContractVersion != ContextTurnDigestVersion || digest.Value == legacy.Result.ContextDigest.Value { t.Fatal("turn identity not bound") }
}
```

- [ ] **Step 2: Run red.** Run `go test ./internal/provider -run 'TestContextTurnIDIsStrict|TestContextTurnChangesV2DigestAndFingerprint' -count=1`; expect failure for missing turn support.

- [ ] **Step 3: Implement identity without changing v1.** Add optional `ContextInput.TurnID`, strict `boundedToken(id, 128)` validation without trimming, and optional `DisclosedContextResult.TurnID`. Carry only an already-authorized turn into `ContextAdmission` before preflight, so the field counts in exact response bytes. Keep `CanonicalContextBytes` and its domain separator byte-for-byte unchanged for empty turn IDs. For a nonempty ID, marshal an explicit fixed-order structure that adds `turn_id`, use `context-turn-digest-v1`, `context-turn-canonical-v1`, and `stele-context-turn-digest-v1\n`. Include `turn_id` only when present in `ContextReadFingerprint`. Accept only exact v1 or exact v2 tuples in receipt validation. Expose disabled-by-default `context_turn_binding` capability with exact contract/digest versions.

- [ ] **Step 4: Add golden and budget cases.** Add empty/nonempty v2 canonical JSON and hash fixtures; test both response modes, 128-byte success, 129-byte rejection, response-byte exhaustion caused by echo, and identical disclosure/selection/citations when only turn changes.

- [ ] **Step 5: Run green and commit.** Run `go test ./internal/provider -count=1`; then run `git add internal/provider; git commit -m "feat(provider): add turn-bound context identity"`.

### Task 3: Add PostgreSQL ownership, receipt, and attribution persistence

**Files:**
- Create: `internal/storage/postgres/migrations/0038_provider_context_turn_binding.up.sql`
- Create: `internal/storage/postgres/migrations/0038_provider_context_turn_binding.down.sql`
- Modify: `internal/storage/postgres/scope_proof_session_repository.go`
- Modify: `internal/storage/postgres/context_continuity.go`
- Create: `internal/storage/postgres/provider_context_turn.go`
- Test: `internal/storage/postgres/scope_proof_session_repository_test.go`
- Test: `internal/storage/postgres/context_continuity_test.go`
- Create: `internal/storage/postgres/provider_context_turn_test.go`
- Create: `internal/storage/postgres/provider_context_turn_integration_test.go`

- [ ] **Step 1: Write failing exact-authority and transaction tests.**

```go
func TestAuthorizeContextTurnDoesNotDiscloseMissingOrForeign(t *testing.T) {
    for _, id := range []string{"missing", "foreign", "unbound"} {
        if err := repo.AuthorizeContextTurn(ctx, binding, id); !errors.Is(err, provider.ErrContextTurnUnavailable) { t.Fatalf("%s: %v", id, err) }
    }
}
func TestTurnReceiptRollbackLeavesNoReceiptOrAttribution(t *testing.T) {
    _, err := repo.IssueContextReceipt(ctx, binding, fp, digest, "turn_1", attr, cfg, func(provider.ContextReceipt) error { return errStop })
    if !errors.Is(err, errStop) { t.Fatal(err) }
    assertCount(t, pool, "provider_context_receipts", 0); assertCount(t, pool, "provider_context_turn_attributions", 0)
}
```

- [ ] **Step 2: Run red.** Run `go test ./internal/storage/postgres -run 'TestAuthorizeContextTurnDoesNotDiscloseMissingOrForeign|TestTurnReceiptRollbackLeavesNoReceiptOrAttribution' -count=1`; expect missing methods/schema.

- [ ] **Step 3: Create additive migration.** Add nullable full binding identity columns to `memory_session_turns` with an all-null-or-all-present CHECK, nullable `turn_id` to `provider_context_receipts`, and append-only `provider_context_turn_attributions`. The audit table contains only opaque ID, binding/turn association reference, exact scope, request/operation IDs, digest version/value, optional receipt reference, and time; it must not contain query/content/selected memory/citation/ranking columns. Add exact lookup indexes. The down migration must deliberately refuse destructive removal.

- [ ] **Step 4: Implement exact, atomic persistence.** Extend the session-turn INSERT/RETURNING query to persist server binding fields atomically. Its idempotency conflict succeeds only when all persisted binding values match. Implement an exact `SELECT EXISTS` over binding ID, principal, all scope dimensions, agent/session/conversation/provider-instance and turn; map any false to only `ErrContextTurnUnavailable`. Extend PC4 receipt read/write/dedupe to carry `TurnID`; issue/reuse receipt and its attribution in one transaction. Provide `RecordContextTurnAttribution` for no-continuity reads after final byte sizing.

- [ ] **Step 5: Add integration coverage.** Test two bindings sharing a session, every foreign scope dimension, wrong session/instance, legacy unbound turn, same-turn receipt dedupe, different-turn receipt separation, restart lookup, and rollback. Assert no audit column/row has prohibited payload or ranking data.

- [ ] **Step 6: Run green and commit.** Run `go test ./internal/storage/postgres -count=1`; then run `git add internal/storage/postgres; git commit -m "feat(provider): persist context turn attribution"`.

### Task 4: Enforce authorization before read and retain PC6 receipt integrity

**Files:**
- Modify: `internal/app/http.go`
- Modify: `internal/app/app.go`
- Modify: `internal/app/provider_http.go`
- Modify: `internal/app/provider_context_continuity.go`
- Modify: `internal/storage/postgres/provider_projection.go`
- Test: `internal/app/provider_context_test.go`
- Test: `internal/app/provider_context_digest_test.go`
- Test: `internal/app/provider_http_test.go`
- Test: `internal/storage/postgres/provider_projection_attribution_integration_test.go`

- [ ] **Step 1: Write failing handler sequencing tests.**

```go
func TestContextRejectsForeignAndMissingTurnBeforeRead(t *testing.T) {
    for _, id := range []string{"foreign", "missing"} {
        got := callContext(t, depsWithTurnError(provider.ErrContextTurnUnavailable), `{"query":"q","budget":1,"turn_id":"`+id+`"}`)
        if got.Code != http.StatusForbidden || got.Body.String() != deniedBody { t.Fatalf("%s: %d %s", id, got.Code, got.Body.String()) }
        if adapter.calls != 0 || receipts.lookupCalls != 0 { t.Fatal("read before authorization") }
    }
}
```

- [ ] **Step 2: Run red.** Run `go test ./internal/app -run TestContextRejectsForeignAndMissingTurnBeforeRead -count=1`; expect dependencies/handler behavior missing.

- [ ] **Step 3: Wire the narrow authority.** Add turn authorizer/attribution store dependencies, wire PostgreSQL in app construction, and advertise capability only when both are present. Immediately after strict decode/schema validation, authorize a nonempty turn before `RetrievalInput`, disclosure, continuity, or assembly. Return one fixed 403 `scope/context_turn_denied` for unavailable/missing/foreign/wrong-binding turns, and retryable 503 for storage failure. Pass verified ID only to admission/continuity/audit; never add it to retrieval input or disclosure resolution. For no continuity, record attribution after `Finalize` and before success; for continuity, use receipt transaction. Require PC6 `ProjectionInput.TurnID` equal a receipt `TurnID` before `verified_receipt`; legacy empty receipt remains compatible.

- [ ] **Step 4: Add regressions.** Test absent authority fails closed only for turn-bound reads; wrong binding/session/scope has identical denial; turnless stays byte-identical; compare mismatches in both directions; same turn compares; write failure produces no 200; PC6 missing/mismatched turns cannot verify receipt.

- [ ] **Step 5: Run green and commit.** Run `go test ./internal/app ./internal/assurance -count=1`; then run `git add internal/app internal/storage/postgres/provider_projection.go internal/storage/postgres/provider_projection_attribution_integration_test.go; git commit -m "feat(provider): authorize turn-bound context reads"`.

### Task 5: Publish OpenAPI and maintenance documentation

**Files:**
- Modify: `openapi/provider_context.go`
- Modify: `openapi/spec.go`
- Modify: `openapi/openapi_test.go`
- Modify: `openapi/provider_context_test.go`
- Modify: `docs/agent-runtime-memory-provider.md`
- Modify: `docs/provider-evidence-projections.md`
- Modify: `README.md`
- Modify: `skills/stele-memory/SKILL.md`
- Test: `docs/provider_context_test.go`

- [ ] **Step 1: Write failing contract tests.** Assert served OpenAPI accepts only token-shaped optional `input.turn_id`, exposes optional result echo and exact v1/v2 digest/capability alternatives, and documentation states same-binding creation, no widening, no snapshot/role/token/business authority, errors, migration, and PC6 mismatch handling.

- [ ] **Step 2: Run red.** Run `go test ./openapi ./docs -run 'TestProviderContext|TestOpenAPI' -count=1`; expect missing schema/docs assertions.

- [ ] **Step 3: Implement public contract.** Update OpenAPI source and generated spec via established repository mechanism. Document capability negotiation, create-turn-then-context sequence, JSON/error examples, exact validator, v1/v2 digest vectors, fresh-only receipt semantics, 0038 migration, no legacy claiming, mixed deployment, audit retention, and rollback. Update README and Agent Skill concisely without adding product state or UI.

- [ ] **Step 4: Run green and commit.** Run `go test ./openapi ./docs -count=1; pwsh -NoProfile -File scripts/check-documentation.ps1`; then run `git add openapi docs README.md skills/stele-memory/SKILL.md; git commit -m "docs(provider): publish context turn binding contract"`.

### Task 6: Full verification and artifact reconciliation

**Files:**
- Modify: `docs/superpowers/plans/2026-10-19-provider-context-turn-binding.md`
- Modify: matching OpenSpec `tasks.md` only if an approved STL-1 change exists

- [ ] **Step 1: Run focused matrix.** Run `go test ./internal/provider ./internal/app ./internal/storage/postgres ./internal/assurance ./openapi ./docs -count=1`; retain output.

- [ ] **Step 2: Run whole-repository verification.** Run `gofmt -w <modified-go-files>; git diff --check; go vet ./...; go test ./... -count=1; openspec validate --all --strict`. Report unavailable Docker/PostgreSQL/pgvector live verification rather than claiming it ran.

- [ ] **Step 3: Reconcile evidence.** Re-read [the approved design](../specs/2026-10-19-provider-context-turn-binding-design.md) against the diff, mark only verified checkboxes, and commit the plan/task artifacts with `git add docs/superpowers/plans/2026-10-19-provider-context-turn-binding.md; git commit -m "test(provider): verify turn-bound context contract"`.
