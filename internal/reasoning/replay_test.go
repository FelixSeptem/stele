package reasoning

import (
	"testing"
)

func TestReplayInsightCandidateDoesNotInvokeProviderAndIsDeterministic(t *testing.T) {
	request, _ := validInsightRequest(t)
	candidate := validInsightCandidate(t, request)
	replayID, err := InsightReplayID(request)
	if err != nil {
		t.Fatal(err)
	}
	envelope := InsightReplayEnvelope{Request: request, Candidate: candidate, ReplayID: replayID}
	first, err := ReplayInsightCandidate(envelope)
	if err != nil {
		t.Fatal(err)
	}
	second, err := ReplayInsightCandidate(envelope)
	if err != nil {
		t.Fatal(err)
	}
	if first.ReplayID != second.ReplayID || first.Disposition != InsightDispositionCandidate || first.Authoritative || second.Authoritative {
		t.Fatalf("replay results differ or became authoritative: first=%+v second=%+v", first, second)
	}
}
