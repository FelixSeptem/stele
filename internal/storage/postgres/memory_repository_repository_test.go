package postgres

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/FelixSeptem/stele/internal/memory"
	"github.com/FelixSeptem/stele/internal/workqueue"
	"github.com/jackc/pgx/v5"
	pgxmock "github.com/pashagolub/pgxmock/v4"
)

func TestRepositoryRegistersMemoryRepositoryWithinExactScope(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()
	scope := memory.Scope{Tenant: "tenant", Project: "project", Namespace: "namespace"}
	now := time.Date(2026, 10, 16, 12, 0, 0, 0, time.UTC)
	record := memory.MemoryRepositoryRecord{ID: "00000000-0000-0000-0000-000000000123", Scope: scope, Name: "notes", SourceURL: "https://example.test", CreatedAt: now}
	mock.ExpectQuery("INSERT INTO memory_repositories").WithArgs(record.ID, scope.Tenant, scope.Project, scope.Namespace, record.Name, record.SourceURL, record.CreatedAt).WillReturnRows(pgxmock.NewRows([]string{"id", "tenant", "project", "namespace", "name", "source_url", "generation", "created_at"}).AddRow(record.ID, scope.Tenant, scope.Project, scope.Namespace, record.Name, record.SourceURL, 0, now))

	created, err := NewRepository(mock).RegisterMemoryRepository(context.Background(), record)
	if err != nil || created.ID != record.ID || created.Scope != scope || created.Generation != 0 {
		t.Fatalf("RegisterMemoryRepository() = %+v, %v", created, err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestRepositoryReadsMemoryRepositoryImportPlanOnlyWithinExactScope(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()
	scope := memory.Scope{Tenant: "tenant", Project: "project", Namespace: "namespace"}
	preview := memory.MemoryRepositoryPreview{ID: "preview", RepositoryID: "repo", Scope: scope, Generation: 1, Revision: "r1", SnapshotDigest: strings.Repeat("a", 64), PlanDigest: strings.Repeat("b", 64), WarningDigest: strings.Repeat("c", 64), ParserVersion: memory.MemoryRepositoryParserVersion, ExpiresAt: time.Now().UTC().Add(time.Hour), EffectiveSource: (memory.SourceMetadata{}).Effective(), Entries: []memory.MemoryRepositoryPreviewEntry{{EntryKey: strings.Repeat("d", 64), Path: "MEMORY.md", Text: "fact", Class: memory.MemoryClassEpisodic, MemoryPath: "notes/fact"}}}
	payload, err := json.Marshal(preview)
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC()
	mock.ExpectQuery("FROM memory_repository_imports").WithArgs("import", "repo", scope.Tenant, scope.Project, scope.Namespace).WillReturnRows(pgxmock.NewRows([]string{"id", "repository_id", "tenant", "project", "namespace", "preview_id", "generation", "idempotency_key", "request_fingerprint", "status", "created_at", "completed_at", "plan_payload"}).AddRow("import", "repo", scope.Tenant, scope.Project, scope.Namespace, "preview", 1, "key", strings.Repeat("e", 64), "queued", now, nil, payload))
	mock.ExpectQuery("FROM memory_repository_import_entries").WithArgs("import").WillReturnRows(pgxmock.NewRows([]string{"entry_key", "dispatch_status", "intent_id"}).AddRow(preview.Entries[0].EntryKey, string(memory.MemoryRepositoryEntrySubmitted), "00000000-0000-0000-0000-000000000125"))
	record, got, err := NewRepository(mock).ReadMemoryRepositoryImportPlan(context.Background(), scope, "repo", "import")
	if err != nil || record.ID != "import" || len(record.Entries) != 1 || record.Entries[0].DispatchStatus != memory.MemoryRepositoryEntrySubmitted || got.RepositoryID != "repo" || len(got.Entries) != 1 {
		t.Fatalf("ReadMemoryRepositoryImportPlan() = %+v, %+v, %v", record, got, err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestRepositoryRecordsDispatchAttemptAndCompletesOnlyScopedSettledImport(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()
	scope := memory.Scope{Tenant: "tenant", Project: "project", Namespace: "namespace"}
	entry := memory.MemoryRepositoryImportEntry{EntryKey: strings.Repeat("d", 64), DispatchStatus: memory.MemoryRepositoryEntrySubmitted, IntentID: "00000000-0000-0000-0000-000000000125"}
	mock.ExpectExec(`NULLIF\(\$5,''\)::uuid`).WithArgs(pgxmock.AnyArg(), "import", entry.EntryKey, string(entry.DispatchStatus), entry.IntentID, "repo", scope.Tenant, scope.Project, scope.Namespace).WillReturnResult(pgxmock.NewResult("INSERT", 1))
	mock.ExpectExec(`NULLIF\(\$2,''\)::uuid`).WithArgs(string(entry.DispatchStatus), entry.IntentID, "import", entry.EntryKey, "repo", scope.Tenant, scope.Project, scope.Namespace).WillReturnResult(pgxmock.NewResult("UPDATE", 1))
	mock.ExpectExec("UPDATE memory_repository_imports").WithArgs(pgxmock.AnyArg(), "import", "repo", scope.Tenant, scope.Project, scope.Namespace).WillReturnResult(pgxmock.NewResult("UPDATE", 1))
	repo := NewRepository(mock)
	if err := repo.RecordMemoryRepositoryImportEntryAttempt(context.Background(), scope, "repo", "import", entry); err != nil {
		t.Fatalf("RecordMemoryRepositoryImportEntryAttempt() error = %v", err)
	}
	if err := repo.CompleteMemoryRepositoryImport(context.Background(), scope, "repo", "import", time.Now().UTC()); err != nil {
		t.Fatalf("CompleteMemoryRepositoryImport() error = %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestRepositoryPrunesOnlyExpiredUnacceptedPreviewPayloads(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()
	before := time.Date(2026, 10, 17, 0, 0, 0, 0, time.UTC)
	mock.ExpectExec("DELETE FROM memory_repository_previews").WithArgs(before, 100).WillReturnResult(pgxmock.NewResult("DELETE", 2))
	deleted, err := NewRepository(mock).PruneExpiredMemoryRepositoryPreviews(context.Background(), before, 100)
	if err != nil || deleted != 2 {
		t.Fatalf("PruneExpiredMemoryRepositoryPreviews() = %d, %v", deleted, err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestRepositoryAcceptsImportWithQueuedEntriesAndDurableWorkInOneTransaction(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()
	scope := memory.Scope{Tenant: "tenant", Project: "project", Namespace: "namespace"}
	preview := memory.MemoryRepositoryPreview{
		ID: "00000000-0000-0000-0000-000000000124", RepositoryID: "00000000-0000-0000-0000-000000000123", Scope: scope,
		Generation: 0, Revision: "r1", Actor: "agent", Reason: "review", SnapshotDigest: strings.Repeat("a", 64), PlanDigest: strings.Repeat("b", 64), WarningDigest: strings.Repeat("c", 64), ParserVersion: memory.MemoryRepositoryParserVersion,
		ExpiresAt: time.Now().UTC().Add(time.Hour), Entries: []memory.MemoryRepositoryPreviewEntry{{EntryKey: strings.Repeat("d", 64), Path: "MEMORY.md", Text: "fact", Class: memory.MemoryClassEpisodic, MemoryPath: "notes/fact"}},
	}
	request := memory.MemoryRepositoryImportRequest{RepositoryID: preview.RepositoryID, PreviewID: preview.ID, Scope: scope, PlanDigest: preview.PlanDigest, WarningDigest: preview.WarningDigest, ExpectedGeneration: 0, IdempotencyKey: "key"}
	mock.ExpectBegin()
	mock.ExpectQuery("FROM memory_repository_imports").WithArgs(request.RepositoryID, scope.Tenant, scope.Project, scope.Namespace, request.IdempotencyKey).WillReturnError(pgx.ErrNoRows)
	mock.ExpectQuery("SELECT generation FROM memory_repositories").WithArgs(request.RepositoryID, scope.Tenant, scope.Project, scope.Namespace).WillReturnRows(pgxmock.NewRows([]string{"generation"}).AddRow(0))
	mock.ExpectQuery("INSERT INTO memory_repository_revisions").WithArgs(request.RepositoryID, scope.Tenant, scope.Project, scope.Namespace, preview.Revision, preview.SnapshotDigest).WillReturnRows(pgxmock.NewRows([]string{"snapshot_digest"}).AddRow(preview.SnapshotDigest))
	mock.ExpectExec("INSERT INTO memory_repository_imports").WithArgs(pgxmock.AnyArg(), request.RepositoryID, scope.Tenant, scope.Project, scope.Namespace, preview.ID, int64(1), request.IdempotencyKey, pgxmock.AnyArg(), preview.SnapshotDigest, preview.PlanDigest, preview.WarningDigest, pgxmock.AnyArg(), string(memory.MemoryRepositoryImportQueued), pgxmock.AnyArg()).WillReturnResult(pgxmock.NewResult("INSERT", 1))
	mock.ExpectExec("INSERT INTO memory_repository_entry_tombstones").WithArgs(request.RepositoryID, scope.Tenant, scope.Project, scope.Namespace, preview.Entries[0].EntryKey).WillReturnResult(pgxmock.NewResult("INSERT", 1))
	mock.ExpectExec("INSERT INTO memory_repository_import_entries").WithArgs(pgxmock.AnyArg(), preview.Entries[0].EntryKey, string(memory.MemoryRepositoryEntryQueued), pgxmock.AnyArg()).WillReturnResult(pgxmock.NewResult("INSERT", 1))
	mock.ExpectExec("INSERT INTO derived_work_items").WithArgs(pgxmock.AnyArg(), string(workqueue.WorkKindMemoryRepositoryImport), scope.Tenant, scope.Project, scope.Namespace, preview.PlanDigest, pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg()).WillReturnResult(pgxmock.NewResult("INSERT", 1))
	mock.ExpectExec("UPDATE memory_repositories SET generation").WithArgs(request.RepositoryID, scope.Tenant, scope.Project, scope.Namespace, int64(1)).WillReturnResult(pgxmock.NewResult("UPDATE", 1))
	mock.ExpectCommit()

	result, replayed, err := NewRepository(mock).AcceptMemoryRepositoryImport(context.Background(), request, preview)
	if err != nil || replayed || result.Status != memory.MemoryRepositoryImportQueued || len(result.Entries) != 1 || result.Entries[0].DispatchStatus != memory.MemoryRepositoryEntryQueued {
		t.Fatalf("AcceptMemoryRepositoryImport() = %+v, replayed=%v, err=%v", result, replayed, err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestRepositoryAcceptsExistingEntryIdentityAsDuplicateWithoutNewChildSubmission(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()
	scope := memory.Scope{Tenant: "tenant", Project: "project", Namespace: "namespace"}
	preview := memory.MemoryRepositoryPreview{
		ID: "00000000-0000-0000-0000-000000000124", RepositoryID: "00000000-0000-0000-0000-000000000123", Scope: scope,
		Generation: 0, Revision: "r1", Actor: "agent", Reason: "review", SnapshotDigest: strings.Repeat("a", 64), PlanDigest: strings.Repeat("b", 64), WarningDigest: strings.Repeat("c", 64), ParserVersion: memory.MemoryRepositoryParserVersion,
		ExpiresAt: time.Now().UTC().Add(time.Hour), Entries: []memory.MemoryRepositoryPreviewEntry{{EntryKey: strings.Repeat("d", 64), Path: "MEMORY.md", Text: "fact", Class: memory.MemoryClassEpisodic, MemoryPath: "notes/fact"}},
	}
	request := memory.MemoryRepositoryImportRequest{RepositoryID: preview.RepositoryID, PreviewID: preview.ID, Scope: scope, PlanDigest: preview.PlanDigest, WarningDigest: preview.WarningDigest, ExpectedGeneration: 0, IdempotencyKey: "key"}
	mock.ExpectBegin()
	mock.ExpectQuery("FROM memory_repository_imports").WithArgs(request.RepositoryID, scope.Tenant, scope.Project, scope.Namespace, request.IdempotencyKey).WillReturnError(pgx.ErrNoRows)
	mock.ExpectQuery("SELECT generation FROM memory_repositories").WithArgs(request.RepositoryID, scope.Tenant, scope.Project, scope.Namespace).WillReturnRows(pgxmock.NewRows([]string{"generation"}).AddRow(0))
	mock.ExpectQuery("INSERT INTO memory_repository_revisions").WithArgs(request.RepositoryID, scope.Tenant, scope.Project, scope.Namespace, preview.Revision, preview.SnapshotDigest).WillReturnRows(pgxmock.NewRows([]string{"snapshot_digest"}).AddRow(preview.SnapshotDigest))
	mock.ExpectExec("INSERT INTO memory_repository_imports").WithArgs(pgxmock.AnyArg(), request.RepositoryID, scope.Tenant, scope.Project, scope.Namespace, preview.ID, int64(1), request.IdempotencyKey, pgxmock.AnyArg(), preview.SnapshotDigest, preview.PlanDigest, preview.WarningDigest, pgxmock.AnyArg(), string(memory.MemoryRepositoryImportQueued), pgxmock.AnyArg()).WillReturnResult(pgxmock.NewResult("INSERT", 1))
	mock.ExpectExec("INSERT INTO memory_repository_entry_tombstones").WithArgs(request.RepositoryID, scope.Tenant, scope.Project, scope.Namespace, preview.Entries[0].EntryKey).WillReturnResult(pgxmock.NewResult("INSERT", 0))
	mock.ExpectExec("INSERT INTO memory_repository_import_entries").WithArgs(pgxmock.AnyArg(), preview.Entries[0].EntryKey, string(memory.MemoryRepositoryEntryDuplicate), pgxmock.AnyArg()).WillReturnResult(pgxmock.NewResult("INSERT", 1))
	mock.ExpectExec("INSERT INTO memory_repository_import_attempts").WithArgs(pgxmock.AnyArg(), pgxmock.AnyArg(), preview.Entries[0].EntryKey, string(memory.MemoryRepositoryEntryDuplicate)).WillReturnResult(pgxmock.NewResult("INSERT", 1))
	mock.ExpectExec("INSERT INTO derived_work_items").WithArgs(pgxmock.AnyArg(), string(workqueue.WorkKindMemoryRepositoryImport), scope.Tenant, scope.Project, scope.Namespace, preview.PlanDigest, pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg()).WillReturnResult(pgxmock.NewResult("INSERT", 1))
	mock.ExpectExec("UPDATE memory_repositories SET generation").WithArgs(request.RepositoryID, scope.Tenant, scope.Project, scope.Namespace, int64(1)).WillReturnResult(pgxmock.NewResult("UPDATE", 1))
	mock.ExpectCommit()

	result, replayed, err := NewRepository(mock).AcceptMemoryRepositoryImport(context.Background(), request, preview)
	if err != nil || replayed || len(result.Entries) != 1 || result.Entries[0].DispatchStatus != memory.MemoryRepositoryEntryDuplicate {
		t.Fatalf("AcceptMemoryRepositoryImport() = %+v, replayed=%v, err=%v", result, replayed, err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestRepositoryReadsMemoryRepositoryOnlyWithinExactScope(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()
	scope := memory.Scope{Tenant: "tenant", Project: "project", Namespace: "namespace"}
	now := time.Date(2026, 10, 16, 12, 0, 0, 0, time.UTC)
	mock.ExpectQuery("FROM memory_repositories").WithArgs("00000000-0000-0000-0000-000000000123", scope.Tenant, scope.Project, scope.Namespace).WillReturnRows(pgxmock.NewRows([]string{"id", "tenant", "project", "namespace", "name", "source_url", "generation", "created_at"}).AddRow("00000000-0000-0000-0000-000000000123", scope.Tenant, scope.Project, scope.Namespace, "notes", "", 2, now))
	record, err := NewRepository(mock).ReadMemoryRepository(context.Background(), scope, "00000000-0000-0000-0000-000000000123")
	if err != nil || record.Generation != 2 {
		t.Fatalf("ReadMemoryRepository() = %+v, %v", record, err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestRepositorySavesMemoryRepositoryPreviewWithinExactScope(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()
	scope := memory.Scope{Tenant: "tenant", Project: "project", Namespace: "namespace"}
	now := time.Date(2026, 10, 16, 12, 0, 0, 0, time.UTC)
	preview := memory.MemoryRepositoryPreview{ID: "00000000-0000-0000-0000-000000000124", RepositoryID: "00000000-0000-0000-0000-000000000123", Scope: scope, Generation: 0, Revision: "r1", Actor: "agent", Reason: "review", SnapshotDigest: strings.Repeat("a", 64), PlanDigest: strings.Repeat("b", 64), WarningDigest: strings.Repeat("c", 64), ParserVersion: memory.MemoryRepositoryParserVersion, ExpiresAt: now.Add(24 * time.Hour), CreatedAt: now}
	mock.ExpectQuery("INSERT INTO memory_repository_previews").WithArgs(preview.ID, preview.RepositoryID, scope.Tenant, scope.Project, scope.Namespace, preview.Generation, preview.SnapshotDigest, preview.PlanDigest, preview.WarningDigest, preview.ParserVersion, pgxmock.AnyArg(), preview.ExpiresAt, preview.CreatedAt).WillReturnRows(pgxmock.NewRows([]string{"id", "repository_id", "tenant", "project", "namespace", "generation", "snapshot_digest", "plan_digest", "warning_digest", "parser_version", "payload", "expires_at", "created_at"}).AddRow(preview.ID, preview.RepositoryID, scope.Tenant, scope.Project, scope.Namespace, 0, preview.SnapshotDigest, preview.PlanDigest, preview.WarningDigest, preview.ParserVersion, []byte("{}"), preview.ExpiresAt, now))
	created, err := NewRepository(mock).SaveMemoryRepositoryPreview(context.Background(), preview)
	if err != nil || created.ID != preview.ID || created.Scope != scope {
		t.Fatalf("SaveMemoryRepositoryPreview() = %+v, %v", created, err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestRepositoryReadsMemoryRepositoryImportWithinExactScopeWithoutPayload(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()
	scope := memory.Scope{Tenant: "tenant", Project: "project", Namespace: "namespace"}
	now := time.Date(2026, 10, 16, 12, 0, 0, 0, time.UTC)
	mock.ExpectQuery("FROM memory_repository_imports").WithArgs("import", "repo", scope.Tenant, scope.Project, scope.Namespace).WillReturnRows(pgxmock.NewRows([]string{"id", "repository_id", "tenant", "project", "namespace", "preview_id", "generation", "idempotency_key", "request_fingerprint", "status", "created_at", "completed_at"}).AddRow("import", "repo", scope.Tenant, scope.Project, scope.Namespace, "preview", 1, "key", strings.Repeat("a", 64), "completed", now, nil))
	mock.ExpectQuery("FROM memory_repository_import_entries").WithArgs("import").WillReturnRows(pgxmock.NewRows([]string{"entry_key", "dispatch_status", "intent_id"}).AddRow(strings.Repeat("b", 64), "submitted", "00000000-0000-0000-0000-000000000125"))
	record, err := NewRepository(mock).ReadMemoryRepositoryImport(context.Background(), scope, "repo", "import")
	if err != nil || len(record.Entries) != 1 || record.Entries[0].Payload.Text != "" || record.Entries[0].IntentID == "" {
		t.Fatalf("ReadMemoryRepositoryImport() = %+v, %v", record, err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestRepositoryManifestReturnsOnlyCurrentEligibleCanonicalMappings(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()
	scope := memory.Scope{Tenant: "tenant", Project: "project", Namespace: "namespace"}
	mock.ExpectQuery("FROM memory_repository_imports").WithArgs("repo", scope.Tenant, scope.Project, scope.Namespace).WillReturnRows(pgxmock.NewRows([]string{"generation", "status"}).AddRow(1, "running"))
	mock.ExpectQuery(`payload->>'Path'`).WithArgs("repo", scope.Tenant, scope.Project, scope.Namespace, int64(1)).WillReturnRows(pgxmock.NewRows([]string{"memory_id", "version", "source_path"}).AddRow("00000000-0000-0000-0000-000000000125", 2, "MEMORY.md"))
	manifest, err := NewRepository(mock).ReadMemoryRepositoryManifest(context.Background(), scope, "repo")
	if err != nil || manifest.Generation != 1 || manifest.ProcessingStatus != memory.MemoryRepositoryImportRunning || len(manifest.Items) != 1 || manifest.Items[0].MemoryID == "" || manifest.Items[0].Version != 2 {
		t.Fatalf("ReadMemoryRepositoryManifest() = %+v, %v", manifest, err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}
