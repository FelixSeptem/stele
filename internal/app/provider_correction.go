package app

import (
	"errors"
	"net/http"
	"strings"

	"github.com/FelixSeptem/stele/internal/auth"
	"github.com/FelixSeptem/stele/internal/memory"
	"github.com/FelixSeptem/stele/internal/provider"
)

func registerProviderCorrectionRoutes(mux *http.ServeMux, deps HTTPDependencies) {
	wrap := func(next http.Handler) http.Handler {
		return provider.RuntimeBindingMiddleware(deps.ProviderBindings, deps.PrincipalAuthorizer)(next)
	}
	mux.Handle("POST /v1/provider/corrections", wrap(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		binding, ok := provider.RuntimeBindingFromContext(r.Context())
		if !ok {
			writeProviderError(w, http.StatusForbidden, provider.ErrorCategoryScope, "forbidden", "runtime binding denied", false)
			return
		}
		req, err := provider.DecodeCorrectionRequest(readBody(r))
		if err != nil {
			writeProviderError(w, http.StatusBadRequest, provider.ErrorCategoryValidation, "invalid_request", "invalid correction request", false)
			return
		}
		if !supportsProviderSchema(deps.ProviderSchemaVersions, req.Metadata.SchemaVersion) {
			writeProviderCompatibilityError(w, deps.ProviderSchemaVersions)
			return
		}
		entitlements, ok := currentDisclosure(w, r, deps.PrincipalAuthorizer, binding.PrincipalID, binding.Scope)
		if !ok {
			return
		}
		if _, err = entitlements.Resolve("", auth.ResponseModeReferenceOnly); err != nil {
			writeProviderError(w, http.StatusForbidden, provider.ErrorCategoryScope, "disclosure_denied", "disclosure denied", false)
			return
		}
		if deps.MemoryIntentPolicy == nil {
			writeProviderError(w, http.StatusServiceUnavailable, provider.ErrorCategoryDependency, "provider_unavailable", "provider correction policy unavailable", true)
			return
		}
		policyInput := memory.MemoryIntentInput{
			Scope: binding.Scope, Type: memory.MemoryIntentUpdate, TargetMemoryID: req.Input.TargetMemoryID,
			TargetVersion: req.Input.ExpectedVersion, Content: req.Input.Content, Actor: binding.PrincipalID,
			Reason: req.Input.Reason, RequestID: req.Metadata.RequestID, OperationID: req.Metadata.OperationID,
			IdempotencyKey: req.Metadata.IdempotencyKey,
		}
		if err := policyInput.ValidateGoverned(); err != nil {
			writeProviderError(w, http.StatusBadRequest, provider.ErrorCategoryValidation, "invalid_request", "invalid correction request", false)
			return
		}
		policyDecision, err := deps.MemoryIntentPolicy.EvaluateMemoryIntent(r.Context(), policyInput)
		if err != nil {
			writeProviderError(w, http.StatusServiceUnavailable, provider.ErrorCategoryDependency, "provider_unavailable", "provider correction policy unavailable", true)
			return
		}
		req.PolicyVersion = strings.TrimSpace(policyDecision.PolicyVersion)
		if !policyDecision.Enabled || req.PolicyVersion == "" || len(req.PolicyVersion) > 128 {
			writeProviderError(w, http.StatusForbidden, provider.ErrorCategoryScope, "policy_disabled", "correction policy denied", false)
			return
		}
		if deps.ProviderCorrections == nil {
			writeProviderError(w, http.StatusServiceUnavailable, provider.ErrorCategoryDependency, "provider_unavailable", "provider correction unavailable", true)
			return
		}
		out, err := deps.ProviderCorrections.SubmitCorrection(r.Context(), binding, req)
		if err != nil {
			writeCorrectionError(w, err)
			return
		}
		if err := out.Validate(); err != nil {
			writeProviderError(w, http.StatusServiceUnavailable, provider.ErrorCategoryDependency, "provider_unavailable", "provider correction unavailable", true)
			return
		}
		writeJSON(w, http.StatusAccepted, map[string]any{"metadata": req.Metadata, "contract_version": provider.CorrectionContractVersion, "result": out})
	})))
}

func writeCorrectionError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, provider.ErrCorrectionUnavailable):
		writeProviderError(w, http.StatusGone, provider.ErrorCategoryLifecycle, "correction_unavailable", "correction unavailable", false)
	case errors.Is(err, provider.ErrCorrectionStale):
		writeProviderError(w, http.StatusConflict, provider.ErrorCategoryStale, "correction_stale", "correction stale", false)
	case errors.Is(err, provider.ErrCorrectionConflict):
		writeProviderError(w, http.StatusConflict, provider.ErrorCategoryConflict, "correction_conflict", "correction conflict", false)
	default:
		writeProviderError(w, http.StatusServiceUnavailable, provider.ErrorCategoryDependency, "provider_unavailable", "provider correction unavailable", true)
	}
}
