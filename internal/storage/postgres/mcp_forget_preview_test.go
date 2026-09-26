package postgres

import (
	"context"
	"testing"
	"time"

	"github.com/FelixSeptem/stele/internal/mcp"
	"github.com/FelixSeptem/stele/internal/memory"
	pgxmock "github.com/pashagolub/pgxmock/v4"
)

func TestSaveForgetPreviewPersistsOnlyBoundedReviewManifest(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()
	scope := memory.Scope{Tenant: "t", Project: "p", Namespace: "n"}
	preview := mcp.ForgetPreviewRecord{ID: "fp_1", Principal: "principal", Scope: scope, MemoryIDs: []string{"m1", "m2"}, ExpiresAt: time.Now().UTC().Add(10 * time.Minute)}
	mock.ExpectExec("WITH expired AS").WillReturnResult(pgxmock.NewResult("DELETE", 0))
	mock.ExpectExec("INSERT INTO mcp_forget_previews").WithArgs(preview.ID, preview.Principal, scope.Tenant, scope.Project, scope.Namespace, pgxmock.AnyArg(), preview.ExpiresAt).WillReturnResult(pgxmock.NewResult("INSERT", 1))
	if err := NewRepository(mock).SaveForgetPreview(context.Background(), preview); err != nil {
		t.Fatalf("SaveForgetPreview() error = %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestLoadForgetPreviewRestoresDurableManifest(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()
	expires := time.Now().UTC().Add(10 * time.Minute)
	mock.ExpectQuery("SELECT preview_id,principal_id").WithArgs("fp_1").WillReturnRows(pgxmock.NewRows([]string{"preview_id", "principal_id", "tenant", "project", "namespace", "memory_ids", "expires_at"}).AddRow("fp_1", "principal", "t", "p", "n", []byte(`["m1","m2"]`), expires))
	got, err := NewRepository(mock).LoadForgetPreview(context.Background(), "fp_1")
	if err != nil {
		t.Fatalf("LoadForgetPreview() error = %v", err)
	}
	if got.Principal != "principal" || got.Scope != (memory.Scope{Tenant: "t", Project: "p", Namespace: "n"}) || len(got.MemoryIDs) != 2 || got.ExpiresAt != expires {
		t.Fatalf("loaded preview = %+v, want original review manifest", got)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestClaimForgetApplyReplaysCompletedOutcomeAndRejectsConflict(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()
	claim := mcp.ForgetApplyClaim{PrincipalID: "principal", Scope: memory.Scope{Tenant: "t", Project: "p", Namespace: "n"}, IdempotencyKey: "key", RequestFingerprint: "fp", ClaimID: "claim-new"}
	responseJSON := []byte(`{"preview_id":"fp_1","applied_ids":["m1"]}`)
	mock.ExpectQuery("INSERT INTO mcp_forget_apply_operations").WithArgs("t", "p", "n", "principal", "key", "fp", "claim-new").WillReturnRows(pgxmock.NewRows([]string{"status", "request_fingerprint", "claim_id", "outcome"}).AddRow("completed", "fp", "claim-old", responseJSON))
	got, err := NewRepository(mock).ClaimForgetApply(context.Background(), claim)
	if err != nil || got.Disposition != mcp.ForgetApplyReplayed || got.Response.PreviewID != "fp_1" || len(got.Response.AppliedIDs) != 1 {
		t.Fatalf("replayed claim = %+v, err=%v", got, err)
	}

	claim.RequestFingerprint = "different"
	mock.ExpectQuery("INSERT INTO mcp_forget_apply_operations").WithArgs("t", "p", "n", "principal", "key", "different", "claim-new").WillReturnRows(pgxmock.NewRows([]string{"status", "request_fingerprint", "claim_id", "outcome"}).AddRow("completed", "fp", "claim-old", responseJSON))
	got, err = NewRepository(mock).ClaimForgetApply(context.Background(), claim)
	if err != nil || got.Disposition != mcp.ForgetApplyConflict {
		t.Fatalf("conflicting claim = %+v, err=%v", got, err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestClaimForgetApplyReportsPendingClaimAsRetryableInProgress(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()
	claim := mcp.ForgetApplyClaim{PrincipalID: "principal", Scope: memory.Scope{Tenant: "t", Project: "p", Namespace: "n"}, IdempotencyKey: "key", RequestFingerprint: "fp", ClaimID: "claim-new"}
	mock.ExpectQuery("INSERT INTO mcp_forget_apply_operations").WithArgs("t", "p", "n", "principal", "key", "fp", "claim-new").WillReturnRows(pgxmock.NewRows(nil))
	mock.ExpectQuery("SELECT status,request_fingerprint").WithArgs("t", "p", "n", "principal", "key").WillReturnRows(pgxmock.NewRows([]string{"status", "request_fingerprint", "claim_id", "outcome"}).AddRow("pending", "fp", "claim-active", []byte(`{}`)))
	got, err := NewRepository(mock).ClaimForgetApply(context.Background(), claim)
	if err != nil || got.Disposition != mcp.ForgetApplyInProgress {
		t.Fatalf("pending claim = %+v, err=%v", got, err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestCompleteAndReleaseForgetApplyUseCurrentClaimToken(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()
	claim := mcp.ForgetApplyClaim{PrincipalID: "principal", Scope: memory.Scope{Tenant: "t", Project: "p", Namespace: "n"}, IdempotencyKey: "key", RequestFingerprint: "fp", ClaimID: "claim-current"}
	mock.ExpectExec("UPDATE mcp_forget_apply_operations").WithArgs("t", "p", "n", "principal", "key", "claim-current", pgxmock.AnyArg(), "fp").WillReturnResult(pgxmock.NewResult("UPDATE", 1))
	if err := NewRepository(mock).CompleteForgetApply(context.Background(), claim, mcp.ForgetApplyResponse{PreviewID: "fp_1", AppliedIDs: []string{"m1"}}); err != nil {
		t.Fatalf("CompleteForgetApply() error = %v", err)
	}
	mock.ExpectExec("DELETE FROM mcp_forget_apply_operations").WithArgs("t", "p", "n", "principal", "key", "claim-current", "fp").WillReturnResult(pgxmock.NewResult("DELETE", 1))
	if err := NewRepository(mock).ReleaseForgetApply(context.Background(), claim); err != nil {
		t.Fatalf("ReleaseForgetApply() error = %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}
