package main

import (
	"bytes"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"

	"github.com/FelixSeptem/stele/internal/app"
	"github.com/FelixSeptem/stele/internal/benchmark"
	"github.com/FelixSeptem/stele/internal/config"
	"github.com/FelixSeptem/stele/internal/memory"
	"github.com/FelixSeptem/stele/internal/storage/postgres"
)

var newRunner = app.NewRunner

func main() {
	if err := run(); err != nil {
		log.Fatal(err)
	}
}

func runBenchmark(args []string, stdout, stderr io.Writer) error {
	if len(args) == 0 {
		return fmt.Errorf("benchmark command must be one of: list, fetch, normalize, run, report, run-smoke, run-postgres-smoke")
	}
	encoder := json.NewEncoder(stdout)
	switch args[0] {
	case "list":
		if len(args) != 1 {
			return fmt.Errorf("benchmark list does not accept arguments")
		}
		return encoder.Encode(benchmark.DefaultRegistry().List())
	case "fetch":
		result, err := benchmarkFetch(args[1:])
		if err != nil {
			return err
		}
		return encoder.Encode(result)
	case "normalize":
		result, err := benchmarkNormalize(args[1:])
		if err != nil {
			return err
		}
		return encoder.Encode(result)
	case "run":
		result, err := benchmarkRun(args[1:])
		if err != nil {
			return err
		}
		return encoder.Encode(result)
	case "report":
		return benchmarkReport(args[1:], stdout)
	case "run-smoke":
		dataDir := os.Getenv("STELE_BENCHMARK_DATA_DIR")
		if dataDir == "" {
			return fmt.Errorf("STELE_BENCHMARK_DATA_DIR is required for benchmark run-smoke")
		}
		result, err := benchmark.RunLoCoMoSmoke(benchmark.NewCache(dataDir), memory.Scope{Tenant: "benchmark", Project: "locomo-smoke", Namespace: "offline"})
		if err != nil {
			return err
		}
		return encoder.Encode(result)
	case "run-postgres-smoke":
		dsn := os.Getenv("STELE_POSTGRES_DSN")
		if dsn == "" {
			return fmt.Errorf("STELE_POSTGRES_DSN is required for benchmark run-postgres-smoke")
		}
		result, err := benchmark.RunLoCoMoPostgresSmoke(context.Background(), dsn, memory.Scope{Tenant: "benchmark", Project: "locomo", Namespace: "local"})
		if err != nil {
			return err
		}
		return encoder.Encode(result)
	default:
		_, _ = fmt.Fprintln(stderr, "benchmark command must be one of: list, fetch, normalize, run, report, run-smoke, run-postgres-smoke")
		return fmt.Errorf("unsupported benchmark command %q", args[0])
	}
}

type benchmarkFetchResult struct {
	Status  benchmark.Status `json:"status"`
	Dataset string           `json:"dataset"`
	Version string           `json:"version"`
	RawPath string           `json:"raw_path"`
}

func benchmarkFetch(args []string) (benchmarkFetchResult, error) {
	flags := flag.NewFlagSet("benchmark fetch", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	manifestPath := flags.String("manifest", "", "dataset manifest path")
	dataDir := flags.String("data-dir", os.Getenv("STELE_BENCHMARK_DATA_DIR"), "benchmark data directory")
	sourcePath := flags.String("source", "", "local source path")
	url := flags.String("url", "", "remote source URL")
	offline := flags.Bool("offline", envBool("STELE_BENCHMARK_OFFLINE", true), "disallow network fetch")
	if err := flags.Parse(args); err != nil {
		return benchmarkFetchResult{}, err
	}
	manifest, err := loadBenchmarkManifest(*manifestPath)
	if err != nil {
		return benchmarkFetchResult{}, err
	}
	cache := benchmark.NewCache(*dataDir)
	var rawPath string
	if strings.TrimSpace(*sourcePath) != "" {
		if *offline == false && strings.TrimSpace(*url) != "" {
			return benchmarkFetchResult{}, fmt.Errorf("benchmark fetch accepts either --source or --url, not both")
		}
		source, openErr := os.Open(*sourcePath)
		if openErr != nil {
			return benchmarkFetchResult{}, openErr
		}
		rawPath, err = cache.StoreVerifiedRaw(manifest, source)
		closeErr := source.Close()
		if err == nil {
			err = closeErr
		}
	} else {
		rawPath, err = cache.Fetch(benchmark.FetchInput{Manifest: manifest, URL: *url, Offline: *offline})
	}
	if err != nil {
		return benchmarkFetchResult{}, err
	}
	return benchmarkFetchResult{Status: benchmark.StatusSuccess, Dataset: manifest.Name, Version: manifest.Version, RawPath: rawPath}, nil
}

type benchmarkNormalizeResult struct {
	Status             benchmark.Status `json:"status"`
	Dataset            string           `json:"dataset"`
	Version            string           `json:"version"`
	Split              string           `json:"split"`
	NormalizedChecksum string           `json:"normalized_checksum"`
	NormalizedPath     string           `json:"normalized_path"`
}

func benchmarkNormalize(args []string) (benchmarkNormalizeResult, error) {
	flags := flag.NewFlagSet("benchmark normalize", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	manifestPath := flags.String("manifest", "", "dataset manifest path")
	dataDir := flags.String("data-dir", os.Getenv("STELE_BENCHMARK_DATA_DIR"), "benchmark data directory")
	split := flags.String("split", "smoke", "declared dataset split")
	tenant := flags.String("tenant", "benchmark", "benchmark tenant")
	project := flags.String("project", "", "benchmark project")
	namespace := flags.String("namespace", "", "benchmark namespace")
	if err := flags.Parse(args); err != nil {
		return benchmarkNormalizeResult{}, err
	}
	manifest, err := loadBenchmarkManifest(*manifestPath)
	if err != nil {
		return benchmarkNormalizeResult{}, err
	}
	if strings.TrimSpace(*project) == "" || strings.TrimSpace(*namespace) == "" {
		return benchmarkNormalizeResult{}, fmt.Errorf("--project and --namespace are required")
	}
	scope := memory.Scope{Tenant: *tenant, Project: *project, Namespace: *namespace}
	if err := scope.Validate(); err != nil {
		return benchmarkNormalizeResult{}, fmt.Errorf("validate benchmark scope: %w", err)
	}
	cache := benchmark.NewCache(*dataDir)
	paths, err := cache.Paths(manifest.Name, manifest.Version)
	if err != nil {
		return benchmarkNormalizeResult{}, err
	}
	if _, err := cache.LoadCacheLock(manifest); err != nil {
		return benchmarkNormalizeResult{}, err
	}
	rawPath := filepath.Join(paths.Raw, filepath.Base(manifest.SourcePath))
	encoded, err := os.ReadFile(rawPath)
	if err != nil {
		return benchmarkNormalizeResult{}, &benchmark.StatusError{Status: benchmark.StatusPrerequisiteMissing, Message: "raw dataset is missing", Cause: err}
	}
	adapter, err := benchmark.DefaultRegistry().Adapter(manifest.Name)
	if err != nil {
		return benchmarkNormalizeResult{}, err
	}
	corpus, err := adapter.NormalizeLocal(scope, encoded)
	if err != nil {
		return benchmarkNormalizeResult{}, err
	}
	checksum, err := cache.WriteNormalized(manifest, *split, corpus)
	if err != nil {
		return benchmarkNormalizeResult{}, err
	}
	return benchmarkNormalizeResult{Status: benchmark.StatusSuccess, Dataset: manifest.Name, Version: manifest.Version, Split: *split, NormalizedChecksum: checksum, NormalizedPath: filepath.Join(paths.Normalized, *split+".json")}, nil
}

func benchmarkRun(args []string) (any, error) {
	flags := flag.NewFlagSet("benchmark run", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	dataDir := flags.String("data-dir", os.Getenv("STELE_BENCHMARK_DATA_DIR"), "benchmark data directory")
	dataset := flags.String("dataset", os.Getenv("STELE_BENCHMARK_DATASET"), "benchmark dataset")
	version := flags.String("version", os.Getenv("STELE_BENCHMARK_DATA_VERSION"), "benchmark dataset version")
	mode := flags.String("mode", envString("STELE_BENCHMARK_MODE", "smoke"), "benchmark run mode")
	strategy := flags.String("strategy", envString("STELE_BENCHMARK_STRATEGY", "lexical"), "benchmark strategy")
	offline := flags.Bool("offline", envBool("STELE_BENCHMARK_OFFLINE", true), "offline execution")
	seed := flags.Int64("seed", envInt64("STELE_BENCHMARK_SEED", 0), "reproducibility seed")
	if err := flags.Parse(args); err != nil {
		return nil, err
	}
	if *dataset == "locomo" && *version == "synthetic-smoke-v1" && *mode == "smoke" && *strategy == "lexical" {
		result, err := benchmark.RunLoCoMoSmoke(benchmark.NewCache(*dataDir), memory.Scope{Tenant: "benchmark", Project: "locomo-smoke", Namespace: "offline"})
		if err != nil {
			return nil, err
		}
		return result, nil
	}
	config := benchmark.RunConfig{DataDir: *dataDir, Dataset: *dataset, Version: *version, Offline: *offline, Mode: benchmark.RunMode(*mode), Strategy: benchmark.RetrievalStrategy(*strategy), Seed: *seed}
	if err := config.Validate(); err != nil {
		return nil, err
	}
	return nil, &benchmark.StatusError{Status: benchmark.StatusPrerequisiteMissing, Message: "requested benchmark run is not implemented for this dataset and profile"}
}

func benchmarkReport(args []string, stdout io.Writer) error {
	flags := flag.NewFlagSet("benchmark report", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	dataDir := flags.String("data-dir", os.Getenv("STELE_BENCHMARK_DATA_DIR"), "benchmark data directory")
	dataset := flags.String("dataset", os.Getenv("STELE_BENCHMARK_DATASET"), "benchmark dataset")
	version := flags.String("version", os.Getenv("STELE_BENCHMARK_DATA_VERSION"), "benchmark dataset version")
	name := flags.String("name", "", "report file name")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if strings.TrimSpace(*dataset) == "" || strings.TrimSpace(*version) == "" || strings.TrimSpace(*name) == "" {
		return fmt.Errorf("--data-dir, --dataset, --version, and --name are required")
	}
	paths, err := benchmark.NewCache(*dataDir).Paths(*dataset, *version)
	if err != nil {
		return err
	}
	if filepath.Base(*name) != *name {
		return fmt.Errorf("report name must be a file name without path separators")
	}
	encoded, err := os.ReadFile(filepath.Join(paths.Reports, *name))
	if err != nil {
		return &benchmark.StatusError{Status: benchmark.StatusPrerequisiteMissing, Message: "benchmark report is missing", Cause: err}
	}
	if !json.Valid(encoded) {
		return &benchmark.StatusError{Status: benchmark.StatusInvalidManifest, Message: "benchmark report is not valid JSON"}
	}
	_, err = io.Copy(stdout, bytes.NewReader(append(encoded, '\n')))
	return err
}

func loadBenchmarkManifest(path string) (benchmark.DatasetManifest, error) {
	if strings.TrimSpace(path) == "" {
		return benchmark.DatasetManifest{}, fmt.Errorf("--manifest is required")
	}
	file, err := os.Open(path)
	if err != nil {
		return benchmark.DatasetManifest{}, err
	}
	defer file.Close()
	manifest, err := benchmark.LoadDatasetManifest(file)
	if err != nil {
		return benchmark.DatasetManifest{}, &benchmark.StatusError{Status: benchmark.StatusInvalidManifest, Message: "load benchmark manifest", Cause: err}
	}
	return manifest, nil
}

func envString(name, fallback string) string {
	if value := strings.TrimSpace(os.Getenv(name)); value != "" {
		return value
	}
	return fallback
}

func envBool(name string, fallback bool) bool {
	value := strings.TrimSpace(os.Getenv(name))
	if value == "" {
		return fallback
	}
	parsed, err := strconv.ParseBool(value)
	if err != nil {
		return fallback
	}
	return parsed
}

func envInt64(name string, fallback int64) int64 {
	value := strings.TrimSpace(os.Getenv(name))
	if value == "" {
		return fallback
	}
	parsed, err := strconv.ParseInt(value, 10, 64)
	if err != nil {
		return fallback
	}
	return parsed
}

func run() error {
	return runArgs(os.Args[1:])
}

func runArgs(args []string) error {
	if len(args) > 0 && args[0] == "benchmark" {
		return runBenchmark(args[1:], os.Stdout, os.Stderr)
	}
	if len(args) > 0 && args[0] == "migrate" {
		return runMigrate(args[1:])
	}
	cfg, err := config.LoadFromEnv()
	if err != nil {
		return err
	}

	runner, err := newRunner(cfg)
	if err != nil {
		return err
	}

	runContext, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	return runner.Start(runContext)
}

func runMigrate(args []string) error {
	if len(args) != 1 || (args[0] != "status" && args[0] != "up") {
		return fmt.Errorf("usage: stele migrate <status|up>")
	}
	dsn := os.Getenv("STELE_POSTGRES_DSN")
	if dsn == "" {
		return fmt.Errorf("STELE_POSTGRES_DSN is required")
	}
	runner := postgres.NewMigrationRunner()
	ctx := context.Background()
	if args[0] == "status" {
		state, err := runner.Status(ctx, dsn)
		if err != nil {
			return err
		}
		if os.Getenv("STELE_MIGRATION_OUTPUT") == "json" {
			return json.NewEncoder(os.Stdout).Encode(state)
		}
		fmt.Fprintf(os.Stdout, "status=%s current_version=%d latest_version=%d dirty=%t pending=%t\n", state.Status, state.CurrentVersion, state.LatestVersion, state.Dirty, state.Pending)
		return nil
	}
	return runner.Apply(ctx, dsn)
}
