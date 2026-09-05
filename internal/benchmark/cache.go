package benchmark

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/FelixSeptem/stele/internal/memory"
)

type Status string

const (
	StatusSuccess             Status = "success"
	StatusQualityGateFailed   Status = "quality_gate_failed"
	StatusPrerequisiteMissing Status = "prerequisite_missing"
	StatusInvalidManifest     Status = "invalid_manifest"
	StatusChecksumMismatch    Status = "checksum_mismatch"
	StatusCapacityRefused     Status = "capacity_refused"
	StatusInternalError       Status = "internal_error"
)

type StatusError struct {
	Status  Status
	Message string
	Cause   error
}

func (e *StatusError) Error() string {
	if e.Cause == nil {
		return e.Message
	}
	return e.Message + ": " + e.Cause.Error()
}

func (e *StatusError) Unwrap() error { return e.Cause }

func StatusOf(err error) Status {
	var statusErr *StatusError
	if errors.As(err, &statusErr) {
		return statusErr.Status
	}
	return StatusInternalError
}

type Cache struct {
	DataDir string
	client  *http.Client
}

type CachePaths struct {
	Root       string
	Family     string
	Raw        string
	Normalized string
	Embeddings string
	Reports    string
}

type NormalizedMetadata struct {
	SchemaVersion string        `json:"schema_version"`
	Family        DatasetFamily `json:"family"`
	Dataset       string        `json:"dataset"`
	Version       string        `json:"version"`
	Split         string        `json:"split"`
	Checksum      string        `json:"checksum"`
	QRELChecksum  string        `json:"qrels_checksum"`
}

type CacheLock struct {
	SchemaVersion    string `json:"schema_version"`
	Dataset          string `json:"dataset"`
	Version          string `json:"version"`
	SHA256           string `json:"sha256"`
	UpstreamURL      string `json:"upstream_url"`
	UpstreamRevision string `json:"upstream_revision"`
	FetchedAt        string `json:"fetched_at"`
}

type RunCleanupResult struct {
	EmbeddingsRemoved bool `json:"embeddings_removed"`
	ReportRetained    bool `json:"report_retained"`
}

func NewCache(dataDir string) Cache {
	return Cache{DataDir: strings.TrimSpace(dataDir), client: &http.Client{Timeout: 30 * time.Second}}
}

func (c Cache) Paths(dataset, version string) (CachePaths, error) {
	if strings.TrimSpace(c.DataDir) == "" || strings.TrimSpace(dataset) == "" || strings.TrimSpace(version) == "" {
		return CachePaths{}, &StatusError{Status: StatusInvalidManifest, Message: "data directory, dataset, and version are required"}
	}
	root := filepath.Join(c.DataDir, dataset, version)
	return CachePaths{Root: root, Raw: filepath.Join(root, "raw"), Normalized: filepath.Join(root, "normalized"), Embeddings: filepath.Join(root, "embeddings"), Reports: filepath.Join(root, "reports")}, nil
}

func (c Cache) EnsureLayout(dataset, version string) (CachePaths, error) {
	paths, err := c.Paths(dataset, version)
	if err != nil {
		return CachePaths{}, err
	}
	for _, path := range []string{paths.Raw, paths.Normalized, paths.Embeddings, paths.Reports} {
		if err := os.MkdirAll(path, 0o755); err != nil {
			return CachePaths{}, fmt.Errorf("create benchmark cache layout: %w", err)
		}
	}
	return paths, nil
}

// ManifestPaths is the family-aware cache layout used by new benchmark
// families. The legacy Paths method remains available for pre-family callers.
func (c Cache) ManifestPaths(manifest DatasetManifest) (CachePaths, error) {
	if err := manifest.Validate(); err != nil {
		return CachePaths{}, &StatusError{Status: StatusInvalidManifest, Message: "validate dataset manifest", Cause: err}
	}
	if strings.TrimSpace(c.DataDir) == "" {
		return CachePaths{}, &StatusError{Status: StatusInvalidManifest, Message: "benchmark data directory is required"}
	}
	root := filepath.Join(c.DataDir, string(manifest.Family), manifest.Name, manifest.Version)
	return CachePaths{Root: root, Family: string(manifest.Family), Raw: filepath.Join(root, "raw"), Normalized: filepath.Join(root, "normalized"), Embeddings: filepath.Join(root, "embeddings"), Reports: filepath.Join(root, "reports")}, nil
}

func (c Cache) EnsureManifestLayout(manifest DatasetManifest) (CachePaths, error) {
	paths, err := c.ManifestPaths(manifest)
	if err != nil {
		return CachePaths{}, err
	}
	for _, path := range []string{paths.Raw, paths.Normalized, paths.Embeddings, paths.Reports} {
		if err := os.MkdirAll(path, 0o755); err != nil {
			return CachePaths{}, fmt.Errorf("create family benchmark cache layout: %w", err)
		}
	}
	return paths, nil
}

// CleanRunArtifacts removes only the named run's embedding directory. Locked
// raw inputs and normalized shared corpus are never targets; reports stay in
// place when retention is requested. Database scope cleanup remains owned by
// the benchmark runtime, not this filesystem-only cache helper.
func (c Cache) CleanRunArtifacts(manifest DatasetManifest, runID string, retainReport bool) (RunCleanupResult, error) {
	if !safeRunArtifactID(runID) {
		return RunCleanupResult{}, &StatusError{Status: StatusInvalidManifest, Message: "benchmark run id is unsafe"}
	}
	paths, err := c.ManifestPaths(manifest)
	if err != nil {
		return RunCleanupResult{}, err
	}
	target := filepath.Join(paths.Embeddings, runID)
	relative, err := filepath.Rel(paths.Embeddings, target)
	if err != nil || relative == "." || strings.HasPrefix(relative, "..") || filepath.IsAbs(relative) {
		return RunCleanupResult{}, &StatusError{Status: StatusInvalidManifest, Message: "benchmark cleanup target escaped embeddings directory"}
	}
	result := RunCleanupResult{}
	if _, err := os.Stat(target); err == nil {
		if err := os.RemoveAll(target); err != nil {
			return RunCleanupResult{}, fmt.Errorf("remove benchmark run embeddings: %w", err)
		}
		result.EmbeddingsRemoved = true
	} else if !errors.Is(err, os.ErrNotExist) {
		return RunCleanupResult{}, fmt.Errorf("stat benchmark run embeddings: %w", err)
	}
	reportPath := filepath.Join(paths.Reports, runID+".json")
	if retainReport {
		if _, err := os.Stat(reportPath); err == nil {
			result.ReportRetained = true
		} else if !errors.Is(err, os.ErrNotExist) {
			return RunCleanupResult{}, fmt.Errorf("stat retained benchmark report: %w", err)
		}
	} else if err := os.Remove(reportPath); err != nil && !errors.Is(err, os.ErrNotExist) {
		return RunCleanupResult{}, fmt.Errorf("remove benchmark report: %w", err)
	}
	return result, nil
}

// WriteFamilyReport stores one machine-readable report under the manifest's
// family-specific cache path. Reports are accepted only for a scope derived by
// NewRunScope, so a benchmark artifact can never be retained on behalf of a
// production project or namespace.
func (c Cache) WriteFamilyReport(manifest DatasetManifest, runID string, report FamilyReport) (string, error) {
	if err := manifest.Validate(); err != nil {
		return "", &StatusError{Status: StatusInvalidManifest, Message: "validate dataset manifest", Cause: err}
	}
	if !safeRunArtifactID(runID) {
		return "", &StatusError{Status: StatusInvalidManifest, Message: "benchmark run id is unsafe"}
	}
	if _, found := manifest.Splits[report.Split]; !found {
		return "", &StatusError{Status: StatusInvalidManifest, Message: "report split is not declared by manifest"}
	}
	if report.Family != manifest.Family {
		return "", &StatusError{Status: StatusInvalidManifest, Message: "report family does not match manifest"}
	}
	if err := report.ValidateAgainst(manifest); err != nil {
		return "", err
	}
	expectedRun, err := NewRunScope(memory.Scope{Tenant: report.Scope.Tenant, Project: "benchmark-base", Namespace: "benchmark-base"}, manifest.Name, runID)
	if err != nil {
		return "", &StatusError{Status: StatusInvalidManifest, Message: "derive benchmark report scope", Cause: err}
	}
	if report.Scope != expectedRun.Scope {
		return "", &StatusError{Status: StatusInvalidManifest, Message: "report scope is not the derived benchmark run scope"}
	}
	paths, err := c.EnsureManifestLayout(manifest)
	if err != nil {
		return "", err
	}
	target := filepath.Join(paths.Reports, runID+".json")
	report.ArtifactPaths = []string{target}
	encoded, err := json.Marshal(report)
	if err != nil {
		return "", fmt.Errorf("marshal benchmark family report: %w", err)
	}
	if err := writeAtomic(target, encoded); err != nil {
		return "", fmt.Errorf("write benchmark family report: %w", err)
	}
	return target, nil
}

// LoadFamilyReport reads a retained report only when it still matches the
// supplied locked manifest and the exact derived benchmark run scope.
func (c Cache) LoadFamilyReport(manifest DatasetManifest, runID string) (FamilyReport, error) {
	if err := manifest.Validate(); err != nil {
		return FamilyReport{}, &StatusError{Status: StatusInvalidManifest, Message: "validate dataset manifest", Cause: err}
	}
	if !safeRunArtifactID(runID) {
		return FamilyReport{}, &StatusError{Status: StatusInvalidManifest, Message: "benchmark run id is unsafe"}
	}
	paths, err := c.ManifestPaths(manifest)
	if err != nil {
		return FamilyReport{}, err
	}
	encoded, err := os.ReadFile(filepath.Join(paths.Reports, runID+".json"))
	if errors.Is(err, os.ErrNotExist) {
		return FamilyReport{}, &StatusError{Status: StatusPrerequisiteMissing, Message: "retained benchmark report is missing"}
	}
	if err != nil {
		return FamilyReport{}, fmt.Errorf("read benchmark family report: %w", err)
	}
	var report FamilyReport
	if err := json.Unmarshal(encoded, &report); err != nil {
		return FamilyReport{}, &StatusError{Status: StatusInvalidManifest, Message: "decode benchmark family report", Cause: err}
	}
	if report.Family != manifest.Family {
		return FamilyReport{}, &StatusError{Status: StatusInvalidManifest, Message: "report family does not match manifest"}
	}
	if err := report.ValidateAgainst(manifest); err != nil {
		return FamilyReport{}, err
	}
	expectedRun, err := NewRunScope(memory.Scope{Tenant: report.Scope.Tenant, Project: "benchmark-base", Namespace: "benchmark-base"}, manifest.Name, runID)
	if err != nil {
		return FamilyReport{}, &StatusError{Status: StatusInvalidManifest, Message: "derive benchmark report scope", Cause: err}
	}
	if report.Scope != expectedRun.Scope {
		return FamilyReport{}, &StatusError{Status: StatusInvalidManifest, Message: "report scope is not the derived benchmark run scope"}
	}
	return report, nil
}

func safeRunArtifactID(value string) bool {
	if strings.TrimSpace(value) == "" || value != filepath.Base(value) || value == "." || value == ".." {
		return false
	}
	for _, character := range value {
		if !(character >= 'a' && character <= 'z') && !(character >= 'A' && character <= 'Z') && !(character >= '0' && character <= '9') && character != '-' && character != '_' {
			return false
		}
	}
	return true
}

func (c Cache) StoreVerifiedRaw(manifest DatasetManifest, source io.Reader) (string, error) {
	if err := manifest.Validate(); err != nil {
		return "", &StatusError{Status: StatusInvalidManifest, Message: "validate dataset manifest", Cause: err}
	}
	if source == nil {
		return "", &StatusError{Status: StatusPrerequisiteMissing, Message: "raw source is required"}
	}
	paths, err := c.EnsureManifestLayout(manifest)
	if err != nil {
		return "", err
	}
	target := filepath.Join(paths.Raw, filepath.Base(manifest.SourcePath))
	temporary, err := os.CreateTemp(paths.Raw, ".fetch-*")
	if err != nil {
		return "", fmt.Errorf("create temporary raw cache file: %w", err)
	}
	temporaryName := temporary.Name()
	defer os.Remove(temporaryName)
	hasher := sha256.New()
	if _, err := io.Copy(io.MultiWriter(temporary, hasher), source); err != nil {
		temporary.Close()
		return "", fmt.Errorf("write raw cache data: %w", err)
	}
	if err := temporary.Close(); err != nil {
		return "", fmt.Errorf("close raw cache data: %w", err)
	}
	actual := hex.EncodeToString(hasher.Sum(nil))
	if !strings.EqualFold(actual, manifest.SHA256) {
		return "", &StatusError{Status: StatusChecksumMismatch, Message: "raw dataset checksum does not match manifest"}
	}
	if err := os.Rename(temporaryName, target); err != nil {
		return "", fmt.Errorf("commit verified raw cache: %w", err)
	}
	lock, err := json.Marshal(CacheLock{SchemaVersion: SchemaVersion, Dataset: manifest.Name, Version: manifest.Version, SHA256: strings.ToLower(manifest.SHA256), UpstreamURL: manifest.UpstreamURL, UpstreamRevision: manifest.UpstreamRevision, FetchedAt: time.Now().UTC().Format(time.RFC3339Nano)})
	if err != nil {
		return "", fmt.Errorf("marshal raw cache lock: %w", err)
	}
	if err := writeAtomic(filepath.Join(paths.Raw, "cache-lock.json"), lock); err != nil {
		return "", fmt.Errorf("write raw cache lock: %w", err)
	}
	return target, nil
}

func (c Cache) LoadCacheLock(manifest DatasetManifest) (CacheLock, error) {
	paths, err := c.ManifestPaths(manifest)
	if err != nil {
		return CacheLock{}, err
	}
	encoded, err := os.ReadFile(filepath.Join(paths.Raw, "cache-lock.json"))
	if errors.Is(err, os.ErrNotExist) {
		return CacheLock{}, &StatusError{Status: StatusPrerequisiteMissing, Message: "raw cache lock is missing"}
	}
	if err != nil {
		return CacheLock{}, fmt.Errorf("read raw cache lock: %w", err)
	}
	var lock CacheLock
	if err := json.Unmarshal(encoded, &lock); err != nil {
		return CacheLock{}, &StatusError{Status: StatusInvalidManifest, Message: "decode raw cache lock", Cause: err}
	}
	if lock.SchemaVersion != SchemaVersion || lock.Dataset != manifest.Name || lock.Version != manifest.Version || !strings.EqualFold(lock.SHA256, manifest.SHA256) || lock.UpstreamRevision != manifest.UpstreamRevision {
		return CacheLock{}, &StatusError{Status: StatusChecksumMismatch, Message: "raw cache lock does not match manifest"}
	}
	return lock, nil
}

type FetchInput struct {
	Manifest DatasetManifest
	URL      string
	Offline  bool
}

func (c Cache) Fetch(input FetchInput) (string, error) {
	if err := input.Manifest.Validate(); err != nil {
		return "", &StatusError{Status: StatusInvalidManifest, Message: "validate dataset manifest", Cause: err}
	}
	if input.Offline {
		return "", &StatusError{Status: StatusPrerequisiteMissing, Message: "offline mode does not permit network fetch; prepare local raw cache first"}
	}
	url := strings.TrimSpace(input.URL)
	if url == "" {
		url = input.Manifest.UpstreamURL
	}
	response, err := c.client.Get(url)
	if err != nil {
		return "", &StatusError{Status: StatusPrerequisiteMissing, Message: "fetch remote dataset", Cause: err}
	}
	defer response.Body.Close()
	if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices {
		return "", &StatusError{Status: StatusPrerequisiteMissing, Message: fmt.Sprintf("fetch remote dataset returned HTTP %d", response.StatusCode)}
	}
	return c.StoreVerifiedRaw(input.Manifest, response.Body)
}

func (c Cache) WriteNormalized(manifest DatasetManifest, split string, corpus NormalizedCorpus) (string, error) {
	if err := manifest.Validate(); err != nil {
		return "", &StatusError{Status: StatusInvalidManifest, Message: "validate dataset manifest", Cause: err}
	}
	if _, ok := manifest.Splits[split]; !ok {
		return "", &StatusError{Status: StatusInvalidManifest, Message: "requested split is not declared by manifest"}
	}
	if err := corpus.Validate(); err != nil {
		return "", &StatusError{Status: StatusInvalidManifest, Message: "validate normalized corpus", Cause: err}
	}
	checksum, err := corpus.Checksum()
	if err != nil {
		return "", err
	}
	paths, err := c.EnsureManifestLayout(manifest)
	if err != nil {
		return "", err
	}
	corpus.SchemaVersion = SchemaVersion
	encoded, err := json.Marshal(corpus)
	if err != nil {
		return "", fmt.Errorf("marshal normalized corpus: %w", err)
	}
	metadata, err := json.Marshal(NormalizedMetadata{SchemaVersion: SchemaVersion, Family: manifest.Family, Dataset: manifest.Name, Version: manifest.Version, Split: split, Checksum: checksum, QRELChecksum: manifest.QRELChecksum})
	if err != nil {
		return "", fmt.Errorf("marshal normalized metadata: %w", err)
	}
	if err := writeAtomic(filepath.Join(paths.Normalized, split+".json"), encoded); err != nil {
		return "", err
	}
	if err := writeAtomic(filepath.Join(paths.Normalized, split+".metadata.json"), metadata); err != nil {
		return "", err
	}
	return checksum, nil
}

func (c Cache) LoadNormalized(manifest DatasetManifest, split string) (NormalizedCorpus, NormalizedMetadata, error) {
	if err := manifest.Validate(); err != nil {
		return NormalizedCorpus{}, NormalizedMetadata{}, &StatusError{Status: StatusInvalidManifest, Message: "validate dataset manifest", Cause: err}
	}
	paths, err := c.ManifestPaths(manifest)
	if err != nil {
		return NormalizedCorpus{}, NormalizedMetadata{}, err
	}
	encoded, err := os.ReadFile(filepath.Join(paths.Normalized, split+".json"))
	if errors.Is(err, os.ErrNotExist) {
		return NormalizedCorpus{}, NormalizedMetadata{}, &StatusError{Status: StatusPrerequisiteMissing, Message: "normalized corpus is missing"}
	}
	if err != nil {
		return NormalizedCorpus{}, NormalizedMetadata{}, fmt.Errorf("read normalized corpus: %w", err)
	}
	metadataBytes, err := os.ReadFile(filepath.Join(paths.Normalized, split+".metadata.json"))
	if errors.Is(err, os.ErrNotExist) {
		return NormalizedCorpus{}, NormalizedMetadata{}, &StatusError{Status: StatusPrerequisiteMissing, Message: "normalized corpus metadata is missing"}
	}
	if err != nil {
		return NormalizedCorpus{}, NormalizedMetadata{}, fmt.Errorf("read normalized corpus metadata: %w", err)
	}
	var corpus NormalizedCorpus
	if err := json.Unmarshal(encoded, &corpus); err != nil {
		return NormalizedCorpus{}, NormalizedMetadata{}, &StatusError{Status: StatusInvalidManifest, Message: "decode normalized corpus", Cause: err}
	}
	var metadata NormalizedMetadata
	if err := json.Unmarshal(metadataBytes, &metadata); err != nil {
		return NormalizedCorpus{}, NormalizedMetadata{}, &StatusError{Status: StatusInvalidManifest, Message: "decode normalized corpus metadata", Cause: err}
	}
	checksum, err := corpus.Checksum()
	if err != nil {
		return NormalizedCorpus{}, NormalizedMetadata{}, err
	}
	if metadata.SchemaVersion != SchemaVersion || metadata.Family != manifest.Family || metadata.Dataset != manifest.Name || metadata.Version != manifest.Version || metadata.Split != split || metadata.Checksum != checksum || !strings.EqualFold(metadata.QRELChecksum, manifest.QRELChecksum) {
		return NormalizedCorpus{}, NormalizedMetadata{}, &StatusError{Status: StatusChecksumMismatch, Message: "normalized corpus metadata does not match corpus"}
	}
	return corpus, metadata, nil
}

func writeAtomic(target string, data []byte) error {
	temporary, err := os.CreateTemp(filepath.Dir(target), ".write-*")
	if err != nil {
		return fmt.Errorf("create temporary benchmark file: %w", err)
	}
	temporaryName := temporary.Name()
	defer os.Remove(temporaryName)
	if _, err := temporary.Write(data); err != nil {
		temporary.Close()
		return fmt.Errorf("write temporary benchmark file: %w", err)
	}
	if err := temporary.Close(); err != nil {
		return fmt.Errorf("close temporary benchmark file: %w", err)
	}
	if err := os.Rename(temporaryName, target); err != nil {
		return fmt.Errorf("commit benchmark file: %w", err)
	}
	return nil
}
