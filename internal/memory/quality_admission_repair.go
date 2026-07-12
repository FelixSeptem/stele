package memory

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"
)

type AdmissionPressureDecision string

const (
	AdmissionPressureDecisionAccept         AdmissionPressureDecision = "accept"
	AdmissionPressureDecisionAcceptDegraded AdmissionPressureDecision = "accept_degraded"
	AdmissionPressureDecisionQueue          AdmissionPressureDecision = "queue"
	AdmissionPressureDecisionReject         AdmissionPressureDecision = "reject"
)

func (d AdmissionPressureDecision) Valid() bool {
	switch d {
	case AdmissionPressureDecisionAccept,
		AdmissionPressureDecisionAcceptDegraded,
		AdmissionPressureDecisionQueue,
		AdmissionPressureDecisionReject:
		return true
	default:
		return false
	}
}

type AdmissionPressureOperation string

const (
	AdmissionPressureOperationIngest AdmissionPressureOperation = "ingest"
	AdmissionPressureOperationRepair AdmissionPressureOperation = "repair"
)

func (o AdmissionPressureOperation) Valid() bool {
	switch o {
	case AdmissionPressureOperationIngest, AdmissionPressureOperationRepair:
		return true
	default:
		return false
	}
}

type QualityFindingSeverity string

const (
	QualityFindingSeverityBlocker QualityFindingSeverity = "blocker"
	QualityFindingSeverityWarning QualityFindingSeverity = "warning"
)

func (s QualityFindingSeverity) Valid() bool {
	switch s {
	case QualityFindingSeverityBlocker, QualityFindingSeverityWarning:
		return true
	default:
		return false
	}
}

type QualityFindingComponent string

const (
	QualityFindingComponentIngestion  QualityFindingComponent = "ingestion"
	QualityFindingComponentGovernance QualityFindingComponent = "governance"
	QualityFindingComponentEmbedding  QualityFindingComponent = "embedding"
	QualityFindingComponentRetrieval  QualityFindingComponent = "retrieval"
	QualityFindingComponentRepair     QualityFindingComponent = "repair"
	QualityFindingComponentLifecycle  QualityFindingComponent = "lifecycle"
)

type QualityFindingCategory string

const (
	QualityFindingCategoryAdmission          QualityFindingCategory = "admission"
	QualityFindingCategoryBacklog            QualityFindingCategory = "backlog"
	QualityFindingCategorySemanticProjection QualityFindingCategory = "semantic_projection"
	QualityFindingCategoryLifecycle          QualityFindingCategory = "lifecycle"
	QualityFindingCategoryRepair             QualityFindingCategory = "repair"
)

type QualityFindingCode string

const (
	QualityFindingIntentNotWritable               QualityFindingCode = "intent_not_writable"
	QualityFindingGovernanceBacklogHigh           QualityFindingCode = "governance_backlog_high"
	QualityFindingWorkerLeasePressureHigh         QualityFindingCode = "worker_lease_pressure_high"
	QualityFindingSemanticProjectionDegraded      QualityFindingCode = "semantic_projection_degraded"
	QualityFindingLifecycleHiddenReturned         QualityFindingCode = "lifecycle_hidden_returned"
	QualityFindingExpectedRecallMissing           QualityFindingCode = "expected_recall_missing"
	QualityFindingUnsupportedAutomaticRepair      QualityFindingCode = "unsupported_automatic_repair"
	QualityFindingCanonicalRewriteRequired        QualityFindingCode = "canonical_rewrite_required"
	QualityFindingFeedbackNoisyRepeated           QualityFindingCode = "feedback_noisy_repeated"
	QualityFindingFeedbackStaleRepeated           QualityFindingCode = "feedback_stale_repeated"
	QualityFindingFeedbackIrrelevantRepeated      QualityFindingCode = "feedback_irrelevant_repeated"
	QualityFindingFeedbackMissingExpectedRepeated QualityFindingCode = "feedback_missing_expected_repeated"
	QualityFindingFeedbackUnsafeOrHidden          QualityFindingCode = "feedback_unsafe_or_hidden"
	QualityFindingFeedbackNeedsReview             QualityFindingCode = "feedback_needs_review"
)

type QualityFinding struct {
	Code                    QualityFindingCode      `json:"code"`
	Severity                QualityFindingSeverity  `json:"severity"`
	Component               QualityFindingComponent `json:"component"`
	Category                QualityFindingCategory  `json:"category"`
	Message                 string                  `json:"message,omitempty"`
	SuggestedActionCategory RepairActionCategory    `json:"suggested_action_category,omitempty"`
	Metadata                map[string]string       `json:"metadata,omitempty"`
	Evidence                map[string]any          `json:"evidence,omitempty"`
}

type AdmissionPressureSnapshot struct {
	IntentWritable             bool `json:"intent_writable"`
	PendingGovernance          int  `json:"pending_governance"`
	LeasedGovernance           int  `json:"leased_governance"`
	PendingEmbedding           int  `json:"pending_embedding"`
	FailedEmbedding            int  `json:"failed_embedding"`
	PendingRepair              int  `json:"pending_repair"`
	SemanticProjectionDegraded bool `json:"semantic_projection_degraded"`
}

type AdmissionPressureInput struct {
	Scope      Scope                      `json:"scope"`
	Operation  AdmissionPressureOperation `json:"operation"`
	Snapshot   AdmissionPressureSnapshot  `json:"snapshot"`
	ObservedAt time.Time                  `json:"observed_at"`
}

type AdmissionPressureReport struct {
	Scope      Scope                      `json:"scope"`
	Operation  AdmissionPressureOperation `json:"operation"`
	Decision   AdmissionPressureDecision  `json:"decision"`
	Findings   []QualityFinding           `json:"findings,omitempty"`
	Snapshot   AdmissionPressureSnapshot  `json:"snapshot"`
	ObservedAt time.Time                  `json:"observed_at"`
}

type AdmissionPressureEvaluator struct {
	MaxPending int
	MaxLeased  int
}

func (e AdmissionPressureEvaluator) Evaluate(input AdmissionPressureInput) AdmissionPressureReport {
	observedAt := input.ObservedAt
	if observedAt.IsZero() {
		observedAt = time.Now().UTC()
	}
	snapshot := input.Snapshot
	if !snapshot.IntentWritable {
		return AdmissionPressureReport{
			Scope:      input.Scope,
			Operation:  input.Operation,
			Decision:   AdmissionPressureDecisionReject,
			Snapshot:   snapshot,
			ObservedAt: observedAt.UTC(),
			Findings: []QualityFinding{{
				Code:      QualityFindingIntentNotWritable,
				Severity:  QualityFindingSeverityBlocker,
				Component: QualityFindingComponentIngestion,
				Category:  QualityFindingCategoryAdmission,
				Message:   "durable write intent is not currently safe",
			}},
		}
	}

	findings := make([]QualityFinding, 0)
	decision := AdmissionPressureDecisionAccept
	maxPending := e.MaxPending
	if maxPending <= 0 {
		maxPending = 1000
	}
	maxLeased := e.MaxLeased
	if maxLeased <= 0 {
		maxLeased = 1000
	}
	if snapshot.PendingGovernance > maxPending || snapshot.PendingRepair > maxPending {
		decision = AdmissionPressureDecisionQueue
		findings = append(findings, QualityFinding{
			Code:      QualityFindingGovernanceBacklogHigh,
			Severity:  QualityFindingSeverityWarning,
			Component: QualityFindingComponentGovernance,
			Category:  QualityFindingCategoryBacklog,
			Message:   "scoped governance or repair backlog exceeds configured pressure limits",
		})
	}
	if snapshot.LeasedGovernance > maxLeased {
		decision = AdmissionPressureDecisionQueue
		findings = append(findings, QualityFinding{
			Code:      QualityFindingWorkerLeasePressureHigh,
			Severity:  QualityFindingSeverityWarning,
			Component: QualityFindingComponentGovernance,
			Category:  QualityFindingCategoryBacklog,
			Message:   "scoped worker lease pressure exceeds configured limits",
		})
	}
	if snapshot.SemanticProjectionDegraded {
		if decision == AdmissionPressureDecisionAccept {
			decision = AdmissionPressureDecisionAcceptDegraded
		}
		findings = append(findings, QualityFinding{
			Code:                    QualityFindingSemanticProjectionDegraded,
			Severity:                QualityFindingSeverityWarning,
			Component:               QualityFindingComponentEmbedding,
			Category:                QualityFindingCategorySemanticProjection,
			Message:                 "semantic projection is currently degraded",
			SuggestedActionCategory: RepairActionCategoryEmbeddingRetry,
		})
	}

	return AdmissionPressureReport{
		Scope:      input.Scope,
		Operation:  input.Operation,
		Decision:   decision,
		Findings:   findings,
		Snapshot:   snapshot,
		ObservedAt: observedAt.UTC(),
	}
}

type QualityEvaluationCheck string

const (
	QualityEvaluationCheckRetrieval         QualityEvaluationCheck = "retrieval"
	QualityEvaluationCheckContext           QualityEvaluationCheck = "context"
	QualityEvaluationCheckAdmissionPressure QualityEvaluationCheck = "admission_pressure"
	QualityEvaluationCheckRepairPressure    QualityEvaluationCheck = "repair_pressure"
)

func (c QualityEvaluationCheck) Valid() bool {
	switch c {
	case QualityEvaluationCheckRetrieval,
		QualityEvaluationCheckContext,
		QualityEvaluationCheckAdmissionPressure,
		QualityEvaluationCheckRepairPressure:
		return true
	default:
		return false
	}
}

type QualityEvaluationStatus string

const (
	QualityEvaluationStatusPending      QualityEvaluationStatus = "pending"
	QualityEvaluationStatusRunning      QualityEvaluationStatus = "running"
	QualityEvaluationStatusCompleted    QualityEvaluationStatus = "completed"
	QualityEvaluationStatusFailed       QualityEvaluationStatus = "failed"
	QualityEvaluationStatusManualReview QualityEvaluationStatus = "manual_review"
)

func (s QualityEvaluationStatus) Valid() bool {
	switch s {
	case QualityEvaluationStatusPending,
		QualityEvaluationStatusRunning,
		QualityEvaluationStatusCompleted,
		QualityEvaluationStatusFailed,
		QualityEvaluationStatusManualReview:
		return true
	default:
		return false
	}
}

type QualityEvaluationRun struct {
	ID         string                   `json:"id"`
	Scope      Scope                    `json:"scope"`
	Status     QualityEvaluationStatus  `json:"status"`
	Checks     []QualityEvaluationCheck `json:"checks,omitempty"`
	Actor      string                   `json:"actor,omitempty"`
	Reason     string                   `json:"reason,omitempty"`
	CreatedAt  time.Time                `json:"created_at"`
	UpdatedAt  time.Time                `json:"updated_at"`
	StartedAt  time.Time                `json:"started_at,omitempty"`
	FinishedAt time.Time                `json:"finished_at,omitempty"`
}

type QualityEvaluationFinding struct {
	ID                      string                  `json:"id"`
	EvaluationRunID         string                  `json:"evaluation_run_id"`
	Scope                   Scope                   `json:"scope"`
	Code                    QualityFindingCode      `json:"code"`
	Severity                QualityFindingSeverity  `json:"severity"`
	Component               QualityFindingComponent `json:"component"`
	Category                QualityFindingCategory  `json:"category"`
	Message                 string                  `json:"message,omitempty"`
	SuggestedActionCategory RepairActionCategory    `json:"suggested_action_category,omitempty"`
	Metadata                map[string]string       `json:"metadata,omitempty"`
	Evidence                map[string]any          `json:"evidence,omitempty"`
	CreatedAt               time.Time               `json:"created_at"`
}

type RepairActionCategory string

const (
	RepairActionCategoryEmbeddingRetry       RepairActionCategory = "embedding_retry"
	RepairActionCategoryGovernanceRequeue    RepairActionCategory = "governance_requeue"
	RepairActionCategoryInsightReplay        RepairActionCategory = "derived_insight_replay"
	RepairActionCategoryManualReview         RepairActionCategory = "manual_review"
	RepairActionCategoryCanonicalRewrite     RepairActionCategory = "canonical_rewrite"
	RepairActionCategorySuppressionReview    RepairActionCategory = "suppression_review"
	RepairActionCategoryGovernanceInspection RepairActionCategory = "governance_inspection"
)

func (c RepairActionCategory) Valid() bool {
	switch c {
	case RepairActionCategoryEmbeddingRetry,
		RepairActionCategoryGovernanceRequeue,
		RepairActionCategoryInsightReplay,
		RepairActionCategoryManualReview,
		RepairActionCategoryCanonicalRewrite,
		RepairActionCategorySuppressionReview,
		RepairActionCategoryGovernanceInspection:
		return true
	default:
		return false
	}
}

type RepairPlanStatus string

const (
	RepairPlanStatusDraft        RepairPlanStatus = "draft"
	RepairPlanStatusApproved     RepairPlanStatus = "approved"
	RepairPlanStatusRunning      RepairPlanStatus = "running"
	RepairPlanStatusCompleted    RepairPlanStatus = "completed"
	RepairPlanStatusFailed       RepairPlanStatus = "failed"
	RepairPlanStatusManualReview RepairPlanStatus = "manual_review"
)

func (s RepairPlanStatus) Valid() bool {
	switch s {
	case RepairPlanStatusDraft,
		RepairPlanStatusApproved,
		RepairPlanStatusRunning,
		RepairPlanStatusCompleted,
		RepairPlanStatusFailed,
		RepairPlanStatusManualReview:
		return true
	default:
		return false
	}
}

type RepairActionStatus string

const (
	RepairActionStatusPending      RepairActionStatus = "pending"
	RepairActionStatusRunning      RepairActionStatus = "running"
	RepairActionStatusCompleted    RepairActionStatus = "completed"
	RepairActionStatusFailed       RepairActionStatus = "failed"
	RepairActionStatusSkipped      RepairActionStatus = "skipped"
	RepairActionStatusManualReview RepairActionStatus = "manual_review"
	RepairActionStatusExhausted    RepairActionStatus = "exhausted"
)

func (s RepairActionStatus) Valid() bool {
	switch s {
	case RepairActionStatusPending,
		RepairActionStatusRunning,
		RepairActionStatusCompleted,
		RepairActionStatusFailed,
		RepairActionStatusSkipped,
		RepairActionStatusManualReview,
		RepairActionStatusExhausted:
		return true
	default:
		return false
	}
}

type RepairVerificationStatus string

const (
	RepairVerificationStatusPending      RepairVerificationStatus = "pending"
	RepairVerificationStatusPassed       RepairVerificationStatus = "passed"
	RepairVerificationStatusFailed       RepairVerificationStatus = "failed"
	RepairVerificationStatusManualReview RepairVerificationStatus = "manual_review"
)

type RepairPlan struct {
	ID                 string                   `json:"id"`
	Scope              Scope                    `json:"scope"`
	EvaluationRunID    string                   `json:"evaluation_run_id"`
	BaselineRunID      string                   `json:"baseline_run_id,omitempty"`
	VerificationRunID  string                   `json:"verification_run_id,omitempty"`
	Status             RepairPlanStatus         `json:"status"`
	VerificationStatus RepairVerificationStatus `json:"verification_status,omitempty"`
	DryRun             bool                     `json:"dry_run"`
	Actor              string                   `json:"actor"`
	Reason             string                   `json:"reason"`
	CreatedAt          time.Time                `json:"created_at"`
	UpdatedAt          time.Time                `json:"updated_at"`
	ApprovedAt         time.Time                `json:"approved_at,omitempty"`
	CompletedAt        time.Time                `json:"completed_at,omitempty"`
	Actions            []RepairAction           `json:"actions,omitempty"`
}

type RepairAction struct {
	ID              string               `json:"id"`
	PlanID          string               `json:"plan_id"`
	EvaluationRunID string               `json:"evaluation_run_id"`
	FindingID       string               `json:"finding_id,omitempty"`
	Scope           Scope                `json:"scope"`
	Category        RepairActionCategory `json:"category"`
	Status          RepairActionStatus   `json:"status"`
	TargetKind      string               `json:"target_kind,omitempty"`
	TargetID        string               `json:"target_id,omitempty"`
	ReasonCode      QualityFindingCode   `json:"reason_code,omitempty"`
	Attempt         int                  `json:"attempt"`
	WorkerID        string               `json:"worker_id,omitempty"`
	LeaseUntil      time.Time            `json:"lease_until,omitempty"`
	LastError       string               `json:"last_error,omitempty"`
	NextAttemptAt   time.Time            `json:"next_attempt_at,omitempty"`
	CreatedAt       time.Time            `json:"created_at"`
	UpdatedAt       time.Time            `json:"updated_at"`
	CompletedAt     time.Time            `json:"completed_at,omitempty"`
}

type ClaimRepairActionsInput struct {
	Scope         Scope
	WorkerID      string
	Now           time.Time
	LeaseDuration time.Duration
	Limit         int
}

func (i ClaimRepairActionsInput) Validate() error {
	if err := i.Scope.Validate(); err != nil {
		return err
	}
	switch {
	case strings.TrimSpace(i.WorkerID) == "":
		return fmt.Errorf("worker id is required")
	case i.Now.IsZero():
		return fmt.Errorf("now is required")
	case i.LeaseDuration <= 0:
		return fmt.Errorf("lease duration must be greater than zero")
	case i.Limit <= 0:
		return fmt.Errorf("limit must be greater than zero")
	default:
		return nil
	}
}

type RecordRepairActionFailureInput struct {
	Scope         Scope
	ActionID      string
	WorkerID      string
	ErrorMessage  string
	FailedAt      time.Time
	NextAttemptAt time.Time
	Exhausted     bool
}

type CompleteRepairActionInput struct {
	Scope       Scope
	ActionID    string
	WorkerID    string
	CompletedAt time.Time
	Status      RepairActionStatus
}

var ErrRepairActionRejected = errors.New("repair action rejected")
var ErrAdmissionRejected = errors.New("admission rejected")

type AdmissionPressureSnapshotReader interface {
	ReadAdmissionPressureSnapshot(ctx context.Context, scope Scope, operation AdmissionPressureOperation, observedAt time.Time) (AdmissionPressureSnapshot, error)
}

type QualityStore interface {
	CreateQualityEvaluationRun(ctx context.Context, run QualityEvaluationRun) (QualityEvaluationRun, error)
	CreateQualityEvaluationFinding(ctx context.Context, finding QualityEvaluationFinding) (QualityEvaluationFinding, error)
	ReadQualityEvaluationRun(ctx context.Context, input ReadQualityEvaluationRunInput) (QualityEvaluationRun, error)
	ListQualityEvaluationFindings(ctx context.Context, input ListQualityEvaluationFindingsInput) ([]QualityEvaluationFinding, error)
	CreateRepairPlan(ctx context.Context, plan RepairPlan) (RepairPlan, error)
	CreateRepairAction(ctx context.Context, action RepairAction) (RepairAction, error)
	ReadRepairPlan(ctx context.Context, input ReadRepairPlanInput) (RepairPlan, error)
	ApproveRepairPlan(ctx context.Context, input ApproveRepairPlanInput) (RepairPlan, error)
	UpdateRepairPlanVerification(ctx context.Context, input UpdateRepairPlanVerificationInput) (RepairPlan, error)
}

type QualityDiagnosticsReader interface {
	ReadQualityDiagnostics(ctx context.Context, input ReadQualityDiagnosticsInput) (QualityDiagnostics, error)
}

type UsefulnessFeedbackLister interface {
	ListUsefulnessFeedback(ctx context.Context, input ListUsefulnessFeedbackInput) ([]UsefulnessFeedback, error)
}

type TaskEvaluationSummarizer interface {
	SummarizeTaskEvaluations(ctx context.Context, input SummarizeTaskEvaluationsInput) (TaskEvaluationSummary, error)
}

type QualityServiceOptions struct {
	Store              QualityStore
	Probe              QualityProbe
	UsefulnessFeedback UsefulnessFeedbackLister
	TaskSummarizer     TaskEvaluationSummarizer
	Now                func() time.Time
	NewID              func(prefix string) string
	MaxPlanItems       int
}

type QualityService struct {
	store              QualityStore
	probe              QualityProbe
	usefulnessFeedback UsefulnessFeedbackLister
	taskSummarizer     TaskEvaluationSummarizer
	now                func() time.Time
	newID              func(prefix string) string
	maxPlanItems       int
}

func NewQualityService(options QualityServiceOptions) *QualityService {
	now := options.Now
	if now == nil {
		now = time.Now
	}
	newID := options.NewID
	if newID == nil {
		newID = func(prefix string) string {
			return fmt.Sprintf("%s_%d", strings.TrimSpace(prefix), now().UnixNano())
		}
	}
	maxPlanItems := options.MaxPlanItems
	if maxPlanItems <= 0 {
		maxPlanItems = 100
	}
	return &QualityService{store: options.Store, probe: options.Probe, usefulnessFeedback: options.UsefulnessFeedback, taskSummarizer: options.TaskSummarizer, now: now, newID: newID, maxPlanItems: maxPlanItems}
}

type CreateQualityEvaluationInput struct {
	Scope             Scope                    `json:"scope"`
	Checks            []QualityEvaluationCheck `json:"checks"`
	Query             string                   `json:"query,omitempty"`
	ExpectedMemoryIDs []string                 `json:"expected_memory_ids,omitempty"`
	ContextBudget     int                      `json:"context_budget,omitempty"`
	Actor             string                   `json:"actor"`
	Reason            string                   `json:"reason,omitempty"`
}

type QualityProbeInput struct {
	Scope             Scope
	EvaluationRunID   string
	Checks            []QualityEvaluationCheck
	Query             string
	ExpectedMemoryIDs []string
	ContextBudget     int
	ObservedAt        time.Time
}

type QualityProbe interface {
	RunQualityProbe(ctx context.Context, input QualityProbeInput) ([]QualityEvaluationFinding, error)
}

type ReadQualityDiagnosticsInput struct {
	Scope Scope
}

func (i ReadQualityDiagnosticsInput) Validate() error {
	return i.Scope.Validate()
}

type QualityDiagnostics struct {
	Scope             Scope            `json:"scope"`
	EvaluationStatus  map[string]int64 `json:"evaluation_status,omitempty"`
	FindingCategories map[string]int64 `json:"finding_categories,omitempty"`
	RepairStatus      map[string]int64 `json:"repair_status,omitempty"`
	ObservedAt        time.Time        `json:"observed_at"`
}

func (i CreateQualityEvaluationInput) Validate() error {
	if err := i.Scope.Validate(); err != nil {
		return err
	}
	if len(i.Checks) == 0 {
		return fmt.Errorf("at least one evaluation check is required")
	}
	for _, check := range i.Checks {
		if !check.Valid() {
			return fmt.Errorf("quality evaluation check %q is invalid", check)
		}
	}
	if strings.TrimSpace(i.Actor) == "" {
		return fmt.Errorf("actor is required")
	}
	return nil
}

type ReadQualityEvaluationRunInput struct {
	Scope           Scope
	EvaluationRunID string
}

func (i ReadQualityEvaluationRunInput) Validate() error {
	if err := i.Scope.Validate(); err != nil {
		return err
	}
	if strings.TrimSpace(i.EvaluationRunID) == "" {
		return fmt.Errorf("evaluation run id is required")
	}
	return nil
}

type ListQualityEvaluationFindingsInput struct {
	Scope           Scope
	EvaluationRunID string
}

func (i ListQualityEvaluationFindingsInput) Validate() error {
	if err := i.Scope.Validate(); err != nil {
		return err
	}
	if strings.TrimSpace(i.EvaluationRunID) == "" {
		return fmt.Errorf("evaluation run id is required")
	}
	return nil
}

type CreateRepairPlanInput struct {
	Scope           Scope  `json:"scope"`
	EvaluationRunID string `json:"evaluation_run_id"`
	Actor           string `json:"actor"`
	Reason          string `json:"reason"`
	DryRun          bool   `json:"dry_run"`
}

func (i CreateRepairPlanInput) Validate() error {
	if err := i.Scope.Validate(); err != nil {
		return err
	}
	switch {
	case strings.TrimSpace(i.EvaluationRunID) == "":
		return fmt.Errorf("evaluation run id is required")
	case strings.TrimSpace(i.Actor) == "":
		return fmt.Errorf("actor is required")
	case strings.TrimSpace(i.Reason) == "":
		return fmt.Errorf("reason is required")
	default:
		return nil
	}
}

func (s *QualityService) CreateEvaluation(ctx context.Context, input CreateQualityEvaluationInput) (QualityEvaluationRun, error) {
	if err := input.Validate(); err != nil {
		return QualityEvaluationRun{}, err
	}
	if s.store == nil {
		return QualityEvaluationRun{}, fmt.Errorf("quality store is not configured")
	}
	now := s.now().UTC()
	run, err := s.store.CreateQualityEvaluationRun(ctx, QualityEvaluationRun{
		ID:         s.newID("eval"),
		Scope:      input.Scope.Normalized(),
		Status:     QualityEvaluationStatusCompleted,
		Checks:     append([]QualityEvaluationCheck(nil), input.Checks...),
		Actor:      strings.TrimSpace(input.Actor),
		Reason:     strings.TrimSpace(input.Reason),
		CreatedAt:  now,
		UpdatedAt:  now,
		StartedAt:  now,
		FinishedAt: now,
	})
	if err != nil {
		return QualityEvaluationRun{}, err
	}
	if reader, ok := s.store.(AdmissionPressureSnapshotReader); ok {
		for _, check := range input.Checks {
			if check != QualityEvaluationCheckAdmissionPressure && check != QualityEvaluationCheckRepairPressure && check != QualityEvaluationCheckRetrieval && check != QualityEvaluationCheckContext {
				continue
			}
			operation := AdmissionPressureOperationIngest
			if check == QualityEvaluationCheckRepairPressure {
				operation = AdmissionPressureOperationRepair
			}
			snapshot, err := reader.ReadAdmissionPressureSnapshot(ctx, input.Scope, operation, now)
			if err != nil {
				return QualityEvaluationRun{}, err
			}
			report := AdmissionPressureEvaluator{}.Evaluate(AdmissionPressureInput{
				Scope:      input.Scope,
				Operation:  operation,
				Snapshot:   snapshot,
				ObservedAt: now,
			})
			for _, finding := range report.Findings {
				created, err := s.store.CreateQualityEvaluationFinding(ctx, QualityEvaluationFinding{
					ID:                      s.newID("finding"),
					EvaluationRunID:         run.ID,
					Scope:                   input.Scope.Normalized(),
					Code:                    finding.Code,
					Severity:                finding.Severity,
					Component:               finding.Component,
					Category:                finding.Category,
					Message:                 finding.Message,
					SuggestedActionCategory: finding.SuggestedActionCategory,
					Metadata: map[string]string{
						"check":     string(check),
						"operation": string(operation),
						"decision":  string(report.Decision),
					},
					Evidence:  map[string]any{"snapshot": report.Snapshot},
					CreatedAt: now,
				})
				if err != nil {
					return QualityEvaluationRun{}, err
				}
				_ = created
			}
		}
	}
	if s.probe != nil && hasRetrievalQualityCheck(input.Checks) {
		findings, err := s.probe.RunQualityProbe(ctx, QualityProbeInput{
			Scope:             input.Scope.Normalized(),
			EvaluationRunID:   run.ID,
			Checks:            append([]QualityEvaluationCheck(nil), input.Checks...),
			Query:             strings.TrimSpace(input.Query),
			ExpectedMemoryIDs: append([]string(nil), input.ExpectedMemoryIDs...),
			ContextBudget:     input.ContextBudget,
			ObservedAt:        now,
		})
		if err != nil {
			return QualityEvaluationRun{}, err
		}
		for _, finding := range findings {
			if finding.ID == "" {
				finding.ID = s.newID("finding")
			}
			if finding.EvaluationRunID == "" {
				finding.EvaluationRunID = run.ID
			}
			if finding.Scope == (Scope{}) {
				finding.Scope = input.Scope.Normalized()
			}
			if finding.CreatedAt.IsZero() {
				finding.CreatedAt = now
			}
			if _, err := s.store.CreateQualityEvaluationFinding(ctx, finding); err != nil {
				return QualityEvaluationRun{}, err
			}
		}
	}
	if s.usefulnessFeedback != nil && hasRetrievalQualityCheck(input.Checks) {
		findings, err := s.feedbackDerivedFindings(ctx, input.Scope.Normalized(), run.ID, now)
		if err != nil {
			return QualityEvaluationRun{}, err
		}
		for _, finding := range findings {
			if _, err := s.store.CreateQualityEvaluationFinding(ctx, finding); err != nil {
				return QualityEvaluationRun{}, err
			}
		}
	}
	if s.taskSummarizer != nil && hasRetrievalQualityCheck(input.Checks) {
		findings, err := s.taskDerivedFindings(ctx, input.Scope.Normalized(), run.ID, now)
		if err != nil {
			return QualityEvaluationRun{}, err
		}
		for _, finding := range findings {
			if _, err := s.store.CreateQualityEvaluationFinding(ctx, finding); err != nil {
				return QualityEvaluationRun{}, err
			}
		}
	}
	return run, nil
}

func (s *QualityService) taskDerivedFindings(ctx context.Context, scope Scope, evaluationRunID string, observedAt time.Time) ([]QualityEvaluationFinding, error) {
	summary, err := s.taskSummarizer.SummarizeTaskEvaluations(ctx, SummarizeTaskEvaluationsInput{Scope: scope})
	if err != nil {
		return nil, err
	}
	if summary.ActiveEvaluations == 0 {
		return nil, nil
	}
	findings := make([]QualityEvaluationFinding, 0)
	for _, code := range taskQualityFindingCodes(summary) {
		category, ok := taskFindingAction(code)
		if !ok {
			continue
		}
		findings = append(findings, QualityEvaluationFinding{
			ID:                      s.newID("finding"),
			EvaluationRunID:         evaluationRunID,
			Scope:                   scope,
			Code:                    code,
			Severity:                taskFindingSeverity(code),
			Component:               QualityFindingComponentRetrieval,
			Category:                QualityFindingCategorySemanticProjection,
			Message:                 "active task evaluations indicate a memory quality issue",
			SuggestedActionCategory: category,
			Metadata: map[string]string{
				"source":       "task_evaluations",
				"task_count":   fmt.Sprintf("%d", summary.ActiveEvaluations),
				"task_last_id": summary.LastTaskEvaluationID,
			},
			Evidence: map[string]any{
				"task_evaluation_ids": summary.TaskEvaluationIDs,
				"task_finding_codes":  []string{string(code)},
			},
			CreatedAt: observedAt,
		})
	}
	return findings, nil
}

func taskFindingAction(code QualityFindingCode) (RepairActionCategory, bool) {
	switch code {
	case QualityFindingExpectedRecallMissing:
		return RepairActionCategoryEmbeddingRetry, true
	case QualityFindingFeedbackNoisyRepeated,
		QualityFindingFeedbackStaleRepeated,
		QualityFindingFeedbackIrrelevantRepeated:
		return RepairActionCategorySuppressionReview, true
	case QualityFindingFeedbackUnsafeOrHidden:
		return RepairActionCategoryGovernanceInspection, true
	case QualityFindingFeedbackNeedsReview:
		return RepairActionCategoryManualReview, true
	default:
		return RepairActionCategoryManualReview, false
	}
}

func taskFindingSeverity(code QualityFindingCode) QualityFindingSeverity {
	if code == QualityFindingFeedbackUnsafeOrHidden {
		return QualityFindingSeverityBlocker
	}
	return QualityFindingSeverityWarning
}

func (s *QualityService) feedbackDerivedFindings(ctx context.Context, scope Scope, evaluationRunID string, observedAt time.Time) ([]QualityEvaluationFinding, error) {
	records, err := s.usefulnessFeedback.ListUsefulnessFeedback(ctx, ListUsefulnessFeedbackInput{
		Scope: scope,
		Limit: s.maxPlanItems * 10,
	})
	if err != nil {
		return nil, err
	}
	type aggregate struct {
		feedbackType UsefulnessFeedbackType
		subject      UsefulnessFeedbackSubject
		count        int
	}
	aggregates := map[string]aggregate{}
	for _, record := range records {
		if !record.SupersededAt.IsZero() {
			continue
		}
		for _, subject := range record.Subjects {
			key := string(record.Type) + ":" + usefulnessFeedbackSubjectKey(subject)
			item := aggregates[key]
			item.feedbackType = record.Type
			item.subject = subject
			item.count++
			aggregates[key] = item
		}
	}
	findings := make([]QualityEvaluationFinding, 0)
	for _, item := range aggregates {
		if item.count < 2 && item.feedbackType != UsefulnessFeedbackTypeUnsafeOrHidden && item.feedbackType != UsefulnessFeedbackTypeNeedsReview {
			continue
		}
		code, action, ok := feedbackFindingMapping(item.feedbackType)
		if !ok {
			continue
		}
		evidence := feedbackFindingEvidence(item.subject, item.count)
		findings = append(findings, QualityEvaluationFinding{
			ID:                      s.newID("finding"),
			EvaluationRunID:         evaluationRunID,
			Scope:                   scope,
			Code:                    code,
			Severity:                feedbackFindingSeverity(item.feedbackType),
			Component:               QualityFindingComponentRetrieval,
			Category:                QualityFindingCategorySemanticProjection,
			Message:                 "active usefulness feedback indicates a memory quality issue",
			SuggestedActionCategory: action,
			Metadata: map[string]string{
				"source":        "usefulness_feedback",
				"feedback_type": string(item.feedbackType),
				"active_count":  fmt.Sprintf("%d", item.count),
			},
			Evidence:  evidence,
			CreatedAt: observedAt,
		})
	}
	return findings, nil
}

func feedbackFindingMapping(feedbackType UsefulnessFeedbackType) (QualityFindingCode, RepairActionCategory, bool) {
	switch feedbackType {
	case UsefulnessFeedbackTypeNoisy:
		return QualityFindingFeedbackNoisyRepeated, RepairActionCategorySuppressionReview, true
	case UsefulnessFeedbackTypeStale:
		return QualityFindingFeedbackStaleRepeated, RepairActionCategorySuppressionReview, true
	case UsefulnessFeedbackTypeIrrelevant:
		return QualityFindingFeedbackIrrelevantRepeated, RepairActionCategorySuppressionReview, true
	case UsefulnessFeedbackTypeMissingExpected:
		return QualityFindingFeedbackMissingExpectedRepeated, RepairActionCategoryEmbeddingRetry, true
	case UsefulnessFeedbackTypeUnsafeOrHidden:
		return QualityFindingFeedbackUnsafeOrHidden, RepairActionCategoryGovernanceInspection, true
	case UsefulnessFeedbackTypeNeedsReview:
		return QualityFindingFeedbackNeedsReview, RepairActionCategoryManualReview, true
	default:
		return "", "", false
	}
}

func feedbackFindingSeverity(feedbackType UsefulnessFeedbackType) QualityFindingSeverity {
	if feedbackType == UsefulnessFeedbackTypeUnsafeOrHidden {
		return QualityFindingSeverityBlocker
	}
	return QualityFindingSeverityWarning
}

func feedbackFindingEvidence(subject UsefulnessFeedbackSubject, count int) map[string]any {
	evidence := map[string]any{
		"subject_kind": string(subject.Kind),
		"active_count": count,
	}
	if subject.Kind == UsefulnessFeedbackSubjectExpectedRecall {
		evidence["expected_recall_kind"] = string(subject.ExpectedRecallTarget.Kind)
		if subject.ExpectedRecallTarget.Kind != ExpectedRecallTargetOpaque {
			evidence["memory_id"] = subject.ExpectedRecallTarget.ID
		} else {
			evidence["opaque_expected_recall"] = true
		}
		return evidence
	}
	evidence["subject_id"] = subject.ID
	if subject.Kind == UsefulnessFeedbackSubjectMemory {
		evidence["memory_id"] = subject.ID
	}
	if subject.Kind == UsefulnessFeedbackSubjectRawEvent {
		evidence["raw_event_id"] = subject.ID
	}
	return evidence
}

func hasRetrievalQualityCheck(checks []QualityEvaluationCheck) bool {
	for _, check := range checks {
		if check == QualityEvaluationCheckRetrieval || check == QualityEvaluationCheckContext {
			return true
		}
	}
	return false
}

func (s *QualityService) ReadEvaluation(ctx context.Context, input ReadQualityEvaluationRunInput) (QualityEvaluationRun, error) {
	if err := input.Validate(); err != nil {
		return QualityEvaluationRun{}, err
	}
	if s.store == nil {
		return QualityEvaluationRun{}, fmt.Errorf("quality store is not configured")
	}
	return s.store.ReadQualityEvaluationRun(ctx, input)
}

func (s *QualityService) ListEvaluationFindings(ctx context.Context, input ListQualityEvaluationFindingsInput) ([]QualityEvaluationFinding, error) {
	if err := input.Validate(); err != nil {
		return nil, err
	}
	if s.store == nil {
		return nil, fmt.Errorf("quality store is not configured")
	}
	return s.store.ListQualityEvaluationFindings(ctx, input)
}

func (s *QualityService) ReadDiagnostics(ctx context.Context, input ReadQualityDiagnosticsInput) (QualityDiagnostics, error) {
	if err := input.Validate(); err != nil {
		return QualityDiagnostics{}, err
	}
	if s.store == nil {
		return QualityDiagnostics{}, fmt.Errorf("quality store is not configured")
	}
	if reader, ok := s.store.(QualityDiagnosticsReader); ok {
		return reader.ReadQualityDiagnostics(ctx, input)
	}
	return QualityDiagnostics{
		Scope:             input.Scope.Normalized(),
		EvaluationStatus:  map[string]int64{},
		FindingCategories: map[string]int64{},
		RepairStatus:      map[string]int64{},
		ObservedAt:        s.now().UTC(),
	}, nil
}

func (s *QualityService) CreateRepairPlan(ctx context.Context, input CreateRepairPlanInput) (RepairPlan, error) {
	if err := input.Validate(); err != nil {
		return RepairPlan{}, err
	}
	if s.store == nil {
		return RepairPlan{}, fmt.Errorf("quality store is not configured")
	}
	evaluation, err := s.store.ReadQualityEvaluationRun(ctx, ReadQualityEvaluationRunInput{
		Scope:           input.Scope,
		EvaluationRunID: input.EvaluationRunID,
	})
	if err != nil {
		return RepairPlan{}, err
	}
	findings, err := s.store.ListQualityEvaluationFindings(ctx, ListQualityEvaluationFindingsInput{
		Scope:           input.Scope,
		EvaluationRunID: input.EvaluationRunID,
	})
	if err != nil {
		return RepairPlan{}, err
	}
	if len(findings) > s.maxPlanItems {
		return RepairPlan{}, fmt.Errorf("repair plan target count exceeds configured limit")
	}

	now := s.now().UTC()
	plan := RepairPlan{
		ID:                 s.newID("repair_plan"),
		Scope:              input.Scope.Normalized(),
		EvaluationRunID:    evaluation.ID,
		BaselineRunID:      evaluation.ID,
		Status:             RepairPlanStatusDraft,
		VerificationStatus: RepairVerificationStatusPending,
		DryRun:             input.DryRun,
		Actor:              strings.TrimSpace(input.Actor),
		Reason:             strings.TrimSpace(input.Reason),
		CreatedAt:          now,
		UpdatedAt:          now,
	}
	created, err := s.store.CreateRepairPlan(ctx, plan)
	if err != nil {
		return RepairPlan{}, err
	}
	for _, finding := range findings {
		category := finding.SuggestedActionCategory
		if category == "" {
			category = RepairActionCategoryManualReview
		}
		if category == RepairActionCategoryCanonicalRewrite {
			return RepairPlan{}, fmt.Errorf("%w: canonical rewrite is not a supported repair action", ErrRepairActionRejected)
		}
		if !category.Valid() || category == RepairActionCategoryCanonicalRewrite {
			category = RepairActionCategoryManualReview
		}
		action := RepairAction{
			ID:              s.newID("repair_action"),
			PlanID:          created.ID,
			EvaluationRunID: input.EvaluationRunID,
			FindingID:       finding.ID,
			Scope:           input.Scope.Normalized(),
			Category:        category,
			Status:          RepairActionStatusPending,
			TargetKind:      repairTargetKind(category),
			TargetID:        repairTargetID(category, finding.Evidence),
			ReasonCode:      finding.Code,
			CreatedAt:       now,
			UpdatedAt:       now,
		}
		if category == RepairActionCategoryManualReview || category == RepairActionCategorySuppressionReview || category == RepairActionCategoryGovernanceInspection {
			action.Status = RepairActionStatusManualReview
		}
		createdAction, err := s.store.CreateRepairAction(ctx, action)
		if err != nil {
			return RepairPlan{}, err
		}
		created.Actions = append(created.Actions, createdAction)
	}
	return created, nil
}

func repairTargetKind(category RepairActionCategory) string {
	switch category {
	case RepairActionCategoryEmbeddingRetry:
		return "memory"
	case RepairActionCategoryGovernanceRequeue:
		return "raw_event"
	case RepairActionCategorySuppressionReview:
		return "memory"
	case RepairActionCategoryGovernanceInspection:
		return "raw_event"
	default:
		return ""
	}
}

func repairTargetID(category RepairActionCategory, evidence map[string]any) string {
	if len(evidence) == 0 {
		return ""
	}
	switch category {
	case RepairActionCategoryEmbeddingRetry:
		return evidenceString(evidence, "memory_id")
	case RepairActionCategoryGovernanceRequeue:
		return evidenceString(evidence, "raw_event_id")
	case RepairActionCategorySuppressionReview:
		return evidenceString(evidence, "memory_id")
	case RepairActionCategoryGovernanceInspection:
		return evidenceString(evidence, "raw_event_id")
	default:
		return ""
	}
}

func evidenceString(evidence map[string]any, key string) string {
	value, ok := evidence[key]
	if !ok {
		return ""
	}
	switch typed := value.(type) {
	case string:
		return strings.TrimSpace(typed)
	case fmt.Stringer:
		return strings.TrimSpace(typed.String())
	default:
		return strings.TrimSpace(fmt.Sprint(typed))
	}
}

type ReadRepairPlanInput struct {
	Scope        Scope
	RepairPlanID string
}

func (i ReadRepairPlanInput) Validate() error {
	if err := i.Scope.Validate(); err != nil {
		return err
	}
	if strings.TrimSpace(i.RepairPlanID) == "" {
		return fmt.Errorf("repair plan id is required")
	}
	return nil
}

type ApproveRepairPlanInput struct {
	Scope        Scope
	RepairPlanID string
	Actor        string
	Reason       string
	ApprovedAt   time.Time
}

type VerifyRepairPlanInput struct {
	Scope        Scope
	RepairPlanID string
	Checks       []QualityEvaluationCheck
	Actor        string
	Reason       string
}

func (i VerifyRepairPlanInput) Validate() error {
	if err := i.Scope.Validate(); err != nil {
		return err
	}
	switch {
	case strings.TrimSpace(i.RepairPlanID) == "":
		return fmt.Errorf("repair plan id is required")
	case strings.TrimSpace(i.Actor) == "":
		return fmt.Errorf("actor is required")
	case strings.TrimSpace(i.Reason) == "":
		return fmt.Errorf("reason is required")
	default:
		return nil
	}
}

type UpdateRepairPlanVerificationInput struct {
	Scope              Scope
	RepairPlanID       string
	VerificationRunID  string
	VerificationStatus RepairVerificationStatus
	UpdatedAt          time.Time
}

func (i ApproveRepairPlanInput) Validate() error {
	if err := i.Scope.Validate(); err != nil {
		return err
	}
	switch {
	case strings.TrimSpace(i.RepairPlanID) == "":
		return fmt.Errorf("repair plan id is required")
	case strings.TrimSpace(i.Actor) == "":
		return fmt.Errorf("actor is required")
	case strings.TrimSpace(i.Reason) == "":
		return fmt.Errorf("reason is required")
	case i.ApprovedAt.IsZero():
		return fmt.Errorf("approved at is required")
	default:
		return nil
	}
}

func (s *QualityService) ReadRepairPlan(ctx context.Context, input ReadRepairPlanInput) (RepairPlan, error) {
	if err := input.Validate(); err != nil {
		return RepairPlan{}, err
	}
	if s.store == nil {
		return RepairPlan{}, fmt.Errorf("quality store is not configured")
	}
	return s.store.ReadRepairPlan(ctx, input)
}

func (s *QualityService) ApproveRepairPlan(ctx context.Context, input ApproveRepairPlanInput) (RepairPlan, error) {
	if input.ApprovedAt.IsZero() && s.now != nil {
		input.ApprovedAt = s.now().UTC()
	}
	if err := input.Validate(); err != nil {
		return RepairPlan{}, err
	}
	if s.store == nil {
		return RepairPlan{}, fmt.Errorf("quality store is not configured")
	}
	return s.store.ApproveRepairPlan(ctx, input)
}

func (s *QualityService) VerifyRepairPlan(ctx context.Context, input VerifyRepairPlanInput) (RepairPlan, error) {
	if err := input.Validate(); err != nil {
		return RepairPlan{}, err
	}
	if s.store == nil {
		return RepairPlan{}, fmt.Errorf("quality store is not configured")
	}
	plan, err := s.store.ReadRepairPlan(ctx, ReadRepairPlanInput{Scope: input.Scope, RepairPlanID: input.RepairPlanID})
	if err != nil {
		return RepairPlan{}, err
	}
	checks := input.Checks
	if len(checks) == 0 {
		checks = []QualityEvaluationCheck{QualityEvaluationCheckRetrieval, QualityEvaluationCheckAdmissionPressure, QualityEvaluationCheckRepairPressure}
	}
	verification, err := s.CreateEvaluation(ctx, CreateQualityEvaluationInput{
		Scope:  input.Scope,
		Checks: checks,
		Actor:  input.Actor,
		Reason: input.Reason,
	})
	if err != nil {
		return RepairPlan{}, err
	}
	residual, err := s.store.ListQualityEvaluationFindings(ctx, ListQualityEvaluationFindingsInput{
		Scope:           input.Scope,
		EvaluationRunID: verification.ID,
	})
	if err != nil {
		return RepairPlan{}, err
	}
	targeted := make(map[QualityFindingCode]struct{})
	hasExecutable := false
	for _, action := range plan.Actions {
		if action.ReasonCode != "" {
			targeted[action.ReasonCode] = struct{}{}
		}
		if action.Category != RepairActionCategoryManualReview {
			hasExecutable = true
		}
	}
	status := RepairVerificationStatusPassed
	if !hasExecutable {
		status = RepairVerificationStatusManualReview
	}
	for _, finding := range residual {
		if _, ok := targeted[finding.Code]; ok {
			status = RepairVerificationStatusFailed
			break
		}
	}
	return s.store.UpdateRepairPlanVerification(ctx, UpdateRepairPlanVerificationInput{
		Scope:              input.Scope,
		RepairPlanID:       input.RepairPlanID,
		VerificationRunID:  verification.ID,
		VerificationStatus: status,
		UpdatedAt:          s.now().UTC(),
	})
}

func DefaultAdmissionPressureReport(scope Scope, operation AdmissionPressureOperation, observedAt time.Time) AdmissionPressureReport {
	if observedAt.IsZero() {
		observedAt = time.Now().UTC()
	}
	return AdmissionPressureReport{
		Scope:      scope.Normalized(),
		Operation:  operation,
		Decision:   AdmissionPressureDecisionAccept,
		Snapshot:   AdmissionPressureSnapshot{IntentWritable: true},
		ObservedAt: observedAt.UTC(),
	}
}
