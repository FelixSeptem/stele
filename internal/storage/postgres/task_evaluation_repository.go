package postgres

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/FelixSeptem/stele/internal/memory"
	"github.com/jackc/pgx/v5"
)

func (r *Repository) CreateTaskEvaluation(ctx context.Context, evaluation memory.TaskEvaluation) (memory.TaskEvaluation, error) {
	if err := evaluation.Validate(); err != nil {
		return memory.TaskEvaluation{}, err
	}

	tx, err := r.tx.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return memory.TaskEvaluation{}, fmt.Errorf("begin task evaluation transaction: %w", err)
	}
	defer tx.Rollback(ctx)

	metadata, err := json.Marshal(normalizeAnyMap(evaluation.Metadata))
	if err != nil {
		return memory.TaskEvaluation{}, fmt.Errorf("marshal task evaluation metadata: %w", err)
	}

	const query = `
INSERT INTO task_evaluations (
	id, tenant, project, namespace, objective, success_criteria, verdict, contribution_categories,
	actor, reason, idempotency_key, metadata, correction_state, superseded_at,
	superseded_by_task_evaluation_id, superseded_by_actor, superseded_by_reason,
	created_at, updated_at
)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17, $18, $19)
ON CONFLICT (tenant, project, namespace, idempotency_key)
WHERE idempotency_key IS NOT NULL
DO UPDATE SET idempotency_key = EXCLUDED.idempotency_key
RETURNING id, tenant, project, namespace, objective, success_criteria, verdict, contribution_categories,
	actor, reason, idempotency_key, metadata, correction_state, superseded_at,
	superseded_by_task_evaluation_id, superseded_by_actor, superseded_by_reason,
	created_at, updated_at
`
	created, err := scanTaskEvaluation(tx.QueryRow(
		ctx,
		query,
		evaluation.ID,
		evaluation.Scope.Tenant,
		evaluation.Scope.Project,
		evaluation.Scope.Namespace,
		evaluation.Objective,
		evaluation.SuccessCriteria,
		evaluation.Verdict,
		taskContributionCategoryStrings(evaluation.ContributionCategories),
		evaluation.Actor,
		evaluation.Reason,
		nullableString(evaluation.IdempotencyKey),
		metadata,
		nullableString(strings.TrimSpace(string(evaluation.CorrectionState))),
		nullableTime(evaluation.SupersededAt),
		nullableString(evaluation.SupersededByTaskEvaluationID),
		nullableString(evaluation.SupersededByActor),
		nullableString(evaluation.SupersededByReason),
		evaluation.CreatedAt,
		evaluation.UpdatedAt,
	))
	if err != nil {
		return memory.TaskEvaluation{}, fmt.Errorf("create task evaluation: %w", err)
	}

	links := make([]memory.TaskEvidenceLink, 0, len(evaluation.Evidence))
	for _, link := range evaluation.Evidence {
		createdLink, err := createTaskEvidenceLink(ctx, tx, created, link)
		if err != nil {
			return memory.TaskEvaluation{}, err
		}
		if createdLink.Kind != "" {
			links = append(links, createdLink)
		}
	}
	if len(links) > 0 {
		created.Evidence = links
	}

	if err := tx.Commit(ctx); err != nil {
		return memory.TaskEvaluation{}, fmt.Errorf("commit task evaluation transaction: %w", err)
	}

	return created, nil
}

func (r *Repository) ReadTaskEvaluation(ctx context.Context, input memory.ReadTaskEvaluationInput) (memory.TaskEvaluation, error) {
	if err := input.Validate(); err != nil {
		return memory.TaskEvaluation{}, err
	}

	const query = `
SELECT id, tenant, project, namespace, objective, success_criteria, verdict, contribution_categories,
	actor, reason, idempotency_key, metadata, correction_state, superseded_at,
	superseded_by_task_evaluation_id, superseded_by_actor, superseded_by_reason,
	created_at, updated_at
FROM task_evaluations
WHERE tenant = $1 AND project = $2 AND namespace = $3 AND id = $4
`
	evaluation, err := scanTaskEvaluation(r.db.QueryRow(ctx, query, input.Scope.Tenant, input.Scope.Project, input.Scope.Namespace, input.EvaluationID))
	if err != nil {
		return memory.TaskEvaluation{}, fmt.Errorf("read task evaluation: %w", err)
	}

	links, err := listTaskEvidenceLinks(ctx, r.db, evaluation.Scope, evaluation.ID)
	if err != nil {
		return memory.TaskEvaluation{}, err
	}
	evaluation.Evidence = links
	return evaluation, nil
}

func (r *Repository) ListTaskEvaluations(ctx context.Context, input memory.ListTaskEvaluationsInput) ([]memory.TaskEvaluation, error) {
	if err := input.Validate(); err != nil {
		return nil, err
	}

	limit := input.Limit
	if limit <= 0 {
		limit = 50
	}

	args := []any{
		input.Scope.Tenant,
		input.Scope.Project,
		input.Scope.Namespace,
		string(input.Verdict),
		string(input.ContributionCategory),
		string(input.EvidenceTargetKind),
		strings.TrimSpace(input.EvidenceTargetID),
		input.IncludeSuperseded,
		limit,
	}
	const query = `
SELECT id, tenant, project, namespace, objective, success_criteria, verdict, contribution_categories,
	actor, reason, idempotency_key, metadata, correction_state, superseded_at,
	superseded_by_task_evaluation_id, superseded_by_actor, superseded_by_reason,
	created_at, updated_at
FROM task_evaluations te
WHERE te.tenant = $1
	AND te.project = $2
	AND te.namespace = $3
	AND ($4::text = '' OR te.verdict = $4)
	AND ($5::text = '' OR $5 = ANY(te.contribution_categories))
	AND (
		$6::text = ''
		OR EXISTS (
			SELECT 1
			FROM task_evidence_links tel
			WHERE tel.task_evaluation_id = te.id
				AND tel.tenant = te.tenant
				AND tel.project = te.project
				AND tel.namespace = te.namespace
				AND tel.evidence_kind = $6
				AND ($7::text = '' OR COALESCE(tel.evidence_id, '') = $7 OR COALESCE(tel.opaque_token, '') = $7)
		)
	)
	AND ($8::boolean OR te.superseded_at IS NULL)
ORDER BY te.created_at DESC, te.id DESC
LIMIT $9
`
	rows, err := r.db.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("list task evaluations: %w", err)
	}
	defer rows.Close()

	records := make([]memory.TaskEvaluation, 0)
	for rows.Next() {
		record, err := scanTaskEvaluation(rows)
		if err != nil {
			return nil, err
		}
		records = append(records, record)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate task evaluations: %w", err)
	}

	if len(records) == 0 {
		return records, nil
	}
	if err := attachTaskEvidenceLinks(ctx, r.db, records); err != nil {
		return nil, err
	}
	return records, nil
}

func (r *Repository) SupersedeTaskEvaluation(ctx context.Context, input memory.SupersedeTaskEvaluationInput) error {
	if err := input.Validate(); err != nil {
		return err
	}

	tx, err := r.tx.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return fmt.Errorf("begin task evaluation supersession transaction: %w", err)
	}
	defer tx.Rollback(ctx)

	const updateQuery = `
UPDATE task_evaluations
SET correction_state = $6,
	superseded_at = $7,
	superseded_by_actor = $4,
	superseded_by_reason = $5,
	updated_at = $7
WHERE tenant = $1 AND project = $2 AND namespace = $3 AND id = $8 AND superseded_at IS NULL
`
	tag, err := tx.Exec(ctx, updateQuery, input.Scope.Tenant, input.Scope.Project, input.Scope.Namespace, input.Actor, input.Reason, string(memory.TaskEvaluationCorrectionStateSuperseded), input.SupersededAt, input.EvaluationID)
	if err != nil {
		return fmt.Errorf("supersede task evaluation: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("task evaluation not found or already superseded")
	}

	const auditQuery = `
INSERT INTO task_evaluation_supersessions (
	tenant, project, namespace, task_evaluation_id, superseding_task_evaluation_id, actor, reason, superseded_at
)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
`
	if _, err := tx.Exec(ctx, auditQuery, input.Scope.Tenant, input.Scope.Project, input.Scope.Namespace, input.EvaluationID, nullableString(input.SupersedingID), input.Actor, input.Reason, input.SupersededAt); err != nil {
		return fmt.Errorf("insert task evaluation supersession audit: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit task evaluation supersession transaction: %w", err)
	}

	return nil
}

func (r *Repository) SummarizeTaskEvaluations(ctx context.Context, input memory.SummarizeTaskEvaluationsInput) (memory.TaskEvaluationSummary, error) {
	if err := input.Validate(); err != nil {
		return memory.TaskEvaluationSummary{}, err
	}
	records, err := r.ListTaskEvaluations(ctx, memory.ListTaskEvaluationsInput{
		Scope:             input.Scope,
		EvidenceTargetKind: input.EvidenceTargetKind,
		EvidenceTargetID:   input.EvidenceTargetID,
		IncludeSuperseded:  false,
		Limit:             1000,
	})
	if err != nil {
		return memory.TaskEvaluationSummary{}, err
	}
	return memory.SummarizeTaskEvaluations(input, records), nil
}

func createTaskEvidenceLink(ctx context.Context, db queryRower, evaluation memory.TaskEvaluation, link memory.TaskEvidenceLink) (memory.TaskEvidenceLink, error) {
	metadata, err := json.Marshal(normalizeAnyMap(link.Metadata))
	if err != nil {
		return memory.TaskEvidenceLink{}, fmt.Errorf("marshal task evidence metadata: %w", err)
	}
	const query = `
INSERT INTO task_evidence_links (
	task_evaluation_id, tenant, project, namespace, evidence_kind, evidence_id, opaque_token, metadata
)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
ON CONFLICT DO NOTHING
RETURNING evidence_kind, evidence_id, opaque_token, metadata
`
	created, err := scanTaskEvidenceLink(db.QueryRow(ctx, query, evaluation.ID, evaluation.Scope.Tenant, evaluation.Scope.Project, evaluation.Scope.Namespace, link.Kind, nullableString(link.ID), nullableString(link.OpaqueToken), metadata))
	if err != nil {
		if strings.Contains(err.Error(), "no rows in result set") {
			return memory.TaskEvidenceLink{}, nil
		}
		return memory.TaskEvidenceLink{}, fmt.Errorf("create task evidence link: %w", err)
	}
	return created, nil
}

func listTaskEvidenceLinks(ctx context.Context, db queryRower, scope memory.Scope, evaluationID string) ([]memory.TaskEvidenceLink, error) {
	const query = `
SELECT evidence_kind, evidence_id, opaque_token, metadata
FROM task_evidence_links
WHERE task_evaluation_id = $1 AND tenant = $2 AND project = $3 AND namespace = $4
ORDER BY created_at ASC
`
	rows, err := db.Query(ctx, query, evaluationID, scope.Tenant, scope.Project, scope.Namespace)
	if err != nil {
		return nil, fmt.Errorf("list task evidence links: %w", err)
	}
	defer rows.Close()

	links := make([]memory.TaskEvidenceLink, 0)
	for rows.Next() {
		link, err := scanTaskEvidenceLink(rows)
		if err != nil {
			return nil, err
		}
		links = append(links, link)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate task evidence links: %w", err)
	}
	return links, nil
}

func attachTaskEvidenceLinks(ctx context.Context, db queryRower, records []memory.TaskEvaluation) error {
	ids := make([]string, 0, len(records))
	byID := make(map[string]int, len(records))
	for index, record := range records {
		ids = append(ids, record.ID)
		byID[record.ID] = index
	}

	const query = `
SELECT task_evaluation_id, evidence_kind, evidence_id, opaque_token, metadata
FROM task_evidence_links
WHERE task_evaluation_id = ANY($1)
ORDER BY task_evaluation_id ASC, created_at ASC
`
	rows, err := db.Query(ctx, query, ids)
	if err != nil {
		return fmt.Errorf("list task evidence links: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var evaluationID string
		link, err := scanTaskEvidenceLinkWithID(rows, &evaluationID)
		if err != nil {
			return err
		}
		if index, ok := byID[evaluationID]; ok {
			records[index].Evidence = append(records[index].Evidence, link)
		}
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("iterate task evidence links: %w", err)
	}
	return nil
}

func scanTaskEvaluation(scanner provenanceScanner) (memory.TaskEvaluation, error) {
	var evaluation memory.TaskEvaluation
	var successCriteria []string
	var contributionCategories []string
	var idempotencyKey sql.NullString
	var metadata []byte
	var correctionState sql.NullString
	var supersededAt sql.NullTime
	var supersededByEvaluationID sql.NullString
	var supersededByActor sql.NullString
	var supersededByReason sql.NullString
	if err := scanner.Scan(
		&evaluation.ID,
		&evaluation.Scope.Tenant,
		&evaluation.Scope.Project,
		&evaluation.Scope.Namespace,
		&evaluation.Objective,
		&successCriteria,
		&evaluation.Verdict,
		&contributionCategories,
		&evaluation.Actor,
		&evaluation.Reason,
		&idempotencyKey,
		&metadata,
		&correctionState,
		&supersededAt,
		&supersededByEvaluationID,
		&supersededByActor,
		&supersededByReason,
		&evaluation.CreatedAt,
		&evaluation.UpdatedAt,
	); err != nil {
		return memory.TaskEvaluation{}, fmt.Errorf("scan task evaluation: %w", err)
	}
	evaluation.SuccessCriteria = successCriteria
	evaluation.ContributionCategories = taskContributionCategoryValues(contributionCategories)
	if idempotencyKey.Valid {
		evaluation.IdempotencyKey = idempotencyKey.String
	}
	evaluation.Metadata = map[string]any{}
	if len(metadata) > 0 {
		if err := json.Unmarshal(metadata, &evaluation.Metadata); err != nil {
			return memory.TaskEvaluation{}, fmt.Errorf("unmarshal task evaluation metadata: %w", err)
		}
	}
	if correctionState.Valid {
		evaluation.CorrectionState = memory.TaskEvaluationCorrectionState(correctionState.String)
	}
	if supersededAt.Valid {
		evaluation.SupersededAt = supersededAt.Time
	}
	if supersededByEvaluationID.Valid {
		evaluation.SupersededByTaskEvaluationID = supersededByEvaluationID.String
	}
	if supersededByActor.Valid {
		evaluation.SupersededByActor = supersededByActor.String
	}
	if supersededByReason.Valid {
		evaluation.SupersededByReason = supersededByReason.String
	}
	return evaluation, nil
}

func scanTaskEvidenceLink(scanner provenanceScanner) (memory.TaskEvidenceLink, error) {
	var link memory.TaskEvidenceLink
	var evidenceID sql.NullString
	var opaqueToken sql.NullString
	var metadata []byte
	if err := scanner.Scan(&link.Kind, &evidenceID, &opaqueToken, &metadata); err != nil {
		return memory.TaskEvidenceLink{}, fmt.Errorf("scan task evidence link: %w", err)
	}
	if evidenceID.Valid {
		link.ID = evidenceID.String
	}
	if opaqueToken.Valid {
		link.OpaqueToken = opaqueToken.String
	}
	link.Metadata = map[string]any{}
	if len(metadata) > 0 {
		if err := json.Unmarshal(metadata, &link.Metadata); err != nil {
			return memory.TaskEvidenceLink{}, fmt.Errorf("unmarshal task evidence metadata: %w", err)
		}
	}
	return link, nil
}

func scanTaskEvidenceLinkWithID(scanner provenanceScanner, evaluationID *string) (memory.TaskEvidenceLink, error) {
	var link memory.TaskEvidenceLink
	var evidenceID sql.NullString
	var opaqueToken sql.NullString
	var metadata []byte
	if err := scanner.Scan(evaluationID, &link.Kind, &evidenceID, &opaqueToken, &metadata); err != nil {
		return memory.TaskEvidenceLink{}, fmt.Errorf("scan task evidence link: %w", err)
	}
	if evidenceID.Valid {
		link.ID = evidenceID.String
	}
	if opaqueToken.Valid {
		link.OpaqueToken = opaqueToken.String
	}
	link.Metadata = map[string]any{}
	if len(metadata) > 0 {
		if err := json.Unmarshal(metadata, &link.Metadata); err != nil {
			return memory.TaskEvidenceLink{}, fmt.Errorf("unmarshal task evidence metadata: %w", err)
		}
	}
	return link, nil
}

func taskContributionCategoryStrings(values []memory.TaskContributionCategory) []string {
	items := make([]string, 0, len(values))
	for _, value := range values {
		items = append(items, string(value))
	}
	return items
}

func taskContributionCategoryValues(values []string) []memory.TaskContributionCategory {
	items := make([]memory.TaskContributionCategory, 0, len(values))
	for _, value := range values {
		items = append(items, memory.TaskContributionCategory(value))
	}
	return items
}
