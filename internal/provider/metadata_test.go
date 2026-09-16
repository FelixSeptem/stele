package provider

import (
	"strings"
	"testing"
)

func TestOperationMetadataNormalizeAndValidate(t *testing.T) {
	metadata, err := (OperationMetadata{
		RequestID:      "  req-1  ",
		OperationID:    " op_1 ",
		IdempotencyKey: " retry:1 ",
		EventSeq:       7,
	}).Normalize()
	if err != nil {
		t.Fatalf("Normalize() error = %v", err)
	}
	if metadata.RequestID != "req-1" || metadata.OperationID != "op_1" || metadata.IdempotencyKey != "retry:1" {
		t.Fatalf("Normalize() = %+v", metadata)
	}
	if metadata.SchemaVersion != SchemaVersionV1 {
		t.Fatalf("SchemaVersion = %q, want %q", metadata.SchemaVersion, SchemaVersionV1)
	}
}

func TestOperationMetadataRejectsMissingInvalidAndOversizedFields(t *testing.T) {
	tests := []OperationMetadata{
		{OperationID: "op-1", SchemaVersion: SchemaVersionV1},
		{RequestID: "req 1", OperationID: "op-1", SchemaVersion: SchemaVersionV1},
		{RequestID: "req-1", OperationID: "op/1", SchemaVersion: SchemaVersionV1},
		{RequestID: "req-1", OperationID: "op-1", IdempotencyKey: "bad\nkey", SchemaVersion: SchemaVersionV1},
		{RequestID: strings.Repeat("x", MaxMetadataFieldBytes+1), OperationID: "op-1", SchemaVersion: SchemaVersionV1},
		{RequestID: "req-1", OperationID: "op-1", EventSeq: -1, SchemaVersion: SchemaVersionV1},
	}
	for _, test := range tests {
		if _, err := test.Normalize(); err == nil {
			t.Fatalf("Normalize(%+v) error = nil", test)
		}
	}
}

func TestSequenceTrackerReturnsStableDispositions(t *testing.T) {
	tracker := NewSequenceTracker()
	metadata := OperationMetadata{RequestID: "req-1", OperationID: "op-1", IdempotencyKey: "idem-1", EventSeq: 5, SchemaVersion: SchemaVersionV1}

	if got := tracker.CheckAndRecord("session-a", metadata); got != SequenceAccepted {
		t.Fatalf("first disposition = %q", got)
	}
	if got := tracker.CheckAndRecord("session-a", metadata); got != SequenceDuplicate {
		t.Fatalf("duplicate disposition = %q", got)
	}
	stale := metadata
	stale.EventSeq = 4
	if got := tracker.CheckAndRecord("session-a", stale); got != SequenceStale {
		t.Fatalf("stale disposition = %q", got)
	}
	changed := metadata
	changed.OperationID = "op-2"
	if got := tracker.CheckAndRecord("session-a", changed); got != SequenceStale {
		t.Fatalf("conflicting duplicate disposition = %q", got)
	}
	if got := tracker.CheckAndRecord("session-b", stale); got != SequenceAccepted {
		t.Fatalf("independent session disposition = %q", got)
	}
}
