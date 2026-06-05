package policy

import (
	"testing"
	"time"
)

func TestRetentionClassValidate(t *testing.T) {
	valid := []RetentionClass{
		RetentionClassEphemeral,
		RetentionClassSession,
		RetentionClassDurable,
		RetentionClassPermanent,
	}

	for _, class := range valid {
		if err := class.Validate(); err != nil {
			t.Fatalf("Validate() error for %q = %v", class, err)
		}
	}

	if err := RetentionClass("mystery").Validate(); err == nil {
		t.Fatal("Validate() error = nil for invalid retention class")
	}
}

func TestRetentionPolicyExpired(t *testing.T) {
	policy := DefaultRetentionPolicy()
	now := mustTime(t, "2026-06-05T10:00:00Z")

	expired, err := policy.Expired(RetentionClassEphemeral, now.Add(-2*time.Hour), now)
	if err != nil {
		t.Fatalf("Expired() error = %v", err)
	}
	if !expired {
		t.Fatal("Expired() = false, want true for expired ephemeral memory")
	}

	expired, err = policy.Expired(RetentionClassPermanent, now.Add(-365*24*time.Hour), now)
	if err != nil {
		t.Fatalf("Expired() error = %v", err)
	}
	if expired {
		t.Fatal("Expired() = true, want false for permanent memory")
	}
}

func mustTime(t *testing.T, value string) time.Time {
	t.Helper()

	parsed, err := time.Parse(time.RFC3339, value)
	if err != nil {
		t.Fatalf("time.Parse() error = %v", err)
	}

	return parsed
}

func TestForgettingActionValidate(t *testing.T) {
	valid := []ForgettingAction{
		ForgettingActionSuppress,
		ForgettingActionExpire,
		ForgettingActionDelete,
	}

	for _, action := range valid {
		if err := action.Validate(); err != nil {
			t.Fatalf("Validate() error for %q = %v", action, err)
		}
	}

	if err := ForgettingAction("archive").Validate(); err == nil {
		t.Fatal("Validate() error = nil for invalid forgetting action")
	}
}
