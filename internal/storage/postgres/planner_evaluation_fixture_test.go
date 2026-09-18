package postgres

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/FelixSeptem/stele/internal/memory"
	"github.com/FelixSeptem/stele/internal/retrieval"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

const plannerEvaluationEmbeddingRevision = "planner-unit-x3-v1"

func plannerEvaluationOwnedDSN() (string, error) {
	dsn, reason := retrieval.OwnedEvaluationDSN()
	if reason != "" {
		return "", errors.New(reason)
	}
	return dsn, nil
}

func plannerEvaluationFixtureForRun(source retrieval.EvaluationFixture, runID string) (retrieval.EvaluationFixture, error) {
	runID = strings.TrimSpace(runID)
	if runID == "" {
		return retrieval.EvaluationFixture{}, errors.New("planner evaluation run identity is required")
	}
	for _, char := range runID {
		if (char < 'a' || char > 'z') && (char < 'A' || char > 'Z') && (char < '0' || char > '9') && char != '-' && char != '_' {
			return retrieval.EvaluationFixture{}, errors.New("planner evaluation run identity is invalid")
		}
	}
	result := source
	result.Cases = append([]retrieval.EvaluationCase(nil), source.Cases...)
	for index := range result.Cases {
		result.Cases[index].Sources = append([]retrieval.EvaluationSource(nil), source.Cases[index].Sources...)
		scope := source.Cases[index].Scope.Normalized()
		scope.Namespace += "-run-" + strings.ToLower(runID)
		result.Cases[index].Scope = scope
	}
	if err := result.Validate(); err != nil {
		return retrieval.EvaluationFixture{}, err
	}
	return result, nil
}

func TestPlannerEvaluationHarnessRejectsUnsafeDSNBeforeBootstrap(t *testing.T) {
	t.Setenv("STELE_TEST_RETRIEVAL_EVALUATION_DSN", "postgres://owned/evaluation")
	t.Setenv("STELE_TEST_RETRIEVAL_EVALUATION_OWNED", "")
	if _, err := plannerEvaluationOwnedDSN(); err == nil || err.Error() != retrieval.RetrievalEvaluationDSNOwnershipRequired {
		t.Fatalf("err=%v", err)
	}
}

func TestPlannerEvaluationFixtureBuildsExactScopePolicies(t *testing.T) {
	fixture := loadPlannerRetrievalEvaluationFixture(t)
	policy, err := plannerEvaluationPlanPolicy(fixture)
	if err != nil {
		t.Fatal(err)
	}
	if err := policy.Validate(); err != nil {
		t.Fatal(err)
	}
	reader := plannerEvaluationPolicyReader{planner: plannerEvaluationRollouts(fixture, memory.RankingRolloutPolicyStatusActiveForScope), queryAnalysis: plannerEvaluationQueryAnalysisRollouts(fixture)}
	for _, item := range fixture.Cases {
		rollout, err := reader.ReadEffectiveRetrievalPlannerRolloutPolicy(context.Background(), memory.ReadEffectiveRetrievalPlannerRolloutPolicyInput{Scope: item.Scope, Surface: memory.RankingRolloutSurfaceSearch})
		if err != nil || rollout.Scope.Normalized() != item.Scope.Normalized() || rollout.RetrievalPlanner == nil {
			t.Fatalf("scope=%+v rollout=%+v err=%v", item.Scope, rollout, err)
		}
	}
	foreign := memory.Scope{Tenant: "eval", Project: "retrieval-baseline", Namespace: "foreign"}
	if _, err := reader.ReadEffectiveRetrievalPlannerRolloutPolicy(context.Background(), memory.ReadEffectiveRetrievalPlannerRolloutPolicyInput{Scope: foreign, Surface: memory.RankingRolloutSurfaceSearch}); !errors.Is(err, pgx.ErrNoRows) {
		t.Fatalf("foreign scope planner error=%v", err)
	}
}

func TestPlannerEvaluationFixtureRunIsolationOwnsDistinctScopesAndCleanupIDs(t *testing.T) {
	original := loadPlannerRetrievalEvaluationFixture(t)
	runA, err := plannerEvaluationFixtureForRun(original, "run-a")
	if err != nil {
		t.Fatal(err)
	}
	runAAgain, err := plannerEvaluationFixtureForRun(original, "run-a")
	if err != nil {
		t.Fatal(err)
	}
	runB, err := plannerEvaluationFixtureForRun(original, "run-b")
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(runA, runAAgain) {
		t.Fatal("same run identity did not produce deterministic fixture ownership")
	}
	if reflect.DeepEqual(runA, runB) || reflect.DeepEqual(original, runA) {
		t.Fatal("run fixtures did not isolate or mutated the source fixture")
	}
	for index := range original.Cases {
		if runA.Cases[index].Scope.Normalized() == runB.Cases[index].Scope.Normalized() {
			t.Fatalf("case %q reused scope across runs", original.Cases[index].ID)
		}
		for sourceIndex := range original.Cases[index].Sources {
			source := original.Cases[index].Sources[sourceIndex]
			keyA := evaluationFixtureRecordKey(runA.Version, runA.Cases[index], source)
			keyB := evaluationFixtureRecordKey(runB.Version, runB.Cases[index], source)
			if evaluationFixtureID(keyA+":memory") == evaluationFixtureID(keyB+":memory") || evaluationFixtureID(keyA+":raw-event") == evaluationFixtureID(keyB+":raw-event") {
				t.Fatalf("case %q source %q reused cleanup-owned IDs across runs", original.Cases[index].ID, source.Alias)
			}
		}
	}
}

func TestPlannerEvaluationFixtureDeclaresSemanticVectorEvidence(t *testing.T) {
	fixture := loadPlannerRetrievalEvaluationFixture(t)
	for _, item := range fixture.Cases {
		if item.Planner == nil || item.Planner.Family != retrieval.RetrievalQueryFamilySemantic {
			continue
		}
		for _, source := range item.Sources {
			if source.Alias == "semantic-fact" && source.EmbeddingRevision == plannerEvaluationEmbeddingRevision {
				return
			}
		}
		t.Fatal("semantic planner case does not declare seedable pgvector evidence")
	}
	t.Fatal("semantic planner case is missing")
}

func TestPlannerEvaluationPlanPolicyReservesConfiguredCanonicalFallbackChannels(t *testing.T) {
	fixture := loadPlannerRetrievalEvaluationFixture(t)
	policy, err := plannerEvaluationPlanPolicy(fixture)
	if err != nil {
		t.Fatal(err)
	}
	configured := []retrieval.FusionChannel{
		retrieval.FusionChannelLexical,
		retrieval.FusionChannelSemantic,
		retrieval.FusionChannelRelation,
		retrieval.FusionChannelChunk,
	}
	for _, item := range fixture.Cases {
		if item.Planner == nil {
			continue
		}
		template := policy.Templates[item.Planner.Family]
		planned := make(map[retrieval.FusionChannel]bool, len(template.Channels))
		for _, channel := range template.Channels {
			planned[channel] = true
		}
		for _, channel := range configured {
			allocation := template.FallbackChannelCandidates[channel]
			if planned[channel] && allocation != 0 {
				t.Fatalf("family=%s planned channel=%s fallback=%d, want none", item.Planner.Family, channel, allocation)
			}
			if !planned[channel] && allocation <= 0 {
				t.Fatalf("family=%s omitted configured channel=%s fallback=%d, want positive reserve", item.Planner.Family, channel, allocation)
			}
		}
	}
}

func TestPlannerEvaluationCandidateServiceExecutesEveryFixturePlanWithoutDSN(t *testing.T) {
	fixture := loadPlannerRetrievalEvaluationFixture(t)
	policy, err := plannerEvaluationPlanPolicy(fixture)
	if err != nil {
		t.Fatal(err)
	}
	source := newPlannerEvaluationMemorySource(fixture)
	service := newPlannerEvaluationCandidateService(source, source, source, fixture, policy)
	seed := retrieval.EvaluationFixtureSeed{FixtureVersion: fixture.Version}
	for _, item := range fixture.Cases {
		for _, fixtureSource := range item.Sources {
			seed.Aliases = append(seed.Aliases, retrieval.EvaluationSeededAlias{
				CaseID: item.ID, Alias: fixtureSource.Alias, Scope: item.Scope,
				MemoryID: plannerEvaluationMemoryID(item.ID, fixtureSource.Alias), State: memory.MemoryStateActive,
			})
		}
	}
	metadata := retrieval.EvaluationRankingMetadata{
		FixtureVersion: fixture.Version, RepresentationVersion: "canonical-v1",
		RankingVersion: "planner-candidate-v1", CompatibleEmbeddingRevision: plannerEvaluationEmbeddingRevision,
		PolicyVersion: "planner-quality-policy-v1", AnalysisVersion: string(retrieval.QueryAnalysisPolicyVersionV1),
		AnalysisLimitsVersion: string(retrieval.QueryAnalysisLimitsVersionV1), RolloutDisposition: "active_for_scope",
		LexicalMatchMode: retrieval.LexicalMatchAnyTerms,
		PlannerVersion:   string(retrieval.RetrievalPlannerVersionV1), PlannerPolicyVersion: string(retrieval.RetrievalPlanPolicyVersionV1),
	}
	replay, err := retrieval.NewEvaluationRunner(service).Replay(context.Background(), fixture, seed, metadata)
	if err != nil {
		t.Fatalf("Replay() error = %v", err)
	}
	if len(replay.Cases) != len(fixture.Cases) {
		t.Fatalf("cases=%d, want %d", len(replay.Cases), len(fixture.Cases))
	}
	for _, item := range replay.Cases {
		if item.PlannerIdentity == "" || item.FallbackCategory != "none" {
			t.Fatalf("case=%s planner_identity=%q fallback=%q", item.CaseID, item.PlannerIdentity, item.FallbackCategory)
		}
	}
}

// TestPlannerEvaluationFixtureRunsOwnedPostgresEvaluation is the owned
// PostgreSQL+pgvector baseline/planner entrypoint used by the evaluation script.
func TestPlannerEvaluationFixtureRunsOwnedPostgresEvaluation(t *testing.T) {
	dsn, dsnErr := plannerEvaluationOwnedDSN()
	if dsnErr != nil && dsnErr.Error() == retrieval.RetrievalEvaluationDSNSkip {
		t.Skip("SKIP_RETRIEVAL_EVALUATION_DSN_REQUIRED")
	}
	if dsnErr != nil {
		t.Fatalf("refusing unsafe evaluation database before bootstrap: %v", dsnErr)
	}
	fixture := loadPlannerRetrievalEvaluationFixture(t)
	fixture, err := plannerEvaluationFixtureForRun(fixture, uuid.NewString())
	if err != nil {
		t.Fatal(err)
	}
	planPolicy, err := plannerEvaluationPlanPolicy(fixture)
	if err != nil {
		t.Fatal(err)
	}
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
	plannerEvaluationSeedVectors(t, ctx, repo, fixture, seeded)
	plannerEvaluationRequireOwnedSemanticHit(t, ctx, repo, fixture, seeded)

	baseFixture := plannerEvaluationBaselineFixture(fixture)
	queryPolicies := plannerEvaluationQueryAnalysisRollouts(fixture)
	baseMetadata := retrieval.EvaluationRankingMetadata{
		FixtureVersion: fixture.Version, RepresentationVersion: "canonical-v1",
		RankingVersion: "planner-baseline-v1", CompatibleEmbeddingRevision: plannerEvaluationEmbeddingRevision,
		PolicyVersion: "planner-quality-policy-v1", AnalysisVersion: string(retrieval.QueryAnalysisPolicyVersionV1),
		AnalysisLimitsVersion: string(retrieval.QueryAnalysisLimitsVersionV1), RolloutDisposition: "original_only",
		LexicalMatchMode: retrieval.LexicalMatchAnyTerms,
	}
	baselineService := retrieval.NewService(retrieval.ServiceDependencies{
		Lexical: repo, Semantic: repo, Relations: repo,
		RankingRolloutPolicyReader: plannerEvaluationPolicyReader{queryAnalysis: queryPolicies},
		QueryAnalyzer:              plannerEvaluationAnalyzer{families: plannerEvaluationFamilies(fixture)},
		QueryAnalysisLimits:        retrieval.DefaultQueryAnalysisLimits(),
	})
	candidateMetadata := baseMetadata
	candidateMetadata.RolloutDisposition = "active_for_scope"
	candidateMetadata.PlannerVersion = string(retrieval.RetrievalPlannerVersionV1)
	candidateMetadata.PlannerPolicyVersion = string(retrieval.RetrievalPlanPolicyVersionV1)
	candidateService := newPlannerEvaluationCandidateService(repo, repo, repo, fixture, planPolicy)
	baselineRunner := retrieval.NewEvaluationRunner(baselineService)
	candidateRunner := retrieval.NewEvaluationRunner(candidateService)
	if _, err := baselineRunner.Replay(ctx, baseFixture, seeded, baseMetadata); err != nil {
		t.Fatalf("baseline warm-up Replay() error = %v", err)
	}
	if _, err := candidateRunner.Replay(ctx, fixture, seeded, candidateMetadata); err != nil {
		t.Fatalf("planner warm-up Replay() error = %v", err)
	}
	const measuredRounds = 3
	baselineSamples := make([]retrieval.EvaluationReplay, 0, measuredRounds)
	candidateSamples := make([]retrieval.EvaluationReplay, 0, measuredRounds)
	for round := 0; round < measuredRounds; round++ {
		runBaseline := func() {
			replay, replayErr := baselineRunner.Replay(ctx, baseFixture, seeded, baseMetadata)
			if replayErr != nil {
				t.Fatalf("baseline measured Replay() error = %v", replayErr)
			}
			baselineSamples = append(baselineSamples, replay)
		}
		runCandidate := func() {
			replay, replayErr := candidateRunner.Replay(ctx, fixture, seeded, candidateMetadata)
			if replayErr != nil {
				t.Fatalf("planner measured Replay() error = %v", replayErr)
			}
			candidateSamples = append(candidateSamples, replay)
		}
		if round%2 == 0 {
			runBaseline()
			runCandidate()
		} else {
			runCandidate()
			runBaseline()
		}
	}
	baselineReplay, err := retrieval.AggregateEvaluationReplaySamples(baselineSamples)
	if err != nil {
		t.Fatalf("aggregate baseline samples: %v", err)
	}
	candidateReplay, err := retrieval.AggregateEvaluationReplaySamples(candidateSamples)
	if err != nil {
		t.Fatalf("aggregate planner samples: %v", err)
	}
	plannerEvaluationRequireReplaySemanticEvidence(t, candidateReplay)
	baselineReport, err := retrieval.CalculateEvaluationMetrics(baselineReplay)
	if err != nil {
		t.Fatalf("baseline metrics error = %v", err)
	}
	plannerEvaluationAnnotateBaseline(&baselineReport, fixture)

	rollbackService := retrieval.NewService(retrieval.ServiceDependencies{
		Lexical: repo, Semantic: repo, Relations: repo,
		RankingRolloutPolicyReader: plannerEvaluationPolicyReader{queryAnalysis: queryPolicies, planner: plannerEvaluationRollouts(fixture, memory.RankingRolloutPolicyStatusRolledBack)},
		QueryAnalyzer:              plannerEvaluationAnalyzer{families: plannerEvaluationFamilies(fixture)},
		QueryAnalysisLimits:        retrieval.DefaultQueryAnalysisLimits(), RetrievalPlanPolicy: planPolicy,
	})
	rollbackReplay, err := retrieval.NewEvaluationRunner(rollbackService).Replay(ctx, baseFixture, seeded, baseMetadata)
	if err != nil {
		t.Fatalf("rollback Replay() error = %v", err)
	}
	rollbackVerified := retrieval.PlannerRollbackEquivalent(baselineReplay, rollbackReplay)
	for index := range candidateReplay.Cases {
		candidateReplay.Cases[index].RollbackVerified = rollbackVerified
	}
	candidateReport, err := retrieval.CalculateEvaluationMetrics(candidateReplay)
	if err != nil {
		t.Fatalf("planner metrics error = %v", err)
	}

	protectedFamilies, protectedCategories := plannerEvaluationProtected(fixture)
	releasePolicy := retrieval.EvaluationReleasePolicy{
		Version: "planner-quality-policy-v1", ProtectedCutoffs: []int{1, 5, 10},
		ProtectedCategories: protectedCategories, ProtectedFamilies: protectedFamilies,
		MaxP95LatencyMS: 5000, MaxP95LatencyRegressionMS: 500,
		Planner: retrieval.EvaluationPlannerReleasePolicy{RequireCompatible: true, MaxPasses: 2, MaxCandidateCount: 200, MaxFallbackRate: 0, RequireRollbackTested: true},
	}
	comparison, err := retrieval.CompareEvaluationReports(baselineReport, candidateReport, protectedCategories)
	if err != nil {
		t.Fatalf("CompareEvaluationReports() error = %v", err)
	}
	decision, err := retrieval.EvaluateReleasePolicy(releasePolicy, baselineReport, candidateReport)
	if err != nil {
		t.Fatalf("EvaluateReleasePolicy() error = %v", err)
	}
	if !decision.Eligible {
		t.Fatalf("planner candidate release decision rejected: %+v", decision)
	}

	now := time.Now().UTC()
	phase64 := retrieval.RetrievalEvaluationPrerequisiteEvidence{
		Stage: retrieval.RetrievalEvaluationPhase64, Status: retrieval.RetrievalEvaluationEvidencePassed,
		RealStack: true, Metadata: baselineReport.Metadata, GeneratedAt: now, ExpiresAt: now.Add(24 * time.Hour),
		Metrics: retrieval.RetrievalEvaluationPrerequisiteMetrics{
			DuplicateRate: baselineReport.Metrics.DuplicateRate, MaxDuplicateRate: 1,
			ProtectedCoverage: baselineReport.Metrics.ProtectedRecall, MinProtectedCoverage: 0,
			CandidatePoolSize: baselineReport.Metrics.CandidatePoolSize, MaxCandidatePoolSize: 200,
			P95LatencyMS: baselineReport.Metrics.P95LatencyMS, MaxP95LatencyMS: 5000,
		},
	}
	baselineReport.GeneratedAt, baselineReport.RealStack = now, true
	candidateReport.GeneratedAt, candidateReport.RealStack, candidateReport.ReleaseEligible = now, true, true
	writeOwnedEvaluationArtifacts(t, baselineReport, candidateReport, phase64, comparison, decision)
}

type plannerEvaluationPolicyReader struct {
	queryAnalysis map[string]memory.RankingRolloutPolicy
	planner       map[string]memory.RankingRolloutPolicy
}

func (reader plannerEvaluationPolicyReader) ReadActiveRankingRolloutPolicy(context.Context, memory.ReadActiveRankingRolloutPolicyInput) (memory.RankingRolloutPolicy, error) {
	return memory.RankingRolloutPolicy{}, pgx.ErrNoRows
}

func (reader plannerEvaluationPolicyReader) ReadEffectiveQueryAnalysisRolloutPolicy(_ context.Context, input memory.ReadEffectiveQueryAnalysisRolloutPolicyInput) (memory.RankingRolloutPolicy, error) {
	policy, ok := reader.queryAnalysis[evaluationFixtureScopeKey(input.Scope)]
	if !ok {
		return memory.RankingRolloutPolicy{}, pgx.ErrNoRows
	}
	return policy, nil
}

func (reader plannerEvaluationPolicyReader) ReadEffectiveRetrievalPlannerRolloutPolicy(_ context.Context, input memory.ReadEffectiveRetrievalPlannerRolloutPolicyInput) (memory.RankingRolloutPolicy, error) {
	policy, ok := reader.planner[evaluationFixtureScopeKey(input.Scope)]
	if !ok {
		return memory.RankingRolloutPolicy{}, pgx.ErrNoRows
	}
	return policy, nil
}

type plannerEvaluationAnalyzer struct {
	families map[string]retrieval.RetrievalQueryFamily
}

func (analyzer plannerEvaluationAnalyzer) Analyze(input retrieval.QueryAnalysisInput) (retrieval.QueryAnalysisResult, error) {
	family := analyzer.families[input.AcceptedQuery]
	var hints []retrieval.QueryAnalysisHint
	var derived []retrieval.QueryAnalysisSignal
	switch family {
	case retrieval.RetrievalQueryFamilyExactLookup:
		hints = []retrieval.QueryAnalysisHint{{Kind: retrieval.QueryAnalysisHintIntent, Disposition: retrieval.QueryAnalysisHintPresent, Value: "lookup"}}
	case retrieval.RetrievalQueryFamilyTemporal:
		hints = []retrieval.QueryAnalysisHint{{Kind: retrieval.QueryAnalysisHintTemporal, Disposition: retrieval.QueryAnalysisHintPresent, Value: "current"}}
	case retrieval.RetrievalQueryFamilyEntityRelation:
		hints = []retrieval.QueryAnalysisHint{{Kind: retrieval.QueryAnalysisHintEntity, Disposition: retrieval.QueryAnalysisHintPresent, Value: "pgvector"}}
	case retrieval.RetrievalQueryFamilyMultiHop:
		derived = []retrieval.QueryAnalysisSignal{{Kind: retrieval.QueryAnalysisSignalSubquery, Text: "memory storage"}, {Kind: retrieval.QueryAnalysisSignalSubquery, Text: "memory lifecycle"}}
	case retrieval.RetrievalQueryFamilyProcedural:
		hints = []retrieval.QueryAnalysisHint{{Kind: retrieval.QueryAnalysisHintMemoryClass, Disposition: retrieval.QueryAnalysisHintPresent, Value: string(memory.MemoryClassProcedural)}}
	}
	return retrieval.NewQueryAnalysisResult(input, retrieval.QueryAnalysisDispositionComplete, hints, derived)
}

type plannerEvaluationMemorySource struct {
	byScope map[string][]retrieval.ScoredMemory
}

func newPlannerEvaluationMemorySource(fixture retrieval.EvaluationFixture) *plannerEvaluationMemorySource {
	source := &plannerEvaluationMemorySource{byScope: make(map[string][]retrieval.ScoredMemory, len(fixture.Cases))}
	for _, item := range fixture.Cases {
		for _, fixtureSource := range item.Sources {
			class := fixtureSource.Class
			if class == "" {
				class = memory.MemoryClassEpisodic
			}
			source.byScope[evaluationFixtureScopeKey(item.Scope)] = append(source.byScope[evaluationFixtureScopeKey(item.Scope)], retrieval.ScoredMemory{Memory: memory.CanonicalMemory{
				ID: plannerEvaluationMemoryID(item.ID, fixtureSource.Alias), Scope: item.Scope, Class: class,
				State: memory.MemoryStateActive, Content: fixtureSource.Content,
			}})
		}
	}
	return source
}

func plannerEvaluationMemoryID(caseID, alias string) string {
	return uuid.NewSHA1(uuid.NameSpaceOID, []byte(caseID+"\x00"+alias)).String()
}

func (source *plannerEvaluationMemorySource) SearchLexical(_ context.Context, input retrieval.SearchInput) ([]retrieval.ScoredMemory, error) {
	return source.search(input, retrieval.FusionChannelLexical), nil
}

func (source *plannerEvaluationMemorySource) SearchSemantic(_ context.Context, input retrieval.SearchInput) ([]retrieval.ScoredMemory, error) {
	return source.search(input, retrieval.FusionChannelSemantic), nil
}

func (source *plannerEvaluationMemorySource) SearchRelations(_ context.Context, input retrieval.SearchInput) ([]retrieval.ScoredMemory, error) {
	return source.search(input, retrieval.FusionChannelRelation), nil
}

func (source *plannerEvaluationMemorySource) search(input retrieval.SearchInput, channel retrieval.FusionChannel) []retrieval.ScoredMemory {
	items := append([]retrieval.ScoredMemory(nil), source.byScope[evaluationFixtureScopeKey(input.Scope)]...)
	if input.TopK > 0 && len(items) > input.TopK {
		items = items[:input.TopK]
	}
	for index := range items {
		switch channel {
		case retrieval.FusionChannelLexical:
			items[index].LexicalScore = 1
		case retrieval.FusionChannelSemantic:
			items[index].SemanticScore = 1
		case retrieval.FusionChannelRelation:
			items[index].RelationScore = 1
		}
	}
	return items
}

type plannerEvaluationChunkSearcher struct {
	lexical retrieval.LexicalSearcher
}

func (searcher plannerEvaluationChunkSearcher) SearchChunks(ctx context.Context, input retrieval.ChunkSearchInput) ([]retrieval.ChunkCandidate, error) {
	if searcher.lexical == nil {
		return nil, errors.New("planner evaluation chunk lexical source is required")
	}
	hits, err := searcher.lexical.SearchLexical(ctx, retrieval.SearchInput{
		Scope: input.Scope, Query: input.Query, QueryEmbedding: input.QueryEmbedding,
		Classes: input.Classes, TopK: input.TopK, IncludeSummaries: true, IncludeRelations: true,
	})
	if err != nil {
		return nil, err
	}
	result := make([]retrieval.ChunkCandidate, 0, len(hits))
	for _, hit := range hits {
		content := strings.TrimSpace(hit.Memory.Content)
		if content == "" {
			continue
		}
		source := memory.ChunkSourceReference{
			Kind: memory.ChunkSourceKindCanonicalVersion, ID: hit.Memory.ID + "-evaluation-v1", Version: 1,
			MemoryID: hit.Memory.ID, Scope: hit.Memory.Scope, LifecycleState: memory.MemoryStateActive,
		}
		chunk := memory.MemoryChunk{
			ID:    uuid.NewSHA1(uuid.NameSpaceOID, []byte("planner-evaluation-chunk\x00"+hit.Memory.ID)).String(),
			Scope: hit.Memory.Scope, Source: source, Class: hit.Memory.Class, Content: content,
			SourceRange: memory.ChunkRange{Start: 0, End: len(content)}, CharacterCount: len([]rune(content)),
			TokenCount: len(strings.Fields(content)), LifecycleState: memory.MemoryStateActive,
			PolicyVersion: "planner-evaluation-chunk-v1", RendererVersion: "planner-evaluation-chunk-v1",
		}
		result = append(result, retrieval.ChunkCandidate{
			Chunk: chunk, Parent: hit.Memory,
			Score:     retrieval.ScoreBreakdown{Lexical: hit.LexicalScore, Semantic: hit.SemanticScore, Relation: hit.RelationScore},
			Citations: []retrieval.Citation{{MemoryID: hit.Memory.ID, Operation: "chunk_source"}},
		})
	}
	return result, nil
}

func newPlannerEvaluationCandidateService(
	lexical retrieval.LexicalSearcher,
	semantic retrieval.SemanticSearcher,
	relations retrieval.RelationSearcher,
	fixture retrieval.EvaluationFixture,
	planPolicy retrieval.RetrievalPlanPolicy,
) *retrieval.Service {
	return retrieval.NewService(retrieval.ServiceDependencies{
		Lexical: lexical, Semantic: semantic, Relations: relations,
		Chunks: plannerEvaluationChunkSearcher{lexical: lexical}, ChunkRollout: memory.ChunkRolloutModeActive,
		RankingRolloutPolicyReader: plannerEvaluationPolicyReader{
			queryAnalysis: plannerEvaluationQueryAnalysisRollouts(fixture),
			planner:       plannerEvaluationRollouts(fixture, memory.RankingRolloutPolicyStatusActiveForScope),
		},
		QueryAnalyzer:       plannerEvaluationAnalyzer{families: plannerEvaluationFamilies(fixture)},
		QueryAnalysisLimits: retrieval.DefaultQueryAnalysisLimits(), RetrievalPlanPolicy: planPolicy,
	})
}

func loadPlannerRetrievalEvaluationFixture(t *testing.T) retrieval.EvaluationFixture {
	t.Helper()
	payload, err := os.ReadFile(filepath.Join("..", "..", "retrieval", "testdata", "retrieval-planner-evaluation-fixture-v1.json"))
	if err != nil {
		t.Fatal(err)
	}
	var fixture retrieval.EvaluationFixture
	if err := json.Unmarshal(payload, &fixture); err != nil {
		t.Fatal(err)
	}
	if err := fixture.Validate(); err != nil {
		t.Fatal(err)
	}
	return fixture
}

func plannerEvaluationPlanPolicy(fixture retrieval.EvaluationFixture) (retrieval.RetrievalPlanPolicy, error) {
	policy := retrieval.DefaultRetrievalPlanPolicy()
	for _, item := range fixture.Cases {
		expectation := item.Planner
		if expectation == nil {
			continue
		}
		template := policy.Templates[expectation.Family]
		template.Channels = append([]retrieval.FusionChannel(nil), expectation.Channels...)
		template.ChannelCandidates = make(map[retrieval.FusionChannel]int, len(expectation.Channels))
		for _, channel := range expectation.Channels {
			template.ChannelCandidates[channel] = expectation.MaxCandidatesPerChannel
		}
		template.FallbackChannelCandidates = make(map[retrieval.FusionChannel]int, len(expectation.FallbackChannelCandidates))
		for channel, count := range expectation.FallbackChannelCandidates {
			template.FallbackChannelCandidates[channel] = count
		}
		template.TotalCandidates = expectation.MaxCandidates
		template.Fusion.TotalCandidates = expectation.MaxCandidates
		template.Fusion.PerChannelCandidate = expectation.MaxCandidatesPerChannel
		template.MaxPasses = expectation.ExpectedPasses
		template.RerankerEligible = expectation.RerankerEligible
		template.RerankerHeadroom = 0
		if expectation.RerankerEligible {
			template.RerankerHeadroom = minPlannerEvaluationInt(10, expectation.MaxCandidates)
		}
		template.FollowUp = retrieval.RetrievalPlanFollowUpRule{}
		if expectation.FollowUpEligible {
			template.FollowUp = retrieval.RetrievalPlanFollowUpRule{Enabled: true, MinimumVisible: len(item.ExpectedEvidenceGroups) + 1, CandidateAllocation: expectation.MaxCandidatesPerChannel, Channels: append([]retrieval.FusionChannel(nil), expectation.Channels...)}
		}
		policy.Templates[expectation.Family] = template
	}
	return policy, policy.Validate()
}

func plannerEvaluationQueryAnalysisRollouts(fixture retrieval.EvaluationFixture) map[string]memory.RankingRolloutPolicy {
	limits := retrieval.DefaultQueryAnalysisLimits()
	result := make(map[string]memory.RankingRolloutPolicy, len(fixture.Cases))
	for _, item := range fixture.Cases {
		result[evaluationFixtureScopeKey(item.Scope)] = evaluationFixtureQueryAnalysisPolicy(item.Scope, limits)
	}
	return result
}

func plannerEvaluationRollouts(fixture retrieval.EvaluationFixture, status memory.RankingRolloutPolicyStatus) map[string]memory.RankingRolloutPolicy {
	result := make(map[string]memory.RankingRolloutPolicy, len(fixture.Cases))
	for _, item := range fixture.Cases {
		policy := evaluationFixtureQueryAnalysisPolicy(item.Scope, retrieval.DefaultQueryAnalysisLimits())
		policy.QueryAnalysis = nil
		policy.Status = status
		switch status {
		case memory.RankingRolloutPolicyStatusActiveForScope:
			policy.Mode = memory.RankingRolloutModeActiveForScope
		case memory.RankingRolloutPolicyStatusRolledBack:
			policy.Mode = memory.RankingRolloutModeActiveForScope
		}
		policy.RetrievalPlanner = &memory.RetrievalPlannerRolloutPolicy{
			SchemaVersion:  memory.RetrievalPlannerRolloutSchemaVersionV1,
			PlannerVersion: string(retrieval.RetrievalPlannerVersionV1), PolicyVersion: string(retrieval.RetrievalPlanPolicyVersionV1),
			AnalysisPolicyVersion: string(retrieval.QueryAnalysisPolicyVersionV1), FusionVersion: retrieval.DefaultRRFStrategy().Version,
			RankingVersion: "quality-feature-v1", RendererVersion: "context-renderer-v1",
			MaxCandidates: 200, MaxCandidatesPerChannel: 100, MaxPasses: 2, MaxLatency: 5 * time.Second,
			MaxContextItems: 100, MaxRerankerHeadroom: 100, ExpiresAt: time.Now().Add(time.Hour),
		}
		result[evaluationFixtureScopeKey(item.Scope)] = policy
	}
	return result
}

func plannerEvaluationFamilies(fixture retrieval.EvaluationFixture) map[string]retrieval.RetrievalQueryFamily {
	result := make(map[string]retrieval.RetrievalQueryFamily, len(fixture.Cases))
	for _, item := range fixture.Cases {
		if item.Planner != nil {
			result[item.Query] = item.Planner.Family
		}
	}
	return result
}

func plannerEvaluationBaselineFixture(fixture retrieval.EvaluationFixture) retrieval.EvaluationFixture {
	result := fixture
	result.Cases = append([]retrieval.EvaluationCase(nil), fixture.Cases...)
	for index := range result.Cases {
		result.Cases[index].Planner = nil
	}
	return result
}

func plannerEvaluationAnnotateBaseline(report *retrieval.EvaluationReport, fixture retrieval.EvaluationFixture) {
	byID := make(map[string]*retrieval.EvaluationPlannerExpectation, len(fixture.Cases))
	for _, item := range fixture.Cases {
		byID[item.ID] = item.Planner
	}
	for index := range report.Cases {
		if expectation := byID[report.Cases[index].CaseID]; expectation != nil {
			report.Cases[index].QueryFamily = expectation.Family
			report.Cases[index].PlannerProtected = expectation.Protected
		}
	}
}

func plannerEvaluationProtected(fixture retrieval.EvaluationFixture) ([]retrieval.RetrievalQueryFamily, []string) {
	var families []retrieval.RetrievalQueryFamily
	var categories []string
	for _, item := range fixture.Cases {
		if item.Planner == nil || !item.Planner.Protected {
			continue
		}
		families = append(families, item.Planner.Family)
		categories = append(categories, item.Category)
	}
	sort.Slice(families, func(i, j int) bool { return families[i] < families[j] })
	sort.Strings(categories)
	return families, categories
}

func minPlannerEvaluationInt(left, right int) int {
	if left < right {
		return left
	}
	return right
}

func plannerEvaluationSeedVectors(t *testing.T, ctx context.Context, repo *Repository, fixture retrieval.EvaluationFixture, seeded retrieval.EvaluationFixtureSeed) {
	t.Helper()
	seedByAlias := make(map[string]retrieval.EvaluationSeededAlias, len(seeded.Aliases))
	for _, record := range seeded.Aliases {
		seedByAlias[record.CaseID+"\x00"+record.Alias] = record
	}
	for _, item := range fixture.Cases {
		for _, source := range item.Sources {
			if source.EmbeddingRevision == "" {
				continue
			}
			if source.EmbeddingRevision != plannerEvaluationEmbeddingRevision {
				t.Fatalf("unsupported planner evaluation embedding revision %q", source.EmbeddingRevision)
			}
			record, ok := seedByAlias[item.ID+"\x00"+source.Alias]
			if !ok {
				t.Fatalf("seeded alias is missing for vector evidence case=%q alias=%q", item.ID, source.Alias)
			}
			class := source.Class
			if class == "" {
				class = memory.MemoryClassEpisodic
			}
			generatedAt := time.Now().UTC()
			rebuild := memory.EmbeddingRebuildRecord{
				MemoryID: record.MemoryID, Scope: record.Scope, Class: class, Content: source.Content,
				SourceVersion: 1, ContentHash: contentHash(source.Content), RequestedProvider: "stele-evaluation",
				RequestedModel: source.EmbeddingRevision, RequestedDimensions: 3,
				Status: memory.EmbeddingRebuildStatusPending, RequestedAt: generatedAt,
			}
			if err := repo.RecordEmbeddingRebuildRequired(ctx, rebuild); err != nil {
				t.Fatalf("RecordEmbeddingRebuildRequired(%q) error = %v", source.Alias, err)
			}
			revision := memory.VectorRevision{
				ID: uuid.NewString(), MemoryID: record.MemoryID, Scope: record.Scope,
				SourceVersion: 1, ContentHash: rebuild.ContentHash, Provider: rebuild.RequestedProvider,
				Model: rebuild.RequestedModel, Dimensions: rebuild.RequestedDimensions,
				Embedding: []float32{1, 0, 0}, Status: memory.VectorRevisionStatusGenerated,
				GeneratedAt: generatedAt, ActivatedAt: generatedAt.Add(time.Millisecond), LastRebuildRequest: generatedAt,
			}
			if err := repo.AppendVectorRevision(ctx, revision); err != nil {
				t.Fatalf("AppendVectorRevision(%q) error = %v", source.Alias, err)
			}
			if err := repo.PromoteVectorRevision(ctx, revision); err != nil {
				t.Fatalf("PromoteVectorRevision(%q) error = %v", source.Alias, err)
			}
		}
	}
}

func plannerEvaluationRequireOwnedSemanticHit(t *testing.T, ctx context.Context, repo *Repository, fixture retrieval.EvaluationFixture, seeded retrieval.EvaluationFixtureSeed) {
	t.Helper()
	for _, item := range fixture.Cases {
		if item.Planner == nil || item.Planner.Family != retrieval.RetrievalQueryFamilySemantic {
			continue
		}
		var expectedID string
		for _, record := range seeded.Aliases {
			if record.CaseID == item.ID && record.Alias == "semantic-fact" {
				expectedID = record.MemoryID
				break
			}
		}
		if expectedID == "" {
			t.Fatal("semantic evaluation alias was not seeded")
		}
		hits, err := repo.SearchSemantic(ctx, retrieval.SearchInput{Scope: item.Scope, Query: item.Query, QueryEmbedding: []float32{1, 0, 0}, TopK: 10})
		if err != nil {
			t.Fatalf("SearchSemantic() error = %v", err)
		}
		for _, hit := range hits {
			if hit.Memory.ID == expectedID && hit.SemanticScore > 0 {
				return
			}
		}
		t.Fatalf("owned pgvector search did not return semantic fixture memory %q", expectedID)
	}
	t.Fatal("semantic planner case is missing")
}

func plannerEvaluationRequireReplaySemanticEvidence(t *testing.T, replay retrieval.EvaluationReplay) {
	t.Helper()
	for _, item := range replay.Cases {
		if item.PlannerFamily != retrieval.RetrievalQueryFamilySemantic {
			continue
		}
		for _, candidate := range item.Candidates {
			if candidate.Alias == "semantic-fact" && candidate.Semantic {
				return
			}
		}
		t.Fatal("semantic planner replay did not preserve pgvector channel evidence")
	}
	t.Fatal("semantic planner replay case is missing")
}
