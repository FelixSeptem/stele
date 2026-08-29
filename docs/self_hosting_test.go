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
		"STELE_AUTH_BOOTSTRAP_ADMIN_KEY",
		"STELE_AUTH_DEFAULT_TENANT",
		"STELE_AUTH_DEFAULT_PROJECT",
		"STELE_AUTH_DEFAULT_NAMESPACE",
	} {
		if !strings.Contains(content, requiredConfig) {
			t.Fatalf("self-hosting smoke docs missing required config %q", requiredConfig)
		}
	}
	for _, obsolete := range []string{"STELE_AUTH_API_KEYS", "STELE_AUTH_ADMIN_API_KEYS"} {
		if strings.Contains(content, obsolete+"=") {
			t.Fatalf("self-hosting docs must not present obsolete auth setting as live configuration: %q", obsolete)
		}
	}
}

func TestBootstrapSmokeScriptIsDocumentedAndConstrained(t *testing.T) {
	script, err := os.ReadFile("../scripts/stele-bootstrap-smoke.ps1")
	if err != nil {
		t.Fatalf("read bootstrap smoke script: %v", err)
	}
	content := string(script)
	for _, want := range []string{"CredentialOutputDirectory", "/v1/admin/principals", "/grants", "bootstrap credential was still accepted", "/openapi.yaml", "/version", "/v1/events", "Idempotency-Key", "/v1/memories/search", "/v1/context/assemble", "/v1/admin/scope-proofs", "same-scope product smoke proof"} {
		if !strings.Contains(content, want) {
			t.Fatalf("bootstrap smoke script missing %q", want)
		}
	}
}

func TestDeploymentContractRejectsObsoleteAuthAndRequiresBootstrapVariables(t *testing.T) {
	compose, err := os.ReadFile("../docker-compose.yml")
	if err != nil {
		t.Fatalf("read docker-compose.yml: %v", err)
	}
	envExample, err := os.ReadFile("../.env.example")
	if err != nil {
		t.Fatalf("read .env.example: %v", err)
	}
	for _, content := range []string{string(compose), string(envExample)} {
		for _, obsolete := range []string{"STELE_AUTH_API_KEYS", "STELE_AUTH_ADMIN_API_KEYS"} {
			if strings.Contains(content, obsolete) {
				t.Fatalf("deployment asset contains obsolete auth setting %q", obsolete)
			}
		}
	}
	for _, required := range []string{"STELE_AUTH_BOOTSTRAP_ADMIN_KEY", "STELE_AUTH_DEFAULT_TENANT", "STELE_AUTH_DEFAULT_PROJECT", "STELE_AUTH_DEFAULT_NAMESPACE", "STELE_DATABASE_MIGRATION_POLICY"} {
		if !strings.Contains(string(compose), required) || !strings.Contains(string(envExample), required) {
			t.Fatalf("deployment contract missing required variable %q", required)
		}
	}
}

func TestBackupRecoveryScriptsExposeSafetyGuards(t *testing.T) {
	checks := map[string][]string{
		"../scripts/stele-backup.ps1":         {"SourceDsn", "Destination", "pg_dump", "SHA256", "manifest"},
		"../scripts/stele-restore.ps1":        {"Artifact", "Manifest", "TargetDsn", "ConfirmDestructive", "source-equal", "checksum mismatch", "pg_restore"},
		"../scripts/stele-restore-verify.ps1": {"TargetDsn", "Manifest", "schema_migrations", "X-Stele-Tenant", "authorized scoped service proof", "RecordAssurance", "backup_restore_proof"},
	}
	for path, required := range checks {
		contentBytes, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("read %s: %v", path, err)
		}
		content := string(contentBytes)
		for _, want := range required {
			if !strings.Contains(content, want) {
				t.Fatalf("%s missing safety/verification guard %q", path, want)
			}
		}
	}
}

func TestProductVerificationEntryPointDocumentsPrerequisiteAndOwnershipGuards(t *testing.T) {
	contentBytes, err := os.ReadFile("../scripts/stele-product-verify.ps1")
	if err != nil {
		t.Fatalf("read product verification script: %v", err)
	}
	content := string(contentBytes)
	for _, want := range []string{"STELE_PRODUCT_VERIFY_CI", "SKIP:", "COMPOSE_PROJECT_NAME", "--build -d", "--volumes --remove-orphans", "KeepResources", "stele-bootstrap-smoke.ps1"} {
		if !strings.Contains(content, want) {
			t.Fatalf("product verification entrypoint missing %q", want)
		}
	}
}

func TestSelfHostingDocsDescribeBoundedRuntimeTelemetry(t *testing.T) {
	contentBytes, err := os.ReadFile("self-hosting.md")
	if err != nil {
		t.Fatalf("read self-hosting.md: %v", err)
	}
	content := string(contentBytes)
	for _, want := range []string{"Runtime startup and drain telemetry", "migration validation", "readiness/drain", "without emitting DSNs", "Product verification emits", "only phase/result categories"} {
		if !strings.Contains(content, want) {
			t.Fatalf("self-hosting docs missing bounded telemetry contract %q", want)
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

func TestSelfHostingDocsIncludeTaskSuccessLoop(t *testing.T) {
	contentBytes, err := os.ReadFile("self-hosting.md")
	if err != nil {
		t.Fatalf("read self-hosting.md: %v", err)
	}
	content := string(contentBytes)
	spec := openapi.SpecYAML()

	for _, want := range []string{
		"External-agent task success loop",
		"/v1/task-evaluations",
		"/v1/task-evaluations/<task-evaluation-id>/report",
		"/v1/admin/task-evaluations",
		"/v1/admin/task-evaluations/<task-evaluation-id>",
		"/v1/admin/task-evaluations/<task-evaluation-id>/supersede",
		"/v1/admin/task-evaluations/summary",
		"Opaque caller evidence tokens",
		"bounded linked ids",
	} {
		if !strings.Contains(content, want) {
			t.Fatalf("self-hosting task success docs missing %q", want)
		}
	}

	for _, route := range []string{
		"/v1/task-evaluations",
		"/v1/task-evaluations/{evaluation_id}/report",
		"/v1/admin/task-evaluations",
		"/v1/admin/task-evaluations/{evaluation_id}",
		"/v1/admin/task-evaluations/summary",
		"/v1/admin/task-evaluations/{evaluation_id}/supersede",
	} {
		if !strings.Contains(spec, route) {
			t.Fatalf("OpenAPI spec missing documented task success route %q", route)
		}
	}
}

func TestSelfHostingDocsStateRemainingFeedbackLoopProductGaps(t *testing.T) {
	contentBytes, err := os.ReadFile("self-hosting.md")
	if err != nil {
		t.Fatalf("read self-hosting.md: %v", err)
	}
	content := string(contentBytes)

	for _, want := range []string{
		"Remaining feedback-loop product gaps",
		"governed ranking rollout policy",
		"SDK/UI collection surfaces",
		"external agent runtime integration",
		"Hosted incident management",
		"vendor-specific alert integrations",
		"advanced scoring calibration",
	} {
		if !strings.Contains(content, want) {
			t.Fatalf("self-hosting remaining product gaps docs missing %q", want)
		}
	}

	if strings.Contains(content, "Default feedback-aware ranking rollout. Current behavior is diagnostics-first") {
		t.Fatalf("self-hosting docs still describe governed ranking rollout as a remaining gap")
	}
}

func TestSelfHostingDocsIncludeAssuranceConformanceReadinessLoop(t *testing.T) {
	contentBytes, err := os.ReadFile("self-hosting.md")
	if err != nil {
		t.Fatalf("read self-hosting.md: %v", err)
	}
	content := string(contentBytes)
	spec := openapi.SpecYAML()

	for _, want := range []string{
		"Production-readiness assurance and conformance loop",
		"/v1/admin/assurance/health-evaluations",
		"/v1/admin/assurance/conformance-profiles",
		"/v1/admin/assurance/conformance-runs",
		"/v1/admin/assurance/readiness-reports",
		"/v1/admin/assurance/incidents",
		"/v1/admin/assurance/alert-candidates",
		"/v1/admin/assurance/recovery-verifications",
		"capacity/load proof",
		"backup/restore proof",
		"disabled`, `stdout`, and generic `webhook`",
		"HTTPS-by-default",
		"unsafe target rejection",
		"stele_assurance_health_evaluations_total",
		"stele_conformance_runs_total",
		"component=assurance event=lifecycle",
		"component=conformance event=lifecycle",
	} {
		if !strings.Contains(content, want) {
			t.Fatalf("self-hosting assurance/conformance docs missing %q", want)
		}
	}

	for _, route := range []string{
		"/v1/admin/assurance/health-evaluations",
		"/v1/admin/assurance/incidents",
		"/v1/admin/assurance/alert-candidates",
		"/v1/admin/assurance/conformance-profiles",
		"/v1/admin/assurance/conformance-runs",
		"/v1/admin/assurance/readiness-reports",
		"/v1/admin/assurance/recovery-verifications",
	} {
		if !strings.Contains(spec, route) {
			t.Fatalf("OpenAPI spec missing documented assurance route %q", route)
		}
	}
}

func TestSelfHostingDocsStateRemainingAssuranceProductGaps(t *testing.T) {
	contentBytes, err := os.ReadFile("self-hosting.md")
	if err != nil {
		t.Fatalf("read self-hosting.md: %v", err)
	}
	content := string(contentBytes)

	for _, want := range []string{
		"Remaining assurance product gaps",
		"SDK/UI collection surfaces",
		"external agent runtime implementation",
		"vendor-specific alert integrations",
		"hosted incident management",
		"adaptive scoring calibration",
		"model invocation",
		"prompt orchestration",
		"final-answer generation",
	} {
		if !strings.Contains(content, want) {
			t.Fatalf("self-hosting assurance gaps docs missing %q", want)
		}
	}
}

func TestSelfHostingDocsIncludeIntegrationEvidenceWorkflowGoldenPath(t *testing.T) {
	contentBytes, err := os.ReadFile("self-hosting.md")
	if err != nil {
		t.Fatalf("read self-hosting.md: %v", err)
	}
	content := string(contentBytes)
	spec := openapi.SpecYAML()
	for _, want := range []string{
		"Integration evidence workflow golden path",
		"/v1/admin/workflows/templates",
		"/v1/workflows/runs",
		"/v1/workflows/runs/<workflow-run-id>/steps",
		"/v1/workflows/runs/<workflow-run-id>/next-actions",
		"/v1/admin/workflows/runs/<workflow-run-id>/diagnostics",
		"/v1/admin/workflows/evidence-links/<evidence-link-id>/supersede",
		"STELE_WORKFLOW_MAINTENANCE_ENABLED",
		"stele_workflow_lifecycle_total",
		"external agent execution",
		"model invocation",
		"prompt orchestration",
		"final-answer generation",
	} {
		if !strings.Contains(content, want) {
			t.Fatalf("self-hosting workflow docs missing %q", want)
		}
	}
	for _, route := range []string{
		"/v1/admin/workflows/templates",
		"/v1/workflows/runs",
		"/v1/workflows/runs/{workflow_run_id}/steps",
		"/v1/workflows/runs/{workflow_run_id}/next-actions",
		"/v1/admin/workflows/runs/{workflow_run_id}/diagnostics",
		"/v1/admin/workflows/evidence-links/{evidence_link_id}/supersede",
	} {
		if !strings.Contains(spec, route) {
			t.Fatalf("OpenAPI spec missing documented workflow route %q", route)
		}
	}
}
