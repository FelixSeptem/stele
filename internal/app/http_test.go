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
		Readiness:  stubReadinessChecker{},
		APIKeys:    map[string]struct{}{"test-key": {}},
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
		Readiness:    stubReadinessChecker{},
		APIKeys:      map[string]struct{}{"test-key": {}},
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
		Readiness:    stubReadinessChecker{},
		APIKeys:      map[string]struct{}{"test-key": {}},
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
