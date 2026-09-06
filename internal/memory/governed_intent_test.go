package memory

import (
	"context"
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

type intentProcessorStub struct {
	received           MemoryIntentRecord
	canonicalMutations int
	enqueued           bool
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
