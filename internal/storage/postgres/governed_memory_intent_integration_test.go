package postgres

import (
	"context"
	"errors"
	"os"
	"testing"
	"time"

	"github.com/FelixSeptem/stele/internal/memory"
	"github.com/FelixSeptem/stele/internal/workqueue"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

func TestGovernedMemoryIntentPostgresReplayQueueHistoryAndScopeIsolation(t *testing.T) {
	dsn := os.Getenv("STELE_TEST_POSTGRES_INTENT_DSN")
	if dsn == "" {
		t.Skip("STELE_TEST_POSTGRES_INTENT_DSN is not configured")
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
	repo := NewRepository(pool)
	scope := memory.Scope{Tenant: "intent-it-" + uuid.NewString(), Project: "project", Namespace: "namespace"}
	input := memory.MemoryIntentInput{Scope: scope, Type: memory.MemoryIntentRemember, Content: "bounded integration fixture", Actor: "integration", Reason: "verify governed intent", RequestID: "request-1", OperationID: "operation-1", IdempotencyKey: "idempotency-1"}
	service := memory.MemoryIntentService{Processor: repo, Now: func() time.Time { return time.Now().UTC() }}
	first, err := service.Submit(ctx, input)
	if err != nil {
		t.Fatal(err)
	}
	if first.ID == "" || first.Status != memory.MemoryIntentStatusAccepted {
		t.Fatalf("first intent = %+v", first)
	}
	second, err := service.Submit(ctx, input)
	if err != nil {
		t.Fatal(err)
	}
	if second.ID != first.ID || second.Status != memory.MemoryIntentStatusReplayed {
		t.Fatalf("replay = %+v, want original id and replayed status", second)
	}
	conflict := input
	conflict.Content = "different fixture"
	if _, err := service.Submit(ctx, conflict); !errors.Is(err, memory.ErrIdempotencyConflict) {
		t.Fatalf("conflicting retry error = %v", err)
	}

	if err := repo.AppendMemoryIntentTransition(ctx, memory.MemoryIntentTransition{IntentID: first.ID, Scope: scope, Sequence: 2, From: memory.MemoryIntentStatusAccepted, To: memory.MemoryIntentStatusCandidate, Actor: "worker", Reason: "queued", DiagnosticCategory: memory.MemoryIntentDiagnosticAccepted, WorkReference: first.ID, OccurredAt: time.Now().UTC()}); err != nil {
		t.Fatal(err)
	}
	history, err := repo.ReadMemoryIntentHistory(ctx, scope, first.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(history.Transitions) != 2 || history.Transitions[1].Sequence != 2 {
		t.Fatalf("history = %+v", history)
	}
	foreign := memory.Scope{Tenant: "foreign-" + uuid.NewString(), Project: scope.Project, Namespace: scope.Namespace}
	if _, err := repo.ReadMemoryIntent(ctx, foreign, first.ID); !errors.Is(err, pgx.ErrNoRows) {
		t.Fatalf("foreign read error = %v, want not found", err)
	}
	status, err := repo.ReadDerivedWorkStatus(ctx, scope, time.Now().UTC())
	if err != nil {
		t.Fatal(err)
	}
	if status.Depth != 1 {
		t.Fatalf("durable queue status = %+v, want one queued item", status)
	}
	kind := workqueue.WorkKindMemoryIntent
	claims, err := repo.ClaimDerivedWork(ctx, workqueue.ClaimInput{Scope: scope, WorkerID: "intent-integration-worker", Now: time.Now().UTC(), LeaseDuration: time.Minute, Limit: 1, Kind: &kind})
	if err != nil {
		t.Fatal(err)
	}
	if len(claims) != 1 || claims[0].Reference != first.ID || claims[0].Kind != workqueue.WorkKindMemoryIntent {
		t.Fatalf("intent claims = %+v", claims)
	}
	if err := repo.CompleteDerivedWork(ctx, workqueue.TerminalInput{Scope: scope, WorkID: claims[0].ID, WorkerID: "intent-integration-worker", CompletedAt: time.Now().UTC(), Disposition: "completed", CheckpointSequence: 1, EvidenceReference: first.ID}); err != nil {
		t.Fatal(err)
	}
	modern := memory.MemoryIntentRecord{ID: uuid.NewString(), Scope: scope, MemoryPath: "agents/research", Type: memory.MemoryIntentFeedback, Actor: "integration", Reason: "verify extended projection", RequestID: "request-modern", OperationID: "operation-modern", IdempotencyKey: "idempotency-modern", TargetInsightID: "insight-1", Evidence: []memory.MemoryIntentEvidence{{Scope: scope, Kind: "insight", ID: "insight-1", Version: 1}}, PolicyVersion: "intent-v1", Status: memory.MemoryIntentStatusAccepted, CreatedAt: time.Now().UTC()}
	stored, err := repo.AppendMemoryIntent(ctx, modern)
	if err != nil {
		t.Fatal(err)
	}
	readBack, err := repo.ReadMemoryIntent(ctx, scope, stored.ID)
	if err != nil {
		t.Fatal(err)
	}
	if readBack.MemoryPath != modern.MemoryPath || readBack.TargetInsightID != modern.TargetInsightID || readBack.PolicyVersion != modern.PolicyVersion || len(readBack.Evidence) != 1 {
		t.Fatalf("extended intent projection = %+v", readBack)
	}
}
