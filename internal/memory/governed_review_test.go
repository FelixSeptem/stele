package memory

import (
	"context"
	"testing"
)

func TestReflectionReviewServiceRecordsDecision(t *testing.T) {
	processor := &reviewProcessorStub{}
	service := ReflectionReviewService{Processor: processor}
	decision, err := service.Decide(context.Background(), ReflectionReviewInput{
		Scope:       Scope{Tenant: "t1", Project: "p1", Namespace: "n1"},
		CandidateID: "cand-1", Decision: ReflectionReviewAccept, Reviewer: "admin", Reason: "evidence verified", PolicyVersion: "policy-v1",
	})
	if err != nil {
		t.Fatalf("Decide() error = %v", err)
	}
	if decision.Decision != ReflectionReviewAccept || processor.received.CandidateID != "cand-1" {
		t.Fatalf("decision = %+v received = %+v", decision, processor.received)
	}
}

func TestReflectionReviewServiceAcceptPromotesCandidateThroughGovernedIntegrator(t *testing.T) {
	processor := &reviewProcessorStub{}
	service := ReflectionReviewService{Processor: processor, Integrator: processor}
	_, err := service.Decide(context.Background(), ReflectionReviewInput{
		Scope: Scope{Tenant: "t1", Project: "p1", Namespace: "n1"}, CandidateID: "cand-1", Decision: ReflectionReviewAccept,
		Reviewer: "admin", Reason: "verified", PolicyVersion: "policy-v1",
	})
	if err != nil {
		t.Fatalf("Decide() error = %v", err)
	}
	if !processor.promoted {
		t.Fatal("accepted review did not invoke canonical integrator")
	}
}

type reviewProcessorStub struct {
	received ReflectionReviewRecord
	promoted bool
}

func (s *reviewProcessorStub) PromoteReviewedCandidate(_ context.Context, _ ReviewedCandidatePromotionInput) error {
	s.promoted = true
	return nil
}

func (s *reviewProcessorStub) AppendReflectionReview(_ context.Context, record ReflectionReviewRecord) (ReflectionReviewRecord, error) {
	s.received = record
	return record, nil
}
