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

	"github.com/FelixSeptem/stele/internal/memory"
	"github.com/FelixSeptem/stele/internal/retrieval"
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
