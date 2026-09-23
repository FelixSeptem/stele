package retrieval

import (
	"context"
	"testing"
	"time"

	"github.com/FelixSeptem/stele/internal/memory"
)

type contextCalibrationSummaryReaderStub struct {
	summary memory.ContextCalibrationSummary
	err     error
}

func (s contextCalibrationSummaryReaderStub) ReadContextCalibrationSummary(context.Context, memory.ReadContextCalibrationSummaryInput) (memory.ContextCalibrationSummary, error) {
	return s.summary, s.err
}

func TestServiceContextCalibrationActiveExactScopeAndRollback(t *testing.T) {
	now := time.Date(2026, 9, 22, 12, 0, 0, 0, time.UTC)
	scope := memory.Scope{Tenant: "tenant", Project: "project", Namespace: "namespace"}
	policy := &memory.RankingRolloutPolicy{Scope: scope, Status: memory.RankingRolloutPolicyStatusActiveForScope, Mode: memory.RankingRolloutModeActiveForScope, Surfaces: []memory.RankingRolloutSurface{memory.RankingRolloutSurfaceContext}, SignalSources: []memory.RankingRolloutSignalSource{memory.RankingRolloutSignalSourceUsefulnessFeedback}, ThresholdStatus: memory.RankingRolloutThresholdStatusSatisfied, Actor: "operator", Reason: "test", CreatedAt: now, UpdatedAt: now, ContextCalibrationSelector: memory.RetrievalPlannerRolloutSelector{SessionID: "s", UserID: "u"}, ContextCalibration: &memory.ContextCalibrationRolloutPolicy{SchemaVersion: memory.ContextCalibrationRolloutSchemaVersionV1, PolicyVersion: memory.ContextCalibrationPolicyVersionV1, SummaryVersion: "summary-v1", MinimumEvidence: 1, ConfidenceThreshold: .5, DecayWindow: time.Hour, ContributionCap: .25, MaxCandidates: 10, MaxContextItems: 10, MaxElapsed: time.Millisecond, ExpiresAt: now.Add(time.Hour)}}
	reader := contextCalibrationSummaryReaderStub{summary: memory.ContextCalibrationSummary{ID: "summary", Scope: scope, PolicyVersion: memory.ContextCalibrationPolicyVersionV1, SummaryVersion: "summary-v1", SourceWatermark: now, EvidenceCount: 1, PrioritySum: .2, Freshness: memory.ContextCalibrationSummaryFresh, CreatedAt: now, UpdatedAt: now}}
	limits := ContextCalibrationLimits{MaxSummaryAge: 24 * time.Hour, MinimumEvidence: 1, ConfidenceThreshold: .5, DecayWindow: time.Hour, ContributionCap: .25, MaxCandidates: 10, MaxContextItems: 10, MaxElapsed: time.Millisecond}
	service := &Service{contextCalibrationEnabled: true, contextCalibrationLimits: limits, contextCalibrationSummaryReader: reader, now: func() time.Time { return now }}
	hits := []SearchHit{{Memory: memory.CanonicalMemory{ID: "a", Scope: scope, State: memory.MemoryStateActive}, Score: ScoreBreakdown{Overall: .5}}, {Memory: memory.CanonicalMemory{ID: "b", Scope: scope, State: memory.MemoryStateActive}, Score: ScoreBreakdown{Overall: .5}}}
	got, diagnostics := service.applyContextCalibrationRollout(context.Background(), AssembleContextInput{Scope: scope, SessionID: "s", UserID: "u"}, hits, policy)
	if len(got) != 2 || len(diagnostics) != 1 || diagnostics[0].Status != "applied" {
		t.Fatalf("got=%+v diagnostics=%+v", got, diagnostics)
	}
	policy.Status, policy.Mode = memory.RankingRolloutPolicyStatusRolledBack, memory.RankingRolloutModeActiveForScope
	rolledBack, rollbackDiagnostics := service.applyContextCalibrationRollout(context.Background(), AssembleContextInput{Scope: scope, SessionID: "s", UserID: "u"}, hits, policy)
	if len(rolledBack) != len(hits) || rolledBack[0].Score != hits[0].Score || len(rollbackDiagnostics) != 1 || rollbackDiagnostics[0].Status != "baseline_fallback" {
		t.Fatalf("rollback=%+v diagnostics=%+v", rolledBack, rollbackDiagnostics)
	}
}

func TestServiceContextCalibrationDeploymentDisablementAlwaysRetainsBaseline(t *testing.T) {
	now := time.Date(2026, 9, 23, 12, 0, 0, 0, time.UTC)
	scope := memory.Scope{Tenant: "tenant", Project: "project", Namespace: "namespace"}
	policy := &memory.RankingRolloutPolicy{Scope: scope, Status: memory.RankingRolloutPolicyStatusActiveForScope, Mode: memory.RankingRolloutModeActiveForScope, Surfaces: []memory.RankingRolloutSurface{memory.RankingRolloutSurfaceContext}, SignalSources: []memory.RankingRolloutSignalSource{memory.RankingRolloutSignalSourceUsefulnessFeedback}, ThresholdStatus: memory.RankingRolloutThresholdStatusSatisfied, Actor: "operator", Reason: "test", CreatedAt: now, UpdatedAt: now, ContextCalibration: &memory.ContextCalibrationRolloutPolicy{SchemaVersion: memory.ContextCalibrationRolloutSchemaVersionV1, PolicyVersion: memory.ContextCalibrationPolicyVersionV1, SummaryVersion: "summary-v1", MinimumEvidence: 1, ConfidenceThreshold: .5, DecayWindow: time.Hour, ContributionCap: .25, MaxCandidates: 10, MaxContextItems: 10, MaxElapsed: time.Millisecond, ExpiresAt: now.Add(time.Hour)}}
	hits := []SearchHit{{Memory: memory.CanonicalMemory{ID: "a", Scope: scope, State: memory.MemoryStateActive}, Score: ScoreBreakdown{Overall: .5}}}
	got, diagnostics := (&Service{contextCalibrationSummaryReader: contextCalibrationSummaryReaderStub{}, now: func() time.Time { return now }}).applyContextCalibrationRollout(context.Background(), AssembleContextInput{Scope: scope}, hits, policy)
	if len(got) != 1 || got[0].Memory.ID != "a" || len(diagnostics) != 1 || diagnostics[0].Status != "baseline_fallback" || diagnostics[0].Reason != "calibration is disabled by deployment" {
		t.Fatalf("got=%+v diagnostics=%+v, want deployment-disabled baseline", got, diagnostics)
	}
}

func TestServiceContextCalibrationPolicyCannotBroadenDeploymentLimits(t *testing.T) {
	now := time.Date(2026, 9, 23, 12, 0, 0, 0, time.UTC)
	scope := memory.Scope{Tenant: "tenant", Project: "project", Namespace: "namespace"}
	policy := &memory.RankingRolloutPolicy{Scope: scope, Status: memory.RankingRolloutPolicyStatusActiveForScope, Mode: memory.RankingRolloutModeActiveForScope, Surfaces: []memory.RankingRolloutSurface{memory.RankingRolloutSurfaceContext}, SignalSources: []memory.RankingRolloutSignalSource{memory.RankingRolloutSignalSourceUsefulnessFeedback}, ThresholdStatus: memory.RankingRolloutThresholdStatusSatisfied, Actor: "operator", Reason: "test", CreatedAt: now, UpdatedAt: now, ContextCalibration: &memory.ContextCalibrationRolloutPolicy{SchemaVersion: memory.ContextCalibrationRolloutSchemaVersionV1, PolicyVersion: memory.ContextCalibrationPolicyVersionV1, SummaryVersion: "summary-v1", MinimumEvidence: 1, ConfidenceThreshold: .5, DecayWindow: 2 * time.Hour, ContributionCap: .25, MaxCandidates: 10, MaxContextItems: 10, MaxElapsed: time.Millisecond, ExpiresAt: now.Add(time.Hour)}}
	hits := []SearchHit{{Memory: memory.CanonicalMemory{ID: "a", Scope: scope, State: memory.MemoryStateActive}, Score: ScoreBreakdown{Overall: .5}}}
	service := &Service{contextCalibrationEnabled: true, contextCalibrationLimits: ContextCalibrationLimits{MaxSummaryAge: time.Hour, MinimumEvidence: 1, ConfidenceThreshold: .5, DecayWindow: time.Hour, ContributionCap: .25, MaxCandidates: 10, MaxContextItems: 10, MaxElapsed: time.Millisecond}, contextCalibrationSummaryReader: contextCalibrationSummaryReaderStub{}, now: func() time.Time { return now }}
	got, diagnostics := service.applyContextCalibrationRollout(context.Background(), AssembleContextInput{Scope: scope}, hits, policy)
	if len(got) != 1 || len(diagnostics) != 1 || diagnostics[0].Reason != "calibration policy exceeds deployment limits" {
		t.Fatalf("got=%+v diagnostics=%+v, want deployment-limit baseline", got, diagnostics)
	}
}
