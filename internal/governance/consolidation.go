package governance

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/FelixSeptem/stele/internal/memory"
)

type ConsolidationAction string

const (
	ConsolidationActionPromote   ConsolidationAction = "promote"
	ConsolidationActionSupersede ConsolidationAction = "supersede"
	ConsolidationActionCoexist   ConsolidationAction = "coexist"
	ConsolidationActionSuppress  ConsolidationAction = "suppress"
)

type ConsolidationDecision struct {
	Action ConsolidationAction
	Reason string
}

type Consolidator interface {
	Decide(ctx context.Context, candidate CandidateMemory, latest memory.CanonicalMemory, found bool) (ConsolidationDecision, error)
}

type RuleBasedConsolidator struct{}

func (RuleBasedConsolidator) Decide(ctx context.Context, candidate CandidateMemory, latest memory.CanonicalMemory, found bool) (ConsolidationDecision, error) {
	if err := candidate.Validate(); err != nil {
		return ConsolidationDecision{}, err
	}

	if candidate.Confidence < 0.5 {
		return ConsolidationDecision{
			Action: ConsolidationActionSuppress,
			Reason: "confidence_below_threshold",
		}, nil
	}

	if !found {
		return ConsolidationDecision{
			Action: ConsolidationActionPromote,
			Reason: "no_existing_canonical",
		}, nil
	}

	switch candidate.Class {
	case memory.MemoryClassEpisodic:
		return ConsolidationDecision{
			Action: ConsolidationActionCoexist,
			Reason: "episodic_evidence_coexists",
		}, nil
	case memory.MemoryClassProfile, memory.MemoryClassProcedural:
		if candidate.Mutability == MutabilityMutable && strings.TrimSpace(candidate.Content) != strings.TrimSpace(latest.Content) {
			return ConsolidationDecision{
				Action: ConsolidationActionSupersede,
				Reason: "mutable_fact_changed",
			}, nil
		}
	}

	return ConsolidationDecision{
		Action: ConsolidationActionPromote,
		Reason: "accepted_candidate",
	}, nil
}

type CanonicalPromotion struct {
	Candidate CandidateMemory
	MemoryID  string
	VersionID string
	Version   int64
	CreatedAt time.Time
}

type CanonicalRepository interface {
	GetLatestCanonicalByScopeAndClass(ctx context.Context, scope memory.Scope, class memory.MemoryClass) (memory.CanonicalMemory, bool, error)
	PromoteCandidate(ctx context.Context, input CanonicalPromotion) (memory.CanonicalMemory, memory.MemoryVersion, error)
}

type ConsolidationProcessor struct {
	Candidates      CandidateRepository
	Canonicals      CanonicalRepository
	Consolidator    Consolidator
	Now             func() time.Time
	NewMemoryID     func() string
	NewVersionID    func() string
	NewProvenanceID func() string
}

func (p ConsolidationProcessor) ProcessByRawEvent(ctx context.Context, rawEventID string) error {
	if strings.TrimSpace(rawEventID) == "" {
		return fmt.Errorf("raw event id is required")
	}

	if p.Candidates == nil {
		return fmt.Errorf("candidate repository is required")
	}

	if p.Canonicals == nil {
		return fmt.Errorf("canonical repository is required")
	}

	if p.Consolidator == nil {
		return fmt.Errorf("consolidator is required")
	}

	if p.NewProvenanceID == nil {
		return fmt.Errorf("provenance id generator is required")
	}

	now := time.Now
	if p.Now != nil {
		now = p.Now
	}
	currentTime := now().UTC()

	candidates, err := p.Candidates.ListCandidatesByRawEvent(ctx, rawEventID)
	if err != nil {
		return err
	}

	for _, candidate := range candidates {
		latest, found, err := p.Canonicals.GetLatestCanonicalByScopeAndClass(ctx, candidate.Scope, candidate.Class)
		if err != nil {
			return err
		}

		decision, err := p.Consolidator.Decide(ctx, candidate, latest, found)
		if err != nil {
			return err
		}

		switch decision.Action {
		case ConsolidationActionSuppress:
			if _, err := p.Candidates.TransitionCandidateStatus(ctx, CandidateStatusTransition{
				CandidateID: candidate.ID,
				ToStatus:    CandidateStatusSuppressed,
				UpdatedAt:   currentTime,
			}, memory.ProvenanceRecord{
				ID:                p.NewProvenanceID(),
				Scope:             candidate.Scope,
				RawEventID:        candidate.SourceRawEventID,
				CandidateMemoryID: candidate.ID,
				Operation:         "suppress_candidate",
				CreatedAt:         currentTime,
			}); err != nil {
				return err
			}
		case ConsolidationActionPromote, ConsolidationActionSupersede, ConsolidationActionCoexist:
			memoryID := latest.ID
			if decision.Action != ConsolidationActionSupersede {
				memoryID = ""
			}

			if memoryID == "" && p.NewMemoryID == nil {
				return fmt.Errorf("memory id generator is required")
			}
			if p.NewVersionID == nil {
				return fmt.Errorf("version id generator is required")
			}
			if memoryID == "" {
				memoryID = p.NewMemoryID()
			}

			if _, _, err := p.Canonicals.PromoteCandidate(ctx, CanonicalPromotion{
				Candidate: candidate,
				MemoryID:  memoryID,
				VersionID: p.NewVersionID(),
				Version:   1,
				CreatedAt: currentTime,
			}); err != nil {
				return err
			}

			if _, err := p.Candidates.TransitionCandidateStatus(ctx, CandidateStatusTransition{
				CandidateID: candidate.ID,
				ToStatus:    CandidateStatusPromoted,
				UpdatedAt:   currentTime,
			}, memory.ProvenanceRecord{
				ID:                p.NewProvenanceID(),
				Scope:             candidate.Scope,
				RawEventID:        candidate.SourceRawEventID,
				CandidateMemoryID: candidate.ID,
				Operation:         "promote_candidate",
				CreatedAt:         currentTime,
			}); err != nil {
				return err
			}
		default:
			return fmt.Errorf("unsupported consolidation action %q", decision.Action)
		}
	}

	return nil
}
