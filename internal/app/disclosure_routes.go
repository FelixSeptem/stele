package app

import (
	"errors"
	"github.com/FelixSeptem/stele/internal/auth"
	"github.com/FelixSeptem/stele/internal/memory"
	"net/http"
)

// All registered public data routes are denied unless listed here. This keeps
// future native/Provider additions closed until they implement PC2 projections.
func disclosureRouteGate(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		p, ok := auth.PrincipalFromContext(r.Context())
		if ok && p.Role != auth.PrincipalRoleAdmin {
			allowed := r.Method == "POST" && (r.URL.Path == "/v1/provider/runtime" || r.URL.Path == "/v1/provider/context" || r.URL.Path == "/v1/provider/events" || r.URL.Path == "/v1/provider/intents" || r.URL.Path == "/v1/events")
			allowed = allowed || (r.Method == "GET" && (r.URL.Path == "/v1/provider/status" || r.URL.Path == "/v1/provider/capabilities" || r.URL.Path == "/v1/reasoning/capabilities"))
			if !allowed {
				writeProviderError(w, 403, "scope", "disclosure_denied", "disclosure denied", false)
				return
			}
		}
		next.ServeHTTP(w, r)
	})
}

func currentDisclosure(w http.ResponseWriter, r *http.Request, authorizer auth.PrincipalAuthorizer, principalID string, scope memory.Scope) (auth.DisclosureEntitlements, bool) {
	a, ok := authorizer.(auth.ScopeDisclosureAuthorizer)
	if !ok {
		writeProviderError(w, 503, "dependency", "disclosure_unavailable", "disclosure authority unavailable", true)
		return auth.DisclosureEntitlements{}, false
	}
	e, err := a.AuthorizeScopeDisclosure(r.Context(), principalID, scope)
	if err != nil || e.Validate() != nil {
		if errors.Is(err, auth.ErrDisclosureDenied) || err == nil {
			writeProviderError(w, 403, "scope", "disclosure_denied", "disclosure denied", false)
		} else {
			writeProviderError(w, 503, "dependency", "disclosure_unavailable", "disclosure authority unavailable", true)
		}
		return auth.DisclosureEntitlements{}, false
	}
	return e, true
}
