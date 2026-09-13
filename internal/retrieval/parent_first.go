package retrieval

import (
	"fmt"
	"math"
	"strings"
	"time"

	"github.com/FelixSeptem/stele/internal/memory"
)

type ParentFirstMode string

const (
	ParentFirstModeShadow   ParentFirstMode = "shadow"
	ParentFirstModeDisabled ParentFirstMode = "disabled"
	ParentFirstModeRollback ParentFirstMode = "rollback"
)
const ParentFirstFlatStrategy = "flat-fusion-v1"

type ParentFirstFailureReason string

const (
	ParentFirstFailureIsolation       ParentFirstFailureReason = "isolation"
	ParentFirstFailureLifecycle       ParentFirstFailureReason = "lifecycle"
	ParentFirstFailureRecall          ParentFirstFailureReason = "protected_recall_regression"
	ParentFirstFailureMultiHop        ParentFirstFailureReason = "multi_hop_regression"
	ParentFirstFailureDuplicate       ParentFirstFailureReason = "duplicate_rate"
	ParentFirstFailureLatency         ParentFirstFailureReason = "latency"
	ParentFirstFailureCandidateBudget ParentFirstFailureReason = "candidate_budget"
)

type ParentFirstCandidate struct {
	ID               string
	Scope            memory.Scope
	LifecycleState   memory.MemoryState
	Score            float64
	ProjectionStatus memory.ContextProjectionStatus
	UpdatedAt        time.Time
}
type ParentFirstChild struct {
	ParentID         string
	AdjacentDistance int
	Evidence         ProgressiveContextEvidence
}
type ParentFirstExpansionLimits struct{ MaxParents, MaxChildrenPerParent, MaxAdjacentDistance, MaxCandidates int }
type ParentFirstMetrics struct {
	ProtectedRecall, MultiHopCoverage, DuplicateRate, P95LatencyMS float64
	CandidateCount                                                 int
}
type ParentFirstGates struct{ MaxRecallRegression, MaxMultiHopRegression, MaxDuplicateRate, MaxP95LatencyMS float64 }
type ParentFirstDiagnostics struct{ ParentsConsidered, ChildrenIncluded, OmittedByDistance, IsolationFailures, LifecycleFailures, Disabled int }
type ParentFirstDelta struct{ ProtectedRecall, MultiHopCoverage, DuplicateRate, P95LatencyMS float64 }
type ParentFirstEvaluationInput struct {
	Scope               memory.Scope
	StrategyIdentity    string
	Mode                ParentFirstMode
	Parents             []ParentFirstCandidate
	Children            []ParentFirstChild
	Limits              ParentFirstExpansionLimits
	Baseline, Candidate ParentFirstMetrics
	Gates               ParentFirstGates
}
type ParentFirstEvaluationReport struct {
	StrategyIdentity string                       `json:"strategy_identity"`
	Mode             ParentFirstMode              `json:"mode"`
	SelectedStrategy string                       `json:"selected_strategy"`
	Eligible         bool                         `json:"eligible"`
	Expanded         []ProgressiveContextEvidence `json:"expanded,omitempty"`
	Deltas           ParentFirstDelta             `json:"deltas"`
	Diagnostics      ParentFirstDiagnostics       `json:"diagnostics"`
	HardFailures     []ParentFirstFailureReason   `json:"hard_failures,omitempty"`
}

func EvaluateParentFirst(in ParentFirstEvaluationInput) (ParentFirstEvaluationReport, error) {
	if err := in.Scope.Validate(); err != nil {
		return ParentFirstEvaluationReport{}, err
	}
	if strings.TrimSpace(in.StrategyIdentity) == "" {
		return ParentFirstEvaluationReport{}, fmt.Errorf("strategy identity is required")
	}
	r := ParentFirstEvaluationReport{StrategyIdentity: in.StrategyIdentity, Mode: in.Mode, SelectedStrategy: in.StrategyIdentity, Eligible: true}
	if in.Mode == ParentFirstModeDisabled || in.Mode == ParentFirstModeRollback {
		r.SelectedStrategy = ParentFirstFlatStrategy
		r.Eligible = false
		r.Diagnostics.Disabled = 1
		return r, nil
	}
	if in.Mode != ParentFirstModeShadow {
		return ParentFirstEvaluationReport{}, fmt.Errorf("unsupported parent-first mode")
	}
	parents := in.Parents
	if in.Limits.MaxParents > 0 && len(parents) > in.Limits.MaxParents {
		parents = parents[:in.Limits.MaxParents]
	}
	r.Diagnostics.ParentsConsidered = len(parents)
	seen := map[string]struct{}{}
	childCounts := map[string]int{}
	for _, p := range parents {
		if p.Scope.Normalized() != in.Scope.Normalized() {
			addParentFailure(&r, ParentFirstFailureIsolation)
			r.Diagnostics.IsolationFailures++
			continue
		}
		if p.LifecycleState != memory.MemoryStateActive || p.ProjectionStatus != memory.ContextProjectionStatusActive {
			addParentFailure(&r, ParentFirstFailureLifecycle)
			r.Diagnostics.LifecycleFailures++
			continue
		}
		for _, c := range in.Children {
			if c.ParentID != p.ID {
				continue
			}
			if in.Limits.MaxAdjacentDistance >= 0 && c.AdjacentDistance > in.Limits.MaxAdjacentDistance {
				r.Diagnostics.OmittedByDistance++
				continue
			}
			if c.Evidence.Memory.Scope.Normalized() != in.Scope.Normalized() {
				addParentFailure(&r, ParentFirstFailureIsolation)
				r.Diagnostics.IsolationFailures++
				continue
			}
			if c.Evidence.Memory.State != memory.MemoryStateActive {
				addParentFailure(&r, ParentFirstFailureLifecycle)
				r.Diagnostics.LifecycleFailures++
				continue
			}
			if in.Limits.MaxChildrenPerParent > 0 && childCounts[p.ID] >= in.Limits.MaxChildrenPerParent {
				continue
			}
			if _, ok := seen[c.Evidence.Alias]; ok {
				continue
			}
			seen[c.Evidence.Alias] = struct{}{}
			r.Expanded = append(r.Expanded, c.Evidence)
			childCounts[p.ID]++
			r.Diagnostics.ChildrenIncluded++
			if in.Limits.MaxCandidates > 0 && len(r.Expanded) >= in.Limits.MaxCandidates {
				break
			}
		}
	}
	r.Deltas = ParentFirstDelta{ProtectedRecall: in.Candidate.ProtectedRecall - in.Baseline.ProtectedRecall, MultiHopCoverage: in.Candidate.MultiHopCoverage - in.Baseline.MultiHopCoverage, DuplicateRate: in.Candidate.DuplicateRate - in.Baseline.DuplicateRate, P95LatencyMS: in.Candidate.P95LatencyMS - in.Baseline.P95LatencyMS}
	r.Deltas.ProtectedRecall = math.Round(r.Deltas.ProtectedRecall*1e9) / 1e9
	r.Deltas.MultiHopCoverage = math.Round(r.Deltas.MultiHopCoverage*1e9) / 1e9
	if in.Baseline.ProtectedRecall-in.Candidate.ProtectedRecall > in.Gates.MaxRecallRegression {
		addParentFailure(&r, ParentFirstFailureRecall)
	}
	if in.Baseline.MultiHopCoverage-in.Candidate.MultiHopCoverage > in.Gates.MaxMultiHopRegression {
		addParentFailure(&r, ParentFirstFailureMultiHop)
	}
	if in.Candidate.DuplicateRate > in.Gates.MaxDuplicateRate {
		addParentFailure(&r, ParentFirstFailureDuplicate)
	}
	if in.Candidate.P95LatencyMS > in.Gates.MaxP95LatencyMS {
		addParentFailure(&r, ParentFirstFailureLatency)
	}
	if in.Limits.MaxCandidates > 0 && in.Candidate.CandidateCount > in.Limits.MaxCandidates {
		addParentFailure(&r, ParentFirstFailureCandidateBudget)
	}
	return r, nil
}
func addParentFailure(r *ParentFirstEvaluationReport, f ParentFirstFailureReason) {
	for _, x := range r.HardFailures {
		if x == f {
			return
		}
	}
	r.HardFailures = append(r.HardFailures, f)
	r.Eligible = false
}
func containsParentFirstFailure(g []ParentFirstFailureReason, w ParentFirstFailureReason) bool {
	for _, x := range g {
		if x == w {
			return true
		}
	}
	return false
}
