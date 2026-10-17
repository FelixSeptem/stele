package memory

import (
	"context"
	"fmt"
	"strings"
	"time"
)

// MemoryRepositoryImportDispatchStore is the durable worker boundary. It
// returns the accepted immutable plan, records only bounded dispatch outcomes,
// and never exposes the plan through ordinary status reads.
type MemoryRepositoryImportDispatchStore interface {
	ReadMemoryRepositoryImportPlan(context.Context, Scope, string, string) (MemoryRepositoryImportRecord, MemoryRepositoryPreview, error)
	MarkMemoryRepositoryImportRunning(context.Context, Scope, string, string) error
	RecordMemoryRepositoryImportEntryAttempt(context.Context, Scope, string, string, MemoryRepositoryImportEntry) error
	CompleteMemoryRepositoryImport(context.Context, Scope, string, string, time.Time) error
}

type MemoryRepositoryIntentSubmitter interface {
	Submit(context.Context, MemoryIntentInput) (MemoryIntentRecord, error)
}

// MemoryRepositoryImportDispatcher performs only governed intent admission.
// Canonical memory creation remains exclusively behind the existing intent
// worker and governance pipeline.
type MemoryRepositoryImportDispatcher struct {
	Store   MemoryRepositoryImportDispatchStore
	Intents MemoryRepositoryIntentSubmitter
	Now     func() time.Time
}

func (d MemoryRepositoryImportDispatcher) Dispatch(ctx context.Context, scope Scope, repositoryID, importID string) (int, error) {
	if err := scope.Validate(); err != nil {
		return 0, err
	}
	if strings.TrimSpace(repositoryID) == "" || strings.TrimSpace(importID) == "" {
		return 0, fmt.Errorf("memory repository and import id are required")
	}
	if d.Store == nil || d.Intents == nil {
		return 0, fmt.Errorf("memory repository dispatch store and intent submitter are required")
	}
	record, preview, err := d.Store.ReadMemoryRepositoryImportPlan(ctx, scope, repositoryID, importID)
	if err != nil {
		return 0, err
	}
	if record.Scope.Normalized() != scope.Normalized() || record.RepositoryID != repositoryID || record.ID != importID || preview.Scope.Normalized() != scope.Normalized() || preview.RepositoryID != repositoryID {
		return 0, fmt.Errorf("memory repository import plan is unavailable in scope")
	}
	if record.Status == MemoryRepositoryImportCompleted {
		return 0, nil
	}
	if err := d.Store.MarkMemoryRepositoryImportRunning(ctx, scope, repositoryID, importID); err != nil {
		return 0, err
	}
	settledEntries := make(map[string]MemoryRepositoryEntryDispatchStatus, len(record.Entries))
	for _, existing := range record.Entries {
		if existing.EntryKey != "" && existing.DispatchStatus.Valid() && existing.DispatchStatus != MemoryRepositoryEntryQueued {
			settledEntries[existing.EntryKey] = existing.DispatchStatus
		}
	}
	now := time.Now().UTC()
	if d.Now != nil {
		now = d.Now().UTC()
	}
	settled := 0
	for _, entry := range preview.Entries {
		if _, alreadySettled := settledEntries[entry.EntryKey]; alreadySettled {
			continue
		}
		childKey := MemoryRepositoryChildIdempotencyKey(record.ID, entry.EntryKey)
		intent, submitErr := d.Intents.Submit(ctx, MemoryIntentInput{
			Scope:          scope,
			MemoryPath:     entry.MemoryPath,
			Type:           MemoryIntentRemember,
			Content:        entry.Text,
			Actor:          preview.Actor,
			Reason:         preview.Reason,
			RequestID:      record.ID,
			OperationID:    record.ID + ":" + entry.EntryKey,
			IdempotencyKey: childKey,
			Provenance: map[string]any{
				"memory_repository_id": record.RepositoryID,
				"source_revision":      preview.Revision,
				"snapshot_digest":      preview.SnapshotDigest,
				"source_path":          entry.Path,
				"occurrences":          append([]int(nil), entry.Occurrences...),
				"entry_fingerprint":    entry.EntryKey,
				"memory_class":         string(entry.Class),
				"preview_id":           preview.ID,
				"import_id":            record.ID,
				"parser_version":       preview.ParserVersion,
				"source_metadata":      preview.EffectiveSource.Effective(),
				"observed_at":          now,
			},
		})
		outcome := MemoryRepositoryImportEntry{EntryKey: entry.EntryKey, Payload: entry}
		if submitErr != nil {
			outcome.DispatchStatus = MemoryRepositoryEntryFailed
			if err := d.Store.RecordMemoryRepositoryImportEntryAttempt(ctx, scope, repositoryID, record.ID, outcome); err != nil {
				return settled, err
			}
			return settled, fmt.Errorf("submit memory repository entry: %w", submitErr)
		}
		outcome.IntentID = intent.ID
		outcome.DispatchStatus = MemoryRepositoryEntrySubmitted
		if intent.Status == MemoryIntentStatusReplayed {
			outcome.DispatchStatus = MemoryRepositoryEntryDuplicate
		}
		if err := d.Store.RecordMemoryRepositoryImportEntryAttempt(ctx, scope, repositoryID, record.ID, outcome); err != nil {
			return settled, err
		}
		settled++
	}
	if err := d.Store.CompleteMemoryRepositoryImport(ctx, scope, repositoryID, record.ID, now); err != nil {
		return settled, err
	}
	return settled, nil
}

// FinalizeExhaustedImport gives every still-queued entry a durable terminal
// failure after the common derived-work retry budget is exhausted. It never
// creates an intent or retries accepted evidence; submitted/duplicate/rejected
// and already-failed entries retain their recorded outcomes.
func (d MemoryRepositoryImportDispatcher) FinalizeExhaustedImport(ctx context.Context, scope Scope, repositoryID, importID string) error {
	if err := scope.Validate(); err != nil {
		return err
	}
	if strings.TrimSpace(repositoryID) == "" || strings.TrimSpace(importID) == "" {
		return fmt.Errorf("memory repository and import id are required")
	}
	if d.Store == nil {
		return fmt.Errorf("memory repository dispatch store is required")
	}
	record, preview, err := d.Store.ReadMemoryRepositoryImportPlan(ctx, scope, repositoryID, importID)
	if err != nil {
		return err
	}
	if record.Scope.Normalized() != scope.Normalized() || record.RepositoryID != repositoryID || record.ID != importID || preview.Scope.Normalized() != scope.Normalized() || preview.RepositoryID != repositoryID {
		return fmt.Errorf("memory repository import plan is unavailable in scope")
	}
	if record.Status == MemoryRepositoryImportCompleted {
		return nil
	}
	if err := d.Store.MarkMemoryRepositoryImportRunning(ctx, scope, repositoryID, importID); err != nil {
		return err
	}
	entries := make(map[string]MemoryRepositoryPreviewEntry, len(preview.Entries))
	for _, entry := range preview.Entries {
		entries[entry.EntryKey] = entry
	}
	for _, entry := range record.Entries {
		if entry.DispatchStatus != MemoryRepositoryEntryQueued {
			continue
		}
		entry.DispatchStatus = MemoryRepositoryEntryFailed
		entry.Payload = entries[entry.EntryKey]
		if err := d.Store.RecordMemoryRepositoryImportEntryAttempt(ctx, scope, repositoryID, importID, entry); err != nil {
			return err
		}
	}
	now := time.Now().UTC()
	if d.Now != nil {
		now = d.Now().UTC()
	}
	return d.Store.CompleteMemoryRepositoryImport(ctx, scope, repositoryID, importID, now)
}
