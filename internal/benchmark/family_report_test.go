package benchmark

import "testing"

func TestBuildFamilyReportIsDeterministicAndScoped(t *testing.T) {
	report, err := BuildFamilyReport(FamilyGenericRetrieval, map[string]any{"semantic": map[string]int{"recall": 1}, "lexical": map[string]int{"recall": 2}})
	if err != nil {
		t.Fatal(err)
	}
	if report.Family != FamilyGenericRetrieval || len(report.Reports) != 2 || report.Status != StatusSuccess {
		t.Fatalf("report=%#v", report)
	}
	if _, err := BuildFamilyReport(BenchmarkFamily("unknown"), map[string]any{"x": 1}); err == nil {
		t.Fatal("expected invalid family")
	}
}
