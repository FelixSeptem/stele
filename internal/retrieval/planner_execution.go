package retrieval

import (
	"context"
	"errors"
	"time"

	"github.com/FelixSeptem/stele/internal/memory"
	"github.com/jackc/pgx/v5"
)

type retrievalPlannerExecution struct {
	stage         memory.RetrievalPlannerRolloutStage
	plan          *RetrievalPlan
	rolloutPolicy *memory.RankingRolloutPolicy
	ledger        *RetrievalBudgetLedger
	startedAt     time.Time
	deadline      time.Time
}

func (execution retrievalPlannerExecution) active() bool {
	return execution.stage == memory.RetrievalPlannerRolloutStageActive && execution.plan != nil && execution.ledger != nil
}

func (execution retrievalPlannerExecution) shadow() bool {
	return execution.stage == memory.RetrievalPlannerRolloutStageShadow && execution.plan != nil && execution.ledger != nil
}

func (execution retrievalPlannerExecution) planned() bool {
	return (execution.active() || execution.shadow()) && !execution.deadline.IsZero()
}

func (execution retrievalPlannerExecution) callContext(parent context.Context) (context.Context, context.CancelFunc, bool) {
	if !execution.planned() || !time.Now().Before(execution.deadline) {
		return nil, nil, false
	}
	ctx, cancel := context.WithDeadline(parent, execution.deadline)
	return ctx, cancel, true
}

func (execution retrievalPlannerExecution) channelEnabled(channel FusionChannel) bool {
	if !execution.active() {
		return true
	}
	return execution.declaresChannel(channel) || execution.fallbackChannelAllocation(channel) > 0
}

func (execution retrievalPlannerExecution) declaresChannel(channel FusionChannel) bool {
	if execution.plan == nil {
		return false
	}
	for _, planned := range execution.plan.Channels {
		if planned == channel {
			return true
		}
	}
	return false
}

func (execution retrievalPlannerExecution) fallbackChannelAllocation(channel FusionChannel) int {
	if execution.plan == nil {
		return 0
	}
	return execution.plan.FallbackChannelCandidates[channel]
}

func (execution retrievalPlannerExecution) reserveChannelRequest(pass int, channel FusionChannel, input SearchInput) (SearchInput, bool, error) {
	if execution.ledger == nil {
		return input, true, nil
	}
	available := execution.ledger.UnusedChannelAllocation(channel)
	if pass == 1 && execution.active() {
		available = maxInt(available-execution.followUpChannelReservation(channel), 0)
	}
	available = minInt(available, execution.ledger.RemainingForRetrieval())
	if available <= 0 {
		return input, false, nil
	}
	input.TopK = boundedChannelTopK(input.TopK, available)
	if err := execution.ledger.ReserveChannelRequest(pass, channel, input.TopK, time.Now().UTC()); err != nil {
		return input, false, err
	}
	return input, true, nil
}

func (execution retrievalPlannerExecution) reserveBaselineRequest(channel FusionChannel, input SearchInput) (SearchInput, bool, error) {
	if execution.ledger == nil {
		return input, true, nil
	}
	available := execution.ledger.RemainingForBaselineChannel(channel)
	if available <= 0 {
		return input, false, nil
	}
	input.TopK = boundedChannelTopK(input.TopK, available)
	if err := execution.ledger.ReserveBaseline(channel, input.TopK, time.Now().UTC()); err != nil {
		return input, false, err
	}
	return input, true, nil
}

func (execution retrievalPlannerExecution) followUpChannelReservation(channel FusionChannel) int {
	if execution.plan == nil || !execution.plan.FollowUp.Enabled {
		return 0
	}
	remaining := execution.plan.FollowUp.CandidateAllocation
	for _, candidate := range execution.plan.FollowUp.Channels {
		reservation := minInt(execution.plan.ChannelCandidates[candidate], remaining)
		if candidate == channel {
			return reservation
		}
		remaining -= reservation
		if remaining <= 0 {
			break
		}
	}
	return 0
}

func (s *Service) resolveRetrievalPlannerExecution(ctx context.Context, input SearchInput, surface memory.RankingRolloutSurface, analysis QueryAnalysisResult, now time.Time) retrievalPlannerExecution {
	baseline := retrievalPlannerExecution{stage: memory.RetrievalPlannerRolloutStageBaseline}
	reader, ok := s.rankingRolloutPolicyReader.(effectiveRetrievalPlannerPolicyReader)
	if !ok || reader == nil {
		return baseline
	}
	rollout, err := reader.ReadEffectiveRetrievalPlannerRolloutPolicy(ctx, memory.ReadEffectiveRetrievalPlannerRolloutPolicyInput{Scope: input.Scope, Surface: surface, SessionID: input.SessionID, UserID: input.UserID})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return baseline
		}
		return baseline
	}
	resolution := memory.ResolveRetrievalPlannerRollout(&rollout, memory.ResolveRetrievalPlannerRolloutInput{
		Scope: input.Scope, Surface: surface, SessionID: input.SessionID, UserID: input.UserID, Now: now,
		AnalysisPolicyVersion: string(QueryAnalysisPolicyVersionV1), FusionVersion: s.fusionStrategy.Version,
		RankingVersion: retrievalPlannerRankingVersion, RendererVersion: retrievalPlannerRendererVersion,
	})
	if resolution.Stage == memory.RetrievalPlannerRolloutStageBaseline || resolution.Policy == nil {
		return baseline
	}
	rolloutCopy := rollout
	execution := retrievalPlannerExecution{stage: resolution.Stage, rolloutPolicy: &rolloutCopy}
	policy := cloneRetrievalPlanPolicy(s.retrievalPlanPolicy)
	policy.HardLimits = intersectRetrievalPlanLimits(policy.HardLimits, *resolution.Policy)
	if err := policy.Validate(); err != nil {
		return baseline
	}
	plan, err := BuildRetrievalPlan(RetrievalPlanInput{AcceptedQuery: input.Query, Analysis: analysis, EmbeddingAvailable: len(input.QueryEmbedding) > 0, Policy: policy, Now: now})
	if err != nil {
		return baseline
	}
	if plan.Fusion.Version != resolution.Policy.FusionVersion || followUpUsesUnsupportedChannel(plan.FollowUp) {
		return baseline
	}
	ledger, err := NewRetrievalBudgetLedger(RetrievalBudgetEnvelope{TotalCandidates: plan.TotalCandidates, ChannelCandidates: plan.ChannelCandidates, FallbackChannelCandidates: plan.FallbackChannelCandidates, RerankerHeadroom: plan.RerankerHeadroom, MaxPasses: plan.MaxPasses, LatencyBudget: plan.LatencyBudget}, now)
	if err != nil {
		return baseline
	}
	execution.plan, execution.ledger, execution.startedAt, execution.deadline = &plan, ledger, now, now.Add(plan.LatencyBudget)
	if !resolution.AffectsResults && resolution.Stage != memory.RetrievalPlannerRolloutStageShadow {
		execution.ledger = nil
		execution.deadline = time.Time{}
	}
	return execution
}

func followUpUsesUnsupportedChannel(rule RetrievalPlanFollowUpRule) bool {
	if !rule.Enabled {
		return false
	}
	for _, channel := range rule.Channels {
		if channel == FusionChannelChunk {
			return true
		}
	}
	return false
}

func originalQueryAnalysis(input SearchInput) QueryAnalysisResult {
	limits := DefaultQueryAnalysisLimits()
	analysisInput := QueryAnalysisInput{
		AcceptedQuery: input.Query,
		PolicyVersion: QueryAnalysisPolicyVersionV1,
		Limits:        limits,
		Constraints: QueryAnalysisConstraints{
			Classes:  append([]memory.MemoryClass(nil), input.Classes...),
			TimeFrom: input.TimeFrom,
			TimeTo:   input.TimeTo,
		},
	}
	result, err := NewQueryAnalysisResult(analysisInput, QueryAnalysisDispositionOriginalOnly, nil, nil)
	if err != nil {
		return QueryAnalysisResult{
			Identity:    QueryAnalysisIdentity{PolicyVersion: QueryAnalysisPolicyVersionV1, LimitsVersion: QueryAnalysisLimitsVersionV1},
			Disposition: QueryAnalysisDispositionOriginalOnly,
			Signals:     []QueryAnalysisSignal{{Kind: QueryAnalysisSignalOriginal, Text: input.Query, Mandatory: true}},
		}
	}
	return result
}

func boundedChannelTopK(current, limit int) int {
	if limit <= 0 {
		return current
	}
	if current <= 0 || current > limit {
		return limit
	}
	return current
}

func intersectRetrievalPlanLimits(limits RetrievalPlanHardLimits, rollout memory.RetrievalPlannerRolloutPolicy) RetrievalPlanHardLimits {
	limits.MaxCandidates = minPositive(limits.MaxCandidates, rollout.MaxCandidates)
	limits.MaxCandidatesPerChannel = minPositive(limits.MaxCandidatesPerChannel, rollout.MaxCandidatesPerChannel)
	limits.MaxPasses = minPositive(limits.MaxPasses, rollout.MaxPasses)
	limits.MaxLatency = minDuration(limits.MaxLatency, rollout.MaxLatency)
	limits.MaxContextItems = minPositive(limits.MaxContextItems, rollout.MaxContextItems)
	limits.MaxRerankerHeadroom = minNonNegative(limits.MaxRerankerHeadroom, rollout.MaxRerankerHeadroom)
	return limits
}

func cloneRetrievalPlanPolicy(source RetrievalPlanPolicy) RetrievalPlanPolicy {
	result := source
	result.Templates = make(map[RetrievalQueryFamily]RetrievalPlanTemplate, len(source.Templates))
	for family, template := range source.Templates {
		template.Channels = append([]FusionChannel(nil), template.Channels...)
		template.ChannelCandidates = cloneChannelCandidates(template.ChannelCandidates)
		template.FallbackChannelCandidates = cloneChannelCandidates(template.FallbackChannelCandidates)
		template.ComplexityCandidates = cloneComplexityCandidates(template.ComplexityCandidates)
		template.Fusion = cloneFusionStrategy(template.Fusion)
		template.MemoryClassQuotas = cloneClassQuotas(template.MemoryClassQuotas)
		template.ContextPriorities = append([]memory.MemoryClass(nil), template.ContextPriorities...)
		template.FollowUp = cloneFollowUpRule(template.FollowUp)
		result.Templates[family] = template
	}
	return result
}

func minPositive(left, right int) int {
	if left <= 0 || right <= 0 {
		return 0
	}
	if left < right {
		return left
	}
	return right
}

func minNonNegative(left, right int) int {
	if left < right {
		return left
	}
	return right
}

func minDuration(left, right time.Duration) time.Duration {
	if left <= 0 || right <= 0 {
		return 0
	}
	if left < right {
		return left
	}
	return right
}
