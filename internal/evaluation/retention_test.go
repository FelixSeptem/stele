package evaluation

import (
	"testing"
	"time"
)

func TestRetentionOutcomeIsBoundedAndDeterministic(t *testing.T) {
	first, err := BuildRetentionOutcome(RetentionInput{ArtifactCategory: "trajectory", Result: "deleted", DeletedCount: 4, ReasonCategory: "expired", At: time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC)})
	if err != nil {
		t.Fatalf("build retention outcome: %v", err)
	}
	second, err := BuildRetentionOutcome(RetentionInput{ArtifactCategory: "trajectory", Result: "deleted", DeletedCount: 4, ReasonCategory: "expired", At: first.CreatedAt})
	if err != nil {
		t.Fatalf("build retention outcome repeat: %v", err)
	}
	if first != second {
		t.Fatalf("retention outcome is not deterministic: %#v %#v", first, second)
	}
	if _, err := BuildRetentionOutcome(RetentionInput{ArtifactCategory: "raw_events", Result: "deleted", DeletedCount: 1}); err == nil {
		t.Fatal("expected canonical artifact deletion to be rejected")
	}
	if _, err := BuildRetentionOutcome(RetentionInput{ArtifactCategory: "trajectory", Result: "deleted", DeletedCount: 2147483648}); err == nil {
		t.Fatal("expected deleted count above PostgreSQL integer range to be rejected")
	}
}
