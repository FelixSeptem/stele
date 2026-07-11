package memory

import (
	"fmt"
	"strings"
	"time"
)

type ScopeProofStatus string

const (
	ScopeProofStatusPending      ScopeProofStatus = "pending"
	ScopeProofStatusRunning      ScopeProofStatus = "running"
	ScopeProofStatusCompleted    ScopeProofStatus = "completed"
	ScopeProofStatusFailed       ScopeProofStatus = "failed"
	ScopeProofStatusManualReview ScopeProofStatus = "manual_review"
)

func (s ScopeProofStatus) Valid() bool {
	switch s {
	case ScopeProofStatusPending, ScopeProofStatusRunning, ScopeProofStatusCompleted, ScopeProofStatusFailed, ScopeProofStatusManualReview:
		return true
	default:
		return false
	}
}

type ScopeProofStepStatus string

const (
	ScopeProofStepStatusPending      ScopeProofStepStatus = "pending"
	ScopeProofStepStatusRunning      ScopeProofStepStatus = "running"
	ScopeProofStepStatusCompleted    ScopeProofStepStatus = "completed"
	ScopeProofStepStatusFailed       ScopeProofStepStatus = "failed"
	ScopeProofStepStatusSkipped      ScopeProofStepStatus = "skipped"
	ScopeProofStepStatusManualReview ScopeProofStepStatus = "manual_review"
	ScopeProofStepStatusExhausted    ScopeProofStepStatus = "exhausted"
)

func (s ScopeProofStepStatus) Valid() bool {
	switch s {
	case ScopeProofStepStatusPending, ScopeProofStepStatusRunning, ScopeProofStepStatusCompleted, ScopeProofStepStatusFailed, ScopeProofStepStatusSkipped, ScopeProofStepStatusManualReview, ScopeProofStepStatusExhausted:
		return true
	default:
		return false
	}
}

type ScopeProofVerdict string

const (
	ScopeProofVerdictPending        ScopeProofVerdict = "pending"
	ScopeProofVerdictPassed         ScopeProofVerdict = "passed"
	ScopeProofVerdictPassedDegraded ScopeProofVerdict = "passed_degraded"
	ScopeProofVerdictFailed         ScopeProofVerdict = "failed"
	ScopeProofVerdictManualReview   ScopeProofVerdict = "manual_review"
)

func (v ScopeProofVerdict) Valid() bool {
	switch v {
	case ScopeProofVerdictPending, ScopeProofVerdictPassed, ScopeProofVerdictPassedDegraded, ScopeProofVerdictFailed, ScopeProofVerdictManualReview:
		return true
	default:
		return false
	}
}

type ScopeProofCheck string

const (
	ScopeProofCheckScopeResolution ScopeProofCheck = "scope_resolution"
	ScopeProofCheckIngestion       ScopeProofCheck = "ingestion"
	ScopeProofCheckGovernance      ScopeProofCheck = "governance"
	ScopeProofCheckRetrieval       ScopeProofCheck = "retrieval"
	ScopeProofCheckContext         ScopeProofCheck = "context"
	ScopeProofCheckReplay          ScopeProofCheck = "replay"
	ScopeProofCheckQuality         ScopeProofCheck = "quality"
	ScopeProofCheckRepair          ScopeProofCheck = "repair"
)

func (c ScopeProofCheck) Valid() bool {
	switch c {
	case ScopeProofCheckScopeResolution, ScopeProofCheckIngestion, ScopeProofCheckGovernance, ScopeProofCheckRetrieval, ScopeProofCheckContext, ScopeProofCheckReplay, ScopeProofCheckQuality, ScopeProofCheckRepair:
		return true
	default:
		return false
	}
}

type ScopeProofFixtureMode string

const (
	ScopeProofFixtureModeSmoke            ScopeProofFixtureMode = "smoke"
	ScopeProofFixtureModeOperatorProvided ScopeProofFixtureMode = "operator_provided"
	ScopeProofFixtureModeNone             ScopeProofFixtureMode = "none"
)

func (m ScopeProofFixtureMode) Valid() bool {
	switch m {
	case ScopeProofFixtureModeSmoke, ScopeProofFixtureModeOperatorProvided, ScopeProofFixtureModeNone:
		return true
	default:
		return false
	}
}

type ScopeProofStepName string

const (
	ScopeProofStepScopeResolved       ScopeProofStepName = "scope_resolved"
	ScopeProofStepFixturePlanned      ScopeProofStepName = "fixture_planned"
	ScopeProofStepIngestion           ScopeProofStepName = "ingest_accepted"
	ScopeProofStepGovernanceProcessed ScopeProofStepName = "governance_processed"
	ScopeProofStepRetrievalRecalled   ScopeProofStepName = "retrieval_recalled"
	ScopeProofStepContextAssembled    ScopeProofStepName = "context_assembled"
	ScopeProofStepReplayChecked       ScopeProofStepName = "derived_insight_checked"
	ScopeProofStepQualityEvaluated    ScopeProofStepName = "quality_evaluated"
	ScopeProofStepRepairRecommended   ScopeProofStepName = "repair_recommended"
	ScopeProofStepCompleted           ScopeProofStepName = "proof_completed"
)

type ProofFailureCategory string

const (
	ProofFailureCategoryScope       ProofFailureCategory = "scope"
	ProofFailureCategoryIngestion   ProofFailureCategory = "ingestion"
	ProofFailureCategoryGovernance  ProofFailureCategory = "governance"
	ProofFailureCategoryRetrieval   ProofFailureCategory = "retrieval"
	ProofFailureCategoryContext     ProofFailureCategory = "context"
	ProofFailureCategoryReplay      ProofFailureCategory = "replay"
	ProofFailureCategoryQuality     ProofFailureCategory = "quality"
	ProofFailureCategoryRepair      ProofFailureCategory = "repair"
	ProofFailureCategoryWorker      ProofFailureCategory = "worker"
	ProofFailureCategoryUnsupported ProofFailureCategory = "unsupported"
)

type ScopeProofRun struct {
	ID              string                `json:"id"`
	Scope           Scope                 `json:"scope"`
	Status          ScopeProofStatus      `json:"status"`
	Verdict         ScopeProofVerdict     `json:"verdict"`
	Checks          []ScopeProofCheck     `json:"checks,omitempty"`
	FixtureMode     ScopeProofFixtureMode `json:"fixture_mode"`
	Actor           string                `json:"actor"`
	Reason          string                `json:"reason"`
	RerunOf         string                `json:"rerun_of,omitempty"`
	LinkedSessionID string                `json:"linked_session_id,omitempty"`
	FailureCategory ProofFailureCategory  `json:"failure_category,omitempty"`
	Summary         map[string]any        `json:"summary,omitempty"`
	CreatedAt       time.Time             `json:"created_at"`
	UpdatedAt       time.Time             `json:"updated_at"`
	StartedAt       time.Time             `json:"started_at,omitempty"`
	FinishedAt      time.Time             `json:"finished_at,omitempty"`
	Steps           []ScopeProofStep      `json:"steps,omitempty"`
}

type ScopeProofStep struct {
	ID              string               `json:"id"`
	ProofID         string               `json:"proof_id"`
	Scope           Scope                `json:"scope"`
	Step            ScopeProofStepName   `json:"step"`
	Status          ScopeProofStepStatus `json:"status"`
	Verdict         ScopeProofVerdict    `json:"verdict,omitempty"`
	FailureCategory ProofFailureCategory `json:"failure_category,omitempty"`
	Evidence        map[string]any       `json:"evidence,omitempty"`
	Attempt         int                  `json:"attempt"`
	WorkerID        string               `json:"worker_id,omitempty"`
	LeaseUntil      time.Time            `json:"lease_until,omitempty"`
	LastError       string               `json:"last_error,omitempty"`
	NextAttemptAt   time.Time            `json:"next_attempt_at,omitempty"`
	CreatedAt       time.Time            `json:"created_at"`
	UpdatedAt       time.Time            `json:"updated_at"`
	CompletedAt     time.Time            `json:"completed_at,omitempty"`
}

type MemorySessionStatus string

const (
	MemorySessionStatusActive       MemorySessionStatus = "active"
	MemorySessionStatusVerifying    MemorySessionStatus = "verifying"
	MemorySessionStatusCompleted    MemorySessionStatus = "completed"
	MemorySessionStatusFailed       MemorySessionStatus = "failed"
	MemorySessionStatusManualReview MemorySessionStatus = "manual_review"
)

func (s MemorySessionStatus) Valid() bool {
	switch s {
	case MemorySessionStatusActive, MemorySessionStatusVerifying, MemorySessionStatusCompleted, MemorySessionStatusFailed, MemorySessionStatusManualReview:
		return true
	default:
		return false
	}
}

type MemorySessionTurnStatus string

const (
	MemorySessionTurnStatusPending          MemorySessionTurnStatus = "pending"
	MemorySessionTurnStatusContextAssembled MemorySessionTurnStatus = "context_assembled"
	MemorySessionTurnStatusOutcomeRecorded  MemorySessionTurnStatus = "outcome_recorded"
	MemorySessionTurnStatusVerifying        MemorySessionTurnStatus = "verifying"
	MemorySessionTurnStatusVerified         MemorySessionTurnStatus = "verified"
	MemorySessionTurnStatusFailed           MemorySessionTurnStatus = "failed"
)

func (s MemorySessionTurnStatus) Valid() bool {
	switch s {
	case MemorySessionTurnStatusPending, MemorySessionTurnStatusContextAssembled, MemorySessionTurnStatusOutcomeRecorded, MemorySessionTurnStatusVerifying, MemorySessionTurnStatusVerified, MemorySessionTurnStatusFailed:
		return true
	default:
		return false
	}
}

type MemorySessionRun struct {
	ID              string                      `json:"id"`
	Scope           Scope                       `json:"scope"`
	Status          MemorySessionStatus         `json:"status"`
	Verdict         ScopeProofVerdict           `json:"verdict"`
	Actor           string                      `json:"actor,omitempty"`
	Reason          string                      `json:"reason,omitempty"`
	Metadata        map[string]any              `json:"metadata,omitempty"`
	FailureCategory ProofFailureCategory        `json:"failure_category,omitempty"`
	CreatedAt       time.Time                   `json:"created_at"`
	UpdatedAt       time.Time                   `json:"updated_at"`
	StartedAt       time.Time                   `json:"started_at,omitempty"`
	FinishedAt      time.Time                   `json:"finished_at,omitempty"`
	Turns           []MemorySessionTurn         `json:"turns,omitempty"`
	Verifications   []MemorySessionVerification `json:"verifications,omitempty"`
}

type MemorySessionTurn struct {
	ID                    string                  `json:"id"`
	SessionID             string                  `json:"session_id"`
	Scope                 Scope                   `json:"scope"`
	IdempotencyKey        string                  `json:"idempotency_key,omitempty"`
	OutcomeIdempotencyKey string                  `json:"outcome_idempotency_key,omitempty"`
	Status                MemorySessionTurnStatus `json:"status"`
	Query                 string                  `json:"query"`
	ContextEvidence       map[string]any          `json:"context_evidence,omitempty"`
	OutcomeEventIDs       []string                `json:"outcome_event_ids,omitempty"`
	ExpectedRecall        []string                `json:"expected_recall,omitempty"`
	VerificationStatus    ScopeProofVerdict       `json:"verification_status,omitempty"`
	FailureCategory       ProofFailureCategory    `json:"failure_category,omitempty"`
	CreatedAt             time.Time               `json:"created_at"`
	UpdatedAt             time.Time               `json:"updated_at"`
	VerifiedAt            time.Time               `json:"verified_at,omitempty"`
}

type MemorySessionVerification struct {
	ID              string               `json:"id"`
	SessionID       string               `json:"session_id"`
	TurnID          string               `json:"turn_id,omitempty"`
	Scope           Scope                `json:"scope"`
	Status          ScopeProofStepStatus `json:"status"`
	Verdict         ScopeProofVerdict    `json:"verdict"`
	ExpectedRecall  []string             `json:"expected_recall,omitempty"`
	Evidence        map[string]any       `json:"evidence,omitempty"`
	FailureCategory ProofFailureCategory `json:"failure_category,omitempty"`
	Attempt         int                  `json:"attempt"`
	WorkerID        string               `json:"worker_id,omitempty"`
	LeaseUntil      time.Time            `json:"lease_until,omitempty"`
	LastError       string               `json:"last_error,omitempty"`
	NextAttemptAt   time.Time            `json:"next_attempt_at,omitempty"`
	CreatedAt       time.Time            `json:"created_at"`
	UpdatedAt       time.Time            `json:"updated_at"`
	CompletedAt     time.Time            `json:"completed_at,omitempty"`
}

type MemoryLoopEvidenceOwnerKind string

const (
	MemoryLoopEvidenceOwnerProof   MemoryLoopEvidenceOwnerKind = "proof"
	MemoryLoopEvidenceOwnerSession MemoryLoopEvidenceOwnerKind = "session"
	MemoryLoopEvidenceOwnerTurn    MemoryLoopEvidenceOwnerKind = "turn"
)

func (k MemoryLoopEvidenceOwnerKind) Valid() bool {
	switch k {
	case MemoryLoopEvidenceOwnerProof, MemoryLoopEvidenceOwnerSession, MemoryLoopEvidenceOwnerTurn:
		return true
	default:
		return false
	}
}

type MemoryLoopEvidenceKind string

const (
	MemoryLoopEvidenceKindEvent        MemoryLoopEvidenceKind = "event"
	MemoryLoopEvidenceKindMemory       MemoryLoopEvidenceKind = "memory"
	MemoryLoopEvidenceKindEvaluation   MemoryLoopEvidenceKind = "evaluation"
	MemoryLoopEvidenceKindRepairPlan   MemoryLoopEvidenceKind = "repair_plan"
	MemoryLoopEvidenceKindReplayRun    MemoryLoopEvidenceKind = "replay_run"
	MemoryLoopEvidenceKindContext      MemoryLoopEvidenceKind = "context"
	MemoryLoopEvidenceKindVerification MemoryLoopEvidenceKind = "verification"
)

func (k MemoryLoopEvidenceKind) Valid() bool {
	switch k {
	case MemoryLoopEvidenceKindEvent, MemoryLoopEvidenceKindMemory, MemoryLoopEvidenceKindEvaluation, MemoryLoopEvidenceKindRepairPlan, MemoryLoopEvidenceKindReplayRun, MemoryLoopEvidenceKindContext, MemoryLoopEvidenceKindVerification:
		return true
	default:
		return false
	}
}

type MemoryLoopEvidenceLink struct {
	ID           string                      `json:"id"`
	Scope        Scope                       `json:"scope"`
	OwnerKind    MemoryLoopEvidenceOwnerKind `json:"owner_kind"`
	OwnerID      string                      `json:"owner_id"`
	EvidenceKind MemoryLoopEvidenceKind      `json:"evidence_kind"`
	EvidenceID   string                      `json:"evidence_id"`
	Metadata     map[string]any              `json:"metadata,omitempty"`
	CreatedAt    time.Time                   `json:"created_at"`
}

type CreateScopeProofRunInput struct {
	Scope       Scope
	Checks      []ScopeProofCheck
	FixtureMode ScopeProofFixtureMode
	Actor       string
	Reason      string
}

func (i CreateScopeProofRunInput) Validate() error {
	if err := i.Scope.Validate(); err != nil {
		return err
	}
	if len(i.Checks) == 0 {
		return fmt.Errorf("at least one proof check is required")
	}
	for _, check := range i.Checks {
		if !check.Valid() {
			return fmt.Errorf("scope proof check %q is invalid", check)
		}
	}
	if i.FixtureMode == "" {
		i.FixtureMode = ScopeProofFixtureModeSmoke
	}
	if !i.FixtureMode.Valid() {
		return fmt.Errorf("scope proof fixture mode %q is invalid", i.FixtureMode)
	}
	if strings.TrimSpace(i.Actor) == "" {
		return fmt.Errorf("actor is required")
	}
	if strings.TrimSpace(i.Reason) == "" {
		return fmt.Errorf("reason is required")
	}
	return nil
}

type ReadScopeProofRunInput struct {
	Scope   Scope
	ProofID string
}

func (i ReadScopeProofRunInput) Validate() error {
	if err := i.Scope.Validate(); err != nil {
		return err
	}
	if strings.TrimSpace(i.ProofID) == "" {
		return fmt.Errorf("proof id is required")
	}
	return nil
}

type ListScopeProofRunsInput struct {
	Scope Scope
	Limit int
}

func (i ListScopeProofRunsInput) Validate() error {
	if err := i.Scope.Validate(); err != nil {
		return err
	}
	if i.Limit < 0 {
		return fmt.Errorf("limit must be greater than or equal to zero")
	}
	return nil
}

type UpdateScopeProofRunStatusInput struct {
	Scope           Scope
	ProofID         string
	Status          ScopeProofStatus
	Verdict         ScopeProofVerdict
	FailureCategory ProofFailureCategory
	Summary         map[string]any
	UpdatedAt       time.Time
	StartedAt       time.Time
	FinishedAt      time.Time
}

func (i UpdateScopeProofRunStatusInput) Validate() error {
	if err := i.Scope.Validate(); err != nil {
		return err
	}
	if strings.TrimSpace(i.ProofID) == "" {
		return fmt.Errorf("proof id is required")
	}
	if !i.Status.Valid() {
		return fmt.Errorf("scope proof status %q is invalid", i.Status)
	}
	if !i.Verdict.Valid() {
		return fmt.Errorf("scope proof verdict %q is invalid", i.Verdict)
	}
	if i.UpdatedAt.IsZero() {
		return fmt.Errorf("updated at is required")
	}
	return nil
}

type ClaimScopeProofStepsInput struct {
	Scope         Scope
	WorkerID      string
	Now           time.Time
	LeaseDuration time.Duration
	Limit         int
}

func (i ClaimScopeProofStepsInput) Validate() error {
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

type ScopeProofStepResult struct {
	Status          ScopeProofStepStatus
	Verdict         ScopeProofVerdict
	Evidence        map[string]any
	FailureCategory ProofFailureCategory
}

type CompleteScopeProofStepInput struct {
	Scope           Scope
	StepID          string
	ProofID         string
	WorkerID        string
	Status          ScopeProofStepStatus
	Verdict         ScopeProofVerdict
	Evidence        map[string]any
	FailureCategory ProofFailureCategory
	CompletedAt     time.Time
}

func (i CompleteScopeProofStepInput) Validate() error {
	if err := i.Scope.Validate(); err != nil {
		return err
	}
	if strings.TrimSpace(i.StepID) == "" {
		return fmt.Errorf("scope proof step id is required")
	}
	if strings.TrimSpace(i.ProofID) == "" {
		return fmt.Errorf("proof id is required")
	}
	if strings.TrimSpace(i.WorkerID) == "" {
		return fmt.Errorf("worker id is required")
	}
	if !i.Status.Valid() {
		return fmt.Errorf("scope proof step status %q is invalid", i.Status)
	}
	if !i.Verdict.Valid() {
		return fmt.Errorf("scope proof verdict %q is invalid", i.Verdict)
	}
	if i.CompletedAt.IsZero() {
		return fmt.Errorf("completed at is required")
	}
	return nil
}

type RecordScopeProofStepFailureInput struct {
	Scope           Scope
	StepID          string
	ProofID         string
	WorkerID        string
	Status          ScopeProofStepStatus
	Verdict         ScopeProofVerdict
	FailureCategory ProofFailureCategory
	ErrorMessage    string
	FailedAt        time.Time
	NextAttemptAt   time.Time
}

func (i RecordScopeProofStepFailureInput) Validate() error {
	if err := i.Scope.Validate(); err != nil {
		return err
	}
	if strings.TrimSpace(i.StepID) == "" {
		return fmt.Errorf("scope proof step id is required")
	}
	if strings.TrimSpace(i.ProofID) == "" {
		return fmt.Errorf("proof id is required")
	}
	if strings.TrimSpace(i.WorkerID) == "" {
		return fmt.Errorf("worker id is required")
	}
	if !i.Status.Valid() {
		return fmt.Errorf("scope proof step status %q is invalid", i.Status)
	}
	if i.Verdict != "" && !i.Verdict.Valid() {
		return fmt.Errorf("scope proof verdict %q is invalid", i.Verdict)
	}
	if strings.TrimSpace(i.ErrorMessage) == "" {
		return fmt.Errorf("error message is required")
	}
	if i.FailedAt.IsZero() {
		return fmt.Errorf("failed at is required")
	}
	return nil
}

type ReadMemorySessionRunInput struct {
	Scope     Scope
	SessionID string
}

func (i ReadMemorySessionRunInput) Validate() error {
	if err := i.Scope.Validate(); err != nil {
		return err
	}
	if strings.TrimSpace(i.SessionID) == "" {
		return fmt.Errorf("session id is required")
	}
	return nil
}

type ListMemorySessionRunsInput struct {
	Scope Scope
	Limit int
}

func (i ListMemorySessionRunsInput) Validate() error {
	if err := i.Scope.Validate(); err != nil {
		return err
	}
	if i.Limit < 0 {
		return fmt.Errorf("limit must be greater than or equal to zero")
	}
	return nil
}

type UpdateMemorySessionRunStatusInput struct {
	Scope           Scope
	SessionID       string
	Status          MemorySessionStatus
	Verdict         ScopeProofVerdict
	FailureCategory ProofFailureCategory
	UpdatedAt       time.Time
	StartedAt       time.Time
	FinishedAt      time.Time
}

func (i UpdateMemorySessionRunStatusInput) Validate() error {
	if err := i.Scope.Validate(); err != nil {
		return err
	}
	if strings.TrimSpace(i.SessionID) == "" {
		return fmt.Errorf("session id is required")
	}
	if !i.Status.Valid() {
		return fmt.Errorf("memory session status %q is invalid", i.Status)
	}
	if !i.Verdict.Valid() {
		return fmt.Errorf("memory session verdict %q is invalid", i.Verdict)
	}
	if i.UpdatedAt.IsZero() {
		return fmt.Errorf("updated at is required")
	}
	return nil
}

type CreateMemorySessionInput struct {
	Scope    Scope
	Actor    string
	Reason   string
	Metadata map[string]any
}

func (i CreateMemorySessionInput) Validate() error {
	return i.Scope.Validate()
}

type CreateMemorySessionTurnInput struct {
	Scope                     Scope
	SessionID                 string
	TurnID                    string
	IdempotencyKey            string
	Query                     string
	ContextBudget             int
	IncludeRelations          bool
	IncludeExperienceInsights bool
	IncludeDiagnostics        bool
}

func (i CreateMemorySessionTurnInput) Validate() error {
	if err := i.Scope.Validate(); err != nil {
		return err
	}
	if strings.TrimSpace(i.SessionID) == "" {
		return fmt.Errorf("session id is required")
	}
	if strings.TrimSpace(i.Query) == "" {
		return fmt.Errorf("query is required")
	}
	if i.ContextBudget < 0 {
		return fmt.Errorf("context budget must be greater than or equal to zero")
	}
	return nil
}

type RecordMemorySessionTurnOutcomeInput struct {
	Scope                Scope
	SessionID            string
	TurnID               string
	IdempotencyKey       string
	OutcomeEventIDs      []string
	OutcomeEventPayloads []MemorySessionOutcomeEventPayload
	ExpectedRecall       []string
}

func (i RecordMemorySessionTurnOutcomeInput) Validate() error {
	if err := i.Scope.Validate(); err != nil {
		return err
	}
	if strings.TrimSpace(i.SessionID) == "" {
		return fmt.Errorf("session id is required")
	}
	if strings.TrimSpace(i.TurnID) == "" {
		return fmt.Errorf("turn id is required")
	}
	if len(i.OutcomeEventPayloads) > 20 {
		return fmt.Errorf("outcome event payload count must be less than or equal to 20")
	}
	for _, payload := range i.OutcomeEventPayloads {
		if err := payload.Validate(); err != nil {
			return err
		}
	}
	return nil
}

type MemorySessionOutcomeEventPayload struct {
	EventType       string         `json:"event_type"`
	Content         string         `json:"content"`
	Metadata        map[string]any `json:"metadata,omitempty"`
	SourceTimestamp time.Time      `json:"source_timestamp,omitempty"`
}

func (p MemorySessionOutcomeEventPayload) Validate() error {
	return IngestEventInput{
		Scope:           Scope{Tenant: "placeholder", Project: "placeholder", Namespace: "placeholder"},
		EventType:       p.EventType,
		Content:         p.Content,
		Metadata:        p.Metadata,
		SourceTimestamp: p.SourceTimestamp,
	}.Validate()
}

type UpdateMemorySessionTurnOutcomeInput struct {
	Scope                 Scope
	SessionID             string
	TurnID                string
	OutcomeIdempotencyKey string
	Status                MemorySessionTurnStatus
	OutcomeEventIDs       []string
	ExpectedRecall        []string
	VerificationStatus    ScopeProofVerdict
	FailureCategory       ProofFailureCategory
	UpdatedAt             time.Time
	VerifiedAt            time.Time
}

func (i UpdateMemorySessionTurnOutcomeInput) Validate() error {
	if err := i.Scope.Validate(); err != nil {
		return err
	}
	if strings.TrimSpace(i.SessionID) == "" {
		return fmt.Errorf("session id is required")
	}
	if strings.TrimSpace(i.TurnID) == "" {
		return fmt.Errorf("turn id is required")
	}
	if !i.Status.Valid() {
		return fmt.Errorf("memory session turn status %q is invalid", i.Status)
	}
	if i.VerificationStatus != "" && !i.VerificationStatus.Valid() {
		return fmt.Errorf("memory session turn verification status %q is invalid", i.VerificationStatus)
	}
	if i.UpdatedAt.IsZero() {
		return fmt.Errorf("updated at is required")
	}
	return nil
}

type RequestMemorySessionVerificationInput struct {
	Scope          Scope
	SessionID      string
	TurnID         string
	ExpectedRecall []string
}

func (i RequestMemorySessionVerificationInput) Validate() error {
	if err := i.Scope.Validate(); err != nil {
		return err
	}
	if strings.TrimSpace(i.SessionID) == "" {
		return fmt.Errorf("session id is required")
	}
	return nil
}

type ClaimMemorySessionVerificationsInput struct {
	Scope         Scope
	WorkerID      string
	Now           time.Time
	LeaseDuration time.Duration
	Limit         int
}

func (i ClaimMemorySessionVerificationsInput) Validate() error {
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

type MemorySessionVerificationResult struct {
	Status          ScopeProofStepStatus
	Verdict         ScopeProofVerdict
	Evidence        map[string]any
	FailureCategory ProofFailureCategory
}

type CompleteMemorySessionVerificationInput struct {
	Scope           Scope
	VerificationID  string
	SessionID       string
	TurnID          string
	WorkerID        string
	Status          ScopeProofStepStatus
	Verdict         ScopeProofVerdict
	Evidence        map[string]any
	FailureCategory ProofFailureCategory
	CompletedAt     time.Time
}

func (i CompleteMemorySessionVerificationInput) Validate() error {
	if err := i.Scope.Validate(); err != nil {
		return err
	}
	if strings.TrimSpace(i.VerificationID) == "" {
		return fmt.Errorf("verification id is required")
	}
	if strings.TrimSpace(i.SessionID) == "" {
		return fmt.Errorf("session id is required")
	}
	if strings.TrimSpace(i.WorkerID) == "" {
		return fmt.Errorf("worker id is required")
	}
	if !i.Status.Valid() {
		return fmt.Errorf("memory session verification status %q is invalid", i.Status)
	}
	if !i.Verdict.Valid() {
		return fmt.Errorf("memory session verification verdict %q is invalid", i.Verdict)
	}
	if i.CompletedAt.IsZero() {
		return fmt.Errorf("completed at is required")
	}
	return nil
}

type RecordMemorySessionVerificationFailureInput struct {
	Scope           Scope
	VerificationID  string
	SessionID       string
	TurnID          string
	WorkerID        string
	Status          ScopeProofStepStatus
	Verdict         ScopeProofVerdict
	FailureCategory ProofFailureCategory
	ErrorMessage    string
	FailedAt        time.Time
	NextAttemptAt   time.Time
}

func (i RecordMemorySessionVerificationFailureInput) Validate() error {
	if err := i.Scope.Validate(); err != nil {
		return err
	}
	if strings.TrimSpace(i.VerificationID) == "" {
		return fmt.Errorf("verification id is required")
	}
	if strings.TrimSpace(i.SessionID) == "" {
		return fmt.Errorf("session id is required")
	}
	if strings.TrimSpace(i.WorkerID) == "" {
		return fmt.Errorf("worker id is required")
	}
	if !i.Status.Valid() {
		return fmt.Errorf("memory session verification status %q is invalid", i.Status)
	}
	if i.Verdict != "" && !i.Verdict.Valid() {
		return fmt.Errorf("memory session verification verdict %q is invalid", i.Verdict)
	}
	if strings.TrimSpace(i.ErrorMessage) == "" {
		return fmt.Errorf("error message is required")
	}
	if i.FailedAt.IsZero() {
		return fmt.Errorf("failed at is required")
	}
	return nil
}

type CreateMemoryLoopEvidenceLinkInput struct {
	Scope        Scope
	OwnerKind    MemoryLoopEvidenceOwnerKind
	OwnerID      string
	EvidenceKind MemoryLoopEvidenceKind
	EvidenceID   string
	Metadata     map[string]any
}

func (i CreateMemoryLoopEvidenceLinkInput) Validate() error {
	if err := i.Scope.Validate(); err != nil {
		return err
	}
	if !i.OwnerKind.Valid() {
		return fmt.Errorf("evidence owner kind %q is invalid", i.OwnerKind)
	}
	if strings.TrimSpace(i.OwnerID) == "" {
		return fmt.Errorf("owner id is required")
	}
	if !i.EvidenceKind.Valid() {
		return fmt.Errorf("evidence kind %q is invalid", i.EvidenceKind)
	}
	if strings.TrimSpace(i.EvidenceID) == "" {
		return fmt.Errorf("evidence id is required")
	}
	return nil
}
