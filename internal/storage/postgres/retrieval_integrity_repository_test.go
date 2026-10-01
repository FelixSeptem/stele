package postgres

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/FelixSeptem/stele/internal/evaluation"
	"github.com/FelixSeptem/stele/internal/memory"
	pgxmock "github.com/pashagolub/pgxmock/v4"
)

func retrievalIntegrityScope() memory.Scope {
	return memory.Scope{Tenant: "tenant-a", Project: "project-a", Namespace: "namespace-a"}
}

func retrievalIntegrityIdentity() evaluation.CompatibilityIdentity {
	return evaluation.CompatibilityIdentity{
		FixtureVersion: "fixture-v1", PolicyVersion: "policy-v1", Strategy: "strategy-v1",
		Renderer: "renderer-v1", Provider: "provider-v1", SourceWatermark: "watermark-v1",
	}
}

func retrievalIntegrityReport(t *testing.T, id string, scope memory.Scope) evaluation.IntegrityReport {
	t.Helper()
	report, err := evaluation.EvaluateIntegrity(evaluation.IntegrityInput{
		ID: id, Scope: scope, Identity: retrievalIntegrityIdentity(), Action: "consolidation", ActionSuccess: true,
		Findings:  map[evaluation.FindingCategory]int{evaluation.FindingExpectedRecall: 1},
		CreatedAt: time.Date(2026, 10, 1, 12, 0, 0, 0, time.UTC),
	})
	if err != nil {
		t.Fatal(err)
	}
	return report
}

func TestRepositoryListsIntegrityReportsWithExactScopeAndBoundedLimit(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()

	scope := retrievalIntegrityScope()
	report := retrievalIntegrityReport(t, "integrity-a", scope)
	payload, err := json.Marshal(report.Redacted())
	if err != nil {
		t.Fatal(err)
	}
	mock.ExpectQuery(`SELECT[\s\S]*FROM retrieval_integrity_reports[\s\S]*tenant=\$1[\s\S]*project=\$2[\s\S]*namespace=\$3[\s\S]*LIMIT \$4`).
		WithArgs(scope.Tenant, scope.Project, scope.Namespace, 100).
		WillReturnRows(pgxmock.NewRows([]string{"aggregate"}).AddRow(payload))

	reports, err := NewRepository(mock).ListRetrievalIntegrityReports(context.Background(), scope, 999)
	if err != nil {
		t.Fatalf("ListRetrievalIntegrityReports() error = %v", err)
	}
	if len(reports) != 1 || reports[0].ScopeHash == "" {
		t.Fatalf("reports = %+v, want one redacted scoped report", reports)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestRepositoryRejectsIntegrityReportWithForeignScopeHash(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()

	scope := retrievalIntegrityScope()
	foreign := memory.Scope{Tenant: "tenant-b", Project: "project-b", Namespace: "namespace-b"}
	payload, err := json.Marshal(retrievalIntegrityReport(t, "integrity-b", foreign).Redacted())
	if err != nil {
		t.Fatal(err)
	}
	mock.ExpectQuery(`SELECT[\s\S]*FROM retrieval_integrity_reports[\s\S]*tenant=\$1[\s\S]*project=\$2[\s\S]*namespace=\$3`).
		WithArgs(scope.Tenant, scope.Project, scope.Namespace, 50).
		WillReturnRows(pgxmock.NewRows([]string{"aggregate"}).AddRow(payload))

	if _, err := NewRepository(mock).ListRetrievalIntegrityReports(context.Background(), scope, 50); err == nil {
		t.Fatal("ListRetrievalIntegrityReports() error = nil, want foreign scope hash rejection")
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestRepositoryPersistsHistoricalIntegrityReportsAppendOnly(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()

	scope := retrievalIntegrityScope()
	first := retrievalIntegrityReport(t, "integrity-history-a", scope)
	second := retrievalIntegrityReport(t, "integrity-history-b", scope)
	for _, report := range []evaluation.IntegrityReport{first, second} {
		mock.ExpectExec(`INSERT INTO retrieval_integrity_reports`).
			WithArgs(report.ID, scope.Tenant, scope.Project, scope.Namespace,
				report.Identity.FixtureVersion, report.Identity.PolicyVersion, report.Identity.Strategy,
				report.Identity.Renderer, report.Identity.Provider, report.Identity.SourceWatermark,
				report.Verdict, report.Action, report.ActionSuccess, report.IntegritySuccess,
				pgxmock.AnyArg(), report.CreatedAt, nil).
			WillReturnResult(pgxmock.NewResult("INSERT", 1))
	}
	repo := NewRepository(mock)
	if err := repo.CreateRetrievalIntegrityReport(context.Background(), first, nil); err != nil {
		t.Fatalf("CreateRetrievalIntegrityReport(first) error = %v", err)
	}
	if err := repo.CreateRetrievalIntegrityReport(context.Background(), second, nil); err != nil {
		t.Fatalf("CreateRetrievalIntegrityReport(second) error = %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestRetrievalIntegrityTrajectoryIDSeparatesAggregateDimensions(t *testing.T) {
	scope := retrievalIntegrityScope()
	first, err := evaluation.AggregateTrajectory(evaluation.TrajectoryInput{Scope: scope, Identity: retrievalIntegrityIdentity(), Channel: "lexical", CandidateCount: 3, ExpansionCount: 1, Disposition: "selected", Fallback: "none", Freshness: "fresh", Budget: 100, Latency: time.Millisecond})
	if err != nil {
		t.Fatal(err)
	}
	second, err := evaluation.AggregateTrajectory(evaluation.TrajectoryInput{Scope: scope, Identity: retrievalIntegrityIdentity(), Channel: "semantic", CandidateCount: 3, ExpansionCount: 1, Disposition: "selected", Fallback: "none", Freshness: "fresh", Budget: 100, Latency: time.Millisecond})
	if err != nil {
		t.Fatal(err)
	}
	if retrievalIntegrityTrajectoryID("integrity-report", first) == retrievalIntegrityTrajectoryID("integrity-report", second) {
		t.Fatal("trajectory IDs collide for distinct aggregate dimensions")
	}
}

func TestRepositoryPersistsRetentionOutcomeAndBoundsCleanup(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()

	scope := retrievalIntegrityScope()
	now := time.Date(2026, 10, 1, 13, 0, 0, 0, time.UTC)
	outcome, err := evaluation.BuildRetentionOutcome(evaluation.RetentionInput{ArtifactCategory: "trajectory", Result: "deleted", DeletedCount: 2, ReasonCategory: "expired", At: now})
	if err != nil {
		t.Fatal(err)
	}
	mock.ExpectExec(`INSERT INTO retrieval_integrity_retention_outcomes`).
		WithArgs(pgxmock.AnyArg(), scope.Tenant, scope.Project, scope.Namespace, outcome.ArtifactCategory, outcome.Result, outcome.DeletedCount, outcome.ReasonCategory, outcome.CreatedAt).
		WillReturnResult(pgxmock.NewResult("INSERT", 1))
	mock.ExpectQuery(`WITH expired_candidates AS[\s\S]*LIMIT \$2`).
		WithArgs(now, 1000).
		WillReturnRows(pgxmock.NewRows([]string{"count"}).AddRow(int64(2)))

	repo := NewRepository(mock)
	if err := repo.CreateRetrievalIntegrityRetentionOutcome(context.Background(), scope, outcome); err != nil {
		t.Fatalf("CreateRetrievalIntegrityRetentionOutcome() error = %v", err)
	}
	deleted, err := repo.CleanupRetrievalIntegrityEvidence(context.Background(), now, 9999)
	if err != nil {
		t.Fatalf("CleanupRetrievalIntegrityEvidence() error = %v", err)
	}
	if deleted != 2 {
		t.Fatalf("deleted = %d, want 2", deleted)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}
