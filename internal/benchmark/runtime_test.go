package benchmark

import "testing"

func TestRunConfigDefaultsToOfflineSmoke(t *testing.T) {
	t.Setenv("STELE_BENCHMARK_DATA_DIR", t.TempDir())
	t.Setenv("STELE_BENCHMARK_DATASET", "locomo")
	t.Setenv("STELE_BENCHMARK_DATA_VERSION", "synthetic-smoke-v1")
	t.Setenv("STELE_BENCHMARK_OFFLINE", "")
	config, err := LoadRunConfigFromEnv()
	if err != nil {
		t.Fatal(err)
	}
	if !config.Offline || config.Mode != RunModeSmoke {
		t.Fatalf("expected offline smoke defaults, got %#v", config)
	}
}

func TestExtendedRunRequiresExplicitSeed(t *testing.T) {
	t.Setenv("STELE_BENCHMARK_DATA_DIR", t.TempDir())
	t.Setenv("STELE_BENCHMARK_DATASET", "locomo")
	t.Setenv("STELE_BENCHMARK_DATA_VERSION", "v1")
	t.Setenv("STELE_BENCHMARK_MODE", string(RunModeReproducibleExtended))
	t.Setenv("STELE_BENCHMARK_SEED", "")
	if _, err := LoadRunConfigFromEnv(); err == nil {
		t.Fatal("expected reproducible extended mode to require a seed")
	}
	t.Setenv("STELE_BENCHMARK_SEED", "42")
	config, err := LoadRunConfigFromEnv()
	if err != nil || config.Seed != 42 {
		t.Fatalf("expected parsed reproducible seed, config=%#v err=%v", config, err)
	}
}

func TestAdmissionRejectsMissingNormalizedCorpusWithoutFallback(t *testing.T) {
	cache := NewCache(t.TempDir())
	manifest := validManifest()
	config := RunConfig{DataDir: cache.DataDir, Dataset: manifest.Name, Version: manifest.Version, Offline: true, Mode: RunModeSmoke, Strategy: StrategyLexical}
	result := AdmitRun(cache, manifest, config)
	if result.Status != StatusPrerequisiteMissing || len(result.Prerequisites) == 0 {
		t.Fatalf("expected missing normalized corpus diagnostic, got %#v", result)
	}
}

func TestAdmissionRejectsSemanticProfileMismatch(t *testing.T) {
	cache := NewCache(t.TempDir())
	manifest := validManifest()
	manifest.Embedding = EmbeddingProfile{Name: "local", Model: "model", Revision: "r1", Dimensions: 768, Normalization: "l2", VectorSource: "vectors.json"}
	corpus := NormalizedCorpus{Events: []MemoryEventRecord{{ID: "e1", Scope: memoryScope(), Text: "fact"}}}
	if _, err := cache.WriteNormalized(manifest, "smoke", corpus); err != nil {
		t.Fatal(err)
	}
	config := RunConfig{DataDir: cache.DataDir, Dataset: manifest.Name, Version: manifest.Version, Offline: true, Mode: RunModeSmoke, Strategy: StrategySemantic, Embedding: EmbeddingProfile{Name: "local", Model: "model", Revision: "r1", Dimensions: 1536, Normalization: "l2", VectorSource: "vectors.json"}}
	result := AdmitRun(cache, manifest, config)
	if result.Status != StatusPrerequisiteMissing {
		t.Fatalf("expected missing semantic prerequisites, got %#v", result)
	}
}

func TestLexicalSmokeAllowsAbsentEmbeddingProfile(t *testing.T) {
	cache := NewCache(t.TempDir())
	manifest := validManifest()
	corpus := NormalizedCorpus{Events: []MemoryEventRecord{{ID: "e1", Scope: memoryScope(), Text: "fact"}}}
	if _, err := cache.WriteNormalized(manifest, "smoke", corpus); err != nil {
		t.Fatal(err)
	}
	result := AdmitRun(cache, manifest, RunConfig{DataDir: cache.DataDir, Dataset: manifest.Name, Version: manifest.Version, Offline: true, Mode: RunModeSmoke, Strategy: StrategyLexical})
	if result.Status != StatusSuccess || !result.LexicalOnly {
		t.Fatalf("expected lexical-only smoke admission, got %#v", result)
	}
}

func TestLocalFullAndExtendedUseFullSplit(t *testing.T) {
	cache := NewCache(t.TempDir())
	manifest := validManifest()
	manifest.Splits["full"] = SplitSpec{Source: "full.jsonl"}
	corpus := NormalizedCorpus{Events: []MemoryEventRecord{{ID: "e1", Scope: memoryScope(), Text: "fact"}}}
	if _, err := cache.WriteNormalized(manifest, "full", corpus); err != nil {
		t.Fatal(err)
	}
	for _, mode := range []RunMode{RunModeLocalFull, RunModeReproducibleExtended} {
		result := AdmitRun(cache, manifest, RunConfig{DataDir: cache.DataDir, Dataset: manifest.Name, Version: manifest.Version, Offline: true, Mode: mode, Strategy: StrategyLexical, Seed: 42})
		if result.Status != StatusSuccess {
			t.Fatalf("expected %s to use full split, got %#v", mode, result)
		}
	}
}

func TestAdmissionRejectsQrelsThatReferenceUnknownQueryOrEvidence(t *testing.T) {
	cache := NewCache(t.TempDir())
	manifest := validManifest()
	corpus := NormalizedCorpus{
		Events:  []MemoryEventRecord{{ID: "e1", Scope: memoryScope(), Text: "fact"}},
		Queries: []BenchmarkQuery{{ID: "q1", Scope: memoryScope(), Text: "question"}},
		QRELs:   []QREL{{QueryID: "q-missing", EvidenceID: "e1", Grade: 1}},
	}
	if _, err := cache.WriteNormalized(manifest, "smoke", corpus); err == nil {
		t.Fatal("expected invalid qrels to be rejected before cache write")
	}
}
