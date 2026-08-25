package assurance

import (
	"bytes"
	"context"
	"io"
	"log"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/FelixSeptem/stele/internal/memory"
	"github.com/FelixSeptem/stele/internal/telemetry"
)

func TestServiceCreatesHealthEvaluationAndDerivedIncidents(t *testing.T) {
	now := time.Date(2026, 7, 12, 14, 0, 0, 0, time.UTC)
	scope := memory.Scope{Tenant: "tenant-a", Project: "project-a", Namespace: "namespace-a"}
	store := &stubAssuranceStore{}
	service := NewService(ServiceOptions{
		Store: store,
		Now:   func() time.Time { return now },
		NewID: func(prefix string) string { return prefix + "_1" },
	})

	runtimeEvidence := map[string]any{"uptime_seconds": 3600}
	embeddingEvidence := map[string]any{"index_state": "ready"}
	taskEvidence := map[string]any{"total_evaluations": 8}
	backupEvidence := map[string]any{"marker": "backup-1"}
	input := HealthEvaluationInput{
		Scope:        scope,
		ObservedAt:   now,
		EvaluationID: "",
		RuntimeReadiness: HealthObservation{
			Status:     HealthStatusHealthy,
			Severity:   SeverityInfo,
			Reason:     ReasonRuntimeReady,
			Evidence:   runtimeEvidence,
			ObservedAt: now,
		},
		BacklogState: HealthObservation{
			Status:     HealthStatusDegraded,
			Severity:   SeverityWarning,
			Reason:     ReasonBacklogPressure,
			Evidence:   map[string]any{"depth": 42},
			ObservedAt: now,
		},
		EmbeddingHealth: HealthObservation{
			Status:     HealthStatusHealthy,
			Severity:   SeverityInfo,
			Reason:     ReasonRuntimeReady,
			Evidence:   embeddingEvidence,
			ObservedAt: now,
		},
		ProofSessionVerdict: HealthObservation{
			Status:     HealthStatusHealthy,
			Severity:   SeverityInfo,
			Reason:     ReasonRuntimeReady,
			Evidence:   map[string]any{"scope_proof_verdict": "passed"},
			ObservedAt: now,
		},
		UsefulnessFeedback: HealthObservation{
			Status:     HealthStatusHealthy,
			Severity:   SeverityInfo,
			Reason:     ReasonRuntimeReady,
			Evidence:   map[string]any{"effective_quality": "positive"},
			ObservedAt: now,
		},
		TaskEvaluationSummary: HealthObservation{
			Status:     HealthStatusHealthy,
			Severity:   SeverityInfo,
			Reason:     ReasonRuntimeReady,
			Evidence:   taskEvidence,
			ObservedAt: now,
		},
		RepairStatus: HealthObservation{
			Status:     HealthStatusHealthy,
			Severity:   SeverityInfo,
			Reason:     ReasonRuntimeReady,
			Evidence:   map[string]any{"repair_state": "idle"},
			ObservedAt: now,
		},
		RankingRolloutState: HealthObservation{
			Status:     HealthStatusHealthy,
			Severity:   SeverityInfo,
			Reason:     ReasonRuntimeReady,
			Evidence:   map[string]any{"mode": "dry_run"},
			ObservedAt: now,
		},
		ConformanceStatus: HealthObservation{
			Status:     HealthStatusHealthy,
			Severity:   SeverityInfo,
			Reason:     ReasonRuntimeReady,
			Evidence:   map[string]any{"profile_count": 1},
			ObservedAt: now,
		},
		CapacityLoadProof: HealthObservation{
			Status:       HealthStatusUnhealthy,
			Severity:     SeverityCritical,
			Reason:       ReasonCapacityThresholdExceeded,
			Evidence:     map[string]any{"worker_latency_ms": 900},
			ObservedAt:   now,
			FreshThrough: now.Add(30 * time.Minute),
		},
		BackupRestoreProof: HealthObservation{
			Status:       HealthStatusHealthy,
			Severity:     SeverityInfo,
			Reason:       ReasonBackupRestoreFresh,
			Evidence:     backupEvidence,
			ObservedAt:   now,
			FreshThrough: now.Add(24 * time.Hour),
		},
	}

	evaluation, err := service.CreateHealthEvaluation(context.Background(), input)
	if err != nil {
		t.Fatalf("CreateHealthEvaluation() error = %v", err)
	}

	if evaluation.ID != "health_evaluation_1" {
		t.Fatalf("evaluation.ID = %q, want generated ID", evaluation.ID)
	}
	if evaluation.Scope != scope {
		t.Fatalf("evaluation.Scope = %+v, want %+v", evaluation.Scope, scope)
	}
	if evaluation.Status != HealthStatusUnhealthy {
		t.Fatalf("evaluation.Status = %q, want unhealthy", evaluation.Status)
	}
	if evaluation.Severity != SeverityCritical {
		t.Fatalf("evaluation.Severity = %q, want critical", evaluation.Severity)
	}
	if len(evaluation.Components) != 11 {
		t.Fatalf("len(evaluation.Components) = %d, want 11", len(evaluation.Components))
	}
	if evaluation.Components[1].Component != ComponentBacklog || evaluation.Components[1].Status != HealthStatusDegraded {
		t.Fatalf("backlog component = %+v, want degraded backlog", evaluation.Components[1])
	}
	if evaluation.Components[9].Component != ComponentCapacityLoad || evaluation.Components[9].Status != HealthStatusUnhealthy {
		t.Fatalf("capacity component = %+v, want unhealthy capacity/load", evaluation.Components[9])
	}
	if !reflect.DeepEqual(runtimeEvidence, map[string]any{"uptime_seconds": 3600}) {
		t.Fatalf("runtimeEvidence mutated: %+v", runtimeEvidence)
	}
	if !reflect.DeepEqual(embeddingEvidence, map[string]any{"index_state": "ready"}) {
		t.Fatalf("embeddingEvidence mutated: %+v", embeddingEvidence)
	}
	if !reflect.DeepEqual(taskEvidence, map[string]any{"total_evaluations": 8}) {
		t.Fatalf("taskEvidence mutated: %+v", taskEvidence)
	}
	if !reflect.DeepEqual(backupEvidence, map[string]any{"marker": "backup-1"}) {
		t.Fatalf("backupEvidence mutated: %+v", backupEvidence)
	}

	if len(store.createdEvaluations) != 1 {
		t.Fatalf("created evaluations = %d, want 1", len(store.createdEvaluations))
	}
	if len(store.createdIncidents) != 2 {
		t.Fatalf("created incidents = %d, want 2", len(store.createdIncidents))
	}
	if store.createdIncidents[0].Scope != scope || store.createdIncidents[1].Scope != scope {
		t.Fatalf("created incidents not scoped: %+v", store.createdIncidents)
	}
	if store.createdIncidents[0].RunbookHints[0] != RunbookHintReviewBacklog {
		t.Fatalf("backlog incident hints = %+v, want backlog hint", store.createdIncidents[0].RunbookHints)
	}
	if store.createdIncidents[1].RunbookHints[0] != RunbookHintReviewCapacityProof {
		t.Fatalf("capacity incident hints = %+v, want capacity hint", store.createdIncidents[1].RunbookHints)
	}
}

func TestServiceDeduplicatesIncidentsByComponentAndReason(t *testing.T) {
	now := time.Date(2026, 7, 12, 15, 0, 0, 0, time.UTC)
	scope := memory.Scope{Tenant: "tenant-a", Project: "project-a", Namespace: "namespace-a"}
	existing := Incident{
		ID:               "incident_1",
		Scope:            scope,
		Status:           IncidentStatusOpen,
		Severity:         SeverityWarning,
		Component:        ComponentBacklog,
		Reason:           ReasonBacklogPressure,
		DeduplicationKey: "backlog:backlog_pressure",
		OpenedAt:         now.Add(-time.Hour),
		UpdatedAt:        now.Add(-time.Hour),
	}
	store := &stubAssuranceStore{
		incidents: []Incident{existing},
	}
	service := NewService(ServiceOptions{
		Store: store,
		Now:   func() time.Time { return now },
		NewID: func(prefix string) string { return prefix + "_1" },
	})

	_, err := service.CreateHealthEvaluation(context.Background(), HealthEvaluationInput{
		Scope:      scope,
		ObservedAt: now,
		BacklogState: HealthObservation{
			Status:     HealthStatusDegraded,
			Severity:   SeverityWarning,
			Reason:     ReasonBacklogPressure,
			Evidence:   map[string]any{"depth": 99},
			ObservedAt: now,
		},
	})
	if err != nil {
		t.Fatalf("CreateHealthEvaluation() error = %v", err)
	}

	if len(store.createdIncidents) != 0 {
		t.Fatalf("created incidents = %d, want 0 due to deduplication", len(store.createdIncidents))
	}
	if len(store.createdEvaluations) != 1 {
		t.Fatalf("created evaluations = %d, want 1", len(store.createdEvaluations))
	}
}

func TestServiceReportsHealthyWhenObservedSourcesAreHealthy(t *testing.T) {
	now := time.Date(2026, 7, 12, 16, 0, 0, 0, time.UTC)
	scope := memory.Scope{Tenant: "tenant-a", Project: "project-a", Namespace: "namespace-a"}
	service := NewService(ServiceOptions{
		Store: &stubAssuranceStore{},
		Now:   func() time.Time { return now },
		NewID: func(prefix string) string { return prefix + "_1" },
	})

	evaluation, err := service.CreateHealthEvaluation(context.Background(), HealthEvaluationInput{
		Scope:      scope,
		ObservedAt: now,
		RuntimeReadiness: HealthObservation{
			Status:     HealthStatusHealthy,
			Severity:   SeverityInfo,
			Reason:     ReasonRuntimeReady,
			Evidence:   map[string]any{"ready": true},
			ObservedAt: now,
		},
		BackupRestoreProof: HealthObservation{
			Status:       HealthStatusHealthy,
			Severity:     SeverityInfo,
			Reason:       ReasonBackupRestoreFresh,
			Evidence:     map[string]any{"marker": "restore-1"},
			ObservedAt:   now,
			FreshThrough: now.Add(time.Hour),
		},
	})
	if err != nil {
		t.Fatalf("CreateHealthEvaluation() error = %v", err)
	}

	if evaluation.Status != HealthStatusHealthy {
		t.Fatalf("evaluation.Status = %q, want healthy", evaluation.Status)
	}
	if evaluation.Severity != SeverityInfo {
		t.Fatalf("evaluation.Severity = %q, want info", evaluation.Severity)
	}
}

func TestServiceUsesEvaluationScopedComponentIDs(t *testing.T) {
	now := time.Date(2026, 7, 12, 17, 0, 0, 0, time.UTC)
	scope := memory.Scope{Tenant: "tenant-a", Project: "project-a", Namespace: "namespace-a"}
	service := NewService(ServiceOptions{
		Store: &stubAssuranceStore{},
		Now:   func() time.Time { return now },
		NewID: func(prefix string) string { return prefix + "_1" },
	})

	first, err := service.CreateHealthEvaluation(context.Background(), HealthEvaluationInput{
		Scope:        scope,
		EvaluationID: "health_a",
		ObservedAt:   now,
		BacklogState: HealthObservation{
			Status:     HealthStatusHealthy,
			Severity:   SeverityInfo,
			Reason:     ReasonRuntimeReady,
			Evidence:   map[string]any{"depth": 0},
			ObservedAt: now,
		},
	})
	if err != nil {
		t.Fatalf("CreateHealthEvaluation(first) error = %v", err)
	}
	second, err := service.CreateHealthEvaluation(context.Background(), HealthEvaluationInput{
		Scope:        scope,
		EvaluationID: "health_b",
		ObservedAt:   now.Add(time.Minute),
		BacklogState: HealthObservation{
			Status:     HealthStatusHealthy,
			Severity:   SeverityInfo,
			Reason:     ReasonRuntimeReady,
			Evidence:   map[string]any{"depth": 0},
			ObservedAt: now.Add(time.Minute),
		},
	})
	if err != nil {
		t.Fatalf("CreateHealthEvaluation(second) error = %v", err)
	}

	if first.Components[0].ID == second.Components[0].ID {
		t.Fatalf("component IDs should be evaluation-scoped, both were %q", first.Components[0].ID)
	}
}

func TestServiceMarksReadinessDegradedWhenOperationalProofsFail(t *testing.T) {
	now := time.Date(2026, 7, 12, 17, 30, 0, 0, time.UTC)
	scope := memory.Scope{Tenant: "tenant-a", Project: "project-a", Namespace: "namespace-a"}
	service := NewService(ServiceOptions{
		Store: &stubAssuranceStore{},
		Now:   func() time.Time { return now },
		NewID: func(prefix string) string { return prefix + "_1" },
	})

	evaluation, err := service.CreateHealthEvaluation(context.Background(), HealthEvaluationInput{
		Scope:      scope,
		ObservedAt: now,
		CapacityLoadProof: HealthObservation{
			Status:     HealthStatusUnhealthy,
			Severity:   SeverityCritical,
			Reason:     ReasonCapacityThresholdExceeded,
			Evidence:   map[string]any{"worker_latency_ms": 1000},
			ObservedAt: now,
		},
		BackupRestoreProof: HealthObservation{
			Status:       HealthStatusStale,
			Severity:     SeverityWarning,
			Reason:       ReasonBackupRestoreStale,
			Evidence:     map[string]any{"marker": "restore-1"},
			ObservedAt:   now.Add(-24 * time.Hour),
			FreshThrough: now.Add(-time.Hour),
		},
	})
	if err != nil {
		t.Fatalf("CreateHealthEvaluation() error = %v", err)
	}

	if evaluation.Status == HealthStatusHealthy {
		t.Fatalf("evaluation.Status = %q, want degraded or worse", evaluation.Status)
	}
	if evaluation.Components[0].Component != ComponentCapacityLoad || evaluation.Components[0].Status != HealthStatusUnhealthy {
		t.Fatalf("capacity component = %+v, want unhealthy capacity/load", evaluation.Components[0])
	}
	if evaluation.Components[1].Component != ComponentBackupRestore || evaluation.Components[1].Status != HealthStatusStale {
		t.Fatalf("backup component = %+v, want stale backup/restore", evaluation.Components[1])
	}
}

func TestServiceAppliesIncidentLifecycleActionsWithTransitionHistory(t *testing.T) {
	now := time.Date(2026, 7, 12, 18, 0, 0, 0, time.UTC)
	scope := memory.Scope{Tenant: "tenant-a", Project: "project-a", Namespace: "namespace-a"}
	store := &stubAssuranceStore{
		incidents: []Incident{{
			ID:               "incident_1",
			Scope:            scope,
			Status:           IncidentStatusOpen,
			Severity:         SeverityWarning,
			Component:        ComponentBacklog,
			Reason:           ReasonBacklogPressure,
			DeduplicationKey: "backlog:backlog_pressure",
			OpenedAt:         now.Add(-time.Hour),
			UpdatedAt:        now.Add(-time.Hour),
		}},
	}
	service := NewService(ServiceOptions{
		Store: store,
		Now:   func() time.Time { return now },
		NewID: func(prefix string) string { return prefix + "_1" },
	})

	cases := []struct {
		action IncidentAction
		from   IncidentStatus
		to     IncidentStatus
	}{
		{action: IncidentActionAcknowledge, from: IncidentStatusOpen, to: IncidentStatusAcknowledged},
		{action: IncidentActionSuppress, from: IncidentStatusAcknowledged, to: IncidentStatusSuppressed},
		{action: IncidentActionResolve, from: IncidentStatusSuppressed, to: IncidentStatusResolved},
		{action: IncidentActionReopen, from: IncidentStatusResolved, to: IncidentStatusOpen},
		{action: IncidentActionVerify, from: IncidentStatusOpen, to: IncidentStatusOpen},
	}
	for _, tc := range cases {
		updated, err := service.ApplyIncidentAction(context.Background(), IncidentActionInput{
			Scope:      scope,
			IncidentID: "incident_1",
			Action:     tc.action,
			Actor:      "admin-a",
			Reason:     "operator action",
			OccurredAt: now,
		})
		if err != nil {
			t.Fatalf("ApplyIncidentAction(%s) error = %v", tc.action, err)
		}
		if updated.Status != tc.to {
			t.Fatalf("ApplyIncidentAction(%s) status = %q, want %q", tc.action, updated.Status, tc.to)
		}
		got := store.transitions[len(store.transitions)-1]
		if got.IncidentID != "incident_1" || got.Scope != scope {
			t.Fatalf("transition scope/id = %+v, want scoped incident transition", got)
		}
		if got.FromStatus != tc.from || got.ToStatus != tc.to || got.Action != tc.action {
			t.Fatalf("transition = %+v, want from %q to %q action %q", got, tc.from, tc.to, tc.action)
		}
		if got.Actor != "admin-a" || got.Reason != "operator action" || !got.OccurredAt.Equal(now) {
			t.Fatalf("transition attribution = %+v, want actor/reason/occurred_at", got)
		}
	}
}

func TestServiceRejectsIncidentActionWithoutActorReason(t *testing.T) {
	now := time.Date(2026, 7, 12, 19, 0, 0, 0, time.UTC)
	scope := memory.Scope{Tenant: "tenant-a", Project: "project-a", Namespace: "namespace-a"}
	service := NewService(ServiceOptions{
		Store: &stubAssuranceStore{
			incidents: []Incident{{
				ID:               "incident_1",
				Scope:            scope,
				Status:           IncidentStatusOpen,
				Severity:         SeverityWarning,
				Component:        ComponentBacklog,
				Reason:           ReasonBacklogPressure,
				DeduplicationKey: "backlog:backlog_pressure",
				OpenedAt:         now,
				UpdatedAt:        now,
			}},
		},
		Now:   func() time.Time { return now },
		NewID: func(prefix string) string { return prefix + "_1" },
	})

	_, err := service.ApplyIncidentAction(context.Background(), IncidentActionInput{
		Scope:      scope,
		IncidentID: "incident_1",
		Action:     IncidentActionAcknowledge,
		Reason:     "missing actor",
		OccurredAt: now,
	})
	if err == nil {
		t.Fatal("ApplyIncidentAction() error = nil, want actor validation error")
	}
}

func TestServiceGeneratesAlertCandidatesFromIncidentsAndCriticalEvaluation(t *testing.T) {
	now := time.Date(2026, 7, 12, 20, 0, 0, 0, time.UTC)
	scope := memory.Scope{Tenant: "tenant-a", Project: "project-a", Namespace: "namespace-a"}
	store := &stubAssuranceStore{
		incidents: []Incident{
			{
				ID:               "incident_1",
				Scope:            scope,
				Status:           IncidentStatusOpen,
				Severity:         SeverityCritical,
				Component:        ComponentBackupRestore,
				Reason:           ReasonBackupRestoreStale,
				DeduplicationKey: "backup_restore:backup_restore_stale",
				OpenedAt:         now.Add(-time.Hour),
				UpdatedAt:        now.Add(-time.Hour),
			},
			{
				ID:               "incident_resolved",
				Scope:            scope,
				Status:           IncidentStatusResolved,
				Severity:         SeverityCritical,
				Component:        ComponentCapacityLoad,
				Reason:           ReasonCapacityThresholdExceeded,
				DeduplicationKey: "capacity_load:capacity_threshold_exceeded",
				OpenedAt:         now.Add(-time.Hour),
				UpdatedAt:        now.Add(-time.Minute),
				ResolvedAt:       now.Add(-time.Minute),
			},
		},
	}
	service := NewService(ServiceOptions{
		Store: store,
		Now:   func() time.Time { return now },
		NewID: func(prefix string) string { return prefix + "_1" },
	})

	candidates, err := service.GenerateAlertCandidates(context.Background(), AlertCandidateGenerationInput{
		Scope:               scope,
		DeliveryPolicy:      "default",
		DeduplicationWindow: time.Hour,
		CreatedAt:           now,
		Evaluation: HealthEvaluation{
			ID:        "health_1",
			Scope:     scope,
			Status:    HealthStatusUnhealthy,
			Severity:  SeverityCritical,
			Reason:    ReasonCapacityThresholdExceeded,
			CreatedAt: now,
			Components: []HealthComponentSummary{
				{
					ID:           "health_1:capacity_load",
					EvaluationID: "health_1",
					Scope:        scope,
					Component:    ComponentCapacityLoad,
					Status:       HealthStatusUnhealthy,
					Severity:     SeverityCritical,
					Reason:       ReasonCapacityThresholdExceeded,
					ObservedAt:   now,
					Evidence:     map[string]any{"raw_counter": 999},
				},
				{
					ID:           "health_1:backlog",
					EvaluationID: "health_1",
					Scope:        scope,
					Component:    ComponentBacklog,
					Status:       HealthStatusDegraded,
					Severity:     SeverityWarning,
					Reason:       ReasonBacklogPressure,
					ObservedAt:   now,
				},
			},
		},
	})
	if err != nil {
		t.Fatalf("GenerateAlertCandidates() error = %v", err)
	}

	if len(candidates) != 2 {
		t.Fatalf("len(candidates) = %d, want 2", len(candidates))
	}
	if candidates[0].IncidentID != "incident_1" || candidates[0].EvaluationID != "" {
		t.Fatalf("incident candidate = %+v, want incident-linked candidate", candidates[0])
	}
	if candidates[1].IncidentID != "" || candidates[1].EvaluationID != "health_1" {
		t.Fatalf("evaluation candidate = %+v, want evaluation-linked candidate", candidates[1])
	}
	for _, candidate := range candidates {
		if candidate.Scope != scope || candidate.DeliveryPolicy != "default" || !candidate.NextAttemptAt.Equal(now) {
			t.Fatalf("candidate = %+v, want scoped default delivery candidate", candidate)
		}
		if _, ok := candidate.Payload["raw_counter"]; ok {
			t.Fatalf("candidate payload leaked raw evidence: %+v", candidate.Payload)
		}
		if candidate.Payload["admin_surface"] == "" || candidate.Payload["runbook_hint"] == "" {
			t.Fatalf("candidate payload missing bounded admin guidance: %+v", candidate.Payload)
		}
	}
	if len(store.createdAlertCandidates) != 2 {
		t.Fatalf("created alert candidates = %d, want 2", len(store.createdAlertCandidates))
	}
}

func TestServiceDeduplicatesAlertCandidatesWithinWindow(t *testing.T) {
	now := time.Date(2026, 7, 12, 21, 0, 0, 0, time.UTC)
	scope := memory.Scope{Tenant: "tenant-a", Project: "project-a", Namespace: "namespace-a"}
	store := &stubAssuranceStore{
		incidents: []Incident{{
			ID:               "incident_1",
			Scope:            scope,
			Status:           IncidentStatusOpen,
			Severity:         SeverityCritical,
			Component:        ComponentBackupRestore,
			Reason:           ReasonBackupRestoreStale,
			DeduplicationKey: "backup_restore:backup_restore_stale",
			OpenedAt:         now.Add(-time.Hour),
			UpdatedAt:        now.Add(-time.Hour),
		}},
		alertCandidates: []AlertCandidate{{
			ID:               "alert_existing",
			Scope:            scope,
			IncidentID:       "incident_1",
			Severity:         SeverityCritical,
			Component:        ComponentBackupRestore,
			Reason:           ReasonBackupRestoreStale,
			DeduplicationKey: "incident:incident_1:backup_restore:backup_restore_stale",
			DeliveryPolicy:   "default",
			Payload:          map[string]any{"component": "backup_restore"},
			CreatedAt:        now.Add(-30 * time.Minute),
			NextAttemptAt:    now.Add(-30 * time.Minute),
		}},
	}
	service := NewService(ServiceOptions{
		Store: store,
		Now:   func() time.Time { return now },
		NewID: func(prefix string) string { return prefix + "_1" },
	})

	candidates, err := service.GenerateAlertCandidates(context.Background(), AlertCandidateGenerationInput{
		Scope:               scope,
		DeliveryPolicy:      "default",
		DeduplicationWindow: time.Hour,
		CreatedAt:           now,
	})
	if err != nil {
		t.Fatalf("GenerateAlertCandidates() error = %v", err)
	}

	if len(candidates) != 0 {
		t.Fatalf("len(candidates) = %d, want 0 due to deduplication", len(candidates))
	}
	if len(store.createdAlertCandidates) != 0 {
		t.Fatalf("created alert candidates = %d, want 0", len(store.createdAlertCandidates))
	}
}

func TestServiceDeliversAlertCandidatesForDisabledStdoutAndWebhookAdapters(t *testing.T) {
	now := time.Date(2026, 7, 12, 22, 0, 0, 0, time.UTC)
	scope := memory.Scope{Tenant: "tenant-a", Project: "project-a", Namespace: "namespace-a"}
	store := &stubAssuranceStore{}
	service := NewService(ServiceOptions{
		Store: store,
		Now:   func() time.Time { return now },
		NewID: func(prefix string) string { return prefix + "_1" },
	})
	candidate := AlertCandidate{
		ID:               "alert_1",
		Scope:            scope,
		Severity:         SeverityCritical,
		Component:        ComponentCapacityLoad,
		Reason:           ReasonCapacityThresholdExceeded,
		DeduplicationKey: "evaluation:health_1:capacity_load:capacity_threshold_exceeded",
		DeliveryPolicy:   "default",
		Payload:          map[string]any{"component": "capacity_load", "severity": "critical"},
		CreatedAt:        now,
		NextAttemptAt:    now,
	}

	disabledAttempts, err := service.DeliverAlertCandidate(context.Background(), AlertDeliveryInput{
		Scope:       scope,
		Candidate:   candidate,
		Config:      AlertDeliveryConfig{Mode: AlertAdapterDisabled},
		MaxAttempts: 1,
		WorkerID:    "worker-1",
		Now:         now,
	})
	if err != nil {
		t.Fatalf("DeliverAlertCandidate(disabled) error = %v", err)
	}
	if len(disabledAttempts) != 1 || disabledAttempts[0].Result != AlertDeliveryResultDisabled {
		t.Fatalf("disabledAttempts = %+v, want disabled attempt", disabledAttempts)
	}

	var stdout strings.Builder
	stdoutAttempts, err := service.DeliverAlertCandidate(context.Background(), AlertDeliveryInput{
		Scope:       scope,
		Candidate:   candidate,
		Config:      AlertDeliveryConfig{Mode: AlertAdapterStdout},
		MaxAttempts: 1,
		WorkerID:    "worker-1",
		Now:         now,
		Output:      &stdout,
	})
	if err != nil {
		t.Fatalf("DeliverAlertCandidate(stdout) error = %v", err)
	}
	if len(stdoutAttempts) != 1 || stdoutAttempts[0].Result != AlertDeliveryResultSuccess {
		t.Fatalf("stdoutAttempts = %+v, want stdout success", stdoutAttempts)
	}
	if !strings.Contains(stdout.String(), "capacity_load") {
		t.Fatalf("stdout output = %q, want bounded payload", stdout.String())
	}

	webhookStore := &stubAssuranceStore{}
	webhookService := NewService(ServiceOptions{
		Store: webhookStore,
		Now:   func() time.Time { return now },
		NewID: func(prefix string) string { return prefix + "_1" },
	})
	requestCount := 0
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestCount++
		if r.Header.Get("X-Alert-Token") != "token-1" {
			t.Fatalf("header = %q, want secret header forwarded", r.Header.Get("X-Alert-Token"))
		}
		body, _ := io.ReadAll(r.Body)
		if strings.Contains(string(body), "secret") {
			t.Fatalf("webhook body leaked secret material: %s", body)
		}
		if !strings.Contains(string(body), "capacity_load") {
			t.Fatalf("webhook body = %s, want bounded payload", body)
		}
		if requestCount == 1 {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	webhookAttempts, err := webhookService.DeliverAlertCandidate(context.Background(), AlertDeliveryInput{
		Scope:     scope,
		Candidate: candidate,
		Config: AlertDeliveryConfig{
			Mode:            AlertAdapterWebhook,
			WebhookURL:      server.URL,
			WebhookHeaders:  map[string]string{"X-Alert-Token": "token-1"},
			Timeout:         5 * time.Second,
			MaxPayloadBytes: 4096,
		},
		MaxAttempts:  2,
		RetryBackoff: time.Second,
		WorkerID:     "worker-1",
		Now:          now,
		HTTPClient:   server.Client(),
	})
	if err != nil {
		t.Fatalf("DeliverAlertCandidate(webhook) error = %v", err)
	}
	if len(webhookAttempts) != 2 {
		t.Fatalf("len(webhookAttempts) = %d, want 2 attempts with retry", len(webhookAttempts))
	}
	if webhookAttempts[0].Result != AlertDeliveryResultRetry || webhookAttempts[1].Result != AlertDeliveryResultSuccess {
		t.Fatalf("webhookAttempts = %+v, want retry then success", webhookAttempts)
	}
	if len(webhookStore.createdAlertDeliveryAttempts) != 2 {
		t.Fatalf("created alert delivery attempts = %d, want 2", len(webhookStore.createdAlertDeliveryAttempts))
	}
}

func TestServiceRejectsOversizedWebhookPayload(t *testing.T) {
	now := time.Date(2026, 7, 12, 23, 0, 0, 0, time.UTC)
	scope := memory.Scope{Tenant: "tenant-a", Project: "project-a", Namespace: "namespace-a"}
	service := NewService(ServiceOptions{
		Store: &stubAssuranceStore{},
		Now:   func() time.Time { return now },
		NewID: func(prefix string) string { return prefix + "_1" },
	})
	payload := map[string]any{"content": strings.Repeat("x", 1024)}
	_, err := service.DeliverAlertCandidate(context.Background(), AlertDeliveryInput{
		Scope: scope,
		Candidate: AlertCandidate{
			ID:               "alert_1",
			Scope:            scope,
			Severity:         SeverityCritical,
			Component:        ComponentCapacityLoad,
			Reason:           ReasonCapacityThresholdExceeded,
			DeduplicationKey: "evaluation:health_1:capacity_load:capacity_threshold_exceeded",
			DeliveryPolicy:   "default",
			Payload:          payload,
			CreatedAt:        now,
		},
		Config: AlertDeliveryConfig{
			Mode:            AlertAdapterWebhook,
			WebhookURL:      "https://example.com/webhook",
			Timeout:         5 * time.Second,
			MaxPayloadBytes: 128,
		},
		MaxAttempts: 1,
		WorkerID:    "worker-1",
		Now:         now,
	})
	if err == nil {
		t.Fatal("DeliverAlertCandidate() error = nil, want oversized payload error")
	}
}

func TestServiceCRUDsConformanceProfiles(t *testing.T) {
	now := time.Date(2026, 7, 13, 0, 0, 0, 0, time.UTC)
	scope := memory.Scope{Tenant: "tenant-a", Project: "project-a", Namespace: "namespace-a"}
	store := &stubAssuranceStore{
		conformanceProfiles: []ConformanceProfile{{
			ID:     "profile_1",
			Scope:  scope,
			Status: ConformanceProfileStatusActive,
			ExpectedEvidence: []ExpectedEvidence{
				{Kind: ExpectedEvidenceSession, MinimumCount: 1, FreshnessWindow: time.Hour},
			},
			Actor:     "admin-a",
			Reason:    "baseline",
			CreatedAt: now,
			UpdatedAt: now,
		}},
	}
	service := NewService(ServiceOptions{
		Store: store,
		Now:   func() time.Time { return now },
		NewID: func(prefix string) string { return prefix + "_1" },
	})

	created, err := service.CreateConformanceProfile(context.Background(), ConformanceProfile{
		ID:     "profile_2",
		Scope:  scope,
		Status: ConformanceProfileStatusActive,
		ExpectedEvidence: []ExpectedEvidence{
			{Kind: ExpectedEvidenceContext, MinimumCount: 1, FreshnessWindow: time.Hour},
			{Kind: ExpectedEvidenceSession, MinimumCount: 1, FreshnessWindow: time.Hour},
			{Kind: ExpectedEvidenceOutcome, MinimumCount: 1, FreshnessWindow: time.Hour},
			{Kind: ExpectedEvidenceVerification, MinimumCount: 1, FreshnessWindow: time.Hour},
			{Kind: ExpectedEvidenceUsefulnessFeedback, MinimumCount: 1, FreshnessWindow: time.Hour},
			{Kind: ExpectedEvidenceTaskEvaluation, MinimumCount: 1, FreshnessWindow: time.Hour},
			{Kind: ExpectedEvidenceProof, MinimumCount: 1, FreshnessWindow: time.Hour},
			{Kind: ExpectedEvidenceRepair, MinimumCount: 1, FreshnessWindow: time.Hour},
			{Kind: ExpectedEvidenceRankingRollout, MinimumCount: 1, FreshnessWindow: time.Hour},
		},
		Actor:     "admin-a",
		Reason:    "create profile",
		CreatedAt: now,
		UpdatedAt: now,
	})
	if err != nil {
		t.Fatalf("CreateConformanceProfile() error = %v", err)
	}
	if created.ID != "profile_2" || created.Scope != scope {
		t.Fatalf("created = %+v, want scoped profile", created)
	}

	updated, err := service.UpdateConformanceProfile(context.Background(), UpdateConformanceProfileInput{
		Scope:     scope,
		ProfileID: "profile_2",
		ExpectedEvidence: []ExpectedEvidence{
			{Kind: ExpectedEvidenceContext, MinimumCount: 2, FreshnessWindow: 2 * time.Hour},
		},
		Actor:     "admin-b",
		Reason:    "expand evidence",
		UpdatedAt: now.Add(time.Minute),
	})
	if err != nil {
		t.Fatalf("UpdateConformanceProfile() error = %v", err)
	}
	if updated.Actor != "admin-b" || updated.Reason != "expand evidence" || updated.ExpectedEvidence[0].MinimumCount != 2 {
		t.Fatalf("updated = %+v, want updated profile", updated)
	}

	disabled, err := service.DisableConformanceProfile(context.Background(), DisableConformanceProfileInput{
		Scope:      scope,
		ProfileID:  "profile_2",
		Actor:      "admin-c",
		Reason:     "disabled after rollout",
		DisabledAt: now.Add(2 * time.Minute),
	})
	if err != nil {
		t.Fatalf("DisableConformanceProfile() error = %v", err)
	}
	if disabled.Status != ConformanceProfileStatusDisabled || disabled.DisabledAt.IsZero() {
		t.Fatalf("disabled = %+v, want disabled profile with timestamp", disabled)
	}

	read, err := service.ReadConformanceProfile(context.Background(), ReadConformanceProfileInput{Scope: scope, ProfileID: "profile_2"})
	if err != nil {
		t.Fatalf("ReadConformanceProfile() error = %v", err)
	}
	if read.ID != "profile_2" || read.DisabledAt.IsZero() {
		t.Fatalf("read = %+v, want disabled profile", read)
	}

	listed, err := service.ListConformanceProfiles(context.Background(), ListConformanceProfilesInput{Scope: scope, Status: ConformanceProfileStatusDisabled})
	if err != nil {
		t.Fatalf("ListConformanceProfiles() error = %v", err)
	}
	if len(listed) != 1 || listed[0].ID != "profile_2" {
		t.Fatalf("listed = %+v, want disabled profile", listed)
	}
}

func TestServiceRunsConformanceAgainstDurableEvidence(t *testing.T) {
	now := time.Date(2026, 7, 13, 1, 0, 0, 0, time.UTC)
	scope := memory.Scope{Tenant: "tenant-a", Project: "project-a", Namespace: "namespace-a"}
	profile := ConformanceProfile{
		ID:     "profile_1",
		Scope:  scope,
		Status: ConformanceProfileStatusActive,
		ExpectedEvidence: []ExpectedEvidence{
			{Kind: ExpectedEvidenceSession, MinimumCount: 1, FreshnessWindow: time.Hour},
			{Kind: ExpectedEvidenceContext, MinimumCount: 1, FreshnessWindow: time.Hour},
			{Kind: ExpectedEvidenceOutcome, MinimumCount: 1, FreshnessWindow: time.Hour},
			{Kind: ExpectedEvidenceVerification, MinimumCount: 1, FreshnessWindow: time.Hour},
			{Kind: ExpectedEvidenceUsefulnessFeedback, MinimumCount: 1, FreshnessWindow: time.Hour},
			{Kind: ExpectedEvidenceTaskEvaluation, MinimumCount: 1, FreshnessWindow: time.Hour},
			{Kind: ExpectedEvidenceProof, MinimumCount: 1, FreshnessWindow: time.Hour},
			{Kind: ExpectedEvidenceRepair, MinimumCount: 1, FreshnessWindow: time.Hour},
			{Kind: ExpectedEvidenceRankingRollout, MinimumCount: 1, FreshnessWindow: time.Hour},
		},
		Actor:     "admin-a",
		Reason:    "full integration profile",
		CreatedAt: now.Add(-time.Hour),
		UpdatedAt: now.Add(-time.Hour),
	}
	store := &stubAssuranceStore{
		conformanceProfiles: []ConformanceProfile{profile},
		conformanceEvidence: []ConformanceEvidenceObservation{
			{Kind: ExpectedEvidenceSession, Count: 1, FreshestAt: now.Add(-time.Minute)},
			{Kind: ExpectedEvidenceContext, Count: 1, FreshestAt: now.Add(-time.Minute)},
			{Kind: ExpectedEvidenceOutcome, Count: 1, FreshestAt: now.Add(-time.Minute)},
			{Kind: ExpectedEvidenceVerification, Count: 1, FreshestAt: now.Add(-time.Minute)},
			{Kind: ExpectedEvidenceUsefulnessFeedback, Count: 1, FreshestAt: now.Add(-time.Minute)},
			{Kind: ExpectedEvidenceTaskEvaluation, Count: 1, FreshestAt: now.Add(-time.Minute)},
			{Kind: ExpectedEvidenceProof, Count: 1, FreshestAt: now.Add(-time.Minute)},
			{Kind: ExpectedEvidenceRepair, Count: 1, FreshestAt: now.Add(-time.Minute)},
			{Kind: ExpectedEvidenceRankingRollout, Count: 1, FreshestAt: now.Add(-time.Minute)},
		},
	}
	service := NewService(ServiceOptions{
		Store: store,
		Now:   func() time.Time { return now },
		NewID: func(prefix string) string { return prefix + "_1" },
	})

	run, diagnostics, err := service.RunConformance(context.Background(), ConformanceRunInput{
		Scope:     scope,
		ProfileID: profile.ID,
		RunID:     "run_1",
		StartedAt: now,
	})
	if err != nil {
		t.Fatalf("RunConformance() error = %v", err)
	}
	if run.Result != ConformanceResultPassed || run.Scope != scope || run.ProfileID != profile.ID {
		t.Fatalf("run = %+v, want passed scoped conformance run", run)
	}
	if len(diagnostics) != 0 {
		t.Fatalf("diagnostics = %+v, want none for complete evidence", diagnostics)
	}
	if store.inspectInputs[0].Scope != scope || len(store.inspectInputs[0].ExpectedEvidence) != len(profile.ExpectedEvidence) {
		t.Fatalf("inspect input = %+v, want scoped expected evidence inspection", store.inspectInputs[0])
	}
	if len(store.createdConformanceRuns) != 1 || len(store.createdMissingEvidenceDiagnostics) != 0 {
		t.Fatalf("created runs=%d diagnostics=%d, want one run and no diagnostics", len(store.createdConformanceRuns), len(store.createdMissingEvidenceDiagnostics))
	}
}

func TestServiceWorkflowEvidenceDegradesConformanceAndReadiness(t *testing.T) {
	now := time.Date(2026, 7, 18, 12, 0, 0, 0, time.UTC)
	scope := memory.Scope{Tenant: "tenant-a", Project: "project-a", Namespace: "namespace-a"}
	profile := ConformanceProfile{
		ID:     "workflow_profile_1",
		Scope:  scope,
		Status: ConformanceProfileStatusActive,
		ExpectedEvidence: []ExpectedEvidence{
			{Kind: ExpectedEvidenceWorkflow, MinimumCount: 1, FreshnessWindow: time.Hour},
		},
		Actor: "operator-a", Reason: "require integration workflow", CreatedAt: now, UpdatedAt: now,
	}
	store := &stubAssuranceStore{
		conformanceProfiles: []ConformanceProfile{profile},
		conformanceEvidence: []ConformanceEvidenceObservation{{
			Kind: ExpectedEvidenceWorkflow, Count: 1, FreshestAt: now.Add(-time.Minute), Contradictory: true,
		}},
		healthEvaluations: []HealthEvaluation{{ID: "health_1", Scope: scope, Status: HealthStatusHealthy, Severity: SeverityInfo, Reason: ReasonRuntimeReady, CreatedAt: now}},
		operationalProofs: []OperationalProof{
			{ID: "capacity_1", Scope: scope, Target: OperationalProofCapacityLoad, Status: HealthStatusHealthy, Severity: SeverityInfo, Reason: ReasonCapacityWithinThresholds, ObservedAt: now, CreatedAt: now},
			{ID: "backup_1", Scope: scope, Target: OperationalProofBackupRestore, Status: HealthStatusHealthy, Severity: SeverityInfo, Reason: ReasonBackupRestoreFresh, ObservedAt: now, CreatedAt: now},
		},
		workflowHealth: WorkflowHealthSnapshot{Scope: scope, CompletedRuns: 1, BlockingDiagnostics: 1, LatestObservedAt: now.Add(-time.Minute), Status: HealthStatusDegraded, Reason: ReasonWorkflowGap},
	}
	service := NewService(ServiceOptions{
		Store: store,
		Workflow: workflowHealthReaderFunc(func(ctx context.Context, gotScope memory.Scope, observedAt time.Time) (WorkflowHealthSnapshot, error) {
			if gotScope != scope {
				t.Fatalf("workflow health scope = %+v, want %+v", gotScope, scope)
			}
			return store.workflowHealth, nil
		}),
		Now: func() time.Time { return now }, NewID: func(prefix string) string { return prefix + "_1" },
	})

	run, diagnostics, err := service.RunConformance(context.Background(), ConformanceRunInput{Scope: scope, ProfileID: profile.ID, RunID: "workflow_run_1", StartedAt: now})
	if err != nil {
		t.Fatalf("RunConformance() error = %v", err)
	}
	if run.Result != ConformanceResultDegraded || len(diagnostics) != 1 || diagnostics[0].EvidenceKind != ExpectedEvidenceWorkflow {
		t.Fatalf("workflow conformance = %+v diagnostics=%+v, want degraded workflow diagnostic", run, diagnostics)
	}

	report, err := service.CreateReadinessReport(context.Background(), ReadinessReportInput{Scope: scope, ReportID: "readiness_1", GeneratedAt: now})
	if err != nil {
		t.Fatalf("CreateReadinessReport() error = %v", err)
	}
	if report.Status != ReadinessStatusDegraded || report.ComponentSummary["workflow_status"] != string(HealthStatusDegraded) {
		t.Fatalf("readiness report = %+v, want degraded workflow summary", report)
	}
	if !containsRunbookHint(report.RecommendedActions, RunbookHintReviewWorkflow) {
		t.Fatalf("recommended actions = %+v, want workflow runbook action", report.RecommendedActions)
	}
}

func TestServiceWorkflowHealthCreatesDeduplicatedIncidentAndAlertCandidate(t *testing.T) {
	now := time.Date(2026, 7, 18, 13, 0, 0, 0, time.UTC)
	scope := memory.Scope{Tenant: "tenant-a", Project: "project-a", Namespace: "namespace-a"}
	store := &stubAssuranceStore{}
	service := NewService(ServiceOptions{Store: store, Now: func() time.Time { return now }, NewID: func(prefix string) string { return prefix + "_1" }})
	input := HealthEvaluationInput{
		Scope: scope, EvaluationID: "health_1", ObservedAt: now,
		WorkflowHealth: HealthObservation{Status: HealthStatusUnhealthy, Severity: SeverityCritical, Reason: ReasonWorkflowGap, ObservedAt: now, Evidence: map[string]any{"blocking_diagnostics": 2, "workflow_category": "missing"}},
	}
	evaluation, err := service.CreateHealthEvaluation(context.Background(), input)
	if err != nil {
		t.Fatalf("CreateHealthEvaluation() error = %v", err)
	}
	if componentStatus(evaluation, ComponentWorkflow) != HealthStatusUnhealthy || len(store.createdIncidents) != 1 {
		t.Fatalf("workflow health evaluation = %+v incidents=%+v, want workflow incident", evaluation, store.createdIncidents)
	}
	if store.createdIncidents[0].Component != ComponentWorkflow || store.createdIncidents[0].Reason != ReasonWorkflowGap {
		t.Fatalf("workflow incident = %+v, want bounded workflow component and reason", store.createdIncidents[0])
	}
	candidates, err := service.GenerateAlertCandidates(context.Background(), AlertCandidateGenerationInput{Scope: scope, Evaluation: evaluation, DeliveryPolicy: "disabled", DeduplicationWindow: time.Hour, CreatedAt: now})
	if err != nil {
		t.Fatalf("GenerateAlertCandidates() error = %v", err)
	}
	if len(candidates) == 0 || candidates[0].Component != ComponentWorkflow {
		t.Fatalf("workflow alert candidates = %+v, want workflow alert", candidates)
	}
}

func TestServiceRecoveryVerificationAcceptsBoundedWorkflowEvidence(t *testing.T) {
	now := time.Date(2026, 7, 18, 14, 0, 0, 0, time.UTC)
	scope := memory.Scope{Tenant: "tenant-a", Project: "project-a", Namespace: "namespace-a"}
	service := NewService(ServiceOptions{Store: &stubAssuranceStore{}, Now: func() time.Time { return now }, NewID: func(prefix string) string { return prefix + "_1" }})
	record, err := service.CreateRecoveryVerification(context.Background(), RecoveryVerificationInput{
		Scope: scope, RecordID: "recovery_1", Target: RecoveryVerificationTargetWorkflowRun, TargetID: "workflow_run_1", Status: HealthStatusHealthy,
		CheckedSurfaces: []string{"workflow_run", "workflow_diagnostic", "conformance_run"}, ResultCategory: "recovered",
		LinkedEvidence: map[string]any{"workflow_status": "completed", "workflow_gap_category": "missing", "conformance_result": "passed"},
		Actor:          "operator-a", Reason: "workflow recovery verified", VerifiedAt: now,
	})
	if err != nil {
		t.Fatalf("CreateRecoveryVerification() error = %v", err)
	}
	if record.Target != RecoveryVerificationTargetWorkflowRun || record.LinkedEvidence["workflow_status"] != "completed" {
		t.Fatalf("recovery verification = %+v, want bounded workflow evidence", record)
	}
}

func TestServiceConformanceRunRecordsMissingAndStaleDiagnostics(t *testing.T) {
	now := time.Date(2026, 7, 13, 2, 0, 0, 0, time.UTC)
	scope := memory.Scope{Tenant: "tenant-a", Project: "project-a", Namespace: "namespace-a"}
	profile := ConformanceProfile{
		ID:     "profile_1",
		Scope:  scope,
		Status: ConformanceProfileStatusActive,
		ExpectedEvidence: []ExpectedEvidence{
			{Kind: ExpectedEvidenceOutcome, MinimumCount: 1, FreshnessWindow: time.Hour},
			{Kind: ExpectedEvidenceVerification, MinimumCount: 1, FreshnessWindow: time.Hour},
			{Kind: ExpectedEvidenceTaskEvaluation, MinimumCount: 1, FreshnessWindow: time.Hour},
			{Kind: ExpectedEvidenceRankingRollout, MinimumCount: 1, FreshnessWindow: time.Hour},
		},
		Actor:     "admin-a",
		Reason:    "detect incomplete integration",
		CreatedAt: now.Add(-time.Hour),
		UpdatedAt: now.Add(-time.Hour),
	}
	store := &stubAssuranceStore{
		conformanceProfiles: []ConformanceProfile{profile},
		conformanceEvidence: []ConformanceEvidenceObservation{
			{Kind: ExpectedEvidenceVerification, Count: 1, FreshestAt: now.Add(-2 * time.Hour)},
			{Kind: ExpectedEvidenceTaskEvaluation, Count: 1, FreshestAt: now.Add(-time.Minute), OpaqueOnly: true},
			{Kind: ExpectedEvidenceRankingRollout, Count: 1, FreshestAt: now.Add(-time.Minute), Hidden: true},
		},
	}
	service := NewService(ServiceOptions{
		Store: store,
		Now:   func() time.Time { return now },
		NewID: func(prefix string) string { return prefix + "_1" },
	})

	run, diagnostics, err := service.RunConformance(context.Background(), ConformanceRunInput{
		Scope:     scope,
		ProfileID: profile.ID,
		RunID:     "run_1",
		StartedAt: now,
	})
	if err != nil {
		t.Fatalf("RunConformance() error = %v", err)
	}
	if run.Result != ConformanceResultDegraded {
		t.Fatalf("run.Result = %q, want degraded", run.Result)
	}
	if len(diagnostics) != 4 {
		t.Fatalf("len(diagnostics) = %d, want 4: %+v", len(diagnostics), diagnostics)
	}
	wantCategories := map[MissingEvidenceCategory]bool{
		MissingEvidenceSessionWithoutOutcome: false,
		MissingEvidenceStale:                 false,
		MissingEvidenceOpaqueOnly:            false,
		MissingEvidenceHidden:                false,
	}
	for _, diagnostic := range diagnostics {
		if diagnostic.Scope != scope || diagnostic.ConformanceRunID != "run_1" {
			t.Fatalf("diagnostic = %+v, want scoped run diagnostic", diagnostic)
		}
		if _, ok := wantCategories[diagnostic.Category]; ok {
			wantCategories[diagnostic.Category] = true
		}
	}
	for category, seen := range wantCategories {
		if !seen {
			t.Fatalf("missing diagnostic category %q in %+v", category, diagnostics)
		}
	}
	if len(store.createdConformanceRuns) != 1 || len(store.createdMissingEvidenceDiagnostics) != 4 {
		t.Fatalf("created runs=%d diagnostics=%d, want one run and four diagnostics", len(store.createdConformanceRuns), len(store.createdMissingEvidenceDiagnostics))
	}
}

func TestServiceUsesBoundedMissingEvidenceCategories(t *testing.T) {
	cases := []struct {
		kind ExpectedEvidenceKind
		want MissingEvidenceCategory
	}{
		{kind: ExpectedEvidenceSession, want: MissingEvidenceSessionWithoutOutcome},
		{kind: ExpectedEvidenceContext, want: MissingEvidenceTurnWithoutContext},
		{kind: ExpectedEvidenceOutcome, want: MissingEvidenceSessionWithoutOutcome},
		{kind: ExpectedEvidenceVerification, want: MissingEvidenceVerificationMissing},
		{kind: ExpectedEvidenceUsefulnessFeedback, want: MissingEvidenceFeedbackWithoutSubject},
		{kind: ExpectedEvidenceTaskEvaluation, want: MissingEvidenceTaskEvaluationMissingEvidence},
		{kind: ExpectedEvidenceRepair, want: MissingEvidenceRepairWithoutVerification},
		{kind: ExpectedEvidenceRankingRollout, want: MissingEvidenceRolloutWithoutDryRun},
	}

	for _, tc := range cases {
		if got := missingEvidenceCategoryForKind(tc.kind); got != tc.want {
			t.Fatalf("missingEvidenceCategoryForKind(%q) = %q, want %q", tc.kind, got, tc.want)
		}
		if !tc.want.Valid() {
			t.Fatalf("category %q should be a bounded valid diagnostic", tc.want)
		}
	}
}

func TestServiceCreatesReadinessReportFromAssuranceAndConformance(t *testing.T) {
	now := time.Date(2026, 7, 13, 3, 0, 0, 0, time.UTC)
	scope := memory.Scope{Tenant: "tenant-a", Project: "project-a", Namespace: "namespace-a"}
	store := &stubAssuranceStore{
		healthEvaluations: []HealthEvaluation{{
			ID:        "health_1",
			Scope:     scope,
			Status:    HealthStatusHealthy,
			Severity:  SeverityInfo,
			Reason:    ReasonRuntimeReady,
			CreatedAt: now.Add(-time.Minute),
		}},
		conformanceRuns: []ConformanceRun{{
			ID:        "run_1",
			ProfileID: "profile_1",
			Scope:     scope,
			Result:    ConformanceResultDegraded,
			EvidenceCounts: map[string]any{
				"outcome": map[string]any{"count": 0, "minimum_count": 1},
			},
			StartedAt:  now.Add(-time.Minute),
			FinishedAt: now.Add(-time.Minute),
			CreatedAt:  now.Add(-time.Minute),
		}},
		operationalProofs: []OperationalProof{
			{
				ID:           "capacity_1",
				Scope:        scope,
				Target:       OperationalProofCapacityLoad,
				Status:       HealthStatusHealthy,
				Severity:     SeverityInfo,
				Reason:       ReasonCapacityWithinThresholds,
				ObservedAt:   now.Add(-time.Minute),
				FreshThrough: now.Add(time.Hour),
				CreatedAt:    now.Add(-time.Minute),
			},
			{
				ID:           "backup_1",
				Scope:        scope,
				Target:       OperationalProofBackupRestore,
				Status:       HealthStatusStale,
				Severity:     SeverityWarning,
				Reason:       ReasonBackupRestoreStale,
				ObservedAt:   now.Add(-2 * time.Hour),
				FreshThrough: now.Add(-time.Hour),
				CreatedAt:    now.Add(-2 * time.Hour),
			},
		},
		incidents: []Incident{{
			ID:               "incident_1",
			Scope:            scope,
			Status:           IncidentStatusOpen,
			Severity:         SeverityWarning,
			Component:        ComponentBackupRestore,
			Reason:           ReasonBackupRestoreStale,
			DeduplicationKey: "backup_restore:backup_restore_stale",
			OpenedAt:         now.Add(-time.Hour),
			UpdatedAt:        now.Add(-time.Minute),
		}},
		alertCandidates: []AlertCandidate{{
			ID:               "alert_1",
			Scope:            scope,
			Severity:         SeverityWarning,
			Component:        ComponentBackupRestore,
			Reason:           ReasonBackupRestoreStale,
			DeduplicationKey: "incident:incident_1:backup_restore:backup_restore_stale",
			DeliveryPolicy:   "default",
			CreatedAt:        now.Add(-time.Minute),
		}},
	}
	service := NewService(ServiceOptions{
		Store: store,
		Now:   func() time.Time { return now },
		NewID: func(prefix string) string { return prefix + "_1" },
	})

	report, err := service.CreateReadinessReport(context.Background(), ReadinessReportInput{
		Scope:       scope,
		ReportID:    "readiness_1",
		GeneratedAt: now,
	})
	if err != nil {
		t.Fatalf("CreateReadinessReport() error = %v", err)
	}
	if report.Status != ReadinessStatusDegraded {
		t.Fatalf("report.Status = %q, want degraded", report.Status)
	}
	if report.HealthEvaluationID != "health_1" || report.ConformanceRunID != "run_1" {
		t.Fatalf("report links = %+v, want latest health and conformance ids", report)
	}
	if report.ComponentSummary["conformance_status"] != string(ConformanceResultDegraded) {
		t.Fatalf("component summary = %+v, want conformance status", report.ComponentSummary)
	}
	if report.ComponentSummary["active_incidents"] != 1 || report.ComponentSummary["alert_candidates"] != 1 {
		t.Fatalf("component summary = %+v, want incident and alert counters", report.ComponentSummary)
	}
	if len(report.RecommendedActions) == 0 || report.RecommendedActions[0] != RunbookHintReviewConformanceProfile {
		t.Fatalf("recommended actions = %+v, want conformance remediation first", report.RecommendedActions)
	}
	if len(store.createdReadinessReports) != 1 {
		t.Fatalf("created readiness reports = %d, want 1", len(store.createdReadinessReports))
	}
}

func TestServiceReadinessReportIsUnknownWithoutRecentEvidence(t *testing.T) {
	now := time.Date(2026, 7, 13, 4, 0, 0, 0, time.UTC)
	scope := memory.Scope{Tenant: "tenant-a", Project: "project-a", Namespace: "namespace-a"}
	store := &stubAssuranceStore{}
	service := NewService(ServiceOptions{
		Store: store,
		Now:   func() time.Time { return now },
		NewID: func(prefix string) string { return prefix + "_1" },
	})

	report, err := service.CreateReadinessReport(context.Background(), ReadinessReportInput{
		Scope:       scope,
		ReportID:    "readiness_1",
		GeneratedAt: now,
	})
	if err != nil {
		t.Fatalf("CreateReadinessReport() error = %v", err)
	}
	if report.Status != ReadinessStatusUnknown {
		t.Fatalf("report.Status = %q, want unknown without recent evidence", report.Status)
	}
	if len(report.RecommendedActions) == 0 || report.RecommendedActions[0] != RunbookHintReviewConformanceProfile {
		t.Fatalf("recommended actions = %+v, want action to run health/conformance checks", report.RecommendedActions)
	}
}

func TestServiceReadinessReportDegradesOnCapacityProofFailure(t *testing.T) {
	now := time.Date(2026, 7, 13, 4, 30, 0, 0, time.UTC)
	scope := memory.Scope{Tenant: "tenant-a", Project: "project-a", Namespace: "namespace-a"}
	store := &stubAssuranceStore{
		healthEvaluations: []HealthEvaluation{{
			ID:        "health_1",
			Scope:     scope,
			Status:    HealthStatusHealthy,
			Severity:  SeverityInfo,
			Reason:    ReasonRuntimeReady,
			CreatedAt: now.Add(-time.Minute),
		}},
		conformanceRuns: []ConformanceRun{{
			ID:        "run_1",
			ProfileID: "profile_1",
			Scope:     scope,
			Result:    ConformanceResultPassed,
			StartedAt: now.Add(-time.Minute),
			CreatedAt: now.Add(-time.Minute),
		}},
		operationalProofs: []OperationalProof{{
			ID:         "capacity_1",
			Scope:      scope,
			Target:     OperationalProofCapacityLoad,
			Status:     HealthStatusUnhealthy,
			Severity:   SeverityCritical,
			Reason:     ReasonCapacityThresholdExceeded,
			ObservedAt: now.Add(-time.Minute),
			CreatedAt:  now.Add(-time.Minute),
		}},
	}
	service := NewService(ServiceOptions{
		Store: store,
		Now:   func() time.Time { return now },
		NewID: func(prefix string) string { return prefix + "_1" },
	})

	report, err := service.CreateReadinessReport(context.Background(), ReadinessReportInput{
		Scope:       scope,
		ReportID:    "readiness_1",
		GeneratedAt: now,
	})
	if err != nil {
		t.Fatalf("CreateReadinessReport() error = %v", err)
	}
	if report.Status != ReadinessStatusDegraded {
		t.Fatalf("report.Status = %q, want degraded on capacity/load proof failure", report.Status)
	}
	if len(report.RecommendedActions) == 0 || report.RecommendedActions[0] != RunbookHintReviewCapacityProof {
		t.Fatalf("recommended actions = %+v, want capacity proof remediation", report.RecommendedActions)
	}
}

func TestServiceCreatesRecoveryVerificationWithLinkedEvidence(t *testing.T) {
	now := time.Date(2026, 7, 13, 5, 0, 0, 0, time.UTC)
	scope := memory.Scope{Tenant: "tenant-a", Project: "project-a", Namespace: "namespace-a"}
	store := &stubAssuranceStore{}
	service := NewService(ServiceOptions{
		Store: store,
		Now:   func() time.Time { return now },
		NewID: func(prefix string) string { return prefix + "_1" },
	})

	record, err := service.CreateRecoveryVerification(context.Background(), RecoveryVerificationInput{
		Scope:           scope,
		RecordID:        "recovery_1",
		Target:          RecoveryVerificationTargetConformanceRun,
		TargetID:        "run_1",
		Status:          HealthStatusHealthy,
		CheckedSurfaces: []string{"conformance_run", "readiness_report"},
		ResultCategory:  "recovered",
		LinkedEvidence: map[string]any{
			"conformance_run_id": "run_2",
			"readiness_report":   "readiness_1",
		},
		Actor:      "admin-a",
		Reason:     "verified after integration fix",
		VerifiedAt: now,
	})
	if err != nil {
		t.Fatalf("CreateRecoveryVerification() error = %v", err)
	}
	if record.ID != "recovery_1" || record.Scope != scope || record.TargetID != "run_1" {
		t.Fatalf("record = %+v, want scoped recovery verification", record)
	}
	if record.LinkedEvidence["conformance_run_id"] != "run_2" {
		t.Fatalf("linked evidence = %+v, want conformance rerun link", record.LinkedEvidence)
	}
	if len(store.createdRecoveryVerifications) != 1 {
		t.Fatalf("created recovery verifications = %d, want 1", len(store.createdRecoveryVerifications))
	}
}

func TestServiceExposesAdminReadSurfacesThroughScopedStore(t *testing.T) {
	now := time.Date(2026, 7, 13, 6, 0, 0, 0, time.UTC)
	scope := memory.Scope{Tenant: "tenant-a", Project: "project-a", Namespace: "namespace-a"}
	otherScope := memory.Scope{Tenant: "tenant-b", Project: "project-a", Namespace: "namespace-a"}
	store := &stubAssuranceStore{
		healthEvaluations: []HealthEvaluation{
			{ID: "health_1", Scope: scope, Status: HealthStatusHealthy, Severity: SeverityInfo, CreatedAt: now},
			{ID: "health_other", Scope: otherScope, Status: HealthStatusUnhealthy, Severity: SeverityCritical, CreatedAt: now},
		},
		alertCandidates: []AlertCandidate{
			{ID: "alert_1", Scope: scope, Severity: SeverityWarning, Component: ComponentBacklog, Reason: ReasonBacklogPressure, DeduplicationKey: "backlog", DeliveryPolicy: "default", CreatedAt: now},
			{ID: "alert_other", Scope: otherScope, Severity: SeverityCritical, Component: ComponentBacklog, Reason: ReasonBacklogPressure, DeduplicationKey: "other", DeliveryPolicy: "default", CreatedAt: now},
		},
		alertDeliveryAttempts: []AlertDeliveryAttempt{
			{ID: "attempt_1", AlertCandidateID: "alert_1", Scope: scope, Adapter: AlertAdapterDisabled, Result: AlertDeliveryResultDisabled, Attempt: 1, AttemptedAt: now},
			{ID: "attempt_other", AlertCandidateID: "alert_other", Scope: otherScope, Adapter: AlertAdapterDisabled, Result: AlertDeliveryResultDisabled, Attempt: 1, AttemptedAt: now},
		},
		conformanceRuns: []ConformanceRun{
			{ID: "run_1", ProfileID: "profile_1", Scope: scope, Result: ConformanceResultPassed, StartedAt: now, CreatedAt: now},
			{ID: "run_other", ProfileID: "profile_other", Scope: otherScope, Result: ConformanceResultFailed, StartedAt: now, CreatedAt: now},
		},
		readinessReports: []ReadinessReport{
			{ID: "readiness_1", Scope: scope, Status: ReadinessStatusReady, GeneratedAt: now, CreatedAt: now},
			{ID: "readiness_other", Scope: otherScope, Status: ReadinessStatusBlocked, GeneratedAt: now, CreatedAt: now},
		},
		recoveryVerifications: []RecoveryVerification{
			{ID: "recovery_1", Scope: scope, Target: RecoveryVerificationTargetIncident, TargetID: "incident_1", Status: HealthStatusHealthy, ResultCategory: "recovered", Actor: "admin-a", Reason: "verified", CreatedAt: now},
			{ID: "recovery_other", Scope: otherScope, Target: RecoveryVerificationTargetIncident, TargetID: "incident_other", Status: HealthStatusUnhealthy, ResultCategory: "failed", Actor: "admin-a", Reason: "verified", CreatedAt: now},
		},
	}
	service := NewService(ServiceOptions{Store: store})

	health, err := service.ReadHealthEvaluation(context.Background(), ReadHealthEvaluationInput{Scope: scope, EvaluationID: "health_1"})
	if err != nil {
		t.Fatalf("ReadHealthEvaluation() error = %v", err)
	}
	if health.ID != "health_1" {
		t.Fatalf("ReadHealthEvaluation() ID = %q, want health_1", health.ID)
	}

	alerts, err := service.ListAlertCandidates(context.Background(), scope)
	if err != nil {
		t.Fatalf("ListAlertCandidates() error = %v", err)
	}
	if len(alerts) != 1 || alerts[0].ID != "alert_1" {
		t.Fatalf("ListAlertCandidates() = %+v, want scoped alert_1", alerts)
	}

	alert, err := service.ReadAlertCandidate(context.Background(), ReadAlertCandidateInput{Scope: scope, AlertCandidateID: "alert_1"})
	if err != nil {
		t.Fatalf("ReadAlertCandidate() error = %v", err)
	}
	if alert.ID != "alert_1" {
		t.Fatalf("ReadAlertCandidate() ID = %q, want alert_1", alert.ID)
	}

	attempts, err := service.ListAlertDeliveryAttempts(context.Background(), ListAlertDeliveryAttemptsInput{Scope: scope, AlertCandidateID: "alert_1"})
	if err != nil {
		t.Fatalf("ListAlertDeliveryAttempts() error = %v", err)
	}
	if len(attempts) != 1 || attempts[0].ID != "attempt_1" {
		t.Fatalf("ListAlertDeliveryAttempts() = %+v, want scoped attempt_1", attempts)
	}

	run, err := service.ReadConformanceRun(context.Background(), ReadConformanceRunInput{Scope: scope, RunID: "run_1"})
	if err != nil {
		t.Fatalf("ReadConformanceRun() error = %v", err)
	}
	if run.ID != "run_1" {
		t.Fatalf("ReadConformanceRun() ID = %q, want run_1", run.ID)
	}

	readiness, err := service.ReadReadinessReport(context.Background(), ReadReadinessReportInput{Scope: scope, ReportID: "readiness_1"})
	if err != nil {
		t.Fatalf("ReadReadinessReport() error = %v", err)
	}
	if readiness.ID != "readiness_1" {
		t.Fatalf("ReadReadinessReport() ID = %q, want readiness_1", readiness.ID)
	}

	recoveries, err := service.ListRecoveryVerifications(context.Background(), scope)
	if err != nil {
		t.Fatalf("ListRecoveryVerifications() error = %v", err)
	}
	if len(recoveries) != 1 || recoveries[0].ID != "recovery_1" {
		t.Fatalf("ListRecoveryVerifications() = %+v, want scoped recovery_1", recoveries)
	}
}

func TestServiceCreatesRetentionRunThroughScopedStore(t *testing.T) {
	now := time.Date(2026, 7, 13, 7, 0, 0, 0, time.UTC)
	scope := memory.Scope{Tenant: "tenant-a", Project: "project-a", Namespace: "namespace-a"}
	store := &stubAssuranceStore{}
	service := NewService(ServiceOptions{Store: store})

	run, err := service.CreateRetentionRun(context.Background(), RetentionRun{
		ID:             "retention_1",
		Scope:          scope,
		RecordCategory: RetentionClassDiagnostic,
		Cutoff:         now.Add(-7 * 24 * time.Hour),
		DeletedCount:   3,
		Status:         HealthStatusHealthy,
		StartedAt:      now,
		FinishedAt:     now,
	})
	if err != nil {
		t.Fatalf("CreateRetentionRun() error = %v", err)
	}
	if run.ID != "retention_1" || run.Scope != scope || run.RecordCategory != RetentionClassDiagnostic {
		t.Fatalf("CreateRetentionRun() = %+v, want scoped diagnostic retention run", run)
	}
	if len(store.createdRetentionRuns) != 1 {
		t.Fatalf("created retention runs = %d, want 1", len(store.createdRetentionRuns))
	}
}

func TestServiceClaimsAlertCandidatesForDeliveryThroughScopedStore(t *testing.T) {
	now := time.Date(2026, 7, 13, 11, 45, 0, 0, time.UTC)
	scope := memory.Scope{Tenant: "tenant-a", Project: "project-a", Namespace: "namespace-a"}
	store := &stubAssuranceStore{
		alertDeliveryClaims: []AlertDeliveryClaim{
			{
				Candidate: AlertCandidate{
					ID:               "alert_1",
					Scope:            scope,
					Severity:         SeverityCritical,
					Component:        ComponentBackupRestore,
					Reason:           ReasonBackupRestoreStale,
					DeduplicationKey: "incident:incident_1:backup_restore",
					DeliveryPolicy:   "default",
					CreatedAt:        now.Add(-time.Hour),
				},
				Attempt:    2,
				WorkerID:   "worker-a",
				ClaimedAt:  now,
				LeaseUntil: now.Add(2 * time.Minute),
			},
		},
	}
	service := NewService(ServiceOptions{Store: store})

	claims, err := service.ClaimAlertCandidatesForDelivery(context.Background(), ClaimAlertCandidatesForDeliveryInput{
		Scope:         scope,
		WorkerID:      "worker-a",
		Now:           now,
		LeaseDuration: 2 * time.Minute,
		Limit:         5,
		MaxAttempts:   4,
	})
	if err != nil {
		t.Fatalf("ClaimAlertCandidatesForDelivery() error = %v", err)
	}
	if len(claims) != 1 || claims[0].Candidate.ID != "alert_1" || claims[0].Attempt != 2 {
		t.Fatalf("claims = %+v, want alert_1 attempt 2", claims)
	}
	if store.claimAlertInputs[0].Scope != scope || store.claimAlertInputs[0].WorkerID != "worker-a" {
		t.Fatalf("claim input = %+v, want scoped worker input", store.claimAlertInputs[0])
	}
}

func TestServiceRecordsAssuranceConformanceMetricsWithoutHighCardinalityLabels(t *testing.T) {
	now := time.Date(2026, 7, 13, 12, 0, 0, 0, time.UTC)
	scope := memory.Scope{Tenant: "tenant-a", Project: "project-a", Namespace: "namespace-a"}
	observer := telemetry.NewMetricsObserver()
	store := &stubAssuranceStore{
		conformanceProfiles: []ConformanceProfile{{
			ID:     "profile_1",
			Scope:  scope,
			Status: ConformanceProfileStatusActive,
			ExpectedEvidence: []ExpectedEvidence{
				{Kind: ExpectedEvidenceSession, MinimumCount: 1, FreshnessWindow: time.Hour},
			},
			Actor:     "admin-a",
			Reason:    "external integration should record sessions",
			CreatedAt: now.Add(-time.Hour),
			UpdatedAt: now.Add(-time.Hour),
		}},
	}
	service := NewService(ServiceOptions{
		Store:    store,
		Now:      func() time.Time { return now },
		NewID:    func(prefix string) string { return prefix + "_1" },
		Observer: observer,
	})

	evaluation, err := service.CreateHealthEvaluation(context.Background(), HealthEvaluationInput{
		Scope:      scope,
		ObservedAt: now,
		CapacityLoadProof: HealthObservation{
			Status:     HealthStatusUnhealthy,
			Severity:   SeverityCritical,
			Reason:     ReasonCapacityThresholdExceeded,
			Evidence:   map[string]any{"worker_latency_ms": 900, "tenant_hint": scope.Tenant},
			ObservedAt: now,
		},
		BackupRestoreProof: HealthObservation{
			Status:       HealthStatusHealthy,
			Severity:     SeverityInfo,
			Reason:       ReasonBackupRestoreFresh,
			Evidence:     map[string]any{"marker": "restore-1"},
			ObservedAt:   now,
			FreshThrough: now.Add(time.Hour),
		},
	})
	if err != nil {
		t.Fatalf("CreateHealthEvaluation() error = %v", err)
	}

	_, err = service.ApplyIncidentAction(context.Background(), IncidentActionInput{
		Scope:      scope,
		IncidentID: store.createdIncidents[0].ID,
		Action:     IncidentActionAcknowledge,
		Actor:      "admin-a",
		Reason:     "operator acknowledged current degradation",
		OccurredAt: now.Add(time.Minute),
	})
	if err != nil {
		t.Fatalf("ApplyIncidentAction() error = %v", err)
	}

	candidates, err := service.GenerateAlertCandidates(context.Background(), AlertCandidateGenerationInput{
		Scope:               scope,
		Evaluation:          evaluation,
		DeliveryPolicy:      "default",
		DeduplicationWindow: 0,
		CreatedAt:           now.Add(2 * time.Minute),
	})
	if err != nil {
		t.Fatalf("GenerateAlertCandidates() error = %v", err)
	}
	if len(candidates) == 0 {
		t.Fatal("GenerateAlertCandidates() returned no candidates")
	}
	if _, err := service.DeliverAlertCandidate(context.Background(), AlertDeliveryInput{
		Scope:       scope,
		Candidate:   candidates[0],
		Config:      AlertDeliveryConfig{Mode: AlertAdapterDisabled},
		MaxAttempts: 1,
		WorkerID:    "worker-1",
		Now:         now.Add(3 * time.Minute),
	}); err != nil {
		t.Fatalf("DeliverAlertCandidate() error = %v", err)
	}

	if _, _, err := service.RunConformance(context.Background(), ConformanceRunInput{
		Scope:     scope,
		ProfileID: "profile_1",
		RunID:     "run_1",
		StartedAt: now.Add(4 * time.Minute),
	}); err != nil {
		t.Fatalf("RunConformance() error = %v", err)
	}

	if _, err := service.CreateReadinessReport(context.Background(), ReadinessReportInput{
		Scope:       scope,
		ReportID:    "readiness_1",
		GeneratedAt: now.Add(5 * time.Minute),
	}); err != nil {
		t.Fatalf("CreateReadinessReport() error = %v", err)
	}
	if _, err := service.CreateRecoveryVerification(context.Background(), RecoveryVerificationInput{
		Scope:          scope,
		RecordID:       "recovery_1",
		Target:         RecoveryVerificationTargetIncident,
		TargetID:       store.createdIncidents[0].ID,
		Status:         HealthStatusHealthy,
		ResultCategory: "recovered",
		Actor:          "admin-a",
		Reason:         "operator verified remediation",
		VerifiedAt:     now.Add(6 * time.Minute),
	}); err != nil {
		t.Fatalf("CreateRecoveryVerification() error = %v", err)
	}
	if _, err := service.CreateRetentionRun(context.Background(), RetentionRun{
		ID:             "retention_1",
		Scope:          scope,
		RecordCategory: RetentionClassDiagnostic,
		Cutoff:         now.Add(-7 * 24 * time.Hour),
		DeletedCount:   0,
		Status:         HealthStatusHealthy,
		StartedAt:      now.Add(7 * time.Minute),
		FinishedAt:     now.Add(7 * time.Minute),
	}); err != nil {
		t.Fatalf("CreateRetentionRun() error = %v", err)
	}

	metrics := observer.RenderPrometheus()
	for _, want := range []string{
		`stele_assurance_health_evaluations_total{component="capacity_load",operation="create",operational_proof="capacity_load",reason_category="capacity_threshold_exceeded",result="ok",severity="critical",status="unhealthy"} 1`,
		`stele_assurance_incidents_total{component="capacity_load",operation="open",reason_category="capacity_threshold_exceeded",result="ok",severity="critical",status="open"} 1`,
		`stele_assurance_incidents_total{component="capacity_load",operation="acknowledge",reason_category="capacity_threshold_exceeded",result="ok",severity="critical",status="acknowledged"} 1`,
		`stele_assurance_alert_candidates_total{component="capacity_load",operation="generate",reason_category="capacity_threshold_exceeded",result="ok",severity="critical",status="queued"} 2`,
		`stele_assurance_alert_delivery_total{adapter="disabled",component="capacity_load",failure_category="none",result="disabled",severity="critical"} 1`,
		`stele_conformance_runs_total{evidence_category="session",missing_evidence_category="session_without_outcome",profile_status="active",readiness_impact="degraded",result="degraded"} 1`,
		`stele_conformance_missing_evidence_total{evidence_category="session",missing_evidence_category="session_without_outcome",readiness_impact="degraded"} 1`,
		`stele_operational_proofs_total{reason_category="capacity_threshold_exceeded",severity="critical",status="unhealthy",target="capacity_load"} 1`,
		`stele_readiness_reports_total{conformance_category="degraded",incident_category="active",readiness_status="blocked",recommended_action_category="review_conformance_profile",runtime_category="unhealthy"} 1`,
		`stele_recovery_verifications_total{result_category="recovered",status="healthy",target="incident"} 1`,
		`stele_assurance_cleanup_total{deleted_category="none",record_category="diagnostic",result="ok"} 1`,
	} {
		if !strings.Contains(metrics, want) {
			t.Fatalf("metrics missing %q\n%s", want, metrics)
		}
	}
	for _, forbidden := range []string{scope.Tenant, scope.Project, scope.Namespace, "health_evaluation_1", "incident_1", "alert_candidate_id", "run_1", "recovery_1", "admin-a", "operator acknowledged", "tenant_hint", "webhook_url", "recipient"} {
		if strings.Contains(metrics, forbidden) {
			t.Fatalf("metrics contain high-cardinality value %q\n%s", forbidden, metrics)
		}
	}
}

func TestServiceLogsAssuranceConformanceLifecycleWithoutHighCardinalityFields(t *testing.T) {
	now := time.Date(2026, 7, 13, 13, 0, 0, 0, time.UTC)
	scope := memory.Scope{Tenant: "tenant-a", Project: "project-a", Namespace: "namespace-a"}
	var logBuf bytes.Buffer
	store := &stubAssuranceStore{
		conformanceProfiles: []ConformanceProfile{{
			ID:     "profile_1",
			Scope:  scope,
			Status: ConformanceProfileStatusActive,
			ExpectedEvidence: []ExpectedEvidence{
				{Kind: ExpectedEvidenceSession, MinimumCount: 1, FreshnessWindow: time.Hour},
			},
			Actor:     "admin-a",
			Reason:    "external integration should record sessions",
			CreatedAt: now.Add(-time.Hour),
			UpdatedAt: now.Add(-time.Hour),
		}},
	}
	service := NewService(ServiceOptions{
		Store:  store,
		Now:    func() time.Time { return now },
		NewID:  func(prefix string) string { return prefix + "_1" },
		Logger: log.New(&logBuf, "", 0),
	})

	evaluation, err := service.CreateHealthEvaluation(context.Background(), HealthEvaluationInput{
		Scope:      scope,
		ObservedAt: now,
		CapacityLoadProof: HealthObservation{
			Status:     HealthStatusUnhealthy,
			Severity:   SeverityCritical,
			Reason:     ReasonCapacityThresholdExceeded,
			Evidence:   map[string]any{"tenant_hint": scope.Tenant},
			ObservedAt: now,
		},
	})
	if err != nil {
		t.Fatalf("CreateHealthEvaluation() error = %v", err)
	}
	if _, err := service.ApplyIncidentAction(context.Background(), IncidentActionInput{
		Scope:      scope,
		IncidentID: store.createdIncidents[0].ID,
		Action:     IncidentActionAcknowledge,
		Actor:      "admin-a",
		Reason:     "operator acknowledged current degradation",
		OccurredAt: now.Add(time.Minute),
	}); err != nil {
		t.Fatalf("ApplyIncidentAction() error = %v", err)
	}
	candidates, err := service.GenerateAlertCandidates(context.Background(), AlertCandidateGenerationInput{
		Scope:               scope,
		Evaluation:          evaluation,
		DeliveryPolicy:      "default",
		DeduplicationWindow: 0,
		CreatedAt:           now.Add(2 * time.Minute),
	})
	if err != nil {
		t.Fatalf("GenerateAlertCandidates() error = %v", err)
	}
	if _, err := service.DeliverAlertCandidate(context.Background(), AlertDeliveryInput{
		Scope:       scope,
		Candidate:   candidates[0],
		Config:      AlertDeliveryConfig{Mode: AlertAdapterDisabled},
		MaxAttempts: 1,
		WorkerID:    "worker-1",
		Now:         now.Add(3 * time.Minute),
	}); err != nil {
		t.Fatalf("DeliverAlertCandidate() error = %v", err)
	}
	if _, _, err := service.RunConformance(context.Background(), ConformanceRunInput{
		Scope:     scope,
		ProfileID: "profile_1",
		RunID:     "run_1",
		StartedAt: now.Add(4 * time.Minute),
	}); err != nil {
		t.Fatalf("RunConformance() error = %v", err)
	}
	if _, err := service.CreateReadinessReport(context.Background(), ReadinessReportInput{
		Scope:       scope,
		ReportID:    "readiness_1",
		GeneratedAt: now.Add(5 * time.Minute),
	}); err != nil {
		t.Fatalf("CreateReadinessReport() error = %v", err)
	}
	if _, err := service.CreateRecoveryVerification(context.Background(), RecoveryVerificationInput{
		Scope:          scope,
		RecordID:       "recovery_1",
		Target:         RecoveryVerificationTargetIncident,
		TargetID:       store.createdIncidents[0].ID,
		Status:         HealthStatusHealthy,
		ResultCategory: "recovered",
		Actor:          "admin-a",
		Reason:         "operator verified remediation",
		VerifiedAt:     now.Add(6 * time.Minute),
	}); err != nil {
		t.Fatalf("CreateRecoveryVerification() error = %v", err)
	}

	logs := logBuf.String()
	for _, want := range []string{
		"component=assurance event=lifecycle operation=create result=ok status=unhealthy health_component=capacity_load severity=critical reason_category=capacity_threshold_exceeded",
		"component=assurance event=lifecycle operation=open result=ok status=open health_component=capacity_load severity=critical reason_category=capacity_threshold_exceeded",
		"component=assurance event=lifecycle operation=acknowledge result=ok status=acknowledged health_component=capacity_load severity=critical reason_category=capacity_threshold_exceeded",
		"component=assurance event=lifecycle operation=generate result=ok status=queued health_component=capacity_load severity=critical reason_category=capacity_threshold_exceeded",
		"component=assurance event=lifecycle operation=deliver result=disabled status=disabled health_component=capacity_load severity=critical reason_category=none adapter=disabled failure_category=none",
		"component=conformance event=lifecycle operation=run result=degraded evidence_category=session readiness_status=degraded missing_evidence_category=session_without_outcome",
		"component=conformance event=lifecycle operation=diagnostic result=degraded evidence_category=session readiness_status=degraded missing_evidence_category=session_without_outcome",
		"component=conformance event=lifecycle operation=readiness result=blocked evidence_category=unknown readiness_status=blocked missing_evidence_category=unknown",
		"component=assurance event=lifecycle operation=recovery_verify result=recovered status=healthy health_component=unknown severity=unknown reason_category=none",
	} {
		if !strings.Contains(logs, want) {
			t.Fatalf("logs missing %q\n%s", want, logs)
		}
	}
	for _, forbidden := range []string{scope.Tenant, scope.Project, scope.Namespace, "health_evaluation_1", "incident_1", "alert_candidate_id", "run_1", "recovery_1", "admin-a", "operator acknowledged", "tenant_hint", "webhook_url", "recipient"} {
		if strings.Contains(logs, forbidden) {
			t.Fatalf("logs contain high-cardinality value %q\n%s", forbidden, logs)
		}
	}
}

func TestServiceDoesNotExportWebhookSecretsInMetricsOrLifecycleLogs(t *testing.T) {
	now := time.Date(2026, 7, 13, 14, 0, 0, 0, time.UTC)
	scope := memory.Scope{Tenant: "tenant-a", Project: "project-a", Namespace: "namespace-a"}
	var logBuf bytes.Buffer
	observer := telemetry.NewMetricsObserver()
	service := NewService(ServiceOptions{
		Store:    &stubAssuranceStore{},
		Now:      func() time.Time { return now },
		NewID:    func(prefix string) string { return prefix + "_1" },
		Observer: observer,
		Logger:   log.New(&logBuf, "", 0),
	})
	candidate := AlertCandidate{
		ID:               "alert_1",
		Scope:            scope,
		Severity:         SeverityCritical,
		Component:        ComponentBackupRestore,
		Reason:           ReasonBackupRestoreStale,
		DeduplicationKey: "incident:incident_1:backup_restore:backup_restore_stale",
		DeliveryPolicy:   "default",
		Payload:          map[string]any{"component": "backup_restore", "severity": "critical"},
		CreatedAt:        now,
	}
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("Authorization"); got != "Bearer super-secret-token" {
			t.Fatalf("Authorization header = %q, want forwarded configured secret", got)
		}
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	_, err := service.DeliverAlertCandidate(context.Background(), AlertDeliveryInput{
		Scope:     scope,
		Candidate: candidate,
		Config: AlertDeliveryConfig{
			Mode:            AlertAdapterWebhook,
			WebhookURL:      server.URL,
			WebhookHeaders:  map[string]string{"Authorization": "Bearer super-secret-token"},
			Timeout:         5 * time.Second,
			MaxPayloadBytes: 4096,
		},
		MaxAttempts: 1,
		WorkerID:    "worker-1",
		Now:         now,
		HTTPClient:  server.Client(),
	})
	if err != nil {
		t.Fatalf("DeliverAlertCandidate() error = %v", err)
	}

	exported := observer.RenderPrometheus() + "\n" + logBuf.String()
	for _, forbidden := range []string{"super-secret-token", "Authorization", server.URL, "webhook_url", "recipient", "alert_1", "incident_1", scope.Tenant, scope.Project, scope.Namespace} {
		if strings.Contains(exported, forbidden) {
			t.Fatalf("exported telemetry contains forbidden secret or high-cardinality value %q\n%s", forbidden, exported)
		}
	}
	for _, want := range []string{
		`stele_assurance_alert_delivery_total{adapter="webhook",component="backup_restore",failure_category="http_status_500",result="failed",severity="critical"} 1`,
		"component=assurance event=lifecycle operation=deliver result=failed status=failed health_component=backup_restore severity=critical reason_category=none adapter=webhook failure_category=http_status_500",
	} {
		if !strings.Contains(exported, want) {
			t.Fatalf("exported telemetry missing %q\n%s", want, exported)
		}
	}
}

type stubAssuranceStore struct {
	createdEvaluations                []HealthEvaluation
	createdIncidents                  []Incident
	createdAlertCandidates            []AlertCandidate
	createdAlertDeliveryAttempts      []AlertDeliveryAttempt
	createdConformanceProfiles        []ConformanceProfile
	createdConformanceRuns            []ConformanceRun
	createdMissingEvidenceDiagnostics []MissingEvidenceDiagnostic
	createdReadinessReports           []ReadinessReport
	createdRecoveryVerifications      []RecoveryVerification
	createdRetentionRuns              []RetentionRun
	healthEvaluations                 []HealthEvaluation
	incidents                         []Incident
	alertCandidates                   []AlertCandidate
	alertDeliveryAttempts             []AlertDeliveryAttempt
	alertDeliveryClaims               []AlertDeliveryClaim
	conformanceProfiles               []ConformanceProfile
	conformanceRuns                   []ConformanceRun
	readinessReports                  []ReadinessReport
	recoveryVerifications             []RecoveryVerification
	operationalProofs                 []OperationalProof
	conformanceEvidence               []ConformanceEvidenceObservation
	inspectInputs                     []ConformanceEvidenceInspectionInput
	claimAlertInputs                  []ClaimAlertCandidatesForDeliveryInput
	transitions                       []IncidentTransition
	workflowHealth                    WorkflowHealthSnapshot
}

type workflowHealthReaderFunc func(context.Context, memory.Scope, time.Time) (WorkflowHealthSnapshot, error)

func (f workflowHealthReaderFunc) ReadWorkflowHealth(ctx context.Context, scope memory.Scope, observedAt time.Time) (WorkflowHealthSnapshot, error) {
	return f(ctx, scope, observedAt)
}

func containsRunbookHint(hints []RunbookHintCategory, want RunbookHintCategory) bool {
	for _, hint := range hints {
		if hint == want {
			return true
		}
	}
	return false
}

func (s *stubAssuranceStore) CreateHealthEvaluation(ctx context.Context, evaluation HealthEvaluation) (HealthEvaluation, error) {
	s.createdEvaluations = append(s.createdEvaluations, evaluation)
	return evaluation, nil
}

func (s *stubAssuranceStore) ListHealthEvaluations(ctx context.Context, scope memory.Scope) ([]HealthEvaluation, error) {
	evaluations := append(append([]HealthEvaluation{}, s.healthEvaluations...), s.createdEvaluations...)
	filtered := make([]HealthEvaluation, 0, len(evaluations))
	for _, evaluation := range evaluations {
		if evaluation.Scope == scope {
			filtered = append(filtered, evaluation)
		}
	}
	return filtered, nil
}

func (s *stubAssuranceStore) ReadHealthEvaluation(ctx context.Context, input ReadHealthEvaluationInput) (HealthEvaluation, error) {
	for _, evaluation := range append(append([]HealthEvaluation{}, s.healthEvaluations...), s.createdEvaluations...) {
		if evaluation.Scope == input.Scope && evaluation.ID == input.EvaluationID {
			return evaluation, nil
		}
	}
	return HealthEvaluation{}, nil
}

func (s *stubAssuranceStore) ListIncidents(ctx context.Context, input ListIncidentsInput) ([]Incident, error) {
	incidents := make([]Incident, 0, len(s.incidents)+len(s.createdIncidents))
	incidents = append(incidents, s.incidents...)
	incidents = append(incidents, s.createdIncidents...)
	if input.Scope != (memory.Scope{}) {
		filtered := make([]Incident, 0, len(incidents))
		for _, incident := range incidents {
			if incident.Scope == input.Scope {
				filtered = append(filtered, incident)
			}
		}
		return filtered, nil
	}
	return incidents, nil
}

func (s *stubAssuranceStore) CreateIncident(ctx context.Context, incident Incident) (Incident, error) {
	s.createdIncidents = append(s.createdIncidents, incident)
	return incident, nil
}

func (s *stubAssuranceStore) ReadIncident(ctx context.Context, input ReadIncidentInput) (Incident, error) {
	for _, incident := range append(append([]Incident{}, s.incidents...), s.createdIncidents...) {
		if incident.Scope == input.Scope && incident.ID == input.IncidentID {
			return incident, nil
		}
	}
	return Incident{}, nil
}

func (s *stubAssuranceStore) TransitionIncident(ctx context.Context, transition IncidentTransition) (Incident, error) {
	s.transitions = append(s.transitions, transition)
	update := func(incident *Incident) Incident {
		incident.Status = transition.ToStatus
		incident.UpdatedAt = transition.OccurredAt
		if transition.ToStatus == IncidentStatusResolved {
			incident.ResolvedAt = transition.OccurredAt
		}
		return *incident
	}
	for idx, incident := range s.incidents {
		if incident.Scope == transition.Scope && incident.ID == transition.IncidentID {
			return update(&s.incidents[idx]), nil
		}
	}
	for idx, incident := range s.createdIncidents {
		if incident.Scope == transition.Scope && incident.ID == transition.IncidentID {
			return update(&s.createdIncidents[idx]), nil
		}
	}
	return Incident{}, nil
}

func (s *stubAssuranceStore) ListAlertCandidates(ctx context.Context, scope memory.Scope) ([]AlertCandidate, error) {
	candidates := make([]AlertCandidate, 0, len(s.alertCandidates)+len(s.createdAlertCandidates))
	candidates = append(candidates, s.alertCandidates...)
	candidates = append(candidates, s.createdAlertCandidates...)
	filtered := make([]AlertCandidate, 0, len(candidates))
	for _, candidate := range candidates {
		if candidate.Scope == scope {
			filtered = append(filtered, candidate)
		}
	}
	return filtered, nil
}

func (s *stubAssuranceStore) ReadAlertCandidate(ctx context.Context, input ReadAlertCandidateInput) (AlertCandidate, error) {
	for _, candidate := range append(append([]AlertCandidate{}, s.alertCandidates...), s.createdAlertCandidates...) {
		if candidate.Scope == input.Scope && candidate.ID == input.AlertCandidateID {
			return candidate, nil
		}
	}
	return AlertCandidate{}, nil
}

func (s *stubAssuranceStore) CreateAlertCandidate(ctx context.Context, candidate AlertCandidate) (AlertCandidate, error) {
	s.createdAlertCandidates = append(s.createdAlertCandidates, candidate)
	return candidate, nil
}

func (s *stubAssuranceStore) ListAlertDeliveryAttempts(ctx context.Context, input ListAlertDeliveryAttemptsInput) ([]AlertDeliveryAttempt, error) {
	attempts := make([]AlertDeliveryAttempt, 0, len(s.alertDeliveryAttempts)+len(s.createdAlertDeliveryAttempts))
	attempts = append(attempts, s.alertDeliveryAttempts...)
	attempts = append(attempts, s.createdAlertDeliveryAttempts...)
	filtered := make([]AlertDeliveryAttempt, 0, len(attempts))
	for _, attempt := range attempts {
		if attempt.Scope == input.Scope && attempt.AlertCandidateID == input.AlertCandidateID {
			filtered = append(filtered, attempt)
		}
	}
	return filtered, nil
}

func (s *stubAssuranceStore) CreateAlertDeliveryAttempt(ctx context.Context, attempt AlertDeliveryAttempt) (AlertDeliveryAttempt, error) {
	s.createdAlertDeliveryAttempts = append(s.createdAlertDeliveryAttempts, attempt)
	return attempt, nil
}

func (s *stubAssuranceStore) ClaimAlertCandidatesForDelivery(ctx context.Context, input ClaimAlertCandidatesForDeliveryInput) ([]AlertDeliveryClaim, error) {
	s.claimAlertInputs = append(s.claimAlertInputs, input)
	filtered := make([]AlertDeliveryClaim, 0, len(s.alertDeliveryClaims))
	for _, claim := range s.alertDeliveryClaims {
		if claim.Candidate.Scope == input.Scope {
			filtered = append(filtered, claim)
		}
	}
	return filtered, nil
}

func (s *stubAssuranceStore) CreateConformanceProfile(ctx context.Context, profile ConformanceProfile) (ConformanceProfile, error) {
	s.createdConformanceProfiles = append(s.createdConformanceProfiles, profile)
	return profile, nil
}

func (s *stubAssuranceStore) ReadConformanceProfile(ctx context.Context, input ReadConformanceProfileInput) (ConformanceProfile, error) {
	for _, profile := range append(append([]ConformanceProfile{}, s.conformanceProfiles...), s.createdConformanceProfiles...) {
		if profile.Scope == input.Scope && profile.ID == input.ProfileID {
			return profile, nil
		}
	}
	return ConformanceProfile{}, nil
}

func (s *stubAssuranceStore) ListConformanceProfiles(ctx context.Context, input ListConformanceProfilesInput) ([]ConformanceProfile, error) {
	profiles := make([]ConformanceProfile, 0, len(s.conformanceProfiles)+len(s.createdConformanceProfiles))
	profiles = append(profiles, s.conformanceProfiles...)
	profiles = append(profiles, s.createdConformanceProfiles...)
	filtered := make([]ConformanceProfile, 0, len(profiles))
	for _, profile := range profiles {
		if profile.Scope != input.Scope {
			continue
		}
		if input.Status != "" && profile.Status != input.Status {
			continue
		}
		filtered = append(filtered, profile)
	}
	return filtered, nil
}

func (s *stubAssuranceStore) UpdateConformanceProfile(ctx context.Context, input UpdateConformanceProfileInput) (ConformanceProfile, error) {
	updateProfile := func(profile *ConformanceProfile) ConformanceProfile {
		profile.ExpectedEvidence = append([]ExpectedEvidence(nil), input.ExpectedEvidence...)
		profile.Actor = input.Actor
		profile.Reason = input.Reason
		profile.UpdatedAt = input.UpdatedAt
		return *profile
	}
	for idx := range s.conformanceProfiles {
		if s.conformanceProfiles[idx].Scope == input.Scope && s.conformanceProfiles[idx].ID == input.ProfileID {
			return updateProfile(&s.conformanceProfiles[idx]), nil
		}
	}
	for idx := range s.createdConformanceProfiles {
		if s.createdConformanceProfiles[idx].Scope == input.Scope && s.createdConformanceProfiles[idx].ID == input.ProfileID {
			return updateProfile(&s.createdConformanceProfiles[idx]), nil
		}
	}
	return ConformanceProfile{}, nil
}

func (s *stubAssuranceStore) DisableConformanceProfile(ctx context.Context, input DisableConformanceProfileInput) (ConformanceProfile, error) {
	updateDisabled := func(profile *ConformanceProfile) ConformanceProfile {
		profile.Status = ConformanceProfileStatusDisabled
		profile.Actor = input.Actor
		profile.Reason = input.Reason
		profile.UpdatedAt = input.DisabledAt
		profile.DisabledAt = input.DisabledAt
		return *profile
	}
	for idx := range s.conformanceProfiles {
		if s.conformanceProfiles[idx].Scope == input.Scope && s.conformanceProfiles[idx].ID == input.ProfileID {
			return updateDisabled(&s.conformanceProfiles[idx]), nil
		}
	}
	for idx := range s.createdConformanceProfiles {
		if s.createdConformanceProfiles[idx].Scope == input.Scope && s.createdConformanceProfiles[idx].ID == input.ProfileID {
			return updateDisabled(&s.createdConformanceProfiles[idx]), nil
		}
	}
	return ConformanceProfile{}, nil
}

func (s *stubAssuranceStore) InspectConformanceEvidence(ctx context.Context, input ConformanceEvidenceInspectionInput) ([]ConformanceEvidenceObservation, error) {
	s.inspectInputs = append(s.inspectInputs, input)
	return append([]ConformanceEvidenceObservation(nil), s.conformanceEvidence...), nil
}

func (s *stubAssuranceStore) CreateConformanceRun(ctx context.Context, run ConformanceRun) (ConformanceRun, error) {
	s.createdConformanceRuns = append(s.createdConformanceRuns, run)
	return run, nil
}

func (s *stubAssuranceStore) ListConformanceRuns(ctx context.Context, input ListConformanceRunsInput) ([]ConformanceRun, error) {
	runs := append(append([]ConformanceRun{}, s.conformanceRuns...), s.createdConformanceRuns...)
	filtered := make([]ConformanceRun, 0, len(runs))
	for _, run := range runs {
		if run.Scope != input.Scope {
			continue
		}
		if input.ProfileID != "" && run.ProfileID != input.ProfileID {
			continue
		}
		filtered = append(filtered, run)
	}
	return filtered, nil
}

func (s *stubAssuranceStore) ReadConformanceRun(ctx context.Context, input ReadConformanceRunInput) (ConformanceRun, error) {
	for _, run := range append(append([]ConformanceRun{}, s.conformanceRuns...), s.createdConformanceRuns...) {
		if run.Scope == input.Scope && run.ID == input.RunID {
			return run, nil
		}
	}
	return ConformanceRun{}, nil
}

func (s *stubAssuranceStore) CreateMissingEvidenceDiagnostic(ctx context.Context, diagnostic MissingEvidenceDiagnostic) (MissingEvidenceDiagnostic, error) {
	s.createdMissingEvidenceDiagnostics = append(s.createdMissingEvidenceDiagnostics, diagnostic)
	return diagnostic, nil
}

func (s *stubAssuranceStore) ListOperationalProofs(ctx context.Context, scope memory.Scope) ([]OperationalProof, error) {
	filtered := make([]OperationalProof, 0, len(s.operationalProofs))
	for _, proof := range s.operationalProofs {
		if proof.Scope == scope {
			filtered = append(filtered, proof)
		}
	}
	return filtered, nil
}

func (s *stubAssuranceStore) CreateReadinessReport(ctx context.Context, report ReadinessReport) (ReadinessReport, error) {
	s.createdReadinessReports = append(s.createdReadinessReports, report)
	return report, nil
}

func (s *stubAssuranceStore) ListReadinessReports(ctx context.Context, scope memory.Scope) ([]ReadinessReport, error) {
	reports := append(append([]ReadinessReport{}, s.readinessReports...), s.createdReadinessReports...)
	filtered := make([]ReadinessReport, 0, len(reports))
	for _, report := range reports {
		if report.Scope == scope {
			filtered = append(filtered, report)
		}
	}
	return filtered, nil
}

func (s *stubAssuranceStore) ReadReadinessReport(ctx context.Context, input ReadReadinessReportInput) (ReadinessReport, error) {
	for _, report := range append(append([]ReadinessReport{}, s.readinessReports...), s.createdReadinessReports...) {
		if report.Scope == input.Scope && report.ID == input.ReportID {
			return report, nil
		}
	}
	return ReadinessReport{}, nil
}

func (s *stubAssuranceStore) CreateRecoveryVerification(ctx context.Context, verification RecoveryVerification) (RecoveryVerification, error) {
	s.createdRecoveryVerifications = append(s.createdRecoveryVerifications, verification)
	return verification, nil
}

func (s *stubAssuranceStore) ListRecoveryVerifications(ctx context.Context, scope memory.Scope) ([]RecoveryVerification, error) {
	verifications := append(append([]RecoveryVerification{}, s.recoveryVerifications...), s.createdRecoveryVerifications...)
	filtered := make([]RecoveryVerification, 0, len(verifications))
	for _, verification := range verifications {
		if verification.Scope == scope {
			filtered = append(filtered, verification)
		}
	}
	return filtered, nil
}

func (s *stubAssuranceStore) ReadRecoveryVerification(ctx context.Context, input ReadRecoveryVerificationInput) (RecoveryVerification, error) {
	for _, verification := range append(append([]RecoveryVerification{}, s.recoveryVerifications...), s.createdRecoveryVerifications...) {
		if verification.Scope == input.Scope && verification.ID == input.RecordID {
			return verification, nil
		}
	}
	return RecoveryVerification{}, nil
}

func (s *stubAssuranceStore) CreateRetentionRun(ctx context.Context, run RetentionRun) (RetentionRun, error) {
	s.createdRetentionRuns = append(s.createdRetentionRuns, run)
	return run, nil
}
