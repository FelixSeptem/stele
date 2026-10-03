package retrieval

import (
	"context"
	"testing"

	"github.com/FelixSeptem/stele/internal/memory"
)

type goalContextReaderStub struct{ calls int }

func (s *goalContextReaderStub) ReadGoalContext(context.Context, memory.Scope, int) ([]GoalContextItem, string, error) {
	s.calls++
	return []GoalContextItem{{Title: "bounded", Summary: "approved", State: "proposed", ReviewState: "review_approved", PolicyVersion: "goal-visibility-v1"}}, "eligible", nil
}

func TestGoalContextIsIndependentOptIn(t *testing.T) {
	reader := &goalContextReaderStub{}
	svc := NewService(ServiceDependencies{GoalContext: reader})
	base := AssembleContextInput{Scope: memory.Scope{Tenant: "t", Project: "p", Namespace: "n"}, Query: "q", Budget: 1}
	// No lexical backend is needed: the goal section must remain absent by default.
	got, err := svc.AssembleContext(context.Background(), base)
	if err != nil {
		t.Fatal(err)
	}
	if len(got.GoalContext) != 0 || reader.calls != 0 {
		t.Fatalf("ordinary context exposed goal context: %+v calls=%d", got.GoalContext, reader.calls)
	}
	base.IncludeGoalContext = true
	got, err = svc.AssembleContext(context.Background(), base)
	if err != nil {
		t.Fatal(err)
	}
	if len(got.GoalContext) != 1 || reader.calls != 1 {
		t.Fatalf("opt-in goal context missing: %+v calls=%d", got.GoalContext, reader.calls)
	}
}
