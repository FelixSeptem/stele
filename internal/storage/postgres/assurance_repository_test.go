package postgres

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"github.com/FelixSeptem/stele/internal/assurance"
	"github.com/FelixSeptem/stele/internal/memory"
	pgxmock "github.com/pashagolub/pgxmock/v4"
)

func TestRepositoryCreatesAndReadsHealthEvaluationWithOperationalProof(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatalf("pgxmock.NewPool() error = %v", err)
	}
	defer mock.Close()

	scope := memory.Scope{Tenant: "tenant-a", Project: "project-a", Namespace: "namespace-a"}
	now := time.Date(2026, 7, 12, 10, 0, 0, 0, time.UTC)
	evaluation := assurance.HealthEvaluation{
		ID:        "health_1",
		Scope:     scope,
		Status:    assurance.HealthStatusDegraded,
		Severity:  assurance.SeverityWarning,
		Reason:    assurance.ReasonBackupRestoreStale,
		CreatedAt: now,
		Components: []assurance.HealthComponentSummary{
			{
				ID:           "component_1",
				EvaluationID: "health_1",
				Scope:        scope,
				Component:    assurance.ComponentBackupRestore,
				Status:       assurance.HealthStatusDegraded,
				Severity:     assurance.SeverityWarning,
				Reason:       assurance.ReasonBackupRestoreStale,
				ObservedAt:   now,
				FreshThrough: now.Add(-time.Hour),
				Evidence:     map[string]any{"marker": "restore-check-1"},
			},
		},
	}

	mock.ExpectBegin()
	mock.ExpectQuery("INSERT INTO assurance_health_evaluations").
		WithArgs(evaluation.ID, scope.Tenant, scope.Project, scope.Namespace, evaluation.Status, evaluation.Severity, evaluation.Reason, now).
		WillReturnRows(assuranceHealthEvaluationRows().AddRow(evaluation.ID, scope.Tenant, scope.Project, scope.Namespace, evaluation.Status, evaluation.Severity, evaluation.Reason, now))
	mock.ExpectQuery("INSERT INTO assurance_health_components").
		WithArgs("component_1", "health_1", scope.Tenant, scope.Project, scope.Namespace, assurance.ComponentBackupRestore, assurance.HealthStatusDegraded, assurance.SeverityWarning, assurance.ReasonBackupRestoreStale, now, now.Add(-time.Hour), []byte(`{"marker":"restore-check-1"}`)).
		WillReturnRows(assuranceHealthComponentRows().AddRow("component_1", "health_1", scope.Tenant, scope.Project, scope.Namespace, assurance.ComponentBackupRestore, assurance.HealthStatusDegraded, assurance.SeverityWarning, assurance.ReasonBackupRestoreStale, now, now.Add(-time.Hour), []byte(`{"marker":"restore-check-1"}`)))
	mock.ExpectCommit()
	mock.ExpectQuery("SELECT[\\s\\S]*FROM assurance_health_evaluations").
		WithArgs(scope.Tenant, scope.Project, scope.Namespace, "health_1").
		WillReturnRows(assuranceHealthEvaluationRows().AddRow(evaluation.ID, scope.Tenant, scope.Project, scope.Namespace, evaluation.Status, evaluation.Severity, evaluation.Reason, now))
	mock.ExpectQuery("SELECT[\\s\\S]*FROM assurance_health_components").
		WithArgs(scope.Tenant, scope.Project, scope.Namespace, "health_1").
		WillReturnRows(assuranceHealthComponentRows().AddRow("component_1", "health_1", scope.Tenant, scope.Project, scope.Namespace, assurance.ComponentBackupRestore, assurance.HealthStatusDegraded, assurance.SeverityWarning, assurance.ReasonBackupRestoreStale, now, now.Add(-time.Hour), []byte(`{"marker":"restore-check-1"}`)))
	mock.ExpectQuery("SELECT[\\s\\S]*FROM assurance_health_evaluations").
		WithArgs(scope.Tenant, scope.Project, scope.Namespace).
		WillReturnRows(assuranceHealthEvaluationRows().AddRow(evaluation.ID, scope.Tenant, scope.Project, scope.Namespace, evaluation.Status, evaluation.Severity, evaluation.Reason, now))

	repo := NewRepository(mock)
	created, err := repo.CreateHealthEvaluation(context.Background(), evaluation)
	if err != nil {
		t.Fatalf("CreateHealthEvaluation() error = %v", err)
	}
	if created.Scope != scope || len(created.Components) != 1 {
		t.Fatalf("created = %+v, want scoped evaluation with component", created)
	}
	read, err := repo.ReadHealthEvaluation(context.Background(), assurance.ReadHealthEvaluationInput{Scope: scope, EvaluationID: "health_1"})
	if err != nil {
		t.Fatalf("ReadHealthEvaluation() error = %v", err)
	}
	if read.Scope != scope || read.Components[0].Component != assurance.ComponentBackupRestore {
		t.Fatalf("read = %+v, want backup restore component", read)
	}
	listed, err := repo.ListHealthEvaluations(context.Background(), scope)
	if err != nil {
		t.Fatalf("ListHealthEvaluations() error = %v", err)
	}
	if len(listed) != 1 || listed[0].ID != evaluation.ID {
		t.Fatalf("listed = %+v, want latest scoped health evaluation", listed)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("ExpectationsWereMet() error = %v", err)
	}
}

func TestRepositoryCreatesListsAndTransitionsIncidentAndAlert(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatalf("pgxmock.NewPool() error = %v", err)
	}
	defer mock.Close()

	scope := memory.Scope{Tenant: "tenant-a", Project: "project-a", Namespace: "namespace-a"}
	now := time.Date(2026, 7, 12, 11, 0, 0, 0, time.UTC)
	incident := assurance.Incident{
		ID:                 "incident_1",
		Scope:              scope,
		Status:             assurance.IncidentStatusOpen,
		Severity:           assurance.SeverityCritical,
		Component:          assurance.ComponentBackupRestore,
		Reason:             assurance.ReasonBackupRestoreStale,
		DeduplicationKey:   "tenant-a:backup_restore",
		LatestEvaluationID: "health_1",
		RunbookHints:       []assurance.RunbookHintCategory{assurance.RunbookHintReviewBackupRestoreProof},
		Metadata:           map[string]any{"proof_id": "proof_1"},
		OpenedAt:           now,
		UpdatedAt:          now,
	}
	transition := assurance.IncidentTransition{
		ID:         "transition_1",
		IncidentID: incident.ID,
		Scope:      scope,
		FromStatus: assurance.IncidentStatusOpen,
		ToStatus:   assurance.IncidentStatusAcknowledged,
		Action:     assurance.IncidentActionAcknowledge,
		Actor:      "admin-a",
		Reason:     "investigating restore proof",
		OccurredAt: now.Add(time.Minute),
	}
	alert := assurance.AlertCandidate{
		ID:               "alert_1",
		Scope:            scope,
		IncidentID:       incident.ID,
		EvaluationID:     "health_1",
		Severity:         assurance.SeverityCritical,
		Component:        assurance.ComponentBackupRestore,
		Reason:           assurance.ReasonBackupRestoreStale,
		DeduplicationKey: "tenant-a:backup_restore",
		DeliveryPolicy:   "default",
		Payload:          map[string]any{"component": "backup_restore"},
		CreatedAt:        now,
		NextAttemptAt:    now.Add(time.Minute),
	}
	attempt := assurance.AlertDeliveryAttempt{
		ID:               "attempt_1",
		AlertCandidateID: alert.ID,
		Scope:            scope,
		Adapter:          assurance.AlertAdapterStdout,
		Result:           assurance.AlertDeliveryResultSuccess,
		Attempt:          1,
		PayloadHash:      "payload-hash",
		AttemptedAt:      now.Add(time.Minute),
		CompletedAt:      now.Add(2 * time.Minute),
	}

	mock.ExpectQuery("INSERT INTO assurance_incidents").
		WithArgs(incident.ID, scope.Tenant, scope.Project, scope.Namespace, incident.Status, incident.Severity, incident.Component, incident.Reason, incident.DeduplicationKey, now, now, nil, incident.LatestEvaluationID, []string{"review_backup_restore_proof"}, []byte(`{"proof_id":"proof_1"}`)).
		WillReturnRows(assuranceIncidentRows().AddRow(incident.ID, scope.Tenant, scope.Project, scope.Namespace, incident.Status, incident.Severity, incident.Component, incident.Reason, incident.DeduplicationKey, now, now, nil, incident.LatestEvaluationID, []string{"review_backup_restore_proof"}, []byte(`{"proof_id":"proof_1"}`)))
	mock.ExpectQuery("SELECT[\\s\\S]*FROM assurance_incidents").
		WithArgs(scope.Tenant, scope.Project, scope.Namespace, string(assurance.IncidentStatusOpen), 10).
		WillReturnRows(assuranceIncidentRows().AddRow(incident.ID, scope.Tenant, scope.Project, scope.Namespace, incident.Status, incident.Severity, incident.Component, incident.Reason, incident.DeduplicationKey, now, now, nil, incident.LatestEvaluationID, []string{"review_backup_restore_proof"}, []byte(`{"proof_id":"proof_1"}`)))
	mock.ExpectBegin()
	mock.ExpectQuery("INSERT INTO assurance_incident_transitions").
		WithArgs(transition.ID, transition.IncidentID, scope.Tenant, scope.Project, scope.Namespace, transition.FromStatus, transition.ToStatus, transition.Action, transition.Actor, transition.Reason, transition.OccurredAt).
		WillReturnRows(assuranceIncidentTransitionRows().AddRow(transition.ID, transition.IncidentID, scope.Tenant, scope.Project, scope.Namespace, transition.FromStatus, transition.ToStatus, transition.Action, transition.Actor, transition.Reason, transition.OccurredAt))
	mock.ExpectQuery("UPDATE assurance_incidents").
		WithArgs(transition.ToStatus, transition.OccurredAt, nil, transition.IncidentID, scope.Tenant, scope.Project, scope.Namespace).
		WillReturnRows(assuranceIncidentRows().AddRow(incident.ID, scope.Tenant, scope.Project, scope.Namespace, transition.ToStatus, incident.Severity, incident.Component, incident.Reason, incident.DeduplicationKey, now, transition.OccurredAt, nil, incident.LatestEvaluationID, []string{"review_backup_restore_proof"}, []byte(`{"proof_id":"proof_1"}`)))
	mock.ExpectCommit()
	mock.ExpectQuery("INSERT INTO assurance_alert_candidates").
		WithArgs(alert.ID, scope.Tenant, scope.Project, scope.Namespace, alert.IncidentID, alert.EvaluationID, alert.Severity, alert.Component, alert.Reason, alert.DeduplicationKey, alert.DeliveryPolicy, []byte(`{"component":"backup_restore"}`), alert.CreatedAt, alert.NextAttemptAt, nil).
		WillReturnRows(assuranceAlertCandidateRows().AddRow(alert.ID, scope.Tenant, scope.Project, scope.Namespace, alert.IncidentID, alert.EvaluationID, alert.Severity, alert.Component, alert.Reason, alert.DeduplicationKey, alert.DeliveryPolicy, []byte(`{"component":"backup_restore"}`), alert.CreatedAt, alert.NextAttemptAt, nil))
	mock.ExpectQuery("SELECT[\\s\\S]*FROM assurance_alert_candidates").
		WithArgs(scope.Tenant, scope.Project, scope.Namespace, alert.ID).
		WillReturnRows(assuranceAlertCandidateRows().AddRow(alert.ID, scope.Tenant, scope.Project, scope.Namespace, alert.IncidentID, alert.EvaluationID, alert.Severity, alert.Component, alert.Reason, alert.DeduplicationKey, alert.DeliveryPolicy, []byte(`{"component":"backup_restore"}`), alert.CreatedAt, alert.NextAttemptAt, nil))
	mock.ExpectQuery("SELECT[\\s\\S]*FROM assurance_alert_candidates").
		WithArgs(scope.Tenant, scope.Project, scope.Namespace).
		WillReturnRows(assuranceAlertCandidateRows().AddRow(alert.ID, scope.Tenant, scope.Project, scope.Namespace, alert.IncidentID, alert.EvaluationID, alert.Severity, alert.Component, alert.Reason, alert.DeduplicationKey, alert.DeliveryPolicy, []byte(`{"component":"backup_restore"}`), alert.CreatedAt, alert.NextAttemptAt, nil))
	mock.ExpectQuery("INSERT INTO assurance_alert_delivery_attempts").
		WithArgs(attempt.ID, attempt.AlertCandidateID, scope.Tenant, scope.Project, scope.Namespace, attempt.Adapter, attempt.Result, nil, attempt.Attempt, nil, nil, nil, attempt.PayloadHash, attempt.AttemptedAt, attempt.CompletedAt).
		WillReturnRows(assuranceAlertDeliveryAttemptRows().AddRow(attempt.ID, attempt.AlertCandidateID, scope.Tenant, scope.Project, scope.Namespace, attempt.Adapter, attempt.Result, nil, attempt.Attempt, nil, nil, nil, attempt.PayloadHash, attempt.AttemptedAt, attempt.CompletedAt))
	mock.ExpectQuery("SELECT[\\s\\S]*FROM assurance_alert_delivery_attempts").
		WithArgs(scope.Tenant, scope.Project, scope.Namespace, alert.ID).
		WillReturnRows(assuranceAlertDeliveryAttemptRows().AddRow(attempt.ID, attempt.AlertCandidateID, scope.Tenant, scope.Project, scope.Namespace, attempt.Adapter, attempt.Result, nil, attempt.Attempt, nil, nil, nil, attempt.PayloadHash, attempt.AttemptedAt, attempt.CompletedAt))

	repo := NewRepository(mock)
	if _, err := repo.CreateIncident(context.Background(), incident); err != nil {
		t.Fatalf("CreateIncident() error = %v", err)
	}
	incidents, err := repo.ListIncidents(context.Background(), assurance.ListIncidentsInput{Scope: scope, Status: assurance.IncidentStatusOpen, Limit: 10})
	if err != nil {
		t.Fatalf("ListIncidents() error = %v", err)
	}
	if len(incidents) != 1 || incidents[0].Scope != scope {
		t.Fatalf("incidents = %+v, want scoped incident", incidents)
	}
	updated, err := repo.TransitionIncident(context.Background(), transition)
	if err != nil {
		t.Fatalf("TransitionIncident() error = %v", err)
	}
	if updated.Status != assurance.IncidentStatusAcknowledged {
		t.Fatalf("updated.Status = %q, want acknowledged", updated.Status)
	}
	if _, err := repo.CreateAlertCandidate(context.Background(), alert); err != nil {
		t.Fatalf("CreateAlertCandidate() error = %v", err)
	}
	if _, err := repo.ReadAlertCandidate(context.Background(), assurance.ReadAlertCandidateInput{Scope: scope, AlertCandidateID: alert.ID}); err != nil {
		t.Fatalf("ReadAlertCandidate() error = %v", err)
	}
	if _, err := repo.ListAlertCandidates(context.Background(), scope); err != nil {
		t.Fatalf("ListAlertCandidates() error = %v", err)
	}
	if _, err := repo.CreateAlertDeliveryAttempt(context.Background(), attempt); err != nil {
		t.Fatalf("CreateAlertDeliveryAttempt() error = %v", err)
	}
	attempts, err := repo.ListAlertDeliveryAttempts(context.Background(), assurance.ListAlertDeliveryAttemptsInput{Scope: scope, AlertCandidateID: alert.ID})
	if err != nil {
		t.Fatalf("ListAlertDeliveryAttempts() error = %v", err)
	}
	if len(attempts) != 1 || attempts[0].PayloadHash != "payload-hash" {
		t.Fatalf("attempts = %+v, want delivery attempt history", attempts)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("ExpectationsWereMet() error = %v", err)
	}
}

func TestRepositoryClaimAlertCandidatesForDeliveryUsesDurableLeaseAndScope(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatalf("pgxmock.NewPool() error = %v", err)
	}
	defer mock.Close()

	scope := memory.Scope{Tenant: "tenant-a", Project: "project-a", Namespace: "namespace-a"}
	now := time.Date(2026, 7, 12, 11, 30, 0, 0, time.UTC)
	input := assurance.ClaimAlertCandidatesForDeliveryInput{
		Scope:         scope,
		WorkerID:      "worker-a",
		Now:           now,
		LeaseDuration: 2 * time.Minute,
		Limit:         3,
		MaxAttempts:   5,
	}

	mock.ExpectQuery("WITH attempt_state AS").
		WithArgs(scope.Tenant, scope.Project, scope.Namespace, input.WorkerID, input.Now, input.Now.Add(input.LeaseDuration), input.Limit, input.MaxAttempts).
		WillReturnRows(assuranceAlertDeliveryClaimRows().AddRow(
			"alert_1", scope.Tenant, scope.Project, scope.Namespace, "incident_1", nil,
			assurance.SeverityCritical, assurance.ComponentBackupRestore, assurance.ReasonBackupRestoreStale,
			"incident:incident_1:backup_restore", "default", []byte(`{"component":"backup_restore"}`),
			now.Add(-time.Hour), now.Add(-time.Minute), nil,
			2, "worker-a", now, now.Add(2*time.Minute),
		))

	repo := NewRepository(mock)
	claims, err := repo.ClaimAlertCandidatesForDelivery(context.Background(), input)
	if err != nil {
		t.Fatalf("ClaimAlertCandidatesForDelivery() error = %v", err)
	}
	if len(claims) != 1 {
		t.Fatalf("len(claims) = %d, want 1", len(claims))
	}
	if claims[0].Candidate.ID != "alert_1" || claims[0].Candidate.Scope != scope {
		t.Fatalf("claim candidate = %+v, want scoped alert_1", claims[0].Candidate)
	}
	if claims[0].Attempt != 2 || claims[0].WorkerID != "worker-a" || !claims[0].LeaseUntil.Equal(now.Add(2*time.Minute)) {
		t.Fatalf("claim metadata = %+v, want attempt 2 leased by worker-a", claims[0])
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("ExpectationsWereMet() error = %v", err)
	}
}

func TestRepositoryCreatesConformanceReadinessAndRecoveryRecords(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatalf("pgxmock.NewPool() error = %v", err)
	}
	defer mock.Close()

	scope := memory.Scope{Tenant: "tenant-a", Project: "project-a", Namespace: "namespace-a"}
	now := time.Date(2026, 7, 12, 12, 0, 0, 0, time.UTC)
	profile := assurance.ConformanceProfile{
		ID:     "profile_1",
		Scope:  scope,
		Status: assurance.ConformanceProfileStatusActive,
		ExpectedEvidence: []assurance.ExpectedEvidence{
			{Kind: assurance.ExpectedEvidenceSession, MinimumCount: 1, FreshnessWindow: time.Hour},
		},
		Actor:     "admin-a",
		Reason:    "prove integration",
		CreatedAt: now,
		UpdatedAt: now,
	}
	run := assurance.ConformanceRun{
		ID:             "run_1",
		ProfileID:      profile.ID,
		Scope:          scope,
		Result:         assurance.ConformanceResultDegraded,
		EvidenceCounts: map[string]any{"session": 1},
		StartedAt:      now,
		FinishedAt:     now.Add(time.Minute),
		CreatedAt:      now,
	}
	diagnostic := assurance.MissingEvidenceDiagnostic{
		ID:               "diag_1",
		ConformanceRunID: run.ID,
		Scope:            scope,
		EvidenceKind:     assurance.ExpectedEvidenceOutcome,
		Category:         assurance.MissingEvidenceSessionWithoutOutcome,
		ReadinessImpact:  assurance.ReadinessStatusDegraded,
		Metadata:         map[string]any{"session_id": "session_1"},
		CreatedAt:        now,
	}
	proof := assurance.OperationalProof{
		ID:           "op_proof_1",
		Scope:        scope,
		Target:       assurance.OperationalProofCapacityLoad,
		Status:       assurance.HealthStatusHealthy,
		Severity:     assurance.SeverityInfo,
		Reason:       assurance.ReasonCapacityWithinThresholds,
		ObservedAt:   now,
		FreshThrough: now.Add(time.Hour),
		Evidence:     map[string]any{"backlog": 0},
		CreatedAt:    now,
	}
	backupProof := assurance.OperationalProof{
		ID:           "op_proof_2",
		Scope:        scope,
		Target:       assurance.OperationalProofBackupRestore,
		Status:       assurance.HealthStatusHealthy,
		Severity:     assurance.SeverityInfo,
		Reason:       assurance.ReasonBackupRestoreFresh,
		ObservedAt:   now,
		FreshThrough: now.Add(24 * time.Hour),
		Evidence:     map[string]any{"marker": "restore-check-1"},
		CreatedAt:    now,
	}
	report := assurance.ReadinessReport{
		ID:                 "readiness_1",
		Scope:              scope,
		Status:             assurance.ReadinessStatusDegraded,
		HealthEvaluationID: "health_1",
		ConformanceRunID:   run.ID,
		ComponentSummary:   map[string]any{"conformance": "degraded"},
		RecommendedActions: []assurance.RunbookHintCategory{assurance.RunbookHintReviewConformanceProfile},
		GeneratedAt:        now,
		CreatedAt:          now,
	}
	recovery := assurance.RecoveryVerification{
		ID:              "recovery_1",
		Scope:           scope,
		Target:          assurance.RecoveryVerificationTargetConformanceRun,
		TargetID:        run.ID,
		Status:          assurance.HealthStatusHealthy,
		CheckedSurfaces: []string{"conformance_run"},
		ResultCategory:  "recovered",
		LinkedEvidence:  map[string]any{"run_id": "run_2"},
		Actor:           "admin-a",
		Reason:          "verified after integration fix",
		CreatedAt:       now,
		VerifiedAt:      now.Add(time.Minute),
	}
	retention := assurance.RetentionRun{
		ID:             "retention_1",
		Scope:          scope,
		RecordCategory: assurance.RetentionClassDiagnostic,
		Cutoff:         now.Add(-24 * time.Hour),
		DeletedCount:   3,
		Status:         assurance.HealthStatusHealthy,
		StartedAt:      now,
		FinishedAt:     now.Add(time.Minute),
	}

	mock.ExpectQuery("INSERT INTO assurance_conformance_profiles").
		WithArgs(profile.ID, scope.Tenant, scope.Project, scope.Namespace, profile.Status, []byte(`[{"kind":"session","minimum_count":1,"freshness_window":3600000000000}]`), profile.Actor, profile.Reason, now, now, nil).
		WillReturnRows(assuranceConformanceProfileRows().AddRow(profile.ID, scope.Tenant, scope.Project, scope.Namespace, profile.Status, []byte(`[{"kind":"session","minimum_count":1,"freshness_window":3600000000000}]`), profile.Actor, profile.Reason, now, now, nil))
	mock.ExpectQuery("SELECT[\\s\\S]*FROM assurance_conformance_profiles").
		WithArgs(scope.Tenant, scope.Project, scope.Namespace, profile.ID).
		WillReturnRows(assuranceConformanceProfileRows().AddRow(profile.ID, scope.Tenant, scope.Project, scope.Namespace, profile.Status, []byte(`[{"kind":"session","minimum_count":1,"freshness_window":3600000000000}]`), profile.Actor, profile.Reason, now, now, nil))
	mock.ExpectQuery("SELECT[\\s\\S]*FROM assurance_conformance_profiles").
		WithArgs(scope.Tenant, scope.Project, scope.Namespace, string(profile.Status)).
		WillReturnRows(assuranceConformanceProfileRows().AddRow(profile.ID, scope.Tenant, scope.Project, scope.Namespace, profile.Status, []byte(`[{"kind":"session","minimum_count":1,"freshness_window":3600000000000}]`), profile.Actor, profile.Reason, now, now, nil))
	mock.ExpectQuery("INSERT INTO assurance_conformance_runs").
		WithArgs(run.ID, run.ProfileID, scope.Tenant, scope.Project, scope.Namespace, run.Result, []byte(`{"session":1}`), run.StartedAt, run.FinishedAt, run.CreatedAt).
		WillReturnRows(assuranceConformanceRunRows().AddRow(run.ID, run.ProfileID, scope.Tenant, scope.Project, scope.Namespace, run.Result, []byte(`{"session":1}`), run.StartedAt, run.FinishedAt, run.CreatedAt))
	mock.ExpectQuery("SELECT[\\s\\S]*FROM assurance_conformance_runs").
		WithArgs(scope.Tenant, scope.Project, scope.Namespace, run.ID).
		WillReturnRows(assuranceConformanceRunRows().AddRow(run.ID, run.ProfileID, scope.Tenant, scope.Project, scope.Namespace, run.Result, []byte(`{"session":1}`), run.StartedAt, run.FinishedAt, run.CreatedAt))
	mock.ExpectQuery("SELECT[\\s\\S]*FROM assurance_conformance_runs").
		WithArgs(scope.Tenant, scope.Project, scope.Namespace, run.ProfileID).
		WillReturnRows(assuranceConformanceRunRows().AddRow(run.ID, run.ProfileID, scope.Tenant, scope.Project, scope.Namespace, run.Result, []byte(`{"session":1}`), run.StartedAt, run.FinishedAt, run.CreatedAt))
	mock.ExpectQuery("INSERT INTO assurance_missing_evidence_diagnostics").
		WithArgs(diagnostic.ID, diagnostic.ConformanceRunID, scope.Tenant, scope.Project, scope.Namespace, diagnostic.EvidenceKind, diagnostic.Category, diagnostic.ReadinessImpact, []byte(`{"session_id":"session_1"}`), diagnostic.CreatedAt).
		WillReturnRows(assuranceMissingEvidenceDiagnosticRows().AddRow(diagnostic.ID, diagnostic.ConformanceRunID, scope.Tenant, scope.Project, scope.Namespace, diagnostic.EvidenceKind, diagnostic.Category, diagnostic.ReadinessImpact, []byte(`{"session_id":"session_1"}`), diagnostic.CreatedAt))
	mock.ExpectQuery("INSERT INTO assurance_operational_proofs").
		WithArgs(proof.ID, scope.Tenant, scope.Project, scope.Namespace, proof.Target, proof.Status, proof.Severity, proof.Reason, proof.ObservedAt, proof.FreshThrough, []byte(`{"backlog":0}`), proof.CreatedAt).
		WillReturnRows(assuranceOperationalProofRows().AddRow(proof.ID, scope.Tenant, scope.Project, scope.Namespace, proof.Target, proof.Status, proof.Severity, proof.Reason, proof.ObservedAt, proof.FreshThrough, []byte(`{"backlog":0}`), proof.CreatedAt))
	mock.ExpectQuery("SELECT[\\s\\S]*FROM assurance_operational_proofs").
		WithArgs(scope.Tenant, scope.Project, scope.Namespace, proof.ID).
		WillReturnRows(assuranceOperationalProofRows().AddRow(proof.ID, scope.Tenant, scope.Project, scope.Namespace, proof.Target, proof.Status, proof.Severity, proof.Reason, proof.ObservedAt, proof.FreshThrough, []byte(`{"backlog":0}`), proof.CreatedAt))
	mock.ExpectQuery("SELECT[\\s\\S]*FROM assurance_operational_proofs").
		WithArgs(scope.Tenant, scope.Project, scope.Namespace).
		WillReturnRows(assuranceOperationalProofRows().AddRow(proof.ID, scope.Tenant, scope.Project, scope.Namespace, proof.Target, proof.Status, proof.Severity, proof.Reason, proof.ObservedAt, proof.FreshThrough, []byte(`{"backlog":0}`), proof.CreatedAt))
	mock.ExpectQuery("INSERT INTO assurance_operational_proofs").
		WithArgs(backupProof.ID, scope.Tenant, scope.Project, scope.Namespace, backupProof.Target, backupProof.Status, backupProof.Severity, backupProof.Reason, backupProof.ObservedAt, backupProof.FreshThrough, []byte(`{"marker":"restore-check-1"}`), backupProof.CreatedAt).
		WillReturnRows(assuranceOperationalProofRows().AddRow(backupProof.ID, scope.Tenant, scope.Project, scope.Namespace, backupProof.Target, backupProof.Status, backupProof.Severity, backupProof.Reason, backupProof.ObservedAt, backupProof.FreshThrough, []byte(`{"marker":"restore-check-1"}`), backupProof.CreatedAt))
	mock.ExpectQuery("SELECT[\\s\\S]*FROM assurance_operational_proofs").
		WithArgs(scope.Tenant, scope.Project, scope.Namespace, backupProof.ID).
		WillReturnRows(assuranceOperationalProofRows().AddRow(backupProof.ID, scope.Tenant, scope.Project, scope.Namespace, backupProof.Target, backupProof.Status, backupProof.Severity, backupProof.Reason, backupProof.ObservedAt, backupProof.FreshThrough, []byte(`{"marker":"restore-check-1"}`), backupProof.CreatedAt))
	mock.ExpectQuery("SELECT[\\s\\S]*FROM assurance_operational_proofs").
		WithArgs(scope.Tenant, scope.Project, scope.Namespace).
		WillReturnRows(assuranceOperationalProofRows().AddRow(backupProof.ID, scope.Tenant, scope.Project, scope.Namespace, backupProof.Target, backupProof.Status, backupProof.Severity, backupProof.Reason, backupProof.ObservedAt, backupProof.FreshThrough, []byte(`{"marker":"restore-check-1"}`), backupProof.CreatedAt))
	mock.ExpectQuery("INSERT INTO assurance_readiness_reports").
		WithArgs(report.ID, scope.Tenant, scope.Project, scope.Namespace, report.Status, report.HealthEvaluationID, report.ConformanceRunID, []byte(`{"conformance":"degraded"}`), []string{"review_conformance_profile"}, report.GeneratedAt, report.CreatedAt).
		WillReturnRows(assuranceReadinessReportRows().AddRow(report.ID, scope.Tenant, scope.Project, scope.Namespace, report.Status, report.HealthEvaluationID, report.ConformanceRunID, []byte(`{"conformance":"degraded"}`), []string{"review_conformance_profile"}, report.GeneratedAt, report.CreatedAt))
	mock.ExpectQuery("SELECT[\\s\\S]*FROM assurance_readiness_reports").
		WithArgs(scope.Tenant, scope.Project, scope.Namespace, report.ID).
		WillReturnRows(assuranceReadinessReportRows().AddRow(report.ID, scope.Tenant, scope.Project, scope.Namespace, report.Status, report.HealthEvaluationID, report.ConformanceRunID, []byte(`{"conformance":"degraded"}`), []string{"review_conformance_profile"}, report.GeneratedAt, report.CreatedAt))
	mock.ExpectQuery("INSERT INTO assurance_recovery_verifications").
		WithArgs(recovery.ID, scope.Tenant, scope.Project, scope.Namespace, recovery.Target, recovery.TargetID, recovery.Status, recovery.CheckedSurfaces, recovery.ResultCategory, []byte(`{"run_id":"run_2"}`), recovery.Actor, recovery.Reason, recovery.CreatedAt, recovery.VerifiedAt).
		WillReturnRows(assuranceRecoveryVerificationRows().AddRow(recovery.ID, scope.Tenant, scope.Project, scope.Namespace, recovery.Target, recovery.TargetID, recovery.Status, recovery.CheckedSurfaces, recovery.ResultCategory, []byte(`{"run_id":"run_2"}`), recovery.Actor, recovery.Reason, recovery.CreatedAt, recovery.VerifiedAt))
	mock.ExpectQuery("SELECT[\\s\\S]*FROM assurance_recovery_verifications").
		WithArgs(scope.Tenant, scope.Project, scope.Namespace, recovery.ID).
		WillReturnRows(assuranceRecoveryVerificationRows().AddRow(recovery.ID, scope.Tenant, scope.Project, scope.Namespace, recovery.Target, recovery.TargetID, recovery.Status, recovery.CheckedSurfaces, recovery.ResultCategory, []byte(`{"run_id":"run_2"}`), recovery.Actor, recovery.Reason, recovery.CreatedAt, recovery.VerifiedAt))
	mock.ExpectQuery("SELECT[\\s\\S]*FROM assurance_recovery_verifications").
		WithArgs(scope.Tenant, scope.Project, scope.Namespace).
		WillReturnRows(assuranceRecoveryVerificationRows().AddRow(recovery.ID, scope.Tenant, scope.Project, scope.Namespace, recovery.Target, recovery.TargetID, recovery.Status, recovery.CheckedSurfaces, recovery.ResultCategory, []byte(`{"run_id":"run_2"}`), recovery.Actor, recovery.Reason, recovery.CreatedAt, recovery.VerifiedAt))
	mock.ExpectQuery("INSERT INTO assurance_retention_runs").
		WithArgs(retention.ID, scope.Tenant, scope.Project, scope.Namespace, retention.RecordCategory, retention.Cutoff, retention.DeletedCount, retention.Status, retention.StartedAt, retention.FinishedAt).
		WillReturnRows(assuranceRetentionRunRows().AddRow(retention.ID, scope.Tenant, scope.Project, scope.Namespace, retention.RecordCategory, retention.Cutoff, retention.DeletedCount, retention.Status, retention.StartedAt, retention.FinishedAt))
	mock.ExpectQuery("SELECT[\\s\\S]*FROM assurance_retention_runs").
		WithArgs(scope.Tenant, scope.Project, scope.Namespace, retention.ID).
		WillReturnRows(assuranceRetentionRunRows().AddRow(retention.ID, scope.Tenant, scope.Project, scope.Namespace, retention.RecordCategory, retention.Cutoff, retention.DeletedCount, retention.Status, retention.StartedAt, retention.FinishedAt))
	mock.ExpectQuery("SELECT[\\s\\S]*FROM assurance_retention_runs").
		WithArgs(scope.Tenant, scope.Project, scope.Namespace, string(retention.RecordCategory)).
		WillReturnRows(assuranceRetentionRunRows().AddRow(retention.ID, scope.Tenant, scope.Project, scope.Namespace, retention.RecordCategory, retention.Cutoff, retention.DeletedCount, retention.Status, retention.StartedAt, retention.FinishedAt))

	repo := NewRepository(mock)
	if _, err := repo.CreateConformanceProfile(context.Background(), profile); err != nil {
		t.Fatalf("CreateConformanceProfile() error = %v", err)
	}
	if _, err := repo.ReadConformanceProfile(context.Background(), assurance.ReadConformanceProfileInput{Scope: scope, ProfileID: profile.ID}); err != nil {
		t.Fatalf("ReadConformanceProfile() error = %v", err)
	}
	if _, err := repo.ListConformanceProfiles(context.Background(), assurance.ListConformanceProfilesInput{Scope: scope, Status: profile.Status}); err != nil {
		t.Fatalf("ListConformanceProfiles() error = %v", err)
	}
	if _, err := repo.CreateConformanceRun(context.Background(), run); err != nil {
		t.Fatalf("CreateConformanceRun() error = %v", err)
	}
	if _, err := repo.ReadConformanceRun(context.Background(), assurance.ReadConformanceRunInput{Scope: scope, RunID: run.ID}); err != nil {
		t.Fatalf("ReadConformanceRun() error = %v", err)
	}
	if _, err := repo.ListConformanceRuns(context.Background(), assurance.ListConformanceRunsInput{Scope: scope, ProfileID: run.ProfileID}); err != nil {
		t.Fatalf("ListConformanceRuns() error = %v", err)
	}
	if _, err := repo.CreateMissingEvidenceDiagnostic(context.Background(), diagnostic); err != nil {
		t.Fatalf("CreateMissingEvidenceDiagnostic() error = %v", err)
	}
	if _, err := repo.CreateOperationalProof(context.Background(), proof); err != nil {
		t.Fatalf("CreateOperationalProof() error = %v", err)
	}
	if _, err := repo.ReadOperationalProof(context.Background(), assurance.ReadOperationalProofInput{Scope: scope, ProofID: proof.ID}); err != nil {
		t.Fatalf("ReadOperationalProof() error = %v", err)
	}
	if _, err := repo.ListOperationalProofs(context.Background(), scope); err != nil {
		t.Fatalf("ListOperationalProofs() error = %v", err)
	}
	if _, err := repo.CreateOperationalProof(context.Background(), backupProof); err != nil {
		t.Fatalf("CreateOperationalProof(backup) error = %v", err)
	}
	if _, err := repo.ReadOperationalProof(context.Background(), assurance.ReadOperationalProofInput{Scope: scope, ProofID: backupProof.ID}); err != nil {
		t.Fatalf("ReadOperationalProof(backup) error = %v", err)
	}
	if _, err := repo.ListOperationalProofs(context.Background(), scope); err != nil {
		t.Fatalf("ListOperationalProofs(backup) error = %v", err)
	}
	if _, err := repo.CreateReadinessReport(context.Background(), report); err != nil {
		t.Fatalf("CreateReadinessReport() error = %v", err)
	}
	readiness, err := repo.ReadReadinessReport(context.Background(), assurance.ReadReadinessReportInput{Scope: scope, ReportID: report.ID})
	if err != nil {
		t.Fatalf("ReadReadinessReport() error = %v", err)
	}
	if readiness.Status != assurance.ReadinessStatusDegraded {
		t.Fatalf("readiness.Status = %q, want degraded", readiness.Status)
	}
	if _, err := repo.CreateRecoveryVerification(context.Background(), recovery); err != nil {
		t.Fatalf("CreateRecoveryVerification() error = %v", err)
	}
	if _, err := repo.ReadRecoveryVerification(context.Background(), assurance.ReadRecoveryVerificationInput{Scope: scope, RecordID: recovery.ID}); err != nil {
		t.Fatalf("ReadRecoveryVerification() error = %v", err)
	}
	if _, err := repo.ListRecoveryVerifications(context.Background(), scope); err != nil {
		t.Fatalf("ListRecoveryVerifications() error = %v", err)
	}
	if _, err := repo.CreateRetentionRun(context.Background(), retention); err != nil {
		t.Fatalf("CreateRetentionRun() error = %v", err)
	}
	if _, err := repo.ReadRetentionRun(context.Background(), assurance.ReadRetentionRunInput{Scope: scope, RunID: retention.ID}); err != nil {
		t.Fatalf("ReadRetentionRun() error = %v", err)
	}
	if _, err := repo.ListRetentionRuns(context.Background(), assurance.ListRetentionRunsInput{Scope: scope, RecordCategory: retention.RecordCategory}); err != nil {
		t.Fatalf("ListRetentionRuns() error = %v", err)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("ExpectationsWereMet() error = %v", err)
	}
}

func TestRepositoryRejectsInvalidConformanceProfile(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatalf("pgxmock.NewPool() error = %v", err)
	}
	defer mock.Close()

	repo := NewRepository(mock)
	_, err = repo.CreateConformanceProfile(context.Background(), assurance.ConformanceProfile{
		ID:     "profile_invalid",
		Scope:  memory.Scope{Tenant: "tenant-a", Project: "project-a", Namespace: "namespace-a"},
		Status: assurance.ConformanceProfileStatusActive,
		ExpectedEvidence: []assurance.ExpectedEvidence{
			{Kind: assurance.ExpectedEvidenceKind("invalid"), MinimumCount: 1, FreshnessWindow: time.Hour},
		},
		Actor:     "admin-a",
		Reason:    "invalid profile",
		CreatedAt: time.Date(2026, 7, 12, 13, 0, 0, 0, time.UTC),
		UpdatedAt: time.Date(2026, 7, 12, 13, 0, 0, 0, time.UTC),
	})
	if err == nil {
		t.Fatal("CreateConformanceProfile() error = nil, want validation error")
	}
}

func TestRepositoryUpdatesAndDisablesConformanceProfile(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatalf("pgxmock.NewPool() error = %v", err)
	}
	defer mock.Close()

	scope := memory.Scope{Tenant: "tenant-a", Project: "project-a", Namespace: "namespace-a"}
	now := time.Date(2026, 7, 12, 13, 15, 0, 0, time.UTC)
	updatedAt := now.Add(time.Minute)
	disabledAt := now.Add(2 * time.Minute)

	mock.ExpectQuery("UPDATE assurance_conformance_profiles").
		WithArgs(
			scope.Tenant,
			scope.Project,
			scope.Namespace,
			"profile_1",
			[]byte(`[{"kind":"context","minimum_count":2,"freshness_window":7200000000000}]`),
			"admin-b",
			"expand coverage",
			updatedAt,
		).
		WillReturnRows(assuranceConformanceProfileRows().AddRow(
			"profile_1",
			scope.Tenant,
			scope.Project,
			scope.Namespace,
			assurance.ConformanceProfileStatusActive,
			[]byte(`[{"kind":"context","minimum_count":2,"freshness_window":7200000000000}]`),
			"admin-b",
			"expand coverage",
			now,
			updatedAt,
			nil,
		))
	mock.ExpectQuery("UPDATE assurance_conformance_profiles").
		WithArgs(
			scope.Tenant,
			scope.Project,
			scope.Namespace,
			"profile_1",
			assurance.ConformanceProfileStatusDisabled,
			"admin-c",
			"disable coverage",
			disabledAt,
		).
		WillReturnRows(assuranceConformanceProfileRows().AddRow(
			"profile_1",
			scope.Tenant,
			scope.Project,
			scope.Namespace,
			assurance.ConformanceProfileStatusDisabled,
			[]byte(`[{"kind":"context","minimum_count":2,"freshness_window":7200000000000}]`),
			"admin-c",
			"disable coverage",
			now,
			disabledAt,
			disabledAt,
		))

	repo := NewRepository(mock)
	updated, err := repo.UpdateConformanceProfile(context.Background(), assurance.UpdateConformanceProfileInput{
		Scope:     scope,
		ProfileID: "profile_1",
		ExpectedEvidence: []assurance.ExpectedEvidence{
			{Kind: assurance.ExpectedEvidenceContext, MinimumCount: 2, FreshnessWindow: 2 * time.Hour},
		},
		Actor:     "admin-b",
		Reason:    "expand coverage",
		UpdatedAt: updatedAt,
	})
	if err != nil {
		t.Fatalf("UpdateConformanceProfile() error = %v", err)
	}
	if updated.Status != assurance.ConformanceProfileStatusActive || updated.UpdatedAt != updatedAt {
		t.Fatalf("updated = %+v, want active profile with updated timestamp", updated)
	}

	disabled, err := repo.DisableConformanceProfile(context.Background(), assurance.DisableConformanceProfileInput{
		Scope:      scope,
		ProfileID:  "profile_1",
		Actor:      "admin-c",
		Reason:     "disable coverage",
		DisabledAt: disabledAt,
	})
	if err != nil {
		t.Fatalf("DisableConformanceProfile() error = %v", err)
	}
	if disabled.Status != assurance.ConformanceProfileStatusDisabled || disabled.DisabledAt != disabledAt {
		t.Fatalf("disabled = %+v, want disabled profile with timestamp", disabled)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("ExpectationsWereMet() error = %v", err)
	}
}

func TestRepositoryInspectsConformanceEvidenceByScope(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatalf("pgxmock.NewPool() error = %v", err)
	}
	defer mock.Close()

	scope := memory.Scope{Tenant: "tenant-a", Project: "project-a", Namespace: "namespace-a"}
	now := time.Date(2026, 7, 12, 13, 20, 0, 0, time.UTC)
	expected := []assurance.ExpectedEvidence{
		{Kind: assurance.ExpectedEvidenceSession, MinimumCount: 1, FreshnessWindow: time.Hour},
		{Kind: assurance.ExpectedEvidenceContext, MinimumCount: 1, FreshnessWindow: time.Hour},
		{Kind: assurance.ExpectedEvidenceOutcome, MinimumCount: 1, FreshnessWindow: time.Hour},
		{Kind: assurance.ExpectedEvidenceVerification, MinimumCount: 1, FreshnessWindow: time.Hour},
		{Kind: assurance.ExpectedEvidenceUsefulnessFeedback, MinimumCount: 1, FreshnessWindow: time.Hour},
		{Kind: assurance.ExpectedEvidenceTaskEvaluation, MinimumCount: 1, FreshnessWindow: time.Hour},
		{Kind: assurance.ExpectedEvidenceProof, MinimumCount: 1, FreshnessWindow: time.Hour},
		{Kind: assurance.ExpectedEvidenceRepair, MinimumCount: 1, FreshnessWindow: time.Hour},
		{Kind: assurance.ExpectedEvidenceRankingRollout, MinimumCount: 1, FreshnessWindow: time.Hour},
	}

	mock.ExpectQuery("FROM memory_session_runs").
		WithArgs(scope.Tenant, scope.Project, scope.Namespace).
		WillReturnRows(conformanceEvidenceRows().AddRow(int64(1), now, false, false, false))
	mock.ExpectQuery("FROM memory_session_turns").
		WithArgs(scope.Tenant, scope.Project, scope.Namespace).
		WillReturnRows(conformanceEvidenceRows().AddRow(int64(1), now, false, false, false))
	mock.ExpectQuery("FROM memory_session_turns").
		WithArgs(scope.Tenant, scope.Project, scope.Namespace).
		WillReturnRows(conformanceEvidenceRows().AddRow(int64(1), now, false, false, false))
	mock.ExpectQuery("FROM memory_session_verifications").
		WithArgs(scope.Tenant, scope.Project, scope.Namespace).
		WillReturnRows(conformanceEvidenceRows().AddRow(int64(1), now, false, false, false))
	mock.ExpectQuery("FROM usefulness_feedback uf").
		WithArgs(scope.Tenant, scope.Project, scope.Namespace).
		WillReturnRows(conformanceEvidenceRows().AddRow(int64(1), now, false, false, false))
	mock.ExpectQuery("FROM task_evaluations te").
		WithArgs(scope.Tenant, scope.Project, scope.Namespace).
		WillReturnRows(conformanceEvidenceRows().AddRow(int64(1), now, true, false, false))
	mock.ExpectQuery("FROM scope_proof_runs").
		WithArgs(scope.Tenant, scope.Project, scope.Namespace).
		WillReturnRows(conformanceEvidenceRows().AddRow(int64(1), now, false, false, false))
	mock.ExpectQuery("FROM repair_plans").
		WithArgs(scope.Tenant, scope.Project, scope.Namespace).
		WillReturnRows(conformanceEvidenceRows().AddRow(int64(1), now, false, true, false))
	mock.ExpectQuery("FROM ranking_rollout_dry_runs").
		WithArgs(scope.Tenant, scope.Project, scope.Namespace).
		WillReturnRows(conformanceEvidenceRows().AddRow(int64(1), now, false, false, true))

	repo := NewRepository(mock)
	observations, err := repo.InspectConformanceEvidence(context.Background(), assurance.ConformanceEvidenceInspectionInput{
		Scope:            scope,
		ExpectedEvidence: expected,
		ObservedAt:       now,
	})
	if err != nil {
		t.Fatalf("InspectConformanceEvidence() error = %v", err)
	}
	if len(observations) != len(expected) {
		t.Fatalf("len(observations) = %d, want %d", len(observations), len(expected))
	}
	if !observations[5].OpaqueOnly {
		t.Fatalf("task observation = %+v, want opaque-only flag", observations[5])
	}
	if !observations[7].Contradictory {
		t.Fatalf("repair observation = %+v, want contradictory flag", observations[7])
	}
	if !observations[8].Hidden {
		t.Fatalf("ranking observation = %+v, want hidden flag", observations[8])
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("ExpectationsWereMet() error = %v", err)
	}
}

func TestRepositoryPreservesHighCardinalityEvidence(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatalf("pgxmock.NewPool() error = %v", err)
	}
	defer mock.Close()

	scope := memory.Scope{Tenant: "tenant-a", Project: "project-a", Namespace: "namespace-a"}
	now := time.Date(2026, 7, 12, 13, 30, 0, 0, time.UTC)
	evidence := highCardinalityEvidence(32)
	proof := assurance.OperationalProof{
		ID:           "op_proof_high_card",
		Scope:        scope,
		Target:       assurance.OperationalProofCapacityLoad,
		Status:       assurance.HealthStatusHealthy,
		Severity:     assurance.SeverityInfo,
		Reason:       assurance.ReasonCapacityWithinThresholds,
		ObservedAt:   now,
		FreshThrough: now.Add(time.Hour),
		Evidence:     evidence,
		CreatedAt:    now,
	}

	mock.ExpectQuery("INSERT INTO assurance_operational_proofs").
		WithArgs(proof.ID, scope.Tenant, scope.Project, scope.Namespace, proof.Target, proof.Status, proof.Severity, proof.Reason, proof.ObservedAt, proof.FreshThrough, anyJSONBytes(evidence), proof.CreatedAt).
		WillReturnRows(assuranceOperationalProofRows().AddRow(proof.ID, scope.Tenant, scope.Project, scope.Namespace, proof.Target, proof.Status, proof.Severity, proof.Reason, proof.ObservedAt, proof.FreshThrough, anyJSONBytes(evidence), proof.CreatedAt))
	mock.ExpectQuery("SELECT[\\s\\S]*FROM assurance_operational_proofs").
		WithArgs(scope.Tenant, scope.Project, scope.Namespace, proof.ID).
		WillReturnRows(assuranceOperationalProofRows().AddRow(proof.ID, scope.Tenant, scope.Project, scope.Namespace, proof.Target, proof.Status, proof.Severity, proof.Reason, proof.ObservedAt, proof.FreshThrough, anyJSONBytes(evidence), proof.CreatedAt))

	repo := NewRepository(mock)
	created, err := repo.CreateOperationalProof(context.Background(), proof)
	if err != nil {
		t.Fatalf("CreateOperationalProof() error = %v", err)
	}
	if len(created.Evidence) != 32 {
		t.Fatalf("created.Evidence len = %d, want 32", len(created.Evidence))
	}
	read, err := repo.ReadOperationalProof(context.Background(), assurance.ReadOperationalProofInput{Scope: scope, ProofID: proof.ID})
	if err != nil {
		t.Fatalf("ReadOperationalProof() error = %v", err)
	}
	if len(read.Evidence) != 32 {
		t.Fatalf("read.Evidence len = %d, want 32", len(read.Evidence))
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("ExpectationsWereMet() error = %v", err)
	}
}

func assuranceHealthEvaluationRows() *pgxmock.Rows {
	return pgxmock.NewRows([]string{"id", "tenant", "project", "namespace", "status", "severity", "reason", "created_at"})
}

func assuranceHealthComponentRows() *pgxmock.Rows {
	return pgxmock.NewRows([]string{"id", "evaluation_id", "tenant", "project", "namespace", "component", "status", "severity", "reason", "observed_at", "fresh_through", "evidence"})
}

func assuranceIncidentRows() *pgxmock.Rows {
	return pgxmock.NewRows([]string{"id", "tenant", "project", "namespace", "status", "severity", "component", "reason", "deduplication_key", "opened_at", "updated_at", "resolved_at", "latest_evaluation_id", "runbook_hints", "metadata"})
}

func assuranceIncidentTransitionRows() *pgxmock.Rows {
	return pgxmock.NewRows([]string{"id", "incident_id", "tenant", "project", "namespace", "from_status", "to_status", "action", "actor", "reason", "occurred_at"})
}

func assuranceAlertCandidateRows() *pgxmock.Rows {
	return pgxmock.NewRows([]string{"id", "tenant", "project", "namespace", "incident_id", "evaluation_id", "severity", "component", "reason", "deduplication_key", "delivery_policy", "payload", "created_at", "next_attempt_at", "suppressed_until"})
}

func assuranceAlertDeliveryAttemptRows() *pgxmock.Rows {
	return pgxmock.NewRows([]string{"id", "alert_candidate_id", "tenant", "project", "namespace", "adapter_kind", "result", "failure_category", "attempt", "worker_id", "lease_until", "next_attempt_at", "payload_hash", "attempted_at", "completed_at"})
}

func assuranceAlertDeliveryClaimRows() *pgxmock.Rows {
	return pgxmock.NewRows([]string{"id", "tenant", "project", "namespace", "incident_id", "evaluation_id", "severity", "component", "reason", "deduplication_key", "delivery_policy", "payload", "created_at", "next_attempt_at", "suppressed_until", "attempt", "worker_id", "claimed_at", "lease_until"})
}

func assuranceConformanceProfileRows() *pgxmock.Rows {
	return pgxmock.NewRows([]string{"id", "tenant", "project", "namespace", "status", "expected_evidence", "actor", "reason", "created_at", "updated_at", "disabled_at"})
}

func assuranceConformanceRunRows() *pgxmock.Rows {
	return pgxmock.NewRows([]string{"id", "profile_id", "tenant", "project", "namespace", "result", "evidence_counts", "started_at", "finished_at", "created_at"})
}

func assuranceMissingEvidenceDiagnosticRows() *pgxmock.Rows {
	return pgxmock.NewRows([]string{"id", "conformance_run_id", "tenant", "project", "namespace", "evidence_kind", "category", "readiness_impact", "metadata", "created_at"})
}

func assuranceOperationalProofRows() *pgxmock.Rows {
	return pgxmock.NewRows([]string{"id", "tenant", "project", "namespace", "target", "status", "severity", "reason", "observed_at", "fresh_through", "evidence", "created_at"})
}

func assuranceReadinessReportRows() *pgxmock.Rows {
	return pgxmock.NewRows([]string{"id", "tenant", "project", "namespace", "status", "health_evaluation_id", "conformance_run_id", "component_summary", "recommended_actions", "generated_at", "created_at"})
}

func assuranceRecoveryVerificationRows() *pgxmock.Rows {
	return pgxmock.NewRows([]string{"id", "tenant", "project", "namespace", "target_kind", "target_id", "status", "checked_surfaces", "result_category", "linked_evidence", "actor", "reason", "created_at", "verified_at"})
}

func assuranceRetentionRunRows() *pgxmock.Rows {
	return pgxmock.NewRows([]string{"id", "tenant", "project", "namespace", "record_category", "cutoff", "deleted_count", "status", "started_at", "finished_at"})
}

func conformanceEvidenceRows() *pgxmock.Rows {
	return pgxmock.NewRows([]string{"count", "freshest_at", "opaque_only", "contradictory", "hidden"})
}

func highCardinalityEvidence(entries int) map[string]any {
	evidence := make(map[string]any, entries)
	for i := 0; i < entries; i++ {
		evidence[fmt.Sprintf("key_%02d", i)] = fmt.Sprintf("value_%02d", i)
	}
	return evidence
}

func anyJSONBytes(value any) []byte {
	data, _ := json.Marshal(value)
	return data
}
