package postgres

import (
	"context"
	"testing"
	"time"

	"github.com/FelixSeptem/stele/internal/memory"
	pgxmock "github.com/pashagolub/pgxmock/v4"
)

func TestRepositoryUpsertDerivedInsightWritesInsightAndEvidence(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatalf("pgxmock.NewPool() error = %v", err)
	}
	defer mock.Close()

	insight := testDerivedInsight()
	mock.ExpectBegin()
	mock.ExpectQuery("INSERT INTO derived_insights").
		WithArgs(
			insight.ID,
			insight.Scope.Tenant,
			insight.Scope.Project,
			insight.Scope.Namespace,
			insight.Type,
			insight.State,
			insight.Title,
			insight.Summary,
			insight.Confidence.Score,
			insight.Confidence.Method,
			pgxmock.AnyArg(),
			pgxmock.AnyArg(),
			insight.Derivation.Source,
			insight.Derivation.Fingerprint,
			pgxmock.AnyArg(),
			insight.Derivation.EvidenceWindowStart,
			insight.Derivation.EvidenceWindowEnd,
			insight.Derivation.DerivedAt,
			insight.LastObservedAt,
			insight.CreatedAt,
			insight.UpdatedAt,
		).
		WillReturnRows(derivedInsightRows().AddRow(
			insight.ID,
			insight.Scope.Tenant,
			insight.Scope.Project,
			insight.Scope.Namespace,
			insight.Type,
			insight.State,
			insight.Title,
			insight.Summary,
			insight.Confidence.Score,
			insight.Confidence.Method,
			[]byte(`{"normalized_key":"embedding_provider_unavailable"}`),
			nil,
			insight.Derivation.Source,
			insight.Derivation.Fingerprint,
			[]byte(`{"threshold":2}`),
			insight.Derivation.EvidenceWindowStart,
			insight.Derivation.EvidenceWindowEnd,
			insight.Derivation.DerivedAt,
			insight.LastObservedAt,
			insight.CreatedAt,
			insight.UpdatedAt,
		))
	mock.ExpectExec("DELETE FROM derived_insight_evidence").
		WithArgs(insight.ID, insight.Scope.Tenant, insight.Scope.Project, insight.Scope.Namespace).
		WillReturnResult(pgxmock.NewResult("DELETE", 0))
	for _, evidence := range insight.Evidence {
		mock.ExpectExec("INSERT INTO derived_insight_evidence").
			WithArgs(
				insight.ID,
				insight.Scope.Tenant,
				insight.Scope.Project,
				insight.Scope.Namespace,
				evidence.Kind,
				evidence.ID,
				evidence.Relation,
				evidence.ObservedAt,
				pgxmock.AnyArg(),
			).
			WillReturnResult(pgxmock.NewResult("INSERT", 1))
	}
	mock.ExpectCommit()

	repo := NewRepository(mock)
	got, err := repo.UpsertDerivedInsight(context.Background(), insight)
	if err != nil {
		t.Fatalf("UpsertDerivedInsight() error = %v", err)
	}
	if got.ID != insight.ID || got.Scope != insight.Scope || got.Type != insight.Type {
		t.Fatalf("UpsertDerivedInsight() = %+v, want insight id/scope/type preserved", got)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("ExpectationsWereMet() error = %v", err)
	}
}

func TestRepositoryListDerivedInsightsScopesAndFiltersVisibleRecords(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatalf("pgxmock.NewPool() error = %v", err)
	}
	defer mock.Close()

	insight := testDerivedInsight()
	mock.ExpectQuery("SELECT[\\s\\S]*FROM derived_insights").
		WithArgs(insight.Scope.Tenant, insight.Scope.Project, insight.Scope.Namespace, insight.Type, insight.State, 25).
		WillReturnRows(derivedInsightRowsWithEvidenceCount().AddRow(
			insight.ID,
			insight.Scope.Tenant,
			insight.Scope.Project,
			insight.Scope.Namespace,
			insight.Type,
			insight.State,
			insight.Title,
			insight.Summary,
			insight.Confidence.Score,
			insight.Confidence.Method,
			[]byte(`{"normalized_key":"embedding_provider_unavailable"}`),
			nil,
			insight.Derivation.Source,
			insight.Derivation.Fingerprint,
			[]byte(`{"threshold":2}`),
			insight.Derivation.EvidenceWindowStart,
			insight.Derivation.EvidenceWindowEnd,
			insight.Derivation.DerivedAt,
			insight.LastObservedAt,
			insight.CreatedAt,
			insight.UpdatedAt,
			2,
		))
	mock.ExpectQuery("SELECT[\\s\\S]*FROM derived_insight_feedback").
		WithArgs(insight.ID, insight.Scope.Tenant, insight.Scope.Project, insight.Scope.Namespace, 100).
		WillReturnRows(derivedInsightFeedbackRows())

	repo := NewRepository(mock)
	items, err := repo.ListDerivedInsights(context.Background(), memory.ListDerivedInsightsInput{
		Scope: insight.Scope,
		Type:  insight.Type,
		State: insight.State,
		Limit: 25,
	})
	if err != nil {
		t.Fatalf("ListDerivedInsights() error = %v", err)
	}
	if len(items) != 1 || items[0].ID != insight.ID {
		t.Fatalf("ListDerivedInsights() = %+v, want one scoped insight", items)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("ExpectationsWereMet() error = %v", err)
	}
}

func TestRepositoryReadDerivedInsightReturnsEvidenceAndLifecycleForScope(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatalf("pgxmock.NewPool() error = %v", err)
	}
	defer mock.Close()

	insight := testDerivedInsight()
	mock.ExpectQuery("SELECT[\\s\\S]*FROM derived_insights").
		WithArgs(insight.ID, insight.Scope.Tenant, insight.Scope.Project, insight.Scope.Namespace, false).
		WillReturnRows(derivedInsightRows().AddRow(
			insight.ID,
			insight.Scope.Tenant,
			insight.Scope.Project,
			insight.Scope.Namespace,
			insight.Type,
			insight.State,
			insight.Title,
			insight.Summary,
			insight.Confidence.Score,
			insight.Confidence.Method,
			[]byte(`{"normalized_key":"embedding_provider_unavailable"}`),
			nil,
			insight.Derivation.Source,
			insight.Derivation.Fingerprint,
			[]byte(`{"threshold":2}`),
			insight.Derivation.EvidenceWindowStart,
			insight.Derivation.EvidenceWindowEnd,
			insight.Derivation.DerivedAt,
			insight.LastObservedAt,
			insight.CreatedAt,
			insight.UpdatedAt,
		))
	mock.ExpectQuery("SELECT[\\s\\S]*FROM derived_insight_evidence").
		WithArgs(insight.ID, insight.Scope.Tenant, insight.Scope.Project, insight.Scope.Namespace).
		WillReturnRows(derivedInsightEvidenceRows().AddRow(
			memory.DerivedInsightEvidenceKindJobExecution,
			"job_1",
			memory.DerivedInsightEvidenceRelationSupports,
			insight.Evidence[0].ObservedAt,
			[]byte(`{"error":"provider unavailable"}`),
		))
	mock.ExpectQuery("SELECT[\\s\\S]*FROM derived_insight_lifecycle_ledger").
		WithArgs(insight.ID, insight.Scope.Tenant, insight.Scope.Project, insight.Scope.Namespace).
		WillReturnRows(derivedInsightLifecycleRows().AddRow(
			"ledger_1",
			insight.ID,
			insight.Scope.Tenant,
			insight.Scope.Project,
			insight.Scope.Namespace,
			nil,
			memory.DerivedInsightStateActive,
			"failure_pattern_evaluator",
			"threshold reached",
			insight.UpdatedAt,
			[]byte(`{"threshold":2}`),
		))
	mock.ExpectQuery("SELECT[\\s\\S]*FROM derived_insight_feedback").
		WithArgs(insight.ID, insight.Scope.Tenant, insight.Scope.Project, insight.Scope.Namespace, 100).
		WillReturnRows(derivedInsightFeedbackRows().AddRow(
			"feedback_123",
			insight.ID,
			insight.Scope.Tenant,
			insight.Scope.Project,
			insight.Scope.Namespace,
			memory.InsightFeedbackTypeUseful,
			"operator-a",
			"accurate",
			nil,
			insight.UpdatedAt,
			nil,
			nil,
			nil,
			"",
			[]byte(`{}`),
		))

	repo := NewRepository(mock)
	detail, err := repo.ReadDerivedInsight(context.Background(), memory.ReadDerivedInsightInput{
		Scope: insight.Scope,
		ID:    insight.ID,
	})
	if err != nil {
		t.Fatalf("ReadDerivedInsight() error = %v", err)
	}
	if detail.Insight.ID != insight.ID || len(detail.Evidence) != 1 || len(detail.Lifecycle) != 1 || detail.FeedbackSummary.PositiveCount != 1 {
		t.Fatalf("ReadDerivedInsight() = %+v, want insight with evidence and lifecycle", detail)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("ExpectationsWereMet() error = %v", err)
	}
}

func TestRepositoryTransitionDerivedInsightLifecycleUpdatesStateAndAudits(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatalf("pgxmock.NewPool() error = %v", err)
	}
	defer mock.Close()

	scope := memory.Scope{Tenant: "tenant-a", Project: "project-a", Namespace: "namespace-a"}
	occurredAt := time.Date(2026, 7, 4, 10, 0, 0, 0, time.UTC)
	transition := memory.DerivedInsightLifecycleTransition{
		Scope:      scope,
		InsightID:  "insight_123",
		FromState:  memory.DerivedInsightStateActive,
		ToState:    memory.DerivedInsightStateSuppressed,
		Actor:      "operator-a",
		Reason:     "noisy duplicate",
		OccurredAt: occurredAt,
	}

	mock.ExpectBegin()
	mock.ExpectExec("UPDATE derived_insights").
		WithArgs(transition.ToState, occurredAt, transition.InsightID, scope.Tenant, scope.Project, scope.Namespace, transition.FromState).
		WillReturnResult(pgxmock.NewResult("UPDATE", 1))
	mock.ExpectExec("INSERT INTO derived_insight_lifecycle_ledger").
		WithArgs(
			transition.InsightID,
			scope.Tenant,
			scope.Project,
			scope.Namespace,
			transition.FromState,
			transition.ToState,
			transition.Actor,
			transition.Reason,
			pgxmock.AnyArg(),
			transition.OccurredAt,
		).
		WillReturnResult(pgxmock.NewResult("INSERT", 1))
	mock.ExpectCommit()

	repo := NewRepository(mock)
	if err := repo.TransitionDerivedInsightLifecycle(context.Background(), transition); err != nil {
		t.Fatalf("TransitionDerivedInsightLifecycle() error = %v", err)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("ExpectationsWereMet() error = %v", err)
	}
}

func TestRepositoryCreateDerivedInsightFeedbackWritesScopedRecord(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatalf("pgxmock.NewPool() error = %v", err)
	}
	defer mock.Close()

	input := testDerivedInsightFeedbackInput()
	mock.ExpectQuery("INSERT INTO derived_insight_feedback").
		WithArgs(
			input.ID,
			input.InsightID,
			input.Scope.Tenant,
			input.Scope.Project,
			input.Scope.Namespace,
			input.Type,
			input.Actor,
			input.Reason,
			input.QualityScore,
			input.CreatedAt,
			input.RequestID,
			pgxmock.AnyArg(),
		).
		WillReturnRows(derivedInsightFeedbackRows().AddRow(
			input.ID,
			input.InsightID,
			input.Scope.Tenant,
			input.Scope.Project,
			input.Scope.Namespace,
			input.Type,
			input.Actor,
			input.Reason,
			*input.QualityScore,
			input.CreatedAt,
			nil,
			nil,
			nil,
			input.RequestID,
			[]byte(`{"source":"admin"}`),
		))

	repo := NewRepository(mock)
	got, err := repo.CreateDerivedInsightFeedback(context.Background(), input)
	if err != nil {
		t.Fatalf("CreateDerivedInsightFeedback() error = %v", err)
	}
	if got.ID != input.ID || got.Scope != input.Scope || got.Type != input.Type {
		t.Fatalf("CreateDerivedInsightFeedback() = %+v, want scoped feedback", got)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("ExpectationsWereMet() error = %v", err)
	}
}

func TestRepositoryListDerivedInsightFeedbackFiltersSupersededByDefault(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatalf("pgxmock.NewPool() error = %v", err)
	}
	defer mock.Close()

	input := testDerivedInsightFeedbackInput()
	mock.ExpectQuery("SELECT[\\s\\S]*FROM derived_insight_feedback").
		WithArgs(input.InsightID, input.Scope.Tenant, input.Scope.Project, input.Scope.Namespace, memory.InsightFeedbackTypeNoisy, 10).
		WillReturnRows(derivedInsightFeedbackRows().AddRow(
			input.ID,
			input.InsightID,
			input.Scope.Tenant,
			input.Scope.Project,
			input.Scope.Namespace,
			input.Type,
			input.Actor,
			input.Reason,
			*input.QualityScore,
			input.CreatedAt,
			nil,
			nil,
			nil,
			input.RequestID,
			[]byte(`{"source":"admin"}`),
		))

	repo := NewRepository(mock)
	items, err := repo.ListDerivedInsightFeedback(context.Background(), memory.ListDerivedInsightFeedbackInput{
		Scope:     input.Scope,
		InsightID: input.InsightID,
		Type:      memory.InsightFeedbackTypeNoisy,
		Limit:     10,
	})
	if err != nil {
		t.Fatalf("ListDerivedInsightFeedback() error = %v", err)
	}
	if len(items) != 1 || items[0].ID != input.ID {
		t.Fatalf("ListDerivedInsightFeedback() = %+v, want one feedback", items)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("ExpectationsWereMet() error = %v", err)
	}
}

func TestRepositorySupersedeDerivedInsightFeedbackUpdatesScopedRecord(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatalf("pgxmock.NewPool() error = %v", err)
	}
	defer mock.Close()

	input := memory.SupersedeDerivedInsightFeedbackInput{
		Scope:        memory.Scope{Tenant: "tenant-a", Project: "project-a", Namespace: "namespace-a"},
		FeedbackID:   "feedback_123",
		Actor:        "operator-b",
		Reason:       "replaced by more precise review",
		SupersededAt: time.Date(2026, 7, 4, 15, 0, 0, 0, time.UTC),
	}
	mock.ExpectExec("UPDATE derived_insight_feedback").
		WithArgs(input.SupersededAt, input.Actor, input.Reason, input.FeedbackID, input.Scope.Tenant, input.Scope.Project, input.Scope.Namespace).
		WillReturnResult(pgxmock.NewResult("UPDATE", 1))

	repo := NewRepository(mock)
	if err := repo.SupersedeDerivedInsightFeedback(context.Background(), input); err != nil {
		t.Fatalf("SupersedeDerivedInsightFeedback() error = %v", err)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("ExpectationsWereMet() error = %v", err)
	}
}

func TestRepositorySummarizeDerivedInsightFeedbackUsesActiveScopedRecords(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatalf("pgxmock.NewPool() error = %v", err)
	}
	defer mock.Close()

	scope := memory.Scope{Tenant: "tenant-a", Project: "project-a", Namespace: "namespace-a"}
	createdAt := time.Date(2026, 7, 4, 14, 0, 0, 0, time.UTC)
	mock.ExpectQuery("SELECT[\\s\\S]*FROM derived_insight_feedback").
		WithArgs("insight_123", scope.Tenant, scope.Project, scope.Namespace, 100).
		WillReturnRows(derivedInsightFeedbackRows().
			AddRow("feedback_useful", "insight_123", scope.Tenant, scope.Project, scope.Namespace, memory.InsightFeedbackTypeUseful, "operator-a", "works", nil, createdAt, nil, nil, nil, "", []byte(`{}`)).
			AddRow("feedback_noisy", "insight_123", scope.Tenant, scope.Project, scope.Namespace, memory.InsightFeedbackTypeNoisy, "operator-b", "too broad", nil, createdAt.Add(time.Minute), nil, nil, nil, "", []byte(`{}`)))

	repo := NewRepository(mock)
	summary, err := repo.SummarizeDerivedInsightFeedback(context.Background(), memory.SummarizeDerivedInsightFeedbackInput{
		Scope:     scope,
		InsightID: "insight_123",
	})
	if err != nil {
		t.Fatalf("SummarizeDerivedInsightFeedback() error = %v", err)
	}
	if summary.PositiveCount != 1 || summary.NegativeCount != 1 || summary.TotalActive != 2 {
		t.Fatalf("summary = %+v, want useful and noisy active counts", summary)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("ExpectationsWereMet() error = %v", err)
	}
}

func TestRepositoryListFailureEvidenceReturnsScopedDeterministicInputs(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatalf("pgxmock.NewPool() error = %v", err)
	}
	defer mock.Close()

	scope := memory.Scope{Tenant: "tenant-a", Project: "project-a", Namespace: "namespace-a"}
	observedAt := time.Date(2026, 7, 4, 10, 0, 0, 0, time.UTC)
	mock.ExpectQuery("SELECT[\\s\\S]*FROM \\(").
		WithArgs(
			scope.Tenant, scope.Project, scope.Namespace,
			scope.Tenant, scope.Project, scope.Namespace,
			scope.Tenant, scope.Project, scope.Namespace,
			scope.Tenant, scope.Project, scope.Namespace,
			scope.Tenant, scope.Project, scope.Namespace,
			50,
		).
		WillReturnRows(pgxmock.NewRows([]string{"evidence_kind", "evidence_id", "failure_key", "message", "observed_at", "metadata"}).
			AddRow(memory.DerivedInsightEvidenceKindJobExecution, "job_1", "embedding_rebuild:provider unavailable", "provider unavailable", observedAt, []byte(`{"job_name":"embedding_rebuild"}`)).
			AddRow(memory.DerivedInsightEvidenceKindEmbeddingRebuild, "mem_1", "embedding_rebuild:provider unavailable", "provider unavailable", observedAt.Add(time.Minute), []byte(`{"requested_provider":"openai"}`)))

	repo := NewRepository(mock)
	items, err := repo.ListFailureEvidence(context.Background(), scope, 50)
	if err != nil {
		t.Fatalf("ListFailureEvidence() error = %v", err)
	}
	if len(items) != 2 {
		t.Fatalf("len(items) = %d, want 2", len(items))
	}
	if items[0].Kind != memory.DerivedInsightEvidenceKindJobExecution || items[1].Kind != memory.DerivedInsightEvidenceKindEmbeddingRebuild {
		t.Fatalf("evidence kinds = %s/%s, want job_execution/embedding_rebuild", items[0].Kind, items[1].Kind)
	}
	if items[0].Metadata["job_name"] != "embedding_rebuild" {
		t.Fatalf("metadata = %+v, want job_name", items[0].Metadata)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("ExpectationsWereMet() error = %v", err)
	}
}

func TestRepositoryCreateDerivedInsightReplayRunWritesScopedAuditRecord(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatalf("pgxmock.NewPool() error = %v", err)
	}
	defer mock.Close()

	run := testDerivedInsightReplayRun()
	mock.ExpectQuery("INSERT INTO derived_insight_replay_runs").
		WithArgs(
			run.ID,
			run.Scope.Tenant,
			run.Scope.Project,
			run.Scope.Namespace,
			run.Mode,
			run.Status,
			[]string{string(memory.DerivedInsightTypeFailurePattern), string(memory.DerivedInsightTypeLesson)},
			run.Request.EvidenceWindowStart,
			run.Request.EvidenceWindowEnd,
			run.Request.EvidenceLimit,
			run.Actor,
			run.Reason,
			run.Request.IdempotencyKey,
			pgxmock.AnyArg(),
			run.CreatedAt,
			run.UpdatedAt,
		).
		WillReturnRows(derivedInsightReplayRunRows().AddRow(
			run.ID,
			run.Scope.Tenant,
			run.Scope.Project,
			run.Scope.Namespace,
			run.Mode,
			run.Status,
			[]string{string(memory.DerivedInsightTypeFailurePattern), string(memory.DerivedInsightTypeLesson)},
			run.Request.EvidenceWindowStart,
			run.Request.EvidenceWindowEnd,
			run.Request.EvidenceLimit,
			run.Actor,
			run.Reason,
			run.Request.IdempotencyKey,
			[]byte(`{"source":"admin"}`),
			nil,
			nil,
			nil,
			nil,
			nil,
			nil,
			run.CreatedAt,
			run.UpdatedAt,
		))

	repo := NewRepository(mock)
	got, err := repo.CreateDerivedInsightReplayRun(context.Background(), run)
	if err != nil {
		t.Fatalf("CreateDerivedInsightReplayRun() error = %v", err)
	}
	if got.ID != run.ID || got.Scope != run.Scope || got.Request.IdempotencyKey != run.Request.IdempotencyKey {
		t.Fatalf("CreateDerivedInsightReplayRun() = %+v, want scoped replay run", got)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("ExpectationsWereMet() error = %v", err)
	}
}

func TestRepositoryFindDerivedInsightReplayRunByIdempotencyKeyScopesLookup(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatalf("pgxmock.NewPool() error = %v", err)
	}
	defer mock.Close()

	run := testDerivedInsightReplayRun()
	mock.ExpectQuery("SELECT[\\s\\S]*FROM derived_insight_replay_runs").
		WithArgs(run.Scope.Tenant, run.Scope.Project, run.Scope.Namespace, run.Request.IdempotencyKey).
		WillReturnRows(derivedInsightReplayRunRows().AddRow(
			run.ID,
			run.Scope.Tenant,
			run.Scope.Project,
			run.Scope.Namespace,
			run.Mode,
			run.Status,
			[]string{string(memory.DerivedInsightTypeFailurePattern), string(memory.DerivedInsightTypeLesson)},
			run.Request.EvidenceWindowStart,
			run.Request.EvidenceWindowEnd,
			run.Request.EvidenceLimit,
			run.Actor,
			run.Reason,
			run.Request.IdempotencyKey,
			[]byte(`{"source":"admin"}`),
			nil,
			nil,
			nil,
			nil,
			nil,
			nil,
			run.CreatedAt,
			run.UpdatedAt,
		))

	repo := NewRepository(mock)
	got, err := repo.FindDerivedInsightReplayRunByIdempotencyKey(context.Background(), memory.FindDerivedInsightReplayRunByIdempotencyKeyInput{
		Scope:          run.Scope,
		IdempotencyKey: run.Request.IdempotencyKey,
	})
	if err != nil {
		t.Fatalf("FindDerivedInsightReplayRunByIdempotencyKey() error = %v", err)
	}
	if got.ID != run.ID || got.Scope != run.Scope {
		t.Fatalf("FindDerivedInsightReplayRunByIdempotencyKey() = %+v, want scoped run", got)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("ExpectationsWereMet() error = %v", err)
	}
}

func TestRepositoryReadDerivedInsightReplayRunRequiresScopedRun(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatalf("pgxmock.NewPool() error = %v", err)
	}
	defer mock.Close()

	run := testDerivedInsightReplayRun()
	mock.ExpectQuery("SELECT[\\s\\S]*FROM derived_insight_replay_runs").
		WithArgs(run.ID, run.Scope.Tenant, run.Scope.Project, run.Scope.Namespace).
		WillReturnRows(derivedInsightReplayRunRows().AddRow(
			run.ID,
			run.Scope.Tenant,
			run.Scope.Project,
			run.Scope.Namespace,
			run.Mode,
			run.Status,
			[]string{string(memory.DerivedInsightTypeFailurePattern), string(memory.DerivedInsightTypeLesson)},
			run.Request.EvidenceWindowStart,
			run.Request.EvidenceWindowEnd,
			run.Request.EvidenceLimit,
			run.Actor,
			run.Reason,
			run.Request.IdempotencyKey,
			[]byte(`{"source":"admin"}`),
			nil,
			nil,
			nil,
			nil,
			nil,
			nil,
			run.CreatedAt,
			run.UpdatedAt,
		))

	repo := NewRepository(mock)
	got, err := repo.ReadDerivedInsightReplayRun(context.Background(), memory.ReadDerivedInsightReplayRunInput{
		Scope: run.Scope,
		RunID: run.ID,
	})
	if err != nil {
		t.Fatalf("ReadDerivedInsightReplayRun() error = %v", err)
	}
	if got.ID != run.ID || got.Scope != run.Scope {
		t.Fatalf("ReadDerivedInsightReplayRun() = %+v, want scoped run", got)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("ExpectationsWereMet() error = %v", err)
	}
}

func TestRepositoryListDerivedInsightReplayRunsFiltersByScopeStatusAndMode(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatalf("pgxmock.NewPool() error = %v", err)
	}
	defer mock.Close()

	run := testDerivedInsightReplayRun()
	mock.ExpectQuery("SELECT[\\s\\S]*FROM derived_insight_replay_runs").
		WithArgs(run.Scope.Tenant, run.Scope.Project, run.Scope.Namespace, run.Status, run.Mode, 20).
		WillReturnRows(derivedInsightReplayRunRows().AddRow(
			run.ID,
			run.Scope.Tenant,
			run.Scope.Project,
			run.Scope.Namespace,
			run.Mode,
			run.Status,
			[]string{string(memory.DerivedInsightTypeFailurePattern), string(memory.DerivedInsightTypeLesson)},
			run.Request.EvidenceWindowStart,
			run.Request.EvidenceWindowEnd,
			run.Request.EvidenceLimit,
			run.Actor,
			run.Reason,
			run.Request.IdempotencyKey,
			[]byte(`{"source":"admin"}`),
			nil,
			nil,
			nil,
			nil,
			nil,
			nil,
			run.CreatedAt,
			run.UpdatedAt,
		))

	repo := NewRepository(mock)
	items, err := repo.ListDerivedInsightReplayRuns(context.Background(), memory.ListDerivedInsightReplayRunsInput{
		Scope:  run.Scope,
		Status: run.Status,
		Mode:   run.Mode,
		Limit:  20,
	})
	if err != nil {
		t.Fatalf("ListDerivedInsightReplayRuns() error = %v", err)
	}
	if len(items) != 1 || items[0].ID != run.ID {
		t.Fatalf("ListDerivedInsightReplayRuns() = %+v, want one scoped run", items)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("ExpectationsWereMet() error = %v", err)
	}
}

func TestRepositoryUpdateDerivedInsightReplayRunStatusPersistsStatusAndFailure(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatalf("pgxmock.NewPool() error = %v", err)
	}
	defer mock.Close()

	scope := memory.Scope{Tenant: "tenant-a", Project: "project-a", Namespace: "namespace-a"}
	updatedAt := time.Date(2026, 7, 11, 10, 10, 0, 0, time.UTC)
	mock.ExpectExec("UPDATE derived_insight_replay_runs").
		WithArgs(memory.DerivedInsightReplayStatusFailed, "worker failed", updatedAt, nil, updatedAt, "replay_123", scope.Tenant, scope.Project, scope.Namespace).
		WillReturnResult(pgxmock.NewResult("UPDATE", 1))

	repo := NewRepository(mock)
	err = repo.UpdateDerivedInsightReplayRunStatus(context.Background(), memory.UpdateDerivedInsightReplayRunStatusInput{
		Scope:      scope,
		RunID:      "replay_123",
		Status:     memory.DerivedInsightReplayStatusFailed,
		Failure:    "worker failed",
		UpdatedAt:  updatedAt,
		FinishedAt: updatedAt,
	})
	if err != nil {
		t.Fatalf("UpdateDerivedInsightReplayRunStatus() error = %v", err)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("ExpectationsWereMet() error = %v", err)
	}
}

func TestRepositoryStoreDerivedInsightReplayReportPersistsCountersAndDecisions(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatalf("pgxmock.NewPool() error = %v", err)
	}
	defer mock.Close()

	report := testDerivedInsightReplayReport()
	mock.ExpectExec("UPDATE derived_insight_replay_runs").
		WithArgs(pgxmock.AnyArg(), pgxmock.AnyArg(), nil, report.GeneratedAt, report.GeneratedAt, report.RunID, report.Scope.Tenant, report.Scope.Project, report.Scope.Namespace).
		WillReturnResult(pgxmock.NewResult("UPDATE", 1))

	repo := NewRepository(mock)
	if err := repo.StoreDerivedInsightReplayReport(context.Background(), report); err != nil {
		t.Fatalf("StoreDerivedInsightReplayReport() error = %v", err)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("ExpectationsWereMet() error = %v", err)
	}
}

func testDerivedInsightFeedbackInput() memory.CreateDerivedInsightFeedbackInput {
	score := 0.2
	return memory.CreateDerivedInsightFeedbackInput{
		ID:           "feedback_123",
		InsightID:    "insight_123",
		Scope:        memory.Scope{Tenant: "tenant-a", Project: "project-a", Namespace: "namespace-a"},
		Type:         memory.InsightFeedbackTypeNoisy,
		Actor:        "operator-a",
		Reason:       "too broad for context assembly",
		QualityScore: &score,
		CreatedAt:    time.Date(2026, 7, 4, 14, 0, 0, 0, time.UTC),
		RequestID:    "req_123",
		Metadata:     map[string]any{"source": "admin"},
	}
}

func testDerivedInsightReplayRun() memory.DerivedInsightReplayRun {
	request := memory.DerivedInsightReplayRequest{
		Scope:               memory.Scope{Tenant: "tenant-a", Project: "project-a", Namespace: "namespace-a"},
		Mode:                memory.DerivedInsightReplayModeApply,
		InsightTypes:        []memory.DerivedInsightType{memory.DerivedInsightTypeFailurePattern, memory.DerivedInsightTypeLesson},
		EvidenceWindowStart: time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC),
		EvidenceWindowEnd:   time.Date(2026, 7, 2, 0, 0, 0, 0, time.UTC),
		EvidenceLimit:       100,
		Actor:               "operator-a",
		Reason:              "backfill after evaluator update",
		IdempotencyKey:      "replay:tenant-a:project-a:namespace-a:2026-07-01",
		RequestedAt:         time.Date(2026, 7, 11, 10, 0, 0, 0, time.UTC),
		Metadata:            map[string]any{"source": "admin"},
	}
	return memory.DerivedInsightReplayRun{
		ID:        "replay_123",
		Scope:     request.Scope,
		Mode:      request.Mode,
		Status:    memory.DerivedInsightReplayStatusPending,
		Request:   request,
		Actor:     request.Actor,
		Reason:    request.Reason,
		CreatedAt: request.RequestedAt,
		UpdatedAt: request.RequestedAt,
	}
}

func testDerivedInsightReplayReport() memory.DerivedInsightReplayReport {
	return memory.DerivedInsightReplayReport{
		RunID: "replay_123",
		Scope: memory.Scope{Tenant: "tenant-a", Project: "project-a", Namespace: "namespace-a"},
		Counters: memory.DerivedInsightReplayCounters{
			EvidenceEvaluated: 2,
			Created:           1,
			Skipped:           1,
		},
		Decisions: []memory.DerivedInsightReplayDecision{
			{
				InsightType:   memory.DerivedInsightTypeFailurePattern,
				Fingerprint:   "failure_pattern:provider_unavailable",
				Decision:      memory.DerivedInsightReplayDecisionCreate,
				Reason:        memory.DerivedInsightReplayReasonRepeatedEvidence,
				EvidenceCount: 2,
			},
			{
				InsightType: memory.DerivedInsightTypeLesson,
				Fingerprint: "lesson:provider_unavailable",
				Decision:    memory.DerivedInsightReplayDecisionSkip,
				Reason:      memory.DerivedInsightReplayReasonInsufficientEvidence,
			},
		},
		GeneratedAt: time.Date(2026, 7, 11, 10, 30, 0, 0, time.UTC),
	}
}

func testDerivedInsight() memory.DerivedInsight {
	return memory.DerivedInsight{
		ID:      "insight_123",
		Scope:   memory.Scope{Tenant: "tenant-a", Project: "project-a", Namespace: "namespace-a"},
		Type:    memory.DerivedInsightTypeFailurePattern,
		State:   memory.DerivedInsightStateActive,
		Title:   "Embedding rebuild repeatedly fails",
		Summary: "The embedding rebuild path failed repeatedly with provider unavailable.",
		Confidence: memory.DerivedInsightConfidence{
			Score:  0.75,
			Method: "repeated_evidence_ratio",
		},
		Payload: map[string]any{
			"normalized_key": "embedding_provider_unavailable",
		},
		Derivation: memory.DerivedInsightDerivation{
			Source:              "failure_pattern_evaluator",
			Fingerprint:         "failure_pattern:tenant-a:project-a:namespace-a:embedding_provider_unavailable",
			EvidenceWindowStart: time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC),
			EvidenceWindowEnd:   time.Date(2026, 7, 2, 0, 0, 0, 0, time.UTC),
			DerivedAt:           time.Date(2026, 7, 2, 1, 0, 0, 0, time.UTC),
			Metadata:            map[string]any{"threshold": float64(2)},
		},
		Evidence: []memory.DerivedInsightEvidenceRef{
			{
				Kind:       memory.DerivedInsightEvidenceKindJobExecution,
				ID:         "job_1",
				Relation:   memory.DerivedInsightEvidenceRelationSupports,
				ObservedAt: time.Date(2026, 7, 1, 1, 0, 0, 0, time.UTC),
				Metadata:   map[string]any{"error": "provider unavailable"},
			},
			{
				Kind:       memory.DerivedInsightEvidenceKindEmbeddingRebuild,
				ID:         "mem_1",
				Relation:   memory.DerivedInsightEvidenceRelationSupports,
				ObservedAt: time.Date(2026, 7, 1, 2, 0, 0, 0, time.UTC),
			},
		},
		LastObservedAt: time.Date(2026, 7, 1, 2, 0, 0, 0, time.UTC),
		CreatedAt:      time.Date(2026, 7, 2, 1, 0, 0, 0, time.UTC),
		UpdatedAt:      time.Date(2026, 7, 2, 1, 0, 0, 0, time.UTC),
	}
}

func derivedInsightFeedbackRows() *pgxmock.Rows {
	return pgxmock.NewRows([]string{
		"id",
		"insight_id",
		"tenant",
		"project",
		"namespace",
		"feedback_type",
		"actor",
		"reason",
		"quality_score",
		"created_at",
		"superseded_at",
		"superseded_by_actor",
		"superseded_by_reason",
		"request_id",
		"metadata",
	})
}

func derivedInsightRows() *pgxmock.Rows {
	return pgxmock.NewRows([]string{
		"id",
		"tenant",
		"project",
		"namespace",
		"type",
		"lifecycle_state",
		"title",
		"summary",
		"confidence",
		"confidence_method",
		"payload",
		"lesson",
		"derivation_source",
		"derivation_fingerprint",
		"derivation_metadata",
		"evidence_window_start",
		"evidence_window_end",
		"derived_at",
		"last_observed_at",
		"created_at",
		"updated_at",
	})
}

func derivedInsightRowsWithEvidenceCount() *pgxmock.Rows {
	return pgxmock.NewRows([]string{
		"id",
		"tenant",
		"project",
		"namespace",
		"type",
		"lifecycle_state",
		"title",
		"summary",
		"confidence",
		"confidence_method",
		"payload",
		"lesson",
		"derivation_source",
		"derivation_fingerprint",
		"derivation_metadata",
		"evidence_window_start",
		"evidence_window_end",
		"derived_at",
		"last_observed_at",
		"created_at",
		"updated_at",
		"evidence_count",
	})
}

func derivedInsightEvidenceRows() *pgxmock.Rows {
	return pgxmock.NewRows([]string{"evidence_kind", "evidence_id", "relation", "observed_at", "metadata"})
}

func derivedInsightLifecycleRows() *pgxmock.Rows {
	return pgxmock.NewRows([]string{
		"id",
		"insight_id",
		"tenant",
		"project",
		"namespace",
		"from_state",
		"to_state",
		"actor",
		"reason",
		"occurred_at",
		"metadata",
	})
}

func derivedInsightReplayRunRows() *pgxmock.Rows {
	return pgxmock.NewRows([]string{
		"id",
		"tenant",
		"project",
		"namespace",
		"mode",
		"status",
		"insight_types",
		"evidence_window_start",
		"evidence_window_end",
		"evidence_limit",
		"actor",
		"reason",
		"idempotency_key",
		"request_metadata",
		"report_counters",
		"report_decisions",
		"failure",
		"report_generated_at",
		"started_at",
		"finished_at",
		"created_at",
		"updated_at",
	})
}
