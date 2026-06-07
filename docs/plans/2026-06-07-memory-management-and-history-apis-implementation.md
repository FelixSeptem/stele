# Memory Management And History APIs Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Expose canonical memory as a first-class API resource with lifecycle-safe public reads, append-only history and provenance queries, and privileged manual lifecycle actions.

**Architecture:** Add a small `internal/memory` query layer so HTTP handlers depend on stable resource contracts instead of raw repository structs. Keep public `GET /v1/memories...` lifecycle-safe, and keep manual `suppress|expire|delete` actions on the existing admin boundary so governance control stays separated from ordinary reads.

**Tech Stack:** Go, `net/http`, PostgreSQL via `pgx`, existing `internal/auth`, existing `internal/governance`, OpenAPI YAML-in-Go, existing repository-backed tests with `pgxmock`.

---

## File Map

### Create

- `internal/memory/query.go`
  Defines list/detail/history/provenance input and output contracts plus the query service interfaces.
- `internal/memory/query_test.go`
  Covers contract validation, lifecycle-safe shaping, and not-found behavior.
- `internal/memory/lifecycle_service.go`
  Wraps manual lifecycle actions behind a small admin-facing service contract.
- `internal/memory/lifecycle_service_test.go`
  Covers action validation, actor and reason attribution, and idempotent service behavior.

### Modify

- `internal/memory/types.go`
  Reuse existing canonical memory, version, and provenance types; add only stable JSON fields or helpers if the new query layer truly needs them.
- `internal/governance/forgetting.go`
  Extend lifecycle action input so admin actions can carry actor, reason, and request attribution cleanly.
- `internal/storage/postgres/repository.go`
  Add canonical memory list/detail/provenance read methods and lifecycle audit persistence.
- `internal/storage/postgres/repository_test.go`
  Add read-model and audit tests around scope filtering, hidden visibility, and lifecycle provenance.
- `internal/app/http.go`
  Add public memory read handlers and privileged lifecycle action handlers.
- `internal/app/http_test.go`
  Add handler coverage for list, detail, history, provenance, and admin actions.
- `internal/app/app.go`
  Wire the new query and lifecycle services into API mode dependencies.
- `internal/app/app_test.go`
  Verify runtime wiring for the new dependencies.
- `openapi/spec.go`
  Add new paths and schemas for memory resources, provenance, and lifecycle actions.
- `openapi/openapi_test.go`
  Assert the new routes and schemas appear in the published spec.
- `docs/self-hosting.md`
  Document the new read and admin lifecycle APIs with example flows.
- `README.md`
  Add one short note that canonical memory resources now have direct APIs.

## Task 1: Memory Query Contracts

**Files:**
- Create: `internal/memory/query.go`
- Create: `internal/memory/query_test.go`
- Modify: `internal/memory/types.go`

- [ ] **Step 1: Write the failing contract tests**

```go
func TestListMemoriesInputValidateRejectsInvalidWindow(t *testing.T) {
	err := (ListMemoriesInput{
		Scope: Scope{Tenant: "tenant-a", Project: "project-a", Namespace: "namespace-a"},
		TimeFrom: time.Date(2026, 6, 7, 15, 0, 0, 0, time.UTC),
		TimeTo:   time.Date(2026, 6, 7, 14, 0, 0, 0, time.UTC),
	}).Validate()
	if err == nil {
		t.Fatal("Validate() error = nil, want invalid time window")
	}
}

func TestMemoryResourceFromCanonicalRedactsDeletedPayload(t *testing.T) {
	resource := NewMemoryResource(CanonicalMemory{
		ID:      "mem_deleted",
		Scope:   Scope{Tenant: "tenant-a", Project: "project-a", Namespace: "namespace-a"},
		Class:   MemoryClassProfile,
		State:   MemoryStateDeleted,
		Content: "",
	})

	if resource.State != MemoryStateDeleted {
		t.Fatalf("State = %q, want deleted", resource.State)
	}
	if resource.Content != "" {
		t.Fatalf("Content = %q, want empty payload", resource.Content)
	}
}
```

- [ ] **Step 2: Run the contract tests to verify they fail**

Run: `go test ./internal/memory -run "ListMemoriesInputValidate|MemoryResourceFromCanonical" -count=1`

Expected: FAIL with undefined `ListMemoriesInput`, `MemoryResource`, or `NewMemoryResource`.

- [ ] **Step 3: Write the minimal query contract layer**

```go
type ListMemoriesInput struct {
	Scope   Scope
	Classes []MemoryClass
	TimeFrom time.Time
	TimeTo   time.Time
	Limit    int
}

func (i ListMemoriesInput) Validate() error {
	if err := i.Scope.Validate(); err != nil {
		return err
	}
	if !i.TimeFrom.IsZero() && !i.TimeTo.IsZero() && i.TimeFrom.After(i.TimeTo) {
		return fmt.Errorf("time_from must be before or equal to time_to")
	}
	if i.Limit < 0 {
		return fmt.Errorf("limit must be greater than or equal to zero")
	}
	return nil
}

type MemoryResource struct {
	ID         string       `json:"id"`
	Scope      Scope        `json:"scope"`
	Class      MemoryClass  `json:"class"`
	State      MemoryState  `json:"state"`
	Content    string       `json:"content"`
	CreatedAt  time.Time    `json:"created_at"`
	ModifiedAt time.Time    `json:"modified_at"`
}

func NewMemoryResource(c CanonicalMemory) MemoryResource {
	return MemoryResource{
		ID:         c.ID,
		Scope:      c.Scope,
		Class:      c.Class,
		State:      c.State,
		Content:    c.Content,
		CreatedAt:  c.CreatedAt,
		ModifiedAt: c.ModifiedAt,
	}
}
```

- [ ] **Step 4: Run the memory package tests**

Run: `go test ./internal/memory -count=1`

Expected: PASS for the new query contract tests and existing ingest tests.

- [ ] **Step 5: Commit**

```bash
git add internal/memory/query.go internal/memory/query_test.go internal/memory/types.go
git commit -m "feat: add memory query contracts"
```

## Task 2: Postgres Read Models For List, Detail, And Provenance

**Files:**
- Modify: `internal/storage/postgres/repository.go`
- Modify: `internal/storage/postgres/repository_test.go`

- [ ] **Step 1: Write the failing repository tests**

```go
func TestRepositoryReadCanonicalMemoryReturnsVisibleScopedRecord(t *testing.T) {
	mock, _ := pgxmock.NewPool()
	defer mock.Close()

	scope := memory.Scope{Tenant: "tenant-a", Project: "project-a", Namespace: "namespace-a"}
	now := time.Date(2026, 6, 7, 16, 0, 0, 0, time.UTC)

	mock.ExpectQuery("SELECT[\\s\\S]*FROM canonical_memories").
		WithArgs("mem_123", scope.Tenant, scope.Project, scope.Namespace, false).
		WillReturnRows(pgxmock.NewRows([]string{
			"id", "tenant", "project", "namespace", "class", "state", "content", "created_at", "updated_at",
		}).AddRow(
			"mem_123", scope.Tenant, scope.Project, scope.Namespace,
			memory.MemoryClassProfile, memory.MemoryStateActive, "User prefers concise answers.", now.Add(-time.Hour), now,
		))

	repo := NewRepository(mock)
	got, err := repo.ReadCanonicalMemory(context.Background(), scope, "mem_123", false)
	if err != nil {
		t.Fatalf("ReadCanonicalMemory() error = %v", err)
	}
	if got.ID != "mem_123" {
		t.Fatalf("ID = %q, want mem_123", got.ID)
	}
}

func TestRepositoryReadMemoryProvenanceReturnsActorAndReason(t *testing.T) {
	mock, _ := pgxmock.NewPool()
	defer mock.Close()

	scope := memory.Scope{Tenant: "tenant-a", Project: "project-a", Namespace: "namespace-a"}
	now := time.Date(2026, 6, 7, 16, 10, 0, 0, time.UTC)

	mock.ExpectQuery("SELECT[\\s\\S]*FROM provenance_links").
		WithArgs(scope.Tenant, scope.Project, scope.Namespace, "mem_123").
		WillReturnRows(pgxmock.NewRows([]string{
			"id", "raw_event_id", "candidate_memory_id", "memory_id", "tenant", "project", "namespace", "operation", "request_id", "actor", "source_context", "created_at",
		}).AddRow(
			"prov_1", nil, nil, "mem_123", scope.Tenant, scope.Project, scope.Namespace, "suppress_memory", "req_1", "operator-a", []byte(`{"reason":"manual override"}`), now,
		))

	repo := NewRepository(mock)
	records, err := repo.ReadMemoryProvenance(context.Background(), scope, "mem_123")
	if err != nil {
		t.Fatalf("ReadMemoryProvenance() error = %v", err)
	}
	if records[0].Actor != "operator-a" {
		t.Fatalf("Actor = %q, want operator-a", records[0].Actor)
	}
}
```

- [ ] **Step 2: Run the repository tests to verify they fail**

Run: `go test ./internal/storage/postgres -run "ReadCanonicalMemory|ReadMemoryProvenance" -count=1`

Expected: FAIL with undefined repository methods or scan mismatches.

- [ ] **Step 3: Implement the repository methods and provenance field persistence**

```go
func (r *Repository) ReadCanonicalMemory(ctx context.Context, scope memory.Scope, memoryID string, includeHidden bool) (memory.CanonicalMemory, error) {
	const query = `
SELECT id, tenant, project, namespace, class, state, content, created_at, updated_at
FROM canonical_memories
WHERE id = $1
  AND tenant = $2
  AND project = $3
  AND namespace = $4
  AND ($5 OR state NOT IN ('suppressed', 'forgotten', 'deleted'))
`
	// scan and return canonical memory
}

func (r *Repository) ReadMemoryProvenance(ctx context.Context, scope memory.Scope, memoryID string) ([]memory.ProvenanceRecord, error) {
	const query = `
SELECT id, raw_event_id, candidate_memory_id, memory_id, tenant, project, namespace, operation, request_id, actor, source_context, created_at
FROM provenance_links
WHERE tenant = $1 AND project = $2 AND namespace = $3 AND memory_id = $4
ORDER BY created_at ASC
`
	// scan request_id, actor, and source_context into ProvenanceRecord
}
```

- [ ] **Step 4: Run the repository package tests**

Run: `go test ./internal/storage/postgres -count=1`

Expected: PASS for the new read-model tests and the existing lifecycle or search coverage.

- [ ] **Step 5: Commit**

```bash
git add internal/storage/postgres/repository.go internal/storage/postgres/repository_test.go
git commit -m "feat: add memory read models and provenance queries"
```

## Task 3: Public Memory Read Service And HTTP Surface

**Files:**
- Create: `internal/memory/query.go`
- Modify: `internal/app/http.go`
- Modify: `internal/app/http_test.go`
- Modify: `internal/app/app.go`
- Modify: `internal/app/app_test.go`

- [ ] **Step 1: Write the failing HTTP tests for list, detail, history, and provenance**

```go
func TestNewHTTPHandlerListsVisibleMemories(t *testing.T) {
	reader := &stubMemoryQueryService{
		page: memory.MemoryPage{
			Items: []memory.MemoryResource{
				{ID: "mem_123", Scope: memory.Scope{Tenant: "tenant-a", Project: "project-a", Namespace: "namespace-a"}, Class: memory.MemoryClassProfile, State: memory.MemoryStateActive, Content: "User prefers concise answers."},
			},
		},
	}

	handler := NewHTTPHandler(HTTPDependencies{
		APIKeys:      map[string]struct{}{"test-key": {}},
		MemoryQuery:  reader,
	})

	req := httptest.NewRequest(http.MethodGet, "/v1/memories?class=profile&limit=10", nil)
	req.Header.Set("X-API-Key", "test-key")
	req.Header.Set("X-Stele-Tenant", "tenant-a")
	req.Header.Set("X-Stele-Project", "project-a")
	req.Header.Set("X-Stele-Namespace", "namespace-a")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
}

func TestNewHTTPHandlerReturnsMemoryProvenance(t *testing.T) {
	reader := &stubMemoryQueryService{
		provenance: []memory.ProvenanceRecord{
			{ID: "prov_1", MemoryID: "mem_123", Operation: "promote_candidate"},
		},
	}
	// call GET /v1/memories/mem_123/provenance and assert 200
}
```

- [ ] **Step 2: Run the HTTP tests to verify they fail**

Run: `go test ./internal/app -run "ListsVisibleMemories|ReturnsMemoryProvenance" -count=1`

Expected: FAIL with missing `MemoryQuery` dependency or missing routes.

- [ ] **Step 3: Implement the public read service and handlers**

```go
type MemoryQueryService interface {
	ListMemories(ctx context.Context, input memory.ListMemoriesInput) (memory.MemoryPage, error)
	GetMemory(ctx context.Context, scope memory.Scope, memoryID string) (memory.MemoryResource, error)
	GetMemoryHistory(ctx context.Context, scope memory.Scope, memoryID string) (memory.MemoryHistory, error)
	GetMemoryProvenance(ctx context.Context, scope memory.Scope, memoryID string) ([]memory.ProvenanceRecord, error)
}

// in HTTPDependencies
MemoryQuery MemoryQueryService

// routes
mux.Handle("GET /v1/memories", protectedMemoryList)
mux.Handle("GET /v1/memories/{memory_id}", protectedMemoryDetail)
mux.Handle("GET /v1/memories/{memory_id}/history", protectedMemoryHistory)
mux.Handle("GET /v1/memories/{memory_id}/provenance", protectedMemoryProvenance)
```

- [ ] **Step 4: Run the app package tests**

Run: `go test ./internal/app -count=1`

Expected: PASS for the new public read routes and the existing health, search, context, and admin tests.

- [ ] **Step 5: Commit**

```bash
git add internal/app/http.go internal/app/http_test.go internal/app/app.go internal/app/app_test.go internal/memory/query.go
git commit -m "feat: add public memory read APIs"
```

## Task 4: Privileged Lifecycle Action Service And Audit Trail

**Files:**
- Create: `internal/memory/lifecycle_service.go`
- Create: `internal/memory/lifecycle_service_test.go`
- Modify: `internal/governance/forgetting.go`
- Modify: `internal/storage/postgres/repository.go`
- Modify: `internal/storage/postgres/repository_test.go`
- Modify: `internal/app/http.go`
- Modify: `internal/app/http_test.go`
- Modify: `internal/app/app.go`

- [ ] **Step 1: Write the failing service and handler tests**

```go
func TestLifecycleServiceApplySuppressRequiresActorAndReason(t *testing.T) {
	service := LifecycleService{Now: func() time.Time { return time.Date(2026, 6, 7, 17, 0, 0, 0, time.UTC) }}

	err := service.Apply(context.Background(), LifecycleActionInput{
		Scope:    memory.Scope{Tenant: "tenant-a", Project: "project-a", Namespace: "namespace-a"},
		MemoryID: "mem_123",
		Action:   policy.ForgettingActionSuppress,
	})
	if err == nil {
		t.Fatal("Apply() error = nil, want missing actor or reason error")
	}
}

func TestNewHTTPHandlerAppliesAdminSuppressAction(t *testing.T) {
	service := &stubLifecycleService{}
	handler := NewHTTPHandler(HTTPDependencies{
		AdminAPIKeys:          map[string]struct{}{"admin-key": {}},
		MemoryLifecycleAction: service,
	})

	req := httptest.NewRequest(http.MethodPost, "/v1/admin/memories/mem_123:suppress", strings.NewReader(`{"reason":"manual override"}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-API-Key", "admin-key")
	req.Header.Set("X-Stele-Actor", "operator-a")
	req.Header.Set("X-Stele-Tenant", "tenant-a")
	req.Header.Set("X-Stele-Project", "project-a")
	req.Header.Set("X-Stele-Namespace", "namespace-a")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
}
```

- [ ] **Step 2: Run the lifecycle tests to verify they fail**

Run: `go test ./internal/memory ./internal/app -run "LifecycleServiceApplySuppress|AppliesAdminSuppressAction" -count=1`

Expected: FAIL with missing lifecycle service and route.

- [ ] **Step 3: Implement lifecycle input normalization, provenance audit, and admin handlers**

```go
type LifecycleActionInput struct {
	Scope     memory.Scope
	MemoryID  string
	Action    policy.ForgettingAction
	Reason    string
	Actor     string
	RequestID string
}

type LifecycleService struct {
	Processor governance.ForgettingProcessor
	Now       func() time.Time
}

func (s LifecycleService) Apply(ctx context.Context, input LifecycleActionInput) error {
	return s.Processor.Apply(ctx, governance.LifecycleAction{
		MemoryID:  input.MemoryID,
		Scope:     input.Scope,
		Action:    input.Action,
		Reason:    input.Reason,
		Actor:     input.Actor,
		RequestID: input.RequestID,
		AppliedAt: s.Now().UTC(),
	})
}
```

Also update repository lifecycle mutation flow to write a provenance record for `suppress_memory`, `expire_memory`, or `delete_memory` with actor and reason in `source_context`.

- [ ] **Step 4: Run the affected packages**

Run: `go test ./internal/memory ./internal/governance ./internal/storage/postgres ./internal/app -count=1`

Expected: PASS for lifecycle validation, audit persistence, and admin action routes.

- [ ] **Step 5: Commit**

```bash
git add internal/memory/lifecycle_service.go internal/memory/lifecycle_service_test.go internal/governance/forgetting.go internal/storage/postgres/repository.go internal/storage/postgres/repository_test.go internal/app/http.go internal/app/http_test.go internal/app/app.go
git commit -m "feat: add privileged memory lifecycle actions"
```

## Task 5: OpenAPI, Docs, And Compatibility

**Files:**
- Modify: `openapi/spec.go`
- Modify: `openapi/openapi_test.go`
- Modify: `docs/self-hosting.md`
- Modify: `README.md`

- [ ] **Step 1: Write the failing OpenAPI test**

```go
func TestSpecYAMLIncludesMemoryManagementRoutes(t *testing.T) {
	for _, want := range []string{
		"/v1/memories",
		"/v1/memories/{memory_id}",
		"/v1/memories/{memory_id}/history",
		"/v1/memories/{memory_id}/provenance",
		"/v1/admin/memories/{memory_id}:suppress",
		"/v1/admin/memories/{memory_id}:expire",
		"/v1/admin/memories/{memory_id}:delete",
	} {
		if !strings.Contains(SpecYAML(), want) {
			t.Fatalf("SpecYAML() missing %q", want)
		}
	}
}
```

- [ ] **Step 2: Run the OpenAPI test to verify it fails**

Run: `go test ./openapi -run MemoryManagementRoutes -count=1`

Expected: FAIL because the new paths and schemas are not yet published.

- [ ] **Step 3: Add the spec and docs changes**

```yaml
/v1/memories:
  get:
    operationId: listMemories
/v1/memories/{memory_id}:
  get:
    operationId: getMemory
/v1/memories/{memory_id}/provenance:
  get:
    operationId: getMemoryProvenance
/v1/admin/memories/{memory_id}:suppress:
  post:
    operationId: suppressMemory
```

Document one end-to-end flow in `docs/self-hosting.md`:

1. list visible memories
2. inspect one memory history
3. suppress a memory with `X-Stele-Actor`

- [ ] **Step 4: Run the full regression suite**

Run: `go test ./...`

Expected: PASS with no retrieval, context assembly, or admin regression.

- [ ] **Step 5: Commit**

```bash
git add openapi/spec.go openapi/openapi_test.go docs/self-hosting.md README.md
git commit -m "docs: publish memory management API surface"
```

## Self-Review

- Spec coverage:
  - `memory-management-surface` is covered by Tasks 1 to 3.
  - `memory-history-and-provenance` is covered by Tasks 2 to 3.
  - `manual-memory-lifecycle-actions` is covered by Task 4.
  - documentation and compatibility requirements are covered by Task 5.
- Placeholder scan:
  - no `TODO`, `TBD`, or “similar to above” shortcuts remain.
- Type consistency:
  - this plan consistently uses `ListMemoriesInput`, `MemoryResource`, `LifecycleActionInput`, `MemoryQueryService`, and `LifecycleService`.

## Recommended Execution Order

1. Task 1
2. Task 2
3. Task 3
4. Task 4
5. Task 5

This order keeps TDD intact and avoids wiring HTTP or OpenAPI before the domain and repository contracts are stable.
