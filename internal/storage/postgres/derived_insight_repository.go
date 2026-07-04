package postgres

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/FelixSeptem/stele/internal/insights"
	"github.com/FelixSeptem/stele/internal/memory"
	"github.com/jackc/pgx/v5"
)

func (r *Repository) UpsertDerivedInsight(ctx context.Context, insight memory.DerivedInsight) (memory.DerivedInsight, error) {
	if err := insight.Validate(); err != nil {
		return memory.DerivedInsight{}, err
	}

	tx, err := r.tx.BeginTx(ctx, pgxTxOptions())
	if err != nil {
		return memory.DerivedInsight{}, fmt.Errorf("begin derived insight transaction: %w", err)
	}
	defer tx.Rollback(ctx)

	payload, err := marshalDerivedInsightJSON(insight.Payload)
	if err != nil {
		return memory.DerivedInsight{}, fmt.Errorf("marshal derived insight payload: %w", err)
	}
	lesson, err := marshalDerivedInsightNullableJSON(insight.Lesson)
	if err != nil {
		return memory.DerivedInsight{}, fmt.Errorf("marshal derived insight lesson: %w", err)
	}
	derivationMetadata, err := marshalDerivedInsightJSON(insight.Derivation.Metadata)
	if err != nil {
		return memory.DerivedInsight{}, fmt.Errorf("marshal derived insight derivation metadata: %w", err)
	}

	const query = `
INSERT INTO derived_insights (
	id,
	tenant,
	project,
	namespace,
	type,
	lifecycle_state,
	title,
	summary,
	confidence,
	confidence_method,
	payload,
	lesson,
	derivation_source,
	derivation_fingerprint,
	derivation_metadata,
	evidence_window_start,
	evidence_window_end,
	derived_at,
	last_observed_at,
	created_at,
	updated_at
) VALUES (
	$1, $2, $3, $4, $5, $6, $7, $8, $9, $10,
	$11, $12, $13, $14, $15, $16, $17, $18, $19, $20, $21
)
ON CONFLICT (tenant, project, namespace, type, derivation_fingerprint)
DO UPDATE SET
	lifecycle_state = EXCLUDED.lifecycle_state,
	title = EXCLUDED.title,
	summary = EXCLUDED.summary,
	confidence = EXCLUDED.confidence,
	confidence_method = EXCLUDED.confidence_method,
	payload = EXCLUDED.payload,
	lesson = EXCLUDED.lesson,
	derivation_source = EXCLUDED.derivation_source,
	derivation_metadata = EXCLUDED.derivation_metadata,
	evidence_window_start = EXCLUDED.evidence_window_start,
	evidence_window_end = EXCLUDED.evidence_window_end,
	derived_at = EXCLUDED.derived_at,
	last_observed_at = EXCLUDED.last_observed_at,
	updated_at = EXCLUDED.updated_at
RETURNING
	id,
	tenant,
	project,
	namespace,
	type,
	lifecycle_state,
	title,
	summary,
	confidence,
	confidence_method,
	payload,
	lesson,
	derivation_source,
	derivation_fingerprint,
	derivation_metadata,
	evidence_window_start,
	evidence_window_end,
	derived_at,
	last_observed_at,
	created_at,
	updated_at
`

	upserted, err := scanDerivedInsight(tx.QueryRow(
		ctx,
		query,
		insight.ID,
		insight.Scope.Tenant,
		insight.Scope.Project,
		insight.Scope.Namespace,
		insight.Type,
		insight.State,
		insight.Title,
		insight.Summary,
		insight.Confidence.Score,
		insight.Confidence.Method,
		payload,
		lesson,
		insight.Derivation.Source,
		insight.Derivation.Fingerprint,
		derivationMetadata,
		insight.Derivation.EvidenceWindowStart,
		insight.Derivation.EvidenceWindowEnd,
		insight.Derivation.DerivedAt,
		insight.LastObservedAt,
		insight.CreatedAt,
		insight.UpdatedAt,
	))
	if err != nil {
		return memory.DerivedInsight{}, fmt.Errorf("upsert derived insight: %w", err)
	}

	if err := replaceDerivedInsightEvidence(ctx, tx, upserted.Scope, upserted.ID, insight.Evidence); err != nil {
		return memory.DerivedInsight{}, err
	}

	if err := tx.Commit(ctx); err != nil {
		return memory.DerivedInsight{}, fmt.Errorf("commit derived insight transaction: %w", err)
	}

	upserted.Evidence = append([]memory.DerivedInsightEvidenceRef(nil), insight.Evidence...)
	return upserted, nil
}

func (r *Repository) ListDerivedInsights(ctx context.Context, input memory.ListDerivedInsightsInput) ([]memory.DerivedInsight, error) {
	if err := input.Validate(); err != nil {
		return nil, err
	}

	args := []any{input.Scope.Tenant, input.Scope.Project, input.Scope.Namespace}
	conditions := []string{
		"di.tenant = $1",
		"di.project = $2",
		"di.namespace = $3",
	}
	nextArg := 4
	if input.Type != "" {
		conditions = append(conditions, fmt.Sprintf("di.type = $%d", nextArg))
		args = append(args, input.Type)
		nextArg++
	}
	if input.State != "" {
		conditions = append(conditions, fmt.Sprintf("di.lifecycle_state = $%d", nextArg))
		args = append(args, input.State)
		nextArg++
	} else if !input.IncludeHidden {
		conditions = append(conditions, "di.lifecycle_state NOT IN ('suppressed', 'forgotten', 'deleted')")
	}
	if input.MinConfidence != nil {
		conditions = append(conditions, fmt.Sprintf("di.confidence >= $%d", nextArg))
		args = append(args, *input.MinConfidence)
		nextArg++
	}
	if input.MinEvidenceCount > 0 {
		conditions = append(conditions, fmt.Sprintf(`(
			SELECT count(*)
			FROM derived_insight_evidence die
			WHERE die.insight_id = di.id
				AND die.tenant = di.tenant
				AND die.project = di.project
				AND die.namespace = di.namespace
		) >= $%d`, nextArg))
		args = append(args, input.MinEvidenceCount)
		nextArg++
	}

	args = append(args, input.Limit)
	query := fmt.Sprintf(`
SELECT
	di.id,
	di.tenant,
	di.project,
	di.namespace,
	di.type,
	di.lifecycle_state,
	di.title,
	di.summary,
	di.confidence,
	di.confidence_method,
	di.payload,
	di.lesson,
	di.derivation_source,
	di.derivation_fingerprint,
	di.derivation_metadata,
	di.evidence_window_start,
	di.evidence_window_end,
	di.derived_at,
	di.last_observed_at,
	di.created_at,
	di.updated_at,
	(
		SELECT count(*)
		FROM derived_insight_evidence die
		WHERE die.insight_id = di.id
			AND die.tenant = di.tenant
			AND die.project = di.project
			AND die.namespace = di.namespace
	) AS evidence_count
FROM derived_insights di
WHERE %s
ORDER BY di.updated_at DESC, di.confidence DESC
LIMIT $%d
`, strings.Join(conditions, "\n\tAND "), nextArg)

	rows, err := r.db.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("list derived insights: %w", err)
	}
	defer rows.Close()

	items := make([]memory.DerivedInsight, 0)
	for rows.Next() {
		insight, _, err := scanDerivedInsightWithEvidenceCount(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, insight)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate derived insights: %w", err)
	}

	return items, nil
}

func (r *Repository) ReadDerivedInsight(ctx context.Context, input memory.ReadDerivedInsightInput) (memory.DerivedInsightDetail, error) {
	if err := input.Validate(); err != nil {
		return memory.DerivedInsightDetail{}, err
	}

	const query = `
SELECT
	id,
	tenant,
	project,
	namespace,
	type,
	lifecycle_state,
	title,
	summary,
	confidence,
	confidence_method,
	payload,
	lesson,
	derivation_source,
	derivation_fingerprint,
	derivation_metadata,
	evidence_window_start,
	evidence_window_end,
	derived_at,
	last_observed_at,
	created_at,
	updated_at
FROM derived_insights
WHERE id = $1
	AND tenant = $2
	AND project = $3
	AND namespace = $4
	AND ($5 OR lifecycle_state NOT IN ('suppressed', 'forgotten', 'deleted'))
`

	insight, err := scanDerivedInsight(r.db.QueryRow(ctx, query, input.ID, input.Scope.Tenant, input.Scope.Project, input.Scope.Namespace, input.IncludeHidden))
	if err != nil {
		return memory.DerivedInsightDetail{}, fmt.Errorf("read derived insight: %w", err)
	}

	evidence, err := r.ListDerivedInsightEvidence(ctx, input.Scope, input.ID)
	if err != nil {
		return memory.DerivedInsightDetail{}, err
	}
	lifecycle, err := r.ListDerivedInsightLifecycle(ctx, input.Scope, input.ID)
	if err != nil {
		return memory.DerivedInsightDetail{}, err
	}
	insight.Evidence = evidence

	return memory.DerivedInsightDetail{
		Insight:   insight,
		Evidence:  evidence,
		Lifecycle: lifecycle,
	}, nil
}

func (r *Repository) ListDerivedInsightEvidence(ctx context.Context, scope memory.Scope, insightID string) ([]memory.DerivedInsightEvidenceRef, error) {
	if err := scope.Validate(); err != nil {
		return nil, err
	}
	if strings.TrimSpace(insightID) == "" {
		return nil, fmt.Errorf("derived insight id is required")
	}

	const query = `
SELECT
	evidence_kind,
	evidence_id,
	relation,
	observed_at,
	metadata
FROM derived_insight_evidence
WHERE insight_id = $1
	AND tenant = $2
	AND project = $3
	AND namespace = $4
ORDER BY observed_at ASC NULLS LAST, created_at ASC
`

	rows, err := r.db.Query(ctx, query, insightID, scope.Tenant, scope.Project, scope.Namespace)
	if err != nil {
		return nil, fmt.Errorf("list derived insight evidence: %w", err)
	}
	defer rows.Close()

	items := make([]memory.DerivedInsightEvidenceRef, 0)
	for rows.Next() {
		ref, err := scanDerivedInsightEvidenceRef(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, ref)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate derived insight evidence: %w", err)
	}

	return items, nil
}

func (r *Repository) ListDerivedInsightLifecycle(ctx context.Context, scope memory.Scope, insightID string) ([]memory.DerivedInsightLifecycleRecord, error) {
	if err := scope.Validate(); err != nil {
		return nil, err
	}
	if strings.TrimSpace(insightID) == "" {
		return nil, fmt.Errorf("derived insight id is required")
	}

	const query = `
SELECT
	id,
	insight_id,
	tenant,
	project,
	namespace,
	from_state,
	to_state,
	actor,
	reason,
	occurred_at,
	metadata
FROM derived_insight_lifecycle_ledger
WHERE insight_id = $1
	AND tenant = $2
	AND project = $3
	AND namespace = $4
ORDER BY occurred_at ASC
`

	rows, err := r.db.Query(ctx, query, insightID, scope.Tenant, scope.Project, scope.Namespace)
	if err != nil {
		return nil, fmt.Errorf("list derived insight lifecycle: %w", err)
	}
	defer rows.Close()

	items := make([]memory.DerivedInsightLifecycleRecord, 0)
	for rows.Next() {
		record, err := scanDerivedInsightLifecycleRecord(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, record)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate derived insight lifecycle: %w", err)
	}

	return items, nil
}

func (r *Repository) TransitionDerivedInsightLifecycle(ctx context.Context, transition memory.DerivedInsightLifecycleTransition) error {
	if err := transition.Validate(); err != nil {
		return err
	}

	tx, err := r.tx.BeginTx(ctx, pgxTxOptions())
	if err != nil {
		return fmt.Errorf("begin derived insight lifecycle transaction: %w", err)
	}
	defer tx.Rollback(ctx)

	const updateQuery = `
UPDATE derived_insights
SET lifecycle_state = $1,
	updated_at = $2
WHERE id = $3
	AND tenant = $4
	AND project = $5
	AND namespace = $6
	AND lifecycle_state = $7
`

	tag, err := tx.Exec(
		ctx,
		updateQuery,
		transition.ToState,
		transition.OccurredAt,
		transition.InsightID,
		transition.Scope.Tenant,
		transition.Scope.Project,
		transition.Scope.Namespace,
		transition.FromState,
	)
	if err != nil {
		return fmt.Errorf("update derived insight lifecycle: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("derived insight not found or lifecycle state mismatch")
	}

	if err := insertDerivedInsightLifecycle(ctx, tx, transition); err != nil {
		return err
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit derived insight lifecycle transaction: %w", err)
	}

	return nil
}

func (r *Repository) ListFailureEvidence(ctx context.Context, scope memory.Scope, limit int) ([]insights.FailureEvidence, error) {
	if err := scope.Validate(); err != nil {
		return nil, err
	}
	if limit <= 0 {
		return nil, fmt.Errorf("limit must be greater than zero")
	}

	const query = `
SELECT
	evidence_kind,
	evidence_id,
	failure_key,
	message,
	observed_at,
	metadata
FROM (
	SELECT
		'raw_event'::text AS evidence_kind,
		id::text AS evidence_id,
		event_type || ':' || governance_last_error AS failure_key,
		governance_last_error AS message,
		COALESCE(governance_last_failed_at, created_at) AS observed_at,
		jsonb_build_object('event_type', event_type) AS metadata
	FROM raw_events
	WHERE tenant = $1
		AND project = $2
		AND namespace = $3
		AND governance_last_error IS NOT NULL
	UNION ALL
	SELECT
		'job_execution'::text AS evidence_kind,
		id::text AS evidence_id,
		job_name || ':' || error_message AS failure_key,
		error_message AS message,
		COALESCE(finished_at, started_at) AS observed_at,
		jsonb_build_object('job_name', job_name, 'trigger_source', trigger_source) AS metadata
	FROM job_executions
	WHERE tenant = $4
		AND project = $5
		AND namespace = $6
		AND status = 'failed'
		AND error_message IS NOT NULL
	UNION ALL
	SELECT
		'embedding_rebuild'::text AS evidence_kind,
		memory_id::text AS evidence_id,
		'embedding_rebuild:' || failure_reason AS failure_key,
		failure_reason AS message,
		COALESCE(last_attempted_at, requested_at) AS observed_at,
		jsonb_build_object('requested_provider', requested_provider, 'requested_model', requested_model) AS metadata
	FROM embedding_rebuilds
	WHERE tenant = $7
		AND project = $8
		AND namespace = $9
		AND status = 'failed'
		AND failure_reason IS NOT NULL
	UNION ALL
	SELECT
		'recovery_record'::text AS evidence_kind,
		id::text AS evidence_id,
		action || ':' || reason AS failure_key,
		reason AS message,
		created_at AS observed_at,
		jsonb_build_object('action', action, 'actor', actor) AS metadata
	FROM governance_recovery_ledger
	WHERE tenant = $10
		AND project = $11
		AND namespace = $12
	UNION ALL
	SELECT
		CASE class
			WHEN 'procedural' THEN 'procedural_memory'
			WHEN 'summary' THEN 'summary_memory'
			WHEN 'relation' THEN 'relation_memory'
			ELSE 'canonical_memory'
		END AS evidence_kind,
		id::text AS evidence_id,
		class || ':' || content AS failure_key,
		content AS message,
		updated_at AS observed_at,
		jsonb_build_object('class', class, 'state', state) AS metadata
	FROM canonical_memories
	WHERE tenant = $13
		AND project = $14
		AND namespace = $15
		AND state = 'active'
		AND (
			content ILIKE '%fail%'
			OR content ILIKE '%error%'
			OR content ILIKE '%avoid%'
		)
) evidence
ORDER BY observed_at DESC
LIMIT $16
`

	rows, err := r.db.Query(
		ctx,
		query,
		scope.Tenant, scope.Project, scope.Namespace,
		scope.Tenant, scope.Project, scope.Namespace,
		scope.Tenant, scope.Project, scope.Namespace,
		scope.Tenant, scope.Project, scope.Namespace,
		scope.Tenant, scope.Project, scope.Namespace,
		limit,
	)
	if err != nil {
		return nil, fmt.Errorf("list failure evidence: %w", err)
	}
	defer rows.Close()

	items := make([]insights.FailureEvidence, 0)
	for rows.Next() {
		item, err := scanFailureEvidence(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate failure evidence: %w", err)
	}

	return items, nil
}

func replaceDerivedInsightEvidence(ctx context.Context, db queryRower, scope memory.Scope, insightID string, evidence []memory.DerivedInsightEvidenceRef) error {
	const deleteQuery = `
DELETE FROM derived_insight_evidence
WHERE insight_id = $1
	AND tenant = $2
	AND project = $3
	AND namespace = $4
`

	if _, err := db.Exec(ctx, deleteQuery, insightID, scope.Tenant, scope.Project, scope.Namespace); err != nil {
		return fmt.Errorf("delete derived insight evidence: %w", err)
	}

	const insertQuery = `
INSERT INTO derived_insight_evidence (
	insight_id,
	tenant,
	project,
	namespace,
	evidence_kind,
	evidence_id,
	relation,
	observed_at,
	metadata
) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
ON CONFLICT (insight_id, evidence_kind, evidence_id, relation)
DO UPDATE SET
	observed_at = EXCLUDED.observed_at,
	metadata = EXCLUDED.metadata
`

	for _, ref := range evidence {
		metadata, err := marshalDerivedInsightJSON(ref.Metadata)
		if err != nil {
			return fmt.Errorf("marshal derived insight evidence metadata: %w", err)
		}
		if _, err := db.Exec(ctx, insertQuery, insightID, scope.Tenant, scope.Project, scope.Namespace, ref.Kind, ref.ID, ref.Relation, ref.ObservedAt, metadata); err != nil {
			return fmt.Errorf("insert derived insight evidence: %w", err)
		}
	}

	return nil
}

func insertDerivedInsightLifecycle(ctx context.Context, db queryRower, transition memory.DerivedInsightLifecycleTransition) error {
	metadata, err := marshalDerivedInsightJSON(transition.Metadata)
	if err != nil {
		return fmt.Errorf("marshal derived insight lifecycle metadata: %w", err)
	}

	const query = `
INSERT INTO derived_insight_lifecycle_ledger (
	insight_id,
	tenant,
	project,
	namespace,
	from_state,
	to_state,
	actor,
	reason,
	metadata,
	occurred_at
) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
`

	if _, err := db.Exec(
		ctx,
		query,
		transition.InsightID,
		transition.Scope.Tenant,
		transition.Scope.Project,
		transition.Scope.Namespace,
		transition.FromState,
		transition.ToState,
		transition.Actor,
		transition.Reason,
		metadata,
		transition.OccurredAt,
	); err != nil {
		return fmt.Errorf("insert derived insight lifecycle: %w", err)
	}

	return nil
}

type derivedInsightScanner interface {
	Scan(dest ...any) error
}

func scanDerivedInsight(scanner derivedInsightScanner) (memory.DerivedInsight, error) {
	var insight memory.DerivedInsight
	var confidenceMethod sql.NullString
	var payload []byte
	var lesson []byte
	var derivationMetadata []byte
	var evidenceWindowStart sql.NullTime
	var evidenceWindowEnd sql.NullTime
	var lastObservedAt sql.NullTime

	if err := scanner.Scan(
		&insight.ID,
		&insight.Scope.Tenant,
		&insight.Scope.Project,
		&insight.Scope.Namespace,
		&insight.Type,
		&insight.State,
		&insight.Title,
		&insight.Summary,
		&insight.Confidence.Score,
		&confidenceMethod,
		&payload,
		&lesson,
		&insight.Derivation.Source,
		&insight.Derivation.Fingerprint,
		&derivationMetadata,
		&evidenceWindowStart,
		&evidenceWindowEnd,
		&insight.Derivation.DerivedAt,
		&lastObservedAt,
		&insight.CreatedAt,
		&insight.UpdatedAt,
	); err != nil {
		return memory.DerivedInsight{}, err
	}

	insight.Confidence.Method = confidenceMethod.String
	if evidenceWindowStart.Valid {
		insight.Derivation.EvidenceWindowStart = evidenceWindowStart.Time
	}
	if evidenceWindowEnd.Valid {
		insight.Derivation.EvidenceWindowEnd = evidenceWindowEnd.Time
	}
	if lastObservedAt.Valid {
		insight.LastObservedAt = lastObservedAt.Time
	}
	if len(payload) > 0 {
		if err := json.Unmarshal(payload, &insight.Payload); err != nil {
			return memory.DerivedInsight{}, fmt.Errorf("unmarshal derived insight payload: %w", err)
		}
	}
	if len(lesson) > 0 {
		var parsed memory.DerivedInsightLesson
		if err := json.Unmarshal(lesson, &parsed); err != nil {
			return memory.DerivedInsight{}, fmt.Errorf("unmarshal derived insight lesson: %w", err)
		}
		insight.Lesson = &parsed
	}
	if len(derivationMetadata) > 0 {
		if err := json.Unmarshal(derivationMetadata, &insight.Derivation.Metadata); err != nil {
			return memory.DerivedInsight{}, fmt.Errorf("unmarshal derived insight derivation metadata: %w", err)
		}
	}

	return insight, nil
}

func scanDerivedInsightWithEvidenceCount(scanner derivedInsightScanner) (memory.DerivedInsight, int, error) {
	var insight memory.DerivedInsight
	var confidenceMethod sql.NullString
	var payload []byte
	var lesson []byte
	var derivationMetadata []byte
	var evidenceWindowStart sql.NullTime
	var evidenceWindowEnd sql.NullTime
	var lastObservedAt sql.NullTime
	var evidenceCount int

	if err := scanner.Scan(
		&insight.ID,
		&insight.Scope.Tenant,
		&insight.Scope.Project,
		&insight.Scope.Namespace,
		&insight.Type,
		&insight.State,
		&insight.Title,
		&insight.Summary,
		&insight.Confidence.Score,
		&confidenceMethod,
		&payload,
		&lesson,
		&insight.Derivation.Source,
		&insight.Derivation.Fingerprint,
		&derivationMetadata,
		&evidenceWindowStart,
		&evidenceWindowEnd,
		&insight.Derivation.DerivedAt,
		&lastObservedAt,
		&insight.CreatedAt,
		&insight.UpdatedAt,
		&evidenceCount,
	); err != nil {
		return memory.DerivedInsight{}, 0, err
	}

	insight.Confidence.Method = confidenceMethod.String
	if evidenceWindowStart.Valid {
		insight.Derivation.EvidenceWindowStart = evidenceWindowStart.Time
	}
	if evidenceWindowEnd.Valid {
		insight.Derivation.EvidenceWindowEnd = evidenceWindowEnd.Time
	}
	if lastObservedAt.Valid {
		insight.LastObservedAt = lastObservedAt.Time
	}
	if len(payload) > 0 {
		if err := json.Unmarshal(payload, &insight.Payload); err != nil {
			return memory.DerivedInsight{}, 0, fmt.Errorf("unmarshal derived insight payload: %w", err)
		}
	}
	if len(lesson) > 0 {
		var parsed memory.DerivedInsightLesson
		if err := json.Unmarshal(lesson, &parsed); err != nil {
			return memory.DerivedInsight{}, 0, fmt.Errorf("unmarshal derived insight lesson: %w", err)
		}
		insight.Lesson = &parsed
	}
	if len(derivationMetadata) > 0 {
		if err := json.Unmarshal(derivationMetadata, &insight.Derivation.Metadata); err != nil {
			return memory.DerivedInsight{}, 0, fmt.Errorf("unmarshal derived insight derivation metadata: %w", err)
		}
	}

	return insight, evidenceCount, nil
}

func scanDerivedInsightEvidenceRef(scanner derivedInsightScanner) (memory.DerivedInsightEvidenceRef, error) {
	var ref memory.DerivedInsightEvidenceRef
	var observedAt sql.NullTime
	var metadata []byte
	if err := scanner.Scan(&ref.Kind, &ref.ID, &ref.Relation, &observedAt, &metadata); err != nil {
		return memory.DerivedInsightEvidenceRef{}, err
	}
	if observedAt.Valid {
		ref.ObservedAt = observedAt.Time
	}
	if len(metadata) > 0 {
		if err := json.Unmarshal(metadata, &ref.Metadata); err != nil {
			return memory.DerivedInsightEvidenceRef{}, fmt.Errorf("unmarshal derived insight evidence metadata: %w", err)
		}
	}
	return ref, nil
}

func scanDerivedInsightLifecycleRecord(scanner derivedInsightScanner) (memory.DerivedInsightLifecycleRecord, error) {
	var record memory.DerivedInsightLifecycleRecord
	var fromState sql.NullString
	var metadata []byte
	if err := scanner.Scan(
		&record.ID,
		&record.InsightID,
		&record.Scope.Tenant,
		&record.Scope.Project,
		&record.Scope.Namespace,
		&fromState,
		&record.ToState,
		&record.Actor,
		&record.Reason,
		&record.OccurredAt,
		&metadata,
	); err != nil {
		return memory.DerivedInsightLifecycleRecord{}, err
	}
	record.FromState = memory.DerivedInsightState(fromState.String)
	if len(metadata) > 0 {
		if err := json.Unmarshal(metadata, &record.Metadata); err != nil {
			return memory.DerivedInsightLifecycleRecord{}, fmt.Errorf("unmarshal derived insight lifecycle metadata: %w", err)
		}
	}
	return record, nil
}

func scanFailureEvidence(scanner derivedInsightScanner) (insights.FailureEvidence, error) {
	var item insights.FailureEvidence
	var observedAt sql.NullTime
	var metadata []byte
	if err := scanner.Scan(&item.Kind, &item.ID, &item.FailureKey, &item.Message, &observedAt, &metadata); err != nil {
		return insights.FailureEvidence{}, err
	}
	if observedAt.Valid {
		item.ObservedAt = observedAt.Time
	}
	if len(metadata) > 0 {
		if err := json.Unmarshal(metadata, &item.Metadata); err != nil {
			return insights.FailureEvidence{}, fmt.Errorf("unmarshal failure evidence metadata: %w", err)
		}
	}
	return item, nil
}

func marshalDerivedInsightJSON(value map[string]any) ([]byte, error) {
	if value == nil {
		value = map[string]any{}
	}
	return json.Marshal(value)
}

func marshalDerivedInsightNullableJSON(value any) ([]byte, error) {
	if value == nil {
		return nil, nil
	}
	return json.Marshal(value)
}

func pgxTxOptions() pgx.TxOptions {
	return pgx.TxOptions{}
}
