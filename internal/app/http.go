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
	"github.com/FelixSeptem/stele/internal/governance"
	"github.com/FelixSeptem/stele/internal/jobs"
	"github.com/FelixSeptem/stele/internal/memory"
	"github.com/FelixSeptem/stele/internal/policy"
	"github.com/FelixSeptem/stele/internal/retrieval"
	"github.com/FelixSeptem/stele/internal/telemetry"
	"github.com/jackc/pgx/v5"
)

type ReadinessChecker interface {
	Ready(ctx context.Context) error
}

type HTTPDependencies struct {
	Readiness                 ReadinessChecker
	APIKeys                   auth.StaticAPIKeys
	AdminAPIKeys              auth.StaticAPIKeys
	EventIngestor             memory.EventIngestor
	MemoryQuery               MemoryQueryService
	MemoryLifecycleAction     MemoryLifecycleActionService
	MemoryManualMutation      ManualMemoryMutationService
	EmbeddingAdminRead        EmbeddingAdminQueryService
	MemorySearcher            retrieval.MemorySearcher
	ContextAssembler          retrieval.ContextAssembler
	GovernanceStatusRead      GovernanceStatusReader
	GovernanceAdmin           GovernanceAdminService
	DerivedInsightAdmin       DerivedInsightAdminService
	DerivedInsightReplayAdmin DerivedInsightReplayAdminService
	MemoryHistoryRead         MemoryHistoryReader
	JobExecutionRead          JobExecutionReader
	Metrics                   MetricsRecorder
	Logger                    *log.Logger
}

type requestIDContextKey struct{}

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

type GovernanceAdminService interface {
	ListGovernanceRawEvents(ctx context.Context, input governance.ListGovernanceRawEventsInput) (governance.GovernanceRawEventPage, error)
	ReadGovernanceRawEvent(ctx context.Context, input governance.ReadGovernanceRawEventInput) (governance.GovernanceRawEvent, error)
	ListGovernanceRecoveryHistory(ctx context.Context, input governance.ListGovernanceRecoveryHistoryInput) ([]governance.GovernanceRecoveryRecord, error)
	ApplyGovernanceRecovery(ctx context.Context, input governance.ApplyGovernanceRecoveryInput) (governance.GovernanceRecoveryOutcome, error)
}

type DerivedInsightAdminService interface {
	ListDerivedInsights(ctx context.Context, input memory.ListDerivedInsightsInput) ([]memory.DerivedInsight, error)
	ReadDerivedInsight(ctx context.Context, input memory.ReadDerivedInsightInput) (memory.DerivedInsightDetail, error)
	TransitionDerivedInsightLifecycle(ctx context.Context, transition memory.DerivedInsightLifecycleTransition) error
	CreateDerivedInsightFeedback(ctx context.Context, input memory.CreateDerivedInsightFeedbackInput) (memory.DerivedInsightFeedback, error)
	ListDerivedInsightFeedback(ctx context.Context, input memory.ListDerivedInsightFeedbackInput) ([]memory.DerivedInsightFeedback, error)
	SupersedeDerivedInsightFeedback(ctx context.Context, input memory.SupersedeDerivedInsightFeedbackInput) error
}

type DerivedInsightReplayAdminService interface {
	PlanDerivedInsightReplay(ctx context.Context, input memory.DerivedInsightReplayRequest) (memory.DerivedInsightReplayReport, error)
	ApplyDerivedInsightReplay(ctx context.Context, input memory.DerivedInsightReplayRequest) (memory.DerivedInsightReplayRun, error)
	ListDerivedInsightReplayRuns(ctx context.Context, input memory.ListDerivedInsightReplayRunsInput) ([]memory.DerivedInsightReplayRun, error)
	ReadDerivedInsightReplayRun(ctx context.Context, input memory.ReadDerivedInsightReplayRunInput) (memory.DerivedInsightReplayRun, error)
	ReadDerivedInsightReplayReport(ctx context.Context, input memory.ReadDerivedInsightReplayRunInput) (memory.DerivedInsightReplayReport, error)
}

type ManualMemoryMutationService interface {
	CreateMemory(ctx context.Context, input memory.ManualCreateMemoryInput) (memory.MemoryResource, error)
	UpdateMemory(ctx context.Context, input memory.ManualUpdateMemoryInput) (memory.MemoryResource, error)
	MergeMemory(ctx context.Context, input memory.ManualMergeMemoryInput) (memory.MemoryResource, error)
	ReclassifyMemory(ctx context.Context, input memory.ManualReclassifyMemoryInput) (memory.MemoryResource, error)
}

type EmbeddingAdminQueryService interface {
	ListEmbeddingRebuilds(ctx context.Context, input memory.ListEmbeddingRebuildsInput) (memory.EmbeddingRebuildPage, error)
	GetMemoryEmbedding(ctx context.Context, scope memory.Scope, memoryID string) (memory.EmbeddingMemoryInspection, error)
	ApplyEmbeddingRecovery(ctx context.Context, input memory.ApplyEmbeddingRecoveryInput) (memory.EmbeddingRecoveryOutcome, error)
	CreateEmbeddingCutoverPlan(ctx context.Context, input memory.CreateEmbeddingCutoverPlanInput) (memory.EmbeddingCutoverPlan, error)
	ListEmbeddingCutoverPlans(ctx context.Context, input memory.ListEmbeddingCutoverPlansInput) ([]memory.EmbeddingCutoverPlan, error)
	ReadEmbeddingCutoverPlan(ctx context.Context, input memory.ReadEmbeddingCutoverPlanInput) (memory.EmbeddingCutoverPlan, error)
	PreflightEmbeddingCutoverPlan(ctx context.Context, input memory.EmbeddingCutoverPreflightInput) (memory.EmbeddingCutoverPreflightReport, error)
	ApplyEmbeddingCutoverPlanAction(ctx context.Context, input memory.ApplyEmbeddingCutoverPlanActionInput) (memory.EmbeddingCutoverPlan, error)
	ListEmbeddingRecoveryHistory(ctx context.Context, input memory.ListEmbeddingRecoveryHistoryInput) ([]memory.EmbeddingRecoveryRecord, error)
}

type JobExecutionReader interface {
	ListRecentJobExecutions(ctx context.Context, scope memory.Scope, limit int) ([]jobs.JobExecutionRecord, error)
}

type MetricsRecorder interface {
	RenderPrometheus() string
	RecordAdmission(ctx context.Context, event telemetry.AdmissionEvent)
	RecordCutoverPlanState(ctx context.Context, event telemetry.CutoverPlanStateEvent)
	RecordCutoverItemState(ctx context.Context, event telemetry.CutoverItemStateEvent)
	RecordInsightFeedback(ctx context.Context, event telemetry.InsightFeedbackEvent)
}

type lifecycleActionRequest struct {
	Reason string `json:"reason"`
}

type derivedInsightSuppressRequest struct {
	Actor  string `json:"actor"`
	Reason string `json:"reason"`
}

type derivedInsightFeedbackCreateRequest struct {
	Type         memory.InsightFeedbackType `json:"type"`
	Actor        string                     `json:"actor"`
	Reason       string                     `json:"reason"`
	QualityScore *float64                   `json:"quality_score,omitempty"`
	Metadata     map[string]any             `json:"metadata,omitempty"`
}

type derivedInsightFeedbackSupersedeRequest struct {
	Actor  string `json:"actor"`
	Reason string `json:"reason"`
}

type derivedInsightReplayRequest struct {
	InsightTypes        []memory.DerivedInsightType `json:"insight_types"`
	EvidenceWindowStart string                      `json:"evidence_window_start"`
	EvidenceWindowEnd   string                      `json:"evidence_window_end"`
	EvidenceLimit       int                         `json:"evidence_limit"`
	Actor               string                      `json:"actor"`
	Reason              string                      `json:"reason"`
	IdempotencyKey      string                      `json:"idempotency_key"`
	Metadata            map[string]any              `json:"metadata,omitempty"`
}

type governanceRecoveryRequest struct {
	Reason       string `json:"reason"`
	ScheduledFor string `json:"scheduled_for"`
}

type embeddingCutoverCreateRequest struct {
	Target   memory.EmbeddingCutoverTarget `json:"target"`
	Classes  []memory.MemoryClass          `json:"classes"`
	WaveSize int                           `json:"wave_size"`
	Reason   string                        `json:"reason"`
}

type manualCreateMemoryRequest struct {
	Class   memory.MemoryClass `json:"class"`
	Content string             `json:"content"`
	Reason  string             `json:"reason"`
}

type manualUpdateMemoryRequest struct {
	Content         string `json:"content"`
	ExpectedVersion int64  `json:"expected_version"`
	Reason          string `json:"reason"`
}

type manualMergeMemoryRequest struct {
	SourceMemoryID  string `json:"source_memory_id"`
	Content         string `json:"content"`
	ExpectedVersion int64  `json:"expected_version"`
	Reason          string `json:"reason"`
}

type manualReclassifyMemoryRequest struct {
	TargetClass     memory.MemoryClass `json:"target_class"`
	ExpectedVersion int64              `json:"expected_version"`
	Reason          string             `json:"reason"`
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
	Query                     string `json:"query"`
	Budget                    int    `json:"budget"`
	IncludeRelations          bool   `json:"include_relations"`
	IncludeExperienceInsights bool   `json:"include_experience_insights"`
	IncludeDiagnostics        bool   `json:"include_diagnostics"`
}

func NewHTTPHandler(deps HTTPDependencies) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /health", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	})
	mux.HandleFunc("GET /livez", func(w http.ResponseWriter, r *http.Request) {
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
	mux.HandleFunc("GET /readyz", func(w http.ResponseWriter, r *http.Request) {
		if deps.Readiness != nil {
			if err := deps.Readiness.Ready(r.Context()); err != nil {
				writeJSON(w, http.StatusServiceUnavailable, map[string]string{"status": "not_ready"})
				return
			}
		}

		writeJSON(w, http.StatusOK, map[string]string{"status": "ready"})
	})
	mux.HandleFunc("GET /metrics", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain; version=0.0.4; charset=utf-8")
		if deps.Metrics != nil {
			_, _ = w.Write([]byte(deps.Metrics.RenderPrometheus()))
			return
		}
		_, _ = w.Write([]byte("# HELP stele_runtime_info Stele runtime information\n# TYPE stele_runtime_info gauge\nstele_runtime_info 1\n"))
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

	adminGovernanceRawEvents := auth.APIKeyMiddleware(deps.AdminAPIKeys)(
		auth.ScopeMiddleware()(
			http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				handleAdminGovernanceRawEventList(w, r, deps.GovernanceAdmin)
			}),
		),
	)
	mux.Handle("GET /v1/admin/governance/raw-events", adminGovernanceRawEvents)

	adminGovernanceRawEventDetail := auth.APIKeyMiddleware(deps.AdminAPIKeys)(
		auth.ScopeMiddleware()(
			http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				handleAdminGovernanceRawEventDetail(w, r, deps.GovernanceAdmin)
			}),
		),
	)
	mux.Handle("GET /v1/admin/governance/raw-events/{raw_event_id}", adminGovernanceRawEventDetail)

	adminGovernanceRecoveryHistory := auth.APIKeyMiddleware(deps.AdminAPIKeys)(
		auth.ScopeMiddleware()(
			http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				handleAdminGovernanceRecoveryHistory(w, r, deps.GovernanceAdmin)
			}),
		),
	)
	mux.Handle("GET /v1/admin/governance/raw-events/{raw_event_id}/recovery-history", adminGovernanceRecoveryHistory)

	adminGovernanceRawEventAction := auth.APIKeyMiddleware(deps.AdminAPIKeys)(
		auth.ScopeMiddleware()(
			http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				handleAdminGovernanceRawEventAction(w, r, deps.GovernanceAdmin)
			}),
		),
	)
	mux.Handle("POST /v1/admin/governance/raw-events/{raw_event_action}", adminGovernanceRawEventAction)

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

	adminDerivedInsightList := auth.APIKeyMiddleware(deps.AdminAPIKeys)(
		auth.ScopeMiddleware()(
			http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				handleAdminDerivedInsightList(w, r, deps.DerivedInsightAdmin)
			}),
		),
	)
	mux.Handle("GET /v1/admin/derived-insights", adminDerivedInsightList)

	adminDerivedInsightDetail := auth.APIKeyMiddleware(deps.AdminAPIKeys)(
		auth.ScopeMiddleware()(
			http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				handleAdminDerivedInsightDetail(w, r, deps.DerivedInsightAdmin)
			}),
		),
	)
	mux.Handle("GET /v1/admin/derived-insights/{insight_id}", adminDerivedInsightDetail)

	adminDerivedInsightFeedback := auth.APIKeyMiddleware(deps.AdminAPIKeys)(
		auth.ScopeMiddleware()(
			http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				handleAdminDerivedInsightFeedback(w, r, deps.DerivedInsightAdmin, deps.Metrics)
			}),
		),
	)
	mux.Handle("GET /v1/admin/derived-insights/{insight_id}/feedback", adminDerivedInsightFeedback)
	mux.Handle("POST /v1/admin/derived-insights/{insight_id}/feedback", adminDerivedInsightFeedback)

	adminDerivedInsightFeedbackAction := auth.APIKeyMiddleware(deps.AdminAPIKeys)(
		auth.ScopeMiddleware()(
			http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				handleAdminDerivedInsightFeedbackAction(w, r, deps.DerivedInsightAdmin, deps.Metrics)
			}),
		),
	)
	mux.Handle("POST /v1/admin/derived-insight-feedback/{feedback_action}", adminDerivedInsightFeedbackAction)

	adminDerivedInsightAction := auth.APIKeyMiddleware(deps.AdminAPIKeys)(
		auth.ScopeMiddleware()(
			http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				handleAdminDerivedInsightAction(w, r, deps.DerivedInsightAdmin)
			}),
		),
	)
	mux.Handle("POST /v1/admin/derived-insights/{insight_action}", adminDerivedInsightAction)

	adminDerivedInsightReplayDryRun := auth.APIKeyMiddleware(deps.AdminAPIKeys)(
		auth.ScopeMiddleware()(
			http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				handleAdminDerivedInsightReplayDryRun(w, r, deps.DerivedInsightReplayAdmin)
			}),
		),
	)
	mux.Handle("POST /v1/admin/derived-insight-replays:dry-run", adminDerivedInsightReplayDryRun)

	adminDerivedInsightReplays := auth.APIKeyMiddleware(deps.AdminAPIKeys)(
		auth.ScopeMiddleware()(
			http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				handleAdminDerivedInsightReplays(w, r, deps.DerivedInsightReplayAdmin)
			}),
		),
	)
	mux.Handle("GET /v1/admin/derived-insight-replays", adminDerivedInsightReplays)
	mux.Handle("POST /v1/admin/derived-insight-replays", adminDerivedInsightReplays)

	adminDerivedInsightReplayDetail := auth.APIKeyMiddleware(deps.AdminAPIKeys)(
		auth.ScopeMiddleware()(
			http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				handleAdminDerivedInsightReplayDetail(w, r, deps.DerivedInsightReplayAdmin)
			}),
		),
	)
	mux.Handle("GET /v1/admin/derived-insight-replays/{replay_run_id}", adminDerivedInsightReplayDetail)

	adminDerivedInsightReplayReport := auth.APIKeyMiddleware(deps.AdminAPIKeys)(
		auth.ScopeMiddleware()(
			http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				handleAdminDerivedInsightReplayReport(w, r, deps.DerivedInsightReplayAdmin)
			}),
		),
	)
	mux.Handle("GET /v1/admin/derived-insight-replays/{replay_run_id}/report", adminDerivedInsightReplayReport)

	adminEmbeddingRebuilds := auth.APIKeyMiddleware(deps.AdminAPIKeys)(
		auth.ScopeMiddleware()(
			http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				handleAdminEmbeddingRebuildList(w, r, deps.EmbeddingAdminRead)
			}),
		),
	)
	mux.Handle("GET /v1/admin/embedding/rebuilds", adminEmbeddingRebuilds)

	adminEmbeddingRebuildAction := auth.APIKeyMiddleware(deps.AdminAPIKeys)(
		auth.ScopeMiddleware()(
			http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				handleAdminEmbeddingRebuildAction(w, r, deps.EmbeddingAdminRead)
			}),
		),
	)
	mux.Handle("POST /v1/admin/embedding/rebuilds/{embedding_action}", adminEmbeddingRebuildAction)

	adminEmbeddingCutovers := auth.APIKeyMiddleware(deps.AdminAPIKeys)(
		auth.ScopeMiddleware()(
			http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				handleAdminEmbeddingCutoverList(w, r, deps.EmbeddingAdminRead, deps.Metrics)
			}),
		),
	)
	mux.Handle("GET /v1/admin/embedding/cutovers", adminEmbeddingCutovers)

	adminEmbeddingCutoverCreate := auth.APIKeyMiddleware(deps.AdminAPIKeys)(
		auth.ScopeMiddleware()(
			http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				handleAdminEmbeddingCutoverCreate(w, r, deps.EmbeddingAdminRead)
			}),
		),
	)
	mux.Handle("POST /v1/admin/embedding/cutovers", adminEmbeddingCutoverCreate)

	adminEmbeddingCutoverDetail := auth.APIKeyMiddleware(deps.AdminAPIKeys)(
		auth.ScopeMiddleware()(
			http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				handleAdminEmbeddingCutoverDetail(w, r, deps.EmbeddingAdminRead, deps.Metrics)
			}),
		),
	)
	mux.Handle("GET /v1/admin/embedding/cutovers/{cutover_plan_id}", adminEmbeddingCutoverDetail)

	adminEmbeddingCutoverAction := auth.APIKeyMiddleware(deps.AdminAPIKeys)(
		auth.ScopeMiddleware()(
			http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				handleAdminEmbeddingCutoverAction(w, r, deps.EmbeddingAdminRead, deps.Metrics)
			}),
		),
	)
	mux.Handle("POST /v1/admin/embedding/cutovers/{cutover_action}", adminEmbeddingCutoverAction)

	adminEmbeddingRecoveryHistory := auth.APIKeyMiddleware(deps.AdminAPIKeys)(
		auth.ScopeMiddleware()(
			http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				handleAdminEmbeddingRecoveryHistory(w, r, deps.EmbeddingAdminRead, "")
			}),
		),
	)
	mux.Handle("GET /v1/admin/embedding/recovery-history", adminEmbeddingRecoveryHistory)

	adminMemoryEmbedding := auth.APIKeyMiddleware(deps.AdminAPIKeys)(
		auth.ScopeMiddleware()(
			http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				handleAdminMemoryEmbedding(w, r, deps.EmbeddingAdminRead)
			}),
		),
	)
	mux.Handle("GET /v1/admin/memories/{memory_id}/embedding", adminMemoryEmbedding)

	adminMemoryEmbeddingRecoveryHistory := auth.APIKeyMiddleware(deps.AdminAPIKeys)(
		auth.ScopeMiddleware()(
			http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				handleAdminEmbeddingRecoveryHistory(w, r, deps.EmbeddingAdminRead, r.PathValue("memory_id"))
			}),
		),
	)
	mux.Handle("GET /v1/admin/memories/{memory_id}/embedding/recovery-history", adminMemoryEmbeddingRecoveryHistory)

	adminMemoryCreate := auth.APIKeyMiddleware(deps.AdminAPIKeys)(
		auth.ScopeMiddleware()(
			http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				handleAdminMemoryCreate(w, r, deps.MemoryManualMutation)
			}),
		),
	)
	mux.Handle("POST /v1/admin/memories", adminMemoryCreate)

	adminMemoryUpdate := auth.APIKeyMiddleware(deps.AdminAPIKeys)(
		auth.ScopeMiddleware()(
			http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				handleAdminMemoryUpdate(w, r, deps.MemoryManualMutation)
			}),
		),
	)
	mux.Handle("PATCH /v1/admin/memories/{memory_id}", adminMemoryUpdate)

	adminMemoryAction := auth.APIKeyMiddleware(deps.AdminAPIKeys)(
		auth.ScopeMiddleware()(
			http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				handleAdminMemoryAction(w, r, deps.MemoryLifecycleAction, deps.MemoryManualMutation)
			}),
		),
	)
	mux.Handle("POST /v1/admin/memories/{memory_action}", adminMemoryAction)

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

		ctx := context.WithValue(r.Context(), requestIDContextKey{}, requestID)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func requestIDFromHeader(value string) string {
	if value != "" {
		return value
	}

	return fmt.Sprintf("req_%08x", rand.Uint32())
}

func requestIDFromContext(ctx context.Context) string {
	value, _ := ctx.Value(requestIDContextKey{}).(string)
	return value
}

func newFeedbackID() string {
	return "feedback_" + strings.TrimPrefix(newID(), "id_")
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
		Scope:                     scope,
		Query:                     req.Query,
		Budget:                    req.Budget,
		IncludeRelations:          req.IncludeRelations,
		IncludeExperienceInsights: req.IncludeExperienceInsights,
		IncludeDiagnostics:        req.IncludeDiagnostics,
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

func handleAdminDerivedInsightList(w http.ResponseWriter, r *http.Request, service DerivedInsightAdminService) {
	if service == nil {
		http.Error(w, "derived insight admin service is not configured", http.StatusServiceUnavailable)
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

	input := memory.ListDerivedInsightsInput{
		Scope:            scope,
		Type:             memory.DerivedInsightType(strings.TrimSpace(r.URL.Query().Get("type"))),
		State:            memory.DerivedInsightState(strings.TrimSpace(r.URL.Query().Get("state"))),
		MinEvidenceCount: 0,
		Limit:            limit,
	}
	if raw := strings.TrimSpace(r.URL.Query().Get("min_confidence")); raw != "" {
		parsed, err := strconv.ParseFloat(raw, 64)
		if err != nil {
			http.Error(w, "invalid min_confidence", http.StatusBadRequest)
			return
		}
		input.MinConfidence = &parsed
	}
	if raw := strings.TrimSpace(r.URL.Query().Get("min_evidence_count")); raw != "" {
		parsed, err := strconv.Atoi(raw)
		if err != nil {
			http.Error(w, "invalid min_evidence_count", http.StatusBadRequest)
			return
		}
		input.MinEvidenceCount = parsed
	}
	if raw := strings.TrimSpace(r.URL.Query().Get("include_hidden")); raw != "" {
		parsed, err := strconv.ParseBool(raw)
		if err != nil {
			http.Error(w, "invalid include_hidden", http.StatusBadRequest)
			return
		}
		input.IncludeHidden = parsed
	}
	if err := input.Validate(); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	items, err := service.ListDerivedInsights(r.Context(), input)
	if err != nil {
		http.Error(w, "failed to read derived insights", http.StatusInternalServerError)
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{"items": items})
}

func handleAdminDerivedInsightDetail(w http.ResponseWriter, r *http.Request, service DerivedInsightAdminService) {
	if service == nil {
		http.Error(w, "derived insight admin service is not configured", http.StatusServiceUnavailable)
		return
	}

	scope, ok := auth.ScopeFromContext(r.Context())
	if !ok {
		http.Error(w, "scope context is missing", http.StatusInternalServerError)
		return
	}

	includeHidden := false
	if raw := strings.TrimSpace(r.URL.Query().Get("include_hidden")); raw != "" {
		parsed, err := strconv.ParseBool(raw)
		if err != nil {
			http.Error(w, "invalid include_hidden", http.StatusBadRequest)
			return
		}
		includeHidden = parsed
	}

	input := memory.ReadDerivedInsightInput{
		Scope:         scope,
		ID:            r.PathValue("insight_id"),
		IncludeHidden: includeHidden,
	}
	if err := input.Validate(); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	detail, err := service.ReadDerivedInsight(r.Context(), input)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			http.Error(w, "derived insight not found", http.StatusNotFound)
			return
		}
		http.Error(w, "failed to read derived insight", http.StatusInternalServerError)
		return
	}

	writeJSON(w, http.StatusOK, detail)
}

func handleAdminDerivedInsightFeedback(w http.ResponseWriter, r *http.Request, service DerivedInsightAdminService, metrics MetricsRecorder) {
	if service == nil {
		http.Error(w, "derived insight admin service is not configured", http.StatusServiceUnavailable)
		return
	}

	switch r.Method {
	case http.MethodGet:
		handleAdminDerivedInsightFeedbackList(w, r, service)
	case http.MethodPost:
		handleAdminDerivedInsightFeedbackCreate(w, r, service, metrics)
	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

func handleAdminDerivedInsightFeedbackCreate(w http.ResponseWriter, r *http.Request, service DerivedInsightAdminService, metrics MetricsRecorder) {
	scope, ok := auth.ScopeFromContext(r.Context())
	if !ok {
		http.Error(w, "scope context is missing", http.StatusInternalServerError)
		return
	}

	var request derivedInsightFeedbackCreateRequest
	if r.Body != nil {
		defer r.Body.Close()
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
			http.Error(w, "invalid JSON body", http.StatusBadRequest)
			return
		}
	}

	input := memory.CreateDerivedInsightFeedbackInput{
		ID:           newFeedbackID(),
		Scope:        scope,
		InsightID:    r.PathValue("insight_id"),
		Type:         request.Type,
		Actor:        strings.TrimSpace(request.Actor),
		Reason:       strings.TrimSpace(request.Reason),
		QualityScore: request.QualityScore,
		CreatedAt:    time.Now().UTC(),
		RequestID:    requestIDFromContext(r.Context()),
		Metadata:     request.Metadata,
	}
	if err := input.Validate(); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	feedback, err := service.CreateDerivedInsightFeedback(r.Context(), input)
	if err != nil {
		recordInsightFeedbackMetric(r.Context(), metrics, "create", "error", string(input.Type), "unknown", "none")
		if errors.Is(err, pgx.ErrNoRows) {
			http.Error(w, "derived insight not found", http.StatusNotFound)
			return
		}
		http.Error(w, "failed to create derived insight feedback", http.StatusInternalServerError)
		return
	}
	recordInsightFeedbackMetric(r.Context(), metrics, "create", "ok", string(feedback.Type), "unknown", "none")

	writeJSON(w, http.StatusCreated, feedback)
}

func handleAdminDerivedInsightFeedbackList(w http.ResponseWriter, r *http.Request, service DerivedInsightAdminService) {
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

	input := memory.ListDerivedInsightFeedbackInput{
		Scope:     scope,
		InsightID: r.PathValue("insight_id"),
		Type:      memory.InsightFeedbackType(strings.TrimSpace(r.URL.Query().Get("type"))),
		Limit:     limit,
	}
	if raw := strings.TrimSpace(r.URL.Query().Get("include_superseded")); raw != "" {
		parsed, err := strconv.ParseBool(raw)
		if err != nil {
			http.Error(w, "invalid include_superseded", http.StatusBadRequest)
			return
		}
		input.IncludeSuperseded = parsed
	}
	if err := input.Validate(); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	items, err := service.ListDerivedInsightFeedback(r.Context(), input)
	if err != nil {
		http.Error(w, "failed to read derived insight feedback", http.StatusInternalServerError)
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{"items": items})
}

func handleAdminDerivedInsightFeedbackAction(w http.ResponseWriter, r *http.Request, service DerivedInsightAdminService, metrics MetricsRecorder) {
	if service == nil {
		http.Error(w, "derived insight admin service is not configured", http.StatusServiceUnavailable)
		return
	}

	scope, ok := auth.ScopeFromContext(r.Context())
	if !ok {
		http.Error(w, "scope context is missing", http.StatusInternalServerError)
		return
	}

	feedbackID, action, err := parseDerivedInsightFeedbackActionTarget(r.PathValue("feedback_action"))
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	if action != "supersede" {
		http.Error(w, "unsupported derived insight feedback action", http.StatusBadRequest)
		return
	}

	var request derivedInsightFeedbackSupersedeRequest
	if r.Body != nil {
		defer r.Body.Close()
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
			http.Error(w, "invalid JSON body", http.StatusBadRequest)
			return
		}
	}

	input := memory.SupersedeDerivedInsightFeedbackInput{
		Scope:        scope,
		FeedbackID:   feedbackID,
		Actor:        strings.TrimSpace(request.Actor),
		Reason:       strings.TrimSpace(request.Reason),
		SupersededAt: time.Now().UTC(),
	}
	if err := input.Validate(); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	if err := service.SupersedeDerivedInsightFeedback(r.Context(), input); err != nil {
		recordInsightFeedbackMetric(r.Context(), metrics, "supersede", "error", "unknown", "unknown", "none")
		if errors.Is(err, pgx.ErrNoRows) {
			http.Error(w, "derived insight feedback not found", http.StatusNotFound)
			return
		}
		http.Error(w, "failed to supersede derived insight feedback", http.StatusInternalServerError)
		return
	}
	recordInsightFeedbackMetric(r.Context(), metrics, "supersede", "ok", "unknown", "unknown", "none")

	writeJSON(w, http.StatusOK, map[string]any{"status": "superseded", "feedback_id": feedbackID})
}

func handleAdminDerivedInsightAction(w http.ResponseWriter, r *http.Request, service DerivedInsightAdminService) {
	if service == nil {
		http.Error(w, "derived insight admin service is not configured", http.StatusServiceUnavailable)
		return
	}

	scope, ok := auth.ScopeFromContext(r.Context())
	if !ok {
		http.Error(w, "scope context is missing", http.StatusInternalServerError)
		return
	}

	insightID, action, err := parseDerivedInsightActionTarget(r.PathValue("insight_action"))
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	if action != "suppress" {
		http.Error(w, "unsupported derived insight action", http.StatusBadRequest)
		return
	}

	var request derivedInsightSuppressRequest
	if r.Body != nil {
		defer r.Body.Close()
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
			http.Error(w, "invalid JSON body", http.StatusBadRequest)
			return
		}
	}

	transition := memory.DerivedInsightLifecycleTransition{
		Scope:      scope,
		InsightID:  insightID,
		FromState:  memory.DerivedInsightStateActive,
		ToState:    memory.DerivedInsightStateSuppressed,
		Actor:      strings.TrimSpace(request.Actor),
		Reason:     strings.TrimSpace(request.Reason),
		OccurredAt: time.Now().UTC(),
	}
	if err := transition.Validate(); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	if err := service.TransitionDerivedInsightLifecycle(r.Context(), transition); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			http.Error(w, "derived insight not found", http.StatusNotFound)
			return
		}
		http.Error(w, "failed to update derived insight lifecycle", http.StatusInternalServerError)
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{"status": "suppressed", "insight_id": insightID})
}

func handleAdminDerivedInsightReplayDryRun(w http.ResponseWriter, r *http.Request, service DerivedInsightReplayAdminService) {
	if service == nil {
		http.Error(w, "derived insight replay admin service is not configured", http.StatusServiceUnavailable)
		return
	}

	input, ok := decodeDerivedInsightReplayRequest(w, r, memory.DerivedInsightReplayModeDryRun)
	if !ok {
		return
	}

	report, err := service.PlanDerivedInsightReplay(r.Context(), input)
	if err != nil {
		writeAdminDerivedInsightReplayError(w, err, "failed to plan derived insight replay")
		return
	}

	writeJSON(w, http.StatusOK, report)
}

func handleAdminDerivedInsightReplays(w http.ResponseWriter, r *http.Request, service DerivedInsightReplayAdminService) {
	if service == nil {
		http.Error(w, "derived insight replay admin service is not configured", http.StatusServiceUnavailable)
		return
	}

	switch r.Method {
	case http.MethodGet:
		handleAdminDerivedInsightReplayList(w, r, service)
	case http.MethodPost:
		handleAdminDerivedInsightReplayApply(w, r, service)
	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

func handleAdminDerivedInsightReplayApply(w http.ResponseWriter, r *http.Request, service DerivedInsightReplayAdminService) {
	input, ok := decodeDerivedInsightReplayRequest(w, r, memory.DerivedInsightReplayModeApply)
	if !ok {
		return
	}

	run, err := service.ApplyDerivedInsightReplay(r.Context(), input)
	if err != nil {
		writeAdminDerivedInsightReplayError(w, err, "failed to apply derived insight replay")
		return
	}

	writeJSON(w, http.StatusAccepted, run)
}

func handleAdminDerivedInsightReplayList(w http.ResponseWriter, r *http.Request, service DerivedInsightReplayAdminService) {
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

	input := memory.ListDerivedInsightReplayRunsInput{
		Scope:  scope,
		Status: memory.DerivedInsightReplayStatus(strings.TrimSpace(r.URL.Query().Get("status"))),
		Mode:   memory.DerivedInsightReplayMode(strings.TrimSpace(r.URL.Query().Get("mode"))),
		Limit:  limit,
	}
	if err := input.Validate(); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	items, err := service.ListDerivedInsightReplayRuns(r.Context(), input)
	if err != nil {
		writeAdminDerivedInsightReplayError(w, err, "failed to list derived insight replays")
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{"items": items})
}

func handleAdminDerivedInsightReplayDetail(w http.ResponseWriter, r *http.Request, service DerivedInsightReplayAdminService) {
	if service == nil {
		http.Error(w, "derived insight replay admin service is not configured", http.StatusServiceUnavailable)
		return
	}

	input, ok := replayRunReadInputFromRequest(w, r)
	if !ok {
		return
	}

	run, err := service.ReadDerivedInsightReplayRun(r.Context(), input)
	if err != nil {
		writeAdminDerivedInsightReplayError(w, err, "failed to read derived insight replay")
		return
	}

	writeJSON(w, http.StatusOK, run)
}

func handleAdminDerivedInsightReplayReport(w http.ResponseWriter, r *http.Request, service DerivedInsightReplayAdminService) {
	if service == nil {
		http.Error(w, "derived insight replay admin service is not configured", http.StatusServiceUnavailable)
		return
	}

	input, ok := replayRunReadInputFromRequest(w, r)
	if !ok {
		return
	}

	report, err := service.ReadDerivedInsightReplayReport(r.Context(), input)
	if err != nil {
		writeAdminDerivedInsightReplayError(w, err, "failed to read derived insight replay report")
		return
	}

	writeJSON(w, http.StatusOK, report)
}

func decodeDerivedInsightReplayRequest(w http.ResponseWriter, r *http.Request, mode memory.DerivedInsightReplayMode) (memory.DerivedInsightReplayRequest, bool) {
	scope, ok := auth.ScopeFromContext(r.Context())
	if !ok {
		http.Error(w, "scope context is missing", http.StatusInternalServerError)
		return memory.DerivedInsightReplayRequest{}, false
	}

	var request derivedInsightReplayRequest
	if !decodeJSONBody(w, r, &request) {
		return memory.DerivedInsightReplayRequest{}, false
	}

	windowStart, err := time.Parse(time.RFC3339, strings.TrimSpace(request.EvidenceWindowStart))
	if err != nil {
		http.Error(w, "invalid evidence_window_start", http.StatusBadRequest)
		return memory.DerivedInsightReplayRequest{}, false
	}
	windowEnd, err := time.Parse(time.RFC3339, strings.TrimSpace(request.EvidenceWindowEnd))
	if err != nil {
		http.Error(w, "invalid evidence_window_end", http.StatusBadRequest)
		return memory.DerivedInsightReplayRequest{}, false
	}

	input := memory.DerivedInsightReplayRequest{
		Scope:               scope,
		Mode:                mode,
		InsightTypes:        request.InsightTypes,
		EvidenceWindowStart: windowStart,
		EvidenceWindowEnd:   windowEnd,
		EvidenceLimit:       request.EvidenceLimit,
		Actor:               strings.TrimSpace(request.Actor),
		Reason:              strings.TrimSpace(request.Reason),
		IdempotencyKey:      strings.TrimSpace(request.IdempotencyKey),
		RequestedAt:         time.Now().UTC(),
		Metadata:            request.Metadata,
	}
	if err := input.Validate(); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return memory.DerivedInsightReplayRequest{}, false
	}

	return input, true
}

func replayRunReadInputFromRequest(w http.ResponseWriter, r *http.Request) (memory.ReadDerivedInsightReplayRunInput, bool) {
	scope, ok := auth.ScopeFromContext(r.Context())
	if !ok {
		http.Error(w, "scope context is missing", http.StatusInternalServerError)
		return memory.ReadDerivedInsightReplayRunInput{}, false
	}

	input := memory.ReadDerivedInsightReplayRunInput{
		Scope: scope,
		RunID: r.PathValue("replay_run_id"),
	}
	if err := input.Validate(); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return memory.ReadDerivedInsightReplayRunInput{}, false
	}

	return input, true
}

func handleAdminEmbeddingRebuildList(w http.ResponseWriter, r *http.Request, service EmbeddingAdminQueryService) {
	if service == nil {
		http.Error(w, "embedding admin service is not configured", http.StatusServiceUnavailable)
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

	input := memory.ListEmbeddingRebuildsInput{
		Scope:             scope,
		Status:            memory.EmbeddingRebuildStatus(strings.TrimSpace(r.URL.Query().Get("status"))),
		RequestedProvider: strings.TrimSpace(r.URL.Query().Get("requested_provider")),
		RequestedModel:    strings.TrimSpace(r.URL.Query().Get("requested_model")),
		Limit:             limit,
	}
	if raw := strings.TrimSpace(r.URL.Query().Get("drifted")); raw != "" {
		parsed, err := strconv.ParseBool(raw)
		if err != nil {
			http.Error(w, "invalid drifted", http.StatusBadRequest)
			return
		}
		input.Drifted = &parsed
	}
	if err := input.Validate(); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	page, err := service.ListEmbeddingRebuilds(r.Context(), input)
	if err != nil {
		http.Error(w, "failed to read embedding rebuilds", http.StatusInternalServerError)
		return
	}

	writeJSON(w, http.StatusOK, page)
}

func handleAdminMemoryEmbedding(w http.ResponseWriter, r *http.Request, service EmbeddingAdminQueryService) {
	if service == nil {
		http.Error(w, "embedding admin service is not configured", http.StatusServiceUnavailable)
		return
	}

	scope, ok := auth.ScopeFromContext(r.Context())
	if !ok {
		http.Error(w, "scope context is missing", http.StatusInternalServerError)
		return
	}

	inspection, err := service.GetMemoryEmbedding(r.Context(), scope, r.PathValue("memory_id"))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			http.Error(w, "memory not found", http.StatusNotFound)
			return
		}
		if strings.Contains(err.Error(), "memory id is required") || strings.Contains(err.Error(), "tenant is required") || strings.Contains(err.Error(), "project is required") || strings.Contains(err.Error(), "namespace is required") {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		http.Error(w, "failed to read memory embedding", http.StatusInternalServerError)
		return
	}

	writeJSON(w, http.StatusOK, inspection)
}

func handleAdminGovernanceRawEventList(w http.ResponseWriter, r *http.Request, service GovernanceAdminService) {
	if service == nil {
		http.Error(w, "governance admin service is not configured", http.StatusServiceUnavailable)
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

	input := governance.ListGovernanceRawEventsInput{
		Scope:     scope,
		State:     governance.GovernanceRawEventState(strings.TrimSpace(r.URL.Query().Get("state"))),
		EventType: strings.TrimSpace(r.URL.Query().Get("event_type")),
		Limit:     limit,
		Cursor:    strings.TrimSpace(r.URL.Query().Get("cursor")),
		Now:       time.Now().UTC(),
	}

	if raw := strings.TrimSpace(r.URL.Query().Get("attempt_gte")); raw != "" {
		parsed, err := strconv.Atoi(raw)
		if err != nil {
			http.Error(w, "invalid attempt_gte", http.StatusBadRequest)
			return
		}
		input.AttemptGTE = &parsed
	}
	if raw := strings.TrimSpace(r.URL.Query().Get("attempt_lte")); raw != "" {
		parsed, err := strconv.Atoi(raw)
		if err != nil {
			http.Error(w, "invalid attempt_lte", http.StatusBadRequest)
			return
		}
		input.AttemptLTE = &parsed
	}
	if raw := strings.TrimSpace(r.URL.Query().Get("failed_from")); raw != "" {
		parsed, err := time.Parse(time.RFC3339, raw)
		if err != nil {
			http.Error(w, "invalid failed_from", http.StatusBadRequest)
			return
		}
		input.FailedFrom = parsed
	}
	if raw := strings.TrimSpace(r.URL.Query().Get("failed_to")); raw != "" {
		parsed, err := time.Parse(time.RFC3339, raw)
		if err != nil {
			http.Error(w, "invalid failed_to", http.StatusBadRequest)
			return
		}
		input.FailedTo = parsed
	}
	if raw := strings.TrimSpace(r.URL.Query().Get("next_attempt_from")); raw != "" {
		parsed, err := time.Parse(time.RFC3339, raw)
		if err != nil {
			http.Error(w, "invalid next_attempt_from", http.StatusBadRequest)
			return
		}
		input.NextAttemptFrom = parsed
	}
	if raw := strings.TrimSpace(r.URL.Query().Get("next_attempt_to")); raw != "" {
		parsed, err := time.Parse(time.RFC3339, raw)
		if err != nil {
			http.Error(w, "invalid next_attempt_to", http.StatusBadRequest)
			return
		}
		input.NextAttemptTo = parsed
	}

	page, err := service.ListGovernanceRawEvents(r.Context(), input)
	if err != nil {
		writeAdminGovernanceError(w, err, "failed to list governance raw events")
		return
	}

	writeJSON(w, http.StatusOK, page)
}

func handleAdminGovernanceRawEventDetail(w http.ResponseWriter, r *http.Request, service GovernanceAdminService) {
	if service == nil {
		http.Error(w, "governance admin service is not configured", http.StatusServiceUnavailable)
		return
	}

	scope, ok := auth.ScopeFromContext(r.Context())
	if !ok {
		http.Error(w, "scope context is missing", http.StatusInternalServerError)
		return
	}

	resource, err := service.ReadGovernanceRawEvent(r.Context(), governance.ReadGovernanceRawEventInput{
		Scope:      scope,
		RawEventID: r.PathValue("raw_event_id"),
		Now:        time.Now().UTC(),
	})
	if err != nil {
		writeAdminGovernanceError(w, err, "failed to read governance raw event")
		return
	}

	writeJSON(w, http.StatusOK, resource)
}

func handleAdminGovernanceRecoveryHistory(w http.ResponseWriter, r *http.Request, service GovernanceAdminService) {
	if service == nil {
		http.Error(w, "governance admin service is not configured", http.StatusServiceUnavailable)
		return
	}

	scope, ok := auth.ScopeFromContext(r.Context())
	if !ok {
		http.Error(w, "scope context is missing", http.StatusInternalServerError)
		return
	}

	history, err := service.ListGovernanceRecoveryHistory(r.Context(), governance.ListGovernanceRecoveryHistoryInput{
		Scope:      scope,
		RawEventID: r.PathValue("raw_event_id"),
	})
	if err != nil {
		writeAdminGovernanceError(w, err, "failed to read governance recovery history")
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{"history": history})
}

func handleAdminGovernanceRawEventAction(w http.ResponseWriter, r *http.Request, service GovernanceAdminService) {
	if service == nil {
		http.Error(w, "governance admin service is not configured", http.StatusServiceUnavailable)
		return
	}

	var req governanceRecoveryRequest
	if !decodeJSONBody(w, r, &req) {
		return
	}

	scope, ok := auth.ScopeFromContext(r.Context())
	if !ok {
		http.Error(w, "scope context is missing", http.StatusInternalServerError)
		return
	}

	rawEventID, action, err := parseGovernanceRawEventActionTarget(r.PathValue("raw_event_action"))
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	input := governance.ApplyGovernanceRecoveryInput{
		Scope:      scope,
		RawEventID: rawEventID,
		Action:     action,
		Actor:      strings.TrimSpace(r.Header.Get("X-Stele-Actor")),
		Reason:     req.Reason,
		AppliedAt:  time.Now().UTC(),
	}
	if strings.TrimSpace(req.ScheduledFor) != "" {
		scheduledFor, err := time.Parse(time.RFC3339, req.ScheduledFor)
		if err != nil {
			http.Error(w, "invalid scheduled_for", http.StatusBadRequest)
			return
		}
		input.ScheduledFor = scheduledFor
	}

	outcome, err := service.ApplyGovernanceRecovery(r.Context(), input)
	if err != nil {
		writeAdminGovernanceError(w, err, "failed to apply governance recovery")
		return
	}

	writeJSON(w, http.StatusOK, outcome)
}

func handleAdminEmbeddingRebuildAction(w http.ResponseWriter, r *http.Request, service EmbeddingAdminQueryService) {
	if service == nil {
		http.Error(w, "embedding admin service is not configured", http.StatusServiceUnavailable)
		return
	}

	var req lifecycleActionRequest
	if !decodeJSONBody(w, r, &req) {
		return
	}

	scope, ok := auth.ScopeFromContext(r.Context())
	if !ok {
		http.Error(w, "scope context is missing", http.StatusInternalServerError)
		return
	}

	memoryID, action, err := parseEmbeddingRebuildActionTarget(r.PathValue("embedding_action"))
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	outcome, err := service.ApplyEmbeddingRecovery(r.Context(), memory.ApplyEmbeddingRecoveryInput{
		Scope:     scope,
		MemoryID:  memoryID,
		Action:    action,
		Actor:     strings.TrimSpace(r.Header.Get("X-Stele-Actor")),
		Reason:    req.Reason,
		AppliedAt: time.Now().UTC(),
	})
	if err != nil {
		writeAdminEmbeddingError(w, err, "failed to apply embedding recovery")
		return
	}

	writeJSON(w, http.StatusOK, outcome)
}

func handleAdminEmbeddingCutoverCreate(w http.ResponseWriter, r *http.Request, service EmbeddingAdminQueryService) {
	if service == nil {
		http.Error(w, "embedding admin service is not configured", http.StatusServiceUnavailable)
		return
	}

	var req embeddingCutoverCreateRequest
	if !decodeJSONBody(w, r, &req) {
		return
	}

	scope, ok := auth.ScopeFromContext(r.Context())
	if !ok {
		http.Error(w, "scope context is missing", http.StatusInternalServerError)
		return
	}

	plan, err := service.CreateEmbeddingCutoverPlan(r.Context(), memory.CreateEmbeddingCutoverPlanInput{
		Scope:     scope,
		Target:    req.Target,
		Classes:   req.Classes,
		WaveSize:  req.WaveSize,
		Actor:     strings.TrimSpace(r.Header.Get("X-Stele-Actor")),
		Reason:    req.Reason,
		CreatedAt: time.Now().UTC(),
	})
	if err != nil {
		writeAdminEmbeddingCutoverError(w, err, "failed to create embedding cutover plan")
		return
	}

	writeJSON(w, http.StatusCreated, plan)
}

func handleAdminEmbeddingCutoverList(w http.ResponseWriter, r *http.Request, service EmbeddingAdminQueryService, metrics MetricsRecorder) {
	if service == nil {
		http.Error(w, "embedding admin service is not configured", http.StatusServiceUnavailable)
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

	plans, err := service.ListEmbeddingCutoverPlans(r.Context(), memory.ListEmbeddingCutoverPlansInput{
		Scope:  scope,
		Status: memory.EmbeddingCutoverPlanStatus(strings.TrimSpace(r.URL.Query().Get("status"))),
		Limit:  limit,
	})
	if err != nil {
		writeAdminEmbeddingCutoverError(w, err, "failed to list embedding cutover plans")
		return
	}

	recordEmbeddingCutoverStateMetrics(r.Context(), metrics, plans)
	writeJSON(w, http.StatusOK, map[string]any{"plans": plans})
}

func handleAdminEmbeddingCutoverDetail(w http.ResponseWriter, r *http.Request, service EmbeddingAdminQueryService, metrics MetricsRecorder) {
	if service == nil {
		http.Error(w, "embedding admin service is not configured", http.StatusServiceUnavailable)
		return
	}

	scope, ok := auth.ScopeFromContext(r.Context())
	if !ok {
		http.Error(w, "scope context is missing", http.StatusInternalServerError)
		return
	}

	plan, err := service.ReadEmbeddingCutoverPlan(r.Context(), memory.ReadEmbeddingCutoverPlanInput{
		Scope:  scope,
		PlanID: r.PathValue("cutover_plan_id"),
	})
	if err != nil {
		writeAdminEmbeddingCutoverError(w, err, "failed to read embedding cutover plan")
		return
	}

	recordEmbeddingCutoverStateMetrics(r.Context(), metrics, []memory.EmbeddingCutoverPlan{plan})
	writeJSON(w, http.StatusOK, plan)
}

func handleAdminEmbeddingCutoverPreflight(w http.ResponseWriter, r *http.Request, service EmbeddingAdminQueryService) {
	if service == nil {
		http.Error(w, "embedding admin service is not configured", http.StatusServiceUnavailable)
		return
	}

	scope, ok := auth.ScopeFromContext(r.Context())
	if !ok {
		http.Error(w, "scope context is missing", http.StatusInternalServerError)
		return
	}

	report, err := service.PreflightEmbeddingCutoverPlan(r.Context(), memory.EmbeddingCutoverPreflightInput{
		Scope:      scope,
		PlanID:     r.PathValue("cutover_plan_id"),
		ObservedAt: time.Now().UTC(),
	})
	if err != nil {
		writeAdminEmbeddingCutoverError(w, err, "failed to preflight embedding cutover plan")
		return
	}

	writeJSON(w, http.StatusOK, report)
}

func handleAdminEmbeddingCutoverAction(w http.ResponseWriter, r *http.Request, service EmbeddingAdminQueryService, metrics MetricsRecorder) {
	if service == nil {
		http.Error(w, "embedding admin service is not configured", http.StatusServiceUnavailable)
		return
	}

	scope, ok := auth.ScopeFromContext(r.Context())
	if !ok {
		http.Error(w, "scope context is missing", http.StatusInternalServerError)
		return
	}

	if planID, ok := parseEmbeddingCutoverPreflightTarget(r.PathValue("cutover_action")); ok {
		report, err := service.PreflightEmbeddingCutoverPlan(r.Context(), memory.EmbeddingCutoverPreflightInput{
			Scope:      scope,
			PlanID:     planID,
			ObservedAt: time.Now().UTC(),
		})
		if err != nil {
			writeAdminEmbeddingCutoverError(w, err, "failed to preflight embedding cutover plan")
			return
		}
		recordCutoverAdmissionMetrics(r.Context(), metrics, "preflight", report)
		writeJSON(w, http.StatusOK, report)
		return
	}

	var req lifecycleActionRequest
	if !decodeJSONBody(w, r, &req) {
		return
	}

	planID, action, err := parseEmbeddingCutoverActionTarget(r.PathValue("cutover_action"))
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	plan, err := service.ApplyEmbeddingCutoverPlanAction(r.Context(), memory.ApplyEmbeddingCutoverPlanActionInput{
		Scope:     scope,
		PlanID:    planID,
		Action:    action,
		Actor:     strings.TrimSpace(r.Header.Get("X-Stele-Actor")),
		Reason:    req.Reason,
		AppliedAt: time.Now().UTC(),
	})
	if err != nil {
		var admissionErr memory.EmbeddingCutoverAdmissionError
		if errors.As(err, &admissionErr) {
			recordCutoverAdmissionMetrics(r.Context(), metrics, "activate", admissionErr.Report)
		}
		writeAdminEmbeddingCutoverError(w, err, "failed to apply embedding cutover action")
		return
	}

	writeJSON(w, http.StatusOK, plan)
}

func handleAdminEmbeddingRecoveryHistory(w http.ResponseWriter, r *http.Request, service EmbeddingAdminQueryService, memoryID string) {
	if service == nil {
		http.Error(w, "embedding admin service is not configured", http.StatusServiceUnavailable)
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

	input := memory.ListEmbeddingRecoveryHistoryInput{
		Scope:         scope,
		MemoryID:      strings.TrimSpace(memoryID),
		Action:        memory.EmbeddingRecoveryAction(strings.TrimSpace(r.URL.Query().Get("action"))),
		Actor:         strings.TrimSpace(r.URL.Query().Get("actor")),
		CutoverPlanID: strings.TrimSpace(r.URL.Query().Get("cutover_plan_id")),
		Limit:         limit,
	}
	if raw := strings.TrimSpace(r.URL.Query().Get("occurred_from")); raw != "" {
		parsed, err := time.Parse(time.RFC3339, raw)
		if err != nil {
			http.Error(w, "invalid occurred_from", http.StatusBadRequest)
			return
		}
		input.OccurredFrom = parsed
	}
	if raw := strings.TrimSpace(r.URL.Query().Get("occurred_to")); raw != "" {
		parsed, err := time.Parse(time.RFC3339, raw)
		if err != nil {
			http.Error(w, "invalid occurred_to", http.StatusBadRequest)
			return
		}
		input.OccurredTo = parsed
	}

	history, err := service.ListEmbeddingRecoveryHistory(r.Context(), input)
	if err != nil {
		writeAdminEmbeddingError(w, err, "failed to read embedding recovery history")
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{"history": history})
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

func handleAdminMemoryCreate(w http.ResponseWriter, r *http.Request, service ManualMemoryMutationService) {
	if service == nil {
		http.Error(w, "manual memory mutation service is not configured", http.StatusServiceUnavailable)
		return
	}

	var req manualCreateMemoryRequest
	if !decodeJSONBody(w, r, &req) {
		return
	}

	scope, ok := auth.ScopeFromContext(r.Context())
	if !ok {
		http.Error(w, "scope context is missing", http.StatusInternalServerError)
		return
	}

	input := memory.ManualCreateMemoryInput{
		Scope:     scope,
		Class:     req.Class,
		Content:   req.Content,
		Reason:    req.Reason,
		Actor:     strings.TrimSpace(r.Header.Get("X-Stele-Actor")),
		RequestID: strings.TrimSpace(r.Header.Get("X-Request-ID")),
	}
	if err := input.Validate(); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	resource, err := service.CreateMemory(r.Context(), input)
	if err != nil {
		writeAdminManualMutationError(w, err, "failed to create memory")
		return
	}

	writeJSON(w, http.StatusCreated, resource)
}

func handleAdminMemoryUpdate(w http.ResponseWriter, r *http.Request, service ManualMemoryMutationService) {
	if service == nil {
		http.Error(w, "manual memory mutation service is not configured", http.StatusServiceUnavailable)
		return
	}

	var req manualUpdateMemoryRequest
	if !decodeJSONBody(w, r, &req) {
		return
	}

	scope, ok := auth.ScopeFromContext(r.Context())
	if !ok {
		http.Error(w, "scope context is missing", http.StatusInternalServerError)
		return
	}

	input := memory.ManualUpdateMemoryInput{
		Scope:           scope,
		MemoryID:        r.PathValue("memory_id"),
		Content:         req.Content,
		ExpectedVersion: req.ExpectedVersion,
		Reason:          req.Reason,
		Actor:           strings.TrimSpace(r.Header.Get("X-Stele-Actor")),
		RequestID:       strings.TrimSpace(r.Header.Get("X-Request-ID")),
	}
	if err := input.Validate(); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	resource, err := service.UpdateMemory(r.Context(), input)
	if err != nil {
		writeAdminManualMutationError(w, err, "failed to update memory")
		return
	}

	writeJSON(w, http.StatusOK, resource)
}

func handleAdminMemoryAction(w http.ResponseWriter, r *http.Request, lifecycleService MemoryLifecycleActionService, mutationService ManualMemoryMutationService) {
	actionTarget := r.PathValue("memory_action")
	switch {
	case strings.HasSuffix(actionTarget, ":merge"):
		handleAdminMemoryMergeAction(w, r, mutationService)
	case strings.HasSuffix(actionTarget, ":reclassify"):
		handleAdminMemoryReclassifyAction(w, r, mutationService)
	default:
		handleMemoryLifecycleAction(w, r, lifecycleService)
	}
}

func handleAdminMemoryMergeAction(w http.ResponseWriter, r *http.Request, service ManualMemoryMutationService) {
	if service == nil {
		http.Error(w, "manual memory mutation service is not configured", http.StatusServiceUnavailable)
		return
	}

	var req manualMergeMemoryRequest
	if !decodeJSONBody(w, r, &req) {
		return
	}

	scope, ok := auth.ScopeFromContext(r.Context())
	if !ok {
		http.Error(w, "scope context is missing", http.StatusInternalServerError)
		return
	}

	memoryID, err := parseAdminMutationActionTarget(r.PathValue("memory_action"), "merge")
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	input := memory.ManualMergeMemoryInput{
		Scope:           scope,
		TargetMemoryID:  memoryID,
		SourceMemoryID:  req.SourceMemoryID,
		Content:         req.Content,
		ExpectedVersion: req.ExpectedVersion,
		Reason:          req.Reason,
		Actor:           strings.TrimSpace(r.Header.Get("X-Stele-Actor")),
		RequestID:       strings.TrimSpace(r.Header.Get("X-Request-ID")),
	}
	if err := input.Validate(); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	resource, err := service.MergeMemory(r.Context(), input)
	if err != nil {
		writeAdminManualMutationError(w, err, "failed to merge memory")
		return
	}

	writeJSON(w, http.StatusOK, resource)
}

func handleAdminMemoryReclassifyAction(w http.ResponseWriter, r *http.Request, service ManualMemoryMutationService) {
	if service == nil {
		http.Error(w, "manual memory mutation service is not configured", http.StatusServiceUnavailable)
		return
	}

	var req manualReclassifyMemoryRequest
	if !decodeJSONBody(w, r, &req) {
		return
	}

	scope, ok := auth.ScopeFromContext(r.Context())
	if !ok {
		http.Error(w, "scope context is missing", http.StatusInternalServerError)
		return
	}

	memoryID, err := parseAdminMutationActionTarget(r.PathValue("memory_action"), "reclassify")
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	input := memory.ManualReclassifyMemoryInput{
		Scope:           scope,
		MemoryID:        memoryID,
		TargetClass:     req.TargetClass,
		ExpectedVersion: req.ExpectedVersion,
		Reason:          req.Reason,
		Actor:           strings.TrimSpace(r.Header.Get("X-Stele-Actor")),
		RequestID:       strings.TrimSpace(r.Header.Get("X-Request-ID")),
	}
	if err := input.Validate(); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	resource, err := service.ReclassifyMemory(r.Context(), input)
	if err != nil {
		writeAdminManualMutationError(w, err, "failed to reclassify memory")
		return
	}

	writeJSON(w, http.StatusOK, resource)
}

func writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}

func decodeJSONBody(w http.ResponseWriter, r *http.Request, target any) bool {
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return false
	}
	if decoder.More() {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return false
	}

	return true
}

func writeAdminManualMutationError(w http.ResponseWriter, err error, fallback string) {
	switch {
	case errors.Is(err, memory.ErrManualMutationVersionConflict):
		http.Error(w, err.Error(), http.StatusConflict)
	case errors.Is(err, memory.ErrManualMutationRejected):
		http.Error(w, err.Error(), http.StatusBadRequest)
	case errors.Is(err, pgx.ErrNoRows):
		http.Error(w, "memory not found", http.StatusNotFound)
	default:
		http.Error(w, fallback, http.StatusInternalServerError)
	}
}

func writeAdminGovernanceError(w http.ResponseWriter, err error, fallback string) {
	switch {
	case errors.Is(err, governance.ErrGovernanceRecoveryConflict):
		http.Error(w, err.Error(), http.StatusConflict)
	case errors.Is(err, governance.ErrGovernanceRecoveryRejected):
		http.Error(w, err.Error(), http.StatusUnprocessableEntity)
	case errors.Is(err, pgx.ErrNoRows):
		http.Error(w, "raw event not found", http.StatusNotFound)
	default:
		if strings.Contains(err.Error(), "required") || strings.Contains(err.Error(), "invalid") || strings.Contains(err.Error(), "must be") {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		http.Error(w, fallback, http.StatusInternalServerError)
	}
}

func writeAdminEmbeddingError(w http.ResponseWriter, err error, fallback string) {
	switch {
	case errors.Is(err, memory.ErrEmbeddingRecoveryConflict):
		http.Error(w, err.Error(), http.StatusConflict)
	case errors.Is(err, pgx.ErrNoRows):
		http.Error(w, "memory not found", http.StatusNotFound)
	default:
		if strings.Contains(err.Error(), "required") || strings.Contains(err.Error(), "invalid") || strings.Contains(err.Error(), "must be") {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		http.Error(w, fallback, http.StatusInternalServerError)
	}
}

func writeAdminDerivedInsightReplayError(w http.ResponseWriter, err error, fallback string) {
	switch {
	case errors.Is(err, pgx.ErrNoRows):
		http.Error(w, "derived insight replay not found", http.StatusNotFound)
	default:
		if strings.Contains(err.Error(), "required") || strings.Contains(err.Error(), "invalid") || strings.Contains(err.Error(), "must be") || strings.Contains(err.Error(), "not supported") {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		http.Error(w, fallback, http.StatusInternalServerError)
	}
}

func writeAdminEmbeddingCutoverError(w http.ResponseWriter, err error, fallback string) {
	var admissionErr memory.EmbeddingCutoverAdmissionError
	if errors.As(err, &admissionErr) {
		writeJSON(w, http.StatusUnprocessableEntity, admissionErr.Report)
		return
	}
	switch {
	case errors.Is(err, memory.ErrEmbeddingCutoverConflict):
		http.Error(w, err.Error(), http.StatusConflict)
	case errors.Is(err, memory.ErrEmbeddingCutoverRejected):
		http.Error(w, err.Error(), http.StatusUnprocessableEntity)
	case errors.Is(err, pgx.ErrNoRows):
		http.Error(w, "cutover plan not found", http.StatusNotFound)
	default:
		if strings.Contains(err.Error(), "required") || strings.Contains(err.Error(), "invalid") || strings.Contains(err.Error(), "must be") {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		http.Error(w, fallback, http.StatusInternalServerError)
	}
}

func recordCutoverAdmissionMetrics(ctx context.Context, metrics MetricsRecorder, operation string, report memory.EmbeddingCutoverPreflightReport) {
	if metrics == nil {
		return
	}
	metrics.RecordAdmission(ctx, telemetry.AdmissionEvent{
		Component: report.Component,
		Operation: operation,
		Decision:  string(report.Decision),
		Blockers:  report.Blockers,
		Warnings:  report.Warnings,
	})
}

func recordEmbeddingCutoverStateMetrics(ctx context.Context, metrics MetricsRecorder, plans []memory.EmbeddingCutoverPlan) {
	if metrics == nil {
		return
	}

	planCounts := make(map[memory.EmbeddingCutoverPlanStatus]int64)
	for _, status := range []memory.EmbeddingCutoverPlanStatus{
		memory.EmbeddingCutoverPlanStatusActive,
		memory.EmbeddingCutoverPlanStatusPaused,
	} {
		planCounts[status] = 0
	}
	itemCounts := make(map[memory.EmbeddingCutoverItemStatus]int64)
	for _, status := range []memory.EmbeddingCutoverItemStatus{
		memory.EmbeddingCutoverItemStatusQueued,
		memory.EmbeddingCutoverItemStatusRebuilding,
		memory.EmbeddingCutoverItemStatusCurrent,
		memory.EmbeddingCutoverItemStatusFailed,
		memory.EmbeddingCutoverItemStatusSkipped,
		memory.EmbeddingCutoverItemStatusPaused,
		memory.EmbeddingCutoverItemStatusCancelled,
	} {
		itemCounts[status] = 0
	}
	for _, plan := range plans {
		planCounts[plan.Status]++
		progress := plan.Progress
		itemCounts[memory.EmbeddingCutoverItemStatusQueued] += int64(progress.Queued)
		itemCounts[memory.EmbeddingCutoverItemStatusRebuilding] += int64(progress.Rebuilding)
		itemCounts[memory.EmbeddingCutoverItemStatusCurrent] += int64(progress.Current)
		itemCounts[memory.EmbeddingCutoverItemStatusFailed] += int64(progress.Failed)
		itemCounts[memory.EmbeddingCutoverItemStatusSkipped] += int64(progress.Skipped)
		itemCounts[memory.EmbeddingCutoverItemStatusPaused] += int64(progress.Paused)
		itemCounts[memory.EmbeddingCutoverItemStatusCancelled] += int64(progress.Cancelled)
	}

	for status, count := range planCounts {
		if status == "" {
			continue
		}
		metrics.RecordCutoverPlanState(ctx, telemetry.CutoverPlanStateEvent{
			Status: string(status),
			Count:  count,
		})
	}
	for status, count := range itemCounts {
		if status == "" {
			continue
		}
		metrics.RecordCutoverItemState(ctx, telemetry.CutoverItemStateEvent{
			Status: string(status),
			Count:  count,
		})
	}
}

func recordInsightFeedbackMetric(ctx context.Context, metrics MetricsRecorder, operation, result, feedbackType, insightType, decision string) {
	if metrics == nil {
		return
	}
	metrics.RecordInsightFeedback(ctx, telemetry.InsightFeedbackEvent{
		Operation:    operation,
		Result:       result,
		FeedbackType: feedbackType,
		InsightType:  insightType,
		Decision:     decision,
	})
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

func parseAdminMutationActionTarget(value string, expectedAction string) (string, error) {
	memoryID, actionName, ok := strings.Cut(value, ":")
	if !ok || strings.TrimSpace(memoryID) == "" || actionName != expectedAction {
		return "", fmt.Errorf("invalid memory action target")
	}

	return memoryID, nil
}

func parseDerivedInsightActionTarget(value string) (string, string, error) {
	insightID, actionName, ok := strings.Cut(value, ":")
	if !ok || strings.TrimSpace(insightID) == "" || strings.TrimSpace(actionName) == "" {
		return "", "", fmt.Errorf("invalid derived insight action target")
	}

	return strings.TrimSpace(insightID), strings.TrimSpace(actionName), nil
}

func parseDerivedInsightFeedbackActionTarget(value string) (string, string, error) {
	feedbackID, actionName, ok := strings.Cut(value, ":")
	if !ok || strings.TrimSpace(feedbackID) == "" || strings.TrimSpace(actionName) == "" {
		return "", "", fmt.Errorf("invalid derived insight feedback action target")
	}

	return strings.TrimSpace(feedbackID), strings.TrimSpace(actionName), nil
}

func parseGovernanceRawEventActionTarget(value string) (string, governance.GovernanceRecoveryAction, error) {
	rawEventID, actionName, ok := strings.Cut(value, ":")
	if !ok || strings.TrimSpace(rawEventID) == "" {
		return "", "", fmt.Errorf("invalid governance raw event action target")
	}

	action := governance.GovernanceRecoveryAction(actionName)
	if !action.Valid() {
		return "", "", fmt.Errorf("invalid governance raw event action target")
	}

	return rawEventID, action, nil
}

func parseEmbeddingRebuildActionTarget(value string) (string, memory.EmbeddingRecoveryAction, error) {
	memoryID, actionName, ok := strings.Cut(value, ":")
	if !ok || strings.TrimSpace(memoryID) == "" {
		return "", "", fmt.Errorf("invalid embedding rebuild action target")
	}

	action := memory.EmbeddingRecoveryAction(actionName)
	if !action.Valid() {
		return "", "", fmt.Errorf("invalid embedding rebuild action target")
	}

	return memoryID, action, nil
}

func parseEmbeddingCutoverActionTarget(value string) (string, memory.EmbeddingCutoverPlanAction, error) {
	planID, actionName, ok := strings.Cut(value, ":")
	if !ok || strings.TrimSpace(planID) == "" {
		return "", "", fmt.Errorf("invalid embedding cutover action target")
	}

	action := memory.EmbeddingCutoverPlanAction(actionName)
	if !action.Valid() {
		return "", "", fmt.Errorf("invalid embedding cutover action target")
	}

	return planID, action, nil
}

func parseEmbeddingCutoverPreflightTarget(value string) (string, bool) {
	planID, actionName, ok := strings.Cut(value, ":")
	if !ok || strings.TrimSpace(planID) == "" || strings.TrimSpace(actionName) != "preflight" {
		return "", false
	}
	return strings.TrimSpace(planID), true
}
