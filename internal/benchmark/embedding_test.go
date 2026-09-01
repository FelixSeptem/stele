package benchmark

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestLoadPrecachedVectorsValidatesProfileAndDimensions(t *testing.T) {
	path := filepath.Join(t.TempDir(), "vectors.json")
	data, err := json.Marshal(map[string]map[string][]float32{"vectors": {
		"event-1": {1, 0, 0},
		"event-2": {0, 1, 0},
	}})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}
	profile := EmbeddingProfile{Name: "local-cache-v1", Provider: "precomputed", Model: "test-model", Revision: "rev-1", Dimensions: 3, Normalization: "l2", VectorSource: path}
	vectors, err := LoadPrecachedVectors(path, profile)
	if err != nil {
		t.Fatal(err)
	}
	if len(vectors) != 2 || len(vectors["event-1"]) != 3 {
		t.Fatalf("unexpected vectors: %#v", vectors)
	}
}

func TestLoadPrecachedVectorsRejectsDimensionMismatchWithoutNetworkFallback(t *testing.T) {
	path := filepath.Join(t.TempDir(), "vectors.json")
	if err := os.WriteFile(path, []byte(`{"vectors":{"event-1":[1,0]}}`), 0o600); err != nil {
		t.Fatal(err)
	}
	profile := EmbeddingProfile{Name: "local-cache-v1", Provider: "precomputed", Model: "test-model", Revision: "rev-1", Dimensions: 3, Normalization: "l2", VectorSource: path}
	if _, err := LoadPrecachedVectors(path, profile); StatusOf(err) != StatusInvalidManifest {
		t.Fatalf("expected invalid manifest dimension error, got %v (status %s)", err, StatusOf(err))
	}
}

func TestValidateLocalEmbeddingProfileRequiresExistingModelRevision(t *testing.T) {
	modelPath := filepath.Join(t.TempDir(), "model.bin")
	if err := os.WriteFile(modelPath, []byte("local model"), 0o600); err != nil {
		t.Fatal(err)
	}
	profile := EmbeddingProfile{Name: "local-model-v1", Provider: "local", Model: modelPath, Revision: "sha256:model-1", Dimensions: 3, Normalization: "l2"}
	if err := ValidateLocalEmbeddingProfile(profile); err != nil {
		t.Fatal(err)
	}
	profile.Revision = ""
	if StatusOf(ValidateLocalEmbeddingProfile(profile)) != StatusInvalidManifest {
		t.Fatal("expected missing model revision to be invalid")
	}
}
