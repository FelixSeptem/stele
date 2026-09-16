package provider

import (
	"context"
	"fmt"
	"regexp"
	"strings"
	"sync"
)

var metadataIdentifierPattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._:-]*$`)

type SequenceDisposition string

const (
	SequenceAccepted  SequenceDisposition = "accepted"
	SequenceDuplicate SequenceDisposition = "duplicate"
	SequenceStale     SequenceDisposition = "stale"
)

type sequenceState struct {
	sequence    int64
	operationID string
	requestID   string
}

type SequenceTracker struct {
	mu       sync.Mutex
	sessions map[string]sequenceState
}

func NewSequenceTracker() *SequenceTracker {
	return &SequenceTracker{sessions: make(map[string]sequenceState)}
}

func (m OperationMetadata) Normalize() (OperationMetadata, error) {
	m.RequestID = strings.TrimSpace(m.RequestID)
	m.OperationID = strings.TrimSpace(m.OperationID)
	m.IdempotencyKey = strings.TrimSpace(m.IdempotencyKey)
	m.SchemaVersion = strings.TrimSpace(m.SchemaVersion)
	if m.SchemaVersion == "" {
		m.SchemaVersion = SchemaVersionV1
	}
	if m.RequestID == "" {
		return OperationMetadata{}, fmt.Errorf("request_id is required")
	}
	if m.OperationID == "" {
		return OperationMetadata{}, fmt.Errorf("operation_id is required")
	}
	if err := m.Validate(); err != nil {
		return OperationMetadata{}, err
	}
	for field, value := range map[string]string{
		"request_id":      m.RequestID,
		"operation_id":    m.OperationID,
		"idempotency_key": m.IdempotencyKey,
	} {
		if value != "" && !metadataIdentifierPattern.MatchString(value) {
			return OperationMetadata{}, fmt.Errorf("%s contains invalid characters", field)
		}
	}
	return m, nil
}

func (m OperationMetadata) Map() map[string]any {
	values := map[string]any{
		"request_id":     m.RequestID,
		"operation_id":   m.OperationID,
		"event_seq":      m.EventSeq,
		"schema_version": m.SchemaVersion,
	}
	if m.IdempotencyKey != "" {
		values["idempotency_key"] = m.IdempotencyKey
	}
	return values
}

func (t *SequenceTracker) CheckAndRecord(sessionID string, metadata OperationMetadata) SequenceDisposition {
	if t == nil || metadata.EventSeq == 0 {
		return SequenceAccepted
	}
	sessionID = strings.TrimSpace(sessionID)
	t.mu.Lock()
	defer t.mu.Unlock()
	state, ok := t.sessions[sessionID]
	if !ok || metadata.EventSeq > state.sequence {
		t.sessions[sessionID] = sequenceState{sequence: metadata.EventSeq, operationID: metadata.OperationID, requestID: metadata.RequestID}
		return SequenceAccepted
	}
	if metadata.EventSeq == state.sequence && metadata.OperationID == state.operationID && metadata.RequestID == state.requestID {
		return SequenceDuplicate
	}
	return SequenceStale
}

type operationMetadataContextKey struct{}

func withOperationMetadata(ctx context.Context, metadata OperationMetadata) context.Context {
	return context.WithValue(ctx, operationMetadataContextKey{}, metadata)
}

func OperationMetadataFromContext(ctx context.Context) (OperationMetadata, bool) {
	metadata, ok := ctx.Value(operationMetadataContextKey{}).(OperationMetadata)
	return metadata, ok
}
