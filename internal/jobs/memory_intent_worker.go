package jobs

import (
	"context"
	"fmt"

	"github.com/FelixSeptem/stele/internal/memory"
	"github.com/FelixSeptem/stele/internal/workqueue"
)

// MemoryIntentWorkExecutor bridges the reference-only durable queue to the
// governed intent contract. It reads the immutable intent inside the claimed
// scope, processes it through the injected governance router, and records the
// append-only outcome transition before the common queue terminal path runs.
type MemoryIntentWorkExecutor struct {
	Reader   memoryIntentRecordReader
	History  memoryIntentHistoryReader
	Worker   memory.MemoryIntentWorker
	Recorder memory.MemoryIntentOutcomeRecorder
}

type memoryIntentRecordReader interface {
	ReadMemoryIntent(context.Context, memory.Scope, string) (memory.MemoryIntentRecord, error)
}

type memoryIntentHistoryReader interface {
	ReadMemoryIntentHistory(context.Context, memory.Scope, string) (memory.MemoryIntentHistory, error)
}

func (e MemoryIntentWorkExecutor) ExecuteDerivedWork(ctx context.Context, item workqueue.DerivedWorkItem) (int64, string, error) {
	if item.Kind != workqueue.WorkKindMemoryIntent {
		return 0, "", fmt.Errorf("memory intent executor received work kind %q", item.Kind)
	}
	if e.Reader == nil || e.Recorder == nil {
		return 0, "", fmt.Errorf("memory intent reader and recorder are required")
	}
	if e.History != nil {
		history, err := e.History.ReadMemoryIntentHistory(ctx, item.Scope, item.Reference)
		if err != nil {
			return 0, "", fmt.Errorf("read memory intent history: %w", err)
		}
		if len(history.Transitions) >= 2 && history.Transitions[len(history.Transitions)-1].To != memory.MemoryIntentStatusFailed {
			return 1, item.Reference, nil
		}
	}
	record, err := e.Reader.ReadMemoryIntent(ctx, item.Scope, item.Reference)
	if err != nil {
		return 0, "", fmt.Errorf("read memory intent: %w", err)
	}
	var outcome memory.MemoryIntentOutcome
	if e.History != nil {
		history, historyErr := e.History.ReadMemoryIntentHistory(ctx, item.Scope, item.Reference)
		if historyErr != nil {
			return 0, "", fmt.Errorf("read memory intent retry history: %w", historyErr)
		}
		if len(history.Transitions) > 0 && history.Transitions[len(history.Transitions)-1].To == memory.MemoryIntentStatusFailed {
			outcome, err = e.Worker.ProcessAndRecordRetry(ctx, record, e.Recorder, int64(len(history.Transitions)+1), memory.MemoryIntentStatusFailed)
		} else {
			outcome, err = e.Worker.ProcessAndRecord(ctx, record, e.Recorder)
		}
	} else {
		outcome, err = e.Worker.ProcessAndRecord(ctx, record, e.Recorder)
	}
	if err != nil {
		return 0, "", err
	}
	return 1, outcome.OutcomeReference, nil
}
