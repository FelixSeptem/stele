package memory

import (
	"strings"
	"testing"
	"time"
)

func TestProceduralVerificationSupportedRequiresIndependentCorroboration(t *testing.T) {
	verification := validProceduralMemoryVerification()
	verification.Disposition = ProceduralVerificationDispositionSupported
	verification.Evidence = []ProceduralVerificationEvidence{{
		Kind: ProceduralVerificationEvidenceTranscript, SourceKind: ProceduralVerificationEvidenceSourceRawEvent,
		ID:            "event-1",
		SourceVersion: 1,
		Corroborates:  false,
	}}

	err := verification.Validate()
	if err == nil || !strings.Contains(err.Error(), "independent corroboration") {
		t.Fatalf("Validate() error = %v, want independent corroboration error", err)
	}
}

func TestProceduralVerificationFingerprintBindsTargetAndEvidenceVersion(t *testing.T) {
	left := validProceduralMemoryVerification()
	right := left
	right.Evidence = append([]ProceduralVerificationEvidence(nil), left.Evidence...)
	right.Evidence[0].SourceVersion++

	leftFingerprint, err := ProceduralMemoryVerificationFingerprint(left)
	if err != nil {
		t.Fatalf("left fingerprint: %v", err)
	}
	rightFingerprint, err := ProceduralMemoryVerificationFingerprint(right)
	if err != nil {
		t.Fatalf("right fingerprint: %v", err)
	}
	if leftFingerprint == rightFingerprint {
		t.Fatal("fingerprint did not change when evidence source version changed")
	}
}

func TestProceduralVerificationRefutedAllowsCorrectionButOtherDispositionsRejectIt(t *testing.T) {
	verification := validProceduralMemoryVerification()
	verification.CorrectedContent = "Wait for confirmation before retrying the command."

	for _, disposition := range []ProceduralVerificationDisposition{
		ProceduralVerificationDispositionSupported,
		ProceduralVerificationDispositionInconclusive,
		ProceduralVerificationDispositionNeedsRevalidation,
	} {
		candidate := verification
		candidate.Disposition = disposition
		if err := candidate.Validate(); err == nil || !strings.Contains(err.Error(), "corrected content") {
			t.Fatalf("disposition %q error = %v, want corrected-content error", disposition, err)
		}
	}

	verification.Disposition = ProceduralVerificationDispositionRefuted
	if err := verification.Validate(); err != nil {
		t.Fatalf("refuted verification Validate() error = %v", err)
	}
}

func TestProceduralVerificationRejectsOutOfScopeEvidence(t *testing.T) {
	verification := validProceduralMemoryVerification()
	verification.Evidence[0].Scope = Scope{Tenant: "other", Project: verification.Scope.Project, Namespace: verification.Scope.Namespace}

	err := verification.Validate()
	if err == nil || !strings.Contains(err.Error(), "scope") {
		t.Fatalf("Validate() error = %v, want scope error", err)
	}
}

func TestProceduralVerificationAllowsInconclusiveAndNeedsRevalidationWithoutCorroboration(t *testing.T) {
	for _, disposition := range []ProceduralVerificationDisposition{
		ProceduralVerificationDispositionInconclusive,
		ProceduralVerificationDispositionNeedsRevalidation,
	} {
		verification := validProceduralMemoryVerification()
		verification.Disposition = disposition
		verification.Evidence = []ProceduralVerificationEvidence{{
			Kind: ProceduralVerificationEvidenceTranscript, SourceKind: ProceduralVerificationEvidenceSourceRawEvent, ID: "transcript-1", SourceVersion: 1,
		}}
		if err := verification.Validate(); err != nil {
			t.Fatalf("%s verification Validate() error = %v", disposition, err)
		}
	}
}

func TestProceduralVerificationModelAssertionNeverCountsAsIndependentCorroboration(t *testing.T) {
	verification := validProceduralMemoryVerification()
	verification.Evidence = []ProceduralVerificationEvidence{{
		Kind: ProceduralVerificationEvidenceModelAssertion, SourceKind: ProceduralVerificationEvidenceSourceRawEvent, ID: "model-1", SourceVersion: 1, Corroborates: true,
	}}

	err := verification.Validate()
	if err == nil || !strings.Contains(err.Error(), "independent corroboration") {
		t.Fatalf("Validate() error = %v, want independent corroboration error", err)
	}
}

func TestProceduralVerificationRejectsInvalidFingerprintAndOversizedReason(t *testing.T) {
	verification := validProceduralMemoryVerification()
	verification.ClaimFingerprint = "not-a-digest"
	if err := verification.Validate(); err == nil || !strings.Contains(err.Error(), "SHA-256") {
		t.Fatalf("invalid fingerprint error = %v", err)
	}

	verification = validProceduralMemoryVerification()
	verification.Reason = strings.Repeat("x", ProceduralVerificationMaxReasonBytes+1)
	if err := verification.Validate(); err == nil || !strings.Contains(err.Error(), "reason exceeds") {
		t.Fatalf("oversized reason error = %v", err)
	}
}

func TestProceduralVerificationFingerprintNormalizesEvidenceOrder(t *testing.T) {
	left := validProceduralMemoryVerification()
	left.Evidence = append(left.Evidence, ProceduralVerificationEvidence{
		Kind: ProceduralVerificationEvidenceExternalEvaluation, SourceKind: ProceduralVerificationEvidenceSourceExternalEvaluation, ID: "evaluation-1", SourceVersion: 2, Corroborates: true,
	})
	right := left
	right.Evidence = append([]ProceduralVerificationEvidence(nil), left.Evidence...)
	right.Evidence[0], right.Evidence[1] = right.Evidence[1], right.Evidence[0]

	leftFingerprint, err := ProceduralMemoryVerificationFingerprint(left)
	if err != nil {
		t.Fatalf("left fingerprint: %v", err)
	}
	rightFingerprint, err := ProceduralMemoryVerificationFingerprint(right)
	if err != nil {
		t.Fatalf("right fingerprint: %v", err)
	}
	if leftFingerprint != rightFingerprint {
		t.Fatalf("equivalent evidence order fingerprints differ: %q != %q", leftFingerprint, rightFingerprint)
	}
}

func TestProceduralRevalidationRunRequiresBoundedDurableIdentity(t *testing.T) {
	run := ProceduralRevalidationRun{
		ID: "run-1", Scope: Scope{Tenant: "tenant-a", Project: "project-a", Namespace: "namespace-a"},
		VerificationID: "verification-1", MemoryID: "memory-1", TargetVersion: 3,
		SourceWatermark: "watermark-1", Trigger: ProceduralRevalidationTriggerAdministrator,
		WorkIdentity: "work-1", Status: ProceduralRevalidationRunStatusPending, MaxAttempts: 3,
		CreatedAt: time.Date(2026, time.November, 8, 0, 0, 0, 0, time.UTC),
	}
	if err := run.Validate(); err != nil {
		t.Fatalf("Validate() error = %v", err)
	}

	run.MaxAttempts = 0
	if err := run.Validate(); err == nil || !strings.Contains(err.Error(), "max attempts") {
		t.Fatalf("invalid max attempts error = %v", err)
	}
}

func TestProceduralVerificationRequiresTypedEvidenceSource(t *testing.T) {
	verification := validProceduralMemoryVerification()
	verification.Evidence[0].SourceKind = ""
	if err := verification.Validate(); err == nil || !strings.Contains(err.Error(), "source kind") {
		t.Fatalf("Validate() error = %v, want source-kind error", err)
	}

	verification.Evidence[0].SourceKind = ProceduralVerificationEvidenceSourceRawEvent
	if err := verification.Validate(); err != nil {
		t.Fatalf("typed source Validate() error = %v", err)
	}
}

func validProceduralMemoryVerification() ProceduralMemoryVerification {
	return ProceduralMemoryVerification{
		ID:                       "verification-1",
		Scope:                    Scope{Tenant: "tenant-a", Project: "project-a", Namespace: "namespace-a"},
		MemoryID:                 "memory-1",
		TargetVersion:            3,
		ClaimFingerprint:         strings.Repeat("a", 64),
		ApplicabilityFingerprint: strings.Repeat("b", 64),
		Disposition:              ProceduralVerificationDispositionSupported,
		Evidence: []ProceduralVerificationEvidence{{
			Scope: Scope{Tenant: "tenant-a", Project: "project-a", Namespace: "namespace-a"},
			Kind:  ProceduralVerificationEvidenceToolObservation, SourceKind: ProceduralVerificationEvidenceSourceRawEvent,
			ID: "event-1", SourceVersion: 1, Corroborates: true,
		}},
		Actor:           "operator",
		Reason:          "verified against the observed tool result",
		PolicyVersion:   "procedural-verification-v1",
		SourceWatermark: "watermark-1",
		CreatedAt:       time.Date(2026, time.November, 8, 0, 0, 0, 0, time.UTC),
	}
}
