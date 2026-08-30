package retrieval

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/FelixSeptem/stele/internal/memory"
)

func TestRepositoryEvaluationFixtureCoversRequiredRetrievalScenarios(t *testing.T) {
	encoded, err := os.ReadFile(filepath.Join("testdata", "retrieval-evaluation-fixture-v1.json"))
	if err != nil {
		t.Fatalf("read repository fixture: %v", err)
	}

	var fixture EvaluationFixture
	if err := json.Unmarshal(encoded, &fixture); err != nil {
		t.Fatalf("unmarshal repository fixture: %v", err)
	}
	if err := fixture.Validate(); err != nil {
		t.Fatalf("repository fixture Validate() error = %v", err)
	}

	wantCategories := map[string]struct{}{
		"single-fact":      {},
		"multi-hop":        {},
		"temporal":         {},
		"profile":          {},
		"episodic":         {},
		"procedural":       {},
		"summary":          {},
		"relation":         {},
		"contradiction":    {},
		"noisy-neighbor":   {},
		"duplicate":        {},
		"hidden-lifecycle": {},
		"cross-scope":      {},
	}
	for _, item := range fixture.Cases {
		delete(wantCategories, item.Category)
	}
	if len(wantCategories) != 0 {
		t.Fatalf("repository fixture missing categories: %v", wantCategories)
	}
}

func TestEvaluationFixtureValidateAcceptsScopedEvidenceGroups(t *testing.T) {
	fixture := EvaluationFixture{
		Version: "retrieval-fixture-v1",
		Cases: []EvaluationCase{{
			ID:    "multi-hop-1",
			Scope: memory.Scope{Tenant: "eval", Project: "baseline", Namespace: "case-1"},
			Query: "What database and migration policy were selected?",
			Sources: []EvaluationSource{
				{Alias: "database", EventType: "fixture", Content: "PostgreSQL is the system of record."},
				{Alias: "policy", EventType: "fixture", Content: "Migrations only move forward."},
			},
			ExpectedEvidenceGroups: [][]string{{"database"}, {"policy"}},
			ExcludedAliases:        []string{"hidden-memory"},
		}},
	}

	if err := fixture.Validate(); err != nil {
		t.Fatalf("Validate() error = %v", err)
	}
}

func TestEvaluationFixtureValidateRejectsDuplicateAliases(t *testing.T) {
	fixture := EvaluationFixture{
		Version: "retrieval-fixture-v1",
		Cases: []EvaluationCase{{
			ID:    "duplicate-alias",
			Scope: memory.Scope{Tenant: "eval", Project: "baseline", Namespace: "case-1"},
			Query: "query",
			Sources: []EvaluationSource{
				{Alias: "fact", EventType: "fixture", Content: "one"},
				{Alias: "fact", EventType: "fixture", Content: "two"},
			},
			ExpectedEvidenceGroups: [][]string{{"fact"}},
		}},
	}

	err := fixture.Validate()
	if err == nil || !strings.Contains(err.Error(), "duplicate source alias") {
		t.Fatalf("Validate() error = %v, want duplicate source alias", err)
	}
}

func TestEvaluationFixtureValidateRejectsForeignExpectedEvidence(t *testing.T) {
	fixture := EvaluationFixture{
		Version: "retrieval-fixture-v1",
		Cases: []EvaluationCase{{
			ID:                     "foreign-alias",
			Scope:                  memory.Scope{Tenant: "eval", Project: "baseline", Namespace: "case-1"},
			Query:                  "query",
			Sources:                []EvaluationSource{{Alias: "fact", EventType: "fixture", Content: "one"}},
			ExpectedEvidenceGroups: [][]string{{"foreign"}},
		}},
	}

	err := fixture.Validate()
	if err == nil || !strings.Contains(err.Error(), "unknown evidence alias") {
		t.Fatalf("Validate() error = %v, want unknown evidence alias", err)
	}
}

func TestEvaluationFixtureValidateRejectsMissingVersion(t *testing.T) {
	fixture := EvaluationFixture{
		Cases: []EvaluationCase{{
			ID:                     "missing-version",
			Scope:                  memory.Scope{Tenant: "eval", Project: "baseline", Namespace: "case-1"},
			Query:                  "query",
			Sources:                []EvaluationSource{{Alias: "fact", EventType: "fixture", Content: "one"}},
			ExpectedEvidenceGroups: [][]string{{"fact"}},
		}},
	}

	err := fixture.Validate()
	if err == nil || !strings.Contains(err.Error(), "fixture version is required") {
		t.Fatalf("Validate() error = %v, want fixture version is required", err)
	}
}

func TestEvaluationFixtureValidateRejectsMalformedScope(t *testing.T) {
	fixture := EvaluationFixture{
		Version: "retrieval-fixture-v1",
		Cases: []EvaluationCase{{
			ID:                     "missing-namespace",
			Scope:                  memory.Scope{Tenant: "eval", Project: "baseline"},
			Query:                  "query",
			Sources:                []EvaluationSource{{Alias: "fact", EventType: "fixture", Content: "one"}},
			ExpectedEvidenceGroups: [][]string{{"fact"}},
		}},
	}

	err := fixture.Validate()
	if err == nil || !strings.Contains(err.Error(), "namespace is required") {
		t.Fatalf("Validate() error = %v, want namespace is required", err)
	}
}

func TestReleasePolicyValidateRequiresVersionAndProtectedCutoffs(t *testing.T) {
	policy := EvaluationReleasePolicy{
		Version:          "quality-policy-v1",
		ProtectedCutoffs: []int{1, 5, 10},
		MaxP95LatencyMS:  500,
	}
	if err := policy.Validate(); err != nil {
		t.Fatalf("Validate() error = %v", err)
	}
}

func TestReleasePolicyValidateRejectsUnsafeThresholds(t *testing.T) {
	policy := EvaluationReleasePolicy{
		Version:          "quality-policy-v1",
		ProtectedCutoffs: []int{5, 1},
		MaxP95LatencyMS:  0,
	}

	err := policy.Validate()
	if err == nil || !strings.Contains(err.Error(), "strictly increasing") {
		t.Fatalf("Validate() error = %v, want strictly increasing protected cutoffs", err)
	}
}

func TestNewEvaluationFailureRedactsSensitiveCause(t *testing.T) {
	err := NewEvaluationFailure(EvaluationSafetyFailureUnsafeDiagnostics, "postgres://operator:secret@db.internal/evaluation hidden-memory-id foreign-tenant")
	message := err.Error()
	for _, prohibited := range []string{"postgres://", "secret", "hidden-memory-id", "foreign-tenant"} {
		if strings.Contains(message, prohibited) {
			t.Fatalf("failure message %q contains prohibited value %q", message, prohibited)
		}
	}
	if !strings.Contains(message, string(EvaluationSafetyFailureUnsafeDiagnostics)) {
		t.Fatalf("failure message %q does not contain stable category", message)
	}
}

func TestMarshalEvaluationReportExcludesFixturePayloads(t *testing.T) {
	report := EvaluationReport{
		Metadata: EvaluationRankingMetadata{
			FixtureVersion:              "retrieval-fixture-v1",
			RepresentationVersion:       "canonical-v1",
			RankingVersion:              "baseline-v1",
			CompatibleEmbeddingRevision: "deterministic-v1",
			PolicyVersion:               "quality-policy-v1",
		},
		Cases: []EvaluationCaseReport{{
			CaseID:   "visible-case",
			Category: "single-fact",
		}},
	}

	encoded, err := MarshalEvaluationReport(report)
	if err != nil {
		t.Fatalf("MarshalEvaluationReport() error = %v", err)
	}
	for _, prohibited := range []string{"PostgreSQL is the only system of record", "postgres://", "secret", "hidden-memory-id", "foreign-tenant"} {
		if strings.Contains(string(encoded), prohibited) {
			t.Fatalf("report contains prohibited value %q: %s", prohibited, encoded)
		}
	}
}

func TestRenderEvaluationReportIncludesCompatibilityVersions(t *testing.T) {
	report := EvaluationReport{
		Metadata: EvaluationRankingMetadata{
			FixtureVersion:              "retrieval-fixture-v1",
			RepresentationVersion:       "canonical-v1",
			RankingVersion:              "baseline-v1",
			CompatibleEmbeddingRevision: "deterministic-v1",
			PolicyVersion:               "quality-policy-v1",
		},
		Metrics: EvaluationMetricReport{RecallAt5: 1, MRR: 1, P95LatencyMS: 12},
	}

	output, err := RenderEvaluationReport(report)
	if err != nil {
		t.Fatalf("RenderEvaluationReport() error = %v", err)
	}
	for _, want := range []string{"retrieval-fixture-v1", "canonical-v1", "baseline-v1", "deterministic-v1", "quality-policy-v1", "recall@5=1.0000", "p95_latency_ms=12.000"} {
		if !strings.Contains(output, want) {
			t.Fatalf("human report %q does not contain %q", output, want)
		}
	}
}

func TestCompareEvaluationReportsRejectsIncompatibleFixture(t *testing.T) {
	baseline := evaluationComparisonReport("retrieval-fixture-v1", "baseline-v1", 1)
	candidate := evaluationComparisonReport("retrieval-fixture-v2", "candidate-v1", 1)

	_, err := CompareEvaluationReports(baseline, candidate, []string{"single-fact"})
	if err == nil || !strings.Contains(err.Error(), "incompatible fixture version") {
		t.Fatalf("CompareEvaluationReports() error = %v, want incompatible fixture version", err)
	}
}

func TestCompareEvaluationReportsReportsDeltaAndProtectedRegression(t *testing.T) {
	baseline := evaluationComparisonReport("retrieval-fixture-v1", "baseline-v1", 1)
	candidate := evaluationComparisonReport("retrieval-fixture-v1", "candidate-v1", 0.5)

	comparison, err := CompareEvaluationReports(baseline, candidate, []string{"single-fact"})
	if err != nil {
		t.Fatalf("CompareEvaluationReports() error = %v", err)
	}
	if comparison.BaselineRankingVersion != "baseline-v1" || comparison.CandidateRankingVersion != "candidate-v1" {
		t.Fatalf("comparison versions = %+v", comparison)
	}
	if len(comparison.MetricDeltas) == 0 || comparison.MetricDeltas[0].Delta >= 0 {
		t.Fatalf("metric deltas = %+v, want a negative recall delta", comparison.MetricDeltas)
	}
	if len(comparison.ProtectedRegressions) == 0 || comparison.ProtectedRegressions[0].Category != "single-fact" {
		t.Fatalf("protected regressions = %+v, want single-fact regression", comparison.ProtectedRegressions)
	}
}

func evaluationComparisonReport(fixtureVersion, rankingVersion string, recall float64) EvaluationReport {
	return EvaluationReport{
		Metadata: EvaluationRankingMetadata{
			FixtureVersion:              fixtureVersion,
			RepresentationVersion:       "canonical-v1",
			RankingVersion:              rankingVersion,
			CompatibleEmbeddingRevision: "deterministic-v1",
			PolicyVersion:               "quality-policy-v1",
		},
		Metrics: EvaluationMetricReport{RecallAt1: recall, RecallAt5: recall, RecallAt10: recall, MultiHopEvidenceCoverage: recall},
		Cases: []EvaluationCaseReport{{
			Category: "single-fact",
			Metrics:  EvaluationMetricReport{RecallAt1: recall, RecallAt5: recall, RecallAt10: recall, MultiHopEvidenceCoverage: recall},
		}},
	}
}

func TestEvaluateReleasePolicyRejectsProtectedRegressionAndAcceptsCandidate(t *testing.T) {
	policy := EvaluationReleasePolicy{
		Version:                       "quality-policy-v1",
		ProtectedCutoffs:              []int{1, 5, 10},
		MaxRecallRegression:           0,
		MaxMultiHopCoverageRegression: 0,
		MaxP95LatencyMS:               500,
	}
	baseline := evaluationComparisonReport("retrieval-fixture-v1", "baseline-v1", 1)
	regressed := evaluationComparisonReport("retrieval-fixture-v1", "candidate-v1", 0.5)
	decision, err := EvaluateReleasePolicy(policy, baseline, regressed)
	if err != nil {
		t.Fatalf("EvaluateReleasePolicy() error = %v", err)
	}
	if decision.Eligible || len(decision.HardFailures) == 0 || decision.HardFailures[0] != "protected_recall_regression" {
		t.Fatalf("decision = %+v, want protected recall rejection", decision)
	}

	accepted := evaluationComparisonReport("retrieval-fixture-v1", "candidate-v2", 1)
	decision, err = EvaluateReleasePolicy(policy, baseline, accepted)
	if err != nil {
		t.Fatalf("EvaluateReleasePolicy() accepted error = %v", err)
	}
	if !decision.Eligible || len(decision.HardFailures) != 0 {
		t.Fatalf("accepted decision = %+v, want eligible candidate", decision)
	}
}
