package postgres

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/FelixSeptem/stele/internal/memory"
	"github.com/FelixSeptem/stele/internal/provider"
	"github.com/jackc/pgx/v5/pgxpool"
)

// An explicitly disposable DSN owns this fixture. No operator database is used.
// The schema starts at 0035, seeds text principals and source history, upgrades
// through the ledger, then exercises the durable receipt repository itself.
func TestContextContinuityPostgresTransactionsAndUpgrade(t *testing.T) {
	dsn := os.Getenv("STELE_TEST_CONTEXT_RECEIPTS_DSN")
	if dsn == "" {
		t.Skip("disposable receipt PostgreSQL DSN not configured")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatal("disposable connection unavailable")
	}
	defer pool.Close()
	var tables int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM information_schema.tables WHERE table_schema='public'`).Scan(&tables); err != nil || tables != 0 {
		t.Fatal("receipt fixture requires fresh disposable database")
	}
	// BootstrapDatabase models the supported pre-ledger release fixture. The
	// migration runner then records and verifies every current asset, including
	// the additive 0036 receipt migration.
	if err := BootstrapDatabase(ctx, pool); err != nil {
		t.Fatal("prior release bootstrap failed", err)
	}
	f := populatedPriorReleaseFixture{Scope: memory.Scope{Tenant: "t", Project: "p", Namespace: "n"}, PrincipalID: "id_pc4_public", CredentialID: "stl_pc4_fixture", EventID: "00000000-0000-0000-0000-000000000101", MemoryID: "00000000-0000-0000-0000-000000000102", VersionID: "00000000-0000-0000-0000-000000000103", ProvenanceID: "00000000-0000-0000-0000-000000000104", CreatedAt: time.Now().UTC().Add(-time.Hour)}
	if err := f.insert(ctx, pool); err != nil {
		t.Fatal("prior fixture persistence failed")
	}
	if _, err := pool.Exec(ctx, `UPDATE canonical_memories SET metadata='{"version":"source-metadata-v1","trust":"untrusted","classification":"medium"}'::jsonb WHERE id=$1`, f.MemoryID); err != nil {
		t.Fatal(err)
	}
	if err := NewMigrationRunner().Apply(ctx, dsn); err != nil {
		t.Fatal("0035 upgrade failed", err)
	}
	repo := NewRepository(pool)
	if ok, err := repo.HasActiveScopeGrant(ctx, f.PrincipalID, f.Scope); err != nil || !ok {
		t.Fatal("upgrade lost exact grant")
	}
	history, err := repo.ReadMemoryHistory(ctx, f.Scope, f.MemoryID, false)
	if err != nil || history.Memory.ID != f.MemoryID || len(history.Versions) != 1 || len(history.Provenance) != 1 {
		t.Fatal("upgrade lost history/source labels")
	}
	b := provider.RuntimeBinding{BindingID: "id_binding", PrincipalID: f.PrincipalID, Scope: f.Scope, AgentID: "a", SessionID: "s", ConversationID: "c", ProviderInstanceID: "pi", ExpiresAt: time.Now().UTC().Add(75 * time.Second)}
	digest := provider.ContextDigest{ContractVersion: provider.ContextDigestVersion, CanonicalizationVersion: provider.ContextCanonicalVersion, Algorithm: "sha-256", SelectionPolicyVersion: provider.ContextSelectionVersion, SelectionPolicyFingerprint: strings.Repeat("a", 64), Value: strings.Repeat("b", 64)}
	fp := strings.Repeat("c", 64)
	config := provider.ContextContinuityConfig{TTLSeconds: 120, MaxReceiptsPerBinding: 2, MaxReceipts: 3, CleanupBatch: 1}
	finalize := func(r provider.ContextReceipt) error {
		if !provider.ValidContextReference(r.Reference) || len(r.ExpiresAt.UTC().Format(time.RFC3339)) != 20 {
			return fmt.Errorf("invalid final envelope")
		}
		return nil
	}
	first, err := repo.IssueContextReceipt(ctx, b, fp, digest, config, finalize)
	if err != nil {
		t.Fatal(err)
	}
	if !first.ExpiresAt.Equal(b.ExpiresAt.UTC().Truncate(time.Second)) || first.IssuedAt.Nanosecond() != 0 {
		t.Fatal("database time/binding expiry cap invalid")
	}
	var wg sync.WaitGroup
	errs := make(chan error, 12)
	for i := 0; i < 12; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			r, err := repo.IssueContextReceipt(ctx, b, fp, digest, config, finalize)
			if err == nil && (r.Reference != first.Reference || !r.ExpiresAt.Equal(first.ExpiresAt)) {
				err = fmt.Errorf("dedup renewed or diverged")
			}
			errs <- err
		}()
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		if err != nil {
			t.Fatal("concurrent dedup failed", err)
		}
	}
	for _, change := range []func(*provider.RuntimeBinding){func(x *provider.RuntimeBinding) { x.BindingID = "other" }, func(x *provider.RuntimeBinding) { x.PrincipalID = "other" }, func(x *provider.RuntimeBinding) { x.Scope.Tenant = "other" }, func(x *provider.RuntimeBinding) { x.Scope.Project = "other" }, func(x *provider.RuntimeBinding) { x.Scope.Namespace = "other" }, func(x *provider.RuntimeBinding) { x.AgentID = "other" }, func(x *provider.RuntimeBinding) { x.SessionID = "other" }, func(x *provider.RuntimeBinding) { x.ConversationID = "other" }, func(x *provider.RuntimeBinding) { x.ProviderInstanceID = "other" }} {
		x := b
		change(&x)
		if _, err := repo.LookupContextReceipt(ctx, x, first.Reference); !errors.Is(err, provider.ErrContinuityUnavailable) {
			t.Fatal("foreign identity lookup disclosed receipt")
		}
	}
	rollback := fmt.Errorf("final serialization interrupted")
	if _, err := repo.IssueContextReceipt(ctx, b, strings.Repeat("d", 64), digest, config, func(provider.ContextReceipt) error { return rollback }); !errors.Is(err, rollback) {
		t.Fatal("callback did not abort transaction")
	}
	var count int
	_ = pool.QueryRow(ctx, `SELECT count(*) FROM provider_context_receipts`).Scan(&count)
	if count != 1 {
		t.Fatal("rollback persisted receipt")
	}
	errs = make(chan error, 16)
	for i := 0; i < 16; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			x := b
			if i%2 == 0 {
				x.BindingID = "second"
			}
			_, err := repo.IssueContextReceipt(ctx, x, fmt.Sprintf("%064x", i+1), digest, config, finalize)
			if err != nil && !errors.Is(err, provider.ErrContinuityCapacity) {
				errs <- err
			}
		}(i)
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		t.Fatal("distinct issuance failed", err)
	}
	_ = pool.QueryRow(ctx, `SELECT count(*) FROM provider_context_receipts`).Scan(&count)
	if count != 3 {
		t.Fatal("global capacity not enforced")
	}
	if _, err := repo.IssueContextReceipt(ctx, b, fp, digest, config, finalize); err != nil {
		t.Fatal("reuse at capacity failed")
	}
	if n, err := repo.CleanupContextReceipts(ctx, 1); err != nil || n != 0 {
		t.Fatal("cleanup deleted live receipt")
	}
	// Focused SQL expiry fixture is explicitly distinct from public elapsed TTL evidence.
	if _, err := pool.Exec(ctx, `UPDATE provider_context_receipts SET issued_at=clock_timestamp()-interval '2 minutes',expires_at=clock_timestamp()-interval '1 minute'`); err != nil {
		t.Fatal(err)
	}
	if _, err := repo.LookupContextReceipt(ctx, b, first.Reference); !errors.Is(err, provider.ErrContinuityExpired) {
		t.Fatal("retained expiry not distinguished")
	}
	if n, err := repo.CleanupContextReceipts(ctx, 1); err != nil || n != 1 {
		t.Fatal("cleanup batch not bounded")
	}
	_ = pool.QueryRow(ctx, `SELECT count(*) FROM provider_context_receipts`).Scan(&count)
	if count != 2 {
		t.Fatal("cleanup exceeded batch")
	}
	if _, err := repo.CleanupContextReceipts(ctx, 5001); err == nil {
		t.Fatal("unbounded cleanup accepted")
	}
	if _, err := repo.CleanupContextReceipts(ctx, 5000); err != nil {
		t.Fatal(err)
	}
	if _, err := repo.LookupContextReceipt(ctx, b, first.Reference); !errors.Is(err, provider.ErrContinuityUnavailable) {
		t.Fatal("purged reference not unavailable")
	}
	// Retained expired rows still count after a single issuance cleanup batch.
	for i := 0; i < 3; i++ {
		x := b
		x.BindingID = fmt.Sprintf("expired-%d", i)
		if _, err := repo.IssueContextReceipt(ctx, x, fmt.Sprintf("%064x", i+100), digest, config, finalize); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := pool.Exec(ctx, `UPDATE provider_context_receipts SET issued_at=clock_timestamp()-interval '2 minutes',expires_at=clock_timestamp()-interval '1 minute'`); err != nil {
		t.Fatal(err)
	}
	tiny := config
	tiny.MaxReceipts = 1
	tiny.MaxReceiptsPerBinding = 1
	if _, err := repo.IssueContextReceipt(ctx, b, fp, digest, tiny, finalize); !errors.Is(err, provider.ErrContinuityCapacity) {
		t.Fatal("retained expired rows bypassed capacity")
	}
}
