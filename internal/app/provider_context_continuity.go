package app

import (
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"github.com/FelixSeptem/stele/internal/provider"
)

func contextContinuityFailure(w http.ResponseWriter, err error) {
	code := "provider_unavailable"
	status := 503
	category := provider.ErrorCategoryDependency
	retry := true
	switch {
	case errors.Is(err, provider.ErrContinuityUnavailable):
		code = "continuity_reference_unavailable"
	case errors.Is(err, provider.ErrContinuityExpired):
		code = "continuity_reference_expired"
	case errors.Is(err, provider.ErrContinuityRequestChanged):
		code = "continuity_request_changed"
	case errors.Is(err, provider.ErrContinuityIncompatible):
		code = "continuity_incompatible"
	}
	if code != "provider_unavailable" {
		status = 409
		category = provider.ErrorCategoryConflict
		retry = false
		if code == "continuity_incompatible" {
			category = provider.ErrorCategoryCompatibility
		}
	}
	writeProviderError(w, status, category, code, "provider context continuity unavailable", retry)
}

// prepareContextContinuity runs after current disclosure and empty-envelope
// preflight, but before assembly. A digest-only read performs no receipt IO.
func prepareContextContinuity(w http.ResponseWriter, r *http.Request, deps HTTPDependencies, b provider.RuntimeBinding, req provider.ContextRequest, a *provider.ContextAdmission) (string, *provider.ContextReceipt, bool) {
	fingerprint, err := provider.ContextReadFingerprint(b, req.Input, a.PreflightResponse())
	if err != nil {
		writeProviderError(w, 400, provider.ErrorCategoryValidation, "invalid_request", "invalid request", false)
		return "", nil, false
	}
	var previous *provider.ContextReceipt
	if req.Input.Continuity != nil {
		if deps.ProviderContextReceipts == nil || deps.ProviderContextContinuity.Validate() != nil {
			contextContinuityFailure(w, nil)
			return "", nil, false
		}
		if req.Input.Continuity.Mode == "compare" {
			receipt, err := deps.ProviderContextReceipts.LookupContextReceipt(r.Context(), b, req.Input.Continuity.Reference)
			if err == nil {
				err = provider.ValidateContextReceipt(receipt, fingerprint)
			}
			if err != nil {
				contextContinuityFailure(w, err)
				return "", nil, false
			}
			previous = &receipt
		}
	}
	if deps.ProviderContextPolicy != nil {
		manifest, err := deps.ProviderContextPolicy(r.Context(), b)
		if err != nil {
			contextContinuityFailure(w, err)
			return "", nil, false
		}
		if err = a.SetPolicyFingerprint(provider.SelectionPolicyFingerprint(manifest)); err != nil {
			contextContinuityFailure(w, err)
			return "", nil, false
		}
	}
	return fingerprint, previous, true
}

func persistContextContinuity(w http.ResponseWriter, r *http.Request, deps HTTPDependencies, b provider.RuntimeBinding, req provider.ContextRequest, a *provider.ContextAdmission, fingerprint string, previous *provider.ContextReceipt, body []byte, finalize func() ([]byte, error)) ([]byte, bool) {
	if req.Input.Continuity == nil {
		return body, true
	}
	var response provider.DisclosedContextResponse
	if err := json.Unmarshal(body, &response); err != nil {
		contextContinuityFailure(w, err)
		return nil, false
	}
	result := response.Result.Continuity
	if previous != nil {
		result.ReadSemantics = "fresh_compare"
		result.Outcome = "changed"
		if previous.Digest == response.Result.ContextDigest {
			result.Outcome = "unchanged"
		}
	}
	var serializationErr error
	_, err := deps.ProviderContextReceipts.IssueContextReceipt(r.Context(), b, fingerprint, response.Result.ContextDigest, deps.ProviderContextContinuity, func(receipt provider.ContextReceipt) error {
		if err := provider.ValidateContextReceipt(receipt, fingerprint); err != nil {
			return err
		}
		if receipt.Digest != response.Result.ContextDigest || !provider.ValidContextReference(receipt.Reference) {
			return provider.ErrContinuityIncompatible
		}
		result.Reference = receipt.Reference
		result.ExpiresAt = receipt.ExpiresAt.UTC().Format(time.RFC3339)
		a.SetContinuity(result)
		body, serializationErr = finalize()
		return serializationErr
	})
	if err != nil {
		if serializationErr != nil {
			writeProviderError(w, 413, provider.ErrorCategoryValidation, "context_limit_exceeded", "context response exceeds allocation", false)
		} else {
			contextContinuityFailure(w, err)
		}
		return nil, false
	}
	return body, true
}
