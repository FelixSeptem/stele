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
	Raw        string
	Normalized string
	Embeddings string
	Reports    string
}

// EnsureRunLayout creates run-scoped artifact directories below a validated
// dataset/version cache. Run ids are kept out of production paths by callers.
func (c Cache) EnsureRunLayout(manifest DatasetManifest, runID string) (CachePaths, error) {
	if err := manifest.Validate(); err != nil {
		return CachePaths{}, &StatusError{Status: StatusInvalidManifest, Message: "validate dataset manifest", Cause: err}
	}
	if err := validateBenchmarkRunID(runID); err != nil {
		return CachePaths{}, err
	}
	paths, err := c.EnsureLayout(manifest.Name, manifest.Version)
	if err != nil {
		return CachePaths{}, err
	}
	paths.Normalized = filepath.Join(paths.Normalized, runID)
	paths.Embeddings = filepath.Join(paths.Embeddings, runID)
	paths.Reports = filepath.Join(paths.Reports, runID)
	for _, path := range []string{paths.Normalized, paths.Embeddings, paths.Reports} {
		if err := os.MkdirAll(path, 0o755); err != nil {
			return CachePaths{}, fmt.Errorf("create benchmark run layout: %w", err)
		}
	}
	return paths, nil
}

// CleanupBenchmarkRun removes only run-scoped normalized and embedding
// artifacts. Reports can be retained as audit evidence.
func (c Cache) CleanupBenchmarkRun(manifest DatasetManifest, runID string, preserveReports bool) error {
	if err := manifest.Validate(); err != nil {
		return &StatusError{Status: StatusInvalidManifest, Message: "validate dataset manifest", Cause: err}
	}
	if err := validateBenchmarkRunID(runID); err != nil {
		return err
	}
	paths, err := c.Paths(manifest.Name, manifest.Version)
	if err != nil {
		return err
	}
	targets := []string{filepath.Join(paths.Normalized, runID), filepath.Join(paths.Embeddings, runID)}
	if !preserveReports {
		targets = append(targets, filepath.Join(paths.Reports, runID))
	}
	for _, target := range targets {
		if err := os.RemoveAll(target); err != nil {
			return fmt.Errorf("cleanup benchmark run artifact: %w", err)
		}
	}
	return nil
}

func validateBenchmarkRunID(runID string) error {
	runID = strings.TrimSpace(runID)
	if runID == "" || runID == "." || runID == ".." || filepath.Base(runID) != runID || strings.ContainsAny(runID, `/\\`) {
		return &StatusError{Status: StatusInvalidManifest, Message: "benchmark run id must be a single safe path component"}
	}
	return nil
}

type NormalizedMetadata struct {
	SchemaVersion string `json:"schema_version"`
	Dataset       string `json:"dataset"`
	Version       string `json:"version"`
	Split         string `json:"split"`
	Checksum      string `json:"checksum"`
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

func (c Cache) StoreVerifiedRaw(manifest DatasetManifest, source io.Reader) (string, error) {
	if err := manifest.Validate(); err != nil {
		return "", &StatusError{Status: StatusInvalidManifest, Message: "validate dataset manifest", Cause: err}
	}
	if source == nil {
		return "", &StatusError{Status: StatusPrerequisiteMissing, Message: "raw source is required"}
	}
	paths, err := c.EnsureLayout(manifest.Name, manifest.Version)
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
	paths, err := c.Paths(manifest.Name, manifest.Version)
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
	paths, err := c.EnsureLayout(manifest.Name, manifest.Version)
	if err != nil {
		return "", err
	}
	corpus.SchemaVersion = SchemaVersion
	encoded, err := json.Marshal(corpus)
	if err != nil {
		return "", fmt.Errorf("marshal normalized corpus: %w", err)
	}
	metadata, err := json.Marshal(NormalizedMetadata{SchemaVersion: SchemaVersion, Dataset: manifest.Name, Version: manifest.Version, Split: split, Checksum: checksum})
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
	paths, err := c.Paths(manifest.Name, manifest.Version)
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
	if metadata.SchemaVersion != SchemaVersion || metadata.Dataset != manifest.Name || metadata.Version != manifest.Version || metadata.Split != split || metadata.Checksum != checksum {
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
