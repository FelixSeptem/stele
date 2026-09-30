package reasoning

import (
	"context"
	"fmt"
	"github.com/FelixSeptem/stele/internal/memory"
)

type ConformanceStatus string

const (
	ConformancePassed     ConformanceStatus = "passed"
	ConformanceFailed     ConformanceStatus = "failed"
	ConformanceIncomplete ConformanceStatus = "incomplete"
)

type ConformanceProfile struct {
	ID       string
	Scope    memory.Scope
	Fixtures []Fixture
}

// ConformanceRun is intentionally a bounded diagnostic view. It carries only
// counters and statuses; prompts and raw provider payloads are never retained.
type ConformanceRun struct {
	ProfileID          string            `json:"profile_id"`
	Scope              memory.Scope      `json:"scope"`
	Status             ConformanceStatus `json:"status"`
	Fixtures           int               `json:"fixtures"`
	Passed             int               `json:"passed"`
	Failed             int               `json:"failed"`
	ForeignIdentifiers int               `json:"foreign_identifiers"`
	Prompts            int               `json:"prompts"`
	RawPayloads        int               `json:"raw_payloads"`
	Credentials        int               `json:"credentials"`
}

// RunAdapterConformance executes a provider against bounded fixtures while
// retaining only counters and safe statuses. It deliberately does not expose
// request content, response bodies, or provider diagnostics.
func RunAdapterConformance(ctx context.Context, provider Provider, profile ConformanceProfile, limits Limits) (ConformanceRun, error) {
	if provider == nil {
		return ConformanceRun{}, fmt.Errorf("reasoning provider is required")
	}
	run, err := RunConformance(ctx, profile, limits)
	if err != nil {
		return ConformanceRun{}, err
	}
	run.Passed = 0
	run.Failed = 0
	run.Status = ConformancePassed
	for _, fixture := range profile.Fixtures {
		candidate, providerErr := provider.Derive(ctx, fixture.Request)
		if providerErr != nil {
			run.Failed++
			run.Status = ConformanceFailed
			continue
		}
		if err := candidate.Validate(limits); err != nil || candidate.Scope.Normalized() != fixture.Request.Scope.Normalized() || !evidenceMatches(candidate.Evidence, fixture.ExpectedCandidate.Evidence) {
			run.Failed++
			run.Status = ConformanceFailed
			continue
		}
		run.Passed++
	}
	return run, nil
}

func evidenceMatches(got, want []Evidence) bool {
	if len(got) != len(want) {
		return false
	}
	for i := range got {
		if got[i] != want[i] {
			return false
		}
	}
	return true
}

func RunConformance(_ context.Context, profile ConformanceProfile, limits Limits) (ConformanceRun, error) {
	if profile.ID == "" || len(profile.Fixtures) == 0 {
		return ConformanceRun{}, fmt.Errorf("conformance profile is incomplete")
	}
	if limits.Validate() != nil {
		limits = DefaultLimits()
	}
	run := ConformanceRun{ProfileID: profile.ID, Scope: profile.Scope, Fixtures: len(profile.Fixtures), Status: ConformancePassed}
	for _, fixture := range profile.Fixtures {
		if err := fixture.Request.Scope.Validate(); err != nil {
			run.Failed++
			run.Status = ConformanceFailed
			continue
		}
		if err := fixture.ExpectedCandidate.Validate(limits); err != nil {
			run.Failed++
			run.Status = ConformanceFailed
			continue
		}
		if fixture.Request.Scope.Normalized() != fixture.ExpectedCandidate.Scope.Normalized() {
			run.Failed++
			run.Status = ConformanceFailed
			continue
		}
		run.Passed++
	}
	return run, nil
}
