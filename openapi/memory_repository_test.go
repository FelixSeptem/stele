package openapi

import (
	"encoding/json"
	"testing"

	"github.com/getkin/kin-openapi/openapi3"
)

func TestMemoryRepositoryOpenAPIHasScopedRegistrationAndPreviewContracts(t *testing.T) {
	doc, err := openapi3.NewLoader().LoadFromData([]byte(SpecYAML()))
	if err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{
		"/v1/memory-repositories",
		"/v1/memory-repositories/{repository_id}/previews",
		"/v1/memory-repositories/{repository_id}/imports",
	} {
		operation := doc.Paths.Value(path)
		if operation == nil || operation.Post == nil {
			t.Fatalf("missing repository operation %s", path)
		}
		for _, ref := range []string{
			"#/components/parameters/PublicAPIKey",
			"#/components/parameters/TenantHeader",
			"#/components/parameters/ProjectHeader",
			"#/components/parameters/NamespaceHeader",
		} {
			found := false
			for _, parameter := range operation.Post.Parameters {
				found = found || parameter.Ref == ref
			}
			if !found {
				t.Fatalf("%s missing parameter %s", path, ref)
			}
		}
	}
	for _, name := range []string{
		"MemoryRepositoryRegistrationRequest",
		"MemoryRepositoryRegistrationResponse",
		"MemoryRepositoryPreviewRequest",
		"MemoryRepositoryPreviewResponse",
		"MemoryRepositorySnapshot",
	} {
		schema := doc.Components.Schemas[name]
		if schema == nil || schema.Value == nil {
			t.Fatalf("missing schema %s", name)
		}
		if schema.Value.AdditionalProperties.Has == nil || *schema.Value.AdditionalProperties.Has {
			t.Fatalf("schema %s must be closed", name)
		}
	}
}

func TestMemoryRepositoryOpenAPISeparatesSubmissionStatusAndManifest(t *testing.T) {
	doc, err := openapi3.NewLoader().LoadFromData([]byte(SpecYAML()))
	if err != nil {
		t.Fatal(err)
	}
	imports := doc.Paths.Value("/v1/memory-repositories/{repository_id}/imports")
	if imports == nil || imports.Post == nil || imports.Post.Responses.Value("202") == nil || imports.Post.Responses.Value("409") == nil {
		t.Fatal("bounded import submission contract is missing")
	}
	status := doc.Paths.Value("/v1/memory-repositories/{repository_id}/imports/{import_id}")
	manifest := doc.Paths.Value("/v1/memory-repositories/{repository_id}/manifest")
	if status == nil || status.Get == nil || manifest == nil || manifest.Get == nil {
		t.Fatal("status or manifest read contract is missing")
	}
	for _, name := range []string{"MemoryRepositoryImportRequest", "MemoryRepositoryImportResponse", "MemoryRepositoryImportStatus", "MemoryRepositoryManifest"} {
		schema := doc.Components.Schemas[name]
		if schema == nil || schema.Value == nil || schema.Value.AdditionalProperties.Has == nil || *schema.Value.AdditionalProperties.Has {
			t.Fatalf("closed schema %s is missing", name)
		}
	}
	items := doc.Components.Schemas["MemoryRepositoryManifest"].Value.Properties["items"].Value
	if items.MaxItems == nil || *items.MaxItems != 100 {
		t.Fatalf("manifest maxItems=%v, want 100", items.MaxItems)
	}
}

func TestMemoryRepositoryPreviewSchemaRequiresExplicitSelectionsAndBoundedSnapshot(t *testing.T) {
	doc, err := openapi3.NewLoader().LoadFromData([]byte(SpecYAML()))
	if err != nil {
		t.Fatal(err)
	}
	schema := doc.Components.Schemas["MemoryRepositoryPreviewRequest"].Value
	var valid map[string]any
	if err := json.Unmarshal([]byte(`{"actor":"agent-1","reason":"import reviewed notes","expected_generation":0,"snapshot":{"format_version":"memory-repo-v1","source_revision":"git:8798cb2","files":[{"path":"MEMORY.md","content":"- [[projects/payments]]\n"}]},"selections":[{"path":"MEMORY.md","entry_key":"entry-1","memory_class":"procedural","memory_path":"projects/payments"}]}`), &valid); err != nil {
		t.Fatal(err)
	}
	if err := schema.VisitJSON(valid); err != nil {
		t.Fatalf("valid preview request rejected: %v", err)
	}
	delete(valid, "selections")
	if err := schema.VisitJSON(valid); err == nil {
		t.Fatal("preview without explicit selections accepted")
	}
}
