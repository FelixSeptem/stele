package benchmark

import (
	"context"
	"os"
	"strings"
	"testing"

	"github.com/FelixSeptem/stele/internal/memory"
)

// TestRunRealLongMemEvalSmallSubsetPostgres is an operator-invoked evidence
// test. It uses a deterministic, non-synthetic query/session slice of the
// full checksum-locked s corpus so a local PG18 instance can prove the real
// ingestion/retrieval boundary before an explicitly batched full-s run.
func TestRunRealLongMemEvalSmallSubsetPostgres(t *testing.T) {
	if os.Getenv("STELE_BENCHMARK_REAL_LONGMEMEVAL") != "1" {
		t.Skip("set STELE_BENCHMARK_REAL_LONGMEMEVAL=1 to run checksum-locked real LongMemEval evidence")
	}
	dsn := os.Getenv("STELE_BENCHMARK_POSTGRES_DSN")
	manifestPath := os.Getenv("STELE_BENCHMARK_MANIFEST")
	dataDir := os.Getenv("STELE_BENCHMARK_DATA_DIR")
	if dsn == "" || manifestPath == "" || dataDir == "" {
		t.Fatal("real LongMemEval run requires PostgreSQL DSN, manifest path, and benchmark data directory")
	}
	manifestSource, err := os.Open(manifestPath)
	if err != nil {
		t.Fatal(err)
	}
	defer manifestSource.Close()
	manifest, err := LoadDatasetManifest(manifestSource)
	if err != nil {
		t.Fatal(err)
	}
	cache := NewCache(dataDir)
	corpus, metadata, err := cache.LoadNormalized(manifest, "s")
	if err != nil {
		t.Fatal(err)
	}
	selected, policy := selectRealLongMemEvalCase(corpus)
	if err := selected.Validate(); err != nil {
		t.Fatal(err)
	}
	runID := "real-s-pg18"
	runScope, err := NewRunScope(memory.Scope{Tenant: "benchmark", Project: "longmemeval-real", Namespace: "source"}, "longmemeval", runID)
	if err != nil {
		t.Fatal(err)
	}
	if err := CleanLongMemEvalPostgresScope(context.Background(), dsn, runScope.Scope); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := CleanLongMemEvalPostgresScope(context.Background(), dsn, runScope.Scope); err != nil {
			t.Errorf("clean real LongMemEval benchmark scope: %v", err)
		}
	})

	run, err := RunLongMemEvalPostgres(context.Background(), LongMemEvalPostgresRunConfig{
		DSN: dsn, Manifest: manifest, Split: "s", Corpus: selected,
		BaseScope: memory.Scope{Tenant: "benchmark", Project: "longmemeval-real", Namespace: "source"}, RunID: runID,
	})
	if err != nil {
		t.Fatal(err)
	}
	run.Report.InputChecksums["full_normalized_s_corpus"] = metadata.Checksum
	run.Report.Metrics = map[string]any{
		"retrieval":              run.Retrieval,
		"selection_policy":       policy,
		"selected_queries":       len(selected.Queries),
		"selected_events":        len(selected.Events),
		"full_split_checksum":    metadata.Checksum,
		"residual_prerequisites": []string{"a batched full-s PostgreSQL runner is required before evaluating all 246738 normalized events in one standard-subset storage run"},
	}
	paths, err := cache.ManifestPaths(manifest)
	if err != nil {
		t.Fatal(err)
	}
	run.Report.ArtifactPaths = []string{paths.Normalized}
	if _, err := cache.WriteFamilyReport(manifest, runID, run.Report); err != nil {
		t.Fatal(err)
	}
	if run.Report.Runtime.PostgreSQL == "" || run.Report.Runtime.PGVector == "" || run.Retrieval.Report.Metrics.QueryCount != 1 || run.Retrieval.Report.Metrics.MustNotReturnViolations != 0 {
		t.Fatalf("unexpected real LongMemEval evidence: %#v", run)
	}
}

func selectRealLongMemEvalCase(corpus NormalizedCorpus) (NormalizedCorpus, string) {
	bestQueryID := ""
	bestScore := -1
	events := make(map[string]MemoryEventRecord, len(corpus.Events))
	for _, event := range corpus.Events {
		events[event.ID] = event
	}
	for _, query := range corpus.Queries {
		for _, qrel := range corpus.QRELs {
			if qrel.QueryID != query.ID || qrel.Grade <= 0 {
				continue
			}
			event, ok := events[qrel.EvidenceID]
			if !ok {
				continue
			}
			score := tokenOverlap(query.Text, event.Text)
			if score > bestScore || (score == bestScore && (bestQueryID == "" || query.ID < bestQueryID)) {
				bestQueryID, bestScore = query.ID, score
			}
		}
	}
	if bestQueryID == "" {
		return NormalizedCorpus{SchemaVersion: SchemaVersion}, "no positive-qrel query available"
	}
	questionID := strings.TrimPrefix(bestQueryID, "query/longmemeval/")
	selected := NormalizedCorpus{SchemaVersion: SchemaVersion}
	for _, event := range corpus.Events {
		if event.Provenance["question_id"] == questionID {
			selected.Events = append(selected.Events, event)
		}
	}
	for _, query := range corpus.Queries {
		if query.ID == bestQueryID {
			selected.Queries = append(selected.Queries, query)
		}
	}
	for _, qrel := range corpus.QRELs {
		if qrel.QueryID == bestQueryID {
			selected.QRELs = append(selected.QRELs, qrel)
		}
	}
	return selected, "deterministic positive-qrel query with maximum lexical overlap; retains every event in its source question/session population"
}

func tokenOverlap(left, right string) int {
	words := make(map[string]struct{})
	for _, word := range strings.Fields(strings.ToLower(left)) {
		words[strings.Trim(word, "?.!,;:\"")] = struct{}{}
	}
	score := 0
	for _, word := range strings.Fields(strings.ToLower(right)) {
		if _, ok := words[strings.Trim(word, "?.!,;:\"")]; ok {
			score++
		}
	}
	return score
}
