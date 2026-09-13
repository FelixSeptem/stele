package postgres

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/FelixSeptem/stele/internal/provider"
)

// Create persists a provider runtime binding in PostgreSQL, the service's
// existing system of record. Bindings are opaque references and contain only
// bounded identity/scope metadata required for revalidation.
func (r *Repository) Create(ctx context.Context, binding provider.RuntimeBinding) error {
	if r == nil || r.db == nil {
		return fmt.Errorf("repository is not configured")
	}
	if err := binding.Validate(binding.CreatedAt); err != nil {
		return err
	}
	_, err := r.db.Exec(ctx, `INSERT INTO provider_runtime_bindings (binding_id, principal_id, tenant, project, namespace, agent_id, session_id, conversation_id, provider_instance_id, created_at, expires_at) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11)`, binding.BindingID, binding.PrincipalID, binding.Scope.Tenant, binding.Scope.Project, binding.Scope.Namespace, binding.AgentID, binding.SessionID, binding.ConversationID, binding.ProviderInstanceID, binding.CreatedAt, binding.ExpiresAt)
	if err != nil {
		return fmt.Errorf("create provider runtime binding: %w", err)
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
