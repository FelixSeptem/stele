package jobs

import (
	"context"
	"errors"
	"sync"
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

type selectiveRawEventProcessor struct {
	gotClaims []governance.ClaimedRawEvent
	errs      map[string]error
}

func (s *selectiveRawEventProcessor) ProcessClaimedRawEvent(ctx context.Context, claim governance.ClaimedRawEvent) error {
	s.gotClaims = append(s.gotClaims, claim)
	if err, ok := s.errs[claim.Event.ID]; ok {
		return err
	}

	return nil
}

type blockingProcessor struct {
	release chan struct{}
}

func (p *blockingProcessor) ProcessClaimedRawEvent(ctx context.Context, claim governance.ClaimedRawEvent) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-p.release:
		return nil
	}
}

type stubRawEventFailureRecorder struct {
	inputs []governance.RecordClaimedRawEventFailureInput
	err    error
}

func (s *stubRawEventFailureRecorder) RecordClaimedRawEventFailure(ctx context.Context, input governance.RecordClaimedRawEventFailureInput) error {
	s.inputs = append(s.inputs, input)
	return s.err
}

type stubRawEventLeaseRenewer struct {
	inputs   []governance.RenewClaimedRawEventLeaseInput
	err      error
	signal   chan struct{}
	signalMu sync.Once
}

func (s *stubRawEventLeaseRenewer) RenewClaimedRawEventLease(ctx context.Context, input governance.RenewClaimedRawEventLeaseInput) error {
	s.inputs = append(s.inputs, input)
	if s.signal != nil {
		s.signalMu.Do(func() {
			close(s.signal)
		})
	}
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

func TestGovernanceWorkerRunOnceRecordsRetryableFailureAndContinues(t *testing.T) {
	now := time.Date(2026, 6, 10, 12, 0, 0, 0, time.UTC)
	claimer := &stubRawEventClaimer{
		claims: []governance.ClaimedRawEvent{
			newClaimedRawEvent(t, "evt_fail", now),
			newClaimedRawEvent(t, "evt_ok", now.Add(time.Second)),
		},
	}
	processor := &selectiveRawEventProcessor{
		errs: map[string]error{
			"evt_fail": errors.New("processing failed"),
		},
	}
	recorder := &stubRawEventFailureRecorder{}
	worker := GovernanceWorker{
		Claimer:         claimer,
		Processor:       processor,
		FailureRecorder: recorder,
		WorkerID:        "worker-a",
		BatchSize:       2,
		LeaseDuration:   2 * time.Minute,
		RetryBackoff:    30 * time.Second,
		MaxAttempts:     3,
		Now:             func() time.Time { return now },
	}

	processed, err := worker.RunOnce(context.Background())
	if err != nil {
		t.Fatalf("RunOnce() error = %v", err)
	}

	if processed != 1 {
		t.Fatalf("processed = %d, want %d", processed, 1)
	}

	if len(processor.gotClaims) != 2 {
		t.Fatalf("len(processor.gotClaims) = %d, want 2", len(processor.gotClaims))
	}

	if len(recorder.inputs) != 1 {
		t.Fatalf("len(recorder.inputs) = %d, want 1", len(recorder.inputs))
	}

	if recorder.inputs[0].RawEventID != "evt_fail" {
		t.Fatalf("RawEventID = %q, want evt_fail", recorder.inputs[0].RawEventID)
	}

	if !recorder.inputs[0].NextAttemptAt.Equal(now.Add(30 * time.Second)) {
		t.Fatalf("NextAttemptAt = %v, want %v", recorder.inputs[0].NextAttemptAt, now.Add(30*time.Second))
	}

	if !recorder.inputs[0].ExhaustedAt.IsZero() {
		t.Fatalf("ExhaustedAt = %v, want zero", recorder.inputs[0].ExhaustedAt)
	}
}

func TestGovernanceWorkerRunOnceExhaustsRawEventAtMaxAttempts(t *testing.T) {
	now := time.Date(2026, 6, 10, 12, 5, 0, 0, time.UTC)
	claim := newClaimedRawEvent(t, "evt_exhaust", now)
	claim.Attempt = 3

	recorder := &stubRawEventFailureRecorder{}
	worker := GovernanceWorker{
		Claimer: &stubRawEventClaimer{
			claims: []governance.ClaimedRawEvent{claim},
		},
		Processor:       &stubRawEventProcessor{err: errors.New("processing failed")},
		FailureRecorder: recorder,
		WorkerID:        "worker-a",
		BatchSize:       1,
		LeaseDuration:   2 * time.Minute,
		RetryBackoff:    30 * time.Second,
		MaxAttempts:     3,
		Now:             func() time.Time { return now },
	}

	processed, err := worker.RunOnce(context.Background())
	if err != nil {
		t.Fatalf("RunOnce() error = %v", err)
	}

	if processed != 0 {
		t.Fatalf("processed = %d, want 0", processed)
	}

	if len(recorder.inputs) != 1 {
		t.Fatalf("len(recorder.inputs) = %d, want 1", len(recorder.inputs))
	}

	if recorder.inputs[0].ExhaustedAt.IsZero() {
		t.Fatal("ExhaustedAt = zero, want exhaustion timestamp")
	}

	if !recorder.inputs[0].NextAttemptAt.IsZero() {
		t.Fatalf("NextAttemptAt = %v, want zero", recorder.inputs[0].NextAttemptAt)
	}
}

func TestGovernanceWorkerRunOnceRenewsLeaseDuringLongProcessing(t *testing.T) {
	now := time.Date(2026, 6, 10, 12, 10, 0, 0, time.UTC)
	release := make(chan struct{})
	renewer := &stubRawEventLeaseRenewer{signal: make(chan struct{})}
	worker := GovernanceWorker{
		Claimer: &stubRawEventClaimer{
			claims: []governance.ClaimedRawEvent{newClaimedRawEvent(t, "evt_lease", now)},
		},
		Processor:          &blockingProcessor{release: release},
		LeaseRenewer:       renewer,
		WorkerID:           "worker-a",
		BatchSize:          1,
		LeaseDuration:      100 * time.Millisecond,
		LeaseRenewInterval: 10 * time.Millisecond,
		Now:                func() time.Time { return now },
	}

	type runResult struct {
		processed int
		err       error
	}

	done := make(chan runResult, 1)
	go func() {
		processed, err := worker.RunOnce(context.Background())
		done <- runResult{processed: processed, err: err}
	}()

	select {
	case <-renewer.signal:
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for lease renewal")
	}

	close(release)

	select {
	case result := <-done:
		if result.err != nil {
			t.Fatalf("RunOnce() error = %v", result.err)
		}
		if result.processed != 1 {
			t.Fatalf("processed = %d, want 1", result.processed)
		}
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for worker completion")
	}

	if len(renewer.inputs) == 0 {
		t.Fatal("len(renewer.inputs) = 0, want at least one renewal")
	}

	if renewer.inputs[0].RawEventID != "evt_lease" {
		t.Fatalf("RawEventID = %q, want evt_lease", renewer.inputs[0].RawEventID)
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
