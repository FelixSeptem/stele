package jobs

import (
	"context"
	"testing"
	"time"

	"github.com/FelixSeptem/stele/internal/memory"
	"github.com/FelixSeptem/stele/internal/retrieval"
)

func TestReleaseEvidenceReconciliationJobIsIdempotentAndAppliesVerdict(t *testing.T) {
	now := time.Date(2026, 10, 3, 12, 0, 0, 0, time.UTC)
	report := retrieval.ReleaseEvidenceReport{RunIdentity: "run:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", ScopeHash: "scope:" + repeated("s", 64), PolicyVersion: "policy-v1", FixtureVersion: "fixture-v1", ProviderProfile: "representation-v1", SourceWatermarkHash: "watermark:" + repeated("w", 64), EvidenceFreshness: retrieval.ReleaseEvidenceFresh, EvidenceExpiresAt: now.Add(time.Hour), Verdict: retrieval.ReleaseEvidencePassed, ReleaseEligible: true, OperationalOutcome: retrieval.ReleaseEvidenceOperationalOutcome{State: retrieval.ReleaseEvidenceRunCompleted, Cleanup: retrieval.ReleaseEvidenceCleanupComplete, Consumable: true}}
	report.Attestation = &retrieval.ReleaseEvidenceAttestation{RunIdentity: report.RunIdentity, ScopeHash: report.ScopeHash, PolicyVersion: report.PolicyVersion, FixtureVersion: report.FixtureVersion, SourceWatermarkHash: report.SourceWatermarkHash, RollbackVerdict: retrieval.ReleaseEvidenceRollbackPassed, Freshness: retrieval.ReleaseEvidenceFresh, ExpiresAt: now.Add(time.Hour), IssuedAt: now.Add(-time.Minute)}
	store := &fakeReconciliationStore{inputs: []retrieval.ReleaseEvidenceReconciliationInput{{ExpectedScopeHash: report.ScopeHash, ExpectedPolicyVersion: report.PolicyVersion, ExpectedFixtureVersion: report.FixtureVersion, ExpectedStrategyIdentity: report.ProviderProfile, ExpectedSourceWatermarkHash: report.SourceWatermarkHash, Report: report, Now: now}}}
	executions := &fakeExecutionStore{}
	job := ReleaseEvidenceReconciliationJob{Scope: memory.Scope{Tenant: "tenant-a", Project: "project-a", Namespace: "namespace-a"}, Store: store, ExecutionStore: executions, Cadence: time.Hour, Now: func() time.Time { return now }}
	processed, err := job.Run(context.Background())
	if err != nil || processed != 1 || len(store.results) != 1 || !store.results[0].Eligible {
		t.Fatalf("processed=%d err=%v results=%+v", processed, err, store.results)
	}
	processed, err = job.Run(context.Background())
	if err != nil || processed != 0 {
		t.Fatalf("duplicate processed=%d err=%v, want no-op", processed, err)
	}
}

func repeated(value string, count int) string {
	result := ""
	for i := 0; i < count; i++ {
		result += value
	}
	return result
}

type fakeReconciliationStore struct {
	inputs  []retrieval.ReleaseEvidenceReconciliationInput
	results []retrieval.ReleaseEvidenceReconciliationResult
}

func (s *fakeReconciliationStore) ListReleaseEvidenceReconciliationInputs(context.Context, memory.Scope, int) ([]retrieval.ReleaseEvidenceReconciliationInput, error) {
	return s.inputs, nil
}

func (s *fakeReconciliationStore) ApplyReleaseEvidenceReconciliation(_ context.Context, _ memory.Scope, _ string, result retrieval.ReleaseEvidenceReconciliationResult) error {
	s.results = append(s.results, result)
	return nil
}

type fakeExecutionStore struct{ started map[string]bool }

func (s *fakeExecutionStore) BeginJobExecution(_ context.Context, execution JobExecution) (bool, error) {
	if s.started == nil {
		s.started = map[string]bool{}
	}
	if s.started[execution.IdempotencyKey] {
		return false, nil
	}
	s.started[execution.IdempotencyKey] = true
	return true, nil
}
func (s *fakeExecutionStore) CompleteJobExecution(context.Context, JobExecutionCompletion) error {
	return nil
}
func (s *fakeExecutionStore) FailJobExecution(context.Context, JobExecutionFailure) error { return nil }
