package postgres

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/FelixSeptem/stele/internal/memory"
	"github.com/FelixSeptem/stele/internal/provider"
)

// This is opt-in because it writes only to an explicitly supplied disposable
// database. It exercises the real PostgreSQL projection and pgvector-backed
// schema without claiming an ambient operator database is safe to use.
func TestProviderSyncPostgresPgvectorIntegration(t *testing.T) {
	dsn := os.Getenv("STELE_TEST_POSTGRES_SYNC_DSN")
	if dsn == "" {
		t.Skip("STELE_TEST_POSTGRES_SYNC_DSN is not configured; skipping provider sync real-stack smoke")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	pool, err := OpenPool(ctx, dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	if err := NewMigrationRunner().Apply(ctx, dsn); err != nil {
		t.Fatal(err)
	}
	scope := memory.Scope{Tenant: "sync-smoke", Project: "p", Namespace: "n"}
	foreign := memory.Scope{Tenant: "sync-smoke-foreign", Project: "p", Namespace: "n"}
	if _, err = pool.Exec(ctx, `DELETE FROM provider_sync_cursors WHERE tenant IN ($1,$2)`, scope.Tenant, foreign.Tenant); err != nil {
		t.Fatal(err)
	}
	if _, err = pool.Exec(ctx, `DELETE FROM raw_events WHERE tenant IN ($1,$2)`, scope.Tenant, foreign.Tenant); err != nil {
		t.Fatal(err)
	}
	for _, item := range []struct {
		scope   memory.Scope
		content string
	}{{scope, "in-scope"}, {foreign, "foreign"}} {
		if _, err := pool.Exec(ctx, `INSERT INTO raw_events (tenant, project, namespace, event_type, content, metadata) VALUES ($1,$2,$3,$4,$5,$6)`, item.scope.Tenant, item.scope.Project, item.scope.Namespace, "sync.smoke", item.content, []byte(`{"fixture":"sync"}`)); err != nil {
			t.Fatal(err)
		}
	}
	binding := provider.RuntimeBinding{BindingID: "sync-smoke-binding", Scope: scope, AgentID: "agent", SessionID: "session", ProviderInstanceID: "provider", CreatedAt: time.Now().Add(-time.Minute), ExpiresAt: time.Now().Add(time.Hour)}
	caps := provider.Discover(provider.CapabilityInput{}).Synchronization
	syncer := provider.Synchronizer{Source: ProviderSyncSource{Repository: NewRepository(pool)}, CursorStore: NewRepository(pool), Capabilities: caps}
	initial, err := syncer.Synchronize(ctx, binding, provider.SyncRequest{SchemaVersion: provider.SyncContractVersion, MaxEvents: 10})
	if err != nil {
		t.Fatal(err)
	}
	resumed, err := syncer.Synchronize(ctx, binding, provider.SyncRequest{SchemaVersion: provider.SyncContractVersion, Cursor: initial.NextCursor, MaxEvents: 10})
	if err != nil {
		t.Fatal(err)
	}
	if len(resumed.Events) != 1 || resumed.Events[0].Payload == nil || resumed.Events[0].ReplayID == "" {
		t.Fatalf("unexpected scoped sync result: %+v", resumed)
	}
	if string(resumed.Events[0].Payload) == "foreign" {
		t.Fatal("foreign event leaked into synchronization payload")
	}
}
