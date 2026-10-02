package insights

import (
	"fmt"
	"strings"

	"github.com/FelixSeptem/stele/internal/memory"
)

type ContradictionReviewState string

const (
	ContradictionReviewRequired  ContradictionReviewState = "review_required"
	ContradictionReviewConfirmed ContradictionReviewState = "confirmed"
	ContradictionReviewCoexists  ContradictionReviewState = "coexists"
	ContradictionReviewIncorrect ContradictionReviewState = "incorrect"
	ContradictionReviewStale     ContradictionReviewState = "stale"
)

func (state ContradictionReviewState) Valid() bool {
	switch state {
	case ContradictionReviewRequired, ContradictionReviewConfirmed, ContradictionReviewCoexists, ContradictionReviewIncorrect, ContradictionReviewStale:
		return true
	default:
		return false
	}
}

func ApplyContradictionReview(insight memory.DerivedInsight, state ContradictionReviewState, actor, reason string) (memory.DerivedInsightLifecycleTransition, error) {
	if insight.Type != memory.DerivedInsightTypeContradiction {
		return memory.DerivedInsightLifecycleTransition{}, fmt.Errorf("insight is not a contradiction")
	}
	if !state.Valid() {
		return memory.DerivedInsightLifecycleTransition{}, fmt.Errorf("contradiction review state %q is invalid", state)
	}
	if strings.TrimSpace(actor) == "" || strings.TrimSpace(reason) == "" {
		return memory.DerivedInsightLifecycleTransition{}, fmt.Errorf("contradiction review actor and reason are required")
	}
	transition := memory.DerivedInsightLifecycleTransition{
		Scope: insight.Scope, InsightID: insight.ID, FromState: insight.State, ToState: insight.State, Actor: actor, Reason: reason,
		OccurredAt: insight.UpdatedAt,
		Metadata:   map[string]any{"contradiction_review_state": string(state)},
	}
	if transition.OccurredAt.IsZero() {
		transition.OccurredAt = insight.CreatedAt
	}
	if state == ContradictionReviewCoexists || state == ContradictionReviewIncorrect || state == ContradictionReviewStale {
		transition.ToState = memory.DerivedInsightStateSuppressed
	}
	if err := transition.Validate(); err != nil {
		return memory.DerivedInsightLifecycleTransition{}, err
	}
	return transition, nil
}
