package app

import (
	"net/http"
	"strings"

	"github.com/FelixSeptem/stele/internal/auth"
	"github.com/FelixSeptem/stele/internal/memory"
)

func handleAdminContextProjectionRebuild(w http.ResponseWriter, r *http.Request, service ContextProjectionAdminService) {
	if service == nil {
		http.Error(w, "context projection admin service is not configured", http.StatusServiceUnavailable)
		return
	}
	var request contextProjectionRebuildRequest
	if !decodeJSONBody(w, r, &request) {
		return
	}
	scope, ok := auth.ScopeFromContext(r.Context())
	if !ok {
		http.Error(w, "scope context is missing", http.StatusInternalServerError)
		return
	}
	projection, err := service.RebuildContextProjection(r.Context(), memory.ContextProjectionRebuildRequest{
		Scope: scope, Kind: request.Kind, Limit: request.Limit,
		SchemaVersion:   strings.TrimSpace(request.SchemaVersion),
		Policy:          memory.DefaultContextProjectionPolicy(strings.TrimSpace(request.PolicyVersion)),
		RendererVersion: strings.TrimSpace(request.RendererVersion),
	})
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	writeJSON(w, http.StatusCreated, projection)
}
