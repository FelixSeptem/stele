package benchmark

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLongMemEvalCapacityPreflightRejectsLargerSubsetWithoutExplicitBudget(t *testing.T) {
	if got := PreflightLongMemEval(LongMemEvalCapacity{Subset: "m", MaxEvents: 100, RequestedEvents: 101, AllowLarge: false}); got != StatusCapacityRefused {
		t.Fatalf("expected capacity refusal, got %s", got)
	}
	if got := PreflightLongMemEval(LongMemEvalCapacity{Subset: "s", MaxEvents: 100, RequestedEvents: 100}); got != StatusSuccess {
		t.Fatalf("expected success, got %s", got)
	}
}

func TestPlanLongMemEvalUsesBatchesAndRefusesUnapprovedLargeRun(t *testing.T) {
	plan, err := PlanLongMemEval(LongMemEvalCapacity{Subset: "s", RequestedEvents: 9, MaxEvents: 10, BatchSize: 4})
	if err != nil || plan.Status != StatusSuccess || plan.BatchCount != 3 || plan.EffectiveBatchSize != 4 {
		t.Fatalf("unexpected standard subset plan: %#v, %v", plan, err)
	}
	plan, err = PlanLongMemEval(LongMemEvalCapacity{Subset: "m", RequestedEvents: 10, MaxEvents: 10, BatchSize: 2})
	if err != nil || plan.Status != StatusCapacityRefused {
		t.Fatalf("expected explicit approval refusal: %#v, %v", plan, err)
	}
}

func TestCleanRunArtifactsRemovesOnlyRunEmbeddingsAndPreservesReportAndRaw(t *testing.T) {
	cache := NewCache(t.TempDir())
	manifest := validManifest()
	paths, err := cache.EnsureManifestLayout(manifest)
	if err != nil {
		t.Fatal(err)
	}
	runEmbeddings := filepath.Join(paths.Embeddings, "run-1")
	if err := os.MkdirAll(runEmbeddings, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(runEmbeddings, "vectors.bin"), []byte("vectors"), 0o644); err != nil {
		t.Fatal(err)
	}
	report := filepath.Join(paths.Reports, "run-1.json")
	if err := os.WriteFile(report, []byte("report"), 0o644); err != nil {
		t.Fatal(err)
	}
	raw := filepath.Join(paths.Raw, "locked.json")
	if err := os.WriteFile(raw, []byte("raw"), 0o644); err != nil {
		t.Fatal(err)
	}
	result, err := cache.CleanRunArtifacts(manifest, "run-1", true)
	if err != nil {
		t.Fatal(err)
	}
	if !result.EmbeddingsRemoved || !result.ReportRetained {
		t.Fatalf("unexpected cleanup result: %#v", result)
	}
	if _, err := os.Stat(runEmbeddings); !os.IsNotExist(err) {
		t.Fatalf("run embeddings remain after cleanup: %v", err)
	}
	if _, err := os.Stat(report); err != nil {
		t.Fatalf("retained report removed: %v", err)
	}
	if _, err := os.Stat(raw); err != nil {
		t.Fatalf("locked raw data removed: %v", err)
	}
	if _, err := cache.CleanRunArtifacts(manifest, "..", true); err == nil {
		t.Fatal("expected unsafe run id to be rejected")
	}
}
