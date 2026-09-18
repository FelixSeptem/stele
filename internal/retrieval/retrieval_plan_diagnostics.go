package retrieval

import (
	"encoding/json"
	"fmt"
	"sort"
	"time"

	"github.com/FelixSeptem/stele/internal/memory"
)

const retrievalPlannerDiagnosticFailureInvalidAggregate = "validation_failed"

type RetrievalPlannerDiagnostics struct {
	PlannerVersion      RetrievalPlannerVersion               `json:"planner_version"`
	PolicyVersion       RetrievalPlanPolicyVersion            `json:"policy_version"`
	QueryFamily         RetrievalQueryFamily                  `json:"query_family"`
	RolloutStage        memory.RetrievalPlannerRolloutStage   `json:"rollout_stage"`
	Disposition         RetrievalPlanDisposition              `json:"disposition"`
	Fallback            RetrievalPlanFallback                 `json:"fallback"`
	EnabledChannels     []FusionChannel                       `json:"enabled_channels"`
	PassCount           int                                   `json:"pass_count"`
	CandidateBucket     string                                `json:"candidate_bucket"`
	EvidenceDisposition EvidenceDisposition                   `json:"evidence_disposition"`
	VisibleBucket       string                                `json:"visible_bucket"`
	AttritionBucket     string                                `json:"attrition_bucket"`
	LatencyBucket       string                                `json:"latency_bucket"`
	RerankerEligibility string                                `json:"reranker_eligibility"`
	ChannelAvailability []RetrievalPlannerChannelAvailability `json:"channel_availability"`
	ChangedRankCount    int                                   `json:"changed_rank_count"`
	ChangedRankBucket   string                                `json:"changed_rank_bucket"`
}

type RetrievalPlannerChannelAvailability struct {
	Channel      FusionChannel `json:"channel"`
	Availability string        `json:"availability"`
}

type RetrievalPlannerDiagnosticsInput struct {
	Plan                RetrievalPlan
	RolloutStage        memory.RetrievalPlannerRolloutStage
	Evidence            EvidenceAssessment
	PassCount           int
	CandidateCount      int
	Elapsed             time.Duration
	ChannelAvailability []RetrievalPlannerChannelAvailability
	ChangedRankCount    int
	ChangedRankObserved bool
}

func RetrievalPlannerDiagnosticsFromExecution(input RetrievalPlannerDiagnosticsInput) (RetrievalPlannerDiagnostics, error) {
	if err := input.Plan.Validate(DefaultRetrievalPlanHardLimits()); err != nil {
		return RetrievalPlannerDiagnostics{}, err
	}
	if !validPlannerDiagnosticStage(input.RolloutStage) {
		return RetrievalPlannerDiagnostics{}, fmt.Errorf("invalid retrieval planner rollout stage")
	}
	if input.PassCount < 1 || input.PassCount > 2 {
		return RetrievalPlannerDiagnostics{}, fmt.Errorf("retrieval planner diagnostic pass count must be one or two")
	}
	if input.CandidateCount < 0 || input.CandidateCount > 5000 {
		return RetrievalPlannerDiagnostics{}, fmt.Errorf("retrieval planner diagnostic candidate count is outside its bound")
	}
	if input.Elapsed < 0 || input.Elapsed > 30*time.Second {
		return RetrievalPlannerDiagnostics{}, fmt.Errorf("retrieval planner diagnostic elapsed is outside its bound")
	}
	diagnostics := RetrievalPlannerDiagnostics{
		PlannerVersion:      input.Plan.Identity.PlannerVersion,
		PolicyVersion:       input.Plan.Identity.PolicyVersion,
		QueryFamily:         input.Plan.Family,
		RolloutStage:        input.RolloutStage,
		Disposition:         input.Plan.Disposition,
		Fallback:            input.Plan.Fallback,
		EnabledChannels:     append([]FusionChannel(nil), input.Plan.Channels...),
		PassCount:           input.PassCount,
		CandidateBucket:     plannerCandidateBucket(input.CandidateCount),
		EvidenceDisposition: input.Evidence.Disposition,
		VisibleBucket:       input.Evidence.VisibleBucket,
		AttritionBucket:     input.Evidence.AttritionBucket,
		LatencyBucket:       plannerLatencyBucket(input.Elapsed),
		RerankerEligibility: "ineligible",
		ChannelAvailability: append([]RetrievalPlannerChannelAvailability(nil), input.ChannelAvailability...),
		ChangedRankCount:    input.ChangedRankCount,
		ChangedRankBucket:   "not_evaluated",
	}
	if len(diagnostics.ChannelAvailability) == 0 {
		for _, channel := range input.Plan.Channels {
			diagnostics.ChannelAvailability = append(diagnostics.ChannelAvailability, RetrievalPlannerChannelAvailability{Channel: channel, Availability: "not_evaluated"})
		}
	}
	sort.Slice(diagnostics.ChannelAvailability, func(i, j int) bool {
		return diagnostics.ChannelAvailability[i].Channel < diagnostics.ChannelAvailability[j].Channel
	})
	if input.ChangedRankObserved {
		diagnostics.ChangedRankBucket = plannerChangedRankBucket(input.ChangedRankCount)
	}
	if input.Plan.RerankerEligible && input.Plan.RerankerHeadroom > 0 {
		diagnostics.RerankerEligibility = "eligible"
	}
	if err := diagnostics.Validate(); err != nil {
		return RetrievalPlannerDiagnostics{}, err
	}
	return diagnostics, nil
}

func buildRetrievalPlannerDiagnostics(input RetrievalPlannerDiagnosticsInput) (RetrievalPlannerDiagnostics, string) {
	diagnostics, err := RetrievalPlannerDiagnosticsFromExecution(input)
	if err != nil {
		return RetrievalPlannerDiagnostics{}, retrievalPlannerDiagnosticFailureInvalidAggregate
	}
	return diagnostics, ""
}

func plannerDiagnosticElapsed(startedAt, observedAt time.Time) time.Duration {
	if startedAt.IsZero() || observedAt.Before(startedAt) {
		return 0
	}
	elapsed := observedAt.Sub(startedAt)
	if elapsed > 30*time.Second {
		return 30 * time.Second
	}
	return elapsed
}

func (diagnostics RetrievalPlannerDiagnostics) Validate() error {
	if diagnostics.PlannerVersion != RetrievalPlannerVersionV1 || diagnostics.PolicyVersion != RetrievalPlanPolicyVersionV1 || !diagnostics.QueryFamily.valid() {
		return fmt.Errorf("invalid retrieval planner diagnostic identity")
	}
	if !validPlannerDiagnosticStage(diagnostics.RolloutStage) || (diagnostics.Disposition != RetrievalPlanDispositionPlanned && diagnostics.Disposition != RetrievalPlanDispositionFallback) || diagnostics.Fallback != RetrievalPlanFallbackBaseline {
		return fmt.Errorf("invalid retrieval planner diagnostic rollout values")
	}
	if len(diagnostics.EnabledChannels) == 0 || len(diagnostics.EnabledChannels) > 4 {
		return fmt.Errorf("invalid retrieval planner diagnostic channels")
	}
	for i, channel := range diagnostics.EnabledChannels {
		if !channel.valid() || (i > 0 && diagnostics.EnabledChannels[i-1] >= channel) {
			return fmt.Errorf("retrieval planner diagnostic channels must be unique and sorted")
		}
	}
	if diagnostics.PassCount < 1 || diagnostics.PassCount > 2 || !validPlannerBucket(diagnostics.CandidateBucket, []string{"0", "1_10", "11_50", "51_plus"}) || !validPlannerBucket(diagnostics.VisibleBucket, []string{"zero", "one_to_five", "six_to_twenty", "over_twenty"}) || !validPlannerBucket(diagnostics.AttritionBucket, []string{"none", "low", "high"}) || !validPlannerBucket(diagnostics.LatencyBucket, []string{"lt_100ms", "100ms_500ms", "500ms_1s", "gt_1s"}) {
		return fmt.Errorf("invalid retrieval planner diagnostic aggregate")
	}
	if !validEvidenceDisposition(diagnostics.EvidenceDisposition) {
		return fmt.Errorf("invalid retrieval planner evidence disposition")
	}
	if diagnostics.RerankerEligibility != "eligible" && diagnostics.RerankerEligibility != "ineligible" {
		return fmt.Errorf("invalid retrieval planner reranker eligibility")
	}
	if len(diagnostics.ChannelAvailability) != len(diagnostics.EnabledChannels) {
		return fmt.Errorf("retrieval planner diagnostic channel availability must match enabled channels")
	}
	for i, observation := range diagnostics.ChannelAvailability {
		if observation.Channel != diagnostics.EnabledChannels[i] || !validPlannerBucket(observation.Availability, []string{"available", "unavailable", "not_evaluated"}) {
			return fmt.Errorf("invalid retrieval planner diagnostic channel availability")
		}
	}
	if diagnostics.ChangedRankCount < 0 || diagnostics.ChangedRankCount > 5000 || !validPlannerBucket(diagnostics.ChangedRankBucket, []string{"0", "1_5", "6_20", "21_plus", "not_evaluated"}) {
		return fmt.Errorf("invalid retrieval planner changed-rank observation")
	}
	if diagnostics.ChangedRankBucket == "not_evaluated" && diagnostics.ChangedRankCount != 0 {
		return fmt.Errorf("unobserved retrieval planner comparison cannot have changed ranks")
	}
	return nil
}

func MarshalRetrievalPlannerDiagnostics(diagnostics RetrievalPlannerDiagnostics) ([]byte, error) {
	if err := diagnostics.Validate(); err != nil {
		return nil, err
	}
	stable := diagnostics
	stable.EnabledChannels = append([]FusionChannel(nil), diagnostics.EnabledChannels...)
	stable.ChannelAvailability = append([]RetrievalPlannerChannelAvailability(nil), diagnostics.ChannelAvailability...)
	sort.Slice(stable.EnabledChannels, func(i, j int) bool { return stable.EnabledChannels[i] < stable.EnabledChannels[j] })
	sort.Slice(stable.ChannelAvailability, func(i, j int) bool {
		return stable.ChannelAvailability[i].Channel < stable.ChannelAvailability[j].Channel
	})
	return json.Marshal(stable)
}

func plannerChangedRankBucket(count int) string {
	switch {
	case count <= 0:
		return "0"
	case count <= 5:
		return "1_5"
	case count <= 20:
		return "6_20"
	default:
		return "21_plus"
	}
}

func validPlannerDiagnosticStage(stage memory.RetrievalPlannerRolloutStage) bool {
	switch stage {
	case memory.RetrievalPlannerRolloutStageBaseline, memory.RetrievalPlannerRolloutStageDiagnosticsOnly, memory.RetrievalPlannerRolloutStageShadow, memory.RetrievalPlannerRolloutStageActive:
		return true
	default:
		return false
	}
}

func validEvidenceDisposition(value EvidenceDisposition) bool {
	switch value {
	case EvidenceDispositionSufficient, EvidenceDispositionZeroHits, EvidenceDispositionBelowMinimum, EvidenceDispositionHighAttrition, EvidenceDispositionChannelUnavailable, EvidenceDispositionNoHeadroom, EvidenceDispositionTerminalIncomplete:
		return true
	default:
		return false
	}
}

func validPlannerBucket(value string, allowed []string) bool {
	for _, candidate := range allowed {
		if value == candidate {
			return true
		}
	}
	return false
}

func plannerCandidateBucket(count int) string {
	switch {
	case count <= 0:
		return "0"
	case count <= 10:
		return "1_10"
	case count <= 50:
		return "11_50"
	default:
		return "51_plus"
	}
}

func plannerLatencyBucket(elapsed time.Duration) string {
	switch {
	case elapsed < 100*time.Millisecond:
		return "lt_100ms"
	case elapsed < 500*time.Millisecond:
		return "100ms_500ms"
	case elapsed < time.Second:
		return "500ms_1s"
	default:
		return "gt_1s"
	}
}
