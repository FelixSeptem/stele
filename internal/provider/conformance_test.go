package provider

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/FelixSeptem/stele/internal/memory"
)

type fixtureExecutorFunc func(context.Context, RuntimeBinding, ConformanceFixture) (Citation, error)

func (f fixtureExecutorFunc) ExecuteProviderFixture(ctx context.Context, b RuntimeBinding, in ConformanceFixture) (Citation, error) {
	return f(ctx, b, in)
}

func TestConformanceProfileRejectsUnknownAndForeignFixtures(t *testing.T) {
	scope := memory.Scope{Tenant: "t", Project: "p", Namespace: "n"}
	profile := ConformanceProfile{ID: "profile", SchemaVersion: SchemaVersionV1, Fixtures: []ConformanceFixture{{ID: "x", Check: "unknown", Operation: "read", Scope: scope}}}
	if err := profile.Validate(scope); err == nil {
		t.Fatal("expected unknown check rejection")
	}
	profile.Fixtures[0].Check = CheckScope
	profile.Fixtures[0].Scope.Namespace = "foreign"
	if err := profile.Validate(scope); err == nil {
		t.Fatal("expected foreign scope rejection")
	}
}

func TestConformanceRunnerCannotClaimReadinessWhenCheckFails(t *testing.T) {
	scope := memory.Scope{Tenant: "t", Project: "p", Namespace: "n"}
	runner := ConformanceRunner{Now: func() time.Time { return time.Unix(1, 0) }, NewID: func() string { return "run-1" }, Executor: fixtureExecutorFunc(func(context.Context, RuntimeBinding, ConformanceFixture) (Citation, error) {
		return Citation{}, errors.New("stale projection")
	})}
	out, err := runner.Run(context.Background(), RuntimeBinding{Scope: scope}, ConformanceProfile{ID: "profile", SchemaVersion: SchemaVersionV1, Fixtures: []ConformanceFixture{{ID: "fresh", Check: CheckFreshness, Operation: "context.assemble", Scope: scope}}}, "run-0")
	if err != nil {
		t.Fatal(err)
	}
	if out.Verdict != "incomplete" || out.Counters["failed"] != 1 || out.PreviousRunID != "run-0" {
		t.Fatalf("outcome = %+v", out)
	}
}

func TestConformanceOutcomeAssuranceEvidenceIsBounded(t *testing.T) {
	out := ConformanceOutcome{RunID: "run-1", PreviousRunID: "run-0", SchemaVersion: SchemaVersionV1, Verdict: "incomplete", Counters: map[string]int{"failed": 1}, NextActions: []string{"retry"}}
	evidence := out.AssuranceEvidence()
	if evidence["verdict"] != "incomplete" || evidence["schema_version"] != SchemaVersionV1 {
		t.Fatalf("evidence = %+v", evidence)
	}
	if _, ok := evidence["scope"]; ok {
		t.Fatal("assurance evidence leaked scope")
	}
}
