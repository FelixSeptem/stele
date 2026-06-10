package postgres

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	pgxmock "github.com/pashagolub/pgxmock/v4"
)

func TestBootstrapDatabaseRunsBaseMigration(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatalf("pgxmock.NewPool() error = %v", err)
	}
	defer mock.Close()

	mock.ExpectBegin()
	mock.ExpectExec("CREATE EXTENSION IF NOT EXISTS pgcrypto").WillReturnResult(pgxmock.NewResult("CREATE EXTENSION", 0))
	mock.ExpectExec("CREATE EXTENSION IF NOT EXISTS vector").WillReturnResult(pgxmock.NewResult("CREATE EXTENSION", 0))
	mock.ExpectExec("CREATE TABLE IF NOT EXISTS raw_events").WillReturnResult(pgxmock.NewResult("CREATE TABLE", 0))
	mock.ExpectExec("ALTER TABLE raw_events").WillReturnResult(pgxmock.NewResult("ALTER TABLE", 0))
	mock.ExpectExec("CREATE TABLE IF NOT EXISTS candidate_memories").WillReturnResult(pgxmock.NewResult("CREATE TABLE", 0))
	mock.ExpectExec("CREATE TABLE IF NOT EXISTS canonical_memories").WillReturnResult(pgxmock.NewResult("CREATE TABLE", 0))
	mock.ExpectExec("ALTER TABLE canonical_memories").WillReturnResult(pgxmock.NewResult("ALTER TABLE", 0))
	mock.ExpectExec("CREATE TABLE IF NOT EXISTS memory_versions").WillReturnResult(pgxmock.NewResult("CREATE TABLE", 0))
	mock.ExpectExec("CREATE TABLE IF NOT EXISTS provenance_links").WillReturnResult(pgxmock.NewResult("CREATE TABLE", 0))
	mock.ExpectExec("CREATE TABLE IF NOT EXISTS deletion_markers").WillReturnResult(pgxmock.NewResult("CREATE TABLE", 0))
	mock.ExpectExec("CREATE TABLE IF NOT EXISTS relation_projections").WillReturnResult(pgxmock.NewResult("CREATE TABLE", 0))
	mock.ExpectExec("CREATE TABLE IF NOT EXISTS job_executions").WillReturnResult(pgxmock.NewResult("CREATE TABLE", 0))
	mock.ExpectExec("CREATE INDEX IF NOT EXISTS raw_events_scope_created_at_idx").WillReturnResult(pgxmock.NewResult("CREATE INDEX", 0))
	mock.ExpectExec("CREATE INDEX IF NOT EXISTS raw_events_governance_claim_idx").WillReturnResult(pgxmock.NewResult("CREATE INDEX", 0))
	mock.ExpectExec("CREATE INDEX IF NOT EXISTS candidate_memories_source_raw_event_idx").WillReturnResult(pgxmock.NewResult("CREATE INDEX", 0))
	mock.ExpectExec("CREATE INDEX IF NOT EXISTS candidate_memories_scope_status_idx").WillReturnResult(pgxmock.NewResult("CREATE INDEX", 0))
	mock.ExpectExec("CREATE INDEX IF NOT EXISTS canonical_memories_scope_state_idx").WillReturnResult(pgxmock.NewResult("CREATE INDEX", 0))
	mock.ExpectExec("CREATE INDEX IF NOT EXISTS canonical_memories_updated_at_idx").WillReturnResult(pgxmock.NewResult("CREATE INDEX", 0))
	mock.ExpectExec("CREATE INDEX IF NOT EXISTS canonical_memories_search_text_idx").WillReturnResult(pgxmock.NewResult("CREATE INDEX", 0))
	mock.ExpectExec("CREATE INDEX IF NOT EXISTS memory_versions_memory_created_at_idx").WillReturnResult(pgxmock.NewResult("CREATE INDEX", 0))
	mock.ExpectExec("CREATE INDEX IF NOT EXISTS provenance_links_scope_created_at_idx").WillReturnResult(pgxmock.NewResult("CREATE INDEX", 0))
	mock.ExpectExec("CREATE INDEX IF NOT EXISTS relation_projections_scope_updated_at_idx").WillReturnResult(pgxmock.NewResult("CREATE INDEX", 0))
	mock.ExpectExec("CREATE INDEX IF NOT EXISTS relation_projections_search_text_idx").WillReturnResult(pgxmock.NewResult("CREATE INDEX", 0))
	mock.ExpectExec("CREATE INDEX IF NOT EXISTS job_executions_scope_started_at_idx").WillReturnResult(pgxmock.NewResult("CREATE INDEX", 0))
	mock.ExpectExec("CREATE INDEX IF NOT EXISTS job_executions_job_name_started_at_idx").WillReturnResult(pgxmock.NewResult("CREATE INDEX", 0))
	mock.ExpectCommit()

	if err := BootstrapDatabase(context.Background(), mock); err != nil {
		t.Fatalf("BootstrapDatabase() error = %v", err)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("ExpectationsWereMet() error = %v", err)
	}
}

func TestBootstrapDatabaseRejectsMissingSchema(t *testing.T) {
	if err := applySQL(context.Background(), nil, ""); err == nil {
		t.Fatal("applySQL() error = nil, want schema error")
	}
}

func TestOpenPoolRejectsInvalidDSN(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	_, err := OpenPool(ctx, "://bad-dsn")
	if err == nil {
		t.Fatal("OpenPool() error = nil, want invalid DSN error")
	}
}

func TestBootstrapDatabaseReturnsBeginFailure(t *testing.T) {
	db := failingBootstrapDB{err: errors.New("connection refused")}

	err := BootstrapDatabase(context.Background(), db)
	if err == nil {
		t.Fatal("BootstrapDatabase() error = nil, want begin failure")
	}

	if !strings.Contains(err.Error(), "begin bootstrap transaction") {
		t.Fatalf("error = %q, want wrapped begin failure", err)
	}
}

type failingBootstrapDB struct {
	err error
}

func (f failingBootstrapDB) BeginTx(ctx context.Context, txOptions pgx.TxOptions) (pgx.Tx, error) {
	return nil, f.err
}
