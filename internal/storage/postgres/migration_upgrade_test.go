package postgres

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/FelixSeptem/stele/internal/memory"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// TestMigrationRunnerUpgradesPopulatedPriorRelease applies the current
// migration ledger to a database created by the supported pre-ledger bootstrap
// path. The harness provides a disposable DSN; this test never targets an
// operator or developer database.
func TestMigrationRunnerUpgradesPopulatedPriorRelease(t *testing.T) {
	dsn := os.Getenv("STELE_TEST_POSTGRES_UPGRADE_DSN")
	if dsn == "" {
		t.Skip("STELE_TEST_POSTGRES_UPGRADE_DSN is not configured; skipping real PostgreSQL upgrade test")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	pool, err := OpenPool(ctx, dsn)
	if err != nil {
		t.Fatalf("OpenPool() error = %v", err)
	}
	defer pool.Close()

	if err := BootstrapDatabase(ctx, pool); err != nil {
		t.Fatalf("BootstrapDatabase() error = %v", err)
	}
	var ledger *string
	if err := pool.QueryRow(ctx, `SELECT to_regclass('schema_migrations')`).Scan(&ledger); err != nil {
		t.Fatalf("read prior-release migration ledger: %v", err)
	}
	if ledger != nil {
		t.Fatalf("prior-release fixture unexpectedly has migration ledger %q", *ledger)
	}

	fixture := populatedPriorReleaseFixture{
		Scope:        memory.Scope{Tenant: "upgrade-tenant", Project: "upgrade-project", Namespace: "upgrade-namespace"},
		OtherScope:   memory.Scope{Tenant: "other-tenant", Project: "other-project", Namespace: "other-namespace"},
		PrincipalID:  "upgrade-principal",
		CredentialID: "stl_upgrade_fixture",
		EventID:      "00000000-0000-0000-0000-000000000101",
		MemoryID:     "00000000-0000-0000-0000-000000000102",
		VersionID:    "00000000-0000-0000-0000-000000000103",
		ProvenanceID: "00000000-0000-0000-0000-000000000104",
		CreatedAt:    time.Date(2026, 8, 1, 12, 0, 0, 0, time.UTC),
	}
	if err := fixture.insert(ctx, pool); err != nil {
		t.Fatalf("insert populated prior-release fixture: %v", err)
	}

	runner := NewMigrationRunner()
	if err := runner.Apply(ctx, dsn); err != nil {
		t.Fatalf("Apply() upgrade error = %v", err)
	}
	state, err := runner.Status(ctx, dsn)
	if err != nil {
		t.Fatalf("Status() after upgrade error = %v", err)
	}
	if state.Status != MigrationStatusCurrent || state.Dirty || state.Pending || state.CurrentVersion != CurrentMigrationVersion {
		t.Fatalf("migration state after upgrade = %+v, want current clean version %d", state, CurrentMigrationVersion)
	}

	repo := NewRepository(pool)
	principal, credential, err := repo.ReadPrincipalCredential(ctx, fixture.CredentialID)
	if err != nil {
		t.Fatalf("ReadPrincipalCredential() error = %v", err)
	}
	if principal.ID != fixture.PrincipalID || credential.CredentialID != fixture.CredentialID {
		t.Fatalf("principal credential = %+v %+v, want preserved prior-release fixture", principal, credential)
	}
	granted, err := repo.HasActiveScopeGrant(ctx, fixture.PrincipalID, fixture.Scope)
	if err != nil || !granted {
		t.Fatalf("HasActiveScopeGrant() = %t, %v; want preserved exact grant", granted, err)
	}

	replay, err := repo.IngestEventIdempotent(ctx,
		memory.IdempotentEventIngestInput{PrincipalID: fixture.PrincipalID, IdempotencyKey: "upgrade-idempotency", RequestFingerprint: "upgrade-fingerprint"},
		memory.IngestEventInput{Scope: fixture.Scope, EventType: "prior.release.event", Content: "preserved idempotent event", SourceTimestamp: fixture.CreatedAt},
		memory.ProvenanceRecord{},
		memory.AdmissionPressureReport{},
	)
	if err != nil || !replay.Replayed || replay.Event.ID != fixture.EventID {
		t.Fatalf("IngestEventIdempotent() = %+v, %v; want replay of preserved event %q", replay, err, fixture.EventID)
	}

	history, err := repo.ReadMemoryHistory(ctx, fixture.Scope, fixture.MemoryID, false)
	if err != nil {
		t.Fatalf("ReadMemoryHistory() error = %v", err)
	}
	if history.Memory.ID != fixture.MemoryID || history.Memory.Content != "preserved canonical memory" {
		t.Fatalf("canonical memory = %+v, want preserved prior-release memory", history.Memory)
	}
	if len(history.Versions) != 1 || history.Versions[0].ID != fixture.VersionID || history.Versions[0].Version != 1 {
		t.Fatalf("memory versions = %+v, want preserved version history", history.Versions)
	}
	if len(history.Provenance) != 1 || history.Provenance[0].ID != fixture.ProvenanceID || history.Provenance[0].RawEventID != fixture.EventID {
		t.Fatalf("memory provenance = %+v, want preserved provenance", history.Provenance)
	}
	if _, err := repo.ReadMemoryHistory(ctx, fixture.OtherScope, fixture.MemoryID, false); err == nil {
		t.Fatal("cross-scope memory history read succeeded after upgrade")
	}
}

type populatedPriorReleaseFixture struct {
	Scope        memory.Scope
	OtherScope   memory.Scope
	PrincipalID  string
	CredentialID string
	EventID      string
	MemoryID     string
	VersionID    string
	ProvenanceID string
	CreatedAt    time.Time
}

func (f populatedPriorReleaseFixture) insert(ctx context.Context, db *pgxpool.Pool) error {
	tx, err := db.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return fmt.Errorf("begin prior-release fixture transaction: %w", err)
	}
	defer tx.Rollback(ctx)

	exec := func(query string, args ...any) error {
		if _, err := tx.Exec(ctx, query, args...); err != nil {
			return err
		}
		return nil
	}
	if err := exec(`INSERT INTO access_principals (id, role, status, label, created_at, updated_at) VALUES ($1, 'public', 'active', 'prior-release fixture', $2, $2)`, f.PrincipalID, f.CreatedAt); err != nil {
		return fmt.Errorf("insert prior-release principal: %w", err)
	}
	if err := exec(`INSERT INTO access_credentials (id, principal_id, status, credential_id, salt, digest, created_at) VALUES ('upgrade-credential', $1, 'active', $2, $3, $4, $5)`, f.PrincipalID, f.CredentialID, []byte("prior-release-salt"), []byte("prior-release-digest"), f.CreatedAt); err != nil {
		return fmt.Errorf("insert prior-release credential: %w", err)
	}
	if err := exec(`INSERT INTO access_scope_grants (id, principal_id, tenant, project, namespace, status, created_at) VALUES ('upgrade-grant', $1, $2, $3, $4, 'active', $5)`, f.PrincipalID, f.Scope.Tenant, f.Scope.Project, f.Scope.Namespace, f.CreatedAt); err != nil {
		return fmt.Errorf("insert prior-release grant: %w", err)
	}
	if err := exec(`INSERT INTO raw_events (id, tenant, project, namespace, event_type, content, metadata, source_timestamp, created_at) VALUES ($1, $2, $3, $4, 'prior.release.event', 'preserved idempotent event', '{}'::jsonb, $5, $5)`, f.EventID, f.Scope.Tenant, f.Scope.Project, f.Scope.Namespace, f.CreatedAt); err != nil {
		return fmt.Errorf("insert prior-release raw event: %w", err)
	}
	if err := exec(`INSERT INTO event_idempotency_records (id, principal_id, tenant, project, namespace, idempotency_key, request_fingerprint, status, raw_event_id, admission, created_at, completed_at) VALUES ('upgrade-idempotency-record', $1, $2, $3, $4, 'upgrade-idempotency', 'upgrade-fingerprint', 'completed', $5, '{}'::jsonb, $6, $6)`, f.PrincipalID, f.Scope.Tenant, f.Scope.Project, f.Scope.Namespace, f.EventID, f.CreatedAt); err != nil {
		return fmt.Errorf("insert prior-release idempotency record: %w", err)
	}
	if err := exec(`INSERT INTO canonical_memories (id, tenant, project, namespace, class, state, content, metadata, created_at, updated_at) VALUES ($1, $2, $3, $4, 'episodic', 'active', 'preserved canonical memory', '{}'::jsonb, $5, $5)`, f.MemoryID, f.Scope.Tenant, f.Scope.Project, f.Scope.Namespace, f.CreatedAt); err != nil {
		return fmt.Errorf("insert prior-release canonical memory: %w", err)
	}
	if err := exec(`INSERT INTO memory_versions (id, memory_id, version, state, content, metadata, created_at, modified_by) VALUES ($1, $2, 1, 'active', 'preserved canonical memory', '{}'::jsonb, $3, 'prior-release')`, f.VersionID, f.MemoryID, f.CreatedAt); err != nil {
		return fmt.Errorf("insert prior-release memory version: %w", err)
	}
	if err := exec(`INSERT INTO provenance_links (id, raw_event_id, memory_id, tenant, project, namespace, operation, source_context, created_at) VALUES ($1, $2, $3, $4, $5, $6, 'promote_candidate', '{}'::jsonb, $7)`, f.ProvenanceID, f.EventID, f.MemoryID, f.Scope.Tenant, f.Scope.Project, f.Scope.Namespace, f.CreatedAt); err != nil {
		return fmt.Errorf("insert prior-release provenance: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit prior-release fixture transaction: %w", err)
	}
	return nil
}
