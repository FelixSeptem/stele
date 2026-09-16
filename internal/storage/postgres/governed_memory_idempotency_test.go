package postgres

import (
	"context"
	"testing"
	"time"

	"github.com/FelixSeptem/stele/internal/memory"
	"github.com/jackc/pgx/v5"
	pgxmock "github.com/pashagolub/pgxmock/v4"
)

func TestAppendMemoryIntentRejectsConflictingIdempotencyFingerprint(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()
	scope := memory.Scope{Tenant: "tenant-a", Project: "project-a", Namespace: "namespace-a"}
	record := memory.MemoryIntentRecord{ID: "intent-new", Scope: scope, Type: memory.MemoryIntentRemember, Actor: "agent", Reason: "remember", RequestID: "request-1", OperationID: "operation-1", IdempotencyKey: "idem-1", Content: "new content", Status: memory.MemoryIntentStatusAccepted, CreatedAt: time.Now().UTC()}
	mock.ExpectQuery("INSERT INTO memory_intents").WithArgs(pgxmock.AnyArg(), scope.Tenant, scope.Project, scope.Namespace, record.Type, record.Actor, record.Reason, pgxmock.AnyArg(), record.RequestID, record.OperationID, record.IdempotencyKey, pgxmock.AnyArg(), record.TargetMemoryID, record.TargetVersion, pgxmock.AnyArg(), record.Status, record.CreatedAt).WillReturnError(pgx.ErrNoRows)
	mock.ExpectQuery("SELECT request_fingerprint").WithArgs(scope.Tenant, scope.Project, scope.Namespace, record.IdempotencyKey, record.OperationID).WillReturnRows(pgxmock.NewRows([]string{"request_fingerprint", "id", "tenant", "project", "namespace", "intent_type", "actor", "reason", "provenance", "request_id", "operation_id", "idempotency_key", "target_memory_id", "target_version", "payload", "status", "created_at"}).AddRow("different-fingerprint", "intent-old", scope.Tenant, scope.Project, scope.Namespace, record.Type, record.Actor, record.Reason, []byte(`{}`), record.RequestID, record.OperationID, record.IdempotencyKey, nil, nil, []byte(`{"content":"old content"}`), record.Status, record.CreatedAt))
	_, err = NewRepository(mock).AppendMemoryIntent(context.Background(), record)
	if err != memory.ErrIdempotencyConflict {
		t.Fatalf("AppendMemoryIntent() error = %v, want %v", err, memory.ErrIdempotencyConflict)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}
