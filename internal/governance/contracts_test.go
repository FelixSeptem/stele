package governance

import (
	"errors"
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

func TestRecordClaimedRawEventFailureInputValidate(t *testing.T) {
	failedAt := time.Date(2026, 6, 10, 12, 0, 0, 0, time.UTC)
	valid := RecordClaimedRawEventFailureInput{
		RawEventID:    "evt_123",
		WorkerID:      "worker-a",
		FailedAt:      failedAt,
		ErrorMessage:  "candidate extraction failed",
		NextAttemptAt: failedAt.Add(30 * time.Second),
	}

	if err := valid.Validate(); err != nil {
		t.Fatalf("Validate() error = %v", err)
	}

	invalidCases := []RecordClaimedRawEventFailureInput{
		{
			WorkerID:      "worker-a",
			FailedAt:      failedAt,
			ErrorMessage:  "candidate extraction failed",
			NextAttemptAt: failedAt.Add(30 * time.Second),
		},
		{
			RawEventID:    "evt_123",
			FailedAt:      failedAt,
			ErrorMessage:  "candidate extraction failed",
			NextAttemptAt: failedAt.Add(30 * time.Second),
		},
		{
			RawEventID:    "evt_123",
			WorkerID:      "worker-a",
			ErrorMessage:  "candidate extraction failed",
			NextAttemptAt: failedAt.Add(30 * time.Second),
		},
		{
			RawEventID:   "evt_123",
			WorkerID:     "worker-a",
			FailedAt:     failedAt,
			NextAttemptAt: failedAt.Add(30 * time.Second),
		},
		{
			RawEventID:    "evt_123",
			WorkerID:      "worker-a",
			FailedAt:      failedAt,
			ErrorMessage:  "candidate extraction failed",
			NextAttemptAt: failedAt,
		},
	}

	for _, invalid := range invalidCases {
		if err := invalid.Validate(); err == nil {
			t.Fatalf("Validate() error = nil for invalid input %+v", invalid)
		}
	}
}

func TestRenewClaimedRawEventLeaseInputValidate(t *testing.T) {
	renewedAt := time.Date(2026, 6, 10, 12, 10, 0, 0, time.UTC)
	valid := RenewClaimedRawEventLeaseInput{
		RawEventID: "evt_123",
		WorkerID:   "worker-a",
		RenewedAt:  renewedAt,
		LeaseUntil: renewedAt.Add(2 * time.Minute),
	}

	if err := valid.Validate(); err != nil {
		t.Fatalf("Validate() error = %v", err)
	}

	invalidCases := []RenewClaimedRawEventLeaseInput{
		{
			WorkerID:   "worker-a",
			RenewedAt:  renewedAt,
			LeaseUntil: renewedAt.Add(2 * time.Minute),
		},
		{
			RawEventID: "evt_123",
			RenewedAt:  renewedAt,
			LeaseUntil: renewedAt.Add(2 * time.Minute),
		},
		{
			RawEventID: "evt_123",
			WorkerID:   "worker-a",
			LeaseUntil: renewedAt.Add(2 * time.Minute),
		},
		{
			RawEventID: "evt_123",
			WorkerID:   "worker-a",
			RenewedAt:  renewedAt,
			LeaseUntil: renewedAt,
		},
	}

	for _, invalid := range invalidCases {
		if err := invalid.Validate(); err == nil {
			t.Fatalf("Validate() error = nil for invalid input %+v", invalid)
		}
	}
}

func TestRawEventGovernanceSnapshotDerivedState(t *testing.T) {
	now := time.Date(2026, 6, 11, 9, 0, 0, 0, time.UTC)

	cases := []struct {
		name     string
		snapshot RawEventGovernanceSnapshot
		want     GovernanceRawEventState
	}{
		{
			name: "processed wins over all other fields",
			snapshot: RawEventGovernanceSnapshot{
				Attempt:       3,
				ProcessedAt:   now.Add(-time.Minute),
				ExhaustedAt:   now.Add(-2 * time.Minute),
				LeaseUntil:    now.Add(time.Minute),
				NextAttemptAt: now.Add(time.Minute),
			},
			want: GovernanceRawEventStateProcessed,
		},
		{
			name: "exhausted wins over lease and retry wait",
			snapshot: RawEventGovernanceSnapshot{
				Attempt:       5,
				ExhaustedAt:   now.Add(-time.Minute),
				LeaseUntil:    now.Add(time.Minute),
				NextAttemptAt: now.Add(time.Minute),
			},
			want: GovernanceRawEventStateExhausted,
		},
		{
			name: "leased when active lease is in force",
			snapshot: RawEventGovernanceSnapshot{
				Attempt:    2,
				WorkerID:   "worker-a",
				LeaseUntil: now.Add(2 * time.Minute),
			},
			want: GovernanceRawEventStateLeased,
		},
		{
			name: "retry wait when next attempt is in the future",
			snapshot: RawEventGovernanceSnapshot{
				Attempt:       2,
				NextAttemptAt: now.Add(30 * time.Second),
			},
			want: GovernanceRawEventStateRetryWait,
		},
		{
			name: "pending when no blocking governance timestamps exist",
			snapshot: RawEventGovernanceSnapshot{
				Attempt: 1,
			},
			want: GovernanceRawEventStatePending,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := tc.snapshot.DerivedState(now); got != tc.want {
				t.Fatalf("DerivedState() = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestListGovernanceRawEventsInputValidate(t *testing.T) {
	now := time.Date(2026, 6, 11, 9, 5, 0, 0, time.UTC)
	attemptGTE := 2
	attemptLTE := 5
	cursor := GovernanceRawEventCursor{
		CreatedAt: now.Add(-time.Minute),
		RawEventID: "evt_123",
	}.Encode()

	valid := ListGovernanceRawEventsInput{
		Scope: memory.Scope{
			Tenant:    "tenant-a",
			Project:   "project-a",
			Namespace: "namespace-a",
		},
		State:         GovernanceRawEventStateRetryWait,
		EventType:     "conversation.message",
		AttemptGTE:    &attemptGTE,
		AttemptLTE:    &attemptLTE,
		FailedFrom:    now.Add(-time.Hour),
		FailedTo:      now,
		NextAttemptFrom: now,
		NextAttemptTo:   now.Add(time.Hour),
		Limit:         25,
		Cursor:        cursor,
		Now:           now,
	}
	if err := valid.Validate(); err != nil {
		t.Fatalf("Validate() error = %v", err)
	}

	negativeAttempt := -1
	tooSmallAttempt := 1
	tooLargeAttempt := 0
	invalidCases := []ListGovernanceRawEventsInput{
		{
			Scope: memory.Scope{Project: "project-a", Namespace: "namespace-a"},
			Limit: 25,
			Now:   now,
		},
		{
			Scope: memory.Scope{Tenant: "tenant-a", Project: "project-a", Namespace: "namespace-a"},
			State: GovernanceRawEventState("broken"),
			Limit: 25,
			Now:   now,
		},
		{
			Scope:      memory.Scope{Tenant: "tenant-a", Project: "project-a", Namespace: "namespace-a"},
			AttemptGTE: &negativeAttempt,
			Limit:      25,
			Now:        now,
		},
		{
			Scope:      memory.Scope{Tenant: "tenant-a", Project: "project-a", Namespace: "namespace-a"},
			AttemptGTE: &tooSmallAttempt,
			AttemptLTE: &tooLargeAttempt,
			Limit:      25,
			Now:        now,
		},
		{
			Scope:      memory.Scope{Tenant: "tenant-a", Project: "project-a", Namespace: "namespace-a"},
			FailedFrom: now,
			FailedTo:   now.Add(-time.Minute),
			Limit:      25,
			Now:        now,
		},
		{
			Scope:           memory.Scope{Tenant: "tenant-a", Project: "project-a", Namespace: "namespace-a"},
			NextAttemptFrom: now,
			NextAttemptTo:   now.Add(-time.Minute),
			Limit:           25,
			Now:             now,
		},
		{
			Scope:  memory.Scope{Tenant: "tenant-a", Project: "project-a", Namespace: "namespace-a"},
			Limit:  0,
			Now:    now,
		},
		{
			Scope:  memory.Scope{Tenant: "tenant-a", Project: "project-a", Namespace: "namespace-a"},
			Limit:  25,
			Cursor: "bad-cursor",
			Now:    now,
		},
		{
			Scope: memory.Scope{Tenant: "tenant-a", Project: "project-a", Namespace: "namespace-a"},
			Limit: 25,
		},
	}

	for _, input := range invalidCases {
		if err := input.Validate(); err == nil {
			t.Fatalf("Validate() error = nil for invalid input %+v", input)
		}
	}
}

func TestApplyGovernanceRecoveryInputValidate(t *testing.T) {
	now := time.Date(2026, 6, 11, 9, 10, 0, 0, time.UTC)
	scope := memory.Scope{Tenant: "tenant-a", Project: "project-a", Namespace: "namespace-a"}

	validRetry := ApplyGovernanceRecoveryInput{
		Scope:      scope,
		RawEventID: "evt_123",
		Action:     GovernanceRecoveryActionRetry,
		Actor:      "operator-a",
		Reason:     "retry now",
		AppliedAt:  now,
	}
	if err := validRetry.Validate(); err != nil {
		t.Fatalf("retry Validate() error = %v", err)
	}

	validReschedule := ApplyGovernanceRecoveryInput{
		Scope:       scope,
		RawEventID:  "evt_123",
		Action:      GovernanceRecoveryActionReschedule,
		Actor:       "operator-a",
		Reason:      "wait for downstream fix",
		AppliedAt:   now,
		ScheduledFor: now.Add(15 * time.Minute),
	}
	if err := validReschedule.Validate(); err != nil {
		t.Fatalf("reschedule Validate() error = %v", err)
	}

	if err := (ApplyGovernanceRecoveryInput{
		Scope:      scope,
		RawEventID: "evt_123",
		Action:     GovernanceRecoveryActionRequeue,
		Actor:      "operator-a",
		Reason:     "requeue exhausted item",
		AppliedAt:  now,
	}).Validate(); err != nil {
		t.Fatalf("requeue Validate() error = %v", err)
	}

	invalidCases := []ApplyGovernanceRecoveryInput{
		{
			Scope:      scope,
			Action:     GovernanceRecoveryActionRetry,
			Actor:      "operator-a",
			Reason:     "retry now",
			AppliedAt:  now,
		},
		{
			Scope:      scope,
			RawEventID: "evt_123",
			Actor:      "operator-a",
			Reason:     "retry now",
			AppliedAt:  now,
		},
		{
			Scope:      scope,
			RawEventID: "evt_123",
			Action:     GovernanceRecoveryActionRetry,
			Reason:     "retry now",
			AppliedAt:  now,
		},
		{
			Scope:      scope,
			RawEventID: "evt_123",
			Action:     GovernanceRecoveryActionRetry,
			Actor:      "operator-a",
			AppliedAt:  now,
		},
		{
			Scope:      scope,
			RawEventID: "evt_123",
			Action:     GovernanceRecoveryActionRetry,
			Actor:      "operator-a",
			Reason:     "retry now",
		},
		{
			Scope:       scope,
			RawEventID:  "evt_123",
			Action:      GovernanceRecoveryActionReschedule,
			Actor:       "operator-a",
			Reason:      "wait",
			AppliedAt:   now,
		},
		{
			Scope:       scope,
			RawEventID:  "evt_123",
			Action:      GovernanceRecoveryActionRetry,
			Actor:       "operator-a",
			Reason:      "retry now",
			AppliedAt:   now,
			ScheduledFor: now.Add(time.Minute),
		},
	}

	for _, input := range invalidCases {
		if err := input.Validate(); err == nil {
			t.Fatalf("Validate() error = nil for invalid input %+v", input)
		}
	}

	rejected := ApplyGovernanceRecoveryInput{
		Scope:       scope,
		RawEventID:  "evt_123",
		Action:      GovernanceRecoveryActionReschedule,
		Actor:       "operator-a",
		Reason:      "broken clock",
		AppliedAt:   now,
		ScheduledFor: now,
	}
	err := rejected.Validate()
	if !errors.Is(err, ErrGovernanceRecoveryRejected) {
		t.Fatalf("Validate() error = %v, want ErrGovernanceRecoveryRejected", err)
	}
}

func TestApplyGovernanceRecovery(t *testing.T) {
	now := time.Date(2026, 6, 11, 9, 15, 0, 0, time.UTC)

	retryCurrent := RawEventGovernanceSnapshot{
		Attempt:       2,
		LastFailedAt:  now.Add(-5 * time.Minute),
		LastError:     "extractor timeout",
		NextAttemptAt: now.Add(10 * time.Minute),
	}
	retried, err := ApplyGovernanceRecovery(retryCurrent, ApplyGovernanceRecoveryInput{
		Scope:      memory.Scope{Tenant: "tenant-a", Project: "project-a", Namespace: "namespace-a"},
		RawEventID: "evt_retry",
		Action:     GovernanceRecoveryActionRetry,
		Actor:      "operator-a",
		Reason:     "retry immediately",
		AppliedAt:  now,
	})
	if err != nil {
		t.Fatalf("ApplyGovernanceRecovery(retry) error = %v", err)
	}
	if !retried.NextAttemptAt.Equal(now) {
		t.Fatalf("NextAttemptAt = %v, want %v", retried.NextAttemptAt, now)
	}
	if retried.Attempt != retryCurrent.Attempt {
		t.Fatalf("Attempt = %d, want %d", retried.Attempt, retryCurrent.Attempt)
	}
	if retried.LastError != retryCurrent.LastError {
		t.Fatalf("LastError = %q, want %q", retried.LastError, retryCurrent.LastError)
	}

	rescheduledAt := now.Add(30 * time.Minute)
	rescheduled, err := ApplyGovernanceRecovery(RawEventGovernanceSnapshot{Attempt: 1}, ApplyGovernanceRecoveryInput{
		Scope:       memory.Scope{Tenant: "tenant-a", Project: "project-a", Namespace: "namespace-a"},
		RawEventID:  "evt_reschedule",
		Action:      GovernanceRecoveryActionReschedule,
		Actor:       "operator-a",
		Reason:      "wait for dependency",
		AppliedAt:   now,
		ScheduledFor: rescheduledAt,
	})
	if err != nil {
		t.Fatalf("ApplyGovernanceRecovery(reschedule) error = %v", err)
	}
	if !rescheduled.NextAttemptAt.Equal(rescheduledAt) {
		t.Fatalf("NextAttemptAt = %v, want %v", rescheduled.NextAttemptAt, rescheduledAt)
	}
	if got := rescheduled.DerivedState(now); got != GovernanceRawEventStateRetryWait {
		t.Fatalf("DerivedState() = %q, want %q", got, GovernanceRawEventStateRetryWait)
	}

	requeued, err := ApplyGovernanceRecovery(RawEventGovernanceSnapshot{
		Attempt:       5,
		WorkerID:      "worker-a",
		ClaimedAt:     now.Add(-10 * time.Minute),
		LeaseUntil:    now.Add(-9 * time.Minute),
		LastFailedAt:  now.Add(-10 * time.Minute),
		LastError:     "poison event",
		NextAttemptAt: now.Add(-8 * time.Minute),
		ExhaustedAt:   now.Add(-7 * time.Minute),
	}, ApplyGovernanceRecoveryInput{
		Scope:      memory.Scope{Tenant: "tenant-a", Project: "project-a", Namespace: "namespace-a"},
		RawEventID: "evt_requeue",
		Action:     GovernanceRecoveryActionRequeue,
		Actor:      "operator-a",
		Reason:     "fix deployed",
		AppliedAt:  now,
	})
	if err != nil {
		t.Fatalf("ApplyGovernanceRecovery(requeue) error = %v", err)
	}
	if requeued.Attempt != 0 {
		t.Fatalf("Attempt = %d, want 0", requeued.Attempt)
	}
	if !requeued.ExhaustedAt.IsZero() {
		t.Fatalf("ExhaustedAt = %v, want zero", requeued.ExhaustedAt)
	}
	if !requeued.NextAttemptAt.Equal(now) {
		t.Fatalf("NextAttemptAt = %v, want %v", requeued.NextAttemptAt, now)
	}
	if requeued.WorkerID != "" || !requeued.ClaimedAt.IsZero() || !requeued.LeaseUntil.IsZero() {
		t.Fatalf("claim fields were not cleared: %+v", requeued)
	}

	_, err = ApplyGovernanceRecovery(RawEventGovernanceSnapshot{
		WorkerID:   "worker-a",
		LeaseUntil: now.Add(time.Minute),
	}, ApplyGovernanceRecoveryInput{
		Scope:      memory.Scope{Tenant: "tenant-a", Project: "project-a", Namespace: "namespace-a"},
		RawEventID: "evt_leased",
		Action:     GovernanceRecoveryActionRetry,
		Actor:      "operator-a",
		Reason:     "override",
		AppliedAt:  now,
	})
	if !errors.Is(err, ErrGovernanceRecoveryConflict) {
		t.Fatalf("leased ApplyGovernanceRecovery() error = %v, want ErrGovernanceRecoveryConflict", err)
	}

	_, err = ApplyGovernanceRecovery(RawEventGovernanceSnapshot{
		ProcessedAt: now.Add(-time.Minute),
	}, ApplyGovernanceRecoveryInput{
		Scope:      memory.Scope{Tenant: "tenant-a", Project: "project-a", Namespace: "namespace-a"},
		RawEventID: "evt_processed",
		Action:     GovernanceRecoveryActionRequeue,
		Actor:      "operator-a",
		Reason:     "should fail",
		AppliedAt:  now,
	})
	if !errors.Is(err, ErrGovernanceRecoveryConflict) {
		t.Fatalf("processed ApplyGovernanceRecovery() error = %v, want ErrGovernanceRecoveryConflict", err)
	}
}
