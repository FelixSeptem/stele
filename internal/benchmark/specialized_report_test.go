package benchmark

import (
	"github.com/FelixSeptem/stele/internal/memory"
	"testing"
)

func TestEvaluateSpecializedCasesProducesTargetedSubfamilyReports(t *testing.T) {
	scope := memory.Scope{Tenant: "benchmark", Project: "benchmark-specialized", Namespace: "fixture"}
	cases := BuiltinSpecializedCases(scope)
	ranks := map[string][]RetrievedEvidence{}
	for _, item := range cases {
		for index, event := range item.Events {
			if event.ExpectedState == memory.MemoryStateActive {
				ranks[item.Query.ID] = append(ranks[item.Query.ID], RetrievedEvidence{EvidenceID: event.ID, Rank: index + 1})
			}
		}
	}
	reports, err := EvaluateSpecializedCases(cases, ranks)
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"profile", "temporal", "multi-hop"} {
		report, ok := reports[name]
		if !ok || report.Subfamily != name || report.Family != FamilySpecializedRetrieval || report.QueryCount != 1 {
			t.Fatalf("missing targeted report %q: %#v", name, reports)
		}
	}
	if reports["profile"].PreferenceConsistency != 1 || reports["temporal"].StaleFactSuppression != 1 || reports["multi-hop"].EvidenceGroupCompleteness != 1 {
		t.Fatalf("unexpected specialized metrics: %#v", reports)
	}
}
