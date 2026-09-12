package retrieval

import (
	"context"
	"fmt"
	"sort"
	"time"

	"github.com/FelixSeptem/stele/internal/memory"
	"github.com/FelixSeptem/stele/internal/telemetry"
)

const evaluationReplayTopK = 100

// EvaluationRunner replays a checked-in fixture through the existing scoped search
// interface. It is an internal evaluation path and does not change SearchInput's
// public contract or ordinary SearchResult output.
type EvaluationRunner struct {
	searcher MemorySearcher
	observer telemetry.Observer
}

func NewEvaluationRunner(searcher MemorySearcher, observers ...telemetry.Observer) *EvaluationRunner {
	var observer telemetry.Observer
	if len(observers) > 0 {
		observer = observers[0]
	}
	if observer == nil {
		observer = telemetry.NoopObserver()
	}
	return &EvaluationRunner{searcher: searcher, observer: observer}
}

type EvaluationReplay struct {
	Metadata EvaluationRankingMetadata
	Cases    []EvaluationReplayCase
}

type EvaluationReplayCase struct {
	CaseID                 string
	Category               string
	Scope                  memory.Scope
	ExpectedEvidenceGroups [][]string
	ExcludedAliases        []string
	Candidates             []EvaluationReplayCandidate
	Diagnostics            []EvaluationCandidateDiagnostic
	ChannelAvailability    map[string]EvaluationChannelStatus
	CandidatePoolSize      int
	Latency                time.Duration
	ChangedRankCount       int
	RerankFallback         string
}

type EvaluationCandidateDisposition string

const (
	EvaluationCandidateDispositionReturned    EvaluationCandidateDisposition = "returned"
	EvaluationCandidateDispositionNotReturned EvaluationCandidateDisposition = "not_returned"
)

type EvaluationChannelStatus string

const (
	EvaluationChannelStatusAvailable   EvaluationChannelStatus = "available"
	EvaluationChannelStatusUnavailable EvaluationChannelStatus = "unavailable"
)

// EvaluationCandidateDiagnostic contains only a fixture alias and bounded rank
// information. Unknown, hidden, and foreign candidates are deliberately omitted from
// this list and represented by aggregate safety categories during evaluation.
type EvaluationCandidateDiagnostic struct {
	Alias          string
	FusionStrategy string
	LexicalRank    int
	SemanticRank   int
	RelationRank   int
	ChunkRank      int
	ChannelStatus  map[string]EvaluationChannelStatus
	FinalRank      int
	Disposition    EvaluationCandidateDisposition
}

// EvaluationReplayCandidate remains internal to replay and metric calculation. It
// is converted to an alias-only report contribution before it crosses any output
// boundary.
type EvaluationReplayCandidate struct {
	Alias         string
	MemoryID      string
	Scope         memory.Scope
	State         memory.MemoryState
	FactCluster   string
	Lexical       bool
	Semantic      bool
	Relation      bool
	Chunk         bool
	FinalRank     int
	lexicalScore  float64
	semanticScore float64
	relationScore float64
	ChunkDerived  bool
}

func (r *EvaluationRunner) Replay(ctx context.Context, fixture EvaluationFixture, seed EvaluationFixtureSeed, metadata EvaluationRankingMetadata) (run EvaluationReplay, replayErr error) {
	started := time.Now()
	defer func() {
		if r == nil || r.observer == nil {
			return
		}
		status := "completed"
		failureCategory := ""
		if replayErr != nil {
			status = "failed"
			failureCategory = string(EvaluationSafetyFailureUnsafeDiagnostics)
		}
		r.rankingEvaluationObserver().RecordRetrievalEvaluation(ctx, telemetry.RetrievalEvaluationEvent{
			Status:          status,
			FixtureVersion:  fixture.Version,
			RankingVersion:  metadata.RankingVersion,
			PolicyVersion:   metadata.PolicyVersion,
			FailureCategory: failureCategory,
			CaseCount:       len(fixture.Cases),
			Duration:        time.Since(started),
		})
	}()
	if err := fixture.Validate(); err != nil {
		return EvaluationReplay{}, NewEvaluationFailure(EvaluationSafetyFailureInvalidFixtureScope, err.Error())
	}
	if metadata.FusionStrategy == "" {
		metadata.FusionStrategy = fusionStrategyIdentity(DefaultRRFStrategy())
	}
	if err := metadata.Validate(); err != nil {
		return EvaluationReplay{}, NewEvaluationFailure(EvaluationSafetyFailureUnsafeDiagnostics, err.Error())
	}
	if metadata.FixtureVersion != fixture.Version || seed.FixtureVersion != fixture.Version {
		return EvaluationReplay{}, fmt.Errorf("evaluation fixture version mismatch")
	}
	if r == nil || r.searcher == nil {
		return EvaluationReplay{}, fmt.Errorf("evaluation searcher is not configured")
	}

	aliases := make(map[string]EvaluationSeededAlias, len(seed.Aliases))
	for _, record := range seed.Aliases {
		key := evaluationSeedAliasKey(record.CaseID, record.Alias)
		if _, exists := aliases[key]; exists {
			return EvaluationReplay{}, fmt.Errorf("duplicate seeded evaluation alias")
		}
		aliases[key] = record
	}

	run = EvaluationReplay{Metadata: metadata, Cases: make([]EvaluationReplayCase, 0, len(fixture.Cases))}
	for _, item := range fixture.Cases {
		caseAliases := make(map[string]EvaluationSeededAlias, len(item.Sources))
		activeAliases := make([]string, 0, len(item.Sources))
		for _, source := range item.Sources {
			record, found := aliases[evaluationSeedAliasKey(item.ID, source.Alias)]
			if !found {
				return EvaluationReplay{}, fmt.Errorf("seeded evaluation alias is missing")
			}
			caseAliases[record.MemoryID] = record
			if record.State == memory.MemoryStateActive {
				activeAliases = append(activeAliases, record.Alias)
			}
		}

		started := time.Now()
		result, err := r.searcher.Search(ctx, SearchInput{
			Scope:                 item.Scope,
			Query:                 item.Query,
			LexicalMatchMode:      metadata.LexicalMatchMode,
			TopK:                  evaluationReplayTopK,
			IncludeSummaries:      true,
			IncludeRelations:      true,
			rankingPolicyDisabled: true,
		})
		if err != nil {
			return EvaluationReplay{}, fmt.Errorf("execute evaluation retrieval query")
		}

		caseRun := EvaluationReplayCase{
			CaseID:                 item.ID,
			Category:               item.Category,
			Scope:                  item.Scope,
			ExpectedEvidenceGroups: item.ExpectedEvidenceGroups,
			ExcludedAliases:        item.ExcludedAliases,
			Candidates:             make([]EvaluationReplayCandidate, 0, len(result.Hits)),
			CandidatePoolSize:      len(result.Hits),
			Latency:                time.Since(started),
			ChannelAvailability:    evaluationChannelAvailability(result.fusionChannelAvailability),
		}
		if result.fusionStrategy.Name != "" {
			metadata.FusionStrategy = fusionStrategyIdentity(result.fusionStrategy)
			run.Metadata.FusionStrategy = metadata.FusionStrategy
		}
		for index, hit := range result.Hits {
			record := caseAliases[hit.Memory.ID]
			caseRun.Candidates = append(caseRun.Candidates, EvaluationReplayCandidate{
				Alias:         record.Alias,
				MemoryID:      hit.Memory.ID,
				Scope:         hit.Memory.Scope,
				State:         hit.Memory.State,
				FactCluster:   record.FactCluster,
				Lexical:       hit.Score.Lexical != 0,
				Semantic:      hit.Score.Semantic != 0,
				Relation:      hit.Score.Relation != 0,
				Chunk:         hit.Chunk != nil,
				FinalRank:     index + 1,
				lexicalScore:  hit.Score.Lexical,
				semanticScore: hit.Score.Semantic,
				relationScore: hit.Score.Relation,
				ChunkDerived:  hit.Chunk != nil,
			})
		}
		caseRun.Diagnostics = evaluationCandidateDiagnostics(caseRun.Candidates, metadata.FusionStrategy)
		caseRun.Diagnostics = append(caseRun.Diagnostics, evaluationMissingCandidateDiagnostics(caseRun.Diagnostics, activeAliases, metadata.FusionStrategy)...)
		run.Cases = append(run.Cases, caseRun)
	}
	return run, nil
}

func evaluationMissingCandidateDiagnostics(returned []EvaluationCandidateDiagnostic, activeAliases []string, fusionStrategy string) []EvaluationCandidateDiagnostic {
	returnedAliases := make(map[string]struct{}, len(returned))
	for _, diagnostic := range returned {
		returnedAliases[diagnostic.Alias] = struct{}{}
	}
	aliases := append([]string(nil), activeAliases...)
	sort.Strings(aliases)
	missing := make([]EvaluationCandidateDiagnostic, 0, len(aliases))
	for _, alias := range aliases {
		if _, found := returnedAliases[alias]; found {
			continue
		}
		if len(returned)+len(missing) >= evaluationReplayTopK {
			break
		}
		missing = append(missing, EvaluationCandidateDiagnostic{
			Alias:          alias,
			FusionStrategy: fusionStrategy,
			ChannelStatus: map[string]EvaluationChannelStatus{
				"lexical":  EvaluationChannelStatusUnavailable,
				"semantic": EvaluationChannelStatusUnavailable,
				"relation": EvaluationChannelStatusUnavailable,
				"chunk":    EvaluationChannelStatusUnavailable,
			},
			Disposition: EvaluationCandidateDispositionNotReturned,
		})
	}
	return missing
}

func evaluationChannelAvailability(availability map[FusionChannel]fusionChannelAvailability) map[string]EvaluationChannelStatus {
	result := make(map[string]EvaluationChannelStatus, 4)
	for _, channel := range []FusionChannel{FusionChannelLexical, FusionChannelSemantic, FusionChannelRelation, FusionChannelChunk} {
		status := EvaluationChannelStatusUnavailable
		if availability[channel] == fusionChannelAvailable {
			status = EvaluationChannelStatusAvailable
		}
		result[string(channel)] = status
	}
	return result
}

func fusionStrategyIdentity(strategy FusionStrategy) string {
	return string(strategy.Name) + ":" + strategy.Version
}

type retrievalEvaluationObserver interface {
	RecordRetrievalEvaluation(context.Context, telemetry.RetrievalEvaluationEvent)
}

func (r *EvaluationRunner) rankingEvaluationObserver() retrievalEvaluationObserver {
	if observer, ok := r.observer.(retrievalEvaluationObserver); ok {
		return observer
	}
	return evaluationObserverAdapter{}
}

type evaluationObserverAdapter struct{}

func (evaluationObserverAdapter) RecordRetrievalEvaluation(context.Context, telemetry.RetrievalEvaluationEvent) {
}

func evaluationCandidateDiagnostics(candidates []EvaluationReplayCandidate, fusionStrategy string) []EvaluationCandidateDiagnostic {
	lexicalRanks := evaluationChannelRanks(candidates, func(candidate EvaluationReplayCandidate) float64 { return candidate.lexicalScore })
	semanticRanks := evaluationChannelRanks(candidates, func(candidate EvaluationReplayCandidate) float64 { return candidate.semanticScore })
	relationRanks := evaluationChannelRanks(candidates, func(candidate EvaluationReplayCandidate) float64 { return candidate.relationScore })
	chunkRanks := evaluationBooleanChannelRanks(candidates, func(candidate EvaluationReplayCandidate) bool { return candidate.Chunk })
	diagnostics := make([]EvaluationCandidateDiagnostic, 0, len(candidates))
	for _, candidate := range candidates {
		if candidate.Alias == "" || candidate.State != memory.MemoryStateActive {
			continue
		}
		diagnostics = append(diagnostics, EvaluationCandidateDiagnostic{
			Alias:          candidate.Alias,
			FusionStrategy: fusionStrategy,
			LexicalRank:    lexicalRanks[candidate.FinalRank],
			SemanticRank:   semanticRanks[candidate.FinalRank],
			RelationRank:   relationRanks[candidate.FinalRank],
			ChunkRank:      chunkRanks[candidate.FinalRank],
			ChannelStatus: map[string]EvaluationChannelStatus{
				"lexical":  evaluationChannelStatus(candidate.Lexical),
				"semantic": evaluationChannelStatus(candidate.Semantic),
				"relation": evaluationChannelStatus(candidate.Relation),
				"chunk":    evaluationChannelStatus(candidate.Chunk),
			},
			FinalRank:   candidate.FinalRank,
			Disposition: EvaluationCandidateDispositionReturned,
		})
	}
	return diagnostics
}

func evaluationBooleanChannelRanks(candidates []EvaluationReplayCandidate, included func(EvaluationReplayCandidate) bool) map[int]int {
	ranks := make(map[int]int)
	rank := 0
	for _, candidate := range candidates {
		if !included(candidate) {
			continue
		}
		rank++
		ranks[candidate.FinalRank] = rank
	}
	return ranks
}

func evaluationChannelStatus(included bool) EvaluationChannelStatus {
	if included {
		return EvaluationChannelStatusAvailable
	}
	return EvaluationChannelStatusUnavailable
}

func evaluationChannelRanks(candidates []EvaluationReplayCandidate, score func(EvaluationReplayCandidate) float64) map[int]int {
	indices := make([]int, 0, len(candidates))
	for index, candidate := range candidates {
		if score(candidate) != 0 {
			indices = append(indices, index)
		}
	}
	sort.SliceStable(indices, func(i, j int) bool {
		return score(candidates[indices[i]]) > score(candidates[indices[j]])
	})
	ranks := make(map[int]int, len(indices))
	for index, candidateIndex := range indices {
		ranks[candidates[candidateIndex].FinalRank] = index + 1
	}
	return ranks
}

func evaluationSeedAliasKey(caseID, alias string) string {
	return caseID + "\x00" + alias
}
