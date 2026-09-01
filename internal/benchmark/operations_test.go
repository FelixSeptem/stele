package benchmark

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/FelixSeptem/stele/internal/memory"
)

func TestPlanBenchmarkBatchesEnforcesBatchAndTotalLimits(t *testing.T) {
	events := []MemoryEventRecord{{ID: "e1", Text: "one", Scope: operationScope()}, {ID: "e2", Text: "two", Scope: operationScope()}, {ID: "e3", Text: "three", Scope: operationScope()}}
	batches, err := PlanBenchmarkBatches(events, 2, 3)
	if err != nil || len(batches) != 2 || len(batches[0]) != 2 || len(batches[1]) != 1 {
		t.Fatalf("batches=%#v err=%v, want 2+1 batches", batches, err)
	}
	if _, err := PlanBenchmarkBatches(events, 0, 3); StatusOf(err) != StatusCapacityRefused {
		t.Fatalf("invalid batch size status=%v, want capacity_refused", StatusOf(err))
	}
	if _, err := PlanBenchmarkBatches(events, 2, 2); StatusOf(err) != StatusCapacityRefused {
		t.Fatalf("total limit status=%v, want capacity_refused", StatusOf(err))
	}
}

func TestCheckBenchmarkCapacityReportsStableDiagnostics(t *testing.T) {
	corpus := NormalizedCorpus{Events: []MemoryEventRecord{{ID: "e1", Scope: operationScope(), Text: "large text"}}, Queries: []BenchmarkQuery{{ID: "q1", Scope: operationScope(), Text: "question"}}}
	dir := t.TempDir()
	report, err := CheckBenchmarkCapacity(dir, corpus, CapacityBudget{MaxEvents: 10, MaxQueries: 10, MaxBytes: 1})
	if StatusOf(err) != StatusCapacityRefused || report.Status != StatusCapacityRefused || len(report.Diagnostics) == 0 || !strings.Contains(report.Diagnostics[0], "bytes") {
		t.Fatalf("capacity report=%#v err=%v, want stable byte diagnostic", report, err)
	}
	if _, err := CheckBenchmarkCapacity(filepath.Join(dir, "missing"), corpus, CapacityBudget{}); StatusOf(err) != StatusPrerequisiteMissing {
		t.Fatalf("missing data dir status=%v, want prerequisite_missing", StatusOf(err))
	}
}

func TestCleanupBenchmarkRunArtifactsPreservesReportsWhenRequested(t *testing.T) {
	cache := NewCache(t.TempDir())
	manifest := validManifest()
	runID := "run-1"
	runPaths, err := cache.EnsureRunLayout(manifest, runID)
	if err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{runPaths.Normalized, runPaths.Embeddings, runPaths.Reports} {
		if err := os.WriteFile(filepath.Join(path, "artifact.json"), []byte("{}"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if err := cache.CleanupBenchmarkRun(manifest, runID, true); err != nil {
		t.Fatalf("cleanup: %v", err)
	}
	if _, err := os.Stat(runPaths.Reports); err != nil {
		t.Fatalf("reports should be retained: %v", err)
	}
	if _, err := os.Stat(runPaths.Normalized); !os.IsNotExist(err) {
		t.Fatalf("normalized artifacts should be removed, err=%v", err)
	}
	if err := cache.CleanupBenchmarkRun(manifest, "..\\escape", false); StatusOf(err) != StatusInvalidManifest {
		t.Fatalf("path traversal status=%v, want invalid_manifest", StatusOf(err))
	}
}

func operationScope() memory.Scope {
	return memory.Scope{Tenant: "bench", Project: "ops", Namespace: "fixture"}
}
