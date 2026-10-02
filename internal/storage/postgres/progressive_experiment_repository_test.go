package postgres

import (
	"context"
	"testing"
	"time"

	"github.com/FelixSeptem/stele/internal/memory"
	"github.com/FelixSeptem/stele/internal/retrieval"
	pgxmock "github.com/pashagolub/pgxmock/v4"
)

func TestRepositoryPersistsAndListsProgressiveExperimentReportsByExactScope(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()
	scope := memory.Scope{Tenant: "tenant-a", Project: "project-a", Namespace: "namespace-a"}
	report := progressiveRepositoryReport(scope)
	payload, err := retrieval.MarshalProgressiveExperimentReport(report)
	if err != nil {
		t.Fatal(err)
	}
	mock.ExpectExec(`INSERT INTO progressive_experiment_reports`).WithArgs(
		report.RunIdentity, scope.Tenant, scope.Project, scope.Namespace, report.PolicyIdentity,
		report.BaselineIdentity, report.StrategyIdentity, report.Mode, report.Verdict, report.Eligible,
		report.FallbackCategory, report.RollbackRequired, "", pgxmock.AnyArg(), nil,
	).WillReturnResult(pgxmock.NewResult("INSERT", 1))
	mock.ExpectQuery(`(?s)SELECT aggregate.*FROM progressive_experiment_reports.*tenant=\$1.*LIMIT \$4`).
		WithArgs(scope.Tenant, scope.Project, scope.Namespace, 50).
		WillReturnRows(pgxmock.NewRows([]string{"aggregate"}).AddRow(payload))
	repo := NewRepository(mock)
	if err := repo.CreateProgressiveExperimentReport(context.Background(), scope, report, nil); err != nil {
		t.Fatal(err)
	}
	reports, err := repo.ListProgressiveExperimentReports(context.Background(), scope, 50)
	if err != nil || len(reports) != 1 || reports[0].RunIdentity != report.RunIdentity {
		t.Fatalf("reports=%+v err=%v", reports, err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestRepositoryCleansOnlyExpiredProgressiveExperimentReportsInScope(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()
	scope := memory.Scope{Tenant: "tenant-a", Project: "project-a", Namespace: "namespace-a"}
	now := time.Date(2026, 10, 2, 12, 0, 0, 0, time.UTC)
	mock.ExpectQuery(`(?s)WITH expired AS.*progressive_experiment_reports.*LIMIT \$5`).
		WithArgs(scope.Tenant, scope.Project, scope.Namespace, now, 100).
		WillReturnRows(pgxmock.NewRows([]string{"count"}).AddRow(int64(2)))
	deleted, err := NewRepository(mock).CleanupProgressiveExperimentReports(context.Background(), scope, now, 100)
	if err != nil || deleted != 2 {
		t.Fatalf("deleted=%d err=%v", deleted, err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func progressiveRepositoryReport(scope memory.Scope) retrieval.ProgressiveExperimentReport {
	return retrieval.ProgressiveExperimentReport{
		RunIdentity:    "experiment:" + "1111111111111111111111111111111111111111111111111111111111111111",
		PolicyIdentity: "policy:" + "2222222222222222222222222222222222222222222222222222222222222222",
		ScopeHash:      progressiveExperimentScopeHash(scope), BaselineIdentity: "flat-fusion-v1", StrategyIdentity: "parent-first-v1",
		Mode: retrieval.ProgressiveExperimentModeShadow, SelectedStrategy: retrieval.ParentFirstFlatStrategy,
		Verdict: retrieval.ProgressiveExperimentShadow, Levels: []retrieval.ProgressiveExperimentLevelSummary{},
	}
}
