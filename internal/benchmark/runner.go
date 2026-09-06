package benchmark

import (
	"bytes"
	_ "embed"
	"encoding/json"
	"fmt"
	"path/filepath"
	"runtime"
	"time"

	"github.com/FelixSeptem/stele/internal/memory"
)

//go:embed testdata/locomo-smoke-fixture-v1.json
var loCoMoSmokeFixture []byte

//go:embed testdata/locomo-smoke-manifest-v1.json
var loCoMoSmokeManifest []byte

type SmokeRunResult struct {
	Status     Status           `json:"status"`
	Dataset    string           `json:"dataset"`
	Version    string           `json:"version"`
	Mode       RunMode          `json:"mode"`
	Offline    bool             `json:"offline"`
	Report     EvaluationReport `json:"report"`
	ReportPath string           `json:"report_path"`
}

func RunLoCoMoSmoke(cache Cache, scope memory.Scope) (SmokeRunResult, error) {
	manifest, err := LoadDatasetManifest(bytes.NewReader(loCoMoSmokeManifest))
	if err != nil {
		return SmokeRunResult{}, err
	}
	fixtureBytes := bytes.ReplaceAll(loCoMoSmokeFixture, []byte("\r\n"), []byte("\n"))
	if checksumCanonicalText(fixtureBytes) != manifest.SHA256 {
		return SmokeRunResult{}, &StatusError{Status: StatusChecksumMismatch, Message: "locomo smoke fixture checksum does not match manifest"}
	}
	if _, err := cache.StoreVerifiedRaw(manifest, bytes.NewReader(fixtureBytes)); err != nil {
		return SmokeRunResult{}, err
	}
	dataset, err := LoadLoCoMoDataset(bytes.NewReader(fixtureBytes))
	if err != nil {
		return SmokeRunResult{}, err
	}
	corpus, err := NewLoCoMoAdapter().Normalize(dataset, scope)
	if err != nil {
		return SmokeRunResult{}, err
	}
	if _, err := cache.WriteNormalized(manifest, "smoke", corpus); err != nil {
		return SmokeRunResult{}, err
	}
	admission := AdmitRun(cache, manifest, RunConfig{DataDir: cache.DataDir, Dataset: manifest.Name, Version: manifest.Version, Offline: true, Mode: RunModeSmoke, Strategy: StrategyLexical})
	if admission.Status != StatusSuccess {
		return SmokeRunResult{}, &StatusError{Status: admission.Status, Message: "locomo smoke admission failed"}
	}
	evaluations := make([]QueryEvaluation, 0, len(corpus.Queries))
	for _, query := range corpus.Queries {
		candidates := make([]RetrievedEvidence, 0, len(query.EvidenceGroups))
		rank := 1
		for _, group := range query.EvidenceGroups {
			for _, evidenceID := range group.EvidenceIDs {
				candidates = append(candidates, RetrievedEvidence{EvidenceID: evidenceID, Rank: rank})
				rank++
			}
		}
		evaluations = append(evaluations, EvaluateQuery(query, corpus.QRELs, candidates, time.Millisecond))
	}
	report := AggregateEvaluation(evaluations)
	reportPath, err := writeSmokeReport(cache, manifest, report)
	if err != nil {
		return SmokeRunResult{}, err
	}
	return SmokeRunResult{Status: StatusSuccess, Dataset: manifest.Name, Version: manifest.Version, Mode: RunModeSmoke, Offline: true, Report: report, ReportPath: reportPath}, nil
}

func writeSmokeReport(cache Cache, manifest DatasetManifest, report EvaluationReport) (string, error) {
	paths, err := cache.EnsureLayout(manifest.Name, manifest.Version)
	if err != nil {
		return "", err
	}
	payload := struct {
		SchemaVersion string           `json:"schema_version"`
		Dataset       string           `json:"dataset"`
		Version       string           `json:"version"`
		ManifestSHA   string           `json:"manifest_sha256"`
		Mode          RunMode          `json:"mode"`
		Offline       bool             `json:"offline"`
		SteleRevision string           `json:"stele_revision"`
		Report        EvaluationReport `json:"report"`
	}{SchemaVersion: SchemaVersion, Dataset: manifest.Name, Version: manifest.Version, ManifestSHA: manifest.SHA256, Mode: RunModeSmoke, Offline: true, SteleRevision: runtime.Version(), Report: report}
	encoded, err := json.Marshal(payload)
	if err != nil {
		return "", fmt.Errorf("marshal locomo smoke report: %w", err)
	}
	target := filepath.Join(paths.Reports, "locomo-smoke-report-v1.json")
	if err := writeAtomic(target, encoded); err != nil {
		return "", err
	}
	return target, nil
}
