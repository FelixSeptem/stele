package postgres

import (
	"context"
	"database/sql"
	"testing"
	"time"

	"github.com/FelixSeptem/stele/internal/memory"
	pgxmock "github.com/pashagolub/pgxmock/v4"
)

func rankingRolloutPolicyColumns() []string {
	return []string{"id", "tenant", "project", "namespace", "status", "mode", "surfaces", "signal_sources", "threshold_status", "evidence_minimum", "actor", "reason", "latest_dry_run_id", "latest_dry_run_status", "fusion_strategy", "fusion_version", "fusion_rank_constant", "fusion_channel_weights", "fusion_per_channel_candidate", "fusion_total_candidates", "diversity_policy_name", "diversity_policy_version", "diversity_mmr_lambda", "diversity_semantic_threshold", "diversity_max_candidates", "diversity_max_pairwise_comparisons", "diversity_max_embedding_dimensions", "diversity_max_citations_per_candidate", "diversity_coverage_weights", "query_analysis_session_id", "query_analysis_user_id", "query_analysis_policy", "activated_at", "disabled_at", "rolled_back_at", "created_at", "updated_at"}
}

func rankingRolloutPolicyRow(id string, scope memory.Scope, status memory.RankingRolloutPolicyStatus, mode memory.RankingRolloutMode, threshold memory.RankingRolloutThresholdStatus, evidence int, actor, reason string, latestID, latestStatus any, activated, disabled, rolledBack, created, updated any) []any {
	return []any{id, scope.Tenant, scope.Project, scope.Namespace, status, mode, []string{"search"}, []string{"task_evaluations"}, threshold, evidence, actor, reason, latestID, latestStatus,
		nil, nil, nil, nil, nil, nil, // fusion
		nil, nil, nil, nil, nil, nil, nil, nil, nil, // diversity
		nil, nil, nil, // query analysis
		activated, disabled, rolledBack, created, updated}
}

func TestRepositoryReadsExactQueryAnalysisRolloutSelectorsAndRoundTripsPayload(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()
	scope := memory.Scope{Tenant: "tenant-a", Project: "project-a", Namespace: "namespace-a"}
	now := time.Date(2026, 9, 7, 12, 0, 0, 0, time.UTC)
	payload := []byte(`{"schema_version":"query-analysis-rollout-v1","policy_version":"query-analysis-v1","limits_version":"query-analysis-limits-v1","max_query_bytes":4096,"max_hints":4,"max_signals":8,"max_subqueries":4,"max_term_bytes":256,"max_subquery_bytes":1024,"max_analysis_work":7,"max_candidates_per_signal":50,"max_aggregate_candidates":200,"max_elapsed_ns":250000000,"expires_at":"2026-09-08T12:00:00Z"}`)
	row := []any{"shared-name", scope.Tenant, scope.Project, scope.Namespace, memory.RankingRolloutPolicyStatusDryRun, memory.RankingRolloutModeDryRun, []string{"search"}, "session-a", "user-a", payload, now, nil, nil, now, now}
	mock.ExpectQuery(`SELECT[\s\S]*FROM ranking_rollout_policies[\s\S]*tenant = \$1[\s\S]*project = \$2[\s\S]*namespace = \$3[\s\S]*query_analysis_session_id IS NOT DISTINCT FROM \$5[\s\S]*query_analysis_user_id IS NOT DISTINCT FROM \$6`).
		WithArgs(scope.Tenant, scope.Project, scope.Namespace, string(memory.RankingRolloutSurfaceSearch), "session-a", "user-a").
		WillReturnRows(pgxmock.NewRows([]string{"id", "tenant", "project", "namespace", "status", "mode", "surfaces", "query_analysis_session_id", "query_analysis_user_id", "query_analysis_policy", "activated_at", "disabled_at", "rolled_back_at", "created_at", "updated_at"}).AddRow(row...))
	repo := NewRepository(mock)
	policy, err := repo.ReadEffectiveQueryAnalysisRolloutPolicy(context.Background(), memory.ReadEffectiveQueryAnalysisRolloutPolicyInput{Scope: scope, Surface: memory.RankingRolloutSurfaceSearch, SessionID: "session-a", UserID: "user-a"})
	if err != nil {
		t.Fatalf("ReadEffectiveQueryAnalysisRolloutPolicy() error = %v", err)
	}
	if policy.ID != "shared-name" || policy.QueryAnalysis == nil || policy.QueryAnalysis.MaxSignals != 8 || policy.QueryAnalysisSelector.SessionID != "session-a" || policy.QueryAnalysisSelector.UserID != "user-a" {
		t.Fatalf("policy = %+v, want exact-selector typed payload", policy)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestRepositoryReadsExactRetrievalPlannerRolloutSelectorsAndPayload(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()
	scope := memory.Scope{Tenant: "tenant-a", Project: "project-a", Namespace: "namespace-a"}
	now := time.Date(2026, 9, 17, 12, 0, 0, 0, time.UTC)
	payload := []byte(`{"schema_version":"retrieval-planner-rollout-v1","planner_version":"retrieval-planner-v1","policy_version":"retrieval-plan-policy-v1","analysis_policy_version":"query-analysis-v1","fusion_version":"rrf-v1","ranking_version":"quality-feature-v1","renderer_version":"context-renderer-v1","max_candidates":200,"max_candidates_per_channel":100,"max_passes":2,"max_latency_ns":5000000000,"max_context_items":100,"max_reranker_headroom":100,"expires_at":"2026-09-18T12:00:00Z"}`)
	mock.ExpectQuery(`SELECT[\s\S]*FROM ranking_rollout_policies[\s\S]*tenant = \$1[\s\S]*project = \$2[\s\S]*namespace = \$3[\s\S]*retrieval_planner_session_id IS NOT DISTINCT FROM \$5[\s\S]*retrieval_planner_user_id IS NOT DISTINCT FROM \$6`).
		WithArgs(scope.Tenant, scope.Project, scope.Namespace, string(memory.RankingRolloutSurfaceSearch), "session-a", "user-a").
		WillReturnRows(pgxmock.NewRows([]string{"id", "tenant", "project", "namespace", "status", "mode", "surfaces", "retrieval_planner_session_id", "retrieval_planner_user_id", "retrieval_planner_policy", "activated_at", "disabled_at", "rolled_back_at", "created_at", "updated_at"}).AddRow("planner", scope.Tenant, scope.Project, scope.Namespace, memory.RankingRolloutPolicyStatusDryRun, memory.RankingRolloutModeDryRun, []string{"search"}, "session-a", "user-a", payload, nil, nil, nil, now, now))

	policy, err := NewRepository(mock).ReadEffectiveRetrievalPlannerRolloutPolicy(context.Background(), memory.ReadEffectiveRetrievalPlannerRolloutPolicyInput{Scope: scope, Surface: memory.RankingRolloutSurfaceSearch, SessionID: "session-a", UserID: "user-a"})
	if err != nil {
		t.Fatalf("ReadEffectiveRetrievalPlannerRolloutPolicy() error = %v", err)
	}
	if policy.RetrievalPlanner == nil || policy.RetrievalPlanner.MaxPasses != 2 || policy.RetrievalPlannerSelector.SessionID != "session-a" {
		t.Fatalf("policy = %+v", policy)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestRepositoryRetrievalPlannerPayloadUnknownFieldFailsClosed(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()
	scope := memory.Scope{Tenant: "tenant-a", Project: "project-a", Namespace: "namespace-a"}
	now := time.Now()
	mock.ExpectQuery("SELECT[\\s\\S]*FROM ranking_rollout_policies").WithArgs(scope.Tenant, scope.Project, scope.Namespace, string(memory.RankingRolloutSurfaceSearch), nil, nil).
		WillReturnRows(pgxmock.NewRows([]string{"id", "tenant", "project", "namespace", "status", "mode", "surfaces", "retrieval_planner_session_id", "retrieval_planner_user_id", "retrieval_planner_policy", "activated_at", "disabled_at", "rolled_back_at", "created_at", "updated_at"}).AddRow("planner", scope.Tenant, scope.Project, scope.Namespace, memory.RankingRolloutPolicyStatusDryRun, memory.RankingRolloutModeDryRun, []string{"search"}, nil, nil, []byte(`{"schema_version":"retrieval-planner-rollout-v1","unknown":true}`), nil, nil, nil, now, now))
	if _, err := NewRepository(mock).ReadEffectiveRetrievalPlannerRolloutPolicy(context.Background(), memory.ReadEffectiveRetrievalPlannerRolloutPolicyInput{Scope: scope, Surface: memory.RankingRolloutSurfaceSearch}); err == nil {
		t.Fatal("error = nil, want malformed payload rejection")
	}
}

func TestRepositoryQueryAnalysisPayloadUnknownFieldsAndVersionsFailClosed(t *testing.T) {
	for _, payload := range [][]byte{
		[]byte(`{"schema_version":"query-analysis-rollout-v1","unknown":true}`),
		[]byte(`{"schema_version":"query-analysis-rollout-v2"}`),
	} {
		t.Run(string(payload), func(t *testing.T) {
			mock, err := pgxmock.NewPool()
			if err != nil {
				t.Fatal(err)
			}
			defer mock.Close()
			scope := memory.Scope{Tenant: "tenant-a", Project: "project-a", Namespace: "namespace-a"}
			row := []any{"policy", scope.Tenant, scope.Project, scope.Namespace, memory.RankingRolloutPolicyStatusDryRun, memory.RankingRolloutModeDryRun, []string{"search"}, nil, nil, payload, nil, nil, nil, time.Now(), time.Now()}
			mock.ExpectQuery("SELECT[\\s\\S]*FROM ranking_rollout_policies").WithArgs(scope.Tenant, scope.Project, scope.Namespace, memory.RankingRolloutSurfaceSearch, nil, nil).WillReturnRows(pgxmock.NewRows([]string{"id", "tenant", "project", "namespace", "status", "mode", "surfaces", "query_analysis_session_id", "query_analysis_user_id", "query_analysis_policy", "activated_at", "disabled_at", "rolled_back_at", "created_at", "updated_at"}).AddRow(row...))
			_, err = NewRepository(mock).ReadEffectiveQueryAnalysisRolloutPolicy(context.Background(), memory.ReadEffectiveQueryAnalysisRolloutPolicyInput{Scope: scope, Surface: memory.RankingRolloutSurfaceSearch})
			if err == nil {
				t.Fatal("error = nil, want malformed payload fail closed")
			}
		})
	}
}

func TestRepositoryCreateQueryAnalysisPolicyPreservesRankingBundles(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()
	scope := memory.Scope{Tenant: "tenant-a", Project: "project-a", Namespace: "namespace-a"}
	now := time.Date(2026, 9, 7, 12, 0, 0, 0, time.UTC)
	policy := memory.RankingRolloutPolicy{
		ID: "qa-policy", Scope: scope, Status: memory.RankingRolloutPolicyStatusDryRun, Mode: memory.RankingRolloutModeDryRun,
		Surfaces: []memory.RankingRolloutSurface{memory.RankingRolloutSurfaceSearch}, SignalSources: []memory.RankingRolloutSignalSource{memory.RankingRolloutSignalSourceTaskEvaluations},
		ThresholdStatus: memory.RankingRolloutThresholdStatusSatisfied, Actor: "operator", Reason: "combined rollout", CreatedAt: now, UpdatedAt: now,
		FusionStrategy: "rrf", FusionVersion: "rrf-v1", FusionRankConstant: 60, FusionChannelWeights: map[string]float64{"lexical": 1, "semantic": 1}, FusionPerChannelCandidate: 20, FusionTotalCandidates: 40,
		DiversityPolicyName: "mmr", DiversityPolicyVersion: "mmr-v1", DiversityMMRLambda: .7, DiversitySemanticThreshold: .8, DiversityMaxCandidates: 40, DiversityMaxPairwiseComparisons: 1000, DiversityMaxEmbeddingDimensions: 1536, DiversityMaxCitationsPerCandidate: 8, DiversityCoverageWeights: map[string]float64{"memory_class": 1, "session": 1, "entity": 1, "time_slice": 1, "unknown": 1},
		QueryAnalysisSelector: memory.QueryAnalysisRolloutSelector{SessionID: "session-a", UserID: "user-a"}, QueryAnalysis: &memory.QueryAnalysisRolloutPolicy{SchemaVersion: memory.QueryAnalysisRolloutSchemaVersionV1, PolicyVersion: memory.QueryAnalysisPolicyVersionV1, LimitsVersion: memory.QueryAnalysisLimitsVersionV1, MaxQueryBytes: 4096, MaxHints: 4, MaxSignals: 8, MaxSubqueries: 4, MaxTermBytes: 256, MaxSubqueryBytes: 1024, MaxAnalysisWork: 7, MaxCandidatesPerSignal: 50, MaxAggregateCandidates: 200, MaxElapsed: 250 * time.Millisecond, ExpiresAt: now.Add(time.Hour)},
	}
	row := rankingRolloutPolicyRow(policy.ID, scope, policy.Status, policy.Mode, policy.ThresholdStatus, 0, policy.Actor, policy.Reason, nil, nil, nil, nil, nil, now, now)
	row[14], row[15], row[16], row[17], row[18], row[19] = policy.FusionStrategy, policy.FusionVersion, int64(60), []byte(`{"lexical":1,"semantic":1}`), int64(20), int64(40)
	row[20], row[21], row[22], row[23], row[24], row[25], row[26], row[27], row[28] = policy.DiversityPolicyName, policy.DiversityPolicyVersion, .7, .8, int64(40), int64(1000), int64(1536), int64(8), []byte(`{"entity":1,"memory_class":1,"session":1,"time_slice":1,"unknown":1}`)
	mock.ExpectBegin()
	mock.ExpectQuery(`INSERT INTO ranking_rollout_policies[\s\S]*latest_dry_run_id[\s\S]*fusion_strategy[\s\S]*diversity_policy_name[\s\S]*query_analysis_session_id`).WithArgs(anyRankingRolloutArgs(37)...).WillReturnRows(pgxmock.NewRows(rankingRolloutPolicyColumns()).AddRow(row...))
	mock.ExpectExec("INSERT INTO ranking_rollout_policy_states").WithArgs(anyRankingRolloutArgs(11)...).WillReturnResult(pgxmock.NewResult("INSERT", 1))
	mock.ExpectCommit()
	created, err := NewRepository(mock).CreateRankingRolloutPolicy(context.Background(), policy)
	if err != nil {
		t.Fatalf("CreateRankingRolloutPolicy() error = %v", err)
	}
	if created.FusionVersion != policy.FusionVersion || created.DiversityPolicyVersion != policy.DiversityPolicyVersion || created.QueryAnalysis == nil {
		t.Fatalf("created policy lost bundle: %+v", created)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestRepositoryCreateRetrievalPlannerPolicyPersistsBundleInSameTransaction(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()
	scope := memory.Scope{Tenant: "tenant-a", Project: "project-a", Namespace: "namespace-a"}
	now := time.Date(2026, 9, 17, 12, 0, 0, 0, time.UTC)
	policy := memory.RankingRolloutPolicy{
		ID: "planner-policy", Scope: scope, Status: memory.RankingRolloutPolicyStatusDryRun, Mode: memory.RankingRolloutModeDryRun,
		Surfaces: []memory.RankingRolloutSurface{memory.RankingRolloutSurfaceSearch}, SignalSources: []memory.RankingRolloutSignalSource{memory.RankingRolloutSignalSourceTaskEvaluations},
		ThresholdStatus: memory.RankingRolloutThresholdStatusSatisfied, Actor: "operator", Reason: "planner rollout", CreatedAt: now, UpdatedAt: now,
		RetrievalPlannerSelector: memory.RetrievalPlannerRolloutSelector{SessionID: "session-a", UserID: "user-a"},
		RetrievalPlanner:         &memory.RetrievalPlannerRolloutPolicy{SchemaVersion: memory.RetrievalPlannerRolloutSchemaVersionV1, PlannerVersion: memory.RetrievalPlannerVersionV1, PolicyVersion: memory.RetrievalPlanPolicyVersionV1, AnalysisPolicyVersion: memory.QueryAnalysisPolicyVersionV1, FusionVersion: "rrf-v1", RankingVersion: "quality-feature-v1", RendererVersion: "context-renderer-v1", MaxCandidates: 200, MaxCandidatesPerChannel: 100, MaxPasses: 2, MaxLatency: 5 * time.Second, MaxContextItems: 100, MaxRerankerHeadroom: 100, ExpiresAt: now.Add(time.Hour)},
	}
	row := rankingRolloutPolicyRow(policy.ID, scope, policy.Status, policy.Mode, policy.ThresholdStatus, 0, policy.Actor, policy.Reason, nil, nil, nil, nil, nil, now, now)
	mock.ExpectBegin()
	mock.ExpectQuery(`INSERT INTO ranking_rollout_policies`).WithArgs(anyRankingRolloutArgs(37)...).WillReturnRows(pgxmock.NewRows(rankingRolloutPolicyColumns()).AddRow(row...))
	mock.ExpectExec(`UPDATE ranking_rollout_policies[\s\S]*retrieval_planner_session_id`).WithArgs("session-a", "user-a", pgxmock.AnyArg(), policy.ID, scope.Tenant, scope.Project, scope.Namespace).WillReturnResult(pgxmock.NewResult("UPDATE", 1))
	mock.ExpectExec("INSERT INTO ranking_rollout_policy_states").WithArgs(anyRankingRolloutArgs(11)...).WillReturnResult(pgxmock.NewResult("INSERT", 1))
	mock.ExpectCommit()
	created, err := NewRepository(mock).CreateRankingRolloutPolicy(context.Background(), policy)
	if err != nil {
		t.Fatalf("CreateRankingRolloutPolicy() error = %v", err)
	}
	if created.RetrievalPlanner == nil || created.RetrievalPlanner.PolicyVersion != memory.RetrievalPlanPolicyVersionV1 || created.RetrievalPlannerSelector.SessionID != "session-a" {
		t.Fatalf("created policy = %+v", created)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestRepositoryPersistsAndLoadsContextCalibrationRolloutByExactScope(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()
	scope := memory.Scope{Tenant: "tenant-a", Project: "project-a", Namespace: "namespace-a"}
	now := time.Date(2026, 9, 23, 12, 0, 0, 0, time.UTC)
	calibration := &memory.ContextCalibrationRolloutPolicy{
		SchemaVersion: memory.ContextCalibrationRolloutSchemaVersionV1, PolicyVersion: memory.ContextCalibrationPolicyVersionV1,
		SummaryVersion: "summary-v1", MinimumEvidence: 2, ConfidenceThreshold: .5, DecayWindow: time.Hour,
		ContributionCap: .25, MaxCandidates: 10, MaxContextItems: 10, MaxElapsed: time.Millisecond, ExpiresAt: now.Add(time.Hour),
	}
	policy := memory.RankingRolloutPolicy{
		ID: "context-policy", Scope: scope, Status: memory.RankingRolloutPolicyStatusActiveForScope, Mode: memory.RankingRolloutModeActiveForScope,
		Surfaces: []memory.RankingRolloutSurface{memory.RankingRolloutSurfaceContext}, SignalSources: []memory.RankingRolloutSignalSource{memory.RankingRolloutSignalSourceUsefulnessFeedback},
		ThresholdStatus: memory.RankingRolloutThresholdStatusSatisfied, Actor: "operator", Reason: "context calibration", CreatedAt: now, UpdatedAt: now,
		ContextCalibrationSelector: memory.RetrievalPlannerRolloutSelector{SessionID: "session-a", UserID: "user-a"}, ContextCalibration: calibration,
	}
	row := rankingRolloutPolicyRow(policy.ID, scope, policy.Status, policy.Mode, policy.ThresholdStatus, 0, policy.Actor, policy.Reason, nil, nil, nil, nil, nil, now, now)
	row[6] = []string{"context"}
	mock.ExpectBegin()
	mock.ExpectQuery("INSERT INTO ranking_rollout_policies").WithArgs(anyRankingRolloutArgs(37)...).WillReturnRows(pgxmock.NewRows(rankingRolloutPolicyColumns()).AddRow(row...))
	mock.ExpectExec("INSERT INTO context_calibration_rollout_policies").WithArgs("context-policy", scope.Tenant, scope.Project, scope.Namespace, "session-a", "user-a", pgxmock.AnyArg(), now, now).WillReturnResult(pgxmock.NewResult("INSERT", 1))
	mock.ExpectExec("INSERT INTO ranking_rollout_policy_states").WithArgs(anyRankingRolloutArgs(11)...).WillReturnResult(pgxmock.NewResult("INSERT", 1))
	mock.ExpectCommit()
	created, err := NewRepository(mock).CreateRankingRolloutPolicy(context.Background(), policy)
	if err != nil {
		t.Fatalf("CreateRankingRolloutPolicy() error = %v", err)
	}
	if created.ContextCalibration == nil || created.ContextCalibration.SummaryVersion != "summary-v1" {
		t.Fatalf("created calibration = %+v", created.ContextCalibration)
	}

	mock.ExpectQuery("SELECT[\\s\\S]*FROM ranking_rollout_policies").WithArgs(scope.Tenant, scope.Project, scope.Namespace, memory.RankingRolloutPolicyStatusActiveForScope, string(memory.RankingRolloutSurfaceContext)).WillReturnRows(pgxmock.NewRows(rankingRolloutPolicyColumns()).AddRow(row...))
	mock.ExpectQuery("SELECT session_id, user_id, payload[\\s\\S]*FROM context_calibration_rollout_policies").WithArgs(policy.ID, scope.Tenant, scope.Project, scope.Namespace).WillReturnRows(pgxmock.NewRows([]string{"session_id", "user_id", "payload"}).AddRow("session-a", "user-a", []byte(`{"schema_version":"context-calibration-rollout-v1","policy_version":"context-calibration-v1","summary_version":"summary-v1","minimum_evidence":2,"confidence_threshold":0.5,"decay_window_ns":3600000000000,"contribution_cap":0.25,"max_candidates":10,"max_context_items":10,"max_elapsed_ns":1000000,"expires_at":"2026-09-23T13:00:00Z"}`)))
	loaded, err := NewRepository(mock).ReadActiveRankingRolloutPolicy(context.Background(), memory.ReadActiveRankingRolloutPolicyInput{Scope: scope, Surface: memory.RankingRolloutSurfaceContext})
	if err != nil {
		t.Fatalf("ReadActiveRankingRolloutPolicy() error = %v", err)
	}
	if loaded.ContextCalibration == nil || loaded.ContextCalibrationSelector.SessionID != "session-a" {
		t.Fatalf("loaded calibration = %+v selector=%+v", loaded.ContextCalibration, loaded.ContextCalibrationSelector)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestRepositoryQueryAnalysisRolloutDoesNotFallbackToSameNameForeignPolicy(t *testing.T) {
	scope := memory.Scope{Tenant: "tenant-a", Project: "project-a", Namespace: "namespace-a"}
	cases := []struct {
		name     string
		request  memory.ReadEffectiveQueryAnalysisRolloutPolicyInput
		wantArgs []any
	}{
		{
			name:     "foreign canonical scope",
			request:  memory.ReadEffectiveQueryAnalysisRolloutPolicyInput{Scope: scope, Surface: memory.RankingRolloutSurfaceSearch, SessionID: "session-a", UserID: "user-a"},
			wantArgs: []any{"tenant-a", "project-a", "namespace-a", memory.RankingRolloutSurfaceSearch, "session-a", "user-a"},
		},
		{
			name:     "foreign optional selector",
			request:  memory.ReadEffectiveQueryAnalysisRolloutPolicyInput{Scope: scope, Surface: memory.RankingRolloutSurfaceSearch, SessionID: "session-a", UserID: "user-a"},
			wantArgs: []any{"tenant-a", "project-a", "namespace-a", memory.RankingRolloutSurfaceSearch, "session-a", "user-a"},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			mock, err := pgxmock.NewPool()
			if err != nil {
				t.Fatal(err)
			}
			defer mock.Close()
			mock.ExpectQuery(`SELECT[\s\S]*FROM ranking_rollout_policies[\s\S]*WHERE tenant = \$1 AND project = \$2 AND namespace = \$3[\s\S]*query_analysis_policy IS NOT NULL[\s\S]*query_analysis_session_id IS NOT DISTINCT FROM \$5[\s\S]*query_analysis_user_id IS NOT DISTINCT FROM \$6`).
				WithArgs(tc.wantArgs[0], tc.wantArgs[1], tc.wantArgs[2], string(memory.RankingRolloutSurfaceSearch), tc.wantArgs[4], tc.wantArgs[5]).
				WillReturnError(sql.ErrNoRows)
			_, err = NewRepository(mock).ReadEffectiveQueryAnalysisRolloutPolicy(context.Background(), tc.request)
			if err == nil {
				t.Fatal("ReadEffectiveQueryAnalysisRolloutPolicy() error = nil, want no broader-scope fallback")
			}
			if err := mock.ExpectationsWereMet(); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestRepositoryCreateActivateRollbackRankingRolloutPolicy(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatalf("pgxmock.NewPool() error = %v", err)
	}
	defer mock.Close()

	scope := memory.Scope{Tenant: "tenant-a", Project: "project-a", Namespace: "namespace-a"}
	now := time.Date(2026, 7, 12, 9, 30, 0, 0, time.UTC)
	policy := memory.RankingRolloutPolicy{
		ID:              "policy_1",
		Scope:           scope,
		Status:          memory.RankingRolloutPolicyStatusDraft,
		Mode:            memory.RankingRolloutModeDryRun,
		Surfaces:        []memory.RankingRolloutSurface{memory.RankingRolloutSurfaceSearch},
		SignalSources:   []memory.RankingRolloutSignalSource{memory.RankingRolloutSignalSourceTaskEvaluations},
		ThresholdStatus: memory.RankingRolloutThresholdStatusInsufficient,
		EvidenceMinimum: 2,
		Actor:           "operator-a",
		Reason:          "initial rollout",
		CreatedAt:       now,
		UpdatedAt:       now,
	}

	mock.ExpectBegin()
	mock.ExpectQuery("INSERT INTO ranking_rollout_policies").
		WithArgs(anyRankingRolloutArgs(37)...).
		WillReturnRows(pgxmock.NewRows(rankingRolloutPolicyColumns()).
			AddRow(rankingRolloutPolicyRow(policy.ID, scope, policy.Status, policy.Mode, policy.ThresholdStatus, policy.EvidenceMinimum, policy.Actor, policy.Reason, nil, nil, nil, nil, nil, now, now)...))
	mock.ExpectExec("INSERT INTO ranking_rollout_policy_states").
		WithArgs(policy.ID, scope.Tenant, scope.Project, scope.Namespace, policy.Status, policy.Actor, policy.Reason, nil, nil, nil, policy.UpdatedAt).
		WillReturnResult(pgxmock.NewResult("INSERT", 1))
	mock.ExpectCommit()

	repo := NewRepository(mock)
	created, err := repo.CreateRankingRolloutPolicy(context.Background(), policy)
	if err != nil {
		t.Fatalf("CreateRankingRolloutPolicy() error = %v", err)
	}
	if created.ID != policy.ID {
		t.Fatalf("created.ID = %q, want %q", created.ID, policy.ID)
	}

	mock.ExpectBegin()
	mock.ExpectQuery("UPDATE ranking_rollout_policies").
		WithArgs(scope.Tenant, scope.Project, scope.Namespace, policy.ID, memory.RankingRolloutPolicyStatusActiveForScope, "operator-b", "activate after dry-run", now.Add(time.Minute), memory.RankingRolloutThresholdStatusSatisfied, memory.RankingRolloutModeActiveForScope, memory.RankingRolloutPolicyStatusDisabled, memory.RankingRolloutPolicyStatusRolledBack).
		WillReturnRows(pgxmock.NewRows(rankingRolloutPolicyColumns()).
			AddRow(rankingRolloutPolicyRow(policy.ID, scope, memory.RankingRolloutPolicyStatusActiveForScope, memory.RankingRolloutModeActiveForScope, memory.RankingRolloutThresholdStatusSatisfied, policy.EvidenceMinimum, "operator-b", "activate after dry-run", "dry_run_1", memory.RankingRolloutThresholdStatusSatisfied, now.Add(time.Minute), nil, nil, now, now.Add(time.Minute))...))
	mock.ExpectExec("INSERT INTO ranking_rollout_policy_states").
		WithArgs(policy.ID, scope.Tenant, scope.Project, scope.Namespace, memory.RankingRolloutPolicyStatusActiveForScope, "operator-b", "activate after dry-run", now.Add(time.Minute), nil, nil, now.Add(time.Minute)).
		WillReturnResult(pgxmock.NewResult("INSERT", 1))
	mock.ExpectCommit()
	activated, err := repo.ActivateRankingRolloutPolicy(context.Background(), memory.ActivateRankingRolloutPolicyInput{
		Scope:       scope,
		PolicyID:    policy.ID,
		Actor:       "operator-b",
		Reason:      "activate after dry-run",
		ActivatedAt: now.Add(time.Minute),
		Gate: memory.RankingRolloutActivationGate{
			DryRunSucceeded:         true,
			EvidenceThresholdStatus: memory.RankingRolloutThresholdStatusSatisfied,
			AttributionRecorded:     true,
		},
	})
	if err != nil {
		t.Fatalf("ActivateRankingRolloutPolicy() error = %v", err)
	}
	if activated.Status != memory.RankingRolloutPolicyStatusActiveForScope {
		t.Fatalf("activated.Status = %q, want active_for_scope", activated.Status)
	}

	mock.ExpectBegin()
	mock.ExpectQuery("SELECT[\\s\\S]*FROM ranking_rollout_policies[\\s\\S]*FOR UPDATE").
		WithArgs(scope.Tenant, scope.Project, scope.Namespace, policy.ID).
		WillReturnRows(pgxmock.NewRows(rankingRolloutPolicyColumns()).
			AddRow(rankingRolloutPolicyRow(policy.ID, scope, memory.RankingRolloutPolicyStatusActiveForScope, policy.Mode, memory.RankingRolloutThresholdStatusSatisfied, policy.EvidenceMinimum, "operator-b", "activate after dry-run", "dry_run_1", memory.RankingRolloutThresholdStatusSatisfied, now.Add(time.Minute), nil, nil, now, now.Add(time.Minute))...))
	mock.ExpectQuery("UPDATE ranking_rollout_policies").
		WithArgs(scope.Tenant, scope.Project, scope.Namespace, policy.ID, memory.RankingRolloutPolicyStatusRolledBack, "operator-c", "rollback degraded ranking", now.Add(2*time.Minute)).
		WillReturnRows(pgxmock.NewRows(rankingRolloutPolicyColumns()).
			AddRow(rankingRolloutPolicyRow(policy.ID, scope, memory.RankingRolloutPolicyStatusRolledBack, policy.Mode, policy.ThresholdStatus, policy.EvidenceMinimum, "operator-c", "rollback degraded ranking", nil, nil, now.Add(time.Minute), nil, now.Add(2*time.Minute), now, now.Add(2*time.Minute))...))
	mock.ExpectExec("INSERT INTO ranking_rollout_policy_states").
		WithArgs(policy.ID, scope.Tenant, scope.Project, scope.Namespace, memory.RankingRolloutPolicyStatusRolledBack, "operator-c", "rollback degraded ranking", now.Add(time.Minute), nil, now.Add(2*time.Minute), now.Add(2*time.Minute)).
		WillReturnResult(pgxmock.NewResult("INSERT", 1))
	mock.ExpectExec("INSERT INTO ranking_rollout_rollback_audit").
		WithArgs(policy.ID, scope.Tenant, scope.Project, scope.Namespace, memory.RankingRolloutPolicyStatusActiveForScope, memory.RankingRolloutPolicyStatusRolledBack, "operator-c", "rollback degraded ranking", now.Add(2*time.Minute)).
		WillReturnResult(pgxmock.NewResult("INSERT", 1))
	mock.ExpectCommit()

	rolledBack, err := repo.RollbackRankingRolloutPolicy(context.Background(), memory.RollbackRankingRolloutPolicyInput{
		Scope:        scope,
		PolicyID:     policy.ID,
		Actor:        "operator-c",
		Reason:       "rollback degraded ranking",
		RolledBackAt: now.Add(2 * time.Minute),
	})
	if err != nil {
		t.Fatalf("RollbackRankingRolloutPolicy() error = %v", err)
	}
	if rolledBack.Status != memory.RankingRolloutPolicyStatusRolledBack {
		t.Fatalf("rolledBack.Status = %q, want rolled_back", rolledBack.Status)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("ExpectationsWereMet() error = %v", err)
	}
}

func TestRepositoryCreateRankingRolloutPolicyIsRetrySafeByID(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatalf("pgxmock.NewPool() error = %v", err)
	}
	defer mock.Close()
	scope := memory.Scope{Tenant: "tenant-a", Project: "project-a", Namespace: "namespace-a"}
	now := time.Date(2026, 7, 12, 9, 30, 0, 0, time.UTC)
	policy := memory.RankingRolloutPolicy{ID: "policy-retry", Scope: scope, Status: memory.RankingRolloutPolicyStatusDraft, Mode: memory.RankingRolloutModeDryRun, Surfaces: []memory.RankingRolloutSurface{memory.RankingRolloutSurfaceSearch}, SignalSources: []memory.RankingRolloutSignalSource{memory.RankingRolloutSignalSourceTaskEvaluations}, ThresholdStatus: memory.RankingRolloutThresholdStatusInsufficient, Actor: "operator", Reason: "retry", CreatedAt: now, UpdatedAt: now}
	for attempt := 0; attempt < 2; attempt++ {
		mock.ExpectBegin()
		mock.ExpectQuery("INSERT INTO ranking_rollout_policies").WithArgs(anyRankingRolloutArgs(37)...).WillReturnRows(pgxmock.NewRows(rankingRolloutPolicyColumns()).AddRow(rankingRolloutPolicyRow(policy.ID, scope, policy.Status, policy.Mode, policy.ThresholdStatus, policy.EvidenceMinimum, policy.Actor, policy.Reason, nil, nil, nil, nil, nil, now, now)...))
		mock.ExpectExec("INSERT INTO ranking_rollout_policy_states").WithArgs(anyRankingRolloutArgs(11)...).WillReturnResult(pgxmock.NewResult("INSERT", 1))
		mock.ExpectCommit()
	}
	repo := NewRepository(mock)
	for attempt := 0; attempt < 2; attempt++ {
		if _, err := repo.CreateRankingRolloutPolicy(context.Background(), policy); err != nil {
			t.Fatalf("retry %d CreateRankingRolloutPolicy() error = %v", attempt, err)
		}
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("ExpectationsWereMet() error = %v", err)
	}
}

func anyRankingRolloutArgs(count int) []any {
	args := make([]any, count)
	for i := range args {
		args[i] = pgxmock.AnyArg()
	}
	return args
}

func TestRepositoryRecordRankingRolloutDryRunPersistsComparisonAndImpact(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatalf("pgxmock.NewPool() error = %v", err)
	}
	defer mock.Close()

	scope := memory.Scope{Tenant: "tenant-a", Project: "project-a", Namespace: "namespace-a"}
	now := time.Date(2026, 7, 12, 10, 0, 0, 0, time.UTC)
	policyID := "policy_1"

	mock.ExpectBegin()
	mock.ExpectQuery("SELECT[\\s\\S]*FROM ranking_rollout_policies[\\s\\S]*FOR UPDATE").
		WithArgs(scope.Tenant, scope.Project, scope.Namespace, policyID).
		WillReturnRows(pgxmock.NewRows(rankingRolloutPolicyColumns()).
			AddRow(rankingRolloutPolicyRow(policyID, scope, memory.RankingRolloutPolicyStatusDryRun, memory.RankingRolloutModeDryRun, memory.RankingRolloutThresholdStatusInsufficient, 2, "operator-a", "test rollout", nil, nil, nil, nil, nil, now, now)...))
	mock.ExpectQuery("INSERT INTO ranking_rollout_dry_runs").
		WithArgs(pgxmock.AnyArg(), policyID, scope.Tenant, scope.Project, scope.Namespace, memory.RankingRolloutSurfaceSearch, memory.RankingRolloutSignalSourceTaskEvaluations, memory.RankingRolloutThresholdStatusSatisfied, 2, 1, []string{"mem_1"}, []string{"subject_boosted"}, []string{"task_evaluations"}, 2, 0, now).
		WillReturnRows(pgxmock.NewRows([]string{"id", "policy_id", "tenant", "project", "namespace", "surface", "signal_source", "threshold_status", "baseline_rank", "adjusted_rank", "changed_subject_ids", "reason_codes", "signal_categories", "evidence_count", "hidden_evidence_count", "created_at"}).
			AddRow("dry_run_1", policyID, scope.Tenant, scope.Project, scope.Namespace, memory.RankingRolloutSurfaceSearch, memory.RankingRolloutSignalSourceTaskEvaluations, memory.RankingRolloutThresholdStatusSatisfied, 2, 1, []string{"mem_1"}, []string{"subject_boosted"}, []string{"task_evaluations"}, 2, 0, now))
	mock.ExpectExec("INSERT INTO ranking_rollout_impact_entries").
		WithArgs(pgxmock.AnyArg(), "dry_run_1", policyID, scope.Tenant, scope.Project, scope.Namespace, memory.RankingRolloutSurfaceSearch, memory.RankingRolloutSignalSourceTaskEvaluations, []string{"task_evaluations"}, "memory", "mem_1", nil, 1, true, 1, 2, 1, memory.RankingRolloutImpactReasonCodeSubjectBoosted, 2, false, now).
		WillReturnResult(pgxmock.NewResult("INSERT", 1))
	mock.ExpectExec("UPDATE ranking_rollout_policies").
		WithArgs(scope.Tenant, scope.Project, scope.Namespace, policyID, "dry_run_1", memory.RankingRolloutThresholdStatusSatisfied, now).
		WillReturnResult(pgxmock.NewResult("UPDATE", 1))
	mock.ExpectCommit()

	repo := NewRepository(mock)
	dryRun, err := repo.RecordRankingRolloutDryRun(context.Background(), memory.RecordRankingRolloutDryRunInput{
		PolicyID:            policyID,
		Scope:               scope,
		Surface:             memory.RankingRolloutSurfaceSearch,
		SignalSource:        memory.RankingRolloutSignalSourceTaskEvaluations,
		ThresholdStatus:     memory.RankingRolloutThresholdStatusSatisfied,
		BaselineRank:        2,
		AdjustedRank:        1,
		ChangedSubjectIDs:   []string{"mem_1"},
		ReasonCodes:         []memory.RankingRolloutImpactReasonCode{memory.RankingRolloutImpactReasonCodeSubjectBoosted},
		SignalCategories:    []string{"task_evaluations"},
		EvidenceCount:       2,
		HiddenEvidenceCount: 0,
		ImpactEntries: []memory.RankingRolloutImpactEntry{{
			SignalCategories:  []string{"task_evaluations"},
			SubjectKind:       "memory",
			SubjectID:         "mem_1",
			CandidatePriority: 1,
			Included:          true,
			BudgetImpact:      1,
			BaselineRank:      2,
			AdjustedRank:      1,
			ReasonCode:        memory.RankingRolloutImpactReasonCodeSubjectBoosted,
			EvidenceCount:     2,
		}},
		CreatedAt: now,
	})
	if err != nil {
		t.Fatalf("RecordRankingRolloutDryRun() error = %v", err)
	}
	if dryRun.ID != "dry_run_1" || dryRun.EvidenceCount != 2 || dryRun.ChangedSubjectIDs[0] != "mem_1" {
		t.Fatalf("dryRun = %+v, want persisted comparison", dryRun)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("ExpectationsWereMet() error = %v", err)
	}
}
