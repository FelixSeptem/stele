package benchmark

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
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
	StrategyLexical  RetrievalStrategy = "lexical"
	StrategySemantic RetrievalStrategy = "semantic"
	StrategyHybrid   RetrievalStrategy = "hybrid"
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

type RunPolicy struct {
	Mode         RunMode `json:"mode"`
	Split        string  `json:"split"`
	Seed         int64   `json:"seed,omitempty"`
	QueryBudget  int     `json:"query_budget,omitempty"`
	Reproducible bool    `json:"reproducible"`
}

func BuildRunPolicy(manifest DatasetManifest, config RunConfig) (RunPolicy, error) {
	if err := config.Validate(); err != nil {
		return RunPolicy{}, &StatusError{Status: StatusInvalidManifest, Message: "validate benchmark run policy", Cause: err}
	}
	split := "smoke"
	if config.Mode == RunModeLocalFull || config.Mode == RunModeReproducibleExtended {
		split = "full"
	}
	spec, ok := manifest.Splits[split]
	if !ok {
		return RunPolicy{}, &StatusError{Status: StatusPrerequisiteMissing, Message: fmt.Sprintf("benchmark split %q is not declared", split)}
	}
	return RunPolicy{Mode: config.Mode, Split: split, Seed: config.Seed, QueryBudget: spec.MaxQueries, Reproducible: config.Mode == RunModeReproducibleExtended}, nil
}

func SelectQueries(queries []BenchmarkQuery, split SplitSpec, config RunConfig) ([]BenchmarkQuery, error) {
	if err := config.Validate(); err != nil {
		return nil, err
	}
	selected := append([]BenchmarkQuery(nil), queries...)
	sort.SliceStable(selected, func(i, j int) bool { return selected[i].ID < selected[j].ID })
	if split.MaxQueries > 0 && len(selected) > split.MaxQueries {
		selected = selected[:split.MaxQueries]
	}
	return selected, nil
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
	case StrategyLexical, StrategySemantic, StrategyHybrid:
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
	paths, err := cache.Paths(manifest.Name, manifest.Version)
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
