package postgres

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/FelixSeptem/stele/internal/memory"
	"github.com/FelixSeptem/stele/internal/provider"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

// ListProjectionFeedbackScopes discovers ledger scopes, including those with
// only pending evidence. The latest logical checkpoint is the scope's last
// service time: even a scope with an unchecked backlog must yield to others.
func (r *Repository) ListProjectionFeedbackScopes(ctx context.Context, limit int) ([]memory.Scope, error) {
	if limit < 1 || r == nil || r.db == nil {
		return nil, fmt.Errorf("invalid projection feedback scope discovery")
	}
	rows, err := r.db.Query(ctx, `SELECT i.tenant,i.project,i.namespace
 FROM provider_projection_identities i
 LEFT JOIN provider_projection_feedback_checks ck ON ck.logical_id=i.logical_id
 GROUP BY i.tenant,i.project,i.namespace
 ORDER BY max(ck.checked_at) ASC NULLS FIRST,i.tenant,i.project,i.namespace
 LIMIT $1`, limit)
	if err != nil {
		return nil, fmt.Errorf("list projection feedback scopes: %w", err)
	}
	defer rows.Close()
	scopes := []memory.Scope{}
	for rows.Next() {
		var scope memory.Scope
		if err := rows.Scan(&scope.Tenant, &scope.Project, &scope.Namespace); err != nil {
			return nil, fmt.Errorf("scan projection feedback scope: %w", err)
		}
		scopes = append(scopes, scope)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("read projection feedback scopes: %w", err)
	}
	return scopes, nil
}

// ReconcileProjectionFeedback serializes with admission on the logical identity.
// A revision can map at most once; superseded evidence is never reactivated.
func (r *Repository) ReconcileProjectionFeedback(ctx context.Context, scope memory.Scope, limit int) (int, error) {
	if scope.Validate() != nil || limit < 1 || limit > 100 || r == nil || r.tx == nil {
		return 0, fmt.Errorf("invalid projection feedback reconciliation")
	}
	tx, err := r.tx.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return 0, err
	}
	defer tx.Rollback(ctx)
	rows, err := tx.Query(ctx, `SELECT i.logical_id::text,b.binding_id,b.principal_id,b.tenant,b.project,b.namespace,b.agent_id,b.session_id,b.conversation_id,b.provider_instance_id,b.created_at,b.expires_at
 FROM provider_projection_identities i JOIN provider_runtime_bindings b ON b.binding_id=i.binding_id
 LEFT JOIN provider_projection_feedback_checks ck ON ck.logical_id=i.logical_id
 WHERE i.tenant=$1 AND i.project=$2 AND i.namespace=$3
 ORDER BY ck.checked_at ASC NULLS FIRST,i.created_at,i.logical_id LIMIT $4 FOR UPDATE OF i SKIP LOCKED`, scope.Tenant, scope.Project, scope.Namespace, limit)
	if err != nil {
		return 0, err
	}
	type item struct {
		logical string
		b       provider.RuntimeBinding
	}
	items := []item{}
	for rows.Next() {
		var x item
		if err = rows.Scan(&x.logical, &x.b.BindingID, &x.b.PrincipalID, &x.b.Scope.Tenant, &x.b.Scope.Project, &x.b.Scope.Namespace, &x.b.AgentID, &x.b.SessionID, &x.b.ConversationID, &x.b.ProviderInstanceID, &x.b.CreatedAt, &x.b.ExpiresAt); err != nil {
			rows.Close()
			return 0, err
		}
		items = append(items, x)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return 0, err
	}
	for _, x := range items {
		// Keep the existing feedback log and supersession audit authoritative.
		_, err = tx.Exec(ctx, `WITH removed AS (UPDATE usefulness_feedback uf SET superseded_at=clock_timestamp(),superseded_by_actor='provider_projection_policy',superseded_by_reason='projection_evidence_ineligible'
   FROM provider_projection_feedback_links l WHERE l.logical_id=$1 AND l.feedback_id=uf.id AND uf.superseded_at IS NULL
   AND NOT EXISTS(SELECT 1 FROM eligible_usefulness_feedback eligible WHERE eligible.id=uf.id)
   RETURNING uf.tenant,uf.project,uf.namespace,uf.id,uf.superseded_at,uf.superseded_by_actor,uf.superseded_by_reason)
   INSERT INTO usefulness_feedback_supersessions(tenant,project,namespace,feedback_id,actor,reason,superseded_at) SELECT tenant,project,namespace,id,superseded_by_actor,superseded_by_reason,superseded_at FROM removed`, x.logical)
		if err != nil {
			return 0, err
		}
		if err = mapProjectionFeedback(ctx, tx, x.logical, x.b); err != nil {
			return 0, err
		}
		_, err = tx.Exec(ctx, `INSERT INTO provider_projection_feedback_checks(logical_id,checked_at) VALUES($1,clock_timestamp()) ON CONFLICT(logical_id) DO UPDATE SET checked_at=EXCLUDED.checked_at`, x.logical)
		if err != nil {
			return 0, err
		}
	}
	return len(items), tx.Commit(ctx)
}

func mapProjectionFeedback(ctx context.Context, tx pgx.Tx, logical string, b provider.RuntimeBinding) error {
	var projection string
	var revision int64
	var payload []byte
	var mapped bool
	err := tx.QueryRow(ctx, `SELECT r.projection_id::text,r.source_revision,r.payload,EXISTS(SELECT 1 FROM provider_projection_feedback_links l WHERE l.projection_id=r.projection_id) FROM provider_projection_revisions r WHERE r.logical_id=$1 ORDER BY r.source_revision DESC LIMIT 1`, logical).Scan(&projection, &revision, &payload, &mapped)
	if err != nil {
		return err
	}
	if mapped {
		return nil
	}
	var input provider.ProjectionInput
	if err = json.Unmarshal(payload, &input); err != nil {
		return err
	}
	if input.Feedback == nil {
		return nil
	}
	profile, err := projectionGrant(ctx, tx, b, true)
	if errors.Is(err, provider.ErrProjectionUnavailable) {
		return nil
	}
	if err != nil {
		return err
	}
	refs, err := projectionReferences(ctx, tx, b, profile, input.References)
	if err != nil {
		return err
	}
	if len(refs) == 0 {
		return nil
	}
	subjects := []memory.UsefulnessFeedbackSubject{}
	seen := map[string]bool{}
	for _, ref := range refs {
		if !ref.Eligible || ref.State != provider.ReferenceStatusCompleted || ref.Lineage == nil {
			return nil
		}
		if !seen[ref.Lineage.MemoryID] {
			subjects = append(subjects, memory.UsefulnessFeedbackSubject{Kind: memory.UsefulnessFeedbackSubjectMemory, ID: ref.Lineage.MemoryID})
			seen[ref.Lineage.MemoryID] = true
		}
	}
	feedback := memory.UsefulnessFeedback{ID: uuid.NewString(), Scope: b.Scope, Type: memory.UsefulnessFeedbackType(input.Feedback.Signal), SourceSurface: memory.UsefulnessFeedbackSourceSession, Subjects: subjects, Actor: "provider_projection_policy", Reason: input.Feedback.ReasonCode, IdempotencyKey: "pc6:" + projection, CreatedAt: time.Now().UTC(), Metadata: map[string]any{"projection_id": projection, "logical_id": logical, "source_revision": revision, "policy_version": provider.ProjectionFeedbackPolicy}}
	if err = feedback.Validate(); err != nil {
		return err
	}
	metadata, err := json.Marshal(feedback.Metadata)
	if err != nil {
		return err
	}
	_, err = tx.Exec(ctx, `INSERT INTO usefulness_feedback(id,tenant,project,namespace,feedback_type,source_surface,actor,reason,idempotency_key,metadata,created_at) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11)`, feedback.ID, b.Scope.Tenant, b.Scope.Project, b.Scope.Namespace, feedback.Type, feedback.SourceSurface, feedback.Actor, feedback.Reason, feedback.IdempotencyKey, metadata, feedback.CreatedAt)
	if err != nil {
		return err
	}
	for _, subject := range subjects {
		if _, err = createUsefulnessFeedbackSubject(ctx, tx, feedback, subject); err != nil {
			return err
		}
	}
	lineage, err := json.Marshal(refs)
	if err != nil {
		return err
	}
	_, err = tx.Exec(ctx, `INSERT INTO provider_projection_feedback_links(projection_id,logical_id,tenant,project,namespace,feedback_id,policy_version,lineage) VALUES($1,$2,$3,$4,$5,$6,$7,$8)`, projection, logical, b.Scope.Tenant, b.Scope.Project, b.Scope.Namespace, feedback.ID, provider.ProjectionFeedbackPolicy, lineage)
	return err
}
