package reasoning

import "testing"

func TestEvaluateContradictionFreshnessFailsClosedAfterSourceCorrection(t *testing.T) {
	candidate := ContradictionCandidate{SourceWatermark: "w1", Left: ContradictionFact{Version: 1}, Right: ContradictionFact{Version: 2}}
	if got := EvaluateContradictionFreshness(candidate, "w2", 1, 2); got != ContradictionFreshnessStale {
		t.Fatalf("watermark disposition = %q, want stale_evidence", got)
	}
	if got := EvaluateContradictionFreshness(candidate, "w1", 1, 3); got != ContradictionFreshnessStale {
		t.Fatalf("version disposition = %q, want stale_evidence", got)
	}
	if got := EvaluateContradictionFreshness(candidate, "w1", 1, 2); got != ContradictionFreshnessFresh {
		t.Fatalf("fresh disposition = %q, want fresh", got)
	}
}
