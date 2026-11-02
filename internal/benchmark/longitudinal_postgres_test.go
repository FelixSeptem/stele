package benchmark

import (
	"bytes"
	"context"
	"os"
	"testing"

	"github.com/FelixSeptem/stele/internal/memory"
	"github.com/jackc/pgx/v5/pgxpool"
)

func TestRunTaskSeriesPostgresWhenConfigured(t *testing.T) {
	dsn := os.Getenv("STELE_TEST_TASK_SERIES_POSTGRES_DSN")
	if dsn == "" {
		t.Skip("STELE_TEST_TASK_SERIES_POSTGRES_DSN is not configured")
	}
	manifest := validTaskSeriesManifest()
	corpora := validTaskSeriesCorpora(manifest)
	baseScope := memory.Scope{Tenant: "benchmark-task-series-test", Project: "task-series", Namespace: "integration"}
	report, err := RunTaskSeriesPostgres(context.Background(), dsn, manifest, corpora, baseScope, DatasetManifest{}, true)
	if err != nil {
		t.Fatalf("RunTaskSeriesPostgres() error = %v", err)
	}
	if report.SyntheticFixture != true || report.NormalizedChecksum == "" || report.Runtime == nil || report.Strategy != StrategyProfileLexical {
		t.Fatalf("incomplete PostgreSQL task-series provenance: %#v", report)
	}
	if report.Status != StatusSuccess || report.SafetyStatus != TaskSeriesSafetyPassed {
		t.Fatalf("task-series gates = %q/%q", report.Status, report.SafetyStatus)
	}
	if !report.CleanupVerified {
		t.Fatal("task-series report did not retain successful cleanup evidence")
	}
	for _, condition := range report.Conditions {
		if condition.EvidenceStages == 0 {
			t.Fatalf("condition %d did not return any retrieval evidence", condition.ConditionIndex)
		}
		if condition.ProvenanceCoverage != 1 || condition.VerifiedProvenanceCount != condition.EvidenceCount {
			t.Fatalf("condition %d provenance coverage = %v (%d/%d), want 1 with every retrieved fixture source verified", condition.ConditionIndex, condition.ProvenanceCoverage, condition.VerifiedProvenanceCount, condition.EvidenceCount)
		}
	}
	second, err := RunTaskSeriesPostgres(context.Background(), dsn, manifest, corpora, baseScope, DatasetManifest{}, true)
	if err != nil {
		t.Fatalf("second RunTaskSeriesPostgres() error = %v", err)
	}
	firstJSON, err := MarshalTaskSeriesReport(report)
	if err != nil {
		t.Fatal(err)
	}
	secondJSON, err := MarshalTaskSeriesReport(second)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(firstJSON, secondJSON) {
		t.Fatalf("replayed bounded reports differ:\n%s\n%s", firstJSON, secondJSON)
	}
	pool, err := pgxpool.New(context.Background(), dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	for _, table := range []string{"raw_events", "candidate_memories", "canonical_memories"} {
		var count int
		if err := pool.QueryRow(context.Background(), "SELECT count(*) FROM "+table+" WHERE tenant=$1", baseScope.Tenant).Scan(&count); err != nil {
			t.Fatalf("count %s: %v", table, err)
		}
		if count != 0 {
			t.Fatalf("fixture cleanup left %d rows in %s", count, table)
		}
	}
	var versionCount int
	if err := pool.QueryRow(context.Background(), `SELECT count(*) FROM memory_versions version JOIN canonical_memories memory ON memory.id=version.memory_id WHERE memory.tenant=$1`, baseScope.Tenant).Scan(&versionCount); err != nil {
		t.Fatalf("count memory_versions: %v", err)
	}
	if versionCount != 0 {
		t.Fatalf("fixture cleanup left %d rows in memory_versions", versionCount)
	}
}
