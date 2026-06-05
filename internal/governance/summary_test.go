package governance

import (
	"context"
	"testing"
	"time"

	"github.com/FelixSeptem/stele/internal/memory"
	"github.com/FelixSeptem/stele/internal/policy"
)

type stubCompactionSource struct {
	candidates []CandidateMemory
	err        error
}

func (s stubCompactionSource) ListCandidatesForCompaction(ctx context.Context, scope memory.Scope, cutoff time.Time, limit int) ([]CandidateMemory, error) {
	if s.err != nil {
		return nil, s.err
	}

	return s.candidates, nil
}

type stubSummaryRepository struct {
	created []SummaryMemoryRecord
	err     error
}

func (s *stubSummaryRepository) CreateSummaryMemory(ctx context.Context, input SummaryMemoryRecord) (memory.CanonicalMemory, memory.MemoryVersion, error) {
	if s.err != nil {
		return memory.CanonicalMemory{}, memory.MemoryVersion{}, s.err
	}

	s.created = append(s.created, input)
	return memory.CanonicalMemory{
			ID:         input.MemoryID,
			Scope:      input.Scope,
			Class:      memory.MemoryClassSummary,
			State:      memory.MemoryStateActive,
			Content:    input.Content,
			CreatedAt:  input.CreatedAt,
			ModifiedAt: input.CreatedAt,
		}, memory.MemoryVersion{
			ID:        input.VersionID,
			MemoryID:  input.MemoryID,
			Version:   1,
			State:     memory.MemoryStateActive,
			Content:   input.Content,
			CreatedAt: input.CreatedAt,
		}, nil
}

func TestDeterministicSummarizerSummarizeCluster(t *testing.T) {
	cluster := SummaryCluster{
		Scope: memory.Scope{
			Tenant:    "tenant-a",
			Project:   "project-a",
			Namespace: "namespace-a",
		},
		Candidates: []CandidateMemory{
			newPromotedEpisodicCandidate("cand_1", "evt_1", "User asked about hotels."),
			newPromotedEpisodicCandidate("cand_2", "evt_2", "Agent recommended two hotels."),
		},
	}

	summary, err := DeterministicSummarizer{}.Summarize(context.Background(), cluster)
	if err != nil {
		t.Fatalf("Summarize() error = %v", err)
	}

	if summary.Class != memory.MemoryClassSummary {
		t.Fatalf("Class = %q, want %q", summary.Class, memory.MemoryClassSummary)
	}

	if len(summary.EvidenceRawEventIDs) != 2 {
		t.Fatalf("len(EvidenceRawEventIDs) = %d, want %d", len(summary.EvidenceRawEventIDs), 2)
	}

	if summary.Content == "" {
		t.Fatal("Content = empty, want non-empty deterministic summary")
	}
}

func TestSummaryProcessorCreatesSummaryMemoryForEligibleCluster(t *testing.T) {
	now := time.Date(2026, 6, 1, 16, 0, 0, 0, time.UTC)
	scope := memory.Scope{
		Tenant:    "tenant-a",
		Project:   "project-a",
		Namespace: "namespace-a",
	}

	repo := &stubSummaryRepository{}
	processor := SummaryProcessor{
		Source: stubCompactionSource{
			candidates: []CandidateMemory{
				newPromotedEpisodicCandidate("cand_1", "evt_1", "User asked about hotels."),
				newPromotedEpisodicCandidate("cand_2", "evt_2", "Agent recommended two hotels."),
			},
		},
		Repository:     repo,
		Summarizer:     DeterministicSummarizer{},
		Now:            func() time.Time { return now },
		NewMemoryID:    func() string { return "sum_123" },
		NewVersionID:   func() string { return "sum_ver_123" },
		MinClusterSize: 2,
		ClusterLimit:   10,
	}

	if err := processor.CompactScope(context.Background(), scope, now); err != nil {
		t.Fatalf("CompactScope() error = %v", err)
	}

	if len(repo.created) != 1 {
		t.Fatalf("len(repo.created) = %d, want %d", len(repo.created), 1)
	}

	if repo.created[0].MemoryID != "sum_123" {
		t.Fatalf("MemoryID = %q, want %q", repo.created[0].MemoryID, "sum_123")
	}

	if len(repo.created[0].EvidenceRawEventIDs) != 2 {
		t.Fatalf("len(EvidenceRawEventIDs) = %d, want %d", len(repo.created[0].EvidenceRawEventIDs), 2)
	}
}

func TestSummaryProcessorSkipsSmallCluster(t *testing.T) {
	now := time.Date(2026, 6, 1, 16, 5, 0, 0, time.UTC)
	repo := &stubSummaryRepository{}
	processor := SummaryProcessor{
		Source: stubCompactionSource{
			candidates: []CandidateMemory{
				newPromotedEpisodicCandidate("cand_1", "evt_1", "Single event only."),
			},
		},
		Repository:     repo,
		Summarizer:     DeterministicSummarizer{},
		Now:            func() time.Time { return now },
		NewMemoryID:    func() string { return "sum_unused" },
		NewVersionID:   func() string { return "sum_ver_unused" },
		MinClusterSize: 2,
		ClusterLimit:   10,
	}

	if err := processor.CompactScope(context.Background(), memory.Scope{
		Tenant:    "tenant-a",
		Project:   "project-a",
		Namespace: "namespace-a",
	}, now); err != nil {
		t.Fatalf("CompactScope() error = %v", err)
	}

	if len(repo.created) != 0 {
		t.Fatalf("len(repo.created) = %d, want %d", len(repo.created), 0)
	}
}

func newPromotedEpisodicCandidate(id, rawEventID, content string) CandidateMemory {
	now := time.Date(2026, 6, 1, 16, 0, 0, 0, time.UTC)
	return CandidateMemory{
		ID:               id,
		SourceRawEventID: rawEventID,
		Scope: memory.Scope{
			Tenant:    "tenant-a",
			Project:   "project-a",
			Namespace: "namespace-a",
		},
		Class:          memory.MemoryClassEpisodic,
		Content:        content,
		Confidence:     0.88,
		Importance:     0.67,
		Freshness:      0.91,
		Sensitivity:    SensitivityLow,
		Mutability:     MutabilityImmutable,
		RetentionClass: policy.RetentionClassSession,
		Status:         CandidateStatusPromoted,
		CreatedAt:      now,
		UpdatedAt:      now,
	}
}
