package retrieval

import (
	"testing"
	"time"
)

func TestContextEfficiencyEvidenceRejectsOutOfRangeMetrics(t *testing.T) {
	evidence := ContextEfficiencyEvidence{
		Identity:    ContextEfficiencyIdentity{FixtureVersion: "fixture-v1", RepresentationVersion: "representation-v1"},
		Metrics:     ContextEfficiencyMetrics{RelevantTokenRatio: 1.1},
		GeneratedAt: time.Unix(1, 0).UTC(),
	}
	if err := evidence.Validate(); err == nil {
		t.Fatal("Validate() error = nil, want bounded metric rejection")
	}
}

func TestContextEfficiencyEvidenceStableIdentityIsVersionBound(t *testing.T) {
	evidence := ContextEfficiencyEvidence{
		Identity:    ContextEfficiencyIdentity{FixtureVersion: "fixture-v1", RepresentationVersion: "representation-v1", RendererVersion: "renderer-v1"},
		Metrics:     ContextEfficiencyMetrics{RelevantTokenRatio: .5, EvidenceDensity: .25, SelectedContextTokens: 12},
		GeneratedAt: time.Unix(1, 0).UTC(),
	}
	first, err := evidence.StableIdentity()
	if err != nil {
		t.Fatal(err)
	}
	second, err := evidence.StableIdentity()
	if err != nil {
		t.Fatal(err)
	}
	if first == "" || first != second {
		t.Fatalf("stable identities = %q and %q", first, second)
	}
	evidence.Identity.RendererVersion = "renderer-v2"
	third, err := evidence.StableIdentity()
	if err != nil {
		t.Fatal(err)
	}
	if third == first {
		t.Fatal("stable identity did not change when renderer version changed")
	}
}

func TestContextEfficiencyEvidenceRequiresVersionedIdentity(t *testing.T) {
	evidence := ContextEfficiencyEvidence{Metrics: ContextEfficiencyMetrics{}, GeneratedAt: time.Unix(1, 0).UTC()}
	if err := evidence.Validate(); err == nil {
		t.Fatal("Validate() error = nil, want missing identity rejection")
	}
}
