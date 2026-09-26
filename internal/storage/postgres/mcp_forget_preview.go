package postgres

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/FelixSeptem/stele/internal/mcp"
	"github.com/jackc/pgx/v5"
)

func (r *Repository) SaveForgetPreview(ctx context.Context, preview mcp.ForgetPreviewRecord) error {
	if err := preview.Scope.Validate(); err != nil {
		return err
	}
	if preview.ID == "" || preview.Principal == "" || preview.ExpiresAt.IsZero() {
		return fmt.Errorf("forget preview is invalid")
	}
	ids, err := json.Marshal(preview.MemoryIDs)
	if err != nil {
		return fmt.Errorf("encode forget preview IDs: %w", err)
	}
	if _, err := r.db.Exec(ctx, `WITH expired AS (SELECT preview_id FROM mcp_forget_previews WHERE expires_at <= NOW() ORDER BY expires_at LIMIT 1000) DELETE FROM mcp_forget_previews p USING expired e WHERE p.preview_id=e.preview_id`); err != nil {
		return fmt.Errorf("prune expired forget previews: %w", err)
	}
	_, err = r.db.Exec(ctx, `INSERT INTO mcp_forget_previews (preview_id,principal_id,tenant,project,namespace,memory_ids,expires_at) VALUES ($1,$2,$3,$4,$5,$6,$7) ON CONFLICT (preview_id) DO UPDATE SET principal_id=EXCLUDED.principal_id,tenant=EXCLUDED.tenant,project=EXCLUDED.project,namespace=EXCLUDED.namespace,memory_ids=EXCLUDED.memory_ids,expires_at=EXCLUDED.expires_at`, preview.ID, preview.Principal, preview.Scope.Tenant, preview.Scope.Project, preview.Scope.Namespace, ids, preview.ExpiresAt)
	if err != nil {
		return fmt.Errorf("save forget preview: %w", err)
	}
	return nil
}

func (r *Repository) LoadForgetPreview(ctx context.Context, previewID string) (mcp.ForgetPreviewRecord, error) {
	var preview mcp.ForgetPreviewRecord
	var rawIDs []byte
	err := r.db.QueryRow(ctx, `SELECT preview_id,principal_id,tenant,project,namespace,memory_ids,expires_at FROM mcp_forget_previews WHERE preview_id=$1`, previewID).Scan(&preview.ID, &preview.Principal, &preview.Scope.Tenant, &preview.Scope.Project, &preview.Scope.Namespace, &rawIDs, &preview.ExpiresAt)
	if err != nil {
		if err == pgx.ErrNoRows {
			return mcp.ForgetPreviewRecord{}, fmt.Errorf("forget preview not found")
		}
		return mcp.ForgetPreviewRecord{}, fmt.Errorf("load forget preview: %w", err)
	}
	if err := json.Unmarshal(rawIDs, &preview.MemoryIDs); err != nil {
		return mcp.ForgetPreviewRecord{}, fmt.Errorf("decode forget preview IDs: %w", err)
	}
	return preview, nil
}

var _ mcp.ForgetPreviewStore = (*Repository)(nil)

func (r *Repository) ClaimForgetApply(ctx context.Context, claim mcp.ForgetApplyClaim) (mcp.ForgetApplyClaimResult, error) {
	if err := claim.Scope.Validate(); err != nil {
		return mcp.ForgetApplyClaimResult{}, err
	}
	var status, fingerprint, claimID string
	var outcome []byte
	err := r.db.QueryRow(ctx, `INSERT INTO mcp_forget_apply_operations (tenant,project,namespace,principal_id,idempotency_key,request_fingerprint,claim_id,lease_until,status) VALUES ($1,$2,$3,$4,$5,$6,$7,NOW()+INTERVAL '1 minute','pending') ON CONFLICT (tenant,project,namespace,principal_id,idempotency_key) DO UPDATE SET claim_id=EXCLUDED.claim_id,lease_until=EXCLUDED.lease_until WHERE mcp_forget_apply_operations.request_fingerprint=EXCLUDED.request_fingerprint AND mcp_forget_apply_operations.status='pending' AND mcp_forget_apply_operations.lease_until <= NOW() RETURNING status,request_fingerprint,claim_id,outcome`, claim.Scope.Tenant, claim.Scope.Project, claim.Scope.Namespace, claim.PrincipalID, claim.IdempotencyKey, claim.RequestFingerprint, claim.ClaimID).Scan(&status, &fingerprint, &claimID, &outcome)
	if err != nil {
		if !errors.Is(err, pgx.ErrNoRows) {
			return mcp.ForgetApplyClaimResult{}, fmt.Errorf("claim MCP forget apply: %w", err)
		}
		err = r.db.QueryRow(ctx, `SELECT status,request_fingerprint,claim_id,outcome FROM mcp_forget_apply_operations WHERE tenant=$1 AND project=$2 AND namespace=$3 AND principal_id=$4 AND idempotency_key=$5`, claim.Scope.Tenant, claim.Scope.Project, claim.Scope.Namespace, claim.PrincipalID, claim.IdempotencyKey).Scan(&status, &fingerprint, &claimID, &outcome)
		if err != nil {
			return mcp.ForgetApplyClaimResult{}, fmt.Errorf("read MCP forget apply claim: %w", err)
		}
	}
	if fingerprint != claim.RequestFingerprint {
		return mcp.ForgetApplyClaimResult{Disposition: mcp.ForgetApplyConflict}, nil
	}
	if status == "completed" {
		var response mcp.ForgetApplyResponse
		if err := json.Unmarshal(outcome, &response); err != nil {
			return mcp.ForgetApplyClaimResult{}, fmt.Errorf("decode MCP forget apply outcome: %w", err)
		}
		return mcp.ForgetApplyClaimResult{Disposition: mcp.ForgetApplyReplayed, Response: response}, nil
	}
	if claimID == claim.ClaimID {
		return mcp.ForgetApplyClaimResult{Disposition: mcp.ForgetApplyClaimed}, nil
	}
	return mcp.ForgetApplyClaimResult{Disposition: mcp.ForgetApplyInProgress}, nil
}

func (r *Repository) CompleteForgetApply(ctx context.Context, claim mcp.ForgetApplyClaim, response mcp.ForgetApplyResponse) error {
	payload, err := json.Marshal(response)
	if err != nil {
		return fmt.Errorf("encode MCP forget apply outcome: %w", err)
	}
	tag, err := r.db.Exec(ctx, `UPDATE mcp_forget_apply_operations SET status='completed',outcome=$7,completed_at=NOW() WHERE tenant=$1 AND project=$2 AND namespace=$3 AND principal_id=$4 AND idempotency_key=$5 AND claim_id=$6 AND request_fingerprint=$8 AND status='pending'`, claim.Scope.Tenant, claim.Scope.Project, claim.Scope.Namespace, claim.PrincipalID, claim.IdempotencyKey, claim.ClaimID, payload, claim.RequestFingerprint)
	if err != nil {
		return fmt.Errorf("complete MCP forget apply: %w", err)
	}
	if tag.RowsAffected() != 1 {
		return fmt.Errorf("MCP forget apply claim unavailable")
	}
	return nil
}

func (r *Repository) ReleaseForgetApply(ctx context.Context, claim mcp.ForgetApplyClaim) error {
	_, err := r.db.Exec(ctx, `DELETE FROM mcp_forget_apply_operations WHERE tenant=$1 AND project=$2 AND namespace=$3 AND principal_id=$4 AND idempotency_key=$5 AND claim_id=$6 AND request_fingerprint=$7 AND status='pending'`, claim.Scope.Tenant, claim.Scope.Project, claim.Scope.Namespace, claim.PrincipalID, claim.IdempotencyKey, claim.ClaimID, claim.RequestFingerprint)
	if err != nil {
		return fmt.Errorf("release MCP forget apply claim: %w", err)
	}
	return nil
}

var _ mcp.ForgetApplyStore = (*Repository)(nil)
