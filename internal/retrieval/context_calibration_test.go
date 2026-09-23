package retrieval

import (
	"testing"
	"time"

	"github.com/FelixSeptem/stele/internal/memory"
)

func TestApplyContextCalibrationIsDeterministicAndScopeLifecycleSafe(t *testing.T) {
	scope := memory.Scope{Tenant: "tenant", Project: "project", Namespace: "namespace"}
	now := time.Unix(10, 0).UTC()
	candidates := []SearchHit{
		{Memory: memory.CanonicalMemory{ID: "b", Scope: scope, State: memory.MemoryStateActive, ModifiedAt: now}, Score: ScoreBreakdown{Overall: .5}},
		{Memory: memory.CanonicalMemory{ID: "a", Scope: scope, State: memory.MemoryStateActive, ModifiedAt: now}, Score: ScoreBreakdown{Overall: .5}},
		{Memory: memory.CanonicalMemory{ID: "hidden", Scope: scope, State: memory.MemoryStateSuppressed, ModifiedAt: now}, Score: ScoreBreakdown{Overall: .9}},
		{Memory: memory.CanonicalMemory{ID: "foreign", Scope: memory.Scope{Tenant: "other", Project: "project", Namespace: "namespace"}, State: memory.MemoryStateActive, ModifiedAt: now}, Score: ScoreBreakdown{Overall: .9}},
	}
	summary := memory.ContextCalibrationSummary{Scope: scope, SummaryVersion: "summary-v1", PolicyVersion: "context-calibration-v1", SourceWatermark: now, EvidenceCount: 2, PrioritySum: .4, Freshness: memory.ContextCalibrationSummaryFresh, CreatedAt: now, UpdatedAt: now, ID: "summary"}
	first, diagnostics := ApplyContextCalibration(scope, candidates, summary, 10)
	second, _ := ApplyContextCalibration(scope, candidates, summary, 10)
	if len(first) != 2 || len(second) != 2 || first[0].Memory.ID != second[0].Memory.ID || first[1].Memory.ID != second[1].Memory.ID {
		t.Fatalf("calibrated candidates = %+v / %+v diagnostics=%+v", first, second, diagnostics)
	}
	for _, candidate := range first {
		if candidate.Memory.State != memory.MemoryStateActive || candidate.Memory.Scope.Normalized() != scope.Normalized() {
			t.Fatalf("unsafe candidate returned: %+v", candidate)
		}
	}
}

func TestApplyContextCalibrationFallsBackForStaleSummary(t *testing.T) {
	scope := memory.Scope{Tenant: "tenant", Project: "project", Namespace: "namespace"}
	now := time.Unix(10, 0).UTC()
	candidates := []SearchHit{{Memory: memory.CanonicalMemory{ID: "a", Scope: scope, State: memory.MemoryStateActive}, Score: ScoreBreakdown{Overall: .5}}}
	summary := memory.ContextCalibrationSummary{Scope: scope, SummaryVersion: "summary-v1", PolicyVersion: "context-calibration-v1", SourceWatermark: now, EvidenceCount: 1, Freshness: memory.ContextCalibrationSummaryStale, CreatedAt: now, UpdatedAt: now, ID: "summary"}
	got, diagnostics := ApplyContextCalibration(scope, candidates, summary, 10)
	if len(got) != 1 || got[0].Score.Overall != candidates[0].Score.Overall || len(diagnostics) == 0 {
		t.Fatalf("got=%+v diagnostics=%+v, want baseline fallback", got, diagnostics)
	}
}
