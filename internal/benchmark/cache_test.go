package benchmark

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/FelixSeptem/stele/internal/memory"
)

func TestCacheLayoutIsDeterministicAndCreatesSplitPaths(t *testing.T) {
	cache := NewCache(t.TempDir())
	paths, err := cache.EnsureLayout("locomo", "v1")
	if err != nil {
		t.Fatal(err)
	}
	wantRoot := filepath.Join(cache.DataDir, "locomo", "v1")
	if paths.Root != wantRoot || paths.Raw != filepath.Join(wantRoot, "raw") || paths.Normalized != filepath.Join(wantRoot, "normalized") {
		t.Fatalf("unexpected paths: %#v", paths)
	}
	if info, err := os.Stat(paths.Reports); err != nil || !info.IsDir() {
		t.Fatalf("reports directory missing: %v", err)
	}
}

func memoryScope() memory.Scope {
	return memory.Scope{Tenant: "tenant", Project: "project", Namespace: "namespace"}
}

func TestFetchVerifiesChecksumAndPreservesExistingRawData(t *testing.T) {
	cache := NewCache(t.TempDir())
	valid := []byte("known good archive")
	digest := sha256.Sum256(valid)
	manifest := validManifest()
	manifest.Version = "v1"
	manifest.SHA256 = hex.EncodeToString(digest[:])
	manifest.SourcePath = "locomo.json"
	if _, err := cache.StoreVerifiedRaw(manifest, bytes.NewReader(valid)); err != nil {
		t.Fatal(err)
	}
	lock, err := cache.LoadCacheLock(manifest)
	if err != nil {
		t.Fatal(err)
	}
	if lock.SHA256 != manifest.SHA256 || lock.UpstreamRevision != manifest.UpstreamRevision {
		t.Fatalf("unexpected cache lock: %#v", lock)
	}
	if _, err := cache.StoreVerifiedRaw(manifest, bytes.NewReader([]byte("tampered"))); StatusOf(err) != StatusChecksumMismatch {
		t.Fatalf("expected checksum mismatch, got %v", err)
	}
	paths, err := cache.ManifestPaths(manifest)
	if err != nil {
		t.Fatal(err)
	}
	stored, err := os.ReadFile(filepath.Join(paths.Raw, filepath.Base(manifest.SourcePath)))
	if err != nil {
		t.Fatal(err)
	}
	if string(stored) != string(valid) {
		t.Fatalf("valid cache was overwritten: %q", stored)
	}
}

func TestStoreVerifiedRawIsIdempotentForSameArchive(t *testing.T) {
	cache := NewCache(t.TempDir())
	data := []byte("known good archive")
	digest := sha256.Sum256(data)
	manifest := validManifest()
	manifest.SHA256 = hex.EncodeToString(digest[:])
	manifest.SourcePath = "archive.json"
	first, err := cache.StoreVerifiedRaw(manifest, bytes.NewReader(data))
	if err != nil {
		t.Fatal(err)
	}
	second, err := cache.StoreVerifiedRaw(manifest, bytes.NewReader(data))
	if err != nil {
		t.Fatal(err)
	}
	if first != second {
		t.Fatalf("expected stable raw cache path, got %q and %q", first, second)
	}
}

func TestOfflineFetchRejectsNetworkSource(t *testing.T) {
	cache := NewCache(t.TempDir())
	manifest := validManifest()
	if _, err := cache.Fetch(FetchInput{Manifest: manifest, Offline: true, URL: manifest.UpstreamURL}); StatusOf(err) != StatusPrerequisiteMissing {
		t.Fatalf("expected prerequisite missing in offline mode, got %v", err)
	}
}

func TestWriteAndLoadNormalizedCorpusIsDeterministic(t *testing.T) {
	cache := NewCache(t.TempDir())
	manifest := validManifest()
	manifest.Version = "v1"
	corpus := NormalizedCorpus{
		Conversations: []ConversationRecord{{ID: "session-2", Scope: memoryScope(), Turns: []ConversationTurn{{ID: "turn-2", Text: "later"}}}, {ID: "session-1", Scope: memoryScope(), Turns: []ConversationTurn{{ID: "turn-1", Text: "first"}}}},
		Events:        []MemoryEventRecord{{ID: "event-1", Scope: memoryScope(), Text: "fact"}},
	}
	first, err := cache.WriteNormalized(manifest, "smoke", corpus)
	if err != nil {
		t.Fatal(err)
	}
	loaded, metadata, err := cache.LoadNormalized(manifest, "smoke")
	if err != nil {
		t.Fatal(err)
	}
	second, err := loaded.Checksum()
	if err != nil {
		t.Fatal(err)
	}
	if first != second || metadata.Checksum != first {
		t.Fatalf("expected stable normalized checksum, write=%s read=%s metadata=%s", first, second, metadata.Checksum)
	}
}

func TestWriteFamilyReportRetainsValidatedBenchmarkReport(t *testing.T) {
	cache := NewCache(t.TempDir())
	manifest := validManifest()
	manifest.Family = FamilySpecialized
	manifest.Name = "specialized-fixtures"
	manifest.Version = "v1"
	manifest.ConversionVersion = "specialized-v1"
	manifest.Splits = map[string]SplitSpec{"profile": {Identity: "specialized/profile", Source: "profile.json"}}
	run, err := NewRunScope(memory.Scope{Tenant: "tenant-a", Project: "production", Namespace: "default"}, manifest.Name, "profile-regression")
	if err != nil {
		t.Fatal(err)
	}
	report := NewFamilyReport(FamilySpecialized, manifest, "profile", run.Scope).WithExecutionProvenance(FamilyReportExecution{
		QRELVersion:     "specialized-qrels-v1",
		StrategyProfile: "fixture-retrieval",
		InputChecksums:  map[string]string{"normalized": "fixture-v1"},
	})
	report.Metrics = SpecializedReport{Family: "specialized_retrieval", Subfamily: "profile_preference", Metrics: SpecializedMetrics{ProfileRecall: 1, PreferenceConsistency: 1, UnmappedEvidenceCount: 0}}
	report.SafetyOutcomes = SpecializedMetrics{ScopeSafetyFailures: 0, SessionIsolationViolations: 0}

	path, err := cache.WriteFamilyReport(manifest, run.ID, report)
	if err != nil {
		t.Fatal(err)
	}
	paths, err := cache.ManifestPaths(manifest)
	if err != nil {
		t.Fatal(err)
	}
	wantPath := filepath.Join(paths.Reports, run.ID+".json")
	if path != wantPath {
		t.Fatalf("report path = %q, want %q", path, wantPath)
	}
	encoded, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var retained FamilyReport
	if err := json.Unmarshal(encoded, &retained); err != nil {
		t.Fatal(err)
	}
	if retained.Family != FamilySpecialized || retained.Scope != run.Scope || len(retained.ArtifactPaths) != 1 || retained.ArtifactPaths[0] != path {
		t.Fatalf("retained report lost specialized provenance: %#v", retained)
	}
	if result, err := cache.CleanRunArtifacts(manifest, run.ID, true); err != nil || !result.ReportRetained {
		t.Fatalf("retained specialized report was not preserved: %#v, %v", result, err)
	}
}

func TestWriteFamilyReportRejectsProductionScope(t *testing.T) {
	cache := NewCache(t.TempDir())
	manifest := validManifest()
	manifest.Family = FamilySpecialized
	manifest.Name = "specialized-fixtures"
	manifest.Version = "v1"
	manifest.ConversionVersion = "specialized-v1"
	manifest.Splits = map[string]SplitSpec{"profile": {Identity: "specialized/profile", Source: "profile.json"}}
	report := NewFamilyReport(FamilySpecialized, manifest, "profile", memory.Scope{Tenant: "tenant-a", Project: "production", Namespace: "default"})

	if _, err := cache.WriteFamilyReport(manifest, "profile-regression", report); StatusOf(err) != StatusInvalidManifest {
		t.Fatalf("expected production report scope rejection, got %v", err)
	}
}
