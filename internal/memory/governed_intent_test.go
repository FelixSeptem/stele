package memory

import (
	"context"
	"strings"
	"testing"
	"time"
)

func TestMemoryIntentServiceSubmitsScopedIntentWithoutCanonicalMutation(t *testing.T) {
	processor := &intentProcessorStub{}
	service := MemoryIntentService{Processor: processor, Now: func() time.Time { return time.Unix(10, 0) }}

	got, err := service.Submit(context.Background(), MemoryIntentInput{
		Scope:          Scope{Tenant: "t1", Project: "p1", Namespace: "n1"},
		Type:           MemoryIntentRemember,
		Content:        "user prefers concise answers",
		Actor:          "agent",
		Reason:         "explicit preference",
		Provenance:     map[string]any{"source": "turn-1"},
		RequestID:      "req-1",
		OperationID:    "op-1",
		IdempotencyKey: "idem-1",
	})
	if err != nil {
		t.Fatalf("Submit() error = %v", err)
	}
	if got.Status != MemoryIntentStatusAccepted || got.ID == "" {
		t.Fatalf("intent = %+v, want accepted durable intent", got)
	}
	if processor.received.Type != MemoryIntentRemember || processor.received.Scope.Tenant != "t1" {
		t.Fatalf("received intent = %+v", processor.received)
	}
	if processor.canonicalMutations != 0 {
		t.Fatalf("canonical mutations = %d, want 0", processor.canonicalMutations)
	}
	if !processor.enqueued {
		t.Fatal("accepted intent was not handed to governance queue")
	}
}

func TestMemoryIntentInputRejectsMissingGovernanceFields(t *testing.T) {
	input := MemoryIntentInput{Type: MemoryIntentRemember, Content: "x"}
	if err := input.Validate(); err == nil {
		t.Fatal("Validate() error = nil, want required governance field error")
	}
}

func governedInput() MemoryIntentInput {
	return MemoryIntentInput{Scope: Scope{Tenant: "t1", Project: "p1", Namespace: "n1"}, Type: MemoryIntentRemember,
		Content: "value", Actor: "agent", Reason: "reason", RequestID: "req", OperationID: "op", IdempotencyKey: "idem"}
}

func TestMemoryIntentFingerprintNormalizesEquivalentRetries(t *testing.T) {
	first := governedInput()
	second := governedInput()
	second.Actor = " agent "
	second.MemoryPath = "/"
	a, err := first.CanonicalFingerprint()
	if err != nil {
		t.Fatal(err)
	}
	b, err := second.CanonicalFingerprint()
	if err != nil {
		t.Fatal(err)
	}
	if a != b {
		t.Fatalf("equivalent fingerprints differ: %s != %s", a, b)
	}
	second.Content = "different"
	c, err := second.CanonicalFingerprint()
	if err != nil {
		t.Fatal(err)
	}
	if a == c {
		t.Fatal("material content change kept fingerprint")
	}
}

func TestMemoryIntentValidationRejectsForeignEvidenceAndIncompleteSpecialTypes(t *testing.T) {
	input := governedInput()
	input.Type = MemoryIntentContradiction
	input.TargetMemoryID = "memory-1"
	input.TargetVersion = 1
	input.Evidence = []MemoryIntentEvidence{{Scope: input.Scope, Kind: "memory", ID: "a", Version: 1}}
	if err := input.ValidateGoverned(); err == nil || !strings.Contains(err.Error(), "two evidence") {
		t.Fatalf("incomplete contradiction error = %v", err)
	}
	input.Evidence = []MemoryIntentEvidence{{Scope: input.Scope, Kind: "memory", ID: "a", Version: 1}, {Scope: Scope{Tenant: "foreign", Project: "p1", Namespace: "n1"}, Kind: "memory", ID: "b", Version: 1}}
	if err := input.ValidateGoverned(); err == nil || !strings.Contains(err.Error(), "does not match") {
		t.Fatalf("foreign evidence error = %v", err)
	}
	input = governedInput()
	input.Type = MemoryIntentFeedback
	if err := input.ValidateGoverned(); err == nil || !strings.Contains(err.Error(), "target insight") {
		t.Fatalf("feedback target error = %v", err)
	}
}

func TestMemoryIntentServicePolicyDisablementPreventsPersistence(t *testing.T) {
	processor := &intentProcessorStub{}
	service := MemoryIntentService{Processor: processor, Policy: disabledIntentPolicy{}}
	_, err := service.Submit(context.Background(), governedInput())
	if err == nil || !strings.Contains(err.Error(), "processing disabled") {
		t.Fatalf("Submit() error = %v, want policy disablement", err)
	}
	if processor.received.ID != "" {
		t.Fatalf("disabled policy persisted intent: %+v", processor.received)
	}
}

func TestMemoryIntentServiceReplayRepairsTransitionAndQueueHandoff(t *testing.T) {
	processor := &replayIntentProcessorStub{}
	service := MemoryIntentService{Processor: processor, Now: func() time.Time { return time.Unix(10, 0) }}
	first, err := service.Submit(context.Background(), governedInput())
	if err != nil || first.Status != MemoryIntentStatusAccepted {
		t.Fatalf("first Submit() = %+v, %v", first, err)
	}
	second, err := service.Submit(context.Background(), governedInput())
	if err != nil || second.Status != MemoryIntentStatusReplayed {
		t.Fatalf("replay Submit() = %+v, %v", second, err)
	}
	if processor.enqueueCalls != 2 || processor.transitionCalls != 2 {
		t.Fatalf("replay repair calls enqueue=%d transition=%d, want 2 each", processor.enqueueCalls, processor.transitionCalls)
	}
}

type disabledIntentPolicy struct{}

func (disabledIntentPolicy) EvaluateMemoryIntent(context.Context, MemoryIntentInput) (MemoryIntentPolicyDecision, error) {
	return MemoryIntentPolicyDecision{Enabled: false, PolicyVersion: "rollback-v1", Category: MemoryIntentDiagnosticPolicyDisabled}, nil
}

type intentProcessorStub struct {
	received           MemoryIntentRecord
	canonicalMutations int
	enqueued           bool
}

type replayIntentProcessorStub struct {
	stored          MemoryIntentRecord
	enqueueCalls    int
	transitionCalls int
}

func (s *replayIntentProcessorStub) AppendMemoryIntent(_ context.Context, record MemoryIntentRecord) (MemoryIntentRecord, error) {
	if s.stored.ID == "" {
		record.ID = "intent-replay"
		record.Status = MemoryIntentStatusAccepted
		s.stored = record
		return record, nil
	}
	s.stored.Status = MemoryIntentStatusReplayed
	return s.stored, nil
}

func (s *replayIntentProcessorStub) AppendMemoryIntentTransition(_ context.Context, _ MemoryIntentTransition) error {
	s.transitionCalls++
	return nil
}

func (s *replayIntentProcessorStub) EnqueueMemoryIntent(_ context.Context, _ MemoryIntentRecord) error {
	s.enqueueCalls++
	return nil
}

func (s *intentProcessorStub) EnqueueMemoryIntent(_ context.Context, _ MemoryIntentRecord) error {
	s.enqueued = true
	return nil
}

func (s *intentProcessorStub) AppendMemoryIntent(_ context.Context, record MemoryIntentRecord) (MemoryIntentRecord, error) {
	s.received = record
	record.ID = "intent-1"
	record.Status = MemoryIntentStatusAccepted
	return record, nil
}
