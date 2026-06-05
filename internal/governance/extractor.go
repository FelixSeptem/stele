package governance

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/FelixSeptem/stele/internal/memory"
	"github.com/FelixSeptem/stele/internal/policy"
)

type ExtractedCandidate struct {
	Class          memory.MemoryClass
	Content        string
	Confidence     float64
	Importance     float64
	Freshness      float64
	Sensitivity    Sensitivity
	Mutability     Mutability
	RetentionClass policy.RetentionClass
}

func (c ExtractedCandidate) Validate() error {
	switch {
	case validateCandidateClass(c.Class) != nil:
		return validateCandidateClass(c.Class)
	case strings.TrimSpace(c.Content) == "":
		return fmt.Errorf("extracted candidate content is required")
	case validateScore("confidence", c.Confidence) != nil:
		return validateScore("confidence", c.Confidence)
	case validateScore("importance", c.Importance) != nil:
		return validateScore("importance", c.Importance)
	case validateScore("freshness", c.Freshness) != nil:
		return validateScore("freshness", c.Freshness)
	case c.Sensitivity.Validate() != nil:
		return c.Sensitivity.Validate()
	case c.Mutability.Validate() != nil:
		return c.Mutability.Validate()
	case c.RetentionClass.Validate() != nil:
		return c.RetentionClass.Validate()
	default:
		return nil
	}
}

func (c ExtractedCandidate) ToCandidateMemory(id string, event memory.RawEvent, now time.Time) CandidateMemory {
	return CandidateMemory{
		ID:               id,
		SourceRawEventID: event.ID,
		Scope:            event.Scope,
		Class:            c.Class,
		Content:          strings.TrimSpace(c.Content),
		Confidence:       c.Confidence,
		Importance:       c.Importance,
		Freshness:        c.Freshness,
		Sensitivity:      c.Sensitivity,
		Mutability:       c.Mutability,
		RetentionClass:   c.RetentionClass,
		Status:           CandidateStatusPending,
		CreatedAt:        now,
		UpdatedAt:        now,
	}
}

type Extractor interface {
	Extract(ctx context.Context, event memory.RawEvent) ([]ExtractedCandidate, error)
}

type RuleBasedExtractor struct{}

func (RuleBasedExtractor) Extract(ctx context.Context, event memory.RawEvent) ([]ExtractedCandidate, error) {
	content := strings.TrimSpace(event.Content)
	if content == "" {
		return nil, nil
	}

	candidate := ExtractedCandidate{
		Class:          memory.MemoryClassEpisodic,
		Content:        content,
		Confidence:     0.7,
		Importance:     0.5,
		Freshness:      0.8,
		Sensitivity:    SensitivityLow,
		Mutability:     MutabilityImmutable,
		RetentionClass: policy.RetentionClassSession,
	}

	switch strings.TrimSpace(event.EventType) {
	case "conversation.message":
		return []ExtractedCandidate{candidate}, nil
	default:
		return nil, nil
	}
}

type ExtractionProcessor struct {
	Extractor       Extractor
	Candidates      CandidateRepository
	RawEvents       RawEventCompletionRecorder
	Now             func() time.Time
	NewCandidateID  func() string
	NewProvenanceID func() string
}

func (p ExtractionProcessor) ProcessClaimedRawEvent(ctx context.Context, claim ClaimedRawEvent) error {
	if err := claim.Validate(); err != nil {
		return err
	}

	if p.Extractor == nil {
		return fmt.Errorf("extractor is required")
	}

	if p.Candidates == nil {
		return fmt.Errorf("candidate repository is required")
	}

	if p.RawEvents == nil {
		return fmt.Errorf("raw event completion recorder is required")
	}

	now := time.Now
	if p.Now != nil {
		now = p.Now
	}

	newCandidateID := p.NewCandidateID
	if newCandidateID == nil {
		return fmt.Errorf("candidate id generator is required")
	}

	newProvenanceID := p.NewProvenanceID
	if newProvenanceID == nil {
		return fmt.Errorf("provenance id generator is required")
	}

	extracted, err := p.Extractor.Extract(ctx, claim.Event)
	if err != nil {
		return err
	}

	processedAt := now().UTC()
	for _, item := range extracted {
		if err := item.Validate(); err != nil {
			return err
		}

		candidate := item.ToCandidateMemory(newCandidateID(), claim.Event, processedAt)
		provenance := memory.ProvenanceRecord{
			ID:                newProvenanceID(),
			Scope:             claim.Event.Scope,
			RawEventID:        claim.Event.ID,
			CandidateMemoryID: candidate.ID,
			Operation:         "extract_candidate",
			CreatedAt:         processedAt,
		}

		if _, err := p.Candidates.CreateCandidate(ctx, candidate, provenance); err != nil {
			return err
		}
	}

	return p.RawEvents.MarkRawEventProcessed(ctx, CompleteClaimedRawEventInput{
		RawEventID:  claim.Event.ID,
		WorkerID:    claim.WorkerID,
		ProcessedAt: processedAt,
	})
}
