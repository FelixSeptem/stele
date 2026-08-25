package openapi

import (
	"strings"
	"testing"

	"github.com/getkin/kin-openapi/openapi3"
)

func TestSpecYAMLContainsBaselineEndpoints(t *testing.T) {
	spec := SpecYAML()

	for _, want := range []string{"/health", "/ready", "/livez", "/readyz", "/metrics", "/v1/events", "/v1/memories/search", "/v1/context/assemble", "/v1/admin/jobs/governance/status", "/v1/admin/jobs/status", "/v1/admin/memories/{memory_id}/history", "/v1/admin/embedding/rebuilds", "/v1/admin/memories/{memory_id}/embedding"} {
		if !strings.Contains(spec, want) {
			t.Fatalf("SpecYAML() missing path %q", want)
		}
	}
}

func TestEventIngestContractRequiresIdempotencyAndDocumentsReplay(t *testing.T) {
	loader := openapi3.NewLoader()
	doc, err := loader.LoadFromData([]byte(SpecYAML()))
	if err != nil {
		t.Fatalf("LoadFromData() error = %v", err)
	}
	operation := doc.Paths.Value("/v1/events").Post
	if operation == nil {
		t.Fatal("POST /v1/events is missing")
	}
	foundKey := false
	for _, parameter := range operation.Parameters {
		if parameter.Value != nil && parameter.Value.Name == "Idempotency-Key" && parameter.Value.Required {
			foundKey = true
		}
	}
	if !foundKey || operation.Responses.Value("200") == nil || operation.Responses.Value("409") == nil || !strings.Contains(SpecYAML(), "replayed") {
		t.Fatal("event ingest contract does not declare required idempotency replay and conflict behavior")
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

func TestSpecYAMLIncludesDerivedInsightAdminRoutesAndSchemas(t *testing.T) {
	for _, want := range []string{
		"/v1/admin/derived-insights",
		"/v1/admin/derived-insights/{insight_id}",
		"/v1/admin/derived-insights/{insight_id}:suppress",
		"/v1/admin/derived-insights/{insight_id}/feedback",
		"/v1/admin/derived-insight-feedback/{feedback_id}:supersede",
		"DerivedInsight",
		"DerivedInsightListResponse",
		"DerivedInsightDetail",
		"DerivedInsightFeedback",
		"DerivedInsightFeedbackCreateRequest",
		"DerivedInsightFeedbackListResponse",
		"DerivedInsightFeedbackSupersedeRequest",
		"DerivedInsightFeedbackSummary",
		"DerivedInsightEvidenceRef",
		"DerivedInsightLifecycleRecord",
		"DerivedInsightSuppressRequest",
		"quality_score",
		"needs_review",
		"superseded_at",
		"failure_pattern",
		"lesson",
		"min_confidence",
		"min_evidence_count",
		"include_hidden",
		"source_failure_pattern_id",
		"derivation_fingerprint",
	} {
		if !strings.Contains(SpecYAML(), want) {
			t.Fatalf("SpecYAML() missing %q", want)
		}
	}
}

func TestSpecYAMLIncludesDerivedInsightReplayRoutesAndSchemas(t *testing.T) {
	for _, want := range []string{
		"/v1/admin/derived-insight-replays:dry-run",
		"/v1/admin/derived-insight-replays",
		"/v1/admin/derived-insight-replays/{replay_run_id}",
		"/v1/admin/derived-insight-replays/{replay_run_id}/report",
		"DerivedInsightReplayRequest",
		"DerivedInsightReplayRun",
		"DerivedInsightReplayReport",
		"DerivedInsightReplayCounters",
		"DerivedInsightReplayDecision",
		"DerivedInsightReplayStatus",
		"DerivedInsightReplayMode",
		"DerivedInsightReplayDecisionKind",
		"DerivedInsightReplayReason",
		"evidence_window_start",
		"evidence_window_end",
		"evidence_limit",
		"idempotency_key",
		"continuation_required",
	} {
		if !strings.Contains(SpecYAML(), want) {
			t.Fatalf("SpecYAML() missing %q", want)
		}
	}
}

func TestSpecYAMLIncludesMemoryQualityRepairRoutesAndSchemas(t *testing.T) {
	for _, want := range []string{
		"/v1/admin/memory-quality/evaluations",
		"/v1/admin/memory-quality/evaluations/{evaluation_run_id}",
		"/v1/admin/memory-quality/evaluations/{evaluation_run_id}/findings",
		"/v1/admin/memory-quality/repair-plans",
		"/v1/admin/memory-quality/repair-plans/{repair_plan_id}",
		"/v1/admin/memory-quality/repair-plans/{repair_plan_id}:approve",
		"/v1/admin/memory-quality/repair-plans/{repair_plan_id}:verify",
		"AdmissionPressureReport",
		"QualityEvaluationRun",
		"QualityEvaluationFinding",
		"RepairPlan",
		"RepairAction",
		"accept_degraded",
		"semantic_projection_degraded",
	} {
		if !strings.Contains(SpecYAML(), want) {
			t.Fatalf("SpecYAML() missing %q", want)
		}
	}
}

func TestSpecYAMLIncludesScopeProofAndMemorySessionRoutesAndSchemas(t *testing.T) {
	for _, want := range []string{
		"/v1/admin/scope-proofs",
		"/v1/admin/scope-proofs/{proof_run_id}",
		"/v1/admin/scope-proofs/{proof_run_id}/report",
		"/v1/admin/scope-proofs/{proof_run_id}:rerun",
		"/v1/memory-sessions",
		"/v1/memory-sessions/{session_id}",
		"/v1/memory-sessions/{session_id}/report",
		"/v1/memory-sessions/{session_id}/turns",
		"/v1/memory-sessions/{session_id}/turns/{turn_id}:outcome",
		"/v1/memory-sessions/{session_id}:verify",
		"ScopeProofRun",
		"ScopeProofStep",
		"ScopeProofReport",
		"LoopReportEvidence",
		"MemorySessionRun",
		"MemorySessionTurn",
		"MemorySessionReport",
		"CreateScopeProofRequest",
		"CreateMemorySessionRequest",
		"CreateMemorySessionTurnRequest",
		"RecordMemorySessionOutcomeRequest",
		"RequestMemorySessionVerificationRequest",
		"failure_categories",
		"quality_evaluation_ids",
		"repair_plan_ids",
		"next_actions",
		"actor",
		"reason",
	} {
		if !strings.Contains(SpecYAML(), want) {
			t.Fatalf("SpecYAML() missing %q", want)
		}
	}
}

func TestSpecYAMLIncludesIntegrationWorkflowContract(t *testing.T) {
	for _, want := range []string{
		"/v1/workflows/runs",
		"/v1/workflows/runs/{workflow_run_id}",
		"/v1/workflows/runs/{workflow_run_id}/steps",
		"/v1/workflows/runs/{workflow_run_id}/next-actions",
		"/v1/admin/workflows/templates",
		"/v1/admin/workflows/templates/{workflow_template_id}",
		"/v1/admin/workflows/templates/{workflow_template_id}/disable",
		"/v1/admin/workflows/runs",
		"/v1/admin/workflows/runs/{workflow_run_id}",
		"/v1/admin/workflows/runs/{workflow_run_id}/steps",
		"/v1/admin/workflows/runs/{workflow_run_id}/evidence-links",
		"/v1/admin/workflows/runs/{workflow_run_id}/diagnostics",
		"/v1/admin/workflows/runs/{workflow_run_id}/next-actions",
		"/v1/admin/workflows/evidence-links/{evidence_link_id}/supersede",
		"WorkflowTemplate",
		"WorkflowRun",
		"WorkflowStepRecord",
		"WorkflowEvidenceLink",
		"WorkflowGapDiagnostic",
		"WorkflowNextAction",
		"WorkflowTemplateCreateRequest",
		"WorkflowRunCreateRequest",
		"WorkflowStepRecordRequest",
		"WorkflowEvidenceLinkSupersedeRequest",
		"session_started",
		"turn_outcome_recorded",
		"task_evaluation_recorded",
		"subject_missing",
		"insufficient_evidence",
		"PublicAPIKey",
		"AdminAPIKey",
		"TenantHeader",
		"ProjectHeader",
		"NamespaceHeader",
		"workflow_gap",
		"workflow_incomplete",
		"review_workflow",
	} {
		if !strings.Contains(SpecYAML(), want) {
			t.Fatalf("SpecYAML() missing workflow contract element %q", want)
		}
	}
}

func TestSpecYAMLWorkflowOperationsRequireScopedCorrectAuth(t *testing.T) {
	loader := openapi3.NewLoader()
	doc, err := loader.LoadFromData([]byte(SpecYAML()))
	if err != nil {
		t.Fatalf("LoadFromData() error = %v", err)
	}

	publicPaths := []string{
		"/v1/workflows/runs",
		"/v1/workflows/runs/{workflow_run_id}",
		"/v1/workflows/runs/{workflow_run_id}/steps",
		"/v1/workflows/runs/{workflow_run_id}/next-actions",
	}
	adminPaths := []string{
		"/v1/admin/workflows/templates",
		"/v1/admin/workflows/templates/{workflow_template_id}",
		"/v1/admin/workflows/templates/{workflow_template_id}/disable",
		"/v1/admin/workflows/runs",
		"/v1/admin/workflows/runs/{workflow_run_id}",
		"/v1/admin/workflows/runs/{workflow_run_id}/steps",
		"/v1/admin/workflows/runs/{workflow_run_id}/evidence-links",
		"/v1/admin/workflows/runs/{workflow_run_id}/diagnostics",
		"/v1/admin/workflows/runs/{workflow_run_id}/next-actions",
		"/v1/admin/workflows/evidence-links/{evidence_link_id}/supersede",
	}

	assertWorkflowPathAuth := func(path, authRef string) {
		t.Helper()
		item := doc.Paths.Value(path)
		if item == nil {
			t.Fatalf("OpenAPI path %q missing", path)
		}
		for method, operation := range item.Operations() {
			refs := map[string]bool{}
			for _, param := range operation.Parameters {
				refs[param.Ref] = true
			}
			for _, ref := range []string{authRef, "#/components/parameters/TenantHeader", "#/components/parameters/ProjectHeader", "#/components/parameters/NamespaceHeader"} {
				if !refs[ref] {
					t.Fatalf("%s %s missing parameter ref %s", method, path, ref)
				}
			}
		}
	}
	for _, path := range publicPaths {
		assertWorkflowPathAuth(path, "#/components/parameters/PublicAPIKey")
	}
	for _, path := range adminPaths {
		assertWorkflowPathAuth(path, "#/components/parameters/AdminAPIKey")
	}
}

func TestSpecYAMLIncludesUsefulnessFeedbackSessionAndDiagnosticsContracts(t *testing.T) {
	for _, want := range []string{
		"/v1/usefulness-feedback",
		"/v1/admin/usefulness-feedback",
		"/v1/admin/usefulness-feedback/summary",
		"/v1/admin/usefulness-feedback/{feedback_id}",
		"/v1/admin/usefulness-feedback/{feedback_id}:supersede",
		"UsefulnessFeedback",
		"UsefulnessFeedbackCreateRequest",
		"UsefulnessFeedbackSupersedeRequest",
		"UsefulnessFeedbackSummary",
		"UsefulnessFeedbackSubject",
		"ExpectedRecallTarget",
		"missing_expected",
		"unsafe_or_hidden",
		"opaque_token",
		"feedback_summaries",
		"quality_finding_ids",
		"quality_finding_codes",
		"verifications",
		"event_payloads",
		"include_feedback_diagnostics",
		"feedback_aware_ranking",
		"feedback_ranking_policy",
	} {
		if !strings.Contains(SpecYAML(), want) {
			t.Fatalf("SpecYAML() missing %q", want)
		}
	}
}

func TestSpecYAMLIncludesTaskEvaluationContracts(t *testing.T) {
	for _, want := range []string{
		"/v1/task-evaluations",
		"/v1/task-evaluations/{evaluation_id}/report",
		"/v1/admin/task-evaluations",
		"/v1/admin/task-evaluations/{evaluation_id}",
		"/v1/admin/task-evaluations/summary",
		"/v1/admin/task-evaluations/{evaluation_id}/supersede",
		"TaskEvaluation",
		"TaskEvaluationCreateRequest",
		"TaskEvaluationListResponse",
		"TaskEvaluationReport",
		"TaskEvaluationSupersedeRequest",
		"TaskEvaluationSummary",
		"TaskEvaluationVerdict",
		"TaskContributionCategory",
		"TaskEvidenceTargetKind",
		"linked_raw_event_ids",
		"linked_outcome_event_ids",
		"linked_expected_recall_ids",
		"linked_context_citation_ids",
		"memory_contribution_categories",
		"next_actions",
	} {
		if !strings.Contains(SpecYAML(), want) {
			t.Fatalf("SpecYAML() missing %q", want)
		}
	}
}

func TestSpecYAMLIncludesAssuranceAdminRoutesAndSchemas(t *testing.T) {
	spec := SpecYAML()
	for _, want := range []string{
		"/v1/admin/assurance/health-evaluations",
		"/v1/admin/assurance/health-evaluations/{health_evaluation_id}",
		"/v1/admin/assurance/incidents",
		"/v1/admin/assurance/incidents/{incident_id}",
		"/v1/admin/assurance/incidents/{incident_id}/{incident_action}",
		"/v1/admin/assurance/alert-candidates",
		"/v1/admin/assurance/alert-candidates/{alert_candidate_id}",
		"/v1/admin/assurance/alert-candidates/{alert_candidate_id}/delivery-attempts",
		"/v1/admin/assurance/conformance-profiles",
		"/v1/admin/assurance/conformance-profiles/{conformance_profile_id}",
		"/v1/admin/assurance/conformance-profiles/{conformance_profile_id}/disable",
		"/v1/admin/assurance/conformance-runs",
		"/v1/admin/assurance/conformance-runs/{conformance_run_id}",
		"/v1/admin/assurance/readiness-reports",
		"/v1/admin/assurance/readiness-reports/{readiness_report_id}",
		"/v1/admin/assurance/recovery-verifications",
		"/v1/admin/assurance/recovery-verifications/{recovery_verification_id}",
		"HealthEvaluation",
		"HealthComponentSummary",
		"Incident",
		"IncidentActionRequest",
		"AlertCandidate",
		"AlertDeliveryAttempt",
		"ConformanceProfile",
		"ConformanceRun",
		"MissingEvidenceDiagnostic",
		"ReadinessReport",
		"RecoveryVerification",
		"ExpectedEvidenceKind",
		"RecoveryVerificationTarget",
		"redacted delivery target",
	} {
		if !strings.Contains(spec, want) {
			t.Fatalf("SpecYAML() missing assurance contract %q", want)
		}
	}
}

func TestSpecYAMLAssuranceAdminOperationsRequireAdminAuthAndScope(t *testing.T) {
	loader := openapi3.NewLoader()
	doc, err := loader.LoadFromData([]byte(SpecYAML()))
	if err != nil {
		t.Fatalf("LoadFromData() error = %v", err)
	}

	for _, path := range []string{
		"/v1/admin/assurance/health-evaluations",
		"/v1/admin/assurance/health-evaluations/{health_evaluation_id}",
		"/v1/admin/assurance/incidents",
		"/v1/admin/assurance/incidents/{incident_id}",
		"/v1/admin/assurance/incidents/{incident_id}/{incident_action}",
		"/v1/admin/assurance/alert-candidates",
		"/v1/admin/assurance/alert-candidates/{alert_candidate_id}",
		"/v1/admin/assurance/alert-candidates/{alert_candidate_id}/delivery-attempts",
		"/v1/admin/assurance/conformance-profiles",
		"/v1/admin/assurance/conformance-profiles/{conformance_profile_id}",
		"/v1/admin/assurance/conformance-profiles/{conformance_profile_id}/disable",
		"/v1/admin/assurance/conformance-runs",
		"/v1/admin/assurance/conformance-runs/{conformance_run_id}",
		"/v1/admin/assurance/readiness-reports",
		"/v1/admin/assurance/readiness-reports/{readiness_report_id}",
		"/v1/admin/assurance/recovery-verifications",
		"/v1/admin/assurance/recovery-verifications/{recovery_verification_id}",
	} {
		item := doc.Paths.Value(path)
		if item == nil {
			t.Fatalf("OpenAPI path %q missing", path)
		}
		for method, operation := range item.Operations() {
			refs := map[string]bool{}
			for _, param := range operation.Parameters {
				refs[param.Ref] = true
			}
			for _, ref := range []string{
				"#/components/parameters/AdminAPIKey",
				"#/components/parameters/TenantHeader",
				"#/components/parameters/ProjectHeader",
				"#/components/parameters/NamespaceHeader",
			} {
				if !refs[ref] {
					t.Fatalf("%s %s missing parameter ref %s", method, path, ref)
				}
			}
			if refs["#/components/parameters/PublicAPIKey"] {
				t.Fatalf("%s %s uses public API key", method, path)
			}
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
		"/v1/admin/embedding/cutovers/{cutover_plan_id}:preflight",
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
		"EmbeddingCutoverPreflightReport",
		"DiagnosticFinding",
		"eligible_total",
		"class_breakdown",
		"conflicting_plan",
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

func TestSpecYAMLIncludesContextExperienceInsightSections(t *testing.T) {
	for _, want := range []string{
		"include_experience_insights",
		"known_failures",
		"experience_lessons",
		"ExperienceInsightContext",
		"InsightCitation",
	} {
		if !strings.Contains(SpecYAML(), want) {
			t.Fatalf("SpecYAML() missing %q", want)
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
