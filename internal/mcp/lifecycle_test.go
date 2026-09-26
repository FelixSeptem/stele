package mcp

import (
	"context"
	"testing"
	"time"

	"github.com/FelixSeptem/stele/internal/auth"
	"github.com/FelixSeptem/stele/internal/memory"
	"github.com/FelixSeptem/stele/internal/policy"
	"github.com/FelixSeptem/stele/internal/provider"
)

type lifecycleAdapterStub struct {
	calls       int
	last        provider.OperationMetadata
	principal   auth.Principal
	principalOK bool
	applyErr    error
}

func (s *lifecycleAdapterStub) ApplyLifecycle(ctx context.Context, binding provider.RuntimeBinding, metadata provider.OperationMetadata, memoryID string, action policy.ForgettingAction, reason, actor string) (provider.OperationOutcome, error) {
	s.calls++
	s.last = metadata
	s.principal, s.principalOK = auth.PrincipalFromContext(ctx)
	if s.applyErr != nil {
		return provider.OperationOutcome{}, s.applyErr
	}
	return provider.OperationOutcome{Metadata: metadata, Citations: []provider.Citation{{Reference: memoryID}}}, nil
}

func TestApplyForgetLifecycleUsesBoundedDistinctDurableProviderOperations(t *testing.T) {
	now := time.Now().UTC()
	scope := memory.Scope{Tenant: "tenant", Project: "project", Namespace: "namespace"}
	stub := &lifecycleAdapterStub{}
	adapter := NewAdapter(AdapterOptions{LifecycleAdapter: stub, Now: func() time.Time { return now }})
	binding := provider.RuntimeBinding{BindingID: "binding", PrincipalID: "principal", Scope: scope, AgentID: "agent", SessionID: "session", ProviderInstanceID: "instance", CreatedAt: now.Add(-time.Minute), ExpiresAt: now.Add(time.Hour)}
	response, err := adapter.applyForgetLifecycle(context.Background(), auth.Principal{ID: "principal", Role: auth.PrincipalRoleAdmin, Status: auth.PrincipalStatusActive}, binding, ForgetApplyRequest{PreviewID: "fp_1", MemoryIDs: []string{"memory-1", "memory-2"}, Action: "suppress", Reason: "requested", IdempotencyKey: "same-key"})
	if err != nil {
		t.Fatalf("applyForgetLifecycle() error = %v", err)
	}
	if stub.calls != 2 || len(response.AppliedIDs) != 2 {
		t.Fatalf("lifecycle calls=%d response=%+v, want one durable call per reviewed ID", stub.calls, response)
	}
	if stub.last.IdempotencyKey == "same-key" || stub.last.RequestID == "" || stub.last.OperationID == "" {
		t.Fatalf("metadata=%+v, want bounded per-item derived metadata", stub.last)
	}
	if len(stub.last.IdempotencyKey) > 256 {
		t.Fatalf("idempotency key has %d bytes, want at most 256", len(stub.last.IdempotencyKey))
	}
	if !stub.principalOK || stub.principal.ID != "principal" || stub.principal.Role != auth.PrincipalRoleAdmin {
		t.Fatalf("provider lifecycle context principal = %+v, %v; want authenticated admin", stub.principal, stub.principalOK)
	}
}

func TestApplyForgetLifecycleRejectsNonAdminBeforeMutation(t *testing.T) {
	stub := &lifecycleAdapterStub{}
	adapter := NewAdapter(AdapterOptions{LifecycleAdapter: stub})
	binding := provider.RuntimeBinding{BindingID: "binding", PrincipalID: "principal", Scope: memory.Scope{Tenant: "t", Project: "p", Namespace: "n"}, SessionID: "session"}
	_, err := adapter.applyForgetLifecycle(context.Background(), auth.Principal{ID: "principal", Role: auth.PrincipalRolePublic, Status: auth.PrincipalStatusActive}, binding, ForgetApplyRequest{PreviewID: "fp_1", MemoryIDs: []string{"memory-1"}, Action: "suppress", Reason: "requested", IdempotencyKey: "key"})
	if err == nil {
		t.Fatal("non-admin lifecycle apply error = nil, want authorization denial")
	}
	if stub.calls != 0 {
		t.Fatalf("provider lifecycle calls = %d, want no mutation", stub.calls)
	}
}
