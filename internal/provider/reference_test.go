package provider

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/FelixSeptem/stele/internal/memory"
)

func TestReferenceBoundaryRejectsNoncanonicalAndUnboundedWrites(t *testing.T) {
	body := `{"metadata":{"request_id":"r","operation_id":"o","idempotency_key":"k","schema_version":"provider-v1"},"input":{"contract_version":"durable-reference-v1","reference_id":"ref","memory_path":"/tasks/a/","event_type":"provider.reference.task","value":"artifact"}}`
	req, err := DecodeReferenceWriteRequest([]byte(body))
	if err != nil || req.Input.MemoryPath != "tasks/a" {
		t.Fatalf("normalized request: %+v %v", req, err)
	}
	for name, data := range map[string]string{
		"alias":       strings.Replace(body, `"value"`, `"Value"`, 1),
		"duplicate":   strings.Replace(body, `"value":"artifact"`, `"value":"artifact","value":"x"`, 1),
		"null":        strings.Replace(body, `"value":"artifact"`, `"value":null`, 1),
		"missing":     strings.Replace(body, `,"value":"artifact"`, "", 1),
		"unknown":     strings.Replace(body, `"value":"artifact"`, `"value":"artifact","extra":1`, 1),
		"unsupported": strings.Replace(body, "provider.reference.task", "task.completed", 1),
		"large":       strings.Replace(body, "artifact", strings.Repeat("a", 4097), 1),
		"missing key": strings.Replace(body, `"idempotency_key":"k",`, "", 1),
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := DecodeReferenceWriteRequest([]byte(data)); err == nil {
				t.Fatal("accepted invalid request")
			}
		})
	}
}

type referenceIngestFixture struct {
	input          memory.IngestEventInput
	principal, key string
	calls          int
}

func (f *referenceIngestFixture) IngestIdempotent(_ context.Context, i memory.IngestEventInput, p, k string) (memory.IdempotentEventIngestResult, error) {
	f.input, f.principal, f.key = i, p, k
	f.calls++
	return memory.IdempotentEventIngestResult{Event: memory.RawEvent{ID: "0192a1a2-01f1-4000-8000-000000000001"}}, nil
}

func TestReferenceWriteRejectsUnboundedBindingBeforeAdmission(t *testing.T) {
	for _, tc := range []struct {
		name   string
		mutate func(*RuntimeBinding)
	}{
		{"long tenant", func(b *RuntimeBinding) { b.Scope.Tenant = strings.Repeat("a", 129) }},
		{"escaped namespace", func(b *RuntimeBinding) { b.Scope.Namespace = "scope<name" }},
		{"long agent", func(b *RuntimeBinding) { b.AgentID = strings.Repeat("a", 129) }},
		{"invalid conversation", func(b *RuntimeBinding) { b.ConversationID = "invalid conversation" }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			fixture := &referenceIngestFixture{}
			adapter := NewAdapter(AdapterDependencies{IdempotentIngestor: fixture})
			binding := RuntimeBinding{BindingID: "binding", PrincipalID: "principal", SessionID: "session", Scope: memory.Scope{Tenant: "tenant", Project: "project", Namespace: "namespace"}}
			tc.mutate(&binding)
			request := ReferenceWriteRequest{Metadata: OperationMetadata{RequestID: "request", OperationID: "operation", IdempotencyKey: "key", SchemaVersion: "provider-v1"}, Input: ReferenceWriteInput{ContractVersion: DurableReferenceContractVersion, ReferenceID: "reference", MemoryPath: "tasks/run", EventType: "provider.reference.run", Value: "evidence"}}
			if _, err := adapter.WriteReference(context.Background(), binding, request); err == nil || fixture.calls != 0 {
				t.Fatalf("invalid bound scope reached durable admission: calls=%d err=%v", fixture.calls, err)
			}
		})
	}
}
func TestReferenceWriteUsesBoundScopeAndDurableAttribution(t *testing.T) {
	f := &referenceIngestFixture{}
	a := NewAdapter(AdapterDependencies{IdempotentIngestor: f})
	b := RuntimeBinding{BindingID: "binding", PrincipalID: "principal", SessionID: "session", AgentID: "agent", ProviderInstanceID: "instance", Scope: memory.Scope{Tenant: "t", Project: "p", Namespace: "n"}}
	r := ReferenceWriteRequest{Metadata: OperationMetadata{RequestID: "r", OperationID: "o", IdempotencyKey: "key", SchemaVersion: "provider-v1"}, Input: ReferenceWriteInput{ContractVersion: memory.DurableReferenceVersion, ReferenceID: "ref", MemoryPath: "/tasks/a/", EventType: "provider.reference.task", Value: "artifact"}}
	out, err := a.WriteReference(context.Background(), b, r)
	if err != nil {
		t.Fatal(err)
	}
	if out.Status != "accepted" || out.EventID == "" || out.Reference != nil || f.input.Scope != b.Scope || f.input.MemoryPath != "tasks/a" || f.input.Metadata["reference_binding_id"] != b.BindingID || f.input.Metadata["reference_operation_id"] != "o" || f.principal != b.PrincipalID {
		t.Fatalf("unsafe write: %+v %+v", out, f)
	}
	if f.input.SourceMetadata != (memory.SourceMetadata{}).Effective() {
		t.Fatalf("omitted labels must use shared restricted default: %+v", f.input.SourceMetadata)
	}
	first, _ := json.Marshal(f.input)
	r.Input.MemoryPath = "tasks/a"
	if _, err = a.WriteReference(context.Background(), b, r); err != nil {
		t.Fatal(err)
	}
	second, _ := json.Marshal(f.input)
	if string(first) != string(second) {
		t.Fatal("equivalent normalized paths changed fingerprint")
	}
}

func TestReferenceWriteRejectsJSONExpansionBeforeAdmission(t *testing.T) {
	input := ReferenceWriteInput{ContractVersion: DurableReferenceContractVersion, ReferenceID: "ref", MemoryPath: "tasks/a", EventType: "provider.reference.task"}
	for _, value := range []string{strings.Repeat("<", 4096), "a" + strings.Repeat("\x00", 4094) + "z"} {
		input.Value = value
		if err := input.Validate(); err == nil {
			t.Fatal("accepted value whose escaped completed response exceeds the contract")
		}
	}
	input.Value = strings.Repeat("a", 4096)
	if err := input.Validate(); err != nil {
		t.Fatalf("maximum ordinary UTF-8 value rejected: %v", err)
	}
}

func TestReferenceBoundaryBoundsEchoedMetadata(t *testing.T) {
	request := ReferenceWriteRequest{Metadata: OperationMetadata{RequestID: "r", OperationID: "o", IdempotencyKey: "k", SchemaVersion: "provider-v1", PrecedenceOutcome: strings.Repeat("a", MaxOperationMetadataBytes)}, Input: ReferenceWriteInput{ContractVersion: DurableReferenceContractVersion, ReferenceID: "ref", MemoryPath: "tasks/a", EventType: "provider.reference.task", Value: "value"}}
	data, err := json.Marshal(request)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := DecodeReferenceWriteRequest(data); err == nil {
		t.Fatal("accepted metadata too large for the bounded response envelope")
	}
}
