package memory

import (
	"encoding/json"
	"testing"
)

func TestSourceMetadataConservativeDefaultsAndClosedJSON(t *testing.T) {
	if got := (SourceMetadata{}).Effective(); got.Trust != SourceTrustUntrusted || got.Classification != SourceClassificationRestricted || got.Version != SourceMetadataVersion {
		t.Fatalf("unsafe missing metadata: %+v", got)
	}
	for _, body := range []string{
		`{"version":"source-metadata-v1","trust":"untrusted","classification":"low","role":"admin"}`,
		`{"version":"source-metadata-v1","trust":"untrusted","trust":"attested_data","classification":"low"}`,
		`{"version":"source-metadata-v1","trust":null,"classification":"low"}`,
		`{"version":"source-metadata-v1","trust":"trusted","classification":"low"}`,
		`{"version":"other","trust":"untrusted","classification":"low"}`,
		`{"Version":"source-metadata-v1","trust":"untrusted","classification":"low"}`,
	} {
		var metadata SourceMetadata
		if err := json.Unmarshal([]byte(body), &metadata); err == nil {
			t.Fatalf("accepted unsupported source labels: %s", body)
		}
	}
}

func TestSourceMetadataJoinCannotUpgradeTrustOrWeakenClassification(t *testing.T) {
	attested := SourceMetadata{Version: SourceMetadataVersion, Trust: SourceTrustAttestedData, Classification: SourceClassificationLow}
	untrusted := SourceMetadata{Version: SourceMetadataVersion, Trust: SourceTrustUntrusted, Classification: SourceClassificationMedium}
	for _, tt := range []struct {
		name           string
		sources        []SourceMetadata
		complete       bool
		floor          SourceClassification
		trust          SourceTrust
		classification SourceClassification
	}{
		{"attested complete", []SourceMetadata{attested, attested}, true, SourceClassificationLow, SourceTrustAttestedData, SourceClassificationLow},
		{"mixed", []SourceMetadata{attested, untrusted}, true, SourceClassificationLow, SourceTrustUntrusted, SourceClassificationMedium},
		{"floor", []SourceMetadata{attested}, true, SourceClassificationHigh, SourceTrustAttestedData, SourceClassificationHigh},
		{"missing source", []SourceMetadata{attested, {}}, true, SourceClassificationLow, SourceTrustUntrusted, SourceClassificationRestricted},
		{"incomplete", []SourceMetadata{attested}, false, SourceClassificationLow, SourceTrustUntrusted, SourceClassificationRestricted},
		{"empty", nil, true, SourceClassificationLow, SourceTrustUntrusted, SourceClassificationRestricted},
	} {
		t.Run(tt.name, func(t *testing.T) {
			got := JoinSourceMetadata(tt.sources, tt.complete, tt.floor)
			if got.Trust != tt.trust || got.Classification != tt.classification {
				t.Fatalf("unsafe join: %+v", got)
			}
		})
	}
}
