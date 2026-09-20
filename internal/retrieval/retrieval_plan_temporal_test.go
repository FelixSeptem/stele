package retrieval

import (
	"testing"
	"time"

	"github.com/FelixSeptem/stele/internal/memory"
)

func TestBuildRetrievalPlanCarriesExplicitTemporalConstraint(t *testing.T) {
	asOf := time.Date(2025, 6, 1, 0, 0, 0, 0, time.UTC)
	input := plannerInputForTest([]QueryAnalysisHint{{Kind: QueryAnalysisHintTemporal, Disposition: QueryAnalysisHintPresent, Value: "2025-06-01"}}, nil, false)
	input.TemporalConstraint = memory.TemporalConstraint{Mode: memory.TemporalSelectionAsOf, AsOf: &asOf}
	plan, err := BuildRetrievalPlan(input)
	if err != nil {
		t.Fatal(err)
	}
	if plan.Family != RetrievalQueryFamilyTemporal || plan.TemporalConstraint.Mode != memory.TemporalSelectionAsOf || plan.TemporalConstraint.AsOf == nil || !plan.TemporalConstraint.AsOf.Equal(asOf) {
		t.Fatalf("plan temporal constraint = %+v", plan)
	}
}

func TestBuildRetrievalPlanDefaultsToCurrentWhenTemporalSelectorMissing(t *testing.T) {
	input := plannerInputForTest([]QueryAnalysisHint{{Kind: QueryAnalysisHintTemporal, Disposition: QueryAnalysisHintPresent, Value: "recent"}}, nil, false)
	plan, err := BuildRetrievalPlan(input)
	if err != nil {
		t.Fatal(err)
	}
	if plan.TemporalConstraint.Mode != memory.TemporalSelectionCurrent {
		t.Fatalf("temporal mode = %q, want current", plan.TemporalConstraint.Mode)
	}
}

func TestBuildRetrievalPlanRejectsMalformedTemporalConstraint(t *testing.T) {
	input := plannerInputForTest(nil, nil, false)
	input.TemporalConstraint = memory.TemporalConstraint{Mode: memory.TemporalSelectionAsOf}
	if _, err := BuildRetrievalPlan(input); err == nil {
		t.Fatal("BuildRetrievalPlan() error = nil, want malformed temporal constraint rejection")
	}
}
