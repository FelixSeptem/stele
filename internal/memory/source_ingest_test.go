package memory

import "testing"

func TestEventSourceLabelsAreStrictAndFingerprintBound(t *testing.T) {
	i := IngestEventInput{Scope: Scope{Tenant: "t", Project: "p", Namespace: "n"}, EventType: "conversation.message", Content: "content"}
	a, err := EventRequestFingerprint(i)
	if err != nil {
		t.Fatal(err)
	}
	i.SourceMetadata = SourceMetadata{Version: SourceMetadataVersion, Trust: SourceTrustUntrusted, Classification: SourceClassificationLow}
	b, err := EventRequestFingerprint(i)
	if err != nil {
		t.Fatal(err)
	}
	if a == b {
		t.Fatal("source labels are not bound to replay identity")
	}
	i.SourceMetadata.Trust = "invented"
	if i.Validate() == nil {
		t.Fatal("invalid typed source accepted")
	}
	i.SourceMetadata = SourceMetadata{}
	i.Metadata = map[string]any{"source_metadata": map[string]any{"trust": "attested_data"}}
	if i.Validate() == nil {
		t.Fatal("reserved source metadata spoof accepted")
	}
}
