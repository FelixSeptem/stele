package postgres

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/FelixSeptem/stele/internal/provider"
)

func (r *Repository) ClaimLifecycle(ctx context.Context, c provider.LifecycleClaim) (provider.LifecycleClaimResult, error) {
	var status, fp string
	var payload []byte
	var inserted bool
	err := r.db.QueryRow(ctx, `INSERT INTO provider_lifecycle_operations (tenant,project,namespace,principal_id,idempotency_key,request_fingerprint,status) VALUES ($1,$2,$3,$4,$5,$6,'pending') ON CONFLICT (tenant,project,namespace,principal_id,idempotency_key) DO UPDATE SET idempotency_key=EXCLUDED.idempotency_key RETURNING status, outcome, request_fingerprint, (xmax = 0)`, c.Scope.Tenant, c.Scope.Project, c.Scope.Namespace, c.PrincipalID, c.IdempotencyKey, c.RequestFingerprint).Scan(&status, &payload, &fp, &inserted)
	if err != nil {
		return provider.LifecycleClaimResult{}, fmt.Errorf("claim lifecycle: %w", err)
	}
	if fp != c.RequestFingerprint {
		return provider.LifecycleClaimResult{}, fmt.Errorf("idempotency conflict")
	}
	if status == "completed" {
		var out provider.OperationOutcome
		_ = json.Unmarshal(payload, &out)
		return provider.LifecycleClaimResult{Disposition: provider.LifecycleReplayed, Outcome: out}, nil
	}
	if status == "pending" && inserted {
		return provider.LifecycleClaimResult{Disposition: provider.LifecycleClaimed}, nil
	}
	return provider.LifecycleClaimResult{Disposition: provider.LifecycleInProgress}, nil
}

func (r *Repository) CompleteLifecycle(ctx context.Context, c provider.LifecycleClaim, out provider.OperationOutcome) error {
	b, _ := json.Marshal(out)
	tag, err := r.db.Exec(ctx, `UPDATE provider_lifecycle_operations SET status='completed', outcome=$6, completed_at=NOW() WHERE tenant=$1 AND project=$2 AND namespace=$3 AND principal_id=$4 AND idempotency_key=$5 AND request_fingerprint=$7 AND status='pending'`, c.Scope.Tenant, c.Scope.Project, c.Scope.Namespace, c.PrincipalID, c.IdempotencyKey, b, c.RequestFingerprint)
	if err != nil {
		return err
	}
	if tag.RowsAffected() != 1 {
		return fmt.Errorf("lifecycle claim unavailable")
	}
	return nil
}

func (r *Repository) ReleaseLifecycle(ctx context.Context, c provider.LifecycleClaim) error {
	tag, err := r.db.Exec(ctx, `DELETE FROM provider_lifecycle_operations WHERE tenant=$1 AND project=$2 AND namespace=$3 AND principal_id=$4 AND idempotency_key=$5 AND request_fingerprint=$6 AND status='pending'`, c.Scope.Tenant, c.Scope.Project, c.Scope.Namespace, c.PrincipalID, c.IdempotencyKey, c.RequestFingerprint)
	if err != nil {
		return fmt.Errorf("release lifecycle claim: %w", err)
	}
	if tag.RowsAffected() > 1 {
		return fmt.Errorf("released unexpected lifecycle claims")
	}
	return nil
}
