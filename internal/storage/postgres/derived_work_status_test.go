package postgres

import (
	"context"
	"testing"
	"time"

	"github.com/FelixSeptem/stele/internal/memory"
	pgxmock "github.com/pashagolub/pgxmock/v4"
)

func TestRepositoryReadDerivedWorkStatusUsesExactScopeAndBoundedCounters(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()
	now := time.Date(2026, 10, 2, 12, 0, 0, 0, time.UTC)
	scope := memory.Scope{Tenant: "t", Project: "p", Namespace: "n"}
	mock.ExpectQuery("SELECT").WithArgs(scope.Tenant, scope.Project, scope.Namespace).WillReturnRows(pgxmock.NewRows([]string{"queued", "running", "retry", "completed", "exhausted", "cancelled", "dropped", "oldest_pending_at"}).AddRow(2, 1, 1, 3, 1, 0, 0, now.Add(-time.Minute)))
	status, err := NewRepository(mock).ReadDerivedWorkStatus(context.Background(), scope, now)
	if err != nil {
		t.Fatal(err)
	}
	if status.Queued != 2 || status.Running != 1 || status.Retry != 1 || status.Completed != 3 || status.Exhausted != 1 || status.Depth != 4 || !status.OldestPendingAt.Equal(now.Add(-time.Minute)) {
		t.Fatalf("status = %+v", status)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}
