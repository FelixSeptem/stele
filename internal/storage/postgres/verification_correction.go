package postgres

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/FelixSeptem/stele/internal/memory"
	"github.com/google/uuid"
)

// SubmitVerificationAttributedIntent is the repository-owned handoff from a
// server-resolved refuted verification to the ordinary governed update path.
// It deliberately rejects a caller-supplied attribution that does not name
// the same immutable target as the update before it can begin durable work.
func (r *Repository) SubmitVerificationAttributedIntent(ctx context.Context, input memory.MemoryIntentInput, attribution memory.VerificationAttribution) (memory.MemoryIntentRecord, error) {
	if err := input.ValidateGoverned(); err != nil {
		return memory.MemoryIntentRecord{}, err
	}
	if input.Type != memory.MemoryIntentUpdate || attribution.Validate() != nil ||
		input.TargetMemoryID != attribution.TargetMemoryID || input.TargetVersion != attribution.TargetVersion {
		return memory.MemoryIntentRecord{}, memory.ErrNotFound
	}
	if r == nil || r.tx == nil {
		return memory.MemoryIntentRecord{}, memory.ErrNotFound
	}
	fingerprint, err := memory.VerificationIntentFingerprint(input, attribution)
	if err != nil {
		return memory.MemoryIntentRecord{}, err
	}
	tx, err := r.beginTx(ctx)
	if err != nil {
		return memory.MemoryIntentRecord{}, fmt.Errorf("begin verification correction acceptance: %w", err)
	}
	defer tx.Rollback(ctx)
	txCtx := withRepositoryTx(ctx, tx)

	verification, err := r.ReadProceduralMemoryVerification(txCtx, input.Scope, attribution.VerificationID)
	if err != nil || verification.Disposition != memory.ProceduralVerificationDispositionRefuted ||
		strings.TrimSpace(verification.CorrectedContent) == "" ||
		verification.MemoryID != input.TargetMemoryID || verification.TargetVersion != input.TargetVersion ||
		verification.SourceWatermark != attribution.SourceWatermark || verification.PolicyVersion != attribution.PolicyVersion {
		return memory.MemoryIntentRecord{}, memory.ErrNotFound
	}
	if err := r.ValidateProceduralVerificationTarget(txCtx, verification); err != nil {
		return memory.MemoryIntentRecord{}, memory.ErrNotFound
	}
	for _, evidence := range verification.Evidence {
		if evidence.Scope == (memory.Scope{}) {
			evidence.Scope = verification.Scope.Normalized()
		}
		if err := r.ValidateProceduralVerificationEvidence(txCtx, input.Scope, evidence); err != nil {
			return memory.MemoryIntentRecord{}, memory.ErrNotFound
		}
	}

	path, err := memory.NormalizeMemoryPath(input.MemoryPath)
	if err != nil {
		return memory.MemoryIntentRecord{}, err
	}
	now := time.Now().UTC()
	record := memory.MemoryIntentRecord{
		ID: uuid.NewString(), Scope: input.Scope.Normalized(), MemoryPath: path,
		Type: memory.MemoryIntentUpdate, TargetMemoryID: input.TargetMemoryID, TargetVersion: input.TargetVersion,
		Content: input.Content, Actor: input.Actor, Reason: input.Reason, Provenance: input.Provenance,
		RequestID: input.RequestID, OperationID: input.OperationID, IdempotencyKey: input.IdempotencyKey,
		Evidence: input.Evidence, RequestFingerprint: fingerprint, PolicyVersion: attribution.PolicyVersion,
		Verification: &attribution, Status: memory.MemoryIntentStatusAccepted, CreatedAt: now,
	}
	created, err := r.AppendMemoryIntent(txCtx, record)
	if err != nil {
		return memory.MemoryIntentRecord{}, err
	}
	if created.Status == memory.MemoryIntentStatusReplayed {
		if err := r.verifyReplayVerificationAttribution(ctx, tx, created, attribution); err != nil {
			return memory.MemoryIntentRecord{}, err
		}
		created.Verification = &attribution
		if err := tx.Commit(ctx); err != nil {
			return memory.MemoryIntentRecord{}, fmt.Errorf("commit verification correction replay: %w", err)
		}
		return created, nil
	}
	if _, err = tx.Exec(ctx, `INSERT INTO memory_intent_verification_attributions
(intent_id,verification_id,tenant,project,namespace,target_memory_id,target_version,source_watermark,policy_version)
VALUES($1::uuid,$2::uuid,$3,$4,$5,$6::uuid,$7,$8,$9)`, created.ID, attribution.VerificationID,
		created.Scope.Tenant, created.Scope.Project, created.Scope.Namespace, attribution.TargetMemoryID,
		attribution.TargetVersion, attribution.SourceWatermark, attribution.PolicyVersion); err != nil {
		return memory.MemoryIntentRecord{}, fmt.Errorf("append verification intent attribution: %w", err)
	}
	if err = r.AppendMemoryIntentTransition(txCtx, memory.MemoryIntentTransition{
		IntentID: created.ID, Scope: created.Scope, Sequence: 1, To: memory.MemoryIntentStatusAccepted,
		Actor: created.Actor, Reason: created.Reason, DiagnosticCategory: memory.MemoryIntentDiagnosticAccepted,
		OccurredAt: created.CreatedAt,
	}); err != nil {
		return memory.MemoryIntentRecord{}, fmt.Errorf("record initial verification intent transition: %w", err)
	}
	jobID, err := r.insertMemoryIntentRiverJob(ctx, tx, created.ID)
	if err != nil {
		return memory.MemoryIntentRecord{}, err
	}
	if err := r.recordMemoryIntentRiverJob(ctx, tx, created, jobID); err != nil {
		return memory.MemoryIntentRecord{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return memory.MemoryIntentRecord{}, fmt.Errorf("commit verification correction acceptance: %w", err)
	}
	created.Verification = &attribution
	return created, nil
}

func (r *Repository) verifyReplayVerificationAttribution(ctx context.Context, tx queryRower, record memory.MemoryIntentRecord, attribution memory.VerificationAttribution) error {
	var matched bool
	err := tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM memory_intent_verification_attributions
WHERE intent_id=$1::uuid AND tenant=$2 AND project=$3 AND namespace=$4 AND verification_id=$5::uuid
  AND target_memory_id=$6::uuid AND target_version=$7 AND source_watermark=$8 AND policy_version=$9)`,
		record.ID, record.Scope.Tenant, record.Scope.Project, record.Scope.Namespace, attribution.VerificationID,
		attribution.TargetMemoryID, attribution.TargetVersion, attribution.SourceWatermark, attribution.PolicyVersion,
	).Scan(&matched)
	if err != nil || !matched {
		return memory.ErrIdempotencyConflict
	}
	return nil
}

var _ memory.VerificationAttributedIntentSubmitter = (*Repository)(nil)
