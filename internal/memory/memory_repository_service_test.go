package memory

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"
)

type stubMemoryRepositoryStore struct {
	repository MemoryRepositoryRecord
	preview    MemoryRepositoryPreview
	saved      bool
	accepted   MemoryRepositoryImportRecord
	manifest   MemoryRepositoryManifest
}

func (s *stubMemoryRepositoryStore) RegisterMemoryRepository(_ context.Context, record MemoryRepositoryRecord) (MemoryRepositoryRecord, error) {
	s.repository = record
	return record, nil
}
func (s *stubMemoryRepositoryStore) ReadMemoryRepository(_ context.Context, _ Scope, _ string) (MemoryRepositoryRecord, error) {
	return s.repository, nil
}
func (s *stubMemoryRepositoryStore) SaveMemoryRepositoryPreview(_ context.Context, preview MemoryRepositoryPreview) (MemoryRepositoryPreview, error) {
	s.saved, s.preview = true, preview
	return preview, nil
}
func (s *stubMemoryRepositoryStore) ReadMemoryRepositoryPreview(_ context.Context, _ Scope, _ string, _ string) (MemoryRepositoryPreview, error) {
	return s.preview, nil
}
func (s *stubMemoryRepositoryStore) AcceptMemoryRepositoryImport(_ context.Context, request MemoryRepositoryImportRequest, preview MemoryRepositoryPreview) (MemoryRepositoryImportRecord, bool, error) {
	if s.accepted.ID != "" {
		return s.accepted, true, nil
	}
	s.accepted = MemoryRepositoryImportRecord{ID: "import", RepositoryID: request.RepositoryID, Scope: request.Scope, PreviewID: request.PreviewID, Generation: request.ExpectedGeneration + 1, IdempotencyKey: request.IdempotencyKey, RequestFingerprint: request.PlanDigest, Status: MemoryRepositoryImportQueued}
	return s.accepted, false, nil
}
func (s *stubMemoryRepositoryStore) ReadMemoryRepositoryImport(_ context.Context, _ Scope, _, _ string) (MemoryRepositoryImportRecord, error) {
	return s.accepted, nil
}
func (s *stubMemoryRepositoryStore) ReadMemoryRepositoryManifest(_ context.Context, _ Scope, _ string) (MemoryRepositoryManifest, error) {
	return s.manifest, nil
}

func TestMemoryRepositoryServiceDefaultsToDisabledAdmission(t *testing.T) {
	service := MemoryRepositoryService{}
	_, err := service.Register(context.Background(), Scope{Tenant: "t", Project: "p", Namespace: "n"}, "notes", "")
	if !errors.Is(err, ErrMemoryRepositoryFeatureDisabled) {
		t.Fatalf("Register() error = %v, want feature disabled", err)
	}
}

func TestMemoryRepositoryServiceSubmitBindsPreviewAndWarningAcknowledgement(t *testing.T) {
	scope := Scope{Tenant: "tenant", Project: "project", Namespace: "namespace"}
	now := time.Date(2026, 10, 16, 12, 0, 0, 0, time.UTC)
	preview := MemoryRepositoryPreview{ID: "preview", RepositoryID: "repo", Scope: scope, Generation: 2, PlanDigest: strings.Repeat("a", 64), WarningDigest: strings.Repeat("b", 64), ExpiresAt: now.Add(time.Hour)}
	store := &stubMemoryRepositoryStore{repository: MemoryRepositoryRecord{ID: "repo", Scope: scope, Generation: 2}, preview: preview}
	service := MemoryRepositoryService{Enabled: true, Store: store, Now: func() time.Time { return now }}
	result, replayed, err := service.Submit(context.Background(), MemoryRepositoryImportRequest{RepositoryID: "repo", PreviewID: "preview", Scope: scope, PlanDigest: preview.PlanDigest, WarningDigest: preview.WarningDigest, ExpectedGeneration: 2, IdempotencyKey: "key"})
	if err != nil || replayed || result.Generation != 3 {
		t.Fatalf("Submit() = %+v, %v, %v", result, replayed, err)
	}
	_, _, err = service.Submit(context.Background(), MemoryRepositoryImportRequest{RepositoryID: "repo", PreviewID: "preview", Scope: scope, PlanDigest: preview.PlanDigest, WarningDigest: strings.Repeat("c", 64), ExpectedGeneration: 2, IdempotencyKey: "key"})
	if err == nil {
		t.Fatal("Submit() accepted a changed warning acknowledgement")
	}
}

func TestMemoryRepositoryServicePreviewPersistsImmutablePlanWithoutIntentMutation(t *testing.T) {
	scope := Scope{Tenant: "tenant", Project: "project", Namespace: "namespace"}
	store := &stubMemoryRepositoryStore{repository: MemoryRepositoryRecord{ID: "repo", Scope: scope, Name: "notes", Generation: 0}}
	now := time.Date(2026, 10, 16, 12, 0, 0, 0, time.UTC)
	snapshot := MemoryRepositorySnapshot{FormatVersion: MemoryRepositoryFormatVersion, Revision: "r1", Files: []MemoryRepositoryFile{{Path: "MEMORY.md", Content: "- fact\n"}}}
	parsed, err := ParseMemoryRepositorySnapshot(snapshot)
	if err != nil {
		t.Fatal(err)
	}
	service := MemoryRepositoryService{Enabled: true, Store: store, Now: func() time.Time { return now }, NewID: func() string { return "preview" }}
	preview, err := service.Preview(context.Background(), MemoryRepositoryPreviewRequest{RepositoryID: "repo", Scope: scope, Actor: "agent", Reason: "review", Snapshot: snapshot, Selections: []MemoryRepositorySelection{{Path: "MEMORY.md", EntryKey: parsed.Entries[0].Key, Class: MemoryClassProfile, MemoryPath: "/"}}})
	if err != nil || !store.saved || preview.ID != "preview" || preview.Revision != "r1" {
		t.Fatalf("Preview() = %+v, saved=%v, err=%v", preview, store.saved, err)
	}
}

func TestMemoryRepositoryServiceReadsStatusAndNavigationOnlyWhenEnabled(t *testing.T) {
	scope := Scope{Tenant: "tenant", Project: "project", Namespace: "namespace"}
	store := &stubMemoryRepositoryStore{
		accepted: MemoryRepositoryImportRecord{ID: "import", RepositoryID: "repo", Scope: scope, Status: MemoryRepositoryImportCompleted},
		manifest: MemoryRepositoryManifest{RepositoryID: "repo", Generation: 1, ProcessingStatus: MemoryRepositoryImportCompleted},
	}
	service := MemoryRepositoryService{Enabled: true, Store: store}
	if result, err := service.ReadImport(context.Background(), scope, "repo", "import"); err != nil || result.ID != "import" {
		t.Fatalf("ReadImport() = %+v, %v", result, err)
	}
	if result, err := service.Manifest(context.Background(), scope, "repo"); err != nil || result.RepositoryID != "repo" {
		t.Fatalf("Manifest() = %+v, %v", result, err)
	}
	if _, err := (MemoryRepositoryService{}).Manifest(context.Background(), scope, "repo"); !errors.Is(err, ErrMemoryRepositoryFeatureDisabled) {
		t.Fatalf("disabled Manifest() error = %v", err)
	}
}
