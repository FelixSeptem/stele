package benchmark

import (
	"encoding/json"
	"fmt"
	"math"
	"os"
	"strings"
)

// PrecachedVectors is the offline vector artifact format used by benchmark runs.
// Keeping the wrapper explicit prevents accidentally accepting arbitrary JSON as a
// vector cache and makes the artifact self-describing for future revisions.
type PrecachedVectors struct {
	Vectors map[string][]float32 `json:"vectors"`
}

// LoadPrecachedVectors loads a local vector artifact and verifies every vector
// against the declared embedding profile. It never contacts a remote provider.
func LoadPrecachedVectors(path string, profile EmbeddingProfile) (map[string][]float32, error) {
	if err := validatePrecachedProfile(profile); err != nil {
		return nil, err
	}
	if strings.TrimSpace(path) == "" {
		return nil, &StatusError{Status: StatusPrerequisiteMissing, Message: "pre-cached vector path is required"}
	}
	encoded, err := os.ReadFile(path)
	if err != nil {
		return nil, &StatusError{Status: StatusPrerequisiteMissing, Message: "pre-cached vectors are missing", Cause: err}
	}
	var artifact PrecachedVectors
	decoder := json.NewDecoder(strings.NewReader(string(encoded)))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&artifact); err != nil {
		return nil, &StatusError{Status: StatusInvalidManifest, Message: "decode pre-cached vectors", Cause: err}
	}
	if len(artifact.Vectors) == 0 {
		return nil, &StatusError{Status: StatusInvalidManifest, Message: "pre-cached vectors cannot be empty"}
	}
	for id, vector := range artifact.Vectors {
		if strings.TrimSpace(id) == "" {
			return nil, &StatusError{Status: StatusInvalidManifest, Message: "pre-cached vector id is required"}
		}
		if len(vector) != profile.Dimensions {
			return nil, &StatusError{Status: StatusInvalidManifest, Message: fmt.Sprintf("pre-cached vector %s has dimension %d, want %d", id, len(vector), profile.Dimensions)}
		}
		for _, value := range vector {
			if math.IsNaN(float64(value)) || math.IsInf(float64(value), 0) {
				return nil, &StatusError{Status: StatusInvalidManifest, Message: fmt.Sprintf("pre-cached vector %s contains a non-finite value", id)}
			}
		}
	}
	return artifact.Vectors, nil
}

// ValidateLocalEmbeddingProfile verifies a profile that points at a local model
// artifact. Model loading itself remains the responsibility of the chosen local
// embedding runtime; this check guarantees benchmark admission is offline-safe.
func ValidateLocalEmbeddingProfile(profile EmbeddingProfile) error {
	if err := profile.Validate(); err != nil {
		return &StatusError{Status: StatusInvalidManifest, Message: "validate local embedding profile", Cause: err}
	}
	if profile.Provider != "local" {
		return &StatusError{Status: StatusInvalidManifest, Message: "local embedding profile provider must be local"}
	}
	if strings.TrimSpace(profile.Model) == "" {
		return &StatusError{Status: StatusInvalidManifest, Message: "local embedding model path is required"}
	}
	if strings.TrimSpace(profile.Revision) == "" {
		return &StatusError{Status: StatusInvalidManifest, Message: "local embedding model revision is required"}
	}
	if profile.Dimensions <= 0 {
		return &StatusError{Status: StatusInvalidManifest, Message: "local embedding dimensions must be positive"}
	}
	if _, err := os.Stat(profile.Model); err != nil {
		return &StatusError{Status: StatusPrerequisiteMissing, Message: "local embedding model is missing", Cause: err}
	}
	return nil
}

func validatePrecachedProfile(profile EmbeddingProfile) error {
	if err := profile.Validate(); err != nil {
		return &StatusError{Status: StatusInvalidManifest, Message: "validate pre-cached embedding profile", Cause: err}
	}
	if profile.Provider != "precomputed" {
		return &StatusError{Status: StatusInvalidManifest, Message: "pre-cached embedding profile provider must be precomputed"}
	}
	if strings.TrimSpace(profile.Revision) == "" || profile.Dimensions <= 0 {
		return &StatusError{Status: StatusInvalidManifest, Message: "pre-cached embedding profile requires revision and positive dimensions"}
	}
	return nil
}
