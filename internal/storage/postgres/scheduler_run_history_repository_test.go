package postgres

import (
	"context"
	"testing"
	"time"

	"github.com/FelixSeptem/stele/internal/jobs"
	"github.com/FelixSeptem/stele/internal/memory"
	pgxmock "github.com/pashagolub/pgxmock/v4"
)

func TestRepositoryRecordsSchedulerRunAttemptAndSummary(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()
	repo := &Repository{db: mock}
	now := time.Date(2026, 10, 2, 10, 0, 0, 0, time.UTC)
	attempt := jobs.SchedulerRunAttempt{
		RunKey: "maintenance:run-1", JobClass: "projection_refresh",
		Scope:         memory.Scope{Tenant: "t", Project: "p", Namespace: "n"},
		CadenceWindow: now, Attempt: 1, State: jobs.SchedulerRunCompleted,
		Disposition: jobs.MaintenanceDispositionCompleted, Recovery: jobs.SchedulerRecoveryNone,
		ObservedAt: now, FinishedAt: now,
	}
	mock.ExpectExec("INSERT INTO scheduler_run_summaries").WithArgs(
		pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg(),
		pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg(),
		pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg(),
	).WillReturnResult(pgxmock.NewResult("INSERT", 1))
	mock.ExpectExec("INSERT INTO scheduler_run_attempts").WithArgs(
		pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg(),
		pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg(),
		pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg(),
	).WillReturnResult(pgxmock.NewResult("INSERT", 1))
	if err := repo.RecordSchedulerRunAttempt(context.Background(), attempt); err != nil {
		t.Fatalf("RecordSchedulerRunAttempt() error = %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestRepositoryRejectsInvalidSchedulerRunCursor(t *testing.T) {
	mock, _ := pgxmock.NewPool()
	defer mock.Close()
	repo := &Repository{db: mock}
	_, err := repo.ListSchedulerRunHistory(context.Background(), jobs.SchedulerRunHistoryQuery{
		Scope: memory.Scope{Tenant: "t", Project: "p", Namespace: "n"}, Cursor: "not-base64",
	})
	if err == nil {
		t.Fatal("invalid scheduler cursor unexpectedly accepted")
	}
}
