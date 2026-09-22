package retrieval

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/FelixSeptem/stele/internal/memory"
)

type GraphTraversalInput struct {
	Scope              memory.Scope
	SeedMemoryIDs      []string
	TemporalConstraint memory.TemporalConstraint
	TimeFrom           time.Time
	TimeTo             time.Time
	TopK               int
	Limits             GraphTraversalLimits
}

func (input GraphTraversalInput) Validate() error {
	if err := input.Scope.Validate(); err != nil {
		return err
	}
	if len(input.SeedMemoryIDs) == 0 || len(input.SeedMemoryIDs) > input.Limits.MaxSeeds {
		return fmt.Errorf("graph traversal seed count is invalid")
	}
	if err := input.TemporalConstraint.Validate(); err != nil && input.TemporalConstraint.Mode != "" {
		return err
	}
	if !input.TimeFrom.IsZero() && !input.TimeTo.IsZero() && input.TimeFrom.After(input.TimeTo) {
		return fmt.Errorf("graph traversal recorded-time window is invalid")
	}
	if input.TopK < 0 {
		return fmt.Errorf("graph traversal top_k cannot be negative")
	}
	return input.Limits.ValidateEffective()
}

type GraphTraversalSearcher interface {
	ExpandGraph(context.Context, GraphTraversalInput) ([]GraphPathCandidate, error)
}

type GraphTraversalOutcomeSearcher interface {
	ExpandGraphResult(context.Context, GraphTraversalInput) (GraphTraversalResult, error)
}

type GraphTraversalTruncation string

const (
	GraphTraversalTruncationNone          GraphTraversalTruncation = "none"
	GraphTraversalTruncationCycle         GraphTraversalTruncation = "cycle"
	GraphTraversalTruncationPerHopBudget  GraphTraversalTruncation = "per_hop_budget"
	GraphTraversalTruncationPerSeedBudget GraphTraversalTruncation = "per_seed_budget"
	GraphTraversalTruncationRequestBudget GraphTraversalTruncation = "request_budget"
	GraphTraversalTruncationCandidate     GraphTraversalTruncation = "candidate_budget"
)

type GraphTraversalResult struct {
	Candidates []GraphPathCandidate
	Truncation GraphTraversalTruncation
	PathsSeen  int
	CyclesSeen int
}

func (result GraphTraversalResult) Normalize() GraphTraversalResult {
	if result.Truncation == "" {
		result.Truncation = GraphTraversalTruncationNone
	}
	if result.PathsSeen < len(result.Candidates) {
		result.PathsSeen = len(result.Candidates)
	}
	if result.CyclesSeen < 0 {
		result.CyclesSeen = 0
	}
	return result
}

// GraphTraversalLimits is the deployment-level hard envelope for optional
// request-time relation traversal. Scope policies may only reduce these
// values; they can never increase them.
type GraphTraversalLimits struct {
	MaxHops            int
	MaxSeeds           int
	MaxEdgesPerHop     int
	MaxPathsPerSeed    int
	MaxPathsPerRequest int
	MaxCandidates      int
	MaxElapsed         time.Duration
}

func DefaultGraphTraversalLimits() GraphTraversalLimits {
	return GraphTraversalLimits{
		MaxHops: 3, MaxSeeds: 32, MaxEdgesPerHop: 64, MaxPathsPerSeed: 32,
		MaxPathsPerRequest: 256, MaxCandidates: 100, MaxElapsed: 250 * time.Millisecond,
	}
}

func (limits GraphTraversalLimits) Validate() error {
	if limits.MaxHops < 1 || limits.MaxHops > 3 {
		return fmt.Errorf("graph traversal max hops must be between 1 and 3")
	}
	return limits.validateNonHopFields()
}

// ValidateEffective validates limits after an exact-scope policy has been
// applied. Effective plans may explicitly disable expansion with zero hops;
// deployment limits themselves remain one through three hops.
func (limits GraphTraversalLimits) ValidateEffective() error {
	if limits.MaxHops < 0 || limits.MaxHops > 3 {
		return fmt.Errorf("effective graph traversal max hops must be between 0 and 3")
	}
	return limits.validateNonHopFields()
}

func (limits GraphTraversalLimits) validateNonHopFields() error {
	if limits.MaxSeeds <= 0 || limits.MaxSeeds > 1000 {
		return fmt.Errorf("graph traversal max seeds must be between 1 and 1000")
	}
	if limits.MaxEdgesPerHop <= 0 || limits.MaxEdgesPerHop > 10000 || limits.MaxPathsPerSeed <= 0 || limits.MaxPathsPerSeed > 1000 || limits.MaxPathsPerRequest <= 0 || limits.MaxPathsPerRequest > 10000 {
		return fmt.Errorf("graph traversal path limits are invalid")
	}
	if limits.MaxCandidates <= 0 || limits.MaxCandidates > 5000 || limits.MaxElapsed <= 0 || limits.MaxElapsed > 30*time.Second {
		return fmt.Errorf("graph traversal candidate or elapsed limit is invalid")
	}
	return nil
}

// EffectiveGraphTraversalLimits applies a scope policy as a monotonic
// narrowing operation over deployment limits.
func EffectiveGraphTraversalLimits(deployment GraphTraversalLimits, policy memory.GraphTraversalPolicy) (GraphTraversalLimits, error) {
	if err := deployment.Validate(); err != nil {
		return GraphTraversalLimits{}, err
	}
	if err := policy.Validate(); err != nil {
		return GraphTraversalLimits{}, err
	}
	result := deployment
	// The deployment cap can be three, but an enabled scope policy executes one
	// hop unless it explicitly narrows the disposition further or selects two
	// or three within that cap.
	if policy.HopsSet {
		if policy.MaxHops > deployment.MaxHops {
			return GraphTraversalLimits{}, fmt.Errorf("graph traversal policy max hops exceeds deployment hard limit")
		}
		result.MaxHops = policy.MaxHops
	} else if policy.MaxHops > 0 {
		result.MaxHops = minGraphPositive(deployment.MaxHops, policy.MaxHops)
	} else {
		result.MaxHops = 1
	}
	if policy.MaxSeeds > 0 {
		result.MaxSeeds = minGraphPositive(result.MaxSeeds, policy.MaxSeeds)
	}
	if policy.MaxEdgesPerHop > 0 {
		result.MaxEdgesPerHop = minGraphPositive(result.MaxEdgesPerHop, policy.MaxEdgesPerHop)
	}
	if policy.MaxPathsPerSeed > 0 {
		result.MaxPathsPerSeed = minGraphPositive(result.MaxPathsPerSeed, policy.MaxPathsPerSeed)
	}
	if policy.MaxPathsPerRequest > 0 {
		result.MaxPathsPerRequest = minGraphPositive(result.MaxPathsPerRequest, policy.MaxPathsPerRequest)
	}
	if policy.MaxCandidates > 0 {
		result.MaxCandidates = minGraphPositive(result.MaxCandidates, policy.MaxCandidates)
	}
	if policy.MaxElapsed > 0 && policy.MaxElapsed < result.MaxElapsed {
		result.MaxElapsed = policy.MaxElapsed
	}
	if err := result.ValidateEffective(); err != nil {
		return GraphTraversalLimits{}, err
	}
	return result, nil
}

func minGraphPositive(left, right int) int {
	if right <= 0 || left < right {
		return left
	}
	return right
}

// GraphPathProof is request-scoped derived evidence. It deliberately contains
// identities and bounded categories, not source text or public path payloads.
type GraphPathProof struct {
	SeedID             string
	EdgeIDs            []string
	SourceVersionIDs   []string
	RelationCategories []string
	Hop                int
	PolicyVersion      string
}

func (proof GraphPathProof) Validate() error {
	if strings.TrimSpace(proof.SeedID) == "" || proof.Hop < 0 || proof.Hop != len(proof.EdgeIDs) {
		return fmt.Errorf("graph path proof identity or hop count is invalid")
	}
	if len(proof.SourceVersionIDs) != len(proof.EdgeIDs) || len(proof.RelationCategories) != len(proof.EdgeIDs) {
		return fmt.Errorf("graph path proof lineage lengths must match edges")
	}
	if len(proof.PolicyVersion) > 64 {
		return fmt.Errorf("graph path policy version is too long")
	}
	return nil
}

func (proof GraphPathProof) StableIdentity() string {
	parts := append([]string{proof.SeedID}, proof.EdgeIDs...)
	return strings.Join(parts, "/")
}

// GraphPathCandidate is the internal endpoint result used before canonical
// candidate fusion. It is never serialized to an ordinary public response.
type GraphPathCandidate struct {
	Memory             memory.CanonicalMemory
	Proof              GraphPathProof
	RelationConfidence float64
	SourceReliability  float64
	Freshness          time.Time
}

func SortGraphPathCandidates(candidates []GraphPathCandidate) {
	sort.SliceStable(candidates, func(i, j int) bool {
		left, right := candidates[i], candidates[j]
		if left.Proof.Hop != right.Proof.Hop {
			return left.Proof.Hop < right.Proof.Hop
		}
		if left.RelationConfidence != right.RelationConfidence {
			return left.RelationConfidence > right.RelationConfidence
		}
		if left.SourceReliability != right.SourceReliability {
			return left.SourceReliability > right.SourceReliability
		}
		if !left.Freshness.Equal(right.Freshness) {
			return left.Freshness.After(right.Freshness)
		}
		if left.Memory.ID != right.Memory.ID {
			return left.Memory.ID < right.Memory.ID
		}
		return left.Proof.StableIdentity() < right.Proof.StableIdentity()
	})
}
