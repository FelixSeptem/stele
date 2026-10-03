package reasoning

import (
	"fmt"
	"strings"
	"time"
)

type GoalState string

const (
	GoalStateProposed  GoalState = "proposed"
	GoalStateActive    GoalState = "active"
	GoalStateCompleted GoalState = "completed"
	GoalStateAbandoned GoalState = "abandoned"
	GoalStateStale     GoalState = "stale"
)

func (s GoalState) Valid() bool {
	switch s {
	case GoalStateProposed, GoalStateActive, GoalStateCompleted, GoalStateAbandoned, GoalStateStale:
		return true
	default:
		return false
	}
}

type GoalReviewState string

const (
	GoalReviewRequired GoalReviewState = "review_required"
	GoalReviewApproved GoalReviewState = "review_approved"
	GoalReviewRejected GoalReviewState = "review_rejected"
)

func (s GoalReviewState) Valid() bool {
	switch s {
	case GoalReviewRequired, GoalReviewApproved, GoalReviewRejected:
		return true
	default:
		return false
	}
}

type GoalMetadata struct {
	State            GoalState       `json:"state"`
	ReviewState      GoalReviewState `json:"review_state"`
	ValidFrom        *time.Time      `json:"valid_from,omitempty"`
	ValidTo          *time.Time      `json:"valid_to,omitempty"`
	ExecuteRequested bool            `json:"execute_requested,omitempty"`
}

func (m GoalMetadata) Validate() error {
	if !m.State.Valid() {
		return fmt.Errorf("goal state %q is invalid", m.State)
	}
	if !m.ReviewState.Valid() {
		return fmt.Errorf("goal review state %q is invalid", m.ReviewState)
	}
	if (m.ValidFrom == nil) != (m.ValidTo == nil) {
		return fmt.Errorf("goal validity interval must include both bounds")
	}
	if m.ValidFrom != nil && m.ValidFrom.After(*m.ValidTo) {
		return fmt.Errorf("goal validity interval start must be before or equal to end")
	}
	if m.ExecuteRequested {
		return fmt.Errorf("goal candidate cannot request execution")
	}
	return nil
}

func (m GoalMetadata) Bounded() bool {
	return strings.TrimSpace(string(m.State)) != "" && strings.TrimSpace(string(m.ReviewState)) != ""
}
