package retrieval

import (
	"strings"
	"testing"
	"time"
)

func TestReconcileReleaseEvidenceRevokesExpiredHandoff(t *testing.T) {
	now := time.Date(2026, 10, 3, 12, 0, 0, 0, time.UTC)
	report := reconciliationReport(now.Add(-2*time.Hour), now.Add(-time.Minute))
	result := ReconcileReleaseEvidence(ReleaseEvidenceReconciliationInput{
		ExpectedScopeHash:           report.ScopeHash,
		ExpectedPolicyVersion:       report.PolicyVersion,
		ExpectedFixtureVersion:      report.FixtureVersion,
		ExpectedStrategyIdentity:    report.ProviderProfile,
		ExpectedSourceWatermarkHash: report.SourceWatermarkHash,
		Report:                      report,
		Now:                         now,
	})
	if result.Eligible || result.Reason != ReconciliationReasonFreshnessExpired {
		t.Fatalf("result = %+v, want revoked freshness verdict", result)
	}
}

func TestReconcileReleaseEvidenceRejectsWatermarkDrift(t *testing.T) {
	now := time.Date(2026, 10, 3, 12, 0, 0, 0, time.UTC)
	report := reconciliationReport(now.Add(-time.Hour), now.Add(time.Hour))
	result := ReconcileReleaseEvidence(ReleaseEvidenceReconciliationInput{
		ExpectedScopeHash:           report.ScopeHash,
		ExpectedPolicyVersion:       report.PolicyVersion,
		ExpectedFixtureVersion:      report.FixtureVersion,
		ExpectedStrategyIdentity:    report.ProviderProfile,
		ExpectedSourceWatermarkHash: "watermark:" + strings.Repeat("b", 64),
		Report:                      report,
		Now:                         now,
	})
	if result.Eligible || result.Reason != ReconciliationReasonWatermarkMismatch {
		t.Fatalf("result = %+v, want revoked watermark verdict", result)
	}
}

func TestReconcileReleaseEvidenceRestoresOnlyWithCompatibleHandoff(t *testing.T) {
	now := time.Date(2026, 10, 3, 12, 0, 0, 0, time.UTC)
	report := reconciliationReport(now.Add(-time.Hour), now.Add(time.Hour))
	result := ReconcileReleaseEvidence(ReleaseEvidenceReconciliationInput{
		ExpectedScopeHash:           report.ScopeHash,
		ExpectedPolicyVersion:       report.PolicyVersion,
		ExpectedFixtureVersion:      report.FixtureVersion,
		ExpectedStrategyIdentity:    report.ProviderProfile,
		ExpectedSourceWatermarkHash: report.SourceWatermarkHash,
		Report:                      report,
		Now:                         now,
	})
	if !result.Eligible || result.Reason != ReconciliationReasonEligible {
		t.Fatalf("result = %+v, want eligible verdict", result)
	}
}

func TestReconcileReleaseEvidenceFailsClosedForMissingAttestation(t *testing.T) {
	now := time.Date(2026, 10, 3, 12, 0, 0, 0, time.UTC)
	report := reconciliationReport(now.Add(-time.Hour), now.Add(time.Hour))
	report.Attestation = nil
	result := ReconcileReleaseEvidence(ReleaseEvidenceReconciliationInput{
		ExpectedScopeHash:           report.ScopeHash,
		ExpectedPolicyVersion:       report.PolicyVersion,
		ExpectedFixtureVersion:      report.FixtureVersion,
		ExpectedStrategyIdentity:    report.ProviderProfile,
		ExpectedSourceWatermarkHash: report.SourceWatermarkHash,
		Report:                      report,
		Now:                         now,
	})
	if result.Eligible || result.Reason != ReconciliationReasonAttestationMissing {
		t.Fatalf("result = %+v, want missing-attestation verdict", result)
	}
}

func TestReconciliationLedgerKeepsHistoryAndBlocksStaleRestore(t *testing.T) {
	now := time.Date(2026, 10, 3, 12, 0, 0, 0, time.UTC)
	report := reconciliationReport(now.Add(-2*time.Hour), now.Add(-time.Minute))
	ledger := NewReconciliationLedger()
	stale := ReconcileReleaseEvidence(ReleaseEvidenceReconciliationInput{ExpectedScopeHash: report.ScopeHash, ExpectedPolicyVersion: report.PolicyVersion, ExpectedFixtureVersion: report.FixtureVersion, ExpectedStrategyIdentity: report.ProviderProfile, ExpectedSourceWatermarkHash: report.SourceWatermarkHash, Report: report, Now: now})
	if err := ledger.Apply("handoff-1", "window-1", stale); err != nil {
		t.Fatal(err)
	}
	if err := ledger.Apply("handoff-1", "window-1", stale); err != nil {
		t.Fatal(err)
	}
	if current, ok := ledger.Current("handoff-1"); !ok || current.Result.Eligible {
		t.Fatalf("current = %+v, ok=%v; want revoked", current, ok)
	}
	restored := reconciliationReport(now.Add(-time.Minute), now.Add(time.Hour))
	eligible := ReconcileReleaseEvidence(ReleaseEvidenceReconciliationInput{ExpectedScopeHash: restored.ScopeHash, ExpectedPolicyVersion: restored.PolicyVersion, ExpectedFixtureVersion: restored.FixtureVersion, ExpectedStrategyIdentity: restored.ProviderProfile, ExpectedSourceWatermarkHash: restored.SourceWatermarkHash, Report: restored, Now: now})
	if err := ledger.Apply("handoff-2", "window-2", eligible); err != nil {
		t.Fatal(err)
	}
	if current, ok := ledger.Current("handoff-2"); !ok || !current.Result.Eligible {
		t.Fatalf("current = %+v, ok=%v; want restored eligibility", current, ok)
	}
	if got := len(ledger.History()); got != 2 {
		t.Fatalf("history length = %d, want duplicate replay collapsed to two handoffs", got)
	}
}

func reconciliationReport(issuedAt, expiresAt time.Time) ReleaseEvidenceReport {
	report := ReleaseEvidenceReport{
		RunIdentity:         "run:" + strings.Repeat("a", 64),
		ProviderProfile:     "representation-v1",
		PolicyVersion:       "policy-v1",
		FixtureVersion:      "fixture-v1",
		ScopeHash:           "scope:" + strings.Repeat("s", 64),
		SourceWatermarkHash: "watermark:" + strings.Repeat("w", 64),
		EvidenceFreshness:   ReleaseEvidenceFresh,
		EvidenceExpiresAt:   expiresAt,
		Verdict:             ReleaseEvidencePassed,
		ReleaseEligible:     true,
		RealStack:           true,
		DeterministicReplay: true,
		RollbackTested:      true,
		GeneratedAt:         issuedAt,
		OperationalOutcome:  ReleaseEvidenceOperationalOutcome{State: ReleaseEvidenceRunCompleted, Cleanup: ReleaseEvidenceCleanupComplete, Consumable: true},
	}
	report.Attestation = &ReleaseEvidenceAttestation{
		RunIdentity:         report.RunIdentity,
		ScopeHash:           report.ScopeHash,
		SourceWatermarkHash: report.SourceWatermarkHash,
		PolicyVersion:       report.PolicyVersion,
		FixtureVersion:      report.FixtureVersion,
		RollbackVerdict:     ReleaseEvidenceRollbackPassed,
		Freshness:           ReleaseEvidenceFresh,
		IssuedAt:            issuedAt,
		ExpiresAt:           expiresAt,
	}
	return report
}
