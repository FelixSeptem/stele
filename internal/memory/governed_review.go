package memory

import (
	"context"
	"fmt"
	"strings"
	"time"
)

type ReflectionReviewDecision string

const (
	ReflectionReviewAccept              ReflectionReviewDecision = "accept"
	ReflectionReviewSuppress            ReflectionReviewDecision = "suppress"
	ReflectionReviewReject              ReflectionReviewDecision = "reject"
	ReflectionReviewRequestMoreEvidence ReflectionReviewDecision = "request_more_evidence"
)

func (d ReflectionReviewDecision) Valid() bool {
	switch d {
	case ReflectionReviewAccept, ReflectionReviewSuppress, ReflectionReviewReject, ReflectionReviewRequestMoreEvidence:
		return true
	default:
		return false
	}
}

type ReflectionReviewInput struct {
	Scope         Scope
	CandidateID   string
	Decision      ReflectionReviewDecision
	Reviewer      string
	Reason        string
	PolicyVersion string
}

func (i ReflectionReviewInput) Validate() error {
	if err := i.Scope.Validate(); err != nil {
		return err
	}
	switch {
	case strings.TrimSpace(i.CandidateID) == "":
		return fmt.Errorf("candidate id is required")
	case !i.Decision.Valid():
		return fmt.Errorf("review decision %q is invalid", i.Decision)
	case strings.TrimSpace(i.Reviewer) == "":
		return fmt.Errorf("reviewer is required")
	case strings.TrimSpace(i.Reason) == "":
		return fmt.Errorf("review reason is required")
	case strings.TrimSpace(i.PolicyVersion) == "":
		return fmt.Errorf("policy version is required")
	default:
		return nil
	}
}

type ReflectionReviewRecord struct {
	ID            string
	Scope         Scope
	CandidateID   string
	Decision      ReflectionReviewDecision
	Reviewer      string
	Reason        string
	PolicyVersion string
	CreatedAt     time.Time
}

type ReflectionReviewProcessor interface {
	AppendReflectionReview(context.Context, ReflectionReviewRecord) (ReflectionReviewRecord, error)
}

type ReviewedCandidatePromotionInput struct {
	Scope       Scope
	CandidateID string
	Reviewer    string
	Reason      string
	OccurredAt  time.Time
}

type ReflectionReviewCanonicalIntegrator interface {
	PromoteReviewedCandidate(context.Context, ReviewedCandidatePromotionInput) error
}

type ReflectionReviewService struct {
	Processor  ReflectionReviewProcessor
	Integrator ReflectionReviewCanonicalIntegrator
	Now        func() time.Time
	NewID      func() string
}

func (s ReflectionReviewService) Decide(ctx context.Context, input ReflectionReviewInput) (ReflectionReviewRecord, error) {
	if err := input.Validate(); err != nil {
		return ReflectionReviewRecord{}, err
	}
	if s.Processor == nil {
		return ReflectionReviewRecord{}, fmt.Errorf("reflection review processor is not configured")
	}
	now := time.Now().UTC()
	if s.Now != nil {
		now = s.Now().UTC()
	}
	id := ""
	if s.NewID != nil {
		id = s.NewID()
	}
	record, err := s.Processor.AppendReflectionReview(ctx, ReflectionReviewRecord{ID: id, Scope: input.Scope, CandidateID: strings.TrimSpace(input.CandidateID), Decision: input.Decision, Reviewer: strings.TrimSpace(input.Reviewer), Reason: input.Reason, PolicyVersion: strings.TrimSpace(input.PolicyVersion), CreatedAt: now})
	if err != nil {
		return ReflectionReviewRecord{}, err
	}
	if input.Decision == ReflectionReviewAccept && s.Integrator != nil {
		if err := s.Integrator.PromoteReviewedCandidate(ctx, ReviewedCandidatePromotionInput{Scope: input.Scope, CandidateID: input.CandidateID, Reviewer: input.Reviewer, Reason: input.Reason, OccurredAt: now}); err != nil {
			return ReflectionReviewRecord{}, fmt.Errorf("promote reviewed candidate: %w", err)
		}
	}
	return record, nil
}
