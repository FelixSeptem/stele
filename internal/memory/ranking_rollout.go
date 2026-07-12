package memory

import (
	"fmt"
	"strings"
	"time"
)

type RankingRolloutPolicyStatus string

const (
	RankingRolloutPolicyStatusDraft           RankingRolloutPolicyStatus = "draft"
	RankingRolloutPolicyStatusDiagnosticsOnly RankingRolloutPolicyStatus = "diagnostics_only"
	RankingRolloutPolicyStatusDryRun          RankingRolloutPolicyStatus = "dry_run"
	RankingRolloutPolicyStatusActiveForScope  RankingRolloutPolicyStatus = "active_for_scope"
	RankingRolloutPolicyStatusDisabled        RankingRolloutPolicyStatus = "disabled"
	RankingRolloutPolicyStatusRolledBack      RankingRolloutPolicyStatus = "rolled_back"
)

func (s RankingRolloutPolicyStatus) Valid() bool {
	switch s {
	case RankingRolloutPolicyStatusDraft,
		RankingRolloutPolicyStatusDiagnosticsOnly,
		RankingRolloutPolicyStatusDryRun,
		RankingRolloutPolicyStatusActiveForScope,
		RankingRolloutPolicyStatusDisabled,
		RankingRolloutPolicyStatusRolledBack:
		return true
	default:
		return false
	}
}

type RankingRolloutMode string

const (
	RankingRolloutModeDiagnosticsOnly RankingRolloutMode = "diagnostics_only"
	RankingRolloutModeDryRun          RankingRolloutMode = "dry_run"
	RankingRolloutModeActiveForScope  RankingRolloutMode = "active_for_scope"
)

func (m RankingRolloutMode) Valid() bool {
	switch m {
	case RankingRolloutModeDiagnosticsOnly, RankingRolloutModeDryRun, RankingRolloutModeActiveForScope:
		return true
	default:
		return false
	}
}

type RankingRolloutSurface string

const (
	RankingRolloutSurfaceSearch  RankingRolloutSurface = "search"
	RankingRolloutSurfaceContext  RankingRolloutSurface = "context"
)

func (s RankingRolloutSurface) Valid() bool {
	switch s {
	case RankingRolloutSurfaceSearch, RankingRolloutSurfaceContext:
		return true
	default:
		return false
	}
}

type RankingRolloutSignalSource string

const (
	RankingRolloutSignalSourceUsefulnessFeedback RankingRolloutSignalSource = "usefulness_feedback"
	RankingRolloutSignalSourceTaskEvaluations    RankingRolloutSignalSource = "task_evaluations"
	RankingRolloutSignalSourceSessionVerification RankingRolloutSignalSource = "session_verification"
	RankingRolloutSignalSourceQualityFindings    RankingRolloutSignalSource = "quality_findings"
)

func (s RankingRolloutSignalSource) Valid() bool {
	switch s {
	case RankingRolloutSignalSourceUsefulnessFeedback,
		RankingRolloutSignalSourceTaskEvaluations,
		RankingRolloutSignalSourceSessionVerification,
		RankingRolloutSignalSourceQualityFindings:
		return true
	default:
		return false
	}
}

type RankingRolloutThresholdStatus string

const (
	RankingRolloutThresholdStatusSatisfied   RankingRolloutThresholdStatus = "satisfied"
	RankingRolloutThresholdStatusInsufficient RankingRolloutThresholdStatus = "insufficient"
	RankingRolloutThresholdStatusBlocked     RankingRolloutThresholdStatus = "blocked"
)

func (s RankingRolloutThresholdStatus) Valid() bool {
	switch s {
	case RankingRolloutThresholdStatusSatisfied,
		RankingRolloutThresholdStatusInsufficient,
		RankingRolloutThresholdStatusBlocked:
		return true
	default:
		return false
	}
}

type RankingRolloutImpactReasonCode string

const (
	RankingRolloutImpactReasonCodeBaselineRetained    RankingRolloutImpactReasonCode = "baseline_retained"
	RankingRolloutImpactReasonCodeSubjectBoosted      RankingRolloutImpactReasonCode = "subject_boosted"
	RankingRolloutImpactReasonCodeSubjectPenalized    RankingRolloutImpactReasonCode = "subject_penalized"
	RankingRolloutImpactReasonCodeSubjectHidden       RankingRolloutImpactReasonCode = "subject_hidden"
	RankingRolloutImpactReasonCodeInsufficientEvidence RankingRolloutImpactReasonCode = "insufficient_evidence"
	RankingRolloutImpactReasonCodeBlockerPresent      RankingRolloutImpactReasonCode = "blocker_present"
	RankingRolloutImpactReasonCodeRollbackRestored    RankingRolloutImpactReasonCode = "rollback_restored"
)

func (c RankingRolloutImpactReasonCode) Valid() bool {
	switch c {
	case RankingRolloutImpactReasonCodeBaselineRetained,
		RankingRolloutImpactReasonCodeSubjectBoosted,
		RankingRolloutImpactReasonCodeSubjectPenalized,
		RankingRolloutImpactReasonCodeSubjectHidden,
		RankingRolloutImpactReasonCodeInsufficientEvidence,
		RankingRolloutImpactReasonCodeBlockerPresent,
		RankingRolloutImpactReasonCodeRollbackRestored:
		return true
	default:
		return false
	}
}

type RankingRolloutActivationGate struct {
	DryRunSucceeded          bool                      `json:"dry_run_succeeded"`
	EvidenceThresholdStatus   RankingRolloutThresholdStatus `json:"evidence_threshold_status"`
	BlockersPresent          bool                      `json:"blockers_present"`
	AttributionRecorded      bool                      `json:"attribution_recorded"`
}

func (g RankingRolloutActivationGate) CanActivate() bool {
	return g.DryRunSucceeded && g.AttributionRecorded && g.EvidenceThresholdStatus == RankingRolloutThresholdStatusSatisfied && !g.BlockersPresent
}

type RankingRolloutPolicy struct {
	ID                   string                        `json:"id"`
	Scope                Scope                         `json:"scope"`
	Status               RankingRolloutPolicyStatus     `json:"status"`
	Mode                 RankingRolloutMode            `json:"mode"`
	Surfaces             []RankingRolloutSurface       `json:"surfaces,omitempty"`
	SignalSources        []RankingRolloutSignalSource   `json:"signal_sources,omitempty"`
	ThresholdStatus      RankingRolloutThresholdStatus  `json:"threshold_status"`
	EvidenceMinimum      int                           `json:"evidence_minimum"`
	Actor                string                        `json:"actor"`
	Reason               string                        `json:"reason"`
	LatestDryRunID       string                        `json:"latest_dry_run_id,omitempty"`
	LatestDryRunStatus   RankingRolloutThresholdStatus  `json:"latest_dry_run_status,omitempty"`
	ActivatedAt          time.Time                     `json:"activated_at,omitempty"`
	DisabledAt           time.Time                     `json:"disabled_at,omitempty"`
	RolledBackAt         time.Time                     `json:"rolled_back_at,omitempty"`
	CreatedAt            time.Time                     `json:"created_at"`
	UpdatedAt            time.Time                     `json:"updated_at"`
}

func (p RankingRolloutPolicy) Validate() error {
	if strings.TrimSpace(p.ID) == "" {
		return fmt.Errorf("ranking rollout policy id is required")
	}
	if err := p.Scope.Validate(); err != nil {
		return err
	}
	if !p.Status.Valid() {
		return fmt.Errorf("ranking rollout policy status %q is invalid", p.Status)
	}
	if !p.Mode.Valid() {
		return fmt.Errorf("ranking rollout mode %q is invalid", p.Mode)
	}
	if len(p.Surfaces) == 0 {
		return fmt.Errorf("at least one ranking rollout surface is required")
	}
	for _, surface := range p.Surfaces {
		if !surface.Valid() {
			return fmt.Errorf("ranking rollout surface %q is invalid", surface)
		}
	}
	if len(p.SignalSources) == 0 {
		return fmt.Errorf("at least one ranking rollout signal source is required")
	}
	for _, source := range p.SignalSources {
		if !source.Valid() {
			return fmt.Errorf("ranking rollout signal source %q is invalid", source)
		}
	}
	if !p.ThresholdStatus.Valid() {
		return fmt.Errorf("ranking rollout threshold status %q is invalid", p.ThresholdStatus)
	}
	if p.LatestDryRunStatus != "" && !p.LatestDryRunStatus.Valid() {
		return fmt.Errorf("ranking rollout threshold status %q is invalid", p.LatestDryRunStatus)
	}
	if p.EvidenceMinimum < 0 {
		return fmt.Errorf("evidence minimum must be greater than or equal to zero")
	}
	if strings.TrimSpace(p.Actor) == "" {
		return fmt.Errorf("actor is required")
	}
	if strings.TrimSpace(p.Reason) == "" {
		return fmt.Errorf("reason is required")
	}
	if p.CreatedAt.IsZero() {
		return fmt.Errorf("created at is required")
	}
	if p.UpdatedAt.IsZero() {
		return fmt.Errorf("updated at is required")
	}
	return nil
}

type RankingRolloutDryRun struct {
	ID                  string                       `json:"id"`
	PolicyID            string                       `json:"policy_id"`
	Scope               Scope                        `json:"scope"`
	Surface             RankingRolloutSurface        `json:"surface"`
	SignalSource        RankingRolloutSignalSource   `json:"signal_source"`
	ThresholdStatus     RankingRolloutThresholdStatus `json:"threshold_status"`
	BaselineRank        int                          `json:"baseline_rank"`
	AdjustedRank        int                          `json:"adjusted_rank"`
	ChangedSubjectIDs   []string                     `json:"changed_subject_ids,omitempty"`
	ReasonCodes         []RankingRolloutImpactReasonCode `json:"reason_codes,omitempty"`
	SignalCategories    []string                     `json:"signal_categories,omitempty"`
	EvidenceCount       int                          `json:"evidence_count"`
	HiddenEvidenceCount int                          `json:"hidden_evidence_count"`
	CreatedAt           time.Time                    `json:"created_at"`
}

type RankingRolloutImpactEntry struct {
	ID              string                       `json:"id"`
	DryRunID        string                       `json:"dry_run_id"`
	PolicyID        string                       `json:"policy_id"`
	Scope           Scope                        `json:"scope"`
	Surface         RankingRolloutSurface        `json:"surface"`
	SignalSource    RankingRolloutSignalSource   `json:"signal_source"`
	SignalCategories []string                    `json:"signal_categories,omitempty"`
	SubjectKind     string                       `json:"subject_kind"`
	SubjectID       string                       `json:"subject_id,omitempty"`
	OpaqueToken     string                       `json:"opaque_token,omitempty"`
	CandidatePriority int                        `json:"candidate_priority,omitempty"`
	Included        bool                         `json:"included"`
	BudgetImpact    int                           `json:"budget_impact,omitempty"`
	BaselineRank    int                          `json:"baseline_rank"`
	AdjustedRank    int                          `json:"adjusted_rank"`
	ReasonCode      RankingRolloutImpactReasonCode `json:"reason_code"`
	EvidenceCount   int                          `json:"evidence_count"`
	HiddenEvidence  bool                         `json:"hidden_evidence"`
	CreatedAt       time.Time                    `json:"created_at"`
}

type RankingRolloutPolicyState struct {
	PolicyID     string                    `json:"policy_id"`
	Scope        Scope                     `json:"scope"`
	Status       RankingRolloutPolicyStatus `json:"status"`
	Actor        string                    `json:"actor"`
	Reason       string                    `json:"reason"`
	ActivatedAt  time.Time                 `json:"activated_at,omitempty"`
	DisabledAt   time.Time                 `json:"disabled_at,omitempty"`
	RolledBackAt time.Time                 `json:"rolled_back_at,omitempty"`
	UpdatedAt    time.Time                 `json:"updated_at"`
}

type RankingRolloutRollbackRecord struct {
	ID          string                    `json:"id"`
	PolicyID    string                    `json:"policy_id"`
	Scope       Scope                     `json:"scope"`
	Actor       string                    `json:"actor"`
	Reason      string                    `json:"reason"`
	FromStatus  RankingRolloutPolicyStatus `json:"from_status"`
	ToStatus    RankingRolloutPolicyStatus `json:"to_status"`
	RolledBackAt time.Time                `json:"rolled_back_at"`
}

type CreateRankingRolloutPolicyInput struct {
	Policy RankingRolloutPolicy
}

func (i CreateRankingRolloutPolicyInput) Validate() error {
	return i.Policy.Validate()
}

type ReadRankingRolloutPolicyInput struct {
	Scope    Scope
	PolicyID string
}

func (i ReadRankingRolloutPolicyInput) Validate() error {
	if err := i.Scope.Validate(); err != nil {
		return err
	}
	if strings.TrimSpace(i.PolicyID) == "" {
		return fmt.Errorf("ranking rollout policy id is required")
	}
	return nil
}

type ReadActiveRankingRolloutPolicyInput struct {
	Scope   Scope
	Surface RankingRolloutSurface
}

func (i ReadActiveRankingRolloutPolicyInput) Validate() error {
	if err := i.Scope.Validate(); err != nil {
		return err
	}
	if !i.Surface.Valid() {
		return fmt.Errorf("ranking rollout surface %q is invalid", i.Surface)
	}
	return nil
}

type ListRankingRolloutPoliciesInput struct {
	Scope Scope
}

func (i ListRankingRolloutPoliciesInput) Validate() error {
	return i.Scope.Validate()
}

type RecordRankingRolloutDryRunInput struct {
	PolicyID        string
	Scope           Scope
	Surface         RankingRolloutSurface
	SignalSource    RankingRolloutSignalSource
	ThresholdStatus RankingRolloutThresholdStatus
	BaselineRank     int
	AdjustedRank     int
	ChangedSubjectIDs []string
	ReasonCodes      []RankingRolloutImpactReasonCode
	SignalCategories []string
	EvidenceCount    int
	HiddenEvidenceCount int
	ImpactEntries    []RankingRolloutImpactEntry
	CreatedAt       time.Time
}

func (i RecordRankingRolloutDryRunInput) Validate() error {
	if err := i.Scope.Validate(); err != nil {
		return err
	}
	if strings.TrimSpace(i.PolicyID) == "" {
		return fmt.Errorf("ranking rollout policy id is required")
	}
	if !i.Surface.Valid() {
		return fmt.Errorf("ranking rollout surface %q is invalid", i.Surface)
	}
	if !i.SignalSource.Valid() {
		return fmt.Errorf("ranking rollout signal source %q is invalid", i.SignalSource)
	}
	if !i.ThresholdStatus.Valid() {
		return fmt.Errorf("ranking rollout threshold status %q is invalid", i.ThresholdStatus)
	}
	for _, code := range i.ReasonCodes {
		if !code.Valid() {
			return fmt.Errorf("ranking rollout impact reason code %q is invalid", code)
		}
	}
	if i.CreatedAt.IsZero() {
		return fmt.Errorf("created at is required")
	}
	return nil
}

type ActivateRankingRolloutPolicyInput struct {
	Scope         Scope
	PolicyID      string
	Actor         string
	Reason        string
	ActivatedAt   time.Time
	Gate          RankingRolloutActivationGate
}

func (i ActivateRankingRolloutPolicyInput) Validate() error {
	if err := i.Scope.Validate(); err != nil {
		return err
	}
	if strings.TrimSpace(i.PolicyID) == "" {
		return fmt.Errorf("ranking rollout policy id is required")
	}
	if strings.TrimSpace(i.Actor) == "" {
		return fmt.Errorf("actor is required")
	}
	if strings.TrimSpace(i.Reason) == "" {
		return fmt.Errorf("reason is required")
	}
	if i.ActivatedAt.IsZero() {
		return fmt.Errorf("activated at is required")
	}
	return nil
}

type DisableRankingRolloutPolicyInput struct {
	Scope       Scope
	PolicyID    string
	Actor       string
	Reason      string
	DisabledAt  time.Time
}

func (i DisableRankingRolloutPolicyInput) Validate() error {
	if err := i.Scope.Validate(); err != nil {
		return err
	}
	if strings.TrimSpace(i.PolicyID) == "" {
		return fmt.Errorf("ranking rollout policy id is required")
	}
	if strings.TrimSpace(i.Actor) == "" {
		return fmt.Errorf("actor is required")
	}
	if strings.TrimSpace(i.Reason) == "" {
		return fmt.Errorf("reason is required")
	}
	if i.DisabledAt.IsZero() {
		return fmt.Errorf("disabled at is required")
	}
	return nil
}

type RollbackRankingRolloutPolicyInput struct {
	Scope      Scope
	PolicyID   string
	Actor      string
	Reason     string
	RolledBackAt time.Time
}

func (i RollbackRankingRolloutPolicyInput) Validate() error {
	if err := i.Scope.Validate(); err != nil {
		return err
	}
	if strings.TrimSpace(i.PolicyID) == "" {
		return fmt.Errorf("ranking rollout policy id is required")
	}
	if strings.TrimSpace(i.Actor) == "" {
		return fmt.Errorf("actor is required")
	}
	if strings.TrimSpace(i.Reason) == "" {
		return fmt.Errorf("reason is required")
	}
	if i.RolledBackAt.IsZero() {
		return fmt.Errorf("rolled back at is required")
	}
	return nil
}

type ListRankingRolloutPolicyImpactInput struct {
	Scope    Scope
	PolicyID string
}

func (i ListRankingRolloutPolicyImpactInput) Validate() error {
	if err := i.Scope.Validate(); err != nil {
		return err
	}
	if strings.TrimSpace(i.PolicyID) == "" {
		return fmt.Errorf("ranking rollout policy id is required")
	}
	return nil
}
