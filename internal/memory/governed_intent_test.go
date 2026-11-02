package memory

import (
	"context"
	"encoding/json"
	"reflect"
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

func TestCorrectionAttributionIsServerOwnedAndChangesCorrectionFingerprint(t *testing.T) {
	input := governedInput()
	input.Type = MemoryIntentUpdate
	input.TargetMemoryID = "09f8bdb0-8aeb-4efd-a82a-8cf1b093997a"
	input.TargetVersion = 2

	if _, exposed := reflect.TypeOf(MemoryIntentInput{}).FieldByName("Correction"); exposed {
		t.Fatal("generic MemoryIntentInput must not expose caller-controlled correction attribution")
	}

	first := CorrectionAttribution{
		ProjectionID:   "2acbd3aa-f0b5-4217-bdf3-d7cfc0304972",
		SourceRevision: 2,
		BindingID:      "binding-1",
		PrincipalID:    "principal-1",
	}
	second := first
	second.SourceRevision = 3

	a, err := CorrectionIntentFingerprint(input, first, "policy-v1")
	if err != nil {
		t.Fatalf("CorrectionIntentFingerprint() first error = %v", err)
	}
	b, err := CorrectionIntentFingerprint(input, second, "policy-v1")
	if err != nil {
		t.Fatalf("CorrectionIntentFingerprint() second error = %v", err)
	}
	if a == b {
		t.Fatal("correction source revision must change correction fingerprint")
	}
	policyFingerprint, err := CorrectionIntentFingerprint(input, first, "policy-v2")
	if err != nil || policyFingerprint == a {
		t.Fatalf("correction policy version must change fingerprint: fingerprint=%q error=%v", policyFingerprint, err)
	}
	if _, err := CorrectionIntentFingerprint(input, first, " "); err == nil {
		t.Fatal("empty correction policy version must be rejected")
	}
	if err := (CorrectionAttribution{ProjectionID: first.ProjectionID, SourceRevision: 1, BindingID: first.BindingID, PrincipalID: first.PrincipalID}).Validate(); err == nil {
		t.Fatal("first projection revision must not be a correction attribution")
	}
}

func TestCorrectionAttributionJSONIsBoundedAndOmitsInternalBindingIdentity(t *testing.T) {
	encoded, err := json.Marshal(CorrectionAttribution{ProjectionID: "11111111-1111-4111-8111-111111111111", SourceRevision: 2, BindingID: "private-binding", PrincipalID: "private-principal", State: "verified"})
	if err != nil {
		t.Fatal(err)
	}
	var got map[string]any
	if err := json.Unmarshal(encoded, &got); err != nil {
		t.Fatal(err)
	}
	if len(got) != 3 || got["projection_id"] != "11111111-1111-4111-8111-111111111111" || got["source_revision"] != float64(2) || got["state"] != "verified" {
		t.Fatalf("bounded correction attribution = %s", encoded)
	}
	if strings.Contains(string(encoded), "private-binding") || strings.Contains(string(encoded), "private-principal") {
		t.Fatalf("correction attribution exposed internal authority: %s", encoded)
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

func TestMemoryIntentServiceUsesAtomicAcceptorWithoutLegacyHandoff(t *testing.T) {
	processor := &atomicIntentAcceptorStub{}
	service := MemoryIntentService{Processor: processor, NewID: func() string { return "intent-atomic" }}
	created, err := service.Submit(context.Background(), governedInput())
	if err != nil || created.ID != "intent-atomic" || processor.atomicCalls != 1 {
		t.Fatalf("Submit() = %+v, %v; atomic calls = %d", created, err, processor.atomicCalls)
	}
	if processor.appendCalls != 0 || processor.transitionCalls != 0 || processor.enqueueCalls != 0 {
		t.Fatalf("legacy writes after atomic acceptance: append=%d transition=%d enqueue=%d", processor.appendCalls, processor.transitionCalls, processor.enqueueCalls)
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

type atomicIntentAcceptorStub struct {
	atomicCalls     int
	appendCalls     int
	transitionCalls int
	enqueueCalls    int
}

func (s *atomicIntentAcceptorStub) AcceptMemoryIntent(_ context.Context, record MemoryIntentRecord) (MemoryIntentRecord, error) {
	s.atomicCalls++
	record.ID = "intent-atomic"
	record.Status = MemoryIntentStatusAccepted
	return record, nil
}

func (s *atomicIntentAcceptorStub) AppendMemoryIntent(context.Context, MemoryIntentRecord) (MemoryIntentRecord, error) {
	s.appendCalls++
	return MemoryIntentRecord{}, nil
}

func (s *atomicIntentAcceptorStub) AppendMemoryIntentTransition(context.Context, MemoryIntentTransition) error {
	s.transitionCalls++
	return nil
}

func (s *atomicIntentAcceptorStub) EnqueueMemoryIntent(context.Context, MemoryIntentRecord) error {
	s.enqueueCalls++
	return nil
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
