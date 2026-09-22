package retrieval

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/FelixSeptem/stele/internal/memory"
)

type graphTraversalRecorder struct {
	inputs []GraphTraversalInput
	paths  []GraphPathCandidate
	err    error
}

func (recorder *graphTraversalRecorder) ExpandGraph(_ context.Context, input GraphTraversalInput) ([]GraphPathCandidate, error) {
	recorder.inputs = append(recorder.inputs, input)
	return recorder.paths, recorder.err
}

func TestEffectiveGraphTraversalLimitsOnlyNarrowsDeploymentCap(t *testing.T) {
	deployment := DefaultGraphTraversalLimits()
	policy := memory.GraphTraversalPolicy{SchemaVersion: memory.GraphTraversalPolicySchemaVersionV1, PolicyVersion: "graph-policy-v1", EnabledFamilies: []string{"entity_relation"}, MaxHops: 2, MaxSeeds: 4, MaxElapsed: 100 * time.Millisecond}
	effective, err := EffectiveGraphTraversalLimits(deployment, policy)
	if err != nil {
		t.Fatalf("EffectiveGraphTraversalLimits() error = %v", err)
	}
	if effective.MaxHops != 2 || effective.MaxSeeds != 4 || effective.MaxEdgesPerHop != deployment.MaxEdgesPerHop || effective.MaxElapsed != 100*time.Millisecond {
		t.Fatalf("effective limits = %+v", effective)
	}
	policy.MaxHops = 4
	if _, err := EffectiveGraphTraversalLimits(deployment, policy); err == nil {
		t.Fatal("expected policy over hard hop cap to fail")
	}
}

func TestEffectiveGraphTraversalLimitsDistinguishesDefaultFromExplicitZeroHop(t *testing.T) {
	deployment := DefaultGraphTraversalLimits()
	defaultPolicy := memory.GraphTraversalPolicy{SchemaVersion: memory.GraphTraversalPolicySchemaVersionV1, PolicyVersion: "default", EnabledFamilies: []string{"entity_relation"}}
	effective, err := EffectiveGraphTraversalLimits(deployment, defaultPolicy)
	if err != nil {
		t.Fatalf("default policy error = %v", err)
	}
	if effective.MaxHops != 1 {
		t.Fatalf("default policy max hops = %d, want 1", effective.MaxHops)
	}
	zeroPolicy := defaultPolicy
	zeroPolicy.PolicyVersion = "zero"
	zeroPolicy.HopsSet = true
	zeroPolicy.MaxHops = 0
	effective, err = EffectiveGraphTraversalLimits(deployment, zeroPolicy)
	if err != nil {
		t.Fatalf("explicit zero-hop policy error = %v", err)
	}
	if effective.MaxHops != 0 {
		t.Fatalf("explicit zero-hop max hops = %d, want 0", effective.MaxHops)
	}
}

func TestEffectiveGraphTraversalLimitsAcceptsConfiguredThreeHopCap(t *testing.T) {
	deployment := DefaultGraphTraversalLimits()
	policy := memory.GraphTraversalPolicy{SchemaVersion: memory.GraphTraversalPolicySchemaVersionV1, PolicyVersion: "three", EnabledFamilies: []string{"multi_hop"}, HopsSet: true, MaxHops: 3}
	effective, err := EffectiveGraphTraversalLimits(deployment, policy)
	if err != nil {
		t.Fatalf("three-hop policy error = %v", err)
	}
	if effective.MaxHops != 3 {
		t.Fatalf("three-hop effective limit = %d", effective.MaxHops)
	}
}

func TestSortGraphPathCandidatesUsesDeterministicPriority(t *testing.T) {
	candidates := []GraphPathCandidate{
		{Memory: memory.CanonicalMemory{ID: "b"}, Proof: GraphPathProof{SeedID: "s", Hop: 2}, RelationConfidence: 1},
		{Memory: memory.CanonicalMemory{ID: "a"}, Proof: GraphPathProof{SeedID: "s", Hop: 1}, RelationConfidence: 0.1},
	}
	SortGraphPathCandidates(candidates)
	if candidates[0].Memory.ID != "a" {
		t.Fatalf("sorted candidates = %+v", candidates)
	}
}

func TestSearchActiveEntityRelationPlanAddsGraphEndpointToRelationChannel(t *testing.T) {
	scope := memory.Scope{Tenant: "tenant", Project: "project", Namespace: "graph-active"}
	limits := DefaultQueryAnalysisLimits()
	analysisInput := QueryAnalysisInput{AcceptedQuery: "who knows alice", PolicyVersion: QueryAnalysisPolicyVersionV1, Limits: limits}
	analysis, err := NewQueryAnalysisResult(analysisInput, QueryAnalysisDispositionComplete, []QueryAnalysisHint{{Kind: QueryAnalysisHintEntity, Disposition: QueryAnalysisHintPresent, Value: "alice"}}, nil)
	if err != nil {
		t.Fatal(err)
	}
	plannerRollout := plannerTestRollout(scope, memory.RankingRolloutSurfaceSearch, memory.RankingRolloutPolicyStatusActiveForScope)
	plannerRollout.RetrievalPlanner.GraphTraversal = &memory.GraphTraversalPolicy{SchemaVersion: memory.GraphTraversalPolicySchemaVersionV1, PolicyVersion: "graph-policy-v1", EnabledFamilies: []string{"entity_relation"}}
	relations := &plannerChannelRecorder{hits: [][]ScoredMemory{{plannerTestHit(scope, "seed", memory.MemoryClassRelation)}}}
	graph := &graphTraversalRecorder{paths: []GraphPathCandidate{{Memory: plannerTestHit(scope, "endpoint", memory.MemoryClassRelation).Memory, Proof: GraphPathProof{SeedID: "seed", EdgeIDs: []string{"edge"}, SourceVersionIDs: []string{"1"}, RelationCategories: []string{"knows"}, Hop: 1}, RelationConfidence: 1}}}
	policy := plannerTestPolicy(func(template *RetrievalPlanTemplate) {
		template.Channels = []FusionChannel{FusionChannelRelation}
		template.ChannelCandidates = map[FusionChannel]int{FusionChannelRelation: 4}
		template.TotalCandidates = 4
	})
	service := NewService(ServiceDependencies{Relations: relations, GraphTraversal: graph, QueryAnalyzer: orchestrationAnalyzer{result: analysis}, QueryAnalysisLimits: limits, RankingRolloutPolicyReader: plannerPolicyReader{policy: plannerRollout, qaPolicy: ptrPolicy(activeQAPolicy(scope, limits))}, RetrievalPlanPolicy: policy})
	result, err := service.Search(context.Background(), SearchInput{Scope: scope, Query: "who knows alice", IncludeRelations: true, TopK: 10})
	if err != nil {
		t.Fatal(err)
	}
	if len(graph.inputs) != 1 || graph.inputs[0].Limits.MaxHops != 1 || len(graph.inputs[0].SeedMemoryIDs) != 1 || graph.inputs[0].SeedMemoryIDs[0] != "seed" {
		t.Fatalf("graph inputs = %+v", graph.inputs)
	}
	if got := hitIDs(result.Hits); len(got) != 2 || got[0] != "seed" || got[1] != "endpoint" {
		t.Fatalf("hits = %v", got)
	}
}

func TestSearchDiagnosticsGraphTraversalComputesButPreservesBaseline(t *testing.T) {
	scope := memory.Scope{Tenant: "tenant", Project: "project", Namespace: "graph-diagnostics"}
	limits := DefaultQueryAnalysisLimits()
	analysisInput := QueryAnalysisInput{AcceptedQuery: "who knows alice", PolicyVersion: QueryAnalysisPolicyVersionV1, Limits: limits}
	analysis, err := NewQueryAnalysisResult(analysisInput, QueryAnalysisDispositionComplete, []QueryAnalysisHint{{Kind: QueryAnalysisHintEntity, Disposition: QueryAnalysisHintPresent, Value: "alice"}}, nil)
	if err != nil {
		t.Fatal(err)
	}
	plannerRollout := plannerTestRollout(scope, memory.RankingRolloutSurfaceSearch, memory.RankingRolloutPolicyStatusDiagnosticsOnly)
	plannerRollout.RetrievalPlanner.GraphTraversal = &memory.GraphTraversalPolicy{SchemaVersion: memory.GraphTraversalPolicySchemaVersionV1, PolicyVersion: "graph-policy-diagnostics", EnabledFamilies: []string{"entity_relation"}}
	relations := &plannerChannelRecorder{hits: [][]ScoredMemory{{plannerTestHit(scope, "seed", memory.MemoryClassRelation)}}}
	graph := &graphTraversalRecorder{paths: []GraphPathCandidate{{Memory: plannerTestHit(scope, "endpoint", memory.MemoryClassRelation).Memory, Proof: GraphPathProof{SeedID: "seed", EdgeIDs: []string{"edge"}, SourceVersionIDs: []string{"1"}, RelationCategories: []string{"knows"}, Hop: 1}, RelationConfidence: 1}}}
	policy := plannerTestPolicy(func(template *RetrievalPlanTemplate) {
		template.Channels = []FusionChannel{FusionChannelRelation}
		template.ChannelCandidates = map[FusionChannel]int{FusionChannelRelation: 4}
		template.TotalCandidates = 4
	})
	service := NewService(ServiceDependencies{Relations: relations, GraphTraversal: graph, QueryAnalyzer: orchestrationAnalyzer{result: analysis}, QueryAnalysisLimits: limits, RankingRolloutPolicyReader: plannerPolicyReader{policy: plannerRollout, qaPolicy: ptrPolicy(activeQAPolicy(scope, limits))}, RetrievalPlanPolicy: policy})
	result, err := service.Search(context.Background(), SearchInput{Scope: scope, Query: "who knows alice", IncludeRelations: true, TopK: 10})
	if err != nil {
		t.Fatal(err)
	}
	if len(graph.inputs) != 1 {
		t.Fatalf("diagnostics graph calls = %d, want 1", len(graph.inputs))
	}
	if got := hitIDs(result.Hits); len(got) != 1 || got[0] != "seed" {
		t.Fatalf("diagnostics hits = %v, want baseline seed only", got)
	}
}

func TestSearchActiveGraphTraversalFailurePreservesRelationBaseline(t *testing.T) {
	scope := memory.Scope{Tenant: "tenant", Project: "project", Namespace: "graph-failure"}
	limits := DefaultQueryAnalysisLimits()
	analysisInput := QueryAnalysisInput{AcceptedQuery: "who knows alice", PolicyVersion: QueryAnalysisPolicyVersionV1, Limits: limits}
	analysis, err := NewQueryAnalysisResult(analysisInput, QueryAnalysisDispositionComplete, []QueryAnalysisHint{{Kind: QueryAnalysisHintEntity, Disposition: QueryAnalysisHintPresent, Value: "alice"}}, nil)
	if err != nil {
		t.Fatal(err)
	}
	plannerRollout := plannerTestRollout(scope, memory.RankingRolloutSurfaceSearch, memory.RankingRolloutPolicyStatusActiveForScope)
	plannerRollout.RetrievalPlanner.GraphTraversal = &memory.GraphTraversalPolicy{SchemaVersion: memory.GraphTraversalPolicySchemaVersionV1, PolicyVersion: "graph-policy-failure", EnabledFamilies: []string{"entity_relation"}}
	relations := &plannerChannelRecorder{hits: [][]ScoredMemory{{plannerTestHit(scope, "seed", memory.MemoryClassRelation)}}}
	graph := &graphTraversalRecorder{err: errors.New("database unavailable")}
	policy := plannerTestPolicy(func(template *RetrievalPlanTemplate) {
		template.Channels = []FusionChannel{FusionChannelRelation}
		template.ChannelCandidates = map[FusionChannel]int{FusionChannelRelation: 4}
		template.TotalCandidates = 4
	})
	service := NewService(ServiceDependencies{Relations: relations, GraphTraversal: graph, QueryAnalyzer: orchestrationAnalyzer{result: analysis}, QueryAnalysisLimits: limits, RankingRolloutPolicyReader: plannerPolicyReader{policy: plannerRollout, qaPolicy: ptrPolicy(activeQAPolicy(scope, limits))}, RetrievalPlanPolicy: policy})
	result, err := service.Search(context.Background(), SearchInput{Scope: scope, Query: "who knows alice", IncludeRelations: true, TopK: 10})
	if err != nil {
		t.Fatal(err)
	}
	if got := hitIDs(result.Hits); len(got) != 1 || got[0] != "seed" {
		t.Fatalf("failure fallback hits = %v", got)
	}
}
