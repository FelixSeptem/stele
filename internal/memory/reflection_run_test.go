package memory

import (
	"context"
	"testing"
	"time"
)

type reflectionStoreCapture struct{ input CreateReflectionRunInput }

func (s *reflectionStoreCapture) CreateReflectionRun(ctx context.Context, in CreateReflectionRunInput) (ReflectionRun, error) {
	s.input = in
	return ReflectionRun{ID: "run-1", Scope: in.Scope, Trigger: in.Trigger, InputWatermark: in.InputWatermark, TranscriptSchemaVersion: in.TranscriptSchemaVersion, IdempotencyKey: in.IdempotencyKey}, nil
}
func (s *reflectionStoreCapture) ClaimReflectionRuns(context.Context, ClaimReflectionRunsInput) ([]ReflectionRun, error) {
	return nil, nil
}
func (s *reflectionStoreCapture) CheckpointReflectionRun(context.Context, CheckpointReflectionRunInput) error {
	return nil
}
func (s *reflectionStoreCapture) CompleteReflectionRun(context.Context, CompleteReflectionRunInput) error {
	return nil
}
func (s *reflectionStoreCapture) FailReflectionRun(context.Context, FailReflectionRunInput) error {
	return nil
}
func (s *reflectionStoreCapture) ReplayReflectionRun(context.Context, ReplayReflectionRunInput) (ReflectionRun, error) {
	return ReflectionRun{}, nil
}

func TestReflectionRunInputValidateRequiresDurableIdentityAndWatermark(t *testing.T) {
	now := time.Date(2026, 9, 6, 0, 0, 0, 0, time.UTC)
	valid := CreateReflectionRunInput{Scope: Scope{Tenant: "t", Project: "p", Namespace: "n"}, Trigger: ReflectionTriggerSessionCompletion, InputWatermark: "42", TranscriptSchemaVersion: "v1", IdempotencyKey: "idem", MaxAttempts: 3, CreatedAt: now}
	if err := valid.Validate(); err != nil {
		t.Fatalf("valid input rejected: %v", err)
	}
	for name, in := range map[string]CreateReflectionRunInput{
		"scope":       valid,
		"trigger":     valid,
		"watermark":   valid,
		"schema":      valid,
		"idempotency": valid,
	} {
		in2 := in
		switch name {
		case "scope":
			in2.Scope = Scope{}
		case "trigger":
			in2.Trigger = "bad"
		case "watermark":
			in2.InputWatermark = ""
		case "schema":
			in2.TranscriptSchemaVersion = ""
		case "idempotency":
			in2.IdempotencyKey = ""
		}
		if err := in2.Validate(); err == nil {
			t.Errorf("%s input accepted", name)
		}
	}
}

func TestReflectionRunFailureCategoryIsBounded(t *testing.T) {
	for _, category := range []ReflectionFailureCategory{ReflectionFailureTransient, ReflectionFailureInvalidInput, ReflectionFailureScope, ReflectionFailurePolicy, ReflectionFailureUnknown} {
		if !category.Valid() {
			t.Fatalf("category %q should be valid", category)
		}
	}
	if ReflectionFailureCategory("arbitrary").Valid() {
		t.Fatal("arbitrary failure category accepted")
	}
}

func TestReflectionTriggerServicePreservesTriggerAndScope(t *testing.T) {
	store := &reflectionStoreCapture{}
	svc := ReflectionTriggerService{Store: store, Now: func() time.Time { return time.Date(2026, 9, 6, 0, 0, 0, 0, time.UTC) }}
	scope := Scope{Tenant: "t", Project: "p", Namespace: "n"}
	if _, err := svc.CompactionPressure(context.Background(), scope, "42", "v1", "compaction-42"); err != nil {
		t.Fatalf("trigger failed: %v", err)
	}
	if store.input.Trigger != ReflectionTriggerCompaction || store.input.Scope != scope || store.input.InputWatermark != "42" {
		t.Fatalf("captured input=%+v", store.input)
	}
}
