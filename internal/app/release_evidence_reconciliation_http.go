package app

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/FelixSeptem/stele/internal/auth"
	"github.com/FelixSeptem/stele/internal/retrieval"
)

func handleAdminReleaseEvidenceReconciliation(w http.ResponseWriter, r *http.Request, service ReleaseEvidenceReconciliationAdminService) {
	if service == nil {
		http.Error(w, "release evidence reconciliation is unavailable", http.StatusServiceUnavailable)
		return
	}
	scope, ok := auth.ScopeFromContext(r.Context())
	if !ok {
		http.Error(w, "exact scope is required", http.StatusBadRequest)
		return
	}
	switch r.Method {
	case http.MethodGet:
		limit := 100
		if raw := r.URL.Query().Get("limit"); raw != "" {
			parsed, err := strconv.Atoi(raw)
			if err != nil || parsed < 1 || parsed > 100 {
				http.Error(w, "limit must be between 1 and 100", http.StatusBadRequest)
				return
			}
			limit = parsed
		}
		items, err := service.ListReleaseEvidenceReconciliation(r.Context(), scope, limit)
		if err != nil {
			http.Error(w, "failed to read reconciliation state", http.StatusNotFound)
			return
		}
		writeReleaseEvidenceJSON(w, http.StatusOK, map[string]any{"items": items})
	case http.MethodPost:
		var trigger retrieval.ReconciliationTrigger
		if !decodeJSONBody(w, r, &trigger) {
			return
		}
		if err := trigger.Validate(); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		runID, err := service.TriggerReleaseEvidenceReconciliation(r.Context(), scope, trigger)
		if err != nil {
			http.Error(w, "failed to queue reconciliation", http.StatusUnprocessableEntity)
			return
		}
		writeReleaseEvidenceJSON(w, http.StatusAccepted, map[string]any{"run_id": runID, "status": "pending"})
	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

func writeReleaseEvidenceJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}
