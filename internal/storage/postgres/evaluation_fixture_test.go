package postgres

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/FelixSeptem/stele/internal/memory"
	"github.com/FelixSeptem/stele/internal/retrieval"
	"github.com/jackc/pgx/v5"
)

const retrievalEvaluationReportDirEnv = "STELE_RETRIEVAL_EVALUATION_REPORT_DIR"

func TestEvaluationFixtureSeederRejectsForeignScopeBeforeWriting(t *testing.T) {
	fixture := retrieval.EvaluationFixture{
		Version: "retrieval-fixture-v1",
		Cases: []retrieval.EvaluationCase{{
			ID:                     "foreign-scope",
			Scope:                  memory.Scope{Tenant: "operator", Project: "production", Namespace: "default"},
			Query:                  "query",
			Sources:                []retrieval.EvaluationSource{{Alias: "fact", EventType: "fixture", Content: "controlled source"}},
			ExpectedEvidenceGroups: [][]string{{"fact"}},
		}},
	}

	_, err := NewEvaluationFixtureSeeder(nil).Seed(context.Background(), fixture)
	if err == nil || !strings.Contains(err.Error(), "evaluation fixture scope is not owned") {
		t.Fatalf("Seed() error = %v, want evaluation fixture scope is not owned", err)
	}
}

func TestEvaluationSeedRecordIDsDeduplicatesAndOrdersKeys(t *testing.T) {
	raw, memories := evaluationSeedRecordIDs(retrieval.EvaluationFixtureSeed{Aliases: []retrieval.EvaluationSeededAlias{
		{RawEventID: "raw-b", MemoryID: "memory-b"},
		{RawEventID: "raw-a", MemoryID: "memory-a"},
		{RawEventID: "raw-b", MemoryID: "memory-b"},
	}})
	if !reflect.DeepEqual(raw, []string{"raw-a", "raw-b"}) || !reflect.DeepEqual(memories, []string{"memory-a", "memory-b"}) {
		t.Fatalf("seed record IDs raw=%#v memories=%#v", raw, memories)
	}
}

func TestEvaluationFixtureScopeAllowsDedicatedBenchmarkProject(t *testing.T) {
	if !isOwnedEvaluationFixtureScope(memory.Scope{Tenant: "benchmark", Project: "benchmark-locomo", Namespace: "run-local"}) {
		t.Fatal("expected benchmark-owned scope to be allowed")
	}
	if isOwnedEvaluationFixtureScope(memory.Scope{Tenant: "benchmark", Project: "production", Namespace: "run-local"}) {
		t.Fatal("expected non-benchmark project to remain disallowed")
	}
}

func TestEvaluationFixtureSeederSeedsOwnedPostgresFixture(t *testing.T) {
	dsn := os.Getenv("STELE_TEST_RETRIEVAL_EVALUATION_DSN")
	if dsn == "" {
		t.Skip("STELE_TEST_RETRIEVAL_EVALUATION_DSN is not configured; skipping real PostgreSQL retrieval evaluation fixture test")
	}

	fixture := loadRetrievalEvaluationFixture(t)
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	pool, err := OpenPool(ctx, dsn)
	if err != nil {
		t.Fatalf("OpenPool() error = %v", err)
	}
	defer pool.Close()
	if err := BootstrapDatabase(ctx, pool); err != nil {
		t.Fatalf("BootstrapDatabase() error = %v", err)
	}
	var pgvectorInstalled bool
	if err := pool.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM pg_extension WHERE extname = 'vector')`).Scan(&pgvectorInstalled); err != nil {
		t.Fatalf("check pgvector extension: %v", err)
	}
	if !pgvectorInstalled {
		t.Skip("SKIP_RETRIEVAL_EVALUATION_PGVECTOR_REQUIRED")
	}

	repo := NewRepository(pool)
	seeder := NewEvaluationFixtureSeeder(repo)
	seeded, err := seeder.SeedBatch(ctx, fixture, 2)
	if err != nil {
		t.Fatalf("SeedBatch() error = %v", err)
	}
	defer func() {
		if cleanupErr := seeder.Cleanup(context.Background(), seeded); cleanupErr != nil {
			t.Errorf("Cleanup() error = %v", cleanupErr)
		}
	}()
	if len(seeded.Aliases) != fixtureSourceCount(fixture) {
		t.Fatalf("seeded aliases = %d, want %d", len(seeded.Aliases), fixtureSourceCount(fixture))
	}
	repeated, err := seeder.SeedBatch(ctx, fixture, 2)
	if err != nil {
		t.Fatalf("repeat SeedBatch() error = %v", err)
	}
	if !reflect.DeepEqual(seeded, repeated) {
		t.Fatalf("repeated SeedBatch() result differs: first=%#v second=%#v", seeded, repeated)
	}

	retrievalService := retrieval.NewService(retrieval.ServiceDependencies{
		Lexical:   repo,
		Semantic:  repo,
		Relations: repo,
	})
	replay, err := retrieval.NewEvaluationRunner(retrievalService).Replay(ctx, fixture, seeded, retrieval.EvaluationRankingMetadata{
		FixtureVersion:              fixture.Version,
		RepresentationVersion:       "canonical-v1",
		RankingVersion:              "baseline-v1",
		CompatibleEmbeddingRevision: "deterministic-v1",
		PolicyVersion:               "quality-policy-v1",
		RolloutDisposition:          "original_only",
	})
	if err != nil {
		t.Fatalf("Replay() error = %v", err)
	}
	report, err := retrieval.CalculateEvaluationMetrics(replay)
	if err != nil {
		t.Fatalf("CalculateEvaluationMetrics() error = %v", err)
	}
	if len(report.SafetyFailures) != 0 {
		t.Fatalf("real PostgreSQL replay safety failures = %+v", report.SafetyFailures)
	}
	if len(report.Cases) != len(fixture.Cases) {
		t.Fatalf("real PostgreSQL replay cases = %d, want %d", len(report.Cases), len(fixture.Cases))
	}

	for _, record := range seeded.Aliases {
		history, err := repo.ReadMemoryHistory(ctx, record.Scope, record.MemoryID, true)
		if err != nil {
			t.Fatalf("ReadMemoryHistory(%q) error = %v", record.Alias, err)
		}
		if len(history.Versions) == 0 || len(history.Provenance) < 2 {
			t.Fatalf("seeded history for %q lacks append-only version/provenance records", record.Alias)
		}
	}
}

// TestEvaluationFixtureRunsOwnedPostgresEvaluation is the real-stack entrypoint
// used by scripts/retrieval-evaluation.ps1. It deliberately requires the
// harness-owned DSN and runs the prerequisite, immutable baseline, analyzed
// candidate, comparison, and release-gate checks through production code.
func TestEvaluationFixtureRunsOwnedPostgresEvaluation(t *testing.T) {
	dsn := os.Getenv("STELE_TEST_RETRIEVAL_EVALUATION_DSN")
	if dsn == "" {
		t.Skip("SKIP_RETRIEVAL_EVALUATION_DSN_REQUIRED")
	}
	fixture := loadRetrievalEvaluationFixture(t)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	pool, err := OpenPool(ctx, dsn)
	if err != nil {
		t.Fatalf("OpenPool() error = %v", err)
	}
	defer pool.Close()
	if err := BootstrapDatabase(ctx, pool); err != nil {
		t.Fatalf("BootstrapDatabase() error = %v", err)
	}
	var pgvectorInstalled bool
	if err := pool.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM pg_extension WHERE extname = 'vector')`).Scan(&pgvectorInstalled); err != nil {
		t.Fatalf("check pgvector extension: %v", err)
	}
	if !pgvectorInstalled {
		t.Fatalf("SKIP_RETRIEVAL_EVALUATION_PGVECTOR_REQUIRED")
	}

	repo := NewRepository(pool)
	seeder := NewEvaluationFixtureSeeder(repo)
	seeded, err := seeder.SeedBatch(ctx, fixture, 2)
	if err != nil {
		t.Fatalf("SeedBatch() error = %v", err)
	}
	defer func() {
		if cleanupErr := seeder.Cleanup(context.Background(), seeded); cleanupErr != nil {
			t.Errorf("Cleanup() error = %v", cleanupErr)
		}
	}()

	baseMetadata := retrieval.EvaluationRankingMetadata{
		FixtureVersion:              fixture.Version,
		RepresentationVersion:       "canonical-v1",
		RankingVersion:              "baseline-v1",
		CompatibleEmbeddingRevision: "deterministic-v1",
		PolicyVersion:               "quality-policy-v1",
		AnalysisVersion:             string(retrieval.QueryAnalysisPolicyVersionV1),
		AnalysisLimitsVersion:       string(retrieval.QueryAnalysisLimitsVersionV1),
		RolloutDisposition:          "original_only",
	}
	candidateMetadata := baseMetadata
	candidateMetadata.RolloutDisposition = "active_for_scope"

	baselineService := retrieval.NewService(retrieval.ServiceDependencies{
		Lexical: repo, Semantic: repo, Relations: repo,
	})
	baselineReplay, err := retrieval.NewEvaluationRunner(baselineService).Replay(ctx, fixture, seeded, baseMetadata)
	if err != nil {
		t.Fatalf("baseline Replay() error = %v", err)
	}
	baselineReport, err := retrieval.CalculateEvaluationMetrics(baselineReplay)
	if err != nil {
		t.Fatalf("baseline metrics error = %v", err)
	}
	if len(baselineReport.SafetyFailures) != 0 {
		t.Fatalf("baseline safety failures = %+v", baselineReport.SafetyFailures)
	}

	limits := retrieval.DefaultQueryAnalysisLimits()
	policyReader := &evaluationFixturePolicyReader{policies: make(map[string]memory.RankingRolloutPolicy, len(fixture.Cases))}
	for _, item := range fixture.Cases {
		policyReader.policies[evaluationFixtureScopeKey(item.Scope)] = evaluationFixtureQueryAnalysisPolicy(item.Scope, limits)
	}
	candidateService := retrieval.NewService(retrieval.ServiceDependencies{
		Lexical: repo, Semantic: repo, Relations: repo,
		RankingRolloutPolicyReader: policyReader,
		QueryAnalyzer:              evaluationFixtureAnalyzer{},
		QueryAnalysisLimits:        limits,
	})
	candidateReplay, err := retrieval.NewEvaluationRunner(candidateService).Replay(ctx, fixture, seeded, candidateMetadata)
	if err != nil {
		t.Fatalf("candidate Replay() error = %v", err)
	}
	candidateReport, err := retrieval.CalculateEvaluationMetrics(candidateReplay)
	if err != nil {
		t.Fatalf("candidate metrics error = %v", err)
	}
	if len(candidateReport.SafetyFailures) != 0 {
		t.Fatalf("candidate safety failures = %+v", candidateReport.SafetyFailures)
	}

	comparison, err := retrieval.CompareEvaluationReports(baselineReport, candidateReport, []string{"single-fact", "temporal", "multi-hop", "multi-hop-budget"})
	if err != nil {
		t.Fatalf("CompareEvaluationReports() error = %v", err)
	}
	_ = comparison
	releasePolicy := retrieval.EvaluationReleasePolicy{
		Version:                       "quality-policy-v1",
		ProtectedCutoffs:              []int{1, 5, 10},
		ProtectedCategories:           []string{"single-fact", "temporal", "multi-hop", "multi-hop-budget"},
		MaxRecallRegression:           0,
		MaxMultiHopCoverageRegression: 0,
		MaxEvidenceCoverageRegression: 0,
		MaxBudgetOmissionIncrease:     0,
		MaxP95LatencyMS:               5000,
		MaxP95LatencyRegressionMS:     500,
	}
	decision, err := retrieval.EvaluateReleasePolicy(releasePolicy, baselineReport, candidateReport)
	if err != nil {
		t.Fatalf("EvaluateReleasePolicy() error = %v", err)
	}
	if !decision.Eligible {
		t.Fatalf("candidate release decision rejected: %+v", decision)
	}

	now := time.Now().UTC()
	phase64Metrics := retrieval.RetrievalEvaluationPrerequisiteMetrics{
		DuplicateRate:        baselineReport.Metrics.DuplicateRate,
		MaxDuplicateRate:     0.1,
		ProtectedCoverage:    baselineReport.Metrics.ProtectedRecall,
		MinProtectedCoverage: 1,
		CandidatePoolSize:    baselineReport.Metrics.CandidatePoolSize,
		MaxCandidatePoolSize: 200,
		P95LatencyMS:         baselineReport.Metrics.P95LatencyMS,
		MaxP95LatencyMS:      5000,
	}
	phase64Status := retrieval.RetrievalEvaluationEvidencePassed
	if phase64Metrics.DuplicateRate > phase64Metrics.MaxDuplicateRate ||
		phase64Metrics.ProtectedCoverage < phase64Metrics.MinProtectedCoverage ||
		phase64Metrics.CandidatePoolSize > phase64Metrics.MaxCandidatePoolSize ||
		phase64Metrics.P95LatencyMS > phase64Metrics.MaxP95LatencyMS {
		phase64Status = retrieval.RetrievalEvaluationEvidenceFailed
	}
	phase64 := retrieval.RetrievalEvaluationPrerequisiteEvidence{
		Stage:       retrieval.RetrievalEvaluationPhase64,
		Status:      phase64Status,
		RealStack:   true,
		Metadata:    baselineReport.Metadata,
		GeneratedAt: now,
		ExpiresAt:   now.Add(24 * time.Hour),
		Metrics:     phase64Metrics,
	}
	gate := retrieval.RetrievalEvaluationGate{Now: now, CandidateMetadata: candidateReport.Metadata, Phase64Evidence: &phase64, BaselineCandidateCompatible: decision.Eligible}

	baselineReport.GeneratedAt, baselineReport.RealStack = now, true
	candidateReport.GeneratedAt, candidateReport.RealStack, candidateReport.ReleaseEligible = now, true, decision.Eligible && gate.ActiveEligible()
	writeOwnedEvaluationArtifacts(t, baselineReport, candidateReport, phase64, comparison, decision)
}

type evaluationFixturePolicyReader struct {
	policies map[string]memory.RankingRolloutPolicy
}

func (r *evaluationFixturePolicyReader) ReadActiveRankingRolloutPolicy(context.Context, memory.ReadActiveRankingRolloutPolicyInput) (memory.RankingRolloutPolicy, error) {
	return memory.RankingRolloutPolicy{}, pgx.ErrNoRows
}

func (r *evaluationFixturePolicyReader) ReadEffectiveQueryAnalysisRolloutPolicy(_ context.Context, input memory.ReadEffectiveQueryAnalysisRolloutPolicyInput) (memory.RankingRolloutPolicy, error) {
	if policy, ok := r.policies[evaluationFixtureScopeKey(input.Scope)]; ok {
		return policy, nil
	}
	return memory.RankingRolloutPolicy{}, errors.New("evaluation policy scope not found")
}

func evaluationFixtureScopeKey(scope memory.Scope) string {
	scope = scope.Normalized()
	return scope.Tenant + "\x00" + scope.Project + "\x00" + scope.Namespace
}

func evaluationFixtureQueryAnalysisPolicy(scope memory.Scope, limits retrieval.QueryAnalysisLimits) memory.RankingRolloutPolicy {
	now := time.Now().UTC()
	return memory.RankingRolloutPolicy{
		ID:              "evaluation-query-analysis",
		Scope:           scope,
		Status:          memory.RankingRolloutPolicyStatusActiveForScope,
		Mode:            memory.RankingRolloutModeActiveForScope,
		Surfaces:        []memory.RankingRolloutSurface{memory.RankingRolloutSurfaceSearch, memory.RankingRolloutSurfaceContext},
		SignalSources:   []memory.RankingRolloutSignalSource{memory.RankingRolloutSignalSourceTaskEvaluations},
		ThresholdStatus: memory.RankingRolloutThresholdStatusSatisfied,
		EvidenceMinimum: 0,
		Actor:           "evaluation",
		Reason:          "owned retrieval evaluation",
		CreatedAt:       now,
		UpdatedAt:       now,
		QueryAnalysis: &memory.QueryAnalysisRolloutPolicy{
			SchemaVersion:          memory.QueryAnalysisRolloutSchemaVersionV1,
			PolicyVersion:          memory.QueryAnalysisPolicyVersionV1,
			LimitsVersion:          memory.QueryAnalysisLimitsVersionV1,
			MaxQueryBytes:          limits.MaxQueryBytes,
			MaxHints:               limits.MaxHints,
			MaxSignals:             limits.MaxSignals,
			MaxSubqueries:          limits.MaxSubqueries,
			MaxTermBytes:           limits.MaxTermBytes,
			MaxSubqueryBytes:       limits.MaxSubqueryBytes,
			MaxAnalysisWork:        limits.MaxAnalysisWork,
			MaxCandidatesPerSignal: limits.MaxCandidatesPerSignal,
			MaxAggregateCandidates: limits.MaxAggregateCandidates,
			MaxElapsed:             limits.MaxElapsed,
			ExpiresAt:              now.Add(24 * time.Hour),
		},
	}
}

type evaluationFixtureAnalyzer struct{}

func (evaluationFixtureAnalyzer) Analyze(input retrieval.QueryAnalysisInput) (retrieval.QueryAnalysisResult, error) {
	if strings.Contains(input.AcceptedQuery, "Which database is authoritative?") {
		return retrieval.QueryAnalysisResult{}, errors.New("fixture analyzer unavailable")
	}
	if strings.Contains(input.AcceptedQuery, "unterminated") {
		return retrieval.QueryAnalysisResult{}, nil
	}
	return (retrieval.RuleBasedQueryAnalyzer{}).Analyze(input)
}

func writeOwnedEvaluationArtifacts(t *testing.T, baseline, candidate retrieval.EvaluationReport, phase64 retrieval.RetrievalEvaluationPrerequisiteEvidence, comparison retrieval.EvaluationComparison, decision retrieval.EvaluationReleaseDecision) {
	t.Helper()
	dir := strings.TrimSpace(os.Getenv(retrievalEvaluationReportDirEnv))
	if dir == "" {
		return
	}
	if err := os.MkdirAll(dir, 0o750); err != nil {
		t.Fatalf("create evaluation report directory: %v", err)
	}
	baselineJSON, err := retrieval.MarshalEvaluationReport(baseline)
	if err != nil {
		t.Fatalf("marshal baseline report: %v", err)
	}
	candidateJSON, err := retrieval.MarshalEvaluationReport(candidate)
	if err != nil {
		t.Fatalf("marshal candidate report: %v", err)
	}
	gateJSON, err := json.Marshal(struct {
		Phase64    retrieval.RetrievalEvaluationPrerequisiteEvidence `json:"phase_6_4"`
		Comparison retrieval.EvaluationComparison                    `json:"comparison"`
		Decision   retrieval.EvaluationReleaseDecision               `json:"decision"`
	}{phase64, comparison, decision})
	if err != nil {
		t.Fatalf("marshal evaluation gate report: %v", err)
	}
	for name, payload := range map[string][]byte{
		"baseline.json": baselineJSON, "candidate.json": candidateJSON, "gate.json": gateJSON,
	} {
		if err := os.WriteFile(filepath.Join(dir, name), payload, 0o640); err != nil {
			t.Fatalf("write evaluation artifact %s: %v", name, err)
		}
	}
	if summary, err := retrieval.RenderEvaluationReport(candidate); err == nil {
		if err := os.WriteFile(filepath.Join(dir, "candidate.txt"), []byte(summary), 0o640); err != nil {
			t.Fatalf("write evaluation summary: %v", err)
		}
	}
}

func loadRetrievalEvaluationFixture(t *testing.T) retrieval.EvaluationFixture {
	t.Helper()
	encoded, err := os.ReadFile(filepath.Join("..", "..", "retrieval", "testdata", "retrieval-evaluation-fixture-v1.json"))
	if err != nil {
		t.Fatalf("read retrieval evaluation fixture: %v", err)
	}
	var fixture retrieval.EvaluationFixture
	if err := json.Unmarshal(encoded, &fixture); err != nil {
		t.Fatalf("unmarshal retrieval evaluation fixture: %v", err)
	}
	return fixture
}

func fixtureSourceCount(fixture retrieval.EvaluationFixture) int {
	count := 0
	for _, item := range fixture.Cases {
		count += len(item.Sources)
	}
	return count
}

func TestEvaluationFixturePolicyReaderIsExactScopeAndRankingInactive(t *testing.T) {
	scope := memory.Scope{Tenant: "eval", Project: "retrieval-baseline", Namespace: "one"}
	other := memory.Scope{Tenant: "eval", Project: "retrieval-baseline", Namespace: "two"}
	reader := &evaluationFixturePolicyReader{policies: map[string]memory.RankingRolloutPolicy{
		evaluationFixtureScopeKey(scope): evaluationFixtureQueryAnalysisPolicy(scope, retrieval.DefaultQueryAnalysisLimits()),
	}}
	if _, err := reader.ReadActiveRankingRolloutPolicy(context.Background(), memory.ReadActiveRankingRolloutPolicyInput{Scope: scope, Surface: memory.RankingRolloutSurfaceSearch}); !errors.Is(err, pgx.ErrNoRows) {
		t.Fatalf("ReadActiveRankingRolloutPolicy() error = %v, want pgx.ErrNoRows", err)
	}
	policy, err := reader.ReadEffectiveQueryAnalysisRolloutPolicy(context.Background(), memory.ReadEffectiveQueryAnalysisRolloutPolicyInput{Scope: scope, Surface: memory.RankingRolloutSurfaceSearch})
	if err != nil || policy.Scope.Normalized() != scope.Normalized() {
		t.Fatalf("ReadEffectiveQueryAnalysisRolloutPolicy() policy=%+v error=%v", policy, err)
	}
	if _, err := reader.ReadEffectiveQueryAnalysisRolloutPolicy(context.Background(), memory.ReadEffectiveQueryAnalysisRolloutPolicyInput{Scope: other, Surface: memory.RankingRolloutSurfaceSearch}); err == nil {
		t.Fatal("foreign scope unexpectedly received query-analysis policy")
	}
}

func TestWriteOwnedEvaluationArtifactsRetainsOnlyRedactedReports(t *testing.T) {
	dir := t.TempDir()
	t.Setenv(retrievalEvaluationReportDirEnv, dir)
	metadata := retrieval.EvaluationRankingMetadata{FixtureVersion: "fixture-v1", RepresentationVersion: "canonical-v1", RankingVersion: "ranking-v1", FusionStrategy: "rrf:rrf-v1", CompatibleEmbeddingRevision: "embedding-v1", PolicyVersion: "quality-policy-v1"}
	baseline := retrieval.EvaluationReport{Metadata: metadata, Cases: []retrieval.EvaluationCaseReport{{CaseID: "case-1", Category: "single-fact"}}}
	candidate := baseline
	candidate.Metadata.RolloutDisposition = "active_for_scope"
	phase64 := retrieval.RetrievalEvaluationPrerequisiteEvidence{Stage: retrieval.RetrievalEvaluationPhase64, Status: retrieval.RetrievalEvaluationEvidencePassed, RealStack: true, Metadata: metadata, GeneratedAt: time.Now().UTC(), ExpiresAt: time.Now().UTC().Add(time.Hour)}
	comparison := retrieval.EvaluationComparison{BaselineRankingVersion: "ranking-v1", CandidateRankingVersion: "ranking-v1", BaselinePolicyVersion: "quality-policy-v1", CandidatePolicyVersion: "quality-policy-v1", SafetyGatePassed: true}
	decision := retrieval.EvaluationReleaseDecision{PolicyVersion: "quality-policy-v1", Eligible: true}
	writeOwnedEvaluationArtifacts(t, baseline, candidate, phase64, comparison, decision)
	for _, name := range []string{"baseline.json", "candidate.json", "candidate.txt", "gate.json"} {
		payload, err := os.ReadFile(filepath.Join(dir, name))
		if err != nil {
			t.Fatalf("read retained artifact %s: %v", name, err)
		}
		for _, forbidden := range []string{"postgres://", "password", "private query", "entity:../../foreign", "candidate_payload"} {
			if strings.Contains(strings.ToLower(string(payload)), forbidden) {
				t.Fatalf("artifact %s contains forbidden material %q", name, forbidden)
			}
		}
	}
}
