package governance

import (
	"testing"
	"time"

	"github.com/FelixSeptem/stele/internal/memory"
	"github.com/FelixSeptem/stele/internal/policy"
)

func TestCandidateMemoryValidate(t *testing.T) {
	now := time.Date(2026, 5, 31, 16, 0, 0, 0, time.UTC)
	candidate := CandidateMemory{
		ID:             "cand_123",
		SourceRawEventID: "evt_123",
		Scope: memory.Scope{
			Tenant:    "tenant-a",
			Project:   "project-a",
			Namespace: "namespace-a",
		},
		Class:          memory.MemoryClassProfile,
		Content:        "User prefers concise answers.",
		Confidence:     0.91,
		Importance:     0.83,
		Freshness:      0.77,
		Sensitivity:    SensitivityLow,
		Mutability:     MutabilityMutable,
		RetentionClass: policy.RetentionClassDurable,
		Status:         CandidateStatusPending,
		CreatedAt:      now,
		UpdatedAt:      now,
	}

	if err := candidate.Validate(); err != nil {
		t.Fatalf("Validate() error = %v", err)
	}

	invalid := []CandidateMemory{
		{
			Scope:          candidate.Scope,
			Class:          candidate.Class,
			Content:        candidate.Content,
			Confidence:     candidate.Confidence,
			Importance:     candidate.Importance,
			Freshness:      candidate.Freshness,
			Sensitivity:    candidate.Sensitivity,
			Mutability:     candidate.Mutability,
			RetentionClass: candidate.RetentionClass,
			Status:         candidate.Status,
			CreatedAt:      now,
			UpdatedAt:      now,
		},
		{
			ID:               candidate.ID,
			SourceRawEventID: candidate.SourceRawEventID,
			Class:            candidate.Class,
			Content:          candidate.Content,
			Confidence:       candidate.Confidence,
			Importance:       candidate.Importance,
			Freshness:        candidate.Freshness,
			Sensitivity:      candidate.Sensitivity,
			Mutability:       candidate.Mutability,
			RetentionClass:   candidate.RetentionClass,
			Status:           candidate.Status,
			CreatedAt:        now,
			UpdatedAt:        now,
		},
		{
			ID:               candidate.ID,
			SourceRawEventID: candidate.SourceRawEventID,
			Scope:            candidate.Scope,
			Class:            candidate.Class,
			Confidence:       candidate.Confidence,
			Importance:       candidate.Importance,
			Freshness:        candidate.Freshness,
			Sensitivity:      candidate.Sensitivity,
			Mutability:       candidate.Mutability,
			RetentionClass:   candidate.RetentionClass,
			Status:           candidate.Status,
			CreatedAt:        now,
			UpdatedAt:        now,
		},
		{
			ID:               candidate.ID,
			SourceRawEventID: candidate.SourceRawEventID,
			Scope:            candidate.Scope,
			Class:            candidate.Class,
			Content:          candidate.Content,
			Confidence:       1.2,
			Importance:       candidate.Importance,
			Freshness:        candidate.Freshness,
			Sensitivity:      candidate.Sensitivity,
			Mutability:       candidate.Mutability,
			RetentionClass:   candidate.RetentionClass,
			Status:           candidate.Status,
			CreatedAt:        now,
			UpdatedAt:        now,
		},
		{
			ID:               candidate.ID,
			SourceRawEventID: candidate.SourceRawEventID,
			Scope:            candidate.Scope,
			Class:            candidate.Class,
			Content:          candidate.Content,
			Confidence:       candidate.Confidence,
			Importance:       candidate.Importance,
			Freshness:        candidate.Freshness,
			Sensitivity:      Sensitivity("secret"),
			Mutability:       candidate.Mutability,
			RetentionClass:   candidate.RetentionClass,
			Status:           candidate.Status,
			CreatedAt:        now,
			UpdatedAt:        now,
		},
	}

	for _, candidate := range invalid {
		if err := candidate.Validate(); err == nil {
			t.Fatalf("Validate() error = nil for invalid candidate %+v", candidate)
		}
	}
}

func TestCandidateStatusTransitionValidate(t *testing.T) {
	input := CandidateStatusTransition{
		CandidateID: "cand_123",
		ToStatus:    CandidateStatusPromoted,
		UpdatedAt:   time.Date(2026, 5, 31, 16, 30, 0, 0, time.UTC),
	}
	if err := input.Validate(); err != nil {
		t.Fatalf("Validate() error = %v", err)
	}

	invalid := []CandidateStatusTransition{
		{
			ToStatus:  CandidateStatusPromoted,
			UpdatedAt: input.UpdatedAt,
		},
		{
			CandidateID: input.CandidateID,
			UpdatedAt:   input.UpdatedAt,
		},
		{
			CandidateID: input.CandidateID,
			ToStatus:    CandidateStatusPromoted,
		},
	}

	for _, transition := range invalid {
		if err := transition.Validate(); err == nil {
			t.Fatalf("Validate() error = nil for invalid transition %+v", transition)
		}
	}
}
