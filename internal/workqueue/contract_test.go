package workqueue

import (
	"strings"
	"testing"
	"time"

	"github.com/FelixSeptem/stele/internal/memory"
)

func TestDerivedWorkInputValidateRejectsUnsupportedAndUnboundedReferences(t *testing.T) {
	base := DerivedWorkInput{
		Scope:       memory.Scope{Tenant: "tenant-a", Project: "project-a", Namespace: "namespace-a"},
		Kind:        WorkKindReflection,
		Watermark:   "wm-1",
		Idempotency: "trigger-1",
		Reference:   "reflection-run-1",
	}
	if err := base.Validate(); err != nil {
		t.Fatalf("valid work input rejected: %v", err)
	}

	for _, tc := range []struct {
		name   string
		mutate func(*DerivedWorkInput)
		want   string
	}{
		{name: "unsupported kind", mutate: func(v *DerivedWorkInput) { v.Kind = WorkKind("raw_event") }, want: "kind"},
		{name: "payload-like reference", mutate: func(v *DerivedWorkInput) { v.Reference = strings.Repeat("x", MaxReferenceBytes+1) }, want: "reference"},
		{name: "missing watermark", mutate: func(v *DerivedWorkInput) { v.Watermark = "" }, want: "watermark"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			input := base
			tc.mutate(&input)
			if err := input.Validate(); err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("Validate() error = %v, want contains %q", err, tc.want)
			}
		})
	}
}

func TestDerivedWorkIdentityIsDeterministicAndScoped(t *testing.T) {
	scope := memory.Scope{Tenant: "tenant-a", Project: "project-a", Namespace: "namespace-a"}
	first, err := NewDerivedWorkIdentity(DerivedWorkInput{Scope: scope, Kind: WorkKindReflection, Watermark: "wm-1", Idempotency: "trigger-1", Reference: "run-1"})
	if err != nil {
		t.Fatal(err)
	}
	second, err := NewDerivedWorkIdentity(DerivedWorkInput{Scope: scope, Kind: WorkKindReflection, Watermark: "wm-1", Idempotency: "trigger-1", Reference: "run-1"})
	if err != nil {
		t.Fatal(err)
	}
	if first != second || first.WorkKey == "" {
		t.Fatalf("identities differ or are empty: first=%+v second=%+v", first, second)
	}
	foreign, err := NewDerivedWorkIdentity(DerivedWorkInput{Scope: memory.Scope{Tenant: "tenant-b", Project: scope.Project, Namespace: scope.Namespace}, Kind: WorkKindReflection, Watermark: "wm-1", Idempotency: "trigger-1", Reference: "run-1"})
	if err != nil {
		t.Fatal(err)
	}
	if foreign.WorkKey == first.WorkKey {
		t.Fatal("foreign scope unexpectedly reused work key")
	}
}

func TestQueueConfigValidateBoundsModesAndLoss(t *testing.T) {
	if err := (QueueConfig{Mode: QueueModePostgresDurable, Capacity: 100, BatchSize: 10, FlushInterval: 1, LeaseDuration: 5, MaxAttempts: 3, Retention: 1}).Validate(); err != nil {
		t.Fatalf("valid durable config rejected: %v", err)
	}
	for _, cfg := range []QueueConfig{
		{Mode: QueueMode("redis"), Capacity: 1, BatchSize: 1, FlushInterval: 1, LeaseDuration: 1, MaxAttempts: 1, Retention: 1},
		{Mode: QueueModeMemoryBuffer, Capacity: 0, BatchSize: 1, FlushInterval: 1, LeaseDuration: 1, MaxAttempts: 1, Retention: 1},
		{Mode: QueueModeMemoryBuffer, Capacity: 1, BatchSize: 2, FlushInterval: 1, LeaseDuration: 1, MaxAttempts: 1, Retention: 1},
	} {
		if err := cfg.Validate(); err == nil {
			t.Fatalf("Validate() = nil for unsafe config %+v", cfg)
		}
	}
}

func TestCursorIsOpaqueAndRoundTrips(t *testing.T) {
	want := Cursor{CreatedAt: time.Date(2026, 10, 2, 12, 0, 0, 0, time.UTC), ID: "work-1"}
	encoded, err := want.Encode()
	if err != nil {
		t.Fatal(err)
	}
	got, err := DecodeCursor(encoded)
	if err != nil || !got.CreatedAt.Equal(want.CreatedAt) || got.ID != want.ID {
		t.Fatalf("cursor = %+v, err=%v", got, err)
	}
	if _, err := DecodeCursor("not-a-cursor"); err == nil {
		t.Fatal("invalid cursor accepted")
	}
}
