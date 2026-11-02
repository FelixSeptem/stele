package openapi

import (
	"context"
	"testing"

	"github.com/getkin/kin-openapi/openapi3"
)

func TestProviderCorrectionOperationAndCapabilityArePublished(t *testing.T) {
	doc, err := openapi3.NewLoader().LoadFromData([]byte(SpecYAML()))
	if err != nil {
		t.Fatal(err)
	}
	if err = doc.Validate(context.Background()); err != nil {
		t.Fatal(err)
	}
	operation := doc.Paths.Value("/v1/provider/corrections")
	if operation == nil || operation.Post == nil {
		t.Fatal("provider correction operation is missing")
	}
	if got := operation.Post.RequestBody.Value.Content.Get("application/json").Schema.Ref; got != "#/components/schemas/ProviderCorrectionRequest" {
		t.Fatalf("request schema ref = %q", got)
	}
	capabilities := doc.Components.Schemas["ProviderCapabilityDocument"].Value
	if capabilities.Properties["corrections"] == nil {
		t.Fatal("correction capability is missing")
	}
	input := doc.Components.Schemas["ProviderCorrectionInput"].Value
	for _, forbidden := range []string{"binding_id", "principal_id", "scope", "disclosure_profile", "source_revision", "reference_lineage"} {
		if input.Properties[forbidden] != nil {
			t.Fatalf("caller-controlled authority field %q is published", forbidden)
		}
	}
}
