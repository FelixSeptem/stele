package benchmark

import (
	"encoding/json"
	"testing"

	"github.com/FelixSeptem/stele/internal/memory"
)

func TestMarshalBenchmarkReportIncludesProvenanceAndStableJSON(t *testing.T) {
	report := BenchmarkReport{
		Dataset: "locomo", Version: "v1", Family: "memory", Split: "smoke",
		ManifestChecksum: "manifest-1", NormalizedChecksum: "normalized-1", QRELChecksum: "qrels-1",
		Embedding: EmbeddingProfile{Name: "lexical-only", Normalization: "none"},
		Strategy:  StrategyProfileLexical, RunID: "run-1", Scope: memory.Scope{Tenant: "benchmark", Project: "benchmark-locomo", Namespace: "run-1"},
		Status: StatusSuccess, Metrics: map[string]float64{"mrr": 1, "recall_at_5": 1},
	}
	first, err := MarshalBenchmarkReport(report)
	if err != nil {
		t.Fatal(err)
	}
	second, err := MarshalBenchmarkReport(report)
	if err != nil {
		t.Fatal(err)
	}
	if string(first) != string(second) {
		t.Fatalf("expected deterministic report JSON, first=%s second=%s", first, second)
	}
	var decoded map[string]any
	if err := json.Unmarshal(first, &decoded); err != nil {
		t.Fatal(err)
	}
	for _, key := range []string{"dataset", "manifest_checksum", "normalized_checksum", "qrels_checksum", "embedding", "strategy", "run_id", "scope", "status", "metrics"} {
		if _, ok := decoded[key]; !ok {
			t.Fatalf("report missing provenance field %q: %s", key, first)
		}
	}
}
