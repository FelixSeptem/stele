package jobs

import (
	"bytes"
	"context"
	"errors"
	"log"
	"strings"
	"testing"
	"time"

	"github.com/FelixSeptem/stele/internal/memory"
	"github.com/FelixSeptem/stele/internal/telemetry"
)

func TestScopeProofStepWorkerCompletesClaimedStep(t *testing.T) {
	now := time.Date(2026, 7, 11, 21, 0, 0, 0, time.UTC)
	scope := memory.Scope{Tenant: "tenant-a", Project: "project-a", Namespace: "namespace-a"}
	store := &stubScopeProofStepWorkerStore{
		claims: []memory.ScopeProofStep{{
			ID:       "step_1",
			ProofID:  "proof_1",
			Scope:    scope,
			Step:     memory.ScopeProofStepScopeResolved,
			Status:   memory.ScopeProofStepStatusRunning,
			WorkerID: "worker-a",
			Attempt:  1,
		}},
	}
	executor := &stubScopeProofStepExecutor{
		result: memory.ScopeProofStepResult{
			Verdict:  memory.ScopeProofVerdictPassed,
			Evidence: map[string]any{"scope_resolved": true},
		},
	}
	worker := ScopeProofStepWorker{
		Store:         store,
		Executor:      executor,
		Scope:         scope,
		WorkerID:      "worker-a",
		BatchSize:     2,
		LeaseDuration: time.Minute,
		Now:           func() time.Time { return now },
	}

	processed, err := worker.RunOnce(context.Background())
	if err != nil {
		t.Fatalf("RunOnce() error = %v", err)
	}
	if processed != 1 || executor.gotStep.ID != "step_1" || store.completed.StepID != "step_1" {
		t.Fatalf("processed=%d executor=%+v completed=%+v, want step completed", processed, executor.gotStep, store.completed)
	}
	if store.completed.Status != memory.ScopeProofStepStatusCompleted || store.completed.Verdict != memory.ScopeProofVerdictPassed {
		t.Fatalf("completed = %+v, want completed passed", store.completed)
	}
}

func TestScopeProofStepWorkerRecordsRetryableFailureAndExhaustion(t *testing.T) {
	now := time.Date(2026, 7, 11, 21, 5, 0, 0, time.UTC)
	scope := memory.Scope{Tenant: "tenant-a", Project: "project-a", Namespace: "namespace-a"}
	store := &stubScopeProofStepWorkerStore{
		claims: []memory.ScopeProofStep{{
			ID:       "step_retry",
			ProofID:  "proof_1",
			Scope:    scope,
			Step:     memory.ScopeProofStepIngestion,
			Status:   memory.ScopeProofStepStatusRunning,
			WorkerID: "worker-a",
			Attempt:  1,
		}, {
			ID:       "step_exhaust",
			ProofID:  "proof_1",
			Scope:    scope,
			Step:     memory.ScopeProofStepContextAssembled,
			Status:   memory.ScopeProofStepStatusRunning,
			WorkerID: "worker-a",
			Attempt:  3,
		}},
	}
	worker := ScopeProofStepWorker{
		Store:         store,
		Executor:      &stubScopeProofStepExecutor{err: errors.New("dependency unavailable")},
		Scope:         scope,
		WorkerID:      "worker-a",
		BatchSize:     2,
		LeaseDuration: time.Minute,
		MaxAttempts:   3,
		RetryBackoff:  5 * time.Minute,
		Now:           func() time.Time { return now },
	}

	processed, err := worker.RunOnce(context.Background())
	if err != nil {
		t.Fatalf("RunOnce() error = %v", err)
	}
	if processed != 0 || len(store.failed) != 2 {
		t.Fatalf("processed=%d failed=%+v, want two recorded failures", processed, store.failed)
	}
	if store.failed[0].StepID != "step_retry" || !store.failed[0].NextAttemptAt.Equal(now.Add(5*time.Minute)) {
		t.Fatalf("retry failure = %+v, want next attempt", store.failed[0])
	}
	if store.failed[1].StepID != "step_exhaust" || store.failed[1].Status != memory.ScopeProofStepStatusExhausted || !store.failed[1].NextAttemptAt.IsZero() {
		t.Fatalf("exhaust failure = %+v, want exhausted without retry", store.failed[1])
	}
}

func TestScopeProofStepWorkerReducesFinalVerdict(t *testing.T) {
	now := time.Date(2026, 7, 11, 21, 8, 0, 0, time.UTC)
	scope := memory.Scope{Tenant: "tenant-a", Project: "project-a", Namespace: "namespace-a"}
	store := &stubScopeProofStepWorkerStore{
		claims: []memory.ScopeProofStep{{
			ID:       "step_final",
			ProofID:  "proof_1",
			Scope:    scope,
			Step:     memory.ScopeProofStepCompleted,
			Status:   memory.ScopeProofStepStatusRunning,
			WorkerID: "worker-a",
			Attempt:  1,
		}},
	}
	worker := ScopeProofStepWorker{
		Store: store,
		Executor: &stubScopeProofStepExecutor{result: memory.ScopeProofStepResult{
			Verdict:         memory.ScopeProofVerdictPassedDegraded,
			FailureCategory: memory.ProofFailureCategoryContext,
			Evidence:        map[string]any{"passed_steps": 9, "degraded_steps": 1},
		}},
		Scope:         scope,
		WorkerID:      "worker-a",
		BatchSize:     1,
		LeaseDuration: time.Minute,
		Now:           func() time.Time { return now },
	}

	processed, err := worker.RunOnce(context.Background())
	if err != nil {
		t.Fatalf("RunOnce() error = %v", err)
	}
	if processed != 1 {
		t.Fatalf("processed = %d, want 1", processed)
	}
	if store.updatedRun.ProofID != "proof_1" || store.updatedRun.Status != memory.ScopeProofStatusCompleted || store.updatedRun.Verdict != memory.ScopeProofVerdictPassedDegraded {
		t.Fatalf("updated run = %+v, want completed passed_degraded proof_1", store.updatedRun)
	}
	if store.updatedRun.FailureCategory != memory.ProofFailureCategoryContext {
		t.Fatalf("failure category = %q, want context", store.updatedRun.FailureCategory)
	}
}

func TestScopeProofStepWorkerTreatsDuplicateDispatchAsNoop(t *testing.T) {
	now := time.Date(2026, 7, 11, 21, 9, 0, 0, time.UTC)
	scope := memory.Scope{Tenant: "tenant-a", Project: "project-a", Namespace: "namespace-a"}
	executor := &stubScopeProofStepExecutor{}
	worker := ScopeProofStepWorker{
		Store:         &stubScopeProofStepWorkerStore{},
		Executor:      executor,
		Scope:         scope,
		WorkerID:      "worker-a",
		BatchSize:     1,
		LeaseDuration: time.Minute,
		Now:           func() time.Time { return now },
	}

	processed, err := worker.RunOnce(context.Background())
	if err != nil {
		t.Fatalf("RunOnce() error = %v", err)
	}
	if processed != 0 || executor.called {
		t.Fatalf("processed=%d executor.called=%v, want duplicate/no-claim noop", processed, executor.called)
	}
}

func TestMemorySessionVerificationWorkerCompletesAndRecordsFailure(t *testing.T) {
	now := time.Date(2026, 7, 11, 21, 10, 0, 0, time.UTC)
	scope := memory.Scope{Tenant: "tenant-a", Project: "project-a", Namespace: "namespace-a"}
	store := &stubSessionVerificationWorkerStore{
		claims: []memory.MemorySessionVerification{{
			ID:        "verification_ok",
			SessionID: "session_1",
			TurnID:    "turn_1",
			Scope:     scope,
			Status:    memory.ScopeProofStepStatusRunning,
			Verdict:   memory.ScopeProofVerdictPending,
			WorkerID:  "worker-a",
			Attempt:   1,
		}},
	}
	runner := &stubSessionVerificationRunner{
		result: memory.MemorySessionVerificationResult{
			Verdict:  memory.ScopeProofVerdictPassed,
			Evidence: map[string]any{"recalled": true},
		},
	}
	worker := MemorySessionVerificationWorker{
		Store:         store,
		Runner:        runner,
		Scope:         scope,
		WorkerID:      "worker-a",
		BatchSize:     1,
		LeaseDuration: time.Minute,
		Now:           func() time.Time { return now },
	}

	processed, err := worker.RunOnce(context.Background())
	if err != nil {
		t.Fatalf("RunOnce() error = %v", err)
	}
	if processed != 1 || runner.gotVerification.ID != "verification_ok" || store.completed.VerificationID != "verification_ok" {
		t.Fatalf("processed=%d runner=%+v completed=%+v, want verification completed", processed, runner.gotVerification, store.completed)
	}
	if store.updatedSession.SessionID != "session_1" || store.updatedSession.Status != memory.MemorySessionStatusCompleted || store.updatedSession.Verdict != memory.ScopeProofVerdictPassed {
		t.Fatalf("updated session = %+v, want completed passed session_1", store.updatedSession)
	}
	if store.updatedTurn.TurnID != "turn_1" || store.updatedTurn.Status != memory.MemorySessionTurnStatusVerified || store.updatedTurn.VerificationStatus != memory.ScopeProofVerdictPassed {
		t.Fatalf("updated turn = %+v, want verified passed turn_1", store.updatedTurn)
	}

	store = &stubSessionVerificationWorkerStore{
		claims: []memory.MemorySessionVerification{{
			ID:        "verification_retry",
			SessionID: "session_1",
			Scope:     scope,
			Status:    memory.ScopeProofStepStatusRunning,
			WorkerID:  "worker-a",
			Attempt:   1,
		}},
	}
	worker.Store = store
	worker.Runner = &stubSessionVerificationRunner{err: errors.New("bounded wait exceeded")}
	worker.RetryBackoff = 5 * time.Minute

	processed, err = worker.RunOnce(context.Background())
	if err != nil {
		t.Fatalf("failure RunOnce() error = %v", err)
	}
	if processed != 0 || store.failed.VerificationID != "verification_retry" || !store.failed.NextAttemptAt.Equal(now.Add(5*time.Minute)) {
		t.Fatalf("processed=%d failed=%+v, want retryable verification failure", processed, store.failed)
	}
	if store.updatedSession.SessionID != "session_1" || store.updatedSession.Status != memory.MemorySessionStatusVerifying {
		t.Fatalf("retry updated session = %+v, want verifying session_1", store.updatedSession)
	}
}

func TestMemorySessionVerificationWorkerRecordsBoundedDegradation(t *testing.T) {
	now := time.Date(2026, 7, 11, 21, 12, 0, 0, time.UTC)
	scope := memory.Scope{Tenant: "tenant-a", Project: "project-a", Namespace: "namespace-a"}
	store := &stubSessionVerificationWorkerStore{
		claims: []memory.MemorySessionVerification{{
			ID:        "verification_degraded",
			SessionID: "session_1",
			TurnID:    "turn_1",
			Scope:     scope,
			Status:    memory.ScopeProofStepStatusRunning,
			Verdict:   memory.ScopeProofVerdictPending,
			WorkerID:  "worker-a",
			Attempt:   1,
		}},
	}
	worker := MemorySessionVerificationWorker{
		Store: store,
		Runner: &stubSessionVerificationRunner{result: memory.MemorySessionVerificationResult{
			Verdict:         memory.ScopeProofVerdictPassedDegraded,
			FailureCategory: memory.ProofFailureCategoryContext,
			Evidence:        map[string]any{"degraded_reason": "bounded_wait_exceeded"},
		}},
		Scope:         scope,
		WorkerID:      "worker-a",
		BatchSize:     1,
		LeaseDuration: time.Minute,
		Now:           func() time.Time { return now },
	}

	processed, err := worker.RunOnce(context.Background())
	if err != nil {
		t.Fatalf("RunOnce() error = %v", err)
	}
	if processed != 1 {
		t.Fatalf("processed = %d, want 1", processed)
	}
	if store.updatedSession.Verdict != memory.ScopeProofVerdictPassedDegraded || store.updatedSession.FailureCategory != memory.ProofFailureCategoryContext {
		t.Fatalf("updated session = %+v, want passed_degraded context", store.updatedSession)
	}
	if store.updatedTurn.VerificationStatus != memory.ScopeProofVerdictPassedDegraded {
		t.Fatalf("updated turn = %+v, want passed_degraded verification", store.updatedTurn)
	}
}

func TestScopeProofAndSessionWorkersRecordLowCardinalityMetrics(t *testing.T) {
	now := time.Date(2026, 7, 11, 21, 15, 0, 0, time.UTC)
	scope := memory.Scope{Tenant: "tenant-a", Project: "project-a", Namespace: "namespace-a"}
	observer := telemetry.NewMetricsObserver()
	proofStore := &stubScopeProofStepWorkerStore{
		claims: []memory.ScopeProofStep{{
			ID:       "step_context",
			ProofID:  "proof_1",
			Scope:    scope,
			Step:     memory.ScopeProofStepContextAssembled,
			Status:   memory.ScopeProofStepStatusRunning,
			WorkerID: "worker-a",
			Attempt:  1,
		}},
	}
	proofWorker := ScopeProofStepWorker{
		Store: proofStore,
		Executor: &stubScopeProofStepExecutor{result: memory.ScopeProofStepResult{
			Verdict:         memory.ScopeProofVerdictPassedDegraded,
			FailureCategory: memory.ProofFailureCategoryContext,
			Evidence:        map[string]any{"memory_ids": []string{"mem_1"}, "event_id": "evt_1"},
		}},
		Scope:         scope,
		WorkerID:      "worker-a",
		BatchSize:     1,
		LeaseDuration: time.Minute,
		Now:           func() time.Time { return now },
		Observer:      observer,
	}

	if _, err := proofWorker.RunOnce(context.Background()); err != nil {
		t.Fatalf("proof RunOnce() error = %v", err)
	}

	sessionStore := &stubSessionVerificationWorkerStore{
		claims: []memory.MemorySessionVerification{{
			ID:        "verification_1",
			SessionID: "session_1",
			TurnID:    "turn_1",
			Scope:     scope,
			Status:    memory.ScopeProofStepStatusRunning,
			WorkerID:  "worker-a",
			Attempt:   1,
		}},
	}
	sessionWorker := MemorySessionVerificationWorker{
		Store: sessionStore,
		Runner: &stubSessionVerificationRunner{result: memory.MemorySessionVerificationResult{
			Verdict:         memory.ScopeProofVerdictPassedDegraded,
			FailureCategory: memory.ProofFailureCategoryRetrieval,
			Evidence:        map[string]any{"memory_ids": []string{"mem_2"}, "citation_ids": []string{"evt_2"}},
		}},
		Scope:         scope,
		WorkerID:      "worker-a",
		BatchSize:     1,
		LeaseDuration: time.Minute,
		Now:           func() time.Time { return now },
		Observer:      observer,
	}

	if _, err := sessionWorker.RunOnce(context.Background()); err != nil {
		t.Fatalf("session RunOnce() error = %v", err)
	}

	metrics := observer.RenderPrometheus()
	for _, want := range []string{
		`stele_scope_proof_steps_total{component="context",failure_category="context",status="completed",step="context_assembled",verdict="passed_degraded"} 1`,
		`stele_memory_session_runs_total{failure_category="retrieval",status="completed",verdict="passed_degraded"} 1`,
		`stele_memory_session_turns_total{failure_category="retrieval",status="verified",verification_status="passed_degraded"} 1`,
		`stele_memory_session_verifications_total{failure_category="retrieval",status="completed",verdict="passed_degraded"} 1`,
	} {
		if !strings.Contains(metrics, want) {
			t.Fatalf("metrics missing %q\n%s", want, metrics)
		}
	}
	for _, forbidden := range []string{"tenant", "project", "namespace", "proof_1", "session_1", "turn_1", "verification_1", "evt_1", "evt_2", "mem_1", "mem_2"} {
		if strings.Contains(metrics, forbidden) {
			t.Fatalf("metrics contain high-cardinality value %q\n%s", forbidden, metrics)
		}
	}
}

func TestScopeProofAndSessionWorkersLogLifecycleTransitions(t *testing.T) {
	now := time.Date(2026, 7, 11, 21, 20, 0, 0, time.UTC)
	scope := memory.Scope{Tenant: "tenant-a", Project: "project-a", Namespace: "namespace-a"}
	var logBuf bytes.Buffer
	logger := log.New(&logBuf, "", 0)
	proofWorker := ScopeProofStepWorker{
		Store: &stubScopeProofStepWorkerStore{
			claims: []memory.ScopeProofStep{{
				ID:       "step_context",
				ProofID:  "proof_1",
				Scope:    scope,
				Step:     memory.ScopeProofStepContextAssembled,
				Status:   memory.ScopeProofStepStatusRunning,
				WorkerID: "worker-a",
				Attempt:  1,
			}},
		},
		Executor: &stubScopeProofStepExecutor{result: memory.ScopeProofStepResult{
			Verdict:         memory.ScopeProofVerdictPassedDegraded,
			FailureCategory: memory.ProofFailureCategoryContext,
		}},
		Scope:         scope,
		WorkerID:      "worker-a",
		BatchSize:     1,
		LeaseDuration: time.Minute,
		Now:           func() time.Time { return now },
		Logger:        logger,
	}

	if _, err := proofWorker.RunOnce(context.Background()); err != nil {
		t.Fatalf("proof RunOnce() error = %v", err)
	}

	sessionWorker := MemorySessionVerificationWorker{
		Store: &stubSessionVerificationWorkerStore{
			claims: []memory.MemorySessionVerification{{
				ID:        "verification_1",
				SessionID: "session_1",
				TurnID:    "turn_1",
				Scope:     scope,
				Status:    memory.ScopeProofStepStatusRunning,
				WorkerID:  "worker-a",
				Attempt:   1,
			}},
		},
		Runner: &stubSessionVerificationRunner{result: memory.MemorySessionVerificationResult{
			Verdict:         memory.ScopeProofVerdictPassedDegraded,
			FailureCategory: memory.ProofFailureCategoryRetrieval,
		}},
		Scope:         scope,
		WorkerID:      "worker-a",
		BatchSize:     1,
		LeaseDuration: time.Minute,
		Now:           func() time.Time { return now },
		Logger:        logger,
	}

	if _, err := sessionWorker.RunOnce(context.Background()); err != nil {
		t.Fatalf("session RunOnce() error = %v", err)
	}

	logs := logBuf.String()
	for _, want := range []string{
		"mode=worker component=scope_proof_step_worker event=step_completed step=context_assembled status=completed verdict=passed_degraded failure_category=context",
		"mode=worker component=memory_session_verification_worker event=verification_completed status=completed verdict=passed_degraded failure_category=retrieval",
		"mode=worker component=memory_session_verification_worker event=session_updated status=completed verdict=passed_degraded failure_category=retrieval",
		"mode=worker component=memory_session_verification_worker event=turn_updated status=verified verification_status=passed_degraded failure_category=retrieval",
	} {
		if !strings.Contains(logs, want) {
			t.Fatalf("logs missing %q\n%s", want, logs)
		}
	}
	for _, forbidden := range []string{"proof_1", "session_1", "turn_1", "verification_1"} {
		if strings.Contains(logs, forbidden) {
			t.Fatalf("logs contain high-cardinality value %q\n%s", forbidden, logs)
		}
	}
}

type stubScopeProofStepWorkerStore struct {
	claims     []memory.ScopeProofStep
	completed  memory.CompleteScopeProofStepInput
	failed     []memory.RecordScopeProofStepFailureInput
	updatedRun memory.UpdateScopeProofRunStatusInput
}

func (s *stubScopeProofStepWorkerStore) ClaimScopeProofSteps(ctx context.Context, input memory.ClaimScopeProofStepsInput) ([]memory.ScopeProofStep, error) {
	return s.claims, nil
}

func (s *stubScopeProofStepWorkerStore) CompleteScopeProofStep(ctx context.Context, input memory.CompleteScopeProofStepInput) error {
	s.completed = input
	return nil
}

func (s *stubScopeProofStepWorkerStore) RecordScopeProofStepFailure(ctx context.Context, input memory.RecordScopeProofStepFailureInput) error {
	s.failed = append(s.failed, input)
	return nil
}

func (s *stubScopeProofStepWorkerStore) UpdateScopeProofRunStatus(ctx context.Context, input memory.UpdateScopeProofRunStatusInput) (memory.ScopeProofRun, error) {
	s.updatedRun = input
	return memory.ScopeProofRun{ID: input.ProofID, Scope: input.Scope, Status: input.Status, Verdict: input.Verdict}, nil
}

type stubScopeProofStepExecutor struct {
	gotStep memory.ScopeProofStep
	result  memory.ScopeProofStepResult
	err     error
	called  bool
}

func (s *stubScopeProofStepExecutor) ExecuteScopeProofStep(ctx context.Context, step memory.ScopeProofStep) (memory.ScopeProofStepResult, error) {
	s.called = true
	s.gotStep = step
	return s.result, s.err
}

type stubSessionVerificationWorkerStore struct {
	claims         []memory.MemorySessionVerification
	completed      memory.CompleteMemorySessionVerificationInput
	failed         memory.RecordMemorySessionVerificationFailureInput
	updatedSession memory.UpdateMemorySessionRunStatusInput
	updatedTurn    memory.UpdateMemorySessionTurnOutcomeInput
}

func (s *stubSessionVerificationWorkerStore) ClaimMemorySessionVerifications(ctx context.Context, input memory.ClaimMemorySessionVerificationsInput) ([]memory.MemorySessionVerification, error) {
	return s.claims, nil
}

func (s *stubSessionVerificationWorkerStore) CompleteMemorySessionVerification(ctx context.Context, input memory.CompleteMemorySessionVerificationInput) error {
	s.completed = input
	return nil
}

func (s *stubSessionVerificationWorkerStore) RecordMemorySessionVerificationFailure(ctx context.Context, input memory.RecordMemorySessionVerificationFailureInput) error {
	s.failed = input
	return nil
}

func (s *stubSessionVerificationWorkerStore) UpdateMemorySessionRunStatus(ctx context.Context, input memory.UpdateMemorySessionRunStatusInput) (memory.MemorySessionRun, error) {
	s.updatedSession = input
	return memory.MemorySessionRun{ID: input.SessionID, Scope: input.Scope, Status: input.Status, Verdict: input.Verdict}, nil
}

func (s *stubSessionVerificationWorkerStore) UpdateMemorySessionTurnOutcome(ctx context.Context, input memory.UpdateMemorySessionTurnOutcomeInput) (memory.MemorySessionTurn, error) {
	s.updatedTurn = input
	return memory.MemorySessionTurn{ID: input.TurnID, SessionID: input.SessionID, Scope: input.Scope, Status: input.Status, VerificationStatus: input.VerificationStatus}, nil
}

type stubSessionVerificationRunner struct {
	gotVerification memory.MemorySessionVerification
	result          memory.MemorySessionVerificationResult
	err             error
}

func (s *stubSessionVerificationRunner) VerifyMemorySession(ctx context.Context, verification memory.MemorySessionVerification) (memory.MemorySessionVerificationResult, error) {
	s.gotVerification = verification
	return s.result, s.err
}
