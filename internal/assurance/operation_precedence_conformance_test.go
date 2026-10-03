package assurance

import (
	"testing"

	"github.com/FelixSeptem/stele/internal/memory"
)

func TestRunOperationPrecedenceConformanceIsBoundedAndDiagnostic(t *testing.T) {
	report, err := RunOperationPrecedenceConformance(memory.Scope{Tenant: "tenant", Project: "project", Namespace: "namespace"})
	if err != nil {
		t.Fatal(err)
	}
	if !report.Passed || len(report.Results) != 8 {
		t.Fatalf("report = %+v", report)
	}
	for _, result := range report.Results {
		if !result.Passed || result.Name == "" {
			t.Fatalf("fixture result = %+v", result)
		}
	}
}

func TestRunOperationPrecedenceConformanceRejectsInvalidScope(t *testing.T) {
	if _, err := RunOperationPrecedenceConformance(memory.Scope{Tenant: "tenant"}); err == nil {
		t.Fatal("invalid scope accepted")
	}
}
