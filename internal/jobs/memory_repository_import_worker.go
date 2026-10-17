package jobs

import (
	"context"
	"fmt"
	"strings"

	"github.com/FelixSeptem/stele/internal/memory"
	"github.com/FelixSeptem/stele/internal/workqueue"
)

// MemoryRepositoryImportWorkDispatcher is deliberately narrow: it can only
// turn an already accepted durable import plan into governed remember intents.
type MemoryRepositoryImportWorkDispatcher interface {
	Dispatch(context.Context, memory.Scope, string, string) (int, error)
}

// MemoryRepositoryImportWorkFinalizer settles durable import records after
// the bounded derived-work retry budget has been exhausted.
type MemoryRepositoryImportWorkFinalizer interface {
	FinalizeExhaustedImport(context.Context, memory.Scope, string, string) error
}

// MemoryRepositoryImportWorkExecutor bridges the reference-only durable queue
// to the repository dispatcher. The reference has two bounded UUIDs and never
// carries Markdown content; the dispatcher re-reads the exact scoped plan.
type MemoryRepositoryImportWorkExecutor struct {
	Dispatcher MemoryRepositoryImportWorkDispatcher
}

func (e MemoryRepositoryImportWorkExecutor) ExecuteDerivedWork(ctx context.Context, item workqueue.DerivedWorkItem) (int64, string, error) {
	if item.Kind != workqueue.WorkKindMemoryRepositoryImport {
		return 0, "", fmt.Errorf("memory repository import executor received work kind %q", item.Kind)
	}
	if e.Dispatcher == nil {
		return 0, "", fmt.Errorf("memory repository import dispatcher is required")
	}
	parts := strings.Split(item.Reference, "|")
	if len(parts) != 2 || strings.TrimSpace(parts[0]) == "" || strings.TrimSpace(parts[1]) == "" {
		return 0, "", fmt.Errorf("memory repository import work reference is invalid")
	}
	settled, err := e.Dispatcher.Dispatch(ctx, item.Scope, parts[0], parts[1])
	if err != nil {
		return 0, "", err
	}
	return int64(settled), parts[1], nil
}

// HandleDerivedWorkExhaustion is invoked by the common worker only after its
// durable queue record becomes exhausted. It never creates a new submission;
// it merely asks the import dispatcher to give still-queued entries bounded
// terminal outcomes.
func (e MemoryRepositoryImportWorkExecutor) HandleDerivedWorkExhaustion(ctx context.Context, item workqueue.DerivedWorkItem) error {
	if item.Kind != workqueue.WorkKindMemoryRepositoryImport {
		return fmt.Errorf("memory repository import executor received work kind %q", item.Kind)
	}
	finalizer, ok := e.Dispatcher.(MemoryRepositoryImportWorkFinalizer)
	if !ok || finalizer == nil {
		return fmt.Errorf("memory repository import exhaustion finalizer is required")
	}
	parts := strings.Split(item.Reference, "|")
	if len(parts) != 2 || strings.TrimSpace(parts[0]) == "" || strings.TrimSpace(parts[1]) == "" {
		return fmt.Errorf("memory repository import work reference is invalid")
	}
	return finalizer.FinalizeExhaustedImport(ctx, item.Scope, parts[0], parts[1])
}
