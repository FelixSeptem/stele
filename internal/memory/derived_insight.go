package memory

import (
	"fmt"
	"strings"
	"time"
)

type DerivedInsightType string

const (
	DerivedInsightTypeFailurePattern DerivedInsightType = "failure_pattern"
	DerivedInsightTypeLesson         DerivedInsightType = "lesson"
	DerivedInsightTypeHypothesis     DerivedInsightType = "hypothesis"
	DerivedInsightTypeGoal           DerivedInsightType = "goal"
	DerivedInsightTypeContradiction  DerivedInsightType = "contradiction"
	DerivedInsightTypeCausalLink     DerivedInsightType = "causal_link"
)

func (t DerivedInsightType) Valid() bool {
	switch t {
	case DerivedInsightTypeFailurePattern,
		DerivedInsightTypeLesson,
		DerivedInsightTypeHypothesis,
		DerivedInsightTypeGoal,
		DerivedInsightTypeContradiction,
		DerivedInsightTypeCausalLink:
		return true
	default:
		return false
	}
}

func (t DerivedInsightType) ActiveSupported() bool {
	switch t {
	case DerivedInsightTypeFailurePattern, DerivedInsightTypeLesson:
		return true
	default:
		return false
	}
}

type DerivedInsightState string

const (
	DerivedInsightStateCandidate  DerivedInsightState = "candidate"
	DerivedInsightStateActive     DerivedInsightState = "active"
	DerivedInsightStateSuppressed DerivedInsightState = "suppressed"
	DerivedInsightStateForgotten  DerivedInsightState = "forgotten"
	DerivedInsightStateDeleted    DerivedInsightState = "deleted"
)

func (s DerivedInsightState) Valid() bool {
	switch s {
	case DerivedInsightStateCandidate,
		DerivedInsightStateActive,
		DerivedInsightStateSuppressed,
		DerivedInsightStateForgotten,
		DerivedInsightStateDeleted:
		return true
	default:
		return false
	}
}

type DerivedInsightEvidenceKind string

const (
	DerivedInsightEvidenceKindRawEvent         DerivedInsightEvidenceKind = "raw_event"
	DerivedInsightEvidenceKindCanonicalMemory  DerivedInsightEvidenceKind = "canonical_memory"
	DerivedInsightEvidenceKindProceduralMemory DerivedInsightEvidenceKind = "procedural_memory"
	DerivedInsightEvidenceKindSummaryMemory    DerivedInsightEvidenceKind = "summary_memory"
	DerivedInsightEvidenceKindRelationMemory   DerivedInsightEvidenceKind = "relation_memory"
	DerivedInsightEvidenceKindJobExecution     DerivedInsightEvidenceKind = "job_execution"
	DerivedInsightEvidenceKindEmbeddingRebuild DerivedInsightEvidenceKind = "embedding_rebuild"
	DerivedInsightEvidenceKindRecoveryRecord   DerivedInsightEvidenceKind = "recovery_record"
)

func (k DerivedInsightEvidenceKind) Valid() bool {
	switch k {
	case DerivedInsightEvidenceKindRawEvent,
		DerivedInsightEvidenceKindCanonicalMemory,
		DerivedInsightEvidenceKindProceduralMemory,
		DerivedInsightEvidenceKindSummaryMemory,
		DerivedInsightEvidenceKindRelationMemory,
		DerivedInsightEvidenceKindJobExecution,
		DerivedInsightEvidenceKindEmbeddingRebuild,
		DerivedInsightEvidenceKindRecoveryRecord:
		return true
	default:
		return false
	}
}

type DerivedInsightEvidenceRelation string

const (
	DerivedInsightEvidenceRelationSupports DerivedInsightEvidenceRelation = "supports"
	DerivedInsightEvidenceRelationUpdates  DerivedInsightEvidenceRelation = "updates"
)

func (r DerivedInsightEvidenceRelation) Valid() bool {
	switch r {
	case DerivedInsightEvidenceRelationSupports,
		DerivedInsightEvidenceRelationUpdates:
		return true
	default:
		return false
	}
}

type DerivedInsightConfidence struct {
	Score  float64 `json:"score"`
	Method string  `json:"method,omitempty"`
}

func (c DerivedInsightConfidence) Validate() error {
	if c.Score < 0 || c.Score > 1 {
		return fmt.Errorf("confidence score must be between 0 and 1")
	}
	return nil
}

type DerivedInsightDerivation struct {
	Source              string         `json:"source"`
	Fingerprint         string         `json:"fingerprint"`
	EvidenceWindowStart time.Time      `json:"evidence_window_start,omitempty"`
	EvidenceWindowEnd   time.Time      `json:"evidence_window_end,omitempty"`
	DerivedAt           time.Time      `json:"derived_at"`
	Metadata            map[string]any `json:"metadata,omitempty"`
}

func (d DerivedInsightDerivation) Validate() error {
	switch {
	case strings.TrimSpace(d.Source) == "":
		return fmt.Errorf("derivation source is required")
	case strings.TrimSpace(d.Fingerprint) == "":
		return fmt.Errorf("derivation fingerprint is required")
	case !d.EvidenceWindowStart.IsZero() && !d.EvidenceWindowEnd.IsZero() && d.EvidenceWindowStart.After(d.EvidenceWindowEnd):
		return fmt.Errorf("evidence window start must be before or equal to evidence window end")
	case d.DerivedAt.IsZero():
		return fmt.Errorf("derived at is required")
	default:
		return nil
	}
}

type DerivedInsightEvidenceRef struct {
	Kind       DerivedInsightEvidenceKind     `json:"kind"`
	ID         string                         `json:"id"`
	Relation   DerivedInsightEvidenceRelation `json:"relation"`
	ObservedAt time.Time                      `json:"observed_at,omitempty"`
	Metadata   map[string]any                 `json:"metadata,omitempty"`
}

func (r DerivedInsightEvidenceRef) Validate() error {
	switch {
	case !r.Kind.Valid():
		return fmt.Errorf("evidence kind %q is invalid", r.Kind)
	case strings.TrimSpace(r.ID) == "":
		return fmt.Errorf("evidence id is required")
	case !r.Relation.Valid():
		return fmt.Errorf("evidence relation %q is invalid", r.Relation)
	default:
		return nil
	}
}

type DerivedInsightLesson struct {
	SourceFailurePatternID string   `json:"source_failure_pattern_id"`
	Guidance               string   `json:"guidance"`
	Avoid                  []string `json:"avoid,omitempty"`
	Prefer                 []string `json:"prefer,omitempty"`
}

func (l DerivedInsightLesson) Validate() error {
	switch {
	case strings.TrimSpace(l.SourceFailurePatternID) == "":
		return fmt.Errorf("source failure pattern id is required")
	case strings.TrimSpace(l.Guidance) == "":
		return fmt.Errorf("lesson guidance is required")
	default:
		return nil
	}
}

type DerivedInsight struct {
	ID             string                      `json:"id"`
	Scope          Scope                       `json:"scope"`
	Type           DerivedInsightType          `json:"type"`
	State          DerivedInsightState         `json:"state"`
	Title          string                      `json:"title"`
	Summary        string                      `json:"summary"`
	Confidence     DerivedInsightConfidence    `json:"confidence"`
	Payload        map[string]any              `json:"payload,omitempty"`
	Lesson         *DerivedInsightLesson       `json:"lesson,omitempty"`
	Derivation     DerivedInsightDerivation    `json:"derivation"`
	Evidence       []DerivedInsightEvidenceRef `json:"evidence"`
	FeedbackSummary DerivedInsightFeedbackSummary `json:"feedback_summary,omitempty"`
	CreatedAt      time.Time                   `json:"created_at,omitempty"`
	UpdatedAt      time.Time                   `json:"updated_at,omitempty"`
	LastObservedAt time.Time                   `json:"last_observed_at,omitempty"`
}

func (i DerivedInsight) Validate() error {
	if strings.TrimSpace(i.ID) == "" {
		return fmt.Errorf("derived insight id is required")
	}
	if err := i.Scope.Validate(); err != nil {
		return err
	}
	if !i.Type.Valid() {
		return fmt.Errorf("derived insight type %q is invalid", i.Type)
	}
	if !i.State.Valid() {
		return fmt.Errorf("derived insight state %q is invalid", i.State)
	}
	if i.State == DerivedInsightStateActive && !i.Type.ActiveSupported() {
		return fmt.Errorf("derived insight type %q is reserved and cannot be active in this change", i.Type)
	}
	if strings.TrimSpace(i.Title) == "" {
		return fmt.Errorf("derived insight title is required")
	}
	if strings.TrimSpace(i.Summary) == "" {
		return fmt.Errorf("derived insight summary is required")
	}
	if err := i.Confidence.Validate(); err != nil {
		return err
	}
	if err := i.Derivation.Validate(); err != nil {
		return err
	}
	for _, evidence := range i.Evidence {
		if err := evidence.Validate(); err != nil {
			return err
		}
	}
	if i.State == DerivedInsightStateActive && i.Type == DerivedInsightTypeFailurePattern && len(i.Evidence) < 2 {
		return fmt.Errorf("active failure_pattern requires at least 2 evidence references")
	}
	if i.Type == DerivedInsightTypeLesson {
		if i.Lesson == nil {
			return fmt.Errorf("lesson payload is required")
		}
		if err := i.Lesson.Validate(); err != nil {
			return err
		}
		if len(i.Evidence) == 0 {
			return fmt.Errorf("lesson requires evidence references")
		}
	}

	return nil
}

type ListDerivedInsightsInput struct {
	Scope            Scope
	Type             DerivedInsightType
	State            DerivedInsightState
	MinConfidence    *float64
	MinEvidenceCount int
	IncludeHidden    bool
	Limit            int
}

func (i ListDerivedInsightsInput) Validate() error {
	if err := i.Scope.Validate(); err != nil {
		return err
	}
	if i.Type != "" && !i.Type.Valid() {
		return fmt.Errorf("derived insight type %q is invalid", i.Type)
	}
	if i.State != "" && !i.State.Valid() {
		return fmt.Errorf("derived insight state %q is invalid", i.State)
	}
	if i.MinConfidence != nil && (*i.MinConfidence < 0 || *i.MinConfidence > 1) {
		return fmt.Errorf("min confidence must be between 0 and 1")
	}
	if i.MinEvidenceCount < 0 {
		return fmt.Errorf("min evidence count must be greater than or equal to zero")
	}
	if i.Limit <= 0 {
		return fmt.Errorf("limit must be greater than zero")
	}
	return nil
}

type ReadDerivedInsightInput struct {
	Scope         Scope
	ID            string
	IncludeHidden bool
}

func (i ReadDerivedInsightInput) Validate() error {
	if err := i.Scope.Validate(); err != nil {
		return err
	}
	if strings.TrimSpace(i.ID) == "" {
		return fmt.Errorf("derived insight id is required")
	}
	return nil
}

type DerivedInsightLifecycleTransition struct {
	Scope      Scope
	InsightID  string
	FromState  DerivedInsightState
	ToState    DerivedInsightState
	Actor      string
	Reason     string
	OccurredAt time.Time
	Metadata   map[string]any
}

func (t DerivedInsightLifecycleTransition) Validate() error {
	if err := t.Scope.Validate(); err != nil {
		return err
	}
	switch {
	case strings.TrimSpace(t.InsightID) == "":
		return fmt.Errorf("derived insight id is required")
	case t.FromState != "" && !t.FromState.Valid():
		return fmt.Errorf("from state %q is invalid", t.FromState)
	case !t.ToState.Valid():
		return fmt.Errorf("to state %q is invalid", t.ToState)
	case strings.TrimSpace(t.Actor) == "":
		return fmt.Errorf("actor is required")
	case strings.TrimSpace(t.Reason) == "":
		return fmt.Errorf("reason is required")
	case t.OccurredAt.IsZero():
		return fmt.Errorf("occurred at is required")
	default:
		return nil
	}
}

type DerivedInsightLifecycleRecord struct {
	ID         string              `json:"id"`
	InsightID  string              `json:"insight_id"`
	Scope      Scope               `json:"scope"`
	FromState  DerivedInsightState `json:"from_state,omitempty"`
	ToState    DerivedInsightState `json:"to_state"`
	Actor      string              `json:"actor"`
	Reason     string              `json:"reason"`
	OccurredAt time.Time           `json:"occurred_at"`
	Metadata   map[string]any      `json:"metadata,omitempty"`
}

type DerivedInsightDetail struct {
	Insight         DerivedInsight                  `json:"insight"`
	Evidence        []DerivedInsightEvidenceRef     `json:"evidence"`
	Lifecycle       []DerivedInsightLifecycleRecord `json:"lifecycle"`
	FeedbackSummary DerivedInsightFeedbackSummary   `json:"feedback_summary,omitempty"`
}
