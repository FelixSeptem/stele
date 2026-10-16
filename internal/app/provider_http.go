package app

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"github.com/FelixSeptem/stele/internal/auth"
	"github.com/FelixSeptem/stele/internal/memory"
	"github.com/FelixSeptem/stele/internal/policy"
	"github.com/FelixSeptem/stele/internal/provider"
	"github.com/FelixSeptem/stele/internal/telemetry"
	"github.com/FelixSeptem/stele/openapi"
	"io"
	"net/http"
	"strings"
)

func registerProviderRoutes(mux *http.ServeMux, deps HTTPDependencies) {
	mux.HandleFunc("GET /v1/provider/capabilities", func(w http.ResponseWriter, r *http.Request) {
		doc := deps.ProviderCapabilities
		if err := doc.Validate(); err != nil {
			doc = provider.Discover(provider.CapabilityInput{ProviderVersion: "provider-v1", SchemaVersion: "schema-v1"})
		}
		doc.ContextContinuity.Enabled = deps.ProviderContextReceipts != nil && deps.ProviderContextContinuity.Validate() == nil && deps.ProviderContextPolicy != nil
		doc.ContextContinuity.FreshCompare = doc.ContextContinuity.Enabled
		doc.DurableReferences = provider.DurableReferenceCapability(deps.ProviderReferences != nil && deps.ProviderAdapter != nil)
		doc.Projections = provider.ProjectionCapability(deps.ProviderProjections != nil)
		if deps.ProviderContextContinuity.Validate() == nil {
			doc.ContextContinuity.TTLSeconds = deps.ProviderContextContinuity.TTLSeconds
		}
		doc.SchemaDigest = fmt.Sprintf("sha256:%x", sha256.Sum256([]byte(openapi.SpecYAMLWithMCPPath(deps.MCP.Path))))
		writeJSON(w, http.StatusOK, doc)
	})
	runtimeInit := auth.PrincipalMiddleware(deps.PrincipalAuthorizer, auth.PrincipalRolePublic)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if deps.ProviderInitializer == nil {
			writeProviderError(w, http.StatusServiceUnavailable, "dependency", "provider_unavailable", "provider runtime is not configured", true)
			return
		}
		var req provider.RuntimeInitialization
		if err := provider.DecodeStrict(readBody(r), &req); err != nil {
			writeProviderError(w, http.StatusBadRequest, "validation", "invalid_request", "invalid request", false)
			return
		}
		principal, ok := auth.PrincipalFromContext(r.Context())
		if !ok {
			writeProviderError(w, http.StatusUnauthorized, "authentication", "unauthorized", "authentication required", false)
			return
		}
		scope, _ := auth.ScopeFromContext(r.Context())
		if _, ok := currentDisclosure(w, r, deps.PrincipalAuthorizer, principal.ID, scope); !ok {
			return
		}
		b, err := deps.ProviderInitializer.Initialize(r.Context(), principal, req)
		if err != nil {
			writeProviderError(w, http.StatusForbidden, "scope", "forbidden", "runtime initialization denied", false)
			return
		}
		writeJSON(w, http.StatusCreated, b)
	}))
	// Runtime initialization is an exact-scope operation. The scope middleware
	// must run before disclosure resolution so the current grant is evaluated
	// against the requested tenant/project/namespace.
	mux.Handle("POST /v1/provider/runtime", auth.ScopeMiddleware()(runtimeInit))
	if deps.ProviderSynchronizer != nil {
		registerProviderSyncRoute(mux, deps)
	}
	registerProviderProjectionRoutes(mux, deps)
	if deps.ProviderAdapter != nil {
		registerProviderOperationRoutes(mux, deps)
		registerProviderReferenceRoutes(mux, deps)
	}
}

func registerProviderSyncRoute(mux *http.ServeMux, deps HTTPDependencies) {
	wrap := func(next http.Handler) http.Handler {
		return provider.RuntimeBindingMiddleware(deps.ProviderBindings, deps.PrincipalAuthorizer)(next)
	}
	mux.Handle("POST /v1/provider/sync", wrap(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		syncOperation := "initial"
		var requestCursor string
		// Decode once below; this lightweight operation marker is populated after
		// validation so malformed requests remain validation failures.
		binding, ok := provider.RuntimeBindingFromContext(r.Context())
		if !ok {
			writeProviderError(w, http.StatusForbidden, provider.ErrorCategoryScope, "forbidden", "runtime binding denied", false)
			return
		}
		var req provider.SyncRequest
		if err := provider.DecodeStrict(readBody(r), &req); err != nil {
			recordProviderSyncMetric(r, deps, syncOperation, "failed", "none", "validation")
			writeProviderError(w, http.StatusBadRequest, provider.ErrorCategoryValidation, "invalid_request", "invalid synchronization request", false)
			return
		}
		requestCursor = req.Cursor
		if strings.TrimSpace(requestCursor) != "" {
			syncOperation = "resume"
		}
		out, err := deps.ProviderSynchronizer.Synchronize(r.Context(), binding, req)
		if err != nil {
			category := providerErrorCategory(err)
			recovery := "none"
			if category == provider.ErrorCategoryResyncRequired {
				recovery = "resync_required"
			}
			recordProviderSyncMetric(r, deps, syncOperation, "failed", recovery, string(category))
			status := http.StatusBadRequest
			code := "synchronization_failed"
			if category == provider.ErrorCategoryScope {
				status = http.StatusForbidden
			}
			if category == provider.ErrorCategoryResyncRequired {
				status, code = http.StatusConflict, "resync_required"
			}
			if category == provider.ErrorCategoryDependency {
				status, code = http.StatusServiceUnavailable, "provider_unavailable"
			}
			writeProviderError(w, status, category, code, boundedSynchronizationMessage(category), category == provider.ErrorCategoryRetryable)
			return
		}
		result := "batch"
		if out.SyncComplete {
			result = "completed"
		}
		recordProviderSyncMetric(r, deps, syncOperation, result, "none", "none")
		writeJSON(w, http.StatusOK, out)
	})))
}

func boundedSynchronizationMessage(category provider.ErrorCategory) string {
	switch category {
	case provider.ErrorCategoryResyncRequired:
		return "full synchronization required"
	case provider.ErrorCategoryScope:
		return "runtime synchronization scope denied"
	case provider.ErrorCategoryCompatibility:
		return "unsupported synchronization contract"
	case provider.ErrorCategoryDependency:
		return "provider synchronization unavailable"
	case provider.ErrorCategoryRetryable:
		return "synchronization retryable failure"
	case provider.ErrorCategoryValidation:
		return "invalid synchronization request"
	default:
		return "synchronization failed"
	}
}

func recordProviderSyncMetric(r *http.Request, deps HTTPDependencies, operation, result, recovery, failureCategory string) {
	if deps.Metrics == nil {
		return
	}
	observer, ok := deps.Metrics.(interface {
		RecordSynchronization(context.Context, telemetry.SynchronizationEvent)
	})
	if !ok {
		return
	}
	observer.RecordSynchronization(r.Context(), telemetry.SynchronizationEvent{
		Operation: operation, Result: result, Recovery: recovery,
		Transport: provider.SyncTransportOpenAPI, FailureCategory: failureCategory,
	})
	if healthObserver, ok := deps.Metrics.(interface {
		RecordSynchronizationHealth(context.Context, telemetry.SynchronizationHealthEvent)
	}); ok {
		health := telemetry.SynchronizationHealthEvent{Retention: "healthy", Cursor: "healthy", Backlog: "empty", Pending: 0}
		if recovery == "resync_required" || failureCategory == string(provider.ErrorCategoryResyncRequired) {
			health.Retention = "degraded"
			health.Cursor = "expired"
		}
		healthObserver.RecordSynchronizationHealth(r.Context(), health)
	}
}

func registerProviderOperationRoutes(mux *http.ServeMux, deps HTTPDependencies) {
	wrap := func(next http.Handler) http.Handler {
		return provider.RuntimeBindingMiddleware(deps.ProviderBindings, deps.PrincipalAuthorizer)(next)
	}
	mux.Handle("POST /v1/provider/events", wrap(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := provider.RuntimeBindingFromContext(r.Context())
		var req struct {
			Metadata       provider.OperationMetadata `json:"metadata"`
			EventType      string                     `json:"event_type"`
			Content        string                     `json:"content"`
			MemoryPath     string                     `json:"memory_path,omitempty"`
			MetadataMap    map[string]any             `json:"metadata_map"`
			SourceMetadata memory.SourceMetadata      `json:"source_metadata,omitempty"`
		}
		data := readBody(r)
		_, canonicalErr := provider.CanonicalObject(data, []string{"metadata", "event_type", "content", "memory_path", "metadata_map", "source_metadata"}, []string{"metadata", "event_type", "content"})
		if err := provider.DecodeStrict(data, &req); err != nil || canonicalErr != nil {
			writeProviderError(w, 400, "validation", "invalid_request", "invalid request", false)
			return
		}
		if !supportsProviderSchema(deps.ProviderSchemaVersions, req.Metadata.SchemaVersion) {
			writeProviderCompatibilityError(w, deps.ProviderSchemaVersions)
			return
		}
		e, ok := currentDisclosure(w, r, deps.PrincipalAuthorizer, b.PrincipalID, b.Scope)
		if !ok {
			return
		}
		if req.SourceMetadata.Trust == memory.SourceTrustAttestedData && !e.SourceAttestation {
			writeProviderError(w, 403, "scope", "disclosure_denied", "disclosure denied", false)
			return
		}
		out, err := deps.ProviderAdapter.Ingest(r.Context(), b, req.Metadata, memory.IngestEventInput{EventType: req.EventType, Content: req.Content, MemoryPath: req.MemoryPath, Metadata: req.MetadataMap, SourceMetadata: req.SourceMetadata})
		if err != nil {
			writeProviderError(w, 400, providerErrorCategory(err), "operation_failed", boundedProviderMessage(err), providerErrorCategory(err) == provider.ErrorCategoryRetryable)
			return
		}
		writeJSON(w, 201, out)
	})))
	mux.Handle("POST /v1/provider/intents", wrap(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := provider.RuntimeBindingFromContext(r.Context())
		var req struct {
			Metadata provider.OperationMetadata `json:"metadata"`
			Intent   memory.MemoryIntentInput   `json:"intent"`
		}
		if err := provider.DecodeStrict(readBody(r), &req); err != nil {
			writeProviderError(w, 400, provider.ErrorCategoryValidation, "invalid_request", "invalid request", false)
			return
		}
		if !supportsProviderSchema(deps.ProviderSchemaVersions, req.Metadata.SchemaVersion) {
			writeProviderCompatibilityError(w, deps.ProviderSchemaVersions)
			return
		}
		if _, ok := currentDisclosure(w, r, deps.PrincipalAuthorizer, b.PrincipalID, b.Scope); !ok {
			return
		}
		out, err := deps.ProviderAdapter.SubmitIntent(r.Context(), b, req.Metadata, req.Intent)
		if err != nil {
			writeProviderError(w, 400, providerErrorCategory(err), "operation_failed", boundedProviderMessage(err), providerErrorCategory(err) == provider.ErrorCategoryRetryable)
			return
		}
		writeJSON(w, http.StatusAccepted, map[string]any{"metadata": req.Metadata, "result": map[string]any{"intent_id": out.ID, "status": out.Status}})
	})))
	mux.Handle("POST /v1/provider/retrieve", wrap(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := provider.RuntimeBindingFromContext(r.Context())
		var req struct {
			Metadata provider.OperationMetadata `json:"metadata"`
			Input    memorySearchRequest        `json:"input"`
		}
		if err := provider.DecodeStrict(readBody(r), &req); err != nil {
			writeProviderError(w, 400, provider.ErrorCategoryValidation, "invalid_request", "invalid request", false)
			return
		}
		if !supportsProviderSchema(deps.ProviderSchemaVersions, req.Metadata.SchemaVersion) {
			writeProviderCompatibilityError(w, deps.ProviderSchemaVersions)
			return
		}
		input, err := req.Input.toRetrievalSearchInput()
		if err != nil {
			writeProviderError(w, 400, provider.ErrorCategoryValidation, "invalid_request", "invalid request", false)
			return
		}
		out, meta, err := deps.ProviderAdapter.Search(r.Context(), b, req.Metadata, input)
		if err != nil {
			writeProviderError(w, 400, providerErrorCategory(err), "operation_failed", boundedProviderMessage(err), providerErrorCategory(err) == provider.ErrorCategoryRetryable)
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"metadata": meta, "result": out, "citations": provider.ShapeSearchCitationsWithLimit(out, providerCitationLimit(deps.ProviderLimits))})
	})))
	mux.Handle("POST /v1/provider/context", wrap(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := provider.RuntimeBindingFromContext(r.Context())
		req, err := provider.DecodeContextRequest(readBody(r))
		if err != nil {
			writeProviderError(w, 400, provider.ErrorCategoryValidation, "invalid_request", "invalid request", false)
			return
		}
		if !supportsProviderSchema(deps.ProviderSchemaVersions, req.Metadata.SchemaVersion) {
			writeProviderCompatibilityError(w, deps.ProviderSchemaVersions)
			return
		}
		input, err := req.Input.RetrievalInput(b.Scope)
		if err != nil {
			writeProviderError(w, 400, provider.ErrorCategoryValidation, "invalid_request", "invalid request", false)
			return
		}
		e, ok := currentDisclosure(w, r, deps.PrincipalAuthorizer, b.PrincipalID, b.Scope)
		if !ok {
			return
		}
		decision, err := e.Resolve(req.Input.DisclosureProfile, req.Input.ResponseMode)
		if err != nil {
			writeProviderError(w, 403, "scope", "disclosure_denied", "disclosure denied", false)
			return
		}
		if req.Input.ProjectionIntent != "" {
			decision.ProjectionIntent = req.Input.ProjectionIntent
		}
		admission, err := provider.NewContextAdmission(req.Input, deps.ProviderLimits, b.Scope, decision, req.Metadata)
		if err != nil {
			writeProviderError(w, 413, "validation", "context_limit_exceeded", "context response exceeds allocation", false)
			return
		}
		fingerprint, previous, ok := prepareContextContinuity(w, r, deps, b, req, admission)
		if !ok {
			return
		}
		input.Budget = min(input.Budget, deps.ProviderLimits.Resolved().MaxContextItems)
		input.FinalAdmission = admission
		input.DisclosureRequired = true
		input.IncludeExperienceInsights = false
		input.IncludeGoalContext = false
		input.IncludeDiagnostics = false
		input.IncludeFeedbackDiagnostics = false
		input.UseProjections = false
		out, meta, err := deps.ProviderAdapter.AssembleContext(r.Context(), b, req.Metadata, input)
		if err != nil {
			// The request and current disclosure were already validated. An
			// assembly/read failure must not become an empty successful context.
			category := providerErrorCategory(err)
			if category == provider.ErrorCategoryScope {
				writeProviderError(w, http.StatusForbidden, category, "disclosure_denied", "disclosure denied", false)
			} else {
				writeProviderError(w, http.StatusServiceUnavailable, provider.ErrorCategoryDependency, "provider_unavailable", "provider context unavailable", true)
			}
			return
		}
		if deps.ProviderContextPolicy != nil {
			manifest, err := deps.ProviderContextPolicy(r.Context(), b)
			if err != nil || provider.SelectionPolicyFingerprint(manifest) != admission.PreflightResponse().Result.ContextDigest.SelectionPolicyFingerprint {
				contextContinuityFailure(w, err)
				return
			}
		}
		serialized, err := admission.Finalize(out, meta)
		if err != nil {
			writeProviderError(w, 413, "validation", "context_limit_exceeded", "context response exceeds allocation", false)
			return
		}

		serialized, ok = persistContextContinuity(w, r, deps, b, req, admission, fingerprint, previous, serialized, func() ([]byte, error) { return admission.Finalize(out, meta) })
		if !ok {
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(200)
		_, _ = w.Write(serialized)
	})))
	mux.Handle("POST /v1/provider/turns", wrap(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := provider.RuntimeBindingFromContext(r.Context())
		var req struct {
			Metadata provider.OperationMetadata          `json:"metadata"`
			Input    memory.CreateMemorySessionTurnInput `json:"input"`
		}
		if err := provider.DecodeStrict(readBody(r), &req); err != nil {
			writeProviderError(w, http.StatusBadRequest, provider.ErrorCategoryValidation, "invalid_request", "invalid request", false)
			return
		}
		if !supportsProviderSchema(deps.ProviderSchemaVersions, req.Metadata.SchemaVersion) {
			writeProviderCompatibilityError(w, deps.ProviderSchemaVersions)
			return
		}
		out, err := deps.ProviderAdapter.CreateTurn(r.Context(), b, req.Metadata, req.Input)
		if err != nil {
			cat := providerErrorCategory(err)
			writeProviderError(w, http.StatusBadRequest, cat, "operation_failed", boundedProviderMessage(err), cat == provider.ErrorCategoryRetryable)
			return
		}
		writeJSON(w, http.StatusCreated, map[string]any{"metadata": req.Metadata, "result": out})
	})))
	mux.Handle("POST /v1/provider/turn-outcomes", wrap(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := provider.RuntimeBindingFromContext(r.Context())
		var req struct {
			Metadata provider.OperationMetadata                 `json:"metadata"`
			Input    memory.RecordMemorySessionTurnOutcomeInput `json:"input"`
		}
		if err := provider.DecodeStrict(readBody(r), &req); err != nil {
			writeProviderError(w, http.StatusBadRequest, provider.ErrorCategoryValidation, "invalid_request", "invalid request", false)
			return
		}
		if !supportsProviderSchema(deps.ProviderSchemaVersions, req.Metadata.SchemaVersion) {
			writeProviderCompatibilityError(w, deps.ProviderSchemaVersions)
			return
		}
		out, err := deps.ProviderAdapter.RecordTurnOutcome(r.Context(), b, req.Metadata, req.Input)
		if err != nil {
			cat := providerErrorCategory(err)
			writeProviderError(w, http.StatusBadRequest, cat, "operation_failed", boundedProviderMessage(err), cat == provider.ErrorCategoryRetryable)
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"metadata": req.Metadata, "result": out})
	})))
	mux.Handle("POST /v1/provider/lifecycle", wrap(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		binding, ok := provider.RuntimeBindingFromContext(r.Context())
		if !ok {
			writeProviderError(w, http.StatusForbidden, provider.ErrorCategoryScope, "forbidden", "runtime binding denied", false)
			return
		}
		principal, ok := auth.PrincipalFromContext(r.Context())
		if !ok || principal.Role != auth.PrincipalRoleAdmin {
			writeProviderError(w, http.StatusForbidden, provider.ErrorCategoryLifecycle, "lifecycle_denied", "provider lifecycle operation requires privileged authorization", false)
			return
		}
		var req struct {
			Metadata provider.OperationMetadata `json:"metadata"`
			MemoryID string                     `json:"memory_id"`
			Action   policy.ForgettingAction    `json:"action"`
			Reason   string                     `json:"reason"`
		}
		if err := provider.DecodeStrict(readBody(r), &req); err != nil {
			writeProviderError(w, http.StatusBadRequest, provider.ErrorCategoryValidation, "invalid_request", "invalid request", false)
			return
		}
		if !supportsProviderSchema(deps.ProviderSchemaVersions, req.Metadata.SchemaVersion) {
			writeProviderCompatibilityError(w, deps.ProviderSchemaVersions)
			return
		}
		if err := req.Action.Validate(); err != nil || strings.TrimSpace(req.MemoryID) == "" || strings.TrimSpace(req.Reason) == "" {
			writeProviderError(w, http.StatusBadRequest, provider.ErrorCategoryValidation, "invalid_request", "invalid lifecycle request", false)
			return
		}
		out, err := deps.ProviderAdapter.ApplyLifecycle(r.Context(), binding, req.Metadata, req.MemoryID, req.Action, req.Reason, principal.ID)
		if err != nil {
			category := providerErrorCategory(err)
			status := http.StatusBadRequest
			if category == provider.ErrorCategoryScope || category == provider.ErrorCategoryLifecycle {
				status = http.StatusForbidden
			}
			writeProviderError(w, status, category, "operation_failed", boundedProviderMessage(err), category == provider.ErrorCategoryRetryable)
			return
		}
		writeJSON(w, http.StatusOK, out)
	})))
	mux.Handle("GET /v1/provider/status", wrap(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		binding, ok := provider.RuntimeBindingFromContext(r.Context())
		if !ok {
			writeProviderError(w, http.StatusForbidden, provider.ErrorCategoryScope, "forbidden", "runtime binding denied", false)
			return
		}
		if _, ok := currentDisclosure(w, r, deps.PrincipalAuthorizer, binding.PrincipalID, binding.Scope); !ok {
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{
			"status":               "ready",
			"scope":                binding.Scope,
			"agent_id":             binding.AgentID,
			"session_id":           binding.SessionID,
			"conversation_id":      binding.ConversationID,
			"provider_instance_id": binding.ProviderInstanceID,
			"expires_at":           binding.ExpiresAt,
		})
	})))
}

func providerCitationLimit(l provider.ProviderLimits) int {
	if l.Validate() != nil {
		return provider.Discover(provider.CapabilityInput{}).Limits.MaxCitations
	}
	return l.MaxCitations
}

func supportsProviderSchema(supported []string, requested string) bool {
	for _, v := range supported {
		if v == strings.TrimSpace(requested) {
			return true
		}
	}
	return false
}
func readBody(r *http.Request) []byte {
	if r.Body == nil {
		return nil
	}
	b, _ := io.ReadAll(io.LimitReader(r.Body, 1<<20))
	return b
}
func boundedProviderMessage(err error) string {
	if err == nil {
		return ""
	}
	return "provider operation failed"
}
func providerErrorCategory(err error) provider.ErrorCategory {
	if err == nil {
		return provider.ErrorCategoryValidation
	}
	s := strings.ToLower(err.Error())
	switch {
	case strings.Contains(s, "resync required"):
		return provider.ErrorCategoryResyncRequired
	case strings.Contains(s, "scope mismatch") || strings.Contains(s, "forbidden"):
		return provider.ErrorCategoryScope
	case strings.Contains(s, "stale") || strings.Contains(s, "sequence"):
		return provider.ErrorCategoryStale
	case strings.Contains(s, "idempotency conflict") || errors.Is(err, memory.ErrIdempotencyConflict):
		return provider.ErrorCategoryConflict
	case strings.Contains(s, "retry") || strings.Contains(s, "in progress") || errors.Is(err, memory.ErrIdempotencyInProgress):
		return provider.ErrorCategoryRetryable
	case strings.Contains(s, "not configured") || strings.Contains(s, "unavailable"):
		return provider.ErrorCategoryDependency
	case strings.Contains(s, "lifecycle") || strings.Contains(s, "privileged"):
		return provider.ErrorCategoryLifecycle
	default:
		return provider.ErrorCategoryValidation
	}
}
func writeProviderError(w http.ResponseWriter, status int, category provider.ErrorCategory, code, msg string, retry bool) {
	writeJSON(w, status, map[string]any{"error": provider.ProviderError{Category: category, Code: code, Message: msg, Retryable: retry}})
}

func writeProviderCompatibilityError(w http.ResponseWriter, supported []string) {
	if len(supported) > 16 {
		supported = supported[:16]
	}
	writeJSON(w, http.StatusBadRequest, map[string]any{"error": provider.ProviderError{Category: provider.ErrorCategoryCompatibility, Code: "unsupported_schema", Message: "unsupported provider schema version", SupportedVersions: supported}})
}
