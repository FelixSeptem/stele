package app

import (
	"testing"

	"github.com/FelixSeptem/stele/internal/memory"
)

func TestMemoryIntentProvenanceContextCarriesOnlyVerifiedCorrectionIdentity(t *testing.T) {
	record := memory.MemoryIntentRecord{ID: "intent-1"}
	if got := memoryIntentProvenanceContext(record); len(got) != 1 || got["intent_id"] != record.ID {
		t.Fatalf("ordinary provenance = %#v", got)
	}
	record.Correction = &memory.CorrectionAttribution{ProjectionID: "11111111-1111-4111-8111-111111111111", SourceRevision: 2, BindingID: "binding", PrincipalID: "principal"}
	got := memoryIntentProvenanceContext(record)
	if got["intent_id"] != record.ID || got["correction_projection_id"] != record.Correction.ProjectionID || got["correction_source_revision"] != int64(2) {
		t.Fatalf("verified correction provenance = %#v", got)
	}
	if _, leaksBinding := got["binding_id"]; leaksBinding {
		t.Fatalf("provenance leaks binding identity: %#v", got)
	}
}
