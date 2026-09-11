package retrieval

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/FelixSeptem/stele/internal/memory"
	"github.com/FelixSeptem/stele/internal/telemetry"
)

func TestServiceSearchUsesDefaultRRFFusionAcrossChannels(t *testing.T) {
	scope := memory.Scope{Tenant: "tenant-a", Project: "project-a", Namespace: "namespace-a"}
	shared := memory.CanonicalMemory{ID: "mem_shared", Scope: scope, Class: memory.MemoryClassProfile, State: memory.MemoryStateActive}
	service := NewService(ServiceDependencies{
		Lexical: &stubLexicalSource{hits: []ScoredMemory{
			{Memory: shared, LexicalScore: 0.1},
			{Memory: memory.CanonicalMemory{ID: "mem_lexical", Scope: scope, Class: memory.MemoryClassEpisodic, State: memory.MemoryStateActive}, LexicalScore: 1},
		}},
		Semantic: &stubSemanticSource{hits: []ScoredMemory{
			{Memory: memory.CanonicalMemory{ID: "mem_semantic", Scope: scope, Class: memory.MemoryClassEpisodic, State: memory.MemoryStateActive}, SemanticScore: 1},
			{Memory: shared, SemanticScore: 0.1},
		}},
	})

	result, err := service.Search(context.Background(), SearchInput{Scope: scope, Query: "shared", TopK: 3})
	if err != nil {
		t.Fatalf("Search() error = %v", err)
	}
	if len(result.Hits) != 3 || result.Hits[0].Memory.ID != "mem_shared" {
		t.Fatalf("hits = %+v, want shared multi-channel candidate first", result.Hits)
	}
	if result.Hits[0].Score.Overall <= result.Hits[1].Score.Overall {
		t.Fatalf("fused scores = %v then %v, want descending", result.Hits[0].Score.Overall, result.Hits[1].Score.Overall)
	}
}

func TestServiceSearchDeduplicatesEquivalentVisibleEvidenceAfterFusion(t *testing.T) {
	scope := memory.Scope{Tenant: "tenant-a", Project: "project-a", Namespace: "namespace-a"}
	first := memory.CanonicalMemory{ID: "mem-first", Scope: scope, Class: memory.MemoryClassEpisodic, State: memory.MemoryStateActive}
	second := memory.CanonicalMemory{ID: "mem-second", Scope: scope, Class: memory.MemoryClassEpisodic, State: memory.MemoryStateActive}
	service := NewService(ServiceDependencies{
		Lexical:  &stubLexicalSource{hits: []ScoredMemory{{Memory: first, LexicalScore: 1}, {Memory: second, LexicalScore: 0.9}}},
		Semantic: &stubSemanticSource{hits: []ScoredMemory{{Memory: second, SemanticScore: 1}}},
	})
	// The current searcher contract does not expose lineage metadata; same
	// canonical IDs are still deduplicated by the post-fusion selector. This
	// regression ensures ordinary responses retain one identity per memory.
	result, err := service.Search(context.Background(), SearchInput{Scope: scope, Query: "evidence", TopK: 3})
	if err != nil {
		t.Fatalf("Search() error = %v", err)
	}
	seen := map[string]struct{}{}
	for _, hit := range result.Hits {
		if _, exists := seen[hit.Memory.ID]; exists {
			t.Fatalf("duplicate public memory id %q in hits = %+v", hit.Memory.ID, result.Hits)
		}
		seen[hit.Memory.ID] = struct{}{}
	}
}

func TestServiceSearchActiveDiversityPolicyIsRedactedFromOrdinaryResponse(t *testing.T) {
	scope := memory.Scope{Tenant: "tenant-a", Project: "project-a", Namespace: "namespace-a"}
	policy := memory.RankingRolloutPolicy{
		ID: "diversity-policy", Scope: scope, Status: memory.RankingRolloutPolicyStatusActiveForScope, Mode: memory.RankingRolloutModeActiveForScope,
		Surfaces: []memory.RankingRolloutSurface{memory.RankingRolloutSurfaceSearch}, SignalSources: []memory.RankingRolloutSignalSource{memory.RankingRolloutSignalSourceQualityFindings}, ThresholdStatus: memory.RankingRolloutThresholdStatusSatisfied,
		Actor: "operator", Reason: "approved", CreatedAt: time.Now().UTC(), UpdatedAt: time.Now().UTC(),
		DiversityPolicyName: "mmr", DiversityPolicyVersion: "mmr-v1", DiversityMMRLambda: 0.5, DiversitySemanticThreshold: 0.9, DiversityMaxCandidates: 10, DiversityMaxPairwiseComparisons: 20, DiversityMaxEmbeddingDimensions: 3, DiversityMaxCitationsPerCandidate: 4,
		DiversityCoverageWeights: map[string]float64{"memory_class": 0.1, "session": 0.1, "entity": 0.1, "time_slice": 0.1, "unknown": 0},
	}
	service := NewService(ServiceDependencies{
		Lexical:                    &stubLexicalSource{hits: []ScoredMemory{{Memory: memory.CanonicalMemory{ID: "mem-1", Scope: scope, Class: memory.MemoryClassProfile, State: memory.MemoryStateActive}, LexicalScore: 1}}},
		RankingRolloutPolicyReader: &stubRankingRolloutPolicyReader{policy: policy},
	})
	result, err := service.Search(context.Background(), SearchInput{Scope: scope, Query: "profile"})
	if err != nil {
		t.Fatalf("Search() error = %v", err)
	}
	for _, diagnostic := range result.Diagnostics {
		if strings.Contains(diagnostic.Section, "diversity") || strings.Contains(diagnostic.Reason, "mmr") || strings.Contains(diagnostic.Reason, "diversity") {
			t.Fatalf("ordinary diagnostics disclose diversity internals = %+v", result.Diagnostics)
		}
	}
	if len(result.Hits) != 1 || result.Hits[0].Memory.ID != "mem-1" {
		t.Fatalf("hits = %+v, want public baseline hit", result.Hits)
	}
}

func TestServiceSearchRejectsMalformedActiveFusionPolicy(t *testing.T) {
	scope := memory.Scope{Tenant: "tenant-a", Project: "project-a", Namespace: "namespace-a"}
	service := NewService(ServiceDependencies{
		Lexical: &stubLexicalSource{hits: []ScoredMemory{{Memory: memory.CanonicalMemory{ID: "mem-1", Scope: scope, Class: memory.MemoryClassProfile, State: memory.MemoryStateActive}, LexicalScore: 1}}},
		RankingRolloutPolicyReader: &stubRankingRolloutPolicyReader{policy: memory.RankingRolloutPolicy{
			ID: "policy-1", Scope: scope,
			Status:         memory.RankingRolloutPolicyStatusActiveForScope,
			Mode:           memory.RankingRolloutModeActiveForScope,
			FusionStrategy: "rrf", FusionRankConstant: 60,
			FusionPerChannelCandidate: 10, FusionTotalCandidates: 10,
			FusionChannelWeights: map[string]float64{"lexical": 1},
		}},
	})
	_, err := service.Search(context.Background(), SearchInput{Scope: scope, Query: "profile", TopK: 1})
	if err == nil {
		t.Fatal("Search() error = nil, want malformed fusion policy rejection")
	}
}

func TestServiceSearchRejectsForeignScopeFusionPolicy(t *testing.T) {
	scope := memory.Scope{Tenant: "tenant-a", Project: "project-a", Namespace: "namespace-a"}
	foreignScope := memory.Scope{Tenant: "tenant-b", Project: "project-a", Namespace: "namespace-a"}
	service := NewService(ServiceDependencies{
		Lexical: &stubLexicalSource{hits: []ScoredMemory{{Memory: memory.CanonicalMemory{ID: "mem-1", Scope: scope, Class: memory.MemoryClassProfile, State: memory.MemoryStateActive}, LexicalScore: 1}}},
		RankingRolloutPolicyReader: &stubRankingRolloutPolicyReader{policy: memory.RankingRolloutPolicy{
			ID: "foreign-policy", Scope: foreignScope,
			Status:         memory.RankingRolloutPolicyStatusActiveForScope,
			Mode:           memory.RankingRolloutModeActiveForScope,
			FusionStrategy: "rrf", FusionVersion: "rrf-v1", FusionRankConstant: 60,
			FusionPerChannelCandidate: 10, FusionTotalCandidates: 10,
			FusionChannelWeights: map[string]float64{"lexical": 1},
		}},
	})

	_, err := service.Search(context.Background(), SearchInput{Scope: scope, Query: "profile", TopK: 1})
	if err == nil {
		t.Fatal("Search() error = nil, want foreign-scope fusion policy rejection")
	}
}

func TestServiceAssembleContextUsesActiveContextFusionPolicy(t *testing.T) {
	scope := memory.Scope{Tenant: "tenant-a", Project: "project-a", Namespace: "namespace-a"}
	lexical := memory.CanonicalMemory{ID: "mem-lexical", Scope: scope, Class: memory.MemoryClassProfile, State: memory.MemoryStateActive}
	semantic := memory.CanonicalMemory{ID: "mem-semantic", Scope: scope, Class: memory.MemoryClassProfile, State: memory.MemoryStateActive}
	service := NewService(ServiceDependencies{
		Lexical:  &stubLexicalSource{hits: []ScoredMemory{{Memory: lexical, LexicalScore: 1}}},
		Semantic: &stubSemanticSource{hits: []ScoredMemory{{Memory: semantic, SemanticScore: 1}}},
		RankingRolloutPolicyReader: &stubRankingRolloutPolicyReader{policy: memory.RankingRolloutPolicy{
			ID: "policy-context", Scope: scope,
			Status:         memory.RankingRolloutPolicyStatusActiveForScope,
			Mode:           memory.RankingRolloutModeActiveForScope,
			FusionStrategy: "normalized_weighted", FusionVersion: "normalized-weighted-v1", FusionRankConstant: 60,
			FusionPerChannelCandidate: 10, FusionTotalCandidates: 10,
			FusionChannelWeights: map[string]float64{"lexical": 0, "semantic": 1},
		}},
	})
	result, err := service.AssembleContext(context.Background(), AssembleContextInput{Scope: scope, Query: "profile", Budget: 2})
	if err != nil {
		t.Fatalf("AssembleContext() error = %v", err)
	}
	if len(result.Profile) == 0 || result.Profile[0].Memory.ID != semantic.ID {
		t.Fatalf("profile = %+v, want semantic fusion candidate first", result.Profile)
	}
}

func TestServiceSearchDegradesWhenOptionalRelationChannelFails(t *testing.T) {
	scope := memory.Scope{Tenant: "tenant-a", Project: "project-a", Namespace: "namespace-a"}
	service := NewService(ServiceDependencies{
		Lexical:   &stubLexicalSource{hits: []ScoredMemory{{Memory: memory.CanonicalMemory{ID: "mem_1", Scope: scope, Class: memory.MemoryClassProfile, State: memory.MemoryStateActive}, LexicalScore: 1}}},
		Relations: &stubRelationSource{err: errors.New("relation backend unavailable")},
	})

	result, err := service.Search(context.Background(), SearchInput{Scope: scope, Query: "profile", IncludeRelations: true, IncludeFeedbackDiagnostics: true})
	if err != nil {
		t.Fatalf("Search() error = %v, want optional relation degradation", err)
	}
	if len(result.Hits) != 1 || result.Hits[0].Memory.ID != "mem_1" {
		t.Fatalf("hits = %+v, want lexical fallback", result.Hits)
	}
	if !hasContextDiagnostic(result.Diagnostics, "fusion", "optional_channel_unavailable") {
		t.Fatalf("diagnostics = %+v, want bounded optional channel status", result.Diagnostics)
	}
	if result.fusionChannelAvailability[FusionChannelLexical] != fusionChannelAvailable || result.fusionChannelAvailability[FusionChannelRelation] != fusionChannelUnavailable {
		t.Fatalf("channel availability = %+v, want lexical available and relation unavailable", result.fusionChannelAvailability)
	}
}

func TestServiceSearchEmitsBoundedFusionTelemetryForOptionalChannelFailure(t *testing.T) {
	scope := memory.Scope{Tenant: "tenant-a", Project: "project-a", Namespace: "namespace-a"}
	observer := telemetry.NewMetricsObserver()
	service := NewService(ServiceDependencies{
		Lexical:   &stubLexicalSource{hits: []ScoredMemory{{Memory: memory.CanonicalMemory{ID: "mem-1", Scope: scope, Class: memory.MemoryClassProfile, State: memory.MemoryStateActive}, LexicalScore: 1}}},
		Relations: &stubRelationSource{err: errors.New("relation backend unavailable")},
	}, observer)

	if _, err := service.Search(context.Background(), SearchInput{Scope: scope, Query: "sensitive query", IncludeRelations: true}); err != nil {
		t.Fatalf("Search() error = %v", err)
	}
	metrics := observer.RenderPrometheus()
	for _, want := range []string{
		`stele_retrieval_fusion_total{availability="available",candidate_count="1_10",channel="lexical",outcome="fused",strategy="rrf",version="rrf-v1"} 1`,
		`stele_retrieval_fusion_total{availability="unavailable",candidate_count="0",channel="relation",outcome="fallback",strategy="rrf",version="rrf-v1"} 1`,
	} {
		if !strings.Contains(metrics, want) {
			t.Fatalf("metrics missing %q\n%s", want, metrics)
		}
	}
	for _, forbidden := range []string{"tenant-a", "project-a", "namespace-a", "sensitive query", "mem-1", "policy_id"} {
		if strings.Contains(metrics, forbidden) {
			t.Fatalf("metrics contain prohibited value %q\n%s", forbidden, metrics)
		}
	}
}
