package memory

import "testing"

func TestCompactionEvidenceValidationAndProjectionEligibility(t *testing.T) {
	scope := Scope{Tenant: "t", Project: "p", Namespace: "n"}
	evidence := CompactionEvidence{
		ID: "compact-1", Scope: scope, Trigger: "pressure",
		SourceWatermark:   ContextProjectionWatermark{RawEventIDs: []string{"event-1"}},
		DerivationVersion: "summarizer-v1", SummaryVersion: "summary-v2",
		EvidenceCoverage: 1, State: CompactionEvidenceStateActive,
		CanonicalVersionRefs: []ContextProjectionSource{{Kind: ContextProjectionSourceCanonicalVersion, ID: "version-1", Version: 1, Scope: scope}},
		RecentTailReferences: []ContextProjectionSource{{Kind: ContextProjectionSourceRawEvent, ID: "event-1", Scope: scope}},
	}
	if err := evidence.Validate(); err != nil {
		t.Fatalf("Validate() error = %v", err)
	}
	if !evidence.ProjectionEligible(scope) {
		t.Fatal("active complete evidence should be projection eligible")
	}
	for _, state := range []CompactionEvidenceState{CompactionEvidenceStateStale, CompactionEvidenceStateFailed, CompactionEvidenceStateSuperseded} {
		evidence.State = state
		if evidence.ProjectionEligible(scope) {
			t.Fatalf("state %q should fail closed", state)
		}
	}
	evidence.State = CompactionEvidenceStateActive
	evidence.Scope = Scope{Tenant: "other", Project: "p", Namespace: "n"}
	if evidence.ProjectionEligible(scope) {
		t.Fatal("foreign scope evidence should fail closed")
	}
}

func TestCompactionEvidenceValidationBoundsRecentTail(t *testing.T) {
	scope := Scope{Tenant: "t", Project: "p", Namespace: "n"}
	evidence := CompactionEvidence{ID: "compact-1", Scope: scope, Trigger: "manual", SourceWatermark: ContextProjectionWatermark{RawEventIDs: []string{"event"}}, DerivationVersion: "d1", SummaryVersion: "s1", EvidenceCoverage: 1, State: CompactionEvidenceStateActive}
	for i := 0; i < MaxCompactionRecentTailReferences+1; i++ {
		evidence.RecentTailReferences = append(evidence.RecentTailReferences, ContextProjectionSource{Kind: ContextProjectionSourceRawEvent, ID: "event", Scope: scope})
	}
	if err := evidence.Validate(); err == nil {
		t.Fatal("expected recent-tail bound validation error")
	}
}

func TestCompactionEvidenceRejectsHiddenSourceLifecycle(t *testing.T) {
	scope := Scope{Tenant: "t", Project: "p", Namespace: "n"}
	evidence := CompactionEvidence{
		ID: "compact-1", Scope: scope, Trigger: "pressure", SourceWatermark: ContextProjectionWatermark{CanonicalVersionIDs: []string{"version-1"}}, DerivationVersion: "d1", SummaryVersion: "s1", EvidenceCoverage: 1, State: CompactionEvidenceStateActive,
		CanonicalVersionRefs: []ContextProjectionSource{{Kind: ContextProjectionSourceCanonicalVersion, ID: "version-1", Version: 1, Scope: scope, LifecycleState: MemoryStateForgotten}},
	}
	if err := evidence.Validate(); err == nil {
		t.Fatal("expected hidden source lifecycle to fail validation")
	}
}
