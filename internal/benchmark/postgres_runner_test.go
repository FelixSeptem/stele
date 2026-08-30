package benchmark

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/FelixSeptem/stele/internal/memory"
	"github.com/FelixSeptem/stele/internal/retrieval"
	"github.com/FelixSeptem/stele/internal/storage/postgres"
)

func TestRunLoCoMoPostgresSmokeReturnsLexicalCandidates(t *testing.T) {
	dsn := os.Getenv("STELE_TEST_BENCHMARK_DSN")
	if dsn == "" {
		t.Skip("STELE_TEST_BENCHMARK_DSN is not configured; skipping real PostgreSQL benchmark smoke test")
	}

	result, err := RunLoCoMoPostgresSmoke(context.Background(), dsn, memory.Scope{
		Tenant:    "benchmark",
		Project:   "locomo",
		Namespace: "postgres-smoke-test",
	})
	if err != nil {
		t.Fatalf("RunLoCoMoPostgresSmoke() error = %v", err)
	}
	if len(result.RetrievalReport.Cases) == 0 {
		t.Fatal("RunLoCoMoPostgresSmoke() returned no evaluation cases")
	}
	for _, item := range result.RetrievalReport.Cases {
		if item.CandidatePoolSize == 0 {
			t.Fatalf("evaluation case %q returned no lexical candidates", item.CaseID)
		}
	}
}

func TestBenchmarkPostgresRetrievalExcludesHiddenAndForeignRunEvidence(t *testing.T) {
	dsn := os.Getenv("STELE_TEST_BENCHMARK_DSN")
	if dsn == "" {
		t.Skip("STELE_TEST_BENCHMARK_DSN is not configured; skipping real PostgreSQL benchmark isolation test")
	}

	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	pool, err := postgres.OpenPool(ctx, dsn)
	if err != nil {
		t.Fatalf("OpenPool() error = %v", err)
	}
	defer pool.Close()
	if err := postgres.BootstrapDatabase(ctx, pool); err != nil {
		t.Fatalf("BootstrapDatabase() error = %v", err)
	}

	base := memory.Scope{Tenant: "benchmark", Project: "locomo", Namespace: "integration"}
	firstRun, err := NewRunScope(base, "locomo", "isolation-first-"+time.Now().UTC().Format("20060102150405.000000000"))
	if err != nil {
		t.Fatal(err)
	}
	namespaceRun, err := NewRunScope(base, "locomo", "isolation-namespace-"+time.Now().UTC().Format("20060102150405.000000000"))
	if err != nil {
		t.Fatal(err)
	}
	tenantRun, err := NewRunScope(memory.Scope{Tenant: "benchmark-foreign", Project: "locomo", Namespace: "integration"}, "locomo", "isolation-tenant-"+time.Now().UTC().Format("20060102150405.000000000"))
	if err != nil {
		t.Fatal(err)
	}

	firstFixture := retrieval.EvaluationFixture{Version: "benchmark-isolation-v1", Cases: []retrieval.EvaluationCase{{
		ID:       "first-run",
		Category: "isolation",
		Scope:    firstRun.Scope,
		Query:    "copper marker",
		Sources: []retrieval.EvaluationSource{
			{Alias: "visible", EventType: "benchmark.episodic", Content: "visible copper marker evidence", Class: memory.MemoryClassEpisodic, State: memory.MemoryStateActive},
			{Alias: "suppressed", EventType: "benchmark.episodic", Content: "suppressed copper marker evidence", Class: memory.MemoryClassEpisodic, State: memory.MemoryStateSuppressed},
		},
		ExpectedEvidenceGroups: [][]string{{"visible"}},
		ExcludedAliases:        []string{"suppressed"},
	}}}
	namespaceFixture := retrieval.EvaluationFixture{Version: "benchmark-isolation-v1", Cases: []retrieval.EvaluationCase{{
		ID:                     "foreign-namespace-run",
		Category:               "isolation",
		Scope:                  namespaceRun.Scope,
		Query:                  "copper marker",
		Sources:                []retrieval.EvaluationSource{{Alias: "foreign-namespace", EventType: "benchmark.episodic", Content: "foreign namespace copper marker evidence", Class: memory.MemoryClassEpisodic, State: memory.MemoryStateActive}},
		ExpectedEvidenceGroups: [][]string{{"foreign-namespace"}},
	}}}
	tenantFixture := retrieval.EvaluationFixture{Version: "benchmark-isolation-v1", Cases: []retrieval.EvaluationCase{{
		ID:                     "foreign-tenant-run",
		Category:               "isolation",
		Scope:                  tenantRun.Scope,
		Query:                  "copper marker",
		Sources:                []retrieval.EvaluationSource{{Alias: "foreign-tenant", EventType: "benchmark.episodic", Content: "foreign tenant copper marker evidence", Class: memory.MemoryClassEpisodic, State: memory.MemoryStateActive}},
		ExpectedEvidenceGroups: [][]string{{"foreign-tenant"}},
	}}}

	repo := postgres.NewRepository(pool)
	seeder := postgres.NewEvaluationFixtureSeeder(repo)
	firstSeed, err := seeder.Seed(ctx, firstFixture)
	if err != nil {
		t.Fatalf("seed first run: %v", err)
	}
	defer func() {
		if cleanupErr := seeder.Cleanup(context.Background(), firstSeed); cleanupErr != nil {
			t.Errorf("cleanup first run: %v", cleanupErr)
		}
	}()
	namespaceSeed, err := seeder.Seed(ctx, namespaceFixture)
	if err != nil {
		t.Fatalf("seed foreign namespace run: %v", err)
	}
	defer func() {
		if cleanupErr := seeder.Cleanup(context.Background(), namespaceSeed); cleanupErr != nil {
			t.Errorf("cleanup foreign namespace run: %v", cleanupErr)
		}
	}()
	tenantSeed, err := seeder.Seed(ctx, tenantFixture)
	if err != nil {
		t.Fatalf("seed foreign tenant run: %v", err)
	}
	defer func() {
		if cleanupErr := seeder.Cleanup(context.Background(), tenantSeed); cleanupErr != nil {
			t.Errorf("cleanup foreign tenant run: %v", cleanupErr)
		}
	}()

	searcher := retrieval.NewService(retrieval.ServiceDependencies{Lexical: repo, Semantic: repo, Relations: repo})
	result, err := searcher.Search(ctx, retrieval.SearchInput{
		Scope:            firstRun.Scope,
		Query:            "copper marker",
		LexicalMatchMode: retrieval.LexicalMatchAnyTerms,
		TopK:             10,
		IncludeSummaries: true,
		IncludeRelations: true,
	})
	if err != nil {
		t.Fatalf("Search() error = %v", err)
	}
	if len(result.Hits) != 1 || result.Hits[0].Memory.ID != firstSeed.Aliases[0].MemoryID {
		t.Fatalf("Search() hits = %#v, want only the first run's visible evidence", result.Hits)
	}
}

func TestCorpusLoaderPostgresReusesRawEventOnRepeatedImport(t *testing.T) {
	dsn := os.Getenv("STELE_TEST_BENCHMARK_DSN")
	if dsn == "" {
		t.Skip("STELE_TEST_BENCHMARK_DSN is not configured; skipping real PostgreSQL repeated-import test")
	}

	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	pool, err := postgres.OpenPool(ctx, dsn)
	if err != nil {
		t.Fatalf("OpenPool() error = %v", err)
	}
	defer pool.Close()
	if err := postgres.BootstrapDatabase(ctx, pool); err != nil {
		t.Fatalf("BootstrapDatabase() error = %v", err)
	}

	run, err := NewRunScope(memory.Scope{Tenant: "benchmark", Project: "locomo", Namespace: "integration"}, "locomo", "repeat-import-"+time.Now().UTC().Format("20060102150405.000000000"))
	if err != nil {
		t.Fatal(err)
	}
	principalID := "benchmark:" + run.ID
	if _, err := pool.Exec(ctx, `INSERT INTO access_principals (id, role, status, label, created_at, updated_at) VALUES ($1, 'public', 'active', 'benchmark repeat-import test', now(), now())`, principalID); err != nil {
		t.Fatalf("create benchmark test principal: %v", err)
	}

	repo := postgres.NewRepository(pool)
	loader := NewCorpusLoader(memory.NewService(repo, time.Now))
	corpus := NormalizedCorpus{Events: []MemoryEventRecord{{
		ID:            "repeat-event",
		Scope:         run.Scope,
		SessionID:     "repeat-session",
		SourceTurnID:  "repeat-turn",
		Text:          "repeat import evidence",
		Class:         memory.MemoryClassEpisodic,
		ExpectedState: memory.MemoryStateActive,
	}}}
	first, err := loader.Load(ctx, run, corpus)
	if err != nil {
		t.Fatalf("first Load() error = %v", err)
	}
	second, err := loader.Load(ctx, run, corpus)
	if err != nil {
		t.Fatalf("second Load() error = %v", err)
	}
	rawEventID := first["repeat-event"].RawEventID
	if rawEventID == "" || second["repeat-event"].RawEventID != rawEventID {
		t.Fatalf("repeated import mappings = %#v and %#v, want the same raw event", first, second)
	}
	defer func() {
		for _, statement := range []string{
			`DELETE FROM provenance_links WHERE raw_event_id = $1`,
			`DELETE FROM event_idempotency_records WHERE raw_event_id = $1`,
			`DELETE FROM raw_events WHERE id = $1`,
		} {
			if _, cleanupErr := pool.Exec(context.Background(), statement, rawEventID); cleanupErr != nil {
				t.Errorf("cleanup repeated import data: %v", cleanupErr)
			}
		}
		if _, cleanupErr := pool.Exec(context.Background(), `DELETE FROM access_principals WHERE id = $1`, principalID); cleanupErr != nil {
			t.Errorf("cleanup benchmark test principal: %v", cleanupErr)
		}
	}()

	var rawEventCount int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM raw_events WHERE tenant = $1 AND project = $2 AND namespace = $3`, run.Scope.Tenant, run.Scope.Project, run.Scope.Namespace).Scan(&rawEventCount); err != nil {
		t.Fatalf("count imported raw events: %v", err)
	}
	if rawEventCount != 1 {
		t.Fatalf("raw event count = %d, want 1 after repeated import", rawEventCount)
	}
}
