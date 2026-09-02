package benchmark

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"strings"

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
	Status             Status                     `json:"status"`
	Dataset            string                     `json:"dataset"`
	Version            string                     `json:"version"`
	RunID              string                     `json:"run_id"`
	ManifestChecksum   string                     `json:"manifest_checksum"`
	Strategy           StrategyProfile            `json:"strategy"`
	SyntheticFixture   bool                       `json:"synthetic_fixture"`
	Runtime            PostgresRuntimeIdentity    `json:"runtime"`
	Scope              memory.Scope               `json:"scope"`
	RetrievalReport    retrieval.EvaluationReport `json:"retrieval_report"`
	NormalizedChecksum string                     `json:"normalized_checksum,omitempty"`
	QRELChecksum       string                     `json:"qrels_checksum,omitempty"`
}

// RunLongMemEvalPostgres executes a locked normalized LongMemEval split through
// the same PostgreSQL + pgvector fixture path as the LoCoMo gate.
func RunLongMemEvalPostgres(ctx context.Context, dsn string, manifest DatasetManifest, corpus NormalizedCorpus, baseScope memory.Scope, split string) (PostgresSmokeRunResult, error) {
	if manifest.Name != "longmemeval" {
		return PostgresSmokeRunResult{}, &StatusError{Status: StatusInvalidManifest, Message: "LongMemEval runner requires longmemeval manifest"}
	}
	if strings.TrimSpace(split) == "" {
		split = "s"
	}
	run, err := NewRunScope(baseScope, manifest.Name, "longmemeval-"+split)
	if err != nil {
		return PostgresSmokeRunResult{}, err
	}
	return RunPostgresCorpus(ctx, dsn, manifest, corpus, run, false)
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
	return RunPostgresCorpus(ctx, dsn, manifest, corpus, run, true)
}

// RunPostgresCorpus executes a normalized benchmark corpus through the real
// PostgreSQL retrieval path. It retains only bounded evaluation output and
// runtime/input identifiers; callers may serialize the returned value as a
// completion artifact without exposing corpus content or a DSN.
func RunPostgresCorpus(ctx context.Context, dsn string, manifest DatasetManifest, corpus NormalizedCorpus, run BenchmarkRunScope, syntheticFixture bool) (PostgresSmokeRunResult, error) {
	if err := manifest.Validate(); err != nil {
		return PostgresSmokeRunResult{}, &StatusError{Status: StatusInvalidManifest, Message: "validate dataset manifest", Cause: err}
	}
	if err := corpus.Validate(); err != nil {
		return PostgresSmokeRunResult{}, &StatusError{Status: StatusInvalidManifest, Message: "validate normalized corpus", Cause: err}
	}
	if err := run.Scope.Validate(); err != nil {
		return PostgresSmokeRunResult{}, fmt.Errorf("validate benchmark run scope: %w", err)
	}
	normalizedChecksum, err := corpus.Checksum()
	if err != nil {
		return PostgresSmokeRunResult{}, err
	}
	qrels, err := json.Marshal(corpus.Canonical().QRELs)
	if err != nil {
		return PostgresSmokeRunResult{}, fmt.Errorf("marshal benchmark qrels: %w", err)
	}
	qrelChecksum := checksumBytes(qrels)
	manifestBytes, err := json.Marshal(manifest)
	if err != nil {
		return PostgresSmokeRunResult{}, fmt.Errorf("marshal dataset manifest: %w", err)
	}
	manifestChecksum := checksumBytes(manifestBytes)
	retrievalCorpus, err := partitionCorpusForRetrieval(corpus, run.Scope)
	if err != nil {
		return PostgresSmokeRunResult{}, err
	}
	retrievalCorpus = filterNonRetrievalQueries(retrievalCorpus)
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
	draftMappings := make(map[string]RetrievalEvidenceMapping, len(retrievalCorpus.Events))
	for _, event := range retrievalCorpus.Events {
		draftMappings[event.ID] = RetrievalEvidenceMapping{MemoryID: "draft-" + event.ID, Scope: event.Scope, State: memory.MemoryStateActive}
	}
	metadata := defaultRetrievalMetadata(manifest, run.ID)
	draft, err := BuildRetrievalEvaluationFixture(retrievalCorpus, draftMappings, metadata)
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
	mappings := make(map[string]RetrievalEvidenceMapping, len(retrievalCorpus.Events))
	for _, item := range seed.Aliases {
		if _, exists := mappings[item.Alias]; !exists {
			mappings[item.Alias] = RetrievalEvidenceMapping{MemoryID: item.MemoryID, RawEventID: item.RawEventID, Scope: item.Scope, State: item.State}
		}
	}
	prepared, err := BuildRetrievalEvaluationFixture(retrievalCorpus, mappings, metadata)
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
	return PostgresSmokeRunResult{Status: StatusSuccess, Dataset: manifest.Name, Version: manifest.Version, RunID: run.ID, ManifestChecksum: manifestChecksum, Strategy: StrategyProfileLexical, SyntheticFixture: syntheticFixture, Runtime: runtime, Scope: run.Scope, RetrievalReport: report, NormalizedChecksum: normalizedChecksum, QRELChecksum: qrelChecksum}, nil
}

func filterNonRetrievalQueries(corpus NormalizedCorpus) NormalizedCorpus {
	filtered := corpus
	filtered.Queries = make([]BenchmarkQuery, 0, len(corpus.Queries))
	allowed := map[string]struct{}{}
	for _, query := range corpus.Queries {
		if len(query.EvidenceGroups) == 0 && query.AbstentionExpected {
			continue
		}
		filtered.Queries = append(filtered.Queries, query)
		allowed[query.ID] = struct{}{}
	}
	filtered.QRELs = filtered.QRELs[:0]
	for _, qrel := range corpus.QRELs {
		if _, ok := allowed[qrel.QueryID]; ok {
			filtered.QRELs = append(filtered.QRELs, qrel)
		}
	}
	return filtered
}

// partitionCorpusForRetrieval derives per-sample benchmark namespaces only for
// the runtime fixture. The persisted normalized corpus and its checksum remain
// unchanged, while each query's PostgreSQL search is bounded to its sample.
func partitionCorpusForRetrieval(corpus NormalizedCorpus, base memory.Scope) (NormalizedCorpus, error) {
	copy := corpus
	copy.Events = append([]MemoryEventRecord(nil), corpus.Events...)
	copy.Queries = append([]BenchmarkQuery(nil), corpus.Queries...)
	for index := range copy.Events {
		sample := copy.Events[index].Provenance["question_id"]
		if sample == "" {
			sample = benchmarkSampleID(copy.Events[index].SessionID)
		}
		if sample == "" {
			return NormalizedCorpus{}, fmt.Errorf("benchmark event %s has no sample session id", copy.Events[index].ID)
		}
		copy.Events[index].Scope = partitionedBenchmarkScope(base, sample)
	}
	for index := range copy.Queries {
		sample := benchmarkSampleID(copy.Queries[index].SessionID)
		if sample == "" {
			return NormalizedCorpus{}, fmt.Errorf("benchmark query %s has no sample session id", copy.Queries[index].ID)
		}
		copy.Queries[index].Scope = partitionedBenchmarkScope(base, sample)
	}
	return copy, nil
}

func benchmarkSampleID(sessionID string) string {
	for index, character := range sessionID {
		if character == '/' {
			return sessionID[:index]
		}
	}
	return sessionID
}

func partitionedBenchmarkScope(base memory.Scope, sample string) memory.Scope {
	return memory.Scope{Tenant: base.Tenant, Project: base.Project, Namespace: base.Namespace + "-" + benchmarkIdentifier(sample)}
}

func defaultRetrievalMetadata(manifest DatasetManifest, runID string) RetrievalEvaluationMetadata {
	return RetrievalEvaluationMetadata{FixtureVersion: manifest.Name + "-" + manifest.Version + "-" + runID, RepresentationVersion: "normalized-" + SchemaVersion, RankingVersion: "benchmark-lexical-any-terms-v1", EmbeddingRevision: manifest.Embedding.Name, LexicalMatchMode: retrieval.LexicalMatchAnyTerms, PolicyVersion: "benchmark-safety-v1"}
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
