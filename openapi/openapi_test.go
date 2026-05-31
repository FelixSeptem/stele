package openapi

import (
	"strings"
	"testing"

	"github.com/getkin/kin-openapi/openapi3"
)

func TestSpecYAMLContainsBaselineEndpoints(t *testing.T) {
	spec := SpecYAML()

	for _, want := range []string{"/health", "/ready", "/v1/events"} {
		if !strings.Contains(spec, want) {
			t.Fatalf("SpecYAML() missing path %q", want)
		}
	}
}

func TestSpecYAMLIsValidOpenAPI(t *testing.T) {
	loader := openapi3.NewLoader()
	doc, err := loader.LoadFromData([]byte(SpecYAML()))
	if err != nil {
		t.Fatalf("LoadFromData() error = %v", err)
	}

	if err := doc.Validate(loader.Context); err != nil {
		t.Fatalf("Validate() error = %v", err)
	}
}
