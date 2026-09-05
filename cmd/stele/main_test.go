package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/FelixSeptem/stele/internal/app"
	"github.com/FelixSeptem/stele/internal/benchmark"
	"github.com/FelixSeptem/stele/internal/config"
	"github.com/FelixSeptem/stele/internal/memory"
)

type stubRunner struct {
	called bool
	err    error
}

func TestRunBenchmarkListPrintsDatasetSupportStates(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	if err := runBenchmark([]string{"list"}, &stdout, &stderr); err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(stdout.Bytes(), []byte("locomo")) || !bytes.Contains(stdout.Bytes(), []byte("metadata-only")) {
		t.Fatalf("unexpected benchmark list: %s", stdout.String())
	}
}

func TestRunBenchmarkSmokeWritesOfflineReport(t *testing.T) {
	t.Setenv("STELE_BENCHMARK_DATA_DIR", t.TempDir())
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	if err := runBenchmark([]string{"run-smoke"}, &stdout, &stderr); err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(stdout.Bytes(), []byte(`"status":"success"`)) || !bytes.Contains(stdout.Bytes(), []byte(`"offline":true`)) {
		t.Fatalf("unexpected smoke output: %s", stdout.String())
	}
}

func TestRunBenchmarkCleanRemovesOnlyNamedRunArtifacts(t *testing.T) {
	dataDir := t.TempDir()
	t.Setenv("STELE_BENCHMARK_DATA_DIR", dataDir)
	entry, ok := benchmark.DefaultRegistry().Get("locomo")
	if !ok {
		t.Fatal("locomo registry entry missing")
	}
	paths, err := benchmark.NewCache(dataDir).EnsureManifestLayout(entry.Manifest)
	if err != nil {
		t.Fatal(err)
	}
	runEmbeddings := filepath.Join(paths.Embeddings, "run-1")
	if err := os.MkdirAll(runEmbeddings, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(runEmbeddings, "vectors.bin"), []byte("vectors"), 0o644); err != nil {
		t.Fatal(err)
	}
	var stdout, stderr bytes.Buffer
	if err := runBenchmark([]string{"clean", "locomo", "run-1"}, &stdout, &stderr); err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(stdout.Bytes(), []byte(`"embeddings_removed":true`)) {
		t.Fatalf("unexpected cleanup result: %s", stdout.String())
	}
	if _, err := os.Stat(runEmbeddings); !os.IsNotExist(err) {
		t.Fatalf("run embeddings remain: %v", err)
	}
}

func TestRunBenchmarkFamilyActionsUseChecksumLockedLocalArtifacts(t *testing.T) {
	dataDir := t.TempDir()
	rawSource := filepath.Join("..", "..", "internal", "benchmark", "testdata", "locomo-smoke-fixture-v1.json")
	raw, err := os.ReadFile(rawSource)
	if err != nil {
		t.Fatal(err)
	}
	digest := sha256.Sum256(raw)
	manifest := benchmark.DatasetManifest{
		SchemaVersion:     benchmark.SchemaVersion,
		Family:            benchmark.FamilyAgentMemory,
		Name:              "locomo",
		Version:           "cli-v1",
		License:           "repository-test-fixture",
		UpstreamURL:       "https://example.test/locomo",
		UpstreamRevision:  "cli-v1",
		SHA256:            hex.EncodeToString(digest[:]),
		QRELChecksum:      "0000000000000000000000000000000000000000000000000000000000000000",
		SourcePath:        "locomo.json",
		ConversionVersion: "cli-v1",
		Redistribution:    benchmark.RedistributionPermitted,
		Support:           benchmark.SupportRunnable,
		Splits:            map[string]benchmark.SplitSpec{"smoke": {Identity: "locomo/cli-smoke", Source: "locomo.json"}},
		Embedding:         benchmark.EmbeddingProfile{Name: "lexical-only", Normalization: "none"},
	}
	encodedManifest, err := json.Marshal(manifest)
	if err != nil {
		t.Fatal(err)
	}
	manifestPath := filepath.Join(t.TempDir(), "manifest.json")
	if err := os.WriteFile(manifestPath, encodedManifest, 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("STELE_BENCHMARK_DATA_DIR", dataDir)
	t.Setenv("STELE_BENCHMARK_MANIFEST", manifestPath)
	t.Setenv("STELE_BENCHMARK_RAW_SOURCE", rawSource)
	t.Setenv("STELE_BENCHMARK_DATASET", manifest.Name)
	t.Setenv("STELE_BENCHMARK_DATA_VERSION", manifest.Version)
	t.Setenv("STELE_BENCHMARK_MODE", "smoke")
	t.Setenv("STELE_BENCHMARK_STRATEGY", "lexical")

	for _, command := range []string{"fetch", "normalize", "run"} {
		var stdout, stderr bytes.Buffer
		if err := runBenchmark([]string{command, "locomo"}, &stdout, &stderr); err != nil {
			t.Fatalf("benchmark %s: %v", command, err)
		}
		if !bytes.Contains(stdout.Bytes(), []byte(`"status":"success"`)) || !bytes.Contains(stdout.Bytes(), []byte(`"family":"agent_memory"`)) {
			t.Fatalf("unexpected benchmark %s output: %s", command, stdout.String())
		}
	}

	run, err := benchmark.NewRunScope(memory.Scope{Tenant: "benchmark", Project: "source", Namespace: "source"}, manifest.Name, "cli-report")
	if err != nil {
		t.Fatal(err)
	}
	report := benchmark.NewFamilyReport(benchmark.FamilyAgentMemory, manifest, "smoke", run.Scope)
	if _, err := benchmark.NewCache(dataDir).WriteFamilyReport(manifest, run.ID, report); err != nil {
		t.Fatal(err)
	}
	t.Setenv("STELE_BENCHMARK_REPORT_ID", run.ID)
	var stdout, stderr bytes.Buffer
	if err := runBenchmark([]string{"report", "locomo"}, &stdout, &stderr); err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(stdout.Bytes(), []byte(`"dataset":"locomo"`)) || !bytes.Contains(stdout.Bytes(), []byte(`"family":"agent_memory"`)) {
		t.Fatalf("unexpected benchmark report output: %s", stdout.String())
	}
}

func TestRunBenchmarkNormalizesChecksumLockedLongMemEvalSmallSubset(t *testing.T) {
	dataDir := t.TempDir()
	raw := []byte(`[{"question_id":"q-s","question":"Where does Ana work now?","question_type":"knowledge-update","answer_session_ids":["session-new"],"obsolete_session_ids":["session-old"],"haystack_sessions":[{"session_id":"session-old","session_date":"2024-01-01","turns":[{"turn_id":"old-turn","role":"user","content":"Ana works at the museum."}]},{"session_id":"session-new","session_date":"2024-02-01","turns":[{"turn_id":"new-turn","role":"user","content":"Ana works at the library now."}]}]}]`)
	digest := sha256.Sum256(raw)
	manifest := benchmark.DatasetManifest{
		SchemaVersion:     benchmark.SchemaVersion,
		Family:            benchmark.FamilyAgentMemory,
		Name:              "longmemeval",
		Version:           "cli-s-v1",
		License:           "MIT",
		UpstreamURL:       "https://www.modelscope.cn/datasets/evalscope/longmemeval-cleaned",
		UpstreamRevision:  "cli-s-v1",
		SHA256:            hex.EncodeToString(digest[:]),
		QRELChecksum:      hex.EncodeToString(digest[:]),
		SourcePath:        "longmemeval_s_cleaned.json",
		ConversionVersion: "longmemeval-cleaned-v1",
		Redistribution:    benchmark.RedistributionRestricted,
		Support:           benchmark.SupportRunnable,
		Splits:            map[string]benchmark.SplitSpec{"s": {Identity: "longmemeval/cli-s", Source: "longmemeval_s_cleaned.json", Checksum: hex.EncodeToString(digest[:])}},
		Embedding:         benchmark.EmbeddingProfile{Name: "lexical-only", Normalization: "none"},
	}
	encodedManifest, err := json.Marshal(manifest)
	if err != nil {
		t.Fatal(err)
	}
	manifestPath := filepath.Join(t.TempDir(), "longmemeval-s-manifest.json")
	if err := os.WriteFile(manifestPath, encodedManifest, 0o600); err != nil {
		t.Fatal(err)
	}
	rawSource := filepath.Join(t.TempDir(), "longmemeval_s_cleaned.json")
	if err := os.WriteFile(rawSource, raw, 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("STELE_BENCHMARK_DATA_DIR", dataDir)
	t.Setenv("STELE_BENCHMARK_MANIFEST", manifestPath)
	t.Setenv("STELE_BENCHMARK_RAW_SOURCE", rawSource)
	t.Setenv("STELE_BENCHMARK_SPLIT", "s")

	for _, command := range []string{"fetch", "normalize"} {
		var stdout, stderr bytes.Buffer
		if err := runBenchmark([]string{command, "longmemeval"}, &stdout, &stderr); err != nil {
			t.Fatalf("benchmark %s longmemeval: %v", command, err)
		}
		if !bytes.Contains(stdout.Bytes(), []byte(`"status":"success"`)) {
			t.Fatalf("unexpected LongMemEval %s output: %s", command, stdout.String())
		}
		if command == "normalize" && !bytes.Contains(stdout.Bytes(), []byte(`"split":"s"`)) {
			t.Fatalf("LongMemEval normalization must retain the locked split identity: %s", stdout.String())
		}
	}
}

func (s *stubRunner) Start(ctx context.Context) error {
	s.called = true
	return s.err
}

func TestRunReturnsConfigErrorForInvalidEnv(t *testing.T) {
	t.Setenv("STELE_MODE", "bad-mode")
	t.Setenv("STELE_POSTGRES_DSN", "postgres://example")

	if err := run(); err == nil {
		t.Fatal("run() error = nil, want config error")
	}
}

func TestRunStartsApplicationForValidEnv(t *testing.T) {
	t.Setenv("STELE_MODE", string(config.ModeAPI))
	t.Setenv("STELE_HTTP_ADDR", ":8080")
	t.Setenv("STELE_POSTGRES_DSN", "postgres://example")

	original := newRunner
	defer func() {
		newRunner = original
	}()

	runner := &stubRunner{}
	newRunner = func(cfg config.Config) (app.Runner, error) {
		return runner, nil
	}

	if err := run(); err != nil {
		t.Fatalf("run() error = %v", err)
	}

	if !runner.called {
		t.Fatal("runner was not started")
	}
}

func TestRunReturnsRunnerConstructionFailure(t *testing.T) {
	t.Setenv("STELE_MODE", string(config.ModeAPI))
	t.Setenv("STELE_HTTP_ADDR", ":8080")
	t.Setenv("STELE_POSTGRES_DSN", "postgres://example")

	original := newRunner
	defer func() {
		newRunner = original
	}()

	newRunner = func(cfg config.Config) (app.Runner, error) {
		return nil, errors.New("runner build failed")
	}

	if err := run(); err == nil {
		t.Fatal("run() error = nil, want runner construction failure")
	}
}
