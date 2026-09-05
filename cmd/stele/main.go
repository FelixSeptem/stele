package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/FelixSeptem/stele/internal/app"
	"github.com/FelixSeptem/stele/internal/benchmark"
	"github.com/FelixSeptem/stele/internal/config"
	"github.com/FelixSeptem/stele/internal/memory"
)

var newRunner = app.NewRunner

func main() {
	if len(os.Args) > 1 && os.Args[1] == "benchmark" {
		if err := runBenchmark(os.Args[2:], os.Stdout, os.Stderr); err != nil {
			log.Fatal(err)
		}
		return
	}
	if err := run(); err != nil {
		log.Fatal(err)
	}
}

func runBenchmark(args []string, stdout, stderr io.Writer) error {
	if len(args) < 1 || len(args) > 3 {
		return fmt.Errorf("benchmark command must be one of: list, fetch, normalize, run, report, clean, run-smoke")
	}
	encoder := json.NewEncoder(stdout)
	switch args[0] {
	case "list":
		if len(args) != 1 {
			return fmt.Errorf("benchmark list does not accept arguments")
		}
		return encoder.Encode(benchmark.DefaultRegistry().List())
	case "clean":
		if len(args) != 3 {
			return fmt.Errorf("benchmark clean requires a dataset name and run id")
		}
		dataDir := os.Getenv("STELE_BENCHMARK_DATA_DIR")
		if dataDir == "" {
			return fmt.Errorf("STELE_BENCHMARK_DATA_DIR is required for benchmark clean")
		}
		entry, ok := benchmark.DefaultRegistry().Get(args[1])
		if !ok {
			return fmt.Errorf("benchmark dataset %q is not registered", args[1])
		}
		result, err := benchmark.NewCache(dataDir).CleanRunArtifacts(entry.Manifest, args[2], true)
		if err != nil {
			return err
		}
		return encoder.Encode(map[string]any{"command": "clean", "dataset": entry.Manifest.Name, "family": entry.Family, "run_id": args[2], "status": benchmark.StatusSuccess, "result": result})
	case "fetch", "normalize", "run", "report":
		if len(args) != 2 {
			return fmt.Errorf("benchmark %s requires a dataset name", args[0])
		}
		entry, manifest, cache, err := benchmarkCLIInputs(args[1])
		if err != nil {
			return err
		}
		switch args[0] {
		case "fetch":
			sourcePath := strings.TrimSpace(os.Getenv("STELE_BENCHMARK_RAW_SOURCE"))
			if sourcePath == "" {
				return &benchmark.StatusError{Status: benchmark.StatusPrerequisiteMissing, Message: "STELE_BENCHMARK_RAW_SOURCE is required for offline benchmark fetch"}
			}
			source, err := os.Open(sourcePath)
			if err != nil {
				return &benchmark.StatusError{Status: benchmark.StatusPrerequisiteMissing, Message: "open local benchmark raw source", Cause: err}
			}
			defer source.Close()
			stored, err := cache.StoreVerifiedRaw(manifest, source)
			if err != nil {
				return err
			}
			return encoder.Encode(map[string]any{"command": "fetch", "dataset": manifest.Name, "family": entry.Family, "status": benchmark.StatusSuccess, "raw_path": stored, "offline": true})
		case "normalize":
			split := benchmarkCLISplit()
			var adapter benchmark.DatasetAdapter
			if args[1] == "longmemeval" {
				adapter, err = benchmark.DefaultRegistry().LongMemEvalDatasetAdapter(benchmark.LongMemEvalSubset(split))
			} else {
				adapter, err = benchmark.DefaultRegistry().Adapter(args[1])
			}
			if err != nil {
				return err
			}
			paths, err := cache.ManifestPaths(manifest)
			if err != nil {
				return err
			}
			raw, err := os.ReadFile(filepath.Join(paths.Raw, filepath.Base(manifest.SourcePath)))
			if err != nil {
				return &benchmark.StatusError{Status: benchmark.StatusPrerequisiteMissing, Message: "read locked local benchmark raw source", Cause: err}
			}
			run, err := benchmark.NewRunScope(benchmarkCLIBaseScope(), manifest.Name, "normalize-"+split)
			if err != nil {
				return err
			}
			corpus, err := adapter.NormalizeLocal(run.Scope, raw)
			if err != nil {
				return err
			}
			checksum, err := cache.WriteNormalized(manifest, split, corpus)
			if err != nil {
				return err
			}
			return encoder.Encode(map[string]any{"command": "normalize", "dataset": manifest.Name, "family": entry.Family, "status": benchmark.StatusSuccess, "split": split, "checksum": checksum, "scope": run.Scope, "offline": true})
		case "run":
			config, err := benchmark.LoadRunConfigFromEnv()
			if err != nil {
				return err
			}
			admission := benchmark.AdmitRun(cache, manifest, config)
			return encoder.Encode(map[string]any{"command": "run", "dataset": manifest.Name, "family": entry.Family, "status": admission.Status, "admission": admission, "offline": config.Offline})
		case "report":
			runID := strings.TrimSpace(os.Getenv("STELE_BENCHMARK_REPORT_ID"))
			if runID == "" {
				return &benchmark.StatusError{Status: benchmark.StatusPrerequisiteMissing, Message: "STELE_BENCHMARK_REPORT_ID is required for benchmark report"}
			}
			report, err := cache.LoadFamilyReport(manifest, runID)
			if err != nil {
				return err
			}
			return encoder.Encode(report)
		}
		return fmt.Errorf("unsupported benchmark command %q", args[0])
	case "run-smoke":
		if len(args) != 1 {
			return fmt.Errorf("benchmark run-smoke does not accept arguments")
		}
		dataDir := os.Getenv("STELE_BENCHMARK_DATA_DIR")
		if dataDir == "" {
			return fmt.Errorf("STELE_BENCHMARK_DATA_DIR is required for benchmark run-smoke")
		}
		result, err := benchmark.RunLoCoMoSmoke(benchmark.NewCache(dataDir), memory.Scope{Tenant: "benchmark", Project: "locomo-smoke", Namespace: "offline"})
		if err != nil {
			return err
		}
		return encoder.Encode(result)
	default:
		_, _ = fmt.Fprintln(stderr, "benchmark command must be one of: list, fetch, normalize, run, report, clean, run-smoke")
		return fmt.Errorf("unsupported benchmark command %q", args[0])
	}
}

func benchmarkCLIInputs(dataset string) (benchmark.DatasetRegistration, benchmark.DatasetManifest, benchmark.Cache, error) {
	entry, ok := benchmark.DefaultRegistry().Get(dataset)
	if !ok {
		return benchmark.DatasetRegistration{}, benchmark.DatasetManifest{}, benchmark.Cache{}, fmt.Errorf("benchmark dataset %q is not registered", dataset)
	}
	dataDir := strings.TrimSpace(os.Getenv("STELE_BENCHMARK_DATA_DIR"))
	if dataDir == "" {
		return benchmark.DatasetRegistration{}, benchmark.DatasetManifest{}, benchmark.Cache{}, &benchmark.StatusError{Status: benchmark.StatusPrerequisiteMissing, Message: "STELE_BENCHMARK_DATA_DIR is required for benchmark command"}
	}
	manifestPath := strings.TrimSpace(os.Getenv("STELE_BENCHMARK_MANIFEST"))
	if manifestPath == "" {
		return benchmark.DatasetRegistration{}, benchmark.DatasetManifest{}, benchmark.Cache{}, &benchmark.StatusError{Status: benchmark.StatusPrerequisiteMissing, Message: "STELE_BENCHMARK_MANIFEST is required for benchmark command"}
	}
	manifestSource, err := os.Open(manifestPath)
	if err != nil {
		return benchmark.DatasetRegistration{}, benchmark.DatasetManifest{}, benchmark.Cache{}, &benchmark.StatusError{Status: benchmark.StatusPrerequisiteMissing, Message: "open local benchmark manifest", Cause: err}
	}
	defer manifestSource.Close()
	manifest, err := benchmark.LoadDatasetManifest(manifestSource)
	if err != nil {
		return benchmark.DatasetRegistration{}, benchmark.DatasetManifest{}, benchmark.Cache{}, err
	}
	if manifest.Name != entry.Manifest.Name || manifest.Family != entry.Family {
		return benchmark.DatasetRegistration{}, benchmark.DatasetManifest{}, benchmark.Cache{}, &benchmark.StatusError{Status: benchmark.StatusInvalidManifest, Message: "local benchmark manifest dataset and family must match registry"}
	}
	return entry, manifest, benchmark.NewCache(dataDir), nil
}

func benchmarkCLISplit() string {
	if split := strings.TrimSpace(os.Getenv("STELE_BENCHMARK_SPLIT")); split != "" {
		return split
	}
	return "smoke"
}

func benchmarkCLIBaseScope() memory.Scope {
	tenant := strings.TrimSpace(os.Getenv("STELE_BENCHMARK_TENANT"))
	if tenant == "" {
		tenant = "benchmark"
	}
	return memory.Scope{Tenant: tenant, Project: "benchmark-cli", Namespace: "offline"}
}

func run() error {
	cfg, err := config.LoadFromEnv()
	if err != nil {
		return err
	}

	runner, err := newRunner(cfg)
	if err != nil {
		return err
	}

	return runner.Start(context.Background())
}
