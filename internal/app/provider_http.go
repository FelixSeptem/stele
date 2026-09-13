package app

import (
	"encoding/json"
	"github.com/FelixSeptem/stele/internal/auth"
	"github.com/FelixSeptem/stele/internal/memory"
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
		out, err := deps.ProviderAdapter.Ingest(r.Context(), b, req.Metadata, memory.IngestEventInput{EventType: req.EventType, Content: req.Content, Metadata: req.MetadataMap})
		if err != nil {
			writeProviderError(w, 400, "validation", "operation_failed", boundedProviderMessage(err), false)
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
			writeProviderError(w, 400, provider.ErrorCategoryCompatibility, "unsupported_schema", "unsupported provider schema version", false)
			return
		}
		out, err := deps.ProviderAdapter.SubmitIntent(r.Context(), b, req.Metadata, req.Intent)
		if err != nil {
			writeProviderError(w, 400, provider.ErrorCategoryValidation, "operation_failed", boundedProviderMessage(err), false)
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
			writeProviderError(w, 400, provider.ErrorCategoryCompatibility, "unsupported_schema", "unsupported provider schema version", false)
			return
		}
		out, meta, err := deps.ProviderAdapter.Search(r.Context(), b, req.Metadata, req.Input)
		if err != nil {
			writeProviderError(w, 400, provider.ErrorCategoryValidation, "operation_failed", boundedProviderMessage(err), false)
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"metadata": meta, "result": out, "citations": provider.ShapeSearchCitations(out)})
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
			writeProviderError(w, 400, provider.ErrorCategoryCompatibility, "unsupported_schema", "unsupported provider schema version", false)
			return
		}
		out, meta, err := deps.ProviderAdapter.AssembleContext(r.Context(), b, req.Metadata, req.Input)
		if err != nil {
			writeProviderError(w, 400, provider.ErrorCategoryValidation, "operation_failed", boundedProviderMessage(err), false)
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"metadata": meta, "result": out, "citations": provider.ShapeContextCitations(out)})
	})))
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
func writeProviderError(w http.ResponseWriter, status int, category provider.ErrorCategory, code, msg string, retry bool) {
	writeJSON(w, status, map[string]any{"error": provider.ProviderError{Category: category, Code: code, Message: msg, Retryable: retry}})
}
