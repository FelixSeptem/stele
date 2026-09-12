package retrieval

import "testing"

func TestComputeQualityAdjustmentClampsAndUsesNeutralMissingValues(t *testing.T) {
	features := QualityFeatureVector{
		Version:           "quality-v1",
		EvidenceCoverage:  1,
		Freshness:         1,
		SourceReliability: 1,
		Conflict:          -1,
		Usefulness:        1,
		TaskSuccess:       1,
		Verification:      1,
	}
	got, err := ComputeQualityAdjustment(features, QualityAdjustmentBounds{PerFeature: 0.1, Total: 0.2})
	if err != nil {
		t.Fatal(err)
	}
	if got < 0.19 || got > 0.200001 {
		t.Fatalf("adjustment=%v, want total clamp near 0.2", got)
	}

	neutral, err := ComputeQualityAdjustment(QualityFeatureVector{Version: "quality-v1"}, QualityAdjustmentBounds{PerFeature: 0.1, Total: 0.2})
	if err != nil {
		t.Fatal(err)
	}
	if neutral != 0 {
		t.Fatalf("neutral missing features adjustment=%v, want 0", neutral)
	}
}

func TestComputeQualityAdjustmentRejectsInvalidFeatures(t *testing.T) {
	if _, err := ComputeQualityAdjustment(QualityFeatureVector{Version: "quality-v1", Freshness: 2}, QualityAdjustmentBounds{PerFeature: 0.1, Total: 0.2}); err == nil {
		t.Fatal("expected out-of-range feature error")
	}
}
