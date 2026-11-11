package benchmark

import (
	"context"
	"encoding/json"
	"os"
	"testing"
	"time"

	"github.com/FelixSeptem/stele/internal/memory"
	"github.com/FelixSeptem/stele/internal/retrieval"
	"github.com/jackc/pgx/v5/pgxpool"
)

func TestTaskSeriesV3PackBudgets(t *testing.T) {
	limits := TaskSeriesLimits{MaxItems: 2, MaxContentBytes: 7, MaxTokens: 2, MaxCitations: 2, MaxLatencyMS: 1000}
	hits := []retrieval.SearchHit{
		{Memory: memory.CanonicalMemory{Content: "界 a"}, Citations: []retrieval.Citation{{}}},
		{Memory: memory.CanonicalMemory{Content: "b"}, Citations: []retrieval.Citation{{}, {}}},
		{Memory: memory.CanonicalMemory{Content: "c"}, Citations: []retrieval.Citation{{}}},
	}
	if selected := taskSeriesPackHits(hits, limits); len(selected) != 1 || selected[0].Memory.Content != "界 a" {
		t.Fatalf("independent Unicode/token/citation packing: %+v", selected)
	}
}

func TestTaskSeriesV3PostgresOwnedSnapshots(t *testing.T) {
	dsn := os.Getenv("STELE_TEST_TASK_SERIES_V3_DSN")
	if dsn == "" {
		t.Skip("owned v3 DSN is not configured")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()
	b := v3Bundle(t)
	base := memory.Scope{Tenant: "benchmark-ev5-owned", Project: "staged", Namespace: "owned"}
	r, err := RunTaskSeriesPostgres(ctx, dsn, b.Manifest, b.Corpora, base, DatasetManifest{}, true)
	if err != nil {
		t.Fatal(err)
	}
	if r.V3 == nil || r.V3.GateStatus != "passed" || r.Status != StatusQualityGateFailed || !r.CleanupVerified || r.Runtime == nil || !r.SyntheticFixture {
		t.Fatalf("actual replay evidence missing: %+v", r)
	}
	if r.Conditions[0].EvidenceCount != 0 {
		t.Fatal("no-memory retrieved evidence")
	}
	for _, c := range r.Conditions[1:] {
		if c.EvidenceCount != 2 || c.ProvenanceCoverage != 1 || c.V3.StageVisibility.Passed != 2 || c.V3.Disclosure.Passed != 2 || c.V3.Rollback.Passed != 2 {
			t.Fatalf("stage windows/lineage/baseline replay not proven: %+v", c)
		}
	}
	encoded, err := MarshalTaskSeriesReport(r)
	if err != nil {
		t.Fatal(err)
	}
	if path := os.Getenv("STELE_TEST_TASK_SERIES_V3_REPORT"); path != "" {
		if err := os.WriteFile(path, encoded, 0600); err != nil {
			t.Fatal(err)
		}
	}
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	for _, table := range []string{"canonical_memories", "raw_events", "candidate_memories", "provenance_links"} {
		var count int
		if err := pool.QueryRow(ctx, "SELECT count(*) FROM "+table+" WHERE tenant=$1", base.Tenant).Scan(&count); err != nil || count != 0 {
			t.Fatalf("owned cleanup %s: %d %v", table, count, err)
		}
	}
	// Replaying after cleanup (and after restart when the wrapper runs again)
	// preserves input identity and task metrics; measured latency is separate.
	second, err := RunTaskSeriesPostgres(ctx, dsn, b.Manifest, b.Corpora, base, DatasetManifest{}, true)
	if err != nil {
		t.Fatal(err)
	}
	for i := range r.Conditions {
		r.Conditions[i].V3.Latency = TaskSeriesGateCounts{}
		second.Conditions[i].V3.Latency = TaskSeriesGateCounts{}
	}
	a, _ := json.Marshal(r.Conditions)
	z, _ := json.Marshal(second.Conditions)
	if string(a) != string(z) || r.V3.InputDigest != second.V3.InputDigest || r.NormalizedChecksum != second.NormalizedChecksum {
		t.Fatal("fixed-input task metrics changed across replay")
	}
}
