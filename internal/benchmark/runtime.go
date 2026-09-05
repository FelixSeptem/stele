package benchmark

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

type RunMode string

const (
	RunModeSmoke                RunMode = "smoke"
	RunModeLocalFull            RunMode = "local-full"
	RunModeReproducibleExtended RunMode = "reproducible-extended"
)

type RetrievalStrategy string

const (
	StrategyLexical    RetrievalStrategy = "lexical"
	StrategySemantic   RetrievalStrategy = "semantic"
	StrategyHybrid     RetrievalStrategy = "hybrid"
	StrategyChunk      RetrievalStrategy = "chunk"
	StrategyHybridRank RetrievalStrategy = "hybrid-rank"
	StrategyReranker   RetrievalStrategy = "reranker"
)

type RunConfig struct {
	DataDir   string
	Dataset   string
	Version   string
	Offline   bool
	Mode      RunMode
	Strategy  RetrievalStrategy
	Embedding EmbeddingProfile
	Seed      int64
}

func LoadRunConfigFromEnv() (RunConfig, error) {
	offline := true
	if value := strings.TrimSpace(os.Getenv("STELE_BENCHMARK_OFFLINE")); value != "" {
		parsed, err := strconv.ParseBool(value)
		if err != nil {
			return RunConfig{}, fmt.Errorf("parse STELE_BENCHMARK_OFFLINE: %w", err)
		}
		offline = parsed
	}
	config := RunConfig{
		DataDir:  strings.TrimSpace(os.Getenv("STELE_BENCHMARK_DATA_DIR")),
		Dataset:  strings.TrimSpace(os.Getenv("STELE_BENCHMARK_DATASET")),
		Version:  strings.TrimSpace(os.Getenv("STELE_BENCHMARK_DATA_VERSION")),
		Offline:  offline,
		Mode:     RunModeSmoke,
		Strategy: StrategyLexical,
	}
	if value := strings.TrimSpace(os.Getenv("STELE_BENCHMARK_MODE")); value != "" {
		config.Mode = RunMode(value)
	}
	if value := strings.TrimSpace(os.Getenv("STELE_BENCHMARK_STRATEGY")); value != "" {
		config.Strategy = RetrievalStrategy(value)
	}
	if value := strings.TrimSpace(os.Getenv("STELE_BENCHMARK_SEED")); value != "" {
		seed, err := strconv.ParseInt(value, 10, 64)
		if err != nil {
			return RunConfig{}, fmt.Errorf("parse STELE_BENCHMARK_SEED: %w", err)
		}
		config.Seed = seed
	}
	if err := config.Validate(); err != nil {
		return RunConfig{}, err
	}
	return config, nil
}

func (c RunConfig) Validate() error {
	if strings.TrimSpace(c.DataDir) == "" || strings.TrimSpace(c.Dataset) == "" || strings.TrimSpace(c.Version) == "" {
		return fmt.Errorf("benchmark data directory, dataset, and version are required")
	}
	switch c.Mode {
	case RunModeSmoke, RunModeLocalFull, RunModeReproducibleExtended:
	default:
		return fmt.Errorf("unsupported benchmark run mode %q", c.Mode)
	}
	switch c.Strategy {
	case StrategyLexical, StrategySemantic, StrategyHybrid, StrategyChunk, StrategyHybridRank, StrategyReranker:
	default:
		return fmt.Errorf("unsupported benchmark strategy %q", c.Strategy)
	}
	if c.Mode == RunModeReproducibleExtended && c.Seed == 0 {
		return fmt.Errorf("reproducible-extended benchmark runs require a non-zero random seed")
	}
	return nil
}

type AdmissionResult struct {
	Status        Status   `json:"status"`
	Prerequisites []string `json:"prerequisites,omitempty"`
	LexicalOnly   bool     `json:"lexical_only"`
}

func AdmitRun(cache Cache, manifest DatasetManifest, config RunConfig) AdmissionResult {
	if err := manifest.Validate(); err != nil {
		return AdmissionResult{Status: StatusInvalidManifest, Prerequisites: []string{err.Error()}}
	}
	if err := config.Validate(); err != nil {
		return AdmissionResult{Status: StatusInvalidManifest, Prerequisites: []string{err.Error()}}
	}
	if config.Dataset != manifest.Name || config.Version != manifest.Version || filepath.Clean(config.DataDir) != filepath.Clean(cache.DataDir) {
		return AdmissionResult{Status: StatusInvalidManifest, Prerequisites: []string{"run configuration must match dataset manifest and cache"}}
	}
	split := "smoke"
	if config.Mode == RunModeLocalFull || config.Mode == RunModeReproducibleExtended {
		split = "full"
	}
	if _, _, err := cache.LoadNormalized(manifest, split); err != nil {
		return AdmissionResult{Status: StatusOf(err), Prerequisites: []string{err.Error()}}
	}
	if config.Strategy == StrategyLexical {
		return AdmissionResult{Status: StatusSuccess, LexicalOnly: true}
	}
	if err := embeddingCompatible(manifest.Embedding, config.Embedding); err != nil {
		return AdmissionResult{Status: StatusPrerequisiteMissing, Prerequisites: []string{err.Error()}}
	}
	paths, err := cache.ManifestPaths(manifest)
	if err != nil {
		return AdmissionResult{Status: StatusOf(err), Prerequisites: []string{err.Error()}}
	}
	vectorPath := filepath.Join(paths.Embeddings, filepath.Base(manifest.Embedding.VectorSource))
	if strings.TrimSpace(manifest.Embedding.VectorSource) == "" {
		return AdmissionResult{Status: StatusPrerequisiteMissing, Prerequisites: []string{"embedding vector source is required for semantic or hybrid retrieval"}}
	}
	if _, err := os.Stat(vectorPath); err != nil {
		return AdmissionResult{Status: StatusPrerequisiteMissing, Prerequisites: []string{"embedding vectors are missing: " + vectorPath}}
	}
	return AdmissionResult{Status: StatusSuccess}
}

func embeddingCompatible(expected, actual EmbeddingProfile) error {
	if strings.TrimSpace(expected.Name) == "" || strings.TrimSpace(actual.Name) == "" {
		return fmt.Errorf("embedding profile is required for semantic or hybrid retrieval")
	}
	if expected.Name != actual.Name || expected.Model != actual.Model || expected.Revision != actual.Revision || expected.Dimensions != actual.Dimensions || expected.Normalization != actual.Normalization {
		return fmt.Errorf("embedding profile does not match manifest")
	}
	return nil
}
