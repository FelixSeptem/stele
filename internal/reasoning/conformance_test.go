package reasoning

import (
	"context"
	"testing"
)

func TestConformanceOmitsSensitiveProviderMaterial(t *testing.T) {
	run, err := RunConformance(context.Background(), ConformanceProfile{ID: "profile-1", Scope: validRequest().Scope, Fixtures: []Fixture{{ID: "f-1", Request: validRequest(), ExpectedCandidate: validCandidate()}}}, DefaultLimits())
	if err != nil {
		t.Fatalf("conformance: %v", err)
	}
	if run.Status != ConformancePassed || run.Scope != validRequest().Scope {
		t.Fatalf("unexpected conformance result: %+v", run)
	}
	if run.Prompts != 0 || run.RawPayloads != 0 || run.Credentials != 0 || run.ForeignIdentifiers != 0 {
		t.Fatalf("sensitive fields leaked: %+v", run)
	}
}

func TestConformanceRecordsIsolationFailureWithoutForeignData(t *testing.T) {
	fixture := Fixture{ID: "f-1", Request: validRequest(), ExpectedCandidate: validCandidate()}
	fixture.ExpectedCandidate.Scope.Tenant = "tenant-b"
	run, err := RunConformance(context.Background(), ConformanceProfile{ID: "profile-1", Scope: validRequest().Scope, Fixtures: []Fixture{fixture}}, DefaultLimits())
	if err != nil {
		t.Fatalf("conformance: %v", err)
	}
	if run.Status != ConformanceFailed || run.Scope != validRequest().Scope || run.ForeignIdentifiers != 0 {
		t.Fatalf("unexpected isolation result: %+v", run)
	}
}
