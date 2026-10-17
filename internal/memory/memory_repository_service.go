package memory

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
)

var ErrMemoryRepositoryFeatureDisabled = fmt.Errorf("memory repository import is disabled")

type MemoryRepositoryStore interface {
	RegisterMemoryRepository(context.Context, MemoryRepositoryRecord) (MemoryRepositoryRecord, error)
	ReadMemoryRepository(context.Context, Scope, string) (MemoryRepositoryRecord, error)
	SaveMemoryRepositoryPreview(context.Context, MemoryRepositoryPreview) (MemoryRepositoryPreview, error)
	ReadMemoryRepositoryPreview(context.Context, Scope, string, string) (MemoryRepositoryPreview, error)
	AcceptMemoryRepositoryImport(context.Context, MemoryRepositoryImportRequest, MemoryRepositoryPreview) (MemoryRepositoryImportRecord, bool, error)
	ReadMemoryRepositoryImport(context.Context, Scope, string, string) (MemoryRepositoryImportRecord, error)
	ReadMemoryRepositoryManifest(context.Context, Scope, string) (MemoryRepositoryManifest, error)
}

type MemoryRepositoryService struct {
	Enabled bool
	Store   MemoryRepositoryStore
	Now     func() time.Time
	NewID   func() string
}

func (s MemoryRepositoryService) Register(ctx context.Context, scope Scope, name, sourceURL string) (MemoryRepositoryRecord, error) {
	if !s.Enabled {
		return MemoryRepositoryRecord{}, ErrMemoryRepositoryFeatureDisabled
	}
	if s.Store == nil {
		return MemoryRepositoryRecord{}, fmt.Errorf("memory repository store is not configured")
	}
	now, newID := s.defaults()
	record := MemoryRepositoryRecord{ID: newID(), Scope: scope.Normalized(), Name: strings.TrimSpace(name), SourceURL: strings.TrimSpace(sourceURL), CreatedAt: now()}
	if err := record.Validate(); err != nil {
		return MemoryRepositoryRecord{}, err
	}
	return s.Store.RegisterMemoryRepository(ctx, record)
}

func (s MemoryRepositoryService) Preview(ctx context.Context, request MemoryRepositoryPreviewRequest) (MemoryRepositoryPreview, error) {
	if !s.Enabled {
		return MemoryRepositoryPreview{}, ErrMemoryRepositoryFeatureDisabled
	}
	if s.Store == nil {
		return MemoryRepositoryPreview{}, fmt.Errorf("memory repository store is not configured")
	}
	repository, err := s.Store.ReadMemoryRepository(ctx, request.Scope, request.RepositoryID)
	if err != nil {
		return MemoryRepositoryPreview{}, err
	}
	if repository.Scope.Normalized() != request.Scope.Normalized() {
		return MemoryRepositoryPreview{}, fmt.Errorf("memory repository unavailable in scope")
	}
	if repository.Generation != request.ExpectedGeneration {
		return MemoryRepositoryPreview{}, ErrMemoryRepositoryStaleGeneration
	}
	now, newID := s.defaults()
	preview, err := BuildMemoryRepositoryPreview(request, now())
	if err != nil {
		return MemoryRepositoryPreview{}, err
	}
	preview.ID = newID()
	return s.Store.SaveMemoryRepositoryPreview(ctx, preview)
}

// Submit accepts exactly the immutable plan that was previewed. The store owns
// the atomic generation CAS and idempotent replay lookup; this service rejects
// stale or altered review evidence before any durable dispatch exists.
func (s MemoryRepositoryService) Submit(ctx context.Context, request MemoryRepositoryImportRequest) (MemoryRepositoryImportRecord, bool, error) {
	if !s.Enabled {
		return MemoryRepositoryImportRecord{}, false, ErrMemoryRepositoryFeatureDisabled
	}
	if s.Store == nil {
		return MemoryRepositoryImportRecord{}, false, fmt.Errorf("memory repository store is not configured")
	}
	if err := request.Validate(); err != nil {
		return MemoryRepositoryImportRecord{}, false, err
	}
	preview, err := s.Store.ReadMemoryRepositoryPreview(ctx, request.Scope, request.RepositoryID, request.PreviewID)
	if err != nil {
		return MemoryRepositoryImportRecord{}, false, err
	}
	now, _ := s.defaults()
	if !preview.ExpiresAt.After(now()) {
		return MemoryRepositoryImportRecord{}, false, ErrMemoryRepositoryPreviewExpired
	}
	if preview.RepositoryID != request.RepositoryID || preview.Scope.Normalized() != request.Scope.Normalized() || preview.Generation != request.ExpectedGeneration || preview.PlanDigest != request.PlanDigest || preview.WarningDigest != request.WarningDigest {
		return MemoryRepositoryImportRecord{}, false, ErrMemoryRepositoryIdempotencyConflict
	}
	return s.Store.AcceptMemoryRepositoryImport(ctx, request, preview)
}

func (s MemoryRepositoryService) ReadImport(ctx context.Context, scope Scope, repositoryID, importID string) (MemoryRepositoryImportRecord, error) {
	if !s.Enabled {
		return MemoryRepositoryImportRecord{}, ErrMemoryRepositoryFeatureDisabled
	}
	if s.Store == nil {
		return MemoryRepositoryImportRecord{}, fmt.Errorf("memory repository store is not configured")
	}
	return s.Store.ReadMemoryRepositoryImport(ctx, scope, repositoryID, importID)
}

func (s MemoryRepositoryService) Manifest(ctx context.Context, scope Scope, repositoryID string) (MemoryRepositoryManifest, error) {
	if !s.Enabled {
		return MemoryRepositoryManifest{}, ErrMemoryRepositoryFeatureDisabled
	}
	if s.Store == nil {
		return MemoryRepositoryManifest{}, fmt.Errorf("memory repository store is not configured")
	}
	return s.Store.ReadMemoryRepositoryManifest(ctx, scope, repositoryID)
}

func (s MemoryRepositoryService) defaults() (func() time.Time, func() string) {
	now := s.Now
	if now == nil {
		now = func() time.Time { return time.Now().UTC() }
	}
	newID := s.NewID
	if newID == nil {
		newID = uuid.NewString
	}
	return now, newID
}
