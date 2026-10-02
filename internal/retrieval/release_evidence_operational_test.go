package retrieval

import (
	"strings"
	"testing"
	"time"
)

func TestReleaseEvidenceOperationalCategoriesHaveStableSerialization(t *testing.T) {
	for _, category := range []ReleaseEvidenceOperationalCategory{
		ReleaseEvidenceCategoryDSNRequired,
		ReleaseEvidenceCategoryOwnershipRequired,
		ReleaseEvidenceCategoryRuntimeDSNReuse,
		ReleaseEvidenceCategoryPrerequisiteUnavailable,
		ReleaseEvidenceCategoryTimeout,
		ReleaseEvidenceCategoryIncompleteCleanup,
		ReleaseEvidenceCategoryAttestationMismatch,
		ReleaseEvidenceCategoryRollbackFailed,
	} {
		if strings.TrimSpace(string(category)) == "" || string(category) != string(category) {
			t.Fatalf("category %q is not stable", category)
		}
	}
}

func TestRunOwnedReleaseEvidenceRequiresOwnershipAndDistinctTarget(t *testing.T) {
	t.Setenv("STELE_TEST_RETRIEVAL_EVALUATION_OWNED", "false")
	input := validReleaseEvidenceInput(t)
	request := ReleaseEvidenceRunRequest{Scope: input.Scope, EvaluationDSN: input.Prerequisites.EvaluationDSN, ProviderProfile: input.ProviderProfile, Policy: input.Policy, Baseline: input.Baseline, Candidate: input.Candidate, Progressive: input.Progressive, ParentFirst: input.ParentFirst}
	report, err := RunOwnedReleaseEvidence(nil, request)
	if err != nil || len(report.FailureCategories) != 1 || report.FailureCategories[0] != string(ReleaseEvidenceCategoryOwnershipRequired) {
		t.Fatalf("report=%+v err=%v", report, err)
	}
}

func TestReleaseEvidenceAttestationRejectsStaleOrMismatchedHandoff(t *testing.T) {
	report := validReleaseEvidenceReportForOperationalTest(t)
	attestation := ReleaseEvidenceAttestation{
		RunIdentity: report.RunIdentity, ScopeHash: report.ScopeHash,
		SourceWatermarkHash: report.SourceWatermarkHash, PolicyVersion: report.PolicyVersion,
		FixtureVersion: report.FixtureVersion, IntegrityIdentity: "integrity:v1",
		RollbackVerdict: ReleaseEvidenceRollbackPassed, Freshness: ReleaseEvidenceFresh,
		IssuedAt: report.GeneratedAt, ExpiresAt: report.EvidenceExpiresAt,
	}
	if err := attestation.ValidateFor(report, report.GeneratedAt); err != nil {
		t.Fatalf("valid attestation rejected: %v", err)
	}
	attestation.RunIdentity = "run:" + strings.Repeat("f", 64)
	if err := attestation.ValidateFor(report, report.GeneratedAt); err == nil || !strings.Contains(err.Error(), string(ReleaseEvidenceCategoryAttestationMismatch)) {
		t.Fatalf("mismatched attestation error=%v", err)
	}
	attestation = ReleaseEvidenceAttestation{
		RunIdentity: report.RunIdentity, ScopeHash: report.ScopeHash,
		SourceWatermarkHash: report.SourceWatermarkHash, PolicyVersion: report.PolicyVersion,
		FixtureVersion: report.FixtureVersion, IntegrityIdentity: "integrity:v1",
		RollbackVerdict: ReleaseEvidenceRollbackPassed, Freshness: ReleaseEvidenceFresh,
		IssuedAt: report.GeneratedAt.Add(-2 * time.Hour), ExpiresAt: report.GeneratedAt.Add(-time.Minute),
	}
	if err := attestation.ValidateFor(report, report.GeneratedAt); err == nil || !strings.Contains(err.Error(), string(ReleaseEvidenceCategoryAttestationStale)) {
		t.Fatalf("stale attestation error=%v", err)
	}
}

func TestReleaseEvidenceHandoffEligibilityFailsClosedWithoutCompleteAttestation(t *testing.T) {
	report := validReleaseEvidenceReportForOperationalTest(t)
	report.ReleaseEligible = true
	report.Verdict = ReleaseEvidencePassed
	report.OperationalOutcome = ReleaseEvidenceOperationalOutcome{State: ReleaseEvidenceRunCompleted, Cleanup: ReleaseEvidenceCleanupComplete, Consumable: true}
	if ok, reason := ReleaseEvidenceHandoffEligible(report, report.GeneratedAt); ok || reason != string(ReleaseEvidenceCategoryAttestationMismatch) {
		t.Fatalf("missing attestation eligible=%v reason=%q", ok, reason)
	}
	report.Attestation = &ReleaseEvidenceAttestation{
		RunIdentity: report.RunIdentity, ScopeHash: report.ScopeHash, SourceWatermarkHash: report.SourceWatermarkHash,
		PolicyVersion: report.PolicyVersion, FixtureVersion: report.FixtureVersion, IntegrityIdentity: "integrity:verified",
		RollbackVerdict: ReleaseEvidenceRollbackPassed, Freshness: ReleaseEvidenceFresh,
		IssuedAt: report.GeneratedAt, ExpiresAt: report.EvidenceExpiresAt,
	}
	if ok, reason := ReleaseEvidenceHandoffEligible(report, report.GeneratedAt); !ok || reason != "" {
		t.Fatalf("complete attestation eligible=%v reason=%q", ok, reason)
	}
}

func TestReleaseEvidenceOperationalOutcomeKeepsIncompleteRunsNonConsumable(t *testing.T) {
	outcome := ReleaseEvidenceOperationalOutcome{State: ReleaseEvidenceRunTimedOut, Cleanup: ReleaseEvidenceCleanupIncomplete, Consumable: true}
	outcome.Normalize()
	if outcome.Consumable || outcome.State != ReleaseEvidenceRunTimedOut || outcome.Cleanup != ReleaseEvidenceCleanupIncomplete {
		t.Fatalf("outcome=%+v, want non-consumable timeout", outcome)
	}
	outcome = ReleaseEvidenceOperationalOutcome{State: ReleaseEvidenceRunCompleted, Cleanup: ReleaseEvidenceCleanupComplete, Consumable: true}
	outcome.Normalize()
	if !outcome.Consumable {
		t.Fatalf("completed outcome became non-consumable: %+v", outcome)
	}
}

func TestReleaseEvidenceLifecycleRecordIsRedactedAndRollbackKeepsBaseline(t *testing.T) {
	record := ReleaseEvidenceLifecycleRecord{Operation: ReleaseEvidenceLifecycleDisablement, Result: "completed", RunIdentity: "run:" + strings.Repeat("a", 64), PolicyVersion: "policy-v1", RollbackVerdict: ReleaseEvidenceRollbackPassed}
	if err := record.Validate(); err != nil {
		t.Fatalf("record rejected: %v", err)
	}
	report := validReleaseEvidenceReportForOperationalTest(t)
	report.ReleaseEligible = true
	report = DisableReleaseEvidence(report)
	if report.ReleaseEligible || report.Verdict == ReleaseEvidencePassed {
		t.Fatalf("disablement did not close eligibility: %+v", report)
	}
	report = RollbackReleaseEvidence(report)
	if report.ReleaseEligible || report.Verdict == ReleaseEvidencePassed || !report.RollbackTested {
		t.Fatalf("rollback must preserve disabled baseline until separately authorized: %+v", report)
	}
}

func validReleaseEvidenceReportForOperationalTest(t *testing.T) ReleaseEvidenceReport {
	t.Helper()
	now := time.Date(2026, 10, 1, 12, 0, 0, 0, time.UTC)
	return ReleaseEvidenceReport{
		RunIdentity: "run:" + strings.Repeat("a", 64), ScopeHash: "scope:" + strings.Repeat("b", 64),
		SourceWatermarkHash: "watermark:" + strings.Repeat("c", 64), ProviderProfile: "canonical-v1",
		PolicyVersion: "policy-v1", FixtureVersion: "fixture-v1", EvidenceFreshness: ReleaseEvidenceFresh,
		GeneratedAt: now, EvidenceExpiresAt: now.Add(time.Hour), RollbackTested: true,
	}
}
