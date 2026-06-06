package app

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"math/rand"
	"net/http"
	"time"

	"github.com/FelixSeptem/stele/internal/auth"
	"github.com/FelixSeptem/stele/internal/memory"
	"github.com/FelixSeptem/stele/internal/retrieval"
)

type ReadinessChecker interface {
	Ready(ctx context.Context) error
}

type HTTPDependencies struct {
	Readiness        ReadinessChecker
	APIKeys          auth.StaticAPIKeys
	EventIngestor    memory.EventIngestor
	MemorySearcher   retrieval.MemorySearcher
	ContextAssembler retrieval.ContextAssembler
	Logger           *log.Logger
}

type eventIngestRequest struct {
	EventType       string         `json:"event_type"`
	Content         string         `json:"content"`
	Metadata        map[string]any `json:"metadata"`
	SourceTimestamp string         `json:"source_timestamp"`
}

type eventIngestResponse struct {
	EventID string `json:"event_id"`
}

type memorySearchRequest struct {
	Query            string               `json:"query"`
	QueryEmbedding   []float32            `json:"query_embedding"`
	Classes          []memory.MemoryClass `json:"classes"`
	TimeFrom         string               `json:"time_from"`
	TimeTo           string               `json:"time_to"`
	TopK             int                  `json:"top_k"`
	IncludeSummaries bool                 `json:"include_summaries"`
	IncludeRelations bool                 `json:"include_relations"`
}

type contextAssembleRequest struct {
	Query            string `json:"query"`
	Budget           int    `json:"budget"`
	IncludeRelations bool   `json:"include_relations"`
}

func NewHTTPHandler(deps HTTPDependencies) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /health", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	})
	mux.HandleFunc("GET /ready", func(w http.ResponseWriter, r *http.Request) {
		if deps.Readiness != nil {
			if err := deps.Readiness.Ready(r.Context()); err != nil {
				writeJSON(w, http.StatusServiceUnavailable, map[string]string{"status": "not_ready"})
				return
			}
		}

		writeJSON(w, http.StatusOK, map[string]string{"status": "ready"})
	})

	protectedEvents := auth.APIKeyMiddleware(deps.APIKeys)(
		auth.ScopeMiddleware()(
			http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				handleEventIngest(w, r, deps.EventIngestor)
			}),
		),
	)
	mux.Handle("POST /v1/events", protectedEvents)

	protectedSearch := auth.APIKeyMiddleware(deps.APIKeys)(
		auth.ScopeMiddleware()(
			http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				handleMemorySearch(w, r, deps.MemorySearcher)
			}),
		),
	)
	mux.Handle("POST /v1/memories/search", protectedSearch)

	protectedContext := auth.APIKeyMiddleware(deps.APIKeys)(
		auth.ScopeMiddleware()(
			http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				handleContextAssembly(w, r, deps.ContextAssembler)
			}),
		),
	)
	mux.Handle("POST /v1/context/assemble", protectedContext)

	return requestMiddleware(mux, deps.Logger)
}

func NewHTTPServer(addr string, deps HTTPDependencies) *http.Server {
	return &http.Server{
		Addr:    addr,
		Handler: NewHTTPHandler(deps),
	}
}

func requestMiddleware(next http.Handler, logger *log.Logger) http.Handler {
	if logger == nil {
		logger = log.Default()
	}

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestID := requestIDFromHeader(r.Header.Get("X-Request-ID"))
		w.Header().Set("X-Request-ID", requestID)

		defer func(start time.Time) {
			if rec := recover(); rec != nil {
				logger.Printf("panic path=%s request_id=%s err=%v", r.URL.Path, requestID, rec)
				http.Error(w, "internal server error", http.StatusInternalServerError)
				return
			}
			logger.Printf("request path=%s method=%s request_id=%s duration=%s", r.URL.Path, r.Method, requestID, time.Since(start))
		}(time.Now())

		next.ServeHTTP(w, r)
	})
}

func requestIDFromHeader(value string) string {
	if value != "" {
		return value
	}

	return fmt.Sprintf("req_%08x", rand.Uint32())
}

func handleEventIngest(w http.ResponseWriter, r *http.Request, ingestor memory.EventIngestor) {
	if ingestor == nil {
		http.Error(w, "event ingestor is not configured", http.StatusServiceUnavailable)
		return
	}

	var req eventIngestRequest
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	if decoder.More() {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	scope, ok := auth.ScopeFromContext(r.Context())
	if !ok {
		http.Error(w, "scope context is missing", http.StatusInternalServerError)
		return
	}

	input := memory.IngestEventInput{
		Scope:     scope,
		EventType: req.EventType,
		Content:   req.Content,
		Metadata:  req.Metadata,
	}
	if req.SourceTimestamp != "" {
		sourceTime, err := time.Parse(time.RFC3339, req.SourceTimestamp)
		if err != nil {
			http.Error(w, "invalid source_timestamp", http.StatusBadRequest)
			return
		}
		input.SourceTimestamp = sourceTime
	}

	if err := input.Validate(); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	event, err := ingestor.Ingest(r.Context(), input)
	if err != nil {
		status := http.StatusInternalServerError
		if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
			status = http.StatusGatewayTimeout
		}
		http.Error(w, "failed to ingest event", status)
		return
	}

	writeJSON(w, http.StatusCreated, eventIngestResponse{EventID: event.ID})
}

func handleMemorySearch(w http.ResponseWriter, r *http.Request, searcher retrieval.MemorySearcher) {
	if searcher == nil {
		http.Error(w, "memory searcher is not configured", http.StatusServiceUnavailable)
		return
	}

	var req memorySearchRequest
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}
	if decoder.More() {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	scope, ok := auth.ScopeFromContext(r.Context())
	if !ok {
		http.Error(w, "scope context is missing", http.StatusInternalServerError)
		return
	}

	input := retrieval.SearchInput{
		Scope:            scope,
		Query:            req.Query,
		QueryEmbedding:   req.QueryEmbedding,
		Classes:          req.Classes,
		TopK:             req.TopK,
		IncludeSummaries: req.IncludeSummaries,
		IncludeRelations: req.IncludeRelations,
	}
	if req.TimeFrom != "" {
		timeFrom, err := time.Parse(time.RFC3339, req.TimeFrom)
		if err != nil {
			http.Error(w, "invalid time_from", http.StatusBadRequest)
			return
		}
		input.TimeFrom = timeFrom
	}
	if req.TimeTo != "" {
		timeTo, err := time.Parse(time.RFC3339, req.TimeTo)
		if err != nil {
			http.Error(w, "invalid time_to", http.StatusBadRequest)
			return
		}
		input.TimeTo = timeTo
	}

	result, err := searcher.Search(r.Context(), input)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	writeJSON(w, http.StatusOK, result)
}

func handleContextAssembly(w http.ResponseWriter, r *http.Request, assembler retrieval.ContextAssembler) {
	if assembler == nil {
		http.Error(w, "context assembler is not configured", http.StatusServiceUnavailable)
		return
	}

	var req contextAssembleRequest
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}
	if decoder.More() {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	scope, ok := auth.ScopeFromContext(r.Context())
	if !ok {
		http.Error(w, "scope context is missing", http.StatusInternalServerError)
		return
	}

	result, err := assembler.AssembleContext(r.Context(), retrieval.AssembleContextInput{
		Scope:            scope,
		Query:            req.Query,
		Budget:           req.Budget,
		IncludeRelations: req.IncludeRelations,
	})
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	writeJSON(w, http.StatusOK, result)
}

func writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}
