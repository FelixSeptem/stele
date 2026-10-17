package app

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/FelixSeptem/stele/internal/auth"
	"github.com/FelixSeptem/stele/internal/governance"
	"github.com/FelixSeptem/stele/internal/jobs"
	"github.com/FelixSeptem/stele/internal/memory"
	"github.com/FelixSeptem/stele/internal/policy"
	"github.com/FelixSeptem/stele/internal/storage/postgres"
	"github.com/FelixSeptem/stele/internal/workqueue"
	"github.com/google/uuid"
)

// TestMemoryRepositoryHTTPPostgresWorkerAcceptanceFixture exercises the
// public review/submit surface against PostgreSQL, then drains the bounded
// import and governed intent workers. Operators opt into this fixture with a
// disposable database because it applies additive migrations.
func TestMemoryRepositoryHTTPPostgresWorkerAcceptanceFixture(t *testing.T) {
	dsn := os.Getenv("STELE_TEST_POSTGRES_MEMORY_REPOSITORY_DSN")
	if dsn == "" {
		t.Skip("STELE_TEST_POSTGRES_MEMORY_REPOSITORY_DSN is not configured")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	if err := postgres.NewMigrationRunner().Apply(ctx, dsn); err != nil {
		t.Fatal(err)
	}
	pool, err := postgres.OpenPool(ctx, dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()

	scope := memory.Scope{Tenant: "memory-repository-it-" + uuid.NewString(), Project: "project", Namespace: "namespace"}
	// The repository store assigns durable queue timestamps at acceptance, so
	// the workers in this real PostgreSQL fixture must use the same live clock
	// domain rather than a fixed pre-submission instant.
	clock := func() time.Time { return time.Now().UTC() }
	repo := postgres.NewRepository(pool)
	service := memory.MemoryRepositoryService{Enabled: true, Store: repo, Now: clock, NewID: uuid.NewString}
	handler := NewHTTPHandler(HTTPDependencies{APIKeys: auth.StaticAPIKeys{"acceptance-key": {}}, MemoryRepository: service})

	registered := struct {
		RepositoryID string `json:"repository_id"`
		Generation   int64  `json:"generation"`
	}{}
	response := memoryRepositoryAcceptanceRequest(t, handler, http.MethodPost, "/v1/memory-repositories", scope, map[string]any{"name": "acceptance fixture"}, "")
	if response.Code != http.StatusCreated {
		t.Fatalf("register response = %d %q", response.Code, response.Body.String())
	}
	decodeMemoryRepositoryAcceptanceJSON(t, response, &registered)
	if registered.RepositoryID == "" || registered.Generation != 0 {
		t.Fatalf("registered = %+v", registered)
	}

	snapshot := memory.MemoryRepositorySnapshot{
		FormatVersion: memory.MemoryRepositoryFormatVersion,
		Revision:      "r1",
		Files: []memory.MemoryRepositoryFile{
			{Path: "MEMORY.md", Content: "- [[topic]]\n"},
			{Path: "topic.md", Content: "- retain governed evidence\n"},
		},
	}
	parsed, err := memory.ParseMemoryRepositorySnapshot(snapshot)
	if err != nil || len(parsed.Entries) != 1 {
		t.Fatalf("parse fixture = %+v, %v", parsed, err)
	}
	selection := map[string]any{"path": "topic.md", "entry_key": parsed.Entries[0].Key, "memory_class": memory.MemoryClassEpisodic, "memory_path": "projects/acceptance"}
	source := memory.SourceMetadata{Version: memory.SourceMetadataVersion, Trust: memory.SourceTrustUntrusted, Classification: memory.SourceClassificationLow}
	previewRequest := func(expectedGeneration int64, next memory.MemoryRepositorySnapshot) map[string]any {
		return map[string]any{
			"actor": "acceptance-agent", "reason": "review supplied snapshot", "expected_generation": expectedGeneration,
			"snapshot": next, "selections": []any{selection}, "source_metadata": source,
		}
	}
	previewResponse := struct {
		PreviewID     string `json:"preview_id"`
		PlanDigest    string `json:"plan_digest"`
		WarningDigest string `json:"warning_digest"`
		Generation    int64  `json:"generation"`
	}{}
	response = memoryRepositoryAcceptanceRequest(t, handler, http.MethodPost, "/v1/memory-repositories/"+registered.RepositoryID+"/previews", scope, previewRequest(0, snapshot), "")
	if response.Code != http.StatusCreated {
		t.Fatalf("first preview response = %d %q", response.Code, response.Body.String())
	}
	decodeMemoryRepositoryAcceptanceJSON(t, response, &previewResponse)

	secondSnapshot := snapshot
	secondSnapshot.Revision = "r2"
	secondPreview := previewResponse
	response = memoryRepositoryAcceptanceRequest(t, handler, http.MethodPost, "/v1/memory-repositories/"+registered.RepositoryID+"/previews", scope, previewRequest(0, secondSnapshot), "")
	if response.Code != http.StatusCreated {
		t.Fatalf("second preview response = %d %q", response.Code, response.Body.String())
	}
	decodeMemoryRepositoryAcceptanceJSON(t, response, &secondPreview)

	submitBody := func(preview struct {
		PreviewID     string `json:"preview_id"`
		PlanDigest    string `json:"plan_digest"`
		WarningDigest string `json:"warning_digest"`
		Generation    int64  `json:"generation"`
	}) map[string]any {
		return map[string]any{"preview_id": preview.PreviewID, "plan_digest": preview.PlanDigest, "warning_digest": preview.WarningDigest, "expected_generation": preview.Generation}
	}
	accepted := struct {
		ImportID string `json:"import_id"`
		Replayed bool   `json:"replayed"`
	}{}
	importPath := "/v1/memory-repositories/" + registered.RepositoryID + "/imports"
	response = memoryRepositoryAcceptanceRequest(t, handler, http.MethodPost, importPath, scope, submitBody(previewResponse), "acceptance-import-1")
	if response.Code != http.StatusAccepted {
		t.Fatalf("accept import response = %d %q", response.Code, response.Body.String())
	}
	decodeMemoryRepositoryAcceptanceJSON(t, response, &accepted)
	if accepted.ImportID == "" || accepted.Replayed {
		t.Fatalf("accepted import = %+v", accepted)
	}

	replay := accepted
	response = memoryRepositoryAcceptanceRequest(t, handler, http.MethodPost, importPath, scope, submitBody(previewResponse), "acceptance-import-1")
	if response.Code != http.StatusAccepted {
		t.Fatalf("replay response = %d %q", response.Code, response.Body.String())
	}
	decodeMemoryRepositoryAcceptanceJSON(t, response, &replay)
	if replay.ImportID != accepted.ImportID || !replay.Replayed {
		t.Fatalf("replay = %+v, want original import replay", replay)
	}

	response = memoryRepositoryAcceptanceRequest(t, handler, http.MethodPost, importPath, scope, submitBody(secondPreview), "acceptance-import-2")
	if response.Code != http.StatusConflict {
		t.Fatalf("stale submission response = %d %q", response.Code, response.Body.String())
	}

	intentPolicy := memory.StaticMemoryIntentPolicy{Scope: scope, PolicyVersion: "memory-repository-acceptance-v1", Enabled: true}
	intentService := memory.MemoryIntentService{Processor: repo, Policy: intentPolicy, Now: clock, NewID: uuid.NewString}
	importKind := workqueue.WorkKindMemoryRepositoryImport
	importWorker := jobs.DerivedWorkWorker{
		Store: repo, Scope: scope, WorkerID: "memory-repository-acceptance-import", BatchSize: 1, LeaseDuration: time.Minute, Now: clock, Kind: &importKind,
		Executor: jobs.MemoryRepositoryImportWorkExecutor{Dispatcher: memory.MemoryRepositoryImportDispatcher{Store: repo, Intents: intentService, Now: clock}},
	}
	if processed, err := importWorker.RunOnce(ctx); err != nil || processed != 1 {
		t.Fatalf("import worker = %d, %v", processed, err)
	}
	intentKind := workqueue.WorkKindMemoryIntent
	intentWorker := jobs.DerivedWorkWorker{
		Store: repo, Scope: scope, WorkerID: "memory-repository-acceptance-intent", BatchSize: 1, LeaseDuration: time.Minute, Now: clock, Kind: &intentKind,
		Executor: jobs.MemoryIntentWorkExecutor{
			Reader: repo, History: repo, Recorder: repo, Policy: intentPolicy,
			Worker: memory.MemoryIntentWorker{Router: memoryIntentGovernanceRouter{repo: repo, now: clock}, Validator: repo, Now: clock},
		},
	}
	if processed, err := intentWorker.RunOnce(ctx); err != nil || processed != 1 {
		t.Fatalf("intent worker = %d, %v", processed, err)
	}

	status := struct {
		Status  memory.MemoryRepositoryImportStatus `json:"status"`
		Entries []struct {
			Status memory.MemoryRepositoryEntryDispatchStatus `json:"status"`
		} `json:"entries"`
	}{}
	response = memoryRepositoryAcceptanceRequest(t, handler, http.MethodGet, "/v1/memory-repositories/"+registered.RepositoryID+"/imports/"+accepted.ImportID, scope, nil, "")
	if response.Code != http.StatusOK || bytes.Contains(response.Body.Bytes(), []byte("retain governed evidence")) {
		t.Fatalf("status response = %d %q", response.Code, response.Body.String())
	}
	decodeMemoryRepositoryAcceptanceJSON(t, response, &status)
	if status.Status != memory.MemoryRepositoryImportCompleted || len(status.Entries) != 1 || status.Entries[0].Status != memory.MemoryRepositoryEntrySubmitted {
		t.Fatalf("status = %+v", status)
	}

	manifest := struct {
		Items []struct {
			MemoryID   string `json:"memory_id"`
			Version    int64  `json:"version"`
			SourcePath string `json:"source_path"`
		} `json:"items"`
	}{}
	manifestPath := "/v1/memory-repositories/" + registered.RepositoryID + "/manifest"
	response = memoryRepositoryAcceptanceRequest(t, handler, http.MethodGet, manifestPath, scope, nil, "")
	if response.Code != http.StatusOK {
		t.Fatalf("manifest response = %d %q", response.Code, response.Body.String())
	}
	decodeMemoryRepositoryAcceptanceJSON(t, response, &manifest)
	if len(manifest.Items) != 1 || manifest.Items[0].SourcePath != "topic.md" {
		t.Fatalf("manifest = %+v", manifest)
	}
	history, err := repo.ReadMemoryHistory(ctx, scope, manifest.Items[0].MemoryID, false)
	if err != nil || history.Memory.State != memory.MemoryStateActive || history.Memory.Content != "retain governed evidence" {
		t.Fatalf("retrieved canonical memory = %+v, %v", history.Memory, err)
	}

	forgetter := governance.ForgettingProcessor{Repository: repo, Now: clock}
	if err := forgetter.Apply(ctx, governance.LifecycleAction{MemoryID: history.Memory.ID, MemoryPath: history.Memory.MemoryPath, Scope: scope, Action: policy.ForgettingActionExpire, Actor: "acceptance-agent", Reason: "verify lifecycle filtering", RequestID: "acceptance-forget-1", AppliedAt: clock()}); err != nil {
		t.Fatal(err)
	}
	response = memoryRepositoryAcceptanceRequest(t, handler, http.MethodGet, manifestPath, scope, nil, "")
	if response.Code != http.StatusOK {
		t.Fatalf("forgotten manifest response = %d %q", response.Code, response.Body.String())
	}
	decodeMemoryRepositoryAcceptanceJSON(t, response, &manifest)
	if len(manifest.Items) != 0 {
		t.Fatalf("forgotten memory leaked into manifest: %+v", manifest)
	}

	thirdSnapshot := snapshot
	thirdSnapshot.Revision = "r3"
	thirdPreview := previewResponse
	response = memoryRepositoryAcceptanceRequest(t, handler, http.MethodPost, "/v1/memory-repositories/"+registered.RepositoryID+"/previews", scope, previewRequest(1, thirdSnapshot), "")
	if response.Code != http.StatusCreated {
		t.Fatalf("reimport preview response = %d %q", response.Code, response.Body.String())
	}
	decodeMemoryRepositoryAcceptanceJSON(t, response, &thirdPreview)
	response = memoryRepositoryAcceptanceRequest(t, handler, http.MethodPost, importPath, scope, submitBody(thirdPreview), "acceptance-import-3")
	if response.Code != http.StatusAccepted {
		t.Fatalf("reimport response = %d %q", response.Code, response.Body.String())
	}
	if processed, err := importWorker.RunOnce(ctx); err != nil || processed != 1 {
		t.Fatalf("duplicate import worker = %d, %v", processed, err)
	}
	var intentCount int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM memory_intents WHERE tenant=$1 AND project=$2 AND namespace=$3`, scope.Tenant, scope.Project, scope.Namespace).Scan(&intentCount); err != nil {
		t.Fatal(err)
	}
	if intentCount != 1 {
		t.Fatalf("identical reimport created %d intents, want 1", intentCount)
	}
	response = memoryRepositoryAcceptanceRequest(t, handler, http.MethodGet, manifestPath, scope, nil, "")
	decodeMemoryRepositoryAcceptanceJSON(t, response, &manifest)
	if response.Code != http.StatusOK || len(manifest.Items) != 0 {
		t.Fatalf("identical reimport resurrected forgotten memory: %d %+v", response.Code, manifest)
	}

	foreignHandler := NewHTTPHandler(HTTPDependencies{
		APIKeys:          auth.StaticAPIKeys{"acceptance-key": {}},
		MemoryRepository: service,
		PrincipalAuthorizer: stubPrincipalAuthorizer{
			principal: auth.Principal{ID: "foreign-principal", Role: auth.PrincipalRolePublic, Status: auth.PrincipalStatusActive},
			granted:   false,
		},
	})
	foreignScope := scope
	foreignScope.Tenant = "foreign-" + uuid.NewString()
	response = memoryRepositoryAcceptanceRequest(t, foreignHandler, http.MethodGet, manifestPath, foreignScope, nil, "")
	if response.Code != http.StatusForbidden {
		t.Fatalf("foreign scope response = %d %q", response.Code, response.Body.String())
	}
}

func memoryRepositoryAcceptanceRequest(t *testing.T, handler http.Handler, method, path string, scope memory.Scope, body any, idempotencyKey string) *httptest.ResponseRecorder {
	t.Helper()
	var content []byte
	if body != nil {
		var err error
		content, err = json.Marshal(body)
		if err != nil {
			t.Fatal(err)
		}
	}
	req := httptest.NewRequest(method, path, bytes.NewReader(content))
	req.Header.Set(auth.HeaderAPIKey, "acceptance-key")
	req.Header.Set(auth.HeaderTenant, scope.Tenant)
	req.Header.Set(auth.HeaderProject, scope.Project)
	req.Header.Set(auth.HeaderNamespace, scope.Namespace)
	if idempotencyKey != "" {
		req.Header.Set("Idempotency-Key", idempotencyKey)
	}
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, req)
	return response
}

func decodeMemoryRepositoryAcceptanceJSON(t *testing.T, response *httptest.ResponseRecorder, target any) {
	t.Helper()
	if err := json.NewDecoder(response.Body).Decode(target); err != nil {
		t.Fatalf("decode response %q: %v", response.Body.String(), err)
	}
}
