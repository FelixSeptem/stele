package policy

import (
	"fmt"
	"strings"
)

type RetentionClass string

const (
	RetentionClassEphemeral RetentionClass = "ephemeral"
	RetentionClassSession   RetentionClass = "session"
	RetentionClassDurable   RetentionClass = "durable"
	RetentionClassPermanent RetentionClass = "permanent"
)

func (r RetentionClass) Validate() error {
	switch RetentionClass(strings.TrimSpace(string(r))) {
	case RetentionClassEphemeral, RetentionClassSession, RetentionClassDurable, RetentionClassPermanent:
		return nil
	default:
		return fmt.Errorf("invalid retention class %q", r)
	}
}

type ForgettingAction string

const (
	ForgettingActionSuppress ForgettingAction = "suppress"
	ForgettingActionExpire   ForgettingAction = "expire"
	ForgettingActionDelete   ForgettingAction = "delete"
)

func (a ForgettingAction) Validate() error {
	switch ForgettingAction(strings.TrimSpace(string(a))) {
	case ForgettingActionSuppress, ForgettingActionExpire, ForgettingActionDelete:
		return nil
	default:
		return fmt.Errorf("invalid forgetting action %q", a)
	}
}
