package insights

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/FelixSeptem/stele/internal/memory"
	"github.com/FelixSeptem/stele/internal/reasoning"
)

const GoalVisibilityPolicyVersion = "goal-visibility-v1"

type GoalVisibilityDisposition string

const (
	GoalVisibilityEligible       GoalVisibilityDisposition = "eligible"
	GoalVisibilityDisabled       GoalVisibilityDisposition = "policy_disabled"
	GoalVisibilityDenied         GoalVisibilityDisposition = "denied"
	GoalVisibilityStale          GoalVisibilityDisposition = "stale"
	GoalVisibilityReviewRequired GoalVisibilityDisposition = "review_required"
	GoalVisibilityReplay         GoalVisibilityDisposition = "replayed"
)

func (d GoalVisibilityDisposition) Valid() bool {
	switch d {
	case GoalVisibilityEligible, GoalVisibilityDisabled, GoalVisibilityDenied, GoalVisibilityStale, GoalVisibilityReviewRequired, GoalVisibilityReplay:
		return true
	default:
		return false
	}
}

// GoalVisibilityPolicy is independently versioned from reserved insight
// activation. It is disabled by default and is always bound to one scope.
type GoalVisibilityPolicy struct {
	Scope           memory.Scope
	Version         string
	Owner           string
	Enabled         bool
	PrincipalGrant  string
	ScopeProof      string
	RequireReview   bool
	MaxEvidenceAge  time.Duration
	SourceWatermark string
	ExpiresAt       time.Time
	RolledBack      bool
}

func DefaultGoalVisibilityPolicy(scope memory.Scope) GoalVisibilityPolicy {
	return GoalVisibilityPolicy{Scope: scope.Normalized(), Version: GoalVisibilityPolicyVersion, MaxEvidenceAge: 15 * time.Minute, RequireReview: true}
}

func (p GoalVisibilityPolicy) ValidateAt(now time.Time) error {
	if err := p.Scope.Validate(); err != nil {
		return err
	}
	if strings.TrimSpace(p.Version) == "" || strings.TrimSpace(p.Owner) == "" {
		return fmt.Errorf("goal visibility policy identity is required")
	}
	if strings.TrimSpace(p.PrincipalGrant) == "" || strings.TrimSpace(p.ScopeProof) == "" {
		return fmt.Errorf("goal visibility grant and scope proof are required")
	}
	if p.MaxEvidenceAge <= 0 {
		return fmt.Errorf("goal visibility evidence age must be positive")
	}
	if p.ExpiresAt.IsZero() {
		return fmt.Errorf("goal visibility policy expiry is required")
	}
	if p.RolledBack {
		return fmt.Errorf("goal visibility policy is rolled back")
	}
	if !now.IsZero() && !p.ExpiresAt.After(now) {
		return fmt.Errorf("goal visibility policy is expired")
	}
	return nil
}

type GoalVisibilityInput struct {
	Scope           memory.Scope
	GoalID          string
	GoalState       reasoning.GoalState
	ReviewState     reasoning.GoalReviewState
	EvidenceCount   int
	EvidenceAt      time.Time
	SourceWatermark string
	ScopeProof      string
	PrincipalGrant  string
	PolicyVersion   string
	ReplayID        string
	Now             time.Time
}

func (i GoalVisibilityInput) Validate() error {
	if err := i.Scope.Validate(); err != nil {
		return err
	}
	if strings.TrimSpace(i.GoalID) == "" || !i.GoalState.Valid() || !i.ReviewState.Valid() {
		return fmt.Errorf("goal visibility candidate is invalid")
	}
	if i.EvidenceCount <= 0 || i.EvidenceAt.IsZero() {
		return fmt.Errorf("goal visibility evidence is required")
	}
	if strings.TrimSpace(i.SourceWatermark) == "" || strings.TrimSpace(i.ScopeProof) == "" || strings.TrimSpace(i.PrincipalGrant) == "" || strings.TrimSpace(i.PolicyVersion) == "" {
		return fmt.Errorf("goal visibility compatibility metadata is required")
	}
	if strings.TrimSpace(i.ReplayID) == "" {
		return fmt.Errorf("goal visibility replay id is required")
	}
	return nil
}

type GoalVisibilityDecision struct {
	ID            string                    `json:"id"`
	Scope         memory.Scope              `json:"scope"`
	GoalID        string                    `json:"goal_id"`
	PolicyVersion string                    `json:"policy_version"`
	ReplayID      string                    `json:"replay_id"`
	Disposition   GoalVisibilityDisposition `json:"disposition"`
	ReviewState   reasoning.GoalReviewState `json:"review_state"`
	Freshness     string                    `json:"freshness"`
	EvidenceCount int                       `json:"evidence_count"`
	Reason        string                    `json:"reason"`
	Actor         string                    `json:"actor,omitempty"`
	CreatedAt     time.Time                 `json:"created_at"`
}

func EvaluateGoalVisibility(policy GoalVisibilityPolicy, input GoalVisibilityInput) GoalVisibilityDecision {
	now := input.Now.UTC()
	if now.IsZero() {
		now = time.Now().UTC()
	}
	d := GoalVisibilityDecision{Scope: input.Scope.Normalized(), GoalID: input.GoalID, PolicyVersion: policy.Version, ReplayID: input.ReplayID, ReviewState: input.ReviewState, EvidenceCount: input.EvidenceCount, CreatedAt: now}
	if err := input.Validate(); err != nil {
		d.Disposition, d.Reason = GoalVisibilityDenied, "invalid_candidate"
		return d
	}
	if err := policy.ValidateAt(now); err != nil {
		d.Disposition, d.Reason = GoalVisibilityDisabled, "policy_unavailable"
		return d
	}
	if policy.Scope.Normalized() != input.Scope.Normalized() || policy.ScopeProof != input.ScopeProof || policy.PrincipalGrant != input.PrincipalGrant || policy.Version != input.PolicyVersion {
		d.Disposition, d.Reason = GoalVisibilityDenied, "scope_or_grant_mismatch"
		return d
	}
	if now.Sub(input.EvidenceAt.UTC()) > policy.MaxEvidenceAge || input.EvidenceAt.After(now.Add(time.Minute)) {
		d.Disposition, d.Freshness, d.Reason = GoalVisibilityStale, "stale", "evidence_stale"
		return d
	}
	d.Freshness = "fresh"
	if policy.RequireReview && input.ReviewState != reasoning.GoalReviewApproved {
		d.Disposition, d.Reason = GoalVisibilityReviewRequired, "review_required"
		return d
	}
	if input.GoalState != reasoning.GoalStateProposed && input.GoalState != reasoning.GoalStateActive {
		d.Disposition, d.Reason = GoalVisibilityDenied, "goal_state_ineligible"
		return d
	}
	d.Disposition, d.Reason = GoalVisibilityEligible, "eligible"
	return d
}

func GoalVisibilityReplayID(input GoalVisibilityInput) (string, error) {
	seed := struct {
		Scope           memory.Scope              `json:"scope"`
		GoalID          string                    `json:"goal_id"`
		GoalState       reasoning.GoalState       `json:"goal_state"`
		ReviewState     reasoning.GoalReviewState `json:"review_state"`
		EvidenceCount   int                       `json:"evidence_count"`
		EvidenceAt      time.Time                 `json:"evidence_at"`
		SourceWatermark string                    `json:"source_watermark"`
		ScopeProof      string                    `json:"scope_proof"`
		PrincipalGrant  string                    `json:"principal_grant"`
		PolicyVersion   string                    `json:"policy_version"`
	}{input.Scope.Normalized(), input.GoalID, input.GoalState, input.ReviewState, input.EvidenceCount, input.EvidenceAt.UTC(), input.SourceWatermark, input.ScopeProof, input.PrincipalGrant, input.PolicyVersion}
	b, err := json.Marshal(seed)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(b)
	return "goal-visibility:" + hex.EncodeToString(sum[:16]), nil
}

// RedactedGoalReview is safe for admin diagnostics and never contains goal text.
type RedactedGoalReview struct {
	Scope         memory.Scope              `json:"scope"`
	GoalIDPresent bool                      `json:"goal_id_present"`
	GoalState     reasoning.GoalState       `json:"goal_state"`
	ReviewState   reasoning.GoalReviewState `json:"review_state"`
	PolicyVersion string                    `json:"policy_version"`
	Disposition   GoalVisibilityDisposition `json:"disposition"`
	Freshness     string                    `json:"freshness"`
	EvidenceCount int                       `json:"evidence_count"`
	Reason        string                    `json:"reason"`
}

type GoalReviewReader interface {
	ListGoalReviews(ctx context.Context, scope memory.Scope, limit int) ([]RedactedGoalReview, error)
}

func RedactGoalReview(input GoalVisibilityInput, decision GoalVisibilityDecision) RedactedGoalReview {
	return RedactedGoalReview{Scope: input.Scope.Normalized(), GoalIDPresent: strings.TrimSpace(input.GoalID) != "", GoalState: input.GoalState, ReviewState: input.ReviewState, PolicyVersion: decision.PolicyVersion, Disposition: decision.Disposition, Freshness: decision.Freshness, EvidenceCount: input.EvidenceCount, Reason: decision.Reason}
}

func NormalizeGoalEvidenceIDs(ids []string) []string {
	out := append([]string(nil), ids...)
	sort.Strings(out)
	return out
}
