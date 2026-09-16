package provider

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestCapabilityDocumentRoundTripAndBounds(t *testing.T) {
	doc := CapabilityDocument{ProviderVersion: "provider-v1", SchemaVersion: "schema-v1", Operations: []string{"ingest", "retrieve"}, ScopeDimensions: []string{"tenant", "project", "namespace", "agent", "session", "conversation", "provider_instance"}, Limits: ProviderLimits{MaxEventBytes: 1024, MaxIntentBytes: 2048, MaxRetrievalResults: 10, MaxContextBytes: 4096, MaxCitations: 8, MaxMetadataBytes: 512}, SchemaDigest: "sha256:abc"}
	if err := doc.Validate(); err != nil {
		t.Fatal(err)
	}
	b, err := json.Marshal(doc)
	if err != nil {
		t.Fatal(err)
	}
	var got CapabilityDocument
	if err := json.Unmarshal(b, &got); err != nil {
		t.Fatal(err)
	}
	if got.SchemaVersion != doc.SchemaVersion || len(got.Operations) != 2 {
		t.Fatalf("roundtrip=%+v", got)
	}
	doc.ProviderVersion = strings.Repeat("x", MaxProviderVersionBytes+1)
	if err := doc.Validate(); err == nil {
		t.Fatal("oversized provider version accepted")
	}
}

func TestDecodeRejectsUnknownRequiredFields(t *testing.T) {
	var doc CapabilityDocument
	if err := DecodeStrict([]byte(`{"provider_version":"p","schema_version":"s","operations":[],"scope_dimensions":[],"limits":{},"bogus":true}`), &doc); err == nil {
		t.Fatal("unknown field accepted")
	}
	if err := DecodeStrict([]byte(`{} {}`), &doc); err == nil {
		t.Fatal("trailing JSON accepted")
	}
}

func TestProviderEnvelopesRoundTripAndValidate(t *testing.T) {
	scope := CanonicalScopeDescriptor{Tenant: "tenant-a", Project: "project-a", Namespace: "namespace-a", AgentID: "agent-a", SessionID: "session-a", ConversationID: "conversation-a", ProviderInstanceID: "instance-a"}
	result := ResultEnvelope{SchemaVersion: "schema-v1", Scope: scope, Result: json.RawMessage(`{"accepted":true}`)}
	if err := result.Validate(); err != nil {
		t.Fatal(err)
	}
	b, err := json.Marshal(result)
	if err != nil {
		t.Fatal(err)
	}
	var got ResultEnvelope
	if err := DecodeStrict(b, &got); err != nil {
		t.Fatal(err)
	}
	if got.Scope != scope {
		t.Fatalf("scope=%+v", got.Scope)
	}

	bounded := ProviderError{Category: ErrorCategoryValidation, Code: "invalid_request", Message: "request is invalid", Retryable: false, SupportedVersions: []string{"schema-v1"}}
	if err := bounded.Validate(); err != nil {
		t.Fatal(err)
	}
	bounded.Message = strings.Repeat("x", MaxErrorMessageBytes+1)
	if err := bounded.Validate(); err == nil {
		t.Fatal("oversized error message accepted")
	}
}

func TestCapabilityDiscoveryIsBoundedAndSecretFree(t *testing.T) {
	doc := Discover(CapabilityInput{ProviderVersion: "provider-v1", SchemaVersion: "schema-v1", ServiceVersion: "dev", BuildID: "unknown", SchemaDigest: "sha256:test"})
	if err := doc.Validate(); err != nil {
		t.Fatal(err)
	}
	if len(doc.Operations) == 0 || len(doc.ScopeDimensions) == 0 || doc.Limits.MaxEventBytes <= 0 {
		t.Fatalf("doc=%+v", doc)
	}
	if strings.Contains(string(mustJSON(doc)), "postgres") {
		t.Fatal("secret leaked")
	}
}
func mustJSON(v any) []byte { b, _ := json.Marshal(v); return b }
