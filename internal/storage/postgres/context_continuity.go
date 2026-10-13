package postgres

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/FelixSeptem/stele/internal/provider"
	"github.com/jackc/pgx/v5"
)

const receiptColumns = `reference,request_fingerprint,digest_contract,canonical_version,digest_algorithm,selection_version,selection_fingerprint,digest_value,issued_at,expires_at`
const receiptIdentity = `binding_id=$1 AND principal_id=$2 AND tenant=$3 AND project=$4 AND namespace=$5 AND agent_id=$6 AND session_id=$7 AND conversation_id=$8 AND provider_instance_id=$9`

func receiptIdentityArgs(b provider.RuntimeBinding) []any {
	return []any{b.BindingID, b.PrincipalID, b.Scope.Tenant, b.Scope.Project, b.Scope.Namespace, b.AgentID, b.SessionID, b.ConversationID, b.ProviderInstanceID}
}
func scanReceipt(row pgx.Row) (provider.ContextReceipt, error) {
	var r provider.ContextReceipt
	err := row.Scan(&r.Reference, &r.RequestFingerprint, &r.Digest.ContractVersion, &r.Digest.CanonicalizationVersion, &r.Digest.Algorithm, &r.Digest.SelectionPolicyVersion, &r.Digest.SelectionPolicyFingerprint, &r.Digest.Value, &r.IssuedAt, &r.ExpiresAt)
	return r, err
}
func (r *Repository) LookupContextReceipt(ctx context.Context, b provider.RuntimeBinding, reference string) (provider.ContextReceipt, error) {
	if r == nil || r.db == nil {
		return provider.ContextReceipt{}, fmt.Errorf("receipt dependency unavailable")
	}
	args := append(receiptIdentityArgs(b), reference)
	// Clock comparison happens in PostgreSQL, never on an API replica's clock.
	var out provider.ContextReceipt
	var expired bool
	err := r.db.QueryRow(ctx, `SELECT `+receiptColumns+`,expires_at<=clock_timestamp() FROM provider_context_receipts WHERE `+receiptIdentity+` AND reference=$10`, args...).Scan(&out.Reference, &out.RequestFingerprint, &out.Digest.ContractVersion, &out.Digest.CanonicalizationVersion, &out.Digest.Algorithm, &out.Digest.SelectionPolicyVersion, &out.Digest.SelectionPolicyFingerprint, &out.Digest.Value, &out.IssuedAt, &out.ExpiresAt, &expired)
	if errors.Is(err, pgx.ErrNoRows) {
		return out, provider.ErrContinuityUnavailable
	}
	if err != nil {
		return out, err
	}
	if expired {
		return out, provider.ErrContinuityExpired
	}
	return out, nil
}
func receiptLock(ctx context.Context, tx pgx.Tx) error {
	var singleton bool
	err := tx.QueryRow(ctx, `SELECT singleton FROM provider_context_receipt_capacity WHERE singleton=true FOR UPDATE`).Scan(&singleton)
	return err
}
func cleanupReceipts(ctx context.Context, tx pgx.Tx, limit int) (int, error) {
	tag, err := tx.Exec(ctx, `DELETE FROM provider_context_receipts WHERE reference IN (SELECT reference FROM provider_context_receipts WHERE expires_at<=clock_timestamp() ORDER BY expires_at,reference LIMIT $1)`, limit)
	return int(tag.RowsAffected()), err
}
func (r *Repository) CleanupContextReceipts(ctx context.Context, limit int) (int, error) {
	if limit < 1 || limit > 5000 || r == nil || r.tx == nil {
		return 0, fmt.Errorf("invalid receipt cleanup")
	}
	tx, err := r.tx.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return 0, err
	}
	defer tx.Rollback(ctx)
	if err = receiptLock(ctx, tx); err != nil {
		return 0, err
	}
	n, err := cleanupReceipts(ctx, tx, limit)
	if err != nil {
		return 0, err
	}
	return n, tx.Commit(ctx)
}
func (r *Repository) IssueContextReceipt(ctx context.Context, b provider.RuntimeBinding, fingerprint string, digest provider.ContextDigest, c provider.ContextContinuityConfig, finalize func(provider.ContextReceipt) error) (provider.ContextReceipt, error) {
	var empty provider.ContextReceipt
	if r == nil || r.tx == nil || c.Validate() != nil || !provider.ValidContextHash(fingerprint) || finalize == nil || provider.ValidateContextReceipt(provider.ContextReceipt{RequestFingerprint: fingerprint, Digest: digest}, fingerprint) != nil {
		return empty, fmt.Errorf("invalid receipt issuance")
	}
	tx, err := r.tx.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return empty, err
	}
	defer tx.Rollback(ctx)
	if err = receiptLock(ctx, tx); err != nil {
		return empty, err
	}
	// Lookup compatible live receipts first: reuse succeeds even at capacity and
	// neither renews TTL nor mutates a retained receipt.
	args := append(receiptIdentityArgs(b), fingerprint, digest.ContractVersion, digest.CanonicalizationVersion, digest.Algorithm, digest.SelectionPolicyVersion, digest.SelectionPolicyFingerprint, digest.Value)
	out, err := scanReceipt(tx.QueryRow(ctx, `SELECT `+receiptColumns+` FROM provider_context_receipts WHERE `+receiptIdentity+` AND request_fingerprint=$10 AND digest_contract=$11 AND canonical_version=$12 AND digest_algorithm=$13 AND selection_version=$14 AND selection_fingerprint=$15 AND digest_value=$16 AND expires_at>clock_timestamp() ORDER BY issued_at,reference LIMIT 1`, args...))
	if err == nil {
		if err = finalize(out); err != nil {
			return empty, err
		}
		return out, tx.Commit(ctx)
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return empty, err
	}
	if _, err = cleanupReceipts(ctx, tx, c.CleanupBatch); err != nil {
		return empty, err
	}
	var global, binding int
	if err = tx.QueryRow(ctx, `SELECT count(*),count(*) FILTER(WHERE binding_id=$1) FROM provider_context_receipts`, b.BindingID).Scan(&global, &binding); err != nil {
		return empty, err
	}
	if global >= c.MaxReceipts || binding >= c.MaxReceiptsPerBinding {
		return empty, provider.ErrContinuityCapacity
	}
	var issued, expires time.Time
	if err = tx.QueryRow(ctx, `SELECT date_trunc('second',clock_timestamp())`).Scan(&issued); err != nil {
		return empty, err
	}
	expires = issued.Add(time.Duration(c.TTLSeconds) * time.Second)
	if cap := b.ExpiresAt.UTC().Truncate(time.Second); cap.Before(expires) {
		expires = cap
	}
	if !expires.After(issued) {
		return empty, provider.ErrContinuityExpired
	}
	reference, err := provider.NewContextReference()
	if err != nil {
		return empty, err
	}
	out = provider.ContextReceipt{Reference: reference, RequestFingerprint: fingerprint, Digest: digest, IssuedAt: issued, ExpiresAt: expires}
	if err = finalize(out); err != nil {
		return empty, err
	}
	args = append(receiptIdentityArgs(b), reference, fingerprint, digest.ContractVersion, digest.CanonicalizationVersion, digest.Algorithm, digest.SelectionPolicyVersion, digest.SelectionPolicyFingerprint, digest.Value, issued, expires)
	_, err = tx.Exec(ctx, `INSERT INTO provider_context_receipts (binding_id,principal_id,tenant,project,namespace,agent_id,session_id,conversation_id,provider_instance_id,reference,request_fingerprint,digest_contract,canonical_version,digest_algorithm,selection_version,selection_fingerprint,digest_value,issued_at,expires_at) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17,$18,$19)`, args...)
	if err != nil {
		return empty, err
	}
	return out, tx.Commit(ctx)
}

var _ provider.ContextReceiptStore = (*Repository)(nil)
