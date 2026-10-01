package retrieval

import (
	"testing"

	"github.com/FelixSeptem/stele/internal/evaluation"
	"github.com/FelixSeptem/stele/internal/memory"
)

func TestReleaseEvidenceRejectsIntegrityFailureEvenWhenQualityPasses(t *testing.T) {
	report := ReleaseEvidenceReport{Verdict: ReleaseEvidencePassed, ReleaseEligible: true, QualityEligible: true}
	report = ApplyIntegrityEvidence(report, &evaluation.IntegrityReport{Scope: memory.Scope{Tenant: "tenant-a", Project: "project-a", Namespace: "namespace-a"}, IntegritySuccess: false, Verdict: evaluation.VerdictRejected})
	if report.ReleaseEligible || report.Verdict == ReleaseEvidencePassed {
		t.Fatalf("integrity failure must block release: %#v", report)
	}
}
