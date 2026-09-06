package memory

import (
	"fmt"
	"strings"
)

// CompactionEvidenceState describes the lifecycle of a derived compaction
// record. Only active records are eligible for default projection.
type CompactionEvidenceState string

const (
	CompactionEvidenceStateActive     CompactionEvidenceState = "active"
	CompactionEvidenceStateSuperseded CompactionEvidenceState = "superseded"
	CompactionEvidenceStateStale      CompactionEvidenceState = "stale"
	CompactionEvidenceStateFailed     CompactionEvidenceState = "failed"
)

func (s CompactionEvidenceState) Valid() bool {
	switch s {
	case CompactionEvidenceStateActive, CompactionEvidenceStateSuperseded,
		CompactionEvidenceStateStale, CompactionEvidenceStateFailed:
		return true
	default:
		return false
	}
}

// MaxCompactionRecentTailReferences bounds the amount of source metadata that
// can be carried into a derived compaction record.
const MaxCompactionRecentTailReferences = 32

// CompactionEvidence is the auditable source contract for a compacted summary.
// It is deliberately a derived record: it references canonical/raw sources
// but never contains authority to mutate them.
type CompactionEvidence struct {
	ID                      string                     `json:"id"`
	Scope                   Scope                      `json:"scope"`
	Trigger                 string                     `json:"trigger"`
	SourceWatermark         ContextProjectionWatermark `json:"source_watermark"`
	RawEventReferences      []ContextProjectionSource  `json:"raw_event_references,omitempty"`
	CanonicalVersionRefs    []ContextProjectionSource  `json:"canonical_version_refs,omitempty"`
	DerivationVersion       string                     `json:"derivation_version"`
	SummaryVersion          string                     `json:"summary_version"`
	InputTokenEstimate      int                        `json:"input_token_estimate,omitempty"`
	OutputTokenEstimate     int                        `json:"output_token_estimate,omitempty"`
	EvidenceCoverage        float64                    `json:"evidence_coverage"`
	RecentTailReferences    []ContextProjectionSource  `json:"recent_tail_references,omitempty"`
	State                   CompactionEvidenceState    `json:"state"`
	FollowUpReflectionRunID string                     `json:"follow_up_reflection_run_id,omitempty"`
}

func (e CompactionEvidence) Validate() error {
	if strings.TrimSpace(e.ID) == "" {
		return fmt.Errorf("compaction evidence id is required")
	}
	if err := e.Scope.Validate(); err != nil {
		return fmt.Errorf("compaction evidence scope: %w", err)
	}
	if strings.TrimSpace(e.Trigger) == "" {
		return fmt.Errorf("compaction evidence trigger is required")
	}
	if err := e.SourceWatermark.Validate(); err != nil {
		return fmt.Errorf("compaction evidence source watermark: %w", err)
	}
	if len(e.SourceWatermark.CanonicalVersionIDs) == 0 && len(e.SourceWatermark.RawEventIDs) == 0 && e.SourceWatermark.WindowFrom.IsZero() && e.SourceWatermark.WindowTo.IsZero() {
		return fmt.Errorf("compaction evidence source watermark is required")
	}
	if strings.TrimSpace(e.DerivationVersion) == "" || strings.TrimSpace(e.SummaryVersion) == "" {
		return fmt.Errorf("compaction evidence derivation and summary versions are required")
	}
	if e.InputTokenEstimate < 0 || e.OutputTokenEstimate < 0 {
		return fmt.Errorf("compaction evidence token estimates cannot be negative")
	}
	if e.EvidenceCoverage < 0 || e.EvidenceCoverage > 1 {
		return fmt.Errorf("compaction evidence coverage must be between zero and one")
	}
	if !e.State.Valid() {
		return fmt.Errorf("invalid compaction evidence state %q", e.State)
	}
	if len(e.RecentTailReferences) > MaxCompactionRecentTailReferences {
		return fmt.Errorf("compaction recent-tail references exceed %d", MaxCompactionRecentTailReferences)
	}
	for _, refs := range [][]ContextProjectionSource{e.RawEventReferences, e.CanonicalVersionRefs, e.RecentTailReferences} {
		for _, ref := range refs {
			if err := ref.Validate(); err != nil {
				return fmt.Errorf("compaction evidence source reference: %w", err)
			}
			if ref.Scope.Normalized() != e.Scope.Normalized() {
				return fmt.Errorf("compaction evidence source reference scope does not match evidence scope")
			}
			if ref.LifecycleState != "" && ref.LifecycleState != MemoryStateActive {
				return fmt.Errorf("compaction evidence source reference lifecycle state %q is not eligible", ref.LifecycleState)
			}
		}
	}
	if len(e.RawEventReferences)+len(e.CanonicalVersionRefs)+len(e.RecentTailReferences) == 0 {
		return fmt.Errorf("compaction evidence requires at least one source reference")
	}
	if strings.TrimSpace(e.FollowUpReflectionRunID) != "" && len(e.FollowUpReflectionRunID) > 256 {
		return fmt.Errorf("follow-up reflection run id is too long")
	}
	return nil
}

// ProjectionEligible reports whether this evidence can support a default
// projection. Stale, superseded and failed records fail closed.
func (e CompactionEvidence) ProjectionEligible(scope Scope) bool {
	if e.State != CompactionEvidenceStateActive || e.EvidenceCoverage <= 0 {
		return false
	}
	if e.Scope.Normalized() != scope.Normalized() || e.Validate() != nil {
		return false
	}
	return true
}
