package openapi

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/FelixSeptem/stele/internal/provider"
	"github.com/getkin/kin-openapi/openapi3"
)

func TestProjectionOpenAPIContract(t *testing.T) {
	d, err := openapi3.NewLoader().LoadFromData([]byte(SpecYAML()))
	if err != nil {
		t.Fatal(err)
	}
	if err = d.Validate(context.Background()); err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{"/v1/provider/projections", "/v1/provider/projections/history"} {
		if d.Paths.Value(path) == nil || d.Paths.Value(path).Post == nil {
			t.Errorf("missing projection route %s", path)
		}
	}
	for _, name := range []string{"ProviderProjectionOperationMetadata", "ProviderProjectionRequest", "ProviderProjectionHistoryRequest", "ProviderProjectionResponse", "ProviderProjectionHistoryResponse", "ProviderProjectionCapabilities", "ProviderProjectionInput", "ProviderProjectionRecord", "ProviderProjectionLineage"} {
		s := d.Components.Schemas[name]
		if s == nil {
			t.Errorf("missing %s", name)
			continue
		}
		if s.Value.AdditionalProperties.Has == nil || *s.Value.AdditionalProperties.Has {
			t.Errorf("unclosed %s", name)
		}
	}
}

func TestProjectionOpenAPIMetadataPreservesLegacySchema(t *testing.T) {
	d, err := openapi3.NewLoader().LoadFromData([]byte(SpecYAML()))
	if err != nil {
		t.Fatal(err)
	}
	value := map[string]any{"request_id": " r ", "operation_id": "o", "schema_version": "provider-v1", "idempotency_key": "key"}
	if err := d.Components.Schemas["ProviderOperationMetadata"].Value.VisitJSON(value); err != nil {
		t.Fatalf("legacy schema was narrowed: %v", err)
	}
	for _, name := range []string{"ProviderProjectionRequest", "ProviderProjectionHistoryRequest", "ProviderProjectionResponse", "ProviderProjectionHistoryResponse"} {
		if err := d.Components.Schemas[name].Value.Properties["metadata"].Value.VisitJSON(value); err == nil {
			t.Errorf("%s metadata accepts a noncanonical request token", name)
		}
	}
}

func TestProjectionOpenAPIRequestSemanticParity(t *testing.T) {
	d, err := openapi3.NewLoader().LoadFromData([]byte(SpecYAML()))
	if err != nil {
		t.Fatal(err)
	}
	base := `{"metadata":{"request_id":"r","operation_id":"o","schema_version":"provider-v1","idempotency_key":"key"},"input":{"contract_version":"turn-outcome-feedback-projection-v1","kind":"outcome","effect_id":"effect","source_revision":1,"occurred_at":"2026-10-16T00:00:00Z","reported_outcome":"unknown"}}`
	for name, input := range map[string]string{
		"plain outcome":                    base,
		"turn without run":                 strings.Replace(base, `"kind":"outcome"`, `"kind":"outcome","turn_id":"turn"`, 1),
		"context reference without digest": strings.Replace(base, `"kind":"outcome"`, `"kind":"outcome","context_reference":"ctxc_aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"`, 1),
		"feedback without body":            strings.Replace(base, `"kind":"outcome"`, `"kind":"feedback"`, 1),
		"feedback without references":      strings.Replace(base, `"kind":"outcome"`, `"kind":"outcome","feedback":{"signal":"useful","reason_code":"helpful"}`, 1),
		"feedback empty references":        strings.Replace(base, `"kind":"outcome"`, `"kind":"feedback","references":[],"feedback":{"signal":"useful","reason_code":"helpful"}`, 1),
		"valid feedback":                   strings.Replace(base, `"kind":"outcome"`, `"kind":"feedback","references":[{"event_id":"11111111-1111-4111-8111-111111111111"}],"feedback":{"signal":"useful","reason_code":"helpful"}`, 1),
	} {
		t.Run(name, func(t *testing.T) {
			var value any
			if err := json.Unmarshal([]byte(input), &value); err != nil {
				t.Fatal(err)
			}
			_, producerErr := provider.DecodeProjectionRequest([]byte(input))
			schemaErr := d.Components.Schemas["ProviderProjectionRequest"].Value.VisitJSON(value)
			if (producerErr == nil) != (schemaErr == nil) {
				t.Fatalf("request schema/handler disagree: schema=%v handler=%v", schemaErr, producerErr)
			}
		})
	}
}

func TestProjectionOpenAPISchemaCompatibilityError(t *testing.T) {
	d, err := openapi3.NewLoader().LoadFromData([]byte(SpecYAML()))
	if err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{"/v1/provider/projections", "/v1/provider/projections/history"} {
		description := *d.Paths.Value(path).Post.Responses.Value("409").Value.Description
		if !strings.Contains(description, "unsupported_schema,") || strings.Contains(description, "unsupported_schema_version") {
			t.Errorf("%s must document the handler's unsupported_schema compatibility code: %s", path, description)
		}
	}
}

func TestProjectionOpenAPIValidPC4Reference(t *testing.T) {
	d, err := openapi3.NewLoader().LoadFromData([]byte(SpecYAML()))
	if err != nil {
		t.Fatal(err)
	}
	var value any
	json.Unmarshal([]byte(`{"contract_version":"turn-outcome-feedback-projection-v1","kind":"outcome","effect_id":"effect","source_revision":1,"occurred_at":"2026-10-16T00:00:00Z","reported_outcome":"unknown","context_digest":"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa","context_reference":"ctxc_aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"}`), &value)
	if err = d.Components.Schemas["ProviderProjectionInput"].Value.VisitJSON(value); err != nil {
		t.Fatal(err)
	}
}

func TestProjectionOpenAPIErrorsAreClosedWithoutNarrowingLegacy(t *testing.T) {
	d, err := openapi3.NewLoader().LoadFromData([]byte(SpecYAML()))
	if err != nil {
		t.Fatal(err)
	}
	var value any
	if err := json.Unmarshal([]byte(`{"error":{"category":"conflict","code":"projection_conflict","message":"bounded failure","retryable":false,"task_state":"done"}}`), &value); err != nil {
		t.Fatal(err)
	}
	if err := d.Components.Schemas["ProviderErrorResponse"].Value.VisitJSON(value); err != nil {
		t.Fatalf("legacy error schema narrowed: %v", err)
	}
	for _, path := range []string{"/v1/provider/projections", "/v1/provider/projections/history"} {
		for _, status := range []string{"400", "403", "409", "410", "413", "503"} {
			schema := d.Paths.Value(path).Post.Responses.Value(status).Value.Content.Get("application/json").Schema
			if schema.Ref != "#/components/schemas/ProviderProjectionErrorResponse" {
				t.Errorf("%s %s uses shared error schema", path, status)
			}
			if err := schema.Value.VisitJSON(value); err == nil {
				t.Errorf("%s %s accepts unknown nested error field", path, status)
			}
		}
	}
}
