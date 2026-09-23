package jobs

import (
	"context"
	"errors"

	"github.com/FelixSeptem/stele/internal/memory"
)

var ErrContextCalibrationSummaryJobNotConfigured = errors.New("context calibration summary job is not configured")

type ContextCalibrationSummaryRebuildService interface {
	BuildContextCalibrationSummary(context.Context, memory.ContextCalibrationBuildInput) (memory.ContextCalibrationSummary, error)
}

type ContextCalibrationSummaryRebuildJob struct {
	Scope   memory.Scope
	Input   memory.ContextCalibrationBuildInput
	Service ContextCalibrationSummaryRebuildService
}

func (j ContextCalibrationSummaryRebuildJob) Name() string {
	return "context_calibration_summary_rebuild"
}

func (j ContextCalibrationSummaryRebuildJob) Run(ctx context.Context) (int, error) {
	if err := j.Scope.Validate(); err != nil {
		return 0, err
	}
	if j.Service == nil {
		return 0, ErrContextCalibrationSummaryJobNotConfigured
	}
	j.Input.Scope = j.Scope.Normalized()
	if _, err := j.Service.BuildContextCalibrationSummary(ctx, j.Input); err != nil {
		return 0, err
	}
	return 1, nil
}
