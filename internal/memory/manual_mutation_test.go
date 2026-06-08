package memory_test

import (
	"context"
	"testing"
	"time"

	"github.com/FelixSeptem/stele/internal/memory"
)

type stubManualMutationProcessor struct {
	gotCreate     memory.ManualCreateMemoryRecord
	gotUpdate     memory.ManualUpdateMemoryRecord
	gotMerge      memory.ManualMergeMemoryRecord
	gotReclassify memory.ManualReclassifyMemoryRecord
	canonical     memory.CanonicalMemory
	err           error
}

func (s *stubManualMutationProcessor) CreateMemory(ctx context.Context, record memory.ManualCreateMemoryRecord) (memory.CanonicalMemory, error) {
	s.gotCreate = record
	if s.err != nil {
		return memory.CanonicalMemory{}, s.err
	}
	return s.canonical, nil
}

func (s *stubManualMutationProcessor) UpdateMemory(ctx context.Context, record memory.ManualUpdateMemoryRecord) (memory.CanonicalMemory, error) {
	s.gotUpdate = record
	if s.err != nil {
		return memory.CanonicalMemory{}, s.err
	}
	return s.canonical, nil
}

func (s *stubManualMutationProcessor) MergeMemory(ctx context.Context, record memory.ManualMergeMemoryRecord) (memory.CanonicalMemory, error) {
	s.gotMerge = record
	if s.err != nil {
		return memory.CanonicalMemory{}, s.err
	}
	return s.canonical, nil
}

func (s *stubManualMutationProcessor) ReclassifyMemory(ctx context.Context, record memory.ManualReclassifyMemoryRecord) (memory.CanonicalMemory, error) {
	s.gotReclassify = record
	if s.err != nil {
		return memory.CanonicalMemory{}, s.err
	}
	return s.canonical, nil
}

func TestManualCreateMemoryInputRejectsExcludedSummaryClass(t *testing.T) {
	err := (memory.ManualCreateMemoryInput{
		Scope:   memory.Scope{Tenant: "tenant-a", Project: "project-a", Namespace: "namespace-a"},
		Class:   memory.MemoryClassSummary,
		Content: "summary content",
		Reason:  "seed summary",
		Actor:   "operator-a",
	}).Validate()
	if err == nil {
		t.Fatal("Validate() error = nil, want excluded class error")
	}
}

func TestManualMutationServiceCreateMemoryNormalizesRecord(t *testing.T) {
	processor := &stubManualMutationProcessor{
		canonical: memory.CanonicalMemory{
			ID:         "mem_123",
			Scope:      memory.Scope{Tenant: "tenant-a", Project: "project-a", Namespace: "namespace-a"},
			Class:      memory.MemoryClassProfile,
			State:      memory.MemoryStateActive,
			Content:    "seed knowledge",
			CreatedAt:  time.Date(2026, 6, 7, 20, 0, 0, 0, time.UTC),
			ModifiedAt: time.Date(2026, 6, 7, 20, 0, 0, 0, time.UTC),
		},
	}
	service := memory.ManualMutationService{
		Processor:    processor,
		Now:          func() time.Time { return time.Date(2026, 6, 7, 20, 0, 0, 0, time.UTC) },
		NewMemoryID:  func() string { return "mem_123" },
		NewVersionID: func() string { return "ver_123" },
	}

	resource, err := service.CreateMemory(context.Background(), memory.ManualCreateMemoryInput{
		Scope:     memory.Scope{Tenant: "tenant-a", Project: "project-a", Namespace: "namespace-a"},
		Class:     memory.MemoryClassProfile,
		Content:   "seed knowledge",
		Reason:    "seed profile",
		Actor:     "operator-a",
		RequestID: "req_123",
	})
	if err != nil {
		t.Fatalf("CreateMemory() error = %v", err)
	}

	if processor.gotCreate.MemoryID != "mem_123" {
		t.Fatalf("MemoryID = %q, want mem_123", processor.gotCreate.MemoryID)
	}
	if processor.gotCreate.VersionID != "ver_123" {
		t.Fatalf("VersionID = %q, want ver_123", processor.gotCreate.VersionID)
	}
	if processor.gotCreate.CreatedAt.IsZero() {
		t.Fatal("CreatedAt = zero, want normalized timestamp")
	}
	if resource.ID != "mem_123" {
		t.Fatalf("resource.ID = %q, want mem_123", resource.ID)
	}
}

func TestManualMutationServiceUpdateMemoryCarriesExpectedVersion(t *testing.T) {
	processor := &stubManualMutationProcessor{
		canonical: memory.CanonicalMemory{
			ID:         "mem_123",
			Scope:      memory.Scope{Tenant: "tenant-a", Project: "project-a", Namespace: "namespace-a"},
			Class:      memory.MemoryClassProfile,
			State:      memory.MemoryStateActive,
			Content:    "corrected",
			CreatedAt:  time.Date(2026, 6, 7, 19, 0, 0, 0, time.UTC),
			ModifiedAt: time.Date(2026, 6, 7, 20, 5, 0, 0, time.UTC),
		},
	}
	service := memory.ManualMutationService{
		Processor:    processor,
		Now:          func() time.Time { return time.Date(2026, 6, 7, 20, 5, 0, 0, time.UTC) },
		NewVersionID: func() string { return "ver_124" },
	}

	_, err := service.UpdateMemory(context.Background(), memory.ManualUpdateMemoryInput{
		Scope:           memory.Scope{Tenant: "tenant-a", Project: "project-a", Namespace: "namespace-a"},
		MemoryID:        "mem_123",
		Content:         "corrected",
		ExpectedVersion: 2,
		Reason:          "manual correction",
		Actor:           "operator-a",
		RequestID:       "req_124",
	})
	if err != nil {
		t.Fatalf("UpdateMemory() error = %v", err)
	}

	if processor.gotUpdate.ExpectedVersion != 2 {
		t.Fatalf("ExpectedVersion = %d, want 2", processor.gotUpdate.ExpectedVersion)
	}
	if processor.gotUpdate.VersionID != "ver_124" {
		t.Fatalf("VersionID = %q, want ver_124", processor.gotUpdate.VersionID)
	}
}

func TestManualReclassifyMemoryInputRejectsRelationTargetClass(t *testing.T) {
	err := (memory.ManualReclassifyMemoryInput{
		Scope:           memory.Scope{Tenant: "tenant-a", Project: "project-a", Namespace: "namespace-a"},
		MemoryID:        "mem_123",
		TargetClass:     memory.MemoryClassRelation,
		ExpectedVersion: 3,
		Reason:          "wrong class",
		Actor:           "operator-a",
	}).Validate()
	if err == nil {
		t.Fatal("Validate() error = nil, want excluded relation class error")
	}
}

func TestManualMutationServiceMergeMemoryNormalizesSourceAndTarget(t *testing.T) {
	processor := &stubManualMutationProcessor{
		canonical: memory.CanonicalMemory{
			ID:         "mem_target",
			Scope:      memory.Scope{Tenant: "tenant-a", Project: "project-a", Namespace: "namespace-a"},
			Class:      memory.MemoryClassProfile,
			State:      memory.MemoryStateActive,
			Content:    "merged content",
			CreatedAt:  time.Date(2026, 6, 7, 19, 0, 0, 0, time.UTC),
			ModifiedAt: time.Date(2026, 6, 7, 20, 10, 0, 0, time.UTC),
		},
	}
	service := memory.ManualMutationService{
		Processor:    processor,
		Now:          func() time.Time { return time.Date(2026, 6, 7, 20, 10, 0, 0, time.UTC) },
		NewVersionID: func() string { return "ver_merge" },
	}

	_, err := service.MergeMemory(context.Background(), memory.ManualMergeMemoryInput{
		Scope:           memory.Scope{Tenant: "tenant-a", Project: "project-a", Namespace: "namespace-a"},
		TargetMemoryID:  "mem_target",
		SourceMemoryID:  "mem_source",
		Content:         "merged content",
		ExpectedVersion: 4,
		Reason:          "dedupe duplicate",
		Actor:           "operator-a",
		RequestID:       "req_merge",
	})
	if err != nil {
		t.Fatalf("MergeMemory() error = %v", err)
	}

	if processor.gotMerge.TargetMemoryID != "mem_target" {
		t.Fatalf("TargetMemoryID = %q, want mem_target", processor.gotMerge.TargetMemoryID)
	}
	if processor.gotMerge.SourceMemoryID != "mem_source" {
		t.Fatalf("SourceMemoryID = %q, want mem_source", processor.gotMerge.SourceMemoryID)
	}
	if processor.gotMerge.ExpectedVersion != 4 {
		t.Fatalf("ExpectedVersion = %d, want 4", processor.gotMerge.ExpectedVersion)
	}
}

func TestManualMutationServiceReclassifyMemoryCarriesTargetClass(t *testing.T) {
	processor := &stubManualMutationProcessor{
		canonical: memory.CanonicalMemory{
			ID:         "mem_123",
			Scope:      memory.Scope{Tenant: "tenant-a", Project: "project-a", Namespace: "namespace-a"},
			Class:      memory.MemoryClassProcedural,
			State:      memory.MemoryStateActive,
			Content:    "respond concisely",
			CreatedAt:  time.Date(2026, 6, 7, 18, 0, 0, 0, time.UTC),
			ModifiedAt: time.Date(2026, 6, 7, 20, 15, 0, 0, time.UTC),
		},
	}
	service := memory.ManualMutationService{
		Processor:    processor,
		Now:          func() time.Time { return time.Date(2026, 6, 7, 20, 15, 0, 0, time.UTC) },
		NewVersionID: func() string { return "ver_reclass" },
	}

	_, err := service.ReclassifyMemory(context.Background(), memory.ManualReclassifyMemoryInput{
		Scope:           memory.Scope{Tenant: "tenant-a", Project: "project-a", Namespace: "namespace-a"},
		MemoryID:        "mem_123",
		TargetClass:     memory.MemoryClassProcedural,
		ExpectedVersion: 3,
		Reason:          "fix class",
		Actor:           "operator-a",
		RequestID:       "req_reclass",
	})
	if err != nil {
		t.Fatalf("ReclassifyMemory() error = %v", err)
	}

	if processor.gotReclassify.TargetClass != memory.MemoryClassProcedural {
		t.Fatalf("TargetClass = %q, want procedural", processor.gotReclassify.TargetClass)
	}
	if processor.gotReclassify.VersionID != "ver_reclass" {
		t.Fatalf("VersionID = %q, want ver_reclass", processor.gotReclassify.VersionID)
	}
}
