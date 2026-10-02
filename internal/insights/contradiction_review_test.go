package insights

import (
	"testing"
	"time"

	"github.com/FelixSeptem/stele/internal/memory"
)

func TestApplyContradictionReviewPreservesSourceAndSuppressesCoexistence(t *testing.T) {
	now := time.Date(2026, 9, 29, 0, 0, 0, 0, time.UTC)
	insight := memory.DerivedInsight{ID: "contradiction-1", Scope: memory.Scope{Tenant: "t", Project: "p", Namespace: "n"}, Type: memory.DerivedInsightTypeContradiction, State: memory.DerivedInsightStateActive, CreatedAt: now, UpdatedAt: now}
	transition, err := ApplyContradictionReview(insight, ContradictionReviewCoexists, "operator", "facts are valid in separate periods")
	if err != nil {
		t.Fatalf("ApplyContradictionReview() error = %v", err)
	}
	if transition.ToState != memory.DerivedInsightStateSuppressed || transition.Metadata["contradiction_review_state"] != string(ContradictionReviewCoexists) {
		t.Fatalf("transition = %+v", transition)
	}
	if transition.InsightID != insight.ID || transition.Scope != insight.Scope {
		t.Fatalf("transition lost source identity: %+v", transition)
	}
}
