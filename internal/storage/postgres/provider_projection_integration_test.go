package postgres

import (
	"context"
	"errors"
	"fmt"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/FelixSeptem/stele/internal/memory"
	"github.com/FelixSeptem/stele/internal/provider"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Only an explicitly supplied, empty disposable database is accepted. This
// fixture applies official migrations rather than recreating production tables.
func TestProjectionPostgresTransactions(t *testing.T) {
	dsn := os.Getenv("STELE_TEST_PROVIDER_PROJECTION_DSN")
	if dsn == "" {
		t.Skip("fresh disposable PC6 PostgreSQL DSN not configured")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatal("fixture connection failed")
	}
	defer pool.Close()
	var n int
	if err = pool.QueryRow(ctx, `SELECT count(*) FROM information_schema.tables WHERE table_schema='public'`).Scan(&n); err != nil || n != 0 {
		t.Fatal("PC6 fixture requires an empty disposable database")
	}
	if err = NewMigrationRunner().Apply(ctx, dsn); err != nil {
		t.Fatal(err)
	}
	status, err := NewMigrationRunner().Status(ctx, dsn)
	if err != nil || status.Dirty || status.CurrentVersion != 37 {
		t.Fatalf("migration not clean: %+v %v", status, err)
	}
	b := provider.RuntimeBinding{BindingID: "pc6_binding", PrincipalID: "pc6_principal", Scope: memory.Scope{Tenant: "t", Project: "p", Namespace: "n"}, AgentID: "a", SessionID: "s", ConversationID: "c", ProviderInstanceID: "pi", CreatedAt: time.Now().UTC(), ExpiresAt: time.Now().UTC().Add(time.Hour)}
	if _, err = pool.Exec(ctx, `INSERT INTO access_principals(id,role,status,label) VALUES('pc6_principal','agent','active','fixture'); INSERT INTO access_scope_grants(id,principal_id,tenant,project,namespace,status,allowed_disclosure_profiles,default_disclosure_profile) VALUES('pc6_grant','pc6_principal','t','p','n','active',ARRAY['memory-reference-v1'],'memory-reference-v1')`); err != nil {
		t.Fatal(err)
	}
	if _, err = pool.Exec(ctx, `INSERT INTO provider_runtime_bindings(binding_id,principal_id,tenant,project,namespace,agent_id,session_id,conversation_id,provider_instance_id,created_at,expires_at) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11)`, append(receiptIdentityArgs(b), b.CreatedAt, b.ExpiresAt)...); err != nil {
		t.Fatal(err)
	}
	repo := NewRepository(pool)
	req := provider.ProjectionRequest{Metadata: provider.OperationMetadata{RequestID: "r", OperationID: "o", SchemaVersion: "v1", IdempotencyKey: "key1"}, Input: provider.ProjectionInput{ContractVersion: provider.ProjectionContractVersion, Kind: "outcome", EffectID: "effect", SourceRevision: 1, OccurredAt: "2026-10-16T00:00:00Z", ReportedOutcome: "succeeded"}}
	first, err := repo.AdmitProjection(ctx, b, req)
	if err != nil || first.Validate() != nil || first.Disposition != "accepted" {
		t.Fatalf("admit: %+v %v", first, err)
	}
	var wg sync.WaitGroup
	errs := make(chan error, 12)
	for i := 0; i < 12; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			r, e := NewRepository(pool).AdmitProjection(ctx, b, req)
			if e == nil && (r.ProjectionID != first.ProjectionID || r.LogicalID != first.LogicalID) {
				e = fmt.Errorf("retry identity changed")
			}
			errs <- e
		}()
	}
	wg.Wait()
	close(errs)
	for e := range errs {
		if e != nil {
			t.Fatal(e)
		}
	}
	changed := req
	changed.Input.ReportedOutcome = "failed"
	if _, err = repo.AdmitProjection(ctx, b, changed); !errors.Is(err, provider.ErrProjectionConflict) {
		t.Fatalf("key collision: %v", err)
	}
	changed.Metadata.IdempotencyKey = "key2"
	changed.Input.SourceRevision = 3
	correction, err := repo.AdmitProjection(ctx, b, changed)
	if err != nil || correction.Disposition != "corrected" || correction.SupersedesID != first.ProjectionID || correction.LogicalID != first.LogicalID {
		t.Fatalf("correction: %+v %v", correction, err)
	}
	stale := req
	stale.Metadata.IdempotencyKey = "delayed"
	stale.Input.SourceRevision = 2
	if _, err = repo.AdmitProjection(ctx, b, stale); !errors.Is(err, provider.ErrProjectionStale) {
		t.Fatalf("stale revision: %v", err)
	}
	if r, e := NewRepository(pool).AdmitProjection(ctx, b, req); e != nil || r.ProjectionID != first.ProjectionID {
		t.Fatal("old equivalent replay failed after correction", e)
	}
	selector := provider.ProjectionHistoryInput{ContractVersion: provider.ProjectionContractVersion, Kind: "outcome", EffectID: "effect", Limit: 1}
	history, more, err := repo.ProjectionHistory(ctx, b, selector)
	if err != nil || !more || len(history) != 1 || history[0].ProjectionID != correction.ProjectionID {
		t.Fatalf("bounded history: %+v %v %v", history, more, err)
	}
	selector.Limit = 8
	history, more, err = repo.ProjectionHistory(ctx, b, selector)
	if err != nil || more || len(history) != 2 || history[1].ProjectionID != first.ProjectionID {
		t.Fatal("immutable history lost", err)
	}
	for _, mutate := range []func(*provider.RuntimeBinding){func(x *provider.RuntimeBinding) { x.BindingID = "other" }, func(x *provider.RuntimeBinding) { x.PrincipalID = "other" }, func(x *provider.RuntimeBinding) { x.Scope.Tenant = "other" }, func(x *provider.RuntimeBinding) { x.Scope.Project = "other" }, func(x *provider.RuntimeBinding) { x.Scope.Namespace = "other" }, func(x *provider.RuntimeBinding) { x.AgentID = "other" }, func(x *provider.RuntimeBinding) { x.SessionID = "other" }, func(x *provider.RuntimeBinding) { x.ConversationID = "other" }, func(x *provider.RuntimeBinding) { x.ProviderInstanceID = "other" }} {
		foreign := b
		mutate(&foreign)
		if _, err = repo.AdmitProjection(ctx, foreign, req); !errors.Is(err, provider.ErrProjectionUnavailable) {
			t.Fatal("foreign write not denied", err)
		}
		if _, _, err = repo.ProjectionHistory(ctx, foreign, selector); !errors.Is(err, provider.ErrProjectionUnavailable) {
			t.Fatal("foreign history not denied", err)
		}
	}
	// Enforced by the database even outside repository callers.
	if _, err = pool.Exec(ctx, `UPDATE provider_projection_revisions SET payload='{}' WHERE projection_id=$1`, first.ProjectionID); err == nil {
		t.Fatal("audit update allowed")
	}
	if _, err = pool.Exec(ctx, `DELETE FROM provider_projection_revisions WHERE projection_id=$1`, first.ProjectionID); err == nil {
		t.Fatal("audit delete allowed")
	}
	var count int
	if err = pool.QueryRow(ctx, `SELECT count(*) FROM provider_projection_revisions`).Scan(&count); err != nil || count != 2 {
		t.Fatal("conflict/stale/retry appended rows", count, err)
	}
	t.Run("context_and_session_attribution", func(t *testing.T) { projectionAttributionFixture(t, ctx, pool, repo, b) })
	t.Run("feedback_scope_fairness", func(t *testing.T) { projectionFeedbackScopeFairnessFixture(t, ctx, pool, repo, b) })
	t.Run("asynchronous_feedback", func(t *testing.T) { projectionFeedbackFixture(t, ctx, pool, repo, b) })
	if _, err = pool.Exec(ctx, `UPDATE access_scope_grants SET access_mode='read_only' WHERE id='pc6_grant'`); err != nil {
		t.Fatal(err)
	}
	if _, err = repo.AdmitProjection(ctx, b, req); !errors.Is(err, provider.ErrProjectionUnavailable) {
		t.Fatal("read-only write allowed", err)
	}
	if _, _, err = repo.ProjectionHistory(ctx, b, selector); err != nil {
		t.Fatal("read-only history denied", err)
	}
	if _, err = pool.Exec(ctx, `UPDATE access_scope_grants SET status='revoked' WHERE id='pc6_grant'`); err != nil {
		t.Fatal(err)
	}
	if _, _, err = repo.ProjectionHistory(ctx, b, selector); !errors.Is(err, provider.ErrProjectionUnavailable) {
		t.Fatal("revoked history allowed", err)
	}
}
