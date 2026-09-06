package postgres

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/FelixSeptem/stele/internal/governance"
	"github.com/FelixSeptem/stele/internal/memory"
	"github.com/google/uuid"
)

// AppendMemoryIntent persists one governed intent. Idempotency and operation
// keys are scoped to tenant/project/namespace; retries return the first row.
func (r *Repository) AppendMemoryIntent(ctx context.Context, record memory.MemoryIntentRecord) (memory.MemoryIntentRecord, error) {
	if err := record.Scope.Validate(); err != nil {
		return memory.MemoryIntentRecord{}, err
	}
	if strings.TrimSpace(string(record.Type)) == "" {
		return memory.MemoryIntentRecord{}, fmt.Errorf("memory intent type is required")
	}
	if record.ID == "" {
		record.ID = uuid.NewString()
	}
	if record.CreatedAt.IsZero() {
		record.CreatedAt = time.Now().UTC()
	}
	payload, _ := json.Marshal(map[string]any{"content": record.Content, "target_memory_id": record.TargetMemoryID, "target_version": record.TargetVersion})
	prov, _ := json.Marshal(record.Provenance)
	fingerprint := sha256.Sum256(payload)
	fp := hex.EncodeToString(fingerprint[:])
	const q = `INSERT INTO memory_intents (id,tenant,project,namespace,intent_type,actor,reason,provenance,request_id,operation_id,idempotency_key,request_fingerprint,target_memory_id,target_version,payload,status,created_at)
VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,NULLIF($13,''),NULLIF($14,0),$15,$16,$17)
ON CONFLICT DO NOTHING
RETURNING id,tenant,project,namespace,intent_type,actor,reason,provenance,request_id,operation_id,idempotency_key,target_memory_id,target_version,payload,status,created_at`
	row := r.db.QueryRow(ctx, q, record.ID, record.Scope.Tenant, record.Scope.Project, record.Scope.Namespace, record.Type, record.Actor, record.Reason, prov, record.RequestID, record.OperationID, record.IdempotencyKey, fp, record.TargetMemoryID, record.TargetVersion, payload, record.Status, record.CreatedAt)
	created, err := scanMemoryIntent(row)
	if err == nil {
		return created, nil
	}
	const existing = `SELECT id,tenant,project,namespace,intent_type,actor,reason,provenance,request_id,operation_id,idempotency_key,target_memory_id,target_version,payload,status,created_at FROM memory_intents WHERE tenant=$1 AND project=$2 AND namespace=$3 AND (idempotency_key=$4 OR operation_id=$5) ORDER BY created_at LIMIT 1`
	existingRecord, lookupErr := scanMemoryIntent(r.db.QueryRow(ctx, existing, record.Scope.Tenant, record.Scope.Project, record.Scope.Namespace, record.IdempotencyKey, record.OperationID))
	if lookupErr != nil {
		return memory.MemoryIntentRecord{}, fmt.Errorf("append memory intent: %w", err)
	}
	return existingRecord, nil
}

// EnqueueMemoryIntent is intentionally side-effect free at the canonical
// memory layer. The accepted ledger row is the durable queue boundary; the
// reflection/governance workers claim subsequent work without mutating it.
func (r *Repository) EnqueueMemoryIntent(context.Context, memory.MemoryIntentRecord) error {
	return nil
}

func (r *Repository) AppendReflectionReview(ctx context.Context, record memory.ReflectionReviewRecord) (memory.ReflectionReviewRecord, error) {
	if err := record.Scope.Validate(); err != nil {
		return memory.ReflectionReviewRecord{}, err
	}
	if !record.Decision.Valid() || strings.TrimSpace(record.CandidateID) == "" || strings.TrimSpace(record.Reviewer) == "" || strings.TrimSpace(record.Reason) == "" || strings.TrimSpace(record.PolicyVersion) == "" {
		return memory.ReflectionReviewRecord{}, fmt.Errorf("invalid reflection review record")
	}
	if record.ID == "" {
		record.ID = uuid.NewString()
	}
	if record.CreatedAt.IsZero() {
		record.CreatedAt = time.Now().UTC()
	}
	const query = `
INSERT INTO reflection_review_decisions (id, candidate_memory_id, tenant, project, namespace, decision, reviewer, reason, policy_version, created_at)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
RETURNING id, candidate_memory_id, tenant, project, namespace, decision, reviewer, reason, policy_version, created_at`
	var candidateID *string
	if record.CandidateID != "" {
		candidateID = &record.CandidateID
	}
	err := r.db.QueryRow(ctx, query, record.ID, candidateID, record.Scope.Tenant, record.Scope.Project, record.Scope.Namespace, record.Decision, record.Reviewer, record.Reason, record.PolicyVersion, record.CreatedAt).Scan(
		&record.ID, &candidateID, &record.Scope.Tenant, &record.Scope.Project, &record.Scope.Namespace, &record.Decision, &record.Reviewer, &record.Reason, &record.PolicyVersion, &record.CreatedAt)
	if err != nil {
		return memory.ReflectionReviewRecord{}, fmt.Errorf("append reflection review: %w", err)
	}
	if candidateID != nil {
		record.CandidateID = *candidateID
	}
	return record, nil
}

var _ memory.ReflectionReviewProcessor = (*Repository)(nil)

func (r *Repository) ValidateMemoryIntentTarget(ctx context.Context, input memory.MemoryIntentInput) error {
	if input.Type == memory.MemoryIntentRemember {
		return nil
	}
	if err := input.Validate(); err != nil {
		return err
	}
	const query = `
SELECT cm.state, COALESCE(MAX(mv.version), 0)
FROM canonical_memories cm
LEFT JOIN memory_versions mv ON mv.memory_id = cm.id
WHERE cm.id::text = $1 AND cm.tenant = $2 AND cm.project = $3 AND cm.namespace = $4
GROUP BY cm.state`
	var state memory.MemoryState
	var version int64
	if err := r.db.QueryRow(ctx, query, input.TargetMemoryID, input.Scope.Tenant, input.Scope.Project, input.Scope.Namespace).Scan(&state, &version); err != nil {
		return fmt.Errorf("memory intent target not found: %w", err)
	}
	if state != memory.MemoryStateActive {
		return fmt.Errorf("memory intent target lifecycle state %q is not writable", state)
	}
	if input.TargetVersion != version {
		return fmt.Errorf("memory intent target version conflict: expected %d, current %d", input.TargetVersion, version)
	}
	return nil
}

func (r *Repository) ReadMemoryIntent(ctx context.Context, scope memory.Scope, intentID string) (memory.MemoryIntentRecord, error) {
	if err := scope.Validate(); err != nil {
		return memory.MemoryIntentRecord{}, err
	}
	if strings.TrimSpace(intentID) == "" {
		return memory.MemoryIntentRecord{}, fmt.Errorf("intent id is required")
	}
	const query = `
SELECT id, tenant, project, namespace, intent_type, actor, reason, provenance,
       request_id, operation_id, idempotency_key, target_memory_id,
       target_version, payload, status, created_at
FROM memory_intents
WHERE id::text = $1 AND tenant = $2 AND project = $3 AND namespace = $4`
	return scanMemoryIntent(r.db.QueryRow(ctx, query, intentID, scope.Tenant, scope.Project, scope.Namespace))
}

var _ interface {
	ReadMemoryIntent(context.Context, memory.Scope, string) (memory.MemoryIntentRecord, error)
} = (*Repository)(nil)

var _ memory.MemoryIntentTargetValidator = (*Repository)(nil)

func (r *Repository) PromoteReviewedCandidate(ctx context.Context, input memory.ReviewedCandidatePromotionInput) error {
	if err := input.Scope.Validate(); err != nil {
		return err
	}
	if strings.TrimSpace(input.CandidateID) == "" || strings.TrimSpace(input.Reviewer) == "" {
		return fmt.Errorf("candidate id and reviewer are required")
	}
	const query = `
SELECT id, source_raw_event_id, tenant, project, namespace, class, content,
       confidence, importance, freshness, sensitivity, mutability,
       retention_class, status, created_at, updated_at
FROM candidate_memories
WHERE id::text = $1 AND tenant = $2 AND project = $3 AND namespace = $4`
	candidate, err := scanCandidate(r.db.QueryRow(ctx, query, input.CandidateID, input.Scope.Tenant, input.Scope.Project, input.Scope.Namespace))
	if err != nil {
		return fmt.Errorf("read reviewed candidate: %w", err)
	}
	if candidate.Status != governance.CandidateStatusPending {
		return fmt.Errorf("candidate status %q is not promotable", candidate.Status)
	}
	latest, found, err := r.GetLatestCanonicalByScopeAndClass(ctx, candidate.Scope, candidate.Class)
	if err != nil {
		return err
	}
	memoryID := uuid.NewString()
	if found {
		memoryID = latest.ID
	}
	_, _, err = r.PromoteCandidate(ctx, governance.CanonicalPromotion{Candidate: candidate, MemoryID: memoryID, VersionID: uuid.NewString(), Version: 1, CreatedAt: input.OccurredAt.UTC()})
	if err != nil {
		return err
	}
	_, err = r.TransitionCandidateStatus(ctx, governance.CandidateStatusTransition{CandidateID: candidate.ID, ToStatus: governance.CandidateStatusPromoted, UpdatedAt: input.OccurredAt.UTC()}, memory.ProvenanceRecord{ID: uuid.NewString(), Scope: candidate.Scope, RawEventID: candidate.SourceRawEventID, CandidateMemoryID: candidate.ID, MemoryID: memoryID, Actor: input.Reviewer, Operation: "review_accept_promote_candidate", CreatedAt: input.OccurredAt.UTC(), SourceContext: map[string]any{"reason": input.Reason}})
	return err
}

var _ memory.ReflectionReviewCanonicalIntegrator = (*Repository)(nil)

func scanMemoryIntent(s interface{ Scan(...any) error }) (memory.MemoryIntentRecord, error) {
	var out memory.MemoryIntentRecord
	var prov, payload []byte
	var targetID *string
	var targetVersion *int64
	if err := s.Scan(&out.ID, &out.Scope.Tenant, &out.Scope.Project, &out.Scope.Namespace, &out.Type, &out.Actor, &out.Reason, &prov, &out.RequestID, &out.OperationID, &out.IdempotencyKey, &targetID, &targetVersion, &payload, &out.Status, &out.CreatedAt); err != nil {
		return out, err
	}
	if targetID != nil {
		out.TargetMemoryID = *targetID
	}
	if targetVersion != nil {
		out.TargetVersion = *targetVersion
	}
	if len(prov) > 0 {
		_ = json.Unmarshal(prov, &out.Provenance)
	}
	if len(payload) > 0 {
		var p struct {
			Content string `json:"content"`
		}
		_ = json.Unmarshal(payload, &p)
		out.Content = p.Content
	}
	return out, nil
}
