package insights

import (
	"testing"
	"time"

	"github.com/FelixSeptem/stele/internal/memory"
	"github.com/FelixSeptem/stele/internal/reasoning"
)

func TestGoalVisibilityFailsClosedAndReplaysDeterministically(t *testing.T) {
	now := time.Date(2026, 10, 3, 0, 0, 0, 0, time.UTC)
	scope := memory.Scope{Tenant: "t", Project: "p", Namespace: "n"}
	policy := DefaultGoalVisibilityPolicy(scope)
	policy.Owner, policy.PrincipalGrant, policy.ScopeProof, policy.Enabled, policy.ExpiresAt = "operator", "grant", "proof", true, now.Add(time.Hour)
	in := GoalVisibilityInput{Scope: scope, GoalID: "goal-1", GoalState: reasoning.GoalStateProposed, ReviewState: reasoning.GoalReviewApproved, EvidenceCount: 1, EvidenceAt: now, SourceWatermark: "wm", ScopeProof: "proof", PrincipalGrant: "grant", PolicyVersion: policy.Version, Now: now}
	replay, err := GoalVisibilityReplayID(in)
	if err != nil {
		t.Fatal(err)
	}
	in.ReplayID = replay
	decision := EvaluateGoalVisibility(policy, in)
	if decision.Disposition != GoalVisibilityEligible {
		t.Fatalf("disposition=%s", decision.Disposition)
	}
	if got, _ := GoalVisibilityReplayID(in); got != replay {
		t.Fatalf("replay id changed: %s/%s", got, replay)
	}
	in.PrincipalGrant = "foreign"
	if got := EvaluateGoalVisibility(policy, in).Disposition; got != GoalVisibilityDenied {
		t.Fatalf("foreign grant disposition=%s", got)
	}
}

func TestRedactedGoalReviewOmitsContent(t *testing.T) {
	scope := memory.Scope{Tenant: "t", Project: "p", Namespace: "n"}
	in := GoalVisibilityInput{Scope: scope, GoalID: "secret-goal", GoalState: reasoning.GoalStateProposed, ReviewState: reasoning.GoalReviewRequired, EvidenceCount: 1}
	redacted := RedactGoalReview(in, GoalVisibilityDecision{PolicyVersion: "v", Disposition: GoalVisibilityReviewRequired, Reason: "review_required"})
	if !redacted.GoalIDPresent || redacted.Reason != "review_required" {
		t.Fatalf("unexpected redaction: %+v", redacted)
	}
}
