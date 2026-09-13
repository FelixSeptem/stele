package assurance

import (
	"testing"
	"time"

	"github.com/FelixSeptem/stele/internal/memory"
)

func TestEvaluateMaintenanceConformancePassesCompleteSafeEvidence(t *testing.T) {
	scope := memory.Scope{Tenant: "t", Project: "p", Namespace: "n"}
	result, err := EvaluateMaintenanceConformance(MaintenanceConformanceInput{Scope: scope, ObservedAt: time.Unix(1, 0), Evidence: []MaintenanceEvidence{
		{Kind: MaintenanceEvidenceCoverage, Scope: scope, Status: MaintenanceEvidencePassed},
		{Kind: MaintenanceEvidenceLeaseRecovery, Scope: scope, Status: MaintenanceEvidencePassed},
		{Kind: MaintenanceEvidenceProjectionFreshness, Scope: scope, Status: MaintenanceEvidencePassed},
		{Kind: MaintenanceEvidenceRetentionSafety, Scope: scope, Status: MaintenanceEvidencePassed},
		{Kind: MaintenanceEvidenceTelemetryRedaction, Scope: scope, Status: MaintenanceEvidencePassed},
	}})
	if err != nil {
		t.Fatalf("EvaluateMaintenanceConformance() error = %v", err)
	}
	if result.Result != ConformanceResultPassed || len(result.Failures) != 0 {
		t.Fatalf("result = %+v, want passed", result)
	}
}

func TestEvaluateMaintenanceConformanceFailsClosedOnIntegrityFailure(t *testing.T) {
	scope := memory.Scope{Tenant: "t", Project: "p", Namespace: "n"}
	foreign := memory.Scope{Tenant: "other", Project: "p", Namespace: "n"}
	result, err := EvaluateMaintenanceConformance(MaintenanceConformanceInput{Scope: scope, ObservedAt: time.Unix(1, 0), ActionSucceeded: true, Evidence: []MaintenanceEvidence{
		{Kind: MaintenanceEvidenceCoverage, Scope: scope, Status: MaintenanceEvidencePassed},
		{Kind: MaintenanceEvidenceLeaseRecovery, Scope: scope, Status: MaintenanceEvidencePassed},
		{Kind: MaintenanceEvidenceProjectionFreshness, Scope: foreign, Status: MaintenanceEvidencePassed},
		{Kind: MaintenanceEvidenceRetentionSafety, Scope: scope, Status: MaintenanceEvidenceFailed, Category: MaintenanceFailureCanonicalTarget},
		{Kind: MaintenanceEvidenceTelemetryRedaction, Scope: scope, Status: MaintenanceEvidencePassed},
	}})
	if err != nil {
		t.Fatalf("EvaluateMaintenanceConformance() error = %v", err)
	}
	if result.Result != ConformanceResultFailed || len(result.Failures) == 0 {
		t.Fatalf("result = %+v, want hard failure", result)
	}
}
