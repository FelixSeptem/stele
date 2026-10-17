package app

import (
	"testing"
	"time"

	"github.com/FelixSeptem/stele/internal/memory"
)

func TestMemoryRepositoryIntentAttributesAcceptsOnlyValidatedGovernedProvenance(t *testing.T) {
	trusted := memory.SourceMetadata{Version: memory.SourceMetadataVersion, Trust: memory.SourceTrustAttestedData, Classification: memory.SourceClassificationLow}
	source, class := memoryRepositoryIntentAttributes(map[string]any{"memory_repository_id": "repo", "source_metadata": trusted, "memory_class": string(memory.MemoryClassProfile)})
	if source != trusted || class != memory.MemoryClassProfile {
		t.Fatalf("attributes = %+v, %q", source, class)
	}

	// PostgreSQL JSONB provenance round-trips object values as map[string]any.
	// It remains valid only when it preserves the exact source metadata shape.
	source, class = memoryRepositoryIntentAttributes(map[string]any{
		"memory_repository_id": "repo",
		"source_metadata": map[string]any{
			"version": string(memory.SourceMetadataVersion), "trust": string(memory.SourceTrustUntrusted), "classification": string(memory.SourceClassificationLow),
		},
		"memory_class": string(memory.MemoryClassProcedural),
	})
	if source != (memory.SourceMetadata{Version: memory.SourceMetadataVersion, Trust: memory.SourceTrustUntrusted, Classification: memory.SourceClassificationLow}) || class != memory.MemoryClassProcedural {
		t.Fatalf("persisted attributes = %+v, %q", source, class)
	}

	source, class = memoryRepositoryIntentAttributes(map[string]any{"memory_repository_id": "repo", "source_metadata": map[string]any{"trust": "attested_data", "classification": "low", "untrusted_extra": "must fail closed"}, "memory_class": "arbitrary"})
	if source != (memory.SourceMetadata{}).Effective() || class != memory.MemoryClassEpisodic {
		t.Fatalf("unsafe attributes were accepted: %+v, %q", source, class)
	}
}

func TestMemoryRepositoryCandidatePreservesEffectiveSourceMetadata(t *testing.T) {
	scope := memory.Scope{Tenant: "tenant", Project: "project", Namespace: "namespace"}
	source := memory.SourceMetadata{Version: memory.SourceMetadataVersion, Trust: memory.SourceTrustUntrusted, Classification: memory.SourceClassificationLow}
	candidate := memoryRepositoryCandidate("candidate", "event", scope, "topics/fact", memory.MemoryClassProcedural, "fact", source, time.Date(2026, 10, 16, 12, 0, 0, 0, time.UTC))
	if candidate.SourceMetadata != source || candidate.Class != memory.MemoryClassProcedural || candidate.Scope != scope {
		t.Fatalf("candidate = %+v", candidate)
	}
}
