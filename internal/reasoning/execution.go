package reasoning

import (
	"context"
	"fmt"
	"strings"
)

type Provider interface {
	Derive(context.Context, InvocationRequest) (Candidate, error)
}

type Fixture struct {
	ID                  string            `json:"id"`
	Request             InvocationRequest `json:"request"`
	ExpectedCandidate   Candidate         `json:"expected_candidate"`
	ExpectedFingerprint string            `json:"expected_fingerprint,omitempty"`
}

type Outcome string

const (
	OutcomeAccepted Outcome = "accepted"
	OutcomeFallback Outcome = "fallback"
	OutcomeShadow   Outcome = "shadow"
	OutcomeStale    Outcome = "stale"
)

type Result struct {
	Outcome       Outcome        `json:"outcome"`
	Candidate     Candidate      `json:"candidate"`
	Error         *ProviderError `json:"error,omitempty"`
	Fingerprint   string         `json:"fingerprint"`
	Equivalent    bool           `json:"equivalent"`
	Authoritative bool           `json:"authoritative"`
}

type OfflineRunner struct {
	Provider Provider
	Limits   Limits
}

func (r OfflineRunner) Replay(_ context.Context, fixture Fixture) (Result, error) {
	if strings.TrimSpace(fixture.ID) == "" {
		return Result{}, fmt.Errorf("fixture id is required")
	}
	limits := r.Limits
	if limits.Validate() != nil {
		limits = DefaultLimits()
	}
	if err := fixture.Request.Validate(limits, fixture.Request.Now); err != nil {
		return Result{}, err
	}
	if err := fixture.ExpectedCandidate.Validate(limits); err != nil {
		return Result{}, fmt.Errorf("fixture candidate: %w", err)
	}
	fingerprint, err := Fingerprint(struct {
		ID        string            `json:"id"`
		Request   InvocationRequest `json:"request"`
		Candidate Candidate         `json:"candidate"`
	}{fixture.ID, fixture.Request, fixture.ExpectedCandidate})
	if err != nil {
		return Result{}, err
	}
	if fixture.ExpectedFingerprint != "" && fixture.ExpectedFingerprint != fingerprint {
		return Result{}, fmt.Errorf("fixture fingerprint mismatch")
	}
	return Result{Outcome: OutcomeAccepted, Candidate: fixture.ExpectedCandidate, Fingerprint: fingerprint, Authoritative: false}, nil
}

type ShadowExecutor struct {
	Provider Provider
	Limits   Limits
}

func (s ShadowExecutor) Execute(ctx context.Context, req InvocationRequest, baseline Candidate) (Result, error) {
	if s.Provider == nil {
		return Result{}, fmt.Errorf("reasoning provider is not configured")
	}
	limits := s.Limits
	if limits.Validate() != nil {
		limits = DefaultLimits()
	}
	if err := req.Validate(limits, req.Now); err != nil {
		return Result{}, err
	}
	candidate, err := s.Provider.Derive(ctx, req)
	if err != nil {
		return fallbackResult(baseline, err), nil
	}
	if err := candidate.Validate(limits); err != nil {
		return fallbackResult(baseline, &ProviderError{Category: ErrorCategoryMalformedOutput, Code: "invalid_output", Message: "provider output failed validation", Retryable: false}), nil
	}
	fp, err := Fingerprint(candidate)
	if err != nil {
		return Result{}, err
	}
	return Result{Outcome: OutcomeShadow, Candidate: baseline, Fingerprint: fp, Equivalent: equivalentCandidate(candidate, baseline), Authoritative: false}, nil
}

type Executor struct {
	Provider Provider
	Limits   Limits
}

func (e Executor) Execute(ctx context.Context, req InvocationRequest, baseline Candidate) (Result, error) {
	if e.Provider == nil {
		return Result{Outcome: OutcomeFallback, Candidate: baseline, Error: &ProviderError{Category: ErrorCategoryUnavailable, Code: "disabled", Message: "reasoning provider is unavailable", Retryable: false}}, nil
	}
	limits := e.Limits
	if limits.Validate() != nil {
		limits = DefaultLimits()
	}
	if err := req.Validate(limits, req.Now); err != nil {
		return Result{}, err
	}
	candidate, err := e.Provider.Derive(ctx, req)
	if err != nil {
		return fallbackResult(baseline, err), nil
	}
	if err := candidate.Validate(limits); err != nil {
		return fallbackResult(baseline, &ProviderError{Category: ErrorCategoryMalformedOutput, Code: "invalid_output", Message: "provider output failed validation", Retryable: false}), nil
	}
	fp, err := Fingerprint(candidate)
	if err != nil {
		return Result{}, err
	}
	return Result{Outcome: OutcomeAccepted, Candidate: candidate, Fingerprint: fp, Authoritative: false}, nil
}

func fallbackResult(baseline Candidate, err error) Result {
	pe := asProviderError(err)
	return Result{Outcome: OutcomeFallback, Candidate: baseline, Error: &pe, Authoritative: false}
}

func asProviderError(err error) ProviderError {
	if pe, ok := err.(*ProviderError); ok && pe != nil {
		return *pe
	}
	return ProviderError{Category: ErrorCategoryUnavailable, Code: "provider_error", Message: "reasoning provider unavailable", Retryable: true}
}

func equivalentCandidate(a, b Candidate) bool {
	return a.Scope.Normalized() == b.Scope.Normalized() && a.InsightType == b.InsightType && a.Content == b.Content && len(a.Evidence) == len(b.Evidence)
}

func (e ProviderError) Error() string { return string(e.Category) + ": " + e.Message }
