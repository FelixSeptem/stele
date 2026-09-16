package app

import (
	"context"

	"github.com/FelixSeptem/stele/internal/assurance"
	"github.com/FelixSeptem/stele/internal/memory"
	"github.com/FelixSeptem/stele/internal/provider"
)

type providerConformanceWriter interface {
	RunConformance(context.Context, assurance.ConformanceRunInput) (assurance.ConformanceRun, []assurance.MissingEvidenceDiagnostic, error)
}

type providerAssuranceRecorder struct {
	writer providerConformanceWriter
}

func (r providerAssuranceRecorder) RecordProviderConformance(ctx context.Context, scope memory.Scope, outcome provider.ConformanceOutcome) error {
	if r.writer == nil {
		return nil
	}
	_, _, err := r.writer.RunConformance(ctx, assurance.ConformanceRunInput{
		Scope:            scope,
		ProfileID:        outcome.ProfileID,
		RunID:            outcome.RunID,
		StartedAt:        outcome.StartedAt,
		ProviderEvidence: outcome.AssuranceEvidence(),
		ProviderVerdict:  outcome.Verdict,
	})
	return err
}
