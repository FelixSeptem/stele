package provider

import (
	"context"
	"errors"
	"time"
)

var ErrContextTurnUnavailable = errors.New("context_turn_unavailable")

// ContextTurnAuthorizer proves a turn was created by the full current runtime
// binding. It must return ErrContextTurnUnavailable for missing and foreign
// turns alike.
type ContextTurnAuthorizer interface {
	AuthorizeContextTurn(context.Context, RuntimeBinding, string) error
}

// ContextTurnAttribution contains only bounded metadata that may be persisted
// for an admitted turn-bound context response.
type ContextTurnAttribution struct {
	TurnID, RequestID, OperationID string
	Digest                         ContextDigest
	ContinuityReference            string
	OccurredAt                     time.Time
}

type ContextTurnAttributionStore interface {
	RecordContextTurnAttribution(context.Context, RuntimeBinding, ContextTurnAttribution) error
}
