package postgres

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	pgxmock "github.com/pashagolub/pgxmock/v4"
)

func TestBootstrapDatabaseRunsBaseMigration(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatalf("pgxmock.NewPool() error = %v", err)
	}
	defer mock.Close()

	mock.ExpectBegin()
	mock.ExpectExec("CREATE EXTENSION IF NOT EXISTS pgcrypto").WillReturnResult(pgxmock.NewResult("CREATE EXTENSION", 0))
	mock.ExpectExec("CREATE EXTENSION IF NOT EXISTS vector").WillReturnResult(pgxmock.NewResult("CREATE EXTENSION", 0))
	mock.ExpectExec("CREATE TABLE IF NOT EXISTS raw_events").WillReturnResult(pgxmock.NewResult("CREATE TABLE", 0))
	mock.ExpectExec("ALTER TABLE raw_events").WillReturnResult(pgxmock.NewResult("ALTER TABLE", 0))
	mock.ExpectExec("CREATE TABLE IF NOT EXISTS candidate_memories").WillReturnResult(pgxmock.NewResult("CREATE TABLE", 0))
	mock.ExpectExec("CREATE TABLE IF NOT EXISTS canonical_memories").WillReturnResult(pgxmock.NewResult("CREATE TABLE", 0))
	mock.ExpectExec("ALTER TABLE canonical_memories").WillReturnResult(pgxmock.NewResult("ALTER TABLE", 0))
	mock.ExpectExec("CREATE TABLE IF NOT EXISTS memory_versions").WillReturnResult(pgxmock.NewResult("CREATE TABLE", 0))
	mock.ExpectExec("CREATE TABLE IF NOT EXISTS embedding_rebuilds").WillReturnResult(pgxmock.NewResult("CREATE TABLE", 0))
	mock.ExpectExec("CREATE TABLE IF NOT EXISTS vector_revisions").WillReturnResult(pgxmock.NewResult("CREATE TABLE", 0))
	mock.ExpectExec("CREATE TABLE IF NOT EXISTS provenance_links").WillReturnResult(pgxmock.NewResult("CREATE TABLE", 0))
	mock.ExpectExec("CREATE TABLE IF NOT EXISTS governance_recovery_ledger").WillReturnResult(pgxmock.NewResult("CREATE TABLE", 0))
	mock.ExpectExec("CREATE TABLE IF NOT EXISTS embedding_cutover_plans").WillReturnResult(pgxmock.NewResult("CREATE TABLE", 0))
	mock.ExpectExec("CREATE TABLE IF NOT EXISTS embedding_cutover_items").WillReturnResult(pgxmock.NewResult("CREATE TABLE", 0))
	mock.ExpectExec("CREATE TABLE IF NOT EXISTS embedding_recovery_ledger").WillReturnResult(pgxmock.NewResult("CREATE TABLE", 0))
	mock.ExpectExec("CREATE TABLE IF NOT EXISTS deletion_markers").WillReturnResult(pgxmock.NewResult("CREATE TABLE", 0))
	mock.ExpectExec("CREATE TABLE IF NOT EXISTS relation_projections").WillReturnResult(pgxmock.NewResult("CREATE TABLE", 0))
	mock.ExpectExec("CREATE TABLE IF NOT EXISTS job_executions").WillReturnResult(pgxmock.NewResult("CREATE TABLE", 0))
	mock.ExpectExec("CREATE TABLE IF NOT EXISTS access_principals").WillReturnResult(pgxmock.NewResult("CREATE TABLE", 0))
	mock.ExpectExec("CREATE TABLE IF NOT EXISTS access_credentials").WillReturnResult(pgxmock.NewResult("CREATE TABLE", 0))
	mock.ExpectExec("CREATE TABLE IF NOT EXISTS access_scope_grants").WillReturnResult(pgxmock.NewResult("CREATE TABLE", 0))
	mock.ExpectExec("CREATE TABLE IF NOT EXISTS access_audit_records").WillReturnResult(pgxmock.NewResult("CREATE TABLE", 0))
	mock.ExpectExec("CREATE TABLE IF NOT EXISTS event_idempotency_records").WillReturnResult(pgxmock.NewResult("CREATE TABLE", 0))
	mock.ExpectExec("CREATE TABLE IF NOT EXISTS derived_insights").WillReturnResult(pgxmock.NewResult("CREATE TABLE", 0))
	mock.ExpectExec("CREATE TABLE IF NOT EXISTS derived_insight_evidence").WillReturnResult(pgxmock.NewResult("CREATE TABLE", 0))
	mock.ExpectExec("CREATE TABLE IF NOT EXISTS derived_insight_lifecycle_ledger").WillReturnResult(pgxmock.NewResult("CREATE TABLE", 0))
	mock.ExpectExec("CREATE TABLE IF NOT EXISTS derived_insight_feedback").WillReturnResult(pgxmock.NewResult("CREATE TABLE", 0))
	mock.ExpectExec("CREATE TABLE IF NOT EXISTS derived_insight_replay_runs").WillReturnResult(pgxmock.NewResult("CREATE TABLE", 0))
	mock.ExpectExec("CREATE TABLE IF NOT EXISTS quality_evaluation_runs").WillReturnResult(pgxmock.NewResult("CREATE TABLE", 0))
	mock.ExpectExec("CREATE TABLE IF NOT EXISTS quality_evaluation_findings").WillReturnResult(pgxmock.NewResult("CREATE TABLE", 0))
	mock.ExpectExec("CREATE TABLE IF NOT EXISTS repair_plans").WillReturnResult(pgxmock.NewResult("CREATE TABLE", 0))
	mock.ExpectExec("CREATE TABLE IF NOT EXISTS repair_actions").WillReturnResult(pgxmock.NewResult("CREATE TABLE", 0))
	mock.ExpectExec("CREATE TABLE IF NOT EXISTS admission_pressure_audit").WillReturnResult(pgxmock.NewResult("CREATE TABLE", 0))
	mock.ExpectExec("CREATE TABLE IF NOT EXISTS scope_proof_runs").WillReturnResult(pgxmock.NewResult("CREATE TABLE", 0))
	mock.ExpectExec("CREATE TABLE IF NOT EXISTS scope_proof_steps").WillReturnResult(pgxmock.NewResult("CREATE TABLE", 0))
	mock.ExpectExec("CREATE TABLE IF NOT EXISTS memory_session_runs").WillReturnResult(pgxmock.NewResult("CREATE TABLE", 0))
	mock.ExpectExec("CREATE TABLE IF NOT EXISTS memory_session_turns").WillReturnResult(pgxmock.NewResult("CREATE TABLE", 0))
	mock.ExpectExec("CREATE TABLE IF NOT EXISTS memory_session_verifications").WillReturnResult(pgxmock.NewResult("CREATE TABLE", 0))
	mock.ExpectExec("CREATE TABLE IF NOT EXISTS memory_loop_evidence_links").WillReturnResult(pgxmock.NewResult("CREATE TABLE", 0))
	mock.ExpectExec("CREATE TABLE IF NOT EXISTS usefulness_feedback").WillReturnResult(pgxmock.NewResult("CREATE TABLE", 0))
	mock.ExpectExec("CREATE TABLE IF NOT EXISTS usefulness_feedback_subjects").WillReturnResult(pgxmock.NewResult("CREATE TABLE", 0))
	mock.ExpectExec("CREATE TABLE IF NOT EXISTS usefulness_feedback_supersessions").WillReturnResult(pgxmock.NewResult("CREATE TABLE", 0))
	mock.ExpectExec("CREATE TABLE IF NOT EXISTS usefulness_feedback_summaries").WillReturnResult(pgxmock.NewResult("CREATE TABLE", 0))
	mock.ExpectExec("CREATE TABLE IF NOT EXISTS task_evaluations").WillReturnResult(pgxmock.NewResult("CREATE TABLE", 0))
	mock.ExpectExec("CREATE TABLE IF NOT EXISTS task_evidence_links").WillReturnResult(pgxmock.NewResult("CREATE TABLE", 0))
	mock.ExpectExec("CREATE TABLE IF NOT EXISTS task_evaluation_supersessions").WillReturnResult(pgxmock.NewResult("CREATE TABLE", 0))
	mock.ExpectExec("CREATE TABLE IF NOT EXISTS task_evaluation_summaries").WillReturnResult(pgxmock.NewResult("CREATE TABLE", 0))
	mock.ExpectExec("CREATE TABLE IF NOT EXISTS ranking_rollout_policies").WillReturnResult(pgxmock.NewResult("CREATE TABLE", 0))
	mock.ExpectExec("CREATE TABLE IF NOT EXISTS ranking_rollout_policy_states").WillReturnResult(pgxmock.NewResult("CREATE TABLE", 0))
	mock.ExpectExec("CREATE TABLE IF NOT EXISTS ranking_rollout_dry_runs").WillReturnResult(pgxmock.NewResult("CREATE TABLE", 0))
	mock.ExpectExec("CREATE TABLE IF NOT EXISTS ranking_rollout_impact_entries").WillReturnResult(pgxmock.NewResult("CREATE TABLE", 0))
	mock.ExpectExec("CREATE TABLE IF NOT EXISTS ranking_rollout_rollback_audit").WillReturnResult(pgxmock.NewResult("CREATE TABLE", 0))
	mock.ExpectExec("CREATE TABLE IF NOT EXISTS assurance_health_evaluations").WillReturnResult(pgxmock.NewResult("CREATE TABLE", 0))
	mock.ExpectExec("CREATE TABLE IF NOT EXISTS assurance_health_components").WillReturnResult(pgxmock.NewResult("CREATE TABLE", 0))
	mock.ExpectExec("CREATE TABLE IF NOT EXISTS assurance_incidents").WillReturnResult(pgxmock.NewResult("CREATE TABLE", 0))
	mock.ExpectExec("CREATE TABLE IF NOT EXISTS assurance_incident_transitions").WillReturnResult(pgxmock.NewResult("CREATE TABLE", 0))
	mock.ExpectExec("CREATE TABLE IF NOT EXISTS assurance_alert_candidates").WillReturnResult(pgxmock.NewResult("CREATE TABLE", 0))
	mock.ExpectExec("CREATE TABLE IF NOT EXISTS assurance_alert_delivery_attempts").WillReturnResult(pgxmock.NewResult("CREATE TABLE", 0))
	mock.ExpectExec("CREATE TABLE IF NOT EXISTS assurance_conformance_profiles").WillReturnResult(pgxmock.NewResult("CREATE TABLE", 0))
	mock.ExpectExec("CREATE TABLE IF NOT EXISTS assurance_conformance_runs").WillReturnResult(pgxmock.NewResult("CREATE TABLE", 0))
	mock.ExpectExec("CREATE TABLE IF NOT EXISTS assurance_missing_evidence_diagnostics").WillReturnResult(pgxmock.NewResult("CREATE TABLE", 0))
	mock.ExpectExec("CREATE TABLE IF NOT EXISTS assurance_operational_proofs").WillReturnResult(pgxmock.NewResult("CREATE TABLE", 0))
	mock.ExpectExec("CREATE TABLE IF NOT EXISTS assurance_readiness_reports").WillReturnResult(pgxmock.NewResult("CREATE TABLE", 0))
	mock.ExpectExec("CREATE TABLE IF NOT EXISTS assurance_recovery_verifications").WillReturnResult(pgxmock.NewResult("CREATE TABLE", 0))
	mock.ExpectExec("CREATE TABLE IF NOT EXISTS assurance_retention_runs").WillReturnResult(pgxmock.NewResult("CREATE TABLE", 0))
	mock.ExpectExec("CREATE TABLE IF NOT EXISTS integration_workflow_templates").WillReturnResult(pgxmock.NewResult("CREATE TABLE", 0))
	mock.ExpectExec("CREATE TABLE IF NOT EXISTS integration_workflow_template_steps").WillReturnResult(pgxmock.NewResult("CREATE TABLE", 0))
	mock.ExpectExec("CREATE TABLE IF NOT EXISTS integration_workflow_runs").WillReturnResult(pgxmock.NewResult("CREATE TABLE", 0))
	mock.ExpectExec("CREATE TABLE IF NOT EXISTS integration_workflow_step_records").WillReturnResult(pgxmock.NewResult("CREATE TABLE", 0))
	mock.ExpectExec("CREATE TABLE IF NOT EXISTS integration_workflow_evidence_links").WillReturnResult(pgxmock.NewResult("CREATE TABLE", 0))
	mock.ExpectExec("CREATE TABLE IF NOT EXISTS integration_workflow_gap_diagnostics").WillReturnResult(pgxmock.NewResult("CREATE TABLE", 0))
	mock.ExpectExec("CREATE TABLE IF NOT EXISTS integration_workflow_next_actions").WillReturnResult(pgxmock.NewResult("CREATE TABLE", 0))
	mock.ExpectExec("CREATE TABLE IF NOT EXISTS integration_workflow_transitions").WillReturnResult(pgxmock.NewResult("CREATE TABLE", 0))
	mock.ExpectExec("CREATE TABLE IF NOT EXISTS integration_workflow_retention_runs").WillReturnResult(pgxmock.NewResult("CREATE TABLE", 0))
	mock.ExpectExec("ALTER TABLE integration_workflow_transitions").WillReturnResult(pgxmock.NewResult("ALTER TABLE", 0))
	mock.ExpectExec("CREATE INDEX IF NOT EXISTS raw_events_scope_created_at_idx").WillReturnResult(pgxmock.NewResult("CREATE INDEX", 0))
	mock.ExpectExec("CREATE INDEX IF NOT EXISTS raw_events_session_outcome_idx").WillReturnResult(pgxmock.NewResult("CREATE INDEX", 0))
	mock.ExpectExec("CREATE INDEX IF NOT EXISTS raw_events_governance_claim_idx").WillReturnResult(pgxmock.NewResult("CREATE INDEX", 0))
	mock.ExpectExec("CREATE INDEX IF NOT EXISTS candidate_memories_source_raw_event_idx").WillReturnResult(pgxmock.NewResult("CREATE INDEX", 0))
	mock.ExpectExec("CREATE INDEX IF NOT EXISTS candidate_memories_scope_status_idx").WillReturnResult(pgxmock.NewResult("CREATE INDEX", 0))
	mock.ExpectExec("CREATE INDEX IF NOT EXISTS canonical_memories_scope_state_idx").WillReturnResult(pgxmock.NewResult("CREATE INDEX", 0))
	mock.ExpectExec("CREATE INDEX IF NOT EXISTS canonical_memories_updated_at_idx").WillReturnResult(pgxmock.NewResult("CREATE INDEX", 0))
	mock.ExpectExec("CREATE INDEX IF NOT EXISTS canonical_memories_search_text_idx").WillReturnResult(pgxmock.NewResult("CREATE INDEX", 0))
	mock.ExpectExec("CREATE INDEX IF NOT EXISTS memory_versions_memory_created_at_idx").WillReturnResult(pgxmock.NewResult("CREATE INDEX", 0))
	mock.ExpectExec("CREATE INDEX IF NOT EXISTS embedding_rebuilds_status_requested_at_idx").WillReturnResult(pgxmock.NewResult("CREATE INDEX", 0))
	mock.ExpectExec("CREATE INDEX IF NOT EXISTS embedding_rebuilds_scope_status_idx").WillReturnResult(pgxmock.NewResult("CREATE INDEX", 0))
	mock.ExpectExec("CREATE INDEX IF NOT EXISTS provenance_links_scope_created_at_idx").WillReturnResult(pgxmock.NewResult("CREATE INDEX", 0))
	mock.ExpectExec("CREATE INDEX IF NOT EXISTS provenance_links_candidate_memory_idx").WillReturnResult(pgxmock.NewResult("CREATE INDEX", 0))
	mock.ExpectExec("CREATE INDEX IF NOT EXISTS provenance_links_raw_event_idx").WillReturnResult(pgxmock.NewResult("CREATE INDEX", 0))
	mock.ExpectExec("CREATE INDEX IF NOT EXISTS provenance_links_memory_idx").WillReturnResult(pgxmock.NewResult("CREATE INDEX", 0))
	mock.ExpectExec("CREATE INDEX IF NOT EXISTS vector_revisions_memory_generated_at_idx").WillReturnResult(pgxmock.NewResult("CREATE INDEX", 0))
	mock.ExpectExec("CREATE INDEX IF NOT EXISTS vector_revisions_scope_status_generated_at_idx").WillReturnResult(pgxmock.NewResult("CREATE INDEX", 0))
	mock.ExpectExec("CREATE INDEX IF NOT EXISTS governance_recovery_ledger_scope_created_at_idx").WillReturnResult(pgxmock.NewResult("CREATE INDEX", 0))
	mock.ExpectExec("CREATE INDEX IF NOT EXISTS governance_recovery_ledger_raw_event_created_at_idx").WillReturnResult(pgxmock.NewResult("CREATE INDEX", 0))
	mock.ExpectExec("CREATE INDEX IF NOT EXISTS embedding_recovery_ledger_scope_created_at_idx").WillReturnResult(pgxmock.NewResult("CREATE INDEX", 0))
	mock.ExpectExec("CREATE INDEX IF NOT EXISTS embedding_recovery_ledger_memory_created_at_idx").WillReturnResult(pgxmock.NewResult("CREATE INDEX", 0))
	mock.ExpectExec("CREATE INDEX IF NOT EXISTS embedding_recovery_ledger_cutover_plan_created_at_idx").WillReturnResult(pgxmock.NewResult("CREATE INDEX", 0))
	mock.ExpectExec("CREATE INDEX IF NOT EXISTS embedding_cutover_plans_scope_status_created_at_idx").WillReturnResult(pgxmock.NewResult("CREATE INDEX", 0))
	mock.ExpectExec("CREATE INDEX IF NOT EXISTS embedding_cutover_items_plan_status_updated_at_idx").WillReturnResult(pgxmock.NewResult("CREATE INDEX", 0))
	mock.ExpectExec("CREATE INDEX IF NOT EXISTS embedding_cutover_items_scope_status_updated_at_idx").WillReturnResult(pgxmock.NewResult("CREATE INDEX", 0))
	mock.ExpectExec("CREATE INDEX IF NOT EXISTS relation_projections_scope_updated_at_idx").WillReturnResult(pgxmock.NewResult("CREATE INDEX", 0))
	mock.ExpectExec("CREATE INDEX IF NOT EXISTS relation_projections_search_text_idx").WillReturnResult(pgxmock.NewResult("CREATE INDEX", 0))
	mock.ExpectExec("CREATE INDEX IF NOT EXISTS job_executions_scope_started_at_idx").WillReturnResult(pgxmock.NewResult("CREATE INDEX", 0))
	mock.ExpectExec("CREATE INDEX IF NOT EXISTS job_executions_job_name_started_at_idx").WillReturnResult(pgxmock.NewResult("CREATE INDEX", 0))
	mock.ExpectExec("CREATE UNIQUE INDEX IF NOT EXISTS access_credentials_lookup_id_idx").WillReturnResult(pgxmock.NewResult("CREATE INDEX", 0))
	mock.ExpectExec("CREATE INDEX IF NOT EXISTS access_scope_grants_principal_scope_idx").WillReturnResult(pgxmock.NewResult("CREATE INDEX", 0))
	mock.ExpectExec("CREATE INDEX IF NOT EXISTS access_audit_records_principal_created_at_idx").WillReturnResult(pgxmock.NewResult("CREATE INDEX", 0))
	mock.ExpectExec("CREATE UNIQUE INDEX IF NOT EXISTS event_idempotency_records_principal_scope_key_idx").WillReturnResult(pgxmock.NewResult("CREATE INDEX", 0))
	mock.ExpectExec("CREATE UNIQUE INDEX IF NOT EXISTS derived_insights_scope_type_fingerprint_idx").WillReturnResult(pgxmock.NewResult("CREATE INDEX", 0))
	mock.ExpectExec("CREATE INDEX IF NOT EXISTS derived_insights_scope_state_type_idx").WillReturnResult(pgxmock.NewResult("CREATE INDEX", 0))
	mock.ExpectExec("CREATE INDEX IF NOT EXISTS derived_insights_scope_confidence_idx").WillReturnResult(pgxmock.NewResult("CREATE INDEX", 0))
	mock.ExpectExec("CREATE INDEX IF NOT EXISTS derived_insight_evidence_insight_idx").WillReturnResult(pgxmock.NewResult("CREATE INDEX", 0))
	mock.ExpectExec("CREATE INDEX IF NOT EXISTS derived_insight_lifecycle_ledger_insight_idx").WillReturnResult(pgxmock.NewResult("CREATE INDEX", 0))
	mock.ExpectExec("CREATE INDEX IF NOT EXISTS derived_insight_feedback_insight_created_at_idx").WillReturnResult(pgxmock.NewResult("CREATE INDEX", 0))
	mock.ExpectExec("CREATE INDEX IF NOT EXISTS derived_insight_feedback_scope_type_active_idx").WillReturnResult(pgxmock.NewResult("CREATE INDEX", 0))
	mock.ExpectExec("CREATE INDEX IF NOT EXISTS derived_insight_replay_runs_scope_status_updated_at_idx").WillReturnResult(pgxmock.NewResult("CREATE INDEX", 0))
	mock.ExpectExec("CREATE UNIQUE INDEX IF NOT EXISTS derived_insight_replay_runs_scope_idempotency_idx").WillReturnResult(pgxmock.NewResult("CREATE INDEX", 0))
	mock.ExpectExec("CREATE INDEX IF NOT EXISTS quality_evaluation_runs_scope_updated_at_idx").WillReturnResult(pgxmock.NewResult("CREATE INDEX", 0))
	mock.ExpectExec("CREATE INDEX IF NOT EXISTS quality_evaluation_findings_run_created_at_idx").WillReturnResult(pgxmock.NewResult("CREATE INDEX", 0))
	mock.ExpectExec("CREATE INDEX IF NOT EXISTS repair_plans_scope_updated_at_idx").WillReturnResult(pgxmock.NewResult("CREATE INDEX", 0))
	mock.ExpectExec("CREATE INDEX IF NOT EXISTS repair_actions_scope_status_next_attempt_idx").WillReturnResult(pgxmock.NewResult("CREATE INDEX", 0))
	mock.ExpectExec("CREATE INDEX IF NOT EXISTS admission_pressure_audit_scope_observed_at_idx").WillReturnResult(pgxmock.NewResult("CREATE INDEX", 0))
	mock.ExpectExec("CREATE INDEX IF NOT EXISTS scope_proof_runs_scope_updated_at_idx").WillReturnResult(pgxmock.NewResult("CREATE INDEX", 0))
	mock.ExpectExec("CREATE INDEX IF NOT EXISTS scope_proof_runs_scope_status_updated_at_idx").WillReturnResult(pgxmock.NewResult("CREATE INDEX", 0))
	mock.ExpectExec("CREATE INDEX IF NOT EXISTS scope_proof_steps_scope_status_next_attempt_idx").WillReturnResult(pgxmock.NewResult("CREATE INDEX", 0))
	mock.ExpectExec("CREATE INDEX IF NOT EXISTS scope_proof_steps_proof_created_at_idx").WillReturnResult(pgxmock.NewResult("CREATE INDEX", 0))
	mock.ExpectExec("CREATE INDEX IF NOT EXISTS memory_session_runs_scope_updated_at_idx").WillReturnResult(pgxmock.NewResult("CREATE INDEX", 0))
	mock.ExpectExec("CREATE INDEX IF NOT EXISTS memory_session_runs_scope_status_updated_at_idx").WillReturnResult(pgxmock.NewResult("CREATE INDEX", 0))
	mock.ExpectExec("CREATE INDEX IF NOT EXISTS memory_session_turns_session_created_at_idx").WillReturnResult(pgxmock.NewResult("CREATE INDEX", 0))
	mock.ExpectExec("CREATE UNIQUE INDEX IF NOT EXISTS memory_session_turns_scope_idempotency_idx").WillReturnResult(pgxmock.NewResult("CREATE INDEX", 0))
	mock.ExpectExec("CREATE UNIQUE INDEX IF NOT EXISTS memory_session_turns_scope_outcome_idempotency_idx").WillReturnResult(pgxmock.NewResult("CREATE INDEX", 0))
	mock.ExpectExec("CREATE INDEX IF NOT EXISTS memory_session_verifications_scope_status_next_attempt_idx").WillReturnResult(pgxmock.NewResult("CREATE INDEX", 0))
	mock.ExpectExec("CREATE INDEX IF NOT EXISTS memory_loop_evidence_links_owner_idx").WillReturnResult(pgxmock.NewResult("CREATE INDEX", 0))
	mock.ExpectExec("CREATE UNIQUE INDEX IF NOT EXISTS usefulness_feedback_scope_idempotency_idx").WillReturnResult(pgxmock.NewResult("CREATE INDEX", 0))
	mock.ExpectExec("CREATE INDEX IF NOT EXISTS usefulness_feedback_scope_type_created_at_idx").WillReturnResult(pgxmock.NewResult("CREATE INDEX", 0))
	mock.ExpectExec("CREATE INDEX IF NOT EXISTS usefulness_feedback_subjects_subject_idx").WillReturnResult(pgxmock.NewResult("CREATE INDEX", 0))
	mock.ExpectExec("CREATE INDEX IF NOT EXISTS usefulness_feedback_supersessions_feedback_idx").WillReturnResult(pgxmock.NewResult("CREATE INDEX", 0))
	mock.ExpectExec("CREATE INDEX IF NOT EXISTS usefulness_feedback_active_subject_idx").WillReturnResult(pgxmock.NewResult("CREATE INDEX", 0))
	mock.ExpectExec("CREATE INDEX IF NOT EXISTS task_evaluations_scope_created_at_idx").WillReturnResult(pgxmock.NewResult("CREATE INDEX", 0))
	mock.ExpectExec("CREATE UNIQUE INDEX IF NOT EXISTS task_evaluations_scope_idempotency_idx").WillReturnResult(pgxmock.NewResult("CREATE INDEX", 0))
	mock.ExpectExec("CREATE INDEX IF NOT EXISTS task_evaluations_scope_verdict_created_at_idx").WillReturnResult(pgxmock.NewResult("CREATE INDEX", 0))
	mock.ExpectExec("CREATE INDEX IF NOT EXISTS task_evidence_links_task_idx").WillReturnResult(pgxmock.NewResult("CREATE INDEX", 0))
	mock.ExpectExec("CREATE INDEX IF NOT EXISTS task_evidence_links_scope_kind_idx").WillReturnResult(pgxmock.NewResult("CREATE INDEX", 0))
	mock.ExpectExec("CREATE INDEX IF NOT EXISTS task_evaluation_supersessions_scope_created_at_idx").WillReturnResult(pgxmock.NewResult("CREATE INDEX", 0))
	mock.ExpectExec("CREATE INDEX IF NOT EXISTS task_evaluation_summaries_scope_subject_idx").WillReturnResult(pgxmock.NewResult("CREATE INDEX", 0))
	mock.ExpectExec("CREATE INDEX IF NOT EXISTS ranking_rollout_policies_scope_status_created_at_idx").WillReturnResult(pgxmock.NewResult("CREATE INDEX", 0))
	mock.ExpectExec("CREATE INDEX IF NOT EXISTS ranking_rollout_policy_states_scope_status_updated_at_idx").WillReturnResult(pgxmock.NewResult("CREATE INDEX", 0))
	mock.ExpectExec("CREATE INDEX IF NOT EXISTS ranking_rollout_dry_runs_scope_created_at_idx").WillReturnResult(pgxmock.NewResult("CREATE INDEX", 0))
	mock.ExpectExec("CREATE INDEX IF NOT EXISTS ranking_rollout_impact_entries_scope_created_at_idx").WillReturnResult(pgxmock.NewResult("CREATE INDEX", 0))
	mock.ExpectExec("CREATE INDEX IF NOT EXISTS ranking_rollout_rollback_audit_scope_created_at_idx").WillReturnResult(pgxmock.NewResult("CREATE INDEX", 0))
	mock.ExpectExec("CREATE INDEX IF NOT EXISTS assurance_health_evaluations_scope_created_at_idx").WillReturnResult(pgxmock.NewResult("CREATE INDEX", 0))
	mock.ExpectExec("CREATE INDEX IF NOT EXISTS assurance_health_components_scope_component_idx").WillReturnResult(pgxmock.NewResult("CREATE INDEX", 0))
	mock.ExpectExec("CREATE INDEX IF NOT EXISTS assurance_incidents_scope_status_updated_at_idx").WillReturnResult(pgxmock.NewResult("CREATE INDEX", 0))
	mock.ExpectExec("CREATE UNIQUE INDEX IF NOT EXISTS assurance_incidents_scope_dedup_idx").WillReturnResult(pgxmock.NewResult("CREATE INDEX", 0))
	mock.ExpectExec("CREATE INDEX IF NOT EXISTS assurance_incident_transitions_incident_idx").WillReturnResult(pgxmock.NewResult("CREATE INDEX", 0))
	mock.ExpectExec("CREATE INDEX IF NOT EXISTS assurance_alert_candidates_scope_next_attempt_idx").WillReturnResult(pgxmock.NewResult("CREATE INDEX", 0))
	mock.ExpectExec("CREATE INDEX IF NOT EXISTS assurance_alert_delivery_attempts_scope_next_attempt_idx").WillReturnResult(pgxmock.NewResult("CREATE INDEX", 0))
	mock.ExpectExec("CREATE INDEX IF NOT EXISTS assurance_conformance_profiles_scope_status_idx").WillReturnResult(pgxmock.NewResult("CREATE INDEX", 0))
	mock.ExpectExec("CREATE INDEX IF NOT EXISTS assurance_conformance_runs_scope_created_at_idx").WillReturnResult(pgxmock.NewResult("CREATE INDEX", 0))
	mock.ExpectExec("CREATE INDEX IF NOT EXISTS assurance_missing_evidence_diagnostics_run_idx").WillReturnResult(pgxmock.NewResult("CREATE INDEX", 0))
	mock.ExpectExec("CREATE INDEX IF NOT EXISTS assurance_operational_proofs_scope_target_observed_idx").WillReturnResult(pgxmock.NewResult("CREATE INDEX", 0))
	mock.ExpectExec("CREATE INDEX IF NOT EXISTS assurance_readiness_reports_scope_generated_at_idx").WillReturnResult(pgxmock.NewResult("CREATE INDEX", 0))
	mock.ExpectExec("CREATE INDEX IF NOT EXISTS assurance_recovery_verifications_scope_target_idx").WillReturnResult(pgxmock.NewResult("CREATE INDEX", 0))
	mock.ExpectExec("CREATE INDEX IF NOT EXISTS assurance_retention_runs_scope_category_started_idx").WillReturnResult(pgxmock.NewResult("CREATE INDEX", 0))
	mock.ExpectExec("CREATE INDEX IF NOT EXISTS integration_workflow_templates_scope_status_updated_idx").WillReturnResult(pgxmock.NewResult("CREATE INDEX", 0))
	mock.ExpectExec("CREATE INDEX IF NOT EXISTS integration_workflow_template_steps_template_position_idx").WillReturnResult(pgxmock.NewResult("CREATE INDEX", 0))
	mock.ExpectExec("CREATE INDEX IF NOT EXISTS integration_workflow_runs_scope_status_updated_idx").WillReturnResult(pgxmock.NewResult("CREATE INDEX", 0))
	mock.ExpectExec("CREATE UNIQUE INDEX IF NOT EXISTS integration_workflow_runs_scope_idempotency_idx").WillReturnResult(pgxmock.NewResult("CREATE INDEX", 0))
	mock.ExpectExec("CREATE INDEX IF NOT EXISTS integration_workflow_step_records_run_created_idx").WillReturnResult(pgxmock.NewResult("CREATE INDEX", 0))
	mock.ExpectExec("CREATE INDEX IF NOT EXISTS integration_workflow_evidence_links_run_kind_status_idx").WillReturnResult(pgxmock.NewResult("CREATE INDEX", 0))
	mock.ExpectExec("CREATE INDEX IF NOT EXISTS integration_workflow_gap_diagnostics_run_status_created_idx").WillReturnResult(pgxmock.NewResult("CREATE INDEX", 0))
	mock.ExpectExec("CREATE INDEX IF NOT EXISTS integration_workflow_next_actions_run_status_created_idx").WillReturnResult(pgxmock.NewResult("CREATE INDEX", 0))
	mock.ExpectExec("CREATE INDEX IF NOT EXISTS integration_workflow_transitions_run_occurred_idx").WillReturnResult(pgxmock.NewResult("CREATE INDEX", 0))
	mock.ExpectExec("CREATE INDEX IF NOT EXISTS integration_workflow_retention_runs_scope_category_started_idx").WillReturnResult(pgxmock.NewResult("CREATE INDEX", 0))
	mock.ExpectCommit()

	if err := BootstrapDatabase(context.Background(), mock); err != nil {
		t.Fatalf("BootstrapDatabase() error = %v", err)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("ExpectationsWereMet() error = %v", err)
	}
}

func TestBootstrapDatabaseRejectsMissingSchema(t *testing.T) {
	if err := applySQL(context.Background(), nil, ""); err == nil {
		t.Fatal("applySQL() error = nil, want schema error")
	}
}

func TestOpenPoolRejectsInvalidDSN(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	_, err := OpenPool(ctx, "://bad-dsn")
	if err == nil {
		t.Fatal("OpenPool() error = nil, want invalid DSN error")
	}
}

func TestBootstrapDatabaseReturnsBeginFailure(t *testing.T) {
	db := failingBootstrapDB{err: errors.New("connection refused")}

	err := BootstrapDatabase(context.Background(), db)
	if err == nil {
		t.Fatal("BootstrapDatabase() error = nil, want begin failure")
	}

	if !strings.Contains(err.Error(), "begin bootstrap transaction") {
		t.Fatalf("error = %q, want wrapped begin failure", err)
	}
}

type failingBootstrapDB struct {
	err error
}

func (f failingBootstrapDB) BeginTx(ctx context.Context, txOptions pgx.TxOptions) (pgx.Tx, error) {
	return nil, f.err
}
