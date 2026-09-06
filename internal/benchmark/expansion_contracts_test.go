package benchmark

import (
	"testing"

	"github.com/FelixSeptem/stele/internal/memory"
)

func expansionScope() memory.Scope {
	return memory.Scope{Tenant: "bench", Project: "expansion", Namespace: "run"}
}

func TestReplayContractSeparatesSubsetsAndRejectsHiddenScope(t *testing.T) {
	scope := expansionScope()
	report := ReplayContract([]ContractCase{
		{ID: "kv-get", Subset: "memory_kv", Operation: ContractOperation{Name: "get", Scope: scope}},
		{ID: "vector-hidden", Subset: "memory_vector", Operation: ContractOperation{Name: "search", Scope: scope, Lifecycle: memory.MemoryStateForgotten, MustRefuse: true}},
		{ID: "foreign", Subset: "memory_rec_sum", Operation: ContractOperation{Name: "search", Scope: memory.Scope{Tenant: "foreign", Project: "expansion", Namespace: "run"}}},
	})
	if report.Family != FamilyProviderContract || report.SubsetCounts["memory_kv"] != 1 {
		t.Fatalf("unexpected contract report: %#v", report)
	}
	if report.ScopeSafetyFailures != 1 || report.LifecycleSafetyFailures != 0 {
		t.Fatalf("safety metrics = %#v", report)
	}
}

func TestSpecializedFixturesHaveTargetedEvidence(t *testing.T) {
	items := BuiltinSpecializedCases(expansionScope())
	if len(items) != 3 {
		t.Fatalf("cases=%d", len(items))
	}
	for _, item := range items {
		if err := item.Query.Scope.Validate(); err != nil {
			t.Fatal(err)
		}
		if len(item.QRELs) == 0 || len(item.Query.EvidenceGroups) == 0 {
			t.Fatalf("case lacks evidence: %#v", item)
		}
	}
}

func TestStressBudgetAndVisualCapabilityFailClosed(t *testing.T) {
	report := EvaluateStress("vtcbench", []StressCase{{ID: "visual", ContextTokens: 100, Mode: "visual"}}, StressBudget{MaxContextTokens: 50}, false)
	if report.Status != StatusCapacityRefused || !report.NonGating || len(report.Failures) != 2 {
		t.Fatalf("stress report=%#v", report)
	}
}

func TestLongMemEvalJSONLSubset(t *testing.T) {
	source := []byte(`{"question_id":"q-s","question":"q","subset":"s","sessions":[{"session_id":"s1","turns":[{"id":"t1","text":"answer"}]}]}` + "\n" + `{"question_id":"q-m","question":"q","subset":"m","sessions":[{"session_id":"s2","turns":[{"id":"t2","text":"answer"}]}]}`)
	dataset, err := LoadLongMemEvalSubset(source, "s")
	if err != nil {
		t.Fatal(err)
	}
	if len(dataset.Samples) != 1 || dataset.Samples[0].QuestionID != "q-s" {
		t.Fatalf("subset=%#v", dataset)
	}
}

func TestControlledStressSubsetGenerationAndLongBenchCapacityGate(t *testing.T) {
	needle := GenerateNeedleStressCases("needle", []int{1024, 4096}, 2)
	if len(needle) != 4 || needle[0].NeedleCount != 2 {
		t.Fatalf("unexpected needle cases: %#v", needle)
	}
	longbench := LongBenchSubset{Dataset: "longbench-v2", Subset: "short-local", ContextTokens: 8192, SampleCount: 4, License: "upstream-review-required"}
	if report := EvaluateLongBenchCapacity(longbench, StressBudget{MaxContextTokens: 4096, MaxSamples: 10}); report.Status != StatusCapacityRefused || !report.NonGating {
		t.Fatalf("expected explicit longbench capacity refusal: %#v", report)
	}
}
