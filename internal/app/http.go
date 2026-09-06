package app

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"math/rand"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/FelixSeptem/stele/internal/assurance"
	"github.com/FelixSeptem/stele/internal/auth"
	"github.com/FelixSeptem/stele/internal/governance"
	"github.com/FelixSeptem/stele/internal/jobs"
	"github.com/FelixSeptem/stele/internal/memory"
	"github.com/FelixSeptem/stele/internal/policy"
	"github.com/FelixSeptem/stele/internal/retrieval"
	"github.com/FelixSeptem/stele/internal/telemetry"
	"github.com/FelixSeptem/stele/internal/workflow"
	"github.com/FelixSeptem/stele/openapi"
	"github.com/jackc/pgx/v5"
)

type ReadinessChecker interface {
	Ready(ctx context.Context) error
}

type HTTPDependencies struct {
	HTTP                      HTTPRuntimeLimits
	Contract                  RuntimeContract
	Readiness                 ReadinessChecker
	APIKeys                   auth.StaticAPIKeys
	AdminAPIKeys              auth.StaticAPIKeys
	PrincipalAuthorizer       auth.PrincipalAuthorizer
	PrincipalAdmin            PrincipalAdministrationService
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
	QualityAdmin              QualityAdminService
	ScopeProofAdmin           ScopeProofAdminService
	MemorySession             MemorySessionService
	UsefulnessFeedback        UsefulnessFeedbackService
	TaskEvaluations           TaskEvaluationService
	RankingRollout            RankingRolloutAdminService
	AssuranceAdmin            AssuranceAdminService
	ContextProjectionAdmin    ContextProjectionAdminService
	Workflow                  WorkflowService
	MemoryHistoryRead         MemoryHistoryReader
	JobExecutionRead          JobExecutionReader
	Metrics                   MetricsRecorder
	Logger                    *log.Logger
}

type ContextProjectionAdminService interface {
	RebuildContextProjection(context.Context, memory.ContextProjectionRebuildRequest) (memory.ContextProjection, error)
}

type RuntimeContract struct {
	ServiceVersion string
	BuildID        string
	BuildTimestamp string
	SchemaVersion  int64
}

func (c *RuntimeContract) Normalize() {
	if c.ServiceVersion == "" {
		c.ServiceVersion = "unknown"
	}
	if c.BuildID == "" {
		c.BuildID = "unknown"
	}
	if c.BuildTimestamp == "" {
		c.BuildTimestamp = "unknown"
	}
	if c.SchemaVersion <= 0 {
		c.SchemaVersion = 1
	}
}

type HTTPRuntimeLimits struct {
	MaxRequestBodyBytes int64
	MaxHeaderBytes      int
	ReadHeaderTimeout   time.Duration
	ReadTimeout         time.Duration
	WriteTimeout        time.Duration
	IdleTimeout         time.Duration
}

type requestIDContextKey struct{}

type GovernanceStatus = jobs.GovernanceStatus

type GovernanceStatusReader interface {
	ReadGovernanceStatus(ctx context.Context) (GovernanceStatus, error)
}

type MemoryHistoryReader interface {
	ReadMemoryHistory(ctx context.Context, scope memory.Scope, memoryID string) (memory.MemoryHistory, error)
}

type PrincipalAdministrationService interface {
	CreatePrincipal(ctx context.Context, input auth.CreatePrincipalInput) (auth.IssuedPrincipal, error)
	ListPrincipals(ctx context.Context, scope memory.Scope, limit int) ([]auth.Principal, error)
	ReadPrincipal(ctx context.Context, scope memory.Scope, principalID string) (auth.Principal, error)
	ListScopeGrants(ctx context.Context, scope memory.Scope, principalID string) ([]auth.ScopeGrant, error)
	RotateCredential(ctx context.Context, scope memory.Scope, principalID, actor, reason string) (auth.IssuedCredential, error)
	DisablePrincipal(ctx context.Context, scope memory.Scope, principalID, actor, reason string) error
	ExpirePrincipal(ctx context.Context, scope memory.Scope, principalID string, expiresAt time.Time, actor, reason string) error
	CreateScopeGrant(ctx context.Context, scope memory.Scope, principalID string, grantScope memory.Scope, actor, reason string) error
	RevokeScopeGrant(ctx context.Context, scope memory.Scope, grantID, actor, reason string) error
	ListAccessAudit(ctx context.Context, scope memory.Scope, principalID string, limit int) ([]auth.AuditRecord, error)
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

type QualityAdminService interface {
	CreateEvaluation(ctx context.Context, input memory.CreateQualityEvaluationInput) (memory.QualityEvaluationRun, error)
	ReadEvaluation(ctx context.Context, input memory.ReadQualityEvaluationRunInput) (memory.QualityEvaluationRun, error)
	ListEvaluationFindings(ctx context.Context, input memory.ListQualityEvaluationFindingsInput) ([]memory.QualityEvaluationFinding, error)
	CreateRepairPlan(ctx context.Context, input memory.CreateRepairPlanInput) (memory.RepairPlan, error)
	ReadRepairPlan(ctx context.Context, input memory.ReadRepairPlanInput) (memory.RepairPlan, error)
	ApproveRepairPlan(ctx context.Context, input memory.ApproveRepairPlanInput) (memory.RepairPlan, error)
	VerifyRepairPlan(ctx context.Context, input memory.VerifyRepairPlanInput) (memory.RepairPlan, error)
	ReadDiagnostics(ctx context.Context, input memory.ReadQualityDiagnosticsInput) (memory.QualityDiagnostics, error)
}

type ScopeProofAdminService interface {
	CreateProofRun(ctx context.Context, input memory.CreateScopeProofRunInput) (memory.ScopeProofRun, error)
	ListProofRuns(ctx context.Context, input memory.ListScopeProofRunsInput) ([]memory.ScopeProofRun, error)
	ReadProofRun(ctx context.Context, input memory.ReadScopeProofRunInput) (memory.ScopeProofRun, error)
	ReadProofReport(ctx context.Context, input memory.ReadScopeProofRunInput) (memory.ScopeProofReport, error)
	RerunProofRun(ctx context.Context, input memory.RerunScopeProofRunInput) (memory.ScopeProofRun, error)
}

type MemorySessionService interface {
	CreateSession(ctx context.Context, input memory.CreateMemorySessionInput) (memory.MemorySessionRun, error)
	ListSessions(ctx context.Context, input memory.ListMemorySessionRunsInput) ([]memory.MemorySessionRun, error)
	ReadSession(ctx context.Context, input memory.ReadMemorySessionRunInput) (memory.MemorySessionRun, error)
	CreateTurn(ctx context.Context, input memory.CreateMemorySessionTurnInput) (memory.MemorySessionTurn, error)
	RecordTurnOutcome(ctx context.Context, input memory.RecordMemorySessionTurnOutcomeInput) (memory.MemorySessionTurn, error)
	RequestVerification(ctx context.Context, input memory.RequestMemorySessionVerificationInput) (memory.MemorySessionVerification, error)
	ReadSessionReport(ctx context.Context, input memory.ReadMemorySessionRunInput) (memory.MemorySessionReport, error)
}

type UsefulnessFeedbackService interface {
	CreateUsefulnessFeedback(ctx context.Context, input memory.UsefulnessFeedback) (memory.UsefulnessFeedback, error)
	ListUsefulnessFeedback(ctx context.Context, input memory.ListUsefulnessFeedbackInput) ([]memory.UsefulnessFeedback, error)
	ReadUsefulnessFeedback(ctx context.Context, input memory.ReadUsefulnessFeedbackInput) (memory.UsefulnessFeedback, error)
	SummarizeUsefulnessFeedback(ctx context.Context, input memory.SummarizeUsefulnessFeedbackInput) (memory.UsefulnessFeedbackSummary, error)
	SupersedeUsefulnessFeedback(ctx context.Context, input memory.SupersedeUsefulnessFeedbackInput) error
}

type TaskEvaluationService interface {
	CreateTaskEvaluation(ctx context.Context, input memory.TaskEvaluation) (memory.TaskEvaluation, error)
	ReadTaskEvaluation(ctx context.Context, input memory.ReadTaskEvaluationInput) (memory.TaskEvaluation, error)
	ListTaskEvaluations(ctx context.Context, input memory.ListTaskEvaluationsInput) ([]memory.TaskEvaluation, error)
	SupersedeTaskEvaluation(ctx context.Context, input memory.SupersedeTaskEvaluationInput) error
	SummarizeTaskEvaluations(ctx context.Context, input memory.SummarizeTaskEvaluationsInput) (memory.TaskEvaluationSummary, error)
}

type WorkflowService interface {
	CreateTemplate(ctx context.Context, input workflow.CreateTemplateInput) (workflow.WorkflowTemplate, error)
	UpdateTemplate(ctx context.Context, input workflow.UpdateTemplateInput) (workflow.WorkflowTemplate, error)
	DisableTemplate(ctx context.Context, input workflow.DisableTemplateInput) (workflow.WorkflowTemplate, error)
	ReadTemplate(ctx context.Context, input workflow.ReadTemplateInput) (workflow.WorkflowTemplate, error)
	ListTemplates(ctx context.Context, input workflow.ListTemplatesInput) ([]workflow.WorkflowTemplate, error)
	StartRun(ctx context.Context, input workflow.StartRunInput) (workflow.WorkflowRun, error)
	ReadRun(ctx context.Context, input workflow.ReadRunInput) (workflow.WorkflowRun, error)
	ListRuns(ctx context.Context, input workflow.ListRunsInput) ([]workflow.WorkflowRun, error)
	RecordStep(ctx context.Context, input workflow.RecordStepInput) (workflow.WorkflowStepRecord, error)
	ListStepRecords(ctx context.Context, input workflow.ListStepRecordsInput) ([]workflow.WorkflowStepRecord, error)
	ListEvidenceLinks(ctx context.Context, input workflow.ListEvidenceLinksInput) ([]workflow.EvidenceLink, error)
	ListDiagnostics(ctx context.Context, input workflow.ListDiagnosticsInput) ([]workflow.GapDiagnostic, error)
	ListNextActions(ctx context.Context, input workflow.ListNextActionsInput) ([]workflow.NextAction, error)
	SupersedeEvidenceLink(ctx context.Context, input workflow.SupersedeEvidenceLinkInput) error
}

type RankingRolloutAdminService interface {
	CreateRankingRolloutPolicy(ctx context.Context, policy memory.RankingRolloutPolicy) (memory.RankingRolloutPolicy, error)
	ReadRankingRolloutPolicy(ctx context.Context, input memory.ReadRankingRolloutPolicyInput) (memory.RankingRolloutPolicy, error)
	ListRankingRolloutPolicies(ctx context.Context, input memory.ListRankingRolloutPoliciesInput) ([]memory.RankingRolloutPolicy, error)
	RecordRankingRolloutDryRun(ctx context.Context, input memory.RecordRankingRolloutDryRunInput) (memory.RankingRolloutDryRun, error)
	ActivateRankingRolloutPolicy(ctx context.Context, input memory.ActivateRankingRolloutPolicyInput) (memory.RankingRolloutPolicy, error)
	DisableRankingRolloutPolicy(ctx context.Context, input memory.DisableRankingRolloutPolicyInput) (memory.RankingRolloutPolicy, error)
	RollbackRankingRolloutPolicy(ctx context.Context, input memory.RollbackRankingRolloutPolicyInput) (memory.RankingRolloutPolicy, error)
	ListRankingRolloutPolicyImpact(ctx context.Context, input memory.ListRankingRolloutPolicyImpactInput) ([]memory.RankingRolloutImpactEntry, error)
}

type AssuranceAdminService interface {
	CreateHealthEvaluation(ctx context.Context, input assurance.HealthEvaluationInput) (assurance.HealthEvaluation, error)
	ListHealthEvaluations(ctx context.Context, scope memory.Scope) ([]assurance.HealthEvaluation, error)
	ReadHealthEvaluation(ctx context.Context, input assurance.ReadHealthEvaluationInput) (assurance.HealthEvaluation, error)
	ListIncidents(ctx context.Context, input assurance.ListIncidentsInput) ([]assurance.Incident, error)
	ReadIncident(ctx context.Context, input assurance.ReadIncidentInput) (assurance.Incident, error)
	ApplyIncidentAction(ctx context.Context, input assurance.IncidentActionInput) (assurance.Incident, error)
	ListAlertCandidates(ctx context.Context, scope memory.Scope) ([]assurance.AlertCandidate, error)
	ReadAlertCandidate(ctx context.Context, input assurance.ReadAlertCandidateInput) (assurance.AlertCandidate, error)
	ListAlertDeliveryAttempts(ctx context.Context, input assurance.ListAlertDeliveryAttemptsInput) ([]assurance.AlertDeliveryAttempt, error)
	CreateConformanceProfile(ctx context.Context, profile assurance.ConformanceProfile) (assurance.ConformanceProfile, error)
	ListConformanceProfiles(ctx context.Context, input assurance.ListConformanceProfilesInput) ([]assurance.ConformanceProfile, error)
	ReadConformanceProfile(ctx context.Context, input assurance.ReadConformanceProfileInput) (assurance.ConformanceProfile, error)
	UpdateConformanceProfile(ctx context.Context, input assurance.UpdateConformanceProfileInput) (assurance.ConformanceProfile, error)
	DisableConformanceProfile(ctx context.Context, input assurance.DisableConformanceProfileInput) (assurance.ConformanceProfile, error)
	RunConformance(ctx context.Context, input assurance.ConformanceRunInput) (assurance.ConformanceRun, []assurance.MissingEvidenceDiagnostic, error)
	ListConformanceRuns(ctx context.Context, input assurance.ListConformanceRunsInput) ([]assurance.ConformanceRun, error)
	ReadConformanceRun(ctx context.Context, input assurance.ReadConformanceRunInput) (assurance.ConformanceRun, error)
	CreateReadinessReport(ctx context.Context, input assurance.ReadinessReportInput) (assurance.ReadinessReport, error)
	ListReadinessReports(ctx context.Context, scope memory.Scope) ([]assurance.ReadinessReport, error)
	ReadReadinessReport(ctx context.Context, input assurance.ReadReadinessReportInput) (assurance.ReadinessReport, error)
	CreateRecoveryVerification(ctx context.Context, input assurance.RecoveryVerificationInput) (assurance.RecoveryVerification, error)
	ListRecoveryVerifications(ctx context.Context, scope memory.Scope) ([]assurance.RecoveryVerification, error)
	ReadRecoveryVerification(ctx context.Context, input assurance.ReadRecoveryVerificationInput) (assurance.RecoveryVerification, error)
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
	RecordUsefulnessFeedback(ctx context.Context, event telemetry.UsefulnessFeedbackEvent)
	RecordTaskEvaluation(ctx context.Context, event telemetry.TaskEvaluationEvent)
	RecordRankingRollout(ctx context.Context, event telemetry.RankingRolloutEvent)
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

type qualityEvaluationCreateRequest struct {
	Checks            []memory.QualityEvaluationCheck `json:"checks"`
	Query             string                          `json:"query"`
	ExpectedMemoryIDs []string                        `json:"expected_memory_ids"`
	ContextBudget     int                             `json:"context_budget"`
	Actor             string                          `json:"actor"`
	Reason            string                          `json:"reason"`
}

type scopeProofCreateRequest struct {
	Checks      []memory.ScopeProofCheck     `json:"checks"`
	FixtureMode memory.ScopeProofFixtureMode `json:"fixture_mode"`
	Actor       string                       `json:"actor"`
	Reason      string                       `json:"reason"`
}

type scopeProofRerunRequest struct {
	Actor  string `json:"actor"`
	Reason string `json:"reason"`
}

type memorySessionCreateRequest struct {
	Actor    string         `json:"actor"`
	Reason   string         `json:"reason"`
	Metadata map[string]any `json:"metadata,omitempty"`
}

type memorySessionTurnCreateRequest struct {
	IdempotencyKey            string `json:"idempotency_key"`
	Query                     string `json:"query"`
	ContextBudget             int    `json:"context_budget"`
	IncludeRelations          bool   `json:"include_relations"`
	IncludeExperienceInsights bool   `json:"include_experience_insights"`
	IncludeDiagnostics        bool   `json:"include_diagnostics"`
}

type memorySessionTurnOutcomeRequest struct {
	IdempotencyKey       string                                    `json:"idempotency_key"`
	OutcomeEventIDs      []string                                  `json:"outcome_event_ids"`
	OutcomeEventPayloads []memory.MemorySessionOutcomeEventPayload `json:"event_payloads"`
	ExpectedRecall       []string                                  `json:"expected_recall"`
}

type usefulnessFeedbackCreateRequest struct {
	Type             memory.UsefulnessFeedbackType          `json:"type"`
	SourceSurface    memory.UsefulnessFeedbackSourceSurface `json:"source_surface"`
	TaskEvaluationID string                                 `json:"task_evaluation_id,omitempty"`
	Subjects         []memory.UsefulnessFeedbackSubject     `json:"subjects"`
	Actor            string                                 `json:"actor"`
	Reason           string                                 `json:"reason"`
	IdempotencyKey   string                                 `json:"idempotency_key"`
	Metadata         map[string]any                         `json:"metadata,omitempty"`
}

type usefulnessFeedbackSupersedeRequest struct {
	Actor  string `json:"actor"`
	Reason string `json:"reason"`
}

type taskEvaluationCreateRequest struct {
	Objective              string                            `json:"objective"`
	SuccessCriteria        []string                          `json:"success_criteria"`
	Verdict                memory.TaskEvaluationVerdict      `json:"verdict"`
	ContributionCategories []memory.TaskContributionCategory `json:"contribution_categories,omitempty"`
	Evidence               []memory.TaskEvidenceLink         `json:"evidence"`
	Actor                  string                            `json:"actor"`
	Reason                 string                            `json:"reason"`
	IdempotencyKey         string                            `json:"idempotency_key,omitempty"`
	Metadata               map[string]any                    `json:"metadata,omitempty"`
}

type taskEvaluationSupersedeRequest struct {
	Actor  string `json:"actor"`
	Reason string `json:"reason"`
}

type workflowTemplateStepRequest struct {
	Kind             workflow.StepKind        `json:"kind"`
	Requirement      workflow.StepRequirement `json:"requirement"`
	AllowedEvidence  []workflow.EvidenceKind  `json:"allowed_evidence"`
	MinimumCount     int                      `json:"minimum_count"`
	RequiresInternal bool                     `json:"requires_internal"`
	FreshnessWindow  string                   `json:"freshness_window"`
	CompletionWindow string                   `json:"completion_window"`
	Position         int                      `json:"position"`
	Metadata         map[string]any           `json:"metadata,omitempty"`
}

type workflowTemplateCreateRequest struct {
	Steps            []workflowTemplateStepRequest `json:"steps"`
	IntegrationKind  workflow.IntegrationKind      `json:"integration_kind"`
	CompletionPolicy workflow.CompletionPolicy     `json:"completion_policy"`
	Actor            string                        `json:"actor"`
	Reason           string                        `json:"reason"`
	Metadata         map[string]any                `json:"metadata,omitempty"`
}

type workflowTemplateUpdateRequest struct {
	Steps            []workflowTemplateStepRequest `json:"steps"`
	IntegrationKind  workflow.IntegrationKind      `json:"integration_kind"`
	CompletionPolicy workflow.CompletionPolicy     `json:"completion_policy"`
	Actor            string                        `json:"actor"`
	Reason           string                        `json:"reason"`
	Metadata         map[string]any                `json:"metadata,omitempty"`
}

type workflowActorReasonRequest struct {
	Actor  string `json:"actor"`
	Reason string `json:"reason"`
}

type workflowRunCreateRequest struct {
	TemplateID     string         `json:"template_id"`
	IdempotencyKey string         `json:"idempotency_key"`
	Actor          string         `json:"actor"`
	Reason         string         `json:"reason"`
	Metadata       map[string]any `json:"metadata,omitempty"`
	ExpiresAt      string         `json:"expires_at,omitempty"`
}

type workflowStepRecordRequest struct {
	Kind          workflow.StepKind       `json:"kind"`
	Actor         string                  `json:"actor"`
	Reason        string                  `json:"reason"`
	ObservedAt    string                  `json:"observed_at,omitempty"`
	Metadata      map[string]any          `json:"metadata,omitempty"`
	EvidenceLinks []workflow.EvidenceLink `json:"evidence_links,omitempty"`
}

type publicWorkflowNextAction struct {
	Category      workflow.NextActionCategory `json:"category"`
	StepKind      workflow.StepKind           `json:"step_kind"`
	EvidenceKind  workflow.EvidenceKind       `json:"evidence_kind"`
	RouteCategory workflow.RouteCategory      `json:"route_category"`
	Status        workflow.NextActionStatus   `json:"status"`
}

// Public workflow responses deliberately exclude scope, attribution, metadata, and source ids.
type publicWorkflowRun struct {
	ID              string                   `json:"id,omitempty"`
	Status          workflow.RunStatus       `json:"status"`
	IntegrationKind workflow.IntegrationKind `json:"integration_kind"`
	CreatedAt       time.Time                `json:"created_at"`
	UpdatedAt       time.Time                `json:"updated_at"`
	StartedAt       time.Time                `json:"started_at"`
	CompletedAt     time.Time                `json:"completed_at,omitempty"`
	ExpiresAt       time.Time                `json:"expires_at,omitempty"`
}

type publicWorkflowStepRecord struct {
	Kind       workflow.StepKind   `json:"kind"`
	Status     workflow.StepStatus `json:"status"`
	Result     workflow.StepResult `json:"result"`
	ObservedAt time.Time           `json:"observed_at"`
	CreatedAt  time.Time           `json:"created_at"`
}

type rankingRolloutPolicyCreateRequest struct {
	ID              string                               `json:"id"`
	Status          memory.RankingRolloutPolicyStatus    `json:"status"`
	Mode            memory.RankingRolloutMode            `json:"mode"`
	Surfaces        []memory.RankingRolloutSurface       `json:"surfaces"`
	SignalSources   []memory.RankingRolloutSignalSource  `json:"signal_sources"`
	ThresholdStatus memory.RankingRolloutThresholdStatus `json:"threshold_status"`
	EvidenceMinimum int                                  `json:"evidence_minimum"`
	Actor           string                               `json:"actor"`
	Reason          string                               `json:"reason"`
}

type rankingRolloutPolicyActionRequest struct {
	Actor  string `json:"actor"`
	Reason string `json:"reason"`
}

type assuranceHealthEvaluationCreateRequest struct {
	EvaluationID          string                      `json:"evaluation_id,omitempty"`
	ObservedAt            string                      `json:"observed_at,omitempty"`
	RuntimeReadiness      assurance.HealthObservation `json:"runtime_readiness,omitempty"`
	BacklogState          assurance.HealthObservation `json:"backlog_state,omitempty"`
	EmbeddingHealth       assurance.HealthObservation `json:"embedding_health,omitempty"`
	ProofSessionVerdict   assurance.HealthObservation `json:"proof_session_verdict,omitempty"`
	UsefulnessFeedback    assurance.HealthObservation `json:"usefulness_feedback,omitempty"`
	TaskEvaluationSummary assurance.HealthObservation `json:"task_evaluation_summary,omitempty"`
	RepairStatus          assurance.HealthObservation `json:"repair_status,omitempty"`
	RankingRolloutState   assurance.HealthObservation `json:"ranking_rollout_state,omitempty"`
	ConformanceStatus     assurance.HealthObservation `json:"conformance_status,omitempty"`
	WorkflowHealth        assurance.HealthObservation `json:"workflow_health,omitempty"`
	CapacityLoadProof     assurance.HealthObservation `json:"capacity_load_proof,omitempty"`
	BackupRestoreProof    assurance.HealthObservation `json:"backup_restore_proof,omitempty"`
}

type assuranceIncidentActionRequest struct {
	Actor  string `json:"actor"`
	Reason string `json:"reason"`
}

type assuranceConformanceProfileRequest struct {
	ID               string                         `json:"id,omitempty"`
	ExpectedEvidence []assuranceExpectedEvidenceDTO `json:"expected_evidence"`
	Actor            string                         `json:"actor"`
	Reason           string                         `json:"reason"`
}

type assuranceExpectedEvidenceDTO struct {
	Kind            assurance.ExpectedEvidenceKind `json:"kind"`
	MinimumCount    int                            `json:"minimum_count"`
	FreshnessWindow string                         `json:"freshness_window"`
}

type assuranceConformanceRunCreateRequest struct {
	ProfileID string `json:"profile_id"`
	RunID     string `json:"run_id,omitempty"`
	StartedAt string `json:"started_at,omitempty"`
}

type assuranceReadinessReportCreateRequest struct {
	ReportID    string `json:"report_id,omitempty"`
	GeneratedAt string `json:"generated_at,omitempty"`
}

type assuranceRecoveryVerificationCreateRequest struct {
	RecordID        string                               `json:"record_id,omitempty"`
	Target          assurance.RecoveryVerificationTarget `json:"target"`
	TargetID        string                               `json:"target_id"`
	Status          assurance.HealthStatus               `json:"status"`
	CheckedSurfaces []string                             `json:"checked_surfaces,omitempty"`
	ResultCategory  string                               `json:"result_category"`
	LinkedEvidence  map[string]any                       `json:"linked_evidence,omitempty"`
	Actor           string                               `json:"actor"`
	Reason          string                               `json:"reason"`
	VerifiedAt      string                               `json:"verified_at,omitempty"`
}

type memorySessionVerificationRequest struct {
	TurnID         string   `json:"turn_id"`
	ExpectedRecall []string `json:"expected_recall"`
}

type repairPlanCreateRequest struct {
	EvaluationRunID string `json:"evaluation_run_id"`
	Actor           string `json:"actor"`
	Reason          string `json:"reason"`
	DryRun          bool   `json:"dry_run"`
}

type repairPlanApproveRequest struct {
	Actor  string `json:"actor"`
	Reason string `json:"reason"`
}

type repairPlanVerifyRequest struct {
	Checks []memory.QualityEvaluationCheck `json:"checks"`
	Actor  string                          `json:"actor"`
	Reason string                          `json:"reason"`
}

type governanceRecoveryRequest struct {
	Reason       string `json:"reason"`
	ScheduledFor string `json:"scheduled_for"`
}

type contextProjectionRebuildRequest struct {
	Kind            memory.ContextProjectionKind `json:"kind"`
	Limit           int                          `json:"limit"`
	SchemaVersion   string                       `json:"schema_version"`
	PolicyVersion   string                       `json:"policy_version"`
	RendererVersion string                       `json:"renderer_version"`
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
	EventID   string                          `json:"event_id"`
	Admission *memory.AdmissionPressureReport `json:"admission,omitempty"`
	Replayed  bool                            `json:"replayed"`
}

type principalCreateRequest struct {
	Role   auth.PrincipalRole `json:"role"`
	Label  string             `json:"label"`
	Actor  string             `json:"actor"`
	Reason string             `json:"reason"`
}

type principalGrantRequest struct {
	Tenant    string `json:"tenant"`
	Project   string `json:"project"`
	Namespace string `json:"namespace"`
	Actor     string `json:"actor"`
	Reason    string `json:"reason"`
}

type principalLifecycleRequest struct {
	Actor  string `json:"actor"`
	Reason string `json:"reason"`
}

type memorySearchRequest struct {
	Query                      string               `json:"query"`
	QueryEmbedding             []float32            `json:"query_embedding"`
	Classes                    []memory.MemoryClass `json:"classes"`
	TimeFrom                   string               `json:"time_from"`
	TimeTo                     string               `json:"time_to"`
	TopK                       int                  `json:"top_k"`
	IncludeSummaries           bool                 `json:"include_summaries"`
	IncludeRelations           bool                 `json:"include_relations"`
	IncludeFeedbackDiagnostics bool                 `json:"include_feedback_diagnostics"`
	FeedbackAwareRanking       bool                 `json:"feedback_aware_ranking"`
	FeedbackRankingPolicy      string               `json:"feedback_ranking_policy"`
}

type contextAssembleRequest struct {
	Query                      string `json:"query"`
	Budget                     int    `json:"budget"`
	IncludeRelations           bool   `json:"include_relations"`
	IncludeExperienceInsights  bool   `json:"include_experience_insights"`
	IncludeDiagnostics         bool   `json:"include_diagnostics"`
	IncludeFeedbackDiagnostics bool   `json:"include_feedback_diagnostics"`
	FeedbackAwareRanking       bool   `json:"feedback_aware_ranking"`
	FeedbackRankingPolicy      string `json:"feedback_ranking_policy"`
}

func NewHTTPHandler(deps HTTPDependencies) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /openapi.yaml", func(w http.ResponseWriter, r *http.Request) {
		body := []byte(openapi.SpecYAML())
		etag := fmt.Sprintf(`"%x"`, sha256.Sum256(body))
		w.Header().Set("ETag", etag)
		if r.Header.Get("If-None-Match") == etag {
			w.WriteHeader(http.StatusNotModified)
			return
		}
		w.Header().Set("Content-Type", "application/yaml; charset=utf-8")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(body)
	})
	mux.HandleFunc("GET /version", func(w http.ResponseWriter, r *http.Request) {
		body := []byte(openapi.SpecYAML())
		contract := deps.Contract
		contract.Normalize()
		writeJSON(w, http.StatusOK, map[string]any{
			"service_version": contract.ServiceVersion,
			"build_id":        contract.BuildID,
			"build_timestamp": contract.BuildTimestamp,
			"openapi_digest":  fmt.Sprintf("sha256:%x", sha256.Sum256(body)),
			"schema_version":  contract.SchemaVersion,
			"migration_compatibility": map[string]any{
				"minimum": 1,
				"maximum": contract.SchemaVersion,
			},
		})
	})
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

	protectedMemorySessions := auth.APIKeyMiddleware(deps.APIKeys)(
		auth.ScopeMiddleware()(
			http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				handleMemorySessions(w, r, deps.MemorySession)
			}),
		),
	)
	mux.Handle("GET /v1/memory-sessions", protectedMemorySessions)
	mux.Handle("POST /v1/memory-sessions", protectedMemorySessions)

	protectedMemorySessionDetail := auth.APIKeyMiddleware(deps.APIKeys)(
		auth.ScopeMiddleware()(
			http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				handleMemorySessionDetail(w, r, deps.MemorySession)
			}),
		),
	)
	mux.Handle("GET /v1/memory-sessions/{session_id}", protectedMemorySessionDetail)

	protectedMemorySessionReport := auth.APIKeyMiddleware(deps.APIKeys)(
		auth.ScopeMiddleware()(
			http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				handleMemorySessionReport(w, r, deps.MemorySession)
			}),
		),
	)
	mux.Handle("GET /v1/memory-sessions/{session_id}/report", protectedMemorySessionReport)

	protectedMemorySessionTurns := auth.APIKeyMiddleware(deps.APIKeys)(
		auth.ScopeMiddleware()(
			http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				handleMemorySessionTurns(w, r, deps.MemorySession)
			}),
		),
	)
	mux.Handle("POST /v1/memory-sessions/{session_id}/turns", protectedMemorySessionTurns)

	protectedMemorySessionTurnAction := auth.APIKeyMiddleware(deps.APIKeys)(
		auth.ScopeMiddleware()(
			http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				handleMemorySessionTurnAction(w, r, deps.MemorySession)
			}),
		),
	)
	mux.Handle("POST /v1/memory-sessions/{session_id}/turns/{turn_action}", protectedMemorySessionTurnAction)

	protectedMemorySessionAction := auth.APIKeyMiddleware(deps.APIKeys)(
		auth.ScopeMiddleware()(
			http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				handleMemorySessionAction(w, r, deps.MemorySession)
			}),
		),
	)
	mux.Handle("POST /v1/memory-sessions/{session_action}", protectedMemorySessionAction)

	protectedUsefulnessFeedback := auth.APIKeyMiddleware(deps.APIKeys)(
		auth.ScopeMiddleware()(
			http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				handleUsefulnessFeedbackCreate(w, r, deps.UsefulnessFeedback, deps.Metrics, deps.Logger)
			}),
		),
	)
	mux.Handle("POST /v1/usefulness-feedback", protectedUsefulnessFeedback)

	protectedTaskEvaluations := auth.APIKeyMiddleware(deps.APIKeys)(
		auth.ScopeMiddleware()(
			http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				handleTaskEvaluationCreate(w, r, deps.TaskEvaluations, deps.Metrics, deps.Logger)
			}),
		),
	)
	mux.Handle("POST /v1/task-evaluations", protectedTaskEvaluations)

	protectedTaskEvaluationReport := auth.APIKeyMiddleware(deps.APIKeys)(
		auth.ScopeMiddleware()(
			http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				handleTaskEvaluationReport(w, r, deps.TaskEvaluations, deps.Metrics, deps.Logger)
			}),
		),
	)
	mux.Handle("GET /v1/task-evaluations/{evaluation_id}/report", protectedTaskEvaluationReport)

	protectedWorkflows := auth.APIKeyMiddleware(deps.APIKeys)(
		auth.ScopeMiddleware()(
			http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				handleWorkflows(w, r, deps.Workflow)
			}),
		),
	)
	mux.Handle("POST /v1/workflows/runs", protectedWorkflows)
	mux.Handle("GET /v1/workflows/runs/{workflow_run_id}", protectedWorkflows)
	mux.Handle("POST /v1/workflows/runs/{workflow_run_id}/steps", protectedWorkflows)
	mux.Handle("GET /v1/workflows/runs/{workflow_run_id}/next-actions", protectedWorkflows)

	adminPrincipals := auth.APIKeyMiddleware(deps.AdminAPIKeys)(
		auth.ScopeMiddleware()(
			http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				handleAdminPrincipals(w, r, deps.PrincipalAdmin)
			}),
		),
	)
	mux.Handle("POST /v1/admin/principals", adminPrincipals)
	mux.Handle("GET /v1/admin/principals", adminPrincipals)
	mux.Handle("GET /v1/admin/principals/{principal_id}", adminPrincipals)
	mux.Handle("POST /v1/admin/principals/{principal_id}/credentials/rotate", adminPrincipals)
	mux.Handle("POST /v1/admin/principals/{principal_id}/disable", adminPrincipals)
	mux.Handle("POST /v1/admin/principals/{principal_id}/expire", adminPrincipals)
	mux.Handle("GET /v1/admin/principals/{principal_id}/grants", adminPrincipals)
	mux.Handle("POST /v1/admin/principals/{principal_id}/grants", adminPrincipals)
	mux.Handle("POST /v1/admin/principals/{principal_id}/grants/{grant_id}/revoke", adminPrincipals)

	adminAccessAudit := auth.APIKeyMiddleware(deps.AdminAPIKeys)(
		auth.ScopeMiddleware()(
			http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				handleAdminAccessAudit(w, r, deps.PrincipalAdmin)
			}),
		))
	mux.Handle("GET /v1/admin/access-audit", adminAccessAudit)

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

	adminUsefulnessFeedbackList := auth.APIKeyMiddleware(deps.AdminAPIKeys)(
		auth.ScopeMiddleware()(
			http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				handleAdminUsefulnessFeedbackList(w, r, deps.UsefulnessFeedback)
			}),
		),
	)
	mux.Handle("GET /v1/admin/usefulness-feedback", adminUsefulnessFeedbackList)

	adminUsefulnessFeedbackSummary := auth.APIKeyMiddleware(deps.AdminAPIKeys)(
		auth.ScopeMiddleware()(
			http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				handleAdminUsefulnessFeedbackSummary(w, r, deps.UsefulnessFeedback, deps.Metrics, deps.Logger)
			}),
		),
	)
	mux.Handle("GET /v1/admin/usefulness-feedback/summary", adminUsefulnessFeedbackSummary)

	adminUsefulnessFeedbackDetail := auth.APIKeyMiddleware(deps.AdminAPIKeys)(
		auth.ScopeMiddleware()(
			http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				handleAdminUsefulnessFeedbackDetail(w, r, deps.UsefulnessFeedback)
			}),
		),
	)
	mux.Handle("GET /v1/admin/usefulness-feedback/{feedback_id}", adminUsefulnessFeedbackDetail)

	adminUsefulnessFeedbackAction := auth.APIKeyMiddleware(deps.AdminAPIKeys)(
		auth.ScopeMiddleware()(
			http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				handleAdminUsefulnessFeedbackAction(w, r, deps.UsefulnessFeedback, deps.Metrics, deps.Logger)
			}),
		),
	)
	mux.Handle("POST /v1/admin/usefulness-feedback/{feedback_action}", adminUsefulnessFeedbackAction)

	adminTaskEvaluations := auth.APIKeyMiddleware(deps.AdminAPIKeys)(
		auth.ScopeMiddleware()(
			http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				handleAdminTaskEvaluations(w, r, deps.TaskEvaluations, deps.Metrics, deps.Logger)
			}),
		),
	)
	mux.Handle("GET /v1/admin/task-evaluations", adminTaskEvaluations)
	mux.Handle("GET /v1/admin/task-evaluations/{evaluation_id}", adminTaskEvaluations)
	mux.Handle("GET /v1/admin/task-evaluations/summary", adminTaskEvaluations)
	mux.Handle("POST /v1/admin/task-evaluations/{evaluation_id}/supersede", adminTaskEvaluations)

	adminWorkflows := auth.APIKeyMiddleware(deps.AdminAPIKeys)(
		auth.ScopeMiddleware()(
			http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				handleAdminWorkflows(w, r, deps.Workflow)
			}),
		),
	)
	mux.Handle("GET /v1/admin/workflows/templates", adminWorkflows)
	mux.Handle("POST /v1/admin/workflows/templates", adminWorkflows)
	mux.Handle("GET /v1/admin/workflows/templates/{workflow_template_id}", adminWorkflows)
	mux.Handle("PATCH /v1/admin/workflows/templates/{workflow_template_id}", adminWorkflows)
	mux.Handle("POST /v1/admin/workflows/templates/{workflow_template_id}/disable", adminWorkflows)
	mux.Handle("GET /v1/admin/workflows/runs", adminWorkflows)
	mux.Handle("GET /v1/admin/workflows/runs/{workflow_run_id}", adminWorkflows)
	mux.Handle("GET /v1/admin/workflows/runs/{workflow_run_id}/steps", adminWorkflows)
	mux.Handle("GET /v1/admin/workflows/runs/{workflow_run_id}/evidence-links", adminWorkflows)
	mux.Handle("GET /v1/admin/workflows/runs/{workflow_run_id}/diagnostics", adminWorkflows)
	mux.Handle("GET /v1/admin/workflows/runs/{workflow_run_id}/next-actions", adminWorkflows)
	mux.Handle("POST /v1/admin/workflows/evidence-links/{evidence_link_id}/supersede", adminWorkflows)

	adminRankingRollouts := auth.APIKeyMiddleware(deps.AdminAPIKeys)(
		auth.ScopeMiddleware()(
			http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				handleAdminRankingRollouts(w, r, deps.RankingRollout, deps.Metrics, deps.Logger)
			}),
		),
	)
	mux.Handle("GET /v1/admin/ranking-rollouts", adminRankingRollouts)
	mux.Handle("POST /v1/admin/ranking-rollouts", adminRankingRollouts)
	mux.Handle("GET /v1/admin/ranking-rollouts/{policy_id}", adminRankingRollouts)
	mux.Handle("GET /v1/admin/ranking-rollouts/{policy_id}/impact", adminRankingRollouts)
	mux.Handle("POST /v1/admin/ranking-rollouts/{policy_id}/dry-run", adminRankingRollouts)
	mux.Handle("POST /v1/admin/ranking-rollouts/{policy_id}/activate", adminRankingRollouts)
	mux.Handle("POST /v1/admin/ranking-rollouts/{policy_id}/disable", adminRankingRollouts)
	mux.Handle("POST /v1/admin/ranking-rollouts/{policy_id}/rollback", adminRankingRollouts)

	adminAssurance := auth.APIKeyMiddleware(deps.AdminAPIKeys)(
		auth.ScopeMiddleware()(
			http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				handleAdminAssurance(w, r, deps.AssuranceAdmin)
			}),
		),
	)
	mux.Handle("GET /v1/admin/assurance/health-evaluations", adminAssurance)
	mux.Handle("POST /v1/admin/assurance/health-evaluations", adminAssurance)
	mux.Handle("GET /v1/admin/assurance/health-evaluations/{health_evaluation_id}", adminAssurance)
	mux.Handle("GET /v1/admin/assurance/incidents", adminAssurance)
	mux.Handle("GET /v1/admin/assurance/incidents/{incident_id}", adminAssurance)
	mux.Handle("POST /v1/admin/assurance/incidents/{incident_id}/{incident_action}", adminAssurance)
	mux.Handle("GET /v1/admin/assurance/alert-candidates", adminAssurance)
	mux.Handle("GET /v1/admin/assurance/alert-candidates/{alert_candidate_id}", adminAssurance)
	mux.Handle("GET /v1/admin/assurance/alert-candidates/{alert_candidate_id}/delivery-attempts", adminAssurance)
	mux.Handle("GET /v1/admin/assurance/conformance-profiles", adminAssurance)
	mux.Handle("POST /v1/admin/assurance/conformance-profiles", adminAssurance)
	mux.Handle("GET /v1/admin/assurance/conformance-profiles/{conformance_profile_id}", adminAssurance)
	mux.Handle("PATCH /v1/admin/assurance/conformance-profiles/{conformance_profile_id}", adminAssurance)
	mux.Handle("POST /v1/admin/assurance/conformance-profiles/{conformance_profile_id}/disable", adminAssurance)
	mux.Handle("GET /v1/admin/assurance/conformance-runs", adminAssurance)
	mux.Handle("POST /v1/admin/assurance/conformance-runs", adminAssurance)
	mux.Handle("GET /v1/admin/assurance/conformance-runs/{conformance_run_id}", adminAssurance)
	mux.Handle("GET /v1/admin/assurance/readiness-reports", adminAssurance)
	mux.Handle("POST /v1/admin/assurance/readiness-reports", adminAssurance)
	mux.Handle("GET /v1/admin/assurance/readiness-reports/{readiness_report_id}", adminAssurance)
	mux.Handle("GET /v1/admin/assurance/recovery-verifications", adminAssurance)
	mux.Handle("POST /v1/admin/assurance/recovery-verifications", adminAssurance)
	mux.Handle("GET /v1/admin/assurance/recovery-verifications/{recovery_verification_id}", adminAssurance)

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

	adminScopeProofs := auth.APIKeyMiddleware(deps.AdminAPIKeys)(
		auth.ScopeMiddleware()(
			http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				handleAdminScopeProofs(w, r, deps.ScopeProofAdmin)
			}),
		),
	)
	mux.Handle("GET /v1/admin/scope-proofs", adminScopeProofs)
	mux.Handle("POST /v1/admin/scope-proofs", adminScopeProofs)

	adminScopeProofDetail := auth.APIKeyMiddleware(deps.AdminAPIKeys)(
		auth.ScopeMiddleware()(
			http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				handleAdminScopeProofDetail(w, r, deps.ScopeProofAdmin)
			}),
		),
	)
	mux.Handle("GET /v1/admin/scope-proofs/{proof_run_id}", adminScopeProofDetail)

	adminScopeProofReport := auth.APIKeyMiddleware(deps.AdminAPIKeys)(
		auth.ScopeMiddleware()(
			http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				handleAdminScopeProofReport(w, r, deps.ScopeProofAdmin)
			}),
		),
	)
	mux.Handle("GET /v1/admin/scope-proofs/{proof_run_id}/report", adminScopeProofReport)

	adminScopeProofAction := auth.APIKeyMiddleware(deps.AdminAPIKeys)(
		auth.ScopeMiddleware()(
			http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				handleAdminScopeProofAction(w, r, deps.ScopeProofAdmin)
			}),
		),
	)
	mux.Handle("POST /v1/admin/scope-proofs/{proof_run_action}", adminScopeProofAction)

	adminQualityEvaluations := auth.APIKeyMiddleware(deps.AdminAPIKeys)(
		auth.ScopeMiddleware()(
			http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				handleAdminQualityEvaluations(w, r, deps.QualityAdmin)
			}),
		),
	)
	mux.Handle("GET /v1/admin/memory-quality/evaluations/{evaluation_run_id}", adminQualityEvaluations)
	mux.Handle("POST /v1/admin/memory-quality/evaluations", adminQualityEvaluations)

	adminQualityEvaluationFindings := auth.APIKeyMiddleware(deps.AdminAPIKeys)(
		auth.ScopeMiddleware()(
			http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				handleAdminQualityEvaluationFindings(w, r, deps.QualityAdmin)
			}),
		),
	)
	mux.Handle("GET /v1/admin/memory-quality/evaluations/{evaluation_run_id}/findings", adminQualityEvaluationFindings)

	adminRepairPlans := auth.APIKeyMiddleware(deps.AdminAPIKeys)(
		auth.ScopeMiddleware()(
			http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				handleAdminRepairPlans(w, r, deps.QualityAdmin)
			}),
		),
	)
	mux.Handle("GET /v1/admin/memory-quality/repair-plans/{repair_plan_id}", adminRepairPlans)
	mux.Handle("POST /v1/admin/memory-quality/repair-plans", adminRepairPlans)

	adminRepairPlanAction := auth.APIKeyMiddleware(deps.AdminAPIKeys)(
		auth.ScopeMiddleware()(
			http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				handleAdminRepairPlanAction(w, r, deps.QualityAdmin)
			}),
		),
	)
	mux.Handle("POST /v1/admin/memory-quality/repair-plans/{repair_plan_action}", adminRepairPlanAction)

	adminQualityDiagnostics := auth.APIKeyMiddleware(deps.AdminAPIKeys)(
		auth.ScopeMiddleware()(
			http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				handleAdminQualityDiagnostics(w, r, deps.QualityAdmin)
			}),
		),
	)
	mux.Handle("GET /v1/admin/memory-quality/diagnostics", adminQualityDiagnostics)

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

	adminContextProjectionRebuild := auth.APIKeyMiddleware(deps.AdminAPIKeys)(
		auth.ScopeMiddleware()(
			http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				handleAdminContextProjectionRebuild(w, r, deps.ContextProjectionAdmin)
			}),
		),
	)
	mux.Handle("POST /v1/admin/context-projections:rebuild", adminContextProjectionRebuild)

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

	handler := http.Handler(mux)
	if deps.PrincipalAuthorizer != nil {
		var observer telemetry.Observer
		if candidate, ok := deps.Metrics.(telemetry.Observer); ok {
			observer = candidate
		}
		handler = principalProtectedRoutes(handler, deps.PrincipalAuthorizer, observer)
	}
	return requestMiddleware(limitRequestBody(handler, deps.HTTP.MaxRequestBodyBytes), deps.Logger)
}

func limitRequestBody(next http.Handler, maxBytes int64) http.Handler {
	if maxBytes <= 0 {
		return next
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Body != nil {
			r.Body = http.MaxBytesReader(w, r.Body, maxBytes)
		}
		next.ServeHTTP(w, r)
	})
}

func principalProtectedRoutes(next http.Handler, authorizer auth.PrincipalAuthorizer, observer telemetry.Observer) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasPrefix(r.URL.Path, "/v1/") {
			next.ServeHTTP(w, r)
			return
		}
		requiredRole := auth.PrincipalRolePublic
		if strings.HasPrefix(r.URL.Path, "/v1/admin/") {
			requiredRole = auth.PrincipalRoleAdmin
		}
		auth.PrincipalMiddlewareWithObserver(authorizer, requiredRole, observer)(next).ServeHTTP(w, r)
	})
}

func NewHTTPServer(addr string, deps HTTPDependencies) *http.Server {
	return &http.Server{
		Addr:              addr,
		Handler:           NewHTTPHandler(deps),
		ReadHeaderTimeout: deps.HTTP.ReadHeaderTimeout,
		ReadTimeout:       deps.HTTP.ReadTimeout,
		WriteTimeout:      deps.HTTP.WriteTimeout,
		IdleTimeout:       deps.HTTP.IdleTimeout,
		MaxHeaderBytes:    deps.HTTP.MaxHeaderBytes,
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

func newTaskEvaluationID() string {
	return "task_eval_" + strings.TrimPrefix(newID(), "id_")
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
		writeJSONDecodeError(w, err)
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
	principal, principalAuthorized := auth.PrincipalFromContext(r.Context())
	if principalAuthorized {
		idempotencyKey := strings.TrimSpace(r.Header.Get("Idempotency-Key"))
		if err := auth.ValidateIdempotencyKey(idempotencyKey); err != nil {
			http.Error(w, "invalid idempotency key", http.StatusBadRequest)
			return
		}
		idempotentIngestor, ok := ingestor.(memory.IdempotentEventIngestor)
		if !ok {
			http.Error(w, "idempotent event ingestor is not configured", http.StatusServiceUnavailable)
			return
		}
		result, err := idempotentIngestor.IngestIdempotent(r.Context(), input, principal.ID, idempotencyKey)
		if err != nil {
			status := http.StatusInternalServerError
			switch {
			case errors.Is(err, memory.ErrAdmissionRejected):
				status = http.StatusUnprocessableEntity
			case errors.Is(err, memory.ErrIdempotencyConflict):
				status = http.StatusConflict
			case errors.Is(err, memory.ErrIdempotencyInProgress):
				status = http.StatusConflict
				w.Header().Set("Retry-After", "1")
			case errors.Is(err, context.Canceled), errors.Is(err, context.DeadlineExceeded):
				status = http.StatusGatewayTimeout
			}
			http.Error(w, "failed to ingest event", status)
			return
		}
		status := http.StatusCreated
		if result.Replayed {
			status = http.StatusOK
		}
		writeJSON(w, status, eventIngestResponse{EventID: result.Event.ID, Admission: result.Event.Admission, Replayed: result.Replayed})
		return
	}

	event, err := ingestor.Ingest(r.Context(), input)
	if err != nil {
		status := http.StatusInternalServerError
		if errors.Is(err, memory.ErrAdmissionRejected) {
			status = http.StatusUnprocessableEntity
		}
		if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
			status = http.StatusGatewayTimeout
		}
		http.Error(w, "failed to ingest event", status)
		return
	}

	writeJSON(w, http.StatusCreated, eventIngestResponse{EventID: event.ID, Admission: event.Admission})
}

func handleAdminPrincipalCreate(w http.ResponseWriter, r *http.Request, service PrincipalAdministrationService) {
	if service == nil {
		http.Error(w, "principal administration service is not configured", http.StatusServiceUnavailable)
		return
	}

	var request principalCreateRequest
	if !decodeJSONBody(w, r, &request) {
		return
	}

	scope, ok := auth.ScopeFromContext(r.Context())
	if !ok {
		http.Error(w, "scope context is missing", http.StatusInternalServerError)
		return
	}

	issued, err := service.CreatePrincipal(r.Context(), auth.CreatePrincipalInput{
		Role:   request.Role,
		Label:  request.Label,
		Grants: []memory.Scope{scope},
		Actor:  request.Actor,
		Reason: request.Reason,
	})
	if err != nil {
		if strings.Contains(err.Error(), "required") || strings.Contains(err.Error(), "invalid") || strings.Contains(err.Error(), "at most") {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		http.Error(w, "failed to create principal", http.StatusInternalServerError)
		return
	}

	writeJSON(w, http.StatusCreated, issued)
}

func handleAdminPrincipals(w http.ResponseWriter, r *http.Request, service PrincipalAdministrationService) {
	if service == nil {
		http.Error(w, "principal administration service is not configured", http.StatusServiceUnavailable)
		return
	}
	scope, ok := auth.ScopeFromContext(r.Context())
	if !ok {
		http.Error(w, "scope context is missing", http.StatusInternalServerError)
		return
	}
	principalID := r.PathValue("principal_id")
	if principalID == "" && r.Method == http.MethodPost {
		handleAdminPrincipalCreate(w, r, service)
		return
	}
	if principalID == "" && r.Method == http.MethodGet {
		principals, err := service.ListPrincipals(r.Context(), scope, queryLimit(r, 20))
		if err != nil {
			http.Error(w, "failed to list principals", http.StatusInternalServerError)
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"principals": principals})
		return
	}
	if r.Method == http.MethodGet && strings.HasSuffix(r.URL.Path, "/grants") {
		grants, err := service.ListScopeGrants(r.Context(), scope, principalID)
		if err != nil {
			http.Error(w, "failed to list scope grants", http.StatusInternalServerError)
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"grants": grants})
		return
	}
	if r.Method == http.MethodGet {
		principal, err := service.ReadPrincipal(r.Context(), scope, principalID)
		if err != nil {
			http.Error(w, "principal not found", http.StatusNotFound)
			return
		}
		writeJSON(w, http.StatusOK, principal)
		return
	}
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	switch {
	case strings.HasSuffix(r.URL.Path, "/credentials/rotate") || strings.HasSuffix(r.URL.Path, "/credentials:rotate"):
		handleAdminCredentialRotate(w, r, service)
	case strings.HasSuffix(r.URL.Path, "/disable"):
		handleAdminPrincipalDisable(w, r, service)
	case strings.HasSuffix(r.URL.Path, "/expire"):
		handleAdminPrincipalExpire(w, r, service)
	case strings.HasSuffix(r.URL.Path, "/grants"):
		handleAdminGrantCreate(w, r, service)
	case strings.HasSuffix(r.URL.Path, "/revoke"):
		handleAdminGrantRevoke(w, r, service)
	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

func handleAdminCredentialRotate(w http.ResponseWriter, r *http.Request, service PrincipalAdministrationService) {
	var request principalLifecycleRequest
	if !decodeJSONBody(w, r, &request) {
		return
	}
	scope, ok := auth.ScopeFromContext(r.Context())
	if !ok {
		http.Error(w, "scope context is missing", http.StatusInternalServerError)
		return
	}
	issued, err := service.RotateCredential(r.Context(), scope, r.PathValue("principal_id"), request.Actor, request.Reason)
	if err != nil {
		http.Error(w, "failed to rotate credential", http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusOK, issued)
}

func handleAdminPrincipalDisable(w http.ResponseWriter, r *http.Request, service PrincipalAdministrationService) {
	var request principalLifecycleRequest
	if !decodeJSONBody(w, r, &request) {
		return
	}
	scope, ok := auth.ScopeFromContext(r.Context())
	if !ok {
		http.Error(w, "scope context is missing", http.StatusInternalServerError)
		return
	}
	if err := service.DisablePrincipal(r.Context(), scope, r.PathValue("principal_id"), request.Actor, request.Reason); err != nil {
		http.Error(w, "failed to disable principal", http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func handleAdminPrincipalExpire(w http.ResponseWriter, r *http.Request, service PrincipalAdministrationService) {
	var request struct {
		ExpiresAt string `json:"expires_at"`
		Actor     string `json:"actor"`
		Reason    string `json:"reason"`
	}
	if !decodeJSONBody(w, r, &request) {
		return
	}
	expiresAt, err := time.Parse(time.RFC3339, strings.TrimSpace(request.ExpiresAt))
	if err != nil {
		http.Error(w, "invalid expires_at", http.StatusBadRequest)
		return
	}
	scope, ok := auth.ScopeFromContext(r.Context())
	if !ok {
		http.Error(w, "scope context is missing", http.StatusInternalServerError)
		return
	}
	if err := service.ExpirePrincipal(r.Context(), scope, r.PathValue("principal_id"), expiresAt, request.Actor, request.Reason); err != nil {
		http.Error(w, "failed to expire principal", http.StatusBadRequest)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func handleAdminGrantCreate(w http.ResponseWriter, r *http.Request, service PrincipalAdministrationService) {
	var request principalGrantRequest
	if !decodeJSONBody(w, r, &request) {
		return
	}
	scope, ok := auth.ScopeFromContext(r.Context())
	if !ok {
		http.Error(w, "scope context is missing", http.StatusInternalServerError)
		return
	}
	grantScope := memory.Scope{Tenant: request.Tenant, Project: request.Project, Namespace: request.Namespace}
	if err := service.CreateScopeGrant(r.Context(), scope, r.PathValue("principal_id"), grantScope, request.Actor, request.Reason); err != nil {
		http.Error(w, "failed to create scope grant", http.StatusBadRequest)
		return
	}
	w.WriteHeader(http.StatusCreated)
}

func handleAdminGrantRevoke(w http.ResponseWriter, r *http.Request, service PrincipalAdministrationService) {
	var request principalLifecycleRequest
	if !decodeJSONBody(w, r, &request) {
		return
	}
	scope, ok := auth.ScopeFromContext(r.Context())
	if !ok {
		http.Error(w, "scope context is missing", http.StatusInternalServerError)
		return
	}
	if err := service.RevokeScopeGrant(r.Context(), scope, r.PathValue("grant_id"), request.Actor, request.Reason); err != nil {
		http.Error(w, "failed to revoke scope grant", http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func handleAdminAccessAudit(w http.ResponseWriter, r *http.Request, service PrincipalAdministrationService) {
	if service == nil {
		http.Error(w, "principal administration service is not configured", http.StatusServiceUnavailable)
		return
	}
	scope, ok := auth.ScopeFromContext(r.Context())
	if !ok {
		http.Error(w, "scope context is missing", http.StatusInternalServerError)
		return
	}
	records, err := service.ListAccessAudit(r.Context(), scope, strings.TrimSpace(r.URL.Query().Get("principal_id")), queryLimit(r, 20))
	if err != nil {
		http.Error(w, "failed to list access audit", http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"audit": records})
}

func queryLimit(r *http.Request, fallback int) int {
	parsed, err := strconv.Atoi(strings.TrimSpace(r.URL.Query().Get("limit")))
	if err != nil || parsed <= 0 {
		return fallback
	}
	if parsed > 100 {
		return 100
	}
	return parsed
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
	if strings.TrimSpace(req.FeedbackRankingPolicy) != "" {
		http.Error(w, "feedback_ranking_policy is not supported; use per-request feedback_aware_ranking only", http.StatusBadRequest)
		return
	}

	input := retrieval.SearchInput{
		Scope:                      scope,
		Query:                      req.Query,
		QueryEmbedding:             req.QueryEmbedding,
		Classes:                    req.Classes,
		TopK:                       req.TopK,
		IncludeSummaries:           req.IncludeSummaries,
		IncludeRelations:           req.IncludeRelations,
		IncludeFeedbackDiagnostics: req.IncludeFeedbackDiagnostics,
		FeedbackAwareRanking:       req.FeedbackAwareRanking,
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
	if strings.TrimSpace(req.FeedbackRankingPolicy) != "" {
		http.Error(w, "feedback_ranking_policy is not supported; use per-request feedback_aware_ranking only", http.StatusBadRequest)
		return
	}

	result, err := assembler.AssembleContext(r.Context(), retrieval.AssembleContextInput{
		Scope:                      scope,
		Query:                      req.Query,
		Budget:                     req.Budget,
		IncludeRelations:           req.IncludeRelations,
		IncludeExperienceInsights:  req.IncludeExperienceInsights,
		IncludeDiagnostics:         req.IncludeDiagnostics,
		IncludeFeedbackDiagnostics: req.IncludeFeedbackDiagnostics,
		FeedbackAwareRanking:       req.FeedbackAwareRanking,
	})
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	writeJSON(w, http.StatusOK, result)
}

func handleMemorySessions(w http.ResponseWriter, r *http.Request, service MemorySessionService) {
	if service == nil {
		http.Error(w, "memory session service is not configured", http.StatusServiceUnavailable)
		return
	}
	scope, ok := auth.ScopeFromContext(r.Context())
	if !ok {
		http.Error(w, "scope context is missing", http.StatusInternalServerError)
		return
	}
	switch r.Method {
	case http.MethodPost:
		var req memorySessionCreateRequest
		if !decodeJSONBody(w, r, &req) {
			return
		}
		session, err := service.CreateSession(r.Context(), memory.CreateMemorySessionInput{
			Scope:    scope,
			Actor:    strings.TrimSpace(req.Actor),
			Reason:   strings.TrimSpace(req.Reason),
			Metadata: req.Metadata,
		})
		if err != nil {
			writeMemorySessionError(w, err, "failed to create memory session")
			return
		}
		writeJSON(w, http.StatusCreated, session)
	case http.MethodGet:
		limit := 20
		if raw := strings.TrimSpace(r.URL.Query().Get("limit")); raw != "" {
			parsed, err := strconv.Atoi(raw)
			if err != nil || parsed < 0 {
				http.Error(w, "invalid limit", http.StatusBadRequest)
				return
			}
			limit = parsed
		}
		sessions, err := service.ListSessions(r.Context(), memory.ListMemorySessionRunsInput{Scope: scope, Limit: limit})
		if err != nil {
			writeMemorySessionError(w, err, "failed to list memory sessions")
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"memory_sessions": sessions})
	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

func handleMemorySessionDetail(w http.ResponseWriter, r *http.Request, service MemorySessionService) {
	if service == nil {
		http.Error(w, "memory session service is not configured", http.StatusServiceUnavailable)
		return
	}
	scope, ok := auth.ScopeFromContext(r.Context())
	if !ok {
		http.Error(w, "scope context is missing", http.StatusInternalServerError)
		return
	}
	session, err := service.ReadSession(r.Context(), memory.ReadMemorySessionRunInput{
		Scope:     scope,
		SessionID: strings.TrimSpace(r.PathValue("session_id")),
	})
	if err != nil {
		writeMemorySessionError(w, err, "failed to read memory session")
		return
	}
	writeJSON(w, http.StatusOK, session)
}

func handleMemorySessionReport(w http.ResponseWriter, r *http.Request, service MemorySessionService) {
	if service == nil {
		http.Error(w, "memory session service is not configured", http.StatusServiceUnavailable)
		return
	}
	scope, ok := auth.ScopeFromContext(r.Context())
	if !ok {
		http.Error(w, "scope context is missing", http.StatusInternalServerError)
		return
	}
	report, err := service.ReadSessionReport(r.Context(), memory.ReadMemorySessionRunInput{
		Scope:     scope,
		SessionID: strings.TrimSpace(r.PathValue("session_id")),
	})
	if err != nil {
		writeMemorySessionError(w, err, "failed to read memory session report")
		return
	}
	writeJSON(w, http.StatusOK, report)
}

func handleMemorySessionTurns(w http.ResponseWriter, r *http.Request, service MemorySessionService) {
	if service == nil {
		http.Error(w, "memory session service is not configured", http.StatusServiceUnavailable)
		return
	}
	scope, ok := auth.ScopeFromContext(r.Context())
	if !ok {
		http.Error(w, "scope context is missing", http.StatusInternalServerError)
		return
	}
	var req memorySessionTurnCreateRequest
	if !decodeJSONBody(w, r, &req) {
		return
	}
	turn, err := service.CreateTurn(r.Context(), memory.CreateMemorySessionTurnInput{
		Scope:                     scope,
		SessionID:                 strings.TrimSpace(r.PathValue("session_id")),
		IdempotencyKey:            strings.TrimSpace(req.IdempotencyKey),
		Query:                     strings.TrimSpace(req.Query),
		ContextBudget:             req.ContextBudget,
		IncludeRelations:          req.IncludeRelations,
		IncludeExperienceInsights: req.IncludeExperienceInsights,
		IncludeDiagnostics:        req.IncludeDiagnostics,
	})
	if err != nil {
		writeMemorySessionError(w, err, "failed to create memory session turn")
		return
	}
	writeJSON(w, http.StatusCreated, turn)
}

func handleMemorySessionTurnAction(w http.ResponseWriter, r *http.Request, service MemorySessionService) {
	if service == nil {
		http.Error(w, "memory session service is not configured", http.StatusServiceUnavailable)
		return
	}
	scope, ok := auth.ScopeFromContext(r.Context())
	if !ok {
		http.Error(w, "scope context is missing", http.StatusInternalServerError)
		return
	}
	turnID, actionName, ok := strings.Cut(r.PathValue("turn_action"), ":")
	if !ok || strings.TrimSpace(turnID) == "" || actionName != "outcome" {
		http.Error(w, "invalid memory session turn action target", http.StatusBadRequest)
		return
	}
	var req memorySessionTurnOutcomeRequest
	if !decodeJSONBody(w, r, &req) {
		return
	}
	turn, err := service.RecordTurnOutcome(r.Context(), memory.RecordMemorySessionTurnOutcomeInput{
		Scope:                scope,
		SessionID:            strings.TrimSpace(r.PathValue("session_id")),
		TurnID:               strings.TrimSpace(turnID),
		IdempotencyKey:       strings.TrimSpace(req.IdempotencyKey),
		OutcomeEventIDs:      append([]string(nil), req.OutcomeEventIDs...),
		OutcomeEventPayloads: append([]memory.MemorySessionOutcomeEventPayload(nil), req.OutcomeEventPayloads...),
		ExpectedRecall:       append([]string(nil), req.ExpectedRecall...),
	})
	if err != nil {
		writeMemorySessionError(w, err, "failed to record memory session turn outcome")
		return
	}
	writeJSON(w, http.StatusOK, turn)
}

func handleMemorySessionAction(w http.ResponseWriter, r *http.Request, service MemorySessionService) {
	if service == nil {
		http.Error(w, "memory session service is not configured", http.StatusServiceUnavailable)
		return
	}
	scope, ok := auth.ScopeFromContext(r.Context())
	if !ok {
		http.Error(w, "scope context is missing", http.StatusInternalServerError)
		return
	}
	sessionID, actionName, ok := strings.Cut(r.PathValue("session_action"), ":")
	if !ok || strings.TrimSpace(sessionID) == "" || actionName != "verify" {
		http.Error(w, "invalid memory session action target", http.StatusBadRequest)
		return
	}
	var req memorySessionVerificationRequest
	if !decodeJSONBody(w, r, &req) {
		return
	}
	verification, err := service.RequestVerification(r.Context(), memory.RequestMemorySessionVerificationInput{
		Scope:          scope,
		SessionID:      strings.TrimSpace(sessionID),
		TurnID:         strings.TrimSpace(req.TurnID),
		ExpectedRecall: append([]string(nil), req.ExpectedRecall...),
	})
	if err != nil {
		writeMemorySessionError(w, err, "failed to request memory session verification")
		return
	}
	writeJSON(w, http.StatusAccepted, verification)
}

func handleUsefulnessFeedbackCreate(w http.ResponseWriter, r *http.Request, service UsefulnessFeedbackService, metrics MetricsRecorder, logger *log.Logger) {
	if service == nil {
		http.Error(w, "usefulness feedback service is not configured", http.StatusServiceUnavailable)
		return
	}
	scope, ok := auth.ScopeFromContext(r.Context())
	if !ok {
		http.Error(w, "scope context is missing", http.StatusInternalServerError)
		return
	}
	var req usefulnessFeedbackCreateRequest
	if !decodeJSONBody(w, r, &req) {
		return
	}
	feedback := memory.UsefulnessFeedback{
		ID:               newFeedbackID(),
		Scope:            scope,
		Type:             req.Type,
		SourceSurface:    req.SourceSurface,
		TaskEvaluationID: strings.TrimSpace(req.TaskEvaluationID),
		Subjects:         append([]memory.UsefulnessFeedbackSubject(nil), req.Subjects...),
		Actor:            strings.TrimSpace(req.Actor),
		Reason:           strings.TrimSpace(req.Reason),
		IdempotencyKey:   strings.TrimSpace(req.IdempotencyKey),
		Metadata:         req.Metadata,
		CreatedAt:        time.Now().UTC(),
	}
	created, err := service.CreateUsefulnessFeedback(r.Context(), feedback)
	if err != nil {
		recordUsefulnessFeedbackMetric(r.Context(), metrics, "create", "rejected", req.Type, firstUsefulnessSubjectKind(req.Subjects), req.SourceSurface, "active")
		recordUsefulnessFeedbackLog(logger, "create", "rejected", req.Type, firstUsefulnessSubjectKind(req.Subjects), req.SourceSurface, "active")
		writeUsefulnessFeedbackError(w, err, "failed to create usefulness feedback")
		return
	}
	recordUsefulnessFeedbackMetric(r.Context(), metrics, "create", "ok", created.Type, firstUsefulnessSubjectKind(created.Subjects), created.SourceSurface, "active")
	recordUsefulnessFeedbackLog(logger, "create", "ok", created.Type, firstUsefulnessSubjectKind(created.Subjects), created.SourceSurface, "active")
	writeJSON(w, http.StatusCreated, created)
}

func handleTaskEvaluationCreate(w http.ResponseWriter, r *http.Request, service TaskEvaluationService, metrics MetricsRecorder, logger *log.Logger) {
	if service == nil {
		http.Error(w, "task evaluation service is not configured", http.StatusServiceUnavailable)
		return
	}
	scope, ok := auth.ScopeFromContext(r.Context())
	if !ok {
		http.Error(w, "scope context is missing", http.StatusInternalServerError)
		return
	}
	var req taskEvaluationCreateRequest
	if !decodeJSONBody(w, r, &req) {
		return
	}
	evaluation := memory.TaskEvaluation{
		ID:                     newTaskEvaluationID(),
		Scope:                  scope,
		Objective:              strings.TrimSpace(req.Objective),
		SuccessCriteria:        append([]string(nil), req.SuccessCriteria...),
		Verdict:                req.Verdict,
		ContributionCategories: append([]memory.TaskContributionCategory(nil), req.ContributionCategories...),
		Evidence:               append([]memory.TaskEvidenceLink(nil), req.Evidence...),
		Actor:                  strings.TrimSpace(req.Actor),
		Reason:                 strings.TrimSpace(req.Reason),
		IdempotencyKey:         strings.TrimSpace(req.IdempotencyKey),
		Metadata:               req.Metadata,
		CreatedAt:              time.Now().UTC(),
		UpdatedAt:              time.Now().UTC(),
		CorrectionState:        memory.TaskEvaluationCorrectionStateActive,
	}
	created, err := service.CreateTaskEvaluation(r.Context(), evaluation)
	if err != nil {
		recordTaskEvaluationMetric(r.Context(), metrics, "create", "rejected", evaluation.Verdict, firstTaskContributionCategory(evaluation.ContributionCategories), evaluation.CorrectionState)
		recordTaskEvaluationLog(logger, "create", "rejected", evaluation.Verdict, firstTaskContributionCategory(evaluation.ContributionCategories), evaluation.CorrectionState)
		writeTaskEvaluationError(w, err, "failed to create task evaluation")
		return
	}
	recordTaskEvaluationMetric(r.Context(), metrics, "create", "ok", created.Verdict, firstTaskContributionCategory(created.ContributionCategories), created.CorrectionState)
	recordTaskEvaluationLog(logger, "create", "ok", created.Verdict, firstTaskContributionCategory(created.ContributionCategories), created.CorrectionState)
	writeJSON(w, http.StatusCreated, created)
}

func handleTaskEvaluationReport(w http.ResponseWriter, r *http.Request, service TaskEvaluationService, metrics MetricsRecorder, logger *log.Logger) {
	if service == nil {
		http.Error(w, "task evaluation service is not configured", http.StatusServiceUnavailable)
		return
	}
	scope, ok := auth.ScopeFromContext(r.Context())
	if !ok {
		http.Error(w, "scope context is missing", http.StatusInternalServerError)
		return
	}
	evaluation, err := service.ReadTaskEvaluation(r.Context(), memory.ReadTaskEvaluationInput{
		Scope:        scope,
		EvaluationID: strings.TrimSpace(r.PathValue("evaluation_id")),
	})
	if err != nil {
		recordTaskEvaluationMetric(r.Context(), metrics, "report", "error", "", "", "")
		recordTaskEvaluationLog(logger, "report", "error", "", "", "")
		writeTaskEvaluationError(w, err, "failed to read task evaluation")
		return
	}
	summary, err := service.SummarizeTaskEvaluations(r.Context(), memory.SummarizeTaskEvaluationsInput{
		Scope: scope,
	})
	if err != nil {
		recordTaskEvaluationMetric(r.Context(), metrics, "report", "error", evaluation.Verdict, firstTaskContributionCategory(evaluation.ContributionCategories), evaluation.CorrectionState)
		recordTaskEvaluationLog(logger, "report", "error", evaluation.Verdict, firstTaskContributionCategory(evaluation.ContributionCategories), evaluation.CorrectionState)
		writeTaskEvaluationError(w, err, "failed to summarize task evaluations")
		return
	}
	recordTaskEvaluationMetric(r.Context(), metrics, "report", "ok", evaluation.Verdict, firstTaskContributionCategory(evaluation.ContributionCategories), evaluation.CorrectionState)
	recordTaskEvaluationLog(logger, "report", "ok", evaluation.Verdict, firstTaskContributionCategory(evaluation.ContributionCategories), evaluation.CorrectionState)
	writeJSON(w, http.StatusOK, memory.BuildTaskEvaluationReport(evaluation, summary))
}

func handleWorkflows(w http.ResponseWriter, r *http.Request, service WorkflowService) {
	if service == nil {
		http.Error(w, "workflow service is not configured", http.StatusServiceUnavailable)
		return
	}
	scope, ok := auth.ScopeFromContext(r.Context())
	if !ok {
		http.Error(w, "scope context is missing", http.StatusInternalServerError)
		return
	}
	switch {
	case r.URL.Path == "/v1/workflows/runs" && r.Method == http.MethodPost:
		handleWorkflowRunCreate(w, r, service, scope)
	case strings.HasSuffix(r.URL.Path, "/steps") && r.Method == http.MethodPost:
		handleWorkflowStepRecord(w, r, service, scope)
	case strings.HasSuffix(r.URL.Path, "/next-actions") && r.Method == http.MethodGet:
		handleWorkflowNextActionList(w, r, service, scope, true)
	case strings.TrimSpace(r.PathValue("workflow_run_id")) != "" && r.Method == http.MethodGet:
		run, err := service.ReadRun(r.Context(), workflow.ReadRunInput{
			Scope: scope,
			RunID: strings.TrimSpace(r.PathValue("workflow_run_id")),
		})
		if err != nil {
			writeWorkflowError(w, err, "failed to read workflow run")
			return
		}
		writeJSON(w, http.StatusOK, publicWorkflowRunFromRun(run, false))
	default:
		http.NotFound(w, r)
	}
}

func handleWorkflowRunCreate(w http.ResponseWriter, r *http.Request, service WorkflowService, scope memory.Scope) {
	var req workflowRunCreateRequest
	if !decodeJSONBody(w, r, &req) {
		return
	}
	var expiresAt time.Time
	if strings.TrimSpace(req.ExpiresAt) != "" {
		parsed, err := time.Parse(time.RFC3339, strings.TrimSpace(req.ExpiresAt))
		if err != nil {
			http.Error(w, "invalid expires_at", http.StatusBadRequest)
			return
		}
		expiresAt = parsed
	}
	run, err := service.StartRun(r.Context(), workflow.StartRunInput{
		Scope:          scope,
		TemplateID:     strings.TrimSpace(req.TemplateID),
		IdempotencyKey: strings.TrimSpace(req.IdempotencyKey),
		Actor:          strings.TrimSpace(req.Actor),
		Reason:         strings.TrimSpace(req.Reason),
		Metadata:       req.Metadata,
		ExpiresAt:      expiresAt,
	})
	if err != nil {
		writeWorkflowError(w, err, "failed to start workflow run")
		return
	}
	writeJSON(w, http.StatusCreated, publicWorkflowRunFromRun(run, true))
}

func handleWorkflowStepRecord(w http.ResponseWriter, r *http.Request, service WorkflowService, scope memory.Scope) {
	var req workflowStepRecordRequest
	if !decodeJSONBody(w, r, &req) {
		return
	}
	var observedAt time.Time
	if strings.TrimSpace(req.ObservedAt) != "" {
		parsed, err := time.Parse(time.RFC3339, strings.TrimSpace(req.ObservedAt))
		if err != nil {
			http.Error(w, "invalid observed_at", http.StatusBadRequest)
			return
		}
		observedAt = parsed
	}
	record, err := service.RecordStep(r.Context(), workflow.RecordStepInput{
		Scope:         scope,
		RunID:         strings.TrimSpace(r.PathValue("workflow_run_id")),
		Kind:          req.Kind,
		Actor:         strings.TrimSpace(req.Actor),
		Reason:        strings.TrimSpace(req.Reason),
		ObservedAt:    observedAt,
		Metadata:      req.Metadata,
		EvidenceLinks: append([]workflow.EvidenceLink(nil), req.EvidenceLinks...),
	})
	if err != nil {
		writeWorkflowError(w, err, "failed to record workflow step")
		return
	}
	writeJSON(w, http.StatusCreated, publicWorkflowStepRecord{
		Kind:       record.Kind,
		Status:     record.Status,
		Result:     record.Result,
		ObservedAt: record.ObservedAt,
		CreatedAt:  record.CreatedAt,
	})
}

func publicWorkflowRunFromRun(run workflow.WorkflowRun, includeID bool) publicWorkflowRun {
	public := publicWorkflowRun{
		Status:          run.Status,
		IntegrationKind: run.IntegrationKind,
		CreatedAt:       run.CreatedAt,
		UpdatedAt:       run.UpdatedAt,
		StartedAt:       run.StartedAt,
		CompletedAt:     run.CompletedAt,
		ExpiresAt:       run.ExpiresAt,
	}
	if includeID {
		public.ID = run.ID
	}
	return public
}

func handleWorkflowNextActionList(w http.ResponseWriter, r *http.Request, service WorkflowService, scope memory.Scope, redact bool) {
	limit, err := parseOptionalLimit(r, 50)
	if err != nil {
		http.Error(w, "invalid limit", http.StatusBadRequest)
		return
	}
	actions, err := service.ListNextActions(r.Context(), workflow.ListNextActionsInput{
		Scope:  scope,
		RunID:  strings.TrimSpace(r.PathValue("workflow_run_id")),
		Status: workflow.NextActionStatus(strings.TrimSpace(r.URL.Query().Get("status"))),
		Limit:  limit,
	})
	if err != nil {
		writeWorkflowError(w, err, "failed to list workflow next actions")
		return
	}
	if !redact {
		writeJSON(w, http.StatusOK, map[string]any{"next_actions": actions})
		return
	}
	publicActions := make([]publicWorkflowNextAction, 0, len(actions))
	for _, action := range actions {
		publicActions = append(publicActions, publicWorkflowNextAction{
			Category:      action.Category,
			StepKind:      action.StepKind,
			EvidenceKind:  action.EvidenceKind,
			RouteCategory: action.RouteCategory,
			Status:        action.Status,
		})
	}
	writeJSON(w, http.StatusOK, map[string]any{"next_actions": publicActions})
}

func handleAdminWorkflows(w http.ResponseWriter, r *http.Request, service WorkflowService) {
	if service == nil {
		http.Error(w, "workflow service is not configured", http.StatusServiceUnavailable)
		return
	}
	scope, ok := auth.ScopeFromContext(r.Context())
	if !ok {
		http.Error(w, "scope context is missing", http.StatusInternalServerError)
		return
	}
	path := r.URL.Path
	switch {
	case path == "/v1/admin/workflows/templates" && r.Method == http.MethodPost:
		handleAdminWorkflowTemplateCreate(w, r, service, scope)
	case path == "/v1/admin/workflows/templates" && r.Method == http.MethodGet:
		handleAdminWorkflowTemplateList(w, r, service, scope)
	case strings.HasSuffix(path, "/disable") && r.Method == http.MethodPost:
		handleAdminWorkflowTemplateDisable(w, r, service, scope)
	case strings.Contains(path, "/v1/admin/workflows/templates/") && r.Method == http.MethodPatch:
		handleAdminWorkflowTemplateUpdate(w, r, service, scope)
	case strings.Contains(path, "/v1/admin/workflows/templates/") && r.Method == http.MethodGet:
		template, err := service.ReadTemplate(r.Context(), workflow.ReadTemplateInput{
			Scope:      scope,
			TemplateID: strings.TrimSpace(r.PathValue("workflow_template_id")),
		})
		if err != nil {
			writeWorkflowError(w, err, "failed to read workflow template")
			return
		}
		writeJSON(w, http.StatusOK, template)
	case path == "/v1/admin/workflows/runs" && r.Method == http.MethodGet:
		handleAdminWorkflowRunList(w, r, service, scope)
	case strings.HasSuffix(path, "/steps") && r.Method == http.MethodGet:
		steps, err := service.ListStepRecords(r.Context(), workflow.ListStepRecordsInput{
			Scope: scope,
			RunID: strings.TrimSpace(r.PathValue("workflow_run_id")),
		})
		if err != nil {
			writeWorkflowError(w, err, "failed to list workflow steps")
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"steps": steps})
	case strings.HasSuffix(path, "/evidence-links") && r.Method == http.MethodGet:
		links, err := service.ListEvidenceLinks(r.Context(), workflow.ListEvidenceLinksInput{
			Scope:  scope,
			RunID:  strings.TrimSpace(r.PathValue("workflow_run_id")),
			Status: workflow.EvidenceLinkStatus(strings.TrimSpace(r.URL.Query().Get("status"))),
		})
		if err != nil {
			writeWorkflowError(w, err, "failed to list workflow evidence links")
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"evidence_links": links})
	case strings.HasSuffix(path, "/diagnostics") && r.Method == http.MethodGet:
		limit, err := parseOptionalLimit(r, 50)
		if err != nil {
			http.Error(w, "invalid limit", http.StatusBadRequest)
			return
		}
		diagnostics, err := service.ListDiagnostics(r.Context(), workflow.ListDiagnosticsInput{
			Scope:    scope,
			RunID:    strings.TrimSpace(r.PathValue("workflow_run_id")),
			Category: workflow.DiagnosticCategory(strings.TrimSpace(r.URL.Query().Get("category"))),
			Limit:    limit,
		})
		if err != nil {
			writeWorkflowError(w, err, "failed to list workflow diagnostics")
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"diagnostics": diagnostics})
	case strings.HasSuffix(path, "/next-actions") && r.Method == http.MethodGet:
		handleWorkflowNextActionList(w, r, service, scope, false)
	case strings.Contains(path, "/v1/admin/workflows/runs/") && r.Method == http.MethodGet:
		run, err := service.ReadRun(r.Context(), workflow.ReadRunInput{
			Scope: scope,
			RunID: strings.TrimSpace(r.PathValue("workflow_run_id")),
		})
		if err != nil {
			writeWorkflowError(w, err, "failed to read workflow run")
			return
		}
		writeJSON(w, http.StatusOK, run)
	case strings.Contains(path, "/v1/admin/workflows/evidence-links/") && strings.HasSuffix(path, "/supersede") && r.Method == http.MethodPost:
		handleAdminWorkflowEvidenceSupersede(w, r, service, scope)
	default:
		http.NotFound(w, r)
	}
}

func handleAdminWorkflowTemplateCreate(w http.ResponseWriter, r *http.Request, service WorkflowService, scope memory.Scope) {
	var req workflowTemplateCreateRequest
	if !decodeJSONBody(w, r, &req) {
		return
	}
	steps, err := workflowTemplateStepsFromRequest(req.Steps)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	template, err := service.CreateTemplate(r.Context(), workflow.CreateTemplateInput{
		Scope:            scope,
		Steps:            steps,
		IntegrationKind:  req.IntegrationKind,
		CompletionPolicy: req.CompletionPolicy,
		Actor:            strings.TrimSpace(req.Actor),
		Reason:           strings.TrimSpace(req.Reason),
		Metadata:         req.Metadata,
	})
	if err != nil {
		writeWorkflowError(w, err, "failed to create workflow template")
		return
	}
	writeJSON(w, http.StatusCreated, template)
}

func handleAdminWorkflowTemplateUpdate(w http.ResponseWriter, r *http.Request, service WorkflowService, scope memory.Scope) {
	var req workflowTemplateUpdateRequest
	if !decodeJSONBody(w, r, &req) {
		return
	}
	steps, err := workflowTemplateStepsFromRequest(req.Steps)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	template, err := service.UpdateTemplate(r.Context(), workflow.UpdateTemplateInput{
		Scope:            scope,
		TemplateID:       strings.TrimSpace(r.PathValue("workflow_template_id")),
		Steps:            steps,
		IntegrationKind:  req.IntegrationKind,
		CompletionPolicy: req.CompletionPolicy,
		Actor:            strings.TrimSpace(req.Actor),
		Reason:           strings.TrimSpace(req.Reason),
		Metadata:         req.Metadata,
		UpdatedAt:        time.Now().UTC(),
	})
	if err != nil {
		writeWorkflowError(w, err, "failed to update workflow template")
		return
	}
	writeJSON(w, http.StatusOK, template)
}

func handleAdminWorkflowTemplateDisable(w http.ResponseWriter, r *http.Request, service WorkflowService, scope memory.Scope) {
	var req workflowActorReasonRequest
	if !decodeJSONBody(w, r, &req) {
		return
	}
	template, err := service.DisableTemplate(r.Context(), workflow.DisableTemplateInput{
		Scope:      scope,
		TemplateID: strings.TrimSpace(r.PathValue("workflow_template_id")),
		Actor:      strings.TrimSpace(req.Actor),
		Reason:     strings.TrimSpace(req.Reason),
		DisabledAt: time.Now().UTC(),
	})
	if err != nil {
		writeWorkflowError(w, err, "failed to disable workflow template")
		return
	}
	writeJSON(w, http.StatusOK, template)
}

func handleAdminWorkflowTemplateList(w http.ResponseWriter, r *http.Request, service WorkflowService, scope memory.Scope) {
	limit, err := parseOptionalLimit(r, 50)
	if err != nil {
		http.Error(w, "invalid limit", http.StatusBadRequest)
		return
	}
	templates, err := service.ListTemplates(r.Context(), workflow.ListTemplatesInput{
		Scope:  scope,
		Status: workflow.TemplateStatus(strings.TrimSpace(r.URL.Query().Get("status"))),
		Limit:  limit,
	})
	if err != nil {
		writeWorkflowError(w, err, "failed to list workflow templates")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"templates": templates})
}

func handleAdminWorkflowRunList(w http.ResponseWriter, r *http.Request, service WorkflowService, scope memory.Scope) {
	limit, err := parseOptionalLimit(r, 50)
	if err != nil {
		http.Error(w, "invalid limit", http.StatusBadRequest)
		return
	}
	runs, err := service.ListRuns(r.Context(), workflow.ListRunsInput{
		Scope:      scope,
		TemplateID: strings.TrimSpace(r.URL.Query().Get("template_id")),
		Status:     workflow.RunStatus(strings.TrimSpace(r.URL.Query().Get("status"))),
		Limit:      limit,
	})
	if err != nil {
		writeWorkflowError(w, err, "failed to list workflow runs")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"runs": runs})
}

func handleAdminWorkflowEvidenceSupersede(w http.ResponseWriter, r *http.Request, service WorkflowService, scope memory.Scope) {
	var req workflowActorReasonRequest
	if !decodeJSONBody(w, r, &req) {
		return
	}
	if err := service.SupersedeEvidenceLink(r.Context(), workflow.SupersedeEvidenceLinkInput{
		Scope:        scope,
		LinkID:       strings.TrimSpace(r.PathValue("evidence_link_id")),
		Actor:        strings.TrimSpace(req.Actor),
		Reason:       strings.TrimSpace(req.Reason),
		SupersededAt: time.Now().UTC(),
	}); err != nil {
		writeWorkflowError(w, err, "failed to supersede workflow evidence link")
		return
	}
	w.WriteHeader(http.StatusAccepted)
}

func workflowTemplateStepsFromRequest(requests []workflowTemplateStepRequest) ([]workflow.TemplateStep, error) {
	steps := make([]workflow.TemplateStep, 0, len(requests))
	for _, req := range requests {
		freshness, err := parseRequiredDuration(req.FreshnessWindow, "freshness_window")
		if err != nil {
			return nil, err
		}
		completion, err := parseRequiredDuration(req.CompletionWindow, "completion_window")
		if err != nil {
			return nil, err
		}
		steps = append(steps, workflow.TemplateStep{
			Kind:             req.Kind,
			Requirement:      req.Requirement,
			AllowedEvidence:  append([]workflow.EvidenceKind(nil), req.AllowedEvidence...),
			MinimumCount:     req.MinimumCount,
			RequiresInternal: req.RequiresInternal,
			FreshnessWindow:  freshness,
			CompletionWindow: completion,
			Position:         req.Position,
			Metadata:         req.Metadata,
		})
	}
	return steps, nil
}

func parseRequiredDuration(value string, field string) (time.Duration, error) {
	if strings.TrimSpace(value) == "" {
		return 0, fmt.Errorf("%s is required", field)
	}
	duration, err := time.ParseDuration(strings.TrimSpace(value))
	if err != nil {
		return 0, fmt.Errorf("invalid %s", field)
	}
	return duration, nil
}

func handleAdminTaskEvaluations(w http.ResponseWriter, r *http.Request, service TaskEvaluationService, metrics MetricsRecorder, logger *log.Logger) {
	if service == nil {
		http.Error(w, "task evaluation service is not configured", http.StatusServiceUnavailable)
		return
	}
	switch {
	case strings.Contains(r.URL.Path, "/summary"):
		handleAdminTaskEvaluationSummary(w, r, service, metrics, logger)
	case strings.HasSuffix(r.URL.Path, "/supersede"):
		handleAdminTaskEvaluationAction(w, r, service, metrics, logger)
	case strings.TrimSpace(r.PathValue("evaluation_id")) != "":
		handleAdminTaskEvaluationDetail(w, r, service, metrics, logger)
	default:
		handleAdminTaskEvaluationList(w, r, service, metrics, logger)
	}
}

func handleAdminRankingRollouts(w http.ResponseWriter, r *http.Request, service RankingRolloutAdminService, metrics MetricsRecorder, logger *log.Logger) {
	if service == nil {
		http.Error(w, "ranking rollout service is not configured", http.StatusServiceUnavailable)
		return
	}
	switch {
	case r.URL.Path == "/v1/admin/ranking-rollouts" && r.Method == http.MethodGet:
		handleAdminRankingRolloutList(w, r, service, metrics, logger)
	case r.URL.Path == "/v1/admin/ranking-rollouts" && r.Method == http.MethodPost:
		handleAdminRankingRolloutCreate(w, r, service, metrics, logger)
	case strings.HasSuffix(r.URL.Path, "/impact"):
		handleAdminRankingRolloutImpact(w, r, service, metrics, logger)
	case strings.HasSuffix(r.URL.Path, "/activate"):
		handleAdminRankingRolloutAction(w, r, service, "activate", metrics, logger)
	case strings.HasSuffix(r.URL.Path, "/disable"):
		handleAdminRankingRolloutAction(w, r, service, "disable", metrics, logger)
	case strings.HasSuffix(r.URL.Path, "/rollback"):
		handleAdminRankingRolloutAction(w, r, service, "rollback", metrics, logger)
	case strings.HasSuffix(r.URL.Path, "/dry-run"):
		handleAdminRankingRolloutDryRun(w, r, service, metrics, logger)
	default:
		handleAdminRankingRolloutDetail(w, r, service, metrics, logger)
	}
}

func handleAdminAssurance(w http.ResponseWriter, r *http.Request, service AssuranceAdminService) {
	if service == nil {
		http.Error(w, "assurance admin service is not configured", http.StatusServiceUnavailable)
		return
	}
	scope, ok := auth.ScopeFromContext(r.Context())
	if !ok {
		http.Error(w, "scope context is missing", http.StatusInternalServerError)
		return
	}

	path := r.URL.Path
	switch {
	case path == "/v1/admin/assurance/health-evaluations" && r.Method == http.MethodPost:
		handleAdminAssuranceHealthCreate(w, r, service, scope)
	case path == "/v1/admin/assurance/health-evaluations" && r.Method == http.MethodGet:
		items, err := service.ListHealthEvaluations(r.Context(), scope)
		if err != nil {
			writeAdminAssuranceError(w, err, "failed to list health evaluations")
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"health_evaluations": items})
	case strings.HasPrefix(path, "/v1/admin/assurance/health-evaluations/") && r.Method == http.MethodGet:
		item, err := service.ReadHealthEvaluation(r.Context(), assurance.ReadHealthEvaluationInput{
			Scope:        scope,
			EvaluationID: strings.TrimSpace(r.PathValue("health_evaluation_id")),
		})
		if err != nil {
			writeAdminAssuranceError(w, err, "failed to read health evaluation")
			return
		}
		writeJSON(w, http.StatusOK, item)
	case path == "/v1/admin/assurance/incidents" && r.Method == http.MethodGet:
		limit, err := parseOptionalLimit(r, 50)
		if err != nil {
			http.Error(w, "invalid limit", http.StatusBadRequest)
			return
		}
		items, err := service.ListIncidents(r.Context(), assurance.ListIncidentsInput{
			Scope:  scope,
			Status: assurance.IncidentStatus(strings.TrimSpace(r.URL.Query().Get("status"))),
			Limit:  limit,
		})
		if err != nil {
			writeAdminAssuranceError(w, err, "failed to list incidents")
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"incidents": items})
	case strings.HasPrefix(path, "/v1/admin/assurance/incidents/") && r.PathValue("incident_action") != "" && r.Method == http.MethodPost:
		handleAdminAssuranceIncidentAction(w, r, service, scope)
	case strings.HasPrefix(path, "/v1/admin/assurance/incidents/") && r.Method == http.MethodGet:
		item, err := service.ReadIncident(r.Context(), assurance.ReadIncidentInput{
			Scope:      scope,
			IncidentID: strings.TrimSpace(r.PathValue("incident_id")),
		})
		if err != nil {
			writeAdminAssuranceError(w, err, "failed to read incident")
			return
		}
		writeJSON(w, http.StatusOK, item)
	case path == "/v1/admin/assurance/alert-candidates" && r.Method == http.MethodGet:
		items, err := service.ListAlertCandidates(r.Context(), scope)
		if err != nil {
			writeAdminAssuranceError(w, err, "failed to list alert candidates")
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"alert_candidates": items})
	case strings.HasSuffix(path, "/delivery-attempts") && r.Method == http.MethodGet:
		items, err := service.ListAlertDeliveryAttempts(r.Context(), assurance.ListAlertDeliveryAttemptsInput{
			Scope:            scope,
			AlertCandidateID: strings.TrimSpace(r.PathValue("alert_candidate_id")),
		})
		if err != nil {
			writeAdminAssuranceError(w, err, "failed to list alert delivery attempts")
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"delivery_attempts": items})
	case strings.HasPrefix(path, "/v1/admin/assurance/alert-candidates/") && r.Method == http.MethodGet:
		item, err := service.ReadAlertCandidate(r.Context(), assurance.ReadAlertCandidateInput{
			Scope:            scope,
			AlertCandidateID: strings.TrimSpace(r.PathValue("alert_candidate_id")),
		})
		if err != nil {
			writeAdminAssuranceError(w, err, "failed to read alert candidate")
			return
		}
		writeJSON(w, http.StatusOK, item)
	case path == "/v1/admin/assurance/conformance-profiles" && r.Method == http.MethodPost:
		handleAdminAssuranceConformanceProfileCreate(w, r, service, scope)
	case path == "/v1/admin/assurance/conformance-profiles" && r.Method == http.MethodGet:
		items, err := service.ListConformanceProfiles(r.Context(), assurance.ListConformanceProfilesInput{
			Scope:  scope,
			Status: assurance.ConformanceProfileStatus(strings.TrimSpace(r.URL.Query().Get("status"))),
		})
		if err != nil {
			writeAdminAssuranceError(w, err, "failed to list conformance profiles")
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"conformance_profiles": items})
	case strings.HasSuffix(path, "/disable") && r.Method == http.MethodPost:
		handleAdminAssuranceConformanceProfileDisable(w, r, service, scope)
	case strings.HasPrefix(path, "/v1/admin/assurance/conformance-profiles/") && r.Method == http.MethodPatch:
		handleAdminAssuranceConformanceProfileUpdate(w, r, service, scope)
	case strings.HasPrefix(path, "/v1/admin/assurance/conformance-profiles/") && r.Method == http.MethodGet:
		item, err := service.ReadConformanceProfile(r.Context(), assurance.ReadConformanceProfileInput{
			Scope:     scope,
			ProfileID: strings.TrimSpace(r.PathValue("conformance_profile_id")),
		})
		if err != nil {
			writeAdminAssuranceError(w, err, "failed to read conformance profile")
			return
		}
		writeJSON(w, http.StatusOK, item)
	case path == "/v1/admin/assurance/conformance-runs" && r.Method == http.MethodPost:
		handleAdminAssuranceConformanceRunCreate(w, r, service, scope)
	case path == "/v1/admin/assurance/conformance-runs" && r.Method == http.MethodGet:
		items, err := service.ListConformanceRuns(r.Context(), assurance.ListConformanceRunsInput{
			Scope:     scope,
			ProfileID: strings.TrimSpace(r.URL.Query().Get("profile_id")),
		})
		if err != nil {
			writeAdminAssuranceError(w, err, "failed to list conformance runs")
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"conformance_runs": items})
	case strings.HasPrefix(path, "/v1/admin/assurance/conformance-runs/") && r.Method == http.MethodGet:
		item, err := service.ReadConformanceRun(r.Context(), assurance.ReadConformanceRunInput{
			Scope: scope,
			RunID: strings.TrimSpace(r.PathValue("conformance_run_id")),
		})
		if err != nil {
			writeAdminAssuranceError(w, err, "failed to read conformance run")
			return
		}
		writeJSON(w, http.StatusOK, item)
	case path == "/v1/admin/assurance/readiness-reports" && r.Method == http.MethodPost:
		handleAdminAssuranceReadinessCreate(w, r, service, scope)
	case path == "/v1/admin/assurance/readiness-reports" && r.Method == http.MethodGet:
		items, err := service.ListReadinessReports(r.Context(), scope)
		if err != nil {
			writeAdminAssuranceError(w, err, "failed to list readiness reports")
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"readiness_reports": items})
	case strings.HasPrefix(path, "/v1/admin/assurance/readiness-reports/") && r.Method == http.MethodGet:
		item, err := service.ReadReadinessReport(r.Context(), assurance.ReadReadinessReportInput{
			Scope:    scope,
			ReportID: strings.TrimSpace(r.PathValue("readiness_report_id")),
		})
		if err != nil {
			writeAdminAssuranceError(w, err, "failed to read readiness report")
			return
		}
		writeJSON(w, http.StatusOK, item)
	case path == "/v1/admin/assurance/recovery-verifications" && r.Method == http.MethodPost:
		handleAdminAssuranceRecoveryCreate(w, r, service, scope)
	case path == "/v1/admin/assurance/recovery-verifications" && r.Method == http.MethodGet:
		items, err := service.ListRecoveryVerifications(r.Context(), scope)
		if err != nil {
			writeAdminAssuranceError(w, err, "failed to list recovery verifications")
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"recovery_verifications": items})
	case strings.HasPrefix(path, "/v1/admin/assurance/recovery-verifications/") && r.Method == http.MethodGet:
		item, err := service.ReadRecoveryVerification(r.Context(), assurance.ReadRecoveryVerificationInput{
			Scope:    scope,
			RecordID: strings.TrimSpace(r.PathValue("recovery_verification_id")),
		})
		if err != nil {
			writeAdminAssuranceError(w, err, "failed to read recovery verification")
			return
		}
		writeJSON(w, http.StatusOK, item)
	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

func handleAdminAssuranceHealthCreate(w http.ResponseWriter, r *http.Request, service AssuranceAdminService, scope memory.Scope) {
	var req assuranceHealthEvaluationCreateRequest
	if !decodeJSONBody(w, r, &req) {
		return
	}
	observedAt, err := parseOptionalRFC3339(req.ObservedAt)
	if err != nil {
		http.Error(w, "invalid observed_at", http.StatusBadRequest)
		return
	}
	item, err := service.CreateHealthEvaluation(r.Context(), assurance.HealthEvaluationInput{
		Scope:                 scope,
		EvaluationID:          strings.TrimSpace(req.EvaluationID),
		ObservedAt:            observedAt,
		RuntimeReadiness:      normalizeHealthObservation(req.RuntimeReadiness),
		BacklogState:          normalizeHealthObservation(req.BacklogState),
		EmbeddingHealth:       normalizeHealthObservation(req.EmbeddingHealth),
		ProofSessionVerdict:   normalizeHealthObservation(req.ProofSessionVerdict),
		UsefulnessFeedback:    normalizeHealthObservation(req.UsefulnessFeedback),
		TaskEvaluationSummary: normalizeHealthObservation(req.TaskEvaluationSummary),
		RepairStatus:          normalizeHealthObservation(req.RepairStatus),
		RankingRolloutState:   normalizeHealthObservation(req.RankingRolloutState),
		ConformanceStatus:     normalizeHealthObservation(req.ConformanceStatus),
		WorkflowHealth:        normalizeHealthObservation(req.WorkflowHealth),
		CapacityLoadProof:     normalizeHealthObservation(req.CapacityLoadProof),
		BackupRestoreProof:    normalizeHealthObservation(req.BackupRestoreProof),
	})
	if err != nil {
		writeAdminAssuranceError(w, err, "failed to create health evaluation")
		return
	}
	writeJSON(w, http.StatusCreated, item)
}

func handleAdminAssuranceIncidentAction(w http.ResponseWriter, r *http.Request, service AssuranceAdminService, scope memory.Scope) {
	var req assuranceIncidentActionRequest
	if !decodeJSONBody(w, r, &req) {
		return
	}
	item, err := service.ApplyIncidentAction(r.Context(), assurance.IncidentActionInput{
		Scope:      scope,
		IncidentID: strings.TrimSpace(r.PathValue("incident_id")),
		Action:     assurance.IncidentAction(strings.TrimSpace(r.PathValue("incident_action"))),
		Actor:      strings.TrimSpace(req.Actor),
		Reason:     strings.TrimSpace(req.Reason),
		OccurredAt: time.Now().UTC(),
	})
	if err != nil {
		writeAdminAssuranceError(w, err, "failed to apply incident action")
		return
	}
	writeJSON(w, http.StatusAccepted, item)
}

func handleAdminAssuranceConformanceProfileCreate(w http.ResponseWriter, r *http.Request, service AssuranceAdminService, scope memory.Scope) {
	var req assuranceConformanceProfileRequest
	if !decodeJSONBody(w, r, &req) {
		return
	}
	expected, err := expectedEvidenceFromRequest(req.ExpectedEvidence)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	now := time.Now().UTC()
	item, err := service.CreateConformanceProfile(r.Context(), assurance.ConformanceProfile{
		ID:               strings.TrimSpace(req.ID),
		Scope:            scope,
		Status:           assurance.ConformanceProfileStatusActive,
		ExpectedEvidence: expected,
		Actor:            strings.TrimSpace(req.Actor),
		Reason:           strings.TrimSpace(req.Reason),
		CreatedAt:        now,
		UpdatedAt:        now,
	})
	if err != nil {
		writeAdminAssuranceError(w, err, "failed to create conformance profile")
		return
	}
	writeJSON(w, http.StatusCreated, item)
}

func handleAdminAssuranceConformanceProfileUpdate(w http.ResponseWriter, r *http.Request, service AssuranceAdminService, scope memory.Scope) {
	var req assuranceConformanceProfileRequest
	if !decodeJSONBody(w, r, &req) {
		return
	}
	expected, err := expectedEvidenceFromRequest(req.ExpectedEvidence)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	item, err := service.UpdateConformanceProfile(r.Context(), assurance.UpdateConformanceProfileInput{
		Scope:            scope,
		ProfileID:        strings.TrimSpace(r.PathValue("conformance_profile_id")),
		ExpectedEvidence: expected,
		Actor:            strings.TrimSpace(req.Actor),
		Reason:           strings.TrimSpace(req.Reason),
		UpdatedAt:        time.Now().UTC(),
	})
	if err != nil {
		writeAdminAssuranceError(w, err, "failed to update conformance profile")
		return
	}
	writeJSON(w, http.StatusOK, item)
}

func handleAdminAssuranceConformanceProfileDisable(w http.ResponseWriter, r *http.Request, service AssuranceAdminService, scope memory.Scope) {
	var req assuranceIncidentActionRequest
	if !decodeJSONBody(w, r, &req) {
		return
	}
	item, err := service.DisableConformanceProfile(r.Context(), assurance.DisableConformanceProfileInput{
		Scope:      scope,
		ProfileID:  strings.TrimSpace(r.PathValue("conformance_profile_id")),
		Actor:      strings.TrimSpace(req.Actor),
		Reason:     strings.TrimSpace(req.Reason),
		DisabledAt: time.Now().UTC(),
	})
	if err != nil {
		writeAdminAssuranceError(w, err, "failed to disable conformance profile")
		return
	}
	writeJSON(w, http.StatusAccepted, item)
}

func handleAdminAssuranceConformanceRunCreate(w http.ResponseWriter, r *http.Request, service AssuranceAdminService, scope memory.Scope) {
	var req assuranceConformanceRunCreateRequest
	if !decodeJSONBody(w, r, &req) {
		return
	}
	startedAt, err := parseOptionalRFC3339(req.StartedAt)
	if err != nil {
		http.Error(w, "invalid started_at", http.StatusBadRequest)
		return
	}
	if startedAt.IsZero() {
		startedAt = time.Now().UTC()
	}
	run, diagnostics, err := service.RunConformance(r.Context(), assurance.ConformanceRunInput{
		Scope:     scope,
		ProfileID: strings.TrimSpace(req.ProfileID),
		RunID:     strings.TrimSpace(req.RunID),
		StartedAt: startedAt,
	})
	if err != nil {
		writeAdminAssuranceError(w, err, "failed to create conformance run")
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{"run": run, "diagnostics": diagnostics})
}

func handleAdminAssuranceReadinessCreate(w http.ResponseWriter, r *http.Request, service AssuranceAdminService, scope memory.Scope) {
	var req assuranceReadinessReportCreateRequest
	if !decodeJSONBody(w, r, &req) {
		return
	}
	generatedAt, err := parseOptionalRFC3339(req.GeneratedAt)
	if err != nil {
		http.Error(w, "invalid generated_at", http.StatusBadRequest)
		return
	}
	if generatedAt.IsZero() {
		generatedAt = time.Now().UTC()
	}
	item, err := service.CreateReadinessReport(r.Context(), assurance.ReadinessReportInput{
		Scope:       scope,
		ReportID:    strings.TrimSpace(req.ReportID),
		GeneratedAt: generatedAt,
	})
	if err != nil {
		writeAdminAssuranceError(w, err, "failed to create readiness report")
		return
	}
	writeJSON(w, http.StatusCreated, item)
}

func handleAdminAssuranceRecoveryCreate(w http.ResponseWriter, r *http.Request, service AssuranceAdminService, scope memory.Scope) {
	var req assuranceRecoveryVerificationCreateRequest
	if !decodeJSONBody(w, r, &req) {
		return
	}
	verifiedAt, err := parseOptionalRFC3339(req.VerifiedAt)
	if err != nil {
		http.Error(w, "invalid verified_at", http.StatusBadRequest)
		return
	}
	item, err := service.CreateRecoveryVerification(r.Context(), assurance.RecoveryVerificationInput{
		Scope:           scope,
		RecordID:        strings.TrimSpace(req.RecordID),
		Target:          req.Target,
		TargetID:        strings.TrimSpace(req.TargetID),
		Status:          req.Status,
		CheckedSurfaces: append([]string(nil), req.CheckedSurfaces...),
		ResultCategory:  strings.TrimSpace(req.ResultCategory),
		LinkedEvidence:  req.LinkedEvidence,
		Actor:           strings.TrimSpace(req.Actor),
		Reason:          strings.TrimSpace(req.Reason),
		VerifiedAt:      verifiedAt,
	})
	if err != nil {
		writeAdminAssuranceError(w, err, "failed to create recovery verification")
		return
	}
	writeJSON(w, http.StatusCreated, item)
}

func expectedEvidenceFromRequest(input []assuranceExpectedEvidenceDTO) ([]assurance.ExpectedEvidence, error) {
	expected := make([]assurance.ExpectedEvidence, 0, len(input))
	for _, item := range input {
		window, err := time.ParseDuration(strings.TrimSpace(item.FreshnessWindow))
		if err != nil {
			return nil, fmt.Errorf("freshness_window is invalid")
		}
		expected = append(expected, assurance.ExpectedEvidence{
			Kind:            item.Kind,
			MinimumCount:    item.MinimumCount,
			FreshnessWindow: window,
		})
	}
	return expected, nil
}

func normalizeHealthObservation(input assurance.HealthObservation) assurance.HealthObservation {
	if input.ObservedAt.IsZero() {
		input.ObservedAt = time.Now().UTC()
	}
	if input.FreshThrough.IsZero() {
		input.FreshThrough = input.ObservedAt
	}
	return input
}

func parseOptionalRFC3339(raw string) (time.Time, error) {
	if strings.TrimSpace(raw) == "" {
		return time.Time{}, nil
	}
	return time.Parse(time.RFC3339, strings.TrimSpace(raw))
}

func parseOptionalLimit(r *http.Request, fallback int) (int, error) {
	raw := strings.TrimSpace(r.URL.Query().Get("limit"))
	if raw == "" {
		return fallback, nil
	}
	parsed, err := strconv.Atoi(raw)
	if err != nil || parsed < 0 {
		return 0, fmt.Errorf("invalid limit")
	}
	return parsed, nil
}

func handleAdminRankingRolloutList(w http.ResponseWriter, r *http.Request, service RankingRolloutAdminService, metrics MetricsRecorder, logger *log.Logger) {
	scope, ok := auth.ScopeFromContext(r.Context())
	if !ok {
		http.Error(w, "scope context is missing", http.StatusInternalServerError)
		return
	}
	items, err := service.ListRankingRolloutPolicies(r.Context(), memory.ListRankingRolloutPoliciesInput{Scope: scope})
	if err != nil {
		recordRankingRolloutMetric(r.Context(), metrics, "list", "error", "", "", "", "", "")
		recordRankingRolloutLog(logger, "list", "error", "", "", "", "", "")
		http.Error(w, "failed to list ranking rollout policies", http.StatusInternalServerError)
		return
	}
	recordRankingRolloutMetric(r.Context(), metrics, "list", "ok", "", "", "", "", "")
	recordRankingRolloutLog(logger, "list", "ok", "", "", "", "", "")
	writeJSON(w, http.StatusOK, map[string]any{"policies": items})
}

func handleAdminRankingRolloutCreate(w http.ResponseWriter, r *http.Request, service RankingRolloutAdminService, metrics MetricsRecorder, logger *log.Logger) {
	scope, ok := auth.ScopeFromContext(r.Context())
	if !ok {
		http.Error(w, "scope context is missing", http.StatusInternalServerError)
		return
	}
	var req rankingRolloutPolicyCreateRequest
	if !decodeJSONBody(w, r, &req) {
		return
	}
	policy := memory.RankingRolloutPolicy{
		ID:              strings.TrimSpace(req.ID),
		Scope:           scope,
		Status:          req.Status,
		Mode:            req.Mode,
		Surfaces:        req.Surfaces,
		SignalSources:   req.SignalSources,
		ThresholdStatus: req.ThresholdStatus,
		EvidenceMinimum: req.EvidenceMinimum,
		Actor:           strings.TrimSpace(req.Actor),
		Reason:          strings.TrimSpace(req.Reason),
		CreatedAt:       time.Now().UTC(),
		UpdatedAt:       time.Now().UTC(),
	}
	created, err := service.CreateRankingRolloutPolicy(r.Context(), policy)
	if err != nil {
		recordRankingRolloutMetric(r.Context(), metrics, "create", "rejected", firstRankingRolloutSurface(policy.Surfaces), firstRankingRolloutSignalSource(policy.SignalSources), policy.ThresholdStatus, policy.Status, "")
		recordRankingRolloutLog(logger, "create", "rejected", firstRankingRolloutSurface(policy.Surfaces), firstRankingRolloutSignalSource(policy.SignalSources), policy.ThresholdStatus, policy.Status, "")
		http.Error(w, "failed to create ranking rollout policy", http.StatusBadRequest)
		return
	}
	recordRankingRolloutMetric(r.Context(), metrics, "create", "ok", firstRankingRolloutSurface(created.Surfaces), firstRankingRolloutSignalSource(created.SignalSources), created.ThresholdStatus, created.Status, "")
	recordRankingRolloutLog(logger, "create", "ok", firstRankingRolloutSurface(created.Surfaces), firstRankingRolloutSignalSource(created.SignalSources), created.ThresholdStatus, created.Status, "")
	writeJSON(w, http.StatusCreated, created)
}

func handleAdminRankingRolloutDetail(w http.ResponseWriter, r *http.Request, service RankingRolloutAdminService, metrics MetricsRecorder, logger *log.Logger) {
	scope, ok := auth.ScopeFromContext(r.Context())
	if !ok {
		http.Error(w, "scope context is missing", http.StatusInternalServerError)
		return
	}
	policy, err := service.ReadRankingRolloutPolicy(r.Context(), memory.ReadRankingRolloutPolicyInput{
		Scope:    scope,
		PolicyID: strings.TrimSpace(r.PathValue("policy_id")),
	})
	if err != nil {
		recordRankingRolloutMetric(r.Context(), metrics, "read", "error", "", "", "", "", "")
		recordRankingRolloutLog(logger, "read", "error", "", "", "", "", "")
		http.Error(w, "failed to read ranking rollout policy", http.StatusNotFound)
		return
	}
	recordRankingRolloutMetric(r.Context(), metrics, "read", "ok", firstRankingRolloutSurface(policy.Surfaces), firstRankingRolloutSignalSource(policy.SignalSources), policy.ThresholdStatus, policy.Status, "")
	recordRankingRolloutLog(logger, "read", "ok", firstRankingRolloutSurface(policy.Surfaces), firstRankingRolloutSignalSource(policy.SignalSources), policy.ThresholdStatus, policy.Status, "")
	writeJSON(w, http.StatusOK, policy)
}

func handleAdminRankingRolloutImpact(w http.ResponseWriter, r *http.Request, service RankingRolloutAdminService, metrics MetricsRecorder, logger *log.Logger) {
	scope, ok := auth.ScopeFromContext(r.Context())
	if !ok {
		http.Error(w, "scope context is missing", http.StatusInternalServerError)
		return
	}
	items, err := service.ListRankingRolloutPolicyImpact(r.Context(), memory.ListRankingRolloutPolicyImpactInput{
		Scope:    scope,
		PolicyID: strings.TrimSpace(r.PathValue("policy_id")),
	})
	if err != nil {
		recordRankingRolloutMetric(r.Context(), metrics, "impact", "error", "", "", "", "", "")
		recordRankingRolloutLog(logger, "impact", "error", "", "", "", "", "")
		http.Error(w, "failed to read ranking rollout impact", http.StatusInternalServerError)
		return
	}
	reasonCode := ""
	if len(items) > 0 {
		reasonCode = string(items[0].ReasonCode)
	}
	recordRankingRolloutMetric(r.Context(), metrics, "impact", "ok", "", "", "", "", reasonCode)
	recordRankingRolloutLog(logger, "impact", "ok", "", "", "", "", reasonCode)
	writeJSON(w, http.StatusOK, map[string]any{"items": items})
}

func handleAdminRankingRolloutAction(w http.ResponseWriter, r *http.Request, service RankingRolloutAdminService, action string, metrics MetricsRecorder, logger *log.Logger) {
	scope, ok := auth.ScopeFromContext(r.Context())
	if !ok {
		http.Error(w, "scope context is missing", http.StatusInternalServerError)
		return
	}
	var req rankingRolloutPolicyActionRequest
	if !decodeJSONBody(w, r, &req) {
		return
	}
	policyID := strings.TrimSpace(r.PathValue("policy_id"))
	switch action {
	case "activate":
		policy, err := service.ActivateRankingRolloutPolicy(r.Context(), memory.ActivateRankingRolloutPolicyInput{
			Scope:       scope,
			PolicyID:    policyID,
			Actor:       strings.TrimSpace(req.Actor),
			Reason:      strings.TrimSpace(req.Reason),
			ActivatedAt: time.Now().UTC(),
			Gate: memory.RankingRolloutActivationGate{
				DryRunSucceeded:         true,
				EvidenceThresholdStatus: memory.RankingRolloutThresholdStatusSatisfied,
				AttributionRecorded:     true,
			},
		})
		if err != nil {
			recordRankingRolloutMetric(r.Context(), metrics, "activate", "rejected", "", "", "", "", "")
			recordRankingRolloutLog(logger, "activate", "rejected", "", "", "", "", "")
			http.Error(w, "failed to activate ranking rollout policy", http.StatusBadRequest)
			return
		}
		recordRankingRolloutMetric(r.Context(), metrics, "activate", "ok", firstRankingRolloutSurface(policy.Surfaces), firstRankingRolloutSignalSource(policy.SignalSources), policy.ThresholdStatus, policy.Status, "")
		recordRankingRolloutLog(logger, "activate", "ok", firstRankingRolloutSurface(policy.Surfaces), firstRankingRolloutSignalSource(policy.SignalSources), policy.ThresholdStatus, policy.Status, "")
	case "disable":
		policy, err := service.DisableRankingRolloutPolicy(r.Context(), memory.DisableRankingRolloutPolicyInput{
			Scope:      scope,
			PolicyID:   policyID,
			Actor:      strings.TrimSpace(req.Actor),
			Reason:     strings.TrimSpace(req.Reason),
			DisabledAt: time.Now().UTC(),
		})
		if err != nil {
			recordRankingRolloutMetric(r.Context(), metrics, "disable", "rejected", "", "", "", "", "")
			recordRankingRolloutLog(logger, "disable", "rejected", "", "", "", "", "")
			http.Error(w, "failed to disable ranking rollout policy", http.StatusBadRequest)
			return
		}
		recordRankingRolloutMetric(r.Context(), metrics, "disable", "ok", firstRankingRolloutSurface(policy.Surfaces), firstRankingRolloutSignalSource(policy.SignalSources), policy.ThresholdStatus, policy.Status, "")
		recordRankingRolloutLog(logger, "disable", "ok", firstRankingRolloutSurface(policy.Surfaces), firstRankingRolloutSignalSource(policy.SignalSources), policy.ThresholdStatus, policy.Status, "")
	case "rollback":
		policy, err := service.RollbackRankingRolloutPolicy(r.Context(), memory.RollbackRankingRolloutPolicyInput{
			Scope:        scope,
			PolicyID:     policyID,
			Actor:        strings.TrimSpace(req.Actor),
			Reason:       strings.TrimSpace(req.Reason),
			RolledBackAt: time.Now().UTC(),
		})
		if err != nil {
			recordRankingRolloutMetric(r.Context(), metrics, "rollback", "rejected", "", "", "", "", "")
			recordRankingRolloutLog(logger, "rollback", "rejected", "", "", "", "", "")
			http.Error(w, "failed to rollback ranking rollout policy", http.StatusBadRequest)
			return
		}
		recordRankingRolloutMetric(r.Context(), metrics, "rollback", "ok", firstRankingRolloutSurface(policy.Surfaces), firstRankingRolloutSignalSource(policy.SignalSources), policy.ThresholdStatus, policy.Status, string(memory.RankingRolloutImpactReasonCodeRollbackRestored))
		recordRankingRolloutLog(logger, "rollback", "ok", firstRankingRolloutSurface(policy.Surfaces), firstRankingRolloutSignalSource(policy.SignalSources), policy.ThresholdStatus, policy.Status, string(memory.RankingRolloutImpactReasonCodeRollbackRestored))
	default:
		http.Error(w, "invalid ranking rollout action", http.StatusBadRequest)
		return
	}
	writeJSON(w, http.StatusAccepted, map[string]string{"status": "accepted"})
}

func handleAdminRankingRolloutDryRun(w http.ResponseWriter, r *http.Request, service RankingRolloutAdminService, metrics MetricsRecorder, logger *log.Logger) {
	scope, ok := auth.ScopeFromContext(r.Context())
	if !ok {
		http.Error(w, "scope context is missing", http.StatusInternalServerError)
		return
	}
	policyID := strings.TrimSpace(r.PathValue("policy_id"))
	dryRun, err := service.RecordRankingRolloutDryRun(r.Context(), memory.RecordRankingRolloutDryRunInput{
		PolicyID:        policyID,
		Scope:           scope,
		Surface:         memory.RankingRolloutSurfaceSearch,
		SignalSource:    memory.RankingRolloutSignalSourceTaskEvaluations,
		ThresholdStatus: memory.RankingRolloutThresholdStatusSatisfied,
		CreatedAt:       time.Now().UTC(),
	})
	if err != nil {
		recordRankingRolloutMetric(r.Context(), metrics, "dry_run", "rejected", memory.RankingRolloutSurfaceSearch, memory.RankingRolloutSignalSourceTaskEvaluations, "", "", "")
		recordRankingRolloutLog(logger, "dry_run", "rejected", memory.RankingRolloutSurfaceSearch, memory.RankingRolloutSignalSourceTaskEvaluations, "", "", "")
		http.Error(w, "failed to record ranking rollout dry run", http.StatusBadRequest)
		return
	}
	reasonCode := ""
	if len(dryRun.ReasonCodes) > 0 {
		reasonCode = string(dryRun.ReasonCodes[0])
	}
	recordRankingRolloutMetric(r.Context(), metrics, "dry_run", "ok", dryRun.Surface, dryRun.SignalSource, dryRun.ThresholdStatus, memory.RankingRolloutPolicyStatusDryRun, reasonCode)
	recordRankingRolloutLog(logger, "dry_run", "ok", dryRun.Surface, dryRun.SignalSource, dryRun.ThresholdStatus, memory.RankingRolloutPolicyStatusDryRun, reasonCode)
	writeJSON(w, http.StatusOK, dryRun)
}

func handleAdminTaskEvaluationList(w http.ResponseWriter, r *http.Request, service TaskEvaluationService, metrics MetricsRecorder, logger *log.Logger) {
	scope, ok := auth.ScopeFromContext(r.Context())
	if !ok {
		http.Error(w, "scope context is missing", http.StatusInternalServerError)
		return
	}
	limit := 50
	if raw := strings.TrimSpace(r.URL.Query().Get("limit")); raw != "" {
		parsed, err := strconv.Atoi(raw)
		if err != nil || parsed < 0 {
			http.Error(w, "invalid limit", http.StatusBadRequest)
			return
		}
		limit = parsed
	}
	input := memory.ListTaskEvaluationsInput{
		Scope:                scope,
		Verdict:              memory.TaskEvaluationVerdict(strings.TrimSpace(r.URL.Query().Get("verdict"))),
		ContributionCategory: memory.TaskContributionCategory(strings.TrimSpace(r.URL.Query().Get("contribution_category"))),
		EvidenceTargetKind:   memory.TaskEvidenceTargetKind(strings.TrimSpace(r.URL.Query().Get("evidence_target_kind"))),
		EvidenceTargetID:     strings.TrimSpace(r.URL.Query().Get("evidence_target_id")),
		IncludeSuperseded:    parseBoolQueryValue(r.URL.Query().Get("include_superseded")),
		Limit:                limit,
	}
	items, err := service.ListTaskEvaluations(r.Context(), input)
	if err != nil {
		recordTaskEvaluationMetric(r.Context(), metrics, "list", "error", input.Verdict, input.ContributionCategory, "")
		recordTaskEvaluationLog(logger, "list", "error", input.Verdict, input.ContributionCategory, "")
		writeTaskEvaluationError(w, err, "failed to list task evaluations")
		return
	}
	recordTaskEvaluationMetric(r.Context(), metrics, "list", "ok", input.Verdict, input.ContributionCategory, "")
	recordTaskEvaluationLog(logger, "list", "ok", input.Verdict, input.ContributionCategory, "")
	writeJSON(w, http.StatusOK, map[string]any{"task_evaluations": items})
}

func handleAdminTaskEvaluationDetail(w http.ResponseWriter, r *http.Request, service TaskEvaluationService, metrics MetricsRecorder, logger *log.Logger) {
	scope, ok := auth.ScopeFromContext(r.Context())
	if !ok {
		http.Error(w, "scope context is missing", http.StatusInternalServerError)
		return
	}
	evaluation, err := service.ReadTaskEvaluation(r.Context(), memory.ReadTaskEvaluationInput{
		Scope:        scope,
		EvaluationID: strings.TrimSpace(r.PathValue("evaluation_id")),
	})
	if err != nil {
		recordTaskEvaluationMetric(r.Context(), metrics, "read", "error", "", "", "")
		recordTaskEvaluationLog(logger, "read", "error", "", "", "")
		writeTaskEvaluationError(w, err, "failed to read task evaluation")
		return
	}
	recordTaskEvaluationMetric(r.Context(), metrics, "read", "ok", evaluation.Verdict, firstTaskContributionCategory(evaluation.ContributionCategories), evaluation.CorrectionState)
	recordTaskEvaluationLog(logger, "read", "ok", evaluation.Verdict, firstTaskContributionCategory(evaluation.ContributionCategories), evaluation.CorrectionState)
	writeJSON(w, http.StatusOK, evaluation)
}

func handleAdminTaskEvaluationSummary(w http.ResponseWriter, r *http.Request, service TaskEvaluationService, metrics MetricsRecorder, logger *log.Logger) {
	scope, ok := auth.ScopeFromContext(r.Context())
	if !ok {
		http.Error(w, "scope context is missing", http.StatusInternalServerError)
		return
	}
	summary, err := service.SummarizeTaskEvaluations(r.Context(), memory.SummarizeTaskEvaluationsInput{
		Scope:              scope,
		EvidenceTargetKind: memory.TaskEvidenceTargetKind(strings.TrimSpace(r.URL.Query().Get("evidence_target_kind"))),
		EvidenceTargetID:   strings.TrimSpace(r.URL.Query().Get("evidence_target_id")),
	})
	if err != nil {
		recordTaskEvaluationMetric(r.Context(), metrics, "summary", "error", "", "", "")
		recordTaskEvaluationLog(logger, "summary", "error", "", "", "")
		writeTaskEvaluationError(w, err, "failed to summarize task evaluations")
		return
	}
	recordTaskEvaluationMetric(r.Context(), metrics, "summary", "ok", "", "", "")
	recordTaskEvaluationLog(logger, "summary", "ok", "", "", "")
	writeJSON(w, http.StatusOK, summary)
}

func handleAdminTaskEvaluationAction(w http.ResponseWriter, r *http.Request, service TaskEvaluationService, metrics MetricsRecorder, logger *log.Logger) {
	scope, ok := auth.ScopeFromContext(r.Context())
	if !ok {
		http.Error(w, "scope context is missing", http.StatusInternalServerError)
		return
	}
	evaluationID := strings.TrimSpace(r.PathValue("evaluation_id"))
	if strings.TrimSpace(evaluationID) == "" {
		http.Error(w, "invalid task evaluation action target", http.StatusBadRequest)
		return
	}
	var req taskEvaluationSupersedeRequest
	if !decodeJSONBody(w, r, &req) {
		return
	}
	if err := service.SupersedeTaskEvaluation(r.Context(), memory.SupersedeTaskEvaluationInput{
		Scope:        scope,
		EvaluationID: strings.TrimSpace(evaluationID),
		Actor:        strings.TrimSpace(req.Actor),
		Reason:       strings.TrimSpace(req.Reason),
		SupersededAt: time.Now().UTC(),
	}); err != nil {
		recordTaskEvaluationMetric(r.Context(), metrics, "supersede", "rejected", "", "", memory.TaskEvaluationCorrectionStateSuperseded)
		recordTaskEvaluationLog(logger, "supersede", "rejected", "", "", memory.TaskEvaluationCorrectionStateSuperseded)
		writeTaskEvaluationError(w, err, "failed to supersede task evaluation")
		return
	}
	recordTaskEvaluationMetric(r.Context(), metrics, "supersede", "ok", "", "", memory.TaskEvaluationCorrectionStateSuperseded)
	recordTaskEvaluationLog(logger, "supersede", "ok", "", "", memory.TaskEvaluationCorrectionStateSuperseded)
	writeJSON(w, http.StatusAccepted, map[string]string{"status": "accepted"})
}

func parseBoolQueryValue(raw string) bool {
	value := strings.TrimSpace(raw)
	if value == "" {
		return false
	}
	parsed, err := strconv.ParseBool(value)
	if err != nil {
		return false
	}
	return parsed
}

func handleAdminUsefulnessFeedbackList(w http.ResponseWriter, r *http.Request, service UsefulnessFeedbackService) {
	if service == nil {
		http.Error(w, "usefulness feedback service is not configured", http.StatusServiceUnavailable)
		return
	}
	scope, ok := auth.ScopeFromContext(r.Context())
	if !ok {
		http.Error(w, "scope context is missing", http.StatusInternalServerError)
		return
	}
	limit := 50
	if raw := strings.TrimSpace(r.URL.Query().Get("limit")); raw != "" {
		parsed, err := strconv.Atoi(raw)
		if err != nil || parsed < 0 {
			http.Error(w, "invalid limit", http.StatusBadRequest)
			return
		}
		limit = parsed
	}
	includeSuperseded, err := parseOptionalBoolQuery(r.URL.Query().Get("include_superseded"))
	if err != nil {
		http.Error(w, "invalid include_superseded", http.StatusBadRequest)
		return
	}
	var subject memory.UsefulnessFeedbackSubject
	if strings.TrimSpace(r.URL.Query().Get("subject_kind")) != "" {
		subject, err = usefulnessFeedbackSubjectFromQuery(r)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
	}
	items, err := service.ListUsefulnessFeedback(r.Context(), memory.ListUsefulnessFeedbackInput{
		Scope:             scope,
		Subject:           subject,
		Type:              memory.UsefulnessFeedbackType(strings.TrimSpace(r.URL.Query().Get("type"))),
		IncludeSuperseded: includeSuperseded,
		Limit:             limit,
	})
	if err != nil {
		writeUsefulnessFeedbackError(w, err, "failed to list usefulness feedback")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"feedback": items})
}

func handleAdminUsefulnessFeedbackDetail(w http.ResponseWriter, r *http.Request, service UsefulnessFeedbackService) {
	if service == nil {
		http.Error(w, "usefulness feedback service is not configured", http.StatusServiceUnavailable)
		return
	}
	scope, ok := auth.ScopeFromContext(r.Context())
	if !ok {
		http.Error(w, "scope context is missing", http.StatusInternalServerError)
		return
	}
	feedback, err := service.ReadUsefulnessFeedback(r.Context(), memory.ReadUsefulnessFeedbackInput{
		Scope:      scope,
		FeedbackID: strings.TrimSpace(r.PathValue("feedback_id")),
	})
	if err != nil {
		writeUsefulnessFeedbackError(w, err, "failed to read usefulness feedback")
		return
	}
	writeJSON(w, http.StatusOK, feedback)
}

func handleAdminUsefulnessFeedbackSummary(w http.ResponseWriter, r *http.Request, service UsefulnessFeedbackService, metrics MetricsRecorder, logger *log.Logger) {
	if service == nil {
		http.Error(w, "usefulness feedback service is not configured", http.StatusServiceUnavailable)
		return
	}
	scope, ok := auth.ScopeFromContext(r.Context())
	if !ok {
		http.Error(w, "scope context is missing", http.StatusInternalServerError)
		return
	}
	subject, err := usefulnessFeedbackSubjectFromQuery(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	summary, err := service.SummarizeUsefulnessFeedback(r.Context(), memory.SummarizeUsefulnessFeedbackInput{
		Scope:   scope,
		Subject: subject,
	})
	if err != nil {
		recordUsefulnessFeedbackMetric(r.Context(), metrics, "summary", "error", "", subject.Kind, memory.UsefulnessFeedbackSourceAdmin, "active")
		recordUsefulnessFeedbackLog(logger, "summary", "error", "", subject.Kind, memory.UsefulnessFeedbackSourceAdmin, "active")
		writeUsefulnessFeedbackError(w, err, "failed to summarize usefulness feedback")
		return
	}
	recordUsefulnessFeedbackMetric(r.Context(), metrics, "summary", "ok", "", subject.Kind, memory.UsefulnessFeedbackSourceAdmin, string(summary.EffectiveQuality))
	recordUsefulnessFeedbackLog(logger, "summary", "ok", "", subject.Kind, memory.UsefulnessFeedbackSourceAdmin, string(summary.EffectiveQuality))
	writeJSON(w, http.StatusOK, summary)
}

func handleAdminUsefulnessFeedbackAction(w http.ResponseWriter, r *http.Request, service UsefulnessFeedbackService, metrics MetricsRecorder, logger *log.Logger) {
	if service == nil {
		http.Error(w, "usefulness feedback service is not configured", http.StatusServiceUnavailable)
		return
	}
	scope, ok := auth.ScopeFromContext(r.Context())
	if !ok {
		http.Error(w, "scope context is missing", http.StatusInternalServerError)
		return
	}
	feedbackID, action, err := parseUsefulnessFeedbackActionTarget(r.PathValue("feedback_action"))
	if err != nil || action != "supersede" {
		http.Error(w, "invalid usefulness feedback action target", http.StatusBadRequest)
		return
	}
	var req usefulnessFeedbackSupersedeRequest
	if !decodeJSONBody(w, r, &req) {
		return
	}
	if err := service.SupersedeUsefulnessFeedback(r.Context(), memory.SupersedeUsefulnessFeedbackInput{
		Scope:        scope,
		FeedbackID:   feedbackID,
		Actor:        strings.TrimSpace(req.Actor),
		Reason:       strings.TrimSpace(req.Reason),
		SupersededAt: time.Now().UTC(),
	}); err != nil {
		recordUsefulnessFeedbackMetric(r.Context(), metrics, "supersede", "rejected", "", "", memory.UsefulnessFeedbackSourceAdmin, "superseded")
		recordUsefulnessFeedbackLog(logger, "supersede", "rejected", "", "", memory.UsefulnessFeedbackSourceAdmin, "superseded")
		writeUsefulnessFeedbackError(w, err, "failed to supersede usefulness feedback")
		return
	}
	recordUsefulnessFeedbackMetric(r.Context(), metrics, "supersede", "ok", "", "", memory.UsefulnessFeedbackSourceAdmin, "superseded")
	recordUsefulnessFeedbackLog(logger, "supersede", "ok", "", "", memory.UsefulnessFeedbackSourceAdmin, "superseded")
	writeJSON(w, http.StatusAccepted, map[string]string{"status": "accepted"})
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

func handleAdminScopeProofs(w http.ResponseWriter, r *http.Request, service ScopeProofAdminService) {
	if service == nil {
		http.Error(w, "scope proof admin service is not configured", http.StatusServiceUnavailable)
		return
	}
	scope, ok := auth.ScopeFromContext(r.Context())
	if !ok {
		http.Error(w, "scope context is missing", http.StatusInternalServerError)
		return
	}
	switch r.Method {
	case http.MethodPost:
		var req scopeProofCreateRequest
		if !decodeJSONBody(w, r, &req) {
			return
		}
		run, err := service.CreateProofRun(r.Context(), memory.CreateScopeProofRunInput{
			Scope:       scope,
			Checks:      req.Checks,
			FixtureMode: req.FixtureMode,
			Actor:       strings.TrimSpace(req.Actor),
			Reason:      strings.TrimSpace(req.Reason),
		})
		if err != nil {
			writeAdminScopeProofError(w, err, "failed to create scope proof")
			return
		}
		writeJSON(w, http.StatusCreated, run)
	case http.MethodGet:
		limit := 20
		if raw := strings.TrimSpace(r.URL.Query().Get("limit")); raw != "" {
			parsed, err := strconv.Atoi(raw)
			if err != nil || parsed < 0 {
				http.Error(w, "invalid limit", http.StatusBadRequest)
				return
			}
			limit = parsed
		}
		runs, err := service.ListProofRuns(r.Context(), memory.ListScopeProofRunsInput{Scope: scope, Limit: limit})
		if err != nil {
			writeAdminScopeProofError(w, err, "failed to list scope proofs")
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"proof_runs": runs})
	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

func handleAdminScopeProofDetail(w http.ResponseWriter, r *http.Request, service ScopeProofAdminService) {
	if service == nil {
		http.Error(w, "scope proof admin service is not configured", http.StatusServiceUnavailable)
		return
	}
	scope, ok := auth.ScopeFromContext(r.Context())
	if !ok {
		http.Error(w, "scope context is missing", http.StatusInternalServerError)
		return
	}
	run, err := service.ReadProofRun(r.Context(), memory.ReadScopeProofRunInput{
		Scope:   scope,
		ProofID: strings.TrimSpace(r.PathValue("proof_run_id")),
	})
	if err != nil {
		writeAdminScopeProofError(w, err, "failed to read scope proof")
		return
	}
	writeJSON(w, http.StatusOK, run)
}

func handleAdminScopeProofReport(w http.ResponseWriter, r *http.Request, service ScopeProofAdminService) {
	if service == nil {
		http.Error(w, "scope proof admin service is not configured", http.StatusServiceUnavailable)
		return
	}
	scope, ok := auth.ScopeFromContext(r.Context())
	if !ok {
		http.Error(w, "scope context is missing", http.StatusInternalServerError)
		return
	}
	report, err := service.ReadProofReport(r.Context(), memory.ReadScopeProofRunInput{
		Scope:   scope,
		ProofID: strings.TrimSpace(r.PathValue("proof_run_id")),
	})
	if err != nil {
		writeAdminScopeProofError(w, err, "failed to read scope proof report")
		return
	}
	writeJSON(w, http.StatusOK, report)
}

func handleAdminScopeProofAction(w http.ResponseWriter, r *http.Request, service ScopeProofAdminService) {
	if service == nil {
		http.Error(w, "scope proof admin service is not configured", http.StatusServiceUnavailable)
		return
	}
	scope, ok := auth.ScopeFromContext(r.Context())
	if !ok {
		http.Error(w, "scope context is missing", http.StatusInternalServerError)
		return
	}
	proofID, actionName, ok := strings.Cut(r.PathValue("proof_run_action"), ":")
	if !ok || strings.TrimSpace(proofID) == "" || actionName != "rerun" {
		http.Error(w, "invalid scope proof action target", http.StatusBadRequest)
		return
	}
	var req scopeProofRerunRequest
	if !decodeJSONBody(w, r, &req) {
		return
	}
	run, err := service.RerunProofRun(r.Context(), memory.RerunScopeProofRunInput{
		Scope:   scope,
		ProofID: strings.TrimSpace(proofID),
		Actor:   strings.TrimSpace(req.Actor),
		Reason:  strings.TrimSpace(req.Reason),
	})
	if err != nil {
		writeAdminScopeProofError(w, err, "failed to rerun scope proof")
		return
	}
	writeJSON(w, http.StatusCreated, run)
}

func handleAdminQualityEvaluations(w http.ResponseWriter, r *http.Request, service QualityAdminService) {
	if service == nil {
		http.Error(w, "quality admin service is not configured", http.StatusServiceUnavailable)
		return
	}
	scope, ok := auth.ScopeFromContext(r.Context())
	if !ok {
		http.Error(w, "scope context is missing", http.StatusInternalServerError)
		return
	}
	switch r.Method {
	case http.MethodPost:
		var req qualityEvaluationCreateRequest
		if !decodeJSONBody(w, r, &req) {
			return
		}
		run, err := service.CreateEvaluation(r.Context(), memory.CreateQualityEvaluationInput{
			Scope:             scope,
			Checks:            req.Checks,
			Query:             strings.TrimSpace(req.Query),
			ExpectedMemoryIDs: append([]string(nil), req.ExpectedMemoryIDs...),
			ContextBudget:     req.ContextBudget,
			Actor:             strings.TrimSpace(req.Actor),
			Reason:            strings.TrimSpace(req.Reason),
		})
		if err != nil {
			writeAdminQualityError(w, err, "failed to create quality evaluation")
			return
		}
		writeJSON(w, http.StatusCreated, run)
	case http.MethodGet:
		run, err := service.ReadEvaluation(r.Context(), memory.ReadQualityEvaluationRunInput{
			Scope:           scope,
			EvaluationRunID: strings.TrimSpace(r.PathValue("evaluation_run_id")),
		})
		if err != nil {
			writeAdminQualityError(w, err, "failed to read quality evaluation")
			return
		}
		writeJSON(w, http.StatusOK, run)
	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

func handleAdminQualityEvaluationFindings(w http.ResponseWriter, r *http.Request, service QualityAdminService) {
	if service == nil {
		http.Error(w, "quality admin service is not configured", http.StatusServiceUnavailable)
		return
	}
	scope, ok := auth.ScopeFromContext(r.Context())
	if !ok {
		http.Error(w, "scope context is missing", http.StatusInternalServerError)
		return
	}
	findings, err := service.ListEvaluationFindings(r.Context(), memory.ListQualityEvaluationFindingsInput{
		Scope:           scope,
		EvaluationRunID: strings.TrimSpace(r.PathValue("evaluation_run_id")),
	})
	if err != nil {
		writeAdminQualityError(w, err, "failed to read quality evaluation findings")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"findings": findings})
}

func handleAdminRepairPlans(w http.ResponseWriter, r *http.Request, service QualityAdminService) {
	if service == nil {
		http.Error(w, "quality admin service is not configured", http.StatusServiceUnavailable)
		return
	}
	scope, ok := auth.ScopeFromContext(r.Context())
	if !ok {
		http.Error(w, "scope context is missing", http.StatusInternalServerError)
		return
	}
	switch r.Method {
	case http.MethodPost:
		var req repairPlanCreateRequest
		if !decodeJSONBody(w, r, &req) {
			return
		}
		plan, err := service.CreateRepairPlan(r.Context(), memory.CreateRepairPlanInput{
			Scope:           scope,
			EvaluationRunID: strings.TrimSpace(req.EvaluationRunID),
			Actor:           strings.TrimSpace(req.Actor),
			Reason:          strings.TrimSpace(req.Reason),
			DryRun:          req.DryRun,
		})
		if err != nil {
			writeAdminQualityError(w, err, "failed to create repair plan")
			return
		}
		writeJSON(w, http.StatusCreated, plan)
	case http.MethodGet:
		plan, err := service.ReadRepairPlan(r.Context(), memory.ReadRepairPlanInput{
			Scope:        scope,
			RepairPlanID: strings.TrimSpace(r.PathValue("repair_plan_id")),
		})
		if err != nil {
			writeAdminQualityError(w, err, "failed to read repair plan")
			return
		}
		writeJSON(w, http.StatusOK, plan)
	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

func handleAdminRepairPlanAction(w http.ResponseWriter, r *http.Request, service QualityAdminService) {
	if service == nil {
		http.Error(w, "quality admin service is not configured", http.StatusServiceUnavailable)
		return
	}
	scope, ok := auth.ScopeFromContext(r.Context())
	if !ok {
		http.Error(w, "scope context is missing", http.StatusInternalServerError)
		return
	}
	planID, actionName, ok := strings.Cut(r.PathValue("repair_plan_action"), ":")
	if !ok || strings.TrimSpace(planID) == "" || (actionName != "approve" && actionName != "verify") {
		http.Error(w, "invalid repair plan action target", http.StatusBadRequest)
		return
	}
	if actionName == "verify" {
		var req repairPlanVerifyRequest
		if !decodeJSONBody(w, r, &req) {
			return
		}
		plan, err := service.VerifyRepairPlan(r.Context(), memory.VerifyRepairPlanInput{
			Scope:        scope,
			RepairPlanID: strings.TrimSpace(planID),
			Checks:       req.Checks,
			Actor:        strings.TrimSpace(req.Actor),
			Reason:       strings.TrimSpace(req.Reason),
		})
		if err != nil {
			writeAdminQualityError(w, err, "failed to verify repair plan")
			return
		}
		writeJSON(w, http.StatusOK, plan)
		return
	}
	var req repairPlanApproveRequest
	if !decodeJSONBody(w, r, &req) {
		return
	}
	plan, err := service.ApproveRepairPlan(r.Context(), memory.ApproveRepairPlanInput{
		Scope:        scope,
		RepairPlanID: strings.TrimSpace(planID),
		Actor:        strings.TrimSpace(req.Actor),
		Reason:       strings.TrimSpace(req.Reason),
		ApprovedAt:   time.Now().UTC(),
	})
	if err != nil {
		writeAdminQualityError(w, err, "failed to approve repair plan")
		return
	}
	writeJSON(w, http.StatusOK, plan)
}

func handleAdminQualityDiagnostics(w http.ResponseWriter, r *http.Request, service QualityAdminService) {
	if service == nil {
		http.Error(w, "quality admin service is not configured", http.StatusServiceUnavailable)
		return
	}
	scope, ok := auth.ScopeFromContext(r.Context())
	if !ok {
		http.Error(w, "scope context is missing", http.StatusInternalServerError)
		return
	}
	diagnostics, err := service.ReadDiagnostics(r.Context(), memory.ReadQualityDiagnosticsInput{Scope: scope})
	if err != nil {
		writeAdminQualityError(w, err, "failed to read quality diagnostics")
		return
	}
	writeJSON(w, http.StatusOK, diagnostics)
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
		writeJSONDecodeError(w, err)
		return false
	}
	if decoder.More() {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return false
	}

	return true
}

func writeJSONDecodeError(w http.ResponseWriter, err error) {
	var maxBytesErr *http.MaxBytesError
	if errors.As(err, &maxBytesErr) {
		http.Error(w, "request body too large", http.StatusRequestEntityTooLarge)
		return
	}
	http.Error(w, "invalid request body", http.StatusBadRequest)
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
		if strings.Contains(err.Error(), "required") || strings.Contains(err.Error(), "invalid") || strings.Contains(err.Error(), "must be") || strings.Contains(err.Error(), "must not") {
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
		if strings.Contains(err.Error(), "required") || strings.Contains(err.Error(), "invalid") || strings.Contains(err.Error(), "must be") || strings.Contains(err.Error(), "must not") {
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

func writeAdminQualityError(w http.ResponseWriter, err error, fallback string) {
	switch {
	case errors.Is(err, memory.ErrRepairActionRejected):
		http.Error(w, err.Error(), http.StatusUnprocessableEntity)
	case errors.Is(err, pgx.ErrNoRows):
		http.Error(w, "quality resource not found", http.StatusNotFound)
	default:
		if strings.Contains(err.Error(), "required") || strings.Contains(err.Error(), "invalid") || strings.Contains(err.Error(), "must be") || strings.Contains(err.Error(), "exceeds") {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		http.Error(w, fallback, http.StatusInternalServerError)
	}
}

func writeAdminScopeProofError(w http.ResponseWriter, err error, fallback string) {
	switch {
	case errors.Is(err, pgx.ErrNoRows):
		http.Error(w, "scope proof resource not found", http.StatusNotFound)
	default:
		if strings.Contains(err.Error(), "required") || strings.Contains(err.Error(), "invalid") || strings.Contains(err.Error(), "must be") {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		http.Error(w, fallback, http.StatusInternalServerError)
	}
}

func writeAdminAssuranceError(w http.ResponseWriter, err error, fallback string) {
	switch {
	case errors.Is(err, pgx.ErrNoRows):
		http.Error(w, "assurance resource not found", http.StatusNotFound)
	default:
		if strings.Contains(err.Error(), "required") || strings.Contains(err.Error(), "invalid") || strings.Contains(err.Error(), "must be") || strings.Contains(err.Error(), "must not") || strings.Contains(err.Error(), "exceeds") || strings.Contains(err.Error(), "not active") {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		http.Error(w, fallback, http.StatusInternalServerError)
	}
}

func writeMemorySessionError(w http.ResponseWriter, err error, fallback string) {
	switch {
	case errors.Is(err, pgx.ErrNoRows):
		http.Error(w, "memory session resource not found", http.StatusNotFound)
	case strings.Contains(err.Error(), "not configured"):
		http.Error(w, err.Error(), http.StatusServiceUnavailable)
	default:
		if strings.Contains(err.Error(), "required") || strings.Contains(err.Error(), "invalid") || strings.Contains(err.Error(), "must be") {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		http.Error(w, fallback, http.StatusInternalServerError)
	}
}

func writeUsefulnessFeedbackError(w http.ResponseWriter, err error, fallback string) {
	switch {
	case errors.Is(err, pgx.ErrNoRows):
		http.Error(w, "usefulness feedback resource not found", http.StatusNotFound)
	case strings.Contains(err.Error(), "not configured"):
		http.Error(w, err.Error(), http.StatusServiceUnavailable)
	default:
		if strings.Contains(err.Error(), "required") || strings.Contains(err.Error(), "invalid") || strings.Contains(err.Error(), "must be") || strings.Contains(err.Error(), "must not") {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		http.Error(w, fallback, http.StatusInternalServerError)
	}
}

func writeTaskEvaluationError(w http.ResponseWriter, err error, fallback string) {
	switch {
	case errors.Is(err, pgx.ErrNoRows):
		http.Error(w, "task evaluation resource not found", http.StatusNotFound)
	case strings.Contains(err.Error(), "not configured"):
		http.Error(w, err.Error(), http.StatusServiceUnavailable)
	default:
		if strings.Contains(err.Error(), "required") || strings.Contains(err.Error(), "invalid") || strings.Contains(err.Error(), "must be") || strings.Contains(err.Error(), "at least one") {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		http.Error(w, fallback, http.StatusInternalServerError)
	}
}

func writeWorkflowError(w http.ResponseWriter, err error, fallback string) {
	switch {
	case errors.Is(err, pgx.ErrNoRows):
		http.Error(w, "workflow resource not found", http.StatusNotFound)
	case strings.Contains(err.Error(), "not configured"):
		http.Error(w, err.Error(), http.StatusServiceUnavailable)
	default:
		if strings.Contains(err.Error(), "required") || strings.Contains(err.Error(), "invalid") ||
			strings.Contains(err.Error(), "must be") || strings.Contains(err.Error(), "must not") ||
			strings.Contains(err.Error(), "out of scope") || strings.Contains(err.Error(), "not active") {
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

func recordUsefulnessFeedbackMetric(ctx context.Context, metrics MetricsRecorder, operation, result string, feedbackType memory.UsefulnessFeedbackType, subjectKind memory.UsefulnessFeedbackSubjectKind, source memory.UsefulnessFeedbackSourceSurface, decision string) {
	if metrics == nil {
		return
	}
	metrics.RecordUsefulnessFeedback(ctx, telemetry.UsefulnessFeedbackEvent{
		Operation:     operation,
		Result:        result,
		FeedbackType:  string(feedbackType),
		SubjectKind:   string(subjectKind),
		SourceSurface: string(source),
		Decision:      decision,
	})
}

func recordUsefulnessFeedbackLog(logger *log.Logger, operation, result string, feedbackType memory.UsefulnessFeedbackType, subjectKind memory.UsefulnessFeedbackSubjectKind, source memory.UsefulnessFeedbackSourceSurface, decision string) {
	if logger == nil {
		return
	}
	logger.Printf(
		"mode=api component=usefulness_feedback event=lifecycle operation=%s result=%s feedback_type=%s subject_kind=%s source_surface=%s decision=%s",
		boundedLogLabel(operation),
		boundedLogLabel(result),
		boundedLogLabel(string(feedbackType)),
		boundedLogLabel(string(subjectKind)),
		boundedLogLabel(string(source)),
		boundedLogLabel(decision),
	)
}

func recordTaskEvaluationMetric(ctx context.Context, metrics MetricsRecorder, operation, result string, verdict memory.TaskEvaluationVerdict, category memory.TaskContributionCategory, correction memory.TaskEvaluationCorrectionState) {
	if metrics == nil {
		return
	}
	metrics.RecordTaskEvaluation(ctx, telemetry.TaskEvaluationEvent{
		Operation:            operation,
		Result:               result,
		Verdict:              string(verdict),
		ContributionCategory: string(category),
		CorrectionState:      string(correction),
	})
}

func recordTaskEvaluationLog(logger *log.Logger, operation, result string, verdict memory.TaskEvaluationVerdict, category memory.TaskContributionCategory, correction memory.TaskEvaluationCorrectionState) {
	if logger == nil {
		return
	}
	logger.Printf(
		"mode=api component=task_evaluation event=lifecycle operation=%s result=%s verdict=%s contribution_category=%s correction_state=%s",
		boundedLogLabel(operation),
		boundedLogLabel(result),
		boundedLogLabel(string(verdict)),
		boundedLogLabel(string(category)),
		boundedLogLabel(string(correction)),
	)
}

func recordRankingRolloutMetric(ctx context.Context, metrics MetricsRecorder, operation, result string, surface memory.RankingRolloutSurface, source memory.RankingRolloutSignalSource, threshold memory.RankingRolloutThresholdStatus, status memory.RankingRolloutPolicyStatus, reasonCode string) {
	if metrics == nil {
		return
	}
	metrics.RecordRankingRollout(ctx, telemetry.RankingRolloutEvent{
		Operation:       operation,
		Result:          result,
		Surface:         string(surface),
		SignalSource:    string(source),
		ThresholdStatus: string(threshold),
		PolicyStatus:    string(status),
		ReasonCode:      reasonCode,
	})
}

func recordRankingRolloutLog(logger *log.Logger, operation, result string, surface memory.RankingRolloutSurface, source memory.RankingRolloutSignalSource, threshold memory.RankingRolloutThresholdStatus, status memory.RankingRolloutPolicyStatus, reasonCode string) {
	if logger == nil {
		return
	}
	logger.Printf(
		"mode=api component=ranking_rollout event=lifecycle operation=%s result=%s surface=%s signal_source=%s threshold_status=%s policy_status=%s reason_code=%s",
		boundedLogLabel(operation),
		boundedLogLabel(result),
		boundedLogLabel(string(surface)),
		boundedLogLabel(string(source)),
		boundedLogLabel(string(threshold)),
		boundedLogLabel(string(status)),
		boundedLogLabel(reasonCode),
	)
}

func boundedLogLabel(value string) string {
	if strings.TrimSpace(value) == "" {
		return "unknown"
	}
	return strings.TrimSpace(value)
}

func firstUsefulnessSubjectKind(subjects []memory.UsefulnessFeedbackSubject) memory.UsefulnessFeedbackSubjectKind {
	if len(subjects) == 0 {
		return ""
	}
	return subjects[0].Kind
}

func firstTaskContributionCategory(categories []memory.TaskContributionCategory) memory.TaskContributionCategory {
	if len(categories) == 0 {
		return ""
	}
	return categories[0]
}

func firstRankingRolloutSurface(surfaces []memory.RankingRolloutSurface) memory.RankingRolloutSurface {
	if len(surfaces) == 0 {
		return ""
	}
	return surfaces[0]
}

func firstRankingRolloutSignalSource(sources []memory.RankingRolloutSignalSource) memory.RankingRolloutSignalSource {
	if len(sources) == 0 {
		return ""
	}
	return sources[0]
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

func parseUsefulnessFeedbackActionTarget(value string) (string, string, error) {
	feedbackID, actionName, ok := strings.Cut(value, ":")
	if !ok || strings.TrimSpace(feedbackID) == "" || strings.TrimSpace(actionName) == "" {
		return "", "", fmt.Errorf("invalid usefulness feedback action target")
	}

	return strings.TrimSpace(feedbackID), strings.TrimSpace(actionName), nil
}

func usefulnessFeedbackSubjectFromQuery(r *http.Request) (memory.UsefulnessFeedbackSubject, error) {
	subject := memory.UsefulnessFeedbackSubject{
		Kind: memory.UsefulnessFeedbackSubjectKind(strings.TrimSpace(r.URL.Query().Get("subject_kind"))),
		ID:   strings.TrimSpace(r.URL.Query().Get("subject_id")),
	}
	if subject.Kind == memory.UsefulnessFeedbackSubjectExpectedRecall {
		subject.ID = ""
		subject.ExpectedRecallTarget = memory.ExpectedRecallTarget{
			Kind:        memory.ExpectedRecallTargetKind(strings.TrimSpace(r.URL.Query().Get("expected_recall_kind"))),
			ID:          strings.TrimSpace(r.URL.Query().Get("expected_recall_id")),
			OpaqueToken: strings.TrimSpace(r.URL.Query().Get("opaque_token")),
		}
	}
	if err := subject.Validate(); err != nil {
		return memory.UsefulnessFeedbackSubject{}, err
	}
	return subject, nil
}

func parseOptionalBoolQuery(value string) (bool, error) {
	if strings.TrimSpace(value) == "" {
		return false, nil
	}
	return strconv.ParseBool(strings.TrimSpace(value))
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
