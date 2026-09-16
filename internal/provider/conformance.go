package provider

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/FelixSeptem/stele/internal/memory"
)

type ConformanceCheck string

const (
	CheckCapability ConformanceCheck = "capability"
	CheckScope      ConformanceCheck = "scope"
	CheckReplay     ConformanceCheck = "replay"
	CheckLifecycle  ConformanceCheck = "lifecycle"
	CheckCitation   ConformanceCheck = "citation"
	CheckRestart    ConformanceCheck = "restart_fallback"
	CheckFreshness  ConformanceCheck = "freshness"
)

func (c ConformanceCheck) Valid() bool {
	switch c {
	case CheckCapability, CheckScope, CheckReplay, CheckLifecycle, CheckCitation, CheckRestart, CheckFreshness:
		return true
	default:
		return false
	}
}

type ConformanceFixture struct {
	ID        string           `json:"id"`
	Check     ConformanceCheck `json:"check"`
	Operation string           `json:"operation"`
	Scope     memory.Scope     `json:"scope"`
}

func (f ConformanceFixture) Validate(exactScope memory.Scope) error {
	if strings.TrimSpace(f.ID) == "" || len(f.ID) > 128 {
		return fmt.Errorf("fixture id is invalid")
	}
	if !f.Check.Valid() {
		return fmt.Errorf("unsupported conformance check %q", f.Check)
	}
	if strings.TrimSpace(f.Operation) == "" || len(f.Operation) > 64 {
		return fmt.Errorf("fixture operation is invalid")
	}
	if err := f.Scope.Validate(); err != nil {
		return err
	}
	if f.Scope.Normalized() != exactScope.Normalized() {
		return fmt.Errorf("fixture is out of scope")
	}
	return nil
}

type ConformanceProfile struct {
	ID            string               `json:"id"`
	SchemaVersion string               `json:"schema_version"`
	Fixtures      []ConformanceFixture `json:"fixtures"`
}

func (p ConformanceProfile) Validate(scope memory.Scope) error {
	if strings.TrimSpace(p.ID) == "" || len(p.ID) > 128 {
		return fmt.Errorf("profile id is invalid")
	}
	if p.SchemaVersion != SchemaVersionV1 {
		return fmt.Errorf("unsupported schema version")
	}
	if len(p.Fixtures) == 0 || len(p.Fixtures) > 32 {
		return fmt.Errorf("fixtures must contain 1 to 32 operations")
	}
	for _, fixture := range p.Fixtures {
		if err := fixture.Validate(scope); err != nil {
			return err
		}
	}
	return nil
}

type ConformanceOutcome struct {
	RunID         string         `json:"run_id"`
	PreviousRunID string         `json:"previous_run_id,omitempty"`
	ProfileID     string         `json:"profile_id"`
	SchemaVersion string         `json:"schema_version"`
	Verdict       string         `json:"verdict"`
	Counters      map[string]int `json:"counters"`
	Evidence      []Citation     `json:"evidence,omitempty"`
	NextActions   []string       `json:"next_actions,omitempty"`
	StartedAt     time.Time      `json:"started_at"`
	FinishedAt    time.Time      `json:"finished_at"`
}

// AssuranceEvidence returns the bounded, label-free shape accepted by the
// existing assurance ConformanceRunInput. It intentionally excludes fixture
// payloads, scope values, diagnostics, and provider internals.
func (o ConformanceOutcome) AssuranceEvidence() map[string]any {
	return map[string]any{
		"schema_version":  o.SchemaVersion,
		"verdict":         o.Verdict,
		"counters":        o.Counters,
		"evidence":        o.Evidence,
		"next_actions":    o.NextActions,
		"run_id":          o.RunID,
		"previous_run_id": o.PreviousRunID,
	}
}

type FixtureExecutor interface {
	ExecuteProviderFixture(context.Context, RuntimeBinding, ConformanceFixture) (Citation, error)
}

type ConformanceRecorder interface {
	RecordProviderConformance(context.Context, memory.Scope, ConformanceOutcome) error
}

type ConformanceRunner struct {
	Executor FixtureExecutor
	Recorder ConformanceRecorder
	Now      func() time.Time
	NewID    func() string
}

func (r ConformanceRunner) Run(ctx context.Context, binding RuntimeBinding, profile ConformanceProfile, previousRunID string) (ConformanceOutcome, error) {
	if err := profile.Validate(binding.Scope); err != nil {
		return ConformanceOutcome{}, err
	}
	if r.Executor == nil {
		return ConformanceOutcome{}, fmt.Errorf("provider fixture executor is not configured")
	}
	now := r.Now
	if now == nil {
		now = time.Now
	}
	newID := r.NewID
	if newID == nil {
		newID = func() string { return opaqueID("pcr_") }
	}
	out := ConformanceOutcome{RunID: newID(), PreviousRunID: strings.TrimSpace(previousRunID), ProfileID: profile.ID, SchemaVersion: profile.SchemaVersion, Verdict: "pass", Counters: map[string]int{"total": len(profile.Fixtures)}, StartedAt: now().UTC()}
	for _, fixture := range profile.Fixtures {
		citation, err := r.Executor.ExecuteProviderFixture(ctx, binding, fixture)
		if err != nil {
			out.Counters["failed"]++
			out.Verdict = "incomplete"
			continue
		}
		if err := citation.Validate(); err != nil {
			out.Counters["failed"]++
			out.Verdict = "incomplete"
			continue
		}
		out.Counters["passed"]++
		out.Evidence = append(out.Evidence, citation)
	}
	if out.Verdict != "pass" {
		out.NextActions = []string{"retry_after_dependencies_recover"}
	}
	out.FinishedAt = now().UTC()
	if r.Recorder != nil {
		if err := r.Recorder.RecordProviderConformance(ctx, binding.Scope, out); err != nil {
			return ConformanceOutcome{}, err
		}
	}
	return out, nil
}
