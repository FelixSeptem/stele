package reasoning

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/FelixSeptem/stele/internal/memory"
)

// ContradictionTemporalDisposition is deliberately separate from temporal
// correction dispositions: it describes how two independent fact versions
// relate, not which version is allowed to become current.
type ContradictionTemporalDisposition string

const (
	ContradictionTemporalOverlap     ContradictionTemporalDisposition = "contradiction"
	ContradictionTemporalCoexistence ContradictionTemporalDisposition = "temporal_coexistence"
	ContradictionTemporalUnresolved  ContradictionTemporalDisposition = "unresolved_temporal"
)

type ContradictionFact struct {
	Scope       memory.Scope
	ID          string
	Version     int64
	Key         string
	ValueDigest string
	Validity    memory.TemporalValidity
	Evidence    memory.DerivedInsightEvidenceRef
	State       memory.MemoryState
}

func (f ContradictionFact) Validate() error {
	if err := f.Scope.Validate(); err != nil {
		return fmt.Errorf("contradiction fact scope: %w", err)
	}
	if strings.TrimSpace(f.ID) == "" {
		return fmt.Errorf("contradiction fact id is required")
	}
	if f.Version <= 0 {
		return fmt.Errorf("contradiction fact version must be greater than zero")
	}
	if !bounded(f.Key, 256) || strings.TrimSpace(f.Key) == "" {
		return fmt.Errorf("contradiction fact key is invalid")
	}
	if !bounded(f.ValueDigest, 256) || strings.TrimSpace(f.ValueDigest) == "" {
		return fmt.Errorf("contradiction fact value digest is invalid")
	}
	if f.State != "" && f.State != memory.MemoryStateActive {
		return fmt.Errorf("contradiction fact state is not lifecycle-visible")
	}
	if f.Validity.IsUnset() {
		// Missing validity is a valid input to detection, but can only produce
		// unresolved_temporal and can never become an active candidate.
		return nil
	}
	if err := f.Validity.Validate(); err != nil {
		return fmt.Errorf("contradiction fact validity: %w", err)
	}
	if err := f.Evidence.Validate(); err != nil {
		return fmt.Errorf("contradiction fact evidence: %w", err)
	}
	return nil
}

type ContradictionDetectionPolicy struct {
	Scope           memory.Scope
	MaxPairs        int
	SourceWatermark string
}

func (p ContradictionDetectionPolicy) Validate() error {
	if err := p.Scope.Validate(); err != nil {
		return fmt.Errorf("contradiction policy scope: %w", err)
	}
	if p.MaxPairs <= 0 || p.MaxPairs > 10000 {
		return fmt.Errorf("contradiction policy max pairs must be between 1 and 10000")
	}
	if !bounded(p.SourceWatermark, 256) {
		return fmt.Errorf("contradiction policy source watermark is invalid")
	}
	return nil
}

type ContradictionCandidate struct {
	Scope               memory.Scope
	Key                 string
	Left                ContradictionFact
	Right               ContradictionFact
	TemporalDisposition ContradictionTemporalDisposition
	OverlapFrom         time.Time
	OverlapTo           *time.Time
	Evidence            []memory.DerivedInsightEvidenceRef
	EvidenceDigest      string
	SourceWatermark     string
	ReplayID            string
	Uncertainty         float64
	ReviewRequired      bool
}

func (c ContradictionCandidate) Validate() error {
	if err := c.Scope.Validate(); err != nil {
		return err
	}
	if strings.TrimSpace(c.Key) == "" || !bounded(c.Key, 256) {
		return fmt.Errorf("contradiction candidate key is invalid")
	}
	if c.Left.Scope.Normalized() != c.Scope.Normalized() || c.Right.Scope.Normalized() != c.Scope.Normalized() {
		return fmt.Errorf("contradiction candidate evidence scope does not match")
	}
	if err := c.Left.Validate(); err != nil {
		return fmt.Errorf("left contradiction fact: %w", err)
	}
	if err := c.Right.Validate(); err != nil {
		return fmt.Errorf("right contradiction fact: %w", err)
	}
	if c.TemporalDisposition != ContradictionTemporalOverlap {
		return fmt.Errorf("contradiction candidate temporal disposition must be overlap")
	}
	if len(c.Evidence) != 2 {
		return fmt.Errorf("contradiction candidate requires two evidence references")
	}
	digest, err := EvidenceDigest(c.Evidence)
	if err != nil {
		return err
	}
	if digest != c.EvidenceDigest {
		return fmt.Errorf("contradiction candidate evidence digest does not match")
	}
	if c.ReplayID == "" {
		return fmt.Errorf("contradiction candidate replay identity is required")
	}
	return nil
}

type ContradictionClassificationDisposition string

const (
	ContradictionClassificationCandidate   ContradictionClassificationDisposition = "candidate"
	ContradictionClassificationQuarantined ContradictionClassificationDisposition = "quarantined"
	ContradictionClassificationRejected    ContradictionClassificationDisposition = "rejected"
)

// ContradictionFailureDisposition is the stable, low-cardinality outcome for
// bounded detection/provider failures. Every disposition is non-authoritative.
type ContradictionFailureDisposition string

const (
	ContradictionFailureStaleWatermark   ContradictionFailureDisposition = "stale_watermark"
	ContradictionFailureSchemaMismatch   ContradictionFailureDisposition = "incompatible_schema"
	ContradictionFailureTimeout          ContradictionFailureDisposition = "provider_timeout"
	ContradictionFailureBudgetExhausted  ContradictionFailureDisposition = "budget_exhausted"
	ContradictionFailureIncompleteSource ContradictionFailureDisposition = "incomplete_provenance"
	ContradictionFailureUnsafeProvider   ContradictionFailureDisposition = "unsafe_provider"
)

func (d ContradictionFailureDisposition) Valid() bool {
	switch d {
	case ContradictionFailureStaleWatermark, ContradictionFailureSchemaMismatch,
		ContradictionFailureTimeout, ContradictionFailureBudgetExhausted,
		ContradictionFailureIncompleteSource, ContradictionFailureUnsafeProvider:
		return true
	default:
		return false
	}
}

// ClassifyContradictionFailure maps bounded execution facts to a stable
// disposition. It intentionally carries no provider error text or source data.
func ClassifyContradictionFailure(ctx context.Context, watermarkFresh, provenanceComplete, schemaCompatible, budgetAvailable, providerSafe bool) ContradictionFailureDisposition {
	if !watermarkFresh {
		return ContradictionFailureStaleWatermark
	}
	if !schemaCompatible {
		return ContradictionFailureSchemaMismatch
	}
	if !provenanceComplete {
		return ContradictionFailureIncompleteSource
	}
	if !budgetAvailable {
		return ContradictionFailureBudgetExhausted
	}
	if ctx != nil {
		select {
		case <-ctx.Done():
			return ContradictionFailureTimeout
		default:
		}
	}
	if !providerSafe {
		return ContradictionFailureUnsafeProvider
	}
	return ""
}

type ContradictionClassification struct {
	Summary           string
	Uncertainty       float64
	DirectActivation  bool
	CanonicalMutation bool
}

type ContradictionClassifier interface {
	ClassifyContradiction(context.Context, ContradictionCandidate) (ContradictionClassification, error)
}

type ContradictionClassificationResult struct {
	Disposition ContradictionClassificationDisposition
	Candidate   ContradictionCandidate
	Reason      string
}

func ClassifyContradiction(ctx context.Context, classifier ContradictionClassifier, candidate ContradictionCandidate) ContradictionClassificationResult {
	result := ContradictionClassificationResult{Disposition: ContradictionClassificationRejected, Candidate: candidate}
	if err := candidate.Validate(); err != nil {
		result.Reason = err.Error()
		return result
	}
	if classifier == nil {
		result.Disposition = ContradictionClassificationQuarantined
		result.Reason = "contradiction classifier is unavailable"
		return result
	}
	classification, err := classifier.ClassifyContradiction(ctx, candidate)
	if err != nil {
		result.Disposition = ContradictionClassificationQuarantined
		result.Reason = err.Error()
		return result
	}
	if classification.DirectActivation || classification.CanonicalMutation {
		result.Disposition = ContradictionClassificationQuarantined
		result.Reason = "provider requested an unsafe mutation"
		return result
	}
	if classification.Uncertainty < 0 || classification.Uncertainty > 1 || !bounded(classification.Summary, 4096) {
		result.Disposition = ContradictionClassificationQuarantined
		result.Reason = "provider classification is invalid"
		return result
	}
	result.Disposition = ContradictionClassificationCandidate
	result.Candidate.Uncertainty = classification.Uncertainty
	return result
}

type ContradictionDetectionResult struct {
	Candidates          []ContradictionCandidate
	Disposition         map[ContradictionTemporalDisposition]int
	PairBudgetExhausted bool
}

type ContradictionFreshnessDisposition string

const (
	ContradictionFreshnessFresh ContradictionFreshnessDisposition = "fresh"
	ContradictionFreshnessStale ContradictionFreshnessDisposition = "stale_evidence"
)

func EvaluateContradictionFreshness(candidate ContradictionCandidate, currentWatermark string, leftVersion, rightVersion int64) ContradictionFreshnessDisposition {
	if strings.TrimSpace(currentWatermark) == "" || currentWatermark != candidate.SourceWatermark || leftVersion != candidate.Left.Version || rightVersion != candidate.Right.Version {
		return ContradictionFreshnessStale
	}
	return ContradictionFreshnessFresh
}

func (c ContradictionCandidate) ToInsightCandidate(providerVersion, schemaVersion, policyVersion string, mode Mode, scopeProof string) (InsightCandidate, error) {
	if err := c.Validate(); err != nil {
		return InsightCandidate{}, err
	}
	if mode != ModeOffline && mode != ModeShadow {
		return InsightCandidate{}, fmt.Errorf("contradiction candidate mode must be offline or shadow")
	}
	if strings.TrimSpace(providerVersion) == "" || strings.TrimSpace(schemaVersion) == "" || strings.TrimSpace(policyVersion) == "" || strings.TrimSpace(scopeProof) == "" {
		return InsightCandidate{}, fmt.Errorf("contradiction candidate compatibility metadata is required")
	}
	title := "Contradictory fact versions detected"
	summary := "Two mutually exclusive fact versions overlap in valid time and require review."
	createdAt := c.Left.Validity.IngestedAt
	if c.Right.Validity.IngestedAt.After(createdAt) {
		createdAt = c.Right.Validity.IngestedAt
	}
	request := InsightDerivationRequest{
		Scope:               c.Scope,
		InsightType:         memory.DerivedInsightTypeContradiction,
		Mode:                mode,
		Evidence:            c.Evidence,
		SourceWatermark:     c.SourceWatermark,
		ScopeProof:          scopeProof,
		LifecycleVisibility: "active_only",
		RedactionPolicy:     "references_only",
		ProviderVersion:     providerVersion,
		SchemaVersion:       schemaVersion,
		PolicyVersion:       policyVersion,
		InputDigest:         c.ReplayID,
		Limits:              DefaultLimits(),
		Now:                 createdAt,
	}
	replayID, err := InsightReplayID(request)
	if err != nil {
		return InsightCandidate{}, err
	}
	candidate := InsightCandidate{
		ID:                  replayID,
		Scope:               c.Scope,
		InsightType:         memory.DerivedInsightTypeContradiction,
		Title:               title,
		Summary:             summary,
		Evidence:            append([]memory.DerivedInsightEvidenceRef(nil), c.Evidence...),
		EvidenceDigest:      c.EvidenceDigest,
		SourceWatermark:     c.SourceWatermark,
		ScopeProof:          scopeProof,
		LifecycleVisibility: "active_only",
		RedactionPolicy:     "references_only",
		ProviderVersion:     providerVersion,
		SchemaVersion:       schemaVersion,
		PolicyVersion:       policyVersion,
		ReplayID:            replayID,
		Uncertainty:         c.Uncertainty,
		Mode:                mode,
		CreatedAt:           createdAt,
		Metadata: map[string]any{
			"contradiction_temporal_disposition": string(c.TemporalDisposition),
			"contradiction_review_state":         "review_required",
			"contradiction_overlap_from":         c.OverlapFrom.UTC().Format(time.RFC3339Nano),
		},
	}
	if c.OverlapTo != nil {
		candidate.Metadata["contradiction_overlap_to"] = c.OverlapTo.UTC().Format(time.RFC3339Nano)
	}
	return candidate, nil
}

func NormalizeContradictionKey(subject, predicate, policyClass string) (string, error) {
	parts := []string{strings.ToLower(strings.TrimSpace(subject)), strings.ToLower(strings.TrimSpace(predicate)), strings.ToLower(strings.TrimSpace(policyClass))}
	for _, part := range parts {
		if part == "" || !bounded(part, 256) {
			return "", fmt.Errorf("contradiction key components are required and bounded")
		}
	}
	seed := strings.Join(parts, "\x00")
	sum := sha256.Sum256([]byte(seed))
	return "contradiction-key:" + hex.EncodeToString(sum[:16]), nil
}

func CompareContradictionTemporalValidity(left, right memory.TemporalValidity) (ContradictionTemporalDisposition, time.Time, *time.Time, error) {
	if err := left.Validate(); err != nil {
		return ContradictionTemporalUnresolved, time.Time{}, nil, nil
	}
	if err := right.Validate(); err != nil {
		return ContradictionTemporalUnresolved, time.Time{}, nil, nil
	}
	if !memory.TemporalIntervalOverlap(left, right) {
		return ContradictionTemporalCoexistence, time.Time{}, nil, nil
	}
	from := left.ValidFrom
	if right.ValidFrom.After(from) {
		from = right.ValidFrom
	}
	var to *time.Time
	switch {
	case left.ValidTo == nil:
		if right.ValidTo != nil {
			v := *right.ValidTo
			to = &v
		}
	case right.ValidTo == nil:
		v := *left.ValidTo
		to = &v
	case left.ValidTo.Before(*right.ValidTo):
		v := *left.ValidTo
		to = &v
	default:
		v := *right.ValidTo
		to = &v
	}
	return ContradictionTemporalOverlap, from, to, nil
}

func DetectContradictions(facts []ContradictionFact, policy ContradictionDetectionPolicy) (ContradictionDetectionResult, error) {
	if err := policy.Validate(); err != nil {
		return ContradictionDetectionResult{}, err
	}
	result := ContradictionDetectionResult{Disposition: map[ContradictionTemporalDisposition]int{}}
	eligible := append([]ContradictionFact(nil), facts...)
	for i := range eligible {
		if err := eligible[i].Validate(); err != nil {
			return ContradictionDetectionResult{}, err
		}
		if eligible[i].Scope.Normalized() != policy.Scope.Normalized() {
			return ContradictionDetectionResult{}, fmt.Errorf("contradiction fact scope does not match policy")
		}
	}
	sort.Slice(eligible, func(i, j int) bool {
		if eligible[i].Key != eligible[j].Key {
			return eligible[i].Key < eligible[j].Key
		}
		if eligible[i].ID != eligible[j].ID {
			return eligible[i].ID < eligible[j].ID
		}
		return eligible[i].Version < eligible[j].Version
	})
	for i := 0; i < len(eligible); i++ {
		for j := i + 1; j < len(eligible); j++ {
			left, right := eligible[i], eligible[j]
			if left.Key != right.Key {
				break
			}
			if left.ID == right.ID && left.Version == right.Version || left.ValueDigest == right.ValueDigest {
				continue
			}
			if len(result.Candidates) >= policy.MaxPairs {
				result.PairBudgetExhausted = true
				return result, nil
			}
			disposition, from, to, err := CompareContradictionTemporalValidity(left.Validity, right.Validity)
			if err != nil {
				return ContradictionDetectionResult{}, err
			}
			result.Disposition[disposition]++
			if disposition != ContradictionTemporalOverlap {
				continue
			}
			evidence := []memory.DerivedInsightEvidenceRef{left.Evidence, right.Evidence}
			digest, err := EvidenceDigest(evidence)
			if err != nil {
				return ContradictionDetectionResult{}, err
			}
			candidate := ContradictionCandidate{Scope: policy.Scope.Normalized(), Key: left.Key, Left: left, Right: right, TemporalDisposition: disposition, OverlapFrom: from, OverlapTo: to, Evidence: evidence, EvidenceDigest: digest, SourceWatermark: policy.SourceWatermark, Uncertainty: 0.5, ReviewRequired: true}
			candidate.ReplayID, err = contradictionReplayID(candidate)
			if err != nil {
				return ContradictionDetectionResult{}, err
			}
			result.Candidates = append(result.Candidates, candidate)
		}
	}
	return result, nil
}

func contradictionReplayID(candidate ContradictionCandidate) (string, error) {
	seed := struct {
		Scope           memory.Scope `json:"scope"`
		Key             string       `json:"key"`
		LeftID          string       `json:"left_id"`
		LeftVersion     int64        `json:"left_version"`
		RightID         string       `json:"right_id"`
		RightVersion    int64        `json:"right_version"`
		EvidenceDigest  string       `json:"evidence_digest"`
		SourceWatermark string       `json:"source_watermark"`
	}{candidate.Scope.Normalized(), candidate.Key, candidate.Left.ID, candidate.Left.Version, candidate.Right.ID, candidate.Right.Version, candidate.EvidenceDigest, candidate.SourceWatermark}
	b, err := json.Marshal(seed)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(b)
	return "contradiction:" + hex.EncodeToString(sum[:16]), nil
}
