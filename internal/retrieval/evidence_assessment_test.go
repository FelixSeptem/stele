package retrieval

import "testing"

func TestAssessRetrievalEvidence(t *testing.T) {
	tests := []struct {
		name   string
		input  EvidenceAssessmentInput
		want   EvidenceDisposition
		follow bool
	}{
		{name: "sufficient", input: EvidenceAssessmentInput{VisibleCandidates: 5, MinimumVisible: 3, RemainingCandidates: 10}, want: EvidenceDispositionSufficient},
		{name: "zero hits", input: EvidenceAssessmentInput{MinimumVisible: 3, RemainingCandidates: 10}, want: EvidenceDispositionZeroHits, follow: true},
		{name: "below minimum", input: EvidenceAssessmentInput{VisibleCandidates: 2, MinimumVisible: 3, RemainingCandidates: 10}, want: EvidenceDispositionBelowMinimum, follow: true},
		{name: "high attrition", input: EvidenceAssessmentInput{VisibleCandidates: 5, MinimumVisible: 3, FilteredCandidates: 8, RecalledCandidates: 10, RemainingCandidates: 10}, want: EvidenceDispositionHighAttrition, follow: true},
		{name: "required channel unavailable", input: EvidenceAssessmentInput{VisibleCandidates: 5, MinimumVisible: 3, RequiredChannelUnavailable: true, RemainingCandidates: 10}, want: EvidenceDispositionChannelUnavailable, follow: true},
		{name: "no headroom", input: EvidenceAssessmentInput{VisibleCandidates: 1, MinimumVisible: 3}, want: EvidenceDispositionNoHeadroom},
		{name: "second pass terminal", input: EvidenceAssessmentInput{Pass: 2, VisibleCandidates: 1, MinimumVisible: 3, RemainingCandidates: 10}, want: EvidenceDispositionTerminalIncomplete},
		{name: "second pass high attrition is terminal", input: EvidenceAssessmentInput{Pass: 2, VisibleCandidates: 5, MinimumVisible: 3, FilteredCandidates: 8, RecalledCandidates: 10, RemainingCandidates: 10}, want: EvidenceDispositionTerminalIncomplete},
		{name: "second pass zero hits with zero minimum is terminal", input: EvidenceAssessmentInput{Pass: 2, MinimumVisible: 0, RemainingCandidates: 10}, want: EvidenceDispositionTerminalIncomplete},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			assessment, err := AssessRetrievalEvidence(test.input)
			if err != nil {
				t.Fatalf("AssessRetrievalEvidence() error = %v", err)
			}
			if assessment.Disposition != test.want || assessment.FollowUpEligible != test.follow {
				t.Fatalf("assessment = %+v, want disposition %q follow-up %t", assessment, test.want, test.follow)
			}
		})
	}
}

func TestAssessRetrievalEvidenceRejectsInvalidCounts(t *testing.T) {
	for _, input := range []EvidenceAssessmentInput{
		{Pass: 3, MinimumVisible: 1},
		{Pass: 1, VisibleCandidates: -1, MinimumVisible: 1},
		{Pass: 1, VisibleCandidates: 2, RecalledCandidates: 1, MinimumVisible: 1},
		{Pass: 1, FilteredCandidates: 2, RecalledCandidates: 1, MinimumVisible: 1},
	} {
		if _, err := AssessRetrievalEvidence(input); err == nil {
			t.Fatalf("AssessRetrievalEvidence(%+v) error = nil", input)
		}
	}
}
