package app

import (
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/FelixSeptem/stele/internal/auth"
	"github.com/FelixSeptem/stele/internal/memory"
	"github.com/jackc/pgx/v5"
)

// proceduralVerificationEvidenceRequest intentionally has no scope field:
// evidence is always bound to the authenticated request scope by the server.
type proceduralVerificationEvidenceRequest struct {
	Kind          memory.ProceduralVerificationEvidenceKind       `json:"kind"`
	SourceKind    memory.ProceduralVerificationEvidenceSourceKind `json:"source_kind"`
	Polarity      memory.ProceduralVerificationEvidencePolarity   `json:"polarity,omitempty"`
	ID            string                                          `json:"id"`
	SourceVersion int64                                           `json:"source_version"`
	Corroborates  bool                                            `json:"corroborates,omitempty"`
}

type proceduralVerificationRecordRequest struct {
	MemoryID                 string                                   `json:"memory_id"`
	TargetVersion            int64                                    `json:"target_version"`
	ClaimFingerprint         string                                   `json:"claim_fingerprint"`
	ApplicabilityFingerprint string                                   `json:"applicability_fingerprint"`
	Disposition              memory.ProceduralVerificationDisposition `json:"disposition"`
	Evidence                 []proceduralVerificationEvidenceRequest  `json:"evidence"`
	Reason                   string                                   `json:"reason"`
	SourceWatermark          string                                   `json:"source_watermark"`
	CorrectedContent         string                                   `json:"corrected_content,omitempty"`
}

func handleAdminProceduralVerificationRecord(w http.ResponseWriter, r *http.Request, service ProceduralVerificationAdminAPI, policyVersion string) {
	if service == nil {
		http.Error(w, "procedural verification service is not configured", http.StatusServiceUnavailable)
		return
	}
	if strings.TrimSpace(policyVersion) == "" {
		http.Error(w, "procedural verification policy is not configured", http.StatusServiceUnavailable)
		return
	}
	var req proceduralVerificationRecordRequest
	if !decodeJSONBody(w, r, &req) {
		return
	}
	scope, ok := auth.ScopeFromContext(r.Context())
	if !ok {
		http.Error(w, "scope context is missing", http.StatusInternalServerError)
		return
	}
	evidence := make([]memory.ProceduralVerificationEvidence, 0, len(req.Evidence))
	for _, item := range req.Evidence {
		evidence = append(evidence, memory.ProceduralVerificationEvidence{
			Scope: scope, Kind: item.Kind, SourceKind: item.SourceKind, Polarity: item.Polarity,
			ID: item.ID, SourceVersion: item.SourceVersion, Corroborates: item.Corroborates,
		})
	}
	record := memory.ProceduralMemoryVerification{
		ID:                       newID(),
		Scope:                    scope,
		MemoryID:                 strings.TrimSpace(req.MemoryID),
		TargetVersion:            req.TargetVersion,
		ClaimFingerprint:         strings.TrimSpace(req.ClaimFingerprint),
		ApplicabilityFingerprint: strings.TrimSpace(req.ApplicabilityFingerprint),
		Disposition:              req.Disposition,
		Evidence:                 evidence,
		Actor:                    strings.TrimSpace(r.Header.Get("X-Stele-Actor")),
		Reason:                   req.Reason,
		PolicyVersion:            strings.TrimSpace(policyVersion),
		SourceWatermark:          req.SourceWatermark,
		CorrectedContent:         req.CorrectedContent,
		CreatedAt:                time.Now().UTC(),
	}
	stored, err := service.Record(r.Context(), record)
	if err != nil {
		writeAdminProceduralVerificationError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, proceduralVerificationRecordResponse(stored))
}

func handleAdminProceduralVerificationDetail(w http.ResponseWriter, r *http.Request, service ProceduralVerificationAdminAPI) {
	if service == nil {
		http.Error(w, "procedural verification service is not configured", http.StatusServiceUnavailable)
		return
	}
	scope, ok := auth.ScopeFromContext(r.Context())
	if !ok {
		http.Error(w, "scope context is missing", http.StatusInternalServerError)
		return
	}
	record, err := service.ReadProceduralMemoryVerification(r.Context(), scope, strings.TrimSpace(r.PathValue("verification_id")))
	if err != nil {
		writeAdminProceduralVerificationError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, proceduralVerificationRecordResponse(record))
}

func handleAdminProceduralVerificationHistory(w http.ResponseWriter, r *http.Request, service ProceduralVerificationAdminAPI) {
	if service == nil {
		http.Error(w, "procedural verification service is not configured", http.StatusServiceUnavailable)
		return
	}
	scope, ok := auth.ScopeFromContext(r.Context())
	if !ok {
		http.Error(w, "scope context is missing", http.StatusInternalServerError)
		return
	}
	memoryID := strings.TrimSpace(r.URL.Query().Get("memory_id"))
	targetVersion, err := strconv.ParseInt(strings.TrimSpace(r.URL.Query().Get("target_version")), 10, 64)
	if err != nil || memoryID == "" || targetVersion <= 0 {
		http.Error(w, "invalid procedural verification history request", http.StatusBadRequest)
		return
	}
	limit := 50
	if rawLimit := strings.TrimSpace(r.URL.Query().Get("limit")); rawLimit != "" {
		limit, err = strconv.Atoi(rawLimit)
		if err != nil || limit <= 0 || limit > 100 {
			http.Error(w, "invalid procedural verification history request", http.StatusBadRequest)
			return
		}
	}
	history, err := service.ListProceduralMemoryVerificationHistory(r.Context(), scope, memoryID, targetVersion, limit)
	if err != nil {
		writeAdminProceduralVerificationError(w, err)
		return
	}
	items := make([]map[string]any, 0, len(history))
	for _, record := range history {
		items = append(items, proceduralVerificationRecordResponse(record))
	}
	writeJSON(w, http.StatusOK, map[string]any{"verifications": items})
}

type proceduralVerificationRevalidationRequest struct {
	Reason string `json:"reason"`
}

func handleAdminProceduralVerificationAction(w http.ResponseWriter, r *http.Request, service ProceduralVerificationAdminAPI) {
	parts := strings.Split(strings.TrimSpace(r.PathValue("verification_action")), ":")
	if len(parts) != 2 || strings.TrimSpace(parts[0]) == "" {
		http.Error(w, "invalid procedural verification action target", http.StatusBadRequest)
		return
	}
	r.SetPathValue("verification_id", strings.TrimSpace(parts[0]))
	switch parts[1] {
	case "revalidate":
		handleAdminProceduralVerificationRevalidate(w, r, service)
	case "correct":
		handleAdminProceduralVerificationCorrection(w, r, service)
	default:
		http.Error(w, "invalid procedural verification action target", http.StatusBadRequest)
	}
}

func handleAdminProceduralVerificationRevalidate(w http.ResponseWriter, r *http.Request, service ProceduralVerificationAdminAPI) {
	if service == nil {
		http.Error(w, "procedural verification service is not configured", http.StatusServiceUnavailable)
		return
	}
	var req proceduralVerificationRevalidationRequest
	if !decodeJSONBody(w, r, &req) {
		return
	}
	scope, ok := auth.ScopeFromContext(r.Context())
	if !ok {
		http.Error(w, "scope context is missing", http.StatusInternalServerError)
		return
	}
	run, err := service.Request(r.Context(), memory.ProceduralRevalidationRequest{
		Scope: scope, VerificationID: strings.TrimSpace(r.PathValue("verification_id")), Trigger: memory.ProceduralRevalidationTriggerAdministrator,
		Actor: strings.TrimSpace(r.Header.Get("X-Stele-Actor")), Reason: req.Reason,
	})
	if err != nil {
		writeAdminProceduralVerificationError(w, err)
		return
	}
	writeJSON(w, http.StatusAccepted, map[string]any{
		"id": run.ID, "verification_id": run.VerificationID, "status": run.Status, "trigger": run.Trigger,
		"attempt": run.Attempt, "max_attempts": run.MaxAttempts, "created_at": run.CreatedAt,
	})
}

type proceduralVerificationCorrectionRequest struct {
	Reason         string `json:"reason"`
	OperationID    string `json:"operation_id"`
	IdempotencyKey string `json:"idempotency_key"`
}

func handleAdminProceduralVerificationCorrection(w http.ResponseWriter, r *http.Request, service ProceduralVerificationAdminAPI) {
	if service == nil {
		http.Error(w, "procedural verification service is not configured", http.StatusServiceUnavailable)
		return
	}
	var req proceduralVerificationCorrectionRequest
	if !decodeJSONBody(w, r, &req) {
		return
	}
	scope, ok := auth.ScopeFromContext(r.Context())
	if !ok {
		http.Error(w, "scope context is missing", http.StatusInternalServerError)
		return
	}
	intent, err := service.Submit(r.Context(), memory.VerificationCorrectionRequest{
		Scope: scope, VerificationID: strings.TrimSpace(r.PathValue("verification_id")), Actor: strings.TrimSpace(r.Header.Get("X-Stele-Actor")),
		Reason: req.Reason, RequestID: strings.TrimSpace(r.Header.Get("X-Request-ID")), OperationID: req.OperationID, IdempotencyKey: req.IdempotencyKey,
	})
	if err != nil {
		writeAdminProceduralVerificationError(w, err)
		return
	}
	writeJSON(w, http.StatusAccepted, map[string]any{"intent_id": intent.ID, "status": intent.Status})
}

func handleAdminProceduralRevalidationRunDetail(w http.ResponseWriter, r *http.Request, service ProceduralVerificationAdminAPI) {
	if service == nil {
		http.Error(w, "procedural verification service is not configured", http.StatusServiceUnavailable)
		return
	}
	scope, ok := auth.ScopeFromContext(r.Context())
	if !ok {
		http.Error(w, "scope context is missing", http.StatusInternalServerError)
		return
	}
	run, err := service.ReadProceduralRevalidationRun(r.Context(), scope, strings.TrimSpace(r.PathValue("run_id")))
	if err != nil {
		writeAdminProceduralVerificationError(w, err)
		return
	}
	// Lease owner and work identity are operational internals; they are neither
	// required for an audit decision nor safe to disclose in API responses.
	writeJSON(w, http.StatusOK, map[string]any{
		"id": run.ID, "verification_id": run.VerificationID, "memory_id": run.MemoryID,
		"target_version": run.TargetVersion, "source_watermark": run.SourceWatermark,
		"trigger": run.Trigger, "status": run.Status, "attempt": run.Attempt,
		"max_attempts": run.MaxAttempts, "terminal_verification_id": run.TerminalVerificationID,
		"failure_category": run.FailureCategory, "created_at": run.CreatedAt, "updated_at": run.UpdatedAt,
	})
}

// proceduralVerificationRecordResponse is a bounded assertion receipt. The
// raw evidence list remains available only through a later, explicitly
// privileged lifecycle-filtered detail implementation.
func proceduralVerificationRecordResponse(record memory.ProceduralMemoryVerification) map[string]any {
	return map[string]any{
		"id": record.ID, "memory_id": record.MemoryID, "target_version": record.TargetVersion,
		"disposition": record.Disposition, "policy_version": record.PolicyVersion,
		"source_watermark": record.SourceWatermark, "created_at": record.CreatedAt,
	}
}

func writeAdminProceduralVerificationError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, memory.ErrNotFound), errors.Is(err, pgx.ErrNoRows):
		http.Error(w, "procedural verification not found", http.StatusNotFound)
	case strings.Contains(err.Error(), "conflict"):
		http.Error(w, "procedural verification conflict", http.StatusConflict)
	case strings.Contains(err.Error(), "disabled"):
		http.Error(w, "procedural verification is unavailable", http.StatusNotFound)
	case strings.Contains(err.Error(), "invalid"), strings.Contains(err.Error(), "required"), strings.Contains(err.Error(), "must be"), strings.Contains(err.Error(), "exceeds"):
		http.Error(w, "invalid procedural verification request", http.StatusBadRequest)
	default:
		http.Error(w, "failed to record procedural verification", http.StatusInternalServerError)
	}
}
