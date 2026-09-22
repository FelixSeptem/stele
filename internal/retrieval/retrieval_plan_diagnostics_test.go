package retrieval

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/FelixSeptem/stele/internal/memory"
	"github.com/FelixSeptem/stele/internal/telemetry"
)

type plannerTelemetryObserver struct {
	events             []telemetry.RetrievalPlannerEvent
	channels           []telemetry.RetrievalPlannerChannelEvent
	changedRanks       []telemetry.RetrievalPlannerChangedRankEvent
	diagnosticFailures []telemetry.RetrievalPlannerDiagnosticEvent
}

func (o *plannerTelemetryObserver) RecordRetrievalPlannerChannel(_ context.Context, event telemetry.RetrievalPlannerChannelEvent) {
	o.channels = append(o.channels, event)
}

func (o *plannerTelemetryObserver) RecordRetrievalPlannerChangedRank(_ context.Context, event telemetry.RetrievalPlannerChangedRankEvent) {
	o.changedRanks = append(o.changedRanks, event)
}

func (o *plannerTelemetryObserver) RecordRetrievalPlannerDiagnostic(_ context.Context, event telemetry.RetrievalPlannerDiagnosticEvent) {
	o.diagnosticFailures = append(o.diagnosticFailures, event)
}

func (*plannerTelemetryObserver) RecordOperation(context.Context, telemetry.OperationEvent) {}
func (*plannerTelemetryObserver) RecordBacklog(context.Context, telemetry.BacklogEvent)     {}
func (o *plannerTelemetryObserver) RecordRetrievalPlanner(_ context.Context, event telemetry.RetrievalPlannerEvent) {
	o.events = append(o.events, event)
}

func TestRetrievalPlannerDiagnosticsAreBoundedAndRedacted(t *testing.T) {
	plan := diagnosticTestPlan(t, "private query tenant-a memory-id-1")
	diagnostics, err := RetrievalPlannerDiagnosticsFromExecution(RetrievalPlannerDiagnosticsInput{
		Plan:                plan,
		RolloutStage:        memory.RetrievalPlannerRolloutStageActive,
		Evidence:            EvidenceAssessment{Disposition: EvidenceDispositionBelowMinimum, FollowUpEligible: true, VisibleBucket: "one_to_five", AttritionBucket: "high"},
		PassCount:           2,
		CandidateCount:      37,
		Elapsed:             750 * time.Millisecond,
		ChannelAvailability: []RetrievalPlannerChannelAvailability{{Channel: FusionChannelChunk, Availability: "unavailable"}, {Channel: FusionChannelLexical, Availability: "available"}, {Channel: FusionChannelRelation, Availability: "unavailable"}, {Channel: FusionChannelSemantic, Availability: "unavailable"}},
		ChangedRankCount:    3,
		ChangedRankObserved: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	payload, err := MarshalRetrievalPlannerDiagnostics(diagnostics)
	if err != nil {
		t.Fatal(err)
	}
	text := string(payload)
	for _, want := range []string{
		`"planner_version":"retrieval-planner-v1"`, `"policy_version":"retrieval-plan-policy-v1"`,
		`"query_family":"semantic"`, `"rollout_stage":"active_for_scope"`,
		`"candidate_bucket":"11_50"`, `"latency_bucket":"500ms_1s"`,
		`"evidence_disposition":"below_minimum"`, `"reranker_eligibility":"ineligible"`,
		`"channel_availability":[{"channel":"chunk","availability":"unavailable"},{"channel":"lexical","availability":"available"},{"channel":"relation","availability":"unavailable"},{"channel":"semantic","availability":"unavailable"}]`,
		`"changed_rank_count":3`, `"changed_rank_bucket":"1_5"`,
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("diagnostics missing %s: %s", want, text)
		}
	}
	for _, forbidden := range []string{
		"private query", "tenant-a", "memory-id-1", "query_text", "scope_values", "candidate_id",
		"raw_score", "provider_payload", "credential", "reasoning", "channel_candidates",
	} {
		if strings.Contains(text, forbidden) {
			t.Fatalf("planner diagnostics leaked %q: %s", forbidden, text)
		}
	}
}

func TestRetrievalPlannerDiagnosticsAllowOnlyBoundedGraphAggregates(t *testing.T) {
	diagnostics := validPlannerDiagnosticsForTest(t)
	diagnostics.GraphTraversal = &GraphTraversalDiagnostics{
		PolicyVersion: "graph-policy-v1", HopBucket: "one", PathBucket: "1_10",
		Truncation: "cycle", Failure: "none",
	}
	payload, err := MarshalRetrievalPlannerDiagnostics(diagnostics)
	if err != nil {
		t.Fatalf("MarshalRetrievalPlannerDiagnostics() error = %v", err)
	}
	for _, forbidden := range []string{"tenant", "memory_id", "edge-", "private-query", "raw_score", "content"} {
		if strings.Contains(string(payload), forbidden) {
			t.Fatalf("graph diagnostics leaked %q: %s", forbidden, payload)
		}
	}
	diagnostics.GraphTraversal.Failure = "database error with private query"
	if _, err := MarshalRetrievalPlannerDiagnostics(diagnostics); err == nil {
		t.Fatal("expected unbounded graph diagnostic category rejection")
	}
}

func TestRetrievalPlannerDiagnosticsCarryOnlyBoundedTemporalOmissions(t *testing.T) {
	plan := diagnosticTestPlan(t, "private query tenant-a memory-id-1")
	var omissions TemporalOmissionReport
	omissions.Add(TemporalOmissionUnsupportedClass)
	omissions.Add(TemporalOmissionExpiredVersion)
	omissions.Add(TemporalOmissionExpiredVersion)

	diagnostics, err := RetrievalPlannerDiagnosticsFromExecution(RetrievalPlannerDiagnosticsInput{
		Plan:              plan,
		RolloutStage:      memory.RetrievalPlannerRolloutStageDiagnosticsOnly,
		Evidence:          EvidenceAssessment{Disposition: EvidenceDispositionSufficient, VisibleBucket: "one_to_five", AttritionBucket: "none"},
		PassCount:         1,
		CandidateCount:    3,
		Elapsed:           time.Millisecond,
		TemporalOmissions: omissions,
	})
	if err != nil {
		t.Fatal(err)
	}
	payload, err := MarshalRetrievalPlannerDiagnostics(diagnostics)
	if err != nil {
		t.Fatal(err)
	}
	text := string(payload)
	if !strings.Contains(text, `"temporal_omissions":[{"category":"expired_version","count":2},{"category":"unsupported_class","count":1}]`) {
		t.Fatalf("authorized diagnostics = %s, want sorted bounded temporal categories", text)
	}
	for _, forbidden := range []string{"private query", "tenant-a", "memory-id-1", "query_text", "scope_values", "candidate_id", "raw_score", "postgres://", "provider_payload"} {
		if strings.Contains(text, forbidden) {
			t.Fatalf("authorized temporal diagnostics leaked %q: %s", forbidden, text)
		}
	}
}

func TestRetrievalPlannerDiagnosticsRejectInvalidAggregates(t *testing.T) {
	diagnostics := RetrievalPlannerDiagnostics{
		PlannerVersion:      RetrievalPlannerVersionV1,
		PolicyVersion:       RetrievalPlanPolicyVersionV1,
		QueryFamily:         RetrievalQueryFamilySemantic,
		RolloutStage:        memory.RetrievalPlannerRolloutStageActive,
		Disposition:         RetrievalPlanDispositionPlanned,
		Fallback:            RetrievalPlanFallbackBaseline,
		EnabledChannels:     []FusionChannel{FusionChannelLexical},
		PassCount:           3,
		CandidateBucket:     "11_50",
		EvidenceDisposition: EvidenceDispositionSufficient,
		VisibleBucket:       "one_to_five",
		AttritionBucket:     "none",
		LatencyBucket:       "lt_100ms",
		RerankerEligibility: "ineligible",
		ChannelAvailability: []RetrievalPlannerChannelAvailability{{Channel: FusionChannelLexical, Availability: "available"}},
		ChangedRankBucket:   "not_evaluated",
	}
	if _, err := MarshalRetrievalPlannerDiagnostics(diagnostics); err == nil {
		t.Fatal("expected invalid pass count to be rejected")
	}
}

func TestRetrievalPlannerDiagnosticsRejectSensitiveOrUnboundedComparisonCategories(t *testing.T) {
	diagnostics := validPlannerDiagnosticsForTest(t)
	diagnostics.ChannelAvailability[0].Availability = "tenant-secret"
	if _, err := MarshalRetrievalPlannerDiagnostics(diagnostics); err == nil {
		t.Fatal("expected sensitive availability category to be rejected")
	}
	diagnostics = validPlannerDiagnosticsForTest(t)
	diagnostics.ChangedRankBucket = "memory-id-1"
	if _, err := MarshalRetrievalPlannerDiagnostics(diagnostics); err == nil {
		t.Fatal("expected unbounded changed-rank category to be rejected")
	}
}

func TestSearchPlannerDiagnosticsRequireExplicitAuthorization(t *testing.T) {
	scope := memory.Scope{Tenant: "tenant-secret", Project: "project-secret", Namespace: "namespace-secret"}
	rollout := plannerTestRollout(scope, memory.RankingRolloutSurfaceSearch, memory.RankingRolloutPolicyStatusActiveForScope)
	newService := func() *Service {
		lexical := &plannerChannelRecorder{hits: [][]ScoredMemory{{plannerTestHit(scope, "memory-secret", memory.MemoryClassEpisodic)}}}
		return NewService(ServiceDependencies{Lexical: lexical, RankingRolloutPolicyReader: plannerPolicyReader{policy: rollout}, RetrievalPlanPolicy: lexicalOnlyPlannerPolicy(4)})
	}

	ordinary, err := newService().Search(context.Background(), SearchInput{Scope: scope, Query: "private query", TopK: 10, IncludeFeedbackDiagnostics: true})
	if err != nil {
		t.Fatal(err)
	}
	if len(ordinary.plannerDiagnostics) != 0 {
		t.Fatalf("ordinary search exposed planner diagnostics: %+v", ordinary.plannerDiagnostics)
	}

	authorized, err := newService().Search(context.Background(), SearchInput{Scope: scope, Query: "private query", TopK: 10, IncludeFeedbackDiagnostics: true, retrievalPlannerDiagnosticsAuthorized: true})
	if err != nil {
		t.Fatal(err)
	}
	if len(authorized.plannerDiagnostics) != 1 || authorized.plannerDiagnostics[0].QueryFamily != RetrievalQueryFamilyGeneral {
		t.Fatalf("authorized planner diagnostics = %+v", authorized.plannerDiagnostics)
	}
}

func TestSearchRecordsBoundedRetrievalPlannerTelemetry(t *testing.T) {
	scope := memory.Scope{Tenant: "tenant-secret", Project: "project-secret", Namespace: "namespace-secret"}
	observer := &plannerTelemetryObserver{}
	lexical := &plannerChannelRecorder{hits: [][]ScoredMemory{{plannerTestHit(scope, "memory-secret", memory.MemoryClassEpisodic)}}}
	service := NewService(ServiceDependencies{Lexical: lexical, RankingRolloutPolicyReader: plannerPolicyReader{policy: plannerTestRollout(scope, memory.RankingRolloutSurfaceSearch, memory.RankingRolloutPolicyStatusActiveForScope)}, RetrievalPlanPolicy: lexicalOnlyPlannerPolicy(4)}, observer)
	if _, err := service.Search(context.Background(), SearchInput{Scope: scope, Query: "private query", TopK: 10}); err != nil {
		t.Fatal(err)
	}
	if len(observer.events) != 1 {
		t.Fatalf("planner events=%d, want 1", len(observer.events))
	}
	got := observer.events[0]
	if got.PlannerVersion != string(RetrievalPlannerVersionV1) || got.PolicyVersion != string(RetrievalPlanPolicyVersionV1) || got.Family != string(RetrievalQueryFamilyGeneral) || got.Stage != string(memory.RetrievalPlannerRolloutStageActive) || got.Pass != 1 {
		t.Fatalf("planner event=%+v", got)
	}
}

func TestPlannerDiagnosticsClampElapsedAndCategorizeValidationFailure(t *testing.T) {
	started := time.Date(2026, 9, 17, 0, 0, 0, 0, time.UTC)
	if got := plannerDiagnosticElapsed(started, started.Add(time.Minute)); got != 30*time.Second {
		t.Fatalf("clamped elapsed=%s, want 30s", got)
	}
	observer := &plannerTelemetryObserver{}
	service := NewService(ServiceDependencies{}, observer)
	input := RetrievalPlannerDiagnosticsInput{
		Plan: diagnosticTestPlan(t, "q"), RolloutStage: memory.RetrievalPlannerRolloutStageActive,
		Evidence:  EvidenceAssessment{Disposition: EvidenceDispositionSufficient, VisibleBucket: "one_to_five", AttritionBucket: "none"},
		PassCount: 1, CandidateCount: 5001, Elapsed: time.Second,
	}
	if diagnostics := service.retrievalPlannerDiagnostics(context.Background(), input); len(diagnostics) != 0 {
		t.Fatalf("invalid diagnostics became public: %+v", diagnostics)
	}
	if len(observer.diagnosticFailures) != 1 || observer.diagnosticFailures[0].FailureCategory != retrievalPlannerDiagnosticFailureInvalidAggregate {
		t.Fatalf("diagnostic failures=%+v", observer.diagnosticFailures)
	}
	for _, forbidden := range []string{"q", "5001", "tenant", "memory", "score", "provider"} {
		if strings.Contains(observer.diagnosticFailures[0].FailureCategory, forbidden) {
			t.Fatalf("private diagnostic failure leaked %q: %+v", forbidden, observer.diagnosticFailures[0])
		}
	}
}

func diagnosticTestPlan(t *testing.T, query string) RetrievalPlan {
	t.Helper()
	analysis := originalQueryAnalysis(SearchInput{Scope: memory.Scope{Tenant: "t", Project: "p", Namespace: "n"}, Query: query, QueryEmbedding: []float32{1}})
	plan, err := BuildRetrievalPlan(RetrievalPlanInput{AcceptedQuery: query, Analysis: analysis, EmbeddingAvailable: true, Policy: DefaultRetrievalPlanPolicy(), Now: time.Now().UTC()})
	if err != nil {
		t.Fatal(err)
	}
	return plan
}

func validPlannerDiagnosticsForTest(t *testing.T) RetrievalPlannerDiagnostics {
	t.Helper()
	diagnostics, err := RetrievalPlannerDiagnosticsFromExecution(RetrievalPlannerDiagnosticsInput{Plan: diagnosticTestPlan(t, "q"), RolloutStage: memory.RetrievalPlannerRolloutStageDiagnosticsOnly, Evidence: EvidenceAssessment{Disposition: EvidenceDispositionSufficient, VisibleBucket: "one_to_five", AttritionBucket: "none"}, PassCount: 1, CandidateCount: 1, ChannelAvailability: []RetrievalPlannerChannelAvailability{{Channel: FusionChannelLexical, Availability: "not_evaluated"}, {Channel: FusionChannelSemantic, Availability: "not_evaluated"}, {Channel: FusionChannelRelation, Availability: "not_evaluated"}, {Channel: FusionChannelChunk, Availability: "not_evaluated"}}})
	if err != nil {
		t.Fatal(err)
	}
	return diagnostics
}
