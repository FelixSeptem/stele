package benchmark

import (
	"bytes"
	"context"
	"os"
	"testing"

	"github.com/FelixSeptem/stele/internal/memory"
)

func TestRunLongMemEvalPostgresKeepsRunsAndLifecycleIsolated(t *testing.T) {
	dsn := os.Getenv("STELE_BENCHMARK_POSTGRES_DSN")
	if dsn == "" {
		t.Skip("set STELE_BENCHMARK_POSTGRES_DSN to run PostgreSQL 18 + pgvector integration coverage")
	}
	dataset, err := LoadLongMemEvalDataset(bytes.NewBufferString(longMemEvalFixture), LongMemEvalSubsetSmall)
	if err != nil {
		t.Fatal(err)
	}
	corpus, err := NewLongMemEvalAdapter().Normalize(dataset, memory.Scope{Tenant: "benchmark", Project: "longmemeval-integration", Namespace: "source"})
	if err != nil {
		t.Fatal(err)
	}
	// Keep this storage integration focused on qrels-to-canonical alignment.
	// The production lexical strategy uses simple FTS, so an exact evidence
	// token avoids turning this test into a stemming-quality assertion.
	corpus.Queries[0].Text = "library"
	manifest := postgresLongMemEvalManifest(t)
	firstScope, err := NewRunScope(memory.Scope{Tenant: "benchmark", Project: "longmemeval-integration", Namespace: "source"}, "longmemeval", "pg18-isolation")
	if err != nil {
		t.Fatal(err)
	}
	secondScope, err := NewRunScope(memory.Scope{Tenant: "benchmark", Project: "longmemeval-integration", Namespace: "source"}, "longmemeval", "pg18-isolation-second")
	if err != nil {
		t.Fatal(err)
	}
	for _, scope := range []memory.Scope{firstScope.Scope, secondScope.Scope} {
		if err := CleanLongMemEvalPostgresScope(context.Background(), dsn, scope); err != nil {
			t.Fatal(err)
		}
	}
	t.Cleanup(func() {
		for _, scope := range []memory.Scope{firstScope.Scope, secondScope.Scope} {
			if err := CleanLongMemEvalPostgresScope(context.Background(), dsn, scope); err != nil {
				t.Errorf("clean benchmark scope %s: %v", scope.Namespace, err)
			}
		}
	})
	run, err := RunLongMemEvalPostgres(context.Background(), LongMemEvalPostgresRunConfig{
		DSN:       dsn,
		Manifest:  manifest,
		Split:     "s",
		Corpus:    corpus,
		BaseScope: memory.Scope{Tenant: "benchmark", Project: "longmemeval-integration", Namespace: "source"},
		RunID:     "pg18-isolation",
	})
	if err != nil {
		t.Fatal(err)
	}
	if run.Report.Status != StatusSuccess || run.Report.Runtime.PostgreSQL == "" || run.Report.Runtime.PGVector == "" {
		t.Fatalf("expected successful PostgreSQL + pgvector report: %#v", run.Report)
	}
	if run.ImportedRawEvents != 2 || run.CanonicalMemories != 2 || run.Report.Scope.Project != "benchmark-longmemeval" {
		t.Fatalf("unexpected controlled import result: %#v", run)
	}
	if run.Retrieval.Report.Metrics.MustNotReturnViolations != 0 || run.Retrieval.Report.Metrics.RecallAt1 != 1 {
		t.Fatalf("expected qrels-aligned retrieval without obsolete evidence: %#v", run.Retrieval)
	}

	second, err := RunLongMemEvalPostgres(context.Background(), LongMemEvalPostgresRunConfig{
		DSN:       dsn,
		Manifest:  manifest,
		Split:     "s",
		Corpus:    corpus,
		BaseScope: memory.Scope{Tenant: "benchmark", Project: "longmemeval-integration", Namespace: "source"},
		RunID:     "pg18-isolation-second",
	})
	if err != nil {
		t.Fatal(err)
	}
	if second.Report.Scope.Namespace == run.Report.Scope.Namespace || second.Retrieval.Report.Metrics.MustNotReturnViolations != 0 {
		t.Fatalf("repeated import must have independent run scope and lifecycle safety: first=%#v second=%#v", run, second)
	}
}

func postgresLongMemEvalManifest(t *testing.T) DatasetManifest {
	t.Helper()
	manifest, err := LoadDatasetManifest(bytes.NewBufferString(`{
  "schema_version":"v1",
  "family":"agent_memory",
  "name":"longmemeval",
  "version":"postgres-integration-v1",
  "license":"MIT",
  "upstream_url":"https://www.modelscope.cn/datasets/evalscope/longmemeval-cleaned",
  "upstream_revision":"postgres-integration-v1",
  "sha256":"d6f21ea9d60a0d56f34a05b609c79c88a451d2ae03597821ea3d5a9678c3a442",
  "qrels_checksum":"d6f21ea9d60a0d56f34a05b609c79c88a451d2ae03597821ea3d5a9678c3a442",
  "source_path":"longmemeval_s_cleaned.json",
  "conversion_version":"longmemeval-cleaned-v1",
  "redistribution":"restricted",
  "support":"runnable",
  "splits":{"s":{"identity":"longmemeval/postgres-integration/s","source":"longmemeval_s_cleaned.json"}},
  "embedding":{"name":"lexical-only","normalization":"none"}
}`))
	if err != nil {
		t.Fatal(err)
	}
	return manifest
}
