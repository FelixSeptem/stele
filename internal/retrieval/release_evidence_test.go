package retrieval

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/FelixSeptem/stele/internal/memory"
)

func TestRunOwnedReleaseEvidenceNeverFallsBackToRuntimeDSN(t *testing.T) {
	in := validReleaseEvidenceInput(t)
	report, err := RunOwnedReleaseEvidence(context.Background(), ReleaseEvidenceRunRequest{
		Scope: in.Scope, EvaluationDSN: "", RuntimeDSN: in.Prerequisites.RuntimeDSN,
		ProviderProfile: in.ProviderProfile, Policy: in.Policy, Baseline: in.Baseline,
		Candidate: in.Candidate, Progressive: in.Progressive, ParentFirst: in.ParentFirst,
		PostgreSQLReady: true, PGVectorReady: true, FixtureCompatible: true, ProjectionFresh: true, RollbackTested: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	if report.ReleaseEligible || report.Verdict != ReleaseEvidenceSkipped || len(report.FailureCategories) != 1 || report.FailureCategories[0] != ReleaseEvidenceSkipDSN {
		t.Fatalf("report=%+v", report)
	}
}

func TestMarshalReleaseEvidenceReportIsBoundedAndRedacted(t *testing.T) {
	report := validReleaseEvidenceInput(t)
	evidence, err := EvaluateReleaseEvidence(report)
	if err != nil {
		t.Fatal(err)
	}
	payload, err := MarshalReleaseEvidenceReport(evidence)
	if err != nil {
		t.Fatal(err)
	}
	var decoded map[string]any
	if err := json.Unmarshal(payload, &decoded); err != nil {
		t.Fatal(err)
	}
	encoded := string(payload)
	for _, forbidden := range []string{"postgres://", "tenant-secret", "raw-query", "provider-payload", "memory_id", "event_id"} {
		if strings.Contains(encoded, forbidden) {
			t.Fatalf("payload leaked %q: %s", forbidden, encoded)
		}
	}
	if _, ok := decoded["scope_hash"]; !ok {
		t.Fatalf("payload=%s", encoded)
	}
	if got := RenderReleaseEvidenceSummary(evidence); !strings.Contains(got, "retrieval release evidence") {
		t.Fatalf("summary=%q", got)
	}
}

func TestEvaluateReleaseEvidenceSkipsWithoutOwnedEvaluationDSN(t *testing.T) {
	in := validReleaseEvidenceInput(t)
	in.Prerequisites.EvaluationDSN = ""
	report, err := EvaluateReleaseEvidence(in)
	if err != nil {
		t.Fatal(err)
	}
	if report.Verdict != ReleaseEvidenceSkipped || report.ReleaseEligible || report.FailureCategories[0] != ReleaseEvidenceSkipDSN {
		t.Fatalf("report=%+v", report)
	}
}

func TestEvaluateReleaseEvidenceRejectsRuntimeDSNFallbackAndSafetyFailure(t *testing.T) {
	in := validReleaseEvidenceInput(t)
	in.Prerequisites.RuntimeDSN = in.Prerequisites.EvaluationDSN
	report, err := EvaluateReleaseEvidence(in)
	if err != nil {
		t.Fatal(err)
	}
	if report.ReleaseEligible || report.Verdict != ReleaseEvidenceSkipped || report.FailureCategories[0] != ReleaseEvidenceIncompatible {
		t.Fatalf("runtime fallback report=%+v", report)
	}
	in = validReleaseEvidenceInput(t)
	in.Candidate.SafetyFailures = []EvaluationSafetyFailure{{Category: EvaluationSafetyFailureCrossScope, Count: 1}}
	report, err = EvaluateReleaseEvidence(in)
	if err != nil {
		t.Fatal(err)
	}
	if report.ReleaseEligible || report.Verdict != ReleaseEvidenceRejected {
		t.Fatalf("safety report=%+v", report)
	}
}

func TestEvaluateReleaseEvidenceRejectsTemporalPolicyWithoutTemporalCoverage(t *testing.T) {
	const temporalEvidenceRequired = "RETRIEVAL_RELEASE_EVIDENCE_TEMPORAL_REQUIRED"
	in := validReleaseEvidenceInput(t)
	in.Candidate.Metadata.TemporalPolicyVersion = "bi-temporal-policy-v1"
	in.Candidate.Metadata.TemporalCoverageVersion = "temporal-coverage-v1"

	report, err := EvaluateReleaseEvidence(in)
	if err != nil {
		t.Fatal(err)
	}
	if report.ReleaseEligible || report.Verdict != ReleaseEvidenceRejected || !containsString(report.FailureCategories, temporalEvidenceRequired) {
		t.Fatalf("report=%+v, want missing temporal coverage to be a hard non-pass", report)
	}
}

func TestEvaluateReleaseEvidenceRequiresProgressiveAndParentFirstEligibility(t *testing.T) {
	in := validReleaseEvidenceInput(t)
	in.Progressive.Levels[1].Eligible = false
	in.ParentFirst.Eligible = false
	report, err := EvaluateReleaseEvidence(in)
	if err != nil {
		t.Fatal(err)
	}
	if report.ReleaseEligible || !containsString(report.FailureCategories, ReleaseEvidenceProgressiveFailure) || !containsString(report.FailureCategories, ReleaseEvidenceParentFirstFailure) {
		t.Fatalf("report=%+v", report)
	}
}

func validReleaseEvidenceInput(t *testing.T) ReleaseEvidenceInput {
	t.Helper()
	now := time.Date(2026, 9, 15, 12, 0, 0, 0, time.UTC)
	metadata := EvaluationRankingMetadata{FixtureVersion: "fixture-v1", RepresentationVersion: "representation-v1", RankingVersion: "ranking-v1", FusionStrategy: "rrf:v1", CompatibleEmbeddingRevision: "embedding-v1", PolicyVersion: "policy-v1"}
	baseline := EvaluationReport{Metadata: metadata, Metrics: EvaluationMetricReport{RecallAt1: 1, MultiHopEvidenceCoverage: 1, EvidenceCoverage: 1, P95LatencyMS: 10}}
	candidate := baseline
	policy := EvaluationReleasePolicy{Version: "policy-v1", ProtectedCutoffs: []int{1}, MaxP95LatencyMS: 100, Prerequisites: EvaluationPrerequisites{RequireRealStack: true, RequireOwnedDSN: true}, Rollback: EvaluationRollbackContract{Enabled: true, Strategy: "flat-v1"}}
	return ReleaseEvidenceInput{Scope: memory.Scope{Tenant: "t", Project: "p", Namespace: "n"}, ProviderProfile: "canonical-v1", Policy: policy, Baseline: baseline, Candidate: candidate, Progressive: ProgressiveContextEvaluationReport{BaselineIdentity: "baseline-v1", Levels: []ProgressiveContextLevelReport{{Identity: "short-v1", Eligible: true}, {Identity: "medium-v1", Eligible: true}, {Identity: "canonical-v1", Eligible: true}}}, ParentFirst: ParentFirstEvaluationReport{StrategyIdentity: "parent-first-v1", Mode: ParentFirstModeShadow, Eligible: true}, Prerequisites: ReleaseEvidencePrerequisites{EvaluationDSN: "postgres://owned/eval", PostgreSQLReady: true, PGVectorReady: true, FixtureCompatible: true, ProjectionFresh: true, RollbackTested: true}, EvaluatedAt: now}
}

func containsString(values []string, want string) bool {
	for _, value := range values {
		if strings.TrimSpace(value) == want {
			return true
		}
	}
	return false
}
