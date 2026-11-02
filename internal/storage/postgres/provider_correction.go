package postgres

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/FelixSeptem/stele/internal/memory"
	"github.com/FelixSeptem/stele/internal/provider"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

// SubmitCorrection atomically resolves a correction source, validates its
// current durable lineage and target version, and writes the governed update
// intent plus immutable attribution. The client cannot supply either source
// attribution or authority separately from its authenticated runtime binding.
func (r *Repository) SubmitCorrection(ctx context.Context, binding provider.RuntimeBinding, request provider.CorrectionRequest) (provider.CorrectionResult, error) {
	if r == nil || r.tx == nil || request.Metadata.NormalizeValidate() != nil || request.Input.Validate() != nil || request.Metadata.IdempotencyKey == "" {
		return provider.CorrectionResult{}, provider.ErrCorrectionUnavailable
	}
	tx, err := r.tx.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return provider.CorrectionResult{}, fmt.Errorf("begin correction transaction: %w", err)
	}
	defer tx.Rollback(ctx)
	if intentID, replayed, replayErr := r.replayCorrectionIntent(ctx, tx, binding, request); replayErr != nil {
		return provider.CorrectionResult{}, replayErr
	} else if replayed {
		if err = tx.Commit(ctx); err != nil {
			return provider.CorrectionResult{}, fmt.Errorf("commit correction replay: %w", err)
		}
		return provider.CorrectionResult{IntentID: intentID, Status: memory.MemoryIntentStatusReplayed, AttributionState: "verified"}, nil
	}
	input, attribution, logicalID, err := correctionResolution(ctx, tx, binding, request.Input)
	if err != nil {
		return provider.CorrectionResult{}, err
	}
	input.RequestID, input.OperationID, input.IdempotencyKey = request.Metadata.RequestID, request.Metadata.OperationID, request.Metadata.IdempotencyKey
	if strings.TrimSpace(request.PolicyVersion) == "" || len(request.PolicyVersion) > 128 {
		return provider.CorrectionResult{}, provider.ErrCorrectionUnavailable
	}
	fingerprint, err := memory.CorrectionIntentFingerprint(input, attribution, request.PolicyVersion)
	if err != nil {
		return provider.CorrectionResult{}, provider.ErrCorrectionUnavailable
	}
	intentID, replayed, err := r.appendCorrectionIntent(ctx, tx, input, attribution, logicalID, fingerprint, request.PolicyVersion)
	if err != nil {
		return provider.CorrectionResult{}, err
	}
	if err = tx.Commit(ctx); err != nil {
		return provider.CorrectionResult{}, fmt.Errorf("commit correction transaction: %w", err)
	}
	status := memory.MemoryIntentStatusAccepted
	if replayed {
		status = memory.MemoryIntentStatusReplayed
	}
	return provider.CorrectionResult{IntentID: intentID, Status: status, AttributionState: "verified"}, nil
}

func (r *Repository) replayCorrectionIntent(ctx context.Context, tx pgx.Tx, binding provider.RuntimeBinding, request provider.CorrectionRequest) (string, bool, error) {
	if _, err := projectionGrant(ctx, tx, binding, true); err != nil {
		return "", false, correctionUnavailable(err)
	}
	var intentID string
	err := tx.QueryRow(ctx, `SELECT id::text FROM memory_intents
WHERE tenant=$1 AND project=$2 AND namespace=$3 AND (idempotency_key=$4 OR operation_id=$5)
ORDER BY created_at LIMIT 1 FOR SHARE`, binding.Scope.Tenant, binding.Scope.Project, binding.Scope.Namespace, request.Metadata.IdempotencyKey, request.Metadata.OperationID).Scan(&intentID)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", false, nil
	}
	if err != nil {
		return "", false, fmt.Errorf("read correction replay identity: %w", err)
	}
	var verifiedSource bool
	err = tx.QueryRow(ctx, `SELECT EXISTS(
SELECT 1 FROM memory_intent_correction_attributions a
JOIN provider_projection_revisions r ON r.projection_id=a.projection_id AND r.logical_id=a.logical_id
  AND r.tenant=a.tenant AND r.project=a.project AND r.namespace=a.namespace
JOIN provider_projection_identities i ON i.logical_id=r.logical_id AND i.tenant=r.tenant AND i.project=r.project AND i.namespace=r.namespace
WHERE a.intent_id=$1::uuid AND a.binding_id=$2 AND a.principal_id=$3
  AND a.projection_id=$4::uuid AND a.target_memory_id=$5::uuid AND a.target_version=$6
  AND a.tenant=$7 AND a.project=$8 AND a.namespace=$9
  AND r.source_revision>1 AND r.supersedes_id IS NOT NULL
  AND i.binding_id=$2 AND i.principal_id=$3 AND i.agent_id=$10 AND i.session_id=$11
  AND i.conversation_id=$12 AND i.provider_instance_id=$13
  AND NOT EXISTS(SELECT 1 FROM provider_projection_revisions newer WHERE newer.logical_id=r.logical_id AND newer.source_revision>r.source_revision)
)`, intentID, binding.BindingID, binding.PrincipalID, request.Input.ProjectionID, request.Input.TargetMemoryID, request.Input.ExpectedVersion, binding.Scope.Tenant, binding.Scope.Project, binding.Scope.Namespace, binding.AgentID, binding.SessionID, binding.ConversationID, binding.ProviderInstanceID).Scan(&verifiedSource)
	if err != nil {
		return "", false, fmt.Errorf("verify correction replay source: %w", err)
	}
	if !verifiedSource {
		return "", false, fmt.Errorf("%w: replay source no longer matches", provider.ErrCorrectionConflict)
	}
	record, err := r.ReadMemoryIntent(withRepositoryTx(ctx, tx), binding.Scope, intentID)
	if err != nil {
		return "", false, fmt.Errorf("read correction replay intent: %w", err)
	}
	if record.Correction == nil || record.Correction.BindingID != binding.BindingID || record.Correction.PrincipalID != binding.PrincipalID ||
		record.RequestID != request.Metadata.RequestID || record.OperationID != request.Metadata.OperationID || record.IdempotencyKey != request.Metadata.IdempotencyKey ||
		record.TargetMemoryID != request.Input.TargetMemoryID || record.TargetVersion != request.Input.ExpectedVersion ||
		record.Content != request.Input.Content || record.Reason != request.Input.Reason || record.PolicyVersion != request.PolicyVersion {
		return "", false, fmt.Errorf("%w: replay intent fields differ", provider.ErrCorrectionConflict)
	}
	input := memory.MemoryIntentInput{Scope: record.Scope, MemoryPath: record.MemoryPath, Type: record.Type, TargetMemoryID: record.TargetMemoryID, TargetVersion: record.TargetVersion,
		Content: record.Content, Actor: record.Actor, Reason: record.Reason, Provenance: record.Provenance, RequestID: record.RequestID, OperationID: record.OperationID,
		IdempotencyKey: record.IdempotencyKey, TargetInsightID: record.TargetInsightID, Evidence: record.Evidence}
	fingerprint, err := memory.CorrectionIntentFingerprint(input, *record.Correction, request.PolicyVersion)
	var storedFingerprint string
	if scanErr := tx.QueryRow(ctx, `SELECT request_fingerprint FROM memory_intents WHERE id=$1::uuid AND tenant=$2 AND project=$3 AND namespace=$4`, intentID, binding.Scope.Tenant, binding.Scope.Project, binding.Scope.Namespace).Scan(&storedFingerprint); scanErr != nil {
		return "", false, fmt.Errorf("read correction replay fingerprint: %w", scanErr)
	}
	if err != nil || fingerprint != storedFingerprint {
		return "", false, fmt.Errorf("%w: replay fingerprint differs", provider.ErrCorrectionConflict)
	}
	return intentID, true, nil
}

func correctionResolution(ctx context.Context, tx pgx.Tx, binding provider.RuntimeBinding, request provider.CorrectionInput) (memory.MemoryIntentInput, memory.CorrectionAttribution, string, error) {
	profile, err := projectionGrant(ctx, tx, binding, true)
	if err != nil {
		return memory.MemoryIntentInput{}, memory.CorrectionAttribution{}, "", correctionUnavailable(err)
	}
	var logicalID, projectionID, supersedes string
	var revision int64
	var payload []byte
	args := append(receiptIdentityArgs(binding), request.ProjectionID)
	err = tx.QueryRow(ctx, `SELECT r.logical_id::text,r.projection_id::text,r.source_revision,COALESCE(r.supersedes_id::text,''),r.payload
FROM provider_projection_revisions r JOIN provider_projection_identities i USING(logical_id,tenant,project,namespace)
WHERE i.`+receiptIdentity+` AND r.projection_id=$10::uuid FOR SHARE OF r,i`, args...).Scan(&logicalID, &projectionID, &revision, &supersedes, &payload)
	if errors.Is(err, pgx.ErrNoRows) || revision <= 1 || supersedes == "" {
		return memory.MemoryIntentInput{}, memory.CorrectionAttribution{}, "", provider.ErrCorrectionUnavailable
	}
	if err != nil {
		return memory.MemoryIntentInput{}, memory.CorrectionAttribution{}, "", fmt.Errorf("read correction projection: %w", err)
	}
	var newer bool
	if err = tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM provider_projection_revisions WHERE logical_id=$1::uuid AND source_revision>$2)`, logicalID, revision).Scan(&newer); err != nil {
		return memory.MemoryIntentInput{}, memory.CorrectionAttribution{}, "", fmt.Errorf("read correction head: %w", err)
	}
	if newer {
		return memory.MemoryIntentInput{}, memory.CorrectionAttribution{}, "", provider.ErrCorrectionStale
	}
	var projectionInput provider.ProjectionInput
	if json.Unmarshal(payload, &projectionInput) != nil {
		return memory.MemoryIntentInput{}, memory.CorrectionAttribution{}, "", provider.ErrCorrectionUnavailable
	}
	refs, err := projectionReferences(ctx, tx, binding, profile, projectionInput.References)
	if err != nil {
		return memory.MemoryIntentInput{}, memory.CorrectionAttribution{}, "", correctionUnavailable(err)
	}
	matched := false
	for _, ref := range refs {
		if ref.State == provider.ReferenceStatusCompleted && ref.Lineage != nil && ref.Lineage.MemoryID == request.TargetMemoryID && ref.Lineage.Version == request.ExpectedVersion {
			matched = true
			break
		}
	}
	if !matched {
		return memory.MemoryIntentInput{}, memory.CorrectionAttribution{}, "", provider.ErrCorrectionUnavailable
	}
	var state memory.MemoryState
	var path string
	var currentVersion int64
	err = tx.QueryRow(ctx, `SELECT cm.state,cm.memory_path,COALESCE((SELECT MAX(v.version) FROM memory_versions v WHERE v.memory_id=cm.id),0)
FROM canonical_memories cm WHERE cm.id=$1::uuid AND cm.tenant=$2 AND cm.project=$3 AND cm.namespace=$4 FOR SHARE`, request.TargetMemoryID, binding.Scope.Tenant, binding.Scope.Project, binding.Scope.Namespace).Scan(&state, &path, &currentVersion)
	if errors.Is(err, pgx.ErrNoRows) || state != memory.MemoryStateActive {
		return memory.MemoryIntentInput{}, memory.CorrectionAttribution{}, "", provider.ErrCorrectionUnavailable
	}
	if err != nil {
		return memory.MemoryIntentInput{}, memory.CorrectionAttribution{}, "", fmt.Errorf("read correction target: %w", err)
	}
	if currentVersion != request.ExpectedVersion {
		return memory.MemoryIntentInput{}, memory.CorrectionAttribution{}, "", provider.ErrCorrectionStale
	}
	attribution := memory.CorrectionAttribution{ProjectionID: projectionID, SourceRevision: revision, BindingID: binding.BindingID, PrincipalID: binding.PrincipalID, State: "verified"}
	if attribution.Validate() != nil {
		return memory.MemoryIntentInput{}, memory.CorrectionAttribution{}, "", provider.ErrCorrectionUnavailable
	}
	return memory.MemoryIntentInput{Scope: binding.Scope, MemoryPath: path, Type: memory.MemoryIntentUpdate, TargetMemoryID: request.TargetMemoryID, TargetVersion: request.ExpectedVersion, Content: request.Content, Actor: binding.PrincipalID, Reason: request.Reason, Provenance: map[string]any{"provider_correction": "verified"}}, attribution, logicalID, nil
}

func (r *Repository) appendCorrectionIntent(ctx context.Context, tx pgx.Tx, input memory.MemoryIntentInput, attribution memory.CorrectionAttribution, logicalID, fingerprint, policyVersion string) (string, bool, error) {
	if input.ValidateGoverned() != nil {
		return "", false, provider.ErrCorrectionUnavailable
	}
	payload, _ := json.Marshal(map[string]any{"content": input.Content, "target_memory_id": input.TargetMemoryID, "target_version": input.TargetVersion, "memory_path": input.MemoryPath, "policy_version": policyVersion})
	provenance, _ := json.Marshal(input.Provenance)
	intentID := uuid.NewString()
	precedence, err := memory.EvaluateOperationPrecedence(memory.OperationPrecedenceInput{
		Operation: "memory.intent.submit", Scope: input.Scope, GrantedScope: input.Scope,
		PrincipalID: input.Actor, GrantID: input.Actor, LifecycleChecked: true, LifecycleVisible: true, PrincipalGranted: input.Actor != "",
		ApprovalRequired: true, ApprovalEnabled: true, PolicyVersion: policyVersion,
		RequireIdempotency: true, IdempotencyKey: input.IdempotencyKey, RequestFingerprint: fingerprint,
		HandoffAllowed: r.riverIntentJobs != nil, MutationAllowed: true,
	})
	if err != nil || precedence.Outcome != memory.OperationOutcomeAccepted {
		return "", false, provider.ErrCorrectionUnavailable
	}
	err = tx.QueryRow(ctx, `INSERT INTO memory_intents(id,tenant,project,namespace,memory_path,intent_type,actor,reason,provenance,request_id,operation_id,idempotency_key,request_fingerprint,target_memory_id,target_version,policy_version,precedence_version,precedence_stage,precedence_outcome,evidence_refs,payload,status,created_at)
VALUES($1,$2,$3,$4,$5,'update',$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17,$18,'[]'::jsonb,$19,'accepted',$20) ON CONFLICT DO NOTHING RETURNING id::text`, intentID, input.Scope.Tenant, input.Scope.Project, input.Scope.Namespace, input.MemoryPath, input.Actor, input.Reason, provenance, input.RequestID, input.OperationID, input.IdempotencyKey, fingerprint, input.TargetMemoryID, input.TargetVersion, policyVersion, precedence.Version, precedence.Stage, precedence.Outcome, payload, time.Now().UTC()).Scan(&intentID)
	if errors.Is(err, pgx.ErrNoRows) {
		var existingID, existingFingerprint string
		err = tx.QueryRow(ctx, `SELECT id::text,request_fingerprint FROM memory_intents WHERE tenant=$1 AND project=$2 AND namespace=$3 AND (idempotency_key=$4 OR operation_id=$5) ORDER BY created_at LIMIT 1 FOR SHARE`, input.Scope.Tenant, input.Scope.Project, input.Scope.Namespace, input.IdempotencyKey, input.OperationID).Scan(&existingID, &existingFingerprint)
		if errors.Is(err, pgx.ErrNoRows) || existingFingerprint != fingerprint {
			return "", false, provider.ErrCorrectionConflict
		}
		if err != nil {
			return "", false, fmt.Errorf("read correction replay: %w", err)
		}
		return existingID, true, nil
	}
	if err != nil {
		return "", false, fmt.Errorf("append correction intent: %w", err)
	}
	_, err = tx.Exec(ctx, `INSERT INTO memory_intent_correction_attributions(intent_id,projection_id,logical_id,tenant,project,namespace,source_revision,binding_id,principal_id,target_memory_id,target_version,attribution_state)
VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,'verified')`, intentID, attribution.ProjectionID, logicalID, input.Scope.Tenant, input.Scope.Project, input.Scope.Namespace, attribution.SourceRevision, attribution.BindingID, attribution.PrincipalID, input.TargetMemoryID, input.TargetVersion)
	if err != nil {
		return "", false, fmt.Errorf("append correction attribution: %w", err)
	}
	if _, err = tx.Exec(ctx, `INSERT INTO memory_intent_transitions(id,intent_id,tenant,project,namespace,sequence,to_status,actor,reason,diagnostic_category,occurred_at)
VALUES($1,$2,$3,$4,$5,1,'accepted',$6,$7,'accepted',clock_timestamp()) ON CONFLICT (intent_id,sequence) DO NOTHING`, uuid.NewString(), intentID, input.Scope.Tenant, input.Scope.Project, input.Scope.Namespace, input.Actor, input.Reason); err != nil {
		return "", false, fmt.Errorf("append correction intent transition: %w", err)
	}
	jobID, err := r.insertMemoryIntentRiverJob(ctx, tx, intentID)
	if err != nil {
		return "", false, fmt.Errorf("enqueue correction intent: %w", err)
	}
	if err := r.recordMemoryIntentRiverJob(ctx, tx, memory.MemoryIntentRecord{ID: intentID, Scope: input.Scope}, jobID); err != nil {
		return "", false, err
	}
	return intentID, false, nil
}

func correctionUnavailable(err error) error {
	if errors.Is(err, provider.ErrProjectionUnavailable) {
		return provider.ErrCorrectionUnavailable
	}
	return err
}

// ValidateMemoryIntentCorrection re-resolves the association before routing so
// a superseded, revoked, hidden, or stale source never creates a candidate.
func (r *Repository) ValidateMemoryIntentCorrection(ctx context.Context, record memory.MemoryIntentRecord) error {
	if record.Correction == nil || record.Correction.Validate() != nil || r == nil || r.tx == nil {
		return provider.ErrCorrectionUnavailable
	}
	tx, err := r.beginTx(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	var binding provider.RuntimeBinding
	err = tx.QueryRow(ctx, `SELECT binding_id,principal_id,tenant,project,namespace,agent_id,session_id,conversation_id,provider_instance_id,created_at,expires_at FROM provider_runtime_bindings WHERE binding_id=$1 AND principal_id=$2 AND tenant=$3 AND project=$4 AND namespace=$5 FOR SHARE`, record.Correction.BindingID, record.Correction.PrincipalID, record.Scope.Tenant, record.Scope.Project, record.Scope.Namespace).Scan(&binding.BindingID, &binding.PrincipalID, &binding.Scope.Tenant, &binding.Scope.Project, &binding.Scope.Namespace, &binding.AgentID, &binding.SessionID, &binding.ConversationID, &binding.ProviderInstanceID, &binding.CreatedAt, &binding.ExpiresAt)
	if err != nil {
		return provider.ErrCorrectionUnavailable
	}
	request := provider.CorrectionInput{ContractVersion: provider.CorrectionContractVersion, ProjectionID: record.Correction.ProjectionID, TargetMemoryID: record.TargetMemoryID, ExpectedVersion: record.TargetVersion, Content: record.Content, Reason: record.Reason}
	_, current, _, err := correctionResolution(ctx, tx, binding, request)
	if err != nil || current != *record.Correction {
		if errors.Is(err, provider.ErrCorrectionStale) {
			return err
		}
		return provider.ErrCorrectionUnavailable
	}
	return tx.Commit(ctx)
}

var _ provider.CorrectionResolver = (*Repository)(nil)
var _ memory.MemoryIntentCorrectionValidator = (*Repository)(nil)
