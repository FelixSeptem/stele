package memory

import (
	"errors"
	"testing"
	"time"
)

func TestCreateEmbeddingCutoverPlanInputValidate(t *testing.T) {
	scope := Scope{Tenant: "tenant-a", Project: "project-a", Namespace: "namespace-a"}
	valid := CreateEmbeddingCutoverPlanInput{
		Scope: scope,
		Target: EmbeddingCutoverTarget{
			Provider:   "openai",
			Model:      "text-embedding-3-small",
			Dimensions: 1536,
		},
		Classes:   []MemoryClass{MemoryClassProfile, MemoryClassEpisodic},
		WaveSize:  25,
		Actor:     "operator-a",
		Reason:    "migrate to current provider target",
		CreatedAt: time.Date(2026, 6, 28, 13, 0, 0, 0, time.UTC),
	}
	if err := valid.Validate(); err != nil {
		t.Fatalf("Validate() error = %v, want nil", err)
	}

	tests := []struct {
		name  string
		input CreateEmbeddingCutoverPlanInput
		want  string
	}{
		{
			name: "missing provider",
			input: CreateEmbeddingCutoverPlanInput{
				Scope: scope,
				Target: EmbeddingCutoverTarget{
					Model:      "text-embedding-3-small",
					Dimensions: 1536,
				},
				WaveSize:  25,
				Actor:     "operator-a",
				Reason:    "migrate",
				CreatedAt: valid.CreatedAt,
			},
			want: "provider is required",
		},
		{
			name: "missing model",
			input: CreateEmbeddingCutoverPlanInput{
				Scope: scope,
				Target: EmbeddingCutoverTarget{
					Provider:   "openai",
					Dimensions: 1536,
				},
				WaveSize:  25,
				Actor:     "operator-a",
				Reason:    "migrate",
				CreatedAt: valid.CreatedAt,
			},
			want: "model is required",
		},
		{
			name: "missing dimensions",
			input: CreateEmbeddingCutoverPlanInput{
				Scope: scope,
				Target: EmbeddingCutoverTarget{
					Provider: "openai",
					Model:    "text-embedding-3-small",
				},
				WaveSize:  25,
				Actor:     "operator-a",
				Reason:    "migrate",
				CreatedAt: valid.CreatedAt,
			},
			want: "dimensions must be greater than zero",
		},
		{
			name: "invalid class filter",
			input: CreateEmbeddingCutoverPlanInput{
				Scope:  scope,
				Target: valid.Target,
				Classes: []MemoryClass{
					MemoryClassProfile,
					MemoryClass("invalid"),
				},
				WaveSize:  25,
				Actor:     "operator-a",
				Reason:    "migrate",
				CreatedAt: valid.CreatedAt,
			},
			want: "memory class \"invalid\" is invalid",
		},
		{
			name: "missing wave size",
			input: CreateEmbeddingCutoverPlanInput{
				Scope:     scope,
				Target:    valid.Target,
				Actor:     "operator-a",
				Reason:    "migrate",
				CreatedAt: valid.CreatedAt,
			},
			want: "wave size must be greater than zero",
		},
		{
			name: "missing actor",
			input: CreateEmbeddingCutoverPlanInput{
				Scope:     scope,
				Target:    valid.Target,
				WaveSize:  25,
				Reason:    "migrate",
				CreatedAt: valid.CreatedAt,
			},
			want: "actor is required",
		},
		{
			name: "missing reason",
			input: CreateEmbeddingCutoverPlanInput{
				Scope:     scope,
				Target:    valid.Target,
				WaveSize:  25,
				Actor:     "operator-a",
				CreatedAt: valid.CreatedAt,
			},
			want: "reason is required",
		},
		{
			name: "missing created at",
			input: CreateEmbeddingCutoverPlanInput{
				Scope:    scope,
				Target:   valid.Target,
				WaveSize: 25,
				Actor:    "operator-a",
				Reason:   "migrate",
			},
			want: "created at is required",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.input.Validate()
			if err == nil || err.Error() != tc.want {
				t.Fatalf("Validate() error = %v, want %q", err, tc.want)
			}
		})
	}
}

func TestListAndReadEmbeddingCutoverInputsValidate(t *testing.T) {
	scope := Scope{Tenant: "tenant-a", Project: "project-a", Namespace: "namespace-a"}

	if err := (ListEmbeddingCutoverPlansInput{
		Scope: scope,
		Limit: 10,
	}).Validate(); err != nil {
		t.Fatalf("ListEmbeddingCutoverPlansInput.Validate() error = %v, want nil", err)
	}

	if err := (ReadEmbeddingCutoverPlanInput{
		Scope:  scope,
		PlanID: "plan_123",
	}).Validate(); err != nil {
		t.Fatalf("ReadEmbeddingCutoverPlanInput.Validate() error = %v, want nil", err)
	}

	if err := (ListEmbeddingCutoverPlansInput{
		Scope: scope,
	}).Validate(); err == nil || err.Error() != "limit must be greater than zero" {
		t.Fatalf("ListEmbeddingCutoverPlansInput.Validate() error = %v, want limit validation", err)
	}

	if err := (ReadEmbeddingCutoverPlanInput{
		Scope: scope,
	}).Validate(); err == nil || err.Error() != "cutover plan id is required" {
		t.Fatalf("ReadEmbeddingCutoverPlanInput.Validate() error = %v, want plan id validation", err)
	}
}

func TestApplyEmbeddingCutoverPlanActionTransitions(t *testing.T) {
	scope := Scope{Tenant: "tenant-a", Project: "project-a", Namespace: "namespace-a"}
	appliedAt := time.Date(2026, 6, 28, 14, 0, 0, 0, time.UTC)

	tests := []struct {
		name         string
		current      EmbeddingCutoverPlan
		input        ApplyEmbeddingCutoverPlanActionInput
		wantStatus   EmbeddingCutoverPlanStatus
		wantTime     time.Time
		wantLastBy   string
		wantLastNote string
	}{
		{
			name: "activate draft",
			current: EmbeddingCutoverPlan{
				ID:        "plan_123",
				Scope:     scope,
				Status:    EmbeddingCutoverPlanStatusDraft,
				Target:    EmbeddingCutoverTarget{Provider: "openai", Model: "text-embedding-3-small", Dimensions: 1536},
				WaveSize:  25,
				CreatedBy: "operator-a",
				CreatedAt: appliedAt.Add(-time.Hour),
			},
			input: ApplyEmbeddingCutoverPlanActionInput{
				Scope:     scope,
				PlanID:    "plan_123",
				Action:    EmbeddingCutoverPlanActionActivate,
				Actor:     "operator-b",
				Reason:    "roll out now",
				AppliedAt: appliedAt,
			},
			wantStatus:   EmbeddingCutoverPlanStatusActive,
			wantTime:     appliedAt,
			wantLastBy:   "operator-b",
			wantLastNote: "roll out now",
		},
		{
			name: "reactivate paused",
			current: EmbeddingCutoverPlan{
				ID:          "plan_123",
				Scope:       scope,
				Status:      EmbeddingCutoverPlanStatusPaused,
				Target:      EmbeddingCutoverTarget{Provider: "openai", Model: "text-embedding-3-small", Dimensions: 1536},
				WaveSize:    25,
				CreatedBy:   "operator-a",
				CreatedAt:   appliedAt.Add(-2 * time.Hour),
				ActivatedAt: appliedAt.Add(-90 * time.Minute),
				PausedAt:    appliedAt.Add(-30 * time.Minute),
			},
			input: ApplyEmbeddingCutoverPlanActionInput{
				Scope:     scope,
				PlanID:    "plan_123",
				Action:    EmbeddingCutoverPlanActionActivate,
				Actor:     "operator-c",
				Reason:    "resume rollout",
				AppliedAt: appliedAt,
			},
			wantStatus:   EmbeddingCutoverPlanStatusActive,
			wantTime:     appliedAt,
			wantLastBy:   "operator-c",
			wantLastNote: "resume rollout",
		},
		{
			name: "pause active",
			current: EmbeddingCutoverPlan{
				ID:          "plan_123",
				Scope:       scope,
				Status:      EmbeddingCutoverPlanStatusActive,
				Target:      EmbeddingCutoverTarget{Provider: "openai", Model: "text-embedding-3-small", Dimensions: 1536},
				WaveSize:    25,
				CreatedBy:   "operator-a",
				CreatedAt:   appliedAt.Add(-2 * time.Hour),
				ActivatedAt: appliedAt.Add(-time.Hour),
			},
			input: ApplyEmbeddingCutoverPlanActionInput{
				Scope:     scope,
				PlanID:    "plan_123",
				Action:    EmbeddingCutoverPlanActionPause,
				Actor:     "operator-b",
				Reason:    "halt next wave",
				AppliedAt: appliedAt,
			},
			wantStatus:   EmbeddingCutoverPlanStatusPaused,
			wantTime:     appliedAt,
			wantLastBy:   "operator-b",
			wantLastNote: "halt next wave",
		},
		{
			name: "cancel paused",
			current: EmbeddingCutoverPlan{
				ID:          "plan_123",
				Scope:       scope,
				Status:      EmbeddingCutoverPlanStatusPaused,
				Target:      EmbeddingCutoverTarget{Provider: "openai", Model: "text-embedding-3-small", Dimensions: 1536},
				WaveSize:    25,
				CreatedBy:   "operator-a",
				CreatedAt:   appliedAt.Add(-2 * time.Hour),
				ActivatedAt: appliedAt.Add(-time.Hour),
				PausedAt:    appliedAt.Add(-30 * time.Minute),
			},
			input: ApplyEmbeddingCutoverPlanActionInput{
				Scope:     scope,
				PlanID:    "plan_123",
				Action:    EmbeddingCutoverPlanActionCancel,
				Actor:     "operator-b",
				Reason:    "rollback with new plan",
				AppliedAt: appliedAt,
			},
			wantStatus:   EmbeddingCutoverPlanStatusCancelled,
			wantTime:     appliedAt,
			wantLastBy:   "operator-b",
			wantLastNote: "rollback with new plan",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			next, err := ApplyEmbeddingCutoverPlanAction(tc.current, tc.input)
			if err != nil {
				t.Fatalf("ApplyEmbeddingCutoverPlanAction() error = %v, want nil", err)
			}
			if next.Status != tc.wantStatus {
				t.Fatalf("Status = %q, want %q", next.Status, tc.wantStatus)
			}
			if next.LastActionBy != tc.wantLastBy {
				t.Fatalf("LastActionBy = %q, want %q", next.LastActionBy, tc.wantLastBy)
			}
			if next.LastActionReason != tc.wantLastNote {
				t.Fatalf("LastActionReason = %q, want %q", next.LastActionReason, tc.wantLastNote)
			}
			if !next.LastActionAt.Equal(tc.wantTime) {
				t.Fatalf("LastActionAt = %v, want %v", next.LastActionAt, tc.wantTime)
			}

			switch tc.wantStatus {
			case EmbeddingCutoverPlanStatusActive:
				if !next.ActivatedAt.Equal(tc.wantTime) {
					t.Fatalf("ActivatedAt = %v, want %v", next.ActivatedAt, tc.wantTime)
				}
			case EmbeddingCutoverPlanStatusPaused:
				if !next.PausedAt.Equal(tc.wantTime) {
					t.Fatalf("PausedAt = %v, want %v", next.PausedAt, tc.wantTime)
				}
			case EmbeddingCutoverPlanStatusCancelled:
				if !next.CancelledAt.Equal(tc.wantTime) {
					t.Fatalf("CancelledAt = %v, want %v", next.CancelledAt, tc.wantTime)
				}
			}
		})
	}
}

func TestApplyEmbeddingCutoverPlanActionRejectsInvalidTransitions(t *testing.T) {
	scope := Scope{Tenant: "tenant-a", Project: "project-a", Namespace: "namespace-a"}
	appliedAt := time.Date(2026, 6, 28, 14, 0, 0, 0, time.UTC)

	_, err := ApplyEmbeddingCutoverPlanAction(EmbeddingCutoverPlan{
		ID:        "plan_123",
		Scope:     scope,
		Status:    EmbeddingCutoverPlanStatusDraft,
		Target:    EmbeddingCutoverTarget{Provider: "openai", Model: "text-embedding-3-small", Dimensions: 1536},
		WaveSize:  25,
		CreatedBy: "operator-a",
		CreatedAt: appliedAt.Add(-time.Hour),
	}, ApplyEmbeddingCutoverPlanActionInput{
		Scope:     scope,
		PlanID:    "plan_123",
		Action:    EmbeddingCutoverPlanActionPause,
		Actor:     "operator-b",
		Reason:    "halt next wave",
		AppliedAt: appliedAt,
	})
	if !errors.Is(err, ErrEmbeddingCutoverConflict) {
		t.Fatalf("error = %v, want ErrEmbeddingCutoverConflict", err)
	}

	_, err = ApplyEmbeddingCutoverPlanAction(EmbeddingCutoverPlan{
		ID:          "plan_123",
		Scope:       scope,
		Status:      EmbeddingCutoverPlanStatusCompleted,
		Target:      EmbeddingCutoverTarget{Provider: "openai", Model: "text-embedding-3-small", Dimensions: 1536},
		WaveSize:    25,
		CreatedBy:   "operator-a",
		CreatedAt:   appliedAt.Add(-2 * time.Hour),
		CompletedAt: appliedAt.Add(-time.Minute),
	}, ApplyEmbeddingCutoverPlanActionInput{
		Scope:     scope,
		PlanID:    "plan_123",
		Action:    EmbeddingCutoverPlanActionCancel,
		Actor:     "operator-b",
		Reason:    "too late",
		AppliedAt: appliedAt,
	})
	if !errors.Is(err, ErrEmbeddingCutoverConflict) {
		t.Fatalf("error = %v, want ErrEmbeddingCutoverConflict", err)
	}
}

func TestListEmbeddingRecoveryHistoryInputValidate(t *testing.T) {
	scope := Scope{Tenant: "tenant-a", Project: "project-a", Namespace: "namespace-a"}
	valid := ListEmbeddingRecoveryHistoryInput{
		Scope:         scope,
		MemoryID:      "mem_123",
		Action:        EmbeddingRecoveryActionRetry,
		Actor:         "operator-a",
		CutoverPlanID: "plan_123",
		OccurredFrom:  time.Date(2026, 6, 28, 12, 0, 0, 0, time.UTC),
		OccurredTo:    time.Date(2026, 6, 28, 13, 0, 0, 0, time.UTC),
		Limit:         25,
	}
	if err := valid.Validate(); err != nil {
		t.Fatalf("Validate() error = %v, want nil", err)
	}

	tests := []struct {
		name  string
		input ListEmbeddingRecoveryHistoryInput
		want  string
	}{
		{
			name: "invalid action",
			input: ListEmbeddingRecoveryHistoryInput{
				Scope:  scope,
				Action: EmbeddingRecoveryAction("invalid"),
				Limit:  25,
			},
			want: "embedding recovery action \"invalid\" is invalid",
		},
		{
			name: "invalid window",
			input: ListEmbeddingRecoveryHistoryInput{
				Scope:        scope,
				OccurredFrom: valid.OccurredTo,
				OccurredTo:   valid.OccurredFrom,
				Limit:        25,
			},
			want: "occurred_from must be before or equal to occurred_to",
		},
		{
			name: "missing limit",
			input: ListEmbeddingRecoveryHistoryInput{
				Scope: scope,
			},
			want: "limit must be greater than zero",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.input.Validate()
			if err == nil || err.Error() != tc.want {
				t.Fatalf("Validate() error = %v, want %q", err, tc.want)
			}
		})
	}
}
