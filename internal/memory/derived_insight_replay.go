package memory

import (
	"fmt"
	"strings"
	"time"
)

type DerivedInsightReplayMode string

const (
	DerivedInsightReplayModeDryRun DerivedInsightReplayMode = "dry_run"
	DerivedInsightReplayModeApply  DerivedInsightReplayMode = "apply"
)

func (m DerivedInsightReplayMode) Valid() bool {
	switch m {
	case DerivedInsightReplayModeDryRun, DerivedInsightReplayModeApply:
		return true
	default:
		return false
	}
}

type DerivedInsightReplayStatus string

const (
	DerivedInsightReplayStatusPending              DerivedInsightReplayStatus = "pending"
	DerivedInsightReplayStatusRunning              DerivedInsightReplayStatus = "running"
	DerivedInsightReplayStatusCompleted            DerivedInsightReplayStatus = "completed"
	DerivedInsightReplayStatusFailed               DerivedInsightReplayStatus = "failed"
	DerivedInsightReplayStatusContinuationRequired DerivedInsightReplayStatus = "continuation_required"
)

func (s DerivedInsightReplayStatus) Valid() bool {
	switch s {
	case DerivedInsightReplayStatusPending,
		DerivedInsightReplayStatusRunning,
		DerivedInsightReplayStatusCompleted,
		DerivedInsightReplayStatusFailed,
		DerivedInsightReplayStatusContinuationRequired:
		return true
	default:
		return false
	}
}

type DerivedInsightReplayDecisionKind string

const (
	DerivedInsightReplayDecisionCreate   DerivedInsightReplayDecisionKind = "create"
	DerivedInsightReplayDecisionUpdate   DerivedInsightReplayDecisionKind = "update"
	DerivedInsightReplayDecisionSuppress DerivedInsightReplayDecisionKind = "suppress"
	DerivedInsightReplayDecisionPreserve DerivedInsightReplayDecisionKind = "preserve"
	DerivedInsightReplayDecisionSkip     DerivedInsightReplayDecisionKind = "skip"
)

func (d DerivedInsightReplayDecisionKind) Valid() bool {
	switch d {
	case DerivedInsightReplayDecisionCreate,
		DerivedInsightReplayDecisionUpdate,
		DerivedInsightReplayDecisionSuppress,
		DerivedInsightReplayDecisionPreserve,
		DerivedInsightReplayDecisionSkip:
		return true
	default:
		return false
	}
}

type DerivedInsightReplayReason string

const (
	DerivedInsightReplayReasonRepeatedEvidence     DerivedInsightReplayReason = "repeated_evidence"
	DerivedInsightReplayReasonInsufficientEvidence DerivedInsightReplayReason = "insufficient_evidence"
	DerivedInsightReplayReasonUnsupportedType      DerivedInsightReplayReason = "unsupported_type"
	DerivedInsightReplayReasonFeedbackPolicy       DerivedInsightReplayReason = "feedback_policy"
	DerivedInsightReplayReasonLifecycleHidden      DerivedInsightReplayReason = "lifecycle_hidden"
	DerivedInsightReplayReasonOutOfScope           DerivedInsightReplayReason = "out_of_scope"
	DerivedInsightReplayReasonIdempotentDuplicate  DerivedInsightReplayReason = "idempotent_duplicate"
	DerivedInsightReplayReasonExecutionFailed      DerivedInsightReplayReason = "execution_failed"
)

func (r DerivedInsightReplayReason) Valid() bool {
	switch r {
	case DerivedInsightReplayReasonRepeatedEvidence,
		DerivedInsightReplayReasonInsufficientEvidence,
		DerivedInsightReplayReasonUnsupportedType,
		DerivedInsightReplayReasonFeedbackPolicy,
		DerivedInsightReplayReasonLifecycleHidden,
		DerivedInsightReplayReasonOutOfScope,
		DerivedInsightReplayReasonIdempotentDuplicate,
		DerivedInsightReplayReasonExecutionFailed:
		return true
	default:
		return false
	}
}

type DerivedInsightReplayRequest struct {
	Scope               Scope                    `json:"scope"`
	Mode                DerivedInsightReplayMode `json:"mode"`
	InsightTypes        []DerivedInsightType     `json:"insight_types,omitempty"`
	EvidenceWindowStart time.Time                `json:"evidence_window_start"`
	EvidenceWindowEnd   time.Time                `json:"evidence_window_end"`
	EvidenceLimit       int                      `json:"evidence_limit"`
	Actor               string                   `json:"actor"`
	Reason              string                   `json:"reason"`
	IdempotencyKey      string                   `json:"idempotency_key,omitempty"`
	RequestedAt         time.Time                `json:"requested_at"`
	Metadata            map[string]any           `json:"metadata,omitempty"`
}

func (r DerivedInsightReplayRequest) Validate() error {
	if err := r.Scope.Validate(); err != nil {
		return err
	}
	switch {
	case !r.Mode.Valid():
		return fmt.Errorf("derived insight replay mode %q is invalid", r.Mode)
	case r.EvidenceWindowStart.IsZero():
		return fmt.Errorf("evidence window start is required")
	case r.EvidenceWindowEnd.IsZero():
		return fmt.Errorf("evidence window end is required")
	case r.EvidenceWindowStart.After(r.EvidenceWindowEnd):
		return fmt.Errorf("evidence window start must be before or equal to evidence window end")
	case r.EvidenceLimit <= 0:
		return fmt.Errorf("evidence limit must be greater than zero")
	case strings.TrimSpace(r.Actor) == "":
		return fmt.Errorf("actor is required")
	case strings.TrimSpace(r.Reason) == "":
		return fmt.Errorf("reason is required")
	case r.RequestedAt.IsZero():
		return fmt.Errorf("requested at is required")
	}
	for _, insightType := range r.InsightTypes {
		if !insightType.Valid() {
			return fmt.Errorf("derived insight type %q is invalid", insightType)
		}
		if !insightType.ActiveSupported() {
			return fmt.Errorf("derived insight type %q is not supported for replay", insightType)
		}
	}
	return nil
}

type DerivedInsightReplayCounters struct {
	EvidenceEvaluated int `json:"evidence_evaluated"`
	Created           int `json:"created"`
	Updated           int `json:"updated"`
	Suppressed        int `json:"suppressed"`
	Preserved         int `json:"preserved"`
	Skipped           int `json:"skipped"`
	Failed            int `json:"failed"`
}

func (c DerivedInsightReplayCounters) Validate() error {
	if c.EvidenceEvaluated < 0 || c.Created < 0 || c.Updated < 0 || c.Suppressed < 0 || c.Preserved < 0 || c.Skipped < 0 || c.Failed < 0 {
		return fmt.Errorf("replay counters must be greater than or equal to zero")
	}
	return nil
}

type DerivedInsightReplayDecision struct {
	InsightID     string                           `json:"insight_id,omitempty"`
	InsightType   DerivedInsightType               `json:"insight_type"`
	Fingerprint   string                           `json:"fingerprint"`
	Decision      DerivedInsightReplayDecisionKind `json:"decision"`
	Reason        DerivedInsightReplayReason       `json:"reason"`
	EvidenceCount int                              `json:"evidence_count,omitempty"`
	Message       string                           `json:"message,omitempty"`
}

func (d DerivedInsightReplayDecision) Validate() error {
	switch {
	case !d.InsightType.Valid():
		return fmt.Errorf("derived insight type %q is invalid", d.InsightType)
	case strings.TrimSpace(d.Fingerprint) == "":
		return fmt.Errorf("replay decision fingerprint is required")
	case !d.Decision.Valid():
		return fmt.Errorf("replay decision %q is invalid", d.Decision)
	case !d.Reason.Valid():
		return fmt.Errorf("replay reason %q is invalid", d.Reason)
	case d.EvidenceCount < 0:
		return fmt.Errorf("evidence count must be greater than or equal to zero")
	default:
		return nil
	}
}

type DerivedInsightReplayReport struct {
	RunID       string                         `json:"run_id"`
	Scope       Scope                          `json:"scope"`
	Counters    DerivedInsightReplayCounters   `json:"counters"`
	Decisions   []DerivedInsightReplayDecision `json:"decisions,omitempty"`
	Failure     string                         `json:"failure,omitempty"`
	GeneratedAt time.Time                      `json:"generated_at"`
}

func (r DerivedInsightReplayReport) Validate() error {
	if strings.TrimSpace(r.RunID) == "" {
		return fmt.Errorf("replay run id is required")
	}
	if err := r.Scope.Validate(); err != nil {
		return err
	}
	if err := r.Counters.Validate(); err != nil {
		return err
	}
	for _, decision := range r.Decisions {
		if err := decision.Validate(); err != nil {
			return err
		}
	}
	if r.GeneratedAt.IsZero() {
		return fmt.Errorf("generated at is required")
	}
	return nil
}

type DerivedInsightReplayRun struct {
	ID         string                      `json:"id"`
	Scope      Scope                       `json:"scope"`
	Mode       DerivedInsightReplayMode    `json:"mode"`
	Status     DerivedInsightReplayStatus  `json:"status"`
	Request    DerivedInsightReplayRequest `json:"request"`
	Report     *DerivedInsightReplayReport `json:"report,omitempty"`
	Actor      string                      `json:"actor"`
	Reason     string                      `json:"reason"`
	Failure    string                      `json:"failure,omitempty"`
	CreatedAt  time.Time                   `json:"created_at"`
	UpdatedAt  time.Time                   `json:"updated_at"`
	StartedAt  time.Time                   `json:"started_at,omitempty"`
	FinishedAt time.Time                   `json:"finished_at,omitempty"`
}

func (r DerivedInsightReplayRun) Validate() error {
	switch {
	case strings.TrimSpace(r.ID) == "":
		return fmt.Errorf("replay run id is required")
	case !r.Mode.Valid():
		return fmt.Errorf("derived insight replay mode %q is invalid", r.Mode)
	case !r.Status.Valid():
		return fmt.Errorf("derived insight replay status %q is invalid", r.Status)
	case strings.TrimSpace(r.Actor) == "":
		return fmt.Errorf("actor is required")
	case strings.TrimSpace(r.Reason) == "":
		return fmt.Errorf("reason is required")
	case r.CreatedAt.IsZero():
		return fmt.Errorf("created at is required")
	case r.UpdatedAt.IsZero():
		return fmt.Errorf("updated at is required")
	}
	if err := r.Scope.Validate(); err != nil {
		return err
	}
	if err := r.Request.Validate(); err != nil {
		return err
	}
	if r.Request.Scope != r.Scope {
		return fmt.Errorf("replay request scope must match run scope")
	}
	if r.Request.Mode != r.Mode {
		return fmt.Errorf("replay request mode must match run mode")
	}
	if r.Report != nil {
		if err := r.Report.Validate(); err != nil {
			return err
		}
		if r.Report.RunID != r.ID {
			return fmt.Errorf("replay report run id must match run id")
		}
	}
	return nil
}

type ReadDerivedInsightReplayRunInput struct {
	Scope Scope
	RunID string
}

func (i ReadDerivedInsightReplayRunInput) Validate() error {
	if err := i.Scope.Validate(); err != nil {
		return err
	}
	if strings.TrimSpace(i.RunID) == "" {
		return fmt.Errorf("replay run id is required")
	}
	return nil
}

type ListDerivedInsightReplayRunsInput struct {
	Scope  Scope
	Status DerivedInsightReplayStatus
	Mode   DerivedInsightReplayMode
	Limit  int
}

func (i ListDerivedInsightReplayRunsInput) Validate() error {
	if err := i.Scope.Validate(); err != nil {
		return err
	}
	if i.Status != "" && !i.Status.Valid() {
		return fmt.Errorf("derived insight replay status %q is invalid", i.Status)
	}
	if i.Mode != "" && !i.Mode.Valid() {
		return fmt.Errorf("derived insight replay mode %q is invalid", i.Mode)
	}
	if i.Limit <= 0 {
		return fmt.Errorf("limit must be greater than zero")
	}
	return nil
}

type FindDerivedInsightReplayRunByIdempotencyKeyInput struct {
	Scope          Scope
	IdempotencyKey string
}

func (i FindDerivedInsightReplayRunByIdempotencyKeyInput) Validate() error {
	if err := i.Scope.Validate(); err != nil {
		return err
	}
	if strings.TrimSpace(i.IdempotencyKey) == "" {
		return fmt.Errorf("idempotency key is required")
	}
	return nil
}

type UpdateDerivedInsightReplayRunStatusInput struct {
	Scope      Scope
	RunID      string
	Status     DerivedInsightReplayStatus
	Failure    string
	UpdatedAt  time.Time
	StartedAt  time.Time
	FinishedAt time.Time
}

func (i UpdateDerivedInsightReplayRunStatusInput) Validate() error {
	if err := i.Scope.Validate(); err != nil {
		return err
	}
	switch {
	case strings.TrimSpace(i.RunID) == "":
		return fmt.Errorf("replay run id is required")
	case !i.Status.Valid():
		return fmt.Errorf("derived insight replay status %q is invalid", i.Status)
	case i.UpdatedAt.IsZero():
		return fmt.Errorf("updated at is required")
	default:
		return nil
	}
}
