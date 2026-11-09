package jobs

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/FelixSeptem/stele/internal/memory"
	"github.com/FelixSeptem/stele/internal/telemetry"
	"github.com/FelixSeptem/stele/internal/workqueue"
)

func TestProceduralRevalidationWorkerAppendsNeedsRevalidationForEvidenceDrift(t *testing.T) {
	store := &proceduralRevalidationWorkerStoreStub{run: workerRevalidationRun(), verification: workerVerification()}
	observer := telemetry.NewMetricsObserver()
	worker := ProceduralRevalidationWorker{Store: store, Targets: proceduralTargetResolver{}, Evidence: proceduralEvidenceResolver{err: errors.New("source drift")}, Policy: workerPolicy(true), WorkerID: "revalidation-worker", Observer: observer, Now: func() time.Time { return time.Date(2026, time.November, 8, 2, 0, 0, 0, time.UTC) }, NewID: func() string { return "verification-drift" }}
	_, evidence, err := worker.ExecuteDerivedWork(context.Background(), workerDerivedItem())
	if err != nil {
		t.Fatal(err)
	}
	if evidence != "verification-drift" || len(store.appended) != 1 || store.appended[0].Disposition != memory.ProceduralVerificationDispositionNeedsRevalidation || store.appended[0].CorrectedContent != "" {
		t.Fatalf("appended = %+v, evidence = %q", store.appended, evidence)
	}
	if len(store.completed) != 1 || store.completed[0].Status != memory.ProceduralRevalidationRunStatusCompleted {
		t.Fatalf("completed = %+v", store.completed)
	}
	if !strings.Contains(observer.RenderPrometheus(), `disposition="needs_revalidation"`) {
		t.Fatalf("worker did not emit bounded revalidation metric: %s", observer.RenderPrometheus())
	}
}

func TestProceduralRevalidationWorkerAppendsInconclusiveWhenEvidenceRemainsEligible(t *testing.T) {
	store := &proceduralRevalidationWorkerStoreStub{run: workerRevalidationRun(), verification: workerVerification()}
	worker := ProceduralRevalidationWorker{Store: store, Targets: proceduralTargetResolver{}, Evidence: proceduralEvidenceResolver{}, Policy: workerPolicy(true), WorkerID: "revalidation-worker", Now: func() time.Time { return time.Date(2026, time.November, 8, 2, 0, 0, 0, time.UTC) }, NewID: func() string { return "verification-inconclusive" }}
	_, _, err := worker.ExecuteDerivedWork(context.Background(), workerDerivedItem())
	if err != nil {
		t.Fatal(err)
	}
	if len(store.appended) != 1 || store.appended[0].Disposition != memory.ProceduralVerificationDispositionInconclusive {
		t.Fatalf("appended = %+v", store.appended)
	}
}

func TestProceduralRevalidationWorkerSettlesStaleTargetWithoutAppendingInference(t *testing.T) {
	store := &proceduralRevalidationWorkerStoreStub{run: workerRevalidationRun(), verification: workerVerification()}
	observer := telemetry.NewMetricsObserver()
	worker := ProceduralRevalidationWorker{
		Store: store, Targets: proceduralTargetResolver{err: errors.New("target no longer current")}, Evidence: proceduralEvidenceResolver{}, Policy: workerPolicy(true),
		WorkerID: "revalidation-worker", Now: func() time.Time { return time.Date(2026, time.November, 8, 2, 0, 0, 0, time.UTC) }, NewID: func() string { return "must-not-append" }, Observer: observer,
	}
	processed, evidence, err := worker.ExecuteDerivedWork(context.Background(), workerDerivedItem())
	if err != nil {
		t.Fatal(err)
	}
	if processed != 1 || evidence != "verification-1" || len(store.appended) != 0 {
		t.Fatalf("stale target processed=%d evidence=%q appended=%+v, want settled without inferred result", processed, evidence, store.appended)
	}
	if len(store.completed) != 1 || store.completed[0].Status != memory.ProceduralRevalidationRunStatusFailed || store.completed[0].TerminalVerificationID != "verification-1" {
		t.Fatalf("stale target completion=%+v, want failed original verification terminal", store.completed)
	}
	metrics := observer.RenderPrometheus()
	for _, want := range []string{`operation="revalidate"`, `result="failed"`, `failure_category="target_ineligible"`} {
		if !strings.Contains(metrics, want) {
			t.Fatalf("stale target telemetry missing %s:\n%s", want, metrics)
		}
	}
}

func TestProceduralRevalidationWorkerRejectsForeignOrWrongKindBeforeLookup(t *testing.T) {
	store := &proceduralRevalidationWorkerStoreStub{run: workerRevalidationRun(), verification: workerVerification()}
	worker := ProceduralRevalidationWorker{Store: store, Targets: proceduralTargetResolver{}, Evidence: proceduralEvidenceResolver{}, Policy: workerPolicy(true), WorkerID: "revalidation-worker"}
	item := workerDerivedItem()
	item.Kind = workqueue.WorkKindReflection
	if _, _, err := worker.ExecuteDerivedWork(context.Background(), item); err == nil {
		t.Fatal("wrong kind executed")
	}
	if store.readRuns != 0 {
		t.Fatalf("wrong kind lookup count = %d", store.readRuns)
	}
}

func TestProceduralRevalidationWorkerRejectsDisabledPolicyBeforeRunClaim(t *testing.T) {
	store := &proceduralRevalidationWorkerStoreStub{run: workerRevalidationRun(), verification: workerVerification()}
	worker := ProceduralRevalidationWorker{Store: store, Targets: proceduralTargetResolver{}, Evidence: proceduralEvidenceResolver{}, Policy: workerPolicy(false), WorkerID: "revalidation-worker"}
	if _, _, err := worker.ExecuteDerivedWork(context.Background(), workerDerivedItem()); err == nil {
		t.Fatal("disabled policy executed revalidation")
	}
	if store.readRuns != 0 {
		t.Fatalf("disabled policy claimed run %d times", store.readRuns)
	}
}

func TestProceduralRevalidationWorkerSettlesOwnedRunWhenDerivedWorkExhausts(t *testing.T) {
	store := &proceduralRevalidationWorkerStoreStub{run: workerRevalidationRun(), verification: workerVerification()}
	observer := telemetry.NewMetricsObserver()
	worker := ProceduralRevalidationWorker{
		Store: store, Targets: proceduralTargetResolver{}, Evidence: proceduralEvidenceResolver{}, Policy: workerPolicy(true), WorkerID: "revalidation-worker",
		Now: func() time.Time { return time.Date(2026, time.November, 8, 2, 0, 0, 0, time.UTC) }, Observer: observer,
	}
	if err := worker.HandleDerivedWorkExhaustion(context.Background(), workerDerivedItem()); err != nil {
		t.Fatal(err)
	}
	if len(store.exhausted) != 1 || store.exhausted[0].RunID != "run-1" || store.exhausted[0].WorkerID != "revalidation-worker" {
		t.Fatalf("exhausted runs = %+v", store.exhausted)
	}
	metrics := observer.RenderPrometheus()
	for _, want := range []string{`operation="exhaust"`, `result="exhausted"`, `recovery="exhausted"`} {
		if !strings.Contains(metrics, want) {
			t.Fatalf("exhaustion telemetry missing %s:\n%s", want, metrics)
		}
	}
}

func workerDerivedItem() workqueue.DerivedWorkItem {
	return workqueue.DerivedWorkItem{ID: "work-1", DerivedWorkIdentity: workqueue.DerivedWorkIdentity{Scope: memory.Scope{Tenant: "tenant-a", Project: "project-a", Namespace: "namespace-a"}}, Kind: workqueue.WorkKindProceduralRevalidation, Reference: "run-1", Watermark: "watermark-1"}
}

func workerRevalidationRun() memory.ProceduralRevalidationRun {
	return memory.ProceduralRevalidationRun{ID: "run-1", Scope: memory.Scope{Tenant: "tenant-a", Project: "project-a", Namespace: "namespace-a"}, VerificationID: "verification-1", MemoryID: "memory-1", TargetVersion: 2, SourceWatermark: "watermark-1", Trigger: memory.ProceduralRevalidationTriggerSourceDrift, WorkIdentity: "identity-1", Status: memory.ProceduralRevalidationRunStatusPending, MaxAttempts: 3, CreatedAt: time.Date(2026, time.November, 8, 1, 0, 0, 0, time.UTC)}
}

func workerVerification() memory.ProceduralMemoryVerification {
	return memory.ProceduralMemoryVerification{ID: "verification-1", Scope: memory.Scope{Tenant: "tenant-a", Project: "project-a", Namespace: "namespace-a"}, MemoryID: "memory-1", TargetVersion: 2, ClaimFingerprint: strings.Repeat("a", 64), ApplicabilityFingerprint: strings.Repeat("b", 64), Disposition: memory.ProceduralVerificationDispositionSupported, Evidence: []memory.ProceduralVerificationEvidence{{Kind: memory.ProceduralVerificationEvidenceToolObservation, SourceKind: memory.ProceduralVerificationEvidenceSourceRawEvent, ID: "event-1", SourceVersion: 1, Corroborates: true}}, Actor: "admin", Reason: "checked", PolicyVersion: "policy-v1", SourceWatermark: "watermark-1", CreatedAt: time.Date(2026, time.November, 8, 1, 0, 0, 0, time.UTC)}
}

func workerPolicy(enabled bool) memory.StaticProceduralVerificationPolicy {
	return memory.StaticProceduralVerificationPolicy{Scope: memory.Scope{Tenant: "tenant-a", Project: "project-a", Namespace: "namespace-a"}, Enabled: enabled, Version: "policy-v1"}
}

type proceduralRevalidationWorkerStoreStub struct {
	run          memory.ProceduralRevalidationRun
	verification memory.ProceduralMemoryVerification
	appended     []memory.ProceduralMemoryVerification
	completed    []memory.CompleteProceduralRevalidationRunInput
	exhausted    []memory.ExhaustProceduralRevalidationRunInput
	readRuns     int
	claimed      []memory.ClaimProceduralRevalidationRunInput
}

func (s *proceduralRevalidationWorkerStoreStub) ClaimProceduralRevalidationRun(_ context.Context, input memory.ClaimProceduralRevalidationRunInput) (memory.ProceduralRevalidationRun, error) {
	s.readRuns++
	s.claimed = append(s.claimed, input)
	s.run.Status = memory.ProceduralRevalidationRunStatusRunning
	s.run.LeaseOwner = input.WorkerID
	return s.run, nil
}
func (s *proceduralRevalidationWorkerStoreStub) ReadProceduralMemoryVerification(context.Context, memory.Scope, string) (memory.ProceduralMemoryVerification, error) {
	return s.verification, nil
}
func (s *proceduralRevalidationWorkerStoreStub) AppendProceduralMemoryVerification(_ context.Context, record memory.ProceduralMemoryVerification) (memory.ProceduralMemoryVerification, error) {
	s.appended = append(s.appended, record)
	return record, nil
}
func (s *proceduralRevalidationWorkerStoreStub) CompleteProceduralRevalidationRun(_ context.Context, input memory.CompleteProceduralRevalidationRunInput) error {
	s.completed = append(s.completed, input)
	return nil
}
func (s *proceduralRevalidationWorkerStoreStub) ExhaustProceduralRevalidationRun(_ context.Context, input memory.ExhaustProceduralRevalidationRunInput) error {
	s.exhausted = append(s.exhausted, input)
	return nil
}

type proceduralTargetResolver struct{ err error }

func (r proceduralTargetResolver) ValidateProceduralVerificationTarget(context.Context, memory.ProceduralMemoryVerification) error {
	return r.err
}

type proceduralEvidenceResolver struct{ err error }

func (r proceduralEvidenceResolver) ValidateProceduralVerificationEvidence(context.Context, memory.Scope, memory.ProceduralVerificationEvidence) error {
	return r.err
}
