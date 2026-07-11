package memory

import (
	"fmt"
	"strings"
	"time"
)

type InsightFeedbackType string

const (
	InsightFeedbackTypeUseful      InsightFeedbackType = "useful"
	InsightFeedbackTypeNoisy       InsightFeedbackType = "noisy"
	InsightFeedbackTypeIncorrect   InsightFeedbackType = "incorrect"
	InsightFeedbackTypeStale       InsightFeedbackType = "stale"
	InsightFeedbackTypeRedundant   InsightFeedbackType = "redundant"
	InsightFeedbackTypeNeedsReview InsightFeedbackType = "needs_review"
)

func (t InsightFeedbackType) Valid() bool {
	switch t {
	case InsightFeedbackTypeUseful,
		InsightFeedbackTypeNoisy,
		InsightFeedbackTypeIncorrect,
		InsightFeedbackTypeStale,
		InsightFeedbackTypeRedundant,
		InsightFeedbackTypeNeedsReview:
		return true
	default:
		return false
	}
}

func (t InsightFeedbackType) Negative() bool {
	switch t {
	case InsightFeedbackTypeNoisy, InsightFeedbackTypeIncorrect, InsightFeedbackTypeStale, InsightFeedbackTypeRedundant:
		return true
	default:
		return false
	}
}

type DerivedInsightFeedback struct {
	ID                 string              `json:"id"`
	InsightID          string              `json:"insight_id"`
	Scope              Scope               `json:"scope"`
	Type               InsightFeedbackType `json:"type"`
	Actor              string              `json:"actor"`
	Reason             string              `json:"reason"`
	QualityScore       *float64            `json:"quality_score,omitempty"`
	CreatedAt          time.Time           `json:"created_at"`
	SupersededAt        time.Time           `json:"superseded_at,omitempty"`
	SupersededByActor   string              `json:"superseded_by_actor,omitempty"`
	SupersededByReason  string              `json:"superseded_by_reason,omitempty"`
	RequestID          string              `json:"request_id,omitempty"`
	Metadata           map[string]any      `json:"metadata,omitempty"`
}

func (f DerivedInsightFeedback) Validate() error {
	if strings.TrimSpace(f.ID) == "" {
		return fmt.Errorf("insight feedback id is required")
	}
	if strings.TrimSpace(f.InsightID) == "" {
		return fmt.Errorf("derived insight id is required")
	}
	if err := f.Scope.Validate(); err != nil {
		return err
	}
	if !f.Type.Valid() {
		return fmt.Errorf("insight feedback type %q is invalid", f.Type)
	}
	if strings.TrimSpace(f.Actor) == "" {
		return fmt.Errorf("actor is required")
	}
	if strings.TrimSpace(f.Reason) == "" {
		return fmt.Errorf("reason is required")
	}
	if f.QualityScore != nil && (*f.QualityScore < 0 || *f.QualityScore > 1) {
		return fmt.Errorf("quality score must be between 0 and 1")
	}
	if f.CreatedAt.IsZero() {
		return fmt.Errorf("created at is required")
	}
	if !f.SupersededAt.IsZero() {
		if strings.TrimSpace(f.SupersededByActor) == "" {
			return fmt.Errorf("superseded by actor is required")
		}
		if strings.TrimSpace(f.SupersededByReason) == "" {
			return fmt.Errorf("superseded by reason is required")
		}
	}
	return nil
}

type CreateDerivedInsightFeedbackInput struct {
	ID           string
	Scope        Scope
	InsightID    string
	Type         InsightFeedbackType
	Actor        string
	Reason       string
	QualityScore *float64
	CreatedAt    time.Time
	RequestID     string
	Metadata      map[string]any
}

func (i CreateDerivedInsightFeedbackInput) Validate() error {
	feedback := DerivedInsightFeedback{
		ID:           i.ID,
		InsightID:    i.InsightID,
		Scope:        i.Scope,
		Type:         i.Type,
		Actor:        i.Actor,
		Reason:       i.Reason,
		QualityScore: i.QualityScore,
		CreatedAt:    i.CreatedAt,
		RequestID:     i.RequestID,
		Metadata:      i.Metadata,
	}
	return feedback.Validate()
}

type ListDerivedInsightFeedbackInput struct {
	Scope             Scope
	InsightID         string
	Type              InsightFeedbackType
	IncludeSuperseded bool
	Limit             int
}

func (i ListDerivedInsightFeedbackInput) Validate() error {
	if err := i.Scope.Validate(); err != nil {
		return err
	}
	if strings.TrimSpace(i.InsightID) == "" {
		return fmt.Errorf("derived insight id is required")
	}
	if i.Type != "" && !i.Type.Valid() {
		return fmt.Errorf("insight feedback type %q is invalid", i.Type)
	}
	if i.Limit <= 0 {
		return fmt.Errorf("limit must be greater than zero")
	}
	return nil
}

type ReadDerivedInsightFeedbackInput struct {
	Scope      Scope
	FeedbackID string
}

func (i ReadDerivedInsightFeedbackInput) Validate() error {
	if err := i.Scope.Validate(); err != nil {
		return err
	}
	if strings.TrimSpace(i.FeedbackID) == "" {
		return fmt.Errorf("insight feedback id is required")
	}
	return nil
}

type SummarizeDerivedInsightFeedbackInput struct {
	Scope     Scope
	InsightID string
}

func (i SummarizeDerivedInsightFeedbackInput) Validate() error {
	if err := i.Scope.Validate(); err != nil {
		return err
	}
	if strings.TrimSpace(i.InsightID) == "" {
		return fmt.Errorf("derived insight id is required")
	}
	return nil
}

type SupersedeDerivedInsightFeedbackInput struct {
	Scope        Scope
	FeedbackID   string
	Actor        string
	Reason       string
	SupersededAt time.Time
}

func (i SupersedeDerivedInsightFeedbackInput) Validate() error {
	if err := i.Scope.Validate(); err != nil {
		return err
	}
	if strings.TrimSpace(i.FeedbackID) == "" {
		return fmt.Errorf("insight feedback id is required")
	}
	if strings.TrimSpace(i.Actor) == "" {
		return fmt.Errorf("actor is required")
	}
	if strings.TrimSpace(i.Reason) == "" {
		return fmt.Errorf("reason is required")
	}
	if i.SupersededAt.IsZero() {
		return fmt.Errorf("superseded at is required")
	}
	return nil
}

type DerivedInsightFeedbackSummary struct {
	InsightID     string                       `json:"insight_id,omitempty"`
	Counts        map[InsightFeedbackType]int  `json:"counts"`
	TotalActive   int                          `json:"total_active"`
	PositiveCount int                          `json:"positive_count"`
	NegativeCount int                          `json:"negative_count"`
	NeedsReview   bool                         `json:"needs_review"`
	LastFeedbackAt time.Time                    `json:"last_feedback_at,omitempty"`
}

func SummarizeDerivedInsightFeedback(records []DerivedInsightFeedback) DerivedInsightFeedbackSummary {
	summary := DerivedInsightFeedbackSummary{
		Counts: map[InsightFeedbackType]int{
			InsightFeedbackTypeUseful:      0,
			InsightFeedbackTypeNoisy:       0,
			InsightFeedbackTypeIncorrect:   0,
			InsightFeedbackTypeStale:       0,
			InsightFeedbackTypeRedundant:   0,
			InsightFeedbackTypeNeedsReview: 0,
		},
	}
	for _, record := range records {
		if summary.InsightID == "" {
			summary.InsightID = record.InsightID
		}
		if !record.SupersededAt.IsZero() {
			continue
		}
		summary.Counts[record.Type]++
		summary.TotalActive++
		if record.Type == InsightFeedbackTypeUseful {
			summary.PositiveCount++
		}
		if record.Type.Negative() {
			summary.NegativeCount++
		}
		if record.Type == InsightFeedbackTypeNeedsReview {
			summary.NeedsReview = true
		}
		if record.CreatedAt.After(summary.LastFeedbackAt) {
			summary.LastFeedbackAt = record.CreatedAt
		}
	}
	return summary
}
