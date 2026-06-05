package governance

import (
	"context"
	"fmt"
	"time"

	"github.com/FelixSeptem/stele/internal/memory"
)

type RawEventConsolidator interface {
	ProcessByRawEvent(ctx context.Context, rawEventID string) error
}

type ScopeCompactor interface {
	CompactScope(ctx context.Context, scope memory.Scope, cutoff time.Time) error
}

type PipelineProcessor struct {
	Extraction    ExtractionProcessor
	Consolidation RawEventConsolidator
	RawEvents     RawEventCompletionRecorder
	Summary       ScopeCompactor
	Now           func() time.Time
}

func (p PipelineProcessor) ProcessClaimedRawEvent(ctx context.Context, claim ClaimedRawEvent) error {
	if err := claim.Validate(); err != nil {
		return err
	}

	if p.Consolidation == nil {
		return fmt.Errorf("consolidation processor is required")
	}

	if p.RawEvents == nil {
		return fmt.Errorf("raw event completion recorder is required")
	}

	extraction := p.Extraction
	extraction.RawEvents = noopCompletionRecorder{}
	if err := extraction.ProcessClaimedRawEvent(ctx, claim); err != nil {
		return err
	}

	if err := p.Consolidation.ProcessByRawEvent(ctx, claim.Event.ID); err != nil {
		return err
	}

	now := time.Now
	if p.Now != nil {
		now = p.Now
	}
	processedAt := now().UTC()

	if p.Summary != nil {
		if err := p.Summary.CompactScope(ctx, claim.Event.Scope, processedAt); err != nil {
			return err
		}
	}

	return p.RawEvents.MarkRawEventProcessed(ctx, CompleteClaimedRawEventInput{
		RawEventID:  claim.Event.ID,
		WorkerID:    claim.WorkerID,
		ProcessedAt: processedAt,
	})
}

type noopCompletionRecorder struct{}

func (noopCompletionRecorder) MarkRawEventProcessed(ctx context.Context, input CompleteClaimedRawEventInput) error {
	return nil
}
