package governance

import (
	"testing"
	"time"

	"github.com/FelixSeptem/stele/internal/memory"
)

func TestClaimPendingRawEventsInputValidate(t *testing.T) {
	now := time.Date(2026, 5, 31, 15, 0, 0, 0, time.UTC)

	valid := ClaimPendingRawEventsInput{
		WorkerID:      "worker-a",
		BatchSize:     16,
		LeaseDuration: 2 * time.Minute,
		Now:           now,
	}
	if err := valid.Validate(); err != nil {
		t.Fatalf("Validate() error = %v", err)
	}

	cases := []ClaimPendingRawEventsInput{
		{
			BatchSize:     16,
			LeaseDuration: time.Minute,
			Now:           now,
		},
		{
			WorkerID:      "worker-a",
			LeaseDuration: time.Minute,
			Now:           now,
		},
		{
			WorkerID:  "worker-a",
			BatchSize: 16,
			Now:       now,
		},
		{
			WorkerID:      "worker-a",
			BatchSize:     16,
			LeaseDuration: time.Minute,
		},
	}

	for _, input := range cases {
		if err := input.Validate(); err == nil {
			t.Fatalf("Validate() error = nil for invalid input %+v", input)
		}
	}
}

func TestClaimedRawEventValidate(t *testing.T) {
	claimedAt := time.Date(2026, 5, 31, 15, 0, 0, 0, time.UTC)
	claim := ClaimedRawEvent{
		Event: memory.RawEvent{
			ID: "evt_123",
			Scope: memory.Scope{
				Tenant:    "tenant-a",
				Project:   "project-a",
				Namespace: "namespace-a",
			},
			EventType: "conversation.message",
			Content:   "hello",
			CreatedAt: claimedAt.Add(-time.Minute),
		},
		WorkerID:   "worker-a",
		ClaimedAt:  claimedAt,
		LeaseUntil: claimedAt.Add(2 * time.Minute),
		Attempt:    1,
	}

	if err := claim.Validate(); err != nil {
		t.Fatalf("Validate() error = %v", err)
	}

	invalidClaims := []ClaimedRawEvent{
		{
			Event:      claim.Event,
			ClaimedAt:  claim.ClaimedAt,
			LeaseUntil: claim.LeaseUntil,
			Attempt:    1,
		},
		{
			Event: memory.RawEvent{
				ID:        "evt_123",
				EventType: "conversation.message",
				Content:   "hello",
				CreatedAt: claimedAt,
			},
			WorkerID:   "worker-a",
			ClaimedAt:  claimedAt,
			LeaseUntil: claimedAt.Add(2 * time.Minute),
			Attempt:    1,
		},
		{
			Event:      claim.Event,
			WorkerID:   "worker-a",
			LeaseUntil: claim.LeaseUntil,
			Attempt:    1,
		},
		{
			Event:      claim.Event,
			WorkerID:   "worker-a",
			ClaimedAt:  claim.ClaimedAt,
			LeaseUntil: claim.ClaimedAt,
			Attempt:    1,
		},
		{
			Event:      claim.Event,
			WorkerID:   "worker-a",
			ClaimedAt:  claim.ClaimedAt,
			LeaseUntil: claim.LeaseUntil,
		},
	}

	for _, invalid := range invalidClaims {
		if err := invalid.Validate(); err == nil {
			t.Fatalf("Validate() error = nil for invalid claim %+v", invalid)
		}
	}
}
