package jobs

import (
	"context"
	"testing"

	"github.com/FelixSeptem/stele/internal/memory"
	"github.com/FelixSeptem/stele/internal/workqueue"
)

type intentRecordReaderStub struct{ record memory.MemoryIntentRecord }

func (s intentRecordReaderStub) ReadMemoryIntent(context.Context, memory.Scope, string) (memory.MemoryIntentRecord, error) {
	return s.record, nil
}

type intentHistoryReaderStub struct{ history memory.MemoryIntentHistory }

func (s intentHistoryReaderStub) ReadMemoryIntentHistory(context.Context, memory.Scope, string) (memory.MemoryIntentHistory, error) {
	return s.history, nil
}

type intentOutcomeRecorderStub struct{ calls int }

func (s *intentOutcomeRecorderStub) AppendMemoryIntentTransition(context.Context, memory.MemoryIntentTransition) error {
	s.calls++
	return nil
}

func TestMemoryIntentWorkExecutorProcessesAndReplaysFromHistory(t *testing.T) {
	scope := memory.Scope{Tenant: "t", Project: "p", Namespace: "n"}
	record := memory.MemoryIntentRecord{ID: "intent-1", Scope: scope, Type: memory.MemoryIntentRemember, Content: "hello", Actor: "worker", Reason: "test", RequestID: "req", OperationID: "op", IdempotencyKey: "idem", Status: memory.MemoryIntentStatusAccepted}
	recorder := &intentOutcomeRecorderStub{}
	router := &intentRouterForJobTest{}
	executor := MemoryIntentWorkExecutor{
		Reader:   intentRecordReaderStub{record: record},
		History:  intentHistoryReaderStub{},
		Worker:   memory.MemoryIntentWorker{Router: router},
		Recorder: recorder,
	}
	item := workqueue.DerivedWorkItem{DerivedWorkIdentity: workqueue.DerivedWorkIdentity{Scope: scope}, Kind: workqueue.WorkKindMemoryIntent, Reference: record.ID}
	if _, _, err := executor.ExecuteDerivedWork(context.Background(), item); err != nil {
		t.Fatal(err)
	}
	if router.calls != 1 || recorder.calls != 1 {
		t.Fatalf("first execution calls=%d transitions=%d", router.calls, recorder.calls)
	}
	executor.History = intentHistoryReaderStub{history: memory.MemoryIntentHistory{Intent: record, Transitions: []memory.MemoryIntentTransition{{Sequence: 1}, {Sequence: 2}}}}
	if _, _, err := executor.ExecuteDerivedWork(context.Background(), item); err != nil {
		t.Fatal(err)
	}
	if router.calls != 1 || recorder.calls != 1 {
		t.Fatalf("replay rerouted intent: calls=%d transitions=%d", router.calls, recorder.calls)
	}
}

func TestMemoryIntentWorkExecutorRetriesFailedOutcomeWithNextTransitionSequence(t *testing.T) {
	scope := memory.Scope{Tenant: "t", Project: "p", Namespace: "n"}
	record := memory.MemoryIntentRecord{ID: "intent-retry", Scope: scope, Type: memory.MemoryIntentRemember, Content: "hello", Actor: "worker", Reason: "test", RequestID: "req", OperationID: "op", IdempotencyKey: "idem", Status: memory.MemoryIntentStatusAccepted}
	history := &mutableIntentHistoryReader{history: memory.MemoryIntentHistory{Intent: record, Transitions: []memory.MemoryIntentTransition{{Sequence: 1, To: memory.MemoryIntentStatusAccepted}}}}
	recorder := &recordingIntentOutcomeRecorder{}
	executor := MemoryIntentWorkExecutor{Reader: intentRecordReaderStub{record: record}, History: history, Worker: memory.MemoryIntentWorker{Router: &retryIntentRouter{}}, Recorder: recorder}
	item := workqueue.DerivedWorkItem{DerivedWorkIdentity: workqueue.DerivedWorkIdentity{Scope: scope}, Kind: workqueue.WorkKindMemoryIntent, Reference: record.ID}
	if _, _, err := executor.ExecuteDerivedWork(context.Background(), item); err == nil {
		t.Fatal("first execution error = nil, want retryable failure")
	}
	history.history.Transitions = append(history.history.Transitions, memory.MemoryIntentTransition{Sequence: 2, To: memory.MemoryIntentStatusFailed})
	if _, _, err := executor.ExecuteDerivedWork(context.Background(), item); err != nil {
		t.Fatal(err)
	}
	if len(recorder.transitions) != 2 || recorder.transitions[1].Sequence != 3 || recorder.transitions[1].From != memory.MemoryIntentStatusFailed {
		t.Fatalf("retry transitions = %+v", recorder.transitions)
	}
}

func TestMemoryIntentWorkExecutorHoldsPendingWorkWhenPolicyDisabled(t *testing.T) {
	scope := memory.Scope{Tenant: "t", Project: "p", Namespace: "n"}
	record := memory.MemoryIntentRecord{ID: "intent-held", Scope: scope, Type: memory.MemoryIntentRemember, Content: "hello", Actor: "worker", Reason: "test", RequestID: "req", OperationID: "op", IdempotencyKey: "idem", PolicyVersion: "v1", Status: memory.MemoryIntentStatusPending}
	executor := MemoryIntentWorkExecutor{
		Reader:   intentRecordReaderStub{record: record},
		History:  intentHistoryReaderStub{history: memory.MemoryIntentHistory{Intent: record, Transitions: []memory.MemoryIntentTransition{{Sequence: 1, To: memory.MemoryIntentStatusAccepted}}}},
		Worker:   memory.MemoryIntentWorker{Router: &intentRouterForJobTest{}},
		Recorder: &intentOutcomeRecorderStub{},
		Policy:   memory.StaticMemoryIntentPolicy{Scope: scope, PolicyVersion: "v1", Enabled: false},
	}
	item := workqueue.DerivedWorkItem{DerivedWorkIdentity: workqueue.DerivedWorkIdentity{Scope: scope}, Kind: workqueue.WorkKindMemoryIntent, Reference: record.ID}
	if _, _, err := executor.ExecuteDerivedWork(context.Background(), item); err == nil {
		t.Fatal("disabled policy execution error = nil, want held/retryable error")
	}
}

type intentRouterForJobTest struct{ calls int }

type mutableIntentHistoryReader struct{ history memory.MemoryIntentHistory }

func (r *mutableIntentHistoryReader) ReadMemoryIntentHistory(context.Context, memory.Scope, string) (memory.MemoryIntentHistory, error) {
	return r.history, nil
}

type recordingIntentOutcomeRecorder struct {
	transitions []memory.MemoryIntentTransition
}

func (r *recordingIntentOutcomeRecorder) AppendMemoryIntentTransition(_ context.Context, transition memory.MemoryIntentTransition) error {
	r.transitions = append(r.transitions, transition)
	return nil
}

type retryIntentRouter struct{ calls int }

func (r *retryIntentRouter) ProcessRememberOrUpdate(context.Context, memory.MemoryIntentRecord) (memory.MemoryIntentOutcome, error) {
	r.calls++
	if r.calls == 1 {
		return memory.MemoryIntentOutcome{}, context.DeadlineExceeded
	}
	return memory.MemoryIntentOutcome{Status: memory.MemoryIntentStatusCandidate, DiagnosticCategory: memory.MemoryIntentDiagnosticAccepted, OutcomeReference: "intent-retry"}, nil
}
func (*retryIntentRouter) ProcessForget(context.Context, memory.MemoryIntentRecord) (memory.MemoryIntentOutcome, error) {
	return memory.MemoryIntentOutcome{Status: memory.MemoryIntentStatusSuppressed, DiagnosticCategory: memory.MemoryIntentDiagnosticSuppressed}, nil
}
func (*retryIntentRouter) ProcessContradiction(context.Context, memory.MemoryIntentRecord) (memory.MemoryIntentOutcome, error) {
	return memory.MemoryIntentOutcome{Status: memory.MemoryIntentStatusCandidate, DiagnosticCategory: memory.MemoryIntentDiagnosticPending}, nil
}
func (*retryIntentRouter) ProcessFeedback(context.Context, memory.MemoryIntentRecord) (memory.MemoryIntentOutcome, error) {
	return memory.MemoryIntentOutcome{Status: memory.MemoryIntentStatusCandidate, DiagnosticCategory: memory.MemoryIntentDiagnosticPending}, nil
}

func (s *intentRouterForJobTest) ProcessRememberOrUpdate(context.Context, memory.MemoryIntentRecord) (memory.MemoryIntentOutcome, error) {
	s.calls++
	return memory.MemoryIntentOutcome{Status: memory.MemoryIntentStatusCandidate, DiagnosticCategory: memory.MemoryIntentDiagnosticAccepted, OutcomeReference: "intent-1"}, nil
}
func (*intentRouterForJobTest) ProcessForget(context.Context, memory.MemoryIntentRecord) (memory.MemoryIntentOutcome, error) {
	return memory.MemoryIntentOutcome{Status: memory.MemoryIntentStatusSuppressed, DiagnosticCategory: memory.MemoryIntentDiagnosticSuppressed}, nil
}
func (*intentRouterForJobTest) ProcessContradiction(context.Context, memory.MemoryIntentRecord) (memory.MemoryIntentOutcome, error) {
	return memory.MemoryIntentOutcome{Status: memory.MemoryIntentStatusCandidate, DiagnosticCategory: memory.MemoryIntentDiagnosticPending}, nil
}
func (*intentRouterForJobTest) ProcessFeedback(context.Context, memory.MemoryIntentRecord) (memory.MemoryIntentOutcome, error) {
	return memory.MemoryIntentOutcome{Status: memory.MemoryIntentStatusCandidate, DiagnosticCategory: memory.MemoryIntentDiagnosticPending}, nil
}
