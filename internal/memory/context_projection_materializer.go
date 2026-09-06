package memory

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"sort"
	"strings"
	"time"
)

type ContextProjectionCandidate struct {
	Source     ContextProjectionSource
	Class      MemoryClass
	State      MemoryState
	Content    string
	Confidence float64
	ObservedAt time.Time
}

type MaterializeContextProjectionInput struct {
	Scope           Scope
	Kind            ContextProjectionKind
	Version         int64
	SchemaVersion   string
	Policy          ContextProjectionPolicy
	RendererVersion string
	Sources         []ContextProjectionCandidate
}

type ContextProjectionRebuilder interface {
	ReadLatestContextProjection(context.Context, Scope, ContextProjectionKind) (ContextProjection, error)
	CreateContextProjection(context.Context, ContextProjection) (ContextProjection, error)
}

type ContextProjectionSourceStore interface {
	ListContextProjectionCandidates(context.Context, Scope, ContextProjectionKind, int) ([]ContextProjectionCandidate, error)
}

type ContextProjectionRebuildRequest struct {
	Scope           Scope
	Kind            ContextProjectionKind
	Limit           int
	SchemaVersion   string
	Policy          ContextProjectionPolicy
	RendererVersion string
}

type ContextProjectionRebuildStore interface {
	ContextProjectionRebuilder
	ContextProjectionSourceStore
}

type ContextProjectionMaintenanceService struct {
	Store ContextProjectionRebuildStore
}

func (s ContextProjectionMaintenanceService) RebuildContextProjection(ctx context.Context, request ContextProjectionRebuildRequest) (ContextProjection, error) {
	return RebuildContextProjectionFromStore(ctx, request, s.Store)
}

func NewContextProjectionMaintenanceService(store ContextProjectionRebuildStore) ContextProjectionMaintenanceService {
	return ContextProjectionMaintenanceService{Store: store}
}

func RebuildContextProjectionFromStore(ctx context.Context, request ContextProjectionRebuildRequest, store ContextProjectionRebuildStore) (ContextProjection, error) {
	if store == nil {
		return ContextProjection{}, fmt.Errorf("projection rebuild store is not configured")
	}
	if err := request.Scope.Validate(); err != nil {
		return ContextProjection{}, err
	}
	if request.Limit <= 0 {
		return ContextProjection{}, fmt.Errorf("projection rebuild limit must be greater than zero")
	}
	latest, err := store.ReadLatestContextProjection(ctx, request.Scope, request.Kind)
	version := int64(1)
	if err == nil && latest.Version >= version {
		version = latest.Version + 1
	}
	sources, err := store.ListContextProjectionCandidates(ctx, request.Scope, request.Kind, request.Limit)
	if err != nil {
		return ContextProjection{}, fmt.Errorf("list projection source candidates: %w", err)
	}
	return RebuildContextProjection(ctx, MaterializeContextProjectionInput{
		Scope: request.Scope, Kind: request.Kind, Version: version, SchemaVersion: request.SchemaVersion,
		Policy: request.Policy, RendererVersion: request.RendererVersion, Sources: sources,
	}, store)
}

func RebuildContextProjection(ctx context.Context, input MaterializeContextProjectionInput, store ContextProjectionRebuilder) (ContextProjection, error) {
	if store == nil {
		return ContextProjection{}, fmt.Errorf("projection rebuild store is not configured")
	}
	latest, err := store.ReadLatestContextProjection(ctx, input.Scope, input.Kind)
	if err == nil && latest.Version >= input.Version {
		input.Version = latest.Version + 1
	}
	projection, err := MaterializeContextProjection(ctx, input)
	if err != nil {
		return ContextProjection{}, err
	}
	return store.CreateContextProjection(ctx, projection)
}

// MaterializeContextProjection builds a new derived projection from an
// authorized source snapshot. It never mutates source records.
func MaterializeContextProjection(ctx context.Context, input MaterializeContextProjectionInput) (ContextProjection, error) {
	if err := input.Scope.Validate(); err != nil {
		return ContextProjection{}, err
	}
	if !input.Kind.Valid() {
		return ContextProjection{}, fmt.Errorf("invalid projection kind %q", input.Kind)
	}
	if input.Version <= 0 {
		return ContextProjection{}, fmt.Errorf("projection version must be greater than zero")
	}
	if strings.TrimSpace(input.SchemaVersion) == "" || strings.TrimSpace(input.RendererVersion) == "" {
		return ContextProjection{}, fmt.Errorf("schema and renderer versions are required")
	}
	if err := input.Policy.Validate(); err != nil {
		return ContextProjection{}, err
	}
	watermark := ContextProjectionWatermark{}
	items := make([]ContextProjectionItem, 0, len(input.Sources))
	for _, candidate := range input.Sources {
		select {
		case <-ctx.Done():
			return ContextProjection{}, ctx.Err()
		default:
		}
		if candidate.Source.Scope.Normalized() != input.Scope.Normalized() {
			continue
		}
		if err := candidate.Source.Validate(); err != nil {
			continue
		}
		text := strings.TrimSpace(candidate.Content)
		decision := ResolveContextProjectionPolicy(input.Policy, input.Kind, candidate.Class, candidate.State, candidate.Confidence, len([]byte(text)), candidate.Source.Kind == ContextProjectionSourceRawEvent)
		if !decision.Include {
			continue
		}
		item := ContextProjectionItem{ID: projectionItemID(candidate), Source: candidate.Source, Text: text, Class: candidate.Class, LifecycleState: candidate.State, SortKey: projectionSortKey(candidate)}
		if candidate.Source.Kind == ContextProjectionSourceCanonicalVersion {
			watermark.CanonicalVersionIDs = append(watermark.CanonicalVersionIDs, candidate.Source.ID)
		} else {
			watermark.RawEventIDs = append(watermark.RawEventIDs, candidate.Source.ID)
		}
		item.Citation = ProjectionCitation{MemoryID: candidate.Source.MemoryID, Operation: "context_projection"}
		if candidate.Source.Kind == ContextProjectionSourceRawEvent {
			item.Citation.RawEventID = candidate.Source.ID
		}
		if candidate.Source.Kind == ContextProjectionSourceCanonicalVersion && item.Citation.MemoryID == "" {
			item.Citation.MemoryID = candidate.Source.ID
		}
		// Item-level source validation is repeated after rendering so malformed
		// evidence can never enter the derived record.
		if err := item.Validate(input.Scope); err == nil {
			items = append(items, item)
		}
	}
	sort.Strings(watermark.CanonicalVersionIDs)
	sort.Strings(watermark.RawEventIDs)
	SortContextProjectionItems(items)
	projection := ContextProjection{ID: projectionID(input), Scope: input.Scope.Normalized(), Kind: input.Kind, Version: input.Version, SchemaVersion: input.SchemaVersion, PolicyVersion: input.Policy.Version, RendererVersion: input.RendererVersion, SourceWatermark: watermark, Status: ContextProjectionStatusActive, Items: items, CreatedAt: time.Now().UTC(), UpdatedAt: time.Now().UTC()}
	if err := projection.Validate(); err != nil {
		return ContextProjection{}, err
	}
	return projection, nil
}

func projectionItemID(candidate ContextProjectionCandidate) string {
	s := fmt.Sprintf("%s:%s:%d:%s", candidate.Source.Kind, candidate.Source.ID, candidate.Source.Version, candidate.Content)
	digest := sha256.Sum256([]byte(s))
	return hex.EncodeToString(digest[:])
}

func projectionID(input MaterializeContextProjectionInput) string {
	s := fmt.Sprintf("%s:%s:%s:%s:%d", input.Scope.Tenant, input.Scope.Project, input.Scope.Namespace, input.Kind, input.Version)
	digest := sha256.Sum256([]byte(s))
	return fmt.Sprintf("%s-%s-%s-%s-%s", hex.EncodeToString(digest[0:4]), hex.EncodeToString(digest[4:6]), hex.EncodeToString(digest[6:8]), hex.EncodeToString(digest[8:10]), hex.EncodeToString(digest[10:16]))
}

func projectionSortKey(candidate ContextProjectionCandidate) string {
	return fmt.Sprintf("%02d:%s:%020d", classOrder(candidate.Class), candidate.Source.ID, candidate.Source.Version)
}

func classOrder(class MemoryClass) int {
	switch class {
	case MemoryClassProfile:
		return 1
	case MemoryClassSummary:
		return 2
	case MemoryClassEpisodic:
		return 3
	case MemoryClassProcedural:
		return 4
	case MemoryClassRelation:
		return 5
	default:
		return 9
	}
}
