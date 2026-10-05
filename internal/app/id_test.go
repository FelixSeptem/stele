package app

import (
	"testing"

	"github.com/google/uuid"
)

func TestNewIDProducesPostgresUUIDCompatibleIDs(t *testing.T) {
	if id := newID(); id == "" {
		t.Fatal("newID() returned an empty identifier")
	} else if _, err := uuid.Parse(id); err != nil {
		t.Fatalf("newID() = %q, want a PostgreSQL UUID-compatible identifier: %v", id, err)
	}
}
