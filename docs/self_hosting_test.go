package docs_test

import (
	"os"
	"strings"
	"testing"

	"github.com/FelixSeptem/stele/openapi"
)

func TestSelfHostingSmokeLoopDocumentsReplayContextAndMetrics(t *testing.T) {
	contentBytes, err := os.ReadFile("self-hosting.md")
	if err != nil {
		t.Fatalf("read self-hosting.md: %v", err)
	}
	content := string(contentBytes)
	spec := openapi.SpecYAML()

	for _, want := range []string{
		"Smoke fixture scope",
		"tenant-a",
		"project-a",
		"namespace-a",
		"smoke.provider_failure",
		"smoke.operator_recovery",
		"/v1/admin/derived-insight-replays:dry-run",
		"/v1/admin/derived-insight-replays",
		"/v1/admin/derived-insight-replays/<replay-run-id>/report",
		`"include_diagnostics":true`,
		"stele_derived_insight_replay_total",
	} {
		if !strings.Contains(content, want) {
			t.Fatalf("self-hosting smoke docs missing %q", want)
		}
	}

	for _, route := range []string{
		"/v1/context/assemble",
		"/v1/admin/derived-insight-replays:dry-run",
		"/v1/admin/derived-insight-replays",
		"/v1/admin/derived-insight-replays/{replay_run_id}/report",
	} {
		if !strings.Contains(spec, route) {
			t.Fatalf("OpenAPI spec missing documented route %q", route)
		}
	}

	for _, requiredConfig := range []string{
		"STELE_AUTH_API_KEYS",
		"STELE_AUTH_ADMIN_API_KEYS",
		"STELE_AUTH_DEFAULT_TENANT",
		"STELE_AUTH_DEFAULT_PROJECT",
		"STELE_AUTH_DEFAULT_NAMESPACE",
	} {
		if !strings.Contains(content, requiredConfig) {
			t.Fatalf("self-hosting smoke docs missing required config %q", requiredConfig)
		}
	}
}

func TestSelfHostingDocsIncludeMemoryQualityRepairLoop(t *testing.T) {
	contentBytes, err := os.ReadFile("self-hosting.md")
	if err != nil {
		t.Fatalf("read self-hosting.md: %v", err)
	}
	content := string(contentBytes)
	spec := openapi.SpecYAML()

	for _, want := range []string{
		"Memory quality and repair loop",
		"/v1/admin/memory-quality/evaluations",
		"/v1/admin/memory-quality/evaluations/<evaluation-run-id>/findings",
		"/v1/admin/memory-quality/repair-plans",
		"/v1/admin/memory-quality/repair-plans/<repair-plan-id>:approve",
		"/v1/admin/memory-quality/repair-plans/<repair-plan-id>:verify",
		"admission.decision",
		"stele_quality_",
		"manual_review",
	} {
		if !strings.Contains(content, want) {
			t.Fatalf("self-hosting quality repair docs missing %q", want)
		}
	}

	for _, route := range []string{
		"/v1/admin/memory-quality/evaluations",
		"/v1/admin/memory-quality/evaluations/{evaluation_run_id}/findings",
		"/v1/admin/memory-quality/repair-plans",
		"/v1/admin/memory-quality/repair-plans/{repair_plan_id}:approve",
		"/v1/admin/memory-quality/repair-plans/{repair_plan_id}:verify",
	} {
		if !strings.Contains(spec, route) {
			t.Fatalf("OpenAPI spec missing documented quality route %q", route)
		}
	}
}

func TestSelfHostingDocsIncludeScopeProofAndMemorySessionLoop(t *testing.T) {
	contentBytes, err := os.ReadFile("self-hosting.md")
	if err != nil {
		t.Fatalf("read self-hosting.md: %v", err)
	}
	content := string(contentBytes)
	spec := openapi.SpecYAML()

	for _, want := range []string{
		"Durable scope proof and memory session loop",
		"/v1/admin/scope-proofs",
		"/v1/admin/scope-proofs/<proof-run-id>/report",
		"/v1/admin/scope-proofs/<proof-run-id>:rerun",
		"/v1/memory-sessions",
		"/v1/memory-sessions/<session-id>/turns",
		"/v1/memory-sessions/<session-id>/turns/<turn-id>:outcome",
		"/v1/memory-sessions/<session-id>:verify",
		"/v1/memory-sessions/<session-id>/report",
		"inspect_context_diagnostics",
		"open_quality_evaluation",
		"open_repair_plan",
		"stele_scope_proof_steps_total",
		"stele_memory_session_verifications_total",
		"SDK/UI",
		"external agent runtime integration",
		"capacity/load proof",
		"backup/restore proof",
		"long-term memory usefulness scoring",
	} {
		if !strings.Contains(content, want) {
			t.Fatalf("self-hosting proof/session docs missing %q", want)
		}
	}

	for _, route := range []string{
		"/v1/admin/scope-proofs",
		"/v1/admin/scope-proofs/{proof_run_id}/report",
		"/v1/admin/scope-proofs/{proof_run_id}:rerun",
		"/v1/memory-sessions",
		"/v1/memory-sessions/{session_id}/turns",
		"/v1/memory-sessions/{session_id}/turns/{turn_id}:outcome",
		"/v1/memory-sessions/{session_id}:verify",
		"/v1/memory-sessions/{session_id}/report",
	} {
		if !strings.Contains(spec, route) {
			t.Fatalf("OpenAPI spec missing documented proof/session route %q", route)
		}
	}
}
