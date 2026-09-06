package benchmark

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/FelixSeptem/stele/internal/memory"
)

type BenchmarkReport struct {
	SchemaVersion      string             `json:"schema_version"`
	Dataset            string             `json:"dataset"`
	Version            string             `json:"version"`
	Family             string             `json:"family"`
	Split              string             `json:"split"`
	ManifestChecksum   string             `json:"manifest_checksum"`
	NormalizedChecksum string             `json:"normalized_checksum"`
	QRELChecksum       string             `json:"qrels_checksum"`
	Embedding          EmbeddingProfile   `json:"embedding"`
	Strategy           StrategyProfile    `json:"strategy"`
	RunID              string             `json:"run_id"`
	Scope              memory.Scope       `json:"scope"`
	Status             Status             `json:"status"`
	Prerequisites      []string           `json:"prerequisites,omitempty"`
	Metrics            map[string]float64 `json:"metrics"`
	Errors             []string           `json:"errors,omitempty"`
	SafetyFailures     []string           `json:"safety_failures,omitempty"`
	Artifacts          []string           `json:"artifacts,omitempty"`
}

func (r BenchmarkReport) Validate() error {
	if strings.TrimSpace(r.SchemaVersion) != "" && r.SchemaVersion != SchemaVersion {
		return fmt.Errorf("unsupported benchmark report schema %q", r.SchemaVersion)
	}
	for name, value := range map[string]string{"dataset": r.Dataset, "version": r.Version, "family": r.Family, "split": r.Split, "manifest_checksum": r.ManifestChecksum, "normalized_checksum": r.NormalizedChecksum, "qrels_checksum": r.QRELChecksum, "run_id": r.RunID} {
		if strings.TrimSpace(value) == "" {
			return fmt.Errorf("benchmark report %s is required", name)
		}
	}
	if err := r.Scope.Validate(); err != nil {
		return fmt.Errorf("benchmark report scope: %w", err)
	}
	if err := r.Embedding.Validate(); err != nil {
		return err
	}
	if err := r.Strategy.Validate(); err != nil {
		return err
	}
	if r.Status == "" {
		return fmt.Errorf("benchmark report status is required")
	}
	return nil
}

func MarshalBenchmarkReport(report BenchmarkReport) ([]byte, error) {
	report.SchemaVersion = SchemaVersion
	if err := report.Validate(); err != nil {
		return nil, &StatusError{Status: StatusInvalidManifest, Message: "validate benchmark report", Cause: err}
	}
	return json.Marshal(report)
}
