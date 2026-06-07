package retrieval

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/FelixSeptem/stele/internal/memory"
	"github.com/FelixSeptem/stele/internal/telemetry"
)

type SearchInput struct {
	Scope            memory.Scope
	Query            string
	QueryEmbedding   []float32
	Classes          []memory.MemoryClass
	TimeFrom         time.Time
	TimeTo           time.Time
	TopK             int
	IncludeSummaries bool
	IncludeRelations bool
}

func (i SearchInput) Validate() error {
	if err := i.Scope.Validate(); err != nil {
		return err
	}
	if strings.TrimSpace(i.Query) == "" {
		return fmt.Errorf("query is required")
	}
	if !i.TimeFrom.IsZero() && !i.TimeTo.IsZero() && i.TimeFrom.After(i.TimeTo) {
		return fmt.Errorf("time_from must be before or equal to time_to")
	}
	if i.TopK < 0 {
		return fmt.Errorf("top_k must be greater than or equal to zero")
	}
	return nil
}

type AssembleContextInput struct {
	Scope            memory.Scope
	Query            string
	Budget           int
	IncludeRelations bool
}

func (i AssembleContextInput) Validate() error {
	if err := i.Scope.Validate(); err != nil {
		return err
	}
	if strings.TrimSpace(i.Query) == "" {
		return fmt.Errorf("query is required")
	}
	if i.Budget <= 0 {
		return fmt.Errorf("budget must be greater than zero")
	}
	return nil
}

type ScoreBreakdown struct {
	Overall  float64 `json:"overall"`
	Lexical  float64 `json:"lexical"`
	Semantic float64 `json:"semantic"`
	Relation float64 `json:"relation"`
}

type Citation struct {
	MemoryID   string `json:"memory_id"`
	RawEventID string `json:"raw_event_id"`
	Operation  string `json:"operation"`
}

type SearchHit struct {
	Memory    memory.CanonicalMemory `json:"memory"`
	Score     ScoreBreakdown         `json:"score"`
	Citations []Citation             `json:"citations"`
}

type SearchResult struct {
	Hits []SearchHit `json:"hits"`
}

type AssembledContext struct {
	Profile           []SearchHit `json:"profile"`
	RecentSession     []SearchHit `json:"recent_session"`
	RecentEpisodes    []SearchHit `json:"recent_episodes"`
	RelevantSummaries []SearchHit `json:"relevant_summaries"`
	RelatedEntities   []SearchHit `json:"related_entities"`
	Citations         []Citation  `json:"citations"`
}

type ScoredMemory struct {
	Memory        memory.CanonicalMemory
	LexicalScore  float64
	SemanticScore float64
	RelationScore float64
}

type LexicalSearcher interface {
	SearchLexical(ctx context.Context, input SearchInput) ([]ScoredMemory, error)
}

type SemanticSearcher interface {
	SearchSemantic(ctx context.Context, input SearchInput) ([]ScoredMemory, error)
}

type RelationSearcher interface {
	SearchRelations(ctx context.Context, input SearchInput) ([]ScoredMemory, error)
}

type CitationLister interface {
	ListCitations(ctx context.Context, scope memory.Scope, memoryIDs []string) (map[string][]Citation, error)
}

type MemorySearcher interface {
	Search(ctx context.Context, input SearchInput) (SearchResult, error)
}

type ContextAssembler interface {
	AssembleContext(ctx context.Context, input AssembleContextInput) (AssembledContext, error)
}

type ServiceDependencies struct {
	Lexical   LexicalSearcher
	Semantic  SemanticSearcher
	Relations RelationSearcher
	Citations CitationLister
}

type Service struct {
	lexical   LexicalSearcher
	semantic  SemanticSearcher
	relations RelationSearcher
	citations CitationLister
	observer  telemetry.Observer
}

func NewService(deps ServiceDependencies, observers ...telemetry.Observer) *Service {
	var observer telemetry.Observer
	if len(observers) > 0 {
		observer = observers[0]
	}
	if observer == nil {
		observer = telemetry.NoopObserver()
	}

	return &Service{
		lexical:   deps.Lexical,
		semantic:  deps.Semantic,
		relations: deps.Relations,
		citations: deps.Citations,
		observer:  observer,
	}
}

func (s *Service) Search(ctx context.Context, input SearchInput) (result SearchResult, err error) {
	started := time.Now()
	defer func() {
		if s.observer == nil {
			return
		}

		status := "ok"
		count := len(result.Hits)
		errorMessage := ""
		if err != nil {
			status = "error"
			count = 0
			errorMessage = err.Error()
		}

		s.observer.RecordOperation(ctx, telemetry.OperationEvent{
			Mode:       "api",
			Component:  "retrieval_service",
			Operation:  "search",
			Status:     status,
			Count:      count,
			Duration:   time.Since(started),
			Error:      errorMessage,
			ObservedAt: time.Now().UTC(),
		})
	}()

	if err := input.Validate(); err != nil {
		return SearchResult{}, err
	}

	merged := map[string]ScoredMemory{}
	orderedIDs := make([]string, 0)

	appendHits := func(hits []ScoredMemory) {
		for _, hit := range hits {
			if !matchClassFilter(hit.Memory.Class, input.Classes) {
				continue
			}
			if !input.IncludeSummaries && hit.Memory.Class == memory.MemoryClassSummary {
				continue
			}
			if !input.IncludeRelations && hit.Memory.Class == memory.MemoryClassRelation {
				continue
			}

			current, ok := merged[hit.Memory.ID]
			if !ok {
				merged[hit.Memory.ID] = hit
				orderedIDs = append(orderedIDs, hit.Memory.ID)
				continue
			}

			if hit.Memory.ModifiedAt.After(current.Memory.ModifiedAt) || current.Memory.ModifiedAt.IsZero() {
				current.Memory = hit.Memory
			}
			current.LexicalScore += hit.LexicalScore
			current.SemanticScore += hit.SemanticScore
			current.RelationScore += hit.RelationScore
			merged[hit.Memory.ID] = current
		}
	}

	if s.lexical != nil {
		hits, err := s.lexical.SearchLexical(ctx, input)
		if err != nil {
			return SearchResult{}, err
		}
		appendHits(hits)
	}

	if s.semantic != nil {
		hits, err := s.semantic.SearchSemantic(ctx, input)
		if err != nil {
			return SearchResult{}, err
		}
		appendHits(hits)
	}

	if input.IncludeRelations && s.relations != nil {
		hits, err := s.relations.SearchRelations(ctx, input)
		if err != nil {
			return SearchResult{}, err
		}
		appendHits(hits)
	}

	scored := make([]SearchHit, 0, len(orderedIDs))
	memoryIDs := make([]string, 0, len(orderedIDs))
	for _, id := range orderedIDs {
		hit := merged[id]
		memoryIDs = append(memoryIDs, hit.Memory.ID)
		scored = append(scored, SearchHit{
			Memory: hit.Memory,
			Score: ScoreBreakdown{
				Overall:  hit.LexicalScore + hit.SemanticScore + hit.RelationScore,
				Lexical:  hit.LexicalScore,
				Semantic: hit.SemanticScore,
				Relation: hit.RelationScore,
			},
		})
	}

	sort.Slice(scored, func(i, j int) bool {
		if scored[i].Score.Overall == scored[j].Score.Overall {
			return scored[i].Memory.ModifiedAt.After(scored[j].Memory.ModifiedAt)
		}
		return scored[i].Score.Overall > scored[j].Score.Overall
	})

	if input.TopK > 0 && len(scored) > input.TopK {
		scored = scored[:input.TopK]
	}

	if s.citations != nil && len(scored) > 0 {
		memoryIDs = make([]string, 0, len(scored))
		for _, hit := range scored {
			memoryIDs = append(memoryIDs, hit.Memory.ID)
		}
		citationMap, err := s.citations.ListCitations(ctx, input.Scope, memoryIDs)
		if err != nil {
			return SearchResult{}, err
		}
		for i := range scored {
			scored[i].Citations = citationMap[scored[i].Memory.ID]
		}
	}

	result = SearchResult{Hits: scored}
	return result, nil
}

func (s *Service) AssembleContext(ctx context.Context, input AssembleContextInput) (output AssembledContext, err error) {
	started := time.Now()
	defer func() {
		if s.observer == nil {
			return
		}

		status := "ok"
		count := len(output.Profile) + len(output.RecentSession) + len(output.RecentEpisodes) + len(output.RelevantSummaries) + len(output.RelatedEntities)
		errorMessage := ""
		if err != nil {
			status = "error"
			count = 0
			errorMessage = err.Error()
		}

		s.observer.RecordOperation(ctx, telemetry.OperationEvent{
			Mode:       "api",
			Component:  "retrieval_service",
			Operation:  "assemble_context",
			Status:     status,
			Count:      count,
			Duration:   time.Since(started),
			Error:      errorMessage,
			ObservedAt: time.Now().UTC(),
		})
	}()

	if err := input.Validate(); err != nil {
		return AssembledContext{}, err
	}

	result, err := s.Search(ctx, SearchInput{
		Scope:            input.Scope,
		Query:            input.Query,
		TopK:             maxInt(input.Budget*3, input.Budget),
		IncludeSummaries: true,
		IncludeRelations: input.IncludeRelations,
	})
	if err != nil {
		return AssembledContext{}, err
	}

	output = AssembledContext{
		Profile:           make([]SearchHit, 0),
		RecentSession:     make([]SearchHit, 0),
		RecentEpisodes:    make([]SearchHit, 0),
		RelevantSummaries: make([]SearchHit, 0),
		RelatedEntities:   make([]SearchHit, 0),
		Citations:         make([]Citation, 0),
	}

	profiles := make([]SearchHit, 0)
	summaries := make([]SearchHit, 0)
	relations := make([]SearchHit, 0)
	episodes := make([]SearchHit, 0)
	others := make([]SearchHit, 0)
	citationSeen := map[string]struct{}{}
	for _, hit := range result.Hits {
		switch hit.Memory.Class {
		case memory.MemoryClassProfile:
			profiles = append(profiles, hit)
		case memory.MemoryClassSummary:
			summaries = append(summaries, hit)
		case memory.MemoryClassRelation:
			relations = append(relations, hit)
		case memory.MemoryClassEpisodic:
			episodes = append(episodes, hit)
		default:
			others = append(others, hit)
		}

		for _, citation := range hit.Citations {
			key := citation.MemoryID + ":" + citation.RawEventID + ":" + citation.Operation
			if _, ok := citationSeen[key]; ok {
				continue
			}
			citationSeen[key] = struct{}{}
			output.Citations = append(output.Citations, citation)
		}
	}

	remaining := input.Budget
	if remaining > 0 && len(profiles) > 0 {
		output.Profile = append(output.Profile, profiles[0])
		remaining--
	}

	if remaining > 0 && len(summaries) > 0 {
		take := minInt(len(summaries), remaining)
		output.RelevantSummaries = append(output.RelevantSummaries, summaries[:take]...)
		remaining -= take
	}

	if remaining > 0 && input.IncludeRelations && len(relations) > 0 {
		take := minInt(len(relations), remaining)
		output.RelatedEntities = append(output.RelatedEntities, relations[:take]...)
		remaining -= take
	}

	if remaining > 0 && len(summaries) == 0 && len(episodes) > 0 {
		take := minInt(len(episodes), remaining)
		output.RecentSession = append(output.RecentSession, episodes[:take]...)
		remaining -= take
	}

	if remaining > 0 && len(others) > 0 {
		take := minInt(len(others), remaining)
		output.RecentEpisodes = append(output.RecentEpisodes, others[:take]...)
		remaining -= take
	}

	if remaining > 0 && len(summaries) > 0 && len(episodes) > 0 {
		take := minInt(len(episodes), remaining)
		output.RecentSession = append(output.RecentSession, episodes[:take]...)
		remaining -= take
	}

	return output, nil
}

func matchClassFilter(class memory.MemoryClass, filters []memory.MemoryClass) bool {
	if len(filters) == 0 {
		return true
	}

	for _, filter := range filters {
		if filter == class {
			return true
		}
	}

	return false
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}
