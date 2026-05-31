package memory

import (
	"testing"
	"time"
)

func TestIngestEventInputValidateRequiresTypeAndContent(t *testing.T) {
	input := IngestEventInput{}

	if err := input.Validate(); err == nil {
		t.Fatal("Validate() error = nil, want validation error")
	}
}

func TestIngestEventInputValidateAcceptsValidInput(t *testing.T) {
	input := IngestEventInput{
		Scope: Scope{
			Tenant:    "tenant-a",
			Project:   "project-a",
			Namespace: "namespace-a",
		},
		EventType:       "conversation.message",
		Content:         "hello",
		SourceTimestamp: time.Date(2026, 5, 29, 23, 0, 0, 0, time.UTC),
	}

	if err := input.Validate(); err != nil {
		t.Fatalf("Validate() error = %v", err)
	}
}
