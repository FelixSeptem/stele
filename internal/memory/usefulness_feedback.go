package memory

import (
	"fmt"
	"strings"
	"time"
)

const (
	maxUsefulnessFeedbackIdempotencyKeyLength = 256
	maxUsefulnessFeedbackActorLength          = 256
	maxUsefulnessFeedbackReasonLength         = 2048
	maxUsefulnessFeedbackMetadataEntries      = 32
	maxUsefulnessFeedbackMetadataKeyLength    = 128
	maxUsefulnessFeedbackMetadataValueLength  = 1024
)

type UsefulnessFeedbackType string

const (
	UsefulnessFeedbackTypeUseful          UsefulnessFeedbackType = "useful"
	UsefulnessFeedbackTypeIrrelevant      UsefulnessFeedbackType = "irrelevant"
	UsefulnessFeedbackTypeNoisy           UsefulnessFeedbackType = "noisy"
	UsefulnessFeedbackTypeStale           UsefulnessFeedbackType = "stale"
	UsefulnessFeedbackTypeMissingExpected UsefulnessFeedbackType = "missing_expected"
	UsefulnessFeedbackTypeUnsafeOrHidden  UsefulnessFeedbackType = "unsafe_or_hidden"
	UsefulnessFeedbackTypeNeedsReview     UsefulnessFeedbackType = "needs_review"
)

func (t UsefulnessFeedbackType) Valid() bool {
	switch t {
	case UsefulnessFeedbackTypeUseful,
		UsefulnessFeedbackTypeIrrelevant,
		UsefulnessFeedbackTypeNoisy,
		UsefulnessFeedbackTypeStale,
		UsefulnessFeedbackTypeMissingExpected,
		UsefulnessFeedbackTypeUnsafeOrHidden,
		UsefulnessFeedbackTypeNeedsReview:
		return true
	default:
		return false
	}
}

func (t UsefulnessFeedbackType) Negative() bool {
	switch t {
	case UsefulnessFeedbackTypeIrrelevant,
		UsefulnessFeedbackTypeNoisy,
		UsefulnessFeedbackTypeStale,
		UsefulnessFeedbackTypeMissingExpected,
		UsefulnessFeedbackTypeUnsafeOrHidden:
		return true
	default:
		return false
	}
}

type UsefulnessFeedbackSourceSurface string

const (
	UsefulnessFeedbackSourceSearch       UsefulnessFeedbackSourceSurface = "search"
	UsefulnessFeedbackSourceContext      UsefulnessFeedbackSourceSurface = "context"
	UsefulnessFeedbackSourceSession      UsefulnessFeedbackSourceSurface = "session"
	UsefulnessFeedbackSourceVerification UsefulnessFeedbackSourceSurface = "verification"
	UsefulnessFeedbackSourceAdmin        UsefulnessFeedbackSourceSurface = "admin"
)

func (s UsefulnessFeedbackSourceSurface) Valid() bool {
	switch s {
	case UsefulnessFeedbackSourceSearch,
		UsefulnessFeedbackSourceContext,
		UsefulnessFeedbackSourceSession,
		UsefulnessFeedbackSourceVerification,
		UsefulnessFeedbackSourceAdmin:
		return true
	default:
		return false
	}
}

type UsefulnessFeedbackSubjectKind string

const (
	UsefulnessFeedbackSubjectMemory         UsefulnessFeedbackSubjectKind = "memory"
	UsefulnessFeedbackSubjectRawEvent       UsefulnessFeedbackSubjectKind = "raw_event"
	UsefulnessFeedbackSubjectCitation       UsefulnessFeedbackSubjectKind = "citation"
	UsefulnessFeedbackSubjectDerivedInsight UsefulnessFeedbackSubjectKind = "derived_insight"
	UsefulnessFeedbackSubjectSession        UsefulnessFeedbackSubjectKind = "session"
	UsefulnessFeedbackSubjectTurn           UsefulnessFeedbackSubjectKind = "turn"
	UsefulnessFeedbackSubjectVerification   UsefulnessFeedbackSubjectKind = "verification"
	UsefulnessFeedbackSubjectExpectedRecall UsefulnessFeedbackSubjectKind = "expected_recall"
)

func (k UsefulnessFeedbackSubjectKind) Valid() bool {
	switch k {
	case UsefulnessFeedbackSubjectMemory,
		UsefulnessFeedbackSubjectRawEvent,
		UsefulnessFeedbackSubjectCitation,
		UsefulnessFeedbackSubjectDerivedInsight,
		UsefulnessFeedbackSubjectSession,
		UsefulnessFeedbackSubjectTurn,
		UsefulnessFeedbackSubjectVerification,
		UsefulnessFeedbackSubjectExpectedRecall:
		return true
	default:
		return false
	}
}

type ExpectedRecallTargetKind string

const (
	ExpectedRecallTargetEvent        ExpectedRecallTargetKind = "event"
	ExpectedRecallTargetMemory       ExpectedRecallTargetKind = "memory"
	ExpectedRecallTargetCitation     ExpectedRecallTargetKind = "citation"
	ExpectedRecallTargetInsight      ExpectedRecallTargetKind = "insight"
	ExpectedRecallTargetSession      ExpectedRecallTargetKind = "session"
	ExpectedRecallTargetTurn         ExpectedRecallTargetKind = "turn"
	ExpectedRecallTargetVerification ExpectedRecallTargetKind = "verification"
	ExpectedRecallTargetOpaque       ExpectedRecallTargetKind = "opaque"
)

func (k ExpectedRecallTargetKind) Valid() bool {
	switch k {
	case ExpectedRecallTargetEvent,
		ExpectedRecallTargetMemory,
		ExpectedRecallTargetCitation,
		ExpectedRecallTargetInsight,
		ExpectedRecallTargetSession,
		ExpectedRecallTargetTurn,
		ExpectedRecallTargetVerification,
		ExpectedRecallTargetOpaque:
		return true
	default:
		return false
	}
}

type ExpectedRecallTarget struct {
	Kind        ExpectedRecallTargetKind `json:"kind"`
	ID          string                   `json:"id,omitempty"`
	OpaqueToken string                   `json:"opaque_token,omitempty"`
}

func (t ExpectedRecallTarget) Validate() error {
	if !t.Kind.Valid() {
		return fmt.Errorf("expected recall target kind %q is invalid", t.Kind)
	}
	if t.Kind == ExpectedRecallTargetOpaque {
		if strings.TrimSpace(t.OpaqueToken) == "" {
			return fmt.Errorf("opaque expected recall token is required")
		}
		if strings.TrimSpace(t.ID) != "" {
			return fmt.Errorf("opaque expected recall target must not include an internal id")
		}
		return nil
	}
	if strings.TrimSpace(t.ID) == "" {
		return fmt.Errorf("expected recall target id is required")
	}
	if strings.TrimSpace(t.OpaqueToken) != "" {
		return fmt.Errorf("known expected recall target must not include an opaque token")
	}
	return nil
}

type UsefulnessFeedbackSubject struct {
	Kind                 UsefulnessFeedbackSubjectKind `json:"kind"`
	ID                   string                        `json:"id,omitempty"`
	ExpectedRecallTarget ExpectedRecallTarget          `json:"expected_recall_target,omitempty"`
}

func (s UsefulnessFeedbackSubject) Validate() error {
	if !s.Kind.Valid() {
		return fmt.Errorf("usefulness feedback subject kind %q is invalid", s.Kind)
	}
	if s.Kind == UsefulnessFeedbackSubjectExpectedRecall {
		return s.ExpectedRecallTarget.Validate()
	}
	if strings.TrimSpace(s.ID) == "" {
		return fmt.Errorf("usefulness feedback subject id is required")
	}
	return nil
}

type UsefulnessQuality string

const (
	UsefulnessQualityUnknown     UsefulnessQuality = "unknown"
	UsefulnessQualityPositive    UsefulnessQuality = "positive"
	UsefulnessQualityNegative    UsefulnessQuality = "negative"
	UsefulnessQualityNeedsReview UsefulnessQuality = "needs_review"
	UsefulnessQualityMixed       UsefulnessQuality = "mixed"
)

type UsefulnessFeedback struct {
	ID                 string                          `json:"id"`
	Scope              Scope                           `json:"scope"`
	Type               UsefulnessFeedbackType          `json:"type"`
	SourceSurface      UsefulnessFeedbackSourceSurface `json:"source_surface"`
	Subjects           []UsefulnessFeedbackSubject     `json:"subjects"`
	Actor              string                          `json:"actor"`
	Reason             string                          `json:"reason"`
	IdempotencyKey     string                          `json:"idempotency_key,omitempty"`
	Metadata           map[string]any                  `json:"metadata,omitempty"`
	CreatedAt          time.Time                       `json:"created_at"`
	SupersededAt       time.Time                       `json:"superseded_at,omitempty"`
	SupersededByActor  string                          `json:"superseded_by_actor,omitempty"`
	SupersededByReason string                          `json:"superseded_by_reason,omitempty"`
}

func (f UsefulnessFeedback) Validate() error {
	if strings.TrimSpace(f.ID) == "" {
		return fmt.Errorf("usefulness feedback id is required")
	}
	if err := f.Scope.Validate(); err != nil {
		return err
	}
	if !f.Type.Valid() {
		return fmt.Errorf("usefulness feedback type %q is invalid", f.Type)
	}
	if !f.SourceSurface.Valid() {
		return fmt.Errorf("usefulness feedback source surface %q is invalid", f.SourceSurface)
	}
	if len(f.Subjects) == 0 {
		return fmt.Errorf("at least one usefulness feedback subject is required")
	}
	for _, subject := range f.Subjects {
		if err := subject.Validate(); err != nil {
			return err
		}
	}
	if strings.TrimSpace(f.Actor) == "" {
		return fmt.Errorf("actor is required")
	}
	if len(f.Actor) > maxUsefulnessFeedbackActorLength {
		return fmt.Errorf("actor must be at most %d bytes", maxUsefulnessFeedbackActorLength)
	}
	if strings.TrimSpace(f.Reason) == "" {
		return fmt.Errorf("reason is required")
	}
	if len(f.Reason) > maxUsefulnessFeedbackReasonLength {
		return fmt.Errorf("reason must be at most %d bytes", maxUsefulnessFeedbackReasonLength)
	}
	if f.IdempotencyKey != "" && len(f.IdempotencyKey) > maxUsefulnessFeedbackIdempotencyKeyLength {
		return fmt.Errorf("idempotency key must be at most %d bytes", maxUsefulnessFeedbackIdempotencyKeyLength)
	}
	if err := validateUsefulnessFeedbackMetadata(f.Metadata); err != nil {
		return err
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

func validateUsefulnessFeedbackMetadata(metadata map[string]any) error {
	if len(metadata) > maxUsefulnessFeedbackMetadataEntries {
		return fmt.Errorf("metadata must contain at most %d entries", maxUsefulnessFeedbackMetadataEntries)
	}
	for key, value := range metadata {
		if strings.TrimSpace(key) == "" {
			return fmt.Errorf("metadata key is required")
		}
		if len(key) > maxUsefulnessFeedbackMetadataKeyLength {
			return fmt.Errorf("metadata key must be at most %d bytes", maxUsefulnessFeedbackMetadataKeyLength)
		}
		if len(fmt.Sprint(value)) > maxUsefulnessFeedbackMetadataValueLength {
			return fmt.Errorf("metadata value must be at most %d bytes", maxUsefulnessFeedbackMetadataValueLength)
		}
	}
	return nil
}

type SupersedeUsefulnessFeedbackInput struct {
	Scope        Scope
	FeedbackID   string
	Actor        string
	Reason       string
	SupersededAt time.Time
}

func (i SupersedeUsefulnessFeedbackInput) Validate() error {
	if err := i.Scope.Validate(); err != nil {
		return err
	}
	if strings.TrimSpace(i.FeedbackID) == "" {
		return fmt.Errorf("usefulness feedback id is required")
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

type ReadUsefulnessFeedbackInput struct {
	Scope      Scope
	FeedbackID string
}

func (i ReadUsefulnessFeedbackInput) Validate() error {
	if err := i.Scope.Validate(); err != nil {
		return err
	}
	if strings.TrimSpace(i.FeedbackID) == "" {
		return fmt.Errorf("usefulness feedback id is required")
	}
	return nil
}

type ListUsefulnessFeedbackInput struct {
	Scope             Scope
	Subject           UsefulnessFeedbackSubject
	Type              UsefulnessFeedbackType
	IncludeSuperseded bool
	Limit             int
}

func (i ListUsefulnessFeedbackInput) Validate() error {
	if err := i.Scope.Validate(); err != nil {
		return err
	}
	if i.Subject.Kind != "" {
		if err := i.Subject.Validate(); err != nil {
			return err
		}
	}
	if i.Type != "" && !i.Type.Valid() {
		return fmt.Errorf("usefulness feedback type %q is invalid", i.Type)
	}
	if i.Limit < 0 {
		return fmt.Errorf("limit must be greater than or equal to zero")
	}
	return nil
}

type SummarizeUsefulnessFeedbackInput struct {
	Scope   Scope
	Subject UsefulnessFeedbackSubject
}

func (i SummarizeUsefulnessFeedbackInput) Validate() error {
	if err := i.Scope.Validate(); err != nil {
		return err
	}
	return i.Subject.Validate()
}

type UsefulnessFeedbackSummary struct {
	Subject          UsefulnessFeedbackSubject      `json:"subject"`
	Counts           map[UsefulnessFeedbackType]int `json:"counts"`
	TotalActive      int                            `json:"total_active"`
	PositiveCount    int                            `json:"positive_count"`
	NegativeCount    int                            `json:"negative_count"`
	NeedsReviewCount int                            `json:"needs_review_count"`
	EffectiveQuality UsefulnessQuality              `json:"effective_quality"`
	LastFeedbackAt   time.Time                      `json:"last_feedback_at,omitempty"`
}

func SummarizeUsefulnessFeedback(subject UsefulnessFeedbackSubject, records []UsefulnessFeedback) UsefulnessFeedbackSummary {
	summary := UsefulnessFeedbackSummary{
		Subject: subject,
		Counts: map[UsefulnessFeedbackType]int{
			UsefulnessFeedbackTypeUseful:          0,
			UsefulnessFeedbackTypeIrrelevant:      0,
			UsefulnessFeedbackTypeNoisy:           0,
			UsefulnessFeedbackTypeStale:           0,
			UsefulnessFeedbackTypeMissingExpected: 0,
			UsefulnessFeedbackTypeUnsafeOrHidden:  0,
			UsefulnessFeedbackTypeNeedsReview:     0,
		},
		EffectiveQuality: UsefulnessQualityUnknown,
	}
	for _, record := range records {
		if !record.SupersededAt.IsZero() {
			continue
		}
		summary.Counts[record.Type]++
		summary.TotalActive++
		if record.Type == UsefulnessFeedbackTypeUseful {
			summary.PositiveCount++
		}
		if record.Type.Negative() {
			summary.NegativeCount++
		}
		if record.Type == UsefulnessFeedbackTypeNeedsReview {
			summary.NeedsReviewCount++
		}
		if record.CreatedAt.After(summary.LastFeedbackAt) {
			summary.LastFeedbackAt = record.CreatedAt
		}
	}
	summary.EffectiveQuality = effectiveUsefulnessQuality(summary)
	return summary
}

func effectiveUsefulnessQuality(summary UsefulnessFeedbackSummary) UsefulnessQuality {
	switch {
	case summary.TotalActive == 0:
		return UsefulnessQualityUnknown
	case summary.NeedsReviewCount > 0 || summary.Counts[UsefulnessFeedbackTypeUnsafeOrHidden] > 0:
		return UsefulnessQualityNeedsReview
	case summary.PositiveCount > 0 && summary.NegativeCount > 0:
		return UsefulnessQualityMixed
	case summary.NegativeCount > 0:
		return UsefulnessQualityNegative
	case summary.PositiveCount > 0:
		return UsefulnessQualityPositive
	default:
		return UsefulnessQualityUnknown
	}
}
