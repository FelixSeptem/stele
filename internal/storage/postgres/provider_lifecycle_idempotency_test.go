package postgres

import (
	"context"
	"testing"

	"github.com/FelixSeptem/stele/internal/memory"
	"github.com/FelixSeptem/stele/internal/provider"
	pgxmock "github.com/pashagolub/pgxmock/v4"
)

func TestRepositoryClaimLifecycleReturnsClaimed(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()
	c := provider.LifecycleClaim{Scope: memory.Scope{Tenant: "t", Project: "p", Namespace: "n"}, PrincipalID: "principal", IdempotencyKey: "key", RequestFingerprint: "fp"}
	mock.ExpectQuery("INSERT INTO provider_lifecycle_operations").WithArgs("t", "p", "n", "principal", "key", "fp").WillReturnRows(pgxmock.NewRows([]string{"status", "outcome", "request_fingerprint", "inserted"}).AddRow("pending", []byte(`{}`), "fp", true))
	got, err := NewRepository(mock).ClaimLifecycle(context.Background(), c)
	if err != nil || got.Disposition != provider.LifecycleClaimed {
		t.Fatalf("claim=%+v err=%v", got, err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestRepositoryClaimLifecycleDetectsConflictAndPending(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()
	c := provider.LifecycleClaim{Scope: memory.Scope{Tenant: "t", Project: "p", Namespace: "n"}, PrincipalID: "principal", IdempotencyKey: "key", RequestFingerprint: "fp"}
	mock.ExpectQuery("INSERT INTO provider_lifecycle_operations").WithArgs("t", "p", "n", "principal", "key", "fp").WillReturnRows(pgxmock.NewRows([]string{"status", "outcome", "request_fingerprint", "inserted"}).AddRow("pending", []byte(`{}`), "other", false))
	if _, err := NewRepository(mock).ClaimLifecycle(context.Background(), c); err == nil {
		t.Fatal("expected fingerprint conflict")
	}
	mock.ExpectQuery("INSERT INTO provider_lifecycle_operations").WithArgs("t", "p", "n", "principal", "key", "fp").WillReturnRows(pgxmock.NewRows([]string{"status", "outcome", "request_fingerprint", "inserted"}).AddRow("pending", []byte(`{}`), "fp", false))
	got, err := NewRepository(mock).ClaimLifecycle(context.Background(), c)
	if err != nil || got.Disposition != provider.LifecycleInProgress {
		t.Fatalf("pending=%+v err=%v", got, err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestRepositoryCompleteLifecycleUpdatesClaim(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()
	c := provider.LifecycleClaim{Scope: memory.Scope{Tenant: "t", Project: "p", Namespace: "n"}, PrincipalID: "principal", IdempotencyKey: "key", RequestFingerprint: "fp"}
	mock.ExpectExec("UPDATE provider_lifecycle_operations").WithArgs("t", "p", "n", "principal", "key", pgxmock.AnyArg(), "fp").WillReturnResult(pgxmock.NewResult("UPDATE", 1))
	if err := NewRepository(mock).CompleteLifecycle(context.Background(), c, provider.OperationOutcome{}); err != nil {
		t.Fatal(err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}
