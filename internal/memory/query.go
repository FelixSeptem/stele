package memory

import (
	"context"
	"fmt"
	"time"
)

type ListMemoriesInput struct {
	Scope    Scope
	Classes  []MemoryClass
	TimeFrom time.Time
	TimeTo   time.Time
	Limit    int
}

func (i ListMemoriesInput) Validate() error {
	if err := i.Scope.Validate(); err != nil {
		return err
	}

	if !i.TimeFrom.IsZero() && !i.TimeTo.IsZero() && i.TimeFrom.After(i.TimeTo) {
		return fmt.Errorf("time_from must be before or equal to time_to")
	}

	if i.Limit < 0 {
		return fmt.Errorf("limit must be greater than or equal to zero")
	}

	return nil
}

type MemoryResource struct {
	ID         string      `json:"id"`
	Scope      Scope       `json:"scope"`
	Class      MemoryClass `json:"class"`
	State      MemoryState `json:"state"`
	Content    string      `json:"content"`
	CreatedAt  time.Time   `json:"created_at"`
	ModifiedAt time.Time   `json:"modified_at"`
	// Temporal is the bounded validity summary. It reports shape and provenance
	// only -- not the raw recorded/valid instants -- so an ordinary read learns
	// whether the interval was asserted or inferred without exposing the
	// timestamps that privileged reads retain.
	Temporal TemporalMetadata `json:"temporal"`
}

type MemoryPage struct {
	Items []MemoryResource `json:"items"`
}

type QueryStore interface {
	ListCanonicalMemories(ctx context.Context, scope Scope, includeHidden bool) ([]CanonicalMemory, error)
	ReadCanonicalMemory(ctx context.Context, scope Scope, memoryID string, includeHidden bool) (CanonicalMemory, error)
	ReadMemoryHistory(ctx context.Context, scope Scope, memoryID string, includeHidden bool) (MemoryHistory, error)
	ReadMemoryProvenance(ctx context.Context, scope Scope, memoryID string) ([]ProvenanceRecord, error)
}

type QueryService struct {
	store QueryStore
}

func NewQueryService(store QueryStore) *QueryService {
	return &QueryService{store: store}
}

func (s *QueryService) ListMemories(ctx context.Context, input ListMemoriesInput) (MemoryPage, error) {
	if err := input.Validate(); err != nil {
		return MemoryPage{}, err
	}
	if s.store == nil {
		return MemoryPage{}, fmt.Errorf("query store is not configured")
	}

	memories, err := s.store.ListCanonicalMemories(ctx, input.Scope, false)
	if err != nil {
		return MemoryPage{}, err
	}

	items := make([]MemoryResource, 0, len(memories))
	for _, canonical := range memories {
		if !matchesMemoryClasses(canonical.Class, input.Classes) {
			continue
		}
		if !withinMemoryWindow(canonical.ModifiedAt, input.TimeFrom, input.TimeTo) {
			continue
		}

		items = append(items, NewMemoryResource(canonical))
		if input.Limit > 0 && len(items) >= input.Limit {
			break
		}
	}

	return MemoryPage{Items: items}, nil
}

func (s *QueryService) GetMemory(ctx context.Context, scope Scope, memoryID string) (MemoryResource, error) {
	if s.store == nil {
		return MemoryResource{}, fmt.Errorf("query store is not configured")
	}

	canonical, err := s.store.ReadCanonicalMemory(ctx, scope, memoryID, false)
	if err != nil {
		return MemoryResource{}, err
	}

	return NewMemoryResource(canonical), nil
}

func (s *QueryService) GetMemoryHistory(ctx context.Context, scope Scope, memoryID string) (MemoryHistory, error) {
	if s.store == nil {
		return MemoryHistory{}, fmt.Errorf("query store is not configured")
	}

	history, err := s.store.ReadMemoryHistory(ctx, scope, memoryID, false)
	if err != nil {
		return MemoryHistory{}, err
	}
	history.Memory.Content = NewMemoryResource(history.Memory).Content

	for i := range history.Versions {
		if history.Versions[i].State == MemoryStateDeleted {
			history.Versions[i].Content = ""
		}
	}

	return history, nil
}

func (s *QueryService) GetMemoryProvenance(ctx context.Context, scope Scope, memoryID string) ([]ProvenanceRecord, error) {
	if s.store == nil {
		return nil, fmt.Errorf("query store is not configured")
	}

	return s.store.ReadMemoryProvenance(ctx, scope, memoryID)
}

// GetMemoryProvenanceView returns provenance together with the bounded temporal
// summary. It exists so a caller inspecting lineage can see whether the memory's
// validity was asserted or inferred without a second, privileged round trip.
func (s *QueryService) GetMemoryProvenanceView(ctx context.Context, scope Scope, memoryID string) (ProvenanceView, error) {
	if s.store == nil {
		return ProvenanceView{}, fmt.Errorf("query store is not configured")
	}

	records, err := s.store.ReadMemoryProvenance(ctx, scope, memoryID)
	if err != nil {
		return ProvenanceView{}, err
	}

	// The canonical read is scoped and excludes hidden rows, so an ordinary
	// caller cannot use this path to observe a forgotten memory's validity.
	canonical, err := s.store.ReadCanonicalMemory(ctx, scope, memoryID, false)
	if err != nil {
		return ProvenanceView{}, err
	}

	return ProvenanceView{
		MemoryID:   memoryID,
		Provenance: records,
		Temporal:   NewTemporalMetadata(canonical.TemporalValidity),
	}, nil
}

func NewMemoryResource(c CanonicalMemory) MemoryResource {
	content := c.Content
	if c.State == MemoryStateDeleted {
		content = ""
	}

	return MemoryResource{
		ID:         c.ID,
		Scope:      c.Scope,
		Class:      c.Class,
		State:      c.State,
		Content:    content,
		CreatedAt:  c.CreatedAt,
		ModifiedAt: c.ModifiedAt,
		Temporal:   NewTemporalMetadata(c.TemporalValidity),
	}
}

func matchesMemoryClasses(class MemoryClass, classes []MemoryClass) bool {
	if len(classes) == 0 {
		return true
	}

	for _, want := range classes {
		if class == want {
			return true
		}
	}

	return false
}

func withinMemoryWindow(at, timeFrom, timeTo time.Time) bool {
	if !timeFrom.IsZero() && at.Before(timeFrom) {
		return false
	}
	if !timeTo.IsZero() && at.After(timeTo) {
		return false
	}

	return true
}
