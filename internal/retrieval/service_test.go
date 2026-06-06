package retrieval

import (
	"context"
	"testing"
	"time"

	"github.com/FelixSeptem/stele/internal/memory"
)

type stubLexicalSource struct {
	gotInput SearchInput
	hits     []ScoredMemory
	err      error
}

func (s *stubLexicalSource) SearchLexical(ctx context.Context, input SearchInput) ([]ScoredMemory, error) {
	s.gotInput = input
	return s.hits, s.err
}

type stubSemanticSource struct {
	gotInput SearchInput
	hits     []ScoredMemory
	err      error
}

func (s *stubSemanticSource) SearchSemantic(ctx context.Context, input SearchInput) ([]ScoredMemory, error) {
	s.gotInput = input
	return s.hits, s.err
}

type stubRelationSource struct {
	gotInput SearchInput
	hits     []ScoredMemory
	err      error
}

func (s *stubRelationSource) SearchRelations(ctx context.Context, input SearchInput) ([]ScoredMemory, error) {
	s.gotInput = input
	return s.hits, s.err
}

type stubCitationSource struct {
	gotScope     memory.Scope
	gotMemoryIDs []string
	citations    map[string][]Citation
	err          error
}

func (s *stubCitationSource) ListCitations(ctx context.Context, scope memory.Scope, memoryIDs []string) (map[string][]Citation, error) {
	s.gotScope = scope
	s.gotMemoryIDs = append([]string(nil), memoryIDs...)
	return s.citations, s.err
}

func TestServiceSearchMergesRankedHitsAcrossSources(t *testing.T) {
	now := time.Date(2026, 6, 6, 11, 0, 0, 0, time.UTC)
	scope := memory.Scope{
		Tenant:    "tenant-a",
		Project:   "project-a",
		Namespace: "namespace-a",
	}

	lexical := &stubLexicalSource{
		hits: []ScoredMemory{
			{
				Memory: memory.CanonicalMemory{
					ID:         "mem_profile",
					Scope:      scope,
					Class:      memory.MemoryClassProfile,
					State:      memory.MemoryStateActive,
					Content:    "User prefers concise answers.",
					CreatedAt:  now.Add(-2 * time.Hour),
					ModifiedAt: now.Add(-30 * time.Minute),
				},
				LexicalScore: 0.9,
			},
		},
	}
	semantic := &stubSemanticSource{
		hits: []ScoredMemory{
			{
				Memory: memory.CanonicalMemory{
					ID:         "mem_profile",
					Scope:      scope,
					Class:      memory.MemoryClassProfile,
					State:      memory.MemoryStateActive,
					Content:    "User prefers concise answers.",
					CreatedAt:  now.Add(-2 * time.Hour),
					ModifiedAt: now.Add(-30 * time.Minute),
				},
				SemanticScore: 0.5,
			},
			{
				Memory: memory.CanonicalMemory{
					ID:         "mem_summary",
					Scope:      scope,
					Class:      memory.MemoryClassSummary,
					State:      memory.MemoryStateActive,
					Content:    "Summary: the user asked about travel planning.",
					CreatedAt:  now.Add(-90 * time.Minute),
					ModifiedAt: now.Add(-10 * time.Minute),
				},
				SemanticScore: 0.7,
			},
		},
	}
	relations := &stubRelationSource{
		hits: []ScoredMemory{
			{
				Memory: memory.CanonicalMemory{
					ID:         "mem_relation",
					Scope:      scope,
					Class:      memory.MemoryClassRelation,
					State:      memory.MemoryStateActive,
					Content:    "entity:user relation:interested_in target:travel",
					CreatedAt:  now.Add(-70 * time.Minute),
					ModifiedAt: now.Add(-5 * time.Minute),
				},
				RelationScore: 0.6,
			},
		},
	}
	citations := &stubCitationSource{
		citations: map[string][]Citation{
			"mem_profile":  {{MemoryID: "mem_profile", RawEventID: "evt_profile", Operation: "promote_candidate"}},
			"mem_summary":  {{MemoryID: "mem_summary", RawEventID: "evt_summary", Operation: "create_summary_memory"}},
			"mem_relation": {{MemoryID: "mem_relation", RawEventID: "evt_relation", Operation: "promote_candidate"}},
		},
	}

	service := NewService(ServiceDependencies{
		Lexical:   lexical,
		Semantic:  semantic,
		Relations: relations,
		Citations: citations,
	})

	result, err := service.Search(context.Background(), SearchInput{
		Scope:            scope,
		Query:            "travel preferences",
		TopK:             5,
		IncludeSummaries: true,
		IncludeRelations: true,
	})
	if err != nil {
		t.Fatalf("Search() error = %v", err)
	}

	if len(result.Hits) != 3 {
		t.Fatalf("len(result.Hits) = %d, want %d", len(result.Hits), 3)
	}

	if result.Hits[0].Memory.ID != "mem_profile" {
		t.Fatalf("result.Hits[0].Memory.ID = %q, want %q", result.Hits[0].Memory.ID, "mem_profile")
	}

	if result.Hits[0].Score.Overall <= result.Hits[1].Score.Overall {
		t.Fatalf("overall score ordering = %v then %v, want descending", result.Hits[0].Score.Overall, result.Hits[1].Score.Overall)
	}

	if len(result.Hits[0].Citations) != 1 || result.Hits[0].Citations[0].RawEventID != "evt_profile" {
		t.Fatalf("profile citations = %+v, want evt_profile", result.Hits[0].Citations)
	}

	if len(citations.gotMemoryIDs) != 3 {
		t.Fatalf("citation memory ids = %v, want three memory ids", citations.gotMemoryIDs)
	}
}

func TestServiceAssembleContextReturnsStructuredSections(t *testing.T) {
	now := time.Date(2026, 6, 6, 12, 0, 0, 0, time.UTC)
	scope := memory.Scope{
		Tenant:    "tenant-a",
		Project:   "project-a",
		Namespace: "namespace-a",
	}

	lexical := &stubLexicalSource{
		hits: []ScoredMemory{
			{
				Memory: memory.CanonicalMemory{
					ID:         "mem_profile",
					Scope:      scope,
					Class:      memory.MemoryClassProfile,
					State:      memory.MemoryStateActive,
					Content:    "User prefers concise answers.",
					CreatedAt:  now.Add(-3 * time.Hour),
					ModifiedAt: now.Add(-40 * time.Minute),
				},
				LexicalScore: 0.8,
			},
			{
				Memory: memory.CanonicalMemory{
					ID:         "mem_episode",
					Scope:      scope,
					Class:      memory.MemoryClassEpisodic,
					State:      memory.MemoryStateActive,
					Content:    "The user asked for a weekend travel plan.",
					CreatedAt:  now.Add(-90 * time.Minute),
					ModifiedAt: now.Add(-20 * time.Minute),
				},
				LexicalScore: 0.7,
			},
		},
	}
	semantic := &stubSemanticSource{
		hits: []ScoredMemory{
			{
				Memory: memory.CanonicalMemory{
					ID:         "mem_summary",
					Scope:      scope,
					Class:      memory.MemoryClassSummary,
					State:      memory.MemoryStateActive,
					Content:    "Summary: the user is planning weekend travel.",
					CreatedAt:  now.Add(-80 * time.Minute),
					ModifiedAt: now.Add(-10 * time.Minute),
				},
				SemanticScore: 0.9,
			},
		},
	}
	relations := &stubRelationSource{
		hits: []ScoredMemory{
			{
				Memory: memory.CanonicalMemory{
					ID:         "mem_relation",
					Scope:      scope,
					Class:      memory.MemoryClassRelation,
					State:      memory.MemoryStateActive,
					Content:    "entity:user relation:interested_in target:travel",
					CreatedAt:  now.Add(-70 * time.Minute),
					ModifiedAt: now.Add(-5 * time.Minute),
				},
				RelationScore: 0.5,
			},
		},
	}
	citations := &stubCitationSource{
		citations: map[string][]Citation{
			"mem_profile":  {{MemoryID: "mem_profile", RawEventID: "evt_profile", Operation: "promote_candidate"}},
			"mem_episode":  {{MemoryID: "mem_episode", RawEventID: "evt_episode", Operation: "promote_candidate"}},
			"mem_summary":  {{MemoryID: "mem_summary", RawEventID: "evt_summary", Operation: "create_summary_memory"}},
			"mem_relation": {{MemoryID: "mem_relation", RawEventID: "evt_relation", Operation: "promote_candidate"}},
		},
	}

	service := NewService(ServiceDependencies{
		Lexical:   lexical,
		Semantic:  semantic,
		Relations: relations,
		Citations: citations,
	})

	contextResult, err := service.AssembleContext(context.Background(), AssembleContextInput{
		Scope:            scope,
		Query:            "weekend travel",
		Budget:           4,
		IncludeRelations: true,
	})
	if err != nil {
		t.Fatalf("AssembleContext() error = %v", err)
	}

	if len(contextResult.Profile) != 1 {
		t.Fatalf("len(Profile) = %d, want %d", len(contextResult.Profile), 1)
	}

	if len(contextResult.RelevantSummaries) != 1 {
		t.Fatalf("len(RelevantSummaries) = %d, want %d", len(contextResult.RelevantSummaries), 1)
	}

	if len(contextResult.RelatedEntities) != 1 {
		t.Fatalf("len(RelatedEntities) = %d, want %d", len(contextResult.RelatedEntities), 1)
	}

	if len(contextResult.Citations) < 3 {
		t.Fatalf("len(Citations) = %d, want at least %d", len(contextResult.Citations), 3)
	}
}

func TestServiceSearchAppliesSummaryRelationAndClassFilters(t *testing.T) {
	scope := memory.Scope{
		Tenant:    "tenant-a",
		Project:   "project-a",
		Namespace: "namespace-a",
	}

	service := NewService(ServiceDependencies{
		Lexical: &stubLexicalSource{
			hits: []ScoredMemory{
				{Memory: memory.CanonicalMemory{ID: "mem_profile", Scope: scope, Class: memory.MemoryClassProfile, State: memory.MemoryStateActive, Content: "profile"}, LexicalScore: 0.9},
				{Memory: memory.CanonicalMemory{ID: "mem_summary", Scope: scope, Class: memory.MemoryClassSummary, State: memory.MemoryStateActive, Content: "summary"}, LexicalScore: 0.8},
				{Memory: memory.CanonicalMemory{ID: "mem_relation", Scope: scope, Class: memory.MemoryClassRelation, State: memory.MemoryStateActive, Content: "relation"}, LexicalScore: 0.7},
			},
		},
	})

	result, err := service.Search(context.Background(), SearchInput{
		Scope:            scope,
		Query:            "profile",
		Classes:          []memory.MemoryClass{memory.MemoryClassProfile},
		IncludeSummaries: false,
		IncludeRelations: false,
	})
	if err != nil {
		t.Fatalf("Search() error = %v", err)
	}

	if len(result.Hits) != 1 {
		t.Fatalf("len(result.Hits) = %d, want %d", len(result.Hits), 1)
	}

	if result.Hits[0].Memory.Class != memory.MemoryClassProfile {
		t.Fatalf("result.Hits[0].Memory.Class = %q, want %q", result.Hits[0].Memory.Class, memory.MemoryClassProfile)
	}
}

func TestServiceAssembleContextHonorsBudgetAndKeepsSummary(t *testing.T) {
	scope := memory.Scope{
		Tenant:    "tenant-a",
		Project:   "project-a",
		Namespace: "namespace-a",
	}

	service := NewService(ServiceDependencies{
		Lexical: &stubLexicalSource{
			hits: []ScoredMemory{
				{Memory: memory.CanonicalMemory{ID: "mem_profile", Scope: scope, Class: memory.MemoryClassProfile, State: memory.MemoryStateActive, Content: "profile"}, LexicalScore: 0.9},
				{Memory: memory.CanonicalMemory{ID: "mem_episode_1", Scope: scope, Class: memory.MemoryClassEpisodic, State: memory.MemoryStateActive, Content: "episode 1"}, LexicalScore: 0.8},
				{Memory: memory.CanonicalMemory{ID: "mem_episode_2", Scope: scope, Class: memory.MemoryClassEpisodic, State: memory.MemoryStateActive, Content: "episode 2"}, LexicalScore: 0.7},
			},
		},
		Semantic: &stubSemanticSource{
			hits: []ScoredMemory{
				{Memory: memory.CanonicalMemory{ID: "mem_summary", Scope: scope, Class: memory.MemoryClassSummary, State: memory.MemoryStateActive, Content: "summary"}, SemanticScore: 1.0},
			},
		},
	})

	contextResult, err := service.AssembleContext(context.Background(), AssembleContextInput{
		Scope:  scope,
		Query:  "travel",
		Budget: 2,
	})
	if err != nil {
		t.Fatalf("AssembleContext() error = %v", err)
	}

	if len(contextResult.RelevantSummaries) != 1 {
		t.Fatalf("len(RelevantSummaries) = %d, want %d", len(contextResult.RelevantSummaries), 1)
	}

	total := len(contextResult.Profile) + len(contextResult.RecentSession) + len(contextResult.RecentEpisodes) + len(contextResult.RelevantSummaries) + len(contextResult.RelatedEntities)
	if total > 2 {
		t.Fatalf("total assembled hits = %d, want <= %d", total, 2)
	}
}

func TestServiceSearchTopKUsesSortedHitsForCitationLookup(t *testing.T) {
	scope := memory.Scope{
		Tenant:    "tenant-a",
		Project:   "project-a",
		Namespace: "namespace-a",
	}

	service := NewService(ServiceDependencies{
		Lexical: &stubLexicalSource{
			hits: []ScoredMemory{
				{Memory: memory.CanonicalMemory{ID: "mem_low", Scope: scope, Class: memory.MemoryClassProfile, State: memory.MemoryStateActive, Content: "low"}, LexicalScore: 0.2},
			},
		},
		Semantic: &stubSemanticSource{
			hits: []ScoredMemory{
				{Memory: memory.CanonicalMemory{ID: "mem_high", Scope: scope, Class: memory.MemoryClassSummary, State: memory.MemoryStateActive, Content: "high"}, SemanticScore: 0.9},
			},
		},
		Citations: &stubCitationSource{
			citations: map[string][]Citation{
				"mem_high": {{MemoryID: "mem_high", RawEventID: "evt_high", Operation: "create_summary_memory"}},
			},
		},
	})

	result, err := service.Search(context.Background(), SearchInput{
		Scope:            scope,
		Query:            "high",
		TopK:             1,
		IncludeSummaries: true,
	})
	if err != nil {
		t.Fatalf("Search() error = %v", err)
	}

	if len(result.Hits) != 1 {
		t.Fatalf("len(result.Hits) = %d, want %d", len(result.Hits), 1)
	}

	if result.Hits[0].Memory.ID != "mem_high" {
		t.Fatalf("result.Hits[0].Memory.ID = %q, want %q", result.Hits[0].Memory.ID, "mem_high")
	}

	if len(result.Hits[0].Citations) != 1 || result.Hits[0].Citations[0].RawEventID != "evt_high" {
		t.Fatalf("result.Hits[0].Citations = %+v, want evt_high citation", result.Hits[0].Citations)
	}
}

func TestSearchInputValidateRejectsInvalidTimeWindow(t *testing.T) {
	err := (SearchInput{
		Scope: memory.Scope{
			Tenant:    "tenant-a",
			Project:   "project-a",
			Namespace: "namespace-a",
		},
		Query:    "travel",
		TimeFrom: time.Date(2026, 6, 6, 15, 0, 0, 0, time.UTC),
		TimeTo:   time.Date(2026, 6, 6, 14, 0, 0, 0, time.UTC),
	}).Validate()
	if err == nil {
		t.Fatal("Validate() error = nil, want invalid time window")
	}
}

func TestServiceAssembleContextPrefersSummaryOverExtraEpisodes(t *testing.T) {
	scope := memory.Scope{
		Tenant:    "tenant-a",
		Project:   "project-a",
		Namespace: "namespace-a",
	}

	service := NewService(ServiceDependencies{
		Lexical: &stubLexicalSource{
			hits: []ScoredMemory{
				{Memory: memory.CanonicalMemory{ID: "mem_profile", Scope: scope, Class: memory.MemoryClassProfile, State: memory.MemoryStateActive, Content: "profile"}, LexicalScore: 0.95},
				{Memory: memory.CanonicalMemory{ID: "mem_episode_1", Scope: scope, Class: memory.MemoryClassEpisodic, State: memory.MemoryStateActive, Content: "episode 1"}, LexicalScore: 0.90},
				{Memory: memory.CanonicalMemory{ID: "mem_episode_2", Scope: scope, Class: memory.MemoryClassEpisodic, State: memory.MemoryStateActive, Content: "episode 2"}, LexicalScore: 0.89},
			},
		},
		Semantic: &stubSemanticSource{
			hits: []ScoredMemory{
				{Memory: memory.CanonicalMemory{ID: "mem_summary", Scope: scope, Class: memory.MemoryClassSummary, State: memory.MemoryStateActive, Content: "summary"}, SemanticScore: 0.40},
			},
		},
	})

	contextResult, err := service.AssembleContext(context.Background(), AssembleContextInput{
		Scope:  scope,
		Query:  "travel",
		Budget: 2,
	})
	if err != nil {
		t.Fatalf("AssembleContext() error = %v", err)
	}

	if len(contextResult.Profile) != 1 {
		t.Fatalf("len(Profile) = %d, want %d", len(contextResult.Profile), 1)
	}

	if len(contextResult.RelevantSummaries) != 1 {
		t.Fatalf("len(RelevantSummaries) = %d, want %d", len(contextResult.RelevantSummaries), 1)
	}

	if len(contextResult.RecentSession)+len(contextResult.RecentEpisodes) != 0 {
		t.Fatalf("episodic sections = %d, want 0 after summary-preferred packing", len(contextResult.RecentSession)+len(contextResult.RecentEpisodes))
	}
}
