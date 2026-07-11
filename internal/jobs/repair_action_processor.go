package jobs

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/FelixSeptem/stele/internal/governance"
	"github.com/FelixSeptem/stele/internal/memory"
)

type EmbeddingRecoveryApplier interface {
	ApplyEmbeddingRecovery(ctx context.Context, input memory.ApplyEmbeddingRecoveryInput) (memory.EmbeddingRecoveryOutcome, error)
}

type GovernanceRecoveryApplier interface {
	ApplyGovernanceRecovery(ctx context.Context, input governance.ApplyGovernanceRecoveryInput) (governance.GovernanceRecoveryOutcome, error)
}

type DerivedInsightReplayApplier interface {
	ApplyDerivedInsightReplay(ctx context.Context, input memory.DerivedInsightReplayRequest) (memory.DerivedInsightReplayRun, error)
}

type GovernedRepairActionProcessor struct {
	Embedding           EmbeddingRecoveryApplier
	Governance          GovernanceRecoveryApplier
	Replay              DerivedInsightReplayApplier
	Now                 func() time.Time
	Actor               string
	Reason              string
	ReplayWindow        time.Duration
	ReplayEvidenceLimit int
}

func (p GovernedRepairActionProcessor) ProcessRepairAction(ctx context.Context, action memory.RepairAction) error {
	now := p.nowUTC()
	actor := strings.TrimSpace(p.Actor)
	if actor == "" {
		actor = "quality_repair_worker"
	}
	reason := strings.TrimSpace(p.Reason)
	if reason == "" {
		reason = repairActionReason(action)
	}

	switch action.Category {
	case memory.RepairActionCategoryEmbeddingRetry:
		if p.Embedding == nil {
			return fmt.Errorf("embedding recovery applier is required")
		}
		if strings.TrimSpace(action.TargetID) == "" {
			return fmt.Errorf("embedding repair target id is required")
		}
		_, err := p.Embedding.ApplyEmbeddingRecovery(ctx, memory.ApplyEmbeddingRecoveryInput{
			Scope:     action.Scope,
			MemoryID:  action.TargetID,
			Action:    memory.EmbeddingRecoveryActionRetry,
			Actor:     actor,
			Reason:    reason,
			AppliedAt: now,
		})
		return err
	case memory.RepairActionCategoryGovernanceRequeue:
		if p.Governance == nil {
			return fmt.Errorf("governance recovery applier is required")
		}
		if strings.TrimSpace(action.TargetID) == "" {
			return fmt.Errorf("governance repair target id is required")
		}
		_, err := p.Governance.ApplyGovernanceRecovery(ctx, governance.ApplyGovernanceRecoveryInput{
			Scope:      action.Scope,
			RawEventID: strings.TrimSpace(action.TargetID),
			Action:     governance.GovernanceRecoveryActionRequeue,
			Actor:      actor,
			Reason:     reason,
			AppliedAt:  now,
		})
		return err
	case memory.RepairActionCategoryInsightReplay:
		if p.Replay == nil {
			return fmt.Errorf("derived insight replay applier is required")
		}
		window := p.ReplayWindow
		if window <= 0 {
			window = 24 * time.Hour
		}
		limit := p.ReplayEvidenceLimit
		if limit <= 0 {
			limit = 100
		}
		_, err := p.Replay.ApplyDerivedInsightReplay(ctx, memory.DerivedInsightReplayRequest{
			Scope:               action.Scope,
			Mode:                memory.DerivedInsightReplayModeApply,
			InsightTypes:        []memory.DerivedInsightType{memory.DerivedInsightTypeFailurePattern, memory.DerivedInsightTypeLesson},
			EvidenceWindowStart: now.Add(-window),
			EvidenceWindowEnd:   now,
			EvidenceLimit:       limit,
			Actor:               actor,
			Reason:              reason,
			IdempotencyKey:      "repair_action:" + strings.TrimSpace(action.ID),
			RequestedAt:         now,
			Metadata: map[string]any{
				"repair_action_id": action.ID,
				"repair_plan_id":   action.PlanID,
			},
		})
		return err
	case memory.RepairActionCategoryManualReview:
		return nil
	default:
		return fmt.Errorf("repair action category %q is not supported", action.Category)
	}
}

func (p GovernedRepairActionProcessor) nowUTC() time.Time {
	if p.Now != nil {
		return p.Now().UTC()
	}
	return time.Now().UTC()
}

func repairActionReason(action memory.RepairAction) string {
	if action.ReasonCode != "" {
		return "quality repair action for " + string(action.ReasonCode)
	}
	return "quality repair action"
}

type CompositeWorker struct {
	Workers []LoopWorker
}

func (w CompositeWorker) RunOnce(ctx context.Context) (int, error) {
	total := 0
	for _, worker := range w.Workers {
		if worker == nil {
			continue
		}
		processed, err := worker.RunOnce(ctx)
		if err != nil {
			return total, err
		}
		total += processed
	}
	return total, nil
}
