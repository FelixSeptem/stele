package jobs

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/FelixSeptem/stele/internal/governance"
	"github.com/FelixSeptem/stele/internal/memory"
	"github.com/FelixSeptem/stele/internal/telemetry"
)

type stubRawEventClaimer struct {
	gotInput governance.ClaimPendingRawEventsInput
	claims   []governance.ClaimedRawEvent
	err      error
}

func (s *stubRawEventClaimer) ClaimPendingRawEvents(ctx context.Context, input governance.ClaimPendingRawEventsInput) ([]governance.ClaimedRawEvent, error) {
	s.gotInput = input
	if s.err != nil {
		return nil, s.err
	}

	return s.claims, nil
}

type stubRawEventProcessor struct {
	gotClaims []governance.ClaimedRawEvent
	err       error
}

func (s *stubRawEventProcessor) ProcessClaimedRawEvent(ctx context.Context, claim governance.ClaimedRawEvent) error {
	s.gotClaims = append(s.gotClaims, claim)
	return s.err
}

type stubJobsObserver struct {
	operations []telemetry.OperationEvent
}

func (s *stubJobsObserver) RecordOperation(ctx context.Context, event telemetry.OperationEvent) {
	s.operations = append(s.operations, event)
}

func (s *stubJobsObserver) RecordBacklog(ctx context.Context, event telemetry.BacklogEvent) {}

func TestGovernanceWorkerRunOnceClaimsAndProcessesEvents(t *testing.T) {
	now := time.Date(2026, 5, 31, 15, 30, 0, 0, time.UTC)
	claims := []governance.ClaimedRawEvent{
		newClaimedRawEvent(t, "evt_1", now),
		newClaimedRawEvent(t, "evt_2", now.Add(time.Second)),
	}

	claimer := &stubRawEventClaimer{claims: claims}
	processor := &stubRawEventProcessor{}
	worker := GovernanceWorker{
		Claimer:       claimer,
		Processor:     processor,
		WorkerID:      "worker-a",
		BatchSize:     8,
		LeaseDuration: 2 * time.Minute,
		Now: func() time.Time {
			return now
		},
	}

	processed, err := worker.RunOnce(context.Background())
	if err != nil {
		t.Fatalf("RunOnce() error = %v", err)
	}

	if processed != 2 {
		t.Fatalf("processed = %d, want %d", processed, 2)
	}

	if claimer.gotInput.WorkerID != "worker-a" {
		t.Fatalf("WorkerID = %q, want %q", claimer.gotInput.WorkerID, "worker-a")
	}

	if claimer.gotInput.BatchSize != 8 {
		t.Fatalf("BatchSize = %d, want %d", claimer.gotInput.BatchSize, 8)
	}

	if claimer.gotInput.LeaseDuration != 2*time.Minute {
		t.Fatalf("LeaseDuration = %s, want %s", claimer.gotInput.LeaseDuration, 2*time.Minute)
	}

	if !claimer.gotInput.Now.Equal(now) {
		t.Fatalf("Now = %s, want %s", claimer.gotInput.Now, now)
	}

	if len(processor.gotClaims) != 2 {
		t.Fatalf("processed claims = %d, want %d", len(processor.gotClaims), 2)
	}
}

func TestGovernanceWorkerRunOnceRejectsInvalidConfiguration(t *testing.T) {
	worker := GovernanceWorker{}

	if _, err := worker.RunOnce(context.Background()); err == nil {
		t.Fatal("RunOnce() error = nil, want configuration error")
	}
}

func TestGovernanceWorkerRunOnceReturnsProcessingFailure(t *testing.T) {
	now := time.Date(2026, 5, 31, 15, 30, 0, 0, time.UTC)
	claimer := &stubRawEventClaimer{
		claims: []governance.ClaimedRawEvent{newClaimedRawEvent(t, "evt_1", now)},
	}
	processor := &stubRawEventProcessor{err: errors.New("processing failed")}
	worker := GovernanceWorker{
		Claimer:       claimer,
		Processor:     processor,
		WorkerID:      "worker-a",
		BatchSize:     1,
		LeaseDuration: time.Minute,
		Now: func() time.Time {
			return now
		},
	}

	if _, err := worker.RunOnce(context.Background()); err == nil {
		t.Fatal("RunOnce() error = nil, want processing failure")
	}
}

func TestGovernanceWorkerRunOnceEmitsTelemetryOperation(t *testing.T) {
	now := time.Date(2026, 6, 7, 13, 30, 0, 0, time.UTC)
	observer := &stubJobsObserver{}
	worker := GovernanceWorker{
		Claimer: &stubRawEventClaimer{
			claims: []governance.ClaimedRawEvent{newClaimedRawEvent(t, "evt_telemetry", now)},
		},
		Processor:     &stubRawEventProcessor{},
		WorkerID:      "worker-a",
		BatchSize:     1,
		LeaseDuration: time.Minute,
		Now:           func() time.Time { return now },
		Observer:      observer,
	}

	_, err := worker.RunOnce(context.Background())
	if err != nil {
		t.Fatalf("RunOnce() error = %v", err)
	}

	if len(observer.operations) != 1 {
		t.Fatalf("len(observer.operations) = %d, want 1", len(observer.operations))
	}

	if observer.operations[0].Operation != "governance" || observer.operations[0].Status != "ok" {
		t.Fatalf("operation event = %+v, want governance ok", observer.operations[0])
	}
}

func TestGovernanceWorkerRunOnceSkipsAlreadyClaimedEvents(t *testing.T) {
	now := time.Date(2026, 6, 1, 10, 0, 0, 0, time.UTC)
	claimer := &leaseAwareClaimer{
		event: newClaimedRawEvent(t, "evt_lease", now).Event,
	}
	worker := GovernanceWorker{
		Claimer:       claimer,
		Processor:     &stubRawEventProcessor{},
		WorkerID:      "worker-a",
		BatchSize:     1,
		LeaseDuration: 2 * time.Minute,
		Now:           func() time.Time { return now },
	}

	processed, err := worker.RunOnce(context.Background())
	if err != nil {
		t.Fatalf("RunOnce() first error = %v", err)
	}

	if processed != 1 {
		t.Fatalf("first processed = %d, want %d", processed, 1)
	}

	worker.Now = func() time.Time { return now.Add(time.Minute) }
	processed, err = worker.RunOnce(context.Background())
	if err != nil {
		t.Fatalf("RunOnce() second error = %v", err)
	}

	if processed != 0 {
		t.Fatalf("second processed = %d, want %d", processed, 0)
	}
}

func TestGovernanceWorkerRunOnceSkipsAlreadyProcessedEvents(t *testing.T) {
	now := time.Date(2026, 6, 1, 10, 5, 0, 0, time.UTC)
	state := &processedState{}
	claimer := &processedAwareClaimer{
		state: state,
		event: newClaimedRawEvent(t, "evt_processed", now).Event,
	}
	worker := GovernanceWorker{
		Claimer:       claimer,
		Processor:     &markingProcessor{state: state},
		WorkerID:      "worker-a",
		BatchSize:     1,
		LeaseDuration: 2 * time.Minute,
		Now:           func() time.Time { return now },
	}

	processed, err := worker.RunOnce(context.Background())
	if err != nil {
		t.Fatalf("RunOnce() first error = %v", err)
	}

	if processed != 1 {
		t.Fatalf("first processed = %d, want %d", processed, 1)
	}

	worker.Now = func() time.Time { return now.Add(time.Minute) }
	processed, err = worker.RunOnce(context.Background())
	if err != nil {
		t.Fatalf("RunOnce() second error = %v", err)
	}

	if processed != 0 {
		t.Fatalf("second processed = %d, want %d", processed, 0)
	}
}

func newClaimedRawEvent(t *testing.T, id string, now time.Time) governance.ClaimedRawEvent {
	t.Helper()

	claim := governance.ClaimedRawEvent{
		Event: memory.RawEvent{
			ID: id,
			Scope: memory.Scope{
				Tenant:    "tenant-a",
				Project:   "project-a",
				Namespace: "namespace-a",
			},
			EventType: "conversation.message",
			Content:   "hello",
			CreatedAt: now.Add(-time.Minute),
		},
		WorkerID:   "worker-a",
		ClaimedAt:  now,
		LeaseUntil: now.Add(2 * time.Minute),
		Attempt:    1,
	}

	if err := claim.Validate(); err != nil {
		t.Fatalf("claim.Validate() error = %v", err)
	}

	return claim
}

type leaseAwareClaimer struct {
	event      memory.RawEvent
	claimed    bool
	leaseUntil time.Time
}

func (c *leaseAwareClaimer) ClaimPendingRawEvents(ctx context.Context, input governance.ClaimPendingRawEventsInput) ([]governance.ClaimedRawEvent, error) {
	if c.claimed && input.Now.Before(c.leaseUntil) {
		return nil, nil
	}

	c.claimed = true
	c.leaseUntil = input.Now.Add(input.LeaseDuration)
	return []governance.ClaimedRawEvent{{
		Event:      c.event,
		WorkerID:   input.WorkerID,
		ClaimedAt:  input.Now,
		LeaseUntil: c.leaseUntil,
		Attempt:    1,
	}}, nil
}

type processedState struct {
	done bool
}

type processedAwareClaimer struct {
	state *processedState
	event memory.RawEvent
}

func (c *processedAwareClaimer) ClaimPendingRawEvents(ctx context.Context, input governance.ClaimPendingRawEventsInput) ([]governance.ClaimedRawEvent, error) {
	if c.state.done {
		return nil, nil
	}

	return []governance.ClaimedRawEvent{{
		Event:      c.event,
		WorkerID:   input.WorkerID,
		ClaimedAt:  input.Now,
		LeaseUntil: input.Now.Add(input.LeaseDuration),
		Attempt:    1,
	}}, nil
}

type markingProcessor struct {
	state *processedState
}

func (p *markingProcessor) ProcessClaimedRawEvent(ctx context.Context, claim governance.ClaimedRawEvent) error {
	p.state.done = true
	return nil
}
