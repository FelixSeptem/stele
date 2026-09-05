package benchmark

import (
	"os"
	"path/filepath"
	"testing"
)

func TestGenerateNeedleSubsetPreservesControlledDepthAndBudgets(t *testing.T) {
	subset, err := GenerateNeedleSubset(NeedleSubsetRequest{
		Config: StressConfig{
			ContextLength:    128,
			MaxContextLength: 128,
			SampleCount:      1,
			MaxSamples:       1,
			NeedleCount:      2,
			MaxNeedleCount:   2,
			TimeoutMS:        1_000,
			MaxTimeoutMS:     1_000,
			Mode:             "text",
		},
		ID:       "needle-depths",
		Haystack: "offline filler text",
		Query:    "Which facts were inserted?",
		Needles: []NeedlePlacement{
			{ID: "needle-early", Text: "alpha", Depth: 10},
			{ID: "needle-late", Text: "beta", Depth: 90},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if subset.Dataset != StressDatasetNeedle || len(subset.Samples) != 1 {
		t.Fatalf("unexpected needle subset identity: %#v", subset)
	}
	sample := subset.Samples[0]
	if sample.ContextLength != 128 || sample.NeedleCount != 2 || sample.NeedleDepths["needle-early"] != 10 || sample.NeedleDepths["needle-late"] != 90 || sample.TimeoutMS != 1_000 {
		t.Fatalf("needle controls were not retained: %#v", sample)
	}
}

func TestLoadMRCRSubsetRejectsBudgetOverflowBeforeUse(t *testing.T) {
	subset := StressSubset{
		Dataset: StressDatasetMRCR,
		Config: StressConfig{
			ContextLength:    64,
			MaxContextLength: 64,
			SampleCount:      1,
			MaxSamples:       1,
			TimeoutMS:        500,
			MaxTimeoutMS:     500,
			Mode:             "text",
		},
		Samples: []StressSample{{ID: "mrcr-overflow", Context: "local MRCR fixture", Query: "retrieve the record", ContextLength: 65, NeedleCount: 1, TimeoutMS: 500}},
	}
	if _, err := LoadMRCRSubset(subset); StatusOf(err) != StatusCapacityRefused {
		t.Fatalf("expected pre-import MRCR capacity refusal, got %v", err)
	}
}

func TestPlanLongBenchV2RetainsMetadataAndExcludesAnswerAccuracyFromMemoryGate(t *testing.T) {
	plan, err := PlanLongBenchV2(LongBenchV2Subset{
		Metadata: LongBenchV2Metadata{Subset: "passage-retrieval-mini", Language: "zh", TaskType: "long-context-qa", AnswerMetric: "exact_match"},
		Config: StressConfig{
			ContextLength:    32_768,
			MaxContextLength: 32_768,
			SampleCount:      4,
			MaxSamples:       4,
			TimeoutMS:        30_000,
			MaxTimeoutMS:     30_000,
			Mode:             "text",
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if plan.Status != StatusSuccess || !plan.NonGating || plan.AnswerAccuracyIsMemoryQuality || plan.Metadata.Language != "zh" || plan.Metadata.AnswerMetric != "exact_match" {
		t.Fatalf("LongBench-v2 plan must retain metadata and remain non-gating: %#v", plan)
	}
	plan, err = PlanLongBenchV2(LongBenchV2Subset{
		Metadata: LongBenchV2Metadata{Subset: "passage-retrieval-mini", Language: "zh", TaskType: "long-context-qa", AnswerMetric: "exact_match"},
		Config:   StressConfig{ContextLength: 32_769, MaxContextLength: 32_768, SampleCount: 1, MaxSamples: 1, TimeoutMS: 30_000, MaxTimeoutMS: 30_000, Mode: "text"},
	})
	if StatusOf(err) != StatusCapacityRefused || plan.Status != StatusCapacityRefused {
		t.Fatalf("LongBench-v2 capacity overflow must be refused before execution: %#v, %v", plan, err)
	}
}

func TestLoadVTCBenchSupportsTextAndRefusesVisualWithoutLocalArtifacts(t *testing.T) {
	textArtifact := filepath.Join(t.TempDir(), "vtcbench-text.jsonl")
	if err := os.WriteFile(textArtifact, []byte("local text fixture"), 0o600); err != nil {
		t.Fatal(err)
	}
	text, err := LoadVTCBenchSubset(VTCBenchSubset{
		Config:       StressConfig{ContextLength: 1_024, MaxContextLength: 1_024, SampleCount: 1, MaxSamples: 1, TimeoutMS: 1_000, MaxTimeoutMS: 1_000, Mode: "text"},
		TextArtifact: textArtifact,
	})
	if err != nil || text.Mode != "text" || text.Artifact != textArtifact {
		t.Fatalf("expected local VTCBench text subset, got %#v, %v", text, err)
	}
	if _, err := LoadVTCBenchSubset(VTCBenchSubset{
		Config: StressConfig{ContextLength: 1_024, MaxContextLength: 1_024, SampleCount: 1, MaxSamples: 1, TimeoutMS: 1_000, MaxTimeoutMS: 1_000, Mode: "visual"},
	}); StatusOf(err) != StatusPrerequisiteMissing {
		t.Fatalf("expected visual prerequisite refusal without local capability/artifact, got %v", err)
	}
	if _, err := LoadVTCBenchSubset(VTCBenchSubset{
		Config: StressConfig{ContextLength: 1_024, MaxContextLength: 1_024, SampleCount: 1, MaxSamples: 1, TimeoutMS: 1_000, MaxTimeoutMS: 1_000, Mode: "visual", VisualAvailable: true},
	}); StatusOf(err) != StatusPrerequisiteMissing {
		t.Fatalf("expected visual artifact refusal without silent text fallback, got %v", err)
	}
}

func TestBuildStressReportKeepsPerBucketDegradationAndNonGatingClassification(t *testing.T) {
	report := BuildStressReport([]StressBucketOutcome{
		{ContextLength: 8_192, NeedleCount: 1, NeedleDepth: 10, Mode: "text", LatencyMS: 15.5, Status: StatusSuccess},
		{ContextLength: 32_768, NeedleCount: 2, NeedleDepth: 90, Mode: "text", LatencyMS: 0, Status: StatusCapacityRefused},
	})
	if report.Family != FamilyStress || !report.NonGating || report.CapacityFailures != 1 || len(report.Buckets) != 2 {
		t.Fatalf("unexpected stress classification/report: %#v", report)
	}
	if report.Buckets[0].ContextLength != 8_192 || report.Buckets[0].NeedleDepth != 10 || report.Buckets[0].LatencyMS != 15.5 || report.Buckets[1].Status != StatusCapacityRefused {
		t.Fatalf("bucket degradation data was lost: %#v", report.Buckets)
	}
}
