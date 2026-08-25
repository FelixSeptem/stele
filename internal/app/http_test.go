package app

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/FelixSeptem/stele/internal/assurance"
	"github.com/FelixSeptem/stele/internal/auth"
	"github.com/FelixSeptem/stele/internal/diagnostics"
	"github.com/FelixSeptem/stele/internal/governance"
	"github.com/FelixSeptem/stele/internal/jobs"
	"github.com/FelixSeptem/stele/internal/memory"
	"github.com/FelixSeptem/stele/internal/policy"
	"github.com/FelixSeptem/stele/internal/retrieval"
	"github.com/FelixSeptem/stele/internal/telemetry"
	"github.com/FelixSeptem/stele/internal/workflow"
	"github.com/jackc/pgx/v5"
)

type stubReadinessChecker struct {
	err error
}

type stubPrincipalAuthorizer struct {
	principal auth.Principal
	granted   bool
}

func (s stubPrincipalAuthorizer) Authenticate(context.Context, string) (auth.Principal, auth.Credential, error) {
	return s.principal, auth.Credential{}, nil
}

func (s stubPrincipalAuthorizer) AuthorizeScope(context.Context, string, memory.Scope) (bool, error) {
	return s.granted, nil
}

func (s stubReadinessChecker) Ready(ctx context.Context) error {
	return s.err
}

type stubEventIngestor struct {
	gotInput    memory.IngestEventInput
	ingestCalls int
	eventID     string
	admission   *memory.AdmissionPressureReport
	err         error
}

type stubIdempotentEventIngestor struct {
	stubEventIngestor
	gotPrincipalID   string
	gotKey           string
	idempotentCalls  int
	replayed         bool
	replayAfterFirst bool
}

func (s *stubIdempotentEventIngestor) IngestIdempotent(ctx context.Context, input memory.IngestEventInput, principalID, idempotencyKey string) (memory.IdempotentEventIngestResult, error) {
	s.gotInput = input
	s.gotPrincipalID = principalID
	s.gotKey = idempotencyKey
	s.idempotentCalls++
	if s.err != nil {
		return memory.IdempotentEventIngestResult{}, s.err
	}
	replayed := s.replayed || (s.replayAfterFirst && s.idempotentCalls > 1)
	return memory.IdempotentEventIngestResult{Event: memory.RawEvent{ID: s.eventID, Admission: s.admission}, Replayed: replayed}, nil
}

func (s *stubEventIngestor) Ingest(ctx context.Context, input memory.IngestEventInput) (memory.RawEvent, error) {
	s.gotInput = input
	s.ingestCalls++
	if s.err != nil {
		return memory.RawEvent{}, s.err
	}

	return memory.RawEvent{ID: s.eventID, Admission: s.admission}, nil
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
	gotListInput       memory.ListEmbeddingRebuildsInput
	gotReadScope       memory.Scope
	gotReadMemoryID    string
	gotApplyInput      memory.ApplyEmbeddingRecoveryInput
	gotCreateCutover   memory.CreateEmbeddingCutoverPlanInput
	gotListCutovers    memory.ListEmbeddingCutoverPlansInput
	gotReadCutover     memory.ReadEmbeddingCutoverPlanInput
	gotPreflight       memory.EmbeddingCutoverPreflightInput
	gotApplyCutover    memory.ApplyEmbeddingCutoverPlanActionInput
	gotRecoveryHistory memory.ListEmbeddingRecoveryHistoryInput
	page               memory.EmbeddingRebuildPage
	inspection         memory.EmbeddingMemoryInspection
	outcome            memory.EmbeddingRecoveryOutcome
	cutoverPlan        memory.EmbeddingCutoverPlan
	preflightReport    memory.EmbeddingCutoverPreflightReport
	cutoverPlans       []memory.EmbeddingCutoverPlan
	recoveryHistory    []memory.EmbeddingRecoveryRecord
	listErr            error
	readErr            error
	applyErr           error
	cutoverErr         error
	validateList       bool
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

func (s *stubEmbeddingAdminService) CreateEmbeddingCutoverPlan(ctx context.Context, input memory.CreateEmbeddingCutoverPlanInput) (memory.EmbeddingCutoverPlan, error) {
	s.gotCreateCutover = input
	return s.cutoverPlan, s.cutoverErr
}

func (s *stubEmbeddingAdminService) ListEmbeddingCutoverPlans(ctx context.Context, input memory.ListEmbeddingCutoverPlansInput) ([]memory.EmbeddingCutoverPlan, error) {
	s.gotListCutovers = input
	return s.cutoverPlans, s.cutoverErr
}

func (s *stubEmbeddingAdminService) ReadEmbeddingCutoverPlan(ctx context.Context, input memory.ReadEmbeddingCutoverPlanInput) (memory.EmbeddingCutoverPlan, error) {
	s.gotReadCutover = input
	return s.cutoverPlan, s.cutoverErr
}

func (s *stubEmbeddingAdminService) PreflightEmbeddingCutoverPlan(ctx context.Context, input memory.EmbeddingCutoverPreflightInput) (memory.EmbeddingCutoverPreflightReport, error) {
	s.gotPreflight = input
	return s.preflightReport, s.cutoverErr
}

func (s *stubEmbeddingAdminService) ApplyEmbeddingCutoverPlanAction(ctx context.Context, input memory.ApplyEmbeddingCutoverPlanActionInput) (memory.EmbeddingCutoverPlan, error) {
	s.gotApplyCutover = input
	return s.cutoverPlan, s.cutoverErr
}

func (s *stubEmbeddingAdminService) ListEmbeddingRecoveryHistory(ctx context.Context, input memory.ListEmbeddingRecoveryHistoryInput) ([]memory.EmbeddingRecoveryRecord, error) {
	s.gotRecoveryHistory = input
	return s.recoveryHistory, s.cutoverErr
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

type stubDerivedInsightAdminService struct {
	gotListInput      memory.ListDerivedInsightsInput
	gotReadInput      memory.ReadDerivedInsightInput
	gotTransition     memory.DerivedInsightLifecycleTransition
	gotCreateFeedback memory.CreateDerivedInsightFeedbackInput
	gotListFeedback   memory.ListDerivedInsightFeedbackInput
	gotSupersede      memory.SupersedeDerivedInsightFeedbackInput
	items             []memory.DerivedInsight
	detail            memory.DerivedInsightDetail
	feedback          memory.DerivedInsightFeedback
	feedbackItems     []memory.DerivedInsightFeedback
	listErr           error
	readErr           error
	transitionErr     error
	feedbackErr       error
	validateList      bool
	validateRead      bool
	validateLifecycle bool
	validateFeedback  bool
}

type stubDerivedInsightReplayAdminService struct {
	gotPlanInput  memory.DerivedInsightReplayRequest
	gotApplyInput memory.DerivedInsightReplayRequest
	gotListInput  memory.ListDerivedInsightReplayRunsInput
	gotReadInput  memory.ReadDerivedInsightReplayRunInput
	plan          memory.DerivedInsightReplayReport
	run           memory.DerivedInsightReplayRun
	runs          []memory.DerivedInsightReplayRun
	report        memory.DerivedInsightReplayReport
	err           error
	validate      bool
}

type stubScopeProofAdminService struct {
	gotCreateInput memory.CreateScopeProofRunInput
	gotListInput   memory.ListScopeProofRunsInput
	gotReadInput   memory.ReadScopeProofRunInput
	gotRerunInput  memory.RerunScopeProofRunInput
	run            memory.ScopeProofRun
	runs           []memory.ScopeProofRun
	report         memory.ScopeProofReport
	err            error
	validate       bool
}

type stubMemorySessionService struct {
	gotCreateInput       memory.CreateMemorySessionInput
	gotListInput         memory.ListMemorySessionRunsInput
	gotReadInput         memory.ReadMemorySessionRunInput
	gotCreateTurnInput   memory.CreateMemorySessionTurnInput
	gotOutcomeInput      memory.RecordMemorySessionTurnOutcomeInput
	gotVerificationInput memory.RequestMemorySessionVerificationInput
	session              memory.MemorySessionRun
	sessions             []memory.MemorySessionRun
	turn                 memory.MemorySessionTurn
	verification         memory.MemorySessionVerification
	report               memory.MemorySessionReport
	err                  error
	validate             bool
}

type stubUsefulnessFeedbackService struct {
	gotCreateInput    memory.UsefulnessFeedback
	gotListInput      memory.ListUsefulnessFeedbackInput
	gotReadInput      memory.ReadUsefulnessFeedbackInput
	gotSummaryInput   memory.SummarizeUsefulnessFeedbackInput
	gotSupersedeInput memory.SupersedeUsefulnessFeedbackInput
	createCalls       int
	created           memory.UsefulnessFeedback
	items             []memory.UsefulnessFeedback
	read              memory.UsefulnessFeedback
	summary           memory.UsefulnessFeedbackSummary
	err               error
	validate          bool
}

func (s *stubScopeProofAdminService) CreateProofRun(ctx context.Context, input memory.CreateScopeProofRunInput) (memory.ScopeProofRun, error) {
	s.gotCreateInput = input
	if s.validate {
		if err := input.Validate(); err != nil {
			return memory.ScopeProofRun{}, err
		}
	}
	return s.run, s.err
}

func (s *stubScopeProofAdminService) ListProofRuns(ctx context.Context, input memory.ListScopeProofRunsInput) ([]memory.ScopeProofRun, error) {
	s.gotListInput = input
	if s.validate {
		if err := input.Validate(); err != nil {
			return nil, err
		}
	}
	return s.runs, s.err
}

func (s *stubScopeProofAdminService) ReadProofRun(ctx context.Context, input memory.ReadScopeProofRunInput) (memory.ScopeProofRun, error) {
	s.gotReadInput = input
	if s.validate {
		if err := input.Validate(); err != nil {
			return memory.ScopeProofRun{}, err
		}
	}
	return s.run, s.err
}

func (s *stubScopeProofAdminService) ReadProofReport(ctx context.Context, input memory.ReadScopeProofRunInput) (memory.ScopeProofReport, error) {
	s.gotReadInput = input
	if s.validate {
		if err := input.Validate(); err != nil {
			return memory.ScopeProofReport{}, err
		}
	}
	return s.report, s.err
}

func (s *stubScopeProofAdminService) RerunProofRun(ctx context.Context, input memory.RerunScopeProofRunInput) (memory.ScopeProofRun, error) {
	s.gotRerunInput = input
	if s.validate {
		if err := input.Validate(); err != nil {
			return memory.ScopeProofRun{}, err
		}
	}
	return s.run, s.err
}

func (s *stubMemorySessionService) CreateSession(ctx context.Context, input memory.CreateMemorySessionInput) (memory.MemorySessionRun, error) {
	s.gotCreateInput = input
	if s.validate {
		if err := input.Validate(); err != nil {
			return memory.MemorySessionRun{}, err
		}
	}
	return s.session, s.err
}

func (s *stubMemorySessionService) ListSessions(ctx context.Context, input memory.ListMemorySessionRunsInput) ([]memory.MemorySessionRun, error) {
	s.gotListInput = input
	if s.validate {
		if err := input.Validate(); err != nil {
			return nil, err
		}
	}
	return s.sessions, s.err
}

func (s *stubMemorySessionService) ReadSession(ctx context.Context, input memory.ReadMemorySessionRunInput) (memory.MemorySessionRun, error) {
	s.gotReadInput = input
	if s.validate {
		if err := input.Validate(); err != nil {
			return memory.MemorySessionRun{}, err
		}
	}
	return s.session, s.err
}

func (s *stubMemorySessionService) CreateTurn(ctx context.Context, input memory.CreateMemorySessionTurnInput) (memory.MemorySessionTurn, error) {
	s.gotCreateTurnInput = input
	if s.validate {
		if err := input.Validate(); err != nil {
			return memory.MemorySessionTurn{}, err
		}
	}
	return s.turn, s.err
}

func (s *stubMemorySessionService) RecordTurnOutcome(ctx context.Context, input memory.RecordMemorySessionTurnOutcomeInput) (memory.MemorySessionTurn, error) {
	s.gotOutcomeInput = input
	if s.validate {
		if err := input.Validate(); err != nil {
			return memory.MemorySessionTurn{}, err
		}
	}
	return s.turn, s.err
}

func (s *stubMemorySessionService) RequestVerification(ctx context.Context, input memory.RequestMemorySessionVerificationInput) (memory.MemorySessionVerification, error) {
	s.gotVerificationInput = input
	if s.validate {
		if err := input.Validate(); err != nil {
			return memory.MemorySessionVerification{}, err
		}
	}
	return s.verification, s.err
}

func (s *stubMemorySessionService) ReadSessionReport(ctx context.Context, input memory.ReadMemorySessionRunInput) (memory.MemorySessionReport, error) {
	s.gotReadInput = input
	if s.validate {
		if err := input.Validate(); err != nil {
			return memory.MemorySessionReport{}, err
		}
	}
	return s.report, s.err
}

func (s *stubUsefulnessFeedbackService) CreateUsefulnessFeedback(ctx context.Context, input memory.UsefulnessFeedback) (memory.UsefulnessFeedback, error) {
	s.createCalls++
	s.gotCreateInput = input
	if s.validate {
		if err := input.Validate(); err != nil {
			return memory.UsefulnessFeedback{}, err
		}
	}
	if s.created.ID == "" {
		s.created = input
	}
	return s.created, s.err
}

func (s *stubUsefulnessFeedbackService) ListUsefulnessFeedback(ctx context.Context, input memory.ListUsefulnessFeedbackInput) ([]memory.UsefulnessFeedback, error) {
	s.gotListInput = input
	if s.validate {
		if err := input.Validate(); err != nil {
			return nil, err
		}
	}
	return s.items, s.err
}

func (s *stubUsefulnessFeedbackService) ReadUsefulnessFeedback(ctx context.Context, input memory.ReadUsefulnessFeedbackInput) (memory.UsefulnessFeedback, error) {
	s.gotReadInput = input
	if s.validate {
		if err := input.Validate(); err != nil {
			return memory.UsefulnessFeedback{}, err
		}
	}
	return s.read, s.err
}

func (s *stubUsefulnessFeedbackService) SummarizeUsefulnessFeedback(ctx context.Context, input memory.SummarizeUsefulnessFeedbackInput) (memory.UsefulnessFeedbackSummary, error) {
	s.gotSummaryInput = input
	if s.validate {
		if err := input.Validate(); err != nil {
			return memory.UsefulnessFeedbackSummary{}, err
		}
	}
	return s.summary, s.err
}

func (s *stubUsefulnessFeedbackService) SupersedeUsefulnessFeedback(ctx context.Context, input memory.SupersedeUsefulnessFeedbackInput) error {
	s.gotSupersedeInput = input
	if s.validate {
		if err := input.Validate(); err != nil {
			return err
		}
	}
	return s.err
}

type stubTaskEvaluationService struct {
	gotCreateInput    memory.TaskEvaluation
	gotReadInput      memory.ReadTaskEvaluationInput
	gotListInput      memory.ListTaskEvaluationsInput
	gotSupersedeInput memory.SupersedeTaskEvaluationInput
	gotSummaryInput   memory.SummarizeTaskEvaluationsInput
	created           memory.TaskEvaluation
	items             []memory.TaskEvaluation
	summary           memory.TaskEvaluationSummary
	err               error
	validate          bool
	createCalls       int
}

type stubRankingRolloutAdminService struct {
	gotCreate   memory.RankingRolloutPolicy
	gotRead     memory.ReadRankingRolloutPolicyInput
	gotList     memory.ListRankingRolloutPoliciesInput
	gotDryRun   memory.RecordRankingRolloutDryRunInput
	gotActivate memory.ActivateRankingRolloutPolicyInput
	gotDisable  memory.DisableRankingRolloutPolicyInput
	gotRollback memory.RollbackRankingRolloutPolicyInput
	gotImpact   memory.ListRankingRolloutPolicyImpactInput
	policy      memory.RankingRolloutPolicy
	policies    []memory.RankingRolloutPolicy
	dryRun      memory.RankingRolloutDryRun
	impact      []memory.RankingRolloutImpactEntry
	err         error
}

type stubAssuranceAdminService struct {
	gotHealthCreate        assurance.HealthEvaluationInput
	gotHealthRead          assurance.ReadHealthEvaluationInput
	gotIncidentList        assurance.ListIncidentsInput
	gotIncidentRead        assurance.ReadIncidentInput
	gotIncidentAction      assurance.IncidentActionInput
	gotAlertRead           assurance.ReadAlertCandidateInput
	gotAlertAttempts       assurance.ListAlertDeliveryAttemptsInput
	gotProfileCreate       assurance.ConformanceProfile
	gotProfileList         assurance.ListConformanceProfilesInput
	gotProfileRead         assurance.ReadConformanceProfileInput
	gotProfileUpdate       assurance.UpdateConformanceProfileInput
	gotProfileDisable      assurance.DisableConformanceProfileInput
	gotRunCreate           assurance.ConformanceRunInput
	gotRunList             assurance.ListConformanceRunsInput
	gotRunRead             assurance.ReadConformanceRunInput
	gotReadinessCreate     assurance.ReadinessReportInput
	gotReadinessRead       assurance.ReadReadinessReportInput
	gotRecoveryCreate      assurance.RecoveryVerificationInput
	gotRecoveryRead        assurance.ReadRecoveryVerificationInput
	healthEvaluation       assurance.HealthEvaluation
	healthEvaluations      []assurance.HealthEvaluation
	incident               assurance.Incident
	incidents              []assurance.Incident
	alertCandidate         assurance.AlertCandidate
	alertCandidates        []assurance.AlertCandidate
	alertAttempts          []assurance.AlertDeliveryAttempt
	conformanceProfile     assurance.ConformanceProfile
	conformanceProfiles    []assurance.ConformanceProfile
	conformanceRun         assurance.ConformanceRun
	conformanceRuns        []assurance.ConformanceRun
	conformanceDiagnostics []assurance.MissingEvidenceDiagnostic
	readinessReport        assurance.ReadinessReport
	readinessReports       []assurance.ReadinessReport
	recoveryVerification   assurance.RecoveryVerification
	recoveryVerifications  []assurance.RecoveryVerification
	err                    error
}

func (s *stubAssuranceAdminService) CreateHealthEvaluation(ctx context.Context, input assurance.HealthEvaluationInput) (assurance.HealthEvaluation, error) {
	s.gotHealthCreate = input
	return s.healthEvaluation, s.err
}

func (s *stubAssuranceAdminService) ListHealthEvaluations(ctx context.Context, scope memory.Scope) ([]assurance.HealthEvaluation, error) {
	if s.err != nil {
		return nil, s.err
	}
	return s.healthEvaluations, nil
}

func (s *stubAssuranceAdminService) ReadHealthEvaluation(ctx context.Context, input assurance.ReadHealthEvaluationInput) (assurance.HealthEvaluation, error) {
	s.gotHealthRead = input
	return s.healthEvaluation, s.err
}

func (s *stubAssuranceAdminService) ListIncidents(ctx context.Context, input assurance.ListIncidentsInput) ([]assurance.Incident, error) {
	s.gotIncidentList = input
	if s.err != nil {
		return nil, s.err
	}
	return s.incidents, nil
}

func (s *stubAssuranceAdminService) ReadIncident(ctx context.Context, input assurance.ReadIncidentInput) (assurance.Incident, error) {
	s.gotIncidentRead = input
	return s.incident, s.err
}

func (s *stubAssuranceAdminService) ApplyIncidentAction(ctx context.Context, input assurance.IncidentActionInput) (assurance.Incident, error) {
	s.gotIncidentAction = input
	return s.incident, s.err
}

func (s *stubAssuranceAdminService) ListAlertCandidates(ctx context.Context, scope memory.Scope) ([]assurance.AlertCandidate, error) {
	if s.err != nil {
		return nil, s.err
	}
	return s.alertCandidates, nil
}

func (s *stubAssuranceAdminService) ReadAlertCandidate(ctx context.Context, input assurance.ReadAlertCandidateInput) (assurance.AlertCandidate, error) {
	s.gotAlertRead = input
	return s.alertCandidate, s.err
}

func (s *stubAssuranceAdminService) ListAlertDeliveryAttempts(ctx context.Context, input assurance.ListAlertDeliveryAttemptsInput) ([]assurance.AlertDeliveryAttempt, error) {
	s.gotAlertAttempts = input
	if s.err != nil {
		return nil, s.err
	}
	return s.alertAttempts, nil
}

func (s *stubAssuranceAdminService) CreateConformanceProfile(ctx context.Context, profile assurance.ConformanceProfile) (assurance.ConformanceProfile, error) {
	s.gotProfileCreate = profile
	return s.conformanceProfile, s.err
}

func (s *stubAssuranceAdminService) ListConformanceProfiles(ctx context.Context, input assurance.ListConformanceProfilesInput) ([]assurance.ConformanceProfile, error) {
	s.gotProfileList = input
	if s.err != nil {
		return nil, s.err
	}
	return s.conformanceProfiles, nil
}

func (s *stubAssuranceAdminService) ReadConformanceProfile(ctx context.Context, input assurance.ReadConformanceProfileInput) (assurance.ConformanceProfile, error) {
	s.gotProfileRead = input
	return s.conformanceProfile, s.err
}

func (s *stubAssuranceAdminService) UpdateConformanceProfile(ctx context.Context, input assurance.UpdateConformanceProfileInput) (assurance.ConformanceProfile, error) {
	s.gotProfileUpdate = input
	return s.conformanceProfile, s.err
}

func (s *stubAssuranceAdminService) DisableConformanceProfile(ctx context.Context, input assurance.DisableConformanceProfileInput) (assurance.ConformanceProfile, error) {
	s.gotProfileDisable = input
	return s.conformanceProfile, s.err
}

func (s *stubAssuranceAdminService) RunConformance(ctx context.Context, input assurance.ConformanceRunInput) (assurance.ConformanceRun, []assurance.MissingEvidenceDiagnostic, error) {
	s.gotRunCreate = input
	return s.conformanceRun, s.conformanceDiagnostics, s.err
}

func (s *stubAssuranceAdminService) ListConformanceRuns(ctx context.Context, input assurance.ListConformanceRunsInput) ([]assurance.ConformanceRun, error) {
	s.gotRunList = input
	if s.err != nil {
		return nil, s.err
	}
	return s.conformanceRuns, nil
}

func (s *stubAssuranceAdminService) ReadConformanceRun(ctx context.Context, input assurance.ReadConformanceRunInput) (assurance.ConformanceRun, error) {
	s.gotRunRead = input
	return s.conformanceRun, s.err
}

func (s *stubAssuranceAdminService) CreateReadinessReport(ctx context.Context, input assurance.ReadinessReportInput) (assurance.ReadinessReport, error) {
	s.gotReadinessCreate = input
	return s.readinessReport, s.err
}

func (s *stubAssuranceAdminService) ListReadinessReports(ctx context.Context, scope memory.Scope) ([]assurance.ReadinessReport, error) {
	if s.err != nil {
		return nil, s.err
	}
	return s.readinessReports, nil
}

func (s *stubAssuranceAdminService) ReadReadinessReport(ctx context.Context, input assurance.ReadReadinessReportInput) (assurance.ReadinessReport, error) {
	s.gotReadinessRead = input
	return s.readinessReport, s.err
}

func (s *stubAssuranceAdminService) CreateRecoveryVerification(ctx context.Context, input assurance.RecoveryVerificationInput) (assurance.RecoveryVerification, error) {
	s.gotRecoveryCreate = input
	return s.recoveryVerification, s.err
}

func (s *stubAssuranceAdminService) ListRecoveryVerifications(ctx context.Context, scope memory.Scope) ([]assurance.RecoveryVerification, error) {
	if s.err != nil {
		return nil, s.err
	}
	return s.recoveryVerifications, nil
}

func (s *stubAssuranceAdminService) ReadRecoveryVerification(ctx context.Context, input assurance.ReadRecoveryVerificationInput) (assurance.RecoveryVerification, error) {
	s.gotRecoveryRead = input
	return s.recoveryVerification, s.err
}

func (s *stubRankingRolloutAdminService) CreateRankingRolloutPolicy(ctx context.Context, policy memory.RankingRolloutPolicy) (memory.RankingRolloutPolicy, error) {
	s.gotCreate = policy
	return s.policy, s.err
}

func (s *stubRankingRolloutAdminService) ReadRankingRolloutPolicy(ctx context.Context, input memory.ReadRankingRolloutPolicyInput) (memory.RankingRolloutPolicy, error) {
	s.gotRead = input
	return s.policy, s.err
}

func (s *stubRankingRolloutAdminService) ListRankingRolloutPolicies(ctx context.Context, input memory.ListRankingRolloutPoliciesInput) ([]memory.RankingRolloutPolicy, error) {
	s.gotList = input
	return s.policies, s.err
}

func (s *stubRankingRolloutAdminService) RecordRankingRolloutDryRun(ctx context.Context, input memory.RecordRankingRolloutDryRunInput) (memory.RankingRolloutDryRun, error) {
	s.gotDryRun = input
	return s.dryRun, s.err
}

func (s *stubRankingRolloutAdminService) ActivateRankingRolloutPolicy(ctx context.Context, input memory.ActivateRankingRolloutPolicyInput) (memory.RankingRolloutPolicy, error) {
	s.gotActivate = input
	return s.policy, s.err
}

func (s *stubRankingRolloutAdminService) DisableRankingRolloutPolicy(ctx context.Context, input memory.DisableRankingRolloutPolicyInput) (memory.RankingRolloutPolicy, error) {
	s.gotDisable = input
	return s.policy, s.err
}

func (s *stubRankingRolloutAdminService) RollbackRankingRolloutPolicy(ctx context.Context, input memory.RollbackRankingRolloutPolicyInput) (memory.RankingRolloutPolicy, error) {
	s.gotRollback = input
	return s.policy, s.err
}

func (s *stubRankingRolloutAdminService) ListRankingRolloutPolicyImpact(ctx context.Context, input memory.ListRankingRolloutPolicyImpactInput) ([]memory.RankingRolloutImpactEntry, error) {
	s.gotImpact = input
	return s.impact, s.err
}

func (s *stubTaskEvaluationService) CreateTaskEvaluation(ctx context.Context, evaluation memory.TaskEvaluation) (memory.TaskEvaluation, error) {
	s.createCalls++
	s.gotCreateInput = evaluation
	if s.validate {
		if err := evaluation.Validate(); err != nil {
			return memory.TaskEvaluation{}, err
		}
	}
	if s.created.ID == "" {
		s.created = evaluation
	}
	return s.created, s.err
}

func (s *stubTaskEvaluationService) ReadTaskEvaluation(ctx context.Context, input memory.ReadTaskEvaluationInput) (memory.TaskEvaluation, error) {
	s.gotReadInput = input
	if s.validate {
		if err := input.Validate(); err != nil {
			return memory.TaskEvaluation{}, err
		}
	}
	return s.created, s.err
}

func (s *stubTaskEvaluationService) ListTaskEvaluations(ctx context.Context, input memory.ListTaskEvaluationsInput) ([]memory.TaskEvaluation, error) {
	s.gotListInput = input
	if s.validate {
		if err := input.Validate(); err != nil {
			return nil, err
		}
	}
	return s.items, s.err
}

func (s *stubTaskEvaluationService) SupersedeTaskEvaluation(ctx context.Context, input memory.SupersedeTaskEvaluationInput) error {
	s.gotSupersedeInput = input
	if s.validate {
		if err := input.Validate(); err != nil {
			return err
		}
	}
	return s.err
}

func (s *stubTaskEvaluationService) SummarizeTaskEvaluations(ctx context.Context, input memory.SummarizeTaskEvaluationsInput) (memory.TaskEvaluationSummary, error) {
	s.gotSummaryInput = input
	if s.validate {
		if err := input.Validate(); err != nil {
			return memory.TaskEvaluationSummary{}, err
		}
	}
	return s.summary, s.err
}

func (s *stubDerivedInsightReplayAdminService) PlanDerivedInsightReplay(ctx context.Context, input memory.DerivedInsightReplayRequest) (memory.DerivedInsightReplayReport, error) {
	s.gotPlanInput = input
	if s.validate {
		if err := input.Validate(); err != nil {
			return memory.DerivedInsightReplayReport{}, err
		}
	}
	return s.plan, s.err
}

func (s *stubDerivedInsightReplayAdminService) ApplyDerivedInsightReplay(ctx context.Context, input memory.DerivedInsightReplayRequest) (memory.DerivedInsightReplayRun, error) {
	s.gotApplyInput = input
	if s.validate {
		if err := input.Validate(); err != nil {
			return memory.DerivedInsightReplayRun{}, err
		}
	}
	return s.run, s.err
}

func (s *stubDerivedInsightReplayAdminService) ListDerivedInsightReplayRuns(ctx context.Context, input memory.ListDerivedInsightReplayRunsInput) ([]memory.DerivedInsightReplayRun, error) {
	s.gotListInput = input
	if s.validate {
		if err := input.Validate(); err != nil {
			return nil, err
		}
	}
	return s.runs, s.err
}

func (s *stubDerivedInsightReplayAdminService) ReadDerivedInsightReplayRun(ctx context.Context, input memory.ReadDerivedInsightReplayRunInput) (memory.DerivedInsightReplayRun, error) {
	s.gotReadInput = input
	if s.validate {
		if err := input.Validate(); err != nil {
			return memory.DerivedInsightReplayRun{}, err
		}
	}
	return s.run, s.err
}

func (s *stubDerivedInsightReplayAdminService) ReadDerivedInsightReplayReport(ctx context.Context, input memory.ReadDerivedInsightReplayRunInput) (memory.DerivedInsightReplayReport, error) {
	s.gotReadInput = input
	if s.validate {
		if err := input.Validate(); err != nil {
			return memory.DerivedInsightReplayReport{}, err
		}
	}
	return s.report, s.err
}

func (s *stubDerivedInsightAdminService) ListDerivedInsights(ctx context.Context, input memory.ListDerivedInsightsInput) ([]memory.DerivedInsight, error) {
	s.gotListInput = input
	if s.validateList {
		if err := input.Validate(); err != nil {
			return nil, err
		}
	}
	if s.listErr != nil {
		return nil, s.listErr
	}
	return s.items, nil
}

func (s *stubDerivedInsightAdminService) ReadDerivedInsight(ctx context.Context, input memory.ReadDerivedInsightInput) (memory.DerivedInsightDetail, error) {
	s.gotReadInput = input
	if s.validateRead {
		if err := input.Validate(); err != nil {
			return memory.DerivedInsightDetail{}, err
		}
	}
	if s.readErr != nil {
		return memory.DerivedInsightDetail{}, s.readErr
	}
	return s.detail, nil
}

func (s *stubDerivedInsightAdminService) TransitionDerivedInsightLifecycle(ctx context.Context, transition memory.DerivedInsightLifecycleTransition) error {
	s.gotTransition = transition
	if s.validateLifecycle {
		if err := transition.Validate(); err != nil {
			return err
		}
	}
	return s.transitionErr
}

func (s *stubDerivedInsightAdminService) CreateDerivedInsightFeedback(ctx context.Context, input memory.CreateDerivedInsightFeedbackInput) (memory.DerivedInsightFeedback, error) {
	s.gotCreateFeedback = input
	if s.validateFeedback {
		if err := input.Validate(); err != nil {
			return memory.DerivedInsightFeedback{}, err
		}
	}
	return s.feedback, s.feedbackErr
}

func (s *stubDerivedInsightAdminService) ListDerivedInsightFeedback(ctx context.Context, input memory.ListDerivedInsightFeedbackInput) ([]memory.DerivedInsightFeedback, error) {
	s.gotListFeedback = input
	if s.validateFeedback {
		if err := input.Validate(); err != nil {
			return nil, err
		}
	}
	return s.feedbackItems, s.feedbackErr
}

func (s *stubDerivedInsightAdminService) SupersedeDerivedInsightFeedback(ctx context.Context, input memory.SupersedeDerivedInsightFeedbackInput) error {
	s.gotSupersede = input
	if s.validateFeedback {
		if err := input.Validate(); err != nil {
			return err
		}
	}
	return s.feedbackErr
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

func TestNewHTTPHandlerServesLivenessReadinessAndMetrics(t *testing.T) {
	handler := NewHTTPHandler(HTTPDependencies{
		Readiness: stubReadinessChecker{},
	})

	for _, path := range []string{"/livez", "/readyz", "/metrics"} {
		req := httptest.NewRequest(http.MethodGet, path, nil)
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("%s status = %d, want %d", path, rec.Code, http.StatusOK)
		}
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

func TestNewHTTPHandlerUsesPrincipalAuthorizationForPublicRoutes(t *testing.T) {
	query := &stubMemoryQueryService{}
	handler := NewHTTPHandler(HTTPDependencies{
		APIKeys: auth.StaticAPIKeys{"legacy-key": {}},
		PrincipalAuthorizer: stubPrincipalAuthorizer{
			principal: auth.Principal{ID: "principal_1", Role: auth.PrincipalRolePublic, Status: auth.PrincipalStatusActive, Label: "integration-a", CreatedAt: time.Now().UTC()},
			granted:   true,
		},
		MemoryQuery: query,
	})

	req := httptest.NewRequest(http.MethodGet, "/v1/memories", nil)
	req.Header.Set("X-API-Key", "principal-key")
	req.Header.Set("X-Stele-Tenant", "tenant-a")
	req.Header.Set("X-Stele-Project", "project-a")
	req.Header.Set("X-Stele-Namespace", "namespace-a")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
	if query.gotListInput.Scope != (memory.Scope{Tenant: "tenant-a", Project: "project-a", Namespace: "namespace-a"}) {
		t.Fatalf("scope = %+v, want authorized header scope", query.gotListInput.Scope)
	}
}

func TestNewHTTPHandlerDeniesPublicPrincipalOnAdminRoute(t *testing.T) {
	handler := NewHTTPHandler(HTTPDependencies{
		PrincipalAuthorizer: stubPrincipalAuthorizer{
			principal: auth.Principal{ID: "principal_1", Role: auth.PrincipalRolePublic, Status: auth.PrincipalStatusActive, Label: "integration-a", CreatedAt: time.Now().UTC()},
			granted:   true,
		},
		GovernanceStatusRead: &stubGovernanceStatusReader{},
	})

	req := httptest.NewRequest(http.MethodGet, "/v1/admin/jobs/governance/status", nil)
	req.Header.Set("X-API-Key", "principal-key")
	req.Header.Set("X-Stele-Tenant", "tenant-a")
	req.Header.Set("X-Stele-Project", "project-a")
	req.Header.Set("X-Stele-Namespace", "namespace-a")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusForbidden)
	}
}

func TestNewHTTPHandlerRequiresIdempotencyKeyForPrincipalEventIngest(t *testing.T) {
	ingestor := &stubIdempotentEventIngestor{stubEventIngestor: stubEventIngestor{eventID: "evt_123"}}
	handler := NewHTTPHandler(HTTPDependencies{
		PrincipalAuthorizer: stubPrincipalAuthorizer{principal: auth.Principal{ID: "principal_1", Role: auth.PrincipalRolePublic, Status: auth.PrincipalStatusActive, Label: "integration-a", CreatedAt: time.Now().UTC()}, granted: true},
		EventIngestor:       ingestor,
	})
	req := httptest.NewRequest(http.MethodPost, "/v1/events", strings.NewReader(`{"event_type":"conversation.message","content":"hello"}`))
	req.Header.Set("X-API-Key", "principal-key")
	req.Header.Set("X-Stele-Tenant", "tenant-a")
	req.Header.Set("X-Stele-Project", "project-a")
	req.Header.Set("X-Stele-Namespace", "namespace-a")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest || ingestor.gotKey != "" {
		t.Fatalf("status=%d key=%q, want missing key rejection", rec.Code, ingestor.gotKey)
	}
}

func TestNewHTTPHandlerReplaysIdempotentPrincipalEventIngest(t *testing.T) {
	ingestor := &stubIdempotentEventIngestor{stubEventIngestor: stubEventIngestor{eventID: "evt_123"}, replayed: true}
	handler := NewHTTPHandler(HTTPDependencies{
		PrincipalAuthorizer: stubPrincipalAuthorizer{principal: auth.Principal{ID: "principal_1", Role: auth.PrincipalRolePublic, Status: auth.PrincipalStatusActive, Label: "integration-a", CreatedAt: time.Now().UTC()}, granted: true},
		EventIngestor:       ingestor,
	})
	req := httptest.NewRequest(http.MethodPost, "/v1/events", strings.NewReader(`{"event_type":"conversation.message","content":"hello"}`))
	req.Header.Set("X-API-Key", "principal-key")
	req.Header.Set("Idempotency-Key", "request-key")
	req.Header.Set("X-Stele-Tenant", "tenant-a")
	req.Header.Set("X-Stele-Project", "project-a")
	req.Header.Set("X-Stele-Namespace", "namespace-a")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK || ingestor.gotPrincipalID != "principal_1" || ingestor.gotKey != "request-key" {
		t.Fatalf("status=%d principal=%q key=%q", rec.Code, ingestor.gotPrincipalID, ingestor.gotKey)
	}
}

func TestNewHTTPHandlerRoutesExactPrincipalRetryOnlyThroughIdempotentIngestor(t *testing.T) {
	ingestor := &stubIdempotentEventIngestor{stubEventIngestor: stubEventIngestor{eventID: "evt_123"}, replayAfterFirst: true}
	handler := NewHTTPHandler(HTTPDependencies{
		PrincipalAuthorizer: stubPrincipalAuthorizer{principal: auth.Principal{ID: "principal_1", Role: auth.PrincipalRolePublic, Status: auth.PrincipalStatusActive, Label: "integration-a", CreatedAt: time.Now().UTC()}, granted: true},
		EventIngestor:       ingestor,
	})

	for attempt, wantStatus := range []int{http.StatusCreated, http.StatusOK} {
		req := httptest.NewRequest(http.MethodPost, "/v1/events", strings.NewReader(`{"event_type":"conversation.message","content":"hello"}`))
		req.Header.Set("X-API-Key", "principal-key")
		req.Header.Set("Idempotency-Key", "request-key")
		req.Header.Set("X-Stele-Tenant", "tenant-a")
		req.Header.Set("X-Stele-Project", "project-a")
		req.Header.Set("X-Stele-Namespace", "namespace-a")
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
		if rec.Code != wantStatus {
			t.Fatalf("attempt %d status=%d body=%s, want %d", attempt+1, rec.Code, rec.Body.String(), wantStatus)
		}
	}
	if ingestor.idempotentCalls != 2 || ingestor.ingestCalls != 0 || ingestor.gotPrincipalID != "principal_1" || ingestor.gotKey != "request-key" {
		t.Fatalf("idempotent_calls=%d ingest_calls=%d principal=%q key=%q", ingestor.idempotentCalls, ingestor.ingestCalls, ingestor.gotPrincipalID, ingestor.gotKey)
	}
}

func TestNewHTTPHandlerMapsIdempotencyConflictsToSafeRetryResponses(t *testing.T) {
	tests := []struct {
		name       string
		err        error
		wantStatus int
		wantRetry  string
	}{
		{name: "payload conflict", err: memory.ErrIdempotencyConflict, wantStatus: http.StatusConflict},
		{name: "active claim", err: memory.ErrIdempotencyInProgress, wantStatus: http.StatusConflict, wantRetry: "1"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ingestor := &stubIdempotentEventIngestor{stubEventIngestor: stubEventIngestor{err: tt.err}}
			handler := NewHTTPHandler(HTTPDependencies{
				PrincipalAuthorizer: stubPrincipalAuthorizer{principal: auth.Principal{ID: "principal_1", Role: auth.PrincipalRolePublic, Status: auth.PrincipalStatusActive, CreatedAt: time.Now().UTC()}, granted: true},
				EventIngestor:       ingestor,
			})
			req := httptest.NewRequest(http.MethodPost, "/v1/events", strings.NewReader(`{"event_type":"conversation.message","content":"hello"}`))
			req.Header.Set("X-API-Key", "principal-key")
			req.Header.Set("Idempotency-Key", "request-key")
			req.Header.Set("X-Stele-Tenant", "tenant-a")
			req.Header.Set("X-Stele-Project", "project-a")
			req.Header.Set("X-Stele-Namespace", "namespace-a")
			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, req)
			if rec.Code != tt.wantStatus {
				t.Fatalf("status=%d body=%s, want %d", rec.Code, rec.Body.String(), tt.wantStatus)
			}
			if got := rec.Header().Get("Retry-After"); got != tt.wantRetry {
				t.Fatalf("Retry-After=%q, want %q", got, tt.wantRetry)
			}
			if ingestor.ingestCalls != 0 || ingestor.idempotentCalls != 1 {
				t.Fatalf("idempotent_calls=%d ingest_calls=%d", ingestor.idempotentCalls, ingestor.ingestCalls)
			}
		})
	}
}

func TestNewHTTPHandlerCreatesScopedPrincipalWithOneTimeSecret(t *testing.T) {
	store := &principalAdminStoreHTTPStub{}
	service := auth.NewPrincipalAdminService(store, func() time.Time { return time.Date(2026, 8, 13, 12, 0, 0, 0, time.UTC) }, uniqueTestID)
	handler := NewHTTPHandler(HTTPDependencies{
		PrincipalAuthorizer: stubPrincipalAuthorizer{principal: auth.Principal{ID: "bootstrap-operator", Role: auth.PrincipalRoleAdmin, Status: auth.PrincipalStatusActive, Label: "bootstrap", CreatedAt: time.Now().UTC()}, granted: true},
		PrincipalAdmin:      service,
	})
	req := httptest.NewRequest(http.MethodPost, "/v1/admin/principals", strings.NewReader(`{"role":"admin","label":"first-admin"}`))
	req.Header.Set("X-API-Key", "bootstrap-key")
	req.Header.Set("X-Stele-Tenant", "tenant-a")
	req.Header.Set("X-Stele-Project", "project-a")
	req.Header.Set("X-Stele-Namespace", "namespace-a")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusCreated || !strings.Contains(rec.Body.String(), "credential_secret") || !strings.Contains(rec.Body.String(), "stl_") {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
}

type principalAdminStoreHTTPStub struct{}

func (principalAdminStoreHTTPStub) CreatePrincipal(context.Context, auth.Principal, auth.Credential, []auth.ScopeGrant, auth.AuditRecord) error {
	return nil
}
func (principalAdminStoreHTTPStub) ListPrincipals(context.Context, memory.Scope, int) ([]auth.Principal, error) {
	return nil, nil
}
func (principalAdminStoreHTTPStub) ReadPrincipal(context.Context, memory.Scope, string) (auth.Principal, error) {
	return auth.Principal{}, nil
}
func (principalAdminStoreHTTPStub) ListScopeGrants(context.Context, memory.Scope, string) ([]auth.ScopeGrant, error) {
	return nil, nil
}
func (principalAdminStoreHTTPStub) RotateCredential(context.Context, memory.Scope, string, auth.Credential, auth.AuditRecord) error {
	return nil
}
func (principalAdminStoreHTTPStub) DisablePrincipal(context.Context, memory.Scope, string, time.Time, auth.AuditRecord) error {
	return nil
}
func (principalAdminStoreHTTPStub) ExpirePrincipal(context.Context, memory.Scope, string, time.Time, auth.AuditRecord) error {
	return nil
}
func (principalAdminStoreHTTPStub) CreateScopeGrant(context.Context, memory.Scope, auth.ScopeGrant, auth.AuditRecord) error {
	return nil
}
func (principalAdminStoreHTTPStub) RevokeScopeGrant(context.Context, memory.Scope, string, time.Time, auth.AuditRecord) error {
	return nil
}
func (principalAdminStoreHTTPStub) ListAccessAudit(context.Context, memory.Scope, string, int) ([]auth.AuditRecord, error) {
	return nil, nil
}

var testIDSequence int

func uniqueTestID() string { testIDSequence++; return "test-id-" + strconv.Itoa(testIDSequence) }

func TestNewHTTPHandlerIngestsEvent(t *testing.T) {
	ingestor := &stubEventIngestor{
		eventID: "evt_123",
		admission: &memory.AdmissionPressureReport{
			Decision:  memory.AdmissionPressureDecisionAcceptDegraded,
			Operation: memory.AdmissionPressureOperationIngest,
			Findings: []memory.QualityFinding{{
				Code:      memory.QualityFindingSemanticProjectionDegraded,
				Severity:  memory.QualityFindingSeverityWarning,
				Component: memory.QualityFindingComponentEmbedding,
				Category:  memory.QualityFindingCategorySemanticProjection,
			}},
		},
	}
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

	var response eventIngestResponse
	if err := json.NewDecoder(rec.Body).Decode(&response); err != nil {
		t.Fatalf("json.NewDecoder() error = %v", err)
	}
	if response.EventID != "evt_123" || response.Admission == nil || response.Admission.Decision != memory.AdmissionPressureDecisionAcceptDegraded {
		t.Fatalf("response = %+v, want event id and degraded admission metadata", response)
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

func TestNewHTTPHandlerReturnsUnprocessableWhenAdmissionRejectsIngest(t *testing.T) {
	ingestor := &stubEventIngestor{err: memory.ErrAdmissionRejected}
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

	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusUnprocessableEntity)
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

	req := httptest.NewRequest(http.MethodPost, "/v1/memories/search", bytes.NewBufferString(`{"query":"concise","query_embedding":[0.1,0.2,0.3],"top_k":3,"include_summaries":true,"include_feedback_diagnostics":true,"feedback_aware_ranking":true,"time_from":"2026-06-06T09:00:00Z","time_to":"2026-06-06T12:00:00Z"}`))
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
	if !searcher.gotInput.IncludeFeedbackDiagnostics || !searcher.gotInput.FeedbackAwareRanking {
		t.Fatalf("feedback flags = %v/%v, want per-request diagnostics and ranking", searcher.gotInput.IncludeFeedbackDiagnostics, searcher.gotInput.FeedbackAwareRanking)
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

	req := httptest.NewRequest(http.MethodPost, "/v1/context/assemble", bytes.NewBufferString(`{"query":"preferences","budget":4,"include_experience_insights":true,"include_diagnostics":true,"include_feedback_diagnostics":true,"feedback_aware_ranking":true}`))
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

	if !assembler.gotInput.IncludeExperienceInsights {
		t.Fatal("IncludeExperienceInsights = false, want true")
	}
	if !assembler.gotInput.IncludeDiagnostics {
		t.Fatal("IncludeDiagnostics = false, want true")
	}
	if !assembler.gotInput.IncludeFeedbackDiagnostics || !assembler.gotInput.FeedbackAwareRanking {
		t.Fatalf("feedback flags = %v/%v, want per-request diagnostics and ranking", assembler.gotInput.IncludeFeedbackDiagnostics, assembler.gotInput.FeedbackAwareRanking)
	}
}

func TestNewHTTPHandlerRejectsScopeWideFeedbackRankingPolicy(t *testing.T) {
	searcher := &stubMemorySearcher{}
	assembler := &stubContextAssembler{}
	handler := NewHTTPHandler(HTTPDependencies{
		Readiness:        stubReadinessChecker{},
		APIKeys:          map[string]struct{}{"test-key": {}},
		MemorySearcher:   searcher,
		ContextAssembler: assembler,
	})

	searchReq := httptest.NewRequest(http.MethodPost, "/v1/memories/search", bytes.NewBufferString(`{"query":"concise","feedback_ranking_policy":"scope_default"}`))
	searchReq.Header.Set("Content-Type", "application/json")
	setAPIScopeHeaders(searchReq)
	searchResp := httptest.NewRecorder()
	handler.ServeHTTP(searchResp, searchReq)
	if searchResp.Code != http.StatusBadRequest {
		t.Fatalf("search status = %d body=%s, want 400", searchResp.Code, searchResp.Body.String())
	}
	if searcher.gotInput.Query != "" {
		t.Fatalf("search input = %+v, want rejected before searcher", searcher.gotInput)
	}

	contextReq := httptest.NewRequest(http.MethodPost, "/v1/context/assemble", bytes.NewBufferString(`{"query":"preferences","budget":4,"feedback_ranking_policy":"scope_default"}`))
	contextReq.Header.Set("Content-Type", "application/json")
	setAPIScopeHeaders(contextReq)
	contextResp := httptest.NewRecorder()
	handler.ServeHTTP(contextResp, contextReq)
	if contextResp.Code != http.StatusBadRequest {
		t.Fatalf("context status = %d body=%s, want 400", contextResp.Code, contextResp.Body.String())
	}
	if assembler.gotInput.Query != "" {
		t.Fatalf("context input = %+v, want rejected before assembler", assembler.gotInput)
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

func TestNewHTTPHandlerListsAdminDerivedInsights(t *testing.T) {
	scope := memory.Scope{Tenant: "tenant-a", Project: "project-a", Namespace: "namespace-a"}
	service := &stubDerivedInsightAdminService{
		validateList: true,
		items: []memory.DerivedInsight{
			{
				ID:      "insight_123",
				Scope:   scope,
				Type:    memory.DerivedInsightTypeFailurePattern,
				State:   memory.DerivedInsightStateActive,
				Title:   "Repeated provider failure",
				Summary: "Provider unavailable repeated twice.",
				Confidence: memory.DerivedInsightConfidence{
					Score: 0.75,
				},
				Derivation: memory.DerivedInsightDerivation{
					Source:      "failure_pattern_evaluator",
					Fingerprint: "failure_pattern:fingerprint",
					DerivedAt:   time.Date(2026, 7, 4, 12, 0, 0, 0, time.UTC),
				},
				Evidence: []memory.DerivedInsightEvidenceRef{
					{Kind: memory.DerivedInsightEvidenceKindJobExecution, ID: "job_1", Relation: memory.DerivedInsightEvidenceRelationSupports},
					{Kind: memory.DerivedInsightEvidenceKindJobExecution, ID: "job_2", Relation: memory.DerivedInsightEvidenceRelationSupports},
				},
			},
		},
	}
	handler := NewHTTPHandler(HTTPDependencies{
		AdminAPIKeys:        map[string]struct{}{"admin-key": {}},
		DerivedInsightAdmin: service,
	})

	req := httptest.NewRequest(http.MethodGet, "/v1/admin/derived-insights?type=failure_pattern&state=active&min_confidence=0.5&min_evidence_count=2&include_hidden=true&limit=5", nil)
	req.Header.Set("X-API-Key", "admin-key")
	req.Header.Set("X-Stele-Tenant", scope.Tenant)
	req.Header.Set("X-Stele-Project", scope.Project)
	req.Header.Set("X-Stele-Namespace", scope.Namespace)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body=%s", rec.Code, http.StatusOK, rec.Body.String())
	}
	if service.gotListInput.Scope != scope || service.gotListInput.Type != memory.DerivedInsightTypeFailurePattern || service.gotListInput.State != memory.DerivedInsightStateActive {
		t.Fatalf("got list input = %+v, want scoped filters", service.gotListInput)
	}
	if service.gotListInput.MinConfidence == nil || *service.gotListInput.MinConfidence != 0.5 || service.gotListInput.MinEvidenceCount != 2 || !service.gotListInput.IncludeHidden || service.gotListInput.Limit != 5 {
		t.Fatalf("got list input = %+v, want confidence/evidence/hidden/limit filters", service.gotListInput)
	}

	var payload struct {
		Items []memory.DerivedInsight `json:"items"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&payload); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(payload.Items) != 1 || payload.Items[0].ID != "insight_123" {
		t.Fatalf("items = %+v, want insight_123", payload.Items)
	}
}

func TestNewHTTPHandlerReadsAdminDerivedInsightDetail(t *testing.T) {
	scope := memory.Scope{Tenant: "tenant-a", Project: "project-a", Namespace: "namespace-a"}
	service := &stubDerivedInsightAdminService{
		validateRead: true,
		detail: memory.DerivedInsightDetail{
			Insight: memory.DerivedInsight{
				ID:      "insight_hidden",
				Scope:   scope,
				Type:    memory.DerivedInsightTypeFailurePattern,
				State:   memory.DerivedInsightStateSuppressed,
				Title:   "Repeated provider failure",
				Summary: "Suppressed as noisy.",
				Confidence: memory.DerivedInsightConfidence{
					Score: 0.7,
				},
				Derivation: memory.DerivedInsightDerivation{
					Source:      "failure_pattern_evaluator",
					Fingerprint: "failure_pattern:fingerprint",
					DerivedAt:   time.Date(2026, 7, 4, 12, 0, 0, 0, time.UTC),
				},
			},
			Evidence: []memory.DerivedInsightEvidenceRef{
				{Kind: memory.DerivedInsightEvidenceKindJobExecution, ID: "job_1", Relation: memory.DerivedInsightEvidenceRelationSupports},
			},
			Lifecycle: []memory.DerivedInsightLifecycleRecord{
				{InsightID: "insight_hidden", ToState: memory.DerivedInsightStateSuppressed, Actor: "operator-a", Reason: "noisy", OccurredAt: time.Date(2026, 7, 4, 13, 0, 0, 0, time.UTC)},
			},
		},
	}
	handler := NewHTTPHandler(HTTPDependencies{
		AdminAPIKeys:        map[string]struct{}{"admin-key": {}},
		DerivedInsightAdmin: service,
	})

	req := httptest.NewRequest(http.MethodGet, "/v1/admin/derived-insights/insight_hidden?include_hidden=true", nil)
	req.Header.Set("X-API-Key", "admin-key")
	req.Header.Set("X-Stele-Tenant", scope.Tenant)
	req.Header.Set("X-Stele-Project", scope.Project)
	req.Header.Set("X-Stele-Namespace", scope.Namespace)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body=%s", rec.Code, http.StatusOK, rec.Body.String())
	}
	if service.gotReadInput.ID != "insight_hidden" || !service.gotReadInput.IncludeHidden || service.gotReadInput.Scope != scope {
		t.Fatalf("got read input = %+v, want scoped hidden insight read", service.gotReadInput)
	}

	var payload memory.DerivedInsightDetail
	if err := json.NewDecoder(rec.Body).Decode(&payload); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if payload.Insight.ID != "insight_hidden" || len(payload.Evidence) != 1 || len(payload.Lifecycle) != 1 {
		t.Fatalf("detail = %+v, want insight, evidence, lifecycle", payload)
	}
}

func TestNewHTTPHandlerCreatesAdminDerivedInsightFeedback(t *testing.T) {
	scope := memory.Scope{Tenant: "tenant-a", Project: "project-a", Namespace: "namespace-a"}
	service := &stubDerivedInsightAdminService{
		validateFeedback: true,
		feedback: memory.DerivedInsightFeedback{
			ID:        "feedback_123",
			InsightID: "insight_123",
			Scope:     scope,
			Type:      memory.InsightFeedbackTypeNoisy,
			Actor:     "operator-a",
			Reason:    "too broad",
			CreatedAt: time.Date(2026, 7, 4, 14, 0, 0, 0, time.UTC),
		},
	}
	handler := NewHTTPHandler(HTTPDependencies{
		AdminAPIKeys:        map[string]struct{}{"admin-key": {}},
		DerivedInsightAdmin: service,
	})

	req := httptest.NewRequest(http.MethodPost, "/v1/admin/derived-insights/insight_123/feedback", strings.NewReader(`{"type":"noisy","actor":"operator-a","reason":"too broad","quality_score":0.2}`))
	req.Header.Set("X-API-Key", "admin-key")
	req.Header.Set("X-Stele-Tenant", scope.Tenant)
	req.Header.Set("X-Stele-Project", scope.Project)
	req.Header.Set("X-Stele-Namespace", scope.Namespace)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, want %d; body=%s", rec.Code, http.StatusCreated, rec.Body.String())
	}
	if service.gotCreateFeedback.Scope != scope || service.gotCreateFeedback.InsightID != "insight_123" || service.gotCreateFeedback.Type != memory.InsightFeedbackTypeNoisy {
		t.Fatalf("feedback input = %+v, want scoped noisy feedback", service.gotCreateFeedback)
	}
	if service.gotCreateFeedback.Actor != "operator-a" || service.gotCreateFeedback.Reason != "too broad" || service.gotCreateFeedback.CreatedAt.IsZero() || service.gotCreateFeedback.ID == "" {
		t.Fatalf("feedback input = %+v, want actor/reason/id/created_at", service.gotCreateFeedback)
	}

	var payload memory.DerivedInsightFeedback
	if err := json.NewDecoder(rec.Body).Decode(&payload); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if payload.ID != "feedback_123" {
		t.Fatalf("payload = %+v, want feedback_123", payload)
	}
}

func TestNewHTTPHandlerListsAdminDerivedInsightFeedback(t *testing.T) {
	scope := memory.Scope{Tenant: "tenant-a", Project: "project-a", Namespace: "namespace-a"}
	service := &stubDerivedInsightAdminService{
		validateFeedback: true,
		feedbackItems: []memory.DerivedInsightFeedback{
			{
				ID:        "feedback_123",
				InsightID: "insight_123",
				Scope:     scope,
				Type:      memory.InsightFeedbackTypeUseful,
				Actor:     "operator-a",
				Reason:    "accurate",
				CreatedAt: time.Date(2026, 7, 4, 14, 0, 0, 0, time.UTC),
			},
		},
	}
	handler := NewHTTPHandler(HTTPDependencies{
		AdminAPIKeys:        map[string]struct{}{"admin-key": {}},
		DerivedInsightAdmin: service,
	})

	req := httptest.NewRequest(http.MethodGet, "/v1/admin/derived-insights/insight_123/feedback?type=useful&include_superseded=true&limit=5", nil)
	req.Header.Set("X-API-Key", "admin-key")
	req.Header.Set("X-Stele-Tenant", scope.Tenant)
	req.Header.Set("X-Stele-Project", scope.Project)
	req.Header.Set("X-Stele-Namespace", scope.Namespace)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body=%s", rec.Code, http.StatusOK, rec.Body.String())
	}
	if service.gotListFeedback.Scope != scope || service.gotListFeedback.InsightID != "insight_123" || service.gotListFeedback.Type != memory.InsightFeedbackTypeUseful {
		t.Fatalf("feedback list input = %+v, want scoped useful filter", service.gotListFeedback)
	}
	if !service.gotListFeedback.IncludeSuperseded || service.gotListFeedback.Limit != 5 {
		t.Fatalf("feedback list input = %+v, want include_superseded and limit", service.gotListFeedback)
	}

	var payload struct {
		Items []memory.DerivedInsightFeedback `json:"items"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&payload); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(payload.Items) != 1 || payload.Items[0].ID != "feedback_123" {
		t.Fatalf("payload = %+v, want feedback_123", payload)
	}
}

func TestNewHTTPHandlerSupersedesAdminDerivedInsightFeedback(t *testing.T) {
	scope := memory.Scope{Tenant: "tenant-a", Project: "project-a", Namespace: "namespace-a"}
	service := &stubDerivedInsightAdminService{validateFeedback: true}
	handler := NewHTTPHandler(HTTPDependencies{
		AdminAPIKeys:        map[string]struct{}{"admin-key": {}},
		DerivedInsightAdmin: service,
	})

	req := httptest.NewRequest(http.MethodPost, "/v1/admin/derived-insight-feedback/feedback_123:supersede", strings.NewReader(`{"actor":"operator-b","reason":"replaced by more accurate feedback"}`))
	req.Header.Set("X-API-Key", "admin-key")
	req.Header.Set("X-Stele-Tenant", scope.Tenant)
	req.Header.Set("X-Stele-Project", scope.Project)
	req.Header.Set("X-Stele-Namespace", scope.Namespace)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body=%s", rec.Code, http.StatusOK, rec.Body.String())
	}
	if service.gotSupersede.Scope != scope || service.gotSupersede.FeedbackID != "feedback_123" || service.gotSupersede.Actor != "operator-b" {
		t.Fatalf("supersede input = %+v, want scoped feedback supersede", service.gotSupersede)
	}
	if service.gotSupersede.Reason == "" || service.gotSupersede.SupersededAt.IsZero() {
		t.Fatalf("supersede input = %+v, want reason and superseded_at", service.gotSupersede)
	}
}

func TestNewHTTPHandlerSuppressesAdminDerivedInsight(t *testing.T) {
	scope := memory.Scope{Tenant: "tenant-a", Project: "project-a", Namespace: "namespace-a"}
	service := &stubDerivedInsightAdminService{validateLifecycle: true}
	handler := NewHTTPHandler(HTTPDependencies{
		AdminAPIKeys:        map[string]struct{}{"admin-key": {}},
		DerivedInsightAdmin: service,
	})

	req := httptest.NewRequest(http.MethodPost, "/v1/admin/derived-insights/insight_123:suppress", strings.NewReader(`{"actor":"operator-a","reason":"noisy duplicate"}`))
	req.Header.Set("X-API-Key", "admin-key")
	req.Header.Set("X-Stele-Tenant", scope.Tenant)
	req.Header.Set("X-Stele-Project", scope.Project)
	req.Header.Set("X-Stele-Namespace", scope.Namespace)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body=%s", rec.Code, http.StatusOK, rec.Body.String())
	}
	if service.gotTransition.Scope != scope || service.gotTransition.InsightID != "insight_123" || service.gotTransition.ToState != memory.DerivedInsightStateSuppressed {
		t.Fatalf("transition = %+v, want scoped suppress transition", service.gotTransition)
	}
	if service.gotTransition.Actor != "operator-a" || service.gotTransition.Reason != "noisy duplicate" || service.gotTransition.OccurredAt.IsZero() {
		t.Fatalf("transition = %+v, want actor/reason/occurred_at", service.gotTransition)
	}
}

func TestNewHTTPHandlerPlansAdminDerivedInsightReplayDryRun(t *testing.T) {
	scope := memory.Scope{Tenant: "tenant-a", Project: "project-a", Namespace: "namespace-a"}
	service := &stubDerivedInsightReplayAdminService{
		validate: true,
		plan: memory.DerivedInsightReplayReport{
			RunID: "dry_run",
			Scope: scope,
			Counters: memory.DerivedInsightReplayCounters{
				EvidenceEvaluated: 2,
				Created:           1,
			},
			Decisions: []memory.DerivedInsightReplayDecision{
				{
					InsightType:   memory.DerivedInsightTypeFailurePattern,
					Fingerprint:   "failure_pattern:provider_unavailable",
					Decision:      memory.DerivedInsightReplayDecisionCreate,
					Reason:        memory.DerivedInsightReplayReasonRepeatedEvidence,
					EvidenceCount: 2,
				},
			},
			GeneratedAt: time.Date(2026, 7, 11, 10, 30, 0, 0, time.UTC),
		},
	}
	handler := NewHTTPHandler(HTTPDependencies{
		AdminAPIKeys:              map[string]struct{}{"admin-key": {}},
		DerivedInsightReplayAdmin: service,
	})

	req := httptest.NewRequest(http.MethodPost, "/v1/admin/derived-insight-replays:dry-run", strings.NewReader(`{"insight_types":["failure_pattern"],"evidence_window_start":"2026-07-01T00:00:00Z","evidence_window_end":"2026-07-02T00:00:00Z","evidence_limit":100,"actor":"operator-a","reason":"preview replay","idempotency_key":"dry-run-1"}`))
	req.Header.Set("X-API-Key", "admin-key")
	req.Header.Set("X-Stele-Tenant", scope.Tenant)
	req.Header.Set("X-Stele-Project", scope.Project)
	req.Header.Set("X-Stele-Namespace", scope.Namespace)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body=%s", rec.Code, http.StatusOK, rec.Body.String())
	}
	if service.gotPlanInput.Scope != scope || service.gotPlanInput.Mode != memory.DerivedInsightReplayModeDryRun || service.gotPlanInput.EvidenceLimit != 100 {
		t.Fatalf("plan input = %+v, want scoped bounded dry-run", service.gotPlanInput)
	}
	if len(service.gotPlanInput.InsightTypes) != 1 || service.gotPlanInput.InsightTypes[0] != memory.DerivedInsightTypeFailurePattern {
		t.Fatalf("plan insight types = %+v, want failure_pattern", service.gotPlanInput.InsightTypes)
	}

	var payload memory.DerivedInsightReplayReport
	if err := json.NewDecoder(rec.Body).Decode(&payload); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if payload.Counters.Created != 1 || len(payload.Decisions) != 1 {
		t.Fatalf("payload = %+v, want replay plan", payload)
	}
}

func TestNewHTTPHandlerAppliesAdminDerivedInsightReplay(t *testing.T) {
	scope := memory.Scope{Tenant: "tenant-a", Project: "project-a", Namespace: "namespace-a"}
	run := testHTTPDerivedInsightReplayRun(scope)
	service := &stubDerivedInsightReplayAdminService{
		validate: true,
		run:      run,
	}
	handler := NewHTTPHandler(HTTPDependencies{
		AdminAPIKeys:              map[string]struct{}{"admin-key": {}},
		DerivedInsightReplayAdmin: service,
	})

	req := httptest.NewRequest(http.MethodPost, "/v1/admin/derived-insight-replays", strings.NewReader(`{"insight_types":["failure_pattern","lesson"],"evidence_window_start":"2026-07-01T00:00:00Z","evidence_window_end":"2026-07-02T00:00:00Z","evidence_limit":100,"actor":"operator-a","reason":"apply replay","idempotency_key":"apply-1"}`))
	req.Header.Set("X-API-Key", "admin-key")
	req.Header.Set("X-Stele-Tenant", scope.Tenant)
	req.Header.Set("X-Stele-Project", scope.Project)
	req.Header.Set("X-Stele-Namespace", scope.Namespace)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusAccepted {
		t.Fatalf("status = %d, want %d; body=%s", rec.Code, http.StatusAccepted, rec.Body.String())
	}
	if service.gotApplyInput.Scope != scope || service.gotApplyInput.Mode != memory.DerivedInsightReplayModeApply || service.gotApplyInput.Actor != "operator-a" {
		t.Fatalf("apply input = %+v, want scoped apply", service.gotApplyInput)
	}

	var payload memory.DerivedInsightReplayRun
	if err := json.NewDecoder(rec.Body).Decode(&payload); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if payload.ID != run.ID || payload.Status != memory.DerivedInsightReplayStatusPending {
		t.Fatalf("payload = %+v, want pending replay run", payload)
	}
}

func TestNewHTTPHandlerListsAdminDerivedInsightReplays(t *testing.T) {
	scope := memory.Scope{Tenant: "tenant-a", Project: "project-a", Namespace: "namespace-a"}
	run := testHTTPDerivedInsightReplayRun(scope)
	service := &stubDerivedInsightReplayAdminService{
		validate: true,
		runs:     []memory.DerivedInsightReplayRun{run},
	}
	handler := NewHTTPHandler(HTTPDependencies{
		AdminAPIKeys:              map[string]struct{}{"admin-key": {}},
		DerivedInsightReplayAdmin: service,
	})

	req := httptest.NewRequest(http.MethodGet, "/v1/admin/derived-insight-replays?status=pending&mode=apply&limit=5", nil)
	req.Header.Set("X-API-Key", "admin-key")
	req.Header.Set("X-Stele-Tenant", scope.Tenant)
	req.Header.Set("X-Stele-Project", scope.Project)
	req.Header.Set("X-Stele-Namespace", scope.Namespace)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body=%s", rec.Code, http.StatusOK, rec.Body.String())
	}
	if service.gotListInput.Scope != scope || service.gotListInput.Status != memory.DerivedInsightReplayStatusPending || service.gotListInput.Mode != memory.DerivedInsightReplayModeApply || service.gotListInput.Limit != 5 {
		t.Fatalf("list input = %+v, want scoped replay filters", service.gotListInput)
	}

	var payload struct {
		Items []memory.DerivedInsightReplayRun `json:"items"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&payload); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(payload.Items) != 1 || payload.Items[0].ID != run.ID {
		t.Fatalf("payload = %+v, want replay run", payload)
	}
}

func TestNewHTTPHandlerReadsAdminDerivedInsightReplayReport(t *testing.T) {
	scope := memory.Scope{Tenant: "tenant-a", Project: "project-a", Namespace: "namespace-a"}
	service := &stubDerivedInsightReplayAdminService{
		validate: true,
		report: memory.DerivedInsightReplayReport{
			RunID:       "replay_123",
			Scope:       scope,
			Counters:    memory.DerivedInsightReplayCounters{EvidenceEvaluated: 2, Created: 1},
			GeneratedAt: time.Date(2026, 7, 11, 10, 30, 0, 0, time.UTC),
		},
	}
	handler := NewHTTPHandler(HTTPDependencies{
		AdminAPIKeys:              map[string]struct{}{"admin-key": {}},
		DerivedInsightReplayAdmin: service,
	})

	req := httptest.NewRequest(http.MethodGet, "/v1/admin/derived-insight-replays/replay_123/report", nil)
	req.Header.Set("X-API-Key", "admin-key")
	req.Header.Set("X-Stele-Tenant", scope.Tenant)
	req.Header.Set("X-Stele-Project", scope.Project)
	req.Header.Set("X-Stele-Namespace", scope.Namespace)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body=%s", rec.Code, http.StatusOK, rec.Body.String())
	}
	if service.gotReadInput.Scope != scope || service.gotReadInput.RunID != "replay_123" {
		t.Fatalf("read input = %+v, want scoped report read", service.gotReadInput)
	}

	var payload memory.DerivedInsightReplayReport
	if err := json.NewDecoder(rec.Body).Decode(&payload); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if payload.RunID != "replay_123" || payload.Counters.Created != 1 {
		t.Fatalf("payload = %+v, want replay report", payload)
	}
}

func TestNewHTTPHandlerRejectsAdminDerivedInsightReplayWithoutAdminKey(t *testing.T) {
	scope := memory.Scope{Tenant: "tenant-a", Project: "project-a", Namespace: "namespace-a"}
	service := &stubDerivedInsightReplayAdminService{}
	handler := NewHTTPHandler(HTTPDependencies{
		AdminAPIKeys:              map[string]struct{}{"admin-key": {}},
		DerivedInsightReplayAdmin: service,
	})

	req := httptest.NewRequest(http.MethodPost, "/v1/admin/derived-insight-replays:dry-run", strings.NewReader(`{"insight_types":["failure_pattern"],"evidence_window_start":"2026-07-01T00:00:00Z","evidence_window_end":"2026-07-02T00:00:00Z","evidence_limit":100,"actor":"operator-a","reason":"preview replay"}`))
	req.Header.Set("X-Stele-Tenant", scope.Tenant)
	req.Header.Set("X-Stele-Project", scope.Project)
	req.Header.Set("X-Stele-Namespace", scope.Namespace)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusUnauthorized)
	}
	if service.gotPlanInput.Actor != "" {
		t.Fatalf("service was called with %+v, want auth rejection before service call", service.gotPlanInput)
	}
}

func TestNewHTTPHandlerRejectsAdminDerivedInsightReplayMissingBounds(t *testing.T) {
	scope := memory.Scope{Tenant: "tenant-a", Project: "project-a", Namespace: "namespace-a"}
	service := &stubDerivedInsightReplayAdminService{}
	handler := NewHTTPHandler(HTTPDependencies{
		AdminAPIKeys:              map[string]struct{}{"admin-key": {}},
		DerivedInsightReplayAdmin: service,
	})

	req := httptest.NewRequest(http.MethodPost, "/v1/admin/derived-insight-replays:dry-run", strings.NewReader(`{"insight_types":["failure_pattern"],"evidence_window_start":"2026-07-01T00:00:00Z","evidence_limit":100,"actor":"operator-a","reason":"preview replay"}`))
	req.Header.Set("X-API-Key", "admin-key")
	req.Header.Set("X-Stele-Tenant", scope.Tenant)
	req.Header.Set("X-Stele-Project", scope.Project)
	req.Header.Set("X-Stele-Namespace", scope.Namespace)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d; body=%s", rec.Code, http.StatusBadRequest, rec.Body.String())
	}
	if service.gotPlanInput.Actor != "" {
		t.Fatalf("service was called with %+v, want validation rejection before service call", service.gotPlanInput)
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

func TestNewHTTPHandlerCreatesAdminEmbeddingCutoverPlan(t *testing.T) {
	service := &stubEmbeddingAdminService{
		cutoverPlan: memory.EmbeddingCutoverPlan{
			ID:     "plan_123",
			Scope:  memory.Scope{Tenant: "tenant-a", Project: "project-a", Namespace: "namespace-a"},
			Status: memory.EmbeddingCutoverPlanStatusDraft,
			Target: memory.EmbeddingCutoverTarget{
				Provider:   "openai",
				Model:      "text-embedding-3-small",
				Dimensions: 1536,
			},
			WaveSize:  25,
			CreatedBy: "operator-a",
			CreatedAt: time.Date(2026, 6, 28, 13, 0, 0, 0, time.UTC),
		},
	}
	handler := NewHTTPHandler(HTTPDependencies{
		AdminAPIKeys:       map[string]struct{}{"admin-key": {}},
		EmbeddingAdminRead: service,
	})

	req := httptest.NewRequest(http.MethodPost, "/v1/admin/embedding/cutovers", strings.NewReader(`{"target":{"provider":"openai","model":"text-embedding-3-small","dimensions":1536},"classes":["profile"],"wave_size":25,"reason":"migrate scope"}`))
	setAdminActionHeaders(req)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusCreated)
	}
	if service.gotCreateCutover.Actor != "operator-a" {
		t.Fatalf("actor = %q, want operator-a", service.gotCreateCutover.Actor)
	}
	if service.gotCreateCutover.Scope.Namespace != "namespace-a" {
		t.Fatalf("scope = %+v, want request scope", service.gotCreateCutover.Scope)
	}
	if service.gotCreateCutover.WaveSize != 25 {
		t.Fatalf("wave size = %d, want 25", service.gotCreateCutover.WaveSize)
	}
}

func TestNewHTTPHandlerListsAdminEmbeddingCutoverPlans(t *testing.T) {
	service := &stubEmbeddingAdminService{
		cutoverPlans: []memory.EmbeddingCutoverPlan{
			{
				ID:     "plan_123",
				Scope:  memory.Scope{Tenant: "tenant-a", Project: "project-a", Namespace: "namespace-a"},
				Status: memory.EmbeddingCutoverPlanStatusActive,
				Target: memory.EmbeddingCutoverTarget{Provider: "openai", Model: "text-embedding-3-small", Dimensions: 1536},
				Progress: memory.EmbeddingCutoverProgress{
					Total: 10,
				},
			},
		},
	}
	handler := NewHTTPHandler(HTTPDependencies{
		AdminAPIKeys:       map[string]struct{}{"admin-key": {}},
		EmbeddingAdminRead: service,
	})

	req := httptest.NewRequest(http.MethodGet, "/v1/admin/embedding/cutovers?status=active&limit=5", nil)
	setAdminScopeHeaders(req)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
	if service.gotListCutovers.Status != memory.EmbeddingCutoverPlanStatusActive {
		t.Fatalf("status filter = %q, want active", service.gotListCutovers.Status)
	}

	var payload struct {
		Plans []memory.EmbeddingCutoverPlan `json:"plans"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&payload); err != nil {
		t.Fatalf("json.NewDecoder() error = %v", err)
	}
	if len(payload.Plans) != 1 || payload.Plans[0].ID != "plan_123" {
		t.Fatalf("plans = %+v, want one plan_123", payload.Plans)
	}
}

func TestNewHTTPHandlerRecordsEmbeddingCutoverStateMetrics(t *testing.T) {
	metrics := telemetry.NewMetricsObserver()
	service := &stubEmbeddingAdminService{
		cutoverPlans: []memory.EmbeddingCutoverPlan{
			{
				ID:     "plan_active",
				Scope:  memory.Scope{Tenant: "tenant-a", Project: "project-a", Namespace: "namespace-a"},
				Status: memory.EmbeddingCutoverPlanStatusActive,
				Target: memory.EmbeddingCutoverTarget{Provider: "openai", Model: "text-embedding-3-small", Dimensions: 1536},
				Progress: memory.EmbeddingCutoverProgress{
					Queued:  4,
					Current: 2,
				},
			},
			{
				ID:     "plan_paused",
				Scope:  memory.Scope{Tenant: "tenant-a", Project: "project-a", Namespace: "namespace-a"},
				Status: memory.EmbeddingCutoverPlanStatusPaused,
				Target: memory.EmbeddingCutoverTarget{Provider: "openai", Model: "text-embedding-3-small", Dimensions: 1536},
				Progress: memory.EmbeddingCutoverProgress{
					Failed: 1,
				},
			},
		},
	}
	handler := NewHTTPHandler(HTTPDependencies{
		AdminAPIKeys:       map[string]struct{}{"admin-key": {}},
		EmbeddingAdminRead: service,
		Metrics:            metrics,
	})

	req := httptest.NewRequest(http.MethodGet, "/v1/admin/embedding/cutovers?limit=5", nil)
	setAdminScopeHeaders(req)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}

	metricsReq := httptest.NewRequest(http.MethodGet, "/metrics", nil)
	metricsRec := httptest.NewRecorder()
	handler.ServeHTTP(metricsRec, metricsReq)
	body := metricsRec.Body.String()
	for _, want := range []string{
		`stele_embedding_cutover_plans{status="active"} 1`,
		`stele_embedding_cutover_plans{status="paused"} 1`,
		`stele_embedding_cutover_items{status="queued"} 4`,
		`stele_embedding_cutover_items{status="current"} 2`,
		`stele_embedding_cutover_items{status="failed"} 1`,
	} {
		if !strings.Contains(body, want) {
			t.Fatalf("metrics body missing %q:\n%s", want, body)
		}
	}
}

func TestNewHTTPHandlerReturnsAdminEmbeddingCutoverDetail(t *testing.T) {
	service := &stubEmbeddingAdminService{
		cutoverPlan: memory.EmbeddingCutoverPlan{
			ID:     "plan_123",
			Scope:  memory.Scope{Tenant: "tenant-a", Project: "project-a", Namespace: "namespace-a"},
			Status: memory.EmbeddingCutoverPlanStatusPaused,
			Target: memory.EmbeddingCutoverTarget{Provider: "openai", Model: "text-embedding-3-small", Dimensions: 1536},
		},
	}
	handler := NewHTTPHandler(HTTPDependencies{
		AdminAPIKeys:       map[string]struct{}{"admin-key": {}},
		EmbeddingAdminRead: service,
	})

	req := httptest.NewRequest(http.MethodGet, "/v1/admin/embedding/cutovers/plan_123", nil)
	setAdminScopeHeaders(req)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
	if service.gotReadCutover.PlanID != "plan_123" {
		t.Fatalf("plan id = %q, want plan_123", service.gotReadCutover.PlanID)
	}
}

func TestNewHTTPHandlerPreflightsAdminEmbeddingCutoverPlan(t *testing.T) {
	service := &stubEmbeddingAdminService{
		preflightReport: memory.EmbeddingCutoverPreflightReport{
			Component:     "embedding_cutover",
			Decision:      "allow",
			Target:        memory.EmbeddingCutoverTarget{Provider: "openai", Model: "text-embedding-3-small", Dimensions: 1536},
			Scope:         memory.Scope{Tenant: "tenant-a", Project: "project-a", Namespace: "namespace-a"},
			EligibleTotal: 3,
		},
	}
	handler := NewHTTPHandler(HTTPDependencies{
		AdminAPIKeys:       map[string]struct{}{"admin-key": {}},
		EmbeddingAdminRead: service,
	})

	req := httptest.NewRequest(http.MethodPost, "/v1/admin/embedding/cutovers/plan_123:preflight", nil)
	setAdminScopeHeaders(req)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
	if service.gotPreflight.PlanID != "plan_123" {
		t.Fatalf("preflight plan id = %q, want plan_123", service.gotPreflight.PlanID)
	}
	var payload memory.EmbeddingCutoverPreflightReport
	if err := json.NewDecoder(rec.Body).Decode(&payload); err != nil {
		t.Fatalf("json.NewDecoder() error = %v", err)
	}
	if payload.Decision != "allow" || payload.EligibleTotal != 3 {
		t.Fatalf("payload = %+v, want allow with eligible_total 3", payload)
	}
}

func TestNewHTTPHandlerRecordsCutoverPreflightMetrics(t *testing.T) {
	metrics := telemetry.NewMetricsObserver()
	service := &stubEmbeddingAdminService{
		preflightReport: memory.EmbeddingCutoverPreflightReport{
			Component: "embedding_cutover",
			Decision:  "deny",
			Blockers: []diagnostics.Finding{
				{Severity: diagnostics.SeverityBlocker, Code: "zero_eligible_memory"},
			},
			Target: memory.EmbeddingCutoverTarget{Provider: "openai", Model: "text-embedding-3-small", Dimensions: 1536},
			Scope:  memory.Scope{Tenant: "tenant-a", Project: "project-a", Namespace: "namespace-a"},
		},
	}
	handler := NewHTTPHandler(HTTPDependencies{
		AdminAPIKeys:       map[string]struct{}{"admin-key": {}},
		EmbeddingAdminRead: service,
		Metrics:            metrics,
	})

	req := httptest.NewRequest(http.MethodPost, "/v1/admin/embedding/cutovers/plan_123:preflight", nil)
	setAdminScopeHeaders(req)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("preflight status = %d, want %d", rec.Code, http.StatusOK)
	}

	metricsReq := httptest.NewRequest(http.MethodGet, "/metrics", nil)
	metricsRec := httptest.NewRecorder()
	handler.ServeHTTP(metricsRec, metricsReq)
	if body := metricsRec.Body.String(); !strings.Contains(body, `stele_admission_decisions_total{component="embedding_cutover",decision="deny",operation="preflight"} 1`) {
		t.Fatalf("metrics body missing admission decision:\n%s", body)
	}
}

func TestNewHTTPHandlerAppliesAdminEmbeddingCutoverAction(t *testing.T) {
	service := &stubEmbeddingAdminService{
		cutoverPlan: memory.EmbeddingCutoverPlan{
			ID:     "plan_123",
			Scope:  memory.Scope{Tenant: "tenant-a", Project: "project-a", Namespace: "namespace-a"},
			Status: memory.EmbeddingCutoverPlanStatusActive,
			Target: memory.EmbeddingCutoverTarget{Provider: "openai", Model: "text-embedding-3-small", Dimensions: 1536},
		},
	}
	handler := NewHTTPHandler(HTTPDependencies{
		AdminAPIKeys:       map[string]struct{}{"admin-key": {}},
		EmbeddingAdminRead: service,
	})

	req := httptest.NewRequest(http.MethodPost, "/v1/admin/embedding/cutovers/plan_123:pause", strings.NewReader(`{"reason":"halt next wave"}`))
	setAdminActionHeaders(req)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
	if service.gotApplyCutover.Action != memory.EmbeddingCutoverPlanActionPause {
		t.Fatalf("action = %q, want pause", service.gotApplyCutover.Action)
	}
	if service.gotApplyCutover.Actor != "operator-a" {
		t.Fatalf("actor = %q, want operator-a", service.gotApplyCutover.Actor)
	}
	if service.gotApplyCutover.Scope.Project != "project-a" {
		t.Fatalf("scope = %+v, want request scope", service.gotApplyCutover.Scope)
	}
}

func TestNewHTTPHandlerReturnsAdmissionReportWhenCutoverActivationDenied(t *testing.T) {
	report := memory.EmbeddingCutoverPreflightReport{
		Component: "embedding_cutover",
		Decision:  "deny",
		Blockers: []diagnostics.Finding{
			{Severity: diagnostics.SeverityBlocker, Code: "zero_eligible_memory"},
		},
		Target: memory.EmbeddingCutoverTarget{Provider: "openai", Model: "text-embedding-3-small", Dimensions: 1536},
		Scope:  memory.Scope{Tenant: "tenant-a", Project: "project-a", Namespace: "namespace-a"},
	}
	service := &stubEmbeddingAdminService{
		cutoverErr: memory.EmbeddingCutoverAdmissionError{Report: report},
	}
	handler := NewHTTPHandler(HTTPDependencies{
		AdminAPIKeys:       map[string]struct{}{"admin-key": {}},
		EmbeddingAdminRead: service,
	})

	req := httptest.NewRequest(http.MethodPost, "/v1/admin/embedding/cutovers/plan_123:activate", strings.NewReader(`{"reason":"roll out now"}`))
	setAdminActionHeaders(req)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusUnprocessableEntity)
	}
	var payload memory.EmbeddingCutoverPreflightReport
	if err := json.NewDecoder(rec.Body).Decode(&payload); err != nil {
		t.Fatalf("json.NewDecoder() error = %v", err)
	}
	if payload.Decision != "deny" || len(payload.Blockers) != 1 || payload.Blockers[0].Code != "zero_eligible_memory" {
		t.Fatalf("payload = %+v, want deny report with zero_eligible_memory", payload)
	}
}

func TestNewHTTPHandlerListsAdminEmbeddingRecoveryHistory(t *testing.T) {
	service := &stubEmbeddingAdminService{
		recoveryHistory: []memory.EmbeddingRecoveryRecord{
			{
				ID:            "erl_123",
				MemoryID:      "mem_123",
				Scope:         memory.Scope{Tenant: "tenant-a", Project: "project-a", Namespace: "namespace-a"},
				CutoverPlanID: "plan_123",
				Action:        memory.EmbeddingRecoveryActionRetry,
				Actor:         "operator-a",
				Reason:        "retry now",
				OccurredAt:    time.Date(2026, 6, 28, 12, 0, 0, 0, time.UTC),
			},
		},
	}
	handler := NewHTTPHandler(HTTPDependencies{
		AdminAPIKeys:       map[string]struct{}{"admin-key": {}},
		EmbeddingAdminRead: service,
	})

	req := httptest.NewRequest(http.MethodGet, "/v1/admin/memories/mem_123/embedding/recovery-history?action=retry&actor=operator-a&cutover_plan_id=plan_123&limit=10", nil)
	setAdminScopeHeaders(req)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
	if service.gotRecoveryHistory.MemoryID != "mem_123" {
		t.Fatalf("memory id = %q, want mem_123", service.gotRecoveryHistory.MemoryID)
	}
	if service.gotRecoveryHistory.CutoverPlanID != "plan_123" {
		t.Fatalf("cutover plan id = %q, want plan_123", service.gotRecoveryHistory.CutoverPlanID)
	}

	var payload struct {
		History []memory.EmbeddingRecoveryRecord `json:"history"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&payload); err != nil {
		t.Fatalf("json.NewDecoder() error = %v", err)
	}
	if len(payload.History) != 1 || payload.History[0].ID != "erl_123" {
		t.Fatalf("history = %+v, want one erl_123 record", payload.History)
	}
}

func TestNewHTTPHandlerListsScopeLevelAdminEmbeddingRecoveryHistory(t *testing.T) {
	service := &stubEmbeddingAdminService{
		recoveryHistory: []memory.EmbeddingRecoveryRecord{
			{
				ID:         "erl_234",
				MemoryID:   "mem_234",
				Scope:      memory.Scope{Tenant: "tenant-a", Project: "project-a", Namespace: "namespace-a"},
				Action:     memory.EmbeddingRecoveryActionRequeue,
				Actor:      "operator-b",
				Reason:     "refresh route",
				OccurredAt: time.Date(2026, 6, 28, 13, 0, 0, 0, time.UTC),
			},
		},
	}
	handler := NewHTTPHandler(HTTPDependencies{
		AdminAPIKeys:       map[string]struct{}{"admin-key": {}},
		EmbeddingAdminRead: service,
	})

	req := httptest.NewRequest(http.MethodGet, "/v1/admin/embedding/recovery-history?action=requeue&limit=5", nil)
	setAdminScopeHeaders(req)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
	if service.gotRecoveryHistory.MemoryID != "" {
		t.Fatalf("memory id = %q, want empty for scope history", service.gotRecoveryHistory.MemoryID)
	}
	if service.gotRecoveryHistory.Action != memory.EmbeddingRecoveryActionRequeue {
		t.Fatalf("action = %q, want requeue", service.gotRecoveryHistory.Action)
	}
}

func TestNewHTTPHandlerMapsAdminEmbeddingCutoverErrors(t *testing.T) {
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
			path:       "/v1/admin/embedding/cutovers/plan_123:pause",
			body:       `{"reason":"halt next wave"}`,
			service:    &stubEmbeddingAdminService{cutoverErr: memory.ErrEmbeddingCutoverConflict},
			wantStatus: http.StatusConflict,
			wantBody:   memory.ErrEmbeddingCutoverConflict.Error(),
		},
		{
			name:       "rejected",
			path:       "/v1/admin/embedding/cutovers/plan_123:activate",
			body:       `{"reason":"roll out now"}`,
			service:    &stubEmbeddingAdminService{cutoverErr: memory.ErrEmbeddingCutoverRejected},
			wantStatus: http.StatusUnprocessableEntity,
			wantBody:   memory.ErrEmbeddingCutoverRejected.Error(),
		},
		{
			name:       "invalid target",
			path:       "/v1/admin/embedding/cutovers/pause",
			body:       `{"reason":"halt next wave"}`,
			service:    &stubEmbeddingAdminService{},
			wantStatus: http.StatusBadRequest,
			wantBody:   "invalid embedding cutover action target",
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

func TestNewHTTPHandlerServesAdminScopeProofRuns(t *testing.T) {
	scope := memory.Scope{Tenant: "tenant-a", Project: "project-a", Namespace: "namespace-a"}
	now := time.Date(2026, 7, 11, 19, 30, 0, 0, time.UTC)
	service := &stubScopeProofAdminService{
		run: memory.ScopeProofRun{
			ID:          "proof_1",
			Scope:       scope,
			Status:      memory.ScopeProofStatusPending,
			Verdict:     memory.ScopeProofVerdictPending,
			Checks:      []memory.ScopeProofCheck{memory.ScopeProofCheckIngestion},
			FixtureMode: memory.ScopeProofFixtureModeSmoke,
			Actor:       "operator-a",
			Reason:      "prove scope",
			CreatedAt:   now,
			UpdatedAt:   now,
		},
		runs: []memory.ScopeProofRun{{
			ID:          "proof_1",
			Scope:       scope,
			Status:      memory.ScopeProofStatusPending,
			Verdict:     memory.ScopeProofVerdictPending,
			FixtureMode: memory.ScopeProofFixtureModeSmoke,
			CreatedAt:   now,
			UpdatedAt:   now,
		}},
		report: memory.ScopeProofReport{
			Run:         memory.ScopeProofRun{ID: "proof_1", Scope: scope, Status: memory.ScopeProofStatusPending, Verdict: memory.ScopeProofVerdictPending},
			NextActions: []string{"inspect_proof_steps"},
		},
		validate: true,
	}
	handler := NewHTTPHandler(HTTPDependencies{
		Readiness:       stubReadinessChecker{},
		AdminAPIKeys:    map[string]struct{}{"admin-key": {}},
		ScopeProofAdmin: service,
	})

	createReq := httptest.NewRequest(http.MethodPost, "/v1/admin/scope-proofs", strings.NewReader(`{"checks":["ingestion"],"fixture_mode":"smoke","actor":"operator-a","reason":"prove scope"}`))
	setAdminScopeHeaders(createReq)
	createResp := httptest.NewRecorder()
	handler.ServeHTTP(createResp, createReq)
	if createResp.Code != http.StatusCreated {
		t.Fatalf("create status = %d body=%s, want 201", createResp.Code, createResp.Body.String())
	}
	if service.gotCreateInput.Scope != scope || service.gotCreateInput.Checks[0] != memory.ScopeProofCheckIngestion {
		t.Fatalf("create input = %+v, want scoped ingestion proof", service.gotCreateInput)
	}

	listReq := httptest.NewRequest(http.MethodGet, "/v1/admin/scope-proofs?limit=10", nil)
	setAdminScopeHeaders(listReq)
	listResp := httptest.NewRecorder()
	handler.ServeHTTP(listResp, listReq)
	if listResp.Code != http.StatusOK {
		t.Fatalf("list status = %d body=%s, want 200", listResp.Code, listResp.Body.String())
	}
	if service.gotListInput.Limit != 10 || service.gotListInput.Scope != scope {
		t.Fatalf("list input = %+v, want scoped limit 10", service.gotListInput)
	}

	reportReq := httptest.NewRequest(http.MethodGet, "/v1/admin/scope-proofs/proof_1/report", nil)
	setAdminScopeHeaders(reportReq)
	reportResp := httptest.NewRecorder()
	handler.ServeHTTP(reportResp, reportReq)
	if reportResp.Code != http.StatusOK {
		t.Fatalf("report status = %d body=%s, want 200", reportResp.Code, reportResp.Body.String())
	}
	if service.gotReadInput.ProofID != "proof_1" || service.gotReadInput.Scope != scope {
		t.Fatalf("report input = %+v, want proof_1 scoped", service.gotReadInput)
	}

	rerunReq := httptest.NewRequest(http.MethodPost, "/v1/admin/scope-proofs/proof_1:rerun", strings.NewReader(`{"actor":"operator-b","reason":"verify remediation"}`))
	setAdminScopeHeaders(rerunReq)
	rerunResp := httptest.NewRecorder()
	handler.ServeHTTP(rerunResp, rerunReq)
	if rerunResp.Code != http.StatusCreated {
		t.Fatalf("rerun status = %d body=%s, want 201", rerunResp.Code, rerunResp.Body.String())
	}
	if service.gotRerunInput.ProofID != "proof_1" || service.gotRerunInput.Actor != "operator-b" {
		t.Fatalf("rerun input = %+v, want proof_1 operator-b", service.gotRerunInput)
	}
}

func TestNewHTTPHandlerRejectsAdminScopeProofWithoutAdminKey(t *testing.T) {
	handler := NewHTTPHandler(HTTPDependencies{
		Readiness:       stubReadinessChecker{},
		AdminAPIKeys:    map[string]struct{}{"admin-key": {}},
		ScopeProofAdmin: &stubScopeProofAdminService{},
	})
	req := httptest.NewRequest(http.MethodPost, "/v1/admin/scope-proofs", strings.NewReader(`{"checks":["ingestion"],"fixture_mode":"smoke","actor":"operator-a","reason":"prove scope"}`))
	req.Header.Set("X-Stele-Tenant", "tenant-a")
	req.Header.Set("X-Stele-Project", "project-a")
	req.Header.Set("X-Stele-Namespace", "namespace-a")
	resp := httptest.NewRecorder()

	handler.ServeHTTP(resp, req)

	if resp.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", resp.Code)
	}
}

func TestNewHTTPHandlerServesMemorySessionLoop(t *testing.T) {
	scope := memory.Scope{Tenant: "tenant-a", Project: "project-a", Namespace: "namespace-a"}
	now := time.Date(2026, 7, 11, 20, 45, 0, 0, time.UTC)
	service := &stubMemorySessionService{
		session: memory.MemorySessionRun{
			ID:        "session_1",
			Scope:     scope,
			Status:    memory.MemorySessionStatusActive,
			Verdict:   memory.ScopeProofVerdictPending,
			Actor:     "agent-a",
			Reason:    "serve user turn",
			Metadata:  map[string]any{"integration": "test-agent"},
			CreatedAt: now,
			UpdatedAt: now,
		},
		sessions: []memory.MemorySessionRun{{
			ID:        "session_1",
			Scope:     scope,
			Status:    memory.MemorySessionStatusActive,
			Verdict:   memory.ScopeProofVerdictPending,
			CreatedAt: now,
			UpdatedAt: now,
		}},
		turn: memory.MemorySessionTurn{
			ID:              "turn_1",
			SessionID:       "session_1",
			Scope:           scope,
			Status:          memory.MemorySessionTurnStatusContextAssembled,
			Query:           "remember deployment preference",
			ContextEvidence: map[string]any{"memory_ids": []any{"mem_1"}},
			OutcomeEventIDs: []string{"evt_2"},
			ExpectedRecall:  []string{"evt_2"},
			CreatedAt:       now,
			UpdatedAt:       now,
		},
		verification: memory.MemorySessionVerification{
			ID:             "verification_1",
			SessionID:      "session_1",
			TurnID:         "turn_1",
			Scope:          scope,
			Status:         memory.ScopeProofStepStatusPending,
			Verdict:        memory.ScopeProofVerdictPending,
			ExpectedRecall: []string{"evt_2"},
			CreatedAt:      now,
			UpdatedAt:      now,
		},
		report: memory.MemorySessionReport{
			Session:     memory.MemorySessionRun{ID: "session_1", Scope: scope, Status: memory.MemorySessionStatusActive, Verdict: memory.ScopeProofVerdictPending},
			NextActions: []string{"wait_for_session_verification"},
			FeedbackSummaries: []memory.UsefulnessFeedbackSummary{{
				Subject:          memory.UsefulnessFeedbackSubject{Kind: memory.UsefulnessFeedbackSubjectSession, ID: "session_1"},
				TotalActive:      1,
				PositiveCount:    1,
				EffectiveQuality: memory.UsefulnessQualityPositive,
			}},
		},
		validate: true,
	}
	handler := NewHTTPHandler(HTTPDependencies{
		Readiness:     stubReadinessChecker{},
		APIKeys:       map[string]struct{}{"test-key": {}},
		MemorySession: service,
	})

	createReq := httptest.NewRequest(http.MethodPost, "/v1/memory-sessions", strings.NewReader(`{"actor":"agent-a","reason":"serve user turn","metadata":{"integration":"test-agent"}}`))
	setAPIScopeHeaders(createReq)
	createResp := httptest.NewRecorder()
	handler.ServeHTTP(createResp, createReq)
	if createResp.Code != http.StatusCreated {
		t.Fatalf("create status = %d body=%s, want 201", createResp.Code, createResp.Body.String())
	}
	if service.gotCreateInput.Scope != scope || service.gotCreateInput.Metadata["integration"] != "test-agent" {
		t.Fatalf("create input = %+v, want scoped metadata", service.gotCreateInput)
	}

	listReq := httptest.NewRequest(http.MethodGet, "/v1/memory-sessions?limit=10", nil)
	setAPIScopeHeaders(listReq)
	listResp := httptest.NewRecorder()
	handler.ServeHTTP(listResp, listReq)
	if listResp.Code != http.StatusOK {
		t.Fatalf("list status = %d body=%s, want 200", listResp.Code, listResp.Body.String())
	}
	if service.gotListInput.Scope != scope || service.gotListInput.Limit != 10 {
		t.Fatalf("list input = %+v, want scoped limit 10", service.gotListInput)
	}

	turnReq := httptest.NewRequest(http.MethodPost, "/v1/memory-sessions/session_1/turns", strings.NewReader(`{"idempotency_key":"turn-key-1","query":"remember deployment preference","context_budget":1200,"include_relations":true,"include_experience_insights":true}`))
	setAPIScopeHeaders(turnReq)
	turnResp := httptest.NewRecorder()
	handler.ServeHTTP(turnResp, turnReq)
	if turnResp.Code != http.StatusCreated {
		t.Fatalf("turn status = %d body=%s, want 201", turnResp.Code, turnResp.Body.String())
	}
	if service.gotCreateTurnInput.SessionID != "session_1" || service.gotCreateTurnInput.ContextBudget != 1200 || service.gotCreateTurnInput.IdempotencyKey != "turn-key-1" {
		t.Fatalf("turn input = %+v, want session_1 budget 1200", service.gotCreateTurnInput)
	}

	outcomeReq := httptest.NewRequest(http.MethodPost, "/v1/memory-sessions/session_1/turns/turn_1:outcome", strings.NewReader(`{"idempotency_key":"outcome-key-1","outcome_event_ids":["evt_2"],"event_payloads":[{"event_type":"agent_observation","content":"User prefers staged rollout","metadata":{"source":"test-agent"}}],"expected_recall":["evt_2"]}`))
	setAPIScopeHeaders(outcomeReq)
	outcomeResp := httptest.NewRecorder()
	handler.ServeHTTP(outcomeResp, outcomeReq)
	if outcomeResp.Code != http.StatusOK {
		t.Fatalf("outcome status = %d body=%s, want 200", outcomeResp.Code, outcomeResp.Body.String())
	}
	if service.gotOutcomeInput.TurnID != "turn_1" || service.gotOutcomeInput.IdempotencyKey != "outcome-key-1" || service.gotOutcomeInput.OutcomeEventIDs[0] != "evt_2" || len(service.gotOutcomeInput.OutcomeEventPayloads) != 1 {
		t.Fatalf("outcome input = %+v, want turn_1 evt_2", service.gotOutcomeInput)
	}
	if service.gotOutcomeInput.OutcomeEventPayloads[0].Metadata["source"] != "test-agent" {
		t.Fatalf("outcome payload metadata = %+v, want source preserved", service.gotOutcomeInput.OutcomeEventPayloads[0].Metadata)
	}

	verifyReq := httptest.NewRequest(http.MethodPost, "/v1/memory-sessions/session_1:verify", strings.NewReader(`{"turn_id":"turn_1","expected_recall":["evt_2"]}`))
	setAPIScopeHeaders(verifyReq)
	verifyResp := httptest.NewRecorder()
	handler.ServeHTTP(verifyResp, verifyReq)
	if verifyResp.Code != http.StatusAccepted {
		t.Fatalf("verify status = %d body=%s, want 202", verifyResp.Code, verifyResp.Body.String())
	}
	if service.gotVerificationInput.SessionID != "session_1" || service.gotVerificationInput.ExpectedRecall[0] != "evt_2" {
		t.Fatalf("verification input = %+v, want session_1 evt_2", service.gotVerificationInput)
	}

	reportReq := httptest.NewRequest(http.MethodGet, "/v1/memory-sessions/session_1/report", nil)
	setAPIScopeHeaders(reportReq)
	reportResp := httptest.NewRecorder()
	handler.ServeHTTP(reportResp, reportReq)
	if reportResp.Code != http.StatusOK {
		t.Fatalf("report status = %d body=%s, want 200", reportResp.Code, reportResp.Body.String())
	}
	if service.gotReadInput.SessionID != "session_1" || service.gotReadInput.Scope != scope {
		t.Fatalf("report input = %+v, want scoped session_1", service.gotReadInput)
	}
	if !strings.Contains(reportResp.Body.String(), `"feedback_summaries"`) || !strings.Contains(reportResp.Body.String(), `"effective_quality":"positive"`) {
		t.Fatalf("report body = %s, want bounded feedback summaries", reportResp.Body.String())
	}
}

func TestNewHTTPHandlerRejectsMemorySessionWithoutAPIKey(t *testing.T) {
	handler := NewHTTPHandler(HTTPDependencies{
		Readiness:     stubReadinessChecker{},
		APIKeys:       map[string]struct{}{"test-key": {}},
		MemorySession: &stubMemorySessionService{},
	})
	req := httptest.NewRequest(http.MethodPost, "/v1/memory-sessions", strings.NewReader(`{"actor":"agent-a","reason":"serve user turn"}`))
	req.Header.Set("X-Stele-Tenant", "tenant-a")
	req.Header.Set("X-Stele-Project", "project-a")
	req.Header.Set("X-Stele-Namespace", "namespace-a")
	resp := httptest.NewRecorder()

	handler.ServeHTTP(resp, req)

	if resp.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", resp.Code)
	}
}

func TestNewHTTPHandlerRejectsOutOfScopeMemorySessionReport(t *testing.T) {
	service := &stubMemorySessionService{err: pgx.ErrNoRows}
	handler := NewHTTPHandler(HTTPDependencies{
		Readiness:     stubReadinessChecker{},
		APIKeys:       map[string]struct{}{"test-key": {}},
		MemorySession: service,
	})
	req := httptest.NewRequest(http.MethodGet, "/v1/memory-sessions/session_other/report", nil)
	setAPIScopeHeaders(req)
	resp := httptest.NewRecorder()

	handler.ServeHTTP(resp, req)

	if resp.Code != http.StatusNotFound {
		t.Fatalf("status = %d body=%s, want 404", resp.Code, resp.Body.String())
	}
	if service.gotReadInput.SessionID != "session_other" {
		t.Fatalf("read input = %+v, want session_other scoped", service.gotReadInput)
	}
}

func TestNewHTTPHandlerServesUsefulnessFeedbackAPI(t *testing.T) {
	scope := memory.Scope{Tenant: "tenant-a", Project: "project-a", Namespace: "namespace-a"}
	now := time.Date(2026, 7, 11, 12, 30, 0, 0, time.UTC)
	service := &stubUsefulnessFeedbackService{
		created: memory.UsefulnessFeedback{
			ID:               "feedback_1",
			Scope:            scope,
			Type:             memory.UsefulnessFeedbackTypeUseful,
			SourceSurface:    memory.UsefulnessFeedbackSourceSession,
			TaskEvaluationID: "task_eval_1",
			Subjects: []memory.UsefulnessFeedbackSubject{{
				Kind: memory.UsefulnessFeedbackSubjectMemory,
				ID:   "mem_1",
			}},
			Actor:     "agent-a",
			Reason:    "helped answer",
			CreatedAt: now,
		},
		items: []memory.UsefulnessFeedback{{
			ID:               "feedback_1",
			Scope:            scope,
			Type:             memory.UsefulnessFeedbackTypeUseful,
			SourceSurface:    memory.UsefulnessFeedbackSourceSession,
			TaskEvaluationID: "task_eval_1",
			Actor:            "agent-a",
			Reason:           "helped answer",
			CreatedAt:        now,
		}},
		read: memory.UsefulnessFeedback{
			ID:               "feedback_1",
			Scope:            scope,
			Type:             memory.UsefulnessFeedbackTypeUseful,
			SourceSurface:    memory.UsefulnessFeedbackSourceSession,
			TaskEvaluationID: "task_eval_1",
			Actor:            "agent-a",
			Reason:           "helped answer",
			CreatedAt:        now,
		},
		summary: memory.UsefulnessFeedbackSummary{
			Subject:          memory.UsefulnessFeedbackSubject{Kind: memory.UsefulnessFeedbackSubjectMemory, ID: "mem_1"},
			Counts:           map[memory.UsefulnessFeedbackType]int{memory.UsefulnessFeedbackTypeUseful: 1},
			TotalActive:      1,
			PositiveCount:    1,
			EffectiveQuality: memory.UsefulnessQualityPositive,
			LastFeedbackAt:   now,
		},
		validate: true,
	}
	handler := NewHTTPHandler(HTTPDependencies{
		Readiness:          stubReadinessChecker{},
		APIKeys:            map[string]struct{}{"test-key": {}},
		AdminAPIKeys:       map[string]struct{}{"admin-key": {}},
		UsefulnessFeedback: service,
	})

	createReq := httptest.NewRequest(http.MethodPost, "/v1/usefulness-feedback", strings.NewReader(`{"type":"useful","source_surface":"session","task_evaluation_id":"task_eval_1","subjects":[{"kind":"memory","id":"mem_1"}],"actor":"agent-a","reason":"helped answer","idempotency_key":"idem-1","metadata":{"session_id":"session_1"}}`))
	setAPIScopeHeaders(createReq)
	createResp := httptest.NewRecorder()
	handler.ServeHTTP(createResp, createReq)
	if createResp.Code != http.StatusCreated {
		t.Fatalf("create status = %d body=%s, want 201", createResp.Code, createResp.Body.String())
	}
	if service.gotCreateInput.Scope != scope || service.gotCreateInput.Type != memory.UsefulnessFeedbackTypeUseful || service.gotCreateInput.Subjects[0].ID != "mem_1" || service.gotCreateInput.TaskEvaluationID != "task_eval_1" || service.gotCreateInput.IdempotencyKey != "idem-1" {
		t.Fatalf("create input = %+v, want scoped memory feedback", service.gotCreateInput)
	}

	listReq := httptest.NewRequest(http.MethodGet, "/v1/admin/usefulness-feedback?type=useful&include_superseded=true&limit=25", nil)
	setAdminScopeHeaders(listReq)
	listResp := httptest.NewRecorder()
	handler.ServeHTTP(listResp, listReq)
	if listResp.Code != http.StatusOK {
		t.Fatalf("list status = %d body=%s, want 200", listResp.Code, listResp.Body.String())
	}
	if service.gotListInput.Scope != scope || service.gotListInput.Type != memory.UsefulnessFeedbackTypeUseful || !service.gotListInput.IncludeSuperseded || service.gotListInput.Limit != 25 {
		t.Fatalf("list input = %+v, want scoped admin list filters", service.gotListInput)
	}

	readReq := httptest.NewRequest(http.MethodGet, "/v1/admin/usefulness-feedback/feedback_1", nil)
	setAdminScopeHeaders(readReq)
	readResp := httptest.NewRecorder()
	handler.ServeHTTP(readResp, readReq)
	if readResp.Code != http.StatusOK {
		t.Fatalf("read status = %d body=%s, want 200", readResp.Code, readResp.Body.String())
	}
	if service.gotReadInput.FeedbackID != "feedback_1" || service.gotReadInput.Scope != scope {
		t.Fatalf("read input = %+v, want scoped feedback_1", service.gotReadInput)
	}

	summaryReq := httptest.NewRequest(http.MethodGet, "/v1/admin/usefulness-feedback/summary?subject_kind=memory&subject_id=mem_1", nil)
	setAdminScopeHeaders(summaryReq)
	summaryResp := httptest.NewRecorder()
	handler.ServeHTTP(summaryResp, summaryReq)
	if summaryResp.Code != http.StatusOK {
		t.Fatalf("summary status = %d body=%s, want 200", summaryResp.Code, summaryResp.Body.String())
	}
	if service.gotSummaryInput.Subject.Kind != memory.UsefulnessFeedbackSubjectMemory || service.gotSummaryInput.Subject.ID != "mem_1" {
		t.Fatalf("summary input = %+v, want memory subject", service.gotSummaryInput)
	}

	supersedeReq := httptest.NewRequest(http.MethodPost, "/v1/admin/usefulness-feedback/feedback_1:supersede", strings.NewReader(`{"actor":"operator-a","reason":"incorrect signal"}`))
	setAdminScopeHeaders(supersedeReq)
	supersedeResp := httptest.NewRecorder()
	handler.ServeHTTP(supersedeResp, supersedeReq)
	if supersedeResp.Code != http.StatusAccepted {
		t.Fatalf("supersede status = %d body=%s, want 202", supersedeResp.Code, supersedeResp.Body.String())
	}
	if service.gotSupersedeInput.FeedbackID != "feedback_1" || service.gotSupersedeInput.Actor != "operator-a" || service.gotSupersedeInput.Scope != scope {
		t.Fatalf("supersede input = %+v, want scoped supersession", service.gotSupersedeInput)
	}
}

func TestNewHTTPHandlerValidatesUsefulnessFeedbackCreate(t *testing.T) {
	service := &stubUsefulnessFeedbackService{validate: true}
	handler := NewHTTPHandler(HTTPDependencies{
		Readiness:          stubReadinessChecker{},
		APIKeys:            map[string]struct{}{"test-key": {}},
		UsefulnessFeedback: service,
	})

	invalidTypeReq := httptest.NewRequest(http.MethodPost, "/v1/usefulness-feedback", strings.NewReader(`{"type":"free_form","source_surface":"session","subjects":[{"kind":"memory","id":"mem_1"}],"actor":"agent-a","reason":"helped answer"}`))
	setAPIScopeHeaders(invalidTypeReq)
	invalidTypeResp := httptest.NewRecorder()
	handler.ServeHTTP(invalidTypeResp, invalidTypeReq)
	if invalidTypeResp.Code != http.StatusBadRequest {
		t.Fatalf("invalid type status = %d body=%s, want 400", invalidTypeResp.Code, invalidTypeResp.Body.String())
	}

	knownExpectedReq := httptest.NewRequest(http.MethodPost, "/v1/usefulness-feedback", strings.NewReader(`{"type":"missing_expected","source_surface":"verification","subjects":[{"kind":"expected_recall","expected_recall_target":{"kind":"memory","id":"mem_1"}}],"actor":"agent-a","reason":"expected memory was absent","idempotency_key":"expected-1"}`))
	setAPIScopeHeaders(knownExpectedReq)
	knownExpectedResp := httptest.NewRecorder()
	handler.ServeHTTP(knownExpectedResp, knownExpectedReq)
	if knownExpectedResp.Code != http.StatusCreated {
		t.Fatalf("known expected status = %d body=%s, want 201", knownExpectedResp.Code, knownExpectedResp.Body.String())
	}
	if service.gotCreateInput.Subjects[0].ExpectedRecallTarget.Kind != memory.ExpectedRecallTargetMemory || service.gotCreateInput.Subjects[0].ExpectedRecallTarget.ID != "mem_1" {
		t.Fatalf("expected recall subject = %+v, want known memory target", service.gotCreateInput.Subjects[0])
	}

	opaqueExpectedReq := httptest.NewRequest(http.MethodPost, "/v1/usefulness-feedback", strings.NewReader(`{"type":"missing_expected","source_surface":"verification","subjects":[{"kind":"expected_recall","expected_recall_target":{"kind":"opaque","opaque_token":"caller-expected-fact"}}],"actor":"agent-a","reason":"opaque expectation absent","idempotency_key":"expected-opaque-1"}`))
	setAPIScopeHeaders(opaqueExpectedReq)
	opaqueExpectedResp := httptest.NewRecorder()
	handler.ServeHTTP(opaqueExpectedResp, opaqueExpectedReq)
	if opaqueExpectedResp.Code != http.StatusCreated {
		t.Fatalf("opaque expected status = %d body=%s, want 201", opaqueExpectedResp.Code, opaqueExpectedResp.Body.String())
	}
	if service.gotCreateInput.Subjects[0].ExpectedRecallTarget.Kind != memory.ExpectedRecallTargetOpaque || service.gotCreateInput.Subjects[0].ExpectedRecallTarget.OpaqueToken != "caller-expected-fact" || service.gotCreateInput.Subjects[0].ExpectedRecallTarget.ID != "" {
		t.Fatalf("expected recall subject = %+v, want opaque token only", service.gotCreateInput.Subjects[0])
	}

	invalidExpectedReq := httptest.NewRequest(http.MethodPost, "/v1/usefulness-feedback", strings.NewReader(`{"type":"missing_expected","source_surface":"verification","subjects":[{"kind":"expected_recall","expected_recall_target":{"kind":"opaque","id":"mem_1","opaque_token":"caller-expected-fact"}}],"actor":"agent-a","reason":"invalid target"}`))
	setAPIScopeHeaders(invalidExpectedReq)
	invalidExpectedResp := httptest.NewRecorder()
	handler.ServeHTTP(invalidExpectedResp, invalidExpectedReq)
	if invalidExpectedResp.Code != http.StatusBadRequest {
		t.Fatalf("invalid expected status = %d body=%s, want 400", invalidExpectedResp.Code, invalidExpectedResp.Body.String())
	}
}

func TestNewHTTPHandlerPassesUsefulnessFeedbackIdempotency(t *testing.T) {
	service := &stubUsefulnessFeedbackService{validate: true}
	handler := NewHTTPHandler(HTTPDependencies{
		Readiness:          stubReadinessChecker{},
		APIKeys:            map[string]struct{}{"test-key": {}},
		UsefulnessFeedback: service,
	})
	body := `{"type":"useful","source_surface":"session","subjects":[{"kind":"memory","id":"mem_1"}],"actor":"agent-a","reason":"helped answer","idempotency_key":"idem-1"}`

	firstReq := httptest.NewRequest(http.MethodPost, "/v1/usefulness-feedback", strings.NewReader(body))
	setAPIScopeHeaders(firstReq)
	firstResp := httptest.NewRecorder()
	handler.ServeHTTP(firstResp, firstReq)
	if firstResp.Code != http.StatusCreated {
		t.Fatalf("first status = %d body=%s, want 201", firstResp.Code, firstResp.Body.String())
	}

	secondReq := httptest.NewRequest(http.MethodPost, "/v1/usefulness-feedback", strings.NewReader(body))
	setAPIScopeHeaders(secondReq)
	secondResp := httptest.NewRecorder()
	handler.ServeHTTP(secondResp, secondReq)
	if secondResp.Code != http.StatusCreated {
		t.Fatalf("second status = %d body=%s, want 201", secondResp.Code, secondResp.Body.String())
	}
	if service.createCalls != 2 || service.gotCreateInput.IdempotencyKey != "idem-1" {
		t.Fatalf("create calls/idempotency = %d/%q, want duplicate-safe idempotency passed through", service.createCalls, service.gotCreateInput.IdempotencyKey)
	}
}

func TestNewHTTPHandlerFiltersUsefulnessFeedbackAdminListBySubject(t *testing.T) {
	service := &stubUsefulnessFeedbackService{validate: true}
	handler := NewHTTPHandler(HTTPDependencies{
		Readiness:          stubReadinessChecker{},
		AdminAPIKeys:       map[string]struct{}{"admin-key": {}},
		UsefulnessFeedback: service,
	})

	listReq := httptest.NewRequest(http.MethodGet, "/v1/admin/usefulness-feedback?subject_kind=expected_recall&expected_recall_kind=memory&expected_recall_id=mem_1&include_superseded=false", nil)
	setAdminScopeHeaders(listReq)
	listResp := httptest.NewRecorder()
	handler.ServeHTTP(listResp, listReq)
	if listResp.Code != http.StatusOK {
		t.Fatalf("list status = %d body=%s, want 200", listResp.Code, listResp.Body.String())
	}
	if service.gotListInput.Subject.Kind != memory.UsefulnessFeedbackSubjectExpectedRecall || service.gotListInput.Subject.ExpectedRecallTarget.Kind != memory.ExpectedRecallTargetMemory || service.gotListInput.Subject.ExpectedRecallTarget.ID != "mem_1" {
		t.Fatalf("list subject = %+v, want expected recall memory target", service.gotListInput.Subject)
	}

	invalidBoolReq := httptest.NewRequest(http.MethodGet, "/v1/admin/usefulness-feedback?include_superseded=definitely", nil)
	setAdminScopeHeaders(invalidBoolReq)
	invalidBoolResp := httptest.NewRecorder()
	handler.ServeHTTP(invalidBoolResp, invalidBoolReq)
	if invalidBoolResp.Code != http.StatusBadRequest {
		t.Fatalf("invalid bool status = %d body=%s, want 400", invalidBoolResp.Code, invalidBoolResp.Body.String())
	}
}

func TestNewHTTPHandlerKeepsUsefulnessFeedbackSummaryAdminOnly(t *testing.T) {
	service := &stubUsefulnessFeedbackService{validate: true}
	handler := NewHTTPHandler(HTTPDependencies{
		Readiness:          stubReadinessChecker{},
		APIKeys:            map[string]struct{}{"test-key": {}},
		AdminAPIKeys:       map[string]struct{}{"admin-key": {}},
		UsefulnessFeedback: service,
	})

	req := httptest.NewRequest(http.MethodGet, "/v1/admin/usefulness-feedback/summary?subject_kind=memory&subject_id=mem_1", nil)
	setAPIScopeHeaders(req)
	resp := httptest.NewRecorder()
	handler.ServeHTTP(resp, req)
	if resp.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d body=%s, want 401", resp.Code, resp.Body.String())
	}
	if service.gotSummaryInput.Subject.Kind != "" {
		t.Fatalf("summary input = %+v, want public caller blocked before service call", service.gotSummaryInput)
	}
}

func TestNewHTTPHandlerHidesOutOfScopeUsefulnessFeedbackDetails(t *testing.T) {
	service := &stubUsefulnessFeedbackService{err: pgx.ErrNoRows, validate: true}
	handler := NewHTTPHandler(HTTPDependencies{
		Readiness:          stubReadinessChecker{},
		AdminAPIKeys:       map[string]struct{}{"admin-key": {}},
		UsefulnessFeedback: service,
	})

	req := httptest.NewRequest(http.MethodGet, "/v1/admin/usefulness-feedback/secret_feedback_1", nil)
	setAdminScopeHeaders(req)
	resp := httptest.NewRecorder()
	handler.ServeHTTP(resp, req)
	if resp.Code != http.StatusNotFound {
		t.Fatalf("status = %d body=%s, want 404", resp.Code, resp.Body.String())
	}
	if strings.Contains(resp.Body.String(), "secret_feedback_1") {
		t.Fatalf("body leaks target id: %s", resp.Body.String())
	}
}

func TestNewHTTPHandlerServesTaskEvaluationAPIs(t *testing.T) {
	scope := memory.Scope{Tenant: "tenant-a", Project: "project-a", Namespace: "namespace-a"}
	now := time.Date(2026, 7, 12, 9, 0, 0, 0, time.UTC)
	service := &stubTaskEvaluationService{
		created: memory.TaskEvaluation{
			ID:              "task_eval_1",
			Scope:           scope,
			Objective:       "validate external task",
			SuccessCriteria: []string{"report evidence"},
			Verdict:         memory.TaskEvaluationVerdictSucceeded,
			Evidence:        []memory.TaskEvidenceLink{{Kind: memory.TaskEvidenceTargetSession, ID: "session_1"}},
			Actor:           "operator-a",
			Reason:          "record task outcome",
			CreatedAt:       now,
			UpdatedAt:       now,
		},
		items: []memory.TaskEvaluation{{
			ID:              "task_eval_1",
			Scope:           scope,
			Objective:       "validate external task",
			SuccessCriteria: []string{"report evidence"},
			Verdict:         memory.TaskEvaluationVerdictSucceeded,
			Evidence:        []memory.TaskEvidenceLink{{Kind: memory.TaskEvidenceTargetSession, ID: "session_1"}},
			Actor:           "operator-a",
			Reason:          "record task outcome",
			CreatedAt:       now,
			UpdatedAt:       now,
		}},
		summary: memory.TaskEvaluationSummary{
			Scope:             scope,
			TotalEvaluations:  1,
			ActiveEvaluations: 1,
			VerdictCounts: map[memory.TaskEvaluationVerdict]int{
				memory.TaskEvaluationVerdictSucceeded: 1,
			},
			ContributionCounts: map[memory.TaskContributionCategory]int{},
		},
		validate: true,
	}
	handler := NewHTTPHandler(HTTPDependencies{
		Readiness:       stubReadinessChecker{},
		APIKeys:         map[string]struct{}{"test-key": {}},
		AdminAPIKeys:    map[string]struct{}{"admin-key": {}},
		TaskEvaluations: service,
	})

	createReq := httptest.NewRequest(http.MethodPost, "/v1/task-evaluations", strings.NewReader(`{"objective":"validate external task","success_criteria":["report evidence"],"verdict":"succeeded","evidence":[{"kind":"session","id":"session_1"}],"actor":"operator-a","reason":"record task outcome","metadata":{"session_id":"session_1"}}`))
	setAPIScopeHeaders(createReq)
	createResp := httptest.NewRecorder()
	handler.ServeHTTP(createResp, createReq)
	if createResp.Code != http.StatusCreated {
		t.Fatalf("create status = %d body=%s, want 201", createResp.Code, createResp.Body.String())
	}
	if service.gotCreateInput.Scope != scope || service.gotCreateInput.Objective != "validate external task" || service.gotCreateInput.Verdict != memory.TaskEvaluationVerdictSucceeded {
		t.Fatalf("create input = %+v, want scoped task evaluation", service.gotCreateInput)
	}

	reportReq := httptest.NewRequest(http.MethodGet, "/v1/task-evaluations/task_eval_1/report", nil)
	setAPIScopeHeaders(reportReq)
	reportResp := httptest.NewRecorder()
	handler.ServeHTTP(reportResp, reportReq)
	if reportResp.Code != http.StatusOK {
		t.Fatalf("report status = %d body=%s, want 200", reportResp.Code, reportResp.Body.String())
	}
	if !strings.Contains(reportResp.Body.String(), `"linked_session_ids"`) || !strings.Contains(reportResp.Body.String(), `"evaluation"`) {
		t.Fatalf("report body = %s, want bounded task report", reportResp.Body.String())
	}

	adminListReq := httptest.NewRequest(http.MethodGet, "/v1/admin/task-evaluations?verdict=succeeded&limit=25", nil)
	setAdminScopeHeaders(adminListReq)
	adminListResp := httptest.NewRecorder()
	handler.ServeHTTP(adminListResp, adminListReq)
	if adminListResp.Code != http.StatusOK {
		t.Fatalf("admin list status = %d body=%s, want 200", adminListResp.Code, adminListResp.Body.String())
	}
	if service.gotListInput.Verdict != memory.TaskEvaluationVerdictSucceeded || service.gotListInput.Limit != 25 {
		t.Fatalf("admin list input = %+v, want verdict filter and limit", service.gotListInput)
	}

	adminReadReq := httptest.NewRequest(http.MethodGet, "/v1/admin/task-evaluations/task_eval_1", nil)
	setAdminScopeHeaders(adminReadReq)
	adminReadResp := httptest.NewRecorder()
	handler.ServeHTTP(adminReadResp, adminReadReq)
	if adminReadResp.Code != http.StatusOK {
		t.Fatalf("admin read status = %d body=%s, want 200", adminReadResp.Code, adminReadResp.Body.String())
	}
	if service.gotReadInput.EvaluationID != "task_eval_1" {
		t.Fatalf("admin read input = %+v, want task_eval_1", service.gotReadInput)
	}

	adminSupersedeReq := httptest.NewRequest(http.MethodPost, "/v1/admin/task-evaluations/task_eval_1/supersede", strings.NewReader(`{"actor":"operator-b","reason":"corrected verdict"}`))
	setAdminScopeHeaders(adminSupersedeReq)
	adminSupersedeResp := httptest.NewRecorder()
	handler.ServeHTTP(adminSupersedeResp, adminSupersedeReq)
	if adminSupersedeResp.Code != http.StatusAccepted {
		t.Fatalf("admin supersede status = %d body=%s, want 202", adminSupersedeResp.Code, adminSupersedeResp.Body.String())
	}
	if service.gotSupersedeInput.EvaluationID != "task_eval_1" || service.gotSupersedeInput.Actor != "operator-b" {
		t.Fatalf("supersede input = %+v, want task_eval_1 operator-b", service.gotSupersedeInput)
	}

	adminSummaryReq := httptest.NewRequest(http.MethodGet, "/v1/admin/task-evaluations/summary?evidence_target_kind=session&evidence_target_id=session_1", nil)
	setAdminScopeHeaders(adminSummaryReq)
	adminSummaryResp := httptest.NewRecorder()
	handler.ServeHTTP(adminSummaryResp, adminSummaryReq)
	if adminSummaryResp.Code != http.StatusOK {
		t.Fatalf("summary status = %d body=%s, want 200", adminSummaryResp.Code, adminSummaryResp.Body.String())
	}
	if service.gotSummaryInput.EvidenceTargetKind != memory.TaskEvidenceTargetSession || service.gotSummaryInput.EvidenceTargetID != "session_1" {
		t.Fatalf("summary input = %+v, want session filter", service.gotSummaryInput)
	}
}

func TestNewHTTPHandlerServesRankingRolloutAPIs(t *testing.T) {
	scope := memory.Scope{Tenant: "tenant-a", Project: "project-a", Namespace: "namespace-a"}
	now := time.Date(2026, 7, 12, 10, 0, 0, 0, time.UTC)
	service := &stubRankingRolloutAdminService{
		policy: memory.RankingRolloutPolicy{
			ID:              "policy_1",
			Scope:           scope,
			Status:          memory.RankingRolloutPolicyStatusDraft,
			Mode:            memory.RankingRolloutModeDryRun,
			Surfaces:        []memory.RankingRolloutSurface{memory.RankingRolloutSurfaceSearch},
			SignalSources:   []memory.RankingRolloutSignalSource{memory.RankingRolloutSignalSourceTaskEvaluations},
			ThresholdStatus: memory.RankingRolloutThresholdStatusSatisfied,
			EvidenceMinimum: 1,
			Actor:           "operator-a",
			Reason:          "create policy",
			CreatedAt:       now,
			UpdatedAt:       now,
		},
		policies: []memory.RankingRolloutPolicy{{ID: "policy_1", Scope: scope}},
		dryRun: memory.RankingRolloutDryRun{
			ID:                  "dry_run_1",
			PolicyID:            "policy_1",
			Scope:               scope,
			Surface:             memory.RankingRolloutSurfaceSearch,
			SignalSource:        memory.RankingRolloutSignalSourceTaskEvaluations,
			ThresholdStatus:     memory.RankingRolloutThresholdStatusSatisfied,
			BaselineRank:        1,
			AdjustedRank:        0,
			ReasonCodes:         []memory.RankingRolloutImpactReasonCode{memory.RankingRolloutImpactReasonCodeBaselineRetained},
			EvidenceCount:       1,
			HiddenEvidenceCount: 0,
			CreatedAt:           now,
		},
		impact: []memory.RankingRolloutImpactEntry{{ID: "impact_1", PolicyID: "policy_1", Scope: scope, Surface: memory.RankingRolloutSurfaceSearch, SignalSource: memory.RankingRolloutSignalSourceTaskEvaluations, SubjectKind: "memory", SubjectID: "mem_1", BaselineRank: 1, AdjustedRank: 0, ReasonCode: memory.RankingRolloutImpactReasonCodeSubjectBoosted, EvidenceCount: 1, HiddenEvidence: false, CreatedAt: now}},
	}
	handler := NewHTTPHandler(HTTPDependencies{
		Readiness:      stubReadinessChecker{},
		AdminAPIKeys:   map[string]struct{}{"admin-key": {}},
		RankingRollout: service,
	})

	createReq := httptest.NewRequest(http.MethodPost, "/v1/admin/ranking-rollouts", strings.NewReader(`{"id":"policy_1","status":"draft","mode":"dry_run","surfaces":["search"],"signal_sources":["task_evaluations"],"threshold_status":"satisfied","evidence_minimum":1,"actor":"operator-a","reason":"create policy"}`))
	setAdminScopeHeaders(createReq)
	createResp := httptest.NewRecorder()
	handler.ServeHTTP(createResp, createReq)
	if createResp.Code != http.StatusCreated {
		t.Fatalf("create status = %d body=%s, want 201", createResp.Code, createResp.Body.String())
	}

	listReq := httptest.NewRequest(http.MethodGet, "/v1/admin/ranking-rollouts", nil)
	setAdminScopeHeaders(listReq)
	listResp := httptest.NewRecorder()
	handler.ServeHTTP(listResp, listReq)
	if listResp.Code != http.StatusOK {
		t.Fatalf("list status = %d body=%s, want 200", listResp.Code, listResp.Body.String())
	}

	readReq := httptest.NewRequest(http.MethodGet, "/v1/admin/ranking-rollouts/policy_1", nil)
	setAdminScopeHeaders(readReq)
	readResp := httptest.NewRecorder()
	handler.ServeHTTP(readResp, readReq)
	if readResp.Code != http.StatusOK {
		t.Fatalf("read status = %d body=%s, want 200", readResp.Code, readResp.Body.String())
	}

	dryRunReq := httptest.NewRequest(http.MethodPost, "/v1/admin/ranking-rollouts/policy_1/dry-run", nil)
	setAdminScopeHeaders(dryRunReq)
	dryRunResp := httptest.NewRecorder()
	handler.ServeHTTP(dryRunResp, dryRunReq)
	if dryRunResp.Code != http.StatusOK {
		t.Fatalf("dry-run status = %d body=%s, want 200", dryRunResp.Code, dryRunResp.Body.String())
	}

	activateReq := httptest.NewRequest(http.MethodPost, "/v1/admin/ranking-rollouts/policy_1/activate", strings.NewReader(`{"actor":"operator-b","reason":"activate after dry-run"}`))
	setAdminScopeHeaders(activateReq)
	activateResp := httptest.NewRecorder()
	handler.ServeHTTP(activateResp, activateReq)
	if activateResp.Code != http.StatusAccepted {
		t.Fatalf("activate status = %d body=%s, want 202", activateResp.Code, activateResp.Body.String())
	}

	rollbackReq := httptest.NewRequest(http.MethodPost, "/v1/admin/ranking-rollouts/policy_1/rollback", strings.NewReader(`{"actor":"operator-c","reason":"rollback degraded ranking"}`))
	setAdminScopeHeaders(rollbackReq)
	rollbackResp := httptest.NewRecorder()
	handler.ServeHTTP(rollbackResp, rollbackReq)
	if rollbackResp.Code != http.StatusAccepted {
		t.Fatalf("rollback status = %d body=%s, want 202", rollbackResp.Code, rollbackResp.Body.String())
	}

	impactReq := httptest.NewRequest(http.MethodGet, "/v1/admin/ranking-rollouts/policy_1/impact", nil)
	setAdminScopeHeaders(impactReq)
	impactResp := httptest.NewRecorder()
	handler.ServeHTTP(impactResp, impactReq)
	if impactResp.Code != http.StatusOK {
		t.Fatalf("impact status = %d body=%s, want 200", impactResp.Code, impactResp.Body.String())
	}

	if service.gotCreate.Scope != scope || service.gotCreate.ID != "policy_1" {
		t.Fatalf("create input = %+v, want scoped create", service.gotCreate)
	}
	if service.gotList.Scope != scope || service.gotRead.Scope != scope || service.gotRead.PolicyID != "policy_1" {
		t.Fatalf("read/list input = %+v %+v, want scoped policy", service.gotList, service.gotRead)
	}
	if service.gotDryRun.PolicyID != "policy_1" || service.gotDryRun.Surface != memory.RankingRolloutSurfaceSearch {
		t.Fatalf("dry-run input = %+v, want search dry-run", service.gotDryRun)
	}
	if service.gotActivate.PolicyID != "policy_1" || service.gotRollback.PolicyID != "policy_1" {
		t.Fatalf("action inputs = %+v %+v, want policy_1", service.gotActivate, service.gotRollback)
	}
	if service.gotImpact.PolicyID != "policy_1" || service.gotImpact.Scope != scope {
		t.Fatalf("impact input = %+v, want scoped impact", service.gotImpact)
	}
}

func TestNewHTTPHandlerServesAssuranceAdminAPIs(t *testing.T) {
	scope := memory.Scope{Tenant: "tenant-a", Project: "project-a", Namespace: "namespace-a"}
	now := time.Date(2026, 7, 18, 10, 0, 0, 0, time.UTC)
	service := &stubAssuranceAdminService{
		healthEvaluation: assurance.HealthEvaluation{
			ID:        "health_eval_1",
			Scope:     scope,
			Status:    assurance.HealthStatusHealthy,
			Severity:  assurance.SeverityInfo,
			Reason:    assurance.ReasonRuntimeReady,
			CreatedAt: now,
		},
		healthEvaluations: []assurance.HealthEvaluation{{ID: "health_eval_1", Scope: scope}},
		incident: assurance.Incident{
			ID:               "incident_1",
			Scope:            scope,
			Status:           assurance.IncidentStatusAcknowledged,
			Severity:         assurance.SeverityWarning,
			Component:        assurance.ComponentBacklog,
			Reason:           assurance.ReasonBacklogPressure,
			DeduplicationKey: "backlog-pressure",
			OpenedAt:         now,
			UpdatedAt:        now,
		},
		incidents: []assurance.Incident{{ID: "incident_1", Scope: scope, Status: assurance.IncidentStatusOpen}},
		alertCandidate: assurance.AlertCandidate{
			ID:               "alert_1",
			Scope:            scope,
			IncidentID:       "incident_1",
			Severity:         assurance.SeverityWarning,
			Component:        assurance.ComponentBacklog,
			Reason:           assurance.ReasonBacklogPressure,
			DeduplicationKey: "alert-dedupe",
			DeliveryPolicy:   "default",
			Payload:          map[string]any{"webhook_url": "REDACTED", "authorization": "REDACTED", "reason": "backlog_pressure"},
			CreatedAt:        now,
		},
		alertCandidates: []assurance.AlertCandidate{{ID: "alert_1", Scope: scope}},
		alertAttempts: []assurance.AlertDeliveryAttempt{{
			ID:               "attempt_1",
			AlertCandidateID: "alert_1",
			Scope:            scope,
			Adapter:          assurance.AlertAdapterWebhook,
			Result:           assurance.AlertDeliveryResultFailed,
			FailureCategory:  "request_failed",
			Attempt:          1,
			PayloadHash:      "hash_1",
			AttemptedAt:      now,
		}},
		conformanceProfile: assurance.ConformanceProfile{
			ID:     "profile_1",
			Scope:  scope,
			Status: assurance.ConformanceProfileStatusActive,
			ExpectedEvidence: []assurance.ExpectedEvidence{{
				Kind:            assurance.ExpectedEvidenceSession,
				MinimumCount:    1,
				FreshnessWindow: time.Hour,
			}},
			Actor:     "operator-a",
			Reason:    "validate integration",
			CreatedAt: now,
			UpdatedAt: now,
		},
		conformanceProfiles: []assurance.ConformanceProfile{{ID: "profile_1", Scope: scope, Status: assurance.ConformanceProfileStatusActive}},
		conformanceRun: assurance.ConformanceRun{
			ID:             "run_1",
			ProfileID:      "profile_1",
			Scope:          scope,
			Result:         assurance.ConformanceResultFailed,
			EvidenceCounts: map[string]any{"session": 0},
			StartedAt:      now,
			FinishedAt:     now,
			CreatedAt:      now,
		},
		conformanceRuns: []assurance.ConformanceRun{{ID: "run_1", ProfileID: "profile_1", Scope: scope}},
		conformanceDiagnostics: []assurance.MissingEvidenceDiagnostic{{
			ID:               "diag_1",
			ConformanceRunID: "run_1",
			Scope:            scope,
			EvidenceKind:     assurance.ExpectedEvidenceSession,
			Category:         assurance.MissingEvidenceSessionWithoutOutcome,
			ReadinessImpact:  assurance.ReadinessStatusDegraded,
			CreatedAt:        now,
		}},
		readinessReport: assurance.ReadinessReport{
			ID:                 "readiness_1",
			Scope:              scope,
			Status:             assurance.ReadinessStatusDegraded,
			HealthEvaluationID: "health_eval_1",
			ConformanceRunID:   "run_1",
			ComponentSummary:   map[string]any{"conformance_status": "degraded"},
			RecommendedActions: []assurance.RunbookHintCategory{assurance.RunbookHintReviewConformanceProfile},
			GeneratedAt:        now,
			CreatedAt:          now,
		},
		readinessReports: []assurance.ReadinessReport{{ID: "readiness_1", Scope: scope}},
		recoveryVerification: assurance.RecoveryVerification{
			ID:              "recovery_1",
			Scope:           scope,
			Target:          assurance.RecoveryVerificationTargetIncident,
			TargetID:        "incident_1",
			Status:          assurance.HealthStatusHealthy,
			CheckedSurfaces: []string{"health_evaluation", "conformance_run"},
			ResultCategory:  "recovered",
			LinkedEvidence:  map[string]any{"health_evaluation_id": "health_eval_1"},
			Actor:           "operator-a",
			Reason:          "verify remediation",
			CreatedAt:       now,
			VerifiedAt:      now,
		},
		recoveryVerifications: []assurance.RecoveryVerification{{ID: "recovery_1", Scope: scope}},
	}
	handler := NewHTTPHandler(HTTPDependencies{
		Readiness:      stubReadinessChecker{},
		AdminAPIKeys:   map[string]struct{}{"admin-key": {}},
		AssuranceAdmin: service,
	})

	healthCreateReq := httptest.NewRequest(http.MethodPost, "/v1/admin/assurance/health-evaluations", strings.NewReader(`{"observed_at":"2026-07-18T10:00:00Z","runtime_readiness":{"status":"healthy","severity":"info","reason":"runtime_ready","evidence":{"ready":true}}}`))
	setAdminScopeHeaders(healthCreateReq)
	healthCreateResp := httptest.NewRecorder()
	handler.ServeHTTP(healthCreateResp, healthCreateReq)
	if healthCreateResp.Code != http.StatusCreated {
		t.Fatalf("health create status = %d body=%s, want 201", healthCreateResp.Code, healthCreateResp.Body.String())
	}
	if service.gotHealthCreate.Scope != scope || service.gotHealthCreate.RuntimeReadiness.Status != assurance.HealthStatusHealthy {
		t.Fatalf("health create input = %+v, want scoped healthy runtime observation", service.gotHealthCreate)
	}

	healthListReq := httptest.NewRequest(http.MethodGet, "/v1/admin/assurance/health-evaluations", nil)
	setAdminScopeHeaders(healthListReq)
	healthListResp := httptest.NewRecorder()
	handler.ServeHTTP(healthListResp, healthListReq)
	if healthListResp.Code != http.StatusOK || !strings.Contains(healthListResp.Body.String(), `"health_evaluations"`) {
		t.Fatalf("health list status=%d body=%s, want health_evaluations", healthListResp.Code, healthListResp.Body.String())
	}

	healthReadReq := httptest.NewRequest(http.MethodGet, "/v1/admin/assurance/health-evaluations/health_eval_1", nil)
	setAdminScopeHeaders(healthReadReq)
	healthReadResp := httptest.NewRecorder()
	handler.ServeHTTP(healthReadResp, healthReadReq)
	if healthReadResp.Code != http.StatusOK {
		t.Fatalf("health read status = %d body=%s, want 200", healthReadResp.Code, healthReadResp.Body.String())
	}
	if service.gotHealthRead.EvaluationID != "health_eval_1" || service.gotHealthRead.Scope != scope {
		t.Fatalf("health read input = %+v, want scoped health_eval_1", service.gotHealthRead)
	}

	incidentListReq := httptest.NewRequest(http.MethodGet, "/v1/admin/assurance/incidents?status=open&limit=25", nil)
	setAdminScopeHeaders(incidentListReq)
	incidentListResp := httptest.NewRecorder()
	handler.ServeHTTP(incidentListResp, incidentListReq)
	if incidentListResp.Code != http.StatusOK {
		t.Fatalf("incident list status = %d body=%s, want 200", incidentListResp.Code, incidentListResp.Body.String())
	}
	if service.gotIncidentList.Status != assurance.IncidentStatusOpen || service.gotIncidentList.Limit != 25 {
		t.Fatalf("incident list input = %+v, want open status and limit", service.gotIncidentList)
	}

	incidentReadReq := httptest.NewRequest(http.MethodGet, "/v1/admin/assurance/incidents/incident_1", nil)
	setAdminScopeHeaders(incidentReadReq)
	incidentReadResp := httptest.NewRecorder()
	handler.ServeHTTP(incidentReadResp, incidentReadReq)
	if incidentReadResp.Code != http.StatusOK {
		t.Fatalf("incident read status = %d body=%s, want 200", incidentReadResp.Code, incidentReadResp.Body.String())
	}

	incidentActionReq := httptest.NewRequest(http.MethodPost, "/v1/admin/assurance/incidents/incident_1/acknowledge", strings.NewReader(`{"actor":"operator-a","reason":"investigating"}`))
	setAdminScopeHeaders(incidentActionReq)
	incidentActionResp := httptest.NewRecorder()
	handler.ServeHTTP(incidentActionResp, incidentActionReq)
	if incidentActionResp.Code != http.StatusAccepted {
		t.Fatalf("incident action status = %d body=%s, want 202", incidentActionResp.Code, incidentActionResp.Body.String())
	}
	if service.gotIncidentAction.IncidentID != "incident_1" || service.gotIncidentAction.Action != assurance.IncidentActionAcknowledge || service.gotIncidentAction.Actor != "operator-a" {
		t.Fatalf("incident action input = %+v, want acknowledge by operator-a", service.gotIncidentAction)
	}

	alertListReq := httptest.NewRequest(http.MethodGet, "/v1/admin/assurance/alert-candidates", nil)
	setAdminScopeHeaders(alertListReq)
	alertListResp := httptest.NewRecorder()
	handler.ServeHTTP(alertListResp, alertListReq)
	if alertListResp.Code != http.StatusOK || !strings.Contains(alertListResp.Body.String(), `"alert_candidates"`) {
		t.Fatalf("alert list status=%d body=%s, want alert_candidates", alertListResp.Code, alertListResp.Body.String())
	}

	alertReadReq := httptest.NewRequest(http.MethodGet, "/v1/admin/assurance/alert-candidates/alert_1", nil)
	setAdminScopeHeaders(alertReadReq)
	alertReadResp := httptest.NewRecorder()
	handler.ServeHTTP(alertReadResp, alertReadReq)
	if alertReadResp.Code != http.StatusOK {
		t.Fatalf("alert read status = %d body=%s, want 200", alertReadResp.Code, alertReadResp.Body.String())
	}
	if strings.Contains(alertReadResp.Body.String(), "https://hooks.example.test") || strings.Contains(alertReadResp.Body.String(), "Bearer") {
		t.Fatalf("alert detail body = %s, leaked delivery secret", alertReadResp.Body.String())
	}

	attemptsReq := httptest.NewRequest(http.MethodGet, "/v1/admin/assurance/alert-candidates/alert_1/delivery-attempts", nil)
	setAdminScopeHeaders(attemptsReq)
	attemptsResp := httptest.NewRecorder()
	handler.ServeHTTP(attemptsResp, attemptsReq)
	if attemptsResp.Code != http.StatusOK || !strings.Contains(attemptsResp.Body.String(), `"delivery_attempts"`) {
		t.Fatalf("attempts status=%d body=%s, want delivery_attempts", attemptsResp.Code, attemptsResp.Body.String())
	}
	if service.gotAlertAttempts.AlertCandidateID != "alert_1" {
		t.Fatalf("alert attempts input = %+v, want alert_1", service.gotAlertAttempts)
	}

	profileCreateReq := httptest.NewRequest(http.MethodPost, "/v1/admin/assurance/conformance-profiles", strings.NewReader(`{"id":"profile_1","expected_evidence":[{"kind":"session","minimum_count":1,"freshness_window":"1h"}],"actor":"operator-a","reason":"validate integration"}`))
	setAdminScopeHeaders(profileCreateReq)
	profileCreateResp := httptest.NewRecorder()
	handler.ServeHTTP(profileCreateResp, profileCreateReq)
	if profileCreateResp.Code != http.StatusCreated {
		t.Fatalf("profile create status = %d body=%s, want 201", profileCreateResp.Code, profileCreateResp.Body.String())
	}
	if service.gotProfileCreate.ID != "profile_1" || service.gotProfileCreate.Scope != scope || service.gotProfileCreate.ExpectedEvidence[0].FreshnessWindow != time.Hour {
		t.Fatalf("profile create input = %+v, want scoped profile with 1h freshness", service.gotProfileCreate)
	}

	profileListReq := httptest.NewRequest(http.MethodGet, "/v1/admin/assurance/conformance-profiles?status=active", nil)
	setAdminScopeHeaders(profileListReq)
	profileListResp := httptest.NewRecorder()
	handler.ServeHTTP(profileListResp, profileListReq)
	if profileListResp.Code != http.StatusOK {
		t.Fatalf("profile list status = %d body=%s, want 200", profileListResp.Code, profileListResp.Body.String())
	}
	if service.gotProfileList.Status != assurance.ConformanceProfileStatusActive {
		t.Fatalf("profile list input = %+v, want active status", service.gotProfileList)
	}

	profileReadReq := httptest.NewRequest(http.MethodGet, "/v1/admin/assurance/conformance-profiles/profile_1", nil)
	setAdminScopeHeaders(profileReadReq)
	profileReadResp := httptest.NewRecorder()
	handler.ServeHTTP(profileReadResp, profileReadReq)
	if profileReadResp.Code != http.StatusOK {
		t.Fatalf("profile read status = %d body=%s, want 200", profileReadResp.Code, profileReadResp.Body.String())
	}

	profileUpdateReq := httptest.NewRequest(http.MethodPatch, "/v1/admin/assurance/conformance-profiles/profile_1", strings.NewReader(`{"expected_evidence":[{"kind":"context","minimum_count":2,"freshness_window":"2h"}],"actor":"operator-b","reason":"tighten checks"}`))
	setAdminScopeHeaders(profileUpdateReq)
	profileUpdateResp := httptest.NewRecorder()
	handler.ServeHTTP(profileUpdateResp, profileUpdateReq)
	if profileUpdateResp.Code != http.StatusOK {
		t.Fatalf("profile update status = %d body=%s, want 200", profileUpdateResp.Code, profileUpdateResp.Body.String())
	}
	if service.gotProfileUpdate.ProfileID != "profile_1" || service.gotProfileUpdate.ExpectedEvidence[0].Kind != assurance.ExpectedEvidenceContext {
		t.Fatalf("profile update input = %+v, want context evidence update", service.gotProfileUpdate)
	}

	profileDisableReq := httptest.NewRequest(http.MethodPost, "/v1/admin/assurance/conformance-profiles/profile_1/disable", strings.NewReader(`{"actor":"operator-c","reason":"retired integration"}`))
	setAdminScopeHeaders(profileDisableReq)
	profileDisableResp := httptest.NewRecorder()
	handler.ServeHTTP(profileDisableResp, profileDisableReq)
	if profileDisableResp.Code != http.StatusAccepted {
		t.Fatalf("profile disable status = %d body=%s, want 202", profileDisableResp.Code, profileDisableResp.Body.String())
	}
	if service.gotProfileDisable.ProfileID != "profile_1" || service.gotProfileDisable.Actor != "operator-c" {
		t.Fatalf("profile disable input = %+v, want operator-c", service.gotProfileDisable)
	}

	runCreateReq := httptest.NewRequest(http.MethodPost, "/v1/admin/assurance/conformance-runs", strings.NewReader(`{"profile_id":"profile_1","started_at":"2026-07-18T10:00:00Z"}`))
	setAdminScopeHeaders(runCreateReq)
	runCreateResp := httptest.NewRecorder()
	handler.ServeHTTP(runCreateResp, runCreateReq)
	if runCreateResp.Code != http.StatusCreated || !strings.Contains(runCreateResp.Body.String(), `"diagnostics"`) {
		t.Fatalf("run create status=%d body=%s, want 201 with diagnostics", runCreateResp.Code, runCreateResp.Body.String())
	}
	if service.gotRunCreate.ProfileID != "profile_1" || !service.gotRunCreate.StartedAt.Equal(now) {
		t.Fatalf("run create input = %+v, want profile_1 at now", service.gotRunCreate)
	}

	runListReq := httptest.NewRequest(http.MethodGet, "/v1/admin/assurance/conformance-runs?profile_id=profile_1", nil)
	setAdminScopeHeaders(runListReq)
	runListResp := httptest.NewRecorder()
	handler.ServeHTTP(runListResp, runListReq)
	if runListResp.Code != http.StatusOK {
		t.Fatalf("run list status = %d body=%s, want 200", runListResp.Code, runListResp.Body.String())
	}
	if service.gotRunList.ProfileID != "profile_1" {
		t.Fatalf("run list input = %+v, want profile filter", service.gotRunList)
	}

	runReadReq := httptest.NewRequest(http.MethodGet, "/v1/admin/assurance/conformance-runs/run_1", nil)
	setAdminScopeHeaders(runReadReq)
	runReadResp := httptest.NewRecorder()
	handler.ServeHTTP(runReadResp, runReadReq)
	if runReadResp.Code != http.StatusOK {
		t.Fatalf("run read status = %d body=%s, want 200", runReadResp.Code, runReadResp.Body.String())
	}

	readinessCreateReq := httptest.NewRequest(http.MethodPost, "/v1/admin/assurance/readiness-reports", strings.NewReader(`{"generated_at":"2026-07-18T10:00:00Z"}`))
	setAdminScopeHeaders(readinessCreateReq)
	readinessCreateResp := httptest.NewRecorder()
	handler.ServeHTTP(readinessCreateResp, readinessCreateReq)
	if readinessCreateResp.Code != http.StatusCreated {
		t.Fatalf("readiness create status = %d body=%s, want 201", readinessCreateResp.Code, readinessCreateResp.Body.String())
	}
	if service.gotReadinessCreate.Scope != scope || !service.gotReadinessCreate.GeneratedAt.Equal(now) {
		t.Fatalf("readiness create input = %+v, want scoped now", service.gotReadinessCreate)
	}

	readinessListReq := httptest.NewRequest(http.MethodGet, "/v1/admin/assurance/readiness-reports", nil)
	setAdminScopeHeaders(readinessListReq)
	readinessListResp := httptest.NewRecorder()
	handler.ServeHTTP(readinessListResp, readinessListReq)
	if readinessListResp.Code != http.StatusOK || !strings.Contains(readinessListResp.Body.String(), `"readiness_reports"`) {
		t.Fatalf("readiness list status=%d body=%s, want readiness_reports", readinessListResp.Code, readinessListResp.Body.String())
	}

	readinessReadReq := httptest.NewRequest(http.MethodGet, "/v1/admin/assurance/readiness-reports/readiness_1", nil)
	setAdminScopeHeaders(readinessReadReq)
	readinessReadResp := httptest.NewRecorder()
	handler.ServeHTTP(readinessReadResp, readinessReadReq)
	if readinessReadResp.Code != http.StatusOK {
		t.Fatalf("readiness read status = %d body=%s, want 200", readinessReadResp.Code, readinessReadResp.Body.String())
	}

	recoveryCreateReq := httptest.NewRequest(http.MethodPost, "/v1/admin/assurance/recovery-verifications", strings.NewReader(`{"target":"incident","target_id":"incident_1","status":"healthy","checked_surfaces":["health_evaluation","conformance_run"],"result_category":"recovered","linked_evidence":{"health_evaluation_id":"health_eval_1"},"actor":"operator-a","reason":"verify remediation","verified_at":"2026-07-18T10:00:00Z"}`))
	setAdminScopeHeaders(recoveryCreateReq)
	recoveryCreateResp := httptest.NewRecorder()
	handler.ServeHTTP(recoveryCreateResp, recoveryCreateReq)
	if recoveryCreateResp.Code != http.StatusCreated {
		t.Fatalf("recovery create status = %d body=%s, want 201", recoveryCreateResp.Code, recoveryCreateResp.Body.String())
	}
	if service.gotRecoveryCreate.Target != assurance.RecoveryVerificationTargetIncident || service.gotRecoveryCreate.TargetID != "incident_1" || service.gotRecoveryCreate.Actor != "operator-a" {
		t.Fatalf("recovery create input = %+v, want incident verification", service.gotRecoveryCreate)
	}

	recoveryListReq := httptest.NewRequest(http.MethodGet, "/v1/admin/assurance/recovery-verifications", nil)
	setAdminScopeHeaders(recoveryListReq)
	recoveryListResp := httptest.NewRecorder()
	handler.ServeHTTP(recoveryListResp, recoveryListReq)
	if recoveryListResp.Code != http.StatusOK || !strings.Contains(recoveryListResp.Body.String(), `"recovery_verifications"`) {
		t.Fatalf("recovery list status=%d body=%s, want recovery_verifications", recoveryListResp.Code, recoveryListResp.Body.String())
	}

	recoveryReadReq := httptest.NewRequest(http.MethodGet, "/v1/admin/assurance/recovery-verifications/recovery_1", nil)
	setAdminScopeHeaders(recoveryReadReq)
	recoveryReadResp := httptest.NewRecorder()
	handler.ServeHTTP(recoveryReadResp, recoveryReadReq)
	if recoveryReadResp.Code != http.StatusOK {
		t.Fatalf("recovery read status = %d body=%s, want 200", recoveryReadResp.Code, recoveryReadResp.Body.String())
	}
	if service.gotRecoveryRead.RecordID != "recovery_1" {
		t.Fatalf("recovery read input = %+v, want recovery_1", service.gotRecoveryRead)
	}
}

func TestNewHTTPHandlerRejectsAssuranceAdminWithoutAdminKey(t *testing.T) {
	handler := NewHTTPHandler(HTTPDependencies{
		Readiness:      stubReadinessChecker{},
		AdminAPIKeys:   map[string]struct{}{"admin-key": {}},
		AssuranceAdmin: &stubAssuranceAdminService{},
	})

	req := httptest.NewRequest(http.MethodGet, "/v1/admin/assurance/incidents", nil)
	setAPIScopeHeaders(req)
	resp := httptest.NewRecorder()
	handler.ServeHTTP(resp, req)
	if resp.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", resp.Code)
	}
}

func TestNewHTTPHandlerRejectsAssuranceAdminWithoutScope(t *testing.T) {
	service := &stubAssuranceAdminService{}
	handler := NewHTTPHandler(HTTPDependencies{
		Readiness:      stubReadinessChecker{},
		AdminAPIKeys:   map[string]struct{}{"admin-key": {}},
		AssuranceAdmin: service,
	})

	req := httptest.NewRequest(http.MethodGet, "/v1/admin/assurance/alert-candidates", nil)
	req.Header.Set("X-API-Key", "admin-key")
	resp := httptest.NewRecorder()
	handler.ServeHTTP(resp, req)
	if resp.Code != http.StatusBadRequest {
		t.Fatalf("status = %d body=%s, want 400 for missing scoped boundary", resp.Code, resp.Body.String())
	}
	if service.gotAlertRead.AlertCandidateID != "" || service.gotAlertAttempts.AlertCandidateID != "" || service.gotIncidentList.Scope != (memory.Scope{}) {
		t.Fatalf("assurance service inputs = alert:%+v attempts:%+v incidents:%+v, want no out-of-scope service call", service.gotAlertRead, service.gotAlertAttempts, service.gotIncidentList)
	}
}

func TestNewHTTPHandlerRejectsInvalidTaskEvaluationVerdict(t *testing.T) {
	service := &stubTaskEvaluationService{validate: true}
	handler := NewHTTPHandler(HTTPDependencies{
		Readiness:       stubReadinessChecker{},
		APIKeys:         map[string]struct{}{"test-key": {}},
		TaskEvaluations: service,
	})

	req := httptest.NewRequest(http.MethodPost, "/v1/task-evaluations", strings.NewReader(`{"objective":"validate","success_criteria":["report evidence"],"verdict":"free_form","evidence":[{"kind":"session","id":"session_1"}],"actor":"operator-a","reason":"record task outcome"}`))
	setAPIScopeHeaders(req)
	resp := httptest.NewRecorder()
	handler.ServeHTTP(resp, req)
	if resp.Code != http.StatusBadRequest {
		t.Fatalf("status = %d body=%s, want 400", resp.Code, resp.Body.String())
	}
}

func TestNewHTTPHandlerRejectsInvalidTaskEvaluationContributionCategory(t *testing.T) {
	service := &stubTaskEvaluationService{validate: true}
	handler := NewHTTPHandler(HTTPDependencies{
		Readiness:       stubReadinessChecker{},
		APIKeys:         map[string]struct{}{"test-key": {}},
		TaskEvaluations: service,
	})

	req := httptest.NewRequest(http.MethodPost, "/v1/task-evaluations", strings.NewReader(`{"objective":"validate","success_criteria":["report evidence"],"verdict":"succeeded","contribution_categories":["free_form"],"evidence":[{"kind":"session","id":"session_1"}],"actor":"operator-a","reason":"record task outcome"}`))
	setAPIScopeHeaders(req)
	resp := httptest.NewRecorder()
	handler.ServeHTTP(resp, req)
	if resp.Code != http.StatusBadRequest {
		t.Fatalf("status = %d body=%s, want 400", resp.Code, resp.Body.String())
	}
}

func TestNewHTTPHandlerPassesOpaqueTaskEvaluationEvidence(t *testing.T) {
	service := &stubTaskEvaluationService{validate: true}
	handler := NewHTTPHandler(HTTPDependencies{
		Readiness:       stubReadinessChecker{},
		APIKeys:         map[string]struct{}{"test-key": {}},
		TaskEvaluations: service,
	})

	req := httptest.NewRequest(http.MethodPost, "/v1/task-evaluations", strings.NewReader(`{"objective":"validate","success_criteria":["report evidence"],"verdict":"succeeded","evidence":[{"kind":"opaque","opaque_token":"caller-opaque-evidence"}],"actor":"operator-a","reason":"record task outcome"}`))
	setAPIScopeHeaders(req)
	resp := httptest.NewRecorder()
	handler.ServeHTTP(resp, req)
	if resp.Code != http.StatusCreated {
		t.Fatalf("status = %d body=%s, want 201", resp.Code, resp.Body.String())
	}
	if service.gotCreateInput.Evidence[0].OpaqueToken != "caller-opaque-evidence" || service.gotCreateInput.Evidence[0].ID != "" {
		t.Fatalf("create input = %+v, want opaque evidence token preserved", service.gotCreateInput.Evidence[0])
	}
}

func TestNewHTTPHandlerRejectsOutOfScopeTaskEvaluationReport(t *testing.T) {
	service := &stubTaskEvaluationService{err: pgx.ErrNoRows, validate: true}
	handler := NewHTTPHandler(HTTPDependencies{
		Readiness:       stubReadinessChecker{},
		APIKeys:         map[string]struct{}{"test-key": {}},
		TaskEvaluations: service,
	})

	req := httptest.NewRequest(http.MethodGet, "/v1/task-evaluations/task_eval_other/report", nil)
	setAPIScopeHeaders(req)
	resp := httptest.NewRecorder()
	handler.ServeHTTP(resp, req)
	if resp.Code != http.StatusNotFound {
		t.Fatalf("status = %d body=%s, want 404", resp.Code, resp.Body.String())
	}
	if service.gotReadInput.EvaluationID != "task_eval_other" {
		t.Fatalf("read input = %+v, want task_eval_other", service.gotReadInput)
	}
}

func TestNewHTTPHandlerRejectsTaskEvaluationWithoutAPIKey(t *testing.T) {
	handler := NewHTTPHandler(HTTPDependencies{
		Readiness:       stubReadinessChecker{},
		APIKeys:         map[string]struct{}{"test-key": {}},
		TaskEvaluations: &stubTaskEvaluationService{},
	})

	req := httptest.NewRequest(http.MethodPost, "/v1/task-evaluations", strings.NewReader(`{"objective":"validate","success_criteria":["report evidence"],"verdict":"succeeded","evidence":[{"kind":"session","id":"session_1"}],"actor":"operator-a","reason":"record task outcome"}`))
	req.Header.Set("X-Stele-Tenant", "tenant-a")
	req.Header.Set("X-Stele-Project", "project-a")
	req.Header.Set("X-Stele-Namespace", "namespace-a")
	resp := httptest.NewRecorder()
	handler.ServeHTTP(resp, req)
	if resp.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", resp.Code)
	}
}

func TestNewHTTPHandlerRejectsTaskEvaluationSummaryWithoutAdminKey(t *testing.T) {
	handler := NewHTTPHandler(HTTPDependencies{
		Readiness:       stubReadinessChecker{},
		AdminAPIKeys:    map[string]struct{}{"admin-key": {}},
		TaskEvaluations: &stubTaskEvaluationService{},
	})

	req := httptest.NewRequest(http.MethodGet, "/v1/admin/task-evaluations/summary?evidence_target_kind=session", nil)
	setAPIScopeHeaders(req)
	resp := httptest.NewRecorder()
	handler.ServeHTTP(resp, req)
	if resp.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", resp.Code)
	}
}

func TestUsefulnessFeedbackLifecycleLogUsesBoundedFields(t *testing.T) {
	var out strings.Builder
	logger := log.New(&out, "", 0)

	recordUsefulnessFeedbackLog(logger, "create", "ok", memory.UsefulnessFeedbackTypeNoisy, memory.UsefulnessFeedbackSubjectMemory, memory.UsefulnessFeedbackSourceSearch, "active")

	line := out.String()
	for _, want := range []string{
		"component=usefulness_feedback",
		"event=lifecycle",
		"operation=create",
		"result=ok",
		"feedback_type=noisy",
		"subject_kind=memory",
		"source_surface=search",
		"decision=active",
	} {
		if !strings.Contains(line, want) {
			t.Fatalf("log = %q, missing %q", line, want)
		}
	}
	for _, forbidden := range []string{"feedback_id", "memory_id", "session_id", "actor", "reason"} {
		if strings.Contains(line, forbidden) {
			t.Fatalf("log = %q, contains high-cardinality field %q", line, forbidden)
		}
	}
}

func TestTaskEvaluationLifecycleLogUsesBoundedFields(t *testing.T) {
	var out strings.Builder
	logger := log.New(&out, "", 0)

	recordTaskEvaluationLog(logger, "create", "ok", memory.TaskEvaluationVerdictFailed, memory.TaskContributionCategoryMemoryMissing, memory.TaskEvaluationCorrectionStateActive)

	line := out.String()
	for _, want := range []string{
		"component=task_evaluation",
		"event=lifecycle",
		"operation=create",
		"result=ok",
		"verdict=failed",
		"contribution_category=memory_missing",
		"correction_state=active",
	} {
		if !strings.Contains(line, want) {
			t.Fatalf("log = %q, missing %q", line, want)
		}
	}
	for _, forbidden := range []string{"task_id", "evaluation_id", "memory_id", "session_id", "actor", "reason", "tenant", "project", "namespace"} {
		if strings.Contains(line, forbidden) {
			t.Fatalf("log = %q, contains high-cardinality field %q", line, forbidden)
		}
	}
}

func TestRankingRolloutLifecycleLogUsesBoundedFields(t *testing.T) {
	var out strings.Builder
	logger := log.New(&out, "", 0)

	recordRankingRolloutLog(logger, "dry_run", "ok", memory.RankingRolloutSurfaceSearch, memory.RankingRolloutSignalSourceTaskEvaluations, memory.RankingRolloutThresholdStatusSatisfied, memory.RankingRolloutPolicyStatusDryRun, string(memory.RankingRolloutImpactReasonCodeSubjectBoosted))

	line := out.String()
	for _, want := range []string{
		"component=ranking_rollout",
		"event=lifecycle",
		"operation=dry_run",
		"result=ok",
		"surface=search",
		"signal_source=task_evaluations",
		"threshold_status=satisfied",
		"policy_status=dry_run",
		"reason_code=subject_boosted",
	} {
		if !strings.Contains(line, want) {
			t.Fatalf("log = %q, missing %q", line, want)
		}
	}
	for _, forbidden := range []string{"policy_id", "task_id", "memory_id", "session_id", "query", "actor", "reason_text", "tenant", "project", "namespace"} {
		if strings.Contains(line, forbidden) {
			t.Fatalf("log = %q, contains high-cardinality field %q", line, forbidden)
		}
	}
}

func TestNewHTTPHandlerWorkflowPublicEndpoints(t *testing.T) {
	scope := memory.Scope{Tenant: "tenant-a", Project: "project-a", Namespace: "namespace-a"}
	now := time.Date(2026, 7, 18, 10, 0, 0, 0, time.UTC)
	service := &stubWorkflowHTTPService{
		run: workflow.WorkflowRun{
			ID:              "workflow_run_1",
			TemplateID:      "workflow_template_1",
			Scope:           scope,
			Status:          workflow.RunStatusRunning,
			IntegrationKind: workflow.IntegrationKindAgentTurn,
			IdempotencyKey:  "run-key-1",
			Actor:           "agent-a",
			Reason:          "serve user turn",
			Metadata:        map[string]any{"prompt": "caller prompt", "subject": "user-42"},
			CreatedAt:       now,
			UpdatedAt:       now,
			StartedAt:       now,
		},
		step: workflow.WorkflowStepRecord{
			ID:         "workflow_step_1",
			RunID:      "workflow_run_1",
			Scope:      scope,
			Kind:       workflow.StepKindSessionStarted,
			Status:     workflow.StepStatusSatisfied,
			Result:     workflow.StepResultRecorded,
			Actor:      "agent-a",
			Reason:     "session opened",
			Metadata:   map[string]any{"model_output": "private completion"},
			ObservedAt: now,
			CreatedAt:  now,
		},
		nextActions: []workflow.NextAction{{
			ID:            "workflow_next_action_1",
			RunID:         "workflow_run_1",
			Scope:         scope,
			Category:      workflow.NextActionRecordOutcome,
			StepKind:      workflow.StepKindTurnOutcomeRecorded,
			EvidenceKind:  workflow.EvidenceKindOutcome,
			RouteCategory: workflow.RouteCategoryMemorySessionOutcome,
			Status:        workflow.NextActionStatusOpen,
			Metadata: map[string]any{
				"prompt":        "hidden prompt text",
				"model_output":  "hidden model output",
				"hidden_memory": "mem_secret",
			},
			CreatedAt: now,
		}},
	}
	handler := NewHTTPHandler(HTTPDependencies{
		Readiness: stubReadinessChecker{},
		APIKeys:   map[string]struct{}{"test-key": {}},
		Workflow:  service,
	})

	createReq := httptest.NewRequest(http.MethodPost, "/v1/workflows/runs", strings.NewReader(`{"template_id":"workflow_template_1","idempotency_key":"run-key-1","actor":"agent-a","reason":"serve user turn","metadata":{"integration":"test-agent"},"expires_at":"2026-07-18T11:00:00Z"}`))
	setAPIScopeHeaders(createReq)
	createResp := httptest.NewRecorder()
	handler.ServeHTTP(createResp, createReq)
	if createResp.Code != http.StatusCreated {
		t.Fatalf("create status = %d body=%s, want 201", createResp.Code, createResp.Body.String())
	}
	if service.gotStartRun.TemplateID != "workflow_template_1" || service.gotStartRun.IdempotencyKey != "run-key-1" {
		t.Fatalf("start input = %+v, want template/idempotency", service.gotStartRun)
	}
	if service.gotStartRun.Scope != scope {
		t.Fatalf("start scope = %+v, want %+v", service.gotStartRun.Scope, scope)
	}
	if !strings.Contains(createResp.Body.String(), `"id":"workflow_run_1"`) {
		t.Fatalf("create body = %s, missing run id", createResp.Body.String())
	}
	for _, forbidden := range []string{"workflow_template_1", "tenant-a", "project-a", "namespace-a", "run-key-1", "agent-a", "serve user turn", "caller prompt", "user-42"} {
		if strings.Contains(createResp.Body.String(), forbidden) {
			t.Fatalf("create body = %s, contains sensitive field %q", createResp.Body.String(), forbidden)
		}
	}

	replayReq := httptest.NewRequest(http.MethodPost, "/v1/workflows/runs", strings.NewReader(`{"template_id":"workflow_template_1","idempotency_key":"run-key-1","actor":"agent-a","reason":"serve user turn"}`))
	setAPIScopeHeaders(replayReq)
	replayResp := httptest.NewRecorder()
	handler.ServeHTTP(replayResp, replayReq)
	if replayResp.Code != http.StatusCreated {
		t.Fatalf("idempotent create status = %d body=%s, want 201", replayResp.Code, replayResp.Body.String())
	}
	if strings.Count(replayResp.Body.String(), "workflow_run_1") == 0 {
		t.Fatalf("idempotent create body = %s, missing original run id", replayResp.Body.String())
	}

	readReq := httptest.NewRequest(http.MethodGet, "/v1/workflows/runs/workflow_run_1", nil)
	setAPIScopeHeaders(readReq)
	readResp := httptest.NewRecorder()
	handler.ServeHTTP(readResp, readReq)
	if readResp.Code != http.StatusOK {
		t.Fatalf("read status = %d body=%s, want 200", readResp.Code, readResp.Body.String())
	}
	if service.gotReadRun.RunID != "workflow_run_1" {
		t.Fatalf("read input = %+v, want workflow_run_1", service.gotReadRun)
	}
	for _, forbidden := range []string{"workflow_run_1", "workflow_template_1", "tenant-a", "project-a", "namespace-a", "run-key-1", "agent-a", "serve user turn", "caller prompt", "user-42"} {
		if strings.Contains(readResp.Body.String(), forbidden) {
			t.Fatalf("read body = %s, contains sensitive field %q", readResp.Body.String(), forbidden)
		}
	}

	stepReq := httptest.NewRequest(http.MethodPost, "/v1/workflows/runs/workflow_run_1/steps", strings.NewReader(`{"kind":"session_started","actor":"agent-a","reason":"session opened","observed_at":"2026-07-18T10:00:00Z","evidence_links":[{"kind":"session","source":"public_api","target_id":"session_1"}]}`))
	setAPIScopeHeaders(stepReq)
	stepResp := httptest.NewRecorder()
	handler.ServeHTTP(stepResp, stepReq)
	if stepResp.Code != http.StatusCreated {
		t.Fatalf("step status = %d body=%s, want 201", stepResp.Code, stepResp.Body.String())
	}
	if service.gotRecordStep.RunID != "workflow_run_1" || service.gotRecordStep.Kind != workflow.StepKindSessionStarted {
		t.Fatalf("record step input = %+v, want run/kind", service.gotRecordStep)
	}
	if len(service.gotRecordStep.EvidenceLinks) != 1 || service.gotRecordStep.EvidenceLinks[0].TargetID != "session_1" {
		t.Fatalf("evidence links = %+v, want session_1", service.gotRecordStep.EvidenceLinks)
	}
	for _, forbidden := range []string{"workflow_step_1", "workflow_run_1", "tenant-a", "project-a", "namespace-a", "agent-a", "session opened", "private completion"} {
		if strings.Contains(stepResp.Body.String(), forbidden) {
			t.Fatalf("step body = %s, contains sensitive field %q", stepResp.Body.String(), forbidden)
		}
	}

	actionsReq := httptest.NewRequest(http.MethodGet, "/v1/workflows/runs/workflow_run_1/next-actions", nil)
	setAPIScopeHeaders(actionsReq)
	actionsResp := httptest.NewRecorder()
	handler.ServeHTTP(actionsResp, actionsReq)
	if actionsResp.Code != http.StatusOK {
		t.Fatalf("next actions status = %d body=%s, want 200", actionsResp.Code, actionsResp.Body.String())
	}
	body := actionsResp.Body.String()
	for _, want := range []string{`"category":"record_outcome"`, `"step_kind":"turn_outcome_recorded"`, `"evidence_kind":"outcome"`, `"route_category":"memory_session_outcome"`} {
		if !strings.Contains(body, want) {
			t.Fatalf("next actions body = %s, missing %s", body, want)
		}
	}
	for _, forbidden := range []string{"workflow_run_1", "workflow_next_action_1", "tenant-a", "project-a", "namespace-a", "hidden prompt text", "hidden model output", "mem_secret"} {
		if strings.Contains(body, forbidden) {
			t.Fatalf("next actions body = %s, contains sensitive field %q", body, forbidden)
		}
	}
}

func TestNewHTTPHandlerWorkflowAdminEndpoints(t *testing.T) {
	scope := memory.Scope{Tenant: "tenant-a", Project: "project-a", Namespace: "namespace-a"}
	now := time.Date(2026, 7, 18, 10, 0, 0, 0, time.UTC)
	template := testWorkflowTemplate(scope, now)
	run := workflow.WorkflowRun{
		ID:              "workflow_run_1",
		TemplateID:      template.ID,
		Scope:           scope,
		Status:          workflow.RunStatusRunning,
		IntegrationKind: workflow.IntegrationKindAgentTurn,
		IdempotencyKey:  "run-key-1",
		Actor:           "agent-a",
		Reason:          "serve user turn",
		CreatedAt:       now,
		UpdatedAt:       now,
		StartedAt:       now,
	}
	service := &stubWorkflowHTTPService{
		template:    template,
		templates:   []workflow.WorkflowTemplate{template},
		run:         run,
		runs:        []workflow.WorkflowRun{run},
		steps:       []workflow.WorkflowStepRecord{{ID: "workflow_step_1", RunID: run.ID, Scope: scope, Kind: workflow.StepKindSessionStarted, Status: workflow.StepStatusSatisfied, Result: workflow.StepResultRecorded, Actor: "agent-a", Reason: "session opened", ObservedAt: now, CreatedAt: now}},
		evidence:    []workflow.EvidenceLink{{ID: "workflow_link_1", RunID: run.ID, StepRecordID: "workflow_step_1", Scope: scope, Kind: workflow.EvidenceKindSession, Status: workflow.EvidenceLinkStatusActive, Source: workflow.EvidenceSourcePublicAPI, TargetID: "session_1", CreatedAt: now}},
		diagnostics: []workflow.GapDiagnostic{{ID: "workflow_gap_1", RunID: run.ID, Scope: scope, StepKind: workflow.StepKindTurnOutcomeRecorded, EvidenceKind: workflow.EvidenceKindOutcome, Category: workflow.DiagnosticCategoryMissing, ReadinessImpact: workflow.ReadinessImpactDegraded, Status: "open", CreatedAt: now}},
		nextActions: []workflow.NextAction{{ID: "workflow_next_action_1", RunID: run.ID, Scope: scope, Category: workflow.NextActionRecordOutcome, StepKind: workflow.StepKindTurnOutcomeRecorded, EvidenceKind: workflow.EvidenceKindOutcome, RouteCategory: workflow.RouteCategoryMemorySessionOutcome, Status: workflow.NextActionStatusOpen, CreatedAt: now}},
	}
	handler := NewHTTPHandler(HTTPDependencies{
		Readiness:    stubReadinessChecker{},
		AdminAPIKeys: map[string]struct{}{"admin-key": {}},
		Workflow:     service,
	})

	createReq := httptest.NewRequest(http.MethodPost, "/v1/admin/workflows/templates", strings.NewReader(`{"integration_kind":"agent_turn","completion_policy":"strict","actor":"operator-a","reason":"define product loop","steps":[{"kind":"session_started","requirement":"required","allowed_evidence":["session"],"minimum_count":1,"requires_internal":true,"freshness_window":"24h","completion_window":"1h","position":1}]}`))
	setAdminScopeHeaders(createReq)
	createResp := httptest.NewRecorder()
	handler.ServeHTTP(createResp, createReq)
	if createResp.Code != http.StatusCreated {
		t.Fatalf("template create status = %d body=%s, want 201", createResp.Code, createResp.Body.String())
	}
	if service.gotCreateTemplate.IntegrationKind != workflow.IntegrationKindAgentTurn || service.gotCreateTemplate.Steps[0].FreshnessWindow != 24*time.Hour {
		t.Fatalf("create template input = %+v, want parsed bounded workflow template", service.gotCreateTemplate)
	}

	for _, tc := range []struct {
		method string
		path   string
		body   string
		want   int
	}{
		{http.MethodGet, "/v1/admin/workflows/templates?status=active&limit=10", "", http.StatusOK},
		{http.MethodGet, "/v1/admin/workflows/templates/workflow_template_1", "", http.StatusOK},
		{http.MethodPatch, "/v1/admin/workflows/templates/workflow_template_1", `{"integration_kind":"agent_turn","completion_policy":"permissive","actor":"operator-b","reason":"relax order","steps":[{"kind":"session_started","requirement":"required","allowed_evidence":["session"],"minimum_count":1,"requires_internal":true,"freshness_window":"24h","completion_window":"1h","position":1}]}`, http.StatusOK},
		{http.MethodPost, "/v1/admin/workflows/templates/workflow_template_1/disable", `{"actor":"operator-c","reason":"retire template"}`, http.StatusOK},
		{http.MethodGet, "/v1/admin/workflows/runs?status=running&limit=10", "", http.StatusOK},
		{http.MethodGet, "/v1/admin/workflows/runs/workflow_run_1", "", http.StatusOK},
		{http.MethodGet, "/v1/admin/workflows/runs/workflow_run_1/steps", "", http.StatusOK},
		{http.MethodGet, "/v1/admin/workflows/runs/workflow_run_1/evidence-links?status=active", "", http.StatusOK},
		{http.MethodGet, "/v1/admin/workflows/runs/workflow_run_1/diagnostics?category=missing", "", http.StatusOK},
		{http.MethodGet, "/v1/admin/workflows/runs/workflow_run_1/next-actions?status=open", "", http.StatusOK},
		{http.MethodPost, "/v1/admin/workflows/evidence-links/workflow_link_1/supersede", `{"actor":"operator-d","reason":"bad evidence link"}`, http.StatusAccepted},
	} {
		var reader *strings.Reader
		if tc.body == "" {
			reader = strings.NewReader("")
		} else {
			reader = strings.NewReader(tc.body)
		}
		req := httptest.NewRequest(tc.method, tc.path, reader)
		setAdminScopeHeaders(req)
		resp := httptest.NewRecorder()
		handler.ServeHTTP(resp, req)
		if resp.Code != tc.want {
			t.Fatalf("%s %s status = %d body=%s, want %d", tc.method, tc.path, resp.Code, resp.Body.String(), tc.want)
		}
	}
	if service.gotListTemplates.Status != workflow.TemplateStatusActive || service.gotListRuns.Status != workflow.RunStatusRunning {
		t.Fatalf("list filters templates=%+v runs=%+v, want active/running", service.gotListTemplates, service.gotListRuns)
	}
	if service.gotSupersede.LinkID != "workflow_link_1" || service.gotSupersede.Actor != "operator-d" {
		t.Fatalf("supersede input = %+v, want link and actor", service.gotSupersede)
	}
}

func TestNewHTTPHandlerWorkflowAuthScopeAndValidation(t *testing.T) {
	service := &stubWorkflowHTTPService{
		run: workflow.WorkflowRun{
			ID:              "workflow_run_1",
			TemplateID:      "workflow_template_1",
			Scope:           memory.Scope{Tenant: "tenant-a", Project: "project-a", Namespace: "namespace-a"},
			Status:          workflow.RunStatusRunning,
			IntegrationKind: workflow.IntegrationKindAgentTurn,
			IdempotencyKey:  "run-key-1",
			Actor:           "agent-a",
			Reason:          "serve user turn",
			CreatedAt:       time.Date(2026, 7, 18, 10, 0, 0, 0, time.UTC),
			UpdatedAt:       time.Date(2026, 7, 18, 10, 0, 0, 0, time.UTC),
			StartedAt:       time.Date(2026, 7, 18, 10, 0, 0, 0, time.UTC),
		},
		readErr: pgx.ErrNoRows,
	}
	handler := NewHTTPHandler(HTTPDependencies{
		Readiness:    stubReadinessChecker{},
		APIKeys:      map[string]struct{}{"test-key": {}},
		AdminAPIKeys: map[string]struct{}{"admin-key": {}},
		Workflow:     service,
	})

	noKeyReq := httptest.NewRequest(http.MethodPost, "/v1/workflows/runs", strings.NewReader(`{"template_id":"workflow_template_1","idempotency_key":"run-key-1","actor":"agent-a","reason":"serve turn"}`))
	noKeyReq.Header.Set("X-Stele-Tenant", "tenant-a")
	noKeyReq.Header.Set("X-Stele-Project", "project-a")
	noKeyReq.Header.Set("X-Stele-Namespace", "namespace-a")
	noKeyResp := httptest.NewRecorder()
	handler.ServeHTTP(noKeyResp, noKeyReq)
	if noKeyResp.Code != http.StatusUnauthorized {
		t.Fatalf("missing public key status = %d, want 401", noKeyResp.Code)
	}

	publicKeyAdminReq := httptest.NewRequest(http.MethodGet, "/v1/admin/workflows/templates", nil)
	setAPIScopeHeaders(publicKeyAdminReq)
	publicKeyAdminResp := httptest.NewRecorder()
	handler.ServeHTTP(publicKeyAdminResp, publicKeyAdminReq)
	if publicKeyAdminResp.Code != http.StatusUnauthorized {
		t.Fatalf("public key admin status = %d, want 401", publicKeyAdminResp.Code)
	}

	missingScopeReq := httptest.NewRequest(http.MethodGet, "/v1/workflows/runs/workflow_run_1", nil)
	missingScopeReq.Header.Set("X-API-Key", "test-key")
	missingScopeResp := httptest.NewRecorder()
	handler.ServeHTTP(missingScopeResp, missingScopeReq)
	if missingScopeResp.Code != http.StatusBadRequest {
		t.Fatalf("missing scope status = %d body=%s, want 400", missingScopeResp.Code, missingScopeResp.Body.String())
	}

	invalidStepReq := httptest.NewRequest(http.MethodPost, "/v1/workflows/runs/workflow_run_1/steps", strings.NewReader(`{"kind":"free_form","actor":"agent-a","reason":"bad step","evidence_links":[{"kind":"free_form","source":"public_api","target_id":"session_1"}]}`))
	setAPIScopeHeaders(invalidStepReq)
	invalidStepResp := httptest.NewRecorder()
	handler.ServeHTTP(invalidStepResp, invalidStepReq)
	if invalidStepResp.Code != http.StatusBadRequest {
		t.Fatalf("invalid step status = %d body=%s, want 400", invalidStepResp.Code, invalidStepResp.Body.String())
	}

	outOfScopeReq := httptest.NewRequest(http.MethodGet, "/v1/workflows/runs/secret_run_1", nil)
	setAPIScopeHeaders(outOfScopeReq)
	outOfScopeResp := httptest.NewRecorder()
	handler.ServeHTTP(outOfScopeResp, outOfScopeReq)
	if outOfScopeResp.Code != http.StatusNotFound {
		t.Fatalf("out-of-scope status = %d body=%s, want 404", outOfScopeResp.Code, outOfScopeResp.Body.String())
	}
	for _, forbidden := range []string{"secret_run_1", "tenant-b", "project-b", "namespace-b"} {
		if strings.Contains(outOfScopeResp.Body.String(), forbidden) {
			t.Fatalf("out-of-scope body = %s, contains %q", outOfScopeResp.Body.String(), forbidden)
		}
	}
}

type stubWorkflowHTTPService struct {
	gotCreateTemplate  workflow.CreateTemplateInput
	gotUpdateTemplate  workflow.UpdateTemplateInput
	gotDisable         workflow.DisableTemplateInput
	gotReadTemplate    workflow.ReadTemplateInput
	gotListTemplates   workflow.ListTemplatesInput
	gotStartRun        workflow.StartRunInput
	gotReadRun         workflow.ReadRunInput
	gotListRuns        workflow.ListRunsInput
	gotRecordStep      workflow.RecordStepInput
	gotListSteps       workflow.ListStepRecordsInput
	gotListEvidence    workflow.ListEvidenceLinksInput
	gotListDiagnostics workflow.ListDiagnosticsInput
	gotListNextActions workflow.ListNextActionsInput
	gotSupersede       workflow.SupersedeEvidenceLinkInput

	template    workflow.WorkflowTemplate
	templates   []workflow.WorkflowTemplate
	run         workflow.WorkflowRun
	runs        []workflow.WorkflowRun
	step        workflow.WorkflowStepRecord
	steps       []workflow.WorkflowStepRecord
	evidence    []workflow.EvidenceLink
	diagnostics []workflow.GapDiagnostic
	nextActions []workflow.NextAction

	readErr error
	err     error
}

func (s *stubWorkflowHTTPService) CreateTemplate(ctx context.Context, input workflow.CreateTemplateInput) (workflow.WorkflowTemplate, error) {
	s.gotCreateTemplate = input
	if s.err != nil {
		return workflow.WorkflowTemplate{}, s.err
	}
	if err := validateHTTPWorkflowTemplateInput(input.Steps); err != nil {
		return workflow.WorkflowTemplate{}, err
	}
	if s.template.ID == "" {
		s.template = testWorkflowTemplate(input.Scope, time.Date(2026, 7, 18, 10, 0, 0, 0, time.UTC))
	}
	return s.template, nil
}

func (s *stubWorkflowHTTPService) UpdateTemplate(ctx context.Context, input workflow.UpdateTemplateInput) (workflow.WorkflowTemplate, error) {
	s.gotUpdateTemplate = input
	if s.err != nil {
		return workflow.WorkflowTemplate{}, s.err
	}
	if err := validateHTTPWorkflowTemplateInput(input.Steps); err != nil {
		return workflow.WorkflowTemplate{}, err
	}
	return s.template, nil
}

func (s *stubWorkflowHTTPService) DisableTemplate(ctx context.Context, input workflow.DisableTemplateInput) (workflow.WorkflowTemplate, error) {
	s.gotDisable = input
	if s.err != nil {
		return workflow.WorkflowTemplate{}, s.err
	}
	template := s.template
	template.Status = workflow.TemplateStatusDisabled
	template.DisabledAt = input.DisabledAt
	return template, nil
}

func (s *stubWorkflowHTTPService) ReadTemplate(ctx context.Context, input workflow.ReadTemplateInput) (workflow.WorkflowTemplate, error) {
	s.gotReadTemplate = input
	if s.readErr != nil {
		return workflow.WorkflowTemplate{}, s.readErr
	}
	return s.template, nil
}

func (s *stubWorkflowHTTPService) ListTemplates(ctx context.Context, input workflow.ListTemplatesInput) ([]workflow.WorkflowTemplate, error) {
	s.gotListTemplates = input
	if s.err != nil {
		return nil, s.err
	}
	return s.templates, nil
}

func (s *stubWorkflowHTTPService) StartRun(ctx context.Context, input workflow.StartRunInput) (workflow.WorkflowRun, error) {
	s.gotStartRun = input
	if s.err != nil {
		return workflow.WorkflowRun{}, s.err
	}
	return s.run, nil
}

func (s *stubWorkflowHTTPService) ReadRun(ctx context.Context, input workflow.ReadRunInput) (workflow.WorkflowRun, error) {
	s.gotReadRun = input
	if s.readErr != nil {
		return workflow.WorkflowRun{}, s.readErr
	}
	return s.run, nil
}

func (s *stubWorkflowHTTPService) ListRuns(ctx context.Context, input workflow.ListRunsInput) ([]workflow.WorkflowRun, error) {
	s.gotListRuns = input
	if s.err != nil {
		return nil, s.err
	}
	return s.runs, nil
}

func (s *stubWorkflowHTTPService) RecordStep(ctx context.Context, input workflow.RecordStepInput) (workflow.WorkflowStepRecord, error) {
	s.gotRecordStep = input
	if s.err != nil {
		return workflow.WorkflowStepRecord{}, s.err
	}
	if !input.Kind.Valid() {
		return workflow.WorkflowStepRecord{}, errors.New("workflow step kind is invalid")
	}
	for _, link := range input.EvidenceLinks {
		if !link.Kind.Valid() || !link.Source.Valid() {
			return workflow.WorkflowStepRecord{}, errors.New("workflow evidence link is invalid")
		}
	}
	if s.step.ID != "" {
		return s.step, nil
	}
	return workflow.WorkflowStepRecord{ID: "workflow_step_1", RunID: input.RunID, Scope: input.Scope, Kind: input.Kind, Status: workflow.StepStatusSatisfied, Result: workflow.StepResultRecorded, Actor: input.Actor, Reason: input.Reason, ObservedAt: input.ObservedAt, CreatedAt: input.ObservedAt}, nil
}

func (s *stubWorkflowHTTPService) ListStepRecords(ctx context.Context, input workflow.ListStepRecordsInput) ([]workflow.WorkflowStepRecord, error) {
	s.gotListSteps = input
	if s.err != nil {
		return nil, s.err
	}
	return s.steps, nil
}

func (s *stubWorkflowHTTPService) ListEvidenceLinks(ctx context.Context, input workflow.ListEvidenceLinksInput) ([]workflow.EvidenceLink, error) {
	s.gotListEvidence = input
	if s.err != nil {
		return nil, s.err
	}
	return s.evidence, nil
}

func (s *stubWorkflowHTTPService) ListDiagnostics(ctx context.Context, input workflow.ListDiagnosticsInput) ([]workflow.GapDiagnostic, error) {
	s.gotListDiagnostics = input
	if s.err != nil {
		return nil, s.err
	}
	return s.diagnostics, nil
}

func (s *stubWorkflowHTTPService) ListNextActions(ctx context.Context, input workflow.ListNextActionsInput) ([]workflow.NextAction, error) {
	s.gotListNextActions = input
	if s.err != nil {
		return nil, s.err
	}
	return s.nextActions, nil
}

func (s *stubWorkflowHTTPService) SupersedeEvidenceLink(ctx context.Context, input workflow.SupersedeEvidenceLinkInput) error {
	s.gotSupersede = input
	return s.err
}

func validateHTTPWorkflowTemplateInput(steps []workflow.TemplateStep) error {
	if len(steps) == 0 {
		return errors.New("workflow template steps are required")
	}
	for _, step := range steps {
		if !step.Kind.Valid() || !step.Requirement.Valid() {
			return errors.New("workflow template step is invalid")
		}
		for _, kind := range step.AllowedEvidence {
			if !kind.Valid() {
				return errors.New("workflow evidence kind is invalid")
			}
		}
	}
	return nil
}

func testWorkflowTemplate(scope memory.Scope, now time.Time) workflow.WorkflowTemplate {
	return workflow.WorkflowTemplate{
		ID:               "workflow_template_1",
		Scope:            scope,
		Status:           workflow.TemplateStatusActive,
		IntegrationKind:  workflow.IntegrationKindAgentTurn,
		CompletionPolicy: workflow.CompletionPolicyStrict,
		Actor:            "operator-a",
		Reason:           "define product loop",
		CreatedAt:        now,
		UpdatedAt:        now,
		Steps: []workflow.TemplateStep{{
			ID:               "workflow_template_step_1",
			TemplateID:       "workflow_template_1",
			Scope:            scope,
			Kind:             workflow.StepKindSessionStarted,
			Requirement:      workflow.StepRequirementRequired,
			AllowedEvidence:  []workflow.EvidenceKind{workflow.EvidenceKindSession},
			MinimumCount:     1,
			RequiresInternal: true,
			FreshnessWindow:  24 * time.Hour,
			CompletionWindow: time.Hour,
			Position:         1,
			CreatedAt:        now,
		}},
	}
}

func setAPIScopeHeaders(req *http.Request) {
	req.Header.Set("X-API-Key", "test-key")
	req.Header.Set("X-Stele-Tenant", "tenant-a")
	req.Header.Set("X-Stele-Project", "project-a")
	req.Header.Set("X-Stele-Namespace", "namespace-a")
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

func testHTTPDerivedInsightReplayRun(scope memory.Scope) memory.DerivedInsightReplayRun {
	requestedAt := time.Date(2026, 7, 11, 10, 0, 0, 0, time.UTC)
	request := memory.DerivedInsightReplayRequest{
		Scope:               scope,
		Mode:                memory.DerivedInsightReplayModeApply,
		InsightTypes:        []memory.DerivedInsightType{memory.DerivedInsightTypeFailurePattern, memory.DerivedInsightTypeLesson},
		EvidenceWindowStart: time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC),
		EvidenceWindowEnd:   time.Date(2026, 7, 2, 0, 0, 0, 0, time.UTC),
		EvidenceLimit:       100,
		Actor:               "operator-a",
		Reason:              "apply replay",
		IdempotencyKey:      "apply-1",
		RequestedAt:         requestedAt,
	}
	return memory.DerivedInsightReplayRun{
		ID:        "replay_123",
		Scope:     scope,
		Mode:      memory.DerivedInsightReplayModeApply,
		Status:    memory.DerivedInsightReplayStatusPending,
		Request:   request,
		Actor:     request.Actor,
		Reason:    request.Reason,
		CreatedAt: requestedAt,
		UpdatedAt: requestedAt,
	}
}
