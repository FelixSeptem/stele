package policy

import (
	"fmt"
	"strings"
	"time"
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

type RetentionPolicy struct {
	EphemeralTTL time.Duration
	SessionTTL   time.Duration
	DurableTTL   time.Duration
}

func DefaultRetentionPolicy() RetentionPolicy {
	return RetentionPolicy{
		EphemeralTTL: time.Hour,
		SessionTTL:   24 * time.Hour,
		DurableTTL:   30 * 24 * time.Hour,
	}
}

func (p RetentionPolicy) Expired(class RetentionClass, updatedAt, now time.Time) (bool, error) {
	if err := class.Validate(); err != nil {
		return false, err
	}

	if updatedAt.IsZero() {
		return false, fmt.Errorf("updated at is required")
	}

	if now.IsZero() {
		return false, fmt.Errorf("now is required")
	}

	switch class {
	case RetentionClassPermanent:
		return false, nil
	case RetentionClassEphemeral:
		return !updatedAt.Add(p.EphemeralTTL).After(now), nil
	case RetentionClassSession:
		return !updatedAt.Add(p.SessionTTL).After(now), nil
	case RetentionClassDurable:
		return !updatedAt.Add(p.DurableTTL).After(now), nil
	default:
		return false, fmt.Errorf("unsupported retention class %q", class)
	}
}
