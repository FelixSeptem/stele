package evaluation

import (
	"crypto/sha256"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/FelixSeptem/stele/internal/memory"
)

var logicalIdentityPattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._:-]{0,127}$`)

type CompatibilityIdentity struct {
	FixtureVersion  string `json:"fixture_version"`
	PolicyVersion   string `json:"policy_version"`
	Strategy        string `json:"strategy"`
	Renderer        string `json:"renderer"`
	Provider        string `json:"provider"`
	SourceWatermark string `json:"source_watermark"`
}

func (i CompatibilityIdentity) Validate() error {
	for name, value := range map[string]string{
		"fixture version":  i.FixtureVersion,
		"policy version":   i.PolicyVersion,
		"strategy":         i.Strategy,
		"renderer":         i.Renderer,
		"provider":         i.Provider,
		"source watermark": i.SourceWatermark,
	} {
		value = strings.TrimSpace(value)
		if value == "" || !logicalIdentityPattern.MatchString(value) {
			return fmt.Errorf("%s is invalid", name)
		}
	}
	return nil
}

type TrajectoryInput struct {
	Scope          memory.Scope
	Identity       CompatibilityIdentity
	Channel        string
	CandidateCount int
	ExpansionCount int
	Disposition    string
	Fallback       string
	Freshness      string
	Budget         int
	Latency        time.Duration
}

type TrajectoryAggregate struct {
	ScopeHash       string                `json:"scope_hash"`
	Identity        CompatibilityIdentity `json:"identity"`
	Channel         string                `json:"channel"`
	CandidateBucket string                `json:"candidate_bucket"`
	ExpansionBucket string                `json:"expansion_bucket"`
	Disposition     string                `json:"disposition"`
	Fallback        string                `json:"fallback"`
	Freshness       string                `json:"freshness"`
	BudgetBucket    string                `json:"budget_bucket"`
	LatencyBucket   string                `json:"latency_bucket"`
	Count           int                   `json:"count"`
}

func AggregateTrajectory(input TrajectoryInput) (TrajectoryAggregate, error) {
	if err := input.Scope.Validate(); err != nil {
		return TrajectoryAggregate{}, fmt.Errorf("scope is required: %w", err)
	}
	if err := input.Identity.Validate(); err != nil {
		return TrajectoryAggregate{}, err
	}
	if input.CandidateCount < 0 || input.ExpansionCount < 0 || input.Budget < 0 || input.Latency < 0 {
		return TrajectoryAggregate{}, fmt.Errorf("trajectory counts and latency must not be negative")
	}
	return TrajectoryAggregate{
		ScopeHash:       hashScope(input.Scope),
		Identity:        input.Identity,
		Channel:         normalizeCategory(input.Channel, "lexical", "semantic", "relation", "chunk", "unknown"),
		CandidateBucket: bucket(input.CandidateCount, []int{0, 10, 50}),
		ExpansionBucket: bucket(input.ExpansionCount, []int{0, 5, 20}),
		Disposition:     normalizeCategory(input.Disposition, "selected", "fallback", "filtered", "rejected", "unknown"),
		Fallback:        normalizeFallback(input.Fallback),
		Freshness:       normalizeCategory(input.Freshness, "fresh", "stale", "missing", "unknown"),
		BudgetBucket:    budgetBucket(input.Budget),
		LatencyBucket:   latencyBucket(input.Latency),
		Count:           1,
	}, nil
}

func hashScope(scope memory.Scope) string {
	normalized := scope.Normalized()
	sum := sha256.Sum256([]byte(normalized.Tenant + "\x00" + normalized.Project + "\x00" + normalized.Namespace))
	return fmt.Sprintf("scope:%x", sum[:])
}

func normalizeCategory(value string, allowed ...string) string {
	value = strings.TrimSpace(value)
	for _, candidate := range allowed {
		if value == candidate {
			return candidate
		}
	}
	return "unknown"
}

func normalizeFallback(value string) string {
	value = strings.TrimSpace(value)
	if value == "" || value == "none" {
		return "none"
	}
	switch value {
	case "unavailable", "timeout", "invalid", "not_evaluated":
		return value
	default:
		return "other"
	}
}

func bucket(value int, bounds []int) string {
	if value <= bounds[0] {
		return "0"
	}
	if value <= bounds[1] {
		return fmt.Sprintf("1_%d", bounds[1])
	}
	if value <= bounds[2] {
		return fmt.Sprintf("%d_%d", bounds[1]+1, bounds[2])
	}
	return fmt.Sprintf("%d_plus", bounds[2]+1)
}

func budgetBucket(value int) string {
	switch {
	case value <= 0:
		return "0"
	case value <= 1000:
		return "1_1k"
	case value <= 5000:
		return "1k_5k"
	default:
		return "5k_plus"
	}
}

func latencyBucket(value time.Duration) string {
	switch {
	case value <= time.Second:
		return "lt_1s"
	case value <= 5*time.Second:
		return "1_5s"
	case value <= 30*time.Second:
		return "5_30s"
	default:
		return "30s_plus"
	}
}
