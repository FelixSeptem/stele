package postgres

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/FelixSeptem/stele/internal/auth"
	"github.com/FelixSeptem/stele/internal/provider"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

// Locks current binding, principal and the newest active exact grant until the
// transaction commits. No cached binding can outlive a revocation at admission.
func projectionGrant(ctx context.Context, tx pgx.Tx, b provider.RuntimeBinding, write bool) (string, error) {
	if b.Validate(time.Now().UTC()) != nil || provider.ProjectionScope(b).Validate() != nil {
		return "", provider.ErrProjectionUnavailable
	}
	var profiles []string
	var profile, mode string
	err := tx.QueryRow(ctx, `SELECT g.allowed_disclosure_profiles,g.default_disclosure_profile,g.access_mode
 FROM provider_runtime_bindings b JOIN access_principals p ON p.id=b.principal_id
 JOIN access_scope_grants g ON g.principal_id=p.id AND g.tenant=b.tenant AND g.project=b.project AND g.namespace=b.namespace
 WHERE b.binding_id=$1 AND b.principal_id=$2 AND b.tenant=$3 AND b.project=$4 AND b.namespace=$5
 AND b.agent_id=$6 AND b.session_id=$7 AND b.conversation_id=$8 AND b.provider_instance_id=$9
 AND b.revoked_at IS NULL AND b.expires_at>clock_timestamp() AND p.status='active' AND p.disabled_at IS NULL
 AND (p.expires_at IS NULL OR p.expires_at>clock_timestamp()) AND g.status='active' AND g.revoked_at IS NULL
 ORDER BY g.created_at DESC,g.id DESC LIMIT 1 FOR SHARE OF b,p,g`, receiptIdentityArgs(b)...).Scan(&profiles, &profile, &mode)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", provider.ErrProjectionUnavailable
	}
	if err != nil {
		return "", err
	}
	decision, e := (auth.DisclosureEntitlements{AllowedProfiles: profiles, DefaultProfile: profile}).Resolve("", auth.ResponseModeReferenceOnly)
	if e != nil || (write && mode != "read_write") {
		return "", provider.ErrProjectionUnavailable
	}
	return decision.Profile, nil
}

func (r *Repository) AdmitProjection(ctx context.Context, b provider.RuntimeBinding, req provider.ProjectionRequest) (provider.ProjectionRecord, error) {
	var empty provider.ProjectionRecord
	if req.Input.Validate() != nil || req.Metadata.NormalizeValidate() != nil || req.Metadata.IdempotencyKey == "" {
		return empty, fmt.Errorf("invalid projection request")
	}
	encoded, err := json.Marshal(req)
	if err != nil || len(encoded) > provider.MaxProjectionRequestBytes {
		return empty, fmt.Errorf("projection request exceeds limit")
	}
	if r == nil || r.tx == nil {
		return empty, fmt.Errorf("projection dependency unavailable")
	}
	tx, err := r.tx.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return empty, err
	}
	defer tx.Rollback(ctx)
	profile, err := projectionGrant(ctx, tx, b, true)
	if err != nil {
		return empty, err
	}
	args := append(receiptIdentityArgs(b), req.Input.Kind, req.Input.EffectID, uuid.NewString())
	_, err = tx.Exec(ctx, `INSERT INTO provider_projection_identities(binding_id,principal_id,tenant,project,namespace,agent_id,session_id,conversation_id,provider_instance_id,kind,effect_id,logical_id) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12) ON CONFLICT(binding_id,kind,effect_id) DO NOTHING`, args...)
	if err != nil {
		return empty, err
	}
	var logical string
	err = tx.QueryRow(ctx, `SELECT logical_id::text FROM provider_projection_identities WHERE `+receiptIdentity+` AND kind=$10 AND effect_id=$11 FOR UPDATE`, args[:11]...).Scan(&logical)
	if errors.Is(err, pgx.ErrNoRows) {
		return empty, provider.ErrProjectionUnavailable
	}
	if err != nil {
		return empty, err
	}
	fp := req.Input.Fingerprint()
	var priorFP string
	var admission, payload []byte
	err = tx.QueryRow(ctx, `SELECT fingerprint,admission,payload FROM provider_projection_revisions WHERE logical_id=$1 AND idempotency_key=$2`, logical, req.Metadata.IdempotencyKey).Scan(&priorFP, &admission, &payload)
	if err == nil {
		if priorFP != fp {
			return empty, provider.ErrProjectionConflict
		}
		out, e := projectionRead(ctx, tx, b, profile, admission, payload)
		if e != nil {
			return empty, e
		}
		return out, tx.Commit(ctx)
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return empty, err
	}
	var previousID string
	var previousRevision int64
	err = tx.QueryRow(ctx, `SELECT projection_id::text,source_revision FROM provider_projection_revisions WHERE logical_id=$1 ORDER BY source_revision DESC LIMIT 1`, logical).Scan(&previousID, &previousRevision)
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return empty, err
	}
	if previousRevision == req.Input.SourceRevision {
		return empty, provider.ErrProjectionConflict
	}
	if previousRevision > req.Input.SourceRevision {
		return empty, provider.ErrProjectionStale
	}
	out := provider.ProjectionRecord{ProjectionID: uuid.NewString(), LogicalID: logical, SourceRevision: req.Input.SourceRevision, SupersedesID: previousID, Disposition: "accepted", EvidenceKind: provider.ProjectionEvidenceKind, References: []provider.ProjectionReferenceAttribution{}, CurrentReferences: []provider.ProjectionReferenceAttribution{}, FeedbackState: "none", ContextDigestAttribution: "none"}
	if previousID != "" {
		out.Disposition = "corrected"
	}
	if err = projectionSession(ctx, tx, b, req.Input); err != nil {
		return empty, err
	}
	if req.Input.ContextDigest != "" {
		out.ContextDigestAttribution = "reported"
	}
	if req.Input.ContextReference != "" {
		receipt, e := (&Repository{db: tx}).LookupContextReceipt(ctx, b, req.Input.ContextReference)
		if errors.Is(e, provider.ErrContinuityUnavailable) || errors.Is(e, provider.ErrContinuityExpired) {
			return empty, provider.ErrProjectionUnavailable
		}
		if e != nil {
			return empty, e
		}
		if receipt.Digest.Value != req.Input.ContextDigest {
			return empty, provider.ErrProjectionConflict
		}
		out.ContextDigestAttribution = "verified_receipt"
	}
	out.References, err = projectionReferences(ctx, tx, b, profile, req.Input.References)
	if err != nil {
		return empty, err
	}
	out.CurrentReferences = append([]provider.ProjectionReferenceAttribution{}, out.References...)
	if req.Input.Feedback != nil {
		out.FeedbackState = "pending"
	}
	if err = out.Validate(); err != nil {
		return empty, err
	}
	admission, err = json.Marshal(out)
	if err != nil {
		return empty, err
	}
	payload, err = json.Marshal(req.Input)
	if err != nil {
		return empty, err
	}
	_, err = tx.Exec(ctx, `INSERT INTO provider_projection_revisions(projection_id,logical_id,tenant,project,namespace,source_revision,idempotency_key,fingerprint,supersedes_id,payload,admission) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11)`, out.ProjectionID, logical, b.Scope.Tenant, b.Scope.Project, b.Scope.Namespace, req.Input.SourceRevision, req.Metadata.IdempotencyKey, fp, nullableString(previousID), payload, admission)
	if err != nil {
		return empty, err
	}
	return out, tx.Commit(ctx)
}

func projectionSession(ctx context.Context, tx pgx.Tx, b provider.RuntimeBinding, i provider.ProjectionInput) error {
	if i.SessionRunID == "" {
		return nil
	}
	if i.SessionRunID != b.SessionID {
		return provider.ErrProjectionUnavailable
	}
	var found bool
	err := tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM memory_session_runs s WHERE s.id=$1 AND s.tenant=$2 AND s.project=$3 AND s.namespace=$4 AND ($5='' OR EXISTS(SELECT 1 FROM memory_session_turns t WHERE t.id=$5 AND t.session_id=s.id AND t.tenant=s.tenant AND t.project=s.project AND t.namespace=s.namespace)))`, i.SessionRunID, b.Scope.Tenant, b.Scope.Project, b.Scope.Namespace, i.TurnID).Scan(&found)
	if err != nil {
		return err
	}
	if !found {
		return provider.ErrProjectionUnavailable
	}
	return nil
}

func projectionReferences(ctx context.Context, tx pgx.Tx, b provider.RuntimeBinding, profile string, selectors []provider.ProjectionReferenceSelector) ([]provider.ProjectionReferenceAttribution, error) {
	refs := make([]provider.ProjectionReferenceAttribution, 0, len(selectors))
	for _, selector := range selectors {
		unavailable := provider.ProjectionReferenceAttribution{State: provider.ReferenceStatusUnavailable}
		var owned bool
		err := tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM raw_events WHERE id=$1::uuid AND tenant=$2 AND project=$3 AND namespace=$4 AND metadata->>'reference_binding_id'=$5 AND metadata->>'reference_principal_id'=$6)`, selector.EventID, b.Scope.Tenant, b.Scope.Project, b.Scope.Namespace, b.BindingID, b.PrincipalID).Scan(&owned)
		if err != nil {
			return nil, err
		}
		if !owned {
			refs = append(refs, unavailable)
			continue
		}
		response, e := (&Repository{db: tx}).ReferenceStatus(ctx, b.Scope, b.PrincipalID, selector.EventID, profile, auth.ResponseModeReferenceOnly)
		if e != nil {
			return nil, e
		}
		if response.Status == provider.ReferenceStatusUnavailable {
			refs = append(refs, unavailable)
			continue
		}
		ref := provider.ProjectionReferenceAttribution{EventID: selector.EventID, State: response.Status, Eligible: response.Status == provider.ReferenceStatusCompleted}
		if ref.Eligible {
			if response.Reference == nil {
				return nil, fmt.Errorf("completed reference missing lineage")
			}
			r := response.Reference
			ref.Lineage = &provider.ProjectionReferenceLineage{CandidateID: r.CandidateID, MemoryID: r.MemoryID, VersionID: r.VersionID, ProvenanceID: r.ProvenanceID, Version: r.Version}
		}
		refs = append(refs, ref)
	}
	return refs, nil
}

// Retain immutable snapshots privately but redact an old identity whenever the
// corresponding current reference is unavailable. Reads cannot resurrect access.
func projectionRead(ctx context.Context, tx pgx.Tx, b provider.RuntimeBinding, profile string, admission, payload []byte) (provider.ProjectionRecord, error) {
	var out provider.ProjectionRecord
	var input provider.ProjectionInput
	if err := json.Unmarshal(admission, &out); err != nil {
		return out, err
	}
	if err := json.Unmarshal(payload, &input); err != nil {
		return out, err
	}
	current, err := projectionReferences(ctx, tx, b, profile, input.References)
	if err != nil {
		return out, err
	}
	if len(out.References) != len(current) {
		return out, fmt.Errorf("invalid stored attribution")
	}
	out.CurrentReferences = current
	for n, ref := range current {
		if ref.State == provider.ReferenceStatusUnavailable {
			out.References[n] = ref
		}
	}
	return out, out.Validate()
}

func (r *Repository) ProjectionHistory(ctx context.Context, b provider.RuntimeBinding, input provider.ProjectionHistoryInput) ([]provider.ProjectionRecord, bool, error) {
	if err := input.Validate(); err != nil {
		return nil, false, err
	}
	if r == nil || r.tx == nil {
		return nil, false, fmt.Errorf("projection dependency unavailable")
	}
	tx, err := r.tx.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return nil, false, err
	}
	defer tx.Rollback(ctx)
	profile, err := projectionGrant(ctx, tx, b, false)
	if err != nil {
		return nil, false, err
	}
	args := append(receiptIdentityArgs(b), input.Kind, input.EffectID, input.Limit+1)
	rows, err := tx.Query(ctx, `SELECT v.admission,v.payload FROM provider_projection_revisions v JOIN provider_projection_identities i USING(logical_id,tenant,project,namespace) WHERE i.binding_id=$1 AND i.principal_id=$2 AND i.tenant=$3 AND i.project=$4 AND i.namespace=$5 AND i.agent_id=$6 AND i.session_id=$7 AND i.conversation_id=$8 AND i.provider_instance_id=$9 AND i.kind=$10 AND i.effect_id=$11 ORDER BY v.source_revision DESC LIMIT $12`, args...)
	if err != nil {
		return nil, false, err
	}
	type stored struct{ admission, payload []byte }
	records := []stored{}
	for rows.Next() {
		var s stored
		if err = rows.Scan(&s.admission, &s.payload); err != nil {
			rows.Close()
			return nil, false, err
		}
		records = append(records, s)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return nil, false, err
	}
	more := len(records) > input.Limit
	if more {
		records = records[:input.Limit]
	}
	out := make([]provider.ProjectionRecord, 0, len(records))
	for _, s := range records {
		record, e := projectionRead(ctx, tx, b, profile, s.admission, s.payload)
		if e != nil {
			return nil, false, e
		}
		out = append(out, record)
	}
	return out, more, tx.Commit(ctx)
}

var _ provider.ProjectionStore = (*Repository)(nil)
