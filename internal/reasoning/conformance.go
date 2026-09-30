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
