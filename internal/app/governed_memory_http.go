package app

import (
	"net/http"
	"strings"

	"github.com/FelixSeptem/stele/internal/auth"
	"github.com/FelixSeptem/stele/internal/memory"
)

type memoryIntentRequest struct {
	Type           memory.MemoryIntentType `json:"type"`
	TargetMemoryID string                  `json:"target_memory_id,omitempty"`
	TargetVersion  int64                   `json:"target_version,omitempty"`
	Content        string                  `json:"content,omitempty"`
	Reason         string                  `json:"reason"`
	Provenance     map[string]any          `json:"provenance,omitempty"`
	OperationID    string                  `json:"operation_id"`
	IdempotencyKey string                  `json:"idempotency_key"`
}

func handleMemoryIntentCreate(w http.ResponseWriter, r *http.Request, service MemoryIntentAPI) {
	if service == nil {
		http.Error(w, "memory intent service is not configured", http.StatusServiceUnavailable)
		return
	}
	var req memoryIntentRequest
	if !decodeJSONBody(w, r, &req) {
		return
	}
	scope, ok := auth.ScopeFromContext(r.Context())
	if !ok {
		http.Error(w, "scope context is missing", http.StatusInternalServerError)
		return
	}
	input := memory.MemoryIntentInput{
		Scope: scope, Type: req.Type, TargetMemoryID: strings.TrimSpace(req.TargetMemoryID), TargetVersion: req.TargetVersion,
		Content: req.Content, Actor: strings.TrimSpace(r.Header.Get("X-Stele-Actor")), Reason: req.Reason,
		Provenance: req.Provenance, RequestID: strings.TrimSpace(r.Header.Get("X-Request-ID")), OperationID: req.OperationID,
		IdempotencyKey: req.IdempotencyKey,
	}
	if err := input.Validate(); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	record, err := service.Submit(r.Context(), input)
	if err != nil {
		writeGovernedMemoryHTTPError(w, err, "failed to submit memory intent")
		return
	}
	writeJSON(w, http.StatusAccepted, record)
}

type reflectionReviewRequest struct {
	CandidateID   string                          `json:"candidate_id"`
	Decision      memory.ReflectionReviewDecision `json:"decision"`
	Reason        string                          `json:"reason"`
	PolicyVersion string                          `json:"policy_version"`
}

func handleAdminReflectionReview(w http.ResponseWriter, r *http.Request, service ReflectionReviewAPI) {
	if service == nil {
		http.Error(w, "reflection review service is not configured", http.StatusServiceUnavailable)
		return
	}
	var req reflectionReviewRequest
	if !decodeJSONBody(w, r, &req) {
		return
	}
	scope, ok := auth.ScopeFromContext(r.Context())
	if !ok {
		http.Error(w, "scope context is missing", http.StatusInternalServerError)
		return
	}
	record, err := service.Decide(r.Context(), memory.ReflectionReviewInput{
		Scope: scope, CandidateID: req.CandidateID, Decision: req.Decision, Reviewer: strings.TrimSpace(r.Header.Get("X-Stele-Actor")), Reason: req.Reason, PolicyVersion: req.PolicyVersion,
	})
	if err != nil {
		writeGovernedMemoryHTTPError(w, err, "failed to record reflection review")
		return
	}
	writeJSON(w, http.StatusCreated, record)
}

func handleAdminMemoryIntentDetail(w http.ResponseWriter, r *http.Request, reader MemoryIntentReader) {
	if reader == nil {
		http.Error(w, "memory intent reader is not configured", http.StatusServiceUnavailable)
		return
	}
	scope, ok := auth.ScopeFromContext(r.Context())
	if !ok {
		http.Error(w, "scope context is missing", http.StatusInternalServerError)
		return
	}
	record, err := reader.ReadMemoryIntent(r.Context(), scope, r.PathValue("intent_id"))
	if err != nil {
		writeGovernedMemoryHTTPError(w, err, "failed to read memory intent")
		return
	}
	writeJSON(w, http.StatusOK, record)
}

func handleAdminReflectionRunDetail(w http.ResponseWriter, r *http.Request, reader ReflectionRunReader) {
	if reader == nil {
		http.Error(w, "reflection run reader is not configured", http.StatusServiceUnavailable)
		return
	}
	scope, ok := auth.ScopeFromContext(r.Context())
	if !ok {
		http.Error(w, "scope context is missing", http.StatusInternalServerError)
		return
	}
	run, err := reader.ReadReflectionRun(r.Context(), scope, r.PathValue("run_id"))
	if err != nil {
		writeGovernedMemoryHTTPError(w, err, "failed to read reflection run")
		return
	}
	writeJSON(w, http.StatusOK, run)
}

func handleAdminCompactionEvidenceDetail(w http.ResponseWriter, r *http.Request, reader CompactionEvidenceReader) {
	if reader == nil {
		http.Error(w, "compaction evidence reader is not configured", http.StatusServiceUnavailable)
		return
	}
	scope, ok := auth.ScopeFromContext(r.Context())
	if !ok {
		http.Error(w, "scope context is missing", http.StatusInternalServerError)
		return
	}
	evidence, err := reader.ReadCompactionEvidence(r.Context(), scope, r.PathValue("evidence_id"))
	if err != nil {
		writeGovernedMemoryHTTPError(w, err, "failed to read compaction evidence")
		return
	}
	writeJSON(w, http.StatusOK, evidence)
}

func writeGovernedMemoryHTTPError(w http.ResponseWriter, err error, fallback string) {
	message := err.Error()
	if strings.Contains(message, "required") || strings.Contains(message, "invalid") || strings.Contains(message, "must be") || strings.Contains(message, "conflict") {
		http.Error(w, message, http.StatusBadRequest)
		return
	}
	http.Error(w, fallback, http.StatusInternalServerError)
}
