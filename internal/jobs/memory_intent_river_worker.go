package jobs

import (
	"context"
	"fmt"

	"github.com/FelixSeptem/stele/internal/memory"
	"github.com/riverqueue/river"
)

const MemoryIntentRiverJobKind = "stele_memory_intent"

// MemoryIntentJobArgs carries only the authoritative intent reference.
type MemoryIntentJobArgs struct {
	IntentID string `json:"intent_id"`
}

func (MemoryIntentJobArgs) Kind() string { return MemoryIntentRiverJobKind }

type MemoryIntentRiverJobStore interface {
	ExecuteMemoryIntentJob(context.Context, *river.Job[MemoryIntentJobArgs], func(context.Context, memory.MemoryIntentRecord) error) error
}

type RiverMemoryIntentWorker struct {
	river.WorkerDefaults[MemoryIntentJobArgs]
	Store   MemoryIntentRiverJobStore
	Execute func(context.Context, memory.MemoryIntentRecord) error
}

type MemoryIntentRiverExecution struct {
	History  memoryIntentHistoryReader
	Worker   memory.MemoryIntentWorker
	Recorder memory.MemoryIntentOutcomeRecorder
	Policy   memory.MemoryIntentResumePolicyGate
}

func (e MemoryIntentRiverExecution) Execute(ctx context.Context, record memory.MemoryIntentRecord) error {
	if e.Recorder == nil {
		return fmt.Errorf("memory-intent outcome recorder is required")
	}
	var history memory.MemoryIntentHistory
	if e.History != nil {
		loaded, err := e.History.ReadMemoryIntentHistory(ctx, record.Scope, record.ID)
		if err != nil {
			return fmt.Errorf("read memory-intent history: %w", err)
		}
		history = loaded
		if len(history.Transitions) >= 2 && history.Transitions[len(history.Transitions)-1].To != memory.MemoryIntentStatusFailed {
			return nil
		}
	}
	if e.Policy != nil {
		decision, err := e.Policy.EvaluateMemoryIntentResume(ctx, record)
		if err != nil {
			return fmt.Errorf("evaluate memory-intent resume policy: %w", err)
		}
		if !decision.Enabled {
			category := decision.Category
			if !category.Valid() {
				category = memory.MemoryIntentDiagnosticPolicyDisabled
			}
			return fmt.Errorf("memory-intent policy %s: processing held", category)
		}
	}
	if len(history.Transitions) > 0 && history.Transitions[len(history.Transitions)-1].To == memory.MemoryIntentStatusFailed {
		_, err := e.Worker.ProcessAndRecordRetry(ctx, record, e.Recorder, int64(len(history.Transitions)+1), memory.MemoryIntentStatusFailed)
		return err
	}
	_, err := e.Worker.ProcessAndRecord(ctx, record, e.Recorder)
	return err
}

func (w *RiverMemoryIntentWorker) Work(ctx context.Context, job *river.Job[MemoryIntentJobArgs]) error {
	if w == nil || w.Store == nil || w.Execute == nil {
		return fmt.Errorf("river memory-intent worker is not configured")
	}
	if job == nil || job.JobRow == nil || job.ID <= 0 || job.Args.IntentID == "" {
		return fmt.Errorf("river memory-intent job identity is required")
	}
	return w.Store.ExecuteMemoryIntentJob(ctx, job, w.Execute)
}
