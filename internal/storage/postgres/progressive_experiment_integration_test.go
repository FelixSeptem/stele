package postgres

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/FelixSeptem/stele/internal/memory"
	"github.com/FelixSeptem/stele/internal/retrieval"
)

func TestProgressiveExperimentPostgresRoundTrip(t *testing.T) {
	dsn := os.Getenv("STELE_TEST_PROGRESSIVE_EXPERIMENT_DSN")
	if dsn == "" {
		t.Skip("STELE_TEST_PROGRESSIVE_EXPERIMENT_DSN is not configured; skipping progressive experiment integration test")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	pool, err := OpenPool(ctx, dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	if err := NewMigrationRunner().Apply(ctx, dsn); err != nil {
		t.Fatal(err)
	}
	scope := memory.Scope{Tenant: "progressive-it", Project: "p", Namespace: "n"}
	report := progressiveRepositoryReport(scope)
	expires := time.Now().UTC().Add(time.Hour)
	repo := NewRepository(pool)
	if err := repo.CreateProgressiveExperimentReport(ctx, scope, report, &expires); err != nil {
		t.Fatal(err)
	}
	reports, err := repo.ListProgressiveExperimentReports(ctx, scope, 10)
	if err != nil || len(reports) != 1 || reports[0].RunIdentity != report.RunIdentity {
		t.Fatalf("reports=%+v err=%v", reports, err)
	}
	foreign := scope
	foreign.Tenant = "foreign"
	if reports, err := repo.ListProgressiveExperimentReports(ctx, foreign, 10); err != nil || len(reports) != 0 {
		t.Fatalf("foreign reports=%+v err=%v", reports, err)
	}
	if _, err := repo.CleanupProgressiveExperimentReports(ctx, scope, time.Now().UTC(), 10); err != nil {
		t.Fatal(err)
	}
}

var _ retrieval.ProgressiveExperimentMode = retrieval.ProgressiveExperimentModeShadow
