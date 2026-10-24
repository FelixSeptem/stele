package postgres

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/FelixSeptem/stele/internal/memory"
	"github.com/FelixSeptem/stele/internal/provider"
	"github.com/jackc/pgx/v5"
)

// Create persists a provider runtime binding in PostgreSQL, the service's
// existing system of record. Bindings are opaque references and contain only
// bounded identity/scope metadata required for revalidation.
func (r *Repository) Create(ctx context.Context, binding provider.RuntimeBinding) error {
	if r == nil || r.db == nil {
		return fmt.Errorf("repository is not configured")
	}
	if err := binding.Validate(time.Now().UTC()); err != nil {
		return err
	}
	_, err := r.db.Exec(ctx, `INSERT INTO provider_runtime_bindings (binding_id, principal_id, tenant, project, namespace, agent_id, session_id, conversation_id, provider_instance_id, created_at, expires_at) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11)`, binding.BindingID, binding.PrincipalID, binding.Scope.Tenant, binding.Scope.Project, binding.Scope.Namespace, binding.AgentID, binding.SessionID, binding.ConversationID, binding.ProviderInstanceID, binding.CreatedAt, binding.ExpiresAt)
	if err != nil {
		return fmt.Errorf("create provider runtime binding: %w", err)
	}
	return nil
}

// CreateWithSession commits the runtime binding and its server-owned session
// together. A pre-existing session ID is reusable only when its exact scope
// matches; a foreign session can never be claimed by a new binding.
func (r *Repository) CreateWithSession(ctx context.Context, binding provider.RuntimeBinding, session memory.MemorySessionRun) error {
	if r == nil || r.tx == nil {
		return fmt.Errorf("repository is not configured")
	}
	if err := binding.Validate(time.Now().UTC()); err != nil {
		return err
	}
	if session.ID != binding.SessionID || session.Scope.Normalized() != binding.Scope.Normalized() || session.Status != memory.MemorySessionStatusActive || session.Verdict != memory.ScopeProofVerdictPending {
		return fmt.Errorf("runtime session does not match binding")
	}
	tx, err := r.tx.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return fmt.Errorf("begin runtime bootstrap: %w", err)
	}
	defer tx.Rollback(ctx)
	if _, err = tx.Exec(ctx, `INSERT INTO provider_runtime_bindings (binding_id, principal_id, tenant, project, namespace, agent_id, session_id, conversation_id, provider_instance_id, created_at, expires_at) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11)`, binding.BindingID, binding.PrincipalID, binding.Scope.Tenant, binding.Scope.Project, binding.Scope.Namespace, binding.AgentID, binding.SessionID, binding.ConversationID, binding.ProviderInstanceID, binding.CreatedAt, binding.ExpiresAt); err != nil {
		return fmt.Errorf("create provider runtime binding: %w", err)
	}
	metadata := []byte(`{}`)
	if _, err = tx.Exec(ctx, `INSERT INTO memory_session_runs (id, tenant, project, namespace, status, verdict, actor, reason, metadata, created_at, updated_at, started_at) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12) ON CONFLICT (id) DO NOTHING`, session.ID, session.Scope.Tenant, session.Scope.Project, session.Scope.Namespace, session.Status, session.Verdict, nullableString(session.Actor), nullableString(session.Reason), metadata, session.CreatedAt, session.UpdatedAt, nullableTime(session.StartedAt)); err != nil {
		return fmt.Errorf("create provider runtime session: %w", err)
	}
	var exact bool
	if err = tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM memory_session_runs WHERE id=$1 AND tenant=$2 AND project=$3 AND namespace=$4)`, session.ID, session.Scope.Tenant, session.Scope.Project, session.Scope.Namespace).Scan(&exact); err != nil {
		return fmt.Errorf("verify provider runtime session: %w", err)
	}
	if !exact {
		return fmt.Errorf("provider runtime session binding conflict")
	}
	if err = tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit runtime bootstrap: %w", err)
	}
	return nil
}

func (r *Repository) Lookup(ctx context.Context, bindingID string) (provider.RuntimeBinding, error) {
	if r == nil || r.db == nil {
		return provider.RuntimeBinding{}, fmt.Errorf("repository is not configured")
	}
	var b provider.RuntimeBinding
	var revokedAt sql.NullTime
	err := r.db.QueryRow(ctx, `SELECT binding_id, principal_id, tenant, project, namespace, agent_id, session_id, conversation_id, provider_instance_id, created_at, expires_at, revoked_at FROM provider_runtime_bindings WHERE binding_id = $1`, bindingID).Scan(&b.BindingID, &b.PrincipalID, &b.Scope.Tenant, &b.Scope.Project, &b.Scope.Namespace, &b.AgentID, &b.SessionID, &b.ConversationID, &b.ProviderInstanceID, &b.CreatedAt, &b.ExpiresAt, &revokedAt)
	if err != nil {
		return provider.RuntimeBinding{}, fmt.Errorf("lookup provider runtime binding: %w", err)
	}
	if revokedAt.Valid {
		b.RevokedAt = revokedAt.Time
	}
	return b, nil
}

var _ provider.RuntimeBindingStore = (*Repository)(nil)
var _ provider.RuntimeBindingSessionStore = (*Repository)(nil)
