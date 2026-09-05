package benchmark

import (
	"testing"

	"github.com/FelixSeptem/stele/internal/memory"
)

func TestFamilyReportRejectsIncompatibleInputsAndRendersNonComparableFamilies(t *testing.T) {
	manifest := validManifest()
	report := NewFamilyReport(FamilyAgentMemory, manifest, "smoke", memory.Scope{Tenant: "benchmark", Project: "p", Namespace: "n"})
	if err := report.ValidateAgainst(manifest); err != nil {
		t.Fatal(err)
	}
	changed := manifest
	changed.ConversionVersion = "other"
	if StatusOf(report.ValidateAgainst(changed)) != StatusInvalidManifest {
		t.Fatalf("expected incompatible report inputs to fail")
	}
	if comparable := CanCompareFamilyReports(report, NewFamilyReport(FamilyStress, manifest, "smoke", memory.Scope{Tenant: "benchmark", Project: "p", Namespace: "n"})); comparable {
		t.Fatal("memory and stress reports must not compare")
	}
}

func TestFamilyReportCarriesRuntimeStrategyAndAllInputChecksums(t *testing.T) {
	manifest := validManifest()
	report := NewFamilyReport(FamilyAgentMemory, manifest, "smoke", memory.Scope{Tenant: "benchmark", Project: "p", Namespace: "n"}).WithExecutionProvenance(FamilyReportExecution{
		QRELVersion: "qrels-v1", StrategyProfile: "hybrid-rank-v1",
		InputChecksums: map[string]string{"raw": manifest.SHA256, "normalized": "normalized-v1", "qrels": manifest.QRELChecksum},
		Runtime:        RuntimeIdentity{SteleRevision: "test-revision", PostgreSQL: "18.1", PGVector: "0.8.0"},
	})
	if report.QRELVersion != "qrels-v1" || report.StrategyProfile != "hybrid-rank-v1" || report.Runtime.PGVector != "0.8.0" || report.InputChecksums["normalized"] != "normalized-v1" {
		t.Fatalf("report dropped execution provenance: %#v", report)
	}
	other := report
	other.StrategyProfile = "lexical-v1"
	if CanCompareFamilyReports(report, other) {
		t.Fatal("different strategy profiles must not be combined")
	}
}

func TestRenderFamilyReportStatesFamilyBoundariesAndStressNonGating(t *testing.T) {
	manifest := validManifest()
	report := NewFamilyReport(FamilyStress, manifest, "smoke", memory.Scope{Tenant: "benchmark", Project: "p", Namespace: "n"})
	summary := RenderFamilyReport(report)
	if summary.GateClassification != "non-gating" || len(summary.NotComparableWith) != 4 {
		t.Fatalf("stress report must render explicit family boundary: %#v", summary)
	}
	for _, family := range summary.NotComparableWith {
		if family == FamilyStress {
			t.Fatalf("report cannot list itself as incomparable: %#v", summary)
		}
	}
}
