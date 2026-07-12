package postgres

import (
	"context"
	"testing"
	"time"

	"github.com/FelixSeptem/stele/internal/memory"
	pgxmock "github.com/pashagolub/pgxmock/v4"
)

func TestRepositoryCreatesSupersedesAndSummarizesUsefulnessFeedback(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatalf("pgxmock.NewPool() error = %v", err)
	}
	defer mock.Close()

	scope := memory.Scope{Tenant: "tenant-a", Project: "project-a", Namespace: "namespace-a"}
	now := time.Date(2026, 7, 11, 10, 30, 0, 0, time.UTC)
	feedback := memory.UsefulnessFeedback{
		ID:            "feedback_1",
		Scope:         scope,
		Type:          memory.UsefulnessFeedbackTypeNoisy,
		SourceSurface: memory.UsefulnessFeedbackSourceSession,
		TaskEvaluationID: "task_eval_1",
		Subjects: []memory.UsefulnessFeedbackSubject{{
			Kind: memory.UsefulnessFeedbackSubjectMemory,
			ID:   "mem_1",
		}},
		Actor:          "agent-a",
		Reason:         "too broad for this turn",
		IdempotencyKey: "session_1:turn_1:mem_1:noisy",
		Metadata:       map[string]any{"session_id": "session_1", "turn_id": "turn_1"},
		CreatedAt:      now,
	}

	mock.ExpectBegin()
	mock.ExpectQuery("INSERT INTO usefulness_feedback").
		WithArgs(feedback.ID, scope.Tenant, scope.Project, scope.Namespace, feedback.Type, feedback.SourceSurface, feedback.TaskEvaluationID, feedback.Actor, feedback.Reason, feedback.IdempotencyKey, []byte(`{"session_id":"session_1","turn_id":"turn_1"}`), now).
		WillReturnRows(usefulnessFeedbackRows().AddRow(feedback.ID, scope.Tenant, scope.Project, scope.Namespace, feedback.Type, feedback.SourceSurface, feedback.TaskEvaluationID, feedback.Actor, feedback.Reason, feedback.IdempotencyKey, []byte(`{"session_id":"session_1","turn_id":"turn_1"}`), nil, nil, nil, now))
	mock.ExpectQuery("INSERT INTO usefulness_feedback_subjects").
		WithArgs(feedback.ID, scope.Tenant, scope.Project, scope.Namespace, memory.UsefulnessFeedbackSubjectMemory, "mem_1", nil, nil, nil).
		WillReturnRows(usefulnessFeedbackSubjectRows().AddRow(feedback.ID, scope.Tenant, scope.Project, scope.Namespace, memory.UsefulnessFeedbackSubjectMemory, "mem_1", nil, nil, nil))
	mock.ExpectCommit()
	mock.ExpectBegin()
	mock.ExpectExec("UPDATE usefulness_feedback").
		WithArgs(scope.Tenant, scope.Project, scope.Namespace, feedback.ID, "operator-a", "incorrect signal", now.Add(time.Minute)).
		WillReturnResult(pgxmock.NewResult("UPDATE", 1))
	mock.ExpectExec("INSERT INTO usefulness_feedback_supersessions").
		WithArgs(scope.Tenant, scope.Project, scope.Namespace, feedback.ID, "operator-a", "incorrect signal", now.Add(time.Minute)).
		WillReturnResult(pgxmock.NewResult("INSERT", 1))
	mock.ExpectCommit()
	mock.ExpectQuery("SELECT[\\s\\S]*FROM usefulness_feedback uf").
		WithArgs(scope.Tenant, scope.Project, scope.Namespace, memory.UsefulnessFeedbackSubjectMemory, "mem_1", "", "", "").
		WillReturnRows(usefulnessFeedbackRows().
			AddRow("feedback_1", scope.Tenant, scope.Project, scope.Namespace, memory.UsefulnessFeedbackTypeNoisy, memory.UsefulnessFeedbackSourceSession, "task_eval_1", "agent-a", "too broad", "idem-1", []byte(`{}`), now.Add(time.Minute), "operator-a", "incorrect signal", now).
			AddRow("feedback_2", scope.Tenant, scope.Project, scope.Namespace, memory.UsefulnessFeedbackTypeUseful, memory.UsefulnessFeedbackSourceSession, "", "agent-a", "helped", "idem-2", []byte(`{}`), nil, nil, nil, now.Add(2*time.Minute)))
	mock.ExpectQuery("SELECT[\\s\\S]*FROM usefulness_feedback_subjects").
		WithArgs([]string{"feedback_1", "feedback_2"}, scope.Tenant, scope.Project, scope.Namespace).
		WillReturnRows(usefulnessFeedbackSubjectRows().
			AddRow("feedback_1", scope.Tenant, scope.Project, scope.Namespace, memory.UsefulnessFeedbackSubjectMemory, "mem_1", nil, nil, nil).
			AddRow("feedback_2", scope.Tenant, scope.Project, scope.Namespace, memory.UsefulnessFeedbackSubjectMemory, "mem_1", nil, nil, nil))

	repo := NewRepository(mock)
	created, err := repo.CreateUsefulnessFeedback(context.Background(), feedback)
	if err != nil {
		t.Fatalf("CreateUsefulnessFeedback() error = %v", err)
	}
	if created.ID != feedback.ID || len(created.Subjects) != 1 || created.Subjects[0].ID != "mem_1" {
		t.Fatalf("created = %+v, want feedback with memory subject", created)
	}
	if err := repo.SupersedeUsefulnessFeedback(context.Background(), memory.SupersedeUsefulnessFeedbackInput{
		Scope:        scope,
		FeedbackID:   feedback.ID,
		Actor:        "operator-a",
		Reason:       "incorrect signal",
		SupersededAt: now.Add(time.Minute),
	}); err != nil {
		t.Fatalf("SupersedeUsefulnessFeedback() error = %v", err)
	}
	summary, err := repo.SummarizeUsefulnessFeedback(context.Background(), memory.SummarizeUsefulnessFeedbackInput{
		Scope:   scope,
		Subject: memory.UsefulnessFeedbackSubject{Kind: memory.UsefulnessFeedbackSubjectMemory, ID: "mem_1"},
	})
	if err != nil {
		t.Fatalf("SummarizeUsefulnessFeedback() error = %v", err)
	}
	if summary.TotalActive != 1 || summary.Counts[memory.UsefulnessFeedbackTypeUseful] != 1 || summary.Counts[memory.UsefulnessFeedbackTypeNoisy] != 0 {
		t.Fatalf("summary = %+v, want only active useful feedback counted", summary)
	}
	if created.TaskEvaluationID != "task_eval_1" {
		t.Fatalf("created.TaskEvaluationID = %q, want task_eval_1", created.TaskEvaluationID)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("ExpectationsWereMet() error = %v", err)
	}
}

func TestRepositoryReadsAndListsUsefulnessFeedbackWithSubjects(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatalf("pgxmock.NewPool() error = %v", err)
	}
	defer mock.Close()

	scope := memory.Scope{Tenant: "tenant-a", Project: "project-a", Namespace: "namespace-a"}
	now := time.Date(2026, 7, 11, 11, 0, 0, 0, time.UTC)

	mock.ExpectQuery("SELECT[\\s\\S]*FROM usefulness_feedback[\\s\\S]*WHERE tenant").
		WithArgs(scope.Tenant, scope.Project, scope.Namespace, "feedback_1").
		WillReturnRows(usefulnessFeedbackRows().AddRow("feedback_1", scope.Tenant, scope.Project, scope.Namespace, memory.UsefulnessFeedbackTypeMissingExpected, memory.UsefulnessFeedbackSourceVerification, "", "agent-a", "expected event missing", "idem-1", []byte(`{"verification_id":"verification_1"}`), nil, nil, nil, now))
	mock.ExpectQuery("SELECT[\\s\\S]*FROM usefulness_feedback_subjects").
		WithArgs([]string{"feedback_1"}, scope.Tenant, scope.Project, scope.Namespace).
		WillReturnRows(usefulnessFeedbackSubjectRows().AddRow("feedback_1", scope.Tenant, scope.Project, scope.Namespace, memory.UsefulnessFeedbackSubjectExpectedRecall, nil, memory.ExpectedRecallTargetEvent, "evt_1", nil))
	mock.ExpectQuery("SELECT[\\s\\S]*FROM usefulness_feedback uf").
		WithArgs(scope.Tenant, scope.Project, scope.Namespace, string(memory.UsefulnessFeedbackTypeMissingExpected), false, 25).
		WillReturnRows(usefulnessFeedbackRows().AddRow("feedback_1", scope.Tenant, scope.Project, scope.Namespace, memory.UsefulnessFeedbackTypeMissingExpected, memory.UsefulnessFeedbackSourceVerification, "", "agent-a", "expected event missing", "idem-1", []byte(`{}`), nil, nil, nil, now))
	mock.ExpectQuery("SELECT[\\s\\S]*FROM usefulness_feedback_subjects").
		WithArgs([]string{"feedback_1"}, scope.Tenant, scope.Project, scope.Namespace).
		WillReturnRows(usefulnessFeedbackSubjectRows().AddRow("feedback_1", scope.Tenant, scope.Project, scope.Namespace, memory.UsefulnessFeedbackSubjectExpectedRecall, nil, memory.ExpectedRecallTargetEvent, "evt_1", nil))

	repo := NewRepository(mock)
	read, err := repo.ReadUsefulnessFeedback(context.Background(), memory.ReadUsefulnessFeedbackInput{
		Scope:      scope,
		FeedbackID: "feedback_1",
	})
	if err != nil {
		t.Fatalf("ReadUsefulnessFeedback() error = %v", err)
	}
	if len(read.Subjects) != 1 || read.Subjects[0].ExpectedRecallTarget.ID != "evt_1" {
		t.Fatalf("read = %+v, want expected-recall event subject", read)
	}
	listed, err := repo.ListUsefulnessFeedback(context.Background(), memory.ListUsefulnessFeedbackInput{
		Scope: scope,
		Type:  memory.UsefulnessFeedbackTypeMissingExpected,
		Limit: 25,
	})
	if err != nil {
		t.Fatalf("ListUsefulnessFeedback() error = %v", err)
	}
	if len(listed) != 1 || listed[0].ID != "feedback_1" || listed[0].Subjects[0].ExpectedRecallTarget.Kind != memory.ExpectedRecallTargetEvent {
		t.Fatalf("listed = %+v, want feedback with expected-recall subject", listed)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("ExpectationsWereMet() error = %v", err)
	}
}

func TestRepositoryCreateUsefulnessFeedbackIsIdempotentWithExistingSubject(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatalf("pgxmock.NewPool() error = %v", err)
	}
	defer mock.Close()

	scope := memory.Scope{Tenant: "tenant-a", Project: "project-a", Namespace: "namespace-a"}
	now := time.Date(2026, 7, 11, 11, 30, 0, 0, time.UTC)
	feedback := memory.UsefulnessFeedback{
		ID:            "feedback_retry",
		Scope:         scope,
		Type:          memory.UsefulnessFeedbackTypeUseful,
		SourceSurface: memory.UsefulnessFeedbackSourceSession,
		TaskEvaluationID: "task_eval_1",
		Subjects: []memory.UsefulnessFeedbackSubject{{
			Kind: memory.UsefulnessFeedbackSubjectMemory,
			ID:   "mem_1",
		}},
		Actor:          "agent-a",
		Reason:         "helped",
		IdempotencyKey: "session_1:turn_1:mem_1:useful",
		CreatedAt:      now,
	}

	mock.ExpectBegin()
	mock.ExpectQuery("INSERT INTO usefulness_feedback").
		WithArgs(feedback.ID, scope.Tenant, scope.Project, scope.Namespace, feedback.Type, feedback.SourceSurface, feedback.TaskEvaluationID, feedback.Actor, feedback.Reason, feedback.IdempotencyKey, []byte(`{}`), now).
		WillReturnRows(usefulnessFeedbackRows().AddRow("feedback_existing", scope.Tenant, scope.Project, scope.Namespace, feedback.Type, feedback.SourceSurface, feedback.TaskEvaluationID, feedback.Actor, feedback.Reason, feedback.IdempotencyKey, []byte(`{}`), nil, nil, nil, now))
	mock.ExpectQuery("INSERT INTO usefulness_feedback_subjects").
		WithArgs("feedback_existing", scope.Tenant, scope.Project, scope.Namespace, memory.UsefulnessFeedbackSubjectMemory, "mem_1", nil, nil, nil).
		WillReturnRows(usefulnessFeedbackSubjectRows())
	mock.ExpectQuery("SELECT[\\s\\S]*FROM usefulness_feedback_subjects").
		WithArgs([]string{"feedback_existing"}, scope.Tenant, scope.Project, scope.Namespace).
		WillReturnRows(usefulnessFeedbackSubjectRows().AddRow("feedback_existing", scope.Tenant, scope.Project, scope.Namespace, memory.UsefulnessFeedbackSubjectMemory, "mem_1", nil, nil, nil))
	mock.ExpectCommit()

	repo := NewRepository(mock)
	created, err := repo.CreateUsefulnessFeedback(context.Background(), feedback)
	if err != nil {
		t.Fatalf("CreateUsefulnessFeedback() retry error = %v", err)
	}
	if created.ID != "feedback_existing" || len(created.Subjects) != 1 || created.Subjects[0].ID != "mem_1" {
		t.Fatalf("created = %+v, want existing idempotent feedback with existing subject", created)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("ExpectationsWereMet() error = %v", err)
	}
}

func TestRepositorySupersedeUsefulnessFeedbackWritesAuditRecord(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatalf("pgxmock.NewPool() error = %v", err)
	}
	defer mock.Close()

	scope := memory.Scope{Tenant: "tenant-a", Project: "project-a", Namespace: "namespace-a"}
	now := time.Date(2026, 7, 11, 12, 0, 0, 0, time.UTC)

	mock.ExpectBegin()
	mock.ExpectExec("UPDATE usefulness_feedback").
		WithArgs(scope.Tenant, scope.Project, scope.Namespace, "feedback_1", "operator-a", "incorrect signal", now).
		WillReturnResult(pgxmock.NewResult("UPDATE", 1))
	mock.ExpectExec("INSERT INTO usefulness_feedback_supersessions").
		WithArgs(scope.Tenant, scope.Project, scope.Namespace, "feedback_1", "operator-a", "incorrect signal", now).
		WillReturnResult(pgxmock.NewResult("INSERT", 1))
	mock.ExpectCommit()

	repo := NewRepository(mock)
	if err := repo.SupersedeUsefulnessFeedback(context.Background(), memory.SupersedeUsefulnessFeedbackInput{
		Scope:        scope,
		FeedbackID:   "feedback_1",
		Actor:        "operator-a",
		Reason:       "incorrect signal",
		SupersededAt: now,
	}); err != nil {
		t.Fatalf("SupersedeUsefulnessFeedback() error = %v", err)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("ExpectationsWereMet() error = %v", err)
	}
}

func usefulnessFeedbackRows() *pgxmock.Rows {
	return pgxmock.NewRows([]string{
		"id", "tenant", "project", "namespace", "feedback_type", "source_surface", "task_evaluation_id", "actor", "reason", "idempotency_key", "metadata",
		"superseded_at", "superseded_by_actor", "superseded_by_reason", "created_at",
	})
}

func usefulnessFeedbackSubjectRows() *pgxmock.Rows {
	return pgxmock.NewRows([]string{
		"feedback_id", "tenant", "project", "namespace", "subject_kind", "subject_id", "expected_recall_kind", "expected_recall_id", "opaque_token",
	})
}
