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

// TestQueryServiceGetMemoryHistoryUsesOrdinaryRead proves the service path never
// asks the store for hidden records, so a forgotten memory's history cannot be
// assembled through the ordinary query surface.
func TestQueryServiceGetMemoryHistoryUsesOrdinaryRead(t *testing.T) {
	store := &stubQueryStore{
		history: MemoryHistory{
			Memory: CanonicalMemory{
				ID:    "mem_123",
				Scope: Scope{Tenant: "tenant-a", Project: "project-a", Namespace: "namespace-a"},
				Class: MemoryClassProfile,
			},
		},
	}

	service := NewQueryService(store)
	if _, err := service.GetMemoryHistory(context.Background(), Scope{Tenant: "tenant-a", Project: "project-a", Namespace: "namespace-a"}, "mem_123"); err != nil {
		t.Fatalf("GetMemoryHistory() error = %v", err)
	}

	if store.gotReadIncludeHidden {
		t.Fatal("store was asked to include hidden records; an ordinary read must not")
	}
}

// TestQueryServiceGetMemoryProvenanceViewScopesCanonicalRead proves the
// provenance view reads the canonical row through the ordinary, hidden-excluding
// path, so the bounded temporal summary cannot describe a hidden interval.
func TestQueryServiceGetMemoryProvenanceViewScopesCanonicalRead(t *testing.T) {
	recorded := time.Date(2026, 6, 21, 10, 0, 0, 0, time.UTC)
	scope := Scope{Tenant: "tenant-a", Project: "project-a", Namespace: "namespace-a"}

	store := &stubQueryStore{
		canonical: CanonicalMemory{
			ID:    "mem_123",
			Scope: scope,
			Class: MemoryClassProfile,
			TemporalValidity: TemporalValidity{
				TemporalFactID: "fact_123",
				IngestedAt:     recorded,
				ValidFrom:      recorded,
				ValiditySource: TemporalValiditySourceExplicit,
			},
		},
		provenance: []ProvenanceRecord{{ID: "prov_1", MemoryID: "mem_123"}},
	}

	service := NewQueryService(store)
	view, err := service.GetMemoryProvenanceView(context.Background(), scope, "mem_123")
	if err != nil {
		t.Fatalf("GetMemoryProvenanceView() error = %v", err)
	}

	if store.gotReadIncludeHidden {
		t.Fatal("store was asked to include hidden records; the provenance view must not")
	}
	if len(view.Provenance) != 1 || view.Provenance[0].ID != "prov_1" {
		t.Fatalf("Provenance = %+v, want one record", view.Provenance)
	}
	if view.Temporal.Origin != TemporalValidityOriginExplicit {
		t.Fatalf("Temporal.Origin = %q, want %q", view.Temporal.Origin, TemporalValidityOriginExplicit)
	}
}

// TestQueryServiceMemoryResourceReportsInferredOrigin proves the ordinary memory
// resource distinguishes an inferred interval from an asserted one.
func TestQueryServiceMemoryResourceReportsInferredOrigin(t *testing.T) {
	recorded := time.Date(2026, 6, 21, 10, 0, 0, 0, time.UTC)

	resource := NewMemoryResource(CanonicalMemory{
		ID:               "mem_123",
		Class:            MemoryClassProfile,
		State:            MemoryStateActive,
		Content:          "Likes concise answers",
		TemporalValidity: TemporalValidity{}.LegacyCurrentCompatible(recorded),
	})

	if resource.Temporal.Origin != TemporalValidityOriginInferred {
		t.Fatalf("Temporal.Origin = %q, want %q", resource.Temporal.Origin, TemporalValidityOriginInferred)
	}
	if !resource.Temporal.Set || !resource.Temporal.OpenEnded {
		t.Fatalf("Temporal = %+v, want a set open-ended interval", resource.Temporal)
	}
}

// TestQueryServiceMemoryResourceDerivedArtifactHasNoInterval proves a derived
// artifact reports no interval rather than an inferred one, so a caller cannot
// read a summary as carrying a fact-valid window of its own.
func TestQueryServiceMemoryResourceDerivedArtifactHasNoInterval(t *testing.T) {
	resource := NewMemoryResource(CanonicalMemory{
		ID:      "mem_summary",
		Class:   MemoryClassSummary,
		State:   MemoryStateActive,
		Content: "Summary of the week",
	})

	if resource.Temporal.Set {
		t.Fatalf("Temporal.Set = true, want false for a derived artifact")
	}
	if resource.Temporal.Origin != TemporalValidityOriginNone {
		t.Fatalf("Temporal.Origin = %q, want %q", resource.Temporal.Origin, TemporalValidityOriginNone)
	}
}
