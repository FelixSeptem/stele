package reasoning

import "testing"

func TestInsightDiagnosticsRequireAuthorizedScopeAndBoundCounters(t *testing.T) {
	diagnostics := InsightDiagnostics{AuthorizedScope: true, Mode: "shadow", InsightType: "hypothesis", Result: "would_activate", Candidates: 2, WouldActivate: 1}
	if err := diagnostics.Validate(); err != nil {
		t.Fatalf("valid diagnostics rejected: %v", err)
	}
	diagnostics.AuthorizedScope = false
	if err := diagnostics.Validate(); err == nil {
		t.Fatal("unauthorized diagnostics accepted")
	}
}
