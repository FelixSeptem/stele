package postgres

import (
	"context"
	"errors"
	"fmt"

	"github.com/FelixSeptem/stele/internal/auth"
	"github.com/FelixSeptem/stele/internal/memory"
	"github.com/jackc/pgx/v5"
)

func (r *Repository) ScopeGrantDisclosure(ctx context.Context, principalID string, scope memory.Scope) (auth.DisclosureEntitlements, error) {
	if err := scope.Validate(); err != nil {
		return auth.DisclosureEntitlements{}, err
	}
	scope = scope.Normalized()
	const query = `SELECT allowed_disclosure_profiles, default_disclosure_profile, source_attestation
FROM access_scope_grants
WHERE principal_id=$1 AND tenant=$2 AND project=$3 AND namespace=$4 AND status='active'
ORDER BY created_at DESC, id DESC LIMIT 1`
	var e auth.DisclosureEntitlements
	if err := r.db.QueryRow(ctx, query, principalID, scope.Tenant, scope.Project, scope.Namespace).Scan(&e.AllowedProfiles, &e.DefaultProfile, &e.SourceAttestation); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return e, auth.ErrDisclosureDenied
		}
		return e, fmt.Errorf("disclosure store unavailable: %w", err)
	}
	// An unconfigured legacy grant conveys no disclosure or attestation authority.
	if e.Validate() != nil {
		return auth.DisclosureEntitlements{}, auth.ErrDisclosureDenied
	}
	return e, nil
}

func (r *Repository) UpdateScopeGrantDisclosure(ctx context.Context, scope memory.Scope, principalID, grantID string, e auth.DisclosureEntitlements, audit auth.AuditRecord) error {
	if err := scope.Validate(); err != nil {
		return err
	}
	if err := e.Validate(); err != nil {
		return err
	}
	scope = scope.Normalized()
	if principalID == "" || grantID == "" || audit.Scope.Normalized() != scope || audit.PrincipalID != principalID || audit.Actor == "" || audit.Reason == "" {
		return fmt.Errorf("invalid disclosure audit")
	}
	tx, err := r.tx.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return fmt.Errorf("begin disclosure update: %w", err)
	}
	defer tx.Rollback(ctx)
	const query = `UPDATE access_scope_grants SET allowed_disclosure_profiles=$6, default_disclosure_profile=$7, source_attestation=$8
WHERE id=$1 AND principal_id=$2 AND tenant=$3 AND project=$4 AND namespace=$5 AND status='active' RETURNING id`
	var updated string
	if err := tx.QueryRow(ctx, query, grantID, principalID, scope.Tenant, scope.Project, scope.Namespace, e.AllowedProfiles, e.DefaultProfile, e.SourceAttestation).Scan(&updated); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return auth.ErrDisclosureDenied
		}
		return fmt.Errorf("update disclosure grant: %w", err)
	}
	const auditQuery = `INSERT INTO access_audit_records (id,principal_id,tenant,project,namespace,action,result,created_at,actor,reason) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10)`
	if _, err := tx.Exec(ctx, auditQuery, audit.ID, principalID, scope.Tenant, scope.Project, scope.Namespace, audit.Action, audit.Result, audit.CreatedAt, audit.Actor, audit.Reason); err != nil {
		return fmt.Errorf("audit disclosure update: %w", err)
	}
	return tx.Commit(ctx)
}
