package retrieval

import (
	"context"
	"encoding/json"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/FelixSeptem/stele/internal/memory"
)

func TestEvaluationReleasePolicyExposesRepresentationEvidenceRequirement(t *testing.T) {
	field, ok := reflect.TypeOf(EvaluationReleasePolicy{}).FieldByName("RequireRepresentationEvidence")
	if !ok || field.Type.Kind() != reflect.Bool {
		t.Fatal("representation evidence must be an opt-in boolean release prerequisite")
	}
}

func TestRepresentationReleaseEvidenceMissingIsHardNonPass(t *testing.T) {
	in := validReleaseEvidenceInput(t)
	in.Policy.RequireRepresentationEvidence = true
	in.SourceWatermark = "source-watermark-v1"
	in.EvidenceExpiresAt = in.EvaluatedAt.Add(time.Hour)
	in.DeterministicReplay = true

	report, err := EvaluateReleaseEvidence(in)
	if err != nil {
		t.Fatal(err)
	}
	if report.ReleaseEligible || report.Verdict != ReleaseEvidenceRejected || !containsString(report.FailureCategories, "RETRIEVAL_RELEASE_EVIDENCE_REPRESENTATION_REQUIRED") {
		t.Fatalf("missing representation evidence must be rejected: %+v", report)
	}
}

func TestRepresentationReleaseEvidenceIsPrerequisiteOnly(t *testing.T) {
	in := validReleaseEvidenceInput(t)
	in.Policy.RequireRepresentationEvidence = true
	in.SourceWatermark = "watermark-v1"
	in.EvidenceExpiresAt = in.EvaluatedAt.Add(time.Hour)
	in.DeterministicReplay = true
	in.RepresentationReports = validRepresentationReleaseReports(t, in.Scope, in.EvaluatedAt)

	report, err := EvaluateReleaseEvidence(in)
	if err != nil {
		t.Fatal(err)
	}
	if !report.RepresentationEvidenceEligible || report.RepresentationEvidenceCount != 4 {
		t.Fatalf("complete shadow evidence was not recognized: %+v", report)
	}
	for _, representation := range in.RepresentationReports {
		if representation.SelectedStrategy != ParentFirstFlatStrategy || representation.ActivationEligible || representation.Mode != RepresentationEvaluationModeShadow {
			t.Fatalf("representation evidence must remain shadow-only: %+v", representation)
		}
	}
	payload, err := MarshalReleaseEvidenceReport(report)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(payload), "representation-fixture-v1") || strings.Contains(string(payload), "watermark-v1") {
		t.Fatalf("release report leaked representation evidence material: %s", payload)
	}
}

func TestRepresentationReleaseEvidenceFailsClosedForUnsafeReports(t *testing.T) {
	scope := memory.Scope{Tenant: "t", Project: "p", Namespace: "n"}
	now := time.Date(2026, 9, 15, 12, 0, 0, 0, time.UTC)
	for _, test := range []struct {
		name   string
		mutate func([]RepresentationEvaluationReport)
	}{
		{name: "missing family", mutate: func(reports []RepresentationEvaluationReport) { reports[3].Family = reports[2].Family }},
		{name: "foreign scope", mutate: func(reports []RepresentationEvaluationReport) {
			reports[0].ScopeHash = "scope:" + strings.Repeat("0", 64)
		}},
		{name: "source search missing", mutate: func(reports []RepresentationEvaluationReport) { reports[0].SourceSearchRetained = false }},
		{name: "protected fact loss", mutate: func(reports []RepresentationEvaluationReport) { reports[0].CriticalFactRetention = .5 }},
		{name: "rollback required", mutate: func(reports []RepresentationEvaluationReport) { reports[0].RollbackRequired = true }},
		{name: "future timestamp", mutate: func(reports []RepresentationEvaluationReport) { reports[0].EvaluatedAt = now.Add(time.Minute) }},
		{name: "incompatible watermark", mutate: func(reports []RepresentationEvaluationReport) { reports[0].SourceWatermark = "watermark-v2" }},
		{name: "stale report with refreshed envelope", mutate: func(reports []RepresentationEvaluationReport) {
			for i := range reports {
				reports[i].EvaluatedAt = now.Add(-48 * time.Hour)
				reports[i].RunIdentity = representationRunIdentity(reports[i], reports[i].EvaluatedAt)
			}
		}},
	} {
		t.Run(test.name, func(t *testing.T) {
			reports := validRepresentationReleaseReports(t, scope, now)
			test.mutate(reports)
			if err := ValidateRepresentationReleaseEvidence(scope, reports, now, now.Add(time.Hour)); err == nil {
				t.Fatal("unsafe representation evidence was accepted")
			}
		})
	}
}

func TestRepresentationReleaseBindsWatermarkAndEvidenceToHandoff(t *testing.T) {
	in := validReleaseEvidenceInput(t)
	in.Policy.RequireRepresentationEvidence = true
	in.RepresentationReports = validRepresentationReleaseReports(t, in.Scope, in.EvaluatedAt)
	in.SourceWatermark = "different-source-watermark"
	in.EvidenceExpiresAt = in.EvaluatedAt.Add(time.Hour)
	r, err := EvaluateReleaseEvidence(in)
	if err != nil {
		t.Fatal(err)
	}
	if r.RepresentationEvidenceEligible {
		t.Fatal("report watermark was not bound to release watermark")
	}
}

func TestRepresentationReleaseRejectsSyntheticEvidenceDespiteOwnedDSN(t *testing.T) {
	in := validReleaseEvidenceInput(t)
	in.Policy.RequireRepresentationEvidence = true
	in.SourceWatermark = "watermark-v1"
	in.EvidenceExpiresAt = in.EvaluatedAt.Add(time.Hour)
	for _, test := range []struct {
		family    RepresentationTaskFamily
		candidate RepresentationCandidate
	}{
		{RepresentationTaskFamilyExactFacts, RepresentationCandidateExactExcerptWithSearch},
		{RepresentationTaskFamilyCrossDocument, RepresentationCandidateTopicProjectionWithSearch},
		{RepresentationTaskFamilyProfilePreference, RepresentationCandidateTopicProjectionWithSearch},
		{RepresentationTaskFamilyProceduralLesson, RepresentationCandidateHierarchicalWithSearch},
	} {
		input := validRepresentationEvaluationInput(in.Scope)
		input.Family, input.Policy.Candidate, input.EvaluatedAt = test.family, test.candidate, in.EvaluatedAt
		report, err := EvaluateRepresentation(input)
		if err != nil {
			t.Fatal(err)
		}
		in.RepresentationReports = append(in.RepresentationReports, report)
	}
	r, err := EvaluateReleaseEvidence(in)
	if err != nil {
		t.Fatal(err)
	}
	if r.RepresentationEvidenceEligible {
		t.Fatal("synthetic evidence accepted because release DSN was configured")
	}
}

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
	in := validReleaseEvidenceInput(t)
	evidence, err := EvaluateReleaseEvidence(in)
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
	in.SourceWatermark = "source-watermark-v1"
	in.EvidenceExpiresAt = in.EvaluatedAt.Add(time.Hour)
	in.DeterministicReplay = true
	evidence, err = EvaluateReleaseEvidence(in)
	if err != nil {
		t.Fatal(err)
	}
	summary := RenderReleaseEvidenceSummary(evidence)
	for _, want := range []string{"run=run:", "freshness=fresh", "replay=true", "rollback=true"} {
		if !strings.Contains(summary, want) {
			t.Fatalf("summary=%q, missing %q", summary, want)
		}
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

func TestReleaseEvidenceIdentityIsDeterministicAndRedacted(t *testing.T) {
	in := validReleaseEvidenceInput(t)
	in.SourceWatermark = "source-watermark-v1"
	in.EvidenceExpiresAt = in.EvaluatedAt.Add(time.Hour)
	in.DeterministicReplay = true
	report, err := EvaluateReleaseEvidence(in)
	if err != nil {
		t.Fatal(err)
	}
	reportAgain, err := EvaluateReleaseEvidence(in)
	if err != nil {
		t.Fatal(err)
	}
	if report.RunIdentity == "" || report.RunIdentity != reportAgain.RunIdentity {
		t.Fatalf("run identity = %q/%q, want stable non-empty identity", report.RunIdentity, reportAgain.RunIdentity)
	}
	if report.SourceWatermarkHash == "" || report.EvidenceFreshness != ReleaseEvidenceFresh {
		t.Fatalf("freshness metadata = %+v", report)
	}
	payload, err := MarshalReleaseEvidenceReport(report)
	if err != nil {
		t.Fatal(err)
	}
	encoded := string(payload)
	for _, forbidden := range []string{"source-watermark-v1", "tenant-secret", "postgres://owned/eval"} {
		if strings.Contains(encoded, forbidden) {
			t.Fatalf("report leaked %q: %s", forbidden, encoded)
		}
	}
}

func TestReleaseEvidenceRejectsExpiredEvidence(t *testing.T) {
	in := validReleaseEvidenceInput(t)
	in.SourceWatermark = "source-watermark-v1"
	in.EvidenceExpiresAt = in.EvaluatedAt.Add(-time.Nanosecond)
	in.DeterministicReplay = true
	report, err := EvaluateReleaseEvidence(in)
	if err != nil {
		t.Fatal(err)
	}
	if report.ReleaseEligible || report.EvidenceFreshness != ReleaseEvidenceStale || !containsString(report.FailureCategories, ReleaseEvidenceStaleCategory) {
		t.Fatalf("report=%+v, want stale evidence to be a hard non-pass", report)
	}
}

func TestReleaseEvidenceFailsClosedForMissingOwnedEvidenceCategories(t *testing.T) {
	in := validReleaseEvidenceInput(t)
	in.Policy.RequireFreshEvidence = true
	in.Policy.RequireSemanticHit = true
	in.Policy.RequireTrajectoryIntegrity = true
	in.Policy.RequireResourceBudget = true
	in.SourceWatermark = "source-watermark-v1"
	in.EvidenceExpiresAt = in.EvaluatedAt.Add(time.Hour)
	in.Prerequisites.SemanticHitProven = false
	in.Prerequisites.TrajectoryCompatible = false
	in.Prerequisites.ResourceWithinBounds = false
	report, err := EvaluateReleaseEvidence(in)
	if err != nil {
		t.Fatal(err)
	}
	if report.ReleaseEligible || report.Verdict != ReleaseEvidenceRejected {
		t.Fatalf("report=%+v, want rejected evidence", report)
	}
	for _, category := range []string{ReleaseEvidenceSemanticHitRequired, ReleaseEvidenceTrajectoryRequired, ReleaseEvidenceResourceFailure} {
		if !containsString(report.FailureCategories, category) {
			t.Fatalf("report=%+v, missing failure category %q", report, category)
		}
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

func validRepresentationReleaseReports(t *testing.T, scope memory.Scope, evaluatedAt time.Time) []RepresentationEvaluationReport {
	t.Helper()
	cases := []struct {
		family    RepresentationTaskFamily
		candidate RepresentationCandidate
	}{
		{RepresentationTaskFamilyExactFacts, RepresentationCandidateExactExcerptWithSearch},
		{RepresentationTaskFamilyCrossDocument, RepresentationCandidateTopicProjectionWithSearch},
		{RepresentationTaskFamilyProfilePreference, RepresentationCandidateTopicProjectionWithSearch},
		{RepresentationTaskFamilyProceduralLesson, RepresentationCandidateHierarchicalWithSearch},
	}
	reports := make([]RepresentationEvaluationReport, 0, len(cases))
	for _, test := range cases {
		in := validRepresentationEvaluationInput(scope)
		in.RealStack, in.RollbackTested = true, true
		in.Family = test.family
		in.Policy.Candidate = test.candidate
		in.EvaluatedAt = evaluatedAt.Add(-time.Minute)
		report, err := EvaluateRepresentation(in)
		if err != nil {
			t.Fatal(err)
		}
		reports = append(reports, report)
	}
	return reports
}

func TestRepresentationReleaseEvidenceHonorsPersistedExpiry(t *testing.T) {
	scope := memory.Scope{Tenant: "t", Project: "p", Namespace: "n"}
	now := time.Date(2026, 11, 10, 12, 0, 0, 0, time.UTC)
	reports := validRepresentationReleaseReports(t, scope, now)
	persistedExpiry := now.Add(time.Minute)
	reports[0].PersistenceExpiresAt = &persistedExpiry
	if err := ValidateRepresentationReleaseEvidence(scope, reports, now, now.Add(2*time.Minute)); err == nil {
		t.Fatal("caller extended evidence validity beyond persisted expiry")
	}
	if err := ValidateRepresentationReleaseEvidence(scope, reports, now, persistedExpiry); err != nil {
		t.Fatal(err)
	}
	persistedExpiry = now
	if err := ValidateRepresentationReleaseEvidence(scope, reports, now, now.Add(time.Minute)); err == nil {
		t.Fatal("expired persisted evidence accepted")
	}
}

func containsString(values []string, want string) bool {
	for _, value := range values {
		if strings.TrimSpace(value) == want {
			return true
		}
	}
	return false
}
