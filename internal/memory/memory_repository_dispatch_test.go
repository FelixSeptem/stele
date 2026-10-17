package memory

import (
	"context"
	"testing"
	"time"
)

type memoryRepositoryDispatchStoreStub struct {
	importRecord MemoryRepositoryImportRecord
	preview      MemoryRepositoryPreview
	attempts     []MemoryRepositoryImportEntry
	completed    bool
}

func (s *memoryRepositoryDispatchStoreStub) ReadMemoryRepositoryImportPlan(context.Context, Scope, string, string) (MemoryRepositoryImportRecord, MemoryRepositoryPreview, error) {
	return s.importRecord, s.preview, nil
}
func (*memoryRepositoryDispatchStoreStub) MarkMemoryRepositoryImportRunning(context.Context, Scope, string, string) error {
	return nil
}
func (s *memoryRepositoryDispatchStoreStub) RecordMemoryRepositoryImportEntryAttempt(_ context.Context, _ Scope, _, _ string, entry MemoryRepositoryImportEntry) error {
	s.attempts = append(s.attempts, entry)
	return nil
}
func (s *memoryRepositoryDispatchStoreStub) CompleteMemoryRepositoryImport(context.Context, Scope, string, string, time.Time) error {
	s.completed = true
	return nil
}

type memoryRepositoryIntentSubmitterStub struct {
	inputs []MemoryIntentInput
}

func (s *memoryRepositoryIntentSubmitterStub) Submit(_ context.Context, input MemoryIntentInput) (MemoryIntentRecord, error) {
	s.inputs = append(s.inputs, input)
	return MemoryIntentRecord{ID: "intent-" + input.IdempotencyKey, Status: MemoryIntentStatusAccepted}, nil
}

func TestMemoryRepositoryImportDispatcherUsesGovernedRememberIntentWithStableIdentity(t *testing.T) {
	scope := Scope{Tenant: "tenant", Project: "project", Namespace: "namespace"}
	store := &memoryRepositoryDispatchStoreStub{
		importRecord: MemoryRepositoryImportRecord{ID: "import", RepositoryID: "repo", Scope: scope, PreviewID: "preview", Status: MemoryRepositoryImportQueued},
		preview:      MemoryRepositoryPreview{RepositoryID: "repo", Scope: scope, Revision: "r1", SnapshotDigest: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", ParserVersion: MemoryRepositoryParserVersion, Actor: "agent", Reason: "review", EffectiveSource: SourceMetadata{}.Effective(), Entries: []MemoryRepositoryPreviewEntry{{EntryKey: "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb", Path: "MEMORY.md", Occurrences: []int{4}, Text: "fact", Class: MemoryClassProfile, MemoryPath: "preferences"}}},
	}
	intents := &memoryRepositoryIntentSubmitterStub{}
	dispatcher := MemoryRepositoryImportDispatcher{Store: store, Intents: intents, Now: func() time.Time { return time.Date(2026, 10, 16, 12, 0, 0, 0, time.UTC) }}

	if _, err := dispatcher.Dispatch(context.Background(), scope, "repo", "import"); err != nil {
		t.Fatalf("Dispatch() error = %v", err)
	}
	if len(intents.inputs) != 1 || intents.inputs[0].Type != MemoryIntentRemember || intents.inputs[0].MemoryPath != "preferences" || intents.inputs[0].Content != "fact" {
		t.Fatalf("governed inputs = %+v", intents.inputs)
	}
	if intents.inputs[0].IdempotencyKey != MemoryRepositoryChildIdempotencyKey("import", store.preview.Entries[0].EntryKey) {
		t.Fatalf("idempotency key = %q", intents.inputs[0].IdempotencyKey)
	}
	if len(store.attempts) != 1 || store.attempts[0].DispatchStatus != MemoryRepositoryEntrySubmitted || !store.completed {
		t.Fatalf("attempts=%+v completed=%v", store.attempts, store.completed)
	}
	if source, ok := intents.inputs[0].Provenance["source_metadata"].(SourceMetadata); !ok || source != (SourceMetadata{}).Effective() {
		t.Fatalf("provenance source metadata = %#v", intents.inputs[0].Provenance["source_metadata"])
	}
	if class, ok := intents.inputs[0].Provenance["memory_class"].(string); !ok || class != string(MemoryClassProfile) {
		t.Fatalf("provenance memory class = %#v", intents.inputs[0].Provenance["memory_class"])
	}
}

func TestMemoryRepositoryImportDispatcherDoesNotResubmitSettledEntryAfterRecovery(t *testing.T) {
	scope := Scope{Tenant: "tenant", Project: "project", Namespace: "namespace"}
	entryKey := "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"
	store := &memoryRepositoryDispatchStoreStub{
		importRecord: MemoryRepositoryImportRecord{ID: "import", RepositoryID: "repo", Scope: scope, PreviewID: "preview", Status: MemoryRepositoryImportRunning, Entries: []MemoryRepositoryImportEntry{{EntryKey: entryKey, DispatchStatus: MemoryRepositoryEntrySubmitted, IntentID: "intent"}}},
		preview:      MemoryRepositoryPreview{RepositoryID: "repo", Scope: scope, Revision: "r1", SnapshotDigest: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", ParserVersion: MemoryRepositoryParserVersion, Actor: "agent", Reason: "review", Entries: []MemoryRepositoryPreviewEntry{{EntryKey: entryKey, Path: "MEMORY.md", Text: "fact", Class: MemoryClassEpisodic, MemoryPath: "notes/fact"}}},
	}
	intents := &memoryRepositoryIntentSubmitterStub{}
	if _, err := (MemoryRepositoryImportDispatcher{Store: store, Intents: intents}).Dispatch(context.Background(), scope, "repo", "import"); err != nil {
		t.Fatalf("Dispatch() error = %v", err)
	}
	if len(intents.inputs) != 0 || len(store.attempts) != 0 || !store.completed {
		t.Fatalf("settled entry was resubmitted: inputs=%+v attempts=%+v completed=%v", intents.inputs, store.attempts, store.completed)
	}
}

func TestMemoryRepositoryImportDispatcherFinalizesOnlyQueuedEntriesAfterWorkExhaustion(t *testing.T) {
	scope := Scope{Tenant: "tenant", Project: "project", Namespace: "namespace"}
	queued := "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"
	settled := "cccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccc"
	store := &memoryRepositoryDispatchStoreStub{
		importRecord: MemoryRepositoryImportRecord{
			ID: "import", RepositoryID: "repo", Scope: scope, PreviewID: "preview", Status: MemoryRepositoryImportRunning,
			Entries: []MemoryRepositoryImportEntry{
				{EntryKey: queued, DispatchStatus: MemoryRepositoryEntryQueued},
				{EntryKey: settled, DispatchStatus: MemoryRepositoryEntrySubmitted, IntentID: "intent"},
			},
		},
		preview: MemoryRepositoryPreview{RepositoryID: "repo", Scope: scope},
	}
	dispatcher := MemoryRepositoryImportDispatcher{Store: store, Intents: &memoryRepositoryIntentSubmitterStub{}, Now: func() time.Time { return time.Date(2026, 10, 16, 12, 0, 0, 0, time.UTC) }}

	if err := dispatcher.FinalizeExhaustedImport(context.Background(), scope, "repo", "import"); err != nil {
		t.Fatalf("FinalizeExhaustedImport() error = %v", err)
	}
	if len(store.attempts) != 1 || store.attempts[0].EntryKey != queued || store.attempts[0].DispatchStatus != MemoryRepositoryEntryFailed {
		t.Fatalf("attempts = %+v", store.attempts)
	}
	if !store.completed {
		t.Fatal("import was not completed after all queued entries received a terminal outcome")
	}
}
