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
		"single-fact":          {},
		"multi-hop":            {},
		"temporal":             {},
		"profile":              {},
		"episodic":             {},
		"procedural":           {},
		"summary":              {},
		"relation":             {},
		"contradiction":        {},
		"noisy-neighbor":       {},
		"duplicate":            {},
		"hidden-lifecycle":     {},
		"cross-scope":          {},
		"lineage-dedup":        {},
		"near-identical":       {},
		"multi-hop-budget":     {},
		"distractor-safety":    {},
		"entity-centric":       {},
		"mixed-language":       {},
		"ambiguous":            {},
		"malformed":            {},
		"adversarial":          {},
		"analyzer-unavailable": {},
	}
	for _, item := range fixture.Cases {
		delete(wantCategories, item.Category)
	}
	if len(wantCategories) != 0 {
		t.Fatalf("repository fixture missing categories: %v", wantCategories)
	}
}

func TestRepositoryEvaluationFixtureMatchesRuleBasedAnalyzer(t *testing.T) {
	encoded, err := os.ReadFile(filepath.Join("testdata", "retrieval-evaluation-fixture-v1.json"))
	if err != nil {
		t.Fatal(err)
	}
	var fixture EvaluationFixture
	if err := json.Unmarshal(encoded, &fixture); err != nil {
		t.Fatal(err)
	}
	limits := DefaultQueryAnalysisLimits()
	for _, item := range fixture.Cases {
		if item.ExpectedAnalysis == nil || item.Category == "malformed" || item.Category == "analyzer-unavailable" {
			continue
		}
		t.Run(item.ID, func(t *testing.T) {
			result, err := (RuleBasedQueryAnalyzer{}).Analyze(QueryAnalysisInput{AcceptedQuery: item.Query, PolicyVersion: QueryAnalysisPolicyVersionV1, Limits: limits})
			if err != nil {
				t.Fatal(err)
			}
			diagnostic, err := QueryAnalysisDiagnosticsFromResult(result, limits, queryAnalysisFallbackForResult(result), 0, 0, "active")
			if err != nil {
				t.Fatal(err)
			}
			if err := evaluationAnalysisMatchesExpectation(diagnostic, item.ExpectedAnalysis); err != nil {
				t.Fatalf("fixture analysis expectation does not match analyzer: %v; diagnostic=%+v expectation=%+v", err, diagnostic, *item.ExpectedAnalysis)
			}
		})
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

func TestEvaluationFixtureValidateAnalysisExpectation(t *testing.T) {
	fixture := EvaluationFixture{Version: "retrieval-fixture-v1", Cases: []EvaluationCase{{
		ID: "analysis", Category: "malformed", Scope: memory.Scope{Tenant: "eval", Project: "p", Namespace: "n"}, Query: "q",
		Sources: []EvaluationSource{{Alias: "fact", EventType: "fixture", Content: "fact"}}, ExpectedEvidenceGroups: [][]string{{"fact"}},
		ExpectedAnalysis: &EvaluationAnalysisExpectation{PolicyVersion: QueryAnalysisPolicyVersionV1, LimitsVersion: QueryAnalysisLimitsVersionV1, Disposition: QueryAnalysisDispositionOriginalOnly, Fallback: QueryAnalysisFallbackMalformed, OriginalRetained: true, Categories: []QueryAnalysisDiagnosticCategory{QueryAnalysisDiagnosticMalformed}, MaxSignalCount: 1, MaxCandidateCount: 200},
	}}}
	if err := fixture.Validate(); err != nil {
		t.Fatalf("Validate() error = %v", err)
	}
}

func TestEvaluationFixtureValidateRejectsInvalidAnalysisCategoryAndUnboundedInput(t *testing.T) {
	base := EvaluationCase{ID: "analysis", Category: "single-fact", Scope: memory.Scope{Tenant: "eval", Project: "p", Namespace: "n"}, Query: "q", Sources: []EvaluationSource{{Alias: "fact", EventType: "fixture", Content: "fact"}}, ExpectedEvidenceGroups: [][]string{{"fact"}}}
	base.ExpectedAnalysis = &EvaluationAnalysisExpectation{PolicyVersion: QueryAnalysisPolicyVersionV1, LimitsVersion: QueryAnalysisLimitsVersionV1, Disposition: QueryAnalysisDispositionComplete, Fallback: QueryAnalysisFallbackNone, OriginalRetained: true, Categories: []QueryAnalysisDiagnosticCategory{"bogus"}, MaxSignalCount: 8, MaxSubqueryCount: 4, MaxCandidateCount: 200}
	if err := (EvaluationFixture{Version: "v1", Cases: []EvaluationCase{base}}).Validate(); err == nil || !strings.Contains(err.Error(), "invalid analysis category") {
		t.Fatalf("error = %v", err)
	}
	base.ExpectedAnalysis.Categories = nil
	base.Query = strings.Repeat("x", QueryAnalysisHardMaxQueryBytes+1)
	if err := (EvaluationFixture{Version: "v1", Cases: []EvaluationCase{base}}).Validate(); err == nil || !strings.Contains(err.Error(), "query exceeds") {
		t.Fatalf("error = %v", err)
	}
}

func TestEvaluationFixtureValidateRequiresAnalysisForFailureCategories(t *testing.T) {
	fixture := EvaluationFixture{Version: "v1", Cases: []EvaluationCase{{ID: "bad", Category: "adversarial", Scope: memory.Scope{Tenant: "eval", Project: "p", Namespace: "n"}, Query: "q", Sources: []EvaluationSource{{Alias: "fact", EventType: "fixture", Content: "fact"}}, ExpectedEvidenceGroups: [][]string{{"fact"}}}}}
	if err := fixture.Validate(); err == nil || !strings.Contains(err.Error(), "analysis expectation is required") {
		t.Fatalf("error = %v", err)
	}
}

func TestEvaluationFixtureValidateRejectsUnsafeOrExpectedExclusion(t *testing.T) {
	base := EvaluationCase{ID: "excluded", Category: "single-fact", Scope: memory.Scope{Tenant: "eval", Project: "p", Namespace: "n"}, Query: "q", Sources: []EvaluationSource{{Alias: "fact", EventType: "fixture", Content: "fact"}}, ExpectedEvidenceGroups: [][]string{{"fact"}}, ExcludedAliases: []string{"fact"}}
	if err := (EvaluationFixture{Version: "v1", Cases: []EvaluationCase{base}}).Validate(); err == nil || !strings.Contains(err.Error(), "cannot also be expected") {
		t.Fatalf("error = %v", err)
	}
	base.ExcludedAliases = []string{"foreign scope"}
	if err := (EvaluationFixture{Version: "v1", Cases: []EvaluationCase{base}}).Validate(); err == nil || !strings.Contains(err.Error(), "invalid excluded alias") {
		t.Fatalf("error = %v", err)
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

func TestReleasePolicyValidateAcceptsBoundedVersionedContracts(t *testing.T) {
	p := EvaluationReleasePolicy{Version: "release-policy-v1", ProtectedCutoffs: []int{1, 5, 10}, MaxP95LatencyMS: 500,
		ResourceBudget: EvaluationResourceBudget{MaxCases: 100, MaxCandidates: 200, MaxElapsedMS: 60000},
		Prerequisites:  EvaluationPrerequisites{RequireRealStack: true, RequireOwnedDSN: true},
		Rollback:       EvaluationRollbackContract{Enabled: true, Strategy: "flat-fusion-v1"},
		Retention:      EvaluationRetentionContract{WindowHours: 168, Owner: "retrieval-team"}}
	if err := p.Validate(); err != nil {
		t.Fatalf("Validate() error=%v", err)
	}
}

func TestReleasePolicyValidateRejectsUnknownOrUnboundedContracts(t *testing.T) {
	base := EvaluationReleasePolicy{Version: "release-policy-v1", ProtectedCutoffs: []int{1}, MaxP95LatencyMS: 500}
	tests := []struct {
		name   string
		mutate func(*EvaluationReleasePolicy)
	}{
		{"negative budget", func(p *EvaluationReleasePolicy) { p.ResourceBudget.MaxCases = -1 }},
		{"unbounded elapsed", func(p *EvaluationReleasePolicy) { p.ResourceBudget.MaxElapsedMS = 24*60*60*1000 + 1 }},
		{"unsafe rollback", func(p *EvaluationReleasePolicy) { p.Rollback.Strategy = "https://secret endpoint" }},
		{"unbounded retention", func(p *EvaluationReleasePolicy) { p.Retention.WindowHours = 24*365*10 + 1 }},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := base
			tt.mutate(&p)
			if err := p.Validate(); err == nil {
				t.Fatal("Validate() error=nil")
			}
		})
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

func TestMarshalEvaluationReportRejectsUnsafeFusionStrategyIdentity(t *testing.T) {
	report := EvaluationReport{Metadata: EvaluationRankingMetadata{
		FixtureVersion: "retrieval-fixture-v1", RepresentationVersion: "canonical-v1", RankingVersion: "ranking-v1",
		FusionStrategy: "postgres://operator:secret@db.internal/fusion-error", CompatibleEmbeddingRevision: "embedding-v1", PolicyVersion: "policy-v1",
	}}
	_, err := MarshalEvaluationReport(report)
	if err == nil {
		t.Fatal("MarshalEvaluationReport() error = nil, want unsafe fusion identity rejected")
	}
	for _, prohibited := range []string{"postgres://", "secret", "db.internal"} {
		if strings.Contains(err.Error(), prohibited) {
			t.Fatalf("redacted error %q contains %q", err, prohibited)
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

func TestRenderEvaluationReportIncludesFusionStrategyWithoutRawScores(t *testing.T) {
	report := EvaluationReport{Metadata: EvaluationRankingMetadata{
		FixtureVersion: "fixture-v1", RepresentationVersion: "canonical-v1", RankingVersion: "baseline-v1",
		FusionStrategy: "rrf-v1", CompatibleEmbeddingRevision: "embedding-v1", PolicyVersion: "policy-v1",
	}}
	rendered, err := RenderEvaluationReport(report)
	if err != nil {
		t.Fatalf("RenderEvaluationReport() error = %v", err)
	}
	if !strings.Contains(rendered, "fusion_strategy=rrf-v1") {
		t.Fatalf("rendered report = %q, want fusion strategy identity", rendered)
	}
}

func TestEvaluationReportCarriesBoundedRerankerMetadataWithoutSecrets(t *testing.T) {
	report := EvaluationReport{Metadata: EvaluationRankingMetadata{
		FixtureVersion: "fixture-v1", RepresentationVersion: "repr-v1", RankingVersion: "rank-v2",
		CompatibleEmbeddingRevision: "embed-v1", PolicyVersion: "policy-v1",
		QualityFeatureVersion: "quality-v1", RerankerProvider: "openai-compatible", RerankerVersion: "rerank-v1", RerankerMode: "shadow",
	}, ChangedRankCount: 2, RerankFallbackCounts: map[string]int{"timeout": 1}}
	encoded, err := MarshalEvaluationReport(report)
	if err != nil {
		t.Fatalf("MarshalEvaluationReport() error = %v", err)
	}
	text := string(encoded)
	for _, want := range []string{"quality_feature_version", "reranker_provider", "reranker_version", "reranker_mode", "changed_rank_count", "timeout"} {
		if !strings.Contains(text, want) {
			t.Fatalf("report missing %q: %s", want, text)
		}
	}
	for _, forbidden := range []string{"api_key", "endpoint", "dsn", "postgres://", "raw_score", "query text"} {
		if strings.Contains(text, forbidden) {
			t.Fatalf("report leaked %q: %s", forbidden, text)
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
	candidate := evaluationComparisonReport("retrieval-fixture-v1", "baseline-v1", 0.5)

	comparison, err := CompareEvaluationReports(baseline, candidate, []string{"single-fact"})
	if err != nil {
		t.Fatalf("CompareEvaluationReports() error = %v", err)
	}
	if comparison.BaselineRankingVersion != "baseline-v1" || comparison.CandidateRankingVersion != "baseline-v1" {
		t.Fatalf("comparison versions = %+v", comparison)
	}
	if len(comparison.MetricDeltas) == 0 || comparison.MetricDeltas[0].Delta >= 0 {
		t.Fatalf("metric deltas = %+v, want a negative recall delta", comparison.MetricDeltas)
	}
	if len(comparison.ProtectedRegressions) == 0 || comparison.ProtectedRegressions[0].Category != "single-fact" {
		t.Fatalf("protected regressions = %+v, want single-fact regression", comparison.ProtectedRegressions)
	}
}

func TestCompareEvaluationReportsIdentifiesCompatibleFusionStrategy(t *testing.T) {
	baseline := evaluationComparisonReport("retrieval-fixture-v1", "ranking-v1", 1)
	baseline.Metadata.FusionStrategy = "rrf:rrf-v1"
	candidate := evaluationComparisonReport("retrieval-fixture-v1", "ranking-v1", 1)
	candidate.Metadata.FusionStrategy = "rrf:rrf-v1"

	comparison, err := CompareEvaluationReports(baseline, candidate, nil)
	if err != nil {
		t.Fatalf("CompareEvaluationReports() error = %v", err)
	}
	if comparison.BaselineFusionStrategy != baseline.Metadata.FusionStrategy || comparison.CandidateFusionStrategy != candidate.Metadata.FusionStrategy {
		t.Fatalf("fusion strategies = %+v, want both comparison identities", comparison)
	}
}

func TestCompareEvaluationReportsIncludesDiversityPolicyAndExtendedDeltas(t *testing.T) {
	baseline := evaluationComparisonReport("fixture-v1", "baseline-v1", 1)
	candidate := evaluationComparisonReport("fixture-v1", "baseline-v1", 0.9)
	candidate.Metrics.CandidatePoolSize = 7
	baseline.Metrics.CandidatePoolSize = 10
	candidate.Metrics.EvidenceCoverage = 0.8
	baseline.Metrics.EvidenceCoverage = 1
	comparison, err := CompareEvaluationReports(baseline, candidate, nil)
	if err != nil {
		t.Fatalf("CompareEvaluationReports() error = %v", err)
	}
	if comparison.CandidatePolicyVersion != "quality-policy-v1" {
		t.Fatalf("candidate policy version = %q", comparison.CandidatePolicyVersion)
	}
	want := map[string]bool{"protected_recall": false, "evidence_coverage": false, "candidate_pool_size": false}
	for _, delta := range comparison.MetricDeltas {
		if _, ok := want[delta.Metric]; ok {
			want[delta.Metric] = true
		}
	}
	for metric, found := range want {
		if !found {
			t.Errorf("missing metric delta %q: %+v", metric, comparison.MetricDeltas)
		}
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
	regressed := evaluationComparisonReport("retrieval-fixture-v1", "baseline-v1", 0.5)
	decision, err := EvaluateReleasePolicy(policy, baseline, regressed)
	if err != nil {
		t.Fatalf("EvaluateReleasePolicy() error = %v", err)
	}
	if decision.Eligible || len(decision.HardFailures) == 0 || decision.HardFailures[0] != "protected_recall_regression" {
		t.Fatalf("decision = %+v, want protected recall rejection", decision)
	}

	accepted := evaluationComparisonReport("retrieval-fixture-v1", "baseline-v1", 1)
	decision, err = EvaluateReleasePolicy(policy, baseline, accepted)
	if err != nil {
		t.Fatalf("EvaluateReleasePolicy() accepted error = %v", err)
	}
	if !decision.Eligible || len(decision.HardFailures) != 0 {
		t.Fatalf("accepted decision = %+v, want eligible candidate", decision)
	}
}

func TestEvaluateReleasePolicyRejectsSafetyFailureEvenWhenQualityImproves(t *testing.T) {
	policy := EvaluationReleasePolicy{
		Version:                       "quality-policy-v1",
		ProtectedCutoffs:              []int{1, 5, 10},
		MaxRecallRegression:           0,
		MaxMultiHopCoverageRegression: 0,
		MaxP95LatencyMS:               500,
	}
	baseline := evaluationComparisonReport("retrieval-fixture-v1", "baseline-v1", 0.5)
	candidate := evaluationComparisonReport("retrieval-fixture-v1", "baseline-v1", 1)
	candidate.SafetyFailures = []EvaluationSafetyFailure{{Category: EvaluationSafetyFailureLifecycleVisibility, Count: 1}}

	decision, err := EvaluateReleasePolicy(policy, baseline, candidate)
	if err != nil {
		t.Fatalf("EvaluateReleasePolicy() error = %v", err)
	}
	if decision.Eligible || len(decision.HardFailures) != 1 || decision.HardFailures[0] != "safety_failure" {
		t.Fatalf("safety failure must reject release before quality gains: %+v", decision)
	}
}

func TestEvaluateReleasePolicyRejectsCoverageBudgetAndLatencyRegressions(t *testing.T) {
	policy := EvaluationReleasePolicy{Version: "quality-policy-v1", ProtectedCutoffs: []int{1, 5, 10}, MaxRecallRegression: 1, MaxMultiHopCoverageRegression: 0.1, MaxP95LatencyMS: 500, MaxP95LatencyRegressionMS: 10}
	baseline := evaluationComparisonReport("retrieval-fixture-v1", "baseline-v1", 1)
	baseline.Metrics.EvidenceCoverage = 1
	baseline.Metrics.BudgetOmissionRate = 0
	baseline.Metrics.P95LatencyMS = 20
	candidate := evaluationComparisonReport("retrieval-fixture-v1", "baseline-v1", 1)
	candidate.Metrics.EvidenceCoverage = 0.5
	candidate.Metrics.BudgetOmissionRate = 0.5
	candidate.Metrics.P95LatencyMS = 40
	decision, err := EvaluateReleasePolicy(policy, baseline, candidate)
	if err != nil {
		t.Fatalf("EvaluateReleasePolicy() error = %v", err)
	}
	if decision.Eligible {
		t.Fatalf("decision = %+v, want rejection", decision)
	}
	for _, want := range []string{"protected_evidence_coverage_regression", "budget_omission_regression", "latency_regression"} {
		found := false
		for _, got := range decision.HardFailures {
			if got == want {
				found = true
			}
		}
		if !found {
			t.Errorf("hard failures = %v, missing %q", decision.HardFailures, want)
		}
	}
}

func TestEvaluationReportRejectsUnboundedContextEfficiencyEvidence(t *testing.T) {
	report := EvaluationReport{Metadata: EvaluationRankingMetadata{FixtureVersion: "fixture-v1", RepresentationVersion: "representation-v1", RankingVersion: "ranking-v1", CompatibleEmbeddingRevision: "embedding-v1", PolicyVersion: "policy-v1"}, Metrics: EvaluationMetricReport{ContextEfficiency: &ContextEfficiencyMetrics{RelevantTokenRatio: 1.1}}}
	if _, err := MarshalEvaluationReport(report); err == nil {
		t.Fatal("MarshalEvaluationReport() error = nil, want bounded efficiency rejection")
	}
}

func TestEvaluateReleasePolicyRejectsEfficiencyHardFailures(t *testing.T) {
	policy := EvaluationReleasePolicy{Version: "quality-policy-v1", ProtectedCutoffs: []int{1}, MaxP95LatencyMS: 500}
	policy.MaxDuplicateTokenRate = .2
	policy.MaxStaleTokenRate = .2
	metadata := EvaluationRankingMetadata{FixtureVersion: "fixture-v1", RepresentationVersion: "representation-v1", RankingVersion: "ranking-v1", CompatibleEmbeddingRevision: "embedding-v1", PolicyVersion: policy.Version}
	baseline := EvaluationReport{Metadata: metadata, Metrics: EvaluationMetricReport{ContextEfficiency: &ContextEfficiencyMetrics{DuplicateTokenRate: .1, StaleTokenRate: .1}}}
	candidate := baseline
	candidate.Metrics.ContextEfficiency = &ContextEfficiencyMetrics{DuplicateTokenRate: .4, StaleTokenRate: .3}
	candidate.Metadata.PolicyVersion = policy.Version
	decision, err := EvaluateReleasePolicy(policy, baseline, candidate)
	if err != nil {
		t.Fatal(err)
	}
	if decision.Eligible || len(decision.HardFailures) == 0 {
		t.Fatalf("decision = %+v, want efficiency hard failure", decision)
	}
}

func TestEvaluateReleasePolicyRejectsCrossScopeAndLifecycleFailures(t *testing.T) {
	policy := EvaluationReleasePolicy{Version: "quality-policy-v1", ProtectedCutoffs: []int{1}, MaxP95LatencyMS: 500}
	baseline := evaluationComparisonReport("retrieval-fixture-v1", "baseline-v1", 0.5)
	candidate := evaluationComparisonReport("retrieval-fixture-v1", "baseline-v1", 1)
	candidate.SafetyFailures = []EvaluationSafetyFailure{{Category: EvaluationSafetyFailureCrossScope, Count: 1}, {Category: EvaluationSafetyFailureLifecycleVisibility, Count: 1}}
	decision, err := EvaluateReleasePolicy(policy, baseline, candidate)
	if err != nil {
		t.Fatalf("EvaluateReleasePolicy() error = %v", err)
	}
	if decision.Eligible || len(decision.HardFailures) != 1 || decision.HardFailures[0] != "safety_failure" {
		t.Fatalf("decision = %+v", decision)
	}
}
