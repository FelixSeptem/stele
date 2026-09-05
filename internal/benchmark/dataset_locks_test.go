package benchmark

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestGenericDatasetLocksAreCompleteAndChecksumLocked(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("testdata", "generic-retrieval-dataset-locks.json"))
	if err != nil {
		t.Fatal(err)
	}
	var locks []GenericDatasetLock
	if err := json.Unmarshal(data, &locks); err != nil {
		t.Fatal(err)
	}
	if len(locks) != 2 {
		t.Fatalf("lock count = %d, want 2", len(locks))
	}
	seen := map[string]bool{}
	for _, lock := range locks {
		if err := lock.Validate(); err != nil {
			t.Fatalf("%s: %v", lock.Dataset, err)
		}
		if seen[lock.Dataset] {
			t.Fatalf("duplicate dataset lock %q", lock.Dataset)
		}
		seen[lock.Dataset] = true
		var total int64
		for _, file := range lock.Files {
			if file.Revision != lock.UpstreamRevision {
				t.Fatalf("%s file %s revision drifted from lock revision", lock.Dataset, file.Path)
			}
			total += file.SizeBytes
		}
		if total != lock.RawBytes {
			t.Fatalf("%s raw byte total = %d, lock says %d", lock.Dataset, total, lock.RawBytes)
		}
	}
	if !seen["MTEB/scifact"] || !seen["C-MTEB/BQ"] {
		t.Fatalf("expected scifact and BQ locks, got %#v", seen)
	}
}

func TestGenericDatasetLockRejectsMissingChecksum(t *testing.T) {
	lock := GenericDatasetLock{
		Dataset: "MTEB/scifact", Family: FamilyGenericRetrieval,
		UpstreamURL: "https://example.invalid", UpstreamRevision: "rev", License: "Apache-2.0",
		LicenseStatus: LicenseReviewed, Language: "en", Subset: "test", CorpusRecords: 1,
		QueryRecords: 1, QRELRecords: 1, RawBytes: 1, EstimatedNormalizedBytes: 1,
		LocalStorageBudgetBytes: 1, EmbeddingProfile: "lexical-only", Redistribution: RedistributionRestricted,
		Support: SupportMetadataOnly, Files: []DatasetLockFile{{Path: "corpus.jsonl", Role: "corpus", Revision: "rev", SizeBytes: 1}},
	}
	if err := lock.Validate(); err == nil {
		t.Fatal("expected malformed checksum to be rejected")
	}
}
