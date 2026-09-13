package assurance

import (
	"fmt"
	"time"

	"github.com/FelixSeptem/stele/internal/memory"
)

type MaintenanceEvidenceKind string

const (
	MaintenanceEvidenceCoverage            MaintenanceEvidenceKind = "coverage"
	MaintenanceEvidenceLeaseRecovery       MaintenanceEvidenceKind = "lease_recovery"
	MaintenanceEvidenceProjectionFreshness MaintenanceEvidenceKind = "projection_freshness"
	MaintenanceEvidenceRetentionSafety     MaintenanceEvidenceKind = "retention_safety"
	MaintenanceEvidenceTelemetryRedaction  MaintenanceEvidenceKind = "telemetry_redaction"
)

type MaintenanceEvidenceStatus string

const (
	MaintenanceEvidencePassed MaintenanceEvidenceStatus = "passed"
	MaintenanceEvidenceFailed MaintenanceEvidenceStatus = "failed"
)

type MaintenanceFailureCategory string

const (
	MaintenanceFailureMissing            MaintenanceFailureCategory = "missing_evidence"
	MaintenanceFailureForeignScope       MaintenanceFailureCategory = "foreign_scope"
	MaintenanceFailureCanonicalTarget    MaintenanceFailureCategory = "canonical_target"
	MaintenanceFailureSensitiveTelemetry MaintenanceFailureCategory = "sensitive_telemetry"
	MaintenanceFailureUnsafe             MaintenanceFailureCategory = "unsafe"
)

type MaintenanceEvidence struct {
	Kind     MaintenanceEvidenceKind
	Scope    memory.Scope
	Status   MaintenanceEvidenceStatus
	Category MaintenanceFailureCategory
}

type MaintenanceConformanceInput struct {
	Scope           memory.Scope
	ObservedAt      time.Time
	ActionSucceeded bool
	Evidence        []MaintenanceEvidence
}

type MaintenanceConformanceResult struct {
	Scope           memory.Scope
	Result          ConformanceResult
	Failures        []MaintenanceFailureCategory
	ActionSucceeded bool
	ObservedAt      time.Time
}

func EvaluateMaintenanceConformance(input MaintenanceConformanceInput) (MaintenanceConformanceResult, error) {
	if err := input.Scope.Validate(); err != nil {
		return MaintenanceConformanceResult{}, err
	}
	if input.ObservedAt.IsZero() {
		return MaintenanceConformanceResult{}, fmt.Errorf("maintenance conformance observed at is required")
	}
	result := MaintenanceConformanceResult{Scope: input.Scope.Normalized(), Result: ConformanceResultPassed, ActionSucceeded: input.ActionSucceeded, ObservedAt: input.ObservedAt}
	required := []MaintenanceEvidenceKind{MaintenanceEvidenceCoverage, MaintenanceEvidenceLeaseRecovery, MaintenanceEvidenceProjectionFreshness, MaintenanceEvidenceRetentionSafety, MaintenanceEvidenceTelemetryRedaction}
	seen := make(map[MaintenanceEvidenceKind]bool, len(input.Evidence))
	for _, evidence := range input.Evidence {
		if evidence.Scope.Normalized() != input.Scope.Normalized() {
			result.Failures = append(result.Failures, MaintenanceFailureForeignScope)
			continue
		}
		seen[evidence.Kind] = true
		if evidence.Status != MaintenanceEvidencePassed {
			if evidence.Category == "" {
				evidence.Category = MaintenanceFailureUnsafe
			}
			result.Failures = append(result.Failures, evidence.Category)
		}
	}
	for _, kind := range required {
		if !seen[kind] {
			result.Failures = append(result.Failures, MaintenanceFailureMissing)
		}
	}
	if len(result.Failures) > 0 {
		result.Result = ConformanceResultFailed
	}
	return result, nil
}
