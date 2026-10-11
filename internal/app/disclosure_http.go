package app

import (
	"context"
	"errors"
	"github.com/FelixSeptem/stele/internal/auth"
	"github.com/FelixSeptem/stele/internal/memory"
	"github.com/FelixSeptem/stele/internal/provider"
	"net/http"
	"strings"
)

func handleGrantDisclosure(w http.ResponseWriter, r *http.Request, service PrincipalAdministrationService) {
	admin, ok := service.(interface {
		UpdateScopeGrantDisclosure(context.Context, memory.Scope, string, string, auth.DisclosureEntitlements, string, string) error
	})
	if !ok {
		writeProviderError(w, 503, "dependency", "disclosure_unavailable", "disclosure authority unavailable", true)
		return
	}
	data := readBody(r)
	obj, err := provider.CanonicalObject(data, []string{"disclosure", "actor", "reason"}, []string{"disclosure", "actor", "reason"})
	if err == nil {
		_, err = provider.CanonicalObject(obj["disclosure"], []string{"allowed_profiles", "default_profile", "source_attestation"}, []string{"allowed_profiles", "default_profile", "source_attestation"})
	}
	var req struct {
		Disclosure auth.DisclosureEntitlements `json:"disclosure"`
		Actor      string                      `json:"actor"`
		Reason     string                      `json:"reason"`
	}
	if err != nil || provider.DecodeStrict(data, &req) != nil || req.Disclosure.Validate() != nil || strings.TrimSpace(req.Actor) == "" || len(req.Actor) > 128 || strings.TrimSpace(req.Reason) == "" || len(req.Reason) > 512 {
		writeProviderError(w, 400, "validation", "invalid_request", "invalid request", false)
		return
	}
	scope, ok := auth.ScopeFromContext(r.Context())
	if !ok {
		writeProviderError(w, 403, "scope", "disclosure_denied", "disclosure denied", false)
		return
	}
	if err := admin.UpdateScopeGrantDisclosure(r.Context(), scope, r.PathValue("principal_id"), r.PathValue("grant_id"), req.Disclosure, req.Actor, req.Reason); err != nil {
		if errors.Is(err, auth.ErrDisclosureDenied) {
			writeProviderError(w, 403, "scope", "disclosure_denied", "disclosure denied", false)
		} else {
			writeProviderError(w, 503, "dependency", "disclosure_unavailable", "disclosure authority unavailable", true)
		}
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
