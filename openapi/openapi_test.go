package openapi

import (
	"strings"
	"testing"

	"github.com/getkin/kin-openapi/openapi3"
)

func TestSpecYAMLContainsBaselineEndpoints(t *testing.T) {
	spec := SpecYAML()

	for _, want := range []string{"/health", "/ready", "/v1/events", "/v1/memories/search", "/v1/context/assemble", "/v1/admin/jobs/governance/status", "/v1/admin/jobs/status", "/v1/admin/memories/{memory_id}/history", "/v1/admin/embedding/rebuilds", "/v1/admin/memories/{memory_id}/embedding"} {
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

func TestSpecYAMLIncludesManualMutationRoutes(t *testing.T) {
	for _, want := range []string{
		"/v1/admin/memories",
		"/v1/admin/memories/{memory_id}",
		"/v1/admin/memories/{memory_id}:merge",
		"/v1/admin/memories/{memory_id}:reclassify",
		"expected_version",
		"source_memory_id",
		"target_class",
	} {
		if !strings.Contains(SpecYAML(), want) {
			t.Fatalf("SpecYAML() missing %q", want)
		}
	}
}

func TestSpecYAMLIncludesGovernanceRecoveryRoutes(t *testing.T) {
	for _, want := range []string{
		"/v1/admin/governance/raw-events",
		"/v1/admin/governance/raw-events/{raw_event_id}",
		"/v1/admin/governance/raw-events/{raw_event_id}/recovery-history",
		"/v1/admin/governance/raw-events/{raw_event_id}:retry",
		"/v1/admin/governance/raw-events/{raw_event_id}:reschedule",
		"/v1/admin/governance/raw-events/{raw_event_id}:requeue",
		"attempt_gte",
		"attempt_lte",
		"failed_from",
		"failed_to",
		"next_attempt_from",
		"next_attempt_to",
		"cursor",
		"scheduled_for",
		"next_cursor",
	} {
		if !strings.Contains(SpecYAML(), want) {
			t.Fatalf("SpecYAML() missing %q", want)
		}
	}
}

func TestSpecYAMLIncludesGovernanceRecoverySchemas(t *testing.T) {
	for _, want := range []string{
		"GovernanceRawEvent",
		"GovernanceRawEventListResponse",
		"GovernanceRecoveryHistoryResponse",
		"GovernanceRecoveryActionRequest",
		"GovernanceRecoverySnapshot",
		"GovernanceRecoveryRecord",
		"GovernanceRecoveryOutcome",
		"raw_event_id",
		"worker_id",
		"claimed_at",
		"lease_until",
		"last_failed_at",
		"last_error",
		"next_attempt_at",
		"exhausted_at",
		"processed_at",
		"occurred_at",
		"before",
		"after",
		"raw_event",
		"recovery",
	} {
		if !strings.Contains(SpecYAML(), want) {
			t.Fatalf("SpecYAML() missing %q", want)
		}
	}
}

func TestSpecYAMLIncludesEmbeddingAdminInspectionSchemas(t *testing.T) {
	for _, want := range []string{
		"/v1/admin/embedding/rebuilds",
		"/v1/admin/memories/{memory_id}/embedding",
		"requested_provider",
		"requested_model",
		"requested_dimensions",
		"active_vector_revision_id",
		"drifted",
		"semantic_rebuild_enabled",
		"registered_providers",
		"EmbeddingRuntimeStatus",
		"EmbeddingRebuildView",
		"EmbeddingRebuildListResponse",
		"EmbeddingMemoryInspection",
		"EmbeddingVectorRevision",
	} {
		if !strings.Contains(SpecYAML(), want) {
			t.Fatalf("SpecYAML() missing %q", want)
		}
	}
}

func TestSpecYAMLIncludesEmbeddingRecoveryRoutesAndSchemas(t *testing.T) {
	for _, want := range []string{
		"/v1/admin/embedding/rebuilds/{memory_id}:retry",
		"/v1/admin/embedding/rebuilds/{memory_id}:requeue",
		"/v1/admin/embedding/recovery-history",
		"/v1/admin/memories/{memory_id}/embedding/recovery-history",
		"EmbeddingRecoveryAction",
		"EmbeddingRecoveryActionRequest",
		"EmbeddingRecoverySnapshot",
		"EmbeddingRecoveryRecord",
		"EmbeddingRecoveryHistoryResponse",
		"EmbeddingRecoveryOutcome",
		"cutover_plan_id",
		"occurred_at",
		"before",
		"after",
		"rebuild",
		"recovery",
	} {
		if !strings.Contains(SpecYAML(), want) {
			t.Fatalf("SpecYAML() missing %q", want)
		}
	}
}

func TestSpecYAMLIncludesEmbeddingCutoverRoutesAndSchemas(t *testing.T) {
	for _, want := range []string{
		"/v1/admin/embedding/cutovers",
		"/v1/admin/embedding/cutovers/{cutover_plan_id}",
		"/v1/admin/embedding/cutovers/{cutover_plan_id}:activate",
		"/v1/admin/embedding/cutovers/{cutover_plan_id}:pause",
		"/v1/admin/embedding/cutovers/{cutover_plan_id}:cancel",
		"EmbeddingCutoverTarget",
		"EmbeddingCutoverPlanRequest",
		"EmbeddingCutoverActionRequest",
		"EmbeddingCutoverPlanStatus",
		"EmbeddingCutoverItemStatus",
		"EmbeddingCutoverProgress",
		"EmbeddingCutoverItem",
		"EmbeddingCutoverPlan",
		"EmbeddingCutoverPlanListResponse",
		"wave_size",
		"classes",
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
