package postgres

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/FelixSeptem/stele/internal/auth"
	"github.com/FelixSeptem/stele/internal/memory"
	"github.com/jackc/pgx/v5"
	"github.com/pashagolub/pgxmock/v4"
)

func TestScopeGrantDisclosureReadsCurrentExactScopeAndDeniesMissing(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()
	scope := memory.Scope{Tenant: "t", Project: "p", Namespace: "n"}
	for _, profile := range []string{auth.DisclosureProfileContent, auth.DisclosureProfileReference} {
		mock.ExpectQuery("SELECT allowed_disclosure_profiles.*FROM access_scope_grants").WithArgs("principal", "t", "p", "n").WillReturnRows(pgxmock.NewRows([]string{"allowed_disclosure_profiles", "default_disclosure_profile", "source_attestation"}).AddRow([]string{profile}, profile, false))
		got, err := NewRepository(mock).ScopeGrantDisclosure(context.Background(), "principal", scope)
		if err != nil || got.DefaultProfile != profile || got.SourceAttestation {
			t.Fatalf("stale or widened grant: %+v %v", got, err)
		}
	}
	mock.ExpectQuery("FROM access_scope_grants").WithArgs("principal", "t", "p", "n").WillReturnError(pgx.ErrNoRows)
	if _, err := NewRepository(mock).ScopeGrantDisclosure(context.Background(), "principal", scope); !errors.Is(err, auth.ErrDisclosureDenied) {
		t.Fatalf("missing grant: %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestUpdateDisclosureGrantIsScopedAtomicAndAudited(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()
	scope := memory.Scope{Tenant: "t", Project: "p", Namespace: "n"}
	e := auth.DisclosureEntitlements{AllowedProfiles: []string{auth.DisclosureProfileReference}, DefaultProfile: auth.DisclosureProfileReference}
	now := time.Now().UTC()
	audit := auth.AuditRecord{ID: "audit", Scope: scope, PrincipalID: "principal", Action: "scope_disclosure_updated", Result: "success", CreatedAt: now, Actor: "admin", Reason: "downgrade"}
	mock.ExpectBegin()
	mock.ExpectQuery("UPDATE access_scope_grants").WithArgs("grant", "principal", "t", "p", "n", e.AllowedProfiles, e.DefaultProfile, false).WillReturnRows(pgxmock.NewRows([]string{"id"}).AddRow("grant"))
	mock.ExpectExec("INSERT INTO access_audit_records").WithArgs("audit", "principal", "t", "p", "n", audit.Action, audit.Result, now, "admin", "downgrade").WillReturnResult(pgxmock.NewResult("INSERT", 1))
	mock.ExpectCommit()
	if err := NewRepository(mock).UpdateScopeGrantDisclosure(context.Background(), scope, "principal", "grant", e, audit); err != nil {
		t.Fatal(err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}
