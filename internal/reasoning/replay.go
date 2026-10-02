package reasoning

import "fmt"

// InsightReplayEnvelope is the complete normalized input needed for an
// offline replay. It contains no provider prompt or raw response payload.
type InsightReplayEnvelope struct {
	Request   InsightDerivationRequest `json:"request"`
	Candidate InsightCandidate         `json:"candidate"`
	ReplayID  string                   `json:"replay_id"`
}

// ReplayInsightCandidate validates a previously normalized envelope without
// invoking a provider or changing active state. It is the deterministic path
// used by offline reports and durable replay planning.
func ReplayInsightCandidate(envelope InsightReplayEnvelope) (InsightDerivationResult, error) {
	if envelope.Request.Mode != ModeOffline {
		return InsightDerivationResult{}, fmt.Errorf("offline replay requires offline mode")
	}
	replayID, err := InsightReplayID(envelope.Request)
	if err != nil {
		return InsightDerivationResult{}, err
	}
	if envelope.ReplayID != "" && envelope.ReplayID != replayID {
		return InsightDerivationResult{}, fmt.Errorf("replay envelope identity does not match request")
	}
	if err := ValidateInsightCandidate(envelope.Request, envelope.Candidate); err != nil {
		return InsightDerivationResult{Disposition: InsightDispositionQuarantined, ReplayID: replayID, Reason: err.Error(), Authoritative: false}, nil
	}
	return InsightDerivationResult{Disposition: InsightDispositionCandidate, Candidate: envelope.Candidate, ReplayID: replayID, Authoritative: false}, nil
}
