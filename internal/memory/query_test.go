package memory

import (
	"context"
	"testing"
	"time"
)

type stubQueryStore struct {
	gotListScope         Scope
	gotListIncludeHidden bool
	gotReadScope         Scope
	gotReadMemoryID      string
	gotReadIncludeHidden bool
	list                 []CanonicalMemory
	canonical            CanonicalMemory
	history              MemoryHistory
	provenance           []ProvenanceRecord
	err                  error
}

func (s *stubQueryStore) ListCanonicalMemories(ctx context.Context, scope Scope, includeHidden bool) ([]CanonicalMemory, error) {
	s.gotListScope = scope
	s.gotListIncludeHidden = includeHidden
	return s.list, s.err
}

func (s *stubQueryStore) ReadCanonicalMemory(ctx context.Context, scope Scope, memoryID string, includeHidden bool) (CanonicalMemory, error) {
	s.gotReadScope = scope
	s.gotReadMemoryID = memoryID
	s.gotReadIncludeHidden = includeHidden
	return s.canonical, s.err
}

func (s *stubQueryStore) ReadMemoryHistory(ctx context.Context, scope Scope, memoryID string, includeHidden bool) (MemoryHistory, error) {
	return s.history, s.err
}

func (s *stubQueryStore) ReadMemoryProvenance(ctx context.Context, scope Scope, memoryID string) ([]ProvenanceRecord, error) {
	return s.provenance, s.err
}

func TestListMemoriesInputValidateRejectsInvalidWindow(t *testing.T) {
	err := (ListMemoriesInput{
		Scope:    Scope{Tenant: "tenant-a", Project: "project-a", Namespace: "namespace-a"},
		TimeFrom: time.Date(2026, 6, 7, 15, 0, 0, 0, time.UTC),
		TimeTo:   time.Date(2026, 6, 7, 14, 0, 0, 0, time.UTC),
	}).Validate()
	if err == nil {
		t.Fatal("Validate() error = nil, want invalid time window")
	}
}

func TestMemoryResourceFromCanonicalRedactsDeletedPayload(t *testing.T) {
	resource := NewMemoryResource(CanonicalMemory{
		ID:      "mem_deleted",
		Scope:   Scope{Tenant: "tenant-a", Project: "project-a", Namespace: "namespace-a"},
		Class:   MemoryClassProfile,
		State:   MemoryStateDeleted,
		Content: "secret",
	})

	if resource.State != MemoryStateDeleted {
		t.Fatalf("State = %q, want %q", resource.State, MemoryStateDeleted)
	}
	if resource.Content != "" {
		t.Fatalf("Content = %q, want empty payload", resource.Content)
	}
}

func TestQueryServiceListMemoriesAppliesClassFilterAndLimit(t *testing.T) {
	store := &stubQueryStore{
		list: []CanonicalMemory{
			{
				ID:         "mem_profile",
				Scope:      Scope{Tenant: "tenant-a", Project: "project-a", Namespace: "namespace-a"},
				Class:      MemoryClassProfile,
				State:      MemoryStateActive,
				Content:    "User prefers concise answers.",
				CreatedAt:  time.Date(2026, 6, 7, 13, 0, 0, 0, time.UTC),
				ModifiedAt: time.Date(2026, 6, 7, 14, 0, 0, 0, time.UTC),
			},
			{
				ID:         "mem_episode",
				Scope:      Scope{Tenant: "tenant-a", Project: "project-a", Namespace: "namespace-a"},
				Class:      MemoryClassEpisodic,
				State:      MemoryStateActive,
				Content:    "User asked about flights.",
				CreatedAt:  time.Date(2026, 6, 7, 12, 0, 0, 0, time.UTC),
				ModifiedAt: time.Date(2026, 6, 7, 12, 30, 0, 0, time.UTC),
			},
		},
	}

	service := NewQueryService(store)
	page, err := service.ListMemories(context.Background(), ListMemoriesInput{
		Scope:   Scope{Tenant: "tenant-a", Project: "project-a", Namespace: "namespace-a"},
		Classes: []MemoryClass{MemoryClassProfile},
		Limit:   1,
	})
	if err != nil {
		t.Fatalf("ListMemories() error = %v", err)
	}

	if store.gotListIncludeHidden {
		t.Fatal("includeHidden = true, want false")
	}

	if len(page.Items) != 1 || page.Items[0].ID != "mem_profile" {
		t.Fatalf("page.Items = %+v, want one profile memory", page.Items)
	}
}

func TestQueryServiceGetMemoryUsesVisibleRead(t *testing.T) {
	store := &stubQueryStore{
		canonical: CanonicalMemory{
			ID:         "mem_123",
			Scope:      Scope{Tenant: "tenant-a", Project: "project-a", Namespace: "namespace-a"},
			Class:      MemoryClassProfile,
			State:      MemoryStateActive,
			Content:    "User prefers concise answers.",
			CreatedAt:  time.Date(2026, 6, 7, 13, 0, 0, 0, time.UTC),
			ModifiedAt: time.Date(2026, 6, 7, 14, 0, 0, 0, time.UTC),
		},
	}

	service := NewQueryService(store)
	resource, err := service.GetMemory(context.Background(), Scope{Tenant: "tenant-a", Project: "project-a", Namespace: "namespace-a"}, "mem_123")
	if err != nil {
		t.Fatalf("GetMemory() error = %v", err)
	}

	if store.gotReadIncludeHidden {
		t.Fatal("includeHidden = true, want false")
	}

	if resource.ID != "mem_123" {
		t.Fatalf("resource.ID = %q, want mem_123", resource.ID)
	}
}

func TestQueryServiceGetMemoryHistoryRedactsDeletedVersionPayload(t *testing.T) {
	store := &stubQueryStore{
		history: MemoryHistory{
			Memory: CanonicalMemory{
				ID:         "mem_123",
				Scope:      Scope{Tenant: "tenant-a", Project: "project-a", Namespace: "namespace-a"},
				Class:      MemoryClassProfile,
				State:      MemoryStateActive,
				Content:    "visible",
				CreatedAt:  time.Date(2026, 6, 7, 13, 0, 0, 0, time.UTC),
				ModifiedAt: time.Date(2026, 6, 7, 14, 0, 0, 0, time.UTC),
			},
			Versions: []MemoryVersion{
				{
					ID:         "ver_deleted",
					MemoryID:   "mem_123",
					Version:    2,
					State:      MemoryStateDeleted,
					Content:    "secret",
					CreatedAt:  time.Date(2026, 6, 7, 14, 0, 0, 0, time.UTC),
					ModifiedBy: "operator-a",
				},
			},
		},
	}

	service := NewQueryService(store)
	history, err := service.GetMemoryHistory(context.Background(), Scope{Tenant: "tenant-a", Project: "project-a", Namespace: "namespace-a"}, "mem_123")
	if err != nil {
		t.Fatalf("GetMemoryHistory() error = %v", err)
	}

	if history.Versions[0].Content != "" {
		t.Fatalf("deleted version content = %q, want empty payload", history.Versions[0].Content)
	}
}
