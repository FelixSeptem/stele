package assurance

import (
	"strings"
	"testing"
	"time"

	"github.com/FelixSeptem/stele/internal/memory"
)

func TestHealthEvaluationValidationCoversOperationalProofComponents(t *testing.T) {
	now := time.Date(2026, 7, 12, 10, 0, 0, 0, time.UTC)
	evaluation := HealthEvaluation{
		ID:       "health_1",
		Scope:    memory.Scope{Tenant: "tenant-a", Project: "project-a", Namespace: "namespace-a"},
		Status:   HealthStatusDegraded,
		Severity: SeverityWarning,
		Components: []HealthComponentSummary{
			{
				ID:           "component_1",
				EvaluationID: "health_1",
				Scope:        memory.Scope{Tenant: "tenant-a", Project: "project-a", Namespace: "namespace-a"},
				Component:    ComponentCapacityLoad,
				Status:       HealthStatusHealthy,
				Severity:     SeverityInfo,
				Reason:       ReasonCapacityWithinThresholds,
				ObservedAt:   now,
				FreshThrough: now.Add(15 * time.Minute),
				Evidence:     map[string]any{"backlog_depth": 2},
			},
			{
				ID:           "component_2",
				EvaluationID: "health_1",
				Scope:        memory.Scope{Tenant: "tenant-a", Project: "project-a", Namespace: "namespace-a"},
				Component:    ComponentBackupRestore,
				Status:       HealthStatusDegraded,
				Severity:     SeverityWarning,
				Reason:       ReasonBackupRestoreStale,
				ObservedAt:   now,
				FreshThrough: now.Add(-time.Hour),
				Evidence:     map[string]any{"marker": "restore-check-1"},
			},
		},
		CreatedAt: now,
	}

	if err := evaluation.Validate(); err != nil {
		t.Fatalf("Validate() error = %v", err)
	}
}

func TestConformanceProfileRejectsUnsupportedEvidenceKind(t *testing.T) {
	profile := ConformanceProfile{
		ID:     "profile_1",
		Scope:  memory.Scope{Tenant: "tenant-a", Project: "project-a", Namespace: "namespace-a"},
		Status: ConformanceProfileStatusActive,
		ExpectedEvidence: []ExpectedEvidence{
			{Kind: ExpectedEvidenceKind("free_form"), MinimumCount: 1, FreshnessWindow: time.Hour},
		},
		Actor:     "admin-a",
		Reason:    "prove integration",
		CreatedAt: time.Date(2026, 7, 12, 10, 0, 0, 0, time.UTC),
		UpdatedAt: time.Date(2026, 7, 12, 10, 0, 0, 0, time.UTC),
	}

	err := profile.Validate()
	if err == nil {
		t.Fatal("Validate() error = nil, want unsupported evidence kind error")
	}
	if !strings.Contains(err.Error(), "evidence kind") {
		t.Fatalf("Validate() error = %v, want evidence kind error", err)
	}
}

func TestAlertDeliveryConfigRejectsUnsafeWebhook(t *testing.T) {
	cfg := AlertDeliveryConfig{
		Mode:            AlertAdapterWebhook,
		WebhookURL:      "http://169.254.169.254/latest/meta-data",
		Timeout:         5 * time.Second,
		MaxPayloadBytes: 4096,
	}

	err := cfg.Validate()
	if err == nil {
		t.Fatal("Validate() error = nil, want unsafe webhook target error")
	}
	if !strings.Contains(err.Error(), "unsafe webhook") {
		t.Fatalf("Validate() error = %v, want unsafe webhook target error", err)
	}
}

func TestBoundedEnumsValidate(t *testing.T) {
	validEnums := []struct {
		name string
		ok   bool
	}{
		{"readiness", ReadinessStatusReady.Valid()},
		{"incident_status", IncidentStatusOpen.Valid()},
		{"incident_action", IncidentActionAcknowledge.Valid()},
		{"alert_result", AlertDeliveryResultRetry.Valid()},
		{"missing_evidence", MissingEvidenceSessionWithoutOutcome.Valid()},
		{"conformance_result", ConformanceResultDegraded.Valid()},
		{"operational_proof", OperationalProofBackupRestore.Valid()},
		{"recovery_target", RecoveryVerificationTargetCapacityLoadProof.Valid()},
		{"runbook_hint", RunbookHintReviewBackupRestoreProof.Valid()},
	}
	for _, enum := range validEnums {
		if !enum.ok {
			t.Fatalf("%s enum did not validate", enum.name)
		}
	}

	if ReadinessStatus("ready for anything").Valid() {
		t.Fatal("free-form readiness status validated, want rejection")
	}
	if MissingEvidenceCategory("arbitrary_gap").Valid() {
		t.Fatal("free-form missing evidence category validated, want rejection")
	}
}

func TestDiagnosticRecordValidationRequiresScopeAndBoundedFields(t *testing.T) {
	now := time.Date(2026, 7, 12, 11, 0, 0, 0, time.UTC)
	scope := memory.Scope{Tenant: "tenant-a", Project: "project-a", Namespace: "namespace-a"}

	records := []struct {
		name     string
		validate func() error
	}{
		{
			name: "incident",
			validate: func() error {
				return Incident{
					ID:               "incident_1",
					Scope:            scope,
					Status:           IncidentStatusOpen,
					Severity:         SeverityCritical,
					Component:        ComponentBackupRestore,
					Reason:           ReasonBackupRestoreStale,
					DeduplicationKey: "tenant-a:backup_restore",
					OpenedAt:         now,
					UpdatedAt:        now,
					RunbookHints:     []RunbookHintCategory{RunbookHintReviewBackupRestoreProof},
				}.Validate()
			},
		},
		{
			name: "alert candidate",
			validate: func() error {
				return AlertCandidate{
					ID:               "alert_1",
					Scope:            scope,
					Severity:         SeverityWarning,
					Component:        ComponentCapacityLoad,
					Reason:           ReasonCapacityThresholdExceeded,
					DeduplicationKey: "tenant-a:capacity",
					DeliveryPolicy:   "default",
					Payload:          map[string]any{"component": "capacity_load"},
					CreatedAt:        now,
				}.Validate()
			},
		},
		{
			name: "operational proof",
			validate: func() error {
				return OperationalProof{
					ID:           "proof_1",
					Scope:        scope,
					Target:       OperationalProofCapacityLoad,
					Status:       HealthStatusHealthy,
					Severity:     SeverityInfo,
					Reason:       ReasonCapacityWithinThresholds,
					ObservedAt:   now,
					FreshThrough: now.Add(time.Hour),
					Evidence:     map[string]any{"worker_latency_ms": 120},
					CreatedAt:    now,
				}.Validate()
			},
		},
		{
			name: "readiness report",
			validate: func() error {
				return ReadinessReport{
					ID:                 "readiness_1",
					Scope:              scope,
					Status:             ReadinessStatusDegraded,
					HealthEvaluationID: "health_1",
					ComponentSummary:   map[string]any{"backup_restore": "stale"},
					RecommendedActions: []RunbookHintCategory{RunbookHintReviewBackupRestoreProof},
					GeneratedAt:        now,
					CreatedAt:          now,
				}.Validate()
			},
		},
		{
			name: "recovery verification",
			validate: func() error {
				return RecoveryVerification{
					ID:              "recovery_1",
					Scope:           scope,
					Target:          RecoveryVerificationTargetIncident,
					TargetID:        "incident_1",
					Status:          HealthStatusHealthy,
					CheckedSurfaces: []string{"health_evaluation"},
					ResultCategory:  "recovered",
					LinkedEvidence:  map[string]any{"health_evaluation_id": "health_2"},
					Actor:           "admin-a",
					Reason:          "verified after restore drill",
					CreatedAt:       now,
					VerifiedAt:      now,
				}.Validate()
			},
		},
	}

	for _, record := range records {
		if err := record.validate(); err != nil {
			t.Fatalf("%s Validate() error = %v", record.name, err)
		}
	}
}

func TestDiagnosticRecordValidationRejectsUnboundedMetadata(t *testing.T) {
	now := time.Date(2026, 7, 12, 11, 0, 0, 0, time.UTC)
	scope := memory.Scope{Tenant: "tenant-a", Project: "project-a", Namespace: "namespace-a"}
	payload := make(map[string]any)
	for i := 0; i < 40; i++ {
		payload["key_"+string(rune('a'+i))] = "value"
	}

	err := AlertCandidate{
		ID:               "alert_1",
		Scope:            scope,
		Severity:         SeverityWarning,
		Component:        ComponentCapacityLoad,
		Reason:           ReasonCapacityThresholdExceeded,
		DeduplicationKey: "tenant-a:capacity",
		DeliveryPolicy:   "default",
		Payload:          payload,
		CreatedAt:        now,
	}.Validate()
	if err == nil {
		t.Fatal("Validate() error = nil, want payload bounds error")
	}
}

func TestValidateIdempotencyKeyBounds(t *testing.T) {
	if err := ValidateIdempotencyKey("assurance-job-1"); err != nil {
		t.Fatalf("ValidateIdempotencyKey() error = %v", err)
	}
	if err := ValidateIdempotencyKey(strings.Repeat("x", 257)); err == nil {
		t.Fatal("ValidateIdempotencyKey() error = nil, want length validation")
	}
}
