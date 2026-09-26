package mcp

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"sort"
	"strings"
	"time"

	"github.com/FelixSeptem/stele/internal/auth"
	"github.com/FelixSeptem/stele/internal/memory"
	"github.com/FelixSeptem/stele/internal/policy"
	"github.com/FelixSeptem/stele/internal/provider"
	"github.com/FelixSeptem/stele/internal/retrieval"
	protocolmcp "github.com/modelcontextprotocol/go-sdk/mcp"
)

type AdapterOptions struct {
	Enabled     bool
	Authorizer  auth.PrincipalAuthorizer
	Bindings    provider.RuntimeBindingStore
	Limits      Limits
	Now         func() time.Time
	Searcher    retrieval.MemorySearcher
	Assembler   retrieval.ContextAssembler
	MemoryQuery interface {
		ListMemories(context.Context, memory.ListMemoriesInput) (memory.MemoryPage, error)
	}
	Intent interface {
		Submit(context.Context, memory.MemoryIntentInput) (memory.MemoryIntentRecord, error)
	}
	Lifecycle interface {
		Apply(context.Context, memory.LifecycleActionInput) error
	}
	LifecycleAdapter LifecycleAdapter
	PreviewStore     ForgetPreviewStore
	ForgetApplyStore ForgetApplyStore
}

type ForgetPreviewStore interface {
	SaveForgetPreview(context.Context, ForgetPreviewRecord) error
	LoadForgetPreview(context.Context, string) (ForgetPreviewRecord, error)
}

type ForgetApplyDisposition string

const (
	ForgetApplyClaimed    ForgetApplyDisposition = "claimed"
	ForgetApplyReplayed   ForgetApplyDisposition = "replayed"
	ForgetApplyInProgress ForgetApplyDisposition = "in_progress"
	ForgetApplyConflict   ForgetApplyDisposition = "conflict"
)

type ForgetApplyClaim struct {
	PrincipalID        string
	Scope              memory.Scope
	IdempotencyKey     string
	RequestFingerprint string
	ClaimID            string
}

type ForgetApplyClaimResult struct {
	Disposition ForgetApplyDisposition
	Response    ForgetApplyResponse
}

type ForgetApplyStore interface {
	ClaimForgetApply(context.Context, ForgetApplyClaim) (ForgetApplyClaimResult, error)
	CompleteForgetApply(context.Context, ForgetApplyClaim, ForgetApplyResponse) error
	ReleaseForgetApply(context.Context, ForgetApplyClaim) error
}

type ForgetPreviewRecord struct {
	ID        string
	Principal string
	Scope     memory.Scope
	MemoryIDs []string
	ExpiresAt time.Time
}

// LifecycleAdapter is the existing provider lifecycle boundary. It owns
// durable claim/replay/audit behavior and is deliberately reused by MCP.
type LifecycleAdapter interface {
	ApplyLifecycle(context.Context, provider.RuntimeBinding, provider.OperationMetadata, string, policy.ForgettingAction, string, string) (provider.OperationOutcome, error)
}

type Adapter struct {
	enabled     bool
	authorizer  auth.PrincipalAuthorizer
	bindings    provider.RuntimeBindingStore
	limits      Limits
	now         func() time.Time
	searcher    retrieval.MemorySearcher
	assembler   retrieval.ContextAssembler
	memoryQuery interface {
		ListMemories(context.Context, memory.ListMemoriesInput) (memory.MemoryPage, error)
	}
	intent interface {
		Submit(context.Context, memory.MemoryIntentInput) (memory.MemoryIntentRecord, error)
	}
	lifecycle interface {
		Apply(context.Context, memory.LifecycleActionInput) error
	}
	lifecycleAdapter LifecycleAdapter
	previewStore     ForgetPreviewStore
	forgetApplyStore ForgetApplyStore
	protocol         *protocolmcp.StreamableHTTPHandler
}

type requestState struct {
	principal auth.Principal
	bindingID string
}

type requestStateKey struct{}

type whoAmIInput struct {
	RuntimeBindingID string `json:"runtime_binding_id,omitempty"`
}

func NewAdapter(options AdapterOptions) *Adapter {
	now := options.Now
	if now == nil {
		now = time.Now
	}
	a := &Adapter{enabled: options.Enabled, authorizer: options.Authorizer, bindings: options.Bindings, limits: options.Limits, now: now, searcher: options.Searcher, assembler: options.Assembler, memoryQuery: options.MemoryQuery, intent: options.Intent, lifecycle: options.Lifecycle, lifecycleAdapter: options.LifecycleAdapter, previewStore: options.PreviewStore, forgetApplyStore: options.ForgetApplyStore}
	if a.limits.MaxPayloadBytes <= 0 {
		a.limits.MaxPayloadBytes = 64 << 10
	}
	a.protocol = protocolmcp.NewStreamableHTTPHandler(a.serverForRequest, &protocolmcp.StreamableHTTPOptions{
		Stateless: true, JSONResponse: true, MaxRequestBodyBytes: int64(a.limits.MaxPayloadBytes), PropagateRequestCancellation: true,
	})
	return a
}

func (a *Adapter) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if a == nil || !a.enabled {
		http.NotFound(w, r)
		return
	}
	if a.authorizer == nil {
		http.Error(w, "mcp unavailable", http.StatusServiceUnavailable)
		return
	}
	principal, credential, err := a.authorizer.Authenticate(r.Context(), strings.TrimSpace(r.Header.Get(auth.HeaderAPIKey)))
	if err != nil || !auth.PrincipalCredentialActive(principal, credential, a.now().UTC()) || strings.TrimSpace(principal.ID) == "" {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	bindingID := strings.TrimSpace(r.Header.Get(provider.HeaderRuntimeBinding))
	ctx := context.WithValue(r.Context(), requestStateKey{}, requestState{principal: principal, bindingID: bindingID})
	a.protocol.ServeHTTP(w, r.WithContext(ctx))
}

func (a *Adapter) serverForRequest(r *http.Request) *protocolmcp.Server {
	state, ok := r.Context().Value(requestStateKey{}).(requestState)
	if !ok {
		return nil
	}
	server := protocolmcp.NewServer(&protocolmcp.Implementation{Name: "stele-mcp", Version: "v1"}, nil)
	protocolmcp.AddTool(server, &protocolmcp.Tool{
		Name: ToolWhoAmI, Description: "Resolve the authenticated principal and exact access scope.",
		Annotations: &protocolmcp.ToolAnnotations{ReadOnlyHint: true, IdempotentHint: true, OpenWorldHint: boolPtr(false)},
	}, func(ctx context.Context, req *protocolmcp.CallToolRequest, input whoAmIInput) (*protocolmcp.CallToolResult, WhoAmIResponse, error) {
		bindingID := strings.TrimSpace(input.RuntimeBindingID)
		if bindingID == "" {
			bindingID = state.bindingID
		}
		dispatch := DispatchContext{PrincipalID: state.principal.ID, Role: string(state.principal.Role), AccessMode: "unresolved"}
		activeAvailable := false
		if bindingID != "" {
			resolved, err := NewScopeResolver(a.authorizer, a.bindings, a.now).Resolve(ctx, state.principal, nil, bindingID)
			if err == nil {
				dispatch = resolved
				activeAvailable = true
			}
		}
		return nil, BuildWhoAmI(dispatch, activeAvailable), nil
	})
	protocolmcp.AddTool(server, &protocolmcp.Tool{
		Name: ToolSearch, Description: "Search governed memories relevant to a bounded query.",
		Annotations: &protocolmcp.ToolAnnotations{ReadOnlyHint: true, IdempotentHint: true, OpenWorldHint: boolPtr(false)},
	}, func(ctx context.Context, req *protocolmcp.CallToolRequest, input SearchRequest) (*protocolmcp.CallToolResult, SearchResponse, error) {
		if err := ValidateToolRequest(ToolSearch, &input, a.limits); err != nil {
			return nil, SearchResponse{}, safeValidationError(err)
		}
		dispatch, err := a.resolveRequestScope(ctx, state, input.ScopeInput, input.RuntimeBindingID)
		if err != nil {
			return nil, SearchResponse{}, err
		}
		if a.searcher == nil {
			return nil, SearchResponse{}, mcpError(ErrorDependency, "search_unavailable")
		}
		constraint, err := parseTemporal(input.AsOf, input.ValidFrom, input.ValidTo)
		if err != nil {
			return nil, SearchResponse{}, safeValidationError(err)
		}
		result, err := a.searcher.Search(ctx, retrieval.SearchInput{Scope: dispatch.Scope, Query: input.Query, TopK: effectiveResultLimit(input.Limit, a.limits.MaxResults), TemporalConstraint: constraint})
		if err != nil {
			return nil, SearchResponse{}, mcpError(ErrorDependency, "search_failed")
		}
		response := shapeSearchResponse(result)
		if limit := effectiveResultLimit(input.Limit, a.limits.MaxResults); len(response.Hits) > limit {
			response.Hits = response.Hits[:limit]
		}
		return nil, response, nil
	})
	protocolmcp.AddTool(server, &protocolmcp.Tool{
		Name: ToolContext, Description: "Assemble bounded context from governed projections and memories.",
		Annotations: &protocolmcp.ToolAnnotations{ReadOnlyHint: true, IdempotentHint: true, OpenWorldHint: boolPtr(false)},
	}, func(ctx context.Context, req *protocolmcp.CallToolRequest, input ContextRequest) (*protocolmcp.CallToolResult, ContextResponse, error) {
		if err := ValidateToolRequest(ToolContext, &input, a.limits); err != nil {
			return nil, ContextResponse{}, safeValidationError(err)
		}
		dispatch, err := a.resolveRequestScope(ctx, state, input.ScopeInput, input.RuntimeBindingID)
		if err != nil {
			return nil, ContextResponse{}, err
		}
		if a.assembler == nil {
			return nil, ContextResponse{}, mcpError(ErrorDependency, "context_unavailable")
		}
		result, err := a.assembler.AssembleContext(ctx, retrieval.AssembleContextInput{Scope: dispatch.Scope, Query: input.Query, Budget: a.limits.MaxResults, CharacterBudget: input.BudgetBytes})
		if err != nil {
			return nil, ContextResponse{}, mcpError(ErrorDependency, "context_failed")
		}
		return nil, shapeContextResponse(result), nil
	})
	protocolmcp.AddTool(server, &protocolmcp.Tool{
		Name: ToolBrowse, Description: "Browse visible memories in one exact scope.",
		Annotations: &protocolmcp.ToolAnnotations{ReadOnlyHint: true, IdempotentHint: true, OpenWorldHint: boolPtr(false)},
	}, func(ctx context.Context, req *protocolmcp.CallToolRequest, input BrowseRequest) (*protocolmcp.CallToolResult, BrowseResponse, error) {
		if err := ValidateToolRequest(ToolBrowse, &input, a.limits); err != nil {
			return nil, BrowseResponse{}, safeValidationError(err)
		}
		dispatch, err := a.resolveRequestScope(ctx, state, input.ScopeInput, input.RuntimeBindingID)
		if err != nil {
			return nil, BrowseResponse{}, err
		}
		if a.memoryQuery == nil {
			return nil, BrowseResponse{}, mcpError(ErrorDependency, "browse_unavailable")
		}
		effectiveLimit := input.Limit
		if effectiveLimit <= 0 {
			effectiveLimit = a.limits.MaxResults
		}
		fetchLimit := input.Offset + effectiveLimit
		page, err := a.memoryQuery.ListMemories(ctx, memory.ListMemoriesInput{Scope: dispatch.Scope, Limit: fetchLimit})
		if err != nil {
			return nil, BrowseResponse{}, mcpError(ErrorDependency, "browse_failed")
		}
		return nil, shapeBrowseResponse(page, input.Offset, effectiveLimit), nil
	})
	protocolmcp.AddTool(server, &protocolmcp.Tool{
		Name: ToolRemember, Description: "Submit a governed memory intent.",
		Annotations: &protocolmcp.ToolAnnotations{IdempotentHint: true, OpenWorldHint: boolPtr(false)},
	}, func(ctx context.Context, req *protocolmcp.CallToolRequest, input RememberRequest) (*protocolmcp.CallToolResult, MutationResponse, error) {
		if err := ValidateToolRequest(ToolRemember, &input, a.limits); err != nil {
			return nil, MutationResponse{}, safeValidationError(err)
		}
		dispatch, err := a.resolveRequestScope(ctx, state, input.ScopeInput, input.RuntimeBindingID)
		if err != nil {
			return nil, MutationResponse{}, err
		}
		if !dispatch.Writable {
			return nil, MutationResponse{}, mcpError(ErrorAuth, "read_only_scope")
		}
		if a.intent == nil {
			return nil, MutationResponse{}, mcpError(ErrorDependency, "mutation_unavailable")
		}
		intentType := memory.MemoryIntentRemember
		if strings.TrimSpace(input.TargetMemoryID) != "" {
			intentType = memory.MemoryIntentUpdate
		}
		record, err := a.intent.Submit(ctx, memory.MemoryIntentInput{Scope: dispatch.Scope, Type: intentType, TargetMemoryID: input.TargetMemoryID, TargetVersion: input.TargetVersion, Content: input.Content, Actor: state.principal.ID, Reason: input.Reason, Provenance: map[string]any{"transport": "mcp", "tool": ToolRemember}, RequestID: input.IdempotencyKey, OperationID: input.IdempotencyKey, IdempotencyKey: input.IdempotencyKey})
		if err != nil {
			return nil, MutationResponse{}, safeMutationError(err)
		}
		response := MutationResponse{IntentID: record.ID, Status: string(record.Status)}
		return nil, response, nil
	})
	protocolmcp.AddTool(server, &protocolmcp.Tool{
		Name: ToolForget, Description: "Request a governed lifecycle action for one explicitly identified memory.",
		Annotations: &protocolmcp.ToolAnnotations{DestructiveHint: boolPtr(true), IdempotentHint: true, OpenWorldHint: boolPtr(false)},
	}, func(ctx context.Context, req *protocolmcp.CallToolRequest, input ForgetRequest) (*protocolmcp.CallToolResult, ForgetResponse, error) {
		if err := ValidateToolRequest(ToolForget, &input, a.limits); err != nil {
			return nil, ForgetResponse{}, safeValidationError(err)
		}
		if state.principal.Status != auth.PrincipalStatusActive || state.principal.Role != auth.PrincipalRoleAdmin {
			return nil, ForgetResponse{}, mcpError(ErrorAuth, "lifecycle_forbidden")
		}
		dispatch, err := a.resolveRequestScope(ctx, state, input.ScopeInput, input.RuntimeBindingID)
		if err != nil {
			return nil, ForgetResponse{}, err
		}
		if !dispatch.Writable {
			return nil, ForgetResponse{}, mcpError(ErrorAuth, "read_only_scope")
		}
		bindingID := strings.TrimSpace(input.RuntimeBindingID)
		if bindingID == "" {
			bindingID = state.bindingID
		}
		binding, err := a.bindingForLifecycle(ctx, state.principal, dispatch.Scope, bindingID)
		if err != nil {
			return nil, ForgetResponse{}, err
		}
		action := strings.TrimSpace(input.Action)
		if action == "" {
			action = string(policy.ForgettingActionSuppress)
		}
		_, err = a.applyForgetLifecycle(ctx, state.principal, binding, ForgetApplyRequest{
			PreviewID: "single-" + input.IdempotencyKey, MemoryIDs: []string{strings.TrimSpace(input.MemoryID)},
			Action: action, Reason: input.Reason, IdempotencyKey: input.IdempotencyKey,
		})
		if err != nil {
			return nil, ForgetResponse{}, err
		}
		return nil, ForgetResponse{MemoryID: strings.TrimSpace(input.MemoryID), Action: action, Status: "applied"}, nil
	})
	protocolmcp.AddTool(server, &protocolmcp.Tool{
		Name: ToolForgetPreview, Description: "Preview bounded semantic forget candidates without mutation.",
		Annotations: &protocolmcp.ToolAnnotations{ReadOnlyHint: true, IdempotentHint: true, OpenWorldHint: boolPtr(false)},
	}, func(ctx context.Context, req *protocolmcp.CallToolRequest, input ForgetPreviewRequest) (*protocolmcp.CallToolResult, ForgetPreviewResponse, error) {
		if err := ValidateToolRequest(ToolForgetPreview, &input, a.limits); err != nil {
			return nil, ForgetPreviewResponse{}, safeValidationError(err)
		}
		dispatch, err := a.resolveRequestScope(ctx, state, input.ScopeInput, input.RuntimeBindingID)
		if err != nil {
			return nil, ForgetPreviewResponse{}, err
		}
		if a.searcher == nil {
			return nil, ForgetPreviewResponse{}, mcpError(ErrorDependency, "forget_unavailable")
		}
		previewLimit := effectiveResultLimit(input.Limit, a.limits.MaxResults)
		previewLimit = effectiveResultLimit(previewLimit, a.limits.MaxIDs)
		result, err := a.searcher.Search(ctx, retrieval.SearchInput{Scope: dispatch.Scope, Query: input.Query, TopK: previewLimit})
		if err != nil {
			return nil, ForgetPreviewResponse{}, mcpError(ErrorDependency, "forget_failed")
		}
		ids := make([]string, 0, len(result.Hits))
		seen := make(map[string]struct{}, len(result.Hits))
		for _, hit := range result.Hits {
			if input.Threshold > 0 && hit.Score.Semantic < input.Threshold {
				continue
			}
			if len(ids) >= previewLimit {
				break
			}
			id := strings.TrimSpace(hit.Memory.ID)
			if id == "" || len(id) > 256 {
				continue
			}
			if _, exists := seen[id]; exists {
				continue
			}
			seen[id] = struct{}{}
			ids = append(ids, id)
		}
		if a.previewStore == nil {
			return nil, ForgetPreviewResponse{}, mcpError(ErrorDependency, "forget_preview_unavailable")
		}
		previewID := previewIdentity(state.principal.ID, dispatch.Scope, ids)
		preview := ForgetPreviewRecord{ID: previewID, Principal: state.principal.ID, Scope: dispatch.Scope, MemoryIDs: append([]string(nil), ids...), ExpiresAt: a.now().UTC().Add(10 * time.Minute)}
		if err := a.previewStore.SaveForgetPreview(ctx, preview); err != nil {
			return nil, ForgetPreviewResponse{}, mcpError(ErrorDependency, "forget_preview_unavailable")
		}
		return nil, ForgetPreviewResponse{PreviewID: previewID, MemoryIDs: ids}, nil
	})
	protocolmcp.AddTool(server, &protocolmcp.Tool{
		Name: ToolForgetApply, Description: "Apply only reviewed IDs from a forget preview.",
		Annotations: &protocolmcp.ToolAnnotations{DestructiveHint: boolPtr(true), IdempotentHint: true, OpenWorldHint: boolPtr(false)},
	}, func(ctx context.Context, req *protocolmcp.CallToolRequest, input ForgetApplyRequest) (*protocolmcp.CallToolResult, ForgetApplyResponse, error) {
		if err := ValidateToolRequest(ToolForgetApply, &input, a.limits); err != nil {
			return nil, ForgetApplyResponse{}, safeValidationError(err)
		}
		response, err := a.applyForgetRequest(ctx, state, input)
		return nil, response, err
	})
	return server
}

func (a *Adapter) applyForgetRequest(ctx context.Context, state requestState, input ForgetApplyRequest) (ForgetApplyResponse, error) {
	if err := ValidateToolRequest(ToolForgetApply, &input, a.limits); err != nil {
		return ForgetApplyResponse{}, safeValidationError(err)
	}
	if state.principal.Status != auth.PrincipalStatusActive || state.principal.Role != auth.PrincipalRoleAdmin {
		return ForgetApplyResponse{}, mcpError(ErrorAuth, "lifecycle_forbidden")
	}
	dispatch, err := a.resolveRequestScope(ctx, state, input.ScopeInput, "")
	if err != nil {
		return ForgetApplyResponse{}, err
	}
	if !dispatch.Writable {
		return ForgetApplyResponse{}, mcpError(ErrorAuth, "read_only_scope")
	}
	if a.forgetApplyStore == nil {
		return ForgetApplyResponse{}, mcpError(ErrorDependency, "forget_apply_unavailable")
	}
	claimID, err := newForgetApplyClaimID()
	if err != nil {
		return ForgetApplyResponse{}, mcpError(ErrorRetryable, "forget_apply_claim_failed")
	}
	semanticRequest := input
	semanticRequest.RuntimeBindingID = ""
	semanticRequest.ScopeInput = ScopeInput{Tenant: dispatch.Scope.Tenant, Project: dispatch.Scope.Project, Namespace: dispatch.Scope.Namespace}
	semanticRequest.MemoryIDs = append([]string(nil), input.MemoryIDs...)
	for i := range semanticRequest.MemoryIDs {
		semanticRequest.MemoryIDs[i] = strings.TrimSpace(semanticRequest.MemoryIDs[i])
	}
	sort.Strings(semanticRequest.MemoryIDs)
	claim := ForgetApplyClaim{PrincipalID: state.principal.ID, Scope: dispatch.Scope, IdempotencyKey: input.IdempotencyKey, RequestFingerprint: fingerprint(struct {
		Scope   memory.Scope
		Request ForgetApplyRequest
	}{Scope: dispatch.Scope, Request: semanticRequest}), ClaimID: claimID}
	claimResult, err := a.forgetApplyStore.ClaimForgetApply(ctx, claim)
	if err != nil {
		return ForgetApplyResponse{}, mcpError(ErrorRetryable, "forget_apply_claim_failed")
	}
	switch claimResult.Disposition {
	case ForgetApplyReplayed:
		return claimResult.Response, nil
	case ForgetApplyConflict:
		return ForgetApplyResponse{}, mcpError(ErrorValidation, "idempotency_conflict")
	case ForgetApplyInProgress:
		return ForgetApplyResponse{}, mcpError(ErrorRetryable, "operation_in_progress")
	case ForgetApplyClaimed:
	default:
		return ForgetApplyResponse{}, mcpError(ErrorDependency, "forget_apply_claim_invalid")
	}
	releaseClaim := func(original error) error {
		if err := a.forgetApplyStore.ReleaseForgetApply(ctx, claim); err != nil {
			return mcpError(ErrorRetryable, "forget_apply_release_failed")
		}
		return original
	}
	bindingID := input.RuntimeBindingID
	if bindingID == "" {
		bindingID = state.bindingID
	}
	binding, err := a.bindingForLifecycle(ctx, state.principal, dispatch.Scope, bindingID)
	if err != nil {
		return ForgetApplyResponse{}, releaseClaim(err)
	}
	if a.previewStore == nil {
		return ForgetApplyResponse{}, releaseClaim(mcpError(ErrorDependency, "forget_preview_unavailable"))
	}
	preview, err := a.previewStore.LoadForgetPreview(ctx, input.PreviewID)
	if err != nil || preview.Principal != state.principal.ID || preview.ExpiresAt.Before(a.now().UTC()) || preview.Scope != dispatch.Scope || !sameIDs(preview.MemoryIDs, input.MemoryIDs) {
		return ForgetApplyResponse{}, releaseClaim(mcpError(ErrorValidation, "preview_invalid"))
	}
	response, err := a.applyForgetLifecycle(ctx, state.principal, binding, input)
	if err != nil {
		return ForgetApplyResponse{}, releaseClaim(err)
	}
	if err := a.forgetApplyStore.CompleteForgetApply(ctx, claim, response); err != nil {
		return ForgetApplyResponse{}, releaseClaim(mcpError(ErrorRetryable, "forget_apply_completion_failed"))
	}
	return response, nil
}

func (a *Adapter) bindingForLifecycle(ctx context.Context, principal auth.Principal, scope memory.Scope, bindingID string) (provider.RuntimeBinding, error) {
	if a.bindings == nil || strings.TrimSpace(bindingID) == "" {
		return provider.RuntimeBinding{}, mcpError(ErrorScope, "active_scope_unavailable")
	}
	binding, err := provider.ValidateRuntimeOperation(ctx, a.bindings, a.authorizer, bindingID, principal.ID, "", &scope)
	if err != nil {
		return provider.RuntimeBinding{}, mcpError(ErrorScope, "forbidden")
	}
	return binding, nil
}

func (a *Adapter) applyForgetLifecycle(ctx context.Context, principal auth.Principal, binding provider.RuntimeBinding, input ForgetApplyRequest) (ForgetApplyResponse, error) {
	if principal.Status != auth.PrincipalStatusActive || principal.Role != auth.PrincipalRoleAdmin || principal.ID == "" || principal.ID != binding.PrincipalID {
		return ForgetApplyResponse{}, mcpError(ErrorAuth, "lifecycle_forbidden")
	}
	if a.lifecycleAdapter == nil {
		return ForgetApplyResponse{}, mcpError(ErrorDependency, "lifecycle_unavailable")
	}
	action := policy.ForgettingAction(strings.TrimSpace(input.Action))
	if action == "" {
		action = policy.ForgettingActionSuppress
	}
	if err := action.Validate(); err != nil {
		return ForgetApplyResponse{}, safeValidationError(err)
	}
	ctx = auth.ContextWithPrincipal(ctx, principal)
	for _, id := range input.MemoryIDs {
		derivedKey := lifecycleIdempotencyKey(input.IdempotencyKey, id)
		metadata := provider.OperationMetadata{RequestID: lifecycleOperationToken("request", input.PreviewID), OperationID: lifecycleOperationToken(input.PreviewID, id), IdempotencyKey: derivedKey, SchemaVersion: "provider-v1"}
		if _, err := a.lifecycleAdapter.ApplyLifecycle(ctx, binding, metadata, id, action, input.Reason, principal.ID); err != nil {
			message := strings.ToLower(err.Error())
			if strings.Contains(message, "idempotency conflict") {
				return ForgetApplyResponse{}, mcpError(ErrorValidation, "idempotency_conflict")
			}
			if strings.Contains(message, "operation in progress") {
				return ForgetApplyResponse{}, mcpError(ErrorRetryable, "operation_in_progress")
			}
			return ForgetApplyResponse{}, mcpError(ErrorLifecycle, "lifecycle_failed")
		}
	}
	return ForgetApplyResponse{PreviewID: input.PreviewID, AppliedIDs: append([]string(nil), input.MemoryIDs...)}, nil
}

func lifecycleIdempotencyKey(batchKey, memoryID string) string {
	sum := sha256.Sum256([]byte(batchKey + "\x00" + memoryID))
	return "mcp-life-" + hex.EncodeToString(sum[:])
}

func lifecycleOperationToken(parts ...string) string {
	h := sha256.New()
	for _, part := range parts {
		h.Write([]byte(part))
		h.Write([]byte{0})
	}
	return "mcp-life-op-" + hex.EncodeToString(h.Sum(nil))
}

func newForgetApplyClaimID() (string, error) {
	var value [16]byte
	if _, err := rand.Read(value[:]); err != nil {
		return "", err
	}
	return "mcp-" + hex.EncodeToString(value[:]), nil
}

func previewIdentity(principalID string, scope memory.Scope, ids []string) string {
	h := sha256.New()
	h.Write([]byte(principalID + "\x00"))
	h.Write([]byte(scope.Tenant + "\x00" + scope.Project + "\x00" + scope.Namespace + "\x00"))
	for _, id := range ids {
		h.Write([]byte(id + "\x00"))
	}
	return "fp_" + hex.EncodeToString(h.Sum(nil))[:24]
}

func sameIDs(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	seen := make(map[string]struct{}, len(a))
	for _, id := range a {
		if _, ok := seen[id]; ok {
			return false
		}
		seen[id] = struct{}{}
	}
	for _, id := range b {
		if _, ok := seen[id]; !ok {
			return false
		}
		delete(seen, id)
	}
	return len(seen) == 0
}

func (a *Adapter) resolveRequestScope(ctx context.Context, state requestState, requested ScopeInput, bindingID string) (DispatchContext, error) {
	if requested.Tenant != "" || requested.Project != "" || requested.Namespace != "" {
		scope := requested.ToScope()
		return NewScopeResolver(a.authorizer, a.bindings, a.now).Resolve(ctx, state.principal, &scope, bindingID)
	}
	if bindingID == "" {
		bindingID = state.bindingID
	}
	return NewScopeResolver(a.authorizer, a.bindings, a.now).Resolve(ctx, state.principal, nil, bindingID)
}

func boolPtr(value bool) *bool { return &value }

func effectiveResultLimit(requested, maximum int) int {
	if requested <= 0 || requested > maximum {
		return maximum
	}
	return requested
}

var _ http.Handler = (*Adapter)(nil)
