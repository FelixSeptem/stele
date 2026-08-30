package postgres

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/FelixSeptem/stele/internal/memory"
	"github.com/FelixSeptem/stele/internal/retrieval"
)

func TestEvaluationFixtureSeederRejectsForeignScopeBeforeWriting(t *testing.T) {
	fixture := retrieval.EvaluationFixture{
		Version: "retrieval-fixture-v1",
		Cases: []retrieval.EvaluationCase{{
			ID:                     "foreign-scope",
			Scope:                  memory.Scope{Tenant: "operator", Project: "production", Namespace: "default"},
			Query:                  "query",
			Sources:                []retrieval.EvaluationSource{{Alias: "fact", EventType: "fixture", Content: "controlled source"}},
			ExpectedEvidenceGroups: [][]string{{"fact"}},
		}},
	}

	_, err := NewEvaluationFixtureSeeder(nil).Seed(context.Background(), fixture)
	if err == nil || !strings.Contains(err.Error(), "evaluation fixture scope is not owned") {
		t.Fatalf("Seed() error = %v, want evaluation fixture scope is not owned", err)
	}
}

func TestEvaluationFixtureSeederSeedsOwnedPostgresFixture(t *testing.T) {
	dsn := os.Getenv("STELE_TEST_RETRIEVAL_EVALUATION_DSN")
	if dsn == "" {
		t.Skip("STELE_TEST_RETRIEVAL_EVALUATION_DSN is not configured; skipping real PostgreSQL retrieval evaluation fixture test")
	}

	fixture := loadRetrievalEvaluationFixture(t)
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	pool, err := OpenPool(ctx, dsn)
	if err != nil {
		t.Fatalf("OpenPool() error = %v", err)
	}
	defer pool.Close()
	if err := BootstrapDatabase(ctx, pool); err != nil {
		t.Fatalf("BootstrapDatabase() error = %v", err)
	}

	repo := NewRepository(pool)
	seeder := NewEvaluationFixtureSeeder(repo)
	seeded, err := seeder.Seed(ctx, fixture)
	if err != nil {
		t.Fatalf("Seed() error = %v", err)
	}
	defer func() {
		if cleanupErr := seeder.Cleanup(context.Background(), seeded); cleanupErr != nil {
			t.Errorf("Cleanup() error = %v", cleanupErr)
		}
	}()
	if len(seeded.Aliases) != fixtureSourceCount(fixture) {
		t.Fatalf("seeded aliases = %d, want %d", len(seeded.Aliases), fixtureSourceCount(fixture))
	}

	retrievalService := retrieval.NewService(retrieval.ServiceDependencies{
		Lexical:   repo,
		Semantic:  repo,
		Relations: repo,
	})
	replay, err := retrieval.NewEvaluationRunner(retrievalService).Replay(ctx, fixture, seeded, retrieval.EvaluationRankingMetadata{
		FixtureVersion:              fixture.Version,
		RepresentationVersion:       "canonical-v1",
		RankingVersion:              "baseline-v1",
		CompatibleEmbeddingRevision: "deterministic-v1",
		PolicyVersion:               "quality-policy-v1",
	})
	if err != nil {
		t.Fatalf("Replay() error = %v", err)
	}
	report, err := retrieval.CalculateEvaluationMetrics(replay)
	if err != nil {
		t.Fatalf("CalculateEvaluationMetrics() error = %v", err)
	}
	if len(report.SafetyFailures) != 0 {
		t.Fatalf("real PostgreSQL replay safety failures = %+v", report.SafetyFailures)
	}
	if len(report.Cases) != len(fixture.Cases) {
		t.Fatalf("real PostgreSQL replay cases = %d, want %d", len(report.Cases), len(fixture.Cases))
	}

	for _, record := range seeded.Aliases {
		history, err := repo.ReadMemoryHistory(ctx, record.Scope, record.MemoryID, true)
		if err != nil {
			t.Fatalf("ReadMemoryHistory(%q) error = %v", record.Alias, err)
		}
		if len(history.Versions) == 0 || len(history.Provenance) < 2 {
			t.Fatalf("seeded history for %q lacks append-only version/provenance records", record.Alias)
		}
	}
}

func loadRetrievalEvaluationFixture(t *testing.T) retrieval.EvaluationFixture {
	t.Helper()
	encoded, err := os.ReadFile(filepath.Join("..", "..", "retrieval", "testdata", "retrieval-evaluation-fixture-v1.json"))
	if err != nil {
		t.Fatalf("read retrieval evaluation fixture: %v", err)
	}
	var fixture retrieval.EvaluationFixture
	if err := json.Unmarshal(encoded, &fixture); err != nil {
		t.Fatalf("unmarshal retrieval evaluation fixture: %v", err)
	}
	return fixture
}

func fixtureSourceCount(fixture retrieval.EvaluationFixture) int {
	count := 0
	for _, item := range fixture.Cases {
		count += len(item.Sources)
	}
	return count
}
