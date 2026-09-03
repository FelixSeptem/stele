package benchmark

import (
	"strings"
	"testing"
)

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

func TestFamilyReportsRejectCrossFamilyComparisonAndRenderClassification(t *testing.T) {
	memoryReport, err := BuildFamilyReport(FamilyMemory, map[string]any{"retrieval": map[string]int{"recall_at_10": 1}})
	if err != nil {
		t.Fatal(err)
	}
	contractReport, err := BuildFamilyReport(FamilyProviderContract, map[string]any{"contract": map[string]int{"operation_accuracy": 1}})
	if err != nil {
		t.Fatal(err)
	}
	if err := CompareFamilyReports(memoryReport, contractReport); err == nil {
		t.Fatal("expected cross-family comparison rejection")
	}
	rendered, err := RenderFamilyReport(memoryReport)
	if err != nil {
		t.Fatal(err)
	}
	if !containsString(string(rendered), "non_comparable") || !containsString(string(rendered), "memory") {
		t.Fatalf("rendered family report lacks classification: %s", rendered)
	}
}

func containsString(value, needle string) bool {
	return strings.Contains(value, needle)
}
