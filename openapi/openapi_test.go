package openapi

import (
	"strings"
	"testing"

	"github.com/getkin/kin-openapi/openapi3"
)

func TestSpecYAMLContainsBaselineEndpoints(t *testing.T) {
	spec := SpecYAML()

	for _, want := range []string{"/health", "/ready", "/v1/events", "/v1/memories/search", "/v1/context/assemble", "/v1/admin/jobs/governance/status", "/v1/admin/jobs/status", "/v1/admin/memories/{memory_id}/history"} {
		if !strings.Contains(spec, want) {
			t.Fatalf("SpecYAML() missing path %q", want)
		}
	}
}

func TestSpecYAMLIncludesMemoryManagementRoutes(t *testing.T) {
	for _, want := range []string{
		"/v1/memories",
		"/v1/memories/{memory_id}",
		"/v1/memories/{memory_id}/history",
		"/v1/memories/{memory_id}/provenance",
		"/v1/admin/memories/{memory_id}:suppress",
		"/v1/admin/memories/{memory_id}:expire",
		"/v1/admin/memories/{memory_id}:delete",
		"request_id",
		"source_context",
		"X-Stele-Actor",
	} {
		if !strings.Contains(SpecYAML(), want) {
			t.Fatalf("SpecYAML() missing %q", want)
		}
	}
}

func TestSpecYAMLContainsSearchTimeWindowFields(t *testing.T) {
	spec := SpecYAML()

	for _, want := range []string{"time_from", "time_to", "query_embedding"} {
		if !strings.Contains(spec, want) {
			t.Fatalf("SpecYAML() missing search field %q", want)
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
