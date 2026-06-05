package governance

import (
	"context"
	"testing"
	"time"

	"github.com/FelixSeptem/stele/internal/memory"
	"github.com/FelixSeptem/stele/internal/policy"
)

type stubRawEventConsolidator struct {
	rawEventIDs []string
	err         error
}

func (s *stubRawEventConsolidator) ProcessByRawEvent(ctx context.Context, rawEventID string) error {
	if s.err != nil {
		return s.err
	}

	s.rawEventIDs = append(s.rawEventIDs, rawEventID)
	return nil
}

func TestPipelineProcessorProcessesClaimThroughExtractionAndConsolidation(t *testing.T) {
	now := time.Date(2026, 6, 2, 9, 0, 0, 0, time.UTC)
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

	candidates := &stubCandidateRepository{}
	completions := &stubRawEventCompletionRecorder{}
	consolidation := &stubRawEventConsolidator{}
	processor := PipelineProcessor{
		Extraction: ExtractionProcessor{
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
			Candidates:      candidates,
			RawEvents:       completions,
			Now:             func() time.Time { return now },
			NewCandidateID:  func() string { return "cand_123" },
			NewProvenanceID: func() string { return "prov_123" },
		},
		Consolidation: consolidation,
		RawEvents:     completions,
		Now:           func() time.Time { return now },
	}

	if err := processor.ProcessClaimedRawEvent(context.Background(), claim); err != nil {
		t.Fatalf("ProcessClaimedRawEvent() error = %v", err)
	}

	if len(candidates.created) != 1 {
		t.Fatalf("len(candidates.created) = %d, want %d", len(candidates.created), 1)
	}

	if len(consolidation.rawEventIDs) != 1 || consolidation.rawEventIDs[0] != claim.Event.ID {
		t.Fatalf("consolidation raw event ids = %#v, want [%q]", consolidation.rawEventIDs, claim.Event.ID)
	}

	if len(completions.inputs) != 1 {
		t.Fatalf("len(completions.inputs) = %d, want %d", len(completions.inputs), 1)
	}

	if completions.inputs[0].RawEventID != claim.Event.ID {
		t.Fatalf("processed raw event id = %q, want %q", completions.inputs[0].RawEventID, claim.Event.ID)
	}
}
