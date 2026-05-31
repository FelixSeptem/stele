package postgres

import (
	"context"
	"testing"
	"time"

	pgxmock "github.com/pashagolub/pgxmock/v4"

	"github.com/FelixSeptem/stele/internal/memory"
)

func TestRepositoryWriteRawEvent(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatalf("pgxmock.NewPool() error = %v", err)
	}
	defer mock.Close()

	createdAt := time.Date(2026, 5, 29, 23, 40, 0, 0, time.UTC)
	input := memory.IngestEventInput{
		Scope: memory.Scope{
			Tenant:    "tenant-a",
			Project:   "project-a",
			Namespace: "namespace-a",
		},
		EventType:       "conversation.message",
		Content:         "hello",
		Metadata:        map[string]any{"channel": "chat"},
		SourceTimestamp: time.Date(2026, 5, 29, 23, 39, 0, 0, time.UTC),
	}

	rows := pgxmock.NewRows([]string{"id", "tenant", "project", "namespace", "event_type", "content", "source_timestamp", "created_at"}).
		AddRow("evt_123", input.Scope.Tenant, input.Scope.Project, input.Scope.Namespace, input.EventType, input.Content, input.SourceTimestamp, createdAt)
	mock.ExpectQuery("INSERT INTO raw_events").
		WithArgs(input.Scope.Tenant, input.Scope.Project, input.Scope.Namespace, input.EventType, input.Content, pgxmock.AnyArg(), input.SourceTimestamp).
		WillReturnRows(rows)

	repo := NewRepository(mock)
	event, err := repo.WriteRawEvent(context.Background(), input)
	if err != nil {
		t.Fatalf("WriteRawEvent() error = %v", err)
	}

	if event.ID != "evt_123" {
		t.Fatalf("event.ID = %q, want %q", event.ID, "evt_123")
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("ExpectationsWereMet() error = %v", err)
	}
}

func TestRepositoryWriteProvenance(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatalf("pgxmock.NewPool() error = %v", err)
	}
	defer mock.Close()

	record := memory.ProvenanceRecord{
		ID:         "prov_123",
		RawEventID: "evt_123",
		Scope: memory.Scope{
			Tenant:    "tenant-a",
			Project:   "project-a",
			Namespace: "namespace-a",
		},
		Operation: "ingest_event",
		CreatedAt: time.Date(2026, 5, 29, 23, 45, 0, 0, time.UTC),
	}

	mock.ExpectExec("INSERT INTO provenance_links").
		WithArgs(record.ID, record.RawEventID, record.Scope.Tenant, record.Scope.Project, record.Scope.Namespace, record.Operation, record.CreatedAt).
		WillReturnResult(pgxmock.NewResult("INSERT", 1))

	repo := NewRepository(mock)
	if err := repo.WriteProvenance(context.Background(), record); err != nil {
		t.Fatalf("WriteProvenance() error = %v", err)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("ExpectationsWereMet() error = %v", err)
	}
}

func TestRepositoryIngestEventWritesRawEventAndProvenanceInOneTransaction(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatalf("pgxmock.NewPool() error = %v", err)
	}
	defer mock.Close()

	createdAt := time.Date(2026, 5, 29, 23, 50, 0, 0, time.UTC)
	input := memory.IngestEventInput{
		Scope: memory.Scope{
			Tenant:    "tenant-a",
			Project:   "project-a",
			Namespace: "namespace-a",
		},
		EventType:       "conversation.message",
		Content:         "hello",
		Metadata:        map[string]any{"channel": "chat"},
		SourceTimestamp: time.Date(2026, 5, 29, 23, 49, 0, 0, time.UTC),
	}
	provenance := memory.ProvenanceRecord{
		ID:        "prov_123",
		Scope:     input.Scope,
		Operation: "ingest_event",
		CreatedAt: createdAt,
	}

	mock.ExpectBegin()
	mock.ExpectQuery("INSERT INTO raw_events").
		WithArgs(input.Scope.Tenant, input.Scope.Project, input.Scope.Namespace, input.EventType, input.Content, pgxmock.AnyArg(), input.SourceTimestamp).
		WillReturnRows(pgxmock.NewRows([]string{"id", "tenant", "project", "namespace", "event_type", "content", "source_timestamp", "created_at"}).
			AddRow("evt_123", input.Scope.Tenant, input.Scope.Project, input.Scope.Namespace, input.EventType, input.Content, input.SourceTimestamp, createdAt))
	mock.ExpectExec("INSERT INTO provenance_links").
		WithArgs(provenance.ID, "evt_123", input.Scope.Tenant, input.Scope.Project, input.Scope.Namespace, provenance.Operation, provenance.CreatedAt).
		WillReturnResult(pgxmock.NewResult("INSERT", 1))
	mock.ExpectCommit()

	repo := NewRepository(mock)
	event, err := repo.IngestEvent(context.Background(), input, provenance)
	if err != nil {
		t.Fatalf("IngestEvent() error = %v", err)
	}

	if event.ID != "evt_123" {
		t.Fatalf("event.ID = %q, want %q", event.ID, "evt_123")
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("ExpectationsWereMet() error = %v", err)
	}
}

func TestMigrateRunsBaseSchema(t *testing.T) {
	sql, err := BaseSchemaSQL()
	if err != nil {
		t.Fatalf("BaseSchemaSQL() error = %v", err)
	}

	for _, want := range []string{
		"CREATE TABLE IF NOT EXISTS raw_events",
		"CREATE TABLE IF NOT EXISTS canonical_memories",
		"CREATE TABLE IF NOT EXISTS memory_versions",
		"CREATE TABLE IF NOT EXISTS provenance_links",
	} {
		if !containsSQL(sql, want) {
			t.Fatalf("BaseSchemaSQL() missing %q", want)
		}
	}
}

func containsSQL(haystack, needle string) bool {
	return len(haystack) >= len(needle) && context.Background() != nil && (stringIndex(haystack, needle) >= 0)
}

func stringIndex(s, substr string) int {
	for i := 0; i+len(substr) <= len(s); i++ {
		if s[i:i+len(substr)] == substr {
			return i
		}
	}
	return -1
}
