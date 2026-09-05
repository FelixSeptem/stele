package benchmark

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/FelixSeptem/stele/internal/governance"
	"github.com/FelixSeptem/stele/internal/memory"
	"github.com/FelixSeptem/stele/internal/policy"
	"github.com/FelixSeptem/stele/internal/retrieval"
	"github.com/FelixSeptem/stele/internal/storage/postgres"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

// LongMemEvalPostgresRunConfig binds a checksum-locked normalized corpus to a
// real PostgreSQL + pgvector runtime. It deliberately has no production scope
// option: every run derives a fresh benchmark-only scope from BaseScope.
type LongMemEvalPostgresRunConfig struct {
	DSN       string
	Manifest  DatasetManifest
	Split     string
	Corpus    NormalizedCorpus
	BaseScope memory.Scope
	RunID     string
}

// LongMemEvalPostgresRun is the auditable result of an actual storage-backed
// LongMemEval retrieval replay. Evidence IDs remain the normalized IDs so the
// qrels report is independent of PostgreSQL UUID allocation.
type LongMemEvalPostgresRun struct {
	Report            FamilyReport
	Retrieval         LongMemEvalRunReport
	ImportedRawEvents int
	CanonicalMemories int
}

func (c LongMemEvalPostgresRunConfig) validate() error {
	if strings.TrimSpace(c.DSN) == "" {
		return &StatusError{Status: StatusPrerequisiteMissing, Message: "PostgreSQL DSN is required for LongMemEval retrieval"}
	}
	if c.Manifest.Name != "longmemeval" || c.Manifest.Family != FamilyAgentMemory {
		return &StatusError{Status: StatusInvalidManifest, Message: "LongMemEval PostgreSQL run requires a LongMemEval agent-memory manifest"}
	}
	if err := c.Manifest.Validate(); err != nil {
		return err
	}
	if _, ok := c.Manifest.Splits[c.Split]; !ok {
		return &StatusError{Status: StatusInvalidManifest, Message: "requested split is not declared by manifest"}
	}
	if err := c.Corpus.Validate(); err != nil {
		return fmt.Errorf("validate normalized LongMemEval corpus: %w", err)
	}
	if err := c.BaseScope.Validate(); err != nil {
		return fmt.Errorf("validate benchmark base scope: %w", err)
	}
	if strings.TrimSpace(c.RunID) == "" {
		return &StatusError{Status: StatusInvalidManifest, Message: "LongMemEval run id is required"}
	}
	return nil
}

func RunLongMemEvalPostgres(ctx context.Context, config LongMemEvalPostgresRunConfig) (LongMemEvalPostgresRun, error) {
	if err := config.validate(); err != nil {
		return LongMemEvalPostgresRun{}, err
	}
	pool, err := postgres.OpenPool(ctx, config.DSN)
	if err != nil {
		return LongMemEvalPostgresRun{}, fmt.Errorf("open LongMemEval PostgreSQL pool: %w", err)
	}
	defer pool.Close()
	if err := postgres.BootstrapDatabase(ctx, pool); err != nil {
		return LongMemEvalPostgresRun{}, fmt.Errorf("bootstrap LongMemEval PostgreSQL database: %w", err)
	}

	repo := postgres.NewRepository(pool)
	ingestor := memory.NewService(repo, time.Now)
	// The PostgreSQL repository currently implements the regular ingestion
	// store, not IdempotentIngestStore. Preserve loader behavior by exposing
	// only the supported EventIngestor capability for this real runtime.
	imported, err := NewLongMemEvalImporter(longMemEvalEventIngestor{service: ingestor}).Import(ctx, config.BaseScope, config.RunID, config.Corpus)
	if err != nil {
		return LongMemEvalPostgresRun{}, err
	}

	evidenceToMemory := make(map[string]string, len(config.Corpus.Events))
	events := append([]MemoryEventRecord(nil), config.Corpus.Events...)
	sort.Slice(events, func(i, j int) bool { return events[i].ID < events[j].ID })
	for _, event := range events {
		memoryID := longMemEvalDatabaseID(imported.Run.ID, "memory", event.ID)
		createdAt := time.Now().UTC()
		if _, err := repo.CreateMemory(ctx, memory.ManualCreateMemoryRecord{
			MemoryID:  memoryID,
			VersionID: longMemEvalDatabaseID(imported.Run.ID, "version", event.ID),
			Scope:     imported.Run.Scope,
			Class:     event.Class,
			Content:   event.Text,
			Reason:    "checksum-locked LongMemEval benchmark import",
			Actor:     "benchmark-runner",
			RequestID: "benchmark/" + imported.Run.ID + "/" + event.ID,
			CreatedAt: createdAt,
		}); err != nil {
			return LongMemEvalPostgresRun{}, fmt.Errorf("create LongMemEval canonical memory %s: %w", event.ID, err)
		}
		evidenceToMemory[event.ID] = memoryID
		if event.ExpectedState == memory.MemoryStateSuppressed || event.ExpectedState == memory.MemoryStateForgotten || event.ExpectedState == memory.MemoryStateDeleted {
			action := policy.ForgettingActionSuppress
			switch event.ExpectedState {
			case memory.MemoryStateForgotten:
				action = policy.ForgettingActionExpire
			case memory.MemoryStateDeleted:
				action = policy.ForgettingActionDelete
			}
			if _, err := repo.ApplyLifecycleAction(ctx, governance.LifecycleAction{
				MemoryID:  memoryID,
				Scope:     imported.Run.Scope,
				Action:    action,
				Content:   event.Text,
				Reason:    "LongMemEval expected lifecycle state",
				Actor:     "benchmark-runner",
				RequestID: "benchmark/lifecycle/" + imported.Run.ID + "/" + event.ID,
				AppliedAt: time.Now().UTC(),
			}); err != nil {
				return LongMemEvalPostgresRun{}, fmt.Errorf("apply LongMemEval lifecycle %s: %w", event.ID, err)
			}
		}
	}

	runner := LongMemEvalRunner{Retriever: postgresLongMemEvalRetriever{repo: repo, scope: imported.Run.Scope, evidenceToMemory: evidenceToMemory}}
	retrievalReport, err := runner.Run(ctx, withScope(config.Corpus, imported.Run.Scope), LongMemEvalComparisonSteleRetrieval)
	if err != nil {
		return LongMemEvalPostgresRun{}, err
	}
	corpusChecksum, err := config.Corpus.Checksum()
	if err != nil {
		return LongMemEvalPostgresRun{}, err
	}
	runtime, err := longMemEvalRuntimeIdentity(ctx, pool)
	if err != nil {
		return LongMemEvalPostgresRun{}, err
	}
	report := NewFamilyReport(FamilyAgentMemory, config.Manifest, config.Split, imported.Run.Scope).WithExecutionProvenance(FamilyReportExecution{
		QRELVersion:     config.Manifest.UpstreamRevision,
		StrategyProfile: string(StrategyLexical),
		InputChecksums:  map[string]string{"normalized_corpus": corpusChecksum, "raw_source": config.Manifest.SHA256},
		Runtime:         runtime,
	})
	report.Metrics = retrievalReport
	report.SafetyOutcomes = map[string]any{
		"imported_raw_events":           len(imported.Evidence),
		"canonical_memories":            len(evidenceToMemory),
		"must_not_return_violations":    retrievalReport.Report.Metrics.MustNotReturnViolations,
		"scope":                         imported.Run.Scope,
		"lifecycle_filtered_by_storage": true,
	}
	return LongMemEvalPostgresRun{Report: report, Retrieval: retrievalReport, ImportedRawEvents: len(imported.Evidence), CanonicalMemories: len(evidenceToMemory)}, nil
}

type postgresLongMemEvalRetriever struct {
	repo             *postgres.Repository
	scope            memory.Scope
	evidenceToMemory map[string]string
}

type longMemEvalEventIngestor struct {
	service *memory.Service
}

func (i longMemEvalEventIngestor) Ingest(ctx context.Context, input memory.IngestEventInput) (memory.RawEvent, error) {
	return i.service.Ingest(ctx, input)
}

func (r postgresLongMemEvalRetriever) Retrieve(ctx context.Context, query BenchmarkQuery) ([]RetrievedEvidence, error) {
	hits, err := r.repo.SearchLexical(ctx, retrieval.SearchInput{Scope: r.scope, Query: query.Text, TopK: 10})
	if err != nil {
		return nil, err
	}
	memoryToEvidence := make(map[string]string, len(r.evidenceToMemory))
	for evidenceID, memoryID := range r.evidenceToMemory {
		memoryToEvidence[memoryID] = evidenceID
	}
	items := make([]RetrievedEvidence, 0, len(hits))
	for index, hit := range hits {
		if evidenceID, found := memoryToEvidence[hit.Memory.ID]; found {
			items = append(items, RetrievedEvidence{EvidenceID: evidenceID, Rank: index + 1})
		}
	}
	return items, nil
}

func longMemEvalDatabaseID(runID, kind, sourceID string) string {
	return uuid.NewSHA1(uuid.NameSpaceURL, []byte("stele/longmemeval/"+runID+"/"+kind+"/"+sourceID)).String()
}

func longMemEvalRuntimeIdentity(ctx context.Context, pool *pgxpool.Pool) (RuntimeIdentity, error) {
	var runtime RuntimeIdentity
	if err := pool.QueryRow(ctx, "SELECT current_setting('server_version'), extversion FROM pg_extension WHERE extname = 'vector'").Scan(&runtime.PostgreSQL, &runtime.PGVector); err != nil {
		return RuntimeIdentity{}, fmt.Errorf("read PostgreSQL and pgvector runtime identity: %w", err)
	}
	return runtime, nil
}

// CleanLongMemEvalPostgresScope removes only a validated benchmark run scope.
// It is intentionally narrow so a caller cannot turn benchmark cleanup into a
// production data deletion operation.
func CleanLongMemEvalPostgresScope(ctx context.Context, dsn string, scope memory.Scope) error {
	if strings.TrimSpace(dsn) == "" {
		return &StatusError{Status: StatusPrerequisiteMissing, Message: "PostgreSQL DSN is required for benchmark cleanup"}
	}
	if scope.Project != "benchmark-longmemeval" || !strings.HasPrefix(scope.Namespace, "run-") {
		return &StatusError{Status: StatusInvalidManifest, Message: "LongMemEval cleanup requires a derived benchmark run scope"}
	}
	pool, err := postgres.OpenPool(ctx, dsn)
	if err != nil {
		return fmt.Errorf("open PostgreSQL pool for benchmark cleanup: %w", err)
	}
	defer pool.Close()
	if _, err := pool.Exec(ctx, `
DELETE FROM memory_versions mv
USING canonical_memories cm
WHERE mv.memory_id = cm.id
  AND cm.tenant = $1 AND cm.project = $2 AND cm.namespace = $3
`, scope.Tenant, scope.Project, scope.Namespace); err != nil {
		return fmt.Errorf("clean benchmark memory_versions: %w", err)
	}
	for _, table := range []string{"provenance_links", "raw_events", "canonical_memories"} {
		query := "DELETE FROM " + table + " WHERE tenant = $1 AND project = $2 AND namespace = $3"
		if _, err := pool.Exec(ctx, query, scope.Tenant, scope.Project, scope.Namespace); err != nil {
			return fmt.Errorf("clean benchmark %s: %w", table, err)
		}
	}
	return nil
}

func withScope(corpus NormalizedCorpus, scope memory.Scope) NormalizedCorpus {
	copy := corpus
	copy.Queries = append([]BenchmarkQuery(nil), corpus.Queries...)
	for index := range copy.Queries {
		copy.Queries[index].Scope = scope
	}
	return copy
}
