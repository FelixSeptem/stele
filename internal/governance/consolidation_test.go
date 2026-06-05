package governance

import (
	"context"
	"testing"
	"time"

	"github.com/FelixSeptem/stele/internal/memory"
	"github.com/FelixSeptem/stele/internal/policy"
)

type stubCanonicalRepository struct {
	latest     map[string]memory.CanonicalMemory
	promoted   []CanonicalPromotion
	promoteErr error
}

func (s *stubCanonicalRepository) GetLatestCanonicalByScopeAndClass(ctx context.Context, scope memory.Scope, class memory.MemoryClass) (memory.CanonicalMemory, bool, error) {
	key := scope.Tenant + "/" + scope.Project + "/" + scope.Namespace + "/" + string(class)
	value, ok := s.latest[key]
	return value, ok, nil
}

func (s *stubCanonicalRepository) PromoteCandidate(ctx context.Context, input CanonicalPromotion) (memory.CanonicalMemory, memory.MemoryVersion, error) {
	if s.promoteErr != nil {
		return memory.CanonicalMemory{}, memory.MemoryVersion{}, s.promoteErr
	}

	s.promoted = append(s.promoted, input)
	return memory.CanonicalMemory{
			ID:         input.MemoryID,
			Scope:      input.Candidate.Scope,
			Class:      input.Candidate.Class,
			State:      memory.MemoryStateActive,
			Content:    input.Candidate.Content,
			CreatedAt:  input.CreatedAt,
			ModifiedAt: input.CreatedAt,
		}, memory.MemoryVersion{
			ID:        input.VersionID,
			MemoryID:  input.MemoryID,
			Version:   input.Version,
			State:     memory.MemoryStateActive,
			Content:   input.Candidate.Content,
			CreatedAt: input.CreatedAt,
		}, nil
}

func TestRuleBasedConsolidatorProfilePromotesIntoNewCanonicalMemory(t *testing.T) {
	now := time.Date(2026, 6, 1, 14, 0, 0, 0, time.UTC)
	candidate := CandidateMemory{
		ID:               "cand_123",
		SourceRawEventID: "evt_123",
		Scope: memory.Scope{
			Tenant:    "tenant-a",
			Project:   "project-a",
			Namespace: "namespace-a",
		},
		Class:          memory.MemoryClassProfile,
		Content:        "User prefers concise answers.",
		Confidence:     0.91,
		Importance:     0.84,
		Freshness:      0.74,
		Sensitivity:    SensitivityLow,
		Mutability:     MutabilityMutable,
		RetentionClass: policy.RetentionClassDurable,
		Status:         CandidateStatusPending,
		CreatedAt:      now,
		UpdatedAt:      now,
	}

	result, err := RuleBasedConsolidator{}.Decide(context.Background(), candidate, memory.CanonicalMemory{}, false)
	if err != nil {
		t.Fatalf("Decide() error = %v", err)
	}

	if result.Action != ConsolidationActionPromote {
		t.Fatalf("Action = %q, want %q", result.Action, ConsolidationActionPromote)
	}
}

func TestRuleBasedConsolidatorProfileSupersedesConflictingMutableCanonical(t *testing.T) {
	now := time.Date(2026, 6, 1, 14, 5, 0, 0, time.UTC)
	candidate := CandidateMemory{
		ID:               "cand_123",
		SourceRawEventID: "evt_123",
		Scope: memory.Scope{
			Tenant:    "tenant-a",
			Project:   "project-a",
			Namespace: "namespace-a",
		},
		Class:          memory.MemoryClassProfile,
		Content:        "User prefers concise answers.",
		Confidence:     0.91,
		Importance:     0.84,
		Freshness:      0.74,
		Sensitivity:    SensitivityLow,
		Mutability:     MutabilityMutable,
		RetentionClass: policy.RetentionClassDurable,
		Status:         CandidateStatusPending,
		CreatedAt:      now,
		UpdatedAt:      now,
	}
	existing := memory.CanonicalMemory{
		ID:         "mem_123",
		Scope:      candidate.Scope,
		Class:      memory.MemoryClassProfile,
		State:      memory.MemoryStateActive,
		Content:    "User prefers detailed answers.",
		CreatedAt:  now.Add(-time.Hour),
		ModifiedAt: now.Add(-time.Hour),
	}

	result, err := RuleBasedConsolidator{}.Decide(context.Background(), candidate, existing, true)
	if err != nil {
		t.Fatalf("Decide() error = %v", err)
	}

	if result.Action != ConsolidationActionSupersede {
		t.Fatalf("Action = %q, want %q", result.Action, ConsolidationActionSupersede)
	}
}

func TestRuleBasedConsolidatorEpisodicContradictionCoexists(t *testing.T) {
	now := time.Date(2026, 6, 1, 14, 10, 0, 0, time.UTC)
	candidate := CandidateMemory{
		ID:               "cand_episodic",
		SourceRawEventID: "evt_episodic",
		Scope: memory.Scope{
			Tenant:    "tenant-a",
			Project:   "project-a",
			Namespace: "namespace-a",
		},
		Class:          memory.MemoryClassEpisodic,
		Content:        "User visited Shanghai yesterday.",
		Confidence:     0.88,
		Importance:     0.63,
		Freshness:      0.91,
		Sensitivity:    SensitivityLow,
		Mutability:     MutabilityImmutable,
		RetentionClass: policy.RetentionClassSession,
		Status:         CandidateStatusPending,
		CreatedAt:      now,
		UpdatedAt:      now,
	}
	existing := memory.CanonicalMemory{
		ID:         "mem_epi",
		Scope:      candidate.Scope,
		Class:      memory.MemoryClassEpisodic,
		State:      memory.MemoryStateActive,
		Content:    "User visited Beijing yesterday.",
		CreatedAt:  now.Add(-time.Hour),
		ModifiedAt: now.Add(-time.Hour),
	}

	result, err := RuleBasedConsolidator{}.Decide(context.Background(), candidate, existing, true)
	if err != nil {
		t.Fatalf("Decide() error = %v", err)
	}

	if result.Action != ConsolidationActionCoexist {
		t.Fatalf("Action = %q, want %q", result.Action, ConsolidationActionCoexist)
	}
}

func TestConsolidationProcessorPromotesAndTransitionsCandidate(t *testing.T) {
	now := time.Date(2026, 6, 1, 14, 20, 0, 0, time.UTC)
	candidate := CandidateMemory{
		ID:               "cand_123",
		SourceRawEventID: "evt_123",
		Scope: memory.Scope{
			Tenant:    "tenant-a",
			Project:   "project-a",
			Namespace: "namespace-a",
		},
		Class:          memory.MemoryClassProfile,
		Content:        "User prefers concise answers.",
		Confidence:     0.91,
		Importance:     0.84,
		Freshness:      0.74,
		Sensitivity:    SensitivityLow,
		Mutability:     MutabilityMutable,
		RetentionClass: policy.RetentionClassDurable,
		Status:         CandidateStatusPending,
		CreatedAt:      now,
		UpdatedAt:      now,
	}

	candidates := &stubCandidateRepository{
		listByRawEventResult: []CandidateMemory{candidate},
	}
	canonicals := &stubCanonicalRepository{}
	processor := ConsolidationProcessor{
		Candidates:      candidates,
		Canonicals:      canonicals,
		Consolidator:    RuleBasedConsolidator{},
		Now:             func() time.Time { return now },
		NewMemoryID:     func() string { return "mem_123" },
		NewVersionID:    func() string { return "ver_123" },
		NewProvenanceID: func() string { return "prov_123" },
	}

	if err := processor.ProcessByRawEvent(context.Background(), "evt_123"); err != nil {
		t.Fatalf("ProcessByRawEvent() error = %v", err)
	}

	if len(canonicals.promoted) != 1 {
		t.Fatalf("len(canonicals.promoted) = %d, want %d", len(canonicals.promoted), 1)
	}

	if len(candidates.transitions) != 1 {
		t.Fatalf("len(candidates.transitions) = %d, want %d", len(candidates.transitions), 1)
	}

	if candidates.transitions[0].ToStatus != CandidateStatusPromoted {
		t.Fatalf("transition status = %q, want %q", candidates.transitions[0].ToStatus, CandidateStatusPromoted)
	}
}

func TestConsolidationProcessorSuppressesLowConfidenceCandidate(t *testing.T) {
	now := time.Date(2026, 6, 1, 14, 25, 0, 0, time.UTC)
	candidate := CandidateMemory{
		ID:               "cand_low",
		SourceRawEventID: "evt_low",
		Scope: memory.Scope{
			Tenant:    "tenant-a",
			Project:   "project-a",
			Namespace: "namespace-a",
		},
		Class:          memory.MemoryClassProfile,
		Content:        "Possibly prefers concise answers.",
		Confidence:     0.2,
		Importance:     0.3,
		Freshness:      0.4,
		Sensitivity:    SensitivityLow,
		Mutability:     MutabilityMutable,
		RetentionClass: policy.RetentionClassDurable,
		Status:         CandidateStatusPending,
		CreatedAt:      now,
		UpdatedAt:      now,
	}

	candidates := &stubCandidateRepository{
		listByRawEventResult: []CandidateMemory{candidate},
	}
	processor := ConsolidationProcessor{
		Candidates:      candidates,
		Canonicals:      &stubCanonicalRepository{},
		Consolidator:    RuleBasedConsolidator{},
		Now:             func() time.Time { return now },
		NewMemoryID:     func() string { return "mem_unused" },
		NewVersionID:    func() string { return "ver_unused" },
		NewProvenanceID: func() string { return "prov_456" },
	}

	if err := processor.ProcessByRawEvent(context.Background(), "evt_low"); err != nil {
		t.Fatalf("ProcessByRawEvent() error = %v", err)
	}

	if len(candidates.transitions) != 1 {
		t.Fatalf("len(candidates.transitions) = %d, want %d", len(candidates.transitions), 1)
	}

	if candidates.transitions[0].ToStatus != CandidateStatusSuppressed {
		t.Fatalf("transition status = %q, want %q", candidates.transitions[0].ToStatus, CandidateStatusSuppressed)
	}
}
