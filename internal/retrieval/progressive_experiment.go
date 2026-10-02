package retrieval

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/FelixSeptem/stele/internal/memory"
)

// ProgressiveExperimentMode describes a non-authoritative evaluation run.
type ProgressiveExperimentMode string

const (
	ProgressiveExperimentModeOffline ProgressiveExperimentMode = "offline"
	ProgressiveExperimentModeShadow  ProgressiveExperimentMode = "shadow"
)

type ProgressiveExperimentVerdict string

const (
	ProgressiveExperimentShadow  ProgressiveExperimentVerdict = "shadow"
	ProgressiveExperimentNonPass ProgressiveExperimentVerdict = "non_pass"
)

type ProgressiveExperimentLimits struct {
	MaxCandidates int `json:"max_candidates"`
	MaxTokens     int `json:"max_tokens"`
	MaxCharacters int `json:"max_characters"`
}

type ProgressiveExperimentPolicy struct {
	LevelPolicyVersion string                      `json:"level_policy_version"`
	RendererVersion    string                      `json:"renderer_version"`
	BaselineIdentity   string                      `json:"baseline_identity"`
	StrategyIdentity   string                      `json:"strategy_identity"`
	Mode               ProgressiveExperimentMode   `json:"mode"`
	MaxAge             time.Duration               `json:"max_age"`
	Limits             ProgressiveExperimentLimits `json:"limits"`
}

func (p ProgressiveExperimentPolicy) Validate() error {
	if !evaluationSafeIdentity(p.LevelPolicyVersion) || !evaluationSafeIdentity(p.RendererVersion) ||
		!evaluationSafeIdentity(p.BaselineIdentity) || !evaluationSafeIdentity(p.StrategyIdentity) {
		return fmt.Errorf("experiment policy identities are invalid")
	}
	if p.Mode != ProgressiveExperimentModeOffline && p.Mode != ProgressiveExperimentModeShadow {
		return fmt.Errorf("experiment mode must be offline or shadow")
	}
	if p.Limits.MaxCandidates < 0 || p.Limits.MaxCandidates > 1000 || p.Limits.MaxTokens < 0 || p.Limits.MaxTokens > 100000 || p.Limits.MaxCharacters < 0 || p.Limits.MaxCharacters > 1000000 {
		return fmt.Errorf("experiment limits are invalid")
	}
	return nil
}

type ProgressiveExperimentInput struct {
	Policy      ProgressiveExperimentPolicy
	Scope       memory.Scope
	EvaluatedAt time.Time
	Levels      []ProgressiveContextLevelInput
	ParentFirst ParentFirstEvaluationInput
}

// ReplayProgressiveExperiment runs the baseline-preserving evaluation under a
// fixed clock, scope, and policy envelope. The underlying level and parent-first
// evaluators only return aggregates; ordinary retrieval remains untouched.
func ReplayProgressiveExperiment(in ProgressiveExperimentInput) (ProgressiveExperimentReport, error) {
	return BuildProgressiveExperimentReport(in)
}

// ProgressiveExperimentLevelSummary deliberately contains aggregates only.
// Raw evidence, source content, scope values, and identifiers never cross the
// experiment report boundary.
type ProgressiveExperimentLevelSummary struct {
	Kind              ProgressiveContextLevelKind       `json:"kind"`
	Eligible          bool                              `json:"eligible"`
	Freshness         ProgressiveContextFreshness       `json:"freshness"`
	CharacterCount    int                               `json:"character_count"`
	TokenCount        int                               `json:"token_count"`
	CitationCoverage  float64                           `json:"citation_coverage"`
	EvidenceCoverage  float64                           `json:"evidence_coverage"`
	FailureCategories []ProgressiveContextFailureReason `json:"failure_categories,omitempty"`
}

type ProgressiveExperimentReport struct {
	RunIdentity         string                              `json:"run_identity"`
	PolicyIdentity      string                              `json:"policy_identity"`
	ScopeHash           string                              `json:"scope_hash"`
	BaselineIdentity    string                              `json:"baseline_identity"`
	StrategyIdentity    string                              `json:"strategy_identity"`
	Mode                ProgressiveExperimentMode           `json:"mode"`
	SelectedStrategy    string                              `json:"selected_strategy"`
	Eligible            bool                                `json:"eligible"`
	Verdict             ProgressiveExperimentVerdict        `json:"verdict"`
	FallbackCategory    string                              `json:"fallback_category,omitempty"`
	RollbackRequired    bool                                `json:"rollback_required"`
	Levels              []ProgressiveExperimentLevelSummary `json:"levels"`
	ParentFirstEligible bool                                `json:"parent_first_eligible"`
	ParentFirstFailures []ParentFirstFailureReason          `json:"parent_first_failures,omitempty"`
}

func BuildProgressiveExperimentReport(in ProgressiveExperimentInput) (ProgressiveExperimentReport, error) {
	if err := in.Scope.Validate(); err != nil {
		return ProgressiveExperimentReport{}, err
	}
	if err := in.Policy.Validate(); err != nil {
		return ProgressiveExperimentReport{}, err
	}
	now := in.EvaluatedAt.UTC()
	if now.IsZero() {
		now = time.Now().UTC()
	}
	levelInputs := make([]ProgressiveContextLevelInput, len(in.Levels))
	for index, level := range in.Levels {
		levelInputs[index] = level
		if levelInputs[index].MaxAge <= 0 {
			levelInputs[index].MaxAge = in.Policy.MaxAge
		}
		if in.Policy.Limits.MaxTokens > 0 && (levelInputs[index].TokenBudget <= 0 || levelInputs[index].TokenBudget > in.Policy.Limits.MaxTokens) {
			levelInputs[index].TokenBudget = in.Policy.Limits.MaxTokens
		}
		if in.Policy.Limits.MaxCharacters > 0 && (levelInputs[index].CharacterBudget <= 0 || levelInputs[index].CharacterBudget > in.Policy.Limits.MaxCharacters) {
			levelInputs[index].CharacterBudget = in.Policy.Limits.MaxCharacters
		}
	}
	levels, err := EvaluateProgressiveContext(ProgressiveContextEvaluationInput{Scope: in.Scope, EvaluatedAt: now, BaselineIdentity: in.Policy.BaselineIdentity, Levels: levelInputs})
	if err != nil {
		return ProgressiveExperimentReport{}, err
	}
	parent := in.ParentFirst
	if strings.TrimSpace(parent.StrategyIdentity) == "" {
		parent = ParentFirstEvaluationInput{Scope: in.Scope, StrategyIdentity: in.Policy.StrategyIdentity, Mode: ParentFirstModeDisabled}
	}
	if parent.Scope == (memory.Scope{}) {
		parent.Scope = in.Scope
	}
	if in.Policy.Limits.MaxCandidates > 0 && (parent.Limits.MaxCandidates <= 0 || parent.Limits.MaxCandidates > in.Policy.Limits.MaxCandidates) {
		parent.Limits.MaxCandidates = in.Policy.Limits.MaxCandidates
	}
	parentReport, err := EvaluateParentFirst(parent)
	if err != nil {
		return ProgressiveExperimentReport{}, err
	}
	scopeDigest := scopeHash(in.Scope)
	policyDigest := progressiveExperimentPolicyIdentity(in.Policy)
	r := ProgressiveExperimentReport{
		PolicyIdentity: policyDigest, ScopeHash: scopeDigest, BaselineIdentity: in.Policy.BaselineIdentity,
		StrategyIdentity: in.Policy.StrategyIdentity, Mode: in.Policy.Mode,
		SelectedStrategy: ParentFirstFlatStrategy, Eligible: false, Verdict: ProgressiveExperimentShadow,
		Levels: make([]ProgressiveExperimentLevelSummary, 0, len(levels.Levels)), ParentFirstEligible: parentReport.Eligible,
		ParentFirstFailures: append([]ParentFirstFailureReason(nil), parentReport.HardFailures...),
	}
	for _, level := range levels.Levels {
		r.Levels = append(r.Levels, ProgressiveExperimentLevelSummary{Kind: level.Kind, Eligible: level.Eligible, Freshness: level.Freshness, CharacterCount: level.CharacterCount, TokenCount: level.TokenCount, CitationCoverage: level.CitationCoverage, EvidenceCoverage: level.EvidenceCoverage, FailureCategories: append([]ProgressiveContextFailureReason(nil), level.FailureReasons...)})
		if !level.Eligible {
			r.Eligible = false
			r.Verdict = ProgressiveExperimentNonPass
			r.FallbackCategory = "level_safety_failure"
			r.RollbackRequired = true
		}
	}
	if !parentReport.Eligible {
		r.Eligible = false
		r.Verdict = ProgressiveExperimentNonPass
		r.FallbackCategory = "parent_first_safety_failure"
		r.RollbackRequired = true
	}
	r.RunIdentity = progressiveExperimentRunIdentity(r, now)
	return r, nil
}

func progressiveExperimentPolicyIdentity(policy ProgressiveExperimentPolicy) string {
	b, _ := json.Marshal(policy)
	digest := sha256.Sum256(b)
	return "policy:" + hex.EncodeToString(digest[:])
}

func progressiveExperimentRunIdentity(report ProgressiveExperimentReport, evaluatedAt time.Time) string {
	identity := struct {
		Policy, Scope, Baseline, Strategy, Mode, Verdict string
		Levels                                           []ProgressiveExperimentLevelSummary
	}{report.PolicyIdentity, report.ScopeHash, report.BaselineIdentity, report.StrategyIdentity, string(report.Mode), string(report.Verdict), report.Levels}
	b, _ := json.Marshal(identity)
	digest := sha256.Sum256(append(b, evaluatedAt.UTC().Format(time.RFC3339Nano)...))
	return "experiment:" + hex.EncodeToString(digest[:])
}

// MarshalProgressiveExperimentReport validates and emits the redacted report
// boundary. The returned JSON cannot contain source content or scope values.
func MarshalProgressiveExperimentReport(report ProgressiveExperimentReport) ([]byte, error) {
	if !strings.HasPrefix(report.RunIdentity, "experiment:") || len(report.RunIdentity) != len("experiment:")+64 ||
		!strings.HasPrefix(report.PolicyIdentity, "policy:") || len(report.PolicyIdentity) != len("policy:")+64 ||
		!strings.HasPrefix(report.ScopeHash, "scope:") || len(report.ScopeHash) != len("scope:")+64 {
		return nil, fmt.Errorf("experiment report identities are invalid")
	}
	return json.Marshal(report)
}
