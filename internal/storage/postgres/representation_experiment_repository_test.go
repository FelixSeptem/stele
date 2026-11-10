package postgres

import (
	"bytes"
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/FelixSeptem/stele/internal/memory"
	"github.com/FelixSeptem/stele/internal/retrieval"
	pgxmock "github.com/pashagolub/pgxmock/v4"
)

func TestRepositoryPersistsRepresentationExperimentReportsByExactScope(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()
	scope := memory.Scope{Tenant: "tenant-a", Project: "benchmark-representations", Namespace: "namespace-a"}
	report := representationRepositoryReport(t, scope)
	payload, err := retrieval.MarshalRepresentationEvaluationReport(report)
	if err != nil {
		t.Fatal(err)
	}
	mock.ExpectExec(`INSERT INTO representation_evaluation_reports`).WithArgs(
		report.RunIdentity, scope.Tenant, scope.Project, scope.Namespace, report.PolicyIdentity,
		report.FixtureIdentity, report.Baseline, report.Candidate, report.Mode, report.Verdict,
		report.Comparable, report.RollbackRequired, pgxmock.AnyArg(), nil,
	).WillReturnResult(pgxmock.NewResult("INSERT", 1))
	mock.ExpectQuery(`(?s)SELECT aggregate.*FROM representation_evaluation_reports.*tenant=\$1.*LIMIT \$4`).
		WithArgs(scope.Tenant, scope.Project, scope.Namespace, 50).
		WillReturnRows(pgxmock.NewRows([]string{"aggregate", "expires_at"}).AddRow(payload, nil))
	repo := NewRepository(mock)
	if err := repo.CreateRepresentationEvaluationReport(context.Background(), scope, report, nil); err != nil {
		t.Fatal(err)
	}
	reports, err := repo.ListRepresentationEvaluationReports(context.Background(), scope, 50)
	if err != nil || len(reports) != 1 || reports[0].RunIdentity != report.RunIdentity {
		t.Fatalf("reports=%+v err=%v", reports, err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestRepositoryRepresentationExperimentRefusesScopeMismatchAndMalformedAggregate(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()
	scope := memory.Scope{Tenant: "tenant-a", Project: "benchmark-representations", Namespace: "namespace-a"}
	foreign := scope
	foreign.Tenant = "tenant-b"
	report := representationRepositoryReport(t, scope)
	if err := NewRepository(mock).CreateRepresentationEvaluationReport(context.Background(), foreign, report, nil); err == nil {
		t.Fatal("foreign scope accepted")
	}
	mock.ExpectQuery(`(?s)SELECT aggregate.*FROM representation_evaluation_reports.*tenant=\$1.*LIMIT \$4`).
		WithArgs(scope.Tenant, scope.Project, scope.Namespace, 50).
		WillReturnRows(pgxmock.NewRows([]string{"aggregate", "expires_at"}).AddRow([]byte(`{"run_identity":"unsafe"}`), nil))
	if _, err := NewRepository(mock).ListRepresentationEvaluationReports(context.Background(), scope, 50); err == nil {
		t.Fatal("malformed aggregate accepted")
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestRepositoryCleansOnlyExpiredRepresentationExperimentReportsInScope(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()
	scope := memory.Scope{Tenant: "tenant-a", Project: "benchmark-representations", Namespace: "namespace-a"}
	now := time.Date(2026, 11, 10, 12, 0, 0, 0, time.UTC)
	mock.ExpectQuery(`(?s)WITH expired AS.*representation_evaluation_reports.*LIMIT \$5`).
		WithArgs(scope.Tenant, scope.Project, scope.Namespace, now, 100).
		WillReturnRows(pgxmock.NewRows([]string{"count"}).AddRow(int64(2)))
	deleted, err := NewRepository(mock).CleanupRepresentationEvaluationReports(context.Background(), scope, now, 100)
	if err != nil || deleted != 2 {
		t.Fatalf("deleted=%d err=%v", deleted, err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestRepositoryListsOnlyUnexpiredRepresentationReports(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()
	scope := memory.Scope{Tenant: "tenant-a", Project: "benchmark-representations", Namespace: "namespace-a"}
	mock.ExpectQuery(`(?s)SELECT aggregate.*FROM representation_evaluation_reports WHERE tenant=\$1 AND project=\$2 AND namespace=\$3 AND \(expires_at IS NULL OR expires_at > CURRENT_TIMESTAMP\).*LIMIT \$4`).
		WithArgs(scope.Tenant, scope.Project, scope.Namespace, 50).
		WillReturnRows(pgxmock.NewRows([]string{"aggregate"}))
	reports, err := NewRepository(mock).ListRepresentationEvaluationReports(context.Background(), scope, 50)
	if err != nil || len(reports) != 0 {
		t.Fatalf("reports=%+v err=%v", reports, err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestRepositoryPreservesRepresentationReportExpiryWithoutChangingAggregate(t *testing.T) {
	scope := memory.Scope{Tenant: "tenant-a", Project: "benchmark-representations", Namespace: "namespace-a"}
	report := representationRepositoryReport(t, scope)
	payload, err := retrieval.MarshalRepresentationEvaluationReport(report)
	if err != nil {
		t.Fatal(err)
	}
	expiresAt := report.EvaluatedAt.Add(time.Hour)
	for _, tc := range []struct {
		name      string
		expiresAt *time.Time
	}{
		{name: "bounded retention", expiresAt: &expiresAt},
		{name: "no retention expiry"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			mock, err := pgxmock.NewPool()
			if err != nil {
				t.Fatal(err)
			}
			defer mock.Close()
			mock.ExpectQuery(`(?s)SELECT aggregate, expires_at FROM representation_evaluation_reports.*LIMIT \$4`).
				WithArgs(scope.Tenant, scope.Project, scope.Namespace, 50).
				WillReturnRows(pgxmock.NewRows([]string{"aggregate", "expires_at"}).AddRow(payload, nullableTimeValue(tc.expiresAt)))
			reports, err := NewRepository(mock).ListRepresentationEvaluationReports(context.Background(), scope, 50)
			if err != nil || len(reports) != 1 {
				t.Fatalf("reports=%+v err=%v", reports, err)
			}
			gotExpiry := reports[0].PersistenceExpiresAt
			if tc.expiresAt == nil {
				if gotExpiry != nil {
					t.Fatalf("unexpected persistence expiry: %v", gotExpiry)
				}
			} else if gotExpiry == nil || !gotExpiry.Equal(*tc.expiresAt) {
				t.Fatalf("persistence expiry=%v want=%v", gotExpiry, tc.expiresAt)
			}
			readPayload, err := retrieval.MarshalRepresentationEvaluationReport(reports[0])
			if err != nil || !bytes.Equal(readPayload, payload) || reports[0].RunIdentity != report.RunIdentity {
				t.Fatalf("storage metadata changed hashed aggregate: err=%v payload=%s", err, readPayload)
			}
			if err := mock.ExpectationsWereMet(); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestRepositoryRejectsUnrecognizedRepresentationReportPayload(t *testing.T) {
	scope := memory.Scope{Tenant: "tenant-a", Project: "benchmark-representations", Namespace: "namespace-a"}
	payload, err := retrieval.MarshalRepresentationEvaluationReport(representationRepositoryReport(t, scope))
	if err != nil {
		t.Fatal(err)
	}
	var object map[string]json.RawMessage
	if err := json.Unmarshal(payload, &object); err != nil {
		t.Fatal(err)
	}
	object["source_content"] = json.RawMessage(`"unredacted memory"`)
	unknownField, err := json.Marshal(object)
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name    string
		payload []byte
	}{
		{name: "unknown field", payload: unknownField},
		{name: "second document", payload: append(append([]byte(nil), payload...), []byte(` {}`)...)},
	} {
		t.Run(tc.name, func(t *testing.T) {
			mock, err := pgxmock.NewPool()
			if err != nil {
				t.Fatal(err)
			}
			defer mock.Close()
			mock.ExpectQuery(`(?s)SELECT aggregate.*FROM representation_evaluation_reports.*LIMIT \$4`).
				WithArgs(scope.Tenant, scope.Project, scope.Namespace, 50).
				WillReturnRows(pgxmock.NewRows([]string{"aggregate", "expires_at"}).AddRow(tc.payload, nil))
			if reports, err := NewRepository(mock).ListRepresentationEvaluationReports(context.Background(), scope, 50); err == nil || reports != nil {
				t.Fatalf("unsafe stored aggregate accepted: reports=%+v err=%v", reports, err)
			}
			if err := mock.ExpectationsWereMet(); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func representationRepositoryReport(t *testing.T, scope memory.Scope) retrieval.RepresentationEvaluationReport {
	t.Helper()
	report, err := retrieval.EvaluateRepresentation(retrieval.RepresentationEvaluationInput{
		Scope: scope, Policy: retrieval.RepresentationEvaluationPolicy{Version: "representation-evaluation-v1", Baseline: "flat-fusion-v1", Candidate: retrieval.RepresentationCandidateTopicProjectionWithSearch, Mode: retrieval.RepresentationEvaluationModeShadow, Limits: retrieval.RepresentationEvaluationLimits{MaxCandidates: 4, MaxCharacters: 256, MaxTokens: 64, MaxLatencyMS: 100}},
		Family: retrieval.RepresentationTaskFamilyCrossDocument, FixtureVersion: "representation-fixture-v1", SourceChecksum: strings.Repeat("a", 64), QRELChecksum: strings.Repeat("b", 64), SourceWatermark: "watermark-v1", RendererVersion: "renderer-v1", EvaluatedAt: time.Date(2026, 11, 10, 12, 0, 0, 0, time.UTC),
		CriticalFactsExpected: 1, CriticalFactsReturned: 1, ExpectedSources: 1, ReturnedSources: 1, ExpectedCitations: 1, ReturnedCitations: 1, CandidateCount: 1, CharacterCount: 1, TokenCount: 1, LatencyMS: 1, SourceSearchRetained: true, DeterministicReplay: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	return report
}
