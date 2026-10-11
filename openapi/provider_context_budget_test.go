package openapi

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/getkin/kin-openapi/openapi3"
)

func TestProviderContextPC3BudgetInputContract(t *testing.T) {
	doc, err := openapi3.NewLoader().LoadFromData([]byte(SpecYAML()))
	if err != nil {
		t.Fatal(err)
	}
	if err := doc.Validate(context.Background()); err != nil {
		t.Fatal(err)
	}
	input := doc.Components.Schemas["ProviderContextInput"].Value
	limits := input.Properties["budget_limits"]
	if limits == nil || limits.Value == nil {
		t.Fatal("ProviderContextInput must expose budget_limits")
	}
	if limits.Value.AdditionalProperties.Has == nil || *limits.Value.AdditionalProperties.Has {
		t.Fatal("budget_limits must be closed")
	}
	if got := input.Properties["budget"].Value.Max; got == nil || *got != 2147483647 {
		t.Fatalf("budget maximum = %v, want 2147483647", got)
	}
	for name, wantMin := range map[string]float64{
		"max_content_bytes":  0,
		"max_response_bytes": 1,
		"max_citations":      0,
	} {
		property := limits.Value.Properties[name]
		if property == nil || property.Value == nil {
			t.Fatalf("budget_limits missing %s", name)
		}
		if property.Value.Min == nil || *property.Value.Min != wantMin || property.Value.Max == nil || *property.Value.Max != 2147483647 {
			t.Fatalf("%s bounds = min %v max %v", name, property.Value.Min, property.Value.Max)
		}
	}

	for _, body := range []string{
		`{"query":"q","budget":1,"budget_limits":{}}`,
		`{"query":"q","budget":2147483647,"budget_limits":{"max_content_bytes":0,"max_response_bytes":1,"max_citations":0}}`,
	} {
		var value any
		if err := json.Unmarshal([]byte(body), &value); err != nil {
			t.Fatal(err)
		}
		if err := input.VisitJSON(value); err != nil {
			t.Fatalf("valid budget input rejected: %v", err)
		}
	}
	for _, body := range []string{
		`{"query":"q","budget":0}`,
		`{"query":"q","budget":2147483648}`,
		`{"query":"q","budget":1,"budget_limits":{"max_content_bytes":-1}}`,
		`{"query":"q","budget":1,"budget_limits":{"max_response_bytes":0}}`,
		`{"query":"q","budget":1,"budget_limits":{"max_content_bytes":2147483648}}`,
		`{"query":"q","budget":1,"budget_limits":{"max_tokens":1}}`,
		`{"query":"q","budget":1,"budget_limits":{"max_citations":null}}`,
	} {
		var value any
		if err := json.Unmarshal([]byte(body), &value); err != nil {
			t.Fatal(err)
		}
		if err := input.VisitJSON(value); err == nil {
			t.Fatalf("schema accepted invalid budget input %s", body)
		}
	}
}

func TestProviderContextPC3BudgetAccountingIsClosedAndRequired(t *testing.T) {
	doc, err := openapi3.NewLoader().LoadFromData([]byte(SpecYAML()))
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"ProviderReferenceContextResult", "ProviderContentContextResult"} {
		result := doc.Components.Schemas[name].Value
		if result.Properties["budget_accounting"] == nil {
			t.Fatalf("%s missing budget_accounting", name)
		}
		found := false
		for _, required := range result.Required {
			if required == "budget_accounting" {
				found = true
			}
		}
		if !found {
			t.Fatalf("%s does not require budget_accounting", name)
		}
	}
	accounting := doc.Components.Schemas["ProviderContextBudgetAccounting"]
	if accounting == nil || accounting.Value == nil {
		t.Fatal("ProviderContextBudgetAccounting schema missing")
	}
	if accounting.Value.AdditionalProperties.Has == nil || *accounting.Value.AdditionalProperties.Has {
		t.Fatal("budget accounting must be closed")
	}
	for _, field := range []string{"contract_version", "packing_policy_version", "requested", "effective", "usage", "selection"} {
		if accounting.Value.Properties[field] == nil {
			t.Fatalf("budget accounting missing %s", field)
		}
	}
	for field, version := range map[string]string{"contract_version": "context-budget-v1", "packing_policy_version": "context-packing-v1"} {
		property := accounting.Value.Properties[field]
		if property.Value == nil || len(property.Value.Enum) != 1 || property.Value.Enum[0] != version {
			t.Fatalf("%s must be fixed to its PC3 version", field)
		}
	}
	requested := accounting.Value.Properties["requested"].Value
	for _, field := range []string{"max_content_bytes", "max_response_bytes", "max_citations"} {
		if !requested.Properties[field].Value.Type.IncludesNull() {
			t.Fatalf("requested.%s must declare an OpenAPI 3.1 integer/null type union", field)
		}
	}
	const valid = `{"contract_version":"context-budget-v1","packing_policy_version":"context-packing-v1","requested":{"max_items":1,"max_content_bytes":null,"max_response_bytes":null,"max_citations":null},"effective":{"max_items":1,"max_content_bytes":0,"max_response_bytes":1024,"max_citations":1},"usage":{"items":0,"content_bytes":0,"response_bytes":800,"citations":0},"selection":{"outcome":"empty","omission_reasons":[],"unsupported_sections":[]}}`
	for _, body := range []string{valid, strings.ReplaceAll(valid, "null", "1")} {
		var value any
		if err := json.Unmarshal([]byte(body), &value); err != nil {
			t.Fatal(err)
		}
		if err := accounting.Value.VisitJSON(value); err != nil {
			t.Fatalf("valid accounting rejected: %v", err)
		}
	}
	for _, body := range []string{
		strings.Replace(valid, "context-budget-v1", "context-budget-v2", 1),
		strings.Replace(valid, "context-packing-v1", "experiment-private", 1),
		strings.Replace(valid, `"max_content_bytes":null,`, "", 1),
		strings.Replace(valid, `"max_items":1`, `"max_items":0`, 1),
		strings.Replace(valid, `"max_items":1`, `"max_items":2147483648`, 1),
		strings.Replace(valid, `"items":0`, `"items":-1`, 1),
		strings.Replace(valid, `"items":0`, `"items":1001`, 1),
		strings.Replace(valid, `"outcome":"empty"`, `"outcome":"hidden"`, 1),
		strings.Replace(valid, `"omission_reasons":[]`, `"omission_reasons":["foreign_scope"]`, 1),
		strings.Replace(valid, `"omission_reasons":[]`, `"omission_reasons":["citations","citations"]`, 1),
		strings.Replace(valid, `"unsupported_sections":[]`, `"unsupported_sections":["projections"]`, 1),
		strings.Replace(valid, `"unsupported_sections":[]`, `"unsupported_sections":["diagnostics","diagnostics"]`, 1),
		strings.Replace(valid, `"usage":{`, `"usage":{"hidden_count":1,`, 1),
	} {
		var value any
		if err := json.Unmarshal([]byte(body), &value); err != nil {
			t.Fatal(err)
		}
		if err := accounting.Value.VisitJSON(value); err == nil {
			t.Fatalf("schema accepted invalid accounting %s", body)
		}
	}
	for _, example := range accounting.Value.Examples {
		if err := accounting.Value.VisitJSON(example); err != nil {
			t.Fatalf("budget accounting example: %v", err)
		}
	}
}

func TestProviderContextPC3CapabilitiesAdvertiseBudgetCeilings(t *testing.T) {
	doc, err := openapi3.NewLoader().LoadFromData([]byte(SpecYAML()))
	if err != nil {
		t.Fatal(err)
	}
	capability := doc.Components.Schemas["ProviderCapabilityDocument"].Value
	for _, field := range []string{"context_budget_version", "context_packing_policy_version", "token_budget_supported"} {
		if capability.Properties[field] == nil {
			t.Fatalf("capability document missing %s", field)
		}
	}
	limits := capability.Properties["limits"].Value
	for _, field := range []string{"max_context_items", "max_context_content_bytes", "max_context_bytes", "max_citations"} {
		if limits.Properties[field] == nil {
			t.Fatalf("capability limits missing %s", field)
		}
	}
	items := limits.Properties["max_context_items"].Value
	if items.Min == nil || *items.Min != 1 || items.Max == nil || *items.Max != 1000 {
		t.Fatalf("max_context_items bounds = min %v max %v", items.Min, items.Max)
	}
	content := limits.Properties["max_context_content_bytes"].Value
	if content.Min == nil || *content.Min != 0 || content.Max == nil || *content.Max != 4194304 {
		t.Fatalf("max_context_content_bytes bounds = min %v max %v", content.Min, content.Max)
	}
	if err := capability.Properties["token_budget_supported"].Value.VisitJSON(false); err != nil {
		t.Fatalf("token_budget_supported must accept false: %v", err)
	}
	if err := capability.Properties["token_budget_supported"].Value.VisitJSON(true); err == nil {
		t.Fatal("token_budget_supported must reject true")
	}
}

func TestProviderContextPC3ErrorsPublishBoundedEnvelopes(t *testing.T) {
	doc, err := openapi3.NewLoader().LoadFromData([]byte(SpecYAML()))
	if err != nil {
		t.Fatal(err)
	}
	operation := doc.Paths.Value("/v1/provider/context").Post
	for _, status := range []string{"400", "403", "413", "503"} {
		response := operation.Responses.Value(status)
		if response == nil || response.Value == nil {
			t.Fatalf("missing context response %s", status)
		}
		content := response.Value.Content.Get("application/json")
		if content == nil || content.Schema == nil {
			t.Fatalf("context response %s missing typed error envelope", status)
		}
		if content.Example == nil {
			t.Fatalf("context response %s missing bounded example", status)
		}
		if err := content.Schema.Value.VisitJSON(content.Example); err != nil {
			t.Fatalf("context response %s invalid example: %v", status, err)
		}
	}
}

func TestProviderContextPC3CanonicalResponseExamples(t *testing.T) {
	doc, err := openapi3.NewLoader().LoadFromData([]byte(SpecYAML()))
	if err != nil {
		t.Fatal(err)
	}
	response := doc.Components.Schemas["ProviderContextResponse"].Value
	if len(response.Examples) < 2 {
		t.Fatal("canonical response examples must cover both response modes")
	}
	for _, example := range response.Examples {
		if err := response.VisitJSON(example); err != nil {
			t.Fatalf("canonical response example rejected: %v", err)
		}
		body, err := json.Marshal(example)
		if err != nil {
			t.Fatal(err)
		}
		var decoded map[string]any
		if err := json.Unmarshal(body, &decoded); err != nil {
			t.Fatal(err)
		}
		result := decoded["result"].(map[string]any)
		accounting := result["budget_accounting"].(map[string]any)
		usage := accounting["usage"].(map[string]any)
		if got := usage["response_bytes"].(float64); got != float64(len(body)) {
			t.Errorf("%s example response_bytes = %.0f, actual serialized body = %d", result["response_mode"], got, len(body))
		}
		for _, field := range []string{"budget_accounting", "citations", "profile"} {
			original := result[field]
			delete(result, field)
			if err := response.VisitJSON(decoded); err == nil {
				t.Fatalf("response accepted missing required %s", field)
			}
			result[field] = original
		}
		for _, field := range []string{"diagnostics", "goal_context", "known_failures", "projections"} {
			result[field] = map[string]any{}
			if err := response.VisitJSON(decoded); err == nil {
				t.Fatalf("response reopened unsupported public section %s", field)
			}
			delete(result, field)
		}
	}
}
