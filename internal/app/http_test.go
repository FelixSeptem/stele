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

	"github.com/FelixSeptem/stele/internal/governance"
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

type stubEmbeddingAdminService struct {
	gotListInput    memory.ListEmbeddingRebuildsInput
	gotReadScope    memory.Scope
	gotReadMemoryID string
	gotApplyInput   memory.ApplyEmbeddingRecoveryInput
	page            memory.EmbeddingRebuildPage
	inspection      memory.EmbeddingMemoryInspection
	outcome         memory.EmbeddingRecoveryOutcome
	listErr         error
	readErr         error
	applyErr        error
	validateList    bool
}

func (s *stubEmbeddingAdminService) ListEmbeddingRebuilds(ctx context.Context, input memory.ListEmbeddingRebuildsInput) (memory.EmbeddingRebuildPage, error) {
	s.gotListInput = input
	if s.validateList {
		if err := input.Validate(); err != nil {
			return memory.EmbeddingRebuildPage{}, err
		}
	}
	return s.page, s.listErr
}

func (s *stubEmbeddingAdminService) GetMemoryEmbedding(ctx context.Context, scope memory.Scope, memoryID string) (memory.EmbeddingMemoryInspection, error) {
	s.gotReadScope = scope
	s.gotReadMemoryID = memoryID
	return s.inspection, s.readErr
}

func (s *stubEmbeddingAdminService) ApplyEmbeddingRecovery(ctx context.Context, input memory.ApplyEmbeddingRecoveryInput) (memory.EmbeddingRecoveryOutcome, error) {
	s.gotApplyInput = input
	return s.outcome, s.applyErr
}

type stubGovernanceAdminService struct {
	gotListInput    governance.ListGovernanceRawEventsInput
	gotReadInput    governance.ReadGovernanceRawEventInput
	gotHistoryInput governance.ListGovernanceRecoveryHistoryInput
	gotApplyInput   governance.ApplyGovernanceRecoveryInput

	page    governance.GovernanceRawEventPage
	event   governance.GovernanceRawEvent
	history []governance.GovernanceRecoveryRecord
	outcome governance.GovernanceRecoveryOutcome

	listErr    error
	readErr    error
	historyErr error
	applyErr   error

	validateList  bool
	validateRead  bool
	validateApply bool
}

func (s *stubGovernanceAdminService) ListGovernanceRawEvents(ctx context.Context, input governance.ListGovernanceRawEventsInput) (governance.GovernanceRawEventPage, error) {
	s.gotListInput = input
	if s.validateList {
		if err := input.Validate(); err != nil {
			return governance.GovernanceRawEventPage{}, err
		}
	}
	if s.listErr != nil {
		return governance.GovernanceRawEventPage{}, s.listErr
	}

	return s.page, nil
}

func (s *stubGovernanceAdminService) ReadGovernanceRawEvent(ctx context.Context, input governance.ReadGovernanceRawEventInput) (governance.GovernanceRawEvent, error) {
	s.gotReadInput = input
	if s.validateRead {
		if err := input.Validate(); err != nil {
			return governance.GovernanceRawEvent{}, err
		}
	}
	if s.readErr != nil {
		return governance.GovernanceRawEvent{}, s.readErr
	}

	return s.event, nil
}

func (s *stubGovernanceAdminService) ListGovernanceRecoveryHistory(ctx context.Context, input governance.ListGovernanceRecoveryHistoryInput) ([]governance.GovernanceRecoveryRecord, error) {
	s.gotHistoryInput = input
	if s.historyErr != nil {
		return nil, s.historyErr
	}

	return s.history, nil
}

func (s *stubGovernanceAdminService) ApplyGovernanceRecovery(ctx context.Context, input governance.ApplyGovernanceRecoveryInput) (governance.GovernanceRecoveryOutcome, error) {
	s.gotApplyInput = input
	if s.validateApply {
		if err := input.Validate(); err != nil {
			return governance.GovernanceRecoveryOutcome{}, err
		}
	}
	if s.applyErr != nil {
		return governance.GovernanceRecoveryOutcome{}, s.applyErr
	}

	return s.outcome, nil
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

func TestNewHTTPHandlerListsAdminGovernanceRawEvents(t *testing.T) {
	now := time.Date(2026, 6, 12, 2, 0, 0, 0, time.UTC)
	cursor := governance.GovernanceRawEventCursor{
		CreatedAt:  now.Add(-time.Minute),
		RawEventID: "evt_prev",
	}.Encode()
	service := &stubGovernanceAdminService{
		validateList: true,
		page: governance.GovernanceRawEventPage{
			Items: []governance.GovernanceRawEvent{
				{
					ID:           "evt_123",
					Scope:        memory.Scope{Tenant: "tenant-a", Project: "project-a", Namespace: "namespace-a"},
					EventType:    "conversation.message",
					Content:      "retry later",
					CreatedAt:    now.Add(-2 * time.Minute),
					State:        governance.GovernanceRawEventStateRetryWait,
					Attempt:      2,
					LastFailedAt: now.Add(-time.Minute),
					LastError:    "timeout",
				},
			},
			NextCursor: "next_cursor",
		},
	}
	handler := NewHTTPHandler(HTTPDependencies{
		AdminAPIKeys:    map[string]struct{}{"admin-key": {}},
		GovernanceAdmin: service,
	})

	req := httptest.NewRequest(
		http.MethodGet,
		"/v1/admin/governance/raw-events?state=retry_wait&event_type=conversation.message&attempt_gte=1&attempt_lte=3&failed_from=2026-06-12T01:00:00Z&failed_to=2026-06-12T02:00:00Z&next_attempt_from=2026-06-12T02:00:00Z&next_attempt_to=2026-06-12T03:00:00Z&limit=5&cursor="+cursor,
		nil,
	)
	setAdminScopeHeaders(req)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
	if service.gotListInput.Scope.Namespace != "namespace-a" {
		t.Fatalf("scope = %+v, want resolved request scope", service.gotListInput.Scope)
	}
	if service.gotListInput.State != governance.GovernanceRawEventStateRetryWait {
		t.Fatalf("state = %q, want retry_wait", service.gotListInput.State)
	}
	if service.gotListInput.EventType != "conversation.message" {
		t.Fatalf("event type = %q, want conversation.message", service.gotListInput.EventType)
	}
	if service.gotListInput.AttemptGTE == nil || *service.gotListInput.AttemptGTE != 1 {
		t.Fatalf("attempt_gte = %v, want 1", service.gotListInput.AttemptGTE)
	}
	if service.gotListInput.AttemptLTE == nil || *service.gotListInput.AttemptLTE != 3 {
		t.Fatalf("attempt_lte = %v, want 3", service.gotListInput.AttemptLTE)
	}
	if service.gotListInput.Limit != 5 {
		t.Fatalf("limit = %d, want 5", service.gotListInput.Limit)
	}
	if service.gotListInput.Cursor != cursor {
		t.Fatalf("cursor = %q, want %q", service.gotListInput.Cursor, cursor)
	}

	var payload governance.GovernanceRawEventPage
	if err := json.NewDecoder(rec.Body).Decode(&payload); err != nil {
		t.Fatalf("json.NewDecoder() error = %v", err)
	}
	if len(payload.Items) != 1 || payload.Items[0].ID != "evt_123" {
		t.Fatalf("items = %+v, want one raw event", payload.Items)
	}
	if payload.NextCursor != "next_cursor" {
		t.Fatalf("next cursor = %q, want next_cursor", payload.NextCursor)
	}
}

func TestNewHTTPHandlerReturnsAdminGovernanceRawEventDetail(t *testing.T) {
	now := time.Date(2026, 6, 12, 2, 0, 0, 0, time.UTC)
	service := &stubGovernanceAdminService{
		validateRead: true,
		event: governance.GovernanceRawEvent{
			ID:            "evt_123",
			Scope:         memory.Scope{Tenant: "tenant-a", Project: "project-a", Namespace: "namespace-a"},
			EventType:     "conversation.message",
			Content:       "retry later",
			CreatedAt:     now.Add(-2 * time.Minute),
			State:         governance.GovernanceRawEventStateRetryWait,
			Attempt:       2,
			LastFailedAt:  now.Add(-time.Minute),
			LastError:     "timeout",
			NextAttemptAt: now.Add(10 * time.Minute),
		},
	}
	handler := NewHTTPHandler(HTTPDependencies{
		AdminAPIKeys:    map[string]struct{}{"admin-key": {}},
		GovernanceAdmin: service,
	})

	req := httptest.NewRequest(http.MethodGet, "/v1/admin/governance/raw-events/evt_123", nil)
	setAdminScopeHeaders(req)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
	if service.gotReadInput.RawEventID != "evt_123" {
		t.Fatalf("raw event id = %q, want evt_123", service.gotReadInput.RawEventID)
	}

	var payload governance.GovernanceRawEvent
	if err := json.NewDecoder(rec.Body).Decode(&payload); err != nil {
		t.Fatalf("json.NewDecoder() error = %v", err)
	}
	if payload.ID != "evt_123" || payload.State != governance.GovernanceRawEventStateRetryWait {
		t.Fatalf("payload = %+v, want detail response", payload)
	}
}

func TestNewHTTPHandlerReturnsAdminGovernanceRecoveryHistory(t *testing.T) {
	service := &stubGovernanceAdminService{
		history: []governance.GovernanceRecoveryRecord{
			{
				ID:         "grl_1",
				RawEventID: "evt_123",
				Scope:      memory.Scope{Tenant: "tenant-a", Project: "project-a", Namespace: "namespace-a"},
				Action:     governance.GovernanceRecoveryActionRetry,
				Actor:      "operator-a",
				Reason:     "retry now",
				Before: governance.GovernanceRecoverySnapshot{
					State:   governance.GovernanceRawEventStateRetryWait,
					Attempt: 2,
				},
				After: governance.GovernanceRecoverySnapshot{
					State:   governance.GovernanceRawEventStatePending,
					Attempt: 2,
				},
				OccurredAt: time.Date(2026, 6, 12, 2, 5, 0, 0, time.UTC),
			},
		},
	}
	handler := NewHTTPHandler(HTTPDependencies{
		AdminAPIKeys:    map[string]struct{}{"admin-key": {}},
		GovernanceAdmin: service,
	})

	req := httptest.NewRequest(http.MethodGet, "/v1/admin/governance/raw-events/evt_123/recovery-history", nil)
	setAdminScopeHeaders(req)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
	if service.gotHistoryInput.RawEventID != "evt_123" {
		t.Fatalf("raw event id = %q, want evt_123", service.gotHistoryInput.RawEventID)
	}

	var payload struct {
		History []governance.GovernanceRecoveryRecord `json:"history"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&payload); err != nil {
		t.Fatalf("json.NewDecoder() error = %v", err)
	}
	if len(payload.History) != 1 || payload.History[0].Action != governance.GovernanceRecoveryActionRetry {
		t.Fatalf("history = %+v, want one retry record", payload.History)
	}
}

func TestNewHTTPHandlerAppliesAdminGovernanceRecoveryActions(t *testing.T) {
	now := time.Date(2026, 6, 12, 2, 0, 0, 0, time.UTC)

	tests := []struct {
		name         string
		path         string
		body         string
		wantAction   governance.GovernanceRecoveryAction
		wantSchedule time.Time
	}{
		{
			name:       "retry",
			path:       "/v1/admin/governance/raw-events/evt_123:retry",
			body:       `{"reason":"retry now"}`,
			wantAction: governance.GovernanceRecoveryActionRetry,
		},
		{
			name:         "reschedule",
			path:         "/v1/admin/governance/raw-events/evt_123:reschedule",
			body:         `{"reason":"delay until quiet hours","scheduled_for":"2099-06-12T03:00:00Z"}`,
			wantAction:   governance.GovernanceRecoveryActionReschedule,
			wantSchedule: time.Date(2099, 6, 12, 3, 0, 0, 0, time.UTC),
		},
		{
			name:       "requeue",
			path:       "/v1/admin/governance/raw-events/evt_123:requeue",
			body:       `{"reason":"clear exhausted state"}`,
			wantAction: governance.GovernanceRecoveryActionRequeue,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			service := &stubGovernanceAdminService{
				validateApply: true,
				outcome: governance.GovernanceRecoveryOutcome{
					RawEvent: governance.GovernanceRawEvent{
						ID:      "evt_123",
						Scope:   memory.Scope{Tenant: "tenant-a", Project: "project-a", Namespace: "namespace-a"},
						State:   governance.GovernanceRawEventStatePending,
						Attempt: 2,
					},
					Recovery: governance.GovernanceRecoveryRecord{
						ID:         "grl_1",
						RawEventID: "evt_123",
						Scope:      memory.Scope{Tenant: "tenant-a", Project: "project-a", Namespace: "namespace-a"},
						Action:     tt.wantAction,
						Actor:      "operator-a",
						Reason:     "operator request",
						OccurredAt: now,
					},
				},
			}
			handler := NewHTTPHandler(HTTPDependencies{
				AdminAPIKeys:    map[string]struct{}{"admin-key": {}},
				GovernanceAdmin: service,
			})

			req := httptest.NewRequest(http.MethodPost, tt.path, strings.NewReader(tt.body))
			setAdminActionHeaders(req)
			rec := httptest.NewRecorder()

			handler.ServeHTTP(rec, req)

			if rec.Code != http.StatusOK {
				t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
			}
			if service.gotApplyInput.RawEventID != "evt_123" {
				t.Fatalf("raw event id = %q, want evt_123", service.gotApplyInput.RawEventID)
			}
			if service.gotApplyInput.Action != tt.wantAction {
				t.Fatalf("action = %q, want %q", service.gotApplyInput.Action, tt.wantAction)
			}
			if service.gotApplyInput.Actor != "operator-a" {
				t.Fatalf("actor = %q, want operator-a", service.gotApplyInput.Actor)
			}
			if tt.wantAction == governance.GovernanceRecoveryActionReschedule && !service.gotApplyInput.ScheduledFor.Equal(tt.wantSchedule) {
				t.Fatalf("scheduled_for = %v, want %v", service.gotApplyInput.ScheduledFor, tt.wantSchedule)
			}
		})
	}
}

func TestNewHTTPHandlerValidatesAdminGovernanceRequests(t *testing.T) {
	tests := []struct {
		name       string
		method     string
		path       string
		body       string
		headers    func(*http.Request)
		wantStatus int
		wantBody   string
	}{
		{
			name:   "missing actor on action",
			method: http.MethodPost,
			path:   "/v1/admin/governance/raw-events/evt_123:retry",
			body:   `{"reason":"retry now"}`,
			headers: func(req *http.Request) {
				setAdminScopeHeaders(req)
				req.Header.Set("Content-Type", "application/json")
			},
			wantStatus: http.StatusBadRequest,
			wantBody:   "actor is required",
		},
		{
			name:   "invalid scheduled_for",
			method: http.MethodPost,
			path:   "/v1/admin/governance/raw-events/evt_123:reschedule",
			body:   `{"reason":"delay","scheduled_for":"not-a-time"}`,
			headers: func(req *http.Request) {
				setAdminActionHeaders(req)
			},
			wantStatus: http.StatusBadRequest,
			wantBody:   "invalid scheduled_for",
		},
		{
			name:   "invalid action target",
			method: http.MethodPost,
			path:   "/v1/admin/governance/raw-events/retry",
			body:   `{"reason":"retry now"}`,
			headers: func(req *http.Request) {
				setAdminActionHeaders(req)
			},
			wantStatus: http.StatusBadRequest,
			wantBody:   "invalid governance raw event action target",
		},
		{
			name:   "invalid state filter",
			method: http.MethodGet,
			path:   "/v1/admin/governance/raw-events?state=bogus",
			headers: func(req *http.Request) {
				setAdminScopeHeaders(req)
			},
			wantStatus: http.StatusBadRequest,
			wantBody:   "state \"bogus\" is invalid",
		},
		{
			name:   "invalid attempt filter",
			method: http.MethodGet,
			path:   "/v1/admin/governance/raw-events?attempt_gte=nope",
			headers: func(req *http.Request) {
				setAdminScopeHeaders(req)
			},
			wantStatus: http.StatusBadRequest,
			wantBody:   "invalid attempt_gte",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			handler := NewHTTPHandler(HTTPDependencies{
				AdminAPIKeys: map[string]struct{}{"admin-key": {}},
				GovernanceAdmin: &stubGovernanceAdminService{
					validateList:  true,
					validateApply: true,
				},
			})

			req := httptest.NewRequest(tt.method, tt.path, strings.NewReader(tt.body))
			tt.headers(req)
			rec := httptest.NewRecorder()

			handler.ServeHTTP(rec, req)

			if rec.Code != tt.wantStatus {
				t.Fatalf("status = %d, want %d", rec.Code, tt.wantStatus)
			}
			if body := rec.Body.String(); !strings.Contains(body, tt.wantBody) {
				t.Fatalf("body = %q, want substring %q", body, tt.wantBody)
			}
		})
	}
}

func TestNewHTTPHandlerMapsAdminGovernanceErrors(t *testing.T) {
	tests := []struct {
		name       string
		method     string
		path       string
		body       string
		service    *stubGovernanceAdminService
		wantStatus int
		wantBody   string
	}{
		{
			name:       "detail not found",
			method:     http.MethodGet,
			path:       "/v1/admin/governance/raw-events/evt_missing",
			service:    &stubGovernanceAdminService{readErr: pgx.ErrNoRows, validateRead: true},
			wantStatus: http.StatusNotFound,
			wantBody:   "raw event not found",
		},
		{
			name:       "recovery history not found",
			method:     http.MethodGet,
			path:       "/v1/admin/governance/raw-events/evt_missing/recovery-history",
			service:    &stubGovernanceAdminService{historyErr: pgx.ErrNoRows},
			wantStatus: http.StatusNotFound,
			wantBody:   "raw event not found",
		},
		{
			name:       "recovery conflict",
			method:     http.MethodPost,
			path:       "/v1/admin/governance/raw-events/evt_123:retry",
			body:       `{"reason":"retry now"}`,
			service:    &stubGovernanceAdminService{applyErr: governance.ErrGovernanceRecoveryConflict, validateApply: true},
			wantStatus: http.StatusConflict,
			wantBody:   governance.ErrGovernanceRecoveryConflict.Error(),
		},
		{
			name:       "recovery rejected",
			method:     http.MethodPost,
			path:       "/v1/admin/governance/raw-events/evt_123:retry",
			body:       `{"reason":"retry now"}`,
			service:    &stubGovernanceAdminService{applyErr: governance.ErrGovernanceRecoveryRejected, validateApply: true},
			wantStatus: http.StatusUnprocessableEntity,
			wantBody:   governance.ErrGovernanceRecoveryRejected.Error(),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			handler := NewHTTPHandler(HTTPDependencies{
				AdminAPIKeys:    map[string]struct{}{"admin-key": {}},
				GovernanceAdmin: tt.service,
			})

			req := httptest.NewRequest(tt.method, tt.path, strings.NewReader(tt.body))
			if tt.method == http.MethodGet {
				setAdminScopeHeaders(req)
			} else {
				setAdminActionHeaders(req)
			}
			rec := httptest.NewRecorder()

			handler.ServeHTTP(rec, req)

			if rec.Code != tt.wantStatus {
				t.Fatalf("status = %d, want %d", rec.Code, tt.wantStatus)
			}
			if body := rec.Body.String(); !strings.Contains(body, tt.wantBody) {
				t.Fatalf("body = %q, want substring %q", body, tt.wantBody)
			}
		})
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

func TestNewHTTPHandlerListsAdminEmbeddingRebuilds(t *testing.T) {
	service := &stubEmbeddingAdminService{
		validateList: true,
		page: memory.EmbeddingRebuildPage{
			Runtime: memory.EmbeddingRuntimeStatus{
				Configured:             false,
				SemanticRebuildEnabled: false,
				Reason:                 "semantic rebuild execution is inactive because no embedding routes are configured",
			},
			Items: []memory.EmbeddingRebuildView{
				{
					MemoryID:            "mem_123",
					Scope:               memory.Scope{Tenant: "tenant-a", Project: "project-a", Namespace: "namespace-a"},
					Class:               memory.MemoryClassProfile,
					State:               memory.MemoryStateActive,
					Status:              memory.EmbeddingRebuildStatusFailed,
					RequestedProvider:   "openai",
					RequestedModel:      "text-embedding-3-small",
					RequestedDimensions: 1536,
					FailureReason:       "provider unavailable",
					Drifted:             true,
				},
			},
		},
	}
	handler := NewHTTPHandler(HTTPDependencies{
		AdminAPIKeys:       map[string]struct{}{"admin-key": {}},
		EmbeddingAdminRead: service,
	})

	req := httptest.NewRequest(http.MethodGet, "/v1/admin/embedding/rebuilds?status=failed&requested_provider=openai&requested_model=text-embedding-3-small&drifted=true&limit=5", nil)
	setAdminScopeHeaders(req)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
	if service.gotListInput.Status != memory.EmbeddingRebuildStatusFailed {
		t.Fatalf("status filter = %q, want failed", service.gotListInput.Status)
	}
	if service.gotListInput.RequestedProvider != "openai" {
		t.Fatalf("provider filter = %q, want openai", service.gotListInput.RequestedProvider)
	}
	if service.gotListInput.Drifted == nil || !*service.gotListInput.Drifted {
		t.Fatalf("drifted filter = %v, want true", service.gotListInput.Drifted)
	}

	var payload memory.EmbeddingRebuildPage
	if err := json.NewDecoder(rec.Body).Decode(&payload); err != nil {
		t.Fatalf("json.NewDecoder() error = %v", err)
	}
	if payload.Runtime.SemanticRebuildEnabled {
		t.Fatal("payload.Runtime.SemanticRebuildEnabled = true, want false")
	}
	if len(payload.Items) != 1 || !payload.Items[0].Drifted {
		t.Fatalf("items = %+v, want one drifted rebuild item", payload.Items)
	}
}

func TestNewHTTPHandlerReturnsAdminMemoryEmbeddingInspection(t *testing.T) {
	service := &stubEmbeddingAdminService{
		inspection: memory.EmbeddingMemoryInspection{
			Runtime: memory.EmbeddingRuntimeStatus{
				Configured:             true,
				SemanticRebuildEnabled: true,
				RegisteredProviders:    []string{"openai"},
			},
			Memory: memory.EmbeddingMemorySummary{
				ID:                   "mem_123",
				Scope:                memory.Scope{Tenant: "tenant-a", Project: "project-a", Namespace: "namespace-a"},
				Class:                memory.MemoryClassProfile,
				State:                memory.MemoryStateActive,
				CurrentSourceVersion: 3,
				CurrentContentHash:   "hash_123",
			},
			Rebuild: memory.EmbeddingRebuildView{
				MemoryID:             "mem_123",
				Scope:                memory.Scope{Tenant: "tenant-a", Project: "project-a", Namespace: "namespace-a"},
				Status:               memory.EmbeddingRebuildStatusCurrent,
				RequestedProvider:    "openai",
				RequestedModel:       "text-embedding-3-small",
				RequestedDimensions:  1536,
				ActiveVectorRevision: "vec_active",
			},
			Revisions: []memory.EmbeddingVectorRevisionView{
				{
					ID:            "vec_active",
					Provider:      "openai",
					Model:         "text-embedding-3-small",
					Dimensions:    1536,
					Status:        memory.VectorRevisionStatusActive,
					SourceVersion: 3,
				},
			},
		},
	}
	handler := NewHTTPHandler(HTTPDependencies{
		AdminAPIKeys:       map[string]struct{}{"admin-key": {}},
		EmbeddingAdminRead: service,
	})

	req := httptest.NewRequest(http.MethodGet, "/v1/admin/memories/mem_123/embedding", nil)
	setAdminScopeHeaders(req)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
	if service.gotReadMemoryID != "mem_123" {
		t.Fatalf("memory id = %q, want mem_123", service.gotReadMemoryID)
	}

	var payload memory.EmbeddingMemoryInspection
	if err := json.NewDecoder(rec.Body).Decode(&payload); err != nil {
		t.Fatalf("json.NewDecoder() error = %v", err)
	}
	if payload.Memory.ID != "mem_123" {
		t.Fatalf("payload.Memory.ID = %q, want mem_123", payload.Memory.ID)
	}
	if len(payload.Revisions) != 1 || payload.Revisions[0].ID != "vec_active" {
		t.Fatalf("revisions = %+v, want one active revision", payload.Revisions)
	}
}

func TestNewHTTPHandlerValidatesAdminEmbeddingInspectionRequests(t *testing.T) {
	tests := []struct {
		name       string
		path       string
		wantStatus int
		wantBody   string
	}{
		{
			name:       "invalid status filter",
			path:       "/v1/admin/embedding/rebuilds?status=bogus",
			wantStatus: http.StatusBadRequest,
			wantBody:   "embedding rebuild status \"bogus\" is invalid",
		},
		{
			name:       "invalid drifted filter",
			path:       "/v1/admin/embedding/rebuilds?drifted=maybe",
			wantStatus: http.StatusBadRequest,
			wantBody:   "invalid drifted",
		},
		{
			name:       "invalid limit",
			path:       "/v1/admin/embedding/rebuilds?limit=0",
			wantStatus: http.StatusBadRequest,
			wantBody:   "invalid limit",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			handler := NewHTTPHandler(HTTPDependencies{
				AdminAPIKeys:       map[string]struct{}{"admin-key": {}},
				EmbeddingAdminRead: &stubEmbeddingAdminService{validateList: true},
			})

			req := httptest.NewRequest(http.MethodGet, tt.path, nil)
			setAdminScopeHeaders(req)
			rec := httptest.NewRecorder()

			handler.ServeHTTP(rec, req)

			if rec.Code != tt.wantStatus {
				t.Fatalf("status = %d, want %d", rec.Code, tt.wantStatus)
			}
			if body := rec.Body.String(); !strings.Contains(body, tt.wantBody) {
				t.Fatalf("body = %q, want substring %q", body, tt.wantBody)
			}
		})
	}
}

func TestNewHTTPHandlerAppliesAdminEmbeddingRecoveryActions(t *testing.T) {
	tests := []struct {
		name       string
		path       string
		wantAction memory.EmbeddingRecoveryAction
	}{
		{
			name:       "retry",
			path:       "/v1/admin/embedding/rebuilds/mem_123:retry",
			wantAction: memory.EmbeddingRecoveryActionRetry,
		},
		{
			name:       "requeue",
			path:       "/v1/admin/embedding/rebuilds/mem_123:requeue",
			wantAction: memory.EmbeddingRecoveryActionRequeue,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			service := &stubEmbeddingAdminService{
				outcome: memory.EmbeddingRecoveryOutcome{
					Rebuild: memory.EmbeddingRebuildView{
						MemoryID: "mem_123",
						Status:   memory.EmbeddingRebuildStatusPending,
					},
					Recovery: memory.EmbeddingRecoveryRecord{
						ID:         "erl_1",
						MemoryID:   "mem_123",
						Scope:      memory.Scope{Tenant: "tenant-a", Project: "project-a", Namespace: "namespace-a"},
						Action:     tt.wantAction,
						Actor:      "operator-a",
						Reason:     "operator request",
						OccurredAt: time.Date(2026, 6, 28, 12, 0, 0, 0, time.UTC),
					},
				},
			}
			handler := NewHTTPHandler(HTTPDependencies{
				AdminAPIKeys:       map[string]struct{}{"admin-key": {}},
				EmbeddingAdminRead: service,
			})

			req := httptest.NewRequest(http.MethodPost, tt.path, strings.NewReader(`{"reason":"operator request"}`))
			setAdminActionHeaders(req)
			rec := httptest.NewRecorder()

			handler.ServeHTTP(rec, req)

			if rec.Code != http.StatusOK {
				t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
			}
			if service.gotApplyInput.Action != tt.wantAction {
				t.Fatalf("action = %q, want %q", service.gotApplyInput.Action, tt.wantAction)
			}
			if service.gotApplyInput.Actor != "operator-a" {
				t.Fatalf("actor = %q, want operator-a", service.gotApplyInput.Actor)
			}
		})
	}
}

func TestNewHTTPHandlerMapsAdminEmbeddingRecoveryErrors(t *testing.T) {
	tests := []struct {
		name       string
		path       string
		body       string
		service    *stubEmbeddingAdminService
		wantStatus int
		wantBody   string
	}{
		{
			name:       "conflict",
			path:       "/v1/admin/embedding/rebuilds/mem_123:retry",
			body:       `{"reason":"retry now"}`,
			service:    &stubEmbeddingAdminService{applyErr: memory.ErrEmbeddingRecoveryConflict},
			wantStatus: http.StatusConflict,
			wantBody:   memory.ErrEmbeddingRecoveryConflict.Error(),
		},
		{
			name:       "invalid target",
			path:       "/v1/admin/embedding/rebuilds/retry",
			body:       `{"reason":"retry now"}`,
			service:    &stubEmbeddingAdminService{},
			wantStatus: http.StatusBadRequest,
			wantBody:   "invalid embedding rebuild action target",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			handler := NewHTTPHandler(HTTPDependencies{
				AdminAPIKeys:       map[string]struct{}{"admin-key": {}},
				EmbeddingAdminRead: tt.service,
			})

			req := httptest.NewRequest(http.MethodPost, tt.path, strings.NewReader(tt.body))
			setAdminActionHeaders(req)
			rec := httptest.NewRecorder()

			handler.ServeHTTP(rec, req)

			if rec.Code != tt.wantStatus {
				t.Fatalf("status = %d, want %d", rec.Code, tt.wantStatus)
			}
			if body := rec.Body.String(); !strings.Contains(body, tt.wantBody) {
				t.Fatalf("body = %q, want substring %q", body, tt.wantBody)
			}
		})
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

func setAdminScopeHeaders(req *http.Request) {
	req.Header.Set("X-API-Key", "admin-key")
	req.Header.Set("X-Stele-Tenant", "tenant-a")
	req.Header.Set("X-Stele-Project", "project-a")
	req.Header.Set("X-Stele-Namespace", "namespace-a")
}

func setAdminActionHeaders(req *http.Request) {
	setAdminScopeHeaders(req)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Stele-Actor", "operator-a")
}
