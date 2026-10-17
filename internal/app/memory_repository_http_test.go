package app

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/FelixSeptem/stele/internal/auth"
	"github.com/FelixSeptem/stele/internal/memory"
)

type memoryRepositoryHTTPStub struct {
	importRecord memory.MemoryRepositoryImportRecord
	manifest     memory.MemoryRepositoryManifest
	readScope    memory.Scope
}

func (s *memoryRepositoryHTTPStub) Register(context.Context, memory.Scope, string, string) (memory.MemoryRepositoryRecord, error) {
	return memory.MemoryRepositoryRecord{ID: "repo", Generation: 0}, nil
}
func (s *memoryRepositoryHTTPStub) Preview(context.Context, memory.MemoryRepositoryPreviewRequest) (memory.MemoryRepositoryPreview, error) {
	return memory.MemoryRepositoryPreview{}, nil
}
func (s *memoryRepositoryHTTPStub) Submit(context.Context, memory.MemoryRepositoryImportRequest) (memory.MemoryRepositoryImportRecord, bool, error) {
	return s.importRecord, false, nil
}
func (s *memoryRepositoryHTTPStub) ReadImport(_ context.Context, scope memory.Scope, _, _ string) (memory.MemoryRepositoryImportRecord, error) {
	s.readScope = scope
	return s.importRecord, nil
}
func (s *memoryRepositoryHTTPStub) Manifest(_ context.Context, scope memory.Scope, _ string) (memory.MemoryRepositoryManifest, error) {
	s.readScope = scope
	return s.manifest, nil
}

func memoryRepositoryHTTPHeaders(req *http.Request) {
	req.Header.Set(auth.HeaderAPIKey, "api-key")
	req.Header.Set(auth.HeaderTenant, "tenant-a")
	req.Header.Set(auth.HeaderProject, "project-a")
	req.Header.Set(auth.HeaderNamespace, "namespace-a")
}

func TestMemoryRepositoryHTTPDefaultsToNonDisclosingDisabledRoute(t *testing.T) {
	h := NewHTTPHandler(HTTPDependencies{APIKeys: auth.StaticAPIKeys{"api-key": {}}})
	req := httptest.NewRequest(http.MethodPost, "/v1/memory-repositories", strings.NewReader(`{"name":"notes"}`))
	memoryRepositoryHTTPHeaders(req)
	recorder := httptest.NewRecorder()
	h.ServeHTTP(recorder, req)
	if recorder.Code != http.StatusNotFound || strings.Contains(recorder.Body.String(), "memory-repository") {
		t.Fatalf("disabled response = %d %q", recorder.Code, recorder.Body.String())
	}
}

func TestMemoryRepositoryHTTPStatusOmitsRetainedPayloadAndPreservesScope(t *testing.T) {
	scope := memory.Scope{Tenant: "tenant-a", Project: "project-a", Namespace: "namespace-a"}
	stub := &memoryRepositoryHTTPStub{importRecord: memory.MemoryRepositoryImportRecord{
		ID: "import", RepositoryID: "repo", Scope: scope, Generation: 1, Status: memory.MemoryRepositoryImportRunning,
		Entries: []memory.MemoryRepositoryImportEntry{{EntryKey: strings.Repeat("a", 64), DispatchStatus: memory.MemoryRepositoryEntryQueued, Payload: memory.MemoryRepositoryPreviewEntry{Text: "secret markdown"}}},
	}}
	h := NewHTTPHandler(HTTPDependencies{APIKeys: auth.StaticAPIKeys{"api-key": {}}, MemoryRepository: stub})
	req := httptest.NewRequest(http.MethodGet, "/v1/memory-repositories/repo/imports/import", nil)
	memoryRepositoryHTTPHeaders(req)
	recorder := httptest.NewRecorder()
	h.ServeHTTP(recorder, req)
	if recorder.Code != http.StatusOK {
		t.Fatalf("status response = %d %q", recorder.Code, recorder.Body.String())
	}
	if strings.Contains(recorder.Body.String(), "secret markdown") || !strings.Contains(recorder.Body.String(), `"status":"running"`) {
		t.Fatalf("status payload leaked or missing status: %q", recorder.Body.String())
	}
	if stub.readScope != scope {
		t.Fatalf("read scope = %+v, want %+v", stub.readScope, scope)
	}
}

func TestMemoryRepositoryHTTPManifestIsBoundedNavigationShape(t *testing.T) {
	scope := memory.Scope{Tenant: "tenant-a", Project: "project-a", Namespace: "namespace-a"}
	stub := &memoryRepositoryHTTPStub{manifest: memory.MemoryRepositoryManifest{RepositoryID: "repo", Generation: 2, ProcessingStatus: memory.MemoryRepositoryImportCompleted, Items: []memory.MemoryRepositoryManifestItem{{MemoryID: "memory-1", Version: 3, SourcePath: "topic.md"}}}}
	h := NewHTTPHandler(HTTPDependencies{APIKeys: auth.StaticAPIKeys{"api-key": {}}, MemoryRepository: stub})
	req := httptest.NewRequest(http.MethodGet, "/v1/memory-repositories/repo/manifest", nil)
	memoryRepositoryHTTPHeaders(req)
	recorder := httptest.NewRecorder()
	h.ServeHTTP(recorder, req)
	if recorder.Code != http.StatusOK || !strings.Contains(recorder.Body.String(), `"source_path":"topic.md"`) || strings.Contains(recorder.Body.String(), "secret") {
		t.Fatalf("manifest response = %d %q", recorder.Code, recorder.Body.String())
	}
	if stub.readScope != scope {
		t.Fatalf("manifest scope = %+v, want %+v", stub.readScope, scope)
	}
	_ = time.Now()
}

func TestMemoryRepositoryHTTPForeignScopeIsDeniedBeforeRepositoryLookup(t *testing.T) {
	stub := &memoryRepositoryHTTPStub{}
	h := NewHTTPHandler(HTTPDependencies{
		APIKeys:          auth.StaticAPIKeys{"api-key": {}},
		MemoryRepository: stub,
		PrincipalAuthorizer: stubPrincipalAuthorizer{
			principal: auth.Principal{ID: "principal-a", Role: auth.PrincipalRolePublic, Status: auth.PrincipalStatusActive},
			granted:   false,
		},
	})
	req := httptest.NewRequest(http.MethodGet, "/v1/memory-repositories/foreign/imports/import", nil)
	memoryRepositoryHTTPHeaders(req)
	recorder := httptest.NewRecorder()
	h.ServeHTTP(recorder, req)
	if recorder.Code != http.StatusForbidden || stub.readScope != (memory.Scope{}) {
		t.Fatalf("foreign scope response = %d %q, lookup scope=%+v", recorder.Code, recorder.Body.String(), stub.readScope)
	}
}
