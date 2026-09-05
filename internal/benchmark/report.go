package benchmark

import (
	"fmt"

	"github.com/FelixSeptem/stele/internal/memory"
)

// FamilyReport is the shared, machine-readable envelope. Family-specific
// metric payloads live under Metrics and are intentionally never averaged
// across families.
type FamilyReport struct {
	SchemaVersion     string            `json:"schema_version"`
	Family            DatasetFamily     `json:"family"`
	Dataset           string            `json:"dataset"`
	Version           string            `json:"version"`
	Split             string            `json:"split"`
	ManifestSHA256    string            `json:"manifest_sha256"`
	QRELChecksum      string            `json:"qrels_checksum,omitempty"`
	QRELVersion       string            `json:"qrels_version,omitempty"`
	ConversionVersion string            `json:"conversion_version"`
	Embedding         EmbeddingProfile  `json:"embedding_profile"`
	StrategyProfile   string            `json:"strategy_profile,omitempty"`
	InputChecksums    map[string]string `json:"input_checksums,omitempty"`
	Runtime           RuntimeIdentity   `json:"runtime"`
	Scope             memory.Scope      `json:"run_scope"`
	Status            Status            `json:"status"`
	Metrics           any               `json:"metrics,omitempty"`
	Errors            []string          `json:"errors,omitempty"`
	SafetyOutcomes    any               `json:"safety_outcomes,omitempty"`
	ArtifactPaths     []string          `json:"artifact_paths,omitempty"`
	NonGating         bool              `json:"non_gating"`
}

// RuntimeIdentity distinguishes service and storage execution environments in
// retained reports without requiring a database for fixture-only runs.
type RuntimeIdentity struct {
	SteleRevision string `json:"stele_revision,omitempty"`
	PostgreSQL    string `json:"postgresql,omitempty"`
	PGVector      string `json:"pgvector,omitempty"`
}

type FamilyReportExecution struct {
	QRELVersion     string            `json:"qrels_version,omitempty"`
	StrategyProfile string            `json:"strategy_profile,omitempty"`
	InputChecksums  map[string]string `json:"input_checksums,omitempty"`
	Runtime         RuntimeIdentity   `json:"runtime"`
}

// FamilyReportSummary is a rendering-oriented view for CLIs and human-facing
// reports. It makes the family boundary visible instead of implying that
// memory, contract, generic IR, and stress metrics share a common scale.
type FamilyReportSummary struct {
	Family             DatasetFamily   `json:"family"`
	Dataset            string          `json:"dataset"`
	Status             Status          `json:"status"`
	GateClassification string          `json:"gate_classification"`
	NotComparableWith  []DatasetFamily `json:"not_comparable_with"`
}

func RenderFamilyReport(report FamilyReport) FamilyReportSummary {
	summary := FamilyReportSummary{Family: report.Family, Dataset: report.Dataset, Status: report.Status, GateClassification: "family-scoped"}
	if report.NonGating || report.Family == FamilyStress {
		summary.GateClassification = "non-gating"
	}
	for _, family := range []DatasetFamily{FamilyAgentMemory, FamilyContract, FamilySpecialized, FamilyGenericRetrieval, FamilyStress} {
		if family != report.Family {
			summary.NotComparableWith = append(summary.NotComparableWith, family)
		}
	}
	return summary
}

func NewFamilyReport(family DatasetFamily, manifest DatasetManifest, split string, scope memory.Scope) FamilyReport {
	return FamilyReport{SchemaVersion: SchemaVersion, Family: family, Dataset: manifest.Name, Version: manifest.Version, Split: split, ManifestSHA256: manifest.SHA256, QRELChecksum: manifest.QRELChecksum, ConversionVersion: manifest.ConversionVersion, Embedding: manifest.Embedding, Scope: scope, Status: StatusSuccess, NonGating: family == FamilyStress}
}

func (r FamilyReport) WithExecutionProvenance(execution FamilyReportExecution) FamilyReport {
	r.QRELVersion = execution.QRELVersion
	r.StrategyProfile = execution.StrategyProfile
	r.Runtime = execution.Runtime
	if len(execution.InputChecksums) == 0 {
		r.InputChecksums = nil
		return r
	}
	r.InputChecksums = make(map[string]string, len(execution.InputChecksums))
	for name, checksum := range execution.InputChecksums {
		r.InputChecksums[name] = checksum
	}
	return r
}

func (r FamilyReport) ValidateAgainst(manifest DatasetManifest) error {
	if r.SchemaVersion != SchemaVersion || !r.Family.Valid() {
		return &StatusError{Status: StatusInvalidManifest, Message: "report schema and family are required"}
	}
	if err := r.Scope.Validate(); err != nil {
		return &StatusError{Status: StatusInvalidManifest, Message: "validate report scope", Cause: err}
	}
	if r.Dataset != manifest.Name || r.Version != manifest.Version || r.ManifestSHA256 != manifest.SHA256 || r.ConversionVersion != manifest.ConversionVersion || r.QRELChecksum != manifest.QRELChecksum {
		return &StatusError{Status: StatusInvalidManifest, Message: "report inputs are incompatible with manifest"}
	}
	return nil
}

func CanCompareFamilyReports(left, right FamilyReport) bool {
	return left.Family == right.Family && left.Dataset == right.Dataset && left.Version == right.Version && left.Split == right.Split && left.ManifestSHA256 == right.ManifestSHA256 && left.QRELChecksum == right.QRELChecksum && left.QRELVersion == right.QRELVersion && left.ConversionVersion == right.ConversionVersion && left.Embedding == right.Embedding && left.StrategyProfile == right.StrategyProfile && equalChecksums(left.InputChecksums, right.InputChecksums)
}

func equalChecksums(left, right map[string]string) bool {
	if len(left) != len(right) {
		return false
	}
	for name, checksum := range left {
		if right[name] != checksum {
			return false
		}
	}
	return true
}

func (r FamilyReport) String() string { return fmt.Sprintf("%s/%s:%s", r.Family, r.Dataset, r.Status) }
