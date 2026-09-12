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
