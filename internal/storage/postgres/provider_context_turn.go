package postgres

import (
	"context"
	"errors"
	"fmt"

	"github.com/FelixSeptem/stele/internal/provider"
	"github.com/jackc/pgx/v5"
)

func (r *Repository) AuthorizeContextTurn(ctx context.Context, b provider.RuntimeBinding, turnID string) error {
	if r == nil || r.db == nil {
		return fmt.Errorf("context turn authority unavailable")
	}
	found, err := contextTurnAuthorized(ctx, r.db, b, turnID)
	if err != nil {
		return err
	}
	if !found {
		return provider.ErrContextTurnUnavailable
	}
	return nil
}

func contextTurnAuthorized(ctx context.Context, db queryRower, b provider.RuntimeBinding, turnID string) (bool, error) {
	var found bool
	err := db.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM provider_context_turn_bindings WHERE turn_id=$1 AND binding_id=$2 AND principal_id=$3 AND tenant=$4 AND project=$5 AND namespace=$6 AND agent_id=$7 AND session_id=$8 AND conversation_id=$9 AND provider_instance_id=$10)`, turnID, b.BindingID, b.PrincipalID, b.Scope.Tenant, b.Scope.Project, b.Scope.Namespace, b.AgentID, b.SessionID, b.ConversationID, b.ProviderInstanceID).Scan(&found)
	if err != nil {
		return false, err
	}
	return found, nil
}

func (r *Repository) RecordContextTurnAttribution(ctx context.Context, b provider.RuntimeBinding, a provider.ContextTurnAttribution) error {
	if r == nil || r.db == nil {
		return fmt.Errorf("context turn attribution unavailable")
	}
	if a.TurnID == "" || a.RequestID == "" || a.OperationID == "" || a.Scope != b.Scope || !provider.ValidContextHash(a.Digest.Value) {
		return fmt.Errorf("invalid context turn attribution")
	}
	found, err := contextTurnAuthorized(ctx, r.db, b, a.TurnID)
	if err != nil {
		return err
	}
	if !found {
		return provider.ErrContextTurnUnavailable
	}
	_, err = r.db.Exec(ctx, `INSERT INTO provider_context_turn_attributions (turn_id,binding_id,principal_id,tenant,project,namespace,agent_id,session_id,conversation_id,provider_instance_id,request_id,operation_id,digest_contract,canonical_version,digest_value,continuity_reference,created_at) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,clock_timestamp())`, a.TurnID, b.BindingID, b.PrincipalID, a.Scope.Tenant, a.Scope.Project, a.Scope.Namespace, b.AgentID, b.SessionID, b.ConversationID, b.ProviderInstanceID, a.RequestID, a.OperationID, a.Digest.ContractVersion, a.Digest.CanonicalizationVersion, a.Digest.Value, nullableString(a.ContinuityReference))
	return err
}

var _ provider.ContextTurnAuthorizer = (*Repository)(nil)
var _ provider.ContextTurnAttributionStore = (*Repository)(nil)

func isTurnBindingConflict(err error) bool { return errors.Is(err, pgx.ErrNoRows) }
