package memory

import (
	"testing"
	"time"
)

func TestBuildContextCalibrationSummaryFiltersForeignExpiredAndSupersededSignals(t *testing.T) {
	now := time.Date(2026, 9, 22, 12, 0, 0, 0, time.UTC)
	scope := Scope{Tenant: "tenant-a", Project: "project-a", Namespace: "namespace-a"}
	summary, err := BuildContextCalibrationSummary(ContextCalibrationBuildInput{
		Scope: scope, PolicyVersion: "context-calibration-v1", SummaryVersion: "summary-v1", Now: now,
		MinimumEvidence: 1, ConfidenceThreshold: .5, DecayWindow: 24 * time.Hour, ContributionCap: .25,
		Signals: []ContextCalibrationSignal{
			{Scope: scope, SubjectKey: "keep", Weight: 1, Confidence: 1, CreatedAt: now.Add(-time.Hour)},
			{Scope: Scope{Tenant: "tenant-b", Project: "project-a", Namespace: "namespace-a"}, SubjectKey: "foreign", Weight: 1, Confidence: 1, CreatedAt: now},
			{Scope: scope, SubjectKey: "expired", Weight: 1, Confidence: 1, CreatedAt: now.Add(-48 * time.Hour)},
			{Scope: scope, SubjectKey: "superseded", Weight: 1, Confidence: 1, Superseded: true, CreatedAt: now},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if summary.EvidenceCount != 1 || summary.Freshness != ContextCalibrationSummaryFresh || summary.SourceWatermark != now.Add(-time.Hour) {
		t.Fatalf("summary = %+v, want one exact-scope active signal", summary)
	}
}

func TestBuildContextCalibrationSummaryAppliesContributionCapAndMinimumEvidence(t *testing.T) {
	now := time.Date(2026, 9, 22, 12, 0, 0, 0, time.UTC)
	scope := Scope{Tenant: "tenant-a", Project: "project-a", Namespace: "namespace-a"}
	input := ContextCalibrationBuildInput{Scope: scope, PolicyVersion: "context-calibration-v1", SummaryVersion: "summary-v1", Now: now, MinimumEvidence: 2, ConfidenceThreshold: .5, DecayWindow: 24 * time.Hour, ContributionCap: .25, Signals: []ContextCalibrationSignal{{Scope: scope, SubjectKey: "a", Weight: 10, Confidence: 1, CreatedAt: now}, {Scope: scope, SubjectKey: "b", Weight: .1, Confidence: .5, CreatedAt: now}}}
	summary, err := BuildContextCalibrationSummary(input)
	if err != nil {
		t.Fatal(err)
	}
	if summary.Freshness != ContextCalibrationSummaryFresh || summary.PrioritySum > .35 || summary.PrioritySum <= 0 {
		t.Fatalf("summary = %+v, want capped positive priority", summary)
	}
	input.MinimumEvidence = 3
	summary, err = BuildContextCalibrationSummary(input)
	if err != nil {
		t.Fatal(err)
	}
	if summary.Freshness != ContextCalibrationSummaryUnknown {
		t.Fatalf("summary freshness = %q, want unknown below threshold", summary.Freshness)
	}
}
