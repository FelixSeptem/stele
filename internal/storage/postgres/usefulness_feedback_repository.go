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

func (r *Repository) CreateUsefulnessFeedback(ctx context.Context, feedback memory.UsefulnessFeedback) (memory.UsefulnessFeedback, error) {
	if err := feedback.Validate(); err != nil {
		return memory.UsefulnessFeedback{}, err
	}
	metadata, err := json.Marshal(normalizeAnyMap(feedback.Metadata))
	if err != nil {
		return memory.UsefulnessFeedback{}, fmt.Errorf("marshal usefulness feedback metadata: %w", err)
	}
	tx, err := r.tx.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return memory.UsefulnessFeedback{}, fmt.Errorf("begin usefulness feedback transaction: %w", err)
	}
	defer tx.Rollback(ctx)

	const feedbackQuery = `
INSERT INTO usefulness_feedback (
	id, tenant, project, namespace, feedback_type, source_surface, actor, reason,
	idempotency_key, metadata, created_at
)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
ON CONFLICT (tenant, project, namespace, idempotency_key)
WHERE idempotency_key IS NOT NULL
DO UPDATE SET idempotency_key = EXCLUDED.idempotency_key
RETURNING id, tenant, project, namespace, feedback_type, source_surface, actor, reason,
	idempotency_key, metadata, superseded_at, superseded_by_actor, superseded_by_reason, created_at
`
	created, err := scanUsefulnessFeedback(tx.QueryRow(
		ctx,
		feedbackQuery,
		feedback.ID,
		feedback.Scope.Tenant,
		feedback.Scope.Project,
		feedback.Scope.Namespace,
		feedback.Type,
		feedback.SourceSurface,
		feedback.Actor,
		feedback.Reason,
		nullableString(feedback.IdempotencyKey),
		metadata,
		feedback.CreatedAt,
	))
	if err != nil {
		return memory.UsefulnessFeedback{}, fmt.Errorf("create usefulness feedback: %w", err)
	}

	subjects := make([]memory.UsefulnessFeedbackSubject, 0, len(feedback.Subjects))
	for _, subject := range feedback.Subjects {
		createdSubject, err := createUsefulnessFeedbackSubject(ctx, tx, created, subject)
		if err != nil {
			return memory.UsefulnessFeedback{}, err
		}
		if createdSubject.Kind != "" {
			subjects = append(subjects, createdSubject)
		}
	}
	created.Subjects = subjects
	if len(created.Subjects) == 0 {
		records := []memory.UsefulnessFeedback{created}
		if err := attachUsefulnessFeedbackSubjects(ctx, tx, created.Scope, records); err != nil {
			return memory.UsefulnessFeedback{}, err
		}
		created = records[0]
	}

	if err := tx.Commit(ctx); err != nil {
		return memory.UsefulnessFeedback{}, fmt.Errorf("commit usefulness feedback transaction: %w", err)
	}
	return created, nil
}

func (r *Repository) SupersedeUsefulnessFeedback(ctx context.Context, input memory.SupersedeUsefulnessFeedbackInput) error {
	if err := input.Validate(); err != nil {
		return err
	}
	tx, err := r.tx.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return fmt.Errorf("begin usefulness feedback supersession transaction: %w", err)
	}
	defer tx.Rollback(ctx)

	const updateQuery = `
UPDATE usefulness_feedback
SET superseded_at = $7,
	superseded_by_actor = $5,
	superseded_by_reason = $6
WHERE tenant = $1
	AND project = $2
	AND namespace = $3
	AND id = $4
	AND superseded_at IS NULL
`
	tag, err := tx.Exec(
		ctx,
		updateQuery,
		input.Scope.Tenant,
		input.Scope.Project,
		input.Scope.Namespace,
		input.FeedbackID,
		input.Actor,
		input.Reason,
		input.SupersededAt,
	)
	if err != nil {
		return fmt.Errorf("supersede usefulness feedback: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("usefulness feedback not found or already superseded")
	}
	const auditQuery = `
INSERT INTO usefulness_feedback_supersessions (
	tenant, project, namespace, feedback_id, actor, reason, superseded_at
)
VALUES ($1, $2, $3, $4, $5, $6, $7)
`
	if _, err := tx.Exec(
		ctx,
		auditQuery,
		input.Scope.Tenant,
		input.Scope.Project,
		input.Scope.Namespace,
		input.FeedbackID,
		input.Actor,
		input.Reason,
		input.SupersededAt,
	); err != nil {
		return fmt.Errorf("insert usefulness feedback supersession audit: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit usefulness feedback supersession transaction: %w", err)
	}
	return nil
}

func (r *Repository) ReadUsefulnessFeedback(ctx context.Context, input memory.ReadUsefulnessFeedbackInput) (memory.UsefulnessFeedback, error) {
	if err := input.Validate(); err != nil {
		return memory.UsefulnessFeedback{}, err
	}
	const query = `
SELECT id, tenant, project, namespace, feedback_type, source_surface, actor, reason,
	idempotency_key, metadata, superseded_at, superseded_by_actor, superseded_by_reason, created_at
FROM usefulness_feedback
WHERE tenant = $1
	AND project = $2
	AND namespace = $3
	AND id = $4
`
	feedback, err := scanUsefulnessFeedback(r.db.QueryRow(ctx, query, input.Scope.Tenant, input.Scope.Project, input.Scope.Namespace, input.FeedbackID))
	if err != nil {
		return memory.UsefulnessFeedback{}, fmt.Errorf("read usefulness feedback: %w", err)
	}
	records := []memory.UsefulnessFeedback{feedback}
	if err := attachUsefulnessFeedbackSubjects(ctx, r.db, input.Scope, records); err != nil {
		return memory.UsefulnessFeedback{}, err
	}
	return records[0], nil
}

func (r *Repository) ListUsefulnessFeedback(ctx context.Context, input memory.ListUsefulnessFeedbackInput) ([]memory.UsefulnessFeedback, error) {
	if err := input.Validate(); err != nil {
		return nil, err
	}
	limit := input.Limit
	if limit == 0 {
		limit = 50
	}
	if input.Subject.Kind != "" {
		return r.listUsefulnessFeedbackForSubjectWithFilters(ctx, input.Scope, input.Subject, input.Type, input.IncludeSuperseded, limit)
	}
	const query = `
SELECT id, tenant, project, namespace, feedback_type, source_surface, actor, reason,
	idempotency_key, metadata, superseded_at, superseded_by_actor, superseded_by_reason, created_at
FROM usefulness_feedback uf
WHERE uf.tenant = $1
	AND uf.project = $2
	AND uf.namespace = $3
	AND ($4::text = '' OR uf.feedback_type = $4)
	AND ($5::boolean OR uf.superseded_at IS NULL)
ORDER BY uf.created_at DESC, uf.id DESC
LIMIT $6
`
	rows, err := r.db.Query(
		ctx,
		query,
		input.Scope.Tenant,
		input.Scope.Project,
		input.Scope.Namespace,
		string(input.Type),
		input.IncludeSuperseded,
		limit,
	)
	if err != nil {
		return nil, fmt.Errorf("list usefulness feedback: %w", err)
	}
	defer rows.Close()
	records, err := scanUsefulnessFeedbackRows(rows)
	if err != nil {
		return nil, err
	}
	if len(records) == 0 {
		return records, nil
	}
	if err := attachUsefulnessFeedbackSubjects(ctx, r.db, input.Scope, records); err != nil {
		return nil, err
	}
	return records, nil
}

func (r *Repository) SummarizeUsefulnessFeedback(ctx context.Context, input memory.SummarizeUsefulnessFeedbackInput) (memory.UsefulnessFeedbackSummary, error) {
	if err := input.Validate(); err != nil {
		return memory.UsefulnessFeedbackSummary{}, err
	}
	records, err := r.listUsefulnessFeedbackForSubject(ctx, input.Scope, input.Subject)
	if err != nil {
		return memory.UsefulnessFeedbackSummary{}, err
	}
	return memory.SummarizeUsefulnessFeedback(input.Subject, records), nil
}

func (r *Repository) listUsefulnessFeedbackForSubjectWithFilters(ctx context.Context, scope memory.Scope, subject memory.UsefulnessFeedbackSubject, feedbackType memory.UsefulnessFeedbackType, includeSuperseded bool, limit int) ([]memory.UsefulnessFeedback, error) {
	subjectID, expectedKind, expectedID, opaqueToken := usefulnessFeedbackSubjectStorage(subject)
	const query = `
SELECT uf.id, uf.tenant, uf.project, uf.namespace, uf.feedback_type, uf.source_surface, uf.actor, uf.reason,
	uf.idempotency_key, uf.metadata, uf.superseded_at, uf.superseded_by_actor, uf.superseded_by_reason, uf.created_at
FROM usefulness_feedback uf
JOIN usefulness_feedback_subjects ufs
	ON ufs.feedback_id = uf.id
	AND ufs.tenant = uf.tenant
	AND ufs.project = uf.project
	AND ufs.namespace = uf.namespace
WHERE uf.tenant = $1
	AND uf.project = $2
	AND uf.namespace = $3
	AND ufs.subject_kind = $4
	AND COALESCE(ufs.subject_id, '') = $5
	AND COALESCE(ufs.expected_recall_kind, '') = $6
	AND COALESCE(ufs.expected_recall_id, '') = $7
	AND COALESCE(ufs.opaque_token, '') = $8
	AND ($9::text = '' OR uf.feedback_type = $9)
	AND ($10::boolean OR uf.superseded_at IS NULL)
ORDER BY uf.created_at DESC, uf.id DESC
LIMIT $11
`
	rows, err := r.db.Query(
		ctx,
		query,
		scope.Tenant,
		scope.Project,
		scope.Namespace,
		subject.Kind,
		subjectID,
		expectedKind,
		expectedID,
		opaqueToken,
		string(feedbackType),
		includeSuperseded,
		limit,
	)
	if err != nil {
		return nil, fmt.Errorf("list usefulness feedback for subject: %w", err)
	}
	defer rows.Close()
	records, err := scanUsefulnessFeedbackRows(rows)
	if err != nil {
		return nil, err
	}
	if len(records) == 0 {
		return records, nil
	}
	if err := attachUsefulnessFeedbackSubjects(ctx, r.db, scope, records); err != nil {
		return nil, err
	}
	return records, nil
}

func (r *Repository) listUsefulnessFeedbackForSubject(ctx context.Context, scope memory.Scope, subject memory.UsefulnessFeedbackSubject) ([]memory.UsefulnessFeedback, error) {
	subjectID, expectedKind, expectedID, opaqueToken := usefulnessFeedbackSubjectStorage(subject)
	const query = `
SELECT uf.id, uf.tenant, uf.project, uf.namespace, uf.feedback_type, uf.source_surface, uf.actor, uf.reason,
	uf.idempotency_key, uf.metadata, uf.superseded_at, uf.superseded_by_actor, uf.superseded_by_reason, uf.created_at
FROM usefulness_feedback uf
JOIN usefulness_feedback_subjects ufs
	ON ufs.feedback_id = uf.id
	AND ufs.tenant = uf.tenant
	AND ufs.project = uf.project
	AND ufs.namespace = uf.namespace
WHERE uf.tenant = $1
	AND uf.project = $2
	AND uf.namespace = $3
	AND ufs.subject_kind = $4
	AND COALESCE(ufs.subject_id, '') = $5
	AND COALESCE(ufs.expected_recall_kind, '') = $6
	AND COALESCE(ufs.expected_recall_id, '') = $7
	AND COALESCE(ufs.opaque_token, '') = $8
ORDER BY uf.created_at ASC, uf.id ASC
`
	rows, err := r.db.Query(
		ctx,
		query,
		scope.Tenant,
		scope.Project,
		scope.Namespace,
		subject.Kind,
		subjectID,
		expectedKind,
		expectedID,
		opaqueToken,
	)
	if err != nil {
		return nil, fmt.Errorf("list usefulness feedback for subject: %w", err)
	}
	defer rows.Close()
	records := make([]memory.UsefulnessFeedback, 0)
	for rows.Next() {
		record, err := scanUsefulnessFeedback(rows)
		if err != nil {
			return nil, err
		}
		records = append(records, record)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate usefulness feedback for subject: %w", err)
	}
	if len(records) == 0 {
		return records, nil
	}
	if err := attachUsefulnessFeedbackSubjects(ctx, r.db, scope, records); err != nil {
		return nil, err
	}
	return records, nil
}

func scanUsefulnessFeedbackRows(rows pgx.Rows) ([]memory.UsefulnessFeedback, error) {
	records := make([]memory.UsefulnessFeedback, 0)
	for rows.Next() {
		record, err := scanUsefulnessFeedback(rows)
		if err != nil {
			return nil, err
		}
		records = append(records, record)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate usefulness feedback: %w", err)
	}
	return records, nil
}

func createUsefulnessFeedbackSubject(ctx context.Context, db queryRower, feedback memory.UsefulnessFeedback, subject memory.UsefulnessFeedbackSubject) (memory.UsefulnessFeedbackSubject, error) {
	subjectID, expectedKind, expectedID, opaqueToken := usefulnessFeedbackSubjectStorage(subject)
	const query = `
INSERT INTO usefulness_feedback_subjects (
	feedback_id, tenant, project, namespace, subject_kind, subject_id,
	expected_recall_kind, expected_recall_id, opaque_token
)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
ON CONFLICT DO NOTHING
RETURNING feedback_id, tenant, project, namespace, subject_kind, subject_id,
	expected_recall_kind, expected_recall_id, opaque_token
`
	created, err := scanUsefulnessFeedbackSubject(db.QueryRow(
		ctx,
		query,
		feedback.ID,
		feedback.Scope.Tenant,
		feedback.Scope.Project,
		feedback.Scope.Namespace,
		subject.Kind,
		nullableString(subjectID),
		nullableString(expectedKind),
		nullableString(expectedID),
		nullableString(opaqueToken),
	))
	if err != nil {
		if strings.Contains(err.Error(), "no rows in result set") {
			return memory.UsefulnessFeedbackSubject{}, nil
		}
		return memory.UsefulnessFeedbackSubject{}, fmt.Errorf("create usefulness feedback subject: %w", err)
	}
	return created, nil
}

func attachUsefulnessFeedbackSubjects(ctx context.Context, db queryRower, scope memory.Scope, records []memory.UsefulnessFeedback) error {
	ids := make([]string, 0, len(records))
	byID := make(map[string]int, len(records))
	for index, record := range records {
		ids = append(ids, record.ID)
		byID[record.ID] = index
	}
	const query = `
SELECT feedback_id, tenant, project, namespace, subject_kind, subject_id,
	expected_recall_kind, expected_recall_id, opaque_token
FROM usefulness_feedback_subjects
WHERE feedback_id = ANY($1)
	AND tenant = $2
	AND project = $3
	AND namespace = $4
ORDER BY feedback_id ASC, subject_kind ASC, subject_id ASC
`
	rows, err := db.Query(ctx, query, ids, scope.Tenant, scope.Project, scope.Namespace)
	if err != nil {
		return fmt.Errorf("list usefulness feedback subjects: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		feedbackID, subject, err := scanUsefulnessFeedbackSubjectWithID(rows)
		if err != nil {
			return err
		}
		index, ok := byID[feedbackID]
		if !ok {
			continue
		}
		records[index].Subjects = append(records[index].Subjects, subject)
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("iterate usefulness feedback subjects: %w", err)
	}
	return nil
}

func scanUsefulnessFeedback(scanner provenanceScanner) (memory.UsefulnessFeedback, error) {
	var feedback memory.UsefulnessFeedback
	var idempotencyKey sql.NullString
	var metadata []byte
	var supersededAt sql.NullTime
	var supersededByActor sql.NullString
	var supersededByReason sql.NullString
	if err := scanner.Scan(
		&feedback.ID,
		&feedback.Scope.Tenant,
		&feedback.Scope.Project,
		&feedback.Scope.Namespace,
		&feedback.Type,
		&feedback.SourceSurface,
		&feedback.Actor,
		&feedback.Reason,
		&idempotencyKey,
		&metadata,
		&supersededAt,
		&supersededByActor,
		&supersededByReason,
		&feedback.CreatedAt,
	); err != nil {
		return memory.UsefulnessFeedback{}, fmt.Errorf("scan usefulness feedback: %w", err)
	}
	if idempotencyKey.Valid {
		feedback.IdempotencyKey = idempotencyKey.String
	}
	feedback.Metadata = map[string]any{}
	if len(metadata) > 0 {
		if err := json.Unmarshal(metadata, &feedback.Metadata); err != nil {
			return memory.UsefulnessFeedback{}, fmt.Errorf("unmarshal usefulness feedback metadata: %w", err)
		}
	}
	if supersededAt.Valid {
		feedback.SupersededAt = supersededAt.Time
	}
	if supersededByActor.Valid {
		feedback.SupersededByActor = supersededByActor.String
	}
	if supersededByReason.Valid {
		feedback.SupersededByReason = supersededByReason.String
	}
	return feedback, nil
}

func scanUsefulnessFeedbackSubject(scanner provenanceScanner) (memory.UsefulnessFeedbackSubject, error) {
	_, subject, err := scanUsefulnessFeedbackSubjectWithID(scanner)
	return subject, err
}

func scanUsefulnessFeedbackSubjectWithID(scanner provenanceScanner) (string, memory.UsefulnessFeedbackSubject, error) {
	var feedbackID string
	var scope memory.Scope
	var subject memory.UsefulnessFeedbackSubject
	var subjectID sql.NullString
	var expectedKind sql.NullString
	var expectedID sql.NullString
	var opaqueToken sql.NullString
	if err := scanner.Scan(
		&feedbackID,
		&scope.Tenant,
		&scope.Project,
		&scope.Namespace,
		&subject.Kind,
		&subjectID,
		&expectedKind,
		&expectedID,
		&opaqueToken,
	); err != nil {
		return "", memory.UsefulnessFeedbackSubject{}, fmt.Errorf("scan usefulness feedback subject: %w", err)
	}
	if subjectID.Valid {
		subject.ID = subjectID.String
	}
	if expectedKind.Valid {
		subject.ExpectedRecallTarget.Kind = memory.ExpectedRecallTargetKind(expectedKind.String)
	}
	if expectedID.Valid {
		subject.ExpectedRecallTarget.ID = expectedID.String
	}
	if opaqueToken.Valid {
		subject.ExpectedRecallTarget.OpaqueToken = opaqueToken.String
	}
	return feedbackID, subject, nil
}

func usefulnessFeedbackSubjectStorage(subject memory.UsefulnessFeedbackSubject) (subjectID string, expectedKind string, expectedID string, opaqueToken string) {
	if subject.Kind != memory.UsefulnessFeedbackSubjectExpectedRecall {
		return strings.TrimSpace(subject.ID), "", "", ""
	}
	target := subject.ExpectedRecallTarget
	if target.Kind == memory.ExpectedRecallTargetOpaque {
		return "", string(target.Kind), "", strings.TrimSpace(target.OpaqueToken)
	}
	return "", string(target.Kind), strings.TrimSpace(target.ID), ""
}
