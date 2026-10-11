package openapi

import (
	"github.com/getkin/kin-openapi/openapi3"
	"testing"
)

func TestDisclosureProvisioningSchemas(t *testing.T) {
	doc, err := openapi3.NewLoader().LoadFromData([]byte(SpecYAML()))
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"DisclosureEntitlements", "SourceMetadata", "GrantDisclosureRequest"} {
		ref := doc.Components.Schemas[name]
		if ref == nil {
			t.Fatalf("missing schema %s", name)
		}
		if ref.Value.AdditionalProperties.Has == nil || *ref.Value.AdditionalProperties.Has {
			t.Fatalf("open schema %s", name)
		}
	}
	if doc.Paths.Value("/v1/admin/principals/{principal_id}/grants/{grant_id}/disclosure") == nil {
		t.Fatal("missing admin disclosure provisioning route")
	}
}
