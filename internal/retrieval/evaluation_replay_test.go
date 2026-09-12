package retrieval

import (
	"context"
	"encoding/json"
	"math"
	"strings"
	"testing"
	"time"

	"github.com/FelixSeptem/stele/internal/memory"
	"github.com/FelixSeptem/stele/internal/telemetry"
)

type evaluationTelemetryStub struct {
	event telemetry.RetrievalEvaluationEvent
}

func (s *evaluationTelemetryStub) RecordOperation(context.Context, telemetry.OperationEvent) {}
func (s *evaluationTelemetryStub) RecordBacklog(context.Context, telemetry.BacklogEvent)     {}
func (s *evaluationTelemetryStub) RecordRetrievalEvaluation(_ context.Context, event telemetry.RetrievalEvaluationEvent) {
	s.event = event
}

func TestCalculateEvaluationMetricsUsesEvidenceGroupsAndBoundedLatency(t *testing.T) {
	report, err := CalculateEvaluationMetrics(EvaluationReplay{
		Metadata: EvaluationRankingMetadata{
			FixtureVersion:              "retrieval-fixture-v1",
			RepresentationVersion:       "canonical-v1",
			RankingVersion:              "baseline-v1",
			CompatibleEmbeddingRevision: "deterministic-v1",
			PolicyVersion:               "quality-policy-v1",
		},
		Cases: []EvaluationReplayCase{{
			CaseID:                 "multi-hop",
			Category:               "multi-hop",
			Scope:                  memory.Scope{Tenant: "eval", Project: "baseline", Namespace: "metrics"},
			ExpectedEvidenceGroups: [][]string{{"database"}, {"policy"}},
			CandidatePoolSize:      3,
			Latency:                10 * time.Millisecond,
			Candidates: []EvaluationReplayCandidate{
				{Alias: "database", FinalRank: 1, FactCluster: "storage", Scope: memory.Scope{Tenant: "eval", Project: "baseline", Namespace: "metrics"}, State: memory.MemoryStateActive},
				{Alias: "noise", FinalRank: 2, FactCluster: "noise", Scope: memory.Scope{Tenant: "eval", Project: "baseline", Namespace: "metrics"}, State: memory.MemoryStateActive},
				{Alias: "policy", FinalRank: 3, FactCluster: "storage", Scope: memory.Scope{Tenant: "eval", Project: "baseline", Namespace: "metrics"}, State: memory.MemoryStateActive},
			},
		}},
	})
	if err != nil {
		t.Fatalf("CalculateEvaluationMetrics() error = %v", err)
	}
	metrics := report.Metrics
	if metrics.RecallAt1 != 0.5 || metrics.RecallAt5 != 1 || metrics.RecallAt10 != 1 || metrics.MRR != 1 {
		t.Fatalf("metrics = %+v, want recall and MRR from evidence groups", metrics)
	}
	if math.Abs(metrics.NDCGAt5-0.9197207891481876) > 0.0000001 {
		t.Fatalf("nDCG@5 = %f, want grouped binary nDCG", metrics.NDCGAt5)
	}
	if metrics.MultiHopEvidenceCoverage != 1 || math.Abs(metrics.DuplicateRate-(1.0/3.0)) > 0.0000001 {
		t.Fatalf("metrics = %+v, want full multi-hop coverage and one duplicate cluster", metrics)
	}
	if metrics.CandidatePoolSize != 3 || metrics.P50LatencyMS != 10 || metrics.P95LatencyMS != 10 {
		t.Fatalf("metrics = %+v, want bounded pool and latency", metrics)
	}
}

func TestCalculateEvaluationMetricsReportsDiversityDispositionsAndProtectedMetrics(t *testing.T) {
	scope := memory.Scope{Tenant: "eval", Project: "baseline", Namespace: "diversity"}
	report, err := CalculateEvaluationMetrics(EvaluationReplay{
		Metadata: EvaluationRankingMetadata{FixtureVersion: "fixture-v1", RepresentationVersion: "canonical-v1", RankingVersion: "diversity-v1", CompatibleEmbeddingRevision: "embedding-v1", PolicyVersion: "diversity-policy-v1"},
		Cases: []EvaluationReplayCase{{
			Scope: scope, ExpectedEvidenceGroups: [][]string{{"required"}}, CandidatePoolSize: 4, Latency: 12 * time.Millisecond,
			Candidates: []EvaluationReplayCandidate{{Alias: "required", FinalRank: 1, Scope: scope, State: memory.MemoryStateActive}},
			Diagnostics: []EvaluationCandidateDiagnostic{
				{Alias: "required", Disposition: EvaluationCandidateDispositionReturned},
				{Alias: "duplicate", Disposition: EvaluationCandidateDispositionNotReturned},
			},
		}},
	})
	if err != nil {
		t.Fatalf("CalculateEvaluationMetrics() error = %v", err)
	}
	if report.Metadata.PolicyVersion != "diversity-policy-v1" {
		t.Fatalf("policy version = %q", report.Metadata.PolicyVersion)
	}
	if report.DispositionAggregates["returned"] != 1 || report.DispositionAggregates["not_returned"] != 1 {
		t.Fatalf("dispositions = %#v", report.DispositionAggregates)
	}
	if report.Metrics.ProtectedRecall != 1 || report.Metrics.EvidenceCoverage != 1 || report.Metrics.CandidatePoolSize != 4 || report.Metrics.P95LatencyMS != 12 {
		t.Fatalf("metrics = %+v", report.Metrics)
	}
}

func TestCalculateEvaluationMetricsCountsEveryEvidenceGroupSatisfiedAtSameRank(t *testing.T) {
	report, err := CalculateEvaluationMetrics(EvaluationReplay{
		Metadata: EvaluationRankingMetadata{
			FixtureVersion:              "retrieval-fixture-v1",
			RepresentationVersion:       "canonical-v1",
			RankingVersion:              "baseline-v1",
			CompatibleEmbeddingRevision: "deterministic-v1",
			PolicyVersion:               "quality-policy-v1",
		},
		Cases: []EvaluationReplayCase{{
			Scope:                  memory.Scope{Tenant: "eval", Project: "baseline", Namespace: "shared"},
			ExpectedEvidenceGroups: [][]string{{"shared"}, {"shared"}},
			Candidates:             []EvaluationReplayCandidate{{Alias: "shared", FinalRank: 1, Scope: memory.Scope{Tenant: "eval", Project: "baseline", Namespace: "shared"}, State: memory.MemoryStateActive}},
		}},
	})
	if err != nil {
		t.Fatalf("CalculateEvaluationMetrics() error = %v", err)
	}
	if report.Metrics.RecallAt1 != 1 || report.Metrics.MultiHopEvidenceCoverage != 1 {
		t.Fatalf("metrics = %+v, want every evidence group satisfied at rank one", report.Metrics)
	}
}

func TestCalculateEvaluationMetricsHandlesNoCandidates(t *testing.T) {
	report, err := CalculateEvaluationMetrics(EvaluationReplay{
		Metadata: EvaluationRankingMetadata{
			FixtureVersion:              "retrieval-fixture-v1",
			RepresentationVersion:       "canonical-v1",
			RankingVersion:              "baseline-v1",
			CompatibleEmbeddingRevision: "deterministic-v1",
			PolicyVersion:               "quality-policy-v1",
		},
		Cases: []EvaluationReplayCase{{
			Scope:                  memory.Scope{Tenant: "eval", Project: "baseline", Namespace: "empty"},
			ExpectedEvidenceGroups: [][]string{{"missing"}},
		}},
	})
	if err != nil {
		t.Fatalf("CalculateEvaluationMetrics() error = %v", err)
	}
	if report.Metrics.RecallAt1 != 0 || report.Metrics.MRR != 0 || report.Metrics.NDCGAt10 != 0 || report.Metrics.CandidatePoolSize != 0 {
		t.Fatalf("metrics = %+v, want zero values for an empty result set", report.Metrics)
	}
}

func TestCalculateEvaluationMetricsSafetyFailureOverridesQualityMetrics(t *testing.T) {
	allowedScope := memory.Scope{Tenant: "eval", Project: "baseline", Namespace: "case"}
	report, err := CalculateEvaluationMetrics(EvaluationReplay{
		Metadata: EvaluationRankingMetadata{
			FixtureVersion:              "retrieval-fixture-v1",
			RepresentationVersion:       "canonical-v1",
			RankingVersion:              "baseline-v1",
			CompatibleEmbeddingRevision: "deterministic-v1",
			PolicyVersion:               "quality-policy-v1",
		},
		Cases: []EvaluationReplayCase{{
			Scope:                  allowedScope,
			ExpectedEvidenceGroups: [][]string{{"expected"}},
			Candidates: []EvaluationReplayCandidate{{
				Alias:     "expected",
				FinalRank: 1,
				Scope:     memory.Scope{Tenant: "foreign", Project: "project", Namespace: "namespace"},
				State:     memory.MemoryStateActive,
			}},
		}},
	})
	if err != nil {
		t.Fatalf("CalculateEvaluationMetrics() error = %v", err)
	}
	if len(report.SafetyFailures) != 1 || report.SafetyFailures[0].Category != EvaluationSafetyFailureCrossScope {
		t.Fatalf("safety failures = %+v, want cross-scope failure", report.SafetyFailures)
	}
	if report.Metrics.RecallAt1 != 0 || report.Metrics.MRR != 0 {
		t.Fatalf("metrics = %+v, safety failure must override quality metrics", report.Metrics)
	}
}

func TestCalculateEvaluationMetricsClassifiesLifecycleAndMalformedScopeFailures(t *testing.T) {
	metadata := EvaluationRankingMetadata{
		FixtureVersion:              "retrieval-fixture-v1",
		RepresentationVersion:       "canonical-v1",
		RankingVersion:              "baseline-v1",
		CompatibleEmbeddingRevision: "deterministic-v1",
		PolicyVersion:               "quality-policy-v1",
	}
	for _, test := range []struct {
		name     string
		caseRun  EvaluationReplayCase
		category EvaluationSafetyFailureCategory
	}{
		{
			name: "hidden lifecycle",
			caseRun: EvaluationReplayCase{
				Scope: memory.Scope{Tenant: "eval", Project: "baseline", Namespace: "hidden"},
				Candidates: []EvaluationReplayCandidate{{
					Scope: memory.Scope{Tenant: "eval", Project: "baseline", Namespace: "hidden"},
					State: memory.MemoryStateForgotten,
				}},
			},
			category: EvaluationSafetyFailureLifecycleVisibility,
		},
		{
			name: "malformed fixture scope",
			caseRun: EvaluationReplayCase{
				Scope: memory.Scope{Tenant: "eval", Project: "baseline"},
			},
			category: EvaluationSafetyFailureInvalidFixtureScope,
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			report, err := CalculateEvaluationMetrics(EvaluationReplay{Metadata: metadata, Cases: []EvaluationReplayCase{test.caseRun}})
			if err != nil {
				t.Fatalf("CalculateEvaluationMetrics() error = %v", err)
			}
			if len(report.SafetyFailures) != 1 || report.SafetyFailures[0].Category != test.category {
				t.Fatalf("safety failures = %+v, want %q", report.SafetyFailures, test.category)
			}
		})
	}
}

func TestEvaluationReportSuppressesUnsafeFusionCandidatesAndRawDetails(t *testing.T) {
	scope := memory.Scope{Tenant: "eval", Project: "baseline", Namespace: "redaction"}
	report, err := CalculateEvaluationMetrics(EvaluationReplay{
		Metadata: EvaluationRankingMetadata{
			FixtureVersion: "retrieval-fixture-v1", RepresentationVersion: "canonical-v1", RankingVersion: "ranking-v1",
			FusionStrategy: "rrf:rrf-v1", CompatibleEmbeddingRevision: "embedding-v1", PolicyVersion: "quality-policy-v1",
		},
		Cases: []EvaluationReplayCase{{
			CaseID: "redaction", Scope: scope, ExpectedEvidenceGroups: [][]string{{"visible"}},
			Candidates: []EvaluationReplayCandidate{
				{Alias: "visible", MemoryID: "visible-memory", Scope: scope, State: memory.MemoryStateActive, FinalRank: 1, lexicalScore: 0.987654},
				{Alias: "foreign-alias", MemoryID: "foreign-memory-id", Scope: memory.Scope{Tenant: "foreign-tenant", Project: "foreign-project", Namespace: "foreign-namespace"}, State: memory.MemoryStateActive, FinalRank: 2, semanticScore: 0.876543},
				{Alias: "hidden-alias", MemoryID: "hidden-memory-id", Scope: scope, State: memory.MemoryStateForgotten, FinalRank: 3, relationScore: 0.765432},
			},
		}},
	})
	if err != nil {
		t.Fatalf("CalculateEvaluationMetrics() error = %v", err)
	}
	if len(report.SafetyFailures) == 0 || report.Metrics.RecallAt1 != 0 || report.Metrics.MRR != 0 {
		t.Fatalf("report = %+v, want unsafe candidates to suppress quality metrics", report)
	}
	encoded, err := MarshalEvaluationReport(report)
	if err != nil {
		t.Fatalf("MarshalEvaluationReport() error = %v", err)
	}
	for _, prohibited := range []string{"visible-memory", "foreign-memory-id", "hidden-memory-id", "foreign-tenant", "foreign-project", "foreign-namespace", "0.987654", "0.876543", "0.765432"} {
		if strings.Contains(string(encoded), prohibited) {
			t.Fatalf("report contains prohibited raw detail %q: %s", prohibited, encoded)
		}
	}
}

func TestOrdinarySearchResultDoesNotSerializeEvaluationDiagnostics(t *testing.T) {
	encoded, err := json.Marshal(SearchResult{Hits: []SearchHit{{
		Memory: memory.CanonicalMemory{ID: "memory-1"},
	}}, fusionChannelAvailability: map[FusionChannel]fusionChannelAvailability{FusionChannelRelation: fusionChannelUnavailable}})
	if err != nil {
		t.Fatalf("marshal ordinary SearchResult: %v", err)
	}
	if strings.Contains(string(encoded), "evaluation") || strings.Contains(string(encoded), "candidate_channels") || strings.Contains(string(encoded), "fusionChannelAvailability") || strings.Contains(string(encoded), "unavailable") {
		t.Fatalf("ordinary SearchResult exposes evaluation diagnostics: %s", encoded)
	}
}

type stubEvaluationSearcher struct {
	input  SearchInput
	result SearchResult
	err    error
}

func (s *stubEvaluationSearcher) Search(_ context.Context, input SearchInput) (SearchResult, error) {
	s.input = input
	return s.result, s.err
}

func TestEvaluationRunnerReplaysFixtureThroughMemorySearcher(t *testing.T) {
	fixture := EvaluationFixture{
		Version: "retrieval-fixture-v1",
		Cases: []EvaluationCase{{
			ID:                     "database",
			Category:               "single-fact",
			Scope:                  memory.Scope{Tenant: "eval", Project: "baseline", Namespace: "case"},
			Query:                  "Which database is the system of record?",
			Sources:                []EvaluationSource{{Alias: "database", EventType: "fixture", Content: "controlled"}},
			ExpectedEvidenceGroups: [][]string{{"database"}},
		}},
	}
	seed := EvaluationFixtureSeed{
		FixtureVersion: fixture.Version,
		Aliases: []EvaluationSeededAlias{{
			CaseID:   "database",
			Alias:    "database",
			Scope:    fixture.Cases[0].Scope,
			MemoryID: "memory-1",
			State:    memory.MemoryStateActive,
		}},
	}
	searcher := &stubEvaluationSearcher{result: SearchResult{Hits: []SearchHit{{
		Memory: memory.CanonicalMemory{ID: "memory-1", Scope: fixture.Cases[0].Scope, State: memory.MemoryStateActive},
		Score:  ScoreBreakdown{Lexical: 0.9, Semantic: 0.8, Relation: 0.7, Overall: 0.9},
		Chunk:  &memory.MemoryChunk{ID: "chunk-1"},
	}}}}

	run, err := NewEvaluationRunner(searcher).Replay(context.Background(), fixture, seed, EvaluationRankingMetadata{
		FixtureVersion:              fixture.Version,
		RepresentationVersion:       "canonical-v1",
		RankingVersion:              "baseline-v1",
		CompatibleEmbeddingRevision: "deterministic-v1",
		PolicyVersion:               "quality-policy-v1",
	})
	if err != nil {
		t.Fatalf("Replay() error = %v", err)
	}
	if searcher.input.Scope != fixture.Cases[0].Scope || searcher.input.Query != fixture.Cases[0].Query {
		t.Fatalf("Search() input = %+v, want fixture scope and query", searcher.input)
	}
	if !searcher.input.queryAnalysisPolicyDisabled {
		t.Fatalf("original-query baseline did not disable query-analysis policy: %+v", searcher.input)
	}
	if run.Metadata.FusionStrategy != "rrf:rrf-v1" {
		t.Fatalf("fusion strategy = %q, want default RRF identity", run.Metadata.FusionStrategy)
	}
	if !searcher.input.IncludeRelations || !searcher.input.IncludeSummaries || searcher.input.TopK != evaluationReplayTopK {
		t.Fatalf("Search() input = %+v, want full internal retrieval surface", searcher.input)
	}
	if len(run.Cases) != 1 || len(run.Cases[0].Candidates) != 1 {
		t.Fatalf("replay cases = %+v, want one candidate", run.Cases)
	}
	candidate := run.Cases[0].Candidates[0]
	if candidate.Alias != "database" || candidate.FinalRank != 1 || !candidate.Lexical {
		t.Fatalf("candidate = %+v, want mapped lexical result", candidate)
	}
	if run.Cases[0].CandidatePoolSize != 1 || run.Cases[0].Latency < 0 {
		t.Fatalf("replay case = %+v, want bounded candidate pool and latency", run.Cases[0])
	}
	if len(run.Cases[0].Diagnostics) != 1 {
		t.Fatalf("diagnostics = %+v, want one visible candidate diagnostic", run.Cases[0].Diagnostics)
	}
	diagnostic := run.Cases[0].Diagnostics[0]
	if diagnostic.Alias != "database" || diagnostic.LexicalRank != 1 || diagnostic.SemanticRank != 1 || diagnostic.RelationRank != 1 || diagnostic.ChunkRank != 1 || diagnostic.FinalRank != 1 || diagnostic.Disposition != EvaluationCandidateDispositionReturned {
		t.Fatalf("diagnostic = %+v, want returned lexical candidate diagnostic", diagnostic)
	}
	if diagnostic.ChannelStatus["lexical"] != EvaluationChannelStatusAvailable || diagnostic.ChannelStatus["semantic"] != EvaluationChannelStatusAvailable || diagnostic.ChannelStatus["relation"] != EvaluationChannelStatusAvailable || diagnostic.ChannelStatus["chunk"] != EvaluationChannelStatusAvailable {
		t.Fatalf("channel status = %+v, want all contributing channels available", diagnostic.ChannelStatus)
	}
}

func TestEvaluationRunnerReportsAggregateFusionChannelAvailability(t *testing.T) {
	fixture := EvaluationFixture{
		Version: "retrieval-fixture-v1",
		Cases: []EvaluationCase{{
			ID:                     "optional-relation",
			Scope:                  memory.Scope{Tenant: "eval", Project: "baseline", Namespace: "availability"},
			Query:                  "controlled query",
			Sources:                []EvaluationSource{{Alias: "fact", EventType: "fixture", Content: "controlled"}},
			ExpectedEvidenceGroups: [][]string{{"fact"}},
		}},
	}
	searcher := &stubEvaluationSearcher{result: SearchResult{
		Hits: []SearchHit{{Memory: memory.CanonicalMemory{ID: "memory-1", Scope: fixture.Cases[0].Scope, State: memory.MemoryStateActive}}},
		fusionChannelAvailability: map[FusionChannel]fusionChannelAvailability{
			FusionChannelLexical:  fusionChannelAvailable,
			FusionChannelRelation: fusionChannelUnavailable,
		},
	}}

	run, err := NewEvaluationRunner(searcher).Replay(context.Background(), fixture, EvaluationFixtureSeed{
		FixtureVersion: fixture.Version,
		Aliases: []EvaluationSeededAlias{{
			CaseID: "optional-relation", Alias: "fact", MemoryID: "memory-1", Scope: fixture.Cases[0].Scope, State: memory.MemoryStateActive,
		}},
	}, EvaluationRankingMetadata{
		FixtureVersion: fixture.Version, RepresentationVersion: "canonical-v1", RankingVersion: "baseline-v1", CompatibleEmbeddingRevision: "deterministic-v1", PolicyVersion: "quality-policy-v1",
	})
	if err != nil {
		t.Fatalf("Replay() error = %v", err)
	}
	availability := run.Cases[0].ChannelAvailability
	if availability["lexical"] != EvaluationChannelStatusAvailable || availability["relation"] != EvaluationChannelStatusUnavailable {
		t.Fatalf("channel availability = %+v, want lexical available and relation unavailable", availability)
	}
}

func TestEvaluationRunnerRecordsNotReturnedDispositionForActiveFixtureAlias(t *testing.T) {
	fixture := EvaluationFixture{
		Version: "retrieval-fixture-v1",
		Cases: []EvaluationCase{{
			ID:                     "not-returned",
			Scope:                  memory.Scope{Tenant: "eval", Project: "baseline", Namespace: "disposition"},
			Query:                  "controlled query",
			Sources:                []EvaluationSource{{Alias: "missing", EventType: "fixture", Content: "controlled"}},
			ExpectedEvidenceGroups: [][]string{{"missing"}},
		}},
	}
	run, err := NewEvaluationRunner(&stubEvaluationSearcher{}).Replay(context.Background(), fixture, EvaluationFixtureSeed{
		FixtureVersion: fixture.Version,
		Aliases: []EvaluationSeededAlias{{
			CaseID: "not-returned", Alias: "missing", MemoryID: "memory-1", Scope: fixture.Cases[0].Scope, State: memory.MemoryStateActive,
		}},
	}, EvaluationRankingMetadata{
		FixtureVersion: fixture.Version, RepresentationVersion: "canonical-v1", RankingVersion: "baseline-v1", CompatibleEmbeddingRevision: "deterministic-v1", PolicyVersion: "quality-policy-v1",
	})
	if err != nil {
		t.Fatalf("Replay() error = %v", err)
	}
	if len(run.Cases[0].Diagnostics) != 1 || run.Cases[0].Diagnostics[0].Alias != "missing" || run.Cases[0].Diagnostics[0].Disposition != EvaluationCandidateDispositionNotReturned {
		t.Fatalf("diagnostics = %+v, want active missing alias marked not returned", run.Cases[0].Diagnostics)
	}
}

func TestEvaluationRunnerRecordsRedactedLowCardinalityTelemetry(t *testing.T) {
	fixture := EvaluationFixture{
		Version: "retrieval-fixture-v1",
		Cases: []EvaluationCase{{
			ID: "telemetry", Scope: memory.Scope{Tenant: "eval", Project: "baseline", Namespace: "telemetry"}, Query: "query",
			Sources: []EvaluationSource{{Alias: "fact", EventType: "fixture", Content: "controlled"}}, ExpectedEvidenceGroups: [][]string{{"fact"}},
		}},
	}
	searcher := &stubEvaluationSearcher{result: SearchResult{}}
	observer := &evaluationTelemetryStub{}
	_, err := NewEvaluationRunner(searcher, observer).Replay(context.Background(), fixture, EvaluationFixtureSeed{FixtureVersion: fixture.Version, Aliases: []EvaluationSeededAlias{{CaseID: "telemetry", Alias: "fact", MemoryID: "memory-1", Scope: fixture.Cases[0].Scope, State: memory.MemoryStateActive}}}, EvaluationRankingMetadata{FixtureVersion: fixture.Version, RepresentationVersion: "canonical-v1", RankingVersion: "baseline-v1", CompatibleEmbeddingRevision: "deterministic-v1", PolicyVersion: "quality-policy-v1"})
	if err != nil {
		t.Fatalf("Replay() error = %v", err)
	}
	if observer.event.Status != "completed" || observer.event.FixtureVersion != fixture.Version || observer.event.CaseCount != 1 {
		t.Fatalf("telemetry event = %+v, want bounded completion fields", observer.event)
	}
	if observer.event.Tenant != "" || observer.event.Query != "" || observer.event.MemoryID != "" || observer.event.Error != "" {
		t.Fatalf("telemetry event contains prohibited fields: %+v", observer.event)
	}
}

func TestRecordEvaluationReleaseDecisionUsesBoundedTelemetry(t *testing.T) {
	observer := &evaluationTelemetryStub{}
	RecordEvaluationReleaseDecision(context.Background(), observer, EvaluationReleaseDecision{PolicyVersion: "quality-policy-v1", Eligible: false, HardFailures: []string{"protected_recall_regression"}})
	if observer.event.Decision != "rejected" || observer.event.PolicyVersion != "quality-policy-v1" || observer.event.FailureCategory != "protected_recall_regression" {
		t.Fatalf("telemetry event = %+v, want bounded release decision", observer.event)
	}
}

func TestEvaluationReplayCarriesBoundedAnalysisEvidenceAndStableReport(t *testing.T) {
	scope := memory.Scope{Tenant: "eval", Project: "baseline", Namespace: "analysis"}
	limits := DefaultQueryAnalysisLimits()
	input := QueryAnalysisInput{AcceptedQuery: "temporal multi-hop", PolicyVersion: QueryAnalysisPolicyVersionV1, Limits: limits}
	result, err := NewQueryAnalysisResult(input, QueryAnalysisDispositionComplete, []QueryAnalysisHint{{Kind: QueryAnalysisHintTemporal, Disposition: QueryAnalysisHintPresent, Value: "current"}}, []QueryAnalysisSignal{{Kind: QueryAnalysisSignalSubquery, Text: "hidden"}})
	if err != nil {
		t.Fatal(err)
	}
	diagnostics, err := QueryAnalysisDiagnosticsFromResult(result, limits, QueryAnalysisFallbackNone, 12*time.Millisecond, 4, "active")
	if err != nil {
		t.Fatal(err)
	}
	run := EvaluationReplay{Metadata: EvaluationRankingMetadata{FixtureVersion: "fixture-v1", RepresentationVersion: "canonical-v1", RankingVersion: "ranking-v1", CompatibleEmbeddingRevision: "embedding-v1", PolicyVersion: "policy-v1", AnalysisVersion: string(QueryAnalysisPolicyVersionV1), AnalysisLimitsVersion: string(QueryAnalysisLimitsVersionV1), RolloutDisposition: "active_for_scope"}, Cases: []EvaluationReplayCase{{CaseID: "temporal", Category: "temporal", Scope: scope, AnalysisDiagnostics: &diagnostics, ExpectedEvidenceGroups: [][]string{{"fact"}}, CandidatePoolSize: 4, Latency: 12 * time.Millisecond}}}
	report, err := CalculateEvaluationMetrics(run)
	if err != nil {
		t.Fatal(err)
	}
	if report.Metadata.AnalysisVersion != string(QueryAnalysisPolicyVersionV1) || report.Metadata.AnalysisLimitsVersion != string(QueryAnalysisLimitsVersionV1) || report.Metadata.RolloutDisposition != "active_for_scope" {
		t.Fatalf("metadata = %+v", report.Metadata)
	}
	if report.Cases[0].AnalysisSignalCount != 2 || report.Cases[0].AnalysisSubqueryCount != 1 || report.Cases[0].AnalysisCandidateCount != 4 || !report.Cases[0].AnalysisOriginalRetained {
		t.Fatalf("analysis case = %+v", report.Cases[0])
	}
	if report.Metrics.TemporalEvidenceCoverage != 0 || report.Metrics.AnalysisSignalCount != 2 || report.Metrics.AnalysisSubqueryCount != 1 || report.Metrics.AnalysisCandidateCount != 4 {
		t.Fatalf("analysis metrics = %+v", report.Metrics)
	}
	one, err := MarshalEvaluationReport(report)
	if err != nil {
		t.Fatal(err)
	}
	repeated, err := CalculateEvaluationMetrics(run)
	if err != nil {
		t.Fatal(err)
	}
	two, err := MarshalEvaluationReport(repeated)
	if err != nil {
		t.Fatal(err)
	}
	if string(one) != string(two) {
		t.Fatalf("compatible report serialization is not stable:\n%s\n%s", one, two)
	}
	rendered, err := RenderEvaluationReport(report)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"analysis_version=query-analysis-v1", "analysis_limits_version=query-analysis-limits-v1", "rollout_disposition=active_for_scope", "temporal_evidence_coverage=0.0000", "analysis_signal_count=2", "analysis_subquery_count=1", "analysis_candidate_count=4"} {
		if !strings.Contains(rendered, want) {
			t.Fatalf("rendered report %q does not contain %q", rendered, want)
		}
	}
	for _, forbidden := range []string{"temporal multi-hop", "hidden", "postgres://", "dsn", "password"} {
		if strings.Contains(string(one), forbidden) {
			t.Fatalf("report leaked %q: %s", forbidden, one)
		}
	}
}

func TestMarshalEvaluationReportRejectsUnsafeCaseIdentity(t *testing.T) {
	report := EvaluationReport{Metadata: EvaluationRankingMetadata{FixtureVersion: "fixture-v1", RepresentationVersion: "canonical-v1", RankingVersion: "ranking-v1", CompatibleEmbeddingRevision: "embedding-v1", PolicyVersion: "policy-v1"}, Cases: []EvaluationCaseReport{{CaseID: "postgres://operator:secret@db/internal", Category: "single-fact"}}}
	_, err := MarshalEvaluationReport(report)
	if err == nil {
		t.Fatal("MarshalEvaluationReport() error = nil, want unsafe case identity rejection")
	}
}

func TestMarshalEvaluationReportRejectsUnsafeSafetyCategory(t *testing.T) {
	report := EvaluationReport{Metadata: EvaluationRankingMetadata{FixtureVersion: "fixture-v1", RepresentationVersion: "canonical-v1", RankingVersion: "ranking-v1", CompatibleEmbeddingRevision: "embedding-v1", PolicyVersion: "policy-v1"}, SafetyFailures: []EvaluationSafetyFailure{{Category: "postgres://operator:secret@db/internal", Count: 1}}}
	_, err := MarshalEvaluationReport(report)
	if err == nil {
		t.Fatal("MarshalEvaluationReport() error = nil, want unsafe safety category rejection")
	}
	if strings.Contains(err.Error(), "secret") || strings.Contains(err.Error(), "postgres://") {
		t.Fatalf("error leaked unsafe category: %v", err)
	}
}

func TestEvaluationRunnerCopiesOnlyAuthorizedBoundedAnalysisDiagnostics(t *testing.T) {
	scope := memory.Scope{Tenant: "eval", Project: "baseline", Namespace: "runner-analysis"}
	fixture := EvaluationFixture{Version: "fixture-v1", Cases: []EvaluationCase{{ID: "analysis-case", Category: "multi-hop", Scope: scope, Query: "private query", Sources: []EvaluationSource{{Alias: "fact", EventType: "fixture", Content: "private evidence"}}, ExpectedEvidenceGroups: [][]string{{"fact"}}, ExpectedAnalysis: &EvaluationAnalysisExpectation{PolicyVersion: QueryAnalysisPolicyVersionV1, LimitsVersion: QueryAnalysisLimitsVersionV1, Disposition: QueryAnalysisDispositionComplete, Fallback: QueryAnalysisFallbackNone, OriginalRetained: true, MaxSignalCount: 8, MaxSubqueryCount: 4, MaxCandidateCount: 200}}}}
	searcher := &stubEvaluationSearcher{result: SearchResult{Diagnostics: []ContextDiagnostic{{Section: "query_analysis", PolicyVersion: QueryAnalysisPolicyVersionV1, LimitsVersion: QueryAnalysisLimitsVersionV1, OriginalRetained: true, SignalCount: 3, SubqueryCount: 2, CandidateCount: 5, ElapsedNS: int64(10 * time.Millisecond), Fallback: QueryAnalysisFallbackNone, Disposition: QueryAnalysisDispositionComplete, RolloutStage: "active", TimeStatus: "present"}, {Section: "other", Reason: "postgres://operator:secret@db/private"}}}}
	run, err := NewEvaluationRunner(searcher).Replay(context.Background(), fixture, EvaluationFixtureSeed{FixtureVersion: fixture.Version, Aliases: []EvaluationSeededAlias{{CaseID: "analysis-case", Alias: "fact", Scope: scope, MemoryID: "memory-1", State: memory.MemoryStateActive}}}, EvaluationRankingMetadata{FixtureVersion: fixture.Version, RepresentationVersion: "canonical-v1", RankingVersion: "ranking-v1", CompatibleEmbeddingRevision: "embedding-v1", PolicyVersion: "policy-v1", AnalysisVersion: string(QueryAnalysisPolicyVersionV1), AnalysisLimitsVersion: string(QueryAnalysisLimitsVersionV1), RolloutDisposition: "active_for_scope"})
	if err != nil {
		t.Fatal(err)
	}
	if searcher.input.rankingPolicyDisabled || searcher.input.queryAnalysisPolicyDisabled || !searcher.input.IncludeFeedbackDiagnostics || !searcher.input.queryAnalysisDiagnosticsAuthorized {
		t.Fatalf("evaluation search input did not authorize active bounded analysis: %+v", searcher.input)
	}
	if run.Cases[0].AnalysisDiagnostics == nil || run.Cases[0].AnalysisDiagnostics.SignalCount != 3 || run.Cases[0].AnalysisDiagnostics.CandidateCount != 5 {
		t.Fatalf("analysis diagnostics = %+v", run.Cases[0].AnalysisDiagnostics)
	}
}

func TestEvaluationRunnerAllowsVersionedOriginalOnlyBaselineWithoutAnalysisDiagnostics(t *testing.T) {
	scope := memory.Scope{Tenant: "eval", Project: "baseline", Namespace: "original-only"}
	fixture := EvaluationFixture{Version: "fixture-v1", Cases: []EvaluationCase{{
		ID: "original-only", Category: "single-fact", Scope: scope, Query: "private query",
		Sources: []EvaluationSource{{Alias: "fact", EventType: "fixture", Content: "private evidence"}}, ExpectedEvidenceGroups: [][]string{{"fact"}},
		ExpectedAnalysis: &EvaluationAnalysisExpectation{PolicyVersion: QueryAnalysisPolicyVersionV1, LimitsVersion: QueryAnalysisLimitsVersionV1, Disposition: QueryAnalysisDispositionComplete, Fallback: QueryAnalysisFallbackNone, OriginalRetained: true, MaxSignalCount: 8, MaxSubqueryCount: 4, MaxCandidateCount: 200},
	}}}
	searcher := &stubEvaluationSearcher{}
	run, err := NewEvaluationRunner(searcher).Replay(context.Background(), fixture, EvaluationFixtureSeed{FixtureVersion: fixture.Version, Aliases: []EvaluationSeededAlias{{CaseID: "original-only", Alias: "fact", Scope: scope, MemoryID: "memory-1", State: memory.MemoryStateActive}}}, EvaluationRankingMetadata{
		FixtureVersion: fixture.Version, RepresentationVersion: "canonical-v1", RankingVersion: "ranking-v1", FusionStrategy: "rrf:rrf-v1", CompatibleEmbeddingRevision: "embedding-v1", PolicyVersion: "policy-v1",
		AnalysisVersion: string(QueryAnalysisPolicyVersionV1), AnalysisLimitsVersion: string(QueryAnalysisLimitsVersionV1), RolloutDisposition: "original_only",
	})
	if err != nil {
		t.Fatalf("Replay() original-only baseline error = %v", err)
	}
	if !searcher.input.queryAnalysisPolicyDisabled || run.Cases[0].AnalysisDiagnostics != nil {
		t.Fatalf("original-only baseline input=%+v diagnostics=%+v", searcher.input, run.Cases[0].AnalysisDiagnostics)
	}
	if _, err := CalculateEvaluationMetrics(run); err != nil {
		t.Fatalf("CalculateEvaluationMetrics() original-only baseline error = %v", err)
	}
}

func TestCalculateEvaluationMetricsRejectsUnboundedAnalysisCounts(t *testing.T) {
	scope := memory.Scope{Tenant: "eval", Project: "baseline", Namespace: "bounds"}
	diagnostics := QueryAnalysisDiagnostics{PolicyVersion: QueryAnalysisPolicyVersionV1, LimitsVersion: QueryAnalysisLimitsVersionV1, Disposition: QueryAnalysisDispositionComplete, Fallback: QueryAnalysisFallbackNone, OriginalRetained: true, SignalCount: QueryAnalysisHardMaxSignals + 1}
	_, err := CalculateEvaluationMetrics(EvaluationReplay{Metadata: EvaluationRankingMetadata{FixtureVersion: "fixture-v1", RepresentationVersion: "canonical-v1", RankingVersion: "ranking-v1", CompatibleEmbeddingRevision: "embedding-v1", PolicyVersion: "policy-v1", AnalysisVersion: string(QueryAnalysisPolicyVersionV1), AnalysisLimitsVersion: string(QueryAnalysisLimitsVersionV1), RolloutDisposition: "active_for_scope"}, Cases: []EvaluationReplayCase{{CaseID: "bounds", Category: "single-fact", Scope: scope, AnalysisDiagnostics: &diagnostics, ExpectedEvidenceGroups: [][]string{{"fact"}}}}})
	if err == nil {
		t.Fatal("CalculateEvaluationMetrics() error = nil, want unbounded analysis rejection")
	}
}

func TestCalculateEvaluationMetricsSeparatesProtectedTemporalAndMultiHopCoverage(t *testing.T) {
	scope := memory.Scope{Tenant: "eval", Project: "baseline", Namespace: "coverage"}
	caseWith := func(id, category string, expected [][]string, candidates []EvaluationReplayCandidate) EvaluationReplayCase {
		return EvaluationReplayCase{CaseID: id, Category: category, Scope: scope, ExpectedEvidenceGroups: expected, Candidates: candidates}
	}
	report, err := CalculateEvaluationMetrics(EvaluationReplay{Metadata: EvaluationRankingMetadata{FixtureVersion: "fixture-v1", RepresentationVersion: "canonical-v1", RankingVersion: "ranking-v1", CompatibleEmbeddingRevision: "embedding-v1", PolicyVersion: "policy-v1"}, Cases: []EvaluationReplayCase{
		caseWith("protected", "single-fact", [][]string{{"fact"}}, []EvaluationReplayCandidate{{Alias: "fact", Scope: scope, State: memory.MemoryStateActive, FinalRank: 1}}),
		caseWith("temporal", "temporal", [][]string{{"now"}}, nil),
		caseWith("multi-hop", "multi-hop", [][]string{{"left"}, {"right"}}, []EvaluationReplayCandidate{{Alias: "left", Scope: scope, State: memory.MemoryStateActive, FinalRank: 1}}),
	}})
	if err != nil {
		t.Fatal(err)
	}
	if report.Metrics.ProtectedRecall != 1 || report.Metrics.TemporalEvidenceCoverage != 0 || report.Metrics.MultiHopEvidenceCoverage != 0.5 {
		t.Fatalf("coverage metrics = %+v", report.Metrics)
	}
}

func TestEvaluationRunnerRecordsOriginalOnlyFallbackUnderActiveRollout(t *testing.T) {
	scope := memory.Scope{Tenant: "eval", Project: "baseline", Namespace: "fallback"}
	fixture := EvaluationFixture{Version: "fixture-v1", Cases: []EvaluationCase{{ID: "unavailable", Category: "analyzer-unavailable", Scope: scope, Query: "private", Sources: []EvaluationSource{{Alias: "fact", EventType: "fixture", Content: "private"}}, ExpectedEvidenceGroups: [][]string{{"fact"}}, ExpectedAnalysis: &EvaluationAnalysisExpectation{PolicyVersion: QueryAnalysisPolicyVersionV1, LimitsVersion: QueryAnalysisLimitsVersionV1, Disposition: QueryAnalysisDispositionOriginalOnly, Fallback: QueryAnalysisFallbackUnavailable, OriginalRetained: true, Categories: []QueryAnalysisDiagnosticCategory{QueryAnalysisDiagnosticUnavailable}, MaxSignalCount: 1, MaxCandidateCount: 200}}}}
	searcher := &stubEvaluationSearcher{result: SearchResult{Diagnostics: []ContextDiagnostic{{Section: "query_analysis", PolicyVersion: QueryAnalysisPolicyVersionV1, LimitsVersion: QueryAnalysisLimitsVersionV1, OriginalRetained: true, Fallback: QueryAnalysisFallbackUnavailable, Disposition: QueryAnalysisDispositionOriginalOnly, Categories: []QueryAnalysisDiagnosticCount{{Category: QueryAnalysisDiagnosticUnavailable, Count: 1}}, RolloutStage: "original_only"}}}}
	run, err := NewEvaluationRunner(searcher).Replay(context.Background(), fixture, EvaluationFixtureSeed{FixtureVersion: fixture.Version, Aliases: []EvaluationSeededAlias{{CaseID: "unavailable", Alias: "fact", Scope: scope, MemoryID: "memory-1", State: memory.MemoryStateActive}}}, EvaluationRankingMetadata{FixtureVersion: fixture.Version, RepresentationVersion: "canonical-v1", RankingVersion: "ranking-v1", CompatibleEmbeddingRevision: "embedding-v1", PolicyVersion: "policy-v1", AnalysisVersion: string(QueryAnalysisPolicyVersionV1), AnalysisLimitsVersion: string(QueryAnalysisLimitsVersionV1), RolloutDisposition: "active_for_scope"})
	if err != nil {
		t.Fatal(err)
	}
	if run.Cases[0].AnalysisDiagnostics == nil || run.Cases[0].AnalysisDiagnostics.SignalCount != 1 || run.Cases[0].AnalysisDiagnostics.Fallback != QueryAnalysisFallbackUnavailable {
		t.Fatalf("fallback diagnostics = %+v", run.Cases[0].AnalysisDiagnostics)
	}
}

func TestEvaluationRunnerRejectsAnalysisOutsideFixtureExpectation(t *testing.T) {
	scope := memory.Scope{Tenant: "eval", Project: "baseline", Namespace: "expectation"}
	expectation := &EvaluationAnalysisExpectation{PolicyVersion: QueryAnalysisPolicyVersionV1, LimitsVersion: QueryAnalysisLimitsVersionV1, Disposition: QueryAnalysisDispositionOriginalOnly, Fallback: QueryAnalysisFallbackUnavailable, OriginalRetained: true, Categories: []QueryAnalysisDiagnosticCategory{QueryAnalysisDiagnosticUnavailable}, MaxSignalCount: 1, MaxCandidateCount: 4}
	fixture := EvaluationFixture{Version: "fixture-v1", Cases: []EvaluationCase{{ID: "expectation", Category: "analyzer-unavailable", Scope: scope, Query: "private", Sources: []EvaluationSource{{Alias: "fact", EventType: "fixture", Content: "private"}}, ExpectedEvidenceGroups: [][]string{{"fact"}}, ExpectedAnalysis: expectation}}}
	searcher := &stubEvaluationSearcher{result: SearchResult{Diagnostics: []ContextDiagnostic{{Section: "query_analysis", PolicyVersion: QueryAnalysisPolicyVersionV1, LimitsVersion: QueryAnalysisLimitsVersionV1, OriginalRetained: true, SignalCount: 1, CandidateCount: 5, Fallback: QueryAnalysisFallbackMalformed, Disposition: QueryAnalysisDispositionOriginalOnly, Categories: []QueryAnalysisDiagnosticCount{{Category: QueryAnalysisDiagnosticMalformed, Count: 1}}, RolloutStage: "original_only"}}}}
	_, err := NewEvaluationRunner(searcher).Replay(context.Background(), fixture, EvaluationFixtureSeed{FixtureVersion: fixture.Version, Aliases: []EvaluationSeededAlias{{CaseID: "expectation", Alias: "fact", Scope: scope, MemoryID: "memory-1", State: memory.MemoryStateActive}}}, EvaluationRankingMetadata{FixtureVersion: fixture.Version, RepresentationVersion: "canonical-v1", RankingVersion: "ranking-v1", CompatibleEmbeddingRevision: "embedding-v1", PolicyVersion: "policy-v1", AnalysisVersion: string(QueryAnalysisPolicyVersionV1), AnalysisLimitsVersion: string(QueryAnalysisLimitsVersionV1), RolloutDisposition: "active_for_scope"})
	if err == nil {
		t.Fatal("Replay() error = nil, want fixture analysis expectation rejection")
	}
}
