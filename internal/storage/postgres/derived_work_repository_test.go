package postgres

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/FelixSeptem/stele/internal/memory"
	"github.com/FelixSeptem/stele/internal/workqueue"
	"github.com/jackc/pgx/v5/pgconn"
	pgxmock "github.com/pashagolub/pgxmock/v4"
)

func TestRepositoryEnqueueDerivedWorkReturnsExistingIdentityOnDuplicate(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()
	now := time.Date(2026, 10, 2, 12, 0, 0, 0, time.UTC)
	input := workqueue.EnqueueInput{DerivedWorkInput: workqueue.DerivedWorkInput{Scope: memory.Scope{Tenant: "t", Project: "p", Namespace: "n"}, Kind: workqueue.WorkKindReflection, Watermark: "wm", Idempotency: "key", Reference: "run"}, MaxAttempts: 3, Now: now, DetailExpiresAt: now.Add(time.Hour)}
	mock.ExpectQuery("INSERT INTO derived_work_items").WithArgs(input.WorkKey(), "reflection", "t", "p", "n", "wm", "key", "run", 3, now, input.DetailExpiresAt).WillReturnRows(pgxmock.NewRows([]string{"id", "work_key", "kind", "tenant", "project", "namespace", "watermark", "idempotency_key", "reference", "state", "attempt_count", "max_attempts", "lease_owner", "lease_until", "next_attempt_at", "failure_category", "loss_disposition", "created_at", "updated_at", "terminal_at", "detail_expires_at"}).AddRow("work-1", input.WorkKey(), "reflection", "t", "p", "n", "wm", "key", "run", "queued", 0, 3, nil, nil, now, nil, "none", now, now, nil, input.DetailExpiresAt))
	item, err := NewRepository(mock).EnqueueDerivedWork(context.Background(), input)
	if err != nil {
		t.Fatal(err)
	}
	if item.ID != "work-1" || item.WorkKey != input.WorkKey() {
		t.Fatalf("item = %+v", item)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestRepositoryRenewDerivedWorkLeaseRejectsStaleOwner(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()
	now := time.Date(2026, 10, 2, 12, 0, 0, 0, time.UTC)
	mock.ExpectExec("UPDATE derived_work_items").WithArgs("work-1", "t", "p", "n", "worker-a", now, now.Add(time.Minute)).WillReturnResult(pgconn.NewCommandTag("UPDATE 0"))
	err = NewRepository(mock).RenewDerivedWorkLease(context.Background(), workqueue.RenewLeaseInput{Scope: memory.Scope{Tenant: "t", Project: "p", Namespace: "n"}, WorkID: "work-1", WorkerID: "worker-a", RenewedAt: now, LeaseUntil: now.Add(time.Minute)})
	if !errors.Is(err, workqueue.ErrLeaseOwnershipLost) {
		t.Fatalf("error = %v, want lease ownership lost", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestRepositoryCheckpointDerivedWorkRejectsLostLease(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()
	now := time.Date(2026, 10, 2, 12, 0, 0, 0, time.UTC)
	mock.ExpectExec("WITH advanced AS").WithArgs("work-1", "t", "p", "n", "worker-a", int64(1), int64(10), "wm-1", now).WillReturnResult(pgconn.NewCommandTag("INSERT 0"))
	err = NewRepository(mock).CheckpointDerivedWork(context.Background(), workqueue.CheckpointInput{Scope: memory.Scope{Tenant: "t", Project: "p", Namespace: "n"}, WorkID: "work-1", WorkerID: "worker-a", Sequence: 1, ProcessedOffset: 10, SourceWatermark: "wm-1", CommittedAt: now})
	if !errors.Is(err, workqueue.ErrLeaseOwnershipLost) {
		t.Fatalf("error = %v, want lease ownership lost", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}
