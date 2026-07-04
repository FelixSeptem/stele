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

	repo := NewRepository(mock)
	detail, err := repo.ReadDerivedInsight(context.Background(), memory.ReadDerivedInsightInput{
		Scope: insight.Scope,
		ID:    insight.ID,
	})
	if err != nil {
		t.Fatalf("ReadDerivedInsight() error = %v", err)
	}
	if detail.Insight.ID != insight.ID || len(detail.Evidence) != 1 || len(detail.Lifecycle) != 1 {
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
