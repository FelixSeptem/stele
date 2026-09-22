package retrieval

import (
	"testing"

	"github.com/FelixSeptem/stele/internal/memory"
)

func TestBuildRetrievalPlanCarriesGraphPolicyOnlyForSupportedFamily(t *testing.T) {
	input := plannerInputForTest([]QueryAnalysisHint{{Kind: QueryAnalysisHintEntity, Disposition: QueryAnalysisHintPresent, Value: "person"}}, nil, true)
	input.GraphTraversalPolicy = &memory.GraphTraversalPolicy{SchemaVersion: memory.GraphTraversalPolicySchemaVersionV1, PolicyVersion: "graph-policy-v1", EnabledFamilies: []string{"entity_relation"}, MaxHops: 2}
	input.GraphTraversalLimits = DefaultGraphTraversalLimits()
	plan, err := BuildRetrievalPlan(input)
	if err != nil {
		t.Fatalf("BuildRetrievalPlan() error = %v", err)
	}
	if plan.Family != RetrievalQueryFamilyEntityRelation || plan.GraphTraversalPolicy == nil || plan.Identity.GraphPolicyVersion != "graph-policy-v1" || plan.Identity.GraphHops != 2 {
		t.Fatalf("plan graph fields = %+v", plan)
	}
	semanticInput := plannerInputForTest(nil, nil, true)
	semanticInput.GraphTraversalPolicy = input.GraphTraversalPolicy
	semanticInput.GraphTraversalLimits = DefaultGraphTraversalLimits()
	semanticPlan, err := BuildRetrievalPlan(semanticInput)
	if err != nil {
		t.Fatalf("semantic BuildRetrievalPlan() error = %v", err)
	}
	if semanticPlan.GraphTraversalPolicy != nil {
		t.Fatalf("semantic plan unexpectedly enabled graph policy: %+v", semanticPlan.GraphTraversalPolicy)
	}
}

func TestBuildRetrievalPlanRejectsGraphPolicyOverHardLimit(t *testing.T) {
	input := plannerInputForTest([]QueryAnalysisHint{{Kind: QueryAnalysisHintEntity, Disposition: QueryAnalysisHintPresent, Value: "person"}}, nil, true)
	input.GraphTraversalPolicy = &memory.GraphTraversalPolicy{SchemaVersion: memory.GraphTraversalPolicySchemaVersionV1, PolicyVersion: "graph-policy-v1", EnabledFamilies: []string{"entity_relation"}, MaxHops: 4}
	input.GraphTraversalLimits = DefaultGraphTraversalLimits()
	if _, err := BuildRetrievalPlan(input); err == nil {
		t.Fatal("expected graph policy over hard limit to fail")
	}
}

func TestBuildRetrievalPlanPreservesExplicitZeroHopDisposition(t *testing.T) {
	input := plannerInputForTest([]QueryAnalysisHint{{Kind: QueryAnalysisHintEntity, Disposition: QueryAnalysisHintPresent, Value: "person"}}, nil, true)
	input.GraphTraversalPolicy = &memory.GraphTraversalPolicy{
		SchemaVersion:   memory.GraphTraversalPolicySchemaVersionV1,
		PolicyVersion:   "graph-zero-v1",
		EnabledFamilies: []string{"entity_relation"},
		HopsSet:         true,
		MaxHops:         0,
	}
	input.GraphTraversalLimits = DefaultGraphTraversalLimits()
	plan, err := BuildRetrievalPlan(input)
	if err != nil {
		t.Fatalf("BuildRetrievalPlan() error = %v", err)
	}
	if plan.GraphTraversalPolicy == nil || plan.Identity.GraphHops != 0 || plan.GraphTraversalLimits.MaxHops != 0 {
		t.Fatalf("zero-hop graph plan = %+v", plan)
	}
}
