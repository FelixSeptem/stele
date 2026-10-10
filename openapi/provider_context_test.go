package openapi

import (
	"context"
	"encoding/json"
	"github.com/getkin/kin-openapi/openapi3"
	"testing"
)

func TestProviderContextSchemasAndExamples(t *testing.T) {
	doc, err := openapi3.NewLoader().LoadFromData([]byte(SpecYAML()))
	if err != nil {
		t.Fatal(err)
	}
	if err = doc.Validate(context.Background()); err != nil {
		t.Fatal(err)
	}
	input := doc.Components.Schemas["ProviderContextInput"].Value
	for _, example := range input.Examples {
		if err = input.VisitJSON(example); err != nil {
			t.Fatalf("example: %v", err)
		}
	}
	for _, body := range []string{`{"query":"q","budget":1}`, `{"query":"q","budget":2,"include_goal_context":true,"path":"tasks/a"}`} {
		var value any
		_ = json.Unmarshal([]byte(body), &value)
		if err = input.VisitJSON(value); err != nil {
			t.Fatal(err)
		}
	}
	for _, body := range []string{`{"query":" ","budget":1}`, `{"query":"q","budget":0}`, `{"query":"q","budget":null}`, `{"query":"q","budget":1,"PathPrefix":"a"}`, `{"query":"q","budget":1,"feedback_ranking_policy":"scope"}`, `{"query":"q","budget":1,"include_relations":null}`} {
		var value any
		_ = json.Unmarshal([]byte(body), &value)
		if err = input.VisitJSON(value); err == nil {
			t.Fatalf("schema accepted %s", body)
		}
	}
	if got := doc.Paths.Value("/v1/provider/context").Post.Responses.Value("200").Value.Content.Get("application/json").Schema.Ref; got != "#/components/schemas/ProviderContextResponse" {
		t.Fatalf("response ref=%s", got)
	}
}
