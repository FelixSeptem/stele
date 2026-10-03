package postgres

import (
	"context"
	"fmt"
	"strings"

	"github.com/FelixSeptem/stele/internal/insights"
	"github.com/FelixSeptem/stele/internal/memory"
	"github.com/FelixSeptem/stele/internal/reasoning"
	"github.com/FelixSeptem/stele/internal/retrieval"
)

// ReadGoalContext is the PostgreSQL-backed experimental projection. It is
// deliberately a separate query from ordinary derived insight retrieval and
// returns nothing unless an exact-scope, enabled visibility policy exists.
func (r *Repository) ReadGoalContext(ctx context.Context, scope memory.Scope, limit int) ([]retrieval.GoalContextItem, string, error) {
	if err := scope.Validate(); err != nil {
		return nil, "invalid_scope", err
	}
	if limit <= 0 {
		return nil, "invalid_limit", fmt.Errorf("goal context limit must be positive")
	}
	if limit > 32 {
		limit = 32
	}
	var policyVersion string
	const policyQuery = `
SELECT version
FROM goal_visibility_policies
WHERE tenant=$1 AND project=$2 AND namespace=$3
  AND enabled = true AND rolled_back = false AND expires_at > now()
ORDER BY created_at DESC
LIMIT 1`
	if err := r.db.QueryRow(ctx, policyQuery, scope.Tenant, scope.Project, scope.Namespace).Scan(&policyVersion); err != nil {
		return nil, "policy_disabled", nil
	}
	const candidateQuery = `
SELECT title, summary, goal_metadata->>'state', goal_metadata->>'review_state'
FROM governed_reasoning_insight_candidates
WHERE tenant=$1 AND project=$2 AND namespace=$3
  AND insight_type='goal'
  AND disposition IN ('candidate','would_activate')
  AND goal_metadata->>'review_state' = 'review_approved'
  AND goal_metadata->>'state' IN ('proposed','active')
ORDER BY created_at DESC
LIMIT $4`
	rows, err := r.db.Query(ctx, candidateQuery, scope.Tenant, scope.Project, scope.Namespace, limit)
	if err != nil {
		return nil, "query_failed", fmt.Errorf("read goal context: %w", err)
	}
	defer rows.Close()
	items := make([]retrieval.GoalContextItem, 0, limit)
	for rows.Next() {
		var item retrieval.GoalContextItem
		if err := rows.Scan(&item.Title, &item.Summary, &item.State, &item.ReviewState); err != nil {
			return nil, "query_failed", err
		}
		item.PolicyVersion = strings.TrimSpace(policyVersion)
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, "query_failed", err
	}
	return items, "eligible", nil
}

// ListGoalReviews returns a redacted exact-scope projection. Goal titles,
// summaries, prompts, provider payloads, and identifiers are intentionally not
// selected, even for an administrator.
func (r *Repository) ListGoalReviews(ctx context.Context, scope memory.Scope, limit int) ([]insights.RedactedGoalReview, error) {
	if err := scope.Validate(); err != nil {
		return nil, err
	}
	if limit <= 0 {
		return nil, fmt.Errorf("goal review limit must be positive")
	}
	if limit > 100 {
		limit = 100
	}
	const query = `
SELECT goal_metadata->>'state', goal_metadata->>'review_state', policy_version,
       disposition, jsonb_array_length(evidence)
FROM governed_reasoning_insight_candidates c
WHERE tenant=$1 AND project=$2 AND namespace=$3 AND insight_type='goal'
ORDER BY created_at DESC LIMIT $4`
	rows, err := r.db.Query(ctx, query, scope.Tenant, scope.Project, scope.Namespace, limit)
	if err != nil {
		return nil, fmt.Errorf("list redacted goal reviews: %w", err)
	}
	defer rows.Close()
	items := make([]insights.RedactedGoalReview, 0, limit)
	for rows.Next() {
		var state, reviewState, policyVersion, disposition string
		var evidenceCount int
		if err := rows.Scan(&state, &reviewState, &policyVersion, &disposition, &evidenceCount); err != nil {
			return nil, err
		}
		mapped := insights.GoalVisibilityDisposition(disposition)
		if disposition == "would_activate" || disposition == "candidate" {
			mapped = insights.GoalVisibilityReviewRequired
		}
		items = append(items, insights.RedactedGoalReview{Scope: scope.Normalized(), GoalIDPresent: true, GoalState: reasoning.GoalState(state), ReviewState: reasoning.GoalReviewState(reviewState), PolicyVersion: strings.TrimSpace(policyVersion), Disposition: mapped, EvidenceCount: evidenceCount, Reason: "bounded_review_projection"})
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return items, nil
}
