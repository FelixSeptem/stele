package postgres

import (
	"context"
	"fmt"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/FelixSeptem/stele/internal/memory"
	"github.com/FelixSeptem/stele/internal/workqueue"
)

func TestDerivedWorkPostgresRecoveryAndScopeIsolation(t *testing.T) {
	dsn := os.Getenv("STELE_TEST_POSTGRES_DERIVED_WORK_DSN")
	if dsn == "" {
		t.Skip("STELE_TEST_POSTGRES_DERIVED_WORK_DSN is not configured; skipping derived-work integration test")
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
	suffix := fmt.Sprintf("%d", time.Now().UnixNano())
	scope := memory.Scope{Tenant: "derived-it-" + suffix, Project: "p", Namespace: "n"}
	defer pool.Exec(ctx, `DELETE FROM derived_work_items WHERE tenant = $1`, scope.Tenant)
	repo := NewRepository(pool)
	now := time.Now().UTC().Truncate(time.Microsecond)
	input := workqueue.EnqueueInput{DerivedWorkInput: workqueue.DerivedWorkInput{Scope: scope, Kind: workqueue.WorkKindReflection, Watermark: "wm-1", Idempotency: "idempotent-1", Reference: "ref-1"}, MaxAttempts: 1, Now: now, DetailExpiresAt: now.Add(time.Hour)}
	first, err := repo.EnqueueDerivedWork(ctx, input)
	if err != nil {
		t.Fatal(err)
	}
	duplicate, err := repo.EnqueueDerivedWork(ctx, input)
	if err != nil {
		t.Fatal(err)
	}
	if duplicate.ID != first.ID || duplicate.WorkKey != first.WorkKey {
		t.Fatalf("duplicate identity = %+v/%+v", first, duplicate)
	}

	secondInput := input
	secondInput.Watermark = "wm-2"
	secondInput.Idempotency = "idempotent-2"
	second, err := repo.EnqueueDerivedWork(ctx, secondInput)
	if err != nil {
		t.Fatal(err)
	}
	var wg sync.WaitGroup
	claims := make(chan []workqueue.DerivedWorkItem, 2)
	for _, worker := range []string{"worker-a", "worker-b"} {
		wg.Add(1)
		go func(worker string) {
			defer wg.Done()
			items, claimErr := repo.ClaimDerivedWork(ctx, workqueue.ClaimInput{Scope: scope, WorkerID: worker, Now: now.Add(time.Second), LeaseDuration: time.Minute, Limit: 1})
			if claimErr != nil {
				t.Errorf("claim %s: %v", worker, claimErr)
				return
			}
			claims <- items
		}(worker)
	}
	wg.Wait()
	close(claims)
	claimedIDs := map[string]bool{}
	for items := range claims {
		for _, item := range items {
			claimedIDs[item.ID] = true
		}
	}
	if len(claimedIDs) != 2 || !claimedIDs[first.ID] || !claimedIDs[second.ID] {
		t.Fatalf("concurrent claims = %#v, want both items exactly once", claimedIDs)
	}

	staleInput := input
	staleInput.Watermark = "wm-stale"
	staleInput.Idempotency = "idempotent-stale"
	stale, err := repo.EnqueueDerivedWork(ctx, staleInput)
	if err != nil {
		t.Fatal(err)
	}
	claimed, err := repo.ClaimDerivedWork(ctx, workqueue.ClaimInput{Scope: scope, WorkerID: "worker-stale", Now: now.Add(2 * time.Second), LeaseDuration: time.Second, Limit: 1})
	if err != nil || len(claimed) != 1 || claimed[0].ID != stale.ID {
		t.Fatalf("stale claim = %+v, %v", claimed, err)
	}
	reclaimed, err := repo.ClaimDerivedWork(ctx, workqueue.ClaimInput{Scope: scope, WorkerID: "worker-reclaim", Now: now.Add(4 * time.Second), LeaseDuration: time.Minute, Limit: 1})
	if err != nil || len(reclaimed) != 1 || reclaimed[0].ID != stale.ID || reclaimed[0].LeaseOwner != "worker-reclaim" {
		t.Fatalf("reclaimed = %+v, %v", reclaimed, err)
	}
	if err := repo.RetryDerivedWork(ctx, workqueue.RetryInput{Scope: scope, WorkID: stale.ID, WorkerID: "worker-reclaim", FailureCategory: "transient", RetryAt: now.Add(5 * time.Second), FailedAt: now.Add(4 * time.Second)}); err != nil {
		t.Fatal(err)
	}
	var state string
	if err := pool.QueryRow(ctx, `SELECT state FROM derived_work_items WHERE id = $1`, stale.ID).Scan(&state); err != nil {
		t.Fatal(err)
	}
	if state != string(workqueue.WorkStateExhausted) {
		t.Fatalf("retry exhaustion state = %q", state)
	}
	foreign := scope
	foreign.Tenant += "-foreign"
	items, err := repo.ListDerivedWork(ctx, workqueue.ListInput{Scope: foreign, Limit: 10})
	if err != nil {
		t.Fatal(err)
	}
	if len(items.Items) != 0 {
		t.Fatalf("foreign scope listed %d items", len(items.Items))
	}
}
