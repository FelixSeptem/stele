package mcp

import (
	"context"
	"errors"
	"reflect"
	"testing"
	"time"

	"github.com/FelixSeptem/stele/internal/auth"
	"github.com/FelixSeptem/stele/internal/memory"
	"github.com/FelixSeptem/stele/internal/policy"
	"github.com/FelixSeptem/stele/internal/provider"
)

func TestForgetApplyReleasesClaimAfterLifecycleFailure(t *testing.T) {
	adapter, state, lifecycle, store := newForgetApplyTestAdapter(t, errors.New("lifecycle unavailable"), nil)
	_, err := adapter.applyForgetRequest(context.Background(), state, validForgetApplyRequest())
	if err == nil {
		t.Fatal("applyForgetRequest() error = nil, want lifecycle failure")
	}
	if lifecycle.calls != 1 || store.releases != 1 {
		t.Fatalf("lifecycle calls/releases = %d/%d, want 1/1 so a safe retry is not blocked", lifecycle.calls, store.releases)
	}
}

func TestForgetApplyReleasesClaimAfterCompletionFailure(t *testing.T) {
	adapter, state, lifecycle, store := newForgetApplyTestAdapter(t, nil, errors.New("database interrupted"))
	_, err := adapter.applyForgetRequest(context.Background(), state, validForgetApplyRequest())
	if err == nil {
		t.Fatal("applyForgetRequest() error = nil, want completion failure")
	}
	if lifecycle.calls != 1 || store.releases != 1 {
		t.Fatalf("lifecycle calls/releases = %d/%d, want 1/1; lifecycle operation is independently idempotent", lifecycle.calls, store.releases)
	}
}

func TestForgetApplyReplaysDurableOutcomeWithoutReloadingPreview(t *testing.T) {
	adapter, state, lifecycle, store := newForgetApplyTestAdapter(t, nil, nil)
	store.disposition = ForgetApplyReplayed
	store.response = ForgetApplyResponse{PreviewID: "fp_1", AppliedIDs: []string{"memory-1"}}
	response, err := adapter.applyForgetRequest(context.Background(), state, validForgetApplyRequest())
	if err != nil || response.PreviewID != "fp_1" || lifecycle.calls != 0 || store.completes != 0 {
		t.Fatalf("replayed apply = %+v, %v; lifecycle=%d completes=%d, want durable replay without side effects", response, err, lifecycle.calls, store.completes)
	}
}

func TestForgetApplyConflictingIdempotencyReuseIsBounded(t *testing.T) {
	adapter, state, lifecycle, store := newForgetApplyTestAdapter(t, nil, nil)
	store.disposition = ForgetApplyConflict
	_, err := adapter.applyForgetRequest(context.Background(), state, validForgetApplyRequest())
	var mcpErr MCPError
	if !errors.As(err, &mcpErr) || mcpErr.Code != "idempotency_conflict" || lifecycle.calls != 0 {
		t.Fatalf("conflicting apply error = %v, lifecycle calls = %d, want bounded idempotency conflict without mutation", err, lifecycle.calls)
	}
}

func TestForgetApplyReplaysWhenReviewedIDsAreReordered(t *testing.T) {
	adapter, state, lifecycle, store := newForgetApplyTestAdapter(t, nil, nil)
	store.strictReplay = true
	preview := adapter.previewStore.(*forgetPreviewStoreStub)
	preview.record.MemoryIDs = []string{"memory-1", "memory-2"}
	request := validForgetApplyRequest()
	request.MemoryIDs = []string{"memory-1", "memory-2"}
	first, err := adapter.applyForgetRequest(context.Background(), state, request)
	if err != nil {
		t.Fatalf("first applyForgetRequest() error = %v", err)
	}
	request.MemoryIDs = []string{"memory-2", "memory-1"}
	replayed, err := adapter.applyForgetRequest(context.Background(), state, request)
	if err != nil {
		t.Fatalf("reordered applyForgetRequest() error = %v, want durable replay", err)
	}
	if !reflect.DeepEqual(replayed, first) || lifecycle.calls != 2 || len(store.claims) != 2 {
		t.Fatalf("reordered replay = %+v, lifecycle calls=%d claims=%d; want original outcome and no second lifecycle mutation", replayed, lifecycle.calls, len(store.claims))
	}
}

func TestForgetApplyReplaysWithoutRequiringAnExpiredRuntimeBinding(t *testing.T) {
	now := time.Now().UTC()
	scope := memory.Scope{Tenant: "tenant", Project: "project", Namespace: "namespace"}
	principal := auth.Principal{ID: "admin-1", Role: auth.PrincipalRoleAdmin, Status: auth.PrincipalStatusActive}
	store := &forgetApplyStoreStub{disposition: ForgetApplyReplayed, response: ForgetApplyResponse{PreviewID: "fp_1", AppliedIDs: []string{"memory-1"}}}
	adapter := NewAdapter(AdapterOptions{
		Enabled:          true,
		Authorizer:       scopeAuthorizerStub{grants: map[string]bool{principal.ID + "/tenant/project/namespace": true}},
		ForgetApplyStore: store,
		Now:              func() time.Time { return now },
		Limits:           Limits{MaxQueryBytes: 128, MaxPayloadBytes: 4096, MaxResults: 10, MaxIDs: 10},
	})
	request := validForgetApplyRequest()
	request.RuntimeBindingID = ""
	request.ScopeInput = ScopeInput{Tenant: scope.Tenant, Project: scope.Project, Namespace: scope.Namespace}
	response, err := adapter.applyForgetRequest(context.Background(), requestState{principal: principal}, request)
	if err != nil || response.PreviewID != "fp_1" {
		t.Fatalf("durable replay without runtime binding = %+v, %v; want replay after exact scope authorization", response, err)
	}
}

func validForgetApplyRequest() ForgetApplyRequest {
	return ForgetApplyRequest{PreviewID: "fp_1", MemoryIDs: []string{"memory-1"}, Action: string(policy.ForgettingActionSuppress), Reason: "user request", IdempotencyKey: "batch-1", RuntimeBindingID: "binding-1"}
}

func newForgetApplyTestAdapter(t *testing.T, lifecycleErr, completeErr error) (*Adapter, requestState, *lifecycleAdapterStub, *forgetApplyStoreStub) {
	t.Helper()
	now := time.Now().UTC()
	scope := memory.Scope{Tenant: "tenant", Project: "project", Namespace: "namespace"}
	principal := auth.Principal{ID: "admin-1", Role: auth.PrincipalRoleAdmin, Status: auth.PrincipalStatusActive}
	bindings := provider.NewMemoryBindingStore()
	binding := provider.RuntimeBinding{BindingID: "binding-1", PrincipalID: principal.ID, Scope: scope, AgentID: "agent", SessionID: "session", ProviderInstanceID: "instance", CreatedAt: now.Add(-time.Minute), ExpiresAt: now.Add(time.Hour)}
	if err := bindings.Create(context.Background(), binding); err != nil {
		t.Fatal(err)
	}
	authorizer := scopeAuthorizerStub{grants: map[string]bool{principal.ID + "/tenant/project/namespace": true}}
	lifecycle := &lifecycleAdapterStub{applyErr: lifecycleErr}
	store := &forgetApplyStoreStub{completeErr: completeErr}
	preview := &forgetPreviewStoreStub{record: ForgetPreviewRecord{ID: "fp_1", Principal: principal.ID, Scope: scope, MemoryIDs: []string{"memory-1"}, ExpiresAt: now.Add(time.Minute)}}
	adapter := NewAdapter(AdapterOptions{Enabled: true, Authorizer: authorizer, Bindings: bindings, LifecycleAdapter: lifecycle, PreviewStore: preview, ForgetApplyStore: store, Limits: Limits{MaxQueryBytes: 128, MaxPayloadBytes: 4096, MaxResults: 10, MaxIDs: 10}, Now: func() time.Time { return now }})
	return adapter, requestState{principal: principal, bindingID: binding.BindingID}, lifecycle, store
}

type forgetPreviewStoreStub struct {
	record ForgetPreviewRecord
	err    error
}

func (s *forgetPreviewStoreStub) SaveForgetPreview(context.Context, ForgetPreviewRecord) error {
	return nil
}
func (s *forgetPreviewStoreStub) LoadForgetPreview(context.Context, string) (ForgetPreviewRecord, error) {
	return s.record, s.err
}

type forgetApplyStoreStub struct {
	disposition  ForgetApplyDisposition
	response     ForgetApplyResponse
	completeErr  error
	completes    int
	releases     int
	strictReplay bool
	claims       []ForgetApplyClaim
}

func (s *forgetApplyStoreStub) ClaimForgetApply(_ context.Context, claim ForgetApplyClaim) (ForgetApplyClaimResult, error) {
	s.claims = append(s.claims, claim)
	if s.strictReplay && len(s.claims) > 1 {
		if claim.RequestFingerprint == s.claims[0].RequestFingerprint {
			return ForgetApplyClaimResult{Disposition: ForgetApplyReplayed, Response: s.response}, nil
		}
		return ForgetApplyClaimResult{Disposition: ForgetApplyConflict}, nil
	}
	if s.disposition != "" {
		return ForgetApplyClaimResult{Disposition: s.disposition, Response: s.response}, nil
	}
	return ForgetApplyClaimResult{Disposition: ForgetApplyClaimed}, nil
}
func (s *forgetApplyStoreStub) CompleteForgetApply(_ context.Context, _ ForgetApplyClaim, response ForgetApplyResponse) error {
	s.completes++
	s.response = response
	return s.completeErr
}
func (s *forgetApplyStoreStub) ReleaseForgetApply(context.Context, ForgetApplyClaim) error {
	s.releases++
	return nil
}
