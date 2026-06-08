package app

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/FelixSeptem/stele/internal/jobs"
	"github.com/FelixSeptem/stele/internal/memory"
	"github.com/FelixSeptem/stele/internal/policy"
	"github.com/FelixSeptem/stele/internal/retrieval"
	"github.com/jackc/pgx/v5"
)

type stubReadinessChecker struct {
	err error
}

func (s stubReadinessChecker) Ready(ctx context.Context) error {
	return s.err
}

type stubEventIngestor struct {
	gotInput memory.IngestEventInput
	eventID  string
	err      error
}

func (s *stubEventIngestor) Ingest(ctx context.Context, input memory.IngestEventInput) (memory.RawEvent, error) {
	s.gotInput = input
	if s.err != nil {
		return memory.RawEvent{}, s.err
	}

	return memory.RawEvent{ID: s.eventID}, nil
}

type panicEventIngestor struct{}

func (panicEventIngestor) Ingest(ctx context.Context, input memory.IngestEventInput) (memory.RawEvent, error) {
	panic("boom")
}

type stubMemorySearcher struct {
	gotInput retrieval.SearchInput
	result   retrieval.SearchResult
	err      error
}

func (s *stubMemorySearcher) Search(ctx context.Context, input retrieval.SearchInput) (retrieval.SearchResult, error) {
	s.gotInput = input
	return s.result, s.err
}

type stubContextAssembler struct {
	gotInput retrieval.AssembleContextInput
	result   retrieval.AssembledContext
	err      error
}

func (s *stubContextAssembler) AssembleContext(ctx context.Context, input retrieval.AssembleContextInput) (retrieval.AssembledContext, error) {
	s.gotInput = input
	return s.result, s.err
}

type stubGovernanceStatusReader struct {
	status GovernanceStatus
	err    error
}

func (s *stubGovernanceStatusReader) ReadGovernanceStatus(ctx context.Context) (GovernanceStatus, error) {
	return s.status, s.err
}

type stubMemoryHistoryReader struct {
	history memory.MemoryHistory
	err     error
}

func (s *stubMemoryHistoryReader) ReadMemoryHistory(ctx context.Context, scope memory.Scope, memoryID string) (memory.MemoryHistory, error) {
	if s.err != nil {
		return memory.MemoryHistory{}, s.err
	}

	if s.history.Memory.ID == "" {
		s.history.Memory.ID = memoryID
		s.history.Memory.Scope = scope
	}

	return s.history, nil
}

type stubMemoryQueryService struct {
	gotListInput       memory.ListMemoriesInput
	gotGetScope        memory.Scope
	gotGetMemoryID     string
	gotHistoryScope    memory.Scope
	gotHistoryMemoryID string
	gotProvScope       memory.Scope
	gotProvMemoryID    string
	page               memory.MemoryPage
	resource           memory.MemoryResource
	history            memory.MemoryHistory
	provenance         []memory.ProvenanceRecord
	err                error
}

func (s *stubMemoryQueryService) ListMemories(ctx context.Context, input memory.ListMemoriesInput) (memory.MemoryPage, error) {
	s.gotListInput = input
	return s.page, s.err
}

func (s *stubMemoryQueryService) GetMemory(ctx context.Context, scope memory.Scope, memoryID string) (memory.MemoryResource, error) {
	s.gotGetScope = scope
	s.gotGetMemoryID = memoryID
	return s.resource, s.err
}

func (s *stubMemoryQueryService) GetMemoryHistory(ctx context.Context, scope memory.Scope, memoryID string) (memory.MemoryHistory, error) {
	s.gotHistoryScope = scope
	s.gotHistoryMemoryID = memoryID
	return s.history, s.err
}

func (s *stubMemoryQueryService) GetMemoryProvenance(ctx context.Context, scope memory.Scope, memoryID string) ([]memory.ProvenanceRecord, error) {
	s.gotProvScope = scope
	s.gotProvMemoryID = memoryID
	return s.provenance, s.err
}

type stubJobExecutionReader struct {
	records []jobs.JobExecutionRecord
	err     error
}

func (s *stubJobExecutionReader) ListRecentJobExecutions(ctx context.Context, scope memory.Scope, limit int) ([]jobs.JobExecutionRecord, error) {
	return s.records, s.err
}

type stubLifecycleService struct {
	gotInput memory.LifecycleActionInput
	err      error
}

func (s *stubLifecycleService) Apply(ctx context.Context, input memory.LifecycleActionInput) error {
	s.gotInput = input
	return s.err
}

type stubManualMutationService struct {
	gotCreateInput     memory.ManualCreateMemoryInput
	gotUpdateInput     memory.ManualUpdateMemoryInput
	gotMergeInput      memory.ManualMergeMemoryInput
	gotReclassifyInput memory.ManualReclassifyMemoryInput
	resource           memory.MemoryResource
	err                error
}

func (s *stubManualMutationService) CreateMemory(ctx context.Context, input memory.ManualCreateMemoryInput) (memory.MemoryResource, error) {
	s.gotCreateInput = input
	return s.resource, s.err
}

func (s *stubManualMutationService) UpdateMemory(ctx context.Context, input memory.ManualUpdateMemoryInput) (memory.MemoryResource, error) {
	s.gotUpdateInput = input
	return s.resource, s.err
}

func (s *stubManualMutationService) MergeMemory(ctx context.Context, input memory.ManualMergeMemoryInput) (memory.MemoryResource, error) {
	s.gotMergeInput = input
	return s.resource, s.err
}

func (s *stubManualMutationService) ReclassifyMemory(ctx context.Context, input memory.ManualReclassifyMemoryInput) (memory.MemoryResource, error) {
	s.gotReclassifyInput = input
	return s.resource, s.err
}

func TestNewHTTPHandlerServesHealthAndReadiness(t *testing.T) {
	var logBuf bytes.Buffer
	handler := NewHTTPHandler(HTTPDependencies{
		Readiness: stubReadinessChecker{},
		Logger:    log.New(&logBuf, "", 0),
	})

	healthReq := httptest.NewRequest(http.MethodGet, "/health", nil)
	healthRec := httptest.NewRecorder()
	handler.ServeHTTP(healthRec, healthReq)
	if healthRec.Code != http.StatusOK {
		t.Fatalf("health status = %d, want %d", healthRec.Code, http.StatusOK)
	}

	if got := healthRec.Header().Get("X-Request-ID"); got == "" {
		t.Fatal("health response missing X-Request-ID")
	}

	if !strings.Contains(logBuf.String(), "/health") {
		t.Fatalf("log output %q does not mention /health", logBuf.String())
	}

	readyReq := httptest.NewRequest(http.MethodGet, "/ready", nil)
	readyRec := httptest.NewRecorder()
	handler.ServeHTTP(readyRec, readyReq)
	if readyRec.Code != http.StatusOK {
		t.Fatalf("ready status = %d, want %d", readyRec.Code, http.StatusOK)
	}
}

func TestNewHTTPHandlerMarksReadinessFailure(t *testing.T) {
	handler := NewHTTPHandler(HTTPDependencies{
		Readiness: stubReadinessChecker{err: errors.New("db unavailable")},
	})

	req := httptest.NewRequest(http.MethodGet, "/ready", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusServiceUnavailable)
	}
}

func TestNewHTTPHandlerRejectsMissingAPIKeyForEvents(t *testing.T) {
	handler := NewHTTPHandler(HTTPDependencies{
		Readiness:     stubReadinessChecker{},
		APIKeys:       map[string]struct{}{"test-key": {}},
		EventIngestor: &stubEventIngestor{eventID: "evt_123"},
	})

	body := bytes.NewBufferString(`{"event_type":"conversation.message","content":"hello"}`)
	req := httptest.NewRequest(http.MethodPost, "/v1/events", body)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Stele-Tenant", "tenant-a")
	req.Header.Set("X-Stele-Project", "project-a")
	req.Header.Set("X-Stele-Namespace", "namespace-a")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusUnauthorized)
	}
}

func TestNewHTTPHandlerIngestsEvent(t *testing.T) {
	ingestor := &stubEventIngestor{eventID: "evt_123"}
	handler := NewHTTPHandler(HTTPDependencies{
		Readiness:     stubReadinessChecker{},
		APIKeys:       map[string]struct{}{"test-key": {}},
		EventIngestor: ingestor,
	})

	sourceTime := time.Date(2026, 5, 29, 22, 30, 0, 0, time.UTC)
	body, err := json.Marshal(map[string]any{
		"event_type":       "conversation.message",
		"content":          "hello world",
		"metadata":         map[string]any{"channel": "chat"},
		"source_timestamp": sourceTime.Format(time.RFC3339),
	})
	if err != nil {
		t.Fatalf("json.Marshal() error = %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/v1/events", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-API-Key", "test-key")
	req.Header.Set("X-Stele-Tenant", "tenant-a")
	req.Header.Set("X-Stele-Project", "project-a")
	req.Header.Set("X-Stele-Namespace", "namespace-a")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusCreated)
	}

	if ingestor.gotInput.Scope.Tenant != "tenant-a" || ingestor.gotInput.Scope.Project != "project-a" || ingestor.gotInput.Scope.Namespace != "namespace-a" {
		t.Fatalf("scope = %+v, want resolved headers", ingestor.gotInput.Scope)
	}

	if ingestor.gotInput.EventType != "conversation.message" {
		t.Fatalf("EventType = %q, want %q", ingestor.gotInput.EventType, "conversation.message")
	}
}

func TestNewHTTPHandlerRejectsInvalidEventPayload(t *testing.T) {
	handler := NewHTTPHandler(HTTPDependencies{
		Readiness:     stubReadinessChecker{},
		APIKeys:       map[string]struct{}{"test-key": {}},
		EventIngestor: &stubEventIngestor{eventID: "evt_123"},
	})

	req := httptest.NewRequest(http.MethodPost, "/v1/events", bytes.NewBufferString(`{"event_type":"","content":""}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-API-Key", "test-key")
	req.Header.Set("X-Stele-Tenant", "tenant-a")
	req.Header.Set("X-Stele-Project", "project-a")
	req.Header.Set("X-Stele-Namespace", "namespace-a")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusBadRequest)
	}
}

func TestNewHTTPHandlerRecoversFromPanic(t *testing.T) {
	handler := NewHTTPHandler(HTTPDependencies{
		Readiness:     stubReadinessChecker{},
		APIKeys:       map[string]struct{}{"test-key": {}},
		EventIngestor: panicEventIngestor{},
	})

	req := httptest.NewRequest(http.MethodPost, "/v1/events", bytes.NewBufferString(`{"event_type":"conversation.message","content":"hello"}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-API-Key", "test-key")
	req.Header.Set("X-Stele-Tenant", "tenant-a")
	req.Header.Set("X-Stele-Project", "project-a")
	req.Header.Set("X-Stele-Namespace", "namespace-a")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusInternalServerError)
	}
}

func TestNewHTTPServerUsesConfiguredAddress(t *testing.T) {
	server := NewHTTPServer(":9090", HTTPDependencies{
		Readiness: stubReadinessChecker{},
	})

	if server.Addr != ":9090" {
		t.Fatalf("server.Addr = %q, want %q", server.Addr, ":9090")
	}

	if server.Handler == nil {
		t.Fatal("server.Handler = nil, want handler")
	}
}

func TestNewHTTPHandlerReturnsServerErrorWhenIngestFails(t *testing.T) {
	ingestor := &stubEventIngestor{err: errors.New("ingest failed")}
	handler := NewHTTPHandler(HTTPDependencies{
		Readiness:     stubReadinessChecker{},
		APIKeys:       map[string]struct{}{"test-key": {}},
		EventIngestor: ingestor,
	})

	req := httptest.NewRequest(http.MethodPost, "/v1/events", bytes.NewBufferString(`{"event_type":"conversation.message","content":"hello"}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-API-Key", "test-key")
	req.Header.Set("X-Stele-Tenant", "tenant-a")
	req.Header.Set("X-Stele-Project", "project-a")
	req.Header.Set("X-Stele-Namespace", "namespace-a")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusInternalServerError)
	}
}

func TestNewHTTPHandlerSearchesMemories(t *testing.T) {
	searcher := &stubMemorySearcher{
		result: retrieval.SearchResult{
			Hits: []retrieval.SearchHit{
				{
					Memory: memory.CanonicalMemory{
						ID:         "mem_123",
						Scope:      memory.Scope{Tenant: "tenant-a", Project: "project-a", Namespace: "namespace-a"},
						Class:      memory.MemoryClassProfile,
						State:      memory.MemoryStateActive,
						Content:    "User prefers concise answers.",
						CreatedAt:  time.Date(2026, 6, 6, 10, 0, 0, 0, time.UTC),
						ModifiedAt: time.Date(2026, 6, 6, 10, 30, 0, 0, time.UTC),
					},
					Score: retrieval.ScoreBreakdown{
						Overall:  1.2,
						Lexical:  0.7,
						Semantic: 0.5,
					},
					Citations: []retrieval.Citation{
						{MemoryID: "mem_123", RawEventID: "evt_123", Operation: "promote_candidate"},
					},
				},
			},
		},
	}

	handler := NewHTTPHandler(HTTPDependencies{
		Readiness:      stubReadinessChecker{},
		APIKeys:        map[string]struct{}{"test-key": {}},
		MemorySearcher: searcher,
	})

	req := httptest.NewRequest(http.MethodPost, "/v1/memories/search", bytes.NewBufferString(`{"query":"concise","query_embedding":[0.1,0.2,0.3],"top_k":3,"include_summaries":true,"time_from":"2026-06-06T09:00:00Z","time_to":"2026-06-06T12:00:00Z"}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-API-Key", "test-key")
	req.Header.Set("X-Stele-Tenant", "tenant-a")
	req.Header.Set("X-Stele-Project", "project-a")
	req.Header.Set("X-Stele-Namespace", "namespace-a")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}

	if searcher.gotInput.Scope.Tenant != "tenant-a" {
		t.Fatalf("search scope = %+v, want resolved request scope", searcher.gotInput.Scope)
	}

	if searcher.gotInput.TimeFrom.IsZero() || searcher.gotInput.TimeTo.IsZero() {
		t.Fatalf("time window = %v to %v, want parsed time range", searcher.gotInput.TimeFrom, searcher.gotInput.TimeTo)
	}

	if len(searcher.gotInput.QueryEmbedding) != 3 {
		t.Fatalf("query embedding = %v, want parsed embedding", searcher.gotInput.QueryEmbedding)
	}

	var payload map[string]any
	if err := json.NewDecoder(rec.Body).Decode(&payload); err != nil {
		t.Fatalf("json.NewDecoder() error = %v", err)
	}

	hits, ok := payload["hits"].([]any)
	if !ok || len(hits) != 1 {
		t.Fatalf("hits payload = %#v, want one hit", payload["hits"])
	}
}

func TestNewHTTPHandlerAssemblesContext(t *testing.T) {
	assembler := &stubContextAssembler{
		result: retrieval.AssembledContext{
			Profile: []retrieval.SearchHit{
				{
					Memory: memory.CanonicalMemory{
						ID:         "mem_profile",
						Scope:      memory.Scope{Tenant: "tenant-a", Project: "project-a", Namespace: "namespace-a"},
						Class:      memory.MemoryClassProfile,
						State:      memory.MemoryStateActive,
						Content:    "User prefers concise answers.",
						CreatedAt:  time.Date(2026, 6, 6, 10, 0, 0, 0, time.UTC),
						ModifiedAt: time.Date(2026, 6, 6, 10, 30, 0, 0, time.UTC),
					},
					Score: retrieval.ScoreBreakdown{Overall: 0.9},
				},
			},
			Citations: []retrieval.Citation{
				{MemoryID: "mem_profile", RawEventID: "evt_profile", Operation: "promote_candidate"},
			},
		},
	}

	handler := NewHTTPHandler(HTTPDependencies{
		Readiness:        stubReadinessChecker{},
		APIKeys:          map[string]struct{}{"test-key": {}},
		ContextAssembler: assembler,
	})

	req := httptest.NewRequest(http.MethodPost, "/v1/context/assemble", bytes.NewBufferString(`{"query":"preferences","budget":4}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-API-Key", "test-key")
	req.Header.Set("X-Stele-Tenant", "tenant-a")
	req.Header.Set("X-Stele-Project", "project-a")
	req.Header.Set("X-Stele-Namespace", "namespace-a")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}

	if assembler.gotInput.Scope.Namespace != "namespace-a" {
		t.Fatalf("assemble scope = %+v, want resolved request scope", assembler.gotInput.Scope)
	}
}

func TestNewHTTPHandlerListsVisibleMemories(t *testing.T) {
	reader := &stubMemoryQueryService{
		page: memory.MemoryPage{
			Items: []memory.MemoryResource{
				{
					ID:      "mem_123",
					Scope:   memory.Scope{Tenant: "tenant-a", Project: "project-a", Namespace: "namespace-a"},
					Class:   memory.MemoryClassProfile,
					State:   memory.MemoryStateActive,
					Content: "User prefers concise answers.",
				},
			},
		},
	}

	handler := NewHTTPHandler(HTTPDependencies{
		APIKeys:     map[string]struct{}{"test-key": {}},
		MemoryQuery: reader,
	})

	req := httptest.NewRequest(http.MethodGet, "/v1/memories?class=profile&limit=10", nil)
	req.Header.Set("X-API-Key", "test-key")
	req.Header.Set("X-Stele-Tenant", "tenant-a")
	req.Header.Set("X-Stele-Project", "project-a")
	req.Header.Set("X-Stele-Namespace", "namespace-a")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}

	if reader.gotListInput.Scope.Tenant != "tenant-a" {
		t.Fatalf("scope = %+v, want resolved scope", reader.gotListInput.Scope)
	}

	if reader.gotListInput.Limit != 10 {
		t.Fatalf("limit = %d, want %d", reader.gotListInput.Limit, 10)
	}

	if len(reader.gotListInput.Classes) != 1 || reader.gotListInput.Classes[0] != memory.MemoryClassProfile {
		t.Fatalf("classes = %v, want one profile class", reader.gotListInput.Classes)
	}
}

func TestNewHTTPHandlerReturnsMemoryDetail(t *testing.T) {
	reader := &stubMemoryQueryService{
		resource: memory.MemoryResource{
			ID:      "mem_123",
			Scope:   memory.Scope{Tenant: "tenant-a", Project: "project-a", Namespace: "namespace-a"},
			Class:   memory.MemoryClassProfile,
			State:   memory.MemoryStateActive,
			Content: "User prefers concise answers.",
		},
	}

	handler := NewHTTPHandler(HTTPDependencies{
		APIKeys:     map[string]struct{}{"test-key": {}},
		MemoryQuery: reader,
	})

	req := httptest.NewRequest(http.MethodGet, "/v1/memories/mem_123", nil)
	req.Header.Set("X-API-Key", "test-key")
	req.Header.Set("X-Stele-Tenant", "tenant-a")
	req.Header.Set("X-Stele-Project", "project-a")
	req.Header.Set("X-Stele-Namespace", "namespace-a")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}

	if reader.gotGetMemoryID != "mem_123" {
		t.Fatalf("memory id = %q, want mem_123", reader.gotGetMemoryID)
	}
}

func TestNewHTTPHandlerReturnsNotFoundForMissingMemoryDetail(t *testing.T) {
	reader := &stubMemoryQueryService{err: pgx.ErrNoRows}
	handler := NewHTTPHandler(HTTPDependencies{
		APIKeys:     map[string]struct{}{"test-key": {}},
		MemoryQuery: reader,
	})

	req := httptest.NewRequest(http.MethodGet, "/v1/memories/mem_missing", nil)
	req.Header.Set("X-API-Key", "test-key")
	req.Header.Set("X-Stele-Tenant", "tenant-a")
	req.Header.Set("X-Stele-Project", "project-a")
	req.Header.Set("X-Stele-Namespace", "namespace-a")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusNotFound)
	}
}

func TestNewHTTPHandlerReturnsMemoryHistory(t *testing.T) {
	reader := &stubMemoryQueryService{
		history: memory.MemoryHistory{
			Memory: memory.CanonicalMemory{
				ID:    "mem_123",
				Scope: memory.Scope{Tenant: "tenant-a", Project: "project-a", Namespace: "namespace-a"},
			},
		},
	}

	handler := NewHTTPHandler(HTTPDependencies{
		APIKeys:     map[string]struct{}{"test-key": {}},
		MemoryQuery: reader,
	})

	req := httptest.NewRequest(http.MethodGet, "/v1/memories/mem_123/history", nil)
	req.Header.Set("X-API-Key", "test-key")
	req.Header.Set("X-Stele-Tenant", "tenant-a")
	req.Header.Set("X-Stele-Project", "project-a")
	req.Header.Set("X-Stele-Namespace", "namespace-a")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}

	if reader.gotHistoryMemoryID != "mem_123" {
		t.Fatalf("memory id = %q, want mem_123", reader.gotHistoryMemoryID)
	}
}

func TestNewHTTPHandlerReturnsMemoryProvenance(t *testing.T) {
	reader := &stubMemoryQueryService{
		provenance: []memory.ProvenanceRecord{
			{ID: "prov_1", MemoryID: "mem_123", Operation: "promote_candidate"},
		},
	}

	handler := NewHTTPHandler(HTTPDependencies{
		APIKeys:     map[string]struct{}{"test-key": {}},
		MemoryQuery: reader,
	})

	req := httptest.NewRequest(http.MethodGet, "/v1/memories/mem_123/provenance", nil)
	req.Header.Set("X-API-Key", "test-key")
	req.Header.Set("X-Stele-Tenant", "tenant-a")
	req.Header.Set("X-Stele-Project", "project-a")
	req.Header.Set("X-Stele-Namespace", "namespace-a")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}

	if reader.gotProvMemoryID != "mem_123" {
		t.Fatalf("memory id = %q, want mem_123", reader.gotProvMemoryID)
	}
}

func TestNewHTTPHandlerReturnsAdminGovernanceStatus(t *testing.T) {
	reader := &stubGovernanceStatusReader{
		status: GovernanceStatus{
			PendingRawEvents:   7,
			LeasedRawEvents:    2,
			ProcessedRawEvents: 19,
		},
	}

	handler := NewHTTPHandler(HTTPDependencies{
		Readiness:            stubReadinessChecker{},
		AdminAPIKeys:         map[string]struct{}{"admin-key": {}},
		GovernanceStatusRead: reader,
	})

	req := httptest.NewRequest(http.MethodGet, "/v1/admin/jobs/governance/status", nil)
	req.Header.Set("X-API-Key", "admin-key")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}

	var payload GovernanceStatus
	if err := json.NewDecoder(rec.Body).Decode(&payload); err != nil {
		t.Fatalf("json.NewDecoder() error = %v", err)
	}

	if payload.PendingRawEvents != 7 || payload.LeasedRawEvents != 2 || payload.ProcessedRawEvents != 19 {
		t.Fatalf("payload = %+v, want returned governance status", payload)
	}
}

func TestNewHTTPHandlerRejectsMissingAdminAPIKey(t *testing.T) {
	handler := NewHTTPHandler(HTTPDependencies{
		AdminAPIKeys:         map[string]struct{}{"admin-key": {}},
		GovernanceStatusRead: &stubGovernanceStatusReader{},
	})

	req := httptest.NewRequest(http.MethodGet, "/v1/admin/jobs/governance/status", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusUnauthorized)
	}
}

func TestNewHTTPHandlerReturnsAdminMemoryHistory(t *testing.T) {
	reader := &stubMemoryHistoryReader{
		history: memory.MemoryHistory{
			Memory: memory.CanonicalMemory{
				ID:         "mem_hidden",
				Scope:      memory.Scope{Tenant: "tenant-a", Project: "project-a", Namespace: "namespace-a"},
				Class:      memory.MemoryClassProfile,
				State:      memory.MemoryStateForgotten,
				Content:    "Old preference",
				CreatedAt:  time.Date(2026, 6, 6, 8, 0, 0, 0, time.UTC),
				ModifiedAt: time.Date(2026, 6, 6, 9, 0, 0, 0, time.UTC),
			},
			Versions: []memory.MemoryVersion{
				{
					ID:         "ver_2",
					MemoryID:   "mem_hidden",
					Version:    2,
					State:      memory.MemoryStateForgotten,
					Content:    "Old preference",
					CreatedAt:  time.Date(2026, 6, 6, 9, 0, 0, 0, time.UTC),
					ModifiedBy: "cand_2",
				},
			},
			Provenance: []memory.ProvenanceRecord{
				{
					ID:         "prov_1",
					Scope:      memory.Scope{Tenant: "tenant-a", Project: "project-a", Namespace: "namespace-a"},
					RawEventID: "evt_1",
					MemoryID:   "mem_hidden",
					Operation:  "promote_candidate",
					CreatedAt:  time.Date(2026, 6, 6, 9, 0, 0, 0, time.UTC),
				},
			},
		},
	}

	handler := NewHTTPHandler(HTTPDependencies{
		AdminAPIKeys:      map[string]struct{}{"admin-key": {}},
		MemoryHistoryRead: reader,
	})

	req := httptest.NewRequest(http.MethodGet, "/v1/admin/memories/mem_hidden/history", nil)
	req.Header.Set("X-API-Key", "admin-key")
	req.Header.Set("X-Stele-Tenant", "tenant-a")
	req.Header.Set("X-Stele-Project", "project-a")
	req.Header.Set("X-Stele-Namespace", "namespace-a")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}

	var payload memory.MemoryHistory
	if err := json.NewDecoder(rec.Body).Decode(&payload); err != nil {
		t.Fatalf("json.NewDecoder() error = %v", err)
	}

	if payload.Memory.State != memory.MemoryStateForgotten {
		t.Fatalf("Memory.State = %q, want %q", payload.Memory.State, memory.MemoryStateForgotten)
	}

	if len(payload.Versions) != 1 || len(payload.Provenance) != 1 {
		t.Fatalf("history payload = %+v, want one version and one provenance record", payload)
	}
}

func TestNewHTTPHandlerReturnsAdminRecentJobStatus(t *testing.T) {
	reader := &stubJobExecutionReader{
		records: []jobs.JobExecutionRecord{
			{
				JobName:        "summary_compaction",
				Scope:          memory.Scope{Tenant: "tenant-a", Project: "project-a", Namespace: "namespace-a"},
				TriggerSource:  "scheduler",
				IdempotencyKey: "run-a",
				Status:         jobs.JobExecutionStatusCompleted,
				Attempt:        1,
				ProcessedCount: 1,
				StartedAt:      time.Date(2026, 6, 7, 11, 0, 0, 0, time.UTC),
				FinishedAt:     time.Date(2026, 6, 7, 11, 0, 1, 0, time.UTC),
			},
		},
	}

	handler := NewHTTPHandler(HTTPDependencies{
		AdminAPIKeys:     map[string]struct{}{"admin-key": {}},
		JobExecutionRead: reader,
	})

	req := httptest.NewRequest(http.MethodGet, "/v1/admin/jobs/status?limit=5", nil)
	req.Header.Set("X-API-Key", "admin-key")
	req.Header.Set("X-Stele-Tenant", "tenant-a")
	req.Header.Set("X-Stele-Project", "project-a")
	req.Header.Set("X-Stele-Namespace", "namespace-a")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}

	var payload struct {
		Executions []jobs.JobExecutionRecord `json:"executions"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&payload); err != nil {
		t.Fatalf("json.NewDecoder() error = %v", err)
	}

	if len(payload.Executions) != 1 || payload.Executions[0].JobName != "summary_compaction" {
		t.Fatalf("executions = %+v, want one summary compaction record", payload.Executions)
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
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}

	if service.gotInput.Action != policy.ForgettingActionSuppress {
		t.Fatalf("action = %q, want suppress", service.gotInput.Action)
	}

	if service.gotInput.Actor != "operator-a" {
		t.Fatalf("actor = %q, want operator-a", service.gotInput.Actor)
	}

	if service.gotInput.Reason != "manual override" {
		t.Fatalf("reason = %q, want manual override", service.gotInput.Reason)
	}
}

func TestNewHTTPHandlerCreatesAdminMemory(t *testing.T) {
	service := &stubManualMutationService{
		resource: memory.MemoryResource{
			ID:      "mem_123",
			Scope:   memory.Scope{Tenant: "tenant-a", Project: "project-a", Namespace: "namespace-a"},
			Class:   memory.MemoryClassProfile,
			State:   memory.MemoryStateActive,
			Content: "seed knowledge",
		},
	}
	handler := NewHTTPHandler(HTTPDependencies{
		AdminAPIKeys:         map[string]struct{}{"admin-key": {}},
		MemoryManualMutation: service,
	})

	req := httptest.NewRequest(http.MethodPost, "/v1/admin/memories", strings.NewReader(`{"class":"profile","content":"seed knowledge","reason":"seed"}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-API-Key", "admin-key")
	req.Header.Set("X-Stele-Actor", "operator-a")
	req.Header.Set("X-Stele-Tenant", "tenant-a")
	req.Header.Set("X-Stele-Project", "project-a")
	req.Header.Set("X-Stele-Namespace", "namespace-a")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusCreated)
	}
	if service.gotCreateInput.Class != memory.MemoryClassProfile {
		t.Fatalf("class = %q, want profile", service.gotCreateInput.Class)
	}
	if service.gotCreateInput.Actor != "operator-a" {
		t.Fatalf("actor = %q, want operator-a", service.gotCreateInput.Actor)
	}
}

func TestNewHTTPHandlerUpdatesAdminMemory(t *testing.T) {
	service := &stubManualMutationService{
		resource: memory.MemoryResource{
			ID:      "mem_123",
			Scope:   memory.Scope{Tenant: "tenant-a", Project: "project-a", Namespace: "namespace-a"},
			Class:   memory.MemoryClassProfile,
			State:   memory.MemoryStateActive,
			Content: "corrected knowledge",
		},
	}
	handler := NewHTTPHandler(HTTPDependencies{
		AdminAPIKeys:         map[string]struct{}{"admin-key": {}},
		MemoryManualMutation: service,
	})

	req := httptest.NewRequest(http.MethodPatch, "/v1/admin/memories/mem_123", strings.NewReader(`{"content":"corrected knowledge","expected_version":2,"reason":"correct"}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-API-Key", "admin-key")
	req.Header.Set("X-Stele-Actor", "operator-a")
	req.Header.Set("X-Stele-Tenant", "tenant-a")
	req.Header.Set("X-Stele-Project", "project-a")
	req.Header.Set("X-Stele-Namespace", "namespace-a")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
	if service.gotUpdateInput.MemoryID != "mem_123" {
		t.Fatalf("memory id = %q, want mem_123", service.gotUpdateInput.MemoryID)
	}
	if service.gotUpdateInput.ExpectedVersion != 2 {
		t.Fatalf("expected version = %d, want 2", service.gotUpdateInput.ExpectedVersion)
	}
}

func TestNewHTTPHandlerMergesAdminMemory(t *testing.T) {
	service := &stubManualMutationService{
		resource: memory.MemoryResource{
			ID:      "mem_target",
			Scope:   memory.Scope{Tenant: "tenant-a", Project: "project-a", Namespace: "namespace-a"},
			Class:   memory.MemoryClassProfile,
			State:   memory.MemoryStateActive,
			Content: "merged knowledge",
		},
	}
	handler := NewHTTPHandler(HTTPDependencies{
		AdminAPIKeys:         map[string]struct{}{"admin-key": {}},
		MemoryManualMutation: service,
	})

	req := httptest.NewRequest(http.MethodPost, "/v1/admin/memories/mem_target:merge", strings.NewReader(`{"source_memory_id":"mem_source","content":"merged knowledge","expected_version":3,"reason":"dedupe"}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-API-Key", "admin-key")
	req.Header.Set("X-Stele-Actor", "operator-a")
	req.Header.Set("X-Stele-Tenant", "tenant-a")
	req.Header.Set("X-Stele-Project", "project-a")
	req.Header.Set("X-Stele-Namespace", "namespace-a")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
	if service.gotMergeInput.TargetMemoryID != "mem_target" {
		t.Fatalf("target memory id = %q, want mem_target", service.gotMergeInput.TargetMemoryID)
	}
	if service.gotMergeInput.SourceMemoryID != "mem_source" {
		t.Fatalf("source memory id = %q, want mem_source", service.gotMergeInput.SourceMemoryID)
	}
}

func TestNewHTTPHandlerReclassifiesAdminMemory(t *testing.T) {
	service := &stubManualMutationService{
		resource: memory.MemoryResource{
			ID:      "mem_123",
			Scope:   memory.Scope{Tenant: "tenant-a", Project: "project-a", Namespace: "namespace-a"},
			Class:   memory.MemoryClassProcedural,
			State:   memory.MemoryStateActive,
			Content: "respond concisely",
		},
	}
	handler := NewHTTPHandler(HTTPDependencies{
		AdminAPIKeys:         map[string]struct{}{"admin-key": {}},
		MemoryManualMutation: service,
	})

	req := httptest.NewRequest(http.MethodPost, "/v1/admin/memories/mem_123:reclassify", strings.NewReader(`{"target_class":"procedural","expected_version":4,"reason":"fix class"}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-API-Key", "admin-key")
	req.Header.Set("X-Stele-Actor", "operator-a")
	req.Header.Set("X-Stele-Tenant", "tenant-a")
	req.Header.Set("X-Stele-Project", "project-a")
	req.Header.Set("X-Stele-Namespace", "namespace-a")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
	if service.gotReclassifyInput.MemoryID != "mem_123" {
		t.Fatalf("memory id = %q, want mem_123", service.gotReclassifyInput.MemoryID)
	}
	if service.gotReclassifyInput.TargetClass != memory.MemoryClassProcedural {
		t.Fatalf("target class = %q, want procedural", service.gotReclassifyInput.TargetClass)
	}
}
