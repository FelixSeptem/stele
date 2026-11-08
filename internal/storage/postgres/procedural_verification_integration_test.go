package postgres

import (
	"context"
	"errors"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/FelixSeptem/stele/internal/memory"
	"github.com/google/uuid"
)

// TestProceduralVerificationPostgresReplayScopeAndLeaseRecovery runs against
// the owner-provided PostgreSQL fixture when configured. It proves the real
// trigger, append-only ledger, replay identity, scope boundary, and run lease
// recovery rather than re-creating any production tables in test code.
func TestProceduralVerificationPostgresReplayScopeAndLeaseRecovery(t *testing.T) {
	dsn := strings.TrimSpace(os.Getenv("STELE_TEST_POSTGRES_VERIFICATION_DSN"))
	if dsn == "" {
		t.Skip("STELE_TEST_POSTGRES_VERIFICATION_DSN is not configured")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if err := NewMigrationRunner().Apply(ctx, dsn); err != nil {
		t.Fatal(err)
	}
	pool, err := OpenPool(ctx, dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()

	scope := memory.Scope{Tenant: "verification-it-" + uuid.NewString(), Project: "project", Namespace: "namespace"}
	memoryID, eventID := uuid.NewString(), uuid.NewString()
	if _, err := pool.Exec(ctx, `INSERT INTO canonical_memories (id,tenant,project,namespace,class,state,content) VALUES ($1::uuid,$2,$3,$4,'procedural','active','observe then retry')`, memoryID, scope.Tenant, scope.Project, scope.Namespace); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `INSERT INTO memory_versions (memory_id,version,state,content,modified_by) VALUES ($1::uuid,1,'active','observe then retry','fixture')`, memoryID); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `INSERT INTO raw_events (id,tenant,project,namespace,event_type,content) VALUES ($1::uuid,$2,$3,$4,'tool.result','ok')`, eventID, scope.Tenant, scope.Project, scope.Namespace); err != nil {
		t.Fatal(err)
	}

	now := time.Now().UTC().Truncate(time.Microsecond)
	record := memory.ProceduralMemoryVerification{
		ID: uuid.NewString(), Scope: scope, MemoryID: memoryID, TargetVersion: 1,
		ClaimFingerprint: strings.Repeat("a", 64), ApplicabilityFingerprint: strings.Repeat("b", 64),
		Disposition: memory.ProceduralVerificationDispositionSupported,
		Evidence:    []memory.ProceduralVerificationEvidence{{Kind: memory.ProceduralVerificationEvidenceToolObservation, SourceKind: memory.ProceduralVerificationEvidenceSourceRawEvent, ID: eventID, SourceVersion: 1, Corroborates: true}},
		Actor:       "integration-admin", Reason: "fixture verification", PolicyVersion: "policy-v1", SourceWatermark: "watermark-1", CreatedAt: now,
	}
	repo := NewRepository(pool)
	service := memory.ProceduralVerificationService{Store: repo, Targets: repo, Evidence: repo}
	first, err := service.Record(ctx, record)
	if err != nil {
		t.Fatal(err)
	}
	second, err := service.Record(ctx, record)
	if err != nil || second.ID != first.ID {
		t.Fatalf("replay = %+v, %v; want original %q", second, err, first.ID)
	}
	foreign := scope
	foreign.Tenant += "-foreign"
	if _, err := repo.ReadProceduralMemoryVerification(ctx, foreign, first.ID); !errors.Is(err, memory.ErrNotFound) {
		t.Fatalf("foreign verification read error = %v, want not found", err)
	}

	run := memory.ProceduralRevalidationRun{ID: uuid.NewString(), Scope: scope, VerificationID: first.ID, MemoryID: memoryID, TargetVersion: 1, SourceWatermark: record.SourceWatermark, Trigger: memory.ProceduralRevalidationTriggerAdministrator, WorkIdentity: "run-identity-1", Status: memory.ProceduralRevalidationRunStatusPending, MaxAttempts: 3, NextAttemptAt: now, CreatedAt: now, UpdatedAt: now}
	created, err := repo.CreateProceduralRevalidationRun(ctx, run)
	if err != nil {
		t.Fatal(err)
	}
	claims, err := repo.ClaimProceduralRevalidationRuns(ctx, memory.ClaimProceduralRevalidationRunsInput{Scope: scope, WorkerID: "worker-a", Now: now, LeaseDuration: time.Second, Limit: 1})
	if err != nil || len(claims) != 1 || claims[0].ID != created.ID {
		t.Fatalf("initial claim = %+v, %v", claims, err)
	}
	// A newly constructed repository simulates process restart; the expired
	// durable lease must be reclaimable without a duplicate run.
	restarted := NewRepository(pool)
	reclaimed, err := restarted.ClaimProceduralRevalidationRuns(ctx, memory.ClaimProceduralRevalidationRunsInput{Scope: scope, WorkerID: "worker-b", Now: now.Add(2 * time.Second), LeaseDuration: time.Minute, Limit: 1})
	if err != nil || len(reclaimed) != 1 || reclaimed[0].ID != created.ID || reclaimed[0].LeaseOwner != "worker-b" {
		t.Fatalf("reclaimed = %+v, %v", reclaimed, err)
	}
	if err := restarted.CompleteProceduralRevalidationRun(ctx, memory.CompleteProceduralRevalidationRunInput{Scope: scope, RunID: created.ID, WorkerID: "worker-b", Status: memory.ProceduralRevalidationRunStatusCompleted, TerminalVerificationID: first.ID, CompletedAt: now.Add(3 * time.Second)}); err != nil {
		t.Fatal(err)
	}
}
