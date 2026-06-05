package governance

import (
	"context"
	"testing"
	"time"

	"github.com/FelixSeptem/stele/internal/memory"
	"github.com/FelixSeptem/stele/internal/policy"
)

func TestRuleBasedExtractorExtractConversationMessage(t *testing.T) {
	extractor := RuleBasedExtractor{}
	event := memory.RawEvent{
		ID: "evt_123",
		Scope: memory.Scope{
			Tenant:    "tenant-a",
			Project:   "project-a",
			Namespace: "namespace-a",
		},
		EventType: "conversation.message",
		Content:   "  user said hello to the agent  ",
		CreatedAt: time.Date(2026, 6, 1, 9, 0, 0, 0, time.UTC),
	}

	candidates, err := extractor.Extract(context.Background(), event)
	if err != nil {
		t.Fatalf("Extract() error = %v", err)
	}

	if len(candidates) != 1 {
		t.Fatalf("len(candidates) = %d, want %d", len(candidates), 1)
	}

	candidate := candidates[0]
	if candidate.Class != memory.MemoryClassEpisodic {
		t.Fatalf("Class = %q, want %q", candidate.Class, memory.MemoryClassEpisodic)
	}

	if candidate.Content != "user said hello to the agent" {
		t.Fatalf("Content = %q, want normalized event content", candidate.Content)
	}

	if candidate.Mutability != MutabilityImmutable {
		t.Fatalf("Mutability = %q, want %q", candidate.Mutability, MutabilityImmutable)
	}

	if candidate.RetentionClass != policy.RetentionClassSession {
		t.Fatalf("RetentionClass = %q, want %q", candidate.RetentionClass, policy.RetentionClassSession)
	}
}

func TestExtractedCandidateToCandidateMemory(t *testing.T) {
	extracted := ExtractedCandidate{
		Class:          memory.MemoryClassProfile,
		Content:        "User prefers concise answers.",
		Confidence:     0.91,
		Importance:     0.82,
		Freshness:      0.73,
		Sensitivity:    SensitivityLow,
		Mutability:     MutabilityMutable,
		RetentionClass: policy.RetentionClassDurable,
	}
	event := memory.RawEvent{
		ID: "evt_123",
		Scope: memory.Scope{
			Tenant:    "tenant-a",
			Project:   "project-a",
			Namespace: "namespace-a",
		},
	}
	now := time.Date(2026, 6, 1, 9, 5, 0, 0, time.UTC)

	candidate := extracted.ToCandidateMemory("cand_123", event, now)
	if candidate.SourceRawEventID != event.ID {
		t.Fatalf("SourceRawEventID = %q, want %q", candidate.SourceRawEventID, event.ID)
	}

	if candidate.Status != CandidateStatusPending {
		t.Fatalf("Status = %q, want %q", candidate.Status, CandidateStatusPending)
	}

	if err := candidate.Validate(); err != nil {
		t.Fatalf("candidate.Validate() error = %v", err)
	}
}
