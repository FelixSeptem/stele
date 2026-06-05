package policy

import "testing"

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
