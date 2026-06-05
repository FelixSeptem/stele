package governance

import (
	"context"
	"testing"
	"time"

	"github.com/FelixSeptem/stele/internal/memory"
	"github.com/FelixSeptem/stele/internal/policy"
)

type stubExtractor struct {
	outputs []ExtractedCandidate
	err     error
}

func (s stubExtractor) Extract(ctx context.Context, event memory.RawEvent) ([]ExtractedCandidate, error) {
	if s.err != nil {
		return nil, s.err
	}

	return s.outputs, nil
}

type stubCandidateRepository struct {
	created              []CandidateMemory
	createdProvenance    []memory.ProvenanceRecord
	listedByRawEventID   []string
	listByRawEventResult []CandidateMemory
	transitions          []CandidateStatusTransition
	transitionProv       []memory.ProvenanceRecord
	createErr            error
	listErr              error
	transitionErr        error
}

func (s *stubCandidateRepository) CreateCandidate(ctx context.Context, candidate CandidateMemory, provenance memory.ProvenanceRecord) (CandidateMemory, error) {
	if s.createErr != nil {
		return CandidateMemory{}, s.createErr
	}

	s.created = append(s.created, candidate)
	s.createdProvenance = append(s.createdProvenance, provenance)
	return candidate, nil
}

func (s *stubCandidateRepository) ListCandidatesByRawEvent(ctx context.Context, rawEventID string) ([]CandidateMemory, error) {
	s.listedByRawEventID = append(s.listedByRawEventID, rawEventID)
	if s.listErr != nil {
		return nil, s.listErr
	}

	return s.listByRawEventResult, nil
}

func (s *stubCandidateRepository) TransitionCandidateStatus(ctx context.Context, transition CandidateStatusTransition, provenance memory.ProvenanceRecord) (CandidateMemory, error) {
	if s.transitionErr != nil {
		return CandidateMemory{}, s.transitionErr
	}

	s.transitions = append(s.transitions, transition)
	s.transitionProv = append(s.transitionProv, provenance)

	for _, candidate := range s.listByRawEventResult {
		if candidate.ID == transition.CandidateID {
			candidate.Status = transition.ToStatus
			candidate.UpdatedAt = transition.UpdatedAt
			return candidate, nil
		}
	}

	return CandidateMemory{
		ID:        transition.CandidateID,
		Status:    transition.ToStatus,
		UpdatedAt: transition.UpdatedAt,
	}, nil
}

type stubRawEventCompletionRecorder struct {
	inputs []CompleteClaimedRawEventInput
	err    error
}

func (s *stubRawEventCompletionRecorder) MarkRawEventProcessed(ctx context.Context, input CompleteClaimedRawEventInput) error {
	if s.err != nil {
		return s.err
	}

	s.inputs = append(s.inputs, input)
	return nil
}

func TestExtractionProcessorProcessClaimedRawEventCreatesCandidatesAndMarksProcessed(t *testing.T) {
	now := time.Date(2026, 6, 1, 9, 30, 0, 0, time.UTC)
	claim := ClaimedRawEvent{
		Event: memory.RawEvent{
			ID: "evt_123",
			Scope: memory.Scope{
				Tenant:    "tenant-a",
				Project:   "project-a",
				Namespace: "namespace-a",
			},
			EventType: "conversation.message",
			Content:   "User prefers concise answers.",
			CreatedAt: now.Add(-time.Minute),
		},
		WorkerID:   "worker-a",
		ClaimedAt:  now.Add(-30 * time.Second),
		LeaseUntil: now.Add(time.Minute),
		Attempt:    1,
	}

	repo := &stubCandidateRepository{}
	recorder := &stubRawEventCompletionRecorder{}
	processor := ExtractionProcessor{
		Extractor: stubExtractor{
			outputs: []ExtractedCandidate{{
				Class:          memory.MemoryClassProfile,
				Content:        "User prefers concise answers.",
				Confidence:     0.91,
				Importance:     0.84,
				Freshness:      0.72,
				Sensitivity:    SensitivityLow,
				Mutability:     MutabilityMutable,
				RetentionClass: policy.RetentionClassDurable,
			}},
		},
		Candidates:      repo,
		RawEvents:       recorder,
		Now:             func() time.Time { return now },
		NewCandidateID:  func() string { return "cand_123" },
		NewProvenanceID: func() string { return "prov_123" },
	}

	if err := processor.ProcessClaimedRawEvent(context.Background(), claim); err != nil {
		t.Fatalf("ProcessClaimedRawEvent() error = %v", err)
	}

	if len(repo.created) != 1 {
		t.Fatalf("len(repo.created) = %d, want %d", len(repo.created), 1)
	}

	if repo.created[0].SourceRawEventID != claim.Event.ID {
		t.Fatalf("created candidate raw event id = %q, want %q", repo.created[0].SourceRawEventID, claim.Event.ID)
	}

	if len(repo.createdProvenance) != 1 {
		t.Fatalf("len(repo.createdProvenance) = %d, want %d", len(repo.createdProvenance), 1)
	}

	if repo.createdProvenance[0].CandidateMemoryID != "cand_123" {
		t.Fatalf("CandidateMemoryID = %q, want %q", repo.createdProvenance[0].CandidateMemoryID, "cand_123")
	}

	if len(recorder.inputs) != 1 {
		t.Fatalf("len(recorder.inputs) = %d, want %d", len(recorder.inputs), 1)
	}

	if recorder.inputs[0].RawEventID != claim.Event.ID {
		t.Fatalf("processed raw event id = %q, want %q", recorder.inputs[0].RawEventID, claim.Event.ID)
	}
}
