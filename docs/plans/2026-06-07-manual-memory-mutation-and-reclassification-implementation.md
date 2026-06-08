# Manual Memory Mutation And Reclassification Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add a privileged admin mutation surface for canonical memory create, update, merge, and reclassify while preserving append-only history, audit attribution, and retrieval-safe defaults.

**Architecture:** Extend the existing `internal/memory` service boundary with a dedicated manual mutation service that validates bounded admin actions and passes normalized records into PostgreSQL repository transactions. Reuse the current HTTP admin surface, provenance model, and query/read history APIs so manual governance actions naturally appear in public-safe history/provenance while preserving stricter admin-only mutation routes.

**Tech Stack:** Go, net/http, pgx/pgxmock, PostgreSQL, OpenAPI 3.1

---

### Task 1: Domain Mutation Contracts

**Files:**
- Modify: `internal/memory/types.go`
- Create: `internal/memory/manual_mutation.go`
- Create: `internal/memory/manual_mutation_test.go`

- [ ] **Step 1: Write the failing tests**

```go
func TestManualMutationInputValidateRejectsExcludedSummaryCreate(t *testing.T) {
	input := memory.ManualCreateMemoryInput{
		Scope:   memory.Scope{Tenant: "tenant-a", Project: "project-a", Namespace: "namespace-a"},
		Class:   memory.MemoryClassSummary,
		Content: "derived summary",
		Reason:  "seed",
		Actor:   "operator-a",
	}

	if err := input.Validate(); err == nil {
		t.Fatal("Validate() error = nil, want excluded class error")
	}
}

func TestManualMutationServiceNormalizesUpdateRecord(t *testing.T) {
	store := &stubManualMutationProcessor{}
	service := memory.ManualMutationService{
		Processor: store,
		Now:       func() time.Time { return time.Date(2026, 6, 7, 20, 0, 0, 0, time.UTC) },
		NewID:     func() string { return "ver_123" },
	}

	err := service.UpdateMemory(context.Background(), memory.ManualUpdateMemoryInput{
		Scope:           memory.Scope{Tenant: "tenant-a", Project: "project-a", Namespace: "namespace-a"},
		MemoryID:        "mem_123",
		Content:         "corrected",
		ExpectedVersion: 2,
		Reason:          "manual correction",
		Actor:           "operator-a",
	})

	if err != nil {
		t.Fatalf("UpdateMemory() error = %v", err)
	}
	if store.gotUpdate.ExpectedVersion != 2 {
		t.Fatalf("ExpectedVersion = %d, want 2", store.gotUpdate.ExpectedVersion)
	}
}
```

- [ ] **Step 2: Run the focused test file and verify RED**

Run: `go test ./internal/memory -run "ManualMutation" -count=1`
Expected: FAIL with undefined manual mutation types or service methods

- [ ] **Step 3: Write minimal implementation**

```go
type ManualCreateMemoryInput struct {
	Scope     Scope
	Class     MemoryClass
	Content   string
	Reason    string
	Actor     string
	RequestID string
}

type ManualUpdateMemoryInput struct {
	Scope           Scope
	MemoryID        string
	Content         string
	ExpectedVersion int64
	Reason          string
	Actor           string
	RequestID       string
}

type ManualMutationProcessor interface {
	CreateMemory(ctx context.Context, record ManualCreateMemoryRecord) (CanonicalMemory, error)
	UpdateMemory(ctx context.Context, record ManualUpdateMemoryRecord) (CanonicalMemory, error)
	MergeMemory(ctx context.Context, record ManualMergeMemoryRecord) (CanonicalMemory, error)
	ReclassifyMemory(ctx context.Context, record ManualReclassifyMemoryRecord) (CanonicalMemory, error)
}
```

- [ ] **Step 4: Run the focused test file and verify GREEN**

Run: `go test ./internal/memory -run "ManualMutation" -count=1`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/memory/types.go internal/memory/manual_mutation.go internal/memory/manual_mutation_test.go
git commit -m "feat: add manual memory mutation domain contracts"
```

### Task 2: Repository Transactions For Manual Mutation

**Files:**
- Modify: `internal/storage/postgres/repository.go`
- Modify: `internal/storage/postgres/repository_test.go`

- [ ] **Step 1: Write the failing repository tests**

```go
func TestRepositoryCreateMemoryWritesCanonicalVersionAndProvenance(t *testing.T) {
	mock.ExpectBegin()
	mock.ExpectQuery("INSERT INTO canonical_memories")
	mock.ExpectQuery("INSERT INTO memory_versions")
	mock.ExpectExec("INSERT INTO provenance_links")
	mock.ExpectCommit()
}

func TestRepositoryUpdateMemoryRejectsVersionConflict(t *testing.T) {
	mock.ExpectBegin()
	mock.ExpectQuery("SELECT COALESCE\\(MAX\\(version\\), 0\\)")
}

func TestRepositoryMergeMemorySuppressesSourceAndClearsSourceEmbedding(t *testing.T) {
	mock.ExpectBegin()
	mock.ExpectQuery("UPDATE canonical_memories")
	mock.ExpectExec("INSERT INTO provenance_links")
	mock.ExpectCommit()
}
```

- [ ] **Step 2: Run the focused repository tests and verify RED**

Run: `go test ./internal/storage/postgres -run "CreateMemoryWritesCanonicalVersionAndProvenance|UpdateMemoryRejectsVersionConflict|MergeMemorySuppressesSourceAndClearsSourceEmbedding" -count=1`
Expected: FAIL with missing repository methods or unmet query expectations

- [ ] **Step 3: Write minimal repository implementation**

```go
func (r *Repository) CreateMemory(ctx context.Context, record memory.ManualCreateMemoryRecord) (memory.CanonicalMemory, error) {
	// begin tx
	// insert canonical_memories with active state and search_text
	// insert memory_versions version=1
	// insert provenance_links manual_create_memory
}

func (r *Repository) UpdateMemory(ctx context.Context, record memory.ManualUpdateMemoryRecord) (memory.CanonicalMemory, error) {
	// verify expected version
	// update canonical content/search_text/embedding
	// append memory_versions row
	// insert provenance
}
```

- [ ] **Step 4: Run the focused repository tests and verify GREEN**

Run: `go test ./internal/storage/postgres -run "CreateMemoryWritesCanonicalVersionAndProvenance|UpdateMemoryRejectsVersionConflict|MergeMemorySuppressesSourceAndClearsSourceEmbedding" -count=1`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/storage/postgres/repository.go internal/storage/postgres/repository_test.go
git commit -m "feat: add postgres manual memory mutation transactions"
```

### Task 3: Admin HTTP Surface And Runtime Wiring

**Files:**
- Modify: `internal/app/http.go`
- Modify: `internal/app/http_test.go`
- Modify: `internal/app/app.go`

- [ ] **Step 1: Write the failing HTTP tests**

```go
func TestNewHTTPHandlerCreatesAdminMemory(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/v1/admin/memories", strings.NewReader(`{"class":"profile","content":"seed","reason":"seed"}`))
	req.Header.Set("X-API-Key", "admin-key")
	req.Header.Set("X-Stele-Actor", "operator-a")
}

func TestNewHTTPHandlerMergesAdminMemory(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/v1/admin/memories/mem_target:merge", strings.NewReader(`{"source_memory_id":"mem_source","content":"merged","expected_version":2,"reason":"dedupe"}`))
}
```

- [ ] **Step 2: Run the focused HTTP tests and verify RED**

Run: `go test ./internal/app -run "CreatesAdminMemory|MergesAdminMemory|ReclassifiesAdminMemory" -count=1`
Expected: FAIL with missing routes, request structs, or service interfaces

- [ ] **Step 3: Write minimal handler and wiring implementation**

```go
type ManualMemoryMutationService interface {
	CreateMemory(ctx context.Context, input memory.ManualCreateMemoryInput) (memory.MemoryResource, error)
	UpdateMemory(ctx context.Context, input memory.ManualUpdateMemoryInput) (memory.MemoryResource, error)
	MergeMemory(ctx context.Context, input memory.ManualMergeMemoryInput) (memory.MemoryResource, error)
	ReclassifyMemory(ctx context.Context, input memory.ManualReclassifyMemoryInput) (memory.MemoryResource, error)
}

mux.Handle("POST /v1/admin/memories", adminMemoryMutation)
mux.Handle("PATCH /v1/admin/memories/{memory_id}", adminMemoryMutation)
mux.Handle("POST /v1/admin/memories/{memory_action}", adminMemoryMutation)
```

- [ ] **Step 4: Run the focused HTTP tests and verify GREEN**

Run: `go test ./internal/app -run "CreatesAdminMemory|MergesAdminMemory|ReclassifiesAdminMemory" -count=1`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/app/http.go internal/app/http_test.go internal/app/app.go
git commit -m "feat: add admin manual memory mutation HTTP surface"
```

### Task 4: OpenAPI, Docs, And Task Tracking

**Files:**
- Modify: `openapi/spec.go`
- Modify: `openapi/openapi_test.go`
- Modify: `docs/self-hosting.md`
- Modify: `openspec/changes/manual-memory-mutation-and-reclassification/tasks.md`

- [ ] **Step 1: Write the failing OpenAPI and docs assertions**

```go
func TestSpecYAMLIncludesManualMutationRoutes(t *testing.T) {
	for _, want := range []string{
		"/v1/admin/memories",
		"/v1/admin/memories/{memory_id}:merge",
		"/v1/admin/memories/{memory_id}:reclassify",
		"expected_version",
	} {
		if !strings.Contains(SpecYAML(), want) {
			t.Fatalf("SpecYAML() missing %q", want)
		}
	}
}
```

- [ ] **Step 2: Run focused OpenAPI tests and verify RED**

Run: `go test ./openapi -run "ManualMutationRoutes" -count=1`
Expected: FAIL with missing routes or schemas

- [ ] **Step 3: Write minimal publication updates**

```yaml
/v1/admin/memories:
  post:
    operationId: createAdminMemory
/v1/admin/memories/{memory_id}:merge:
  post:
    operationId: mergeAdminMemory
```

- [ ] **Step 4: Run focused OpenAPI tests and verify GREEN**

Run: `go test ./openapi -run "ManualMutationRoutes" -count=1`
Expected: PASS

- [ ] **Step 5: Mark completed OpenSpec tasks and commit**

```bash
git add openapi/spec.go openapi/openapi_test.go docs/self-hosting.md openspec/changes/manual-memory-mutation-and-reclassification/tasks.md
git commit -m "docs: publish manual memory mutation surface"
```

### Task 5: Full Verification

**Files:**
- Modify: `openspec/changes/manual-memory-mutation-and-reclassification/tasks.md`

- [ ] **Step 1: Run targeted package verification**

Run: `go test ./internal/memory ./internal/storage/postgres ./internal/app ./openapi -count=1`
Expected: PASS

- [ ] **Step 2: Run full repository verification**

Run: `go test ./... -count=1 -timeout 15m`
Expected: PASS

- [ ] **Step 3: Mark remaining OpenSpec tasks complete**

```markdown
- [x] 1.1 Define the privileged route and schema surface for manual create, update, merge, and reclassify operations
- [x] 5.4 Add concise documentation for merge and reclassification semantics, including excluded cases
```

- [ ] **Step 4: Commit verification and task state**

```bash
git add openspec/changes/manual-memory-mutation-and-reclassification/tasks.md
git commit -m "chore: complete manual memory mutation proposal tasks"
```
