package app

import (
	"encoding/json"
	"errors"
	"github.com/FelixSeptem/stele/internal/auth"
	"github.com/FelixSeptem/stele/internal/memory"
	"github.com/FelixSeptem/stele/internal/policy"
	"github.com/FelixSeptem/stele/internal/provider"
	"github.com/FelixSeptem/stele/internal/retrieval"
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
		b, err := deps.ProviderInitializer.Initialize(r.Context(), principal, req)
		if err != nil {
			writeProviderError(w, http.StatusForbidden, "scope", "forbidden", "runtime initialization denied", false)
			return
		}
		writeJSON(w, http.StatusCreated, b)
	}))
	mux.Handle("POST /v1/provider/runtime", runtimeInit)
	if deps.ProviderAdapter != nil {
		registerProviderOperationRoutes(mux, deps)
	}
}

func registerProviderOperationRoutes(mux *http.ServeMux, deps HTTPDependencies) {
	wrap := func(next http.Handler) http.Handler {
		return provider.RuntimeBindingMiddleware(deps.ProviderBindings, deps.PrincipalAuthorizer)(next)
	}
	mux.Handle("POST /v1/provider/events", wrap(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := provider.RuntimeBindingFromContext(r.Context())
		var req struct {
			Metadata    provider.OperationMetadata `json:"metadata"`
			EventType   string                     `json:"event_type"`
			Content     string                     `json:"content"`
			MetadataMap map[string]any             `json:"metadata_map"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeProviderError(w, 400, "validation", "invalid_request", "invalid request", false)
			return
		}
		if !supportsProviderSchema(deps.ProviderSchemaVersions, req.Metadata.SchemaVersion) {
			writeProviderCompatibilityError(w, deps.ProviderSchemaVersions)
			return
		}
		out, err := deps.ProviderAdapter.Ingest(r.Context(), b, req.Metadata, memory.IngestEventInput{EventType: req.EventType, Content: req.Content, Metadata: req.MetadataMap})
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
		out, err := deps.ProviderAdapter.SubmitIntent(r.Context(), b, req.Metadata, req.Intent)
		if err != nil {
			writeProviderError(w, 400, providerErrorCategory(err), "operation_failed", boundedProviderMessage(err), providerErrorCategory(err) == provider.ErrorCategoryRetryable)
			return
		}
		writeJSON(w, http.StatusAccepted, map[string]any{"metadata": req.Metadata, "result": out, "citations": provider.ShapeIntentCitation(out)})
	})))
	mux.Handle("POST /v1/provider/retrieve", wrap(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := provider.RuntimeBindingFromContext(r.Context())
		var req struct {
			Metadata provider.OperationMetadata `json:"metadata"`
			Input    retrieval.SearchInput      `json:"input"`
		}
		if err := provider.DecodeStrict(readBody(r), &req); err != nil {
			writeProviderError(w, 400, provider.ErrorCategoryValidation, "invalid_request", "invalid request", false)
			return
		}
		if !supportsProviderSchema(deps.ProviderSchemaVersions, req.Metadata.SchemaVersion) {
			writeProviderCompatibilityError(w, deps.ProviderSchemaVersions)
			return
		}
		out, meta, err := deps.ProviderAdapter.Search(r.Context(), b, req.Metadata, req.Input)
		if err != nil {
			writeProviderError(w, 400, providerErrorCategory(err), "operation_failed", boundedProviderMessage(err), providerErrorCategory(err) == provider.ErrorCategoryRetryable)
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"metadata": meta, "result": out, "citations": provider.ShapeSearchCitationsWithLimit(out, providerCitationLimit(deps.ProviderLimits))})
	})))
	mux.Handle("POST /v1/provider/context", wrap(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := provider.RuntimeBindingFromContext(r.Context())
		var req struct {
			Metadata provider.OperationMetadata     `json:"metadata"`
			Input    retrieval.AssembleContextInput `json:"input"`
		}
		if err := provider.DecodeStrict(readBody(r), &req); err != nil {
			writeProviderError(w, 400, provider.ErrorCategoryValidation, "invalid_request", "invalid request", false)
			return
		}
		if !supportsProviderSchema(deps.ProviderSchemaVersions, req.Metadata.SchemaVersion) {
			writeProviderCompatibilityError(w, deps.ProviderSchemaVersions)
			return
		}
		out, meta, err := deps.ProviderAdapter.AssembleContext(r.Context(), b, req.Metadata, req.Input)
		if err != nil {
			writeProviderError(w, 400, providerErrorCategory(err), "operation_failed", boundedProviderMessage(err), providerErrorCategory(err) == provider.ErrorCategoryRetryable)
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"metadata": meta, "result": out, "citations": provider.ShapeContextCitationsWithLimit(out, providerCitationLimit(deps.ProviderLimits))})
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
	s := strings.TrimSpace(err.Error())
	if len(s) > 256 {
		s = s[:256]
	}
	return s
}
func providerErrorCategory(err error) provider.ErrorCategory {
	if err == nil {
		return provider.ErrorCategoryValidation
	}
	s := strings.ToLower(err.Error())
	switch {
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
