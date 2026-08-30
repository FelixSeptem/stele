package benchmark

import (
	"testing"
)

func TestRunLoCoMoSmokeUsesLocalFixtureAndProducesDeterministicReport(t *testing.T) {
	cache := NewCache(t.TempDir())
	result, err := RunLoCoMoSmoke(cache, memoryScope())
	if err != nil {
		t.Fatal(err)
	}
	if result.Status != StatusSuccess || result.Report.Metrics.RecallAt5 != 1 || result.Report.Metrics.SafetyFailures != 0 {
		t.Fatalf("unexpected smoke result: %#v", result)
	}
	if result.Dataset != "locomo" || result.Mode != RunModeSmoke || result.ReportPath == "" {
		t.Fatalf("missing smoke provenance: %#v", result)
	}
}
