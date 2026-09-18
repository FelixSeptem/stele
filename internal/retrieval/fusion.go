package retrieval

import (
	"fmt"
	"math"
	"sort"
	"strings"

	"github.com/FelixSeptem/stele/internal/memory"
)

// FusionStrategyName identifies a cross-channel candidate merge algorithm.
type FusionStrategyName string

const (
	FusionStrategyRRF                FusionStrategyName = "rrf"
	FusionStrategyNormalizedWeighted FusionStrategyName = "normalized_weighted"
)

func (n FusionStrategyName) valid() bool {
	return n == FusionStrategyRRF || n == FusionStrategyNormalizedWeighted
}

// FusionChannel identifies one independent retrieval source.
type FusionChannel string

const (
	FusionChannelLexical  FusionChannel = "lexical"
	FusionChannelSemantic FusionChannel = "semantic"
	FusionChannelRelation FusionChannel = "relation"
	FusionChannelChunk    FusionChannel = "chunk"
)

func (c FusionChannel) valid() bool {
	switch c {
	case FusionChannelLexical, FusionChannelSemantic, FusionChannelRelation, FusionChannelChunk:
		return true
	default:
		return false
	}
}

// FusionStrategy is a complete, replayable description of a fusion run.
type FusionStrategy struct {
	Name                FusionStrategyName        `json:"name"`
	Version             string                    `json:"version"`
	RankConstant        int                       `json:"rank_constant"`
	ChannelWeights      map[FusionChannel]float64 `json:"channel_weights"`
	PerChannelCandidate int                       `json:"per_channel_candidate"`
	TotalCandidates     int                       `json:"total_candidates"`
}

func DefaultRRFStrategy() FusionStrategy {
	return FusionStrategy{
		Name:                FusionStrategyRRF,
		Version:             "rrf-v1",
		RankConstant:        60,
		ChannelWeights:      map[FusionChannel]float64{FusionChannelLexical: 1, FusionChannelSemantic: 1, FusionChannelRelation: 0.8, FusionChannelChunk: 0.8},
		PerChannelCandidate: 50,
		TotalCandidates:     200,
	}
}

func DefaultNormalizedWeightedStrategy() FusionStrategy {
	strategy := DefaultRRFStrategy()
	strategy.Name = FusionStrategyNormalizedWeighted
	strategy.Version = "normalized-weighted-v1"
	return strategy
}

func (s FusionStrategy) Validate() error {
	if !s.Name.valid() {
		return fmt.Errorf("unsupported fusion strategy %q", s.Name)
	}
	if strings.TrimSpace(s.Version) == "" {
		return fmt.Errorf("fusion strategy version is required")
	}
	if len(s.Version) > 64 {
		return fmt.Errorf("fusion strategy version exceeds 64 characters")
	}
	expectedVersion := ""
	switch s.Name {
	case FusionStrategyRRF:
		expectedVersion = "rrf-v1"
	case FusionStrategyNormalizedWeighted:
		expectedVersion = "normalized-weighted-v1"
	}
	if s.Version != expectedVersion {
		return fmt.Errorf("fusion strategy %q requires version %q", s.Name, expectedVersion)
	}
	if s.RankConstant <= 0 || s.RankConstant > 10000 {
		return fmt.Errorf("fusion rank constant must be between 1 and 10000")
	}
	if s.PerChannelCandidate <= 0 || s.PerChannelCandidate > 1000 {
		return fmt.Errorf("per-channel candidate limit must be between 1 and 1000")
	}
	if s.TotalCandidates <= 0 || s.TotalCandidates > 5000 {
		return fmt.Errorf("total candidate limit must be between 1 and 5000")
	}
	if len(s.ChannelWeights) == 0 {
		return fmt.Errorf("at least one fusion channel weight is required")
	}
	totalWeight := 0.0
	for channel, weight := range s.ChannelWeights {
		if !channel.valid() {
			return fmt.Errorf("unsupported fusion channel %q", channel)
		}
		if math.IsNaN(weight) || math.IsInf(weight, 0) || weight < 0 || weight > 100 {
			return fmt.Errorf("fusion weight for %q must be between 0 and 100", channel)
		}
		totalWeight += weight
	}
	if totalWeight <= 0 {
		return fmt.Errorf("at least one fusion channel weight must be positive")
	}
	return nil
}

type FusionChannelCandidates struct {
	Channel    FusionChannel
	Candidates []ScoredMemory
}

type FusedCandidate struct {
	Memory        memory.CanonicalMemory
	Score         float64
	ChannelRanks  map[FusionChannel]int
	ChannelScores ScoreBreakdown
}

type fusionAggregate struct {
	candidate    FusedCandidate
	channelScore map[FusionChannel]float64
}

func (c FusedCandidate) Validate() error {
	if strings.TrimSpace(c.Memory.ID) == "" {
		return fmt.Errorf("fused candidate memory id is required")
	}
	if math.IsNaN(c.Score) || math.IsInf(c.Score, 0) {
		return fmt.Errorf("fused candidate score is invalid")
	}
	for channel, rank := range c.ChannelRanks {
		if !channel.valid() || rank <= 0 {
			return fmt.Errorf("invalid fused candidate channel rank")
		}
	}
	return nil
}

// FuseCandidates merges already validated, channel-ranked candidates. The
// input order is the channel rank order supplied by each recall implementation.
func FuseCandidates(strategy FusionStrategy, channels []FusionChannelCandidates) ([]FusedCandidate, error) {
	if err := strategy.Validate(); err != nil {
		return nil, err
	}
	aggregates := make(map[string]*fusionAggregate)
	seenChannels := make(map[FusionChannel]bool)
	for _, channel := range channels {
		if !channel.Channel.valid() {
			return nil, fmt.Errorf("unsupported fusion channel %q", channel.Channel)
		}
		if seenChannels[channel.Channel] {
			return nil, fmt.Errorf("duplicate fusion channel %q", channel.Channel)
		}
		seenChannels[channel.Channel] = true
		limit := len(channel.Candidates)
		if limit > strategy.PerChannelCandidate {
			limit = strategy.PerChannelCandidate
		}
		for index := 0; index < limit; index++ {
			candidate := channel.Candidates[index]
			id := strings.TrimSpace(candidate.Memory.ID)
			if id == "" {
				return nil, fmt.Errorf("fusion candidate memory id is required")
			}
			rank := index + 1
			entry := aggregates[id]
			if entry == nil {
				entry = &fusionAggregate{
					candidate:    FusedCandidate{Memory: candidate.Memory, ChannelRanks: map[FusionChannel]int{}, ChannelScores: scoreBreakdown(candidate)},
					channelScore: map[FusionChannel]float64{},
				}
				aggregates[id] = entry
			} else if candidate.Memory.ModifiedAt.After(entry.candidate.Memory.ModifiedAt) {
				entry.candidate.Memory = candidate.Memory
			}
			if _, exists := entry.candidate.ChannelRanks[channel.Channel]; !exists {
				entry.candidate.ChannelRanks[channel.Channel] = rank
				entry.channelScore[channel.Channel] = scoreForChannel(candidate, channel.Channel)
				if len(entry.candidate.ChannelRanks) > 1 {
					addScoreBreakdown(&entry.candidate.ChannelScores, candidate)
				}
			}
		}
	}

	if strategy.Name == FusionStrategyNormalizedWeighted {
		normalizeChannelScores(aggregates, channels, strategy)
	}
	for _, entry := range aggregates {
		score := 0.0
		for channel, rank := range entry.candidate.ChannelRanks {
			weight := strategy.ChannelWeights[channel]
			if strategy.Name == FusionStrategyRRF {
				score += weight / float64(strategy.RankConstant+rank)
			} else {
				score += weight * entry.channelScore[channel]
			}
		}
		entry.candidate.Score = score
	}

	result := make([]FusedCandidate, 0, len(aggregates))
	for _, entry := range aggregates {
		if err := entry.candidate.Validate(); err != nil {
			return nil, err
		}
		result = append(result, entry.candidate)
	}
	sort.SliceStable(result, func(i, j int) bool {
		if result[i].Score != result[j].Score {
			return result[i].Score > result[j].Score
		}
		leftClass, rightClass := fusionClassPriority(result[i].Memory.Class), fusionClassPriority(result[j].Memory.Class)
		if leftClass != rightClass {
			return leftClass < rightClass
		}
		leftTime, rightTime := result[i].Memory.ModifiedAt, result[j].Memory.ModifiedAt
		if !leftTime.Equal(rightTime) {
			return leftTime.After(rightTime)
		}
		return result[i].Memory.ID < result[j].Memory.ID
	})
	if len(result) > strategy.TotalCandidates {
		result = result[:strategy.TotalCandidates]
	}
	return result, nil
}

func scoreForChannel(candidate ScoredMemory, channel FusionChannel) float64 {
	switch channel {
	case FusionChannelLexical:
		return candidate.LexicalScore
	case FusionChannelSemantic:
		return candidate.SemanticScore
	case FusionChannelRelation:
		return candidate.RelationScore
	case FusionChannelChunk:
		return candidate.LexicalScore + candidate.SemanticScore + candidate.RelationScore
	default:
		return 0
	}
}

func scoreBreakdown(candidate ScoredMemory) ScoreBreakdown {
	return ScoreBreakdown{Lexical: candidate.LexicalScore, Semantic: candidate.SemanticScore, Relation: candidate.RelationScore}
}

func addScoreBreakdown(total *ScoreBreakdown, candidate ScoredMemory) {
	total.Lexical += candidate.LexicalScore
	total.Semantic += candidate.SemanticScore
	total.Relation += candidate.RelationScore
}

func normalizeChannelScores(aggregates map[string]*fusionAggregate, channels []FusionChannelCandidates, strategy FusionStrategy) {
	for _, channel := range channels {
		limit := len(channel.Candidates)
		if limit > strategy.PerChannelCandidate {
			limit = strategy.PerChannelCandidate
		}
		if limit == 0 {
			continue
		}
		minScore, maxScore := math.Inf(1), math.Inf(-1)
		for index := 0; index < limit; index++ {
			score := scoreForChannel(channel.Candidates[index], channel.Channel)
			if score < minScore {
				minScore = score
			}
			if score > maxScore {
				maxScore = score
			}
		}
		for _, entry := range aggregates {
			if _, ok := entry.candidate.ChannelRanks[channel.Channel]; !ok {
				continue
			}
			raw := entry.channelScore[channel.Channel]
			if maxScore == minScore {
				entry.channelScore[channel.Channel] = 1
			} else {
				entry.channelScore[channel.Channel] = (raw - minScore) / (maxScore - minScore)
			}
		}
	}
}

func fusionClassPriority(class memory.MemoryClass) int {
	switch class {
	case memory.MemoryClassSummary:
		return 0
	case memory.MemoryClassProfile:
		return 1
	case memory.MemoryClassProcedural:
		return 2
	case memory.MemoryClassEpisodic:
		return 3
	case memory.MemoryClassRelation:
		return 4
	default:
		return 5
	}
}
