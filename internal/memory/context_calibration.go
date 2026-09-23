package memory

import (
	"fmt"
	"strings"
	"time"
)

const ContextCalibrationSummaryVersionV1 = "context-calibration-summary-v1"

type ContextCalibrationSummaryFreshness string

const (
	ContextCalibrationSummaryFresh   ContextCalibrationSummaryFreshness = "fresh"
	ContextCalibrationSummaryStale   ContextCalibrationSummaryFreshness = "stale"
	ContextCalibrationSummaryUnknown ContextCalibrationSummaryFreshness = "unknown"
)

type ContextCalibrationSummary struct {
	ID              string                             `json:"id"`
	Scope           Scope                              `json:"scope"`
	PolicyVersion   string                             `json:"policy_version"`
	SummaryVersion  string                             `json:"summary_version"`
	SourceWatermark time.Time                          `json:"source_watermark"`
	EvidenceCount   int                                `json:"evidence_count"`
	ConfidenceSum   float64                            `json:"confidence_sum"`
	PrioritySum     float64                            `json:"priority_sum"`
	Freshness       ContextCalibrationSummaryFreshness `json:"freshness"`
	CreatedAt       time.Time                          `json:"created_at"`
	UpdatedAt       time.Time                          `json:"updated_at"`
}

func (s ContextCalibrationSummary) Validate() error {
	if strings.TrimSpace(s.ID) == "" || s.Scope.Validate() != nil || strings.TrimSpace(s.PolicyVersion) == "" || strings.TrimSpace(s.SummaryVersion) == "" || s.SourceWatermark.IsZero() || s.CreatedAt.IsZero() || s.UpdatedAt.IsZero() {
		return fmt.Errorf("context calibration summary identity and timestamps are required")
	}
	if s.EvidenceCount < 0 || s.EvidenceCount > 10000 || s.ConfidenceSum < 0 || s.ConfidenceSum > 10000 || s.PrioritySum < -10000 || s.PrioritySum > 10000 {
		return fmt.Errorf("context calibration summary aggregates are out of bounds")
	}
	switch s.Freshness {
	case ContextCalibrationSummaryFresh, ContextCalibrationSummaryStale, ContextCalibrationSummaryUnknown:
	default:
		return fmt.Errorf("context calibration summary freshness %q is invalid", s.Freshness)
	}
	return nil
}

type ReadContextCalibrationSummaryInput struct {
	Scope          Scope
	PolicyVersion  string
	SummaryVersion string
	Now            time.Time
	MaxAge         time.Duration
}

func (i ReadContextCalibrationSummaryInput) Validate() error {
	if err := i.Scope.Validate(); err != nil {
		return err
	}
	if strings.TrimSpace(i.PolicyVersion) == "" || strings.TrimSpace(i.SummaryVersion) == "" || i.Now.IsZero() || i.MaxAge <= 0 {
		return fmt.Errorf("context calibration summary read identity and age are required")
	}
	return nil
}
