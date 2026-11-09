package openapi

import (
	"context"
	"strings"
	"testing"

	"github.com/getkin/kin-openapi/openapi3"
)

func TestProceduralVerificationOpenAPIContract(t *testing.T) {
	loader := openapi3.NewLoader()
	doc, err := loader.LoadFromData([]byte(SpecYAML()))
	if err != nil {
		t.Fatalf("load OpenAPI document: %v", err)
	}
	if err := doc.Validate(context.Background()); err != nil {
		t.Fatalf("validate OpenAPI document: %v", err)
	}

	requiredParameters := []string{
		"#/components/parameters/AdminAPIKey",
		"#/components/parameters/TenantHeader",
		"#/components/parameters/ProjectHeader",
		"#/components/parameters/NamespaceHeader",
	}
	for path, methods := range map[string][]string{
		"/v1/admin/procedural-memory-verifications":                              {"get", "post"},
		"/v1/admin/procedural-memory-verifications/{verification_id}":            {"get"},
		"/v1/admin/procedural-memory-verifications/{verification_id}:revalidate": {"post"},
		"/v1/admin/procedural-memory-verifications/{verification_id}:correct":    {"post"},
		"/v1/admin/procedural-revalidation-runs/{run_id}":                        {"get"},
	} {
		item := doc.Paths.Value(path)
		if item == nil {
			t.Fatalf("procedural verification path %q is missing", path)
		}
		for _, method := range methods {
			op := operationForMethod(item, method)
			if op == nil {
				t.Fatalf("procedural verification operation %s %s is missing", method, path)
			}
			refs := map[string]bool{}
			for _, parameter := range op.Parameters {
				refs[parameter.Ref] = true
			}
			for _, ref := range requiredParameters {
				if !refs[ref] {
					t.Errorf("%s %s missing required parameter %s", method, path, ref)
				}
			}
			if refs["#/components/parameters/PublicAPIKey"] {
				t.Errorf("%s %s must not publish public API-key access", method, path)
			}
		}
	}

	assertSchemaRef := func(path, method, want string) {
		t.Helper()
		op := operationForMethod(doc.Paths.Value(path), method)
		if op == nil || op.RequestBody == nil || op.RequestBody.Value == nil {
			t.Fatalf("%s %s has no request body", method, path)
		}
		if got := op.RequestBody.Value.Content.Get("application/json").Schema.Ref; got != want {
			t.Fatalf("%s %s request schema = %q, want %q", method, path, got, want)
		}
	}
	assertSchemaRef("/v1/admin/procedural-memory-verifications", "post", "#/components/schemas/ProceduralVerificationRecordRequest")
	assertSchemaRef("/v1/admin/procedural-memory-verifications/{verification_id}:revalidate", "post", "#/components/schemas/ProceduralVerificationRevalidationRequest")
	assertSchemaRef("/v1/admin/procedural-memory-verifications/{verification_id}:correct", "post", "#/components/schemas/ProceduralVerificationCorrectionRequest")

	for _, path := range []string{
		"/v1/admin/procedural-memory-verifications",
		"/v1/admin/procedural-memory-verifications/{verification_id}:revalidate",
		"/v1/admin/procedural-memory-verifications/{verification_id}:correct",
	} {
		op := operationForMethod(doc.Paths.Value(path), "post")
		if op.Responses.Value("409") == nil {
			t.Errorf("POST %s must describe replay/conflict behavior", path)
		}
	}

	for _, schemaName := range []string{"ProceduralVerificationReceipt", "ProceduralRevalidationRunResponse"} {
		schema := doc.Components.Schemas[schemaName]
		if schema == nil || schema.Value == nil {
			t.Fatalf("response schema %q is missing", schemaName)
		}
		for _, forbidden := range []string{"evidence", "corrected_content", "lease_owner", "work_identity"} {
			if schema.Value.Properties[forbidden] != nil {
				t.Errorf("response schema %q exposes forbidden field %q", schemaName, forbidden)
			}
		}
	}

	for path := range doc.Paths.Map() {
		if strings.Contains(path, "procedural-memory-verifications") || strings.Contains(path, "procedural-revalidation-runs") {
			if !strings.HasPrefix(path, "/v1/admin/") {
				t.Errorf("procedural verification path %q must remain administrator-only", path)
			}
		}
	}
}

func TestProceduralVerificationTaskEvidenceIsPublishedWithoutMutationAuthority(t *testing.T) {
	loader := openapi3.NewLoader()
	doc, err := loader.LoadFromData([]byte(SpecYAML()))
	if err != nil {
		t.Fatalf("load OpenAPI document: %v", err)
	}
	kind := doc.Components.Schemas["TaskEvidenceTargetKind"]
	if kind == nil || kind.Value == nil {
		t.Fatal("task evidence target kind schema is missing")
	}
	found := false
	for _, value := range kind.Value.Enum {
		if value == "procedural_verification" {
			found = true
		}
	}
	if !found {
		t.Fatal("task evidence target kind must publish procedural_verification")
	}
	for _, schemaName := range []string{"TaskEvaluation", "TaskEvaluationReport"} {
		schema := doc.Components.Schemas[schemaName]
		if schema == nil || schema.Value == nil {
			t.Fatalf("task evaluation schema %q is missing", schemaName)
		}
		for _, forbidden := range []string{"corrected_content", "canonical_mutation", "verification_evidence"} {
			if schema.Value.Properties[forbidden] != nil {
				t.Errorf("task evaluation schema %q grants mutation/evidence disclosure via %q", schemaName, forbidden)
			}
		}
	}
}
