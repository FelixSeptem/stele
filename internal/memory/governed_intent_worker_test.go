package memory

import (
	"context"
	"testing"
)

func TestMemoryIntentWorkerRejectsStaleTargetWithoutRouting(t *testing.T) {
	router := &intentRouterStub{}
	worker := MemoryIntentWorker{Router: router, Validator: intentTargetRejectingStub{}}
	out, err := worker.Process(context.Background(), workerRecord(MemoryIntentUpdate))
	if err != nil || out.Status != MemoryIntentStatusRejected || router.calls != 0 || out.DiagnosticCategory != MemoryIntentDiagnosticTargetStale {
		t.Fatalf("outcome=%+v err=%v calls=%d", out, err, router.calls)
	}
}

func TestMemoryIntentWorkerKeepsContradictionAndFeedbackOutOfActiveLifecycle(t *testing.T) {
	for _, typ := range []MemoryIntentType{MemoryIntentContradiction, MemoryIntentFeedback} {
		router := &intentRouterStub{active: true}
		worker := MemoryIntentWorker{Router: router}
		record := workerRecord(typ)
		if typ == MemoryIntentContradiction {
			record.Evidence = []MemoryIntentEvidence{{Scope: record.Scope, Kind: "memory", ID: "a", Version: 1}, {Scope: record.Scope, Kind: "memory", ID: "b", Version: 1}}
		}
		if typ == MemoryIntentFeedback {
			record.TargetInsightID = "insight-1"
			record.Content = "review"
		}
		out, err := worker.Process(context.Background(), record)
		if err != nil {
			t.Fatal(err)
		}
		if out.Status == MemoryIntentStatusActive {
			t.Fatalf("%s activated directly: %+v", typ, out)
		}
	}
}

func TestMemoryIntentWorkerRecordsOutcomeWithAttribution(t *testing.T) {
	router := &intentRouterStub{}
	recorder := &transitionRecorderStub{}
	worker := MemoryIntentWorker{Router: router}
	out, err := worker.ProcessAndRecord(context.Background(), workerRecord(MemoryIntentRemember), recorder)
	if err != nil || out.Status != MemoryIntentStatusCandidate || recorder.transition.Sequence != 2 || recorder.transition.IntentID != "intent-1" {
		t.Fatalf("outcome=%+v err=%v transition=%+v", out, err, recorder.transition)
	}
}

func TestMemoryIntentResumePolicyOnlyAllowsExactScopePendingWork(t *testing.T) {
	scope := Scope{Tenant: "t", Project: "p", Namespace: "n"}
	policy := MemoryIntentResumePolicy{Scope: scope, PolicyVersion: "v2", Enabled: true}
	accepted := workerRecord(MemoryIntentRemember)
	accepted.Status = MemoryIntentStatusAccepted
	if !policy.Allows(accepted) {
		t.Fatal("accepted exact-scope intent should resume")
	}
	failed := workerRecord(MemoryIntentRemember)
	failed.Status = MemoryIntentStatusFailed
	if policy.Allows(failed) {
		t.Fatal("failed intent must not resume automatically")
	}
	foreign := workerRecord(MemoryIntentRemember)
	foreign.Status = MemoryIntentStatusPending
	foreign.Scope.Tenant = "foreign"
	if policy.Allows(foreign) {
		t.Fatal("foreign intent must not resume")
	}
	policy.Enabled = false
	pending := workerRecord(MemoryIntentRemember)
	pending.Status = MemoryIntentStatusPending
	if policy.Allows(pending) {
		t.Fatal("disabled policy must not resume")
	}
}

func workerRecord(typ MemoryIntentType) MemoryIntentRecord {
	return MemoryIntentRecord{ID: "intent-1", Scope: Scope{Tenant: "t", Project: "p", Namespace: "n"}, Type: typ, TargetMemoryID: "memory-1", TargetVersion: 1, Actor: "worker", Reason: "test", RequestID: "req", OperationID: "op", IdempotencyKey: "idem", Status: MemoryIntentStatusAccepted}
}

type intentTargetRejectingStub struct{}

func (intentTargetRejectingStub) ValidateMemoryIntentTarget(context.Context, MemoryIntentInput) error {
	return context.Canceled
}

type intentRouterStub struct {
	calls  int
	active bool
}

type transitionRecorderStub struct{ transition MemoryIntentTransition }

func (s *transitionRecorderStub) AppendMemoryIntentTransition(_ context.Context, transition MemoryIntentTransition) error {
	s.transition = transition
	return nil
}

func (s *intentRouterStub) ProcessRememberOrUpdate(context.Context, MemoryIntentRecord) (MemoryIntentOutcome, error) {
	s.calls++
	return MemoryIntentOutcome{Status: MemoryIntentStatusCandidate, DiagnosticCategory: MemoryIntentDiagnosticAccepted}, nil
}
func (s *intentRouterStub) ProcessForget(context.Context, MemoryIntentRecord) (MemoryIntentOutcome, error) {
	s.calls++
	return MemoryIntentOutcome{Status: MemoryIntentStatusSuppressed, DiagnosticCategory: MemoryIntentDiagnosticSuppressed}, nil
}
func (s *intentRouterStub) ProcessContradiction(context.Context, MemoryIntentRecord) (MemoryIntentOutcome, error) {
	s.calls++
	if s.active {
		return MemoryIntentOutcome{Status: MemoryIntentStatusActive, DiagnosticCategory: MemoryIntentDiagnosticAccepted}, nil
	}
	return MemoryIntentOutcome{Status: MemoryIntentStatusCandidate, DiagnosticCategory: MemoryIntentDiagnosticPending}, nil
}
func (s *intentRouterStub) ProcessFeedback(context.Context, MemoryIntentRecord) (MemoryIntentOutcome, error) {
	s.calls++
	if s.active {
		return MemoryIntentOutcome{Status: MemoryIntentStatusActive, DiagnosticCategory: MemoryIntentDiagnosticAccepted}, nil
	}
	return MemoryIntentOutcome{Status: MemoryIntentStatusCandidate, DiagnosticCategory: MemoryIntentDiagnosticPending}, nil
}
