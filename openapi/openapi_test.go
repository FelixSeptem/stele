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
