package postgres

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/FelixSeptem/stele/internal/memory"
)

func (r *Repository) CreateDerivedInsightReplayRun(ctx context.Context, run memory.DerivedInsightReplayRun) (memory.DerivedInsightReplayRun, error) {
	if err := run.Validate(); err != nil {
		return memory.DerivedInsightReplayRun{}, err
	}

	metadata, err := marshalDerivedInsightJSON(run.Request.Metadata)
	if err != nil {
		return memory.DerivedInsightReplayRun{}, fmt.Errorf("marshal derived insight replay metadata: %w", err)
	}

	const query = `
INSERT INTO derived_insight_replay_runs (
	id,
	tenant,
	project,
	namespace,
	mode,
	status,
	insight_types,
	evidence_window_start,
	evidence_window_end,
	evidence_limit,
	actor,
	reason,
	idempotency_key,
	request_metadata,
	created_at,
	updated_at
) VALUES (
	$1, $2, $3, $4, $5, $6, $7, $8,
	$9, $10, $11, $12, $13, $14, $15, $16
)
RETURNING
	id,
	tenant,
	project,
	namespace,
	mode,
	status,
	insight_types,
	evidence_window_start,
	evidence_window_end,
	evidence_limit,
	actor,
	reason,
	idempotency_key,
	request_metadata,
	report_counters,
	report_decisions,
	failure,
	report_generated_at,
	started_at,
	finished_at,
	created_at,
	updated_at
`

	created, err := scanDerivedInsightReplayRun(r.db.QueryRow(
		ctx,
		query,
		run.ID,
		run.Scope.Tenant,
		run.Scope.Project,
		run.Scope.Namespace,
		run.Mode,
		run.Status,
		derivedInsightTypeStrings(run.Request.InsightTypes),
		run.Request.EvidenceWindowStart,
		run.Request.EvidenceWindowEnd,
		run.Request.EvidenceLimit,
		run.Actor,
		run.Reason,
		nullableString(run.Request.IdempotencyKey),
		metadata,
		run.CreatedAt,
		run.UpdatedAt,
	))
	if err != nil {
		return memory.DerivedInsightReplayRun{}, fmt.Errorf("create derived insight replay run: %w", err)
	}
	return created, nil
}

func (r *Repository) ReadDerivedInsightReplayRun(ctx context.Context, input memory.ReadDerivedInsightReplayRunInput) (memory.DerivedInsightReplayRun, error) {
	if err := input.Validate(); err != nil {
		return memory.DerivedInsightReplayRun{}, err
	}

	const query = `
SELECT
	id,
	tenant,
	project,
	namespace,
	mode,
	status,
	insight_types,
	evidence_window_start,
	evidence_window_end,
	evidence_limit,
	actor,
	reason,
	idempotency_key,
	request_metadata,
	report_counters,
	report_decisions,
	failure,
	report_generated_at,
	started_at,
	finished_at,
	created_at,
	updated_at
FROM derived_insight_replay_runs
WHERE id = $1
	AND tenant = $2
	AND project = $3
	AND namespace = $4
`

	run, err := scanDerivedInsightReplayRun(r.db.QueryRow(ctx, query, input.RunID, input.Scope.Tenant, input.Scope.Project, input.Scope.Namespace))
	if err != nil {
		return memory.DerivedInsightReplayRun{}, fmt.Errorf("read derived insight replay run: %w", err)
	}
	return run, nil
}

func (r *Repository) ListDerivedInsightReplayRuns(ctx context.Context, input memory.ListDerivedInsightReplayRunsInput) ([]memory.DerivedInsightReplayRun, error) {
	if err := input.Validate(); err != nil {
		return nil, err
	}

	args := []any{input.Scope.Tenant, input.Scope.Project, input.Scope.Namespace}
	conditions := []string{"tenant = $1", "project = $2", "namespace = $3"}
	nextArg := 4
	if input.Status != "" {
		conditions = append(conditions, fmt.Sprintf("status = $%d", nextArg))
		args = append(args, input.Status)
		nextArg++
	}
	if input.Mode != "" {
		conditions = append(conditions, fmt.Sprintf("mode = $%d", nextArg))
		args = append(args, input.Mode)
		nextArg++
	}

	args = append(args, input.Limit)
	query := fmt.Sprintf(`
SELECT
	id,
	tenant,
	project,
	namespace,
	mode,
	status,
	insight_types,
	evidence_window_start,
	evidence_window_end,
	evidence_limit,
	actor,
	reason,
	idempotency_key,
	request_metadata,
	report_counters,
	report_decisions,
	failure,
	report_generated_at,
	started_at,
	finished_at,
	created_at,
	updated_at
FROM derived_insight_replay_runs
WHERE %s
ORDER BY updated_at DESC, id DESC
LIMIT $%d
`, strings.Join(conditions, "\n\tAND "), nextArg)

	rows, err := r.db.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("list derived insight replay runs: %w", err)
	}
	defer rows.Close()

	items := make([]memory.DerivedInsightReplayRun, 0)
	for rows.Next() {
		run, err := scanDerivedInsightReplayRun(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, run)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate derived insight replay runs: %w", err)
	}
	return items, nil
}

func (r *Repository) FindDerivedInsightReplayRunByIdempotencyKey(ctx context.Context, input memory.FindDerivedInsightReplayRunByIdempotencyKeyInput) (memory.DerivedInsightReplayRun, error) {
	if err := input.Validate(); err != nil {
		return memory.DerivedInsightReplayRun{}, err
	}

	const query = `
SELECT
	id,
	tenant,
	project,
	namespace,
	mode,
	status,
	insight_types,
	evidence_window_start,
	evidence_window_end,
	evidence_limit,
	actor,
	reason,
	idempotency_key,
	request_metadata,
	report_counters,
	report_decisions,
	failure,
	report_generated_at,
	started_at,
	finished_at,
	created_at,
	updated_at
FROM derived_insight_replay_runs
WHERE tenant = $1
	AND project = $2
	AND namespace = $3
	AND idempotency_key = $4
`

	run, err := scanDerivedInsightReplayRun(r.db.QueryRow(ctx, query, input.Scope.Tenant, input.Scope.Project, input.Scope.Namespace, input.IdempotencyKey))
	if err != nil {
		return memory.DerivedInsightReplayRun{}, fmt.Errorf("find derived insight replay run by idempotency key: %w", err)
	}
	return run, nil
}

func (r *Repository) UpdateDerivedInsightReplayRunStatus(ctx context.Context, input memory.UpdateDerivedInsightReplayRunStatusInput) error {
	if err := input.Validate(); err != nil {
		return err
	}

	const query = `
UPDATE derived_insight_replay_runs
SET
	status = $1,
	failure = $2,
	updated_at = $3,
	started_at = COALESCE($4, started_at),
	finished_at = COALESCE($5, finished_at)
WHERE id = $6
	AND tenant = $7
	AND project = $8
	AND namespace = $9
`

	tag, err := r.db.Exec(
		ctx,
		query,
		input.Status,
		nullableString(input.Failure),
		input.UpdatedAt,
		nullableTime(input.StartedAt),
		nullableTime(input.FinishedAt),
		input.RunID,
		input.Scope.Tenant,
		input.Scope.Project,
		input.Scope.Namespace,
	)
	if err != nil {
		return fmt.Errorf("update derived insight replay run status: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("derived insight replay run not found")
	}
	return nil
}

func (r *Repository) StoreDerivedInsightReplayReport(ctx context.Context, report memory.DerivedInsightReplayReport) error {
	if err := report.Validate(); err != nil {
		return err
	}

	counters, err := json.Marshal(report.Counters)
	if err != nil {
		return fmt.Errorf("marshal derived insight replay counters: %w", err)
	}
	decisions, err := json.Marshal(report.Decisions)
	if err != nil {
		return fmt.Errorf("marshal derived insight replay decisions: %w", err)
	}

	const query = `
UPDATE derived_insight_replay_runs
SET
	report_counters = $1,
	report_decisions = $2,
	failure = $3,
	report_generated_at = $4,
	updated_at = $5
WHERE id = $6
	AND tenant = $7
	AND project = $8
	AND namespace = $9
`

	tag, err := r.db.Exec(ctx, query, counters, decisions, nullableString(report.Failure), report.GeneratedAt, report.GeneratedAt, report.RunID, report.Scope.Tenant, report.Scope.Project, report.Scope.Namespace)
	if err != nil {
		return fmt.Errorf("store derived insight replay report: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("derived insight replay run not found")
	}
	return nil
}

func scanDerivedInsightReplayRun(scanner derivedInsightScanner) (memory.DerivedInsightReplayRun, error) {
	var run memory.DerivedInsightReplayRun
	var insightTypes []string
	var idempotencyKey sql.NullString
	var requestMetadata []byte
	var reportCounters []byte
	var reportDecisions []byte
	var failure sql.NullString
	var reportGeneratedAt sql.NullTime
	var startedAt sql.NullTime
	var finishedAt sql.NullTime

	if err := scanner.Scan(
		&run.ID,
		&run.Scope.Tenant,
		&run.Scope.Project,
		&run.Scope.Namespace,
		&run.Mode,
		&run.Status,
		&insightTypes,
		&run.Request.EvidenceWindowStart,
		&run.Request.EvidenceWindowEnd,
		&run.Request.EvidenceLimit,
		&run.Actor,
		&run.Reason,
		&idempotencyKey,
		&requestMetadata,
		&reportCounters,
		&reportDecisions,
		&failure,
		&reportGeneratedAt,
		&startedAt,
		&finishedAt,
		&run.CreatedAt,
		&run.UpdatedAt,
	); err != nil {
		return memory.DerivedInsightReplayRun{}, err
	}

	run.Request.Scope = run.Scope
	run.Request.Mode = run.Mode
	run.Request.InsightTypes = derivedInsightTypes(insightTypes)
	run.Request.Actor = run.Actor
	run.Request.Reason = run.Reason
	run.Request.IdempotencyKey = idempotencyKey.String
	run.Request.RequestedAt = run.CreatedAt
	run.Failure = failure.String
	if startedAt.Valid {
		run.StartedAt = startedAt.Time
	}
	if finishedAt.Valid {
		run.FinishedAt = finishedAt.Time
	}
	if len(requestMetadata) > 0 {
		if err := json.Unmarshal(requestMetadata, &run.Request.Metadata); err != nil {
			return memory.DerivedInsightReplayRun{}, fmt.Errorf("unmarshal derived insight replay metadata: %w", err)
		}
	}
	if reportGeneratedAt.Valid {
		report := memory.DerivedInsightReplayReport{
			RunID:       run.ID,
			Scope:       run.Scope,
			Failure:     run.Failure,
			GeneratedAt: reportGeneratedAt.Time,
		}
		if len(reportCounters) > 0 {
			if err := json.Unmarshal(reportCounters, &report.Counters); err != nil {
				return memory.DerivedInsightReplayRun{}, fmt.Errorf("unmarshal derived insight replay counters: %w", err)
			}
		}
		if len(reportDecisions) > 0 {
			if err := json.Unmarshal(reportDecisions, &report.Decisions); err != nil {
				return memory.DerivedInsightReplayRun{}, fmt.Errorf("unmarshal derived insight replay decisions: %w", err)
			}
		}
		run.Report = &report
	}

	return run, nil
}

func derivedInsightTypeStrings(types []memory.DerivedInsightType) []string {
	values := make([]string, 0, len(types))
	for _, insightType := range types {
		values = append(values, string(insightType))
	}
	return values
}

func derivedInsightTypes(values []string) []memory.DerivedInsightType {
	types := make([]memory.DerivedInsightType, 0, len(values))
	for _, value := range values {
		types = append(types, memory.DerivedInsightType(value))
	}
	return types
}
