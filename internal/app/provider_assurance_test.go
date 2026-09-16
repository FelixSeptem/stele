package app

import (
	"context"
	"testing"
	"time"

	"github.com/FelixSeptem/stele/internal/assurance"
	"github.com/FelixSeptem/stele/internal/memory"
	"github.com/FelixSeptem/stele/internal/provider"
)

type providerConformanceWriterStub struct{ input assurance.ConformanceRunInput }

func (s *providerConformanceWriterStub) RunConformance(_ context.Context, input assurance.ConformanceRunInput) (assurance.ConformanceRun, []assurance.MissingEvidenceDiagnostic, error) {
	s.input = input
	return assurance.ConformanceRun{}, nil, nil
}

func TestProviderAssuranceRecorderUsesExistingConformanceRecord(t *testing.T) {
	writer := &providerConformanceWriterStub{}
	scope := memory.Scope{Tenant: "tenant-a", Project: "project-a", Namespace: "namespace-a"}
	outcome := provider.ConformanceOutcome{RunID: "run-1", PreviousRunID: "run-0", ProfileID: "profile-1", SchemaVersion: provider.SchemaVersionV1, Verdict: "incomplete", Counters: map[string]int{"failed": 1}, StartedAt: time.Unix(1, 0)}
	if err := (providerAssuranceRecorder{writer: writer}).RecordProviderConformance(context.Background(), scope, outcome); err != nil {
		t.Fatal(err)
	}
	if writer.input.Scope != scope || writer.input.ProfileID != outcome.ProfileID || writer.input.RunID != outcome.RunID || writer.input.ProviderVerdict != outcome.Verdict {
		t.Fatalf("assurance input = %+v", writer.input)
	}
	if writer.input.ProviderEvidence["previous_run_id"] != "run-0" {
		t.Fatalf("provider evidence = %+v", writer.input.ProviderEvidence)
	}
}
