package jobs

import (
	"context"
	"testing"

	"github.com/FelixSeptem/stele/internal/memory"
	"github.com/FelixSeptem/stele/internal/workqueue"
)

type memoryRepositoryImportDispatcherStub struct {
	scope        memory.Scope
	repositoryID string
	importID     string
	settled      int
	exhausted    int
}

func (s *memoryRepositoryImportDispatcherStub) Dispatch(_ context.Context, scope memory.Scope, repositoryID, importID string) (int, error) {
	s.scope, s.repositoryID, s.importID = scope, repositoryID, importID
	return s.settled, nil
}

func (s *memoryRepositoryImportDispatcherStub) FinalizeExhaustedImport(_ context.Context, scope memory.Scope, repositoryID, importID string) error {
	s.scope, s.repositoryID, s.importID = scope, repositoryID, importID
	s.exhausted++
	return nil
}

func TestMemoryRepositoryImportWorkExecutorDispatchesOnlyRepositoryImportWork(t *testing.T) {
	scope := memory.Scope{Tenant: "tenant", Project: "project", Namespace: "namespace"}
	dispatcher := &memoryRepositoryImportDispatcherStub{settled: 2}
	executor := MemoryRepositoryImportWorkExecutor{Dispatcher: dispatcher}
	checkpoint, evidence, err := executor.ExecuteDerivedWork(context.Background(), workqueue.DerivedWorkItem{Kind: workqueue.WorkKindMemoryRepositoryImport, DerivedWorkIdentity: workqueue.DerivedWorkIdentity{Scope: scope}, Reference: "repo|import"})
	if err != nil || checkpoint != 2 || evidence != "import" || dispatcher.scope != scope || dispatcher.repositoryID != "repo" || dispatcher.importID != "import" {
		t.Fatalf("ExecuteDerivedWork() = checkpoint=%d evidence=%q err=%v dispatcher=%+v", checkpoint, evidence, err, dispatcher)
	}
	if _, _, err := executor.ExecuteDerivedWork(context.Background(), workqueue.DerivedWorkItem{Kind: workqueue.WorkKindMemoryIntent, DerivedWorkIdentity: workqueue.DerivedWorkIdentity{Scope: scope}, Reference: "repo|import"}); err == nil {
		t.Fatal("memory intent work unexpectedly accepted by repository importer")
	}
}

func TestMemoryRepositoryImportWorkExecutorFinalizesQueuedEntriesWhenWorkExhausts(t *testing.T) {
	scope := memory.Scope{Tenant: "tenant", Project: "project", Namespace: "namespace"}
	dispatcher := &memoryRepositoryImportDispatcherStub{}
	executor := MemoryRepositoryImportWorkExecutor{Dispatcher: dispatcher}
	item := workqueue.DerivedWorkItem{Kind: workqueue.WorkKindMemoryRepositoryImport, DerivedWorkIdentity: workqueue.DerivedWorkIdentity{Scope: scope}, Reference: "repo|import"}

	if err := executor.HandleDerivedWorkExhaustion(context.Background(), item); err != nil {
		t.Fatalf("HandleDerivedWorkExhaustion() error = %v", err)
	}
	if dispatcher.exhausted != 1 || dispatcher.scope != scope || dispatcher.repositoryID != "repo" || dispatcher.importID != "import" {
		t.Fatalf("exhaustion dispatch = %+v", dispatcher)
	}
}
