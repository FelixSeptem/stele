package postgres

import (
	"context"
	"database/sql"
	"fmt"
	"strings"

	"github.com/FelixSeptem/stele/internal/jobs"
	"github.com/FelixSeptem/stele/internal/memory"
	"github.com/jackc/pgx/v5"
	"github.com/riverqueue/river"
	"github.com/riverqueue/river/riverdriver/riverpgxv5"
	"github.com/riverqueue/river/rivertype"
)

type riverIntentJobInserter interface {
	InsertTx(context.Context, pgx.Tx, river.JobArgs, *river.InsertOpts) (*rivertype.JobInsertResult, error)
}

func (r *Repository) SetMemoryIntentRiverInserter(inserter riverIntentJobInserter) {
	r.riverIntentJobs = inserter
}

func (r *Repository) insertMemoryIntentRiverJob(ctx context.Context, tx pgx.Tx, intentID string) (int64, error) {
	if r.riverIntentJobs == nil {
		return 0, fmt.Errorf("memory-intent River queue is not configured")
	}
	if strings.TrimSpace(intentID) == "" {
		return 0, fmt.Errorf("memory-intent id is required for River enqueue")
	}
	result, err := r.riverIntentJobs.InsertTx(ctx, tx, jobs.MemoryIntentJobArgs{IntentID: intentID}, &river.InsertOpts{
		UniqueOpts: river.UniqueOpts{ByArgs: true},
	})
	if err != nil {
		return 0, fmt.Errorf("insert memory-intent River job: %w", err)
	}
	if result == nil || result.Job == nil || result.Job.ID <= 0 {
		return 0, fmt.Errorf("insert memory-intent River job returned no job")
	}
	return result.Job.ID, nil
}

func (r *Repository) AcceptMemoryIntent(ctx context.Context, record memory.MemoryIntentRecord) (memory.MemoryIntentRecord, error) {
	tx, err := r.tx.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return memory.MemoryIntentRecord{}, fmt.Errorf("begin memory-intent acceptance: %w", err)
	}
	defer tx.Rollback(ctx)
	txCtx := withRepositoryTx(ctx, tx)

	created, err := r.AppendMemoryIntent(txCtx, record)
	if err != nil {
		return memory.MemoryIntentRecord{}, err
	}
	if created.Status == memory.MemoryIntentStatusReplayed {
		if err := tx.Commit(ctx); err != nil {
			return memory.MemoryIntentRecord{}, fmt.Errorf("commit memory-intent replay: %w", err)
		}
		return created, nil
	}
	if err := r.AppendMemoryIntentTransition(txCtx, memory.MemoryIntentTransition{
		IntentID: created.ID, Scope: created.Scope, Sequence: 1, To: memory.MemoryIntentStatusAccepted,
		Actor: created.Actor, Reason: created.Reason, DiagnosticCategory: memory.MemoryIntentDiagnosticAccepted,
		OccurredAt: created.CreatedAt,
	}); err != nil {
		return memory.MemoryIntentRecord{}, fmt.Errorf("record initial memory-intent transition: %w", err)
	}
	jobID, err := r.insertMemoryIntentRiverJob(ctx, tx, created.ID)
	if err != nil {
		return memory.MemoryIntentRecord{}, err
	}
	if err := r.recordMemoryIntentRiverJob(ctx, tx, created, jobID); err != nil {
		return memory.MemoryIntentRecord{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return memory.MemoryIntentRecord{}, fmt.Errorf("commit memory-intent acceptance: %w", err)
	}
	return created, nil
}

func (r *Repository) recordMemoryIntentRiverJob(ctx context.Context, tx pgx.Tx, intent memory.MemoryIntentRecord, jobID int64) error {
	_, err := tx.Exec(ctx, `INSERT INTO memory_intent_river_jobs(intent_id,river_job_id,tenant,project,namespace)
VALUES($1::uuid,$2,$3,$4,$5) ON CONFLICT (intent_id) DO NOTHING`, intent.ID, jobID, intent.Scope.Tenant, intent.Scope.Project, intent.Scope.Namespace)
	if err != nil {
		return fmt.Errorf("record memory-intent River job: %w", err)
	}
	return nil
}

func (r *Repository) ExecuteMemoryIntentJob(ctx context.Context, job *river.Job[jobs.MemoryIntentJobArgs], execute func(context.Context, memory.MemoryIntentRecord) error) error {
	if job == nil || job.JobRow == nil || job.ID <= 0 || strings.TrimSpace(job.Args.IntentID) == "" || execute == nil {
		return fmt.Errorf("memory-intent River job is incomplete")
	}
	tx, err := r.tx.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return fmt.Errorf("begin memory-intent execution: %w", err)
	}
	defer tx.Rollback(ctx)
	txCtx := withRepositoryTx(ctx, tx)

	var scope memory.Scope
	if err := tx.QueryRow(ctx, `SELECT tenant,project,namespace FROM memory_intents WHERE id::text=$1 FOR UPDATE`, job.Args.IntentID).Scan(&scope.Tenant, &scope.Project, &scope.Namespace); err != nil {
		return fmt.Errorf("lock memory intent for River job: %w", err)
	}
	record, err := r.ReadMemoryIntent(txCtx, scope, job.Args.IntentID)
	if err != nil {
		return fmt.Errorf("read memory intent for River job: %w", err)
	}
	if err := execute(txCtx, record); err != nil {
		return err
	}
	if _, err := river.JobCompleteTx[*riverpgxv5.Driver](ctx, tx, job); err != nil {
		return fmt.Errorf("complete memory-intent River job: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit memory-intent execution: %w", err)
	}
	return nil
}

func (r *Repository) HandoffLegacyMemoryIntentJobs(ctx context.Context) error {
	if r == nil || r.tx == nil || r.riverIntentJobs == nil {
		return fmt.Errorf("memory-intent River handoff is not configured")
	}
	tx, err := r.tx.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return fmt.Errorf("begin legacy memory-intent handoff: %w", err)
	}
	defer tx.Rollback(ctx)

	rows, err := tx.Query(ctx, `SELECT d.id::text,d.reference,i.id::text,i.tenant,i.project,i.namespace,i.status,j.river_job_id
FROM derived_work_items d
LEFT JOIN memory_intents i ON i.id::text=d.reference
LEFT JOIN memory_intent_river_jobs j ON j.intent_id=i.id
WHERE d.kind='memory_intent' AND d.state IN ('queued','retry','claimed','running')
ORDER BY d.created_at,d.id FOR UPDATE OF d`)
	if err != nil {
		return fmt.Errorf("read legacy memory-intent queue: %w", err)
	}
	type legacyIntentWork struct {
		workID, intentID string
		scope            memory.Scope
		status           sql.NullString
		jobID            sql.NullInt64
	}
	var legacy []legacyIntentWork
	for rows.Next() {
		var item legacyIntentWork
		var intentID, tenant, project, namespace *string
		var status *string
		var riverJobID *int64
		if err := rows.Scan(&item.workID, &item.intentID, &intentID, &tenant, &project, &namespace, &status, &riverJobID); err != nil {
			rows.Close()
			return fmt.Errorf("scan legacy memory-intent queue: %w", err)
		}
		if intentID != nil {
			item.intentID = *intentID
		}
		if tenant != nil {
			item.scope.Tenant = *tenant
		}
		if project != nil {
			item.scope.Project = *project
		}
		if namespace != nil {
			item.scope.Namespace = *namespace
		}
		if status != nil {
			item.status = sql.NullString{String: *status, Valid: true}
		}
		if riverJobID != nil {
			item.jobID = sql.NullInt64{Int64: *riverJobID, Valid: true}
		}
		legacy = append(legacy, item)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return fmt.Errorf("iterate legacy memory-intent queue: %w", err)
	}
	rows.Close()

	for _, item := range legacy {
		if item.intentID == "" || !item.status.Valid || (item.status.String != string(memory.MemoryIntentStatusAccepted) && item.status.String != string(memory.MemoryIntentStatusPending)) {
			if _, err := tx.Exec(ctx, `UPDATE derived_work_items SET state='cancelled',lease_owner=NULL,lease_until=NULL,terminal_at=clock_timestamp(),updated_at=clock_timestamp(),failure_category='intent_unavailable'
WHERE id=$1::uuid`, item.workID); err != nil {
				return fmt.Errorf("cancel unavailable legacy memory-intent work: %w", err)
			}
			continue
		}
		jobID := item.jobID.Int64
		if !item.jobID.Valid {
			jobID, err = r.insertMemoryIntentRiverJob(ctx, tx, item.intentID)
			if err != nil {
				return err
			}
			if err := r.recordMemoryIntentRiverJob(ctx, tx, memory.MemoryIntentRecord{ID: item.intentID, Scope: item.scope}, jobID); err != nil {
				return err
			}
		}
		if _, err := tx.Exec(ctx, `UPDATE derived_work_items SET state='completed',river_job_id=$2,river_handed_off_at=clock_timestamp(),lease_owner=NULL,lease_until=NULL,terminal_at=clock_timestamp(),updated_at=clock_timestamp(),failure_category='river_handoff'
WHERE id=$1::uuid`, item.workID, jobID); err != nil {
			return fmt.Errorf("mark legacy memory-intent work handed off: %w", err)
		}
		if _, err := tx.Exec(ctx, `INSERT INTO derived_work_terminal_summaries(work_id,tenant,project,namespace,state,disposition,evidence_reference,recorded_at)
VALUES($1::uuid,$2,$3,$4,'completed','handed_off_to_river',$5,clock_timestamp()) ON CONFLICT (work_id) DO NOTHING`, item.workID, item.scope.Tenant, item.scope.Project, item.scope.Namespace, fmt.Sprintf("river-job:%d", jobID)); err != nil {
			return fmt.Errorf("record legacy memory-intent handoff: %w", err)
		}
	}

	missingRows, err := tx.Query(ctx, `SELECT i.id::text,i.tenant,i.project,i.namespace
FROM memory_intents i
WHERE i.status IN ('accepted','pending')
  AND NOT EXISTS (SELECT 1 FROM memory_intent_river_jobs j WHERE j.intent_id=i.id)
  AND NOT EXISTS (SELECT 1 FROM derived_work_items d WHERE d.kind='memory_intent' AND d.reference=i.id::text)
ORDER BY i.created_at,i.id FOR UPDATE OF i`)
	if err != nil {
		return fmt.Errorf("find unqueued accepted memory intents: %w", err)
	}
	for missingRows.Next() {
		var record memory.MemoryIntentRecord
		if err := missingRows.Scan(&record.ID, &record.Scope.Tenant, &record.Scope.Project, &record.Scope.Namespace); err != nil {
			missingRows.Close()
			return fmt.Errorf("scan unqueued accepted memory intent: %w", err)
		}
		jobID, err := r.insertMemoryIntentRiverJob(ctx, tx, record.ID)
		if err != nil {
			missingRows.Close()
			return err
		}
		if err := r.recordMemoryIntentRiverJob(ctx, tx, record, jobID); err != nil {
			missingRows.Close()
			return err
		}
	}
	if err := missingRows.Err(); err != nil {
		missingRows.Close()
		return fmt.Errorf("iterate unqueued accepted memory intents: %w", err)
	}
	missingRows.Close()

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit legacy memory-intent handoff: %w", err)
	}
	return nil
}

var _ memory.MemoryIntentAtomicAcceptor = (*Repository)(nil)
var _ jobs.MemoryIntentRiverJobStore = (*Repository)(nil)
