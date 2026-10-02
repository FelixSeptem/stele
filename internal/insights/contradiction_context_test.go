package insights

import (
	"testing"
	"time"

	"github.com/FelixSeptem/stele/internal/memory"
)

func TestAuthorizedContradictionContextExcludesShadowUnresolvedAndForeignRecords(t *testing.T) {
	scope := memory.Scope{Tenant: "t", Project: "p", Namespace: "n"}
	now := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	base := memory.DerivedInsight{ID: "base", Scope: scope, Type: memory.DerivedInsightTypeContradiction, State: memory.DerivedInsightStateActive, UpdatedAt: now, Derivation: memory.DerivedInsightDerivation{Metadata: map[string]any{"contradiction_review_state": "confirmed", "contradiction_temporal_disposition": "contradiction", "source_watermark": "w1"}}}
	candidates := []memory.DerivedInsight{
		base,
		{ID: "shadow", Scope: scope, Type: memory.DerivedInsightTypeContradiction, State: memory.DerivedInsightStateCandidate, UpdatedAt: now, Derivation: base.Derivation},
		{ID: "unresolved", Scope: scope, Type: memory.DerivedInsightTypeContradiction, State: memory.DerivedInsightStateActive, UpdatedAt: now, Derivation: memory.DerivedInsightDerivation{Metadata: map[string]any{"contradiction_review_state": "confirmed", "contradiction_temporal_disposition": "unresolved_temporal", "source_watermark": "w1"}}},
		{ID: "foreign", Scope: memory.Scope{Tenant: "other", Project: "p", Namespace: "n"}, Type: memory.DerivedInsightTypeContradiction, State: memory.DerivedInsightStateActive, UpdatedAt: now, Derivation: base.Derivation},
	}
	got := AuthorizedContradictionContext(ContradictionContextRequest{Scope: scope, Authorized: true, Now: now, SourceWatermark: "w1", MaxItems: 4}, candidates)
	if len(got) != 1 || got[0].ID != "base" {
		t.Fatalf("authorized context = %+v, want only fresh confirmed overlap", got)
	}
}
