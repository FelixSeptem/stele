package openapi

import (
	"encoding/json"
	"github.com/getkin/kin-openapi/openapi3"
	"strings"
	"testing"
)

func TestDurableReferenceOpenAPIHasClosedRequestsAndPublicRoutes(t *testing.T) {
	doc, err := openapi3.NewLoader().LoadFromData([]byte(SpecYAML()))
	if err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{"/v1/provider/references", "/v1/provider/references/status", "/v1/provider/references/lookup"} {
		if doc.Paths.Value(path) == nil || doc.Paths.Value(path).Post == nil {
			t.Errorf("missing public reference route %s", path)
		}
	}
	for _, name := range []string{"ProviderReferenceWriteRequest", "ProviderReferenceStatusRequest", "ProviderReferenceLookupRequest", "ProviderReferenceResponse", "ProviderDurableReferenceCapabilities"} {
		schema := doc.Components.Schemas[name]
		if schema == nil {
			t.Errorf("missing schema %s", name)
			continue
		}
		if schema.Value.AdditionalProperties.Has == nil || *schema.Value.AdditionalProperties.Has {
			t.Errorf("schema %s must be closed", name)
		}
	}
}

func TestDurableReferenceScopeSchemaMatchesBoundedRuntimeProjection(t *testing.T) {
	doc, err := openapi3.NewLoader().LoadFromData([]byte(SpecYAML()))
	if err != nil {
		t.Fatal(err)
	}
	schema := doc.Components.Schemas["CanonicalScopeDescriptor"].Value
	scope := map[string]any{"tenant": "tenant", "project": "project", "namespace": "namespace", "agent_id": "agent", "session_id": "session", "provider_instance_id": "instance"}
	if err := schema.VisitJSON(scope); err != nil {
		t.Fatal(err)
	}
	for _, field := range []string{"tenant", "project", "namespace", "agent_id", "session_id", "conversation_id", "provider_instance_id"} {
		original, present := scope[field]
		for _, invalid := range []string{strings.Repeat("a", 129), "scope<name", "scope name"} {
			scope[field] = invalid
			if err := schema.VisitJSON(scope); err == nil {
				t.Errorf("scope schema accepted %s=%q", field, invalid)
			}
		}
		if present {
			scope[field] = original
		} else {
			delete(scope, field)
		}
	}
	scope["conversation_id"] = ""
	if err := schema.VisitJSON(scope); err != nil {
		t.Fatalf("optional empty conversation rejected: %v", err)
	}
}

func TestDurableReferenceWriteSchemaAcceptsMetadataAndRequiresIdempotency(t *testing.T) {
	doc, err := openapi3.NewLoader().LoadFromData([]byte(SpecYAML()))
	if err != nil {
		t.Fatal(err)
	}
	schema := doc.Components.Schemas["ProviderReferenceWriteRequest"].Value
	body := `{"metadata":{"request_id":"req-1","operation_id":"providerWriteReference","schema_version":"provider-v1","idempotency_key":"ref-key"},"input":{"contract_version":"durable-reference-v1","reference_id":"task-1","memory_path":"tasks/demo","event_type":"provider.reference.task","value":"completed task"}}`
	var value map[string]any
	if err := json.Unmarshal([]byte(body), &value); err != nil {
		t.Fatal(err)
	}
	if err := schema.VisitJSON(value); err != nil {
		t.Fatalf("valid write rejected: %v", err)
	}
	delete(value["metadata"].(map[string]any), "idempotency_key")
	if err := schema.VisitJSON(value); err == nil {
		t.Fatal("write without idempotency key accepted")
	}
}
