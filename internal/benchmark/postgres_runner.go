package benchmark

import (
	"bytes"
	"context"
	"fmt"

	"github.com/FelixSeptem/stele/internal/memory"
	"github.com/FelixSeptem/stele/internal/retrieval"
	"github.com/FelixSeptem/stele/internal/storage/postgres"
	"github.com/jackc/pgx/v5/pgxpool"
)

type PostgresRuntimeIdentity struct {
	PostgreSQLVersion string `json:"postgresql_version"`
	PGVectorVersion   string `json:"pgvector_version"`
}

type PostgresSmokeRunResult struct {
	Status           Status                     `json:"status"`
	Dataset          string                     `json:"dataset"`
	Version          string                     `json:"version"`
	SyntheticFixture bool                       `json:"synthetic_fixture"`
	Runtime          PostgresRuntimeIdentity    `json:"runtime"`
	Scope            memory.Scope               `json:"scope"`
	RetrievalReport  retrieval.EvaluationReport `json:"retrieval_report"`
}

func RunLoCoMoPostgresSmoke(ctx context.Context, dsn string, baseScope memory.Scope) (PostgresSmokeRunResult, error) {
	manifest, err := LoadDatasetManifest(bytes.NewReader(loCoMoSmokeManifest))
	if err != nil {
		return PostgresSmokeRunResult{}, err
	}
	dataset, err := LoadLoCoMoDatasetFromBytes(bytes.ReplaceAll(loCoMoSmokeFixture, []byte("\r\n"), []byte("\n")))
	if err != nil {
		return PostgresSmokeRunResult{}, err
	}
	run, err := NewRunScope(baseScope, manifest.Name, "postgres-smoke-v1")
	if err != nil {
		return PostgresSmokeRunResult{}, err
	}
	corpus, err := NewLoCoMoAdapter().Normalize(dataset, run.Scope)
	if err != nil {
		return PostgresSmokeRunResult{}, err
	}
	pool, err := postgres.OpenPool(ctx, dsn)
	if err != nil {
		return PostgresSmokeRunResult{}, err
	}
	defer pool.Close()
	if err := postgres.BootstrapDatabase(ctx, pool); err != nil {
		return PostgresSmokeRunResult{}, err
	}
	runtime, err := postgresRuntimeIdentity(ctx, pool)
	if err != nil {
		return PostgresSmokeRunResult{}, err
	}
	draftMappings := make(map[string]RetrievalEvidenceMapping, len(corpus.Events))
	for _, event := range corpus.Events {
		draftMappings[event.ID] = RetrievalEvidenceMapping{MemoryID: "draft-" + event.ID, Scope: run.Scope, State: memory.MemoryStateActive}
	}
	draft, err := BuildRetrievalEvaluationFixture(corpus, draftMappings, defaultRetrievalMetadata(manifest))
	if err != nil {
		return PostgresSmokeRunResult{}, err
	}
	repo := postgres.NewRepository(pool)
	seeder := postgres.NewEvaluationFixtureSeeder(repo)
	seed, err := seeder.Seed(ctx, draft.Fixture)
	if err != nil {
		return PostgresSmokeRunResult{}, err
	}
	defer seeder.Cleanup(context.Background(), seed)
	mappings := make(map[string]RetrievalEvidenceMapping, len(corpus.Events))
	for _, item := range seed.Aliases {
		if _, exists := mappings[item.Alias]; !exists {
			mappings[item.Alias] = RetrievalEvidenceMapping{MemoryID: item.MemoryID, RawEventID: item.RawEventID, Scope: item.Scope, State: item.State}
		}
	}
	prepared, err := BuildRetrievalEvaluationFixture(corpus, mappings, defaultRetrievalMetadata(manifest))
	if err != nil {
		return PostgresSmokeRunResult{}, err
	}
	searcher := retrieval.NewService(retrieval.ServiceDependencies{Lexical: repo, Semantic: repo, Relations: repo})
	replay, err := retrieval.NewEvaluationRunner(searcher).Replay(ctx, prepared.Fixture, prepared.Seed, prepared.Metadata)
	if err != nil {
		return PostgresSmokeRunResult{}, err
	}
	report, err := retrieval.CalculateEvaluationMetrics(replay)
	if err != nil {
		return PostgresSmokeRunResult{}, err
	}
	return PostgresSmokeRunResult{Status: StatusSuccess, Dataset: manifest.Name, Version: manifest.Version, SyntheticFixture: true, Runtime: runtime, Scope: run.Scope, RetrievalReport: report}, nil
}

func defaultRetrievalMetadata(manifest DatasetManifest) RetrievalEvaluationMetadata {
	return RetrievalEvaluationMetadata{FixtureVersion: manifest.Name + "-" + manifest.Version, RepresentationVersion: "normalized-" + SchemaVersion, RankingVersion: "benchmark-lexical-v1", EmbeddingRevision: manifest.Embedding.Name, PolicyVersion: "benchmark-safety-v1"}
}

func postgresRuntimeIdentity(ctx context.Context, pool *pgxpool.Pool) (PostgresRuntimeIdentity, error) {
	var identity PostgresRuntimeIdentity
	if err := pool.QueryRow(ctx, "SELECT version()").Scan(&identity.PostgreSQLVersion); err != nil {
		return PostgresRuntimeIdentity{}, fmt.Errorf("read PostgreSQL version: %w", err)
	}
	if err := pool.QueryRow(ctx, "SELECT extversion FROM pg_extension WHERE extname = 'vector'").Scan(&identity.PGVectorVersion); err != nil {
		return PostgresRuntimeIdentity{}, fmt.Errorf("read pgvector version: %w", err)
	}
	return identity, nil
}
