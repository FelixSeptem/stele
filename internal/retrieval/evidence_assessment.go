package retrieval

import "fmt"

type EvidenceDisposition string

const (
	EvidenceDispositionSufficient         EvidenceDisposition = "sufficient"
	EvidenceDispositionZeroHits           EvidenceDisposition = "zero_hits"
	EvidenceDispositionBelowMinimum       EvidenceDisposition = "below_minimum"
	EvidenceDispositionHighAttrition      EvidenceDisposition = "high_attrition"
	EvidenceDispositionChannelUnavailable EvidenceDisposition = "channel_unavailable"
	EvidenceDispositionNoHeadroom         EvidenceDisposition = "no_headroom"
	EvidenceDispositionTerminalIncomplete EvidenceDisposition = "terminal_incomplete"
)

type EvidenceAssessmentInput struct {
	Pass                       int
	VisibleCandidates          int
	MinimumVisible             int
	RecalledCandidates         int
	FilteredCandidates         int
	DuplicateCandidates        int
	RequiredChannelUnavailable bool
	RemainingCandidates        int
}

type EvidenceAssessment struct {
	Disposition      EvidenceDisposition `json:"disposition"`
	FollowUpEligible bool                `json:"follow_up_eligible"`
	VisibleBucket    string              `json:"visible_bucket"`
	AttritionBucket  string              `json:"attrition_bucket"`
}

func AssessRetrievalEvidence(input EvidenceAssessmentInput) (EvidenceAssessment, error) {
	if input.Pass == 0 {
		input.Pass = 1
	}
	if input.Pass < 1 || input.Pass > 2 {
		return EvidenceAssessment{}, fmt.Errorf("evidence assessment pass must be one or two")
	}
	if input.VisibleCandidates < 0 || input.MinimumVisible < 0 || input.RecalledCandidates < 0 || input.FilteredCandidates < 0 || input.DuplicateCandidates < 0 || input.RemainingCandidates < 0 {
		return EvidenceAssessment{}, fmt.Errorf("evidence assessment counts must be non-negative")
	}
	if input.RecalledCandidates > 0 && (input.VisibleCandidates > input.RecalledCandidates || input.FilteredCandidates > input.RecalledCandidates || input.DuplicateCandidates > input.RecalledCandidates) {
		return EvidenceAssessment{}, fmt.Errorf("evidence assessment disposition counts exceed recalled candidates")
	}
	assessment := EvidenceAssessment{VisibleBucket: candidateCountBucket(input.VisibleCandidates), AttritionBucket: attritionBucket(input.FilteredCandidates, input.RecalledCandidates)}
	incomplete := input.VisibleCandidates == 0 || input.VisibleCandidates < input.MinimumVisible || input.RequiredChannelUnavailable || highEvidenceAttrition(input.FilteredCandidates, input.RecalledCandidates)
	if input.Pass == 2 && incomplete {
		assessment.Disposition = EvidenceDispositionTerminalIncomplete
		return assessment, nil
	}
	if input.VisibleCandidates == 0 {
		assessment.Disposition = EvidenceDispositionZeroHits
	} else if input.VisibleCandidates < input.MinimumVisible {
		assessment.Disposition = EvidenceDispositionBelowMinimum
	} else if input.RequiredChannelUnavailable {
		assessment.Disposition = EvidenceDispositionChannelUnavailable
	} else if highEvidenceAttrition(input.FilteredCandidates, input.RecalledCandidates) {
		assessment.Disposition = EvidenceDispositionHighAttrition
	} else {
		assessment.Disposition = EvidenceDispositionSufficient
		return assessment, nil
	}
	if input.RemainingCandidates == 0 {
		assessment.Disposition = EvidenceDispositionNoHeadroom
		return assessment, nil
	}
	assessment.FollowUpEligible = true
	return assessment, nil
}

func highEvidenceAttrition(filtered, recalled int) bool {
	return recalled > 0 && filtered*2 >= recalled
}

func candidateCountBucket(count int) string {
	switch {
	case count == 0:
		return "zero"
	case count <= 5:
		return "one_to_five"
	case count <= 20:
		return "six_to_twenty"
	default:
		return "over_twenty"
	}
}

func attritionBucket(filtered, recalled int) string {
	if recalled == 0 || filtered == 0 {
		return "none"
	}
	if filtered*2 < recalled {
		return "low"
	}
	return "high"
}
