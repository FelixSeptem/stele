package postgres

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/FelixSeptem/stele/internal/assurance"
	"github.com/FelixSeptem/stele/internal/memory"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	pgxmock "github.com/pashagolub/pgxmock/v4"
)

func TestRepositoryCollectsIngestionFreshnessFromExactScopeRecords(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()
	scope := memory.Scope{Tenant: "tenant-a", Project: "project-a", Namespace: "namespace-a"}
	eventID := uuid.NewString()
	base := time.Date(2026, 10, 30, 8, 0, 0, 0, time.UTC)
	processedAt, canonicalActiveAt, publicReadAt := base.Add(time.Second), base.Add(2*time.Second), base.Add(3*time.Second)
	mock.ExpectQuery("FROM raw_events").WithArgs(scope.Tenant, scope.Project, scope.Namespace, eventID).WillReturnRows(
		pgxmock.NewRows([]string{"admitted_at", "processed_at", "failed_at", "exhausted_at", "canonical_active_at", "canonical_hidden", "source_visible", "public_read_at"}).
			AddRow(base, &processedAt, nil, nil, &canonicalActiveAt, false, true, &publicReadAt),
	)
	got, err := NewRepository(mock).CollectIngestionFreshness(context.Background(), scope, eventID, base.Add(4*time.Second))
	if err != nil {
		t.Fatal(err)
	}
	if got.Scope != scope || got.ReportKey != eventID || len(got.Stages) != 4 {
		t.Fatalf("collected input=%+v, want scoped four-stage evidence", got)
	}
	for _, stage := range got.Stages {
		if stage.Disposition != assurance.IngestionDispositionComplete {
			t.Fatalf("stage=%+v, want complete", stage)
		}
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestRepositoryCollectsIngestionFreshnessClassifiesIncompleteAndProtectedStates(t *testing.T) {
	scope := memory.Scope{Tenant: "tenant-a", Project: "project-a", Namespace: "namespace-a"}
	eventID := uuid.NewString()
	base := time.Date(2026, 10, 30, 8, 0, 0, 0, time.UTC)
	processedAt, canonicalActiveAt := base.Add(time.Second), base.Add(2*time.Second)
	cases := []struct {
		name  string
		row   []any
		wants []assurance.IngestionStageDisposition
	}{
		{name: "pending governance", row: []any{base, nil, nil, nil, nil, false, false, nil}, wants: []assurance.IngestionStageDisposition{assurance.IngestionDispositionComplete, assurance.IngestionDispositionPending, assurance.IngestionDispositionUnavailable, assurance.IngestionDispositionUnavailable}},
		{name: "retrying governance", row: []any{base, nil, &processedAt, nil, nil, false, false, nil}, wants: []assurance.IngestionStageDisposition{assurance.IngestionDispositionComplete, assurance.IngestionDispositionRetrying, assurance.IngestionDispositionUnavailable, assurance.IngestionDispositionUnavailable}},
		{name: "exhausted governance", row: []any{base, nil, nil, &processedAt, nil, false, false, nil}, wants: []assurance.IngestionStageDisposition{assurance.IngestionDispositionComplete, assurance.IngestionDispositionExhausted, assurance.IngestionDispositionUnavailable, assurance.IngestionDispositionUnavailable}},
		{name: "hidden canonical", row: []any{base, &processedAt, nil, nil, nil, true, false, nil}, wants: []assurance.IngestionStageDisposition{assurance.IngestionDispositionComplete, assurance.IngestionDispositionComplete, assurance.IngestionDispositionHidden, assurance.IngestionDispositionUnavailable}},
		{name: "ineligible source", row: []any{base, &processedAt, nil, nil, &canonicalActiveAt, false, false, nil}, wants: []assurance.IngestionStageDisposition{assurance.IngestionDispositionComplete, assurance.IngestionDispositionComplete, assurance.IngestionDispositionComplete, assurance.IngestionDispositionIneligible}},
		{name: "no public read", row: []any{base, &processedAt, nil, nil, &canonicalActiveAt, false, true, nil}, wants: []assurance.IngestionStageDisposition{assurance.IngestionDispositionComplete, assurance.IngestionDispositionComplete, assurance.IngestionDispositionComplete, assurance.IngestionDispositionUnavailable}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			mock, err := pgxmock.NewPool()
			if err != nil {
				t.Fatal(err)
			}
			defer mock.Close()
			mock.ExpectQuery("FROM raw_events").WithArgs(scope.Tenant, scope.Project, scope.Namespace, eventID).WillReturnRows(
				pgxmock.NewRows([]string{"admitted_at", "processed_at", "failed_at", "exhausted_at", "canonical_active_at", "canonical_hidden", "source_visible", "public_read_at"}).AddRow(tc.row...),
			)
			got, err := NewRepository(mock).CollectIngestionFreshness(context.Background(), scope, eventID, base.Add(4*time.Second))
			if err != nil {
				t.Fatal(err)
			}
			for index, want := range tc.wants {
				if got.Stages[index].Disposition != want {
					t.Fatalf("stage %d disposition=%q, want %q", index, got.Stages[index].Disposition, want)
				}
			}
			if err := mock.ExpectationsWereMet(); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestRepositoryCollectsIngestionFreshnessDoesNotDiscloseForeignFixture(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()
	scope := memory.Scope{Tenant: "tenant-b", Project: "project-a", Namespace: "namespace-a"}
	eventID := uuid.NewString()
	mock.ExpectQuery("FROM raw_events").WithArgs(scope.Tenant, scope.Project, scope.Namespace, eventID).WillReturnError(pgx.ErrNoRows)
	got, err := NewRepository(mock).CollectIngestionFreshness(context.Background(), scope, eventID, time.Now().UTC())
	if err != nil {
		t.Fatal(err)
	}
	for _, stage := range got.Stages {
		if stage.Disposition != assurance.IngestionDispositionUnavailable {
			t.Fatalf("foreign fixture disposition=%q, want unavailable", stage.Disposition)
		}
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestRepositoryStoresIngestionFreshnessEvidenceIdempotently(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()
	scope := memory.Scope{Tenant: "tenant-a", Project: "project-a", Namespace: "namespace-a"}
	now := time.Date(2026, 10, 30, 8, 0, 0, 0, time.UTC)
	report := ingestionFreshnessReportForRepositoryTest(scope, now)
	evidence := assurance.IngestionFreshnessEvidence{Scope: scope, FixtureHash: report.FixtureHash, IdentityHash: report.IdentityHash, Report: report, ObservedAt: now, CreatedAt: now}
	reportJSON, err := json.Marshal(report)
	if err != nil {
		t.Fatal(err)
	}
	mock.ExpectQuery("INSERT INTO assurance_ingestion_freshness_evidence").WithArgs(scope.Tenant, scope.Project, scope.Namespace, evidence.FixtureHash, evidence.IdentityHash, reportJSON, now, now).WillReturnRows(
		pgxmock.NewRows([]string{"tenant", "project", "namespace", "fixture_hash", "identity_hash", "report", "observed_at", "created_at"}).
			AddRow(scope.Tenant, scope.Project, scope.Namespace, evidence.FixtureHash, evidence.IdentityHash, reportJSON, now, now),
	)
	stored, err := NewRepository(mock).StoreIngestionFreshnessEvidence(context.Background(), evidence)
	if err != nil {
		t.Fatal(err)
	}
	if stored.Report.IdentityHash != evidence.IdentityHash || stored.Scope != scope {
		t.Fatalf("stored evidence=%+v, want preserved redacted evidence", stored)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestRepositoryRejectsConflictingIngestionFreshnessReplay(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()
	scope := memory.Scope{Tenant: "tenant-a", Project: "project-a", Namespace: "namespace-a"}
	now := time.Date(2026, 10, 30, 8, 0, 0, 0, time.UTC)
	report := ingestionFreshnessReportForRepositoryTest(scope, now)
	report.IdentityHash = "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"
	evidence := assurance.IngestionFreshnessEvidence{Scope: scope, FixtureHash: report.FixtureHash, IdentityHash: report.IdentityHash, Report: report, ObservedAt: now, CreatedAt: now}
	reportJSON, _ := json.Marshal(report)
	priorReport := report
	priorReport.IdentityHash = "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	priorReportJSON, _ := json.Marshal(priorReport)
	mock.ExpectQuery("INSERT INTO assurance_ingestion_freshness_evidence").WithArgs(scope.Tenant, scope.Project, scope.Namespace, evidence.FixtureHash, evidence.IdentityHash, reportJSON, now, now).WillReturnError(pgx.ErrNoRows)
	mock.ExpectQuery("SELECT[\\s\\S]*FROM assurance_ingestion_freshness_evidence").WithArgs(scope.Tenant, scope.Project, scope.Namespace, evidence.FixtureHash).WillReturnRows(
		pgxmock.NewRows([]string{"tenant", "project", "namespace", "fixture_hash", "identity_hash", "report", "observed_at", "created_at"}).
			AddRow(scope.Tenant, scope.Project, scope.Namespace, evidence.FixtureHash, "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", priorReportJSON, now, now),
	)
	if _, err := NewRepository(mock).StoreIngestionFreshnessEvidence(context.Background(), evidence); err != assurance.ErrIngestionFreshnessReplayConflict {
		t.Fatalf("conflicting replay error=%v, want bounded conflict", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestRepositoryRecordProviderMemoryReadRejectsInvalidIDsAndIgnoresInactiveMemory(t *testing.T) {
	scope := memory.Scope{Tenant: "tenant-a", Project: "project-a", Namespace: "namespace-a"}
	if err := NewRepository(nil).RecordProviderMemoryRead(context.Background(), scope, []string{"not-a-uuid"}, time.Now().UTC()); err == nil {
		t.Fatal("invalid memory ID was accepted")
	}
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()
	memoryID := uuid.NewString()
	mock.ExpectExec("INSERT INTO provider_memory_read_observations").WithArgs(scope.Tenant, scope.Project, scope.Namespace, []string{memoryID}, pgxmock.AnyArg()).WillReturnResult(pgxmock.NewResult("INSERT", 0))
	if err := NewRepository(mock).RecordProviderMemoryRead(context.Background(), scope, []string{memoryID}, time.Now().UTC()); err != nil {
		t.Fatal(err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestIngestionFreshnessMigrationDefinesExactScopeAndReplayKeys(t *testing.T) {
	up, err := migrationFS.ReadFile("migrations/0041_ingestion_freshness_evidence.up.sql")
	if err != nil {
		t.Fatal(err)
	}
	for _, required := range []string{"provider_memory_read_observations", "assurance_ingestion_freshness_evidence", "tenant", "project", "namespace", "fixture_hash", "identity_hash", "PRIMARY KEY (tenant, project, namespace, fixture_hash)", "FOREIGN KEY (memory_id) REFERENCES canonical_memories(id)"} {
		if !strings.Contains(string(up), required) {
			t.Errorf("migration missing %q", required)
		}
	}
	down, err := migrationFS.ReadFile("migrations/0041_ingestion_freshness_evidence.down.sql")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(down), "disable the capability and retain evidence") {
		t.Fatal("downgrade must preserve freshness audit history")
	}
}

func ingestionFreshnessReportForRepositoryTest(scope memory.Scope, now time.Time) assurance.IngestionFreshnessReport {
	report, err := assurance.EvaluateIngestionFreshness(assurance.IngestionFreshnessInput{
		Scope: scope, ReportKey: uuid.NewString(), ObservedAt: now,
		Policy: assurance.IngestionFreshnessPolicy{ContractVersion: assurance.IngestionFreshnessVersion, AdmissionWindow: time.Hour, GovernanceWindow: time.Hour, RetrievalWindow: time.Hour},
		Stages: []assurance.IngestionFreshnessStage{{Kind: assurance.IngestionStageAdmitted, Disposition: assurance.IngestionDispositionComplete, At: now.Add(-3 * time.Second)}, {Kind: assurance.IngestionStageGovernanceCompleted, Disposition: assurance.IngestionDispositionComplete, At: now.Add(-2 * time.Second)}, {Kind: assurance.IngestionStageCanonicalActive, Disposition: assurance.IngestionDispositionComplete, At: now.Add(-time.Second)}, {Kind: assurance.IngestionStagePublicRetrievable, Disposition: assurance.IngestionDispositionComplete, At: now}},
	})
	if err != nil {
		panic(err)
	}
	return report
}
