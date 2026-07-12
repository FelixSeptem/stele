package postgres

import (
	"context"
	"testing"
	"time"

	"github.com/FelixSeptem/stele/internal/memory"
	pgxmock "github.com/pashagolub/pgxmock/v4"
)

func TestRepositoryCreateTaskEvaluationWritesEvidenceAndIsIdempotent(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatalf("pgxmock.NewPool() error = %v", err)
	}
	defer mock.Close()

	scope := memory.Scope{Tenant: "tenant-a", Project: "project-a", Namespace: "namespace-a"}
	now := time.Date(2026, 7, 12, 9, 0, 0, 0, time.UTC)
	evaluation := memory.TaskEvaluation{
		ID:              "task_eval_1",
		Scope:           scope,
		Objective:       "verify scoped retrieval",
		SuccessCriteria: []string{"only scoped memory returned"},
		Verdict:         memory.TaskEvaluationVerdictSucceeded,
		Evidence: []memory.TaskEvidenceLink{{
			Kind: memory.TaskEvidenceTargetSession,
			ID:   "session_1",
		}},
		Actor:     "operator-a",
		Reason:    "caller recorded success",
		IdempotencyKey: "task-eval-1",
		Metadata:  map[string]any{"session_id": "session_1"},
		CreatedAt: now,
		UpdatedAt: now,
	}

	mock.ExpectBegin()
	mock.ExpectQuery("INSERT INTO task_evaluations").
		WithArgs(evaluation.ID, scope.Tenant, scope.Project, scope.Namespace, evaluation.Objective, []string{"only scoped memory returned"}, evaluation.Verdict, []string{}, evaluation.Actor, evaluation.Reason, evaluation.IdempotencyKey, []byte(`{"session_id":"session_1"}`), nil, nil, nil, nil, nil, evaluation.CreatedAt, evaluation.UpdatedAt).
		WillReturnRows(pgxmock.NewRows([]string{"id", "tenant", "project", "namespace", "objective", "success_criteria", "verdict", "contribution_categories", "actor", "reason", "idempotency_key", "metadata", "correction_state", "superseded_at", "superseded_by_task_evaluation_id", "superseded_by_actor", "superseded_by_reason", "created_at", "updated_at"}).
			AddRow(evaluation.ID, scope.Tenant, scope.Project, scope.Namespace, evaluation.Objective, []string{"only scoped memory returned"}, evaluation.Verdict, []string{}, evaluation.Actor, evaluation.Reason, evaluation.IdempotencyKey, []byte(`{"session_id":"session_1"}`), memory.TaskEvaluationCorrectionStateActive, nil, nil, nil, nil, now, now))
	mock.ExpectQuery("INSERT INTO task_evidence_links").
		WithArgs(evaluation.ID, scope.Tenant, scope.Project, scope.Namespace, memory.TaskEvidenceTargetSession, "session_1", nil, []byte(`{}`)).
		WillReturnRows(pgxmock.NewRows([]string{"evidence_kind", "evidence_id", "opaque_token", "metadata"}).AddRow(memory.TaskEvidenceTargetSession, "session_1", nil, []byte(`{}`)))
	mock.ExpectCommit()

	repo := NewRepository(mock)
	created, err := repo.CreateTaskEvaluation(context.Background(), evaluation)
	if err != nil {
		t.Fatalf("CreateTaskEvaluation() error = %v", err)
	}
	if created.ID != evaluation.ID {
		t.Fatalf("created.ID = %q, want %q", created.ID, evaluation.ID)
	}
	if len(created.Evidence) != 1 || created.Evidence[0].ID != "session_1" {
		t.Fatalf("created.Evidence = %+v, want session_1", created.Evidence)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("ExpectationsWereMet() error = %v", err)
	}
}

func TestRepositorySupersedeTaskEvaluationWritesAuditRecord(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatalf("pgxmock.NewPool() error = %v", err)
	}
	defer mock.Close()

	scope := memory.Scope{Tenant: "tenant-a", Project: "project-a", Namespace: "namespace-a"}
	now := time.Date(2026, 7, 12, 9, 5, 0, 0, time.UTC)

	mock.ExpectBegin()
	mock.ExpectExec("UPDATE task_evaluations").
		WithArgs(scope.Tenant, scope.Project, scope.Namespace, "operator-b", "corrected objective", string(memory.TaskEvaluationCorrectionStateSuperseded), now, "task_eval_1").
		WillReturnResult(pgxmock.NewResult("UPDATE", 1))
	mock.ExpectExec("INSERT INTO task_evaluation_supersessions").
		WithArgs(scope.Tenant, scope.Project, scope.Namespace, "task_eval_1", "task_eval_2", "operator-b", "corrected objective", now).
		WillReturnResult(pgxmock.NewResult("INSERT", 1))
	mock.ExpectCommit()

	repo := NewRepository(mock)
	if err := repo.SupersedeTaskEvaluation(context.Background(), memory.SupersedeTaskEvaluationInput{
		Scope:        scope,
		EvaluationID: "task_eval_1",
		SupersedingID: "task_eval_2",
		Actor:        "operator-b",
		Reason:       "corrected objective",
		SupersededAt: now,
	}); err != nil {
		t.Fatalf("SupersedeTaskEvaluation() error = %v", err)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("ExpectationsWereMet() error = %v", err)
	}
}
