package workflow

import (
	"fmt"
	"strings"
	"time"

	"github.com/FelixSeptem/stele/internal/memory"
)

const (
	maxActorLength          = 256
	maxReasonLength         = 256
	maxIdentifierLength     = 512
	maxIdempotencyKeyLength = 256
	maxMetadataEntries      = 32
	maxMetadataKeyLength    = 128
	maxMetadataValueLength  = 1024
	maxOpaqueTokenLength    = 1024
	minWorkflowWindow       = time.Minute
)

type TemplateStatus string

const (
	TemplateStatusActive   TemplateStatus = "active"
	TemplateStatusDisabled TemplateStatus = "disabled"
)

func (s TemplateStatus) Valid() bool {
	switch s {
	case TemplateStatusActive, TemplateStatusDisabled:
		return true
	default:
		return false
	}
}

type RunStatus string

const (
	RunStatusRunning   RunStatus = "running"
	RunStatusCompleted RunStatus = "completed"
	RunStatusBlocked   RunStatus = "blocked"
	RunStatusExpired   RunStatus = "expired"
	RunStatusAbandoned RunStatus = "abandoned"
)

func (s RunStatus) Valid() bool {
	switch s {
	case RunStatusRunning, RunStatusCompleted, RunStatusBlocked, RunStatusExpired, RunStatusAbandoned:
		return true
	default:
		return false
	}
}

type StepKind string

const (
	StepKindSessionStarted              StepKind = "session_started"
	StepKindContextRequested            StepKind = "context_requested"
	StepKindTurnOutcomeRecorded         StepKind = "turn_outcome_recorded"
	StepKindSessionVerificationRecorded StepKind = "session_verification_recorded"
	StepKindUsefulnessFeedbackRecorded  StepKind = "usefulness_feedback_recorded"
	StepKindTaskEvaluationRecorded      StepKind = "task_evaluation_recorded"
	StepKindQualityChecked              StepKind = "quality_checked"
	StepKindRepairReviewed              StepKind = "repair_reviewed"
	StepKindRankingRolloutChecked       StepKind = "ranking_rollout_checked"
	StepKindConformanceChecked          StepKind = "conformance_checked"
	StepKindReadinessChecked            StepKind = "readiness_checked"
	StepKindRecoveryVerified            StepKind = "recovery_verified"
)

func (k StepKind) Valid() bool {
	switch k {
	case StepKindSessionStarted, StepKindContextRequested, StepKindTurnOutcomeRecorded,
		StepKindSessionVerificationRecorded, StepKindUsefulnessFeedbackRecorded,
		StepKindTaskEvaluationRecorded, StepKindQualityChecked, StepKindRepairReviewed,
		StepKindRankingRolloutChecked, StepKindConformanceChecked, StepKindReadinessChecked,
		StepKindRecoveryVerified:
		return true
	default:
		return false
	}
}

type StepRequirement string

const (
	StepRequirementRequired   StepRequirement = "required"
	StepRequirementOptional   StepRequirement = "optional"
	StepRequirementRepeatable StepRequirement = "repeatable"
)

func (r StepRequirement) Valid() bool {
	switch r {
	case StepRequirementRequired, StepRequirementOptional, StepRequirementRepeatable:
		return true
	default:
		return false
	}
}

type StepStatus string

const (
	StepStatusPending   StepStatus = "pending"
	StepStatusSatisfied StepStatus = "satisfied"
	StepStatusBlocked   StepStatus = "blocked"
	StepStatusStale     StepStatus = "stale"
	StepStatusInvalid   StepStatus = "invalid"
)

func (s StepStatus) Valid() bool {
	switch s {
	case StepStatusPending, StepStatusSatisfied, StepStatusBlocked, StepStatusStale, StepStatusInvalid:
		return true
	default:
		return false
	}
}

type StepResult string

const (
	StepResultRecorded   StepResult = "recorded"
	StepResultDuplicate  StepResult = "duplicate"
	StepResultOutOfOrder StepResult = "out_of_order"
	StepResultRejected   StepResult = "rejected"
)

func (r StepResult) Valid() bool {
	switch r {
	case StepResultRecorded, StepResultDuplicate, StepResultOutOfOrder, StepResultRejected:
		return true
	default:
		return false
	}
}

type EvidenceKind string

const (
	EvidenceKindSession              EvidenceKind = "session"
	EvidenceKindTurn                 EvidenceKind = "turn"
	EvidenceKindContext              EvidenceKind = "context"
	EvidenceKindOutcome              EvidenceKind = "outcome"
	EvidenceKindVerification         EvidenceKind = "verification"
	EvidenceKindUsefulnessFeedback   EvidenceKind = "usefulness_feedback"
	EvidenceKindTaskEvaluation       EvidenceKind = "task_evaluation"
	EvidenceKindProof                EvidenceKind = "proof"
	EvidenceKindQualityFinding       EvidenceKind = "quality_finding"
	EvidenceKindRepairPlan           EvidenceKind = "repair_plan"
	EvidenceKindRankingRollout       EvidenceKind = "ranking_rollout"
	EvidenceKindConformanceRun       EvidenceKind = "conformance_run"
	EvidenceKindReadinessReport      EvidenceKind = "readiness_report"
	EvidenceKindIncident             EvidenceKind = "incident"
	EvidenceKindRecoveryVerification EvidenceKind = "recovery_verification"
	EvidenceKindOpaque               EvidenceKind = "opaque"
)

func (k EvidenceKind) Valid() bool {
	switch k {
	case EvidenceKindSession, EvidenceKindTurn, EvidenceKindContext, EvidenceKindOutcome,
		EvidenceKindVerification, EvidenceKindUsefulnessFeedback, EvidenceKindTaskEvaluation,
		EvidenceKindProof, EvidenceKindQualityFinding, EvidenceKindRepairPlan,
		EvidenceKindRankingRollout, EvidenceKindConformanceRun, EvidenceKindReadinessReport,
		EvidenceKindIncident, EvidenceKindRecoveryVerification, EvidenceKindOpaque:
		return true
	default:
		return false
	}
}

type EvidenceSource string

const (
	EvidenceSourcePublicAPI EvidenceSource = "public_api"
	EvidenceSourceAdminAPI  EvidenceSource = "admin_api"
	EvidenceSourceWorker    EvidenceSource = "worker"
	EvidenceSourceScheduler EvidenceSource = "scheduler"
	EvidenceSourceOpaque    EvidenceSource = "opaque"
)

func (s EvidenceSource) Valid() bool {
	switch s {
	case EvidenceSourcePublicAPI, EvidenceSourceAdminAPI, EvidenceSourceWorker, EvidenceSourceScheduler, EvidenceSourceOpaque:
		return true
	default:
		return false
	}
}

type EvidenceLinkStatus string

const (
	EvidenceLinkStatusActive     EvidenceLinkStatus = "active"
	EvidenceLinkStatusSuperseded EvidenceLinkStatus = "superseded"
	EvidenceLinkStatusInvalid    EvidenceLinkStatus = "invalid"
)

func (s EvidenceLinkStatus) Valid() bool {
	switch s {
	case EvidenceLinkStatusActive, EvidenceLinkStatusSuperseded, EvidenceLinkStatusInvalid:
		return true
	default:
		return false
	}
}

type DiagnosticCategory string

const (
	DiagnosticCategoryMissing              DiagnosticCategory = "missing"
	DiagnosticCategoryStale                DiagnosticCategory = "stale"
	DiagnosticCategoryOutOfOrder           DiagnosticCategory = "out_of_order"
	DiagnosticCategoryDuplicate            DiagnosticCategory = "duplicate"
	DiagnosticCategoryHidden               DiagnosticCategory = "hidden"
	DiagnosticCategoryOpaqueOnly           DiagnosticCategory = "opaque_only"
	DiagnosticCategoryContradictory        DiagnosticCategory = "contradictory"
	DiagnosticCategoryInvalid              DiagnosticCategory = "invalid"
	DiagnosticCategorySubjectMissing       DiagnosticCategory = "subject_missing"
	DiagnosticCategoryInsufficientEvidence DiagnosticCategory = "insufficient_evidence"
	DiagnosticCategoryOutOfScope           DiagnosticCategory = "out_of_scope"
)

func (c DiagnosticCategory) Valid() bool {
	switch c {
	case DiagnosticCategoryMissing, DiagnosticCategoryStale, DiagnosticCategoryOutOfOrder,
		DiagnosticCategoryDuplicate, DiagnosticCategoryHidden, DiagnosticCategoryOpaqueOnly,
		DiagnosticCategoryContradictory, DiagnosticCategoryInvalid, DiagnosticCategorySubjectMissing,
		DiagnosticCategoryInsufficientEvidence, DiagnosticCategoryOutOfScope:
		return true
	default:
		return false
	}
}

type NextActionCategory string

const (
	NextActionStartSession         NextActionCategory = "start_session"
	NextActionRequestContext       NextActionCategory = "request_context"
	NextActionRecordOutcome        NextActionCategory = "record_outcome"
	NextActionRecordVerification   NextActionCategory = "record_verification"
	NextActionRecordFeedback       NextActionCategory = "record_feedback"
	NextActionRecordTaskEvaluation NextActionCategory = "record_task_evaluation"
	NextActionRunScopeProof        NextActionCategory = "run_scope_proof"
	NextActionInspectQuality       NextActionCategory = "inspect_quality"
	NextActionReviewRepair         NextActionCategory = "review_repair"
	NextActionCheckRankingRollout  NextActionCategory = "check_ranking_rollout"
	NextActionRunConformance       NextActionCategory = "run_conformance"
	NextActionReadReadiness        NextActionCategory = "read_readiness"
	NextActionVerifyRecovery       NextActionCategory = "verify_recovery"
	NextActionNone                 NextActionCategory = "none"
)

func (c NextActionCategory) Valid() bool {
	switch c {
	case NextActionStartSession, NextActionRequestContext, NextActionRecordOutcome,
		NextActionRecordVerification, NextActionRecordFeedback, NextActionRecordTaskEvaluation,
		NextActionRunScopeProof, NextActionInspectQuality, NextActionReviewRepair,
		NextActionCheckRankingRollout, NextActionRunConformance, NextActionReadReadiness,
		NextActionVerifyRecovery, NextActionNone:
		return true
	default:
		return false
	}
}

type RouteCategory string

const (
	RouteCategoryMemorySessions            RouteCategory = "memory_sessions"
	RouteCategoryMemorySessionOutcome      RouteCategory = "memory_session_outcome"
	RouteCategoryMemorySessionVerification RouteCategory = "memory_session_verification"
	RouteCategoryUsefulnessFeedback        RouteCategory = "usefulness_feedback"
	RouteCategoryTaskEvaluations           RouteCategory = "task_evaluations"
	RouteCategoryAdminScopeProofs          RouteCategory = "admin_scope_proofs"
	RouteCategoryAdminQuality              RouteCategory = "admin_quality"
	RouteCategoryAdminRepair               RouteCategory = "admin_repair"
	RouteCategoryAdminRankingRollouts      RouteCategory = "admin_ranking_rollouts"
	RouteCategoryAdminConformance          RouteCategory = "admin_conformance"
	RouteCategoryAdminReadiness            RouteCategory = "admin_readiness"
	RouteCategoryAdminRecovery             RouteCategory = "admin_recovery"
	RouteCategoryNone                      RouteCategory = "none"
)

func (c RouteCategory) Valid() bool {
	switch c {
	case RouteCategoryMemorySessions, RouteCategoryMemorySessionOutcome,
		RouteCategoryMemorySessionVerification, RouteCategoryUsefulnessFeedback,
		RouteCategoryTaskEvaluations, RouteCategoryAdminScopeProofs, RouteCategoryAdminQuality,
		RouteCategoryAdminRepair, RouteCategoryAdminRankingRollouts, RouteCategoryAdminConformance,
		RouteCategoryAdminReadiness, RouteCategoryAdminRecovery, RouteCategoryNone:
		return true
	default:
		return false
	}
}

type NextActionStatus string

const (
	NextActionStatusOpen       NextActionStatus = "open"
	NextActionStatusSatisfied  NextActionStatus = "satisfied"
	NextActionStatusSuperseded NextActionStatus = "superseded"
)

func (s NextActionStatus) Valid() bool {
	switch s {
	case NextActionStatusOpen, NextActionStatusSatisfied, NextActionStatusSuperseded:
		return true
	default:
		return false
	}
}

type IntegrationKind string

const (
	IntegrationKindAgentTurn IntegrationKind = "agent_turn"
	IntegrationKindAgentTask IntegrationKind = "agent_task"
	IntegrationKindJob       IntegrationKind = "integration_job"
)

func (k IntegrationKind) Valid() bool {
	switch k {
	case IntegrationKindAgentTurn, IntegrationKindAgentTask, IntegrationKindJob:
		return true
	default:
		return false
	}
}

type CompletionPolicy string

const (
	CompletionPolicyStrict     CompletionPolicy = "strict"
	CompletionPolicyPermissive CompletionPolicy = "permissive"
)

func (p CompletionPolicy) Valid() bool {
	switch p {
	case CompletionPolicyStrict, CompletionPolicyPermissive:
		return true
	default:
		return false
	}
}

type ReadinessImpact string

const (
	ReadinessImpactReady    ReadinessImpact = "ready"
	ReadinessImpactDegraded ReadinessImpact = "degraded"
	ReadinessImpactUnknown  ReadinessImpact = "unknown"
	ReadinessImpactBlocked  ReadinessImpact = "blocked"
)

func (i ReadinessImpact) Valid() bool {
	switch i {
	case ReadinessImpactReady, ReadinessImpactDegraded, ReadinessImpactUnknown, ReadinessImpactBlocked:
		return true
	default:
		return false
	}
}

type RetentionClass string

const (
	RetentionClassDiagnostic RetentionClass = "diagnostic"
	RetentionClassAudit      RetentionClass = "audit"
)

func (c RetentionClass) Valid() bool {
	switch c {
	case RetentionClassDiagnostic, RetentionClassAudit:
		return true
	default:
		return false
	}
}

type WorkflowTemplate struct {
	ID               string           `json:"id"`
	Scope            memory.Scope     `json:"scope"`
	Status           TemplateStatus   `json:"status"`
	Steps            []TemplateStep   `json:"steps,omitempty"`
	IntegrationKind  IntegrationKind  `json:"integration_kind"`
	CompletionPolicy CompletionPolicy `json:"completion_policy"`
	Actor            string           `json:"actor"`
	Reason           string           `json:"reason"`
	Metadata         map[string]any   `json:"metadata,omitempty"`
	CreatedAt        time.Time        `json:"created_at"`
	UpdatedAt        time.Time        `json:"updated_at"`
	DisabledAt       time.Time        `json:"disabled_at,omitempty"`
}

func (t WorkflowTemplate) Validate() error {
	if err := validateID(t.ID, "workflow template id"); err != nil {
		return err
	}
	if err := t.Scope.Validate(); err != nil {
		return err
	}
	if !t.Status.Valid() {
		return fmt.Errorf("workflow template status %q is invalid", t.Status)
	}
	if !t.IntegrationKind.Valid() {
		return fmt.Errorf("integration kind %q is invalid", t.IntegrationKind)
	}
	if !t.CompletionPolicy.Valid() {
		return fmt.Errorf("completion policy %q is invalid", t.CompletionPolicy)
	}
	if len(t.Steps) == 0 {
		return fmt.Errorf("workflow template steps are required")
	}
	for _, step := range t.Steps {
		if step.TemplateID == "" {
			step.TemplateID = t.ID
		}
		if step.Scope == (memory.Scope{}) {
			step.Scope = t.Scope
		}
		if err := step.Validate(); err != nil {
			return err
		}
		if step.TemplateID != t.ID {
			return fmt.Errorf("workflow template step template id does not match template")
		}
		if step.Scope.Normalized() != t.Scope.Normalized() {
			return fmt.Errorf("workflow template step scope does not match template")
		}
	}
	if err := validateActorReason(t.Actor, t.Reason); err != nil {
		return err
	}
	if err := validateMetadata(t.Metadata, "metadata"); err != nil {
		return err
	}
	if t.CreatedAt.IsZero() || t.UpdatedAt.IsZero() {
		return fmt.Errorf("created at and updated at are required")
	}
	return nil
}

type TemplateStep struct {
	ID               string          `json:"id"`
	TemplateID       string          `json:"template_id"`
	Scope            memory.Scope    `json:"scope"`
	Kind             StepKind        `json:"kind"`
	Requirement      StepRequirement `json:"requirement"`
	AllowedEvidence  []EvidenceKind  `json:"allowed_evidence"`
	MinimumCount     int             `json:"minimum_count"`
	RequiresInternal bool            `json:"requires_internal"`
	FreshnessWindow  time.Duration   `json:"freshness_window"`
	CompletionWindow time.Duration   `json:"completion_window"`
	Position         int             `json:"position"`
	Metadata         map[string]any  `json:"metadata,omitempty"`
	CreatedAt        time.Time       `json:"created_at"`
}

func (s TemplateStep) Validate() error {
	if err := validateID(s.ID, "workflow template step id"); err != nil {
		return err
	}
	if err := validateID(s.TemplateID, "workflow template id"); err != nil {
		return err
	}
	if err := s.Scope.Validate(); err != nil {
		return err
	}
	if !s.Kind.Valid() {
		return fmt.Errorf("workflow step kind %q is invalid", s.Kind)
	}
	if !s.Requirement.Valid() {
		return fmt.Errorf("workflow step requirement %q is invalid", s.Requirement)
	}
	if len(s.AllowedEvidence) == 0 {
		return fmt.Errorf("allowed evidence is required")
	}
	for _, kind := range s.AllowedEvidence {
		if !kind.Valid() {
			return fmt.Errorf("workflow evidence kind %q is invalid", kind)
		}
	}
	if s.MinimumCount <= 0 {
		return fmt.Errorf("minimum evidence count must be greater than zero")
	}
	if s.FreshnessWindow < minWorkflowWindow {
		return fmt.Errorf("freshness window must be at least %s", minWorkflowWindow)
	}
	if s.CompletionWindow < minWorkflowWindow {
		return fmt.Errorf("completion window must be at least %s", minWorkflowWindow)
	}
	if s.Position <= 0 {
		return fmt.Errorf("position must be greater than zero")
	}
	if err := validateMetadata(s.Metadata, "metadata"); err != nil {
		return err
	}
	if s.CreatedAt.IsZero() {
		return fmt.Errorf("created at is required")
	}
	return nil
}

type WorkflowRun struct {
	ID              string          `json:"id"`
	TemplateID      string          `json:"template_id"`
	Scope           memory.Scope    `json:"scope"`
	Status          RunStatus       `json:"status"`
	IntegrationKind IntegrationKind `json:"integration_kind"`
	IdempotencyKey  string          `json:"idempotency_key"`
	Actor           string          `json:"actor"`
	Reason          string          `json:"reason"`
	Metadata        map[string]any  `json:"metadata,omitempty"`
	CreatedAt       time.Time       `json:"created_at"`
	UpdatedAt       time.Time       `json:"updated_at"`
	StartedAt       time.Time       `json:"started_at"`
	CompletedAt     time.Time       `json:"completed_at,omitempty"`
	ExpiresAt       time.Time       `json:"expires_at,omitempty"`
}

func (r WorkflowRun) Validate() error {
	if err := validateID(r.ID, "workflow run id"); err != nil {
		return err
	}
	if err := validateID(r.TemplateID, "workflow template id"); err != nil {
		return err
	}
	if err := r.Scope.Validate(); err != nil {
		return err
	}
	if !r.Status.Valid() {
		return fmt.Errorf("workflow run status %q is invalid", r.Status)
	}
	if !r.IntegrationKind.Valid() {
		return fmt.Errorf("integration kind %q is invalid", r.IntegrationKind)
	}
	if err := validateIdempotencyKey(r.IdempotencyKey); err != nil {
		return err
	}
	if err := validateActorReason(r.Actor, r.Reason); err != nil {
		return err
	}
	if err := validateMetadata(r.Metadata, "metadata"); err != nil {
		return err
	}
	if r.CreatedAt.IsZero() || r.UpdatedAt.IsZero() || r.StartedAt.IsZero() {
		return fmt.Errorf("created at, updated at, and started at are required")
	}
	return nil
}

type WorkflowStepRecord struct {
	ID            string         `json:"id"`
	RunID         string         `json:"run_id"`
	Scope         memory.Scope   `json:"scope"`
	Kind          StepKind       `json:"kind"`
	Status        StepStatus     `json:"status"`
	Result        StepResult     `json:"result"`
	Actor         string         `json:"actor"`
	Reason        string         `json:"reason"`
	Metadata      map[string]any `json:"metadata,omitempty"`
	ObservedAt    time.Time      `json:"observed_at"`
	CreatedAt     time.Time      `json:"created_at"`
	EvidenceLinks []EvidenceLink `json:"evidence_links,omitempty"`
}

func (r WorkflowStepRecord) Validate() error {
	if err := validateID(r.ID, "workflow step record id"); err != nil {
		return err
	}
	if err := validateID(r.RunID, "workflow run id"); err != nil {
		return err
	}
	if err := r.Scope.Validate(); err != nil {
		return err
	}
	if !r.Kind.Valid() {
		return fmt.Errorf("workflow step kind %q is invalid", r.Kind)
	}
	if !r.Status.Valid() {
		return fmt.Errorf("workflow step status %q is invalid", r.Status)
	}
	if !r.Result.Valid() {
		return fmt.Errorf("workflow step result %q is invalid", r.Result)
	}
	if err := validateActorReason(r.Actor, r.Reason); err != nil {
		return err
	}
	if err := validateMetadata(r.Metadata, "metadata"); err != nil {
		return err
	}
	if r.ObservedAt.IsZero() || r.CreatedAt.IsZero() {
		return fmt.Errorf("observed at and created at are required")
	}
	for _, link := range r.EvidenceLinks {
		if link.RunID == "" {
			link.RunID = r.RunID
		}
		if link.StepRecordID == "" {
			link.StepRecordID = r.ID
		}
		if err := link.Validate(); err != nil {
			return err
		}
		if link.Scope.Normalized() != r.Scope.Normalized() {
			return fmt.Errorf("evidence link scope does not match workflow step record")
		}
		if link.RunID != r.RunID {
			return fmt.Errorf("evidence link run id does not match workflow step record")
		}
		if link.StepRecordID != "" && link.StepRecordID != r.ID {
			return fmt.Errorf("evidence link step record id does not match workflow step record")
		}
	}
	return nil
}

type EvidenceLink struct {
	ID           string             `json:"id"`
	RunID        string             `json:"run_id"`
	StepRecordID string             `json:"step_record_id,omitempty"`
	Scope        memory.Scope       `json:"scope"`
	Kind         EvidenceKind       `json:"kind"`
	Status       EvidenceLinkStatus `json:"status"`
	Source       EvidenceSource     `json:"source"`
	TargetID     string             `json:"target_id,omitempty"`
	OpaqueToken  string             `json:"opaque_token,omitempty"`
	Metadata     map[string]any     `json:"metadata,omitempty"`
	CreatedAt    time.Time          `json:"created_at"`
	SupersededAt time.Time          `json:"superseded_at,omitempty"`
}

func (l EvidenceLink) Validate() error {
	if err := validateID(l.ID, "evidence link id"); err != nil {
		return err
	}
	if err := validateID(l.RunID, "workflow run id"); err != nil {
		return err
	}
	if err := l.Scope.Validate(); err != nil {
		return err
	}
	if !l.Kind.Valid() {
		return fmt.Errorf("workflow evidence kind %q is invalid", l.Kind)
	}
	if !l.Status.Valid() {
		return fmt.Errorf("evidence link status %q is invalid", l.Status)
	}
	if !l.Source.Valid() {
		return fmt.Errorf("evidence source %q is invalid", l.Source)
	}
	if l.Kind == EvidenceKindOpaque || l.Source == EvidenceSourceOpaque {
		if strings.TrimSpace(l.OpaqueToken) == "" {
			return fmt.Errorf("opaque evidence token is required")
		}
		if len(l.OpaqueToken) > maxOpaqueTokenLength {
			return fmt.Errorf("opaque evidence token must be at most %d bytes", maxOpaqueTokenLength)
		}
	} else if err := validateID(l.TargetID, "evidence target id"); err != nil {
		return err
	}
	if err := validateMetadata(l.Metadata, "metadata"); err != nil {
		return err
	}
	if l.CreatedAt.IsZero() {
		return fmt.Errorf("created at is required")
	}
	return nil
}

type GapDiagnostic struct {
	ID              string             `json:"id"`
	RunID           string             `json:"run_id"`
	StepRecordID    string             `json:"step_record_id,omitempty"`
	EvidenceLinkID  string             `json:"evidence_link_id,omitempty"`
	Scope           memory.Scope       `json:"scope"`
	StepKind        StepKind           `json:"step_kind"`
	EvidenceKind    EvidenceKind       `json:"evidence_kind"`
	Category        DiagnosticCategory `json:"category"`
	ReadinessImpact ReadinessImpact    `json:"readiness_impact"`
	Status          string             `json:"status,omitempty"`
	Metadata        map[string]any     `json:"metadata,omitempty"`
	CreatedAt       time.Time          `json:"created_at"`
	ResolvedAt      time.Time          `json:"resolved_at,omitempty"`
}

func (d GapDiagnostic) Validate() error {
	if err := validateID(d.ID, "workflow gap diagnostic id"); err != nil {
		return err
	}
	if err := validateID(d.RunID, "workflow run id"); err != nil {
		return err
	}
	if err := d.Scope.Validate(); err != nil {
		return err
	}
	if !d.StepKind.Valid() {
		return fmt.Errorf("workflow step kind %q is invalid", d.StepKind)
	}
	if !d.EvidenceKind.Valid() {
		return fmt.Errorf("workflow evidence kind %q is invalid", d.EvidenceKind)
	}
	if !d.Category.Valid() {
		return fmt.Errorf("workflow diagnostic category %q is invalid", d.Category)
	}
	if !d.ReadinessImpact.Valid() {
		return fmt.Errorf("readiness impact %q is invalid", d.ReadinessImpact)
	}
	if err := validateMetadata(d.Metadata, "metadata"); err != nil {
		return err
	}
	if d.CreatedAt.IsZero() {
		return fmt.Errorf("created at is required")
	}
	return nil
}

type NextAction struct {
	ID            string             `json:"id"`
	RunID         string             `json:"run_id"`
	Scope         memory.Scope       `json:"scope"`
	Category      NextActionCategory `json:"category"`
	StepKind      StepKind           `json:"step_kind"`
	EvidenceKind  EvidenceKind       `json:"evidence_kind"`
	RouteCategory RouteCategory      `json:"route_category"`
	Status        NextActionStatus   `json:"status"`
	Metadata      map[string]any     `json:"metadata,omitempty"`
	CreatedAt     time.Time          `json:"created_at"`
	ResolvedAt    time.Time          `json:"resolved_at,omitempty"`
}

func (a NextAction) Validate() error {
	if err := validateID(a.ID, "workflow next action id"); err != nil {
		return err
	}
	if err := validateID(a.RunID, "workflow run id"); err != nil {
		return err
	}
	if err := a.Scope.Validate(); err != nil {
		return err
	}
	if !a.Category.Valid() {
		return fmt.Errorf("next action category %q is invalid", a.Category)
	}
	if !a.StepKind.Valid() {
		return fmt.Errorf("workflow step kind %q is invalid", a.StepKind)
	}
	if !a.EvidenceKind.Valid() {
		return fmt.Errorf("workflow evidence kind %q is invalid", a.EvidenceKind)
	}
	if !a.RouteCategory.Valid() {
		return fmt.Errorf("route category %q is invalid", a.RouteCategory)
	}
	if !a.Status.Valid() {
		return fmt.Errorf("next action status %q is invalid", a.Status)
	}
	if err := validateMetadata(a.Metadata, "metadata"); err != nil {
		return err
	}
	if a.CreatedAt.IsZero() {
		return fmt.Errorf("created at is required")
	}
	return nil
}

type WorkflowTransition struct {
	ID         string       `json:"id"`
	RunID      string       `json:"run_id"`
	Scope      memory.Scope `json:"scope"`
	FromStatus RunStatus    `json:"from_status,omitempty"`
	ToStatus   RunStatus    `json:"to_status"`
	Actor      string       `json:"actor"`
	Reason     string       `json:"reason"`
	OccurredAt time.Time    `json:"occurred_at"`
}

func (t WorkflowTransition) Validate() error {
	if err := validateID(t.ID, "workflow transition id"); err != nil {
		return err
	}
	if err := validateID(t.RunID, "workflow run id"); err != nil {
		return err
	}
	if err := t.Scope.Validate(); err != nil {
		return err
	}
	if t.FromStatus != "" && !t.FromStatus.Valid() {
		return fmt.Errorf("from workflow run status %q is invalid", t.FromStatus)
	}
	if !t.ToStatus.Valid() {
		return fmt.Errorf("to workflow run status %q is invalid", t.ToStatus)
	}
	if err := validateActorReason(t.Actor, t.Reason); err != nil {
		return err
	}
	if t.OccurredAt.IsZero() {
		return fmt.Errorf("occurred at is required")
	}
	return nil
}

type WorkflowRetentionRun struct {
	ID             string         `json:"id"`
	Scope          memory.Scope   `json:"scope"`
	RecordCategory RetentionClass `json:"record_category"`
	Cutoff         time.Time      `json:"cutoff"`
	DeletedCount   int            `json:"deleted_count"`
	StartedAt      time.Time      `json:"started_at"`
	FinishedAt     time.Time      `json:"finished_at,omitempty"`
}

func (r WorkflowRetentionRun) Validate() error {
	if err := validateID(r.ID, "workflow retention run id"); err != nil {
		return err
	}
	if err := r.Scope.Validate(); err != nil {
		return err
	}
	if !r.RecordCategory.Valid() {
		return fmt.Errorf("retention class %q is invalid", r.RecordCategory)
	}
	if r.Cutoff.IsZero() || r.StartedAt.IsZero() {
		return fmt.Errorf("cutoff and started at are required")
	}
	if r.DeletedCount < 0 {
		return fmt.Errorf("deleted count must be greater than or equal to zero")
	}
	return nil
}

type ReadTemplateInput struct {
	Scope      memory.Scope
	TemplateID string
}

func (i ReadTemplateInput) Validate() error {
	if err := i.Scope.Validate(); err != nil {
		return err
	}
	return validateID(i.TemplateID, "workflow template id")
}

type ListTemplatesInput struct {
	Scope  memory.Scope
	Status TemplateStatus
	Limit  int
}

func (i ListTemplatesInput) Validate() error {
	if err := i.Scope.Validate(); err != nil {
		return err
	}
	if i.Status != "" && !i.Status.Valid() {
		return fmt.Errorf("workflow template status %q is invalid", i.Status)
	}
	return validateLimit(i.Limit)
}

type UpdateTemplateInput struct {
	Scope            memory.Scope
	TemplateID       string
	Steps            []TemplateStep
	IntegrationKind  IntegrationKind
	CompletionPolicy CompletionPolicy
	Actor            string
	Reason           string
	Metadata         map[string]any
	UpdatedAt        time.Time
}

func (i UpdateTemplateInput) Validate() error {
	if err := i.Scope.Validate(); err != nil {
		return err
	}
	if err := validateID(i.TemplateID, "workflow template id"); err != nil {
		return err
	}
	if !i.IntegrationKind.Valid() {
		return fmt.Errorf("integration kind %q is invalid", i.IntegrationKind)
	}
	if !i.CompletionPolicy.Valid() {
		return fmt.Errorf("completion policy %q is invalid", i.CompletionPolicy)
	}
	if len(i.Steps) == 0 {
		return fmt.Errorf("workflow template steps are required")
	}
	for _, step := range i.Steps {
		if step.TemplateID == "" {
			step.TemplateID = i.TemplateID
		}
		if step.Scope == (memory.Scope{}) {
			step.Scope = i.Scope
		}
		if err := step.Validate(); err != nil {
			return err
		}
		if step.TemplateID != i.TemplateID {
			return fmt.Errorf("workflow template step template id does not match template")
		}
		if step.Scope.Normalized() != i.Scope.Normalized() {
			return fmt.Errorf("workflow template step scope does not match template")
		}
	}
	if err := validateActorReason(i.Actor, i.Reason); err != nil {
		return err
	}
	if err := validateMetadata(i.Metadata, "metadata"); err != nil {
		return err
	}
	if i.UpdatedAt.IsZero() {
		return fmt.Errorf("updated at is required")
	}
	return nil
}

type DisableTemplateInput struct {
	Scope      memory.Scope
	TemplateID string
	Actor      string
	Reason     string
	DisabledAt time.Time
}

func (i DisableTemplateInput) Validate() error {
	if err := i.Scope.Validate(); err != nil {
		return err
	}
	if err := validateID(i.TemplateID, "workflow template id"); err != nil {
		return err
	}
	if err := validateActorReason(i.Actor, i.Reason); err != nil {
		return err
	}
	if i.DisabledAt.IsZero() {
		return fmt.Errorf("disabled at is required")
	}
	return nil
}

type ReadRunInput struct {
	Scope memory.Scope
	RunID string
}

func (i ReadRunInput) Validate() error {
	if err := i.Scope.Validate(); err != nil {
		return err
	}
	return validateID(i.RunID, "workflow run id")
}

type ListRunsInput struct {
	Scope      memory.Scope
	TemplateID string
	Status     RunStatus
	Limit      int
}

func (i ListRunsInput) Validate() error {
	if err := i.Scope.Validate(); err != nil {
		return err
	}
	if i.TemplateID != "" {
		if err := validateID(i.TemplateID, "workflow template id"); err != nil {
			return err
		}
	}
	if i.Status != "" && !i.Status.Valid() {
		return fmt.Errorf("workflow run status %q is invalid", i.Status)
	}
	return validateLimit(i.Limit)
}

type TransitionRunInput struct {
	Transition WorkflowTransition
	UpdatedAt  time.Time
}

func (i TransitionRunInput) Validate() error {
	if err := i.Transition.Validate(); err != nil {
		return err
	}
	if i.UpdatedAt.IsZero() {
		return fmt.Errorf("updated at is required")
	}
	return nil
}

type ListStepRecordsInput struct {
	Scope memory.Scope
	RunID string
}

func (i ListStepRecordsInput) Validate() error {
	if err := i.Scope.Validate(); err != nil {
		return err
	}
	return validateID(i.RunID, "workflow run id")
}

type ListEvidenceLinksInput struct {
	Scope  memory.Scope
	RunID  string
	Status EvidenceLinkStatus
}

func (i ListEvidenceLinksInput) Validate() error {
	if err := i.Scope.Validate(); err != nil {
		return err
	}
	if err := validateID(i.RunID, "workflow run id"); err != nil {
		return err
	}
	if i.Status != "" && !i.Status.Valid() {
		return fmt.Errorf("evidence link status %q is invalid", i.Status)
	}
	return nil
}

type ListDiagnosticsInput struct {
	Scope    memory.Scope
	RunID    string
	Category DiagnosticCategory
	Limit    int
}

func (i ListDiagnosticsInput) Validate() error {
	if err := i.Scope.Validate(); err != nil {
		return err
	}
	if err := validateID(i.RunID, "workflow run id"); err != nil {
		return err
	}
	if i.Category != "" && !i.Category.Valid() {
		return fmt.Errorf("workflow diagnostic category %q is invalid", i.Category)
	}
	return validateLimit(i.Limit)
}

type ListNextActionsInput struct {
	Scope  memory.Scope
	RunID  string
	Status NextActionStatus
	Limit  int
}

func (i ListNextActionsInput) Validate() error {
	if err := i.Scope.Validate(); err != nil {
		return err
	}
	if err := validateID(i.RunID, "workflow run id"); err != nil {
		return err
	}
	if i.Status != "" && !i.Status.Valid() {
		return fmt.Errorf("next action status %q is invalid", i.Status)
	}
	return validateLimit(i.Limit)
}

type SupersedeEvidenceLinkInput struct {
	Scope        memory.Scope
	LinkID       string
	Actor        string
	Reason       string
	SupersededAt time.Time
}

func (i SupersedeEvidenceLinkInput) Validate() error {
	if err := i.Scope.Validate(); err != nil {
		return err
	}
	if err := validateID(i.LinkID, "evidence link id"); err != nil {
		return err
	}
	if err := validateActorReason(i.Actor, i.Reason); err != nil {
		return err
	}
	if i.SupersededAt.IsZero() {
		return fmt.Errorf("superseded at is required")
	}
	return nil
}

type FindRetentionEligibleHistoryInput struct {
	Scope          memory.Scope
	RecordCategory RetentionClass
	Cutoff         time.Time
	Limit          int
}

type FindMaintenanceRunsInput struct {
	Scope      memory.Scope
	ObservedAt time.Time
	StaleAfter time.Duration
	Limit      int
}

func (i FindMaintenanceRunsInput) Validate() error {
	if err := i.Scope.Validate(); err != nil {
		return err
	}
	if i.ObservedAt.IsZero() {
		return fmt.Errorf("workflow maintenance observed at is required")
	}
	if i.StaleAfter < minWorkflowWindow {
		return fmt.Errorf("workflow stale window must be at least %s", minWorkflowWindow)
	}
	return validateLimit(i.Limit)
}

func (i FindRetentionEligibleHistoryInput) Validate() error {
	if err := i.Scope.Validate(); err != nil {
		return err
	}
	if !i.RecordCategory.Valid() {
		return fmt.Errorf("retention class %q is invalid", i.RecordCategory)
	}
	if i.Cutoff.IsZero() {
		return fmt.Errorf("cutoff is required")
	}
	return validatePositiveLimit(i.Limit)
}

type ListRetentionRunsInput struct {
	Scope          memory.Scope
	RecordCategory RetentionClass
}

func (i ListRetentionRunsInput) Validate() error {
	if err := i.Scope.Validate(); err != nil {
		return err
	}
	if i.RecordCategory != "" && !i.RecordCategory.Valid() {
		return fmt.Errorf("retention class %q is invalid", i.RecordCategory)
	}
	return nil
}

type ReadRetentionRunInput struct {
	Scope memory.Scope
	RunID string
}

func (i ReadRetentionRunInput) Validate() error {
	if err := i.Scope.Validate(); err != nil {
		return err
	}
	return validateID(i.RunID, "workflow retention run id")
}

func StepSatisfiedByEvidence(step TemplateStep, links []EvidenceLink, now time.Time) bool {
	if step.MinimumCount <= 0 {
		return false
	}
	count := 0
	for _, link := range links {
		if link.Status != EvidenceLinkStatusActive {
			continue
		}
		if step.RequiresInternal && (link.Kind == EvidenceKindOpaque || link.Source == EvidenceSourceOpaque) {
			continue
		}
		if !evidenceAllowed(step.AllowedEvidence, link.Kind) {
			continue
		}
		if step.FreshnessWindow >= minWorkflowWindow && !now.IsZero() && now.Sub(link.CreatedAt) > step.FreshnessWindow {
			continue
		}
		count++
		if count >= step.MinimumCount {
			return true
		}
	}
	return false
}

func evidenceAllowed(allowed []EvidenceKind, kind EvidenceKind) bool {
	for _, candidate := range allowed {
		if candidate == kind {
			return true
		}
	}
	return false
}

func validateID(id string, label string) error {
	if strings.TrimSpace(id) == "" {
		return fmt.Errorf("%s is required", label)
	}
	if len(id) > maxIdentifierLength {
		return fmt.Errorf("%s must be at most %d bytes", label, maxIdentifierLength)
	}
	return nil
}

func validateIdempotencyKey(key string) error {
	if strings.TrimSpace(key) == "" {
		return fmt.Errorf("idempotency key is required")
	}
	if len(key) > maxIdempotencyKeyLength {
		return fmt.Errorf("idempotency key must be at most %d bytes", maxIdempotencyKeyLength)
	}
	return nil
}

func validateActorReason(actor string, reason string) error {
	if strings.TrimSpace(actor) == "" {
		return fmt.Errorf("actor is required")
	}
	if len(actor) > maxActorLength {
		return fmt.Errorf("actor must be at most %d bytes", maxActorLength)
	}
	if strings.TrimSpace(reason) == "" {
		return fmt.Errorf("reason is required")
	}
	if len(reason) > maxReasonLength {
		return fmt.Errorf("reason must be at most %d bytes", maxReasonLength)
	}
	return nil
}

func validateMetadata(metadata map[string]any, label string) error {
	if len(metadata) > maxMetadataEntries {
		return fmt.Errorf("%s must contain at most %d entries", label, maxMetadataEntries)
	}
	for key, value := range metadata {
		if strings.TrimSpace(key) == "" {
			return fmt.Errorf("%s key is required", label)
		}
		if len(key) > maxMetadataKeyLength {
			return fmt.Errorf("%s key must be at most %d bytes", label, maxMetadataKeyLength)
		}
		if len(fmt.Sprint(value)) > maxMetadataValueLength {
			return fmt.Errorf("%s value must be at most %d bytes", label, maxMetadataValueLength)
		}
	}
	return nil
}

func validateLimit(limit int) error {
	if limit < 0 {
		return fmt.Errorf("limit must be greater than or equal to zero")
	}
	return nil
}

func validatePositiveLimit(limit int) error {
	if limit <= 0 {
		return fmt.Errorf("limit must be greater than zero")
	}
	return nil
}
