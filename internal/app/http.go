package app

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"math/rand"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/FelixSeptem/stele/internal/auth"
	"github.com/FelixSeptem/stele/internal/jobs"
	"github.com/FelixSeptem/stele/internal/memory"
	"github.com/FelixSeptem/stele/internal/policy"
	"github.com/FelixSeptem/stele/internal/retrieval"
	"github.com/jackc/pgx/v5"
)

type ReadinessChecker interface {
	Ready(ctx context.Context) error
}

type HTTPDependencies struct {
	Readiness             ReadinessChecker
	APIKeys               auth.StaticAPIKeys
	AdminAPIKeys          auth.StaticAPIKeys
	EventIngestor         memory.EventIngestor
	MemoryQuery           MemoryQueryService
	MemoryLifecycleAction MemoryLifecycleActionService
	MemorySearcher        retrieval.MemorySearcher
	ContextAssembler      retrieval.ContextAssembler
	GovernanceStatusRead  GovernanceStatusReader
	MemoryHistoryRead     MemoryHistoryReader
	JobExecutionRead      JobExecutionReader
	Logger                *log.Logger
}

type GovernanceStatus = jobs.GovernanceStatus

type GovernanceStatusReader interface {
	ReadGovernanceStatus(ctx context.Context) (GovernanceStatus, error)
}

type MemoryHistoryReader interface {
	ReadMemoryHistory(ctx context.Context, scope memory.Scope, memoryID string) (memory.MemoryHistory, error)
}

type MemoryQueryService interface {
	ListMemories(ctx context.Context, input memory.ListMemoriesInput) (memory.MemoryPage, error)
	GetMemory(ctx context.Context, scope memory.Scope, memoryID string) (memory.MemoryResource, error)
	GetMemoryHistory(ctx context.Context, scope memory.Scope, memoryID string) (memory.MemoryHistory, error)
	GetMemoryProvenance(ctx context.Context, scope memory.Scope, memoryID string) ([]memory.ProvenanceRecord, error)
}

type MemoryLifecycleActionService interface {
	Apply(ctx context.Context, input memory.LifecycleActionInput) error
}

type JobExecutionReader interface {
	ListRecentJobExecutions(ctx context.Context, scope memory.Scope, limit int) ([]jobs.JobExecutionRecord, error)
}

type lifecycleActionRequest struct {
	Reason string `json:"reason"`
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

	protectedMemoryList := auth.APIKeyMiddleware(deps.APIKeys)(
		auth.ScopeMiddleware()(
			http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				handleMemoryList(w, r, deps.MemoryQuery)
			}),
		),
	)
	mux.Handle("GET /v1/memories", protectedMemoryList)

	protectedMemoryDetail := auth.APIKeyMiddleware(deps.APIKeys)(
		auth.ScopeMiddleware()(
			http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				handleMemoryDetail(w, r, deps.MemoryQuery)
			}),
		),
	)
	mux.Handle("GET /v1/memories/{memory_id}", protectedMemoryDetail)

	protectedMemoryHistory := auth.APIKeyMiddleware(deps.APIKeys)(
		auth.ScopeMiddleware()(
			http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				handlePublicMemoryHistory(w, r, deps.MemoryQuery)
			}),
		),
	)
	mux.Handle("GET /v1/memories/{memory_id}/history", protectedMemoryHistory)

	protectedMemoryProvenance := auth.APIKeyMiddleware(deps.APIKeys)(
		auth.ScopeMiddleware()(
			http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				handleMemoryProvenance(w, r, deps.MemoryQuery)
			}),
		),
	)
	mux.Handle("GET /v1/memories/{memory_id}/provenance", protectedMemoryProvenance)

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

	adminGovernance := auth.APIKeyMiddleware(deps.AdminAPIKeys)(
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			handleGovernanceStatus(w, r, deps.GovernanceStatusRead)
		}),
	)
	mux.Handle("GET /v1/admin/jobs/governance/status", adminGovernance)

	adminJobStatus := auth.APIKeyMiddleware(deps.AdminAPIKeys)(
		auth.ScopeMiddleware()(
			http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				handleRecentJobExecutions(w, r, deps.JobExecutionRead)
			}),
		),
	)
	mux.Handle("GET /v1/admin/jobs/status", adminJobStatus)

	adminMemoryHistory := auth.APIKeyMiddleware(deps.AdminAPIKeys)(
		auth.ScopeMiddleware()(
			http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				handleMemoryHistory(w, r, deps.MemoryHistoryRead)
			}),
		),
	)
	mux.Handle("GET /v1/admin/memories/{memory_id}/history", adminMemoryHistory)

	adminMemoryLifecycle := auth.APIKeyMiddleware(deps.AdminAPIKeys)(
		auth.ScopeMiddleware()(
			http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				handleMemoryLifecycleAction(w, r, deps.MemoryLifecycleAction)
			}),
		),
	)
	mux.Handle("POST /v1/admin/memories/{memory_action}", adminMemoryLifecycle)

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
				logger.Printf("mode=api component=http event=panic path=%s request_id=%s err=%v", r.URL.Path, requestID, rec)
				http.Error(w, "internal server error", http.StatusInternalServerError)
				return
			}
			logger.Printf("mode=api component=http event=request_completed path=%s method=%s request_id=%s duration=%s", r.URL.Path, r.Method, requestID, time.Since(start))
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

func handleMemoryList(w http.ResponseWriter, r *http.Request, reader MemoryQueryService) {
	if reader == nil {
		http.Error(w, "memory query service is not configured", http.StatusServiceUnavailable)
		return
	}

	scope, ok := auth.ScopeFromContext(r.Context())
	if !ok {
		http.Error(w, "scope context is missing", http.StatusInternalServerError)
		return
	}

	limit := 20
	if raw := strings.TrimSpace(r.URL.Query().Get("limit")); raw != "" {
		parsed, err := strconv.Atoi(raw)
		if err != nil || parsed <= 0 {
			http.Error(w, "invalid limit", http.StatusBadRequest)
			return
		}
		if parsed > 100 {
			parsed = 100
		}
		limit = parsed
	}

	input := memory.ListMemoriesInput{
		Scope:   scope,
		Classes: parseMemoryClasses(r.URL.Query()["class"]),
		Limit:   limit,
	}
	if raw := strings.TrimSpace(r.URL.Query().Get("time_from")); raw != "" {
		parsed, err := time.Parse(time.RFC3339, raw)
		if err != nil {
			http.Error(w, "invalid time_from", http.StatusBadRequest)
			return
		}
		input.TimeFrom = parsed
	}
	if raw := strings.TrimSpace(r.URL.Query().Get("time_to")); raw != "" {
		parsed, err := time.Parse(time.RFC3339, raw)
		if err != nil {
			http.Error(w, "invalid time_to", http.StatusBadRequest)
			return
		}
		input.TimeTo = parsed
	}

	page, err := reader.ListMemories(r.Context(), input)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	writeJSON(w, http.StatusOK, page)
}

func handleMemoryDetail(w http.ResponseWriter, r *http.Request, reader MemoryQueryService) {
	if reader == nil {
		http.Error(w, "memory query service is not configured", http.StatusServiceUnavailable)
		return
	}

	scope, ok := auth.ScopeFromContext(r.Context())
	if !ok {
		http.Error(w, "scope context is missing", http.StatusInternalServerError)
		return
	}

	resource, err := reader.GetMemory(r.Context(), scope, r.PathValue("memory_id"))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			http.Error(w, "memory not found", http.StatusNotFound)
			return
		}
		http.Error(w, "failed to read memory", http.StatusInternalServerError)
		return
	}

	writeJSON(w, http.StatusOK, resource)
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

func handlePublicMemoryHistory(w http.ResponseWriter, r *http.Request, reader MemoryQueryService) {
	if reader == nil {
		http.Error(w, "memory query service is not configured", http.StatusServiceUnavailable)
		return
	}

	scope, ok := auth.ScopeFromContext(r.Context())
	if !ok {
		http.Error(w, "scope context is missing", http.StatusInternalServerError)
		return
	}

	history, err := reader.GetMemoryHistory(r.Context(), scope, r.PathValue("memory_id"))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			http.Error(w, "memory not found", http.StatusNotFound)
			return
		}
		http.Error(w, "failed to read memory history", http.StatusInternalServerError)
		return
	}

	writeJSON(w, http.StatusOK, history)
}

func handleMemoryProvenance(w http.ResponseWriter, r *http.Request, reader MemoryQueryService) {
	if reader == nil {
		http.Error(w, "memory query service is not configured", http.StatusServiceUnavailable)
		return
	}

	scope, ok := auth.ScopeFromContext(r.Context())
	if !ok {
		http.Error(w, "scope context is missing", http.StatusInternalServerError)
		return
	}

	records, err := reader.GetMemoryProvenance(r.Context(), scope, r.PathValue("memory_id"))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			http.Error(w, "memory not found", http.StatusNotFound)
			return
		}
		http.Error(w, "failed to read memory provenance", http.StatusInternalServerError)
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{"provenance": records})
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

func handleGovernanceStatus(w http.ResponseWriter, r *http.Request, reader GovernanceStatusReader) {
	if reader == nil {
		http.Error(w, "governance status reader is not configured", http.StatusServiceUnavailable)
		return
	}

	status, err := reader.ReadGovernanceStatus(r.Context())
	if err != nil {
		http.Error(w, "failed to read governance status", http.StatusInternalServerError)
		return
	}

	writeJSON(w, http.StatusOK, status)
}

func handleMemoryHistory(w http.ResponseWriter, r *http.Request, reader MemoryHistoryReader) {
	if reader == nil {
		http.Error(w, "memory history reader is not configured", http.StatusServiceUnavailable)
		return
	}

	scope, ok := auth.ScopeFromContext(r.Context())
	if !ok {
		http.Error(w, "scope context is missing", http.StatusInternalServerError)
		return
	}

	history, err := reader.ReadMemoryHistory(r.Context(), scope, r.PathValue("memory_id"))
	if err != nil {
		http.Error(w, "failed to read memory history", http.StatusInternalServerError)
		return
	}

	writeJSON(w, http.StatusOK, history)
}

func handleRecentJobExecutions(w http.ResponseWriter, r *http.Request, reader JobExecutionReader) {
	if reader == nil {
		http.Error(w, "job execution reader is not configured", http.StatusServiceUnavailable)
		return
	}

	scope, ok := auth.ScopeFromContext(r.Context())
	if !ok {
		http.Error(w, "scope context is missing", http.StatusInternalServerError)
		return
	}

	limit := 20
	if raw := strings.TrimSpace(r.URL.Query().Get("limit")); raw != "" {
		parsed, err := strconv.Atoi(raw)
		if err != nil || parsed <= 0 {
			http.Error(w, "invalid limit", http.StatusBadRequest)
			return
		}
		if parsed > 100 {
			parsed = 100
		}
		limit = parsed
	}

	records, err := reader.ListRecentJobExecutions(r.Context(), scope, limit)
	if err != nil {
		http.Error(w, "failed to read job executions", http.StatusInternalServerError)
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{"executions": records})
}

func handleMemoryLifecycleAction(w http.ResponseWriter, r *http.Request, service MemoryLifecycleActionService) {
	if service == nil {
		http.Error(w, "memory lifecycle service is not configured", http.StatusServiceUnavailable)
		return
	}

	var req lifecycleActionRequest
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

	memoryID, action, err := parseLifecycleActionTarget(r.PathValue("memory_action"))
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	input := memory.LifecycleActionInput{
		Scope:     scope,
		MemoryID:  memoryID,
		Action:    action,
		Reason:    req.Reason,
		Actor:     strings.TrimSpace(r.Header.Get("X-Stele-Actor")),
		RequestID: strings.TrimSpace(r.Header.Get("X-Request-ID")),
	}
	if err := service.Apply(r.Context(), input); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"memory_id": input.MemoryID,
		"action":    input.Action,
		"reason":    input.Reason,
	})
}

func writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}

func parseMemoryClasses(values []string) []memory.MemoryClass {
	if len(values) == 0 {
		return nil
	}

	classes := make([]memory.MemoryClass, 0, len(values))
	for _, value := range values {
		for _, part := range strings.Split(value, ",") {
			part = strings.TrimSpace(part)
			if part == "" {
				continue
			}
			classes = append(classes, memory.MemoryClass(part))
		}
	}

	if len(classes) == 0 {
		return nil
	}

	return classes
}

func parseLifecycleActionTarget(value string) (string, policy.ForgettingAction, error) {
	memoryID, actionName, ok := strings.Cut(value, ":")
	if !ok || strings.TrimSpace(memoryID) == "" {
		return "", "", fmt.Errorf("invalid lifecycle action target")
	}

	switch actionName {
	case "suppress":
		return memoryID, policy.ForgettingActionSuppress, nil
	case "expire":
		return memoryID, policy.ForgettingActionExpire, nil
	case "delete":
		return memoryID, policy.ForgettingActionDelete, nil
	default:
		return "", "", fmt.Errorf("invalid lifecycle action target")
	}
}
