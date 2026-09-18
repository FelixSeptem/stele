package retrieval

import (
	"fmt"
	"time"
)

type RetrievalBudgetEnvelope struct {
	TotalCandidates           int
	ChannelCandidates         map[FusionChannel]int
	FallbackChannelCandidates map[FusionChannel]int
	RerankerHeadroom          int
	MaxPasses                 int
	LatencyBudget             time.Duration
}

func (envelope RetrievalBudgetEnvelope) Validate() error {
	if envelope.TotalCandidates <= 0 || envelope.TotalCandidates > 5000 {
		return fmt.Errorf("retrieval budget total candidates must be between 1 and 5000")
	}
	if envelope.RerankerHeadroom < 0 || envelope.RerankerHeadroom > envelope.TotalCandidates {
		return fmt.Errorf("retrieval budget reranker headroom exceeds total candidates")
	}
	if envelope.MaxPasses < 1 || envelope.MaxPasses > 2 {
		return fmt.Errorf("retrieval budget max passes must be one or two")
	}
	if envelope.LatencyBudget <= 0 || envelope.LatencyBudget > 30*time.Second {
		return fmt.Errorf("retrieval budget latency must be between 1ns and 30s")
	}
	allocated := 0
	for channel, limit := range envelope.ChannelCandidates {
		if !channel.valid() || limit <= 0 || limit > envelope.TotalCandidates {
			return fmt.Errorf("invalid retrieval budget for channel %q", channel)
		}
		allocated += limit
	}
	if len(envelope.ChannelCandidates) == 0 || allocated != envelope.TotalCandidates {
		return fmt.Errorf("retrieval channel budgets must sum to total candidates")
	}
	fallbackAllocated := 0
	for channel, limit := range envelope.FallbackChannelCandidates {
		if !channel.valid() || limit <= 0 || limit > envelope.TotalCandidates {
			return fmt.Errorf("invalid retrieval fallback budget for channel %q", channel)
		}
		if _, planned := envelope.ChannelCandidates[channel]; planned {
			return fmt.Errorf("retrieval fallback channel %q overlaps planned channels", channel)
		}
		fallbackAllocated += limit
	}
	if fallbackAllocated+envelope.RerankerHeadroom >= envelope.TotalCandidates {
		return fmt.Errorf("retrieval fallback and reranker reserves exhaust planned candidate capacity")
	}
	return nil
}

type RetrievalBudgetLedger struct {
	totalCandidates   int
	channelLimits     map[FusionChannel]int
	channelRequested  map[FusionChannel]int
	channelAccepted   map[FusionChannel]int
	passRequested     map[int]map[FusionChannel]int
	passAccepted      map[int]map[FusionChannel]int
	rerankerHeadroom  int
	rerankerConsumed  int
	baselineLimits    map[FusionChannel]int
	baselineRequested map[FusionChannel]int
	maxPasses         int
	startedAt         time.Time
	deadline          time.Time
}

func NewRetrievalBudgetLedger(envelope RetrievalBudgetEnvelope, startedAt time.Time) (*RetrievalBudgetLedger, error) {
	if err := envelope.Validate(); err != nil {
		return nil, err
	}
	if startedAt.IsZero() {
		return nil, fmt.Errorf("retrieval budget start time is required")
	}
	limits := make(map[FusionChannel]int, len(envelope.ChannelCandidates))
	for channel, limit := range envelope.ChannelCandidates {
		limits[channel] = limit
	}
	baselineLimits := make(map[FusionChannel]int, len(envelope.FallbackChannelCandidates))
	for channel, limit := range envelope.FallbackChannelCandidates {
		baselineLimits[channel] = limit
	}
	return &RetrievalBudgetLedger{
		totalCandidates: envelope.TotalCandidates, channelLimits: limits,
		channelRequested: make(map[FusionChannel]int, len(limits)),
		channelAccepted:  make(map[FusionChannel]int, len(limits)),
		passRequested:    make(map[int]map[FusionChannel]int, envelope.MaxPasses),
		passAccepted:     make(map[int]map[FusionChannel]int, envelope.MaxPasses),
		rerankerHeadroom: envelope.RerankerHeadroom,
		baselineLimits:   baselineLimits, baselineRequested: make(map[FusionChannel]int, len(baselineLimits)),
		maxPasses: envelope.MaxPasses,
		startedAt: startedAt, deadline: startedAt.Add(envelope.LatencyBudget),
	}, nil
}

func (ledger *RetrievalBudgetLedger) ConsumeChannel(pass int, channel FusionChannel, count int, now time.Time) error {
	if err := ledger.ReserveChannelRequest(pass, channel, count, now); err != nil {
		return err
	}
	return ledger.RecordChannelAccepted(pass, channel, count, now)
}

func (ledger *RetrievalBudgetLedger) ReserveChannelRequest(pass int, channel FusionChannel, count int, now time.Time) error {
	if ledger == nil {
		return fmt.Errorf("retrieval budget ledger is required")
	}
	if pass < 1 || pass > ledger.maxPasses {
		return fmt.Errorf("retrieval pass %d exceeds budget", pass)
	}
	if now.IsZero() || now.After(ledger.deadline) {
		return fmt.Errorf("retrieval latency budget exhausted")
	}
	limit, ok := ledger.channelLimits[channel]
	if !ok {
		return fmt.Errorf("retrieval channel %q is not declared", channel)
	}
	if count < 0 || ledger.channelRequested[channel]+count > limit {
		return fmt.Errorf("retrieval channel %q exceeds allocation", channel)
	}
	if count > ledger.RemainingForRetrieval() {
		return fmt.Errorf("retrieval candidate envelope exhausted")
	}
	ledger.channelRequested[channel] += count
	ledger.passChannelCounts(ledger.passRequested, pass)[channel] += count
	return nil
}

func (ledger *RetrievalBudgetLedger) RecordChannelAccepted(pass int, channel FusionChannel, count int, now time.Time) error {
	if ledger == nil {
		return fmt.Errorf("retrieval budget ledger is required")
	}
	if pass < 1 || pass > ledger.maxPasses {
		return fmt.Errorf("retrieval pass %d exceeds budget", pass)
	}
	if now.IsZero() || now.After(ledger.deadline) {
		return fmt.Errorf("retrieval latency budget exhausted")
	}
	if _, ok := ledger.channelLimits[channel]; !ok {
		return fmt.Errorf("retrieval channel %q is not declared", channel)
	}
	passRequested := ledger.passChannelCounts(ledger.passRequested, pass)[channel]
	passAccepted := ledger.passChannelCounts(ledger.passAccepted, pass)[channel]
	if count < 0 || ledger.channelAccepted[channel]+count > ledger.channelRequested[channel] || passAccepted+count > passRequested {
		return fmt.Errorf("retrieval channel %q accepted candidates exceed requested work", channel)
	}
	ledger.channelAccepted[channel] += count
	ledger.passChannelCounts(ledger.passAccepted, pass)[channel] += count
	return nil
}

func (ledger *RetrievalBudgetLedger) ConsumeReranker(count int, now time.Time) error {
	if ledger == nil || now.IsZero() || now.After(ledger.deadline) {
		return fmt.Errorf("retrieval reranker latency budget exhausted")
	}
	if count < 0 || ledger.rerankerConsumed+count > ledger.rerankerHeadroom {
		return fmt.Errorf("retrieval reranker headroom exhausted")
	}
	if count > ledger.RemainingCandidates() {
		return fmt.Errorf("retrieval candidate envelope exhausted")
	}
	ledger.rerankerConsumed += count
	return nil
}

func (ledger *RetrievalBudgetLedger) ReserveBaseline(channel FusionChannel, count int, now time.Time) error {
	if ledger == nil || now.IsZero() || now.After(ledger.deadline) {
		return fmt.Errorf("retrieval baseline latency budget exhausted")
	}
	limit, ok := ledger.baselineLimits[channel]
	if !ok {
		return fmt.Errorf("retrieval baseline channel %q is not declared", channel)
	}
	if count < 0 || ledger.baselineRequested[channel]+count > limit {
		return fmt.Errorf("retrieval baseline channel %q exceeds allocation", channel)
	}
	if count > ledger.RemainingCandidates() {
		return fmt.Errorf("retrieval candidate envelope exhausted")
	}
	ledger.baselineRequested[channel] += count
	return nil
}

func (ledger *RetrievalBudgetLedger) Redistribute(from, to FusionChannel, count int) error {
	if ledger == nil || count <= 0 || from == to {
		return fmt.Errorf("invalid retrieval budget redistribution")
	}
	fromLimit, fromOK := ledger.channelLimits[from]
	toLimit, toOK := ledger.channelLimits[to]
	if !fromOK || !toOK || fromLimit-ledger.channelRequested[from] < count {
		return fmt.Errorf("retrieval budget redistribution exceeds unused allocation")
	}
	ledger.channelLimits[from] = fromLimit - count
	ledger.channelLimits[to] = toLimit + count
	return nil
}

func (ledger *RetrievalBudgetLedger) RemainingCandidates() int {
	if ledger == nil {
		return 0
	}
	consumed := ledger.rerankerConsumed
	for _, count := range ledger.baselineRequested {
		consumed += count
	}
	for _, count := range ledger.channelRequested {
		consumed += count
	}
	return maxInt(ledger.totalCandidates-consumed, 0)
}

func (ledger *RetrievalBudgetLedger) RemainingForRetrieval() int {
	if ledger == nil {
		return 0
	}
	reserved := maxInt(ledger.rerankerHeadroom-ledger.rerankerConsumed, 0) + ledger.remainingBaselineReserve()
	return maxInt(ledger.RemainingCandidates()-reserved, 0)
}

func (ledger *RetrievalBudgetLedger) RemainingForBaseline() int {
	if ledger == nil {
		return 0
	}
	return minInt(ledger.remainingBaselineReserve(), ledger.RemainingCandidates())
}

func (ledger *RetrievalBudgetLedger) RemainingForBaselineChannel(channel FusionChannel) int {
	if ledger == nil {
		return 0
	}
	return minInt(maxInt(ledger.baselineLimits[channel]-ledger.baselineRequested[channel], 0), ledger.RemainingCandidates())
}

func (ledger *RetrievalBudgetLedger) remainingBaselineReserve() int {
	if ledger == nil {
		return 0
	}
	remaining := 0
	for channel, limit := range ledger.baselineLimits {
		remaining += maxInt(limit-ledger.baselineRequested[channel], 0)
	}
	return remaining
}

func (ledger *RetrievalBudgetLedger) RemainingForReranker() int {
	if ledger == nil {
		return 0
	}
	return minInt(maxInt(ledger.rerankerHeadroom-ledger.rerankerConsumed, 0), ledger.RemainingCandidates())
}

func (ledger *RetrievalBudgetLedger) UnusedChannelAllocation(channel FusionChannel) int {
	if ledger == nil {
		return 0
	}
	return maxInt(ledger.channelLimits[channel]-ledger.channelRequested[channel], 0)
}

func (ledger *RetrievalBudgetLedger) UnacceptedChannelRequests(channel FusionChannel) int {
	if ledger == nil {
		return 0
	}
	return maxInt(ledger.channelRequested[channel]-ledger.channelAccepted[channel], 0)
}

func (ledger *RetrievalBudgetLedger) UnacceptedChannelRequestsForPass(pass int, channel FusionChannel) int {
	if ledger == nil {
		return 0
	}
	return maxInt(ledger.passRequested[pass][channel]-ledger.passAccepted[pass][channel], 0)
}

func (ledger *RetrievalBudgetLedger) RequestedForChannel(channel FusionChannel) int {
	if ledger == nil {
		return 0
	}
	return ledger.channelRequested[channel]
}

func (ledger *RetrievalBudgetLedger) AcceptedForChannel(channel FusionChannel) int {
	if ledger == nil {
		return 0
	}
	return ledger.channelAccepted[channel]
}

func (ledger *RetrievalBudgetLedger) RequestedForPass(pass int) int {
	if ledger == nil {
		return 0
	}
	return ledger.passTotal(ledger.passRequested, pass)
}

func (ledger *RetrievalBudgetLedger) AcceptedForPass(pass int) int {
	if ledger == nil {
		return 0
	}
	return ledger.passTotal(ledger.passAccepted, pass)
}

func (ledger *RetrievalBudgetLedger) ChannelLimit(channel FusionChannel) int {
	if ledger == nil {
		return 0
	}
	return ledger.channelLimits[channel]
}

func (ledger *RetrievalBudgetLedger) passChannelCounts(counts map[int]map[FusionChannel]int, pass int) map[FusionChannel]int {
	channels := counts[pass]
	if channels == nil {
		channels = make(map[FusionChannel]int, len(ledger.channelLimits))
		counts[pass] = channels
	}
	return channels
}

func (ledger *RetrievalBudgetLedger) passTotal(counts map[int]map[FusionChannel]int, pass int) int {
	if ledger == nil {
		return 0
	}
	total := 0
	for _, count := range counts[pass] {
		total += count
	}
	return total
}
