package jobs

import (
	"context"
	"testing"
	"time"

	"github.com/FelixSeptem/stele/internal/memory"
)

type contextCalibrationSummaryServiceStub struct {
	input memory.ContextCalibrationBuildInput
	calls int
}

func (s *contextCalibrationSummaryServiceStub) BuildContextCalibrationSummary(_ context.Context, input memory.ContextCalibrationBuildInput) (memory.ContextCalibrationSummary, error) {
	s.input = input
	s.calls++
	return memory.ContextCalibrationSummary{ID: "summary", Scope: input.Scope, PolicyVersion: "policy", SummaryVersion: "summary", SourceWatermark: time.Unix(1, 0), Freshness: memory.ContextCalibrationSummaryFresh, CreatedAt: time.Unix(1, 0), UpdatedAt: time.Unix(1, 0)}, nil
}

func TestContextCalibrationSummaryRebuildJobUsesExactScopeAndIsBounded(t *testing.T) {
	scope := memory.Scope{Tenant: "tenant", Project: "project", Namespace: "namespace"}
	service := &contextCalibrationSummaryServiceStub{}
	job := ContextCalibrationSummaryRebuildJob{Scope: scope, Service: service, Input: memory.ContextCalibrationBuildInput{Now: time.Unix(10, 0).UTC()}}
	count, err := job.Run(context.Background())
	if err != nil || count != 1 {
		t.Fatalf("Run() count=%d err=%v", count, err)
	}
	if service.calls != 1 || service.input.Scope != scope {
		t.Fatalf("service input = %+v calls=%d", service.input, service.calls)
	}
}
