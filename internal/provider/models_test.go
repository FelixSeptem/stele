package provider

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestOperationMetadataRoundTripAndValidation(t *testing.T) {
	in := OperationMetadata{RequestID: "req-1", OperationID: "op-1", IdempotencyKey: "idem-1", EventSeq: 2, SchemaVersion: SchemaVersionV1}
	b, err := json.Marshal(in)
	if err != nil {
		t.Fatal(err)
	}
	var out OperationMetadata
	if err := DecodeJSON(b, &out); err != nil {
		t.Fatal(err)
	}
	if out != in {
		t.Fatalf("round trip = %+v, want %+v", out, in)
	}
	if err := out.Validate(); err != nil {
		t.Fatal(err)
	}
}

func TestDecodeJSONRejectsUnknownAndOversized(t *testing.T) {
	var out OperationMetadata
	if err := DecodeJSON([]byte(`{"request_id":"x","unknown":1}`), &out); err == nil {
		t.Fatal("expected unknown field rejection")
	}
	if err := DecodeJSON([]byte(`{"request_id":"`+strings.Repeat("x", MaxMetadataFieldBytes+1)+`"}`), &out); err == nil {
		t.Fatal("expected oversized field rejection")
	}
	if err := DecodeJSON([]byte(`{"request_id":"req-1"}`), &out); err == nil {
		t.Fatal("expected missing schema_version rejection")
	}
	if err := DecodeJSON([]byte(`{"schema_version":"provider-v1"}{}`), &out); err == nil {
		t.Fatal("expected trailing JSON rejection")
	}
}

func TestDiscoverCapabilitiesIsBoundedAndScoped(t *testing.T) {
	c := DiscoverCapabilities(RuntimeMetadata{ServiceVersion: "1.2.3", BuildID: "build", BuildTimestamp: "time", SchemaVersion: 4}, CapabilityConfig{})
	if c.ProviderVersion == "" || c.SchemaVersion != SchemaVersionV1 || c.SchemaDigest == "" || c.RuntimeSchemaVersion != 4 {
		t.Fatalf("capability = %+v", c)
	}
	if len(c.Operations) == 0 || len(c.Scope.Dimensions) != 3 {
		t.Fatalf("capability = %+v", c)
	}
	if strings.Contains(strings.ToLower(string(mustJSON(c))), "secret") {
		t.Fatal("capability leaked secret")
	}
}

func TestDiscoverCapabilitiesBoundsUnsafeRuntimeMetadata(t *testing.T) {
	c := DiscoverCapabilities(RuntimeMetadata{ServiceVersion: strings.Repeat("x", 200), BuildID: "postgres://secret", BuildTimestamp: "line\nbreak"}, CapabilityConfig{})
	if c.ServiceVersion != "unknown" || c.BuildID != "unknown" || c.BuildTimestamp != "unknown" {
		t.Fatalf("runtime metadata was not bounded: %+v", c)
	}
}

func TestContractModelsUseStableJSONFieldsAndRejectInvalidRequiredValues(t *testing.T) {
	limits := Limits{EventBytes: 1, IntentBytes: 2, RetrievalItems: 3, ContextItems: 4, CitationItems: 5, MetadataBytes: 6}
	encoded := string(mustJSON(limits))
	for _, field := range []string{"event_bytes", "intent_bytes", "retrieval_items", "context_items", "citation_items", "metadata_bytes"} {
		if !strings.Contains(encoded, `"`+field+`"`) {
			t.Fatalf("limits JSON %s missing %s", encoded, field)
		}
	}
	for _, tc := range []struct {
		name string
		body string
		dst  any
	}{
		{"citation source", `{"reference":"ref","availability":"available"}`, &Citation{}},
		{"error category", `{"code":"invalid","message":"bad"}`, &BoundedError{}},
		{"result disposition", `{"schema_version":"provider-v1"}`, &Result{}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if err := DecodeJSON([]byte(tc.body), tc.dst); err == nil {
				t.Fatal("expected missing required field rejection")
			}
		})
	}
}

func mustJSON(v any) []byte { b, _ := json.Marshal(v); return b }
