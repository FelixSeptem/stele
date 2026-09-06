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
	"strings"
	"testing"

	"github.com/FelixSeptem/stele/internal/app"
	"github.com/FelixSeptem/stele/internal/benchmark"
	"github.com/FelixSeptem/stele/internal/config"
	"github.com/FelixSeptem/stele/internal/storage/postgres"
)

type stubRunner struct {
	called bool
	err    error
}

type stubMigrationCommandRunner struct {
	state     postgres.MigrationState
	statusErr error
	applyErr  error
	applied   bool
}

func (s *stubMigrationCommandRunner) Status(context.Context, string) (postgres.MigrationState, error) {
	return s.state, s.statusErr
}

func (s *stubMigrationCommandRunner) Apply(context.Context, string) error {
	s.applied = true
	return s.applyErr
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

func TestRunBenchmarkPostgresSmokeRequiresDSN(t *testing.T) {
	t.Setenv("STELE_POSTGRES_DSN", "")
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	err := runBenchmark([]string{"run-postgres-smoke"}, &stdout, &stderr)
	if err == nil || !strings.Contains(err.Error(), "STELE_POSTGRES_DSN") {
		t.Fatalf("expected DSN prerequisite error, got %v", err)
	}
}

func TestRunBenchmarkFetchUsesExplicitManifestAndLocalSource(t *testing.T) {
	data := []byte(`{"samples":[]}`)
	digest := sha256.Sum256(data)
	sourcePath := filepath.Join(t.TempDir(), "locomo.json")
	if err := os.WriteFile(sourcePath, data, 0o600); err != nil {
		t.Fatal(err)
	}
	manifestPath := writeBenchmarkManifest(t, benchmark.DatasetManifest{
		SchemaVersion:     benchmark.SchemaVersion,
		Name:              "locomo",
		Version:           "cli-v1",
		License:           "test-only",
		UpstreamURL:       "https://example.invalid/locomo",
		UpstreamRevision:  "test-revision",
		SHA256:            hex.EncodeToString(digest[:]),
		SourcePath:        "locomo.json",
		ConversionVersion: "locomo-v1",
		Redistribution:    benchmark.RedistributionPermitted,
		Support:           benchmark.SupportRunnable,
		Splits:            map[string]benchmark.SplitSpec{"smoke": {Source: "locomo.json"}},
		Embedding:         benchmark.EmbeddingProfile{Name: "none", Normalization: "none"},
	})
	dataDir := t.TempDir()
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	if err := runBenchmark([]string{"fetch", "--manifest", manifestPath, "--source", sourcePath, "--data-dir", dataDir}, &stdout, &stderr); err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(stdout.Bytes(), []byte(`"status":"success"`)) || !bytes.Contains(stdout.Bytes(), []byte(`"raw_path"`)) {
		t.Fatalf("unexpected fetch output: %s", stdout.String())
	}
	if _, err := os.Stat(filepath.Join(dataDir, "locomo", "cli-v1", "raw", "locomo.json")); err != nil {
		t.Fatalf("expected verified raw cache: %v", err)
	}
}

func TestRunBenchmarkNormalizeUsesExplicitScopeAndCachedRawArtifact(t *testing.T) {
	data := []byte(`{"samples":[{"id":"sample-1","sessions":[{"id":"session-1","turns":[{"id":"turn-1","text":"I prefer tea."}]}],"questions":[{"id":"query-1","text":"What do I prefer?","evidence_turn_ids":["turn-1"]}]}]}`)
	digest := sha256.Sum256(data)
	manifestPath := writeBenchmarkManifest(t, benchmark.DatasetManifest{
		SchemaVersion:     benchmark.SchemaVersion,
		Name:              "locomo",
		Version:           "cli-v1",
		License:           "test-only",
		UpstreamURL:       "https://example.invalid/locomo",
		UpstreamRevision:  "test-revision",
		SHA256:            hex.EncodeToString(digest[:]),
		SourcePath:        "locomo.json",
		ConversionVersion: "locomo-v1",
		Redistribution:    benchmark.RedistributionPermitted,
		Support:           benchmark.SupportRunnable,
		Splits:            map[string]benchmark.SplitSpec{"smoke": {Source: "locomo.json"}},
		Embedding:         benchmark.EmbeddingProfile{Name: "none", Normalization: "none"},
	})
	dataDir := t.TempDir()
	cache := benchmark.NewCache(dataDir)
	manifestFile, err := os.Open(manifestPath)
	if err != nil {
		t.Fatal(err)
	}
	manifest, err := benchmark.LoadDatasetManifest(manifestFile)
	manifestFile.Close()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := cache.StoreVerifiedRaw(manifest, bytes.NewReader(data)); err != nil {
		t.Fatal(err)
	}
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	if err := runBenchmark([]string{"normalize", "--manifest", manifestPath, "--data-dir", dataDir, "--split", "smoke", "--tenant", "benchmark", "--project", "benchmark-cli", "--namespace", "run-cli"}, &stdout, &stderr); err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(stdout.Bytes(), []byte(`"status":"success"`)) || !bytes.Contains(stdout.Bytes(), []byte(`"normalized_checksum"`)) {
		t.Fatalf("unexpected normalize output: %s", stdout.String())
	}
	if _, _, err := cache.LoadNormalized(manifest, "smoke"); err != nil {
		t.Fatalf("expected normalized cache: %v", err)
	}
}

func TestRunBenchmarkRunReportsOfflineAdmission(t *testing.T) {
	dataDir := t.TempDir()
	t.Setenv("STELE_BENCHMARK_DATA_DIR", dataDir)
	t.Setenv("STELE_BENCHMARK_DATASET", "locomo")
	t.Setenv("STELE_BENCHMARK_DATA_VERSION", "synthetic-smoke-v1")
	t.Setenv("STELE_BENCHMARK_MODE", "smoke")
	t.Setenv("STELE_BENCHMARK_STRATEGY", "lexical")
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	if err := runBenchmark([]string{"run"}, &stdout, &stderr); err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(stdout.Bytes(), []byte(`"status":"success"`)) || !bytes.Contains(stdout.Bytes(), []byte(`"offline":true`)) {
		t.Fatalf("unexpected run output: %s", stdout.String())
	}
}

func TestRunBenchmarkReportReturnsPersistedSmokeReport(t *testing.T) {
	dataDir := t.TempDir()
	t.Setenv("STELE_BENCHMARK_DATA_DIR", dataDir)
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	if err := runBenchmark([]string{"run-smoke"}, &stdout, &stderr); err != nil {
		t.Fatal(err)
	}
	stdout.Reset()
	if err := runBenchmark([]string{"report", "--dataset", "locomo", "--version", "synthetic-smoke-v1", "--name", "locomo-smoke-report-v1.json", "--data-dir", dataDir}, &stdout, &stderr); err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(stdout.Bytes(), []byte(`"dataset":"locomo"`)) || !bytes.Contains(stdout.Bytes(), []byte(`"report"`)) {
		t.Fatalf("unexpected report output: %s", stdout.String())
	}
}

func writeBenchmarkManifest(t *testing.T, manifest benchmark.DatasetManifest) string {
	t.Helper()
	encoded, err := json.Marshal(manifest)
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "manifest.json")
	if err := os.WriteFile(path, encoded, 0o600); err != nil {
		t.Fatal(err)
	}
	return path
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

func TestRunMigrateRejectsUnknownSubcommand(t *testing.T) {
	t.Setenv("STELE_POSTGRES_DSN", "postgres://example")
	if err := runArgs([]string{"migrate", "unknown"}); err == nil {
		t.Fatal("runArgs() error = nil, want unknown migrate subcommand error")
	}
}

func TestRunMigrateStatusIncludesBoundedIntegrityFacts(t *testing.T) {
	original := newMigrationCommandRunner
	defer func() { newMigrationCommandRunner = original }()
	newMigrationCommandRunner = func() migrationCommandRunner {
		return &stubMigrationCommandRunner{state: postgres.MigrationState{
			Status:          postgres.MigrationStatusCurrent,
			IntegrityStatus: postgres.MigrationIntegrityVerified,
			IntegrityRows:   1,
			CurrentVersion:  1,
			LatestVersion:   1,
		}}
	}
	t.Setenv("STELE_POSTGRES_DSN", "postgres://example")
	var output bytes.Buffer
	if err := runMigrateWithOutput([]string{"status"}, &output); err != nil {
		t.Fatalf("runMigrateWithOutput() error = %v", err)
	}
	if !strings.Contains(output.String(), "integrity_status=verified") || !strings.Contains(output.String(), "integrity_rows=1") {
		t.Fatalf("status output = %q, want bounded integrity facts", output.String())
	}
}

func TestRunMigrateJSONStatusIncludesIntegrityFacts(t *testing.T) {
	original := newMigrationCommandRunner
	defer func() { newMigrationCommandRunner = original }()
	newMigrationCommandRunner = func() migrationCommandRunner {
		return &stubMigrationCommandRunner{state: postgres.MigrationState{Status: postgres.MigrationStatusDivergent, IntegrityStatus: postgres.MigrationIntegrityUnknown, CurrentVersion: 1, LatestVersion: 1}}
	}
	t.Setenv("STELE_POSTGRES_DSN", "postgres://example")
	t.Setenv("STELE_MIGRATION_OUTPUT", "json")
	var output bytes.Buffer
	if err := runMigrateWithOutput([]string{"status"}, &output); err != nil {
		t.Fatalf("runMigrateWithOutput() error = %v", err)
	}
	if !bytes.Contains(output.Bytes(), []byte(`"integrity_status":"unknown"`)) || !bytes.Contains(output.Bytes(), []byte(`"status":"divergent"`)) {
		t.Fatalf("JSON status output = %s, want integrity and divergent facts", output.String())
	}
}

func TestRunMigrateUpReturnsDivergentIntegrityFailure(t *testing.T) {
	original := newMigrationCommandRunner
	defer func() { newMigrationCommandRunner = original }()
	stub := &stubMigrationCommandRunner{applyErr: errors.New("migration validation failed: status=divergent integrity_status=unknown")}
	newMigrationCommandRunner = func() migrationCommandRunner { return stub }
	t.Setenv("STELE_POSTGRES_DSN", "postgres://example")
	if err := runMigrateWithOutput([]string{"up"}, &bytes.Buffer{}); err == nil || !strings.Contains(err.Error(), "status=divergent") {
		t.Fatalf("runMigrateWithOutput(up) error = %v, want divergent failure", err)
	}
	if !stub.applied {
		t.Fatal("migration up did not delegate to shared integrity-gated runner")
	}
}
