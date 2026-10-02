package postgres

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/FelixSeptem/stele/internal/memory"
	"github.com/FelixSeptem/stele/internal/reasoning"
)

func TestReasoningInsightCandidatePostgresPgvectorIntegration(t *testing.T) {
	dsn := os.Getenv("STELE_TEST_POSTGRES_REASONING_DSN")
	if dsn == "" {
		t.Skip("STELE_TEST_POSTGRES_REASONING_DSN is not configured; skipping PostgreSQL + pgvector reasoning integration test")
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
	var vectorVersion string
	if err := pool.QueryRow(ctx, `SELECT extversion FROM pg_extension WHERE extname = 'vector'`).Scan(&vectorVersion); err != nil {
		t.Fatalf("pgvector extension unavailable: %v", err)
	}
	if vectorVersion == "" {
		t.Fatal("pgvector extension version is empty")
	}
	scope := memory.Scope{Tenant: "reasoning-it-" + time.Now().UTC().Format("20060102150405.000000000"), Project: "p", Namespace: "n"}
	defer pool.Exec(ctx, `DELETE FROM governed_reasoning_insight_candidates WHERE tenant = $1`, scope.Tenant)
	now := time.Now().UTC().Truncate(time.Microsecond)
	evidence := memory.DerivedInsightEvidenceRef{Kind: memory.DerivedInsightEvidenceKindCanonicalMemory, ID: "memory-1", Relation: memory.DerivedInsightEvidenceRelationSupports}
	request := reasoning.InsightDerivationRequest{Scope: scope, InsightType: memory.DerivedInsightTypeHypothesis, Mode: reasoning.ModeShadow, Evidence: []memory.DerivedInsightEvidenceRef{evidence}, SourceWatermark: "wm-1", ScopeProof: "proof-1", LifecycleVisibility: "active_only", RedactionPolicy: "references_only", ProviderVersion: "provider-1", SchemaVersion: reasoning.SchemaVersionV1, PolicyVersion: "policy-1", InputDigest: "input-1", Limits: reasoning.DefaultLimits(), Now: now}
	digest, _ := reasoning.EvidenceDigest(request.Evidence)
	replayID, _ := reasoning.InsightReplayID(request)
	candidate := reasoning.InsightCandidate{ID: "candidate-" + scope.Tenant, Scope: scope, InsightType: request.InsightType, Title: "Hypothesis", Summary: "Evidence-backed", Evidence: request.Evidence, EvidenceDigest: digest, SourceWatermark: request.SourceWatermark, ScopeProof: request.ScopeProof, LifecycleVisibility: request.LifecycleVisibility, RedactionPolicy: request.RedactionPolicy, ProviderVersion: request.ProviderVersion, SchemaVersion: request.SchemaVersion, PolicyVersion: request.PolicyVersion, ReplayID: replayID, Uncertainty: 0.2, Mode: request.Mode, CreatedAt: now}
	if err := NewRepository(pool).PersistReasoningInsightCandidate(ctx, candidate, reasoning.InsightDispositionWouldActivate, "shadow policy result"); err != nil {
		t.Fatal(err)
	}
	var disposition string
	if err := pool.QueryRow(ctx, `SELECT disposition FROM governed_reasoning_insight_candidates WHERE tenant = $1 AND replay_id = $2`, scope.Tenant, replayID).Scan(&disposition); err != nil {
		t.Fatal(err)
	}
	if disposition != string(reasoning.InsightDispositionWouldActivate) {
		t.Fatalf("disposition = %q, want would_activate", disposition)
	}
}
