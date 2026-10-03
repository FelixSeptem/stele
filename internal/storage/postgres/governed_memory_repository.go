package postgres

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/FelixSeptem/stele/internal/governance"
	"github.com/FelixSeptem/stele/internal/memory"
	"github.com/FelixSeptem/stele/internal/workqueue"
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
	var pathErr error
	record.MemoryPath, pathErr = memory.NormalizeMemoryPath(record.MemoryPath)
	if pathErr != nil {
		return memory.MemoryIntentRecord{}, pathErr
	}
	payload, _ := json.Marshal(map[string]any{"content": record.Content, "target_memory_id": record.TargetMemoryID, "target_version": record.TargetVersion, "memory_path": record.MemoryPath, "target_insight_id": record.TargetInsightID, "evidence": record.Evidence, "policy_version": record.PolicyVersion, "precedence_version": record.PrecedenceVersion, "precedence_stage": record.PrecedenceStage, "precedence_outcome": record.PrecedenceOutcome})
	prov, _ := json.Marshal(record.Provenance)
	fp := record.RequestFingerprint
	if strings.TrimSpace(fp) == "" {
		fingerprint := sha256.Sum256(payload)
		fp = hex.EncodeToString(fingerprint[:])
	}
	if record.MemoryPath == memory.MemoryPathRoot && record.TargetInsightID == "" && len(record.Evidence) == 0 && record.PolicyVersion == "" && record.OutcomeReference == "" && record.PrecedenceVersion == "" && record.PrecedenceStage == "" && record.PrecedenceOutcome == "" {
		const legacy = `INSERT INTO memory_intents (id,tenant,project,namespace,intent_type,actor,reason,provenance,request_id,operation_id,idempotency_key,request_fingerprint,target_memory_id,target_version,payload,status,created_at) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,NULLIF($13,''),NULLIF($14,0),$15,$16,$17) RETURNING id,tenant,project,namespace,intent_type,actor,reason,provenance,request_id,operation_id,idempotency_key,target_memory_id,target_version,payload,status,created_at`
		row := r.db.QueryRow(ctx, legacy, record.ID, record.Scope.Tenant, record.Scope.Project, record.Scope.Namespace, record.Type, record.Actor, record.Reason, prov, record.RequestID, record.OperationID, record.IdempotencyKey, fp, record.TargetMemoryID, record.TargetVersion, payload, record.Status, record.CreatedAt)
		created, err := scanMemoryIntentLegacy(row)
		if err == nil {
			created.MemoryPath = memory.MemoryPathRoot
			created.RequestFingerprint = fp
			return created, nil
		}
		const existing = `SELECT request_fingerprint,id,tenant,project,namespace,intent_type,actor,reason,provenance,request_id,operation_id,idempotency_key,target_memory_id,target_version,payload,status,created_at FROM memory_intents WHERE tenant=$1 AND project=$2 AND namespace=$3 AND (idempotency_key=$4 OR operation_id=$5) ORDER BY created_at LIMIT 1`
		existingRecord, existingFingerprint, lookupErr := scanMemoryIntentWithFingerprintLegacy(r.db.QueryRow(ctx, existing, record.Scope.Tenant, record.Scope.Project, record.Scope.Namespace, record.IdempotencyKey, record.OperationID))
		if lookupErr != nil {
			return memory.MemoryIntentRecord{}, fmt.Errorf("append memory intent: %w", err)
		}
		if existingFingerprint != fp {
			return memory.MemoryIntentRecord{}, memory.ErrIdempotencyConflict
		}
		existingRecord.MemoryPath = memory.MemoryPathRoot
		existingRecord.RequestFingerprint = existingFingerprint
		existingRecord.Status = memory.MemoryIntentStatusReplayed
		return existingRecord, nil
	}
	evidenceRefs, _ := json.Marshal(record.Evidence)
	const q = `INSERT INTO memory_intents (id,tenant,project,namespace,memory_path,intent_type,actor,reason,provenance,request_id,operation_id,idempotency_key,request_fingerprint,target_memory_id,target_version,target_insight_id,evidence_refs,outcome_reference,policy_version,precedence_version,precedence_stage,precedence_outcome,payload,status,created_at)
VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,NULLIF($14,''),NULLIF($15,0),NULLIF($16,''),$17,NULLIF($18,''),NULLIF($19,''),$20,$21,$22,$23,$24,$25)
ON CONFLICT DO NOTHING
RETURNING id,tenant,project,namespace,memory_path,intent_type,actor,reason,provenance,request_id,operation_id,idempotency_key,target_memory_id,target_version,target_insight_id,evidence_refs,outcome_reference,policy_version,precedence_version,precedence_stage,precedence_outcome,payload,status,created_at`
	row := r.db.QueryRow(ctx, q, record.ID, record.Scope.Tenant, record.Scope.Project, record.Scope.Namespace, record.MemoryPath, record.Type, record.Actor, record.Reason, prov, record.RequestID, record.OperationID, record.IdempotencyKey, fp, record.TargetMemoryID, record.TargetVersion, record.TargetInsightID, evidenceRefs, record.OutcomeReference, record.PolicyVersion, record.PrecedenceVersion, record.PrecedenceStage, record.PrecedenceOutcome, payload, record.Status, record.CreatedAt)
	created, err := scanMemoryIntent(row)
	if err == nil {
		created.RequestFingerprint = fp
		return created, nil
	}
	const existing = `SELECT request_fingerprint,id,tenant,project,namespace,memory_path,intent_type,actor,reason,provenance,request_id,operation_id,idempotency_key,target_memory_id,target_version,target_insight_id,evidence_refs,outcome_reference,policy_version,precedence_version,precedence_stage,precedence_outcome,payload,status,created_at FROM memory_intents WHERE tenant=$1 AND project=$2 AND namespace=$3 AND (idempotency_key=$4 OR operation_id=$5) ORDER BY created_at LIMIT 1`
	existingRecord, existingFingerprint, lookupErr := scanMemoryIntentWithFingerprint(r.db.QueryRow(ctx, existing, record.Scope.Tenant, record.Scope.Project, record.Scope.Namespace, record.IdempotencyKey, record.OperationID))
	if lookupErr != nil {
		return memory.MemoryIntentRecord{}, fmt.Errorf("append memory intent: %w", err)
	}
	if existingFingerprint != fp {
		return memory.MemoryIntentRecord{}, memory.ErrIdempotencyConflict
	}
	existingRecord.RequestFingerprint = existingFingerprint
	existingRecord.Status = memory.MemoryIntentStatusReplayed
	return existingRecord, nil
}

// EnqueueMemoryIntent hands the immutable intent to the existing durable work
// queue. The queue carries only the intent reference and stable identity.
func (r *Repository) EnqueueMemoryIntent(ctx context.Context, record memory.MemoryIntentRecord) error {
	now := record.CreatedAt
	if now.IsZero() {
		now = time.Now().UTC()
	}
	_, err := r.EnqueueDerivedWork(ctx, workqueue.EnqueueInput{DerivedWorkInput: workqueue.DerivedWorkInput{
		Scope: record.Scope, Kind: workqueue.WorkKindMemoryIntent,
		Watermark: record.RequestFingerprint, Idempotency: record.IdempotencyKey, Reference: record.ID,
	}, MaxAttempts: 3, Now: now, DetailExpiresAt: now.Add(24 * time.Hour)})
	if err != nil {
		return fmt.Errorf("enqueue governed intent work: %w", err)
	}
	return nil
}

func (r *Repository) AppendMemoryIntentTransition(ctx context.Context, transition memory.MemoryIntentTransition) error {
	if err := transition.Validate(); err != nil {
		return err
	}
	if strings.TrimSpace(transition.ID) == "" {
		transition.ID = uuid.NewString()
	}
	const query = `INSERT INTO memory_intent_transitions (id,intent_id,tenant,project,namespace,sequence,from_status,to_status,actor,reason,diagnostic_category,work_reference,outcome_reference,occurred_at) VALUES ($1,$2,$3,$4,$5,$6,NULLIF($7,''),$8,$9,$10,$11,NULLIF($12,''),NULLIF($13,''),$14) ON CONFLICT (intent_id, sequence) DO NOTHING`
	_, err := r.db.Exec(ctx, query, transition.ID, transition.IntentID, transition.Scope.Tenant, transition.Scope.Project, transition.Scope.Namespace, transition.Sequence, transition.From, transition.To, transition.Actor, transition.Reason, transition.DiagnosticCategory, transition.WorkReference, transition.OutcomeReference, transition.OccurredAt.UTC())
	if err != nil {
		return fmt.Errorf("append memory intent transition: %w", err)
	}
	return nil
}

func (r *Repository) ReadMemoryIntentHistory(ctx context.Context, scope memory.Scope, intentID string) (memory.MemoryIntentHistory, error) {
	intent, err := r.ReadMemoryIntent(ctx, scope, intentID)
	if err != nil {
		return memory.MemoryIntentHistory{}, err
	}
	const query = `SELECT id,intent_id,tenant,project,namespace,sequence,from_status,to_status,actor,reason,diagnostic_category,work_reference,outcome_reference,occurred_at FROM memory_intent_transitions WHERE intent_id::text=$1 AND tenant=$2 AND project=$3 AND namespace=$4 ORDER BY sequence ASC`
	rows, err := r.db.Query(ctx, query, intentID, scope.Tenant, scope.Project, scope.Namespace)
	if err != nil {
		return memory.MemoryIntentHistory{}, fmt.Errorf("read memory intent history: %w", err)
	}
	defer rows.Close()
	history := memory.MemoryIntentHistory{Intent: intent}
	for rows.Next() {
		var t memory.MemoryIntentTransition
		var from, work, outcome sql.NullString
		if err := rows.Scan(&t.ID, &t.IntentID, &t.Scope.Tenant, &t.Scope.Project, &t.Scope.Namespace, &t.Sequence, &from, &t.To, &t.Actor, &t.Reason, &t.DiagnosticCategory, &work, &outcome, &t.OccurredAt); err != nil {
			return memory.MemoryIntentHistory{}, err
		}
		t.From, t.WorkReference, t.OutcomeReference = memory.MemoryIntentStatus(from.String), work.String, outcome.String
		history.Transitions = append(history.Transitions, t)
	}
	return history, rows.Err()
}

func (r *Repository) ListMemoryIntents(ctx context.Context, input memory.MemoryIntentListInput) ([]memory.MemoryIntentRecord, error) {
	if err := input.Validate(); err != nil {
		return nil, err
	}
	const query = `SELECT id,tenant,project,namespace,memory_path,intent_type,actor,reason,provenance,request_id,operation_id,idempotency_key,target_memory_id,target_version,target_insight_id,evidence_refs,outcome_reference,policy_version,precedence_version,precedence_stage,precedence_outcome,payload,status,created_at FROM memory_intents WHERE tenant=$1 AND project=$2 AND namespace=$3 AND ($4::timestamptz IS NULL OR created_at < $4) ORDER BY created_at DESC,id DESC LIMIT $5`
	var cursor any
	if !input.Cursor.IsZero() {
		cursor = input.Cursor.UTC()
	}
	rows, err := r.db.Query(ctx, query, input.Scope.Tenant, input.Scope.Project, input.Scope.Namespace, cursor, input.Limit)
	if err != nil {
		return nil, fmt.Errorf("list memory intents: %w", err)
	}
	defer rows.Close()
	items := make([]memory.MemoryIntentRecord, 0, input.Limit)
	for rows.Next() {
		item, err := scanMemoryIntent(rows)
		if err != nil {
			return nil, fmt.Errorf("scan memory intent: %w", err)
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return items, nil
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
SELECT id, tenant, project, namespace, memory_path, intent_type, actor, reason, provenance,
       request_id, operation_id, idempotency_key, target_memory_id,
       target_version, target_insight_id, evidence_refs, outcome_reference,
       policy_version, precedence_version, precedence_stage, precedence_outcome, payload, status, created_at
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
SELECT id, source_raw_event_id, tenant, project, namespace, memory_path, class, content,
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
	var targetInsight, outcome, policyVersion, precedenceVersion, precedenceStage, precedenceOutcome *string
	var evidenceRefs []byte
	if err := s.Scan(&out.ID, &out.Scope.Tenant, &out.Scope.Project, &out.Scope.Namespace, &out.MemoryPath, &out.Type, &out.Actor, &out.Reason, &prov, &out.RequestID, &out.OperationID, &out.IdempotencyKey, &targetID, &targetVersion, &targetInsight, &evidenceRefs, &outcome, &policyVersion, &precedenceVersion, &precedenceStage, &precedenceOutcome, &payload, &out.Status, &out.CreatedAt); err != nil {
		return out, err
	}
	if targetID != nil {
		out.TargetMemoryID = *targetID
	}
	if targetVersion != nil {
		out.TargetVersion = *targetVersion
	}
	if targetInsight != nil {
		out.TargetInsightID = *targetInsight
	}
	if outcome != nil {
		out.OutcomeReference = *outcome
	}
	if policyVersion != nil {
		out.PolicyVersion = *policyVersion
	}
	if precedenceVersion != nil {
		out.PrecedenceVersion = *precedenceVersion
	}
	if precedenceStage != nil {
		out.PrecedenceStage = memory.OperationPrecedenceStage(*precedenceStage)
	}
	if precedenceOutcome != nil {
		out.PrecedenceOutcome = memory.OperationPrecedenceOutcome(*precedenceOutcome)
	}
	if len(evidenceRefs) > 0 {
		_ = json.Unmarshal(evidenceRefs, &out.Evidence)
	}
	if len(prov) > 0 {
		_ = json.Unmarshal(prov, &out.Provenance)
	}
	if len(payload) > 0 {
		var p struct {
			Content         string                        `json:"content"`
			MemoryPath      string                        `json:"memory_path"`
			TargetInsightID string                        `json:"target_insight_id"`
			Evidence        []memory.MemoryIntentEvidence `json:"evidence"`
			PolicyVersion   string                        `json:"policy_version"`
		}
		_ = json.Unmarshal(payload, &p)
		out.Content = p.Content
		if out.MemoryPath == "" {
			out.MemoryPath = p.MemoryPath
		}
		out.TargetInsightID, out.Evidence, out.PolicyVersion = p.TargetInsightID, p.Evidence, p.PolicyVersion
	}
	return out, nil
}

func scanMemoryIntentLegacy(s interface{ Scan(...any) error }) (memory.MemoryIntentRecord, error) {
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

func scanMemoryIntentWithFingerprint(s interface{ Scan(...any) error }) (memory.MemoryIntentRecord, string, error) {
	var fingerprint string
	var out memory.MemoryIntentRecord
	var prov, payload []byte
	var targetID *string
	var targetVersion *int64
	var targetInsight, outcome, policyVersion, precedenceVersion, precedenceStage, precedenceOutcome *string
	var evidenceRefs []byte
	if err := s.Scan(&fingerprint, &out.ID, &out.Scope.Tenant, &out.Scope.Project, &out.Scope.Namespace, &out.MemoryPath, &out.Type, &out.Actor, &out.Reason, &prov, &out.RequestID, &out.OperationID, &out.IdempotencyKey, &targetID, &targetVersion, &targetInsight, &evidenceRefs, &outcome, &policyVersion, &precedenceVersion, &precedenceStage, &precedenceOutcome, &payload, &out.Status, &out.CreatedAt); err != nil {
		return out, "", err
	}
	if targetID != nil {
		out.TargetMemoryID = *targetID
	}
	if targetVersion != nil {
		out.TargetVersion = *targetVersion
	}
	if targetInsight != nil {
		out.TargetInsightID = *targetInsight
	}
	if outcome != nil {
		out.OutcomeReference = *outcome
	}
	if policyVersion != nil {
		out.PolicyVersion = *policyVersion
	}
	if precedenceVersion != nil {
		out.PrecedenceVersion = *precedenceVersion
	}
	if precedenceStage != nil {
		out.PrecedenceStage = memory.OperationPrecedenceStage(*precedenceStage)
	}
	if precedenceOutcome != nil {
		out.PrecedenceOutcome = memory.OperationPrecedenceOutcome(*precedenceOutcome)
	}
	if len(evidenceRefs) > 0 {
		_ = json.Unmarshal(evidenceRefs, &out.Evidence)
	}
	if len(prov) > 0 {
		_ = json.Unmarshal(prov, &out.Provenance)
	}
	if len(payload) > 0 {
		var p struct {
			Content         string                        `json:"content"`
			MemoryPath      string                        `json:"memory_path"`
			TargetInsightID string                        `json:"target_insight_id"`
			Evidence        []memory.MemoryIntentEvidence `json:"evidence"`
			PolicyVersion   string                        `json:"policy_version"`
		}
		_ = json.Unmarshal(payload, &p)
		out.Content = p.Content
		if out.MemoryPath == "" {
			out.MemoryPath = p.MemoryPath
		}
		out.TargetInsightID, out.Evidence, out.PolicyVersion = p.TargetInsightID, p.Evidence, p.PolicyVersion
	}
	return out, fingerprint, nil
}

func scanMemoryIntentWithFingerprintLegacy(s interface{ Scan(...any) error }) (memory.MemoryIntentRecord, string, error) {
	var fp string
	out, err := scanMemoryIntentLegacy(&fingerprintScanner{scanner: s, fp: &fp})
	return out, fp, err
}

type fingerprintScanner struct {
	scanner interface{ Scan(...any) error }
	fp      *string
}

func (s *fingerprintScanner) Scan(dest ...any) error {
	all := append([]any{s.fp}, dest...)
	return s.scanner.Scan(all...)
}
