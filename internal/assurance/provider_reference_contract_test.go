package assurance

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/FelixSeptem/stele/internal/provider"
	"github.com/FelixSeptem/stele/openapi"
)

func referenceConsumerFixture() (provider.CanonicalScopeDescriptor, []byte) {
	scope := provider.CanonicalScopeDescriptor{Tenant: "t", Project: "p", Namespace: "n", AgentID: "a", SessionID: "s", ProviderInstanceID: "i"}
	data := []byte(`{"metadata":{"request_id":"r","operation_id":"o","schema_version":"provider-v1"},"contract_version":"durable-reference-v1","scope":{"tenant":"t","project":"p","namespace":"n","agent_id":"a","session_id":"s","provider_instance_id":"i"},"status":"completed","retry_after_ms":0,"retryable":false,"reference":{"reference_id":"task-one","memory_path":"tasks/one","event_type":"provider.reference.task","event_id":"11111111-1111-4111-8111-111111111111","candidate_id":"22222222-2222-4222-8222-222222222222","memory_id":"33333333-3333-4333-8333-333333333333","version_id":"44444444-4444-4444-8444-444444444444","provenance_id":"55555555-5555-4555-8555-555555555555","version":1,"value":"bounded evidence"}}`)
	return scope, data
}

func TestReferenceConsumerRejectsIncompleteOrNoncanonicalJSON(t *testing.T) {
	c, err := DefaultProviderReferenceContract()
	if err != nil {
		t.Fatal(err)
	}
	scope, data := referenceConsumerFixture()
	if _, err = c.Validate(data, scope, "provider-v1", "bounded_content"); err != nil {
		t.Fatal(err)
	}
	cases := map[string]func(map[string]any){
		"missing status":      func(v map[string]any) { delete(v, "status") },
		"null reference":      func(v map[string]any) { v["reference"] = nil },
		"unknown field":       func(v map[string]any) { v["task_state"] = "completed" },
		"alias":               func(v map[string]any) { v["contractVersion"] = v["contract_version"]; delete(v, "contract_version") },
		"unsupported version": func(v map[string]any) { v["contract_version"] = "durable-reference-v0" },
		"foreign scope":       func(v map[string]any) { v["scope"].(map[string]any)["tenant"] = "foreign" },
		"different session":   func(v map[string]any) { v["scope"].(map[string]any)["session_id"] = "another" },
		"missing lineage":     func(v map[string]any) { delete(v["reference"].(map[string]any), "provenance_id") },
		"invalid UUID":        func(v map[string]any) { v["reference"].(map[string]any)["event_id"] = "event-one" },
		"nil UUID": func(v map[string]any) {
			v["reference"].(map[string]any)["version_id"] = "00000000-0000-0000-0000-000000000000"
		},
		"UUID alias": func(v map[string]any) {
			v["reference"].(map[string]any)["event_id"] = "{11111111-1111-4111-8111-111111111111}"
		},
		"noncanonical path":           func(v map[string]any) { v["reference"].(map[string]any)["memory_path"] = "/tasks/one/" },
		"path traversal":              func(v map[string]any) { v["reference"].(map[string]any)["memory_path"] = "tasks/../one" },
		"invalid ID token":            func(v map[string]any) { v["reference"].(map[string]any)["reference_id"] = "task/one" },
		"unavailable leaks reference": func(v map[string]any) { v["status"] = "unavailable" },
		"pending leaks reference":     func(v map[string]any) { v["status"] = "pending" },
		"terminal retry":              func(v map[string]any) { v["retryable"] = true },
		"oversized value":             func(v map[string]any) { v["reference"].(map[string]any)["value"] = strings.Repeat("🙂", 1025) },
	}
	for name, mutate := range cases {
		t.Run(name, func(t *testing.T) {
			var v map[string]any
			_ = json.Unmarshal(data, &v)
			mutate(v)
			bad, _ := json.Marshal(v)
			if _, err := c.Validate(bad, scope, "provider-v1", "bounded_content"); err == nil || err.Error() != "provider reference failed-contract" {
				t.Fatalf("wanted bounded failure, got %v", err)
			}
		})
	}
	for _, bad := range [][]byte{[]byte(strings.Replace(string(data), `"status":"completed"`, `"status":"completed","status":"completed"`, 1)), []byte(strings.Replace(string(data), `"value":"bounded evidence"`, `"value":"\ud800"`, 1))} {
		if _, err := c.Validate(bad, scope, "provider-v1", "bounded_content"); err == nil {
			t.Fatal("invalid JSON accepted")
		}
	}
	if _, err := c.Validate(data, scope, "provider-v1", "reference_only"); err == nil {
		t.Fatal("reference mode accepted content")
	}
	var v map[string]any
	_ = json.Unmarshal(data, &v)
	delete(v["reference"].(map[string]any), "value")
	referenceOnly, _ := json.Marshal(v)
	if _, err := c.Validate(referenceOnly, scope, "provider-v1", "reference_only"); err != nil {
		t.Fatal(err)
	}
}

func TestReferenceConsumerRequiresExplicitCapability(t *testing.T) {
	valid := []byte(`{"durable_references":{"enabled":true,"contract_version":"durable-reference-v1","event_types":["provider.reference.task","provider.reference.run","provider.reference.handoff","provider.reference.compilation"],"statuses":["accepted","pending","failed","completed","unavailable"],"max_value_bytes":4096,"max_reference_id_bytes":128,"max_path_bytes":512,"max_response_bytes":16384,"poll_after_ms":1000}}`)
	if err := ValidateProviderReferenceCapability(valid); err != nil {
		t.Fatal(err)
	}
	for _, bad := range [][]byte{[]byte(`{"schema_version":"provider-v1"}`), []byte(strings.Replace(string(valid), `"enabled":true`, `"enabled":false`, 1)), []byte(strings.Replace(string(valid), "durable-reference-v1", "durable-reference-v0", 1)), []byte(strings.Replace(string(valid), `"max_value_bytes":4096`, `"max_value_bytes":null`, 1)), []byte(strings.Replace(string(valid), `"poll_after_ms":1000`, `"poll_after_ms":1000,"unknown":1`, 1))} {
		if err := ValidateProviderReferenceCapability(bad); err == nil {
			t.Fatal("incomplete capability accepted")
		}
	}
}

func TestReferenceContractRequiresDedicatedSchemasOnEveryPublicRoute(t *testing.T) {
	spec := openapi.SpecYAML()
	bad := strings.Replace(spec, "schema: {$ref: '#/components/schemas/ProviderReferenceResponse'}", "schema: {$ref: '#/components/schemas/ProviderOperationResponse'}", 1)
	if _, err := NewProviderReferenceContract([]byte(bad)); err == nil {
		t.Fatal("generic acknowledgement accepted as reference schema")
	}
}
