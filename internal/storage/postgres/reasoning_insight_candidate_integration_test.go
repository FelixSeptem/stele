package postgres

import (
	"context"
	"encoding/json"
	"os"
	"testing"
	"time"

	"github.com/FelixSeptem/stele/internal/memory"
	"github.com/FelixSeptem/stele/internal/reasoning"
	"github.com/google/uuid"
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

func TestGovernedGoalInsightPostgresPgvectorConformanceMatrix(t *testing.T) {
	dsn := os.Getenv("STELE_TEST_POSTGRES_GOAL_DSN")
	if dsn == "" {
		t.Skip("STELE_TEST_POSTGRES_GOAL_DSN is not configured; skipping governed goal PostgreSQL + pgvector conformance")
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
	scope := memory.Scope{Tenant: "goal-conformance-" + time.Now().UTC().Format("20060102150405.000000000"), Project: "goal-project", Namespace: "goal-namespace"}
	defer pool.Exec(ctx, `DELETE FROM derived_insight_evidence WHERE tenant = $1`, scope.Tenant)
	defer pool.Exec(ctx, `DELETE FROM derived_insights WHERE tenant = $1`, scope.Tenant)
	defer pool.Exec(ctx, `DELETE FROM governed_reasoning_insight_candidates WHERE tenant = $1`, scope.Tenant)
	now := time.Now().UTC().Truncate(time.Microsecond)
	validTo := now.Add(24 * time.Hour)
	evidence := memory.DerivedInsightEvidenceRef{Kind: memory.DerivedInsightEvidenceKindCanonicalMemory, ID: "goal-evidence-1", Relation: memory.DerivedInsightEvidenceRelationSupports}
	goal := &reasoning.GoalMetadata{State: reasoning.GoalStateProposed, ReviewState: reasoning.GoalReviewRequired, ValidFrom: &now, ValidTo: &validTo}
	request := reasoning.InsightDerivationRequest{Scope: scope, InsightType: memory.DerivedInsightTypeGoal, Mode: reasoning.ModeShadow, Evidence: []memory.DerivedInsightEvidenceRef{evidence}, SourceWatermark: "goal-wm-1", ScopeProof: "goal-proof-1", LifecycleVisibility: "active_only", RedactionPolicy: "references_only", ProviderVersion: "goal-provider-1", SchemaVersion: reasoning.SchemaVersionV1, PolicyVersion: "goal-policy-v1", InputDigest: "goal-input-1", Limits: reasoning.DefaultLimits(), Now: now, Goal: goal}
	digest, err := reasoning.EvidenceDigest(request.Evidence)
	if err != nil {
		t.Fatal(err)
	}
	replayID, err := reasoning.InsightReplayID(request)
	if err != nil {
		t.Fatal(err)
	}
	candidate := reasoning.InsightCandidate{ID: "goal-candidate-" + uuid.NewString(), Scope: scope, InsightType: request.InsightType, Title: "Conformance goal", Summary: "Review only", Evidence: request.Evidence, EvidenceDigest: digest, SourceWatermark: request.SourceWatermark, ScopeProof: request.ScopeProof, LifecycleVisibility: request.LifecycleVisibility, RedactionPolicy: request.RedactionPolicy, ProviderVersion: request.ProviderVersion, SchemaVersion: request.SchemaVersion, PolicyVersion: request.PolicyVersion, ReplayID: replayID, Uncertainty: 0.2, Mode: request.Mode, CreatedAt: now, Goal: goal}
	repo := NewRepository(pool)
	for i := 0; i < 2; i++ {
		if err := repo.PersistReasoningInsightCandidate(ctx, candidate, reasoning.InsightDispositionWouldActivate, "review required"); err != nil {
			t.Fatal(err)
		}
	}
	var candidateCount int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM governed_reasoning_insight_candidates WHERE tenant = $1 AND project = $2 AND namespace = $3 AND replay_id = $4`, scope.Tenant, scope.Project, scope.Namespace, replayID).Scan(&candidateCount); err != nil {
		t.Fatal(err)
	}
	if candidateCount != 1 {
		t.Fatalf("candidate count = %d, want idempotent single candidate", candidateCount)
	}
	var metadataJSON []byte
	if err := pool.QueryRow(ctx, `SELECT goal_metadata FROM governed_reasoning_insight_candidates WHERE tenant = $1 AND project = $2 AND namespace = $3 AND replay_id = $4`, scope.Tenant, scope.Project, scope.Namespace, replayID).Scan(&metadataJSON); err != nil {
		t.Fatal(err)
	}
	var persistedGoal reasoning.GoalMetadata
	if err := json.Unmarshal(metadataJSON, &persistedGoal); err != nil {
		t.Fatal(err)
	}
	if persistedGoal.State != reasoning.GoalStateProposed || persistedGoal.ReviewState != reasoning.GoalReviewRequired {
		t.Fatalf("persisted goal metadata = %+v, want proposed/review_required", persistedGoal)
	}
	offlineRequest := request
	offlineRequest.Mode = reasoning.ModeOffline
	offlineRequest.InputDigest = "goal-offline-input-1"
	offlineReplayID, err := reasoning.InsightReplayID(offlineRequest)
	if err != nil {
		t.Fatal(err)
	}
	offlineCandidate := candidate
	offlineCandidate.ID = "goal-offline-candidate-" + uuid.NewString()
	offlineCandidate.Mode = reasoning.ModeOffline
	offlineCandidate.ReplayID = offlineReplayID
	if err := repo.PersistReasoningInsightCandidate(ctx, offlineCandidate, reasoning.InsightDispositionCandidate, "offline candidate"); err != nil {
		t.Fatal(err)
	}
	rolledBackRequest := request
	rolledBackRequest.PolicyVersion = "goal-policy-rolled-back"
	rolledBackReplayID, err := reasoning.InsightReplayID(rolledBackRequest)
	if err != nil {
		t.Fatal(err)
	}
	rolledBackCandidate := candidate
	rolledBackCandidate.ID = "goal-rollback-candidate-" + uuid.NewString()
	rolledBackCandidate.PolicyVersion = rolledBackRequest.PolicyVersion
	rolledBackCandidate.ReplayID = rolledBackReplayID
	if err := repo.PersistReasoningInsightCandidate(ctx, rolledBackCandidate, reasoning.InsightDispositionRejected, "goal policy disabled after rollback"); err != nil {
		t.Fatal(err)
	}
	var rollbackDisposition string
	if err := pool.QueryRow(ctx, `SELECT disposition FROM governed_reasoning_insight_candidates WHERE tenant = $1 AND project = $2 AND namespace = $3 AND replay_id = $4`, scope.Tenant, scope.Project, scope.Namespace, rolledBackReplayID).Scan(&rollbackDisposition); err != nil {
		t.Fatal(err)
	}
	if rollbackDisposition != string(reasoning.InsightDispositionRejected) {
		t.Fatalf("rollback disposition = %q, want rejected", rollbackDisposition)
	}
	restartedPool, err := OpenPool(ctx, dsn)
	if err != nil {
		t.Fatal(err)
	}
	var restartedCount int
	if err := restartedPool.QueryRow(ctx, `SELECT count(*) FROM governed_reasoning_insight_candidates WHERE tenant = $1 AND project = $2 AND namespace = $3`, scope.Tenant, scope.Project, scope.Namespace).Scan(&restartedCount); err != nil {
		restartedPool.Close()
		t.Fatal(err)
	}
	restartedPool.Close()
	if restartedCount != 3 {
		t.Fatalf("restart candidate count = %d, want three durable dispositions", restartedCount)
	}
	var canonicalBefore, canonicalAfter int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM canonical_memories WHERE tenant = $1 AND project = $2 AND namespace = $3`, scope.Tenant, scope.Project, scope.Namespace).Scan(&canonicalBefore); err != nil {
		t.Fatal(err)
	}
	goalInsight := memory.DerivedInsight{ID: uuid.NewString(), Scope: scope, Type: memory.DerivedInsightTypeGoal, State: memory.DerivedInsightStateCandidate, Title: "Conformance goal", Summary: "Review only", Confidence: memory.DerivedInsightConfidence{Score: 0.8, Method: "bounded"}, Payload: map[string]any{"goal_state": "proposed"}, Derivation: memory.DerivedInsightDerivation{Source: "goal-conformance", Fingerprint: "goal:" + replayID, Metadata: map[string]any{"goal_state": "proposed", "goal_review_state": "review_required", "activation_decision": "review_required"}, DerivedAt: now}, Evidence: []memory.DerivedInsightEvidenceRef{evidence}, CreatedAt: now, UpdatedAt: now, LastObservedAt: now}
	if _, err := repo.UpsertDerivedInsight(ctx, goalInsight); err != nil {
		t.Fatal(err)
	}
	visible, err := repo.ListDerivedInsights(ctx, memory.ListDerivedInsightsInput{Scope: scope, Limit: 10})
	if err != nil {
		t.Fatal(err)
	}
	if len(visible) != 0 {
		t.Fatalf("default derived insight visibility = %+v, want goal omitted", visible)
	}
	experimental, err := repo.ListDerivedInsights(ctx, memory.ListDerivedInsightsInput{Scope: scope, Type: memory.DerivedInsightTypeGoal, IncludeGoals: true, Limit: 10})
	if err != nil {
		t.Fatal(err)
	}
	if len(experimental) != 1 || experimental[0].Type != memory.DerivedInsightTypeGoal {
		t.Fatalf("explicit goal visibility = %+v, want one goal", experimental)
	}
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM canonical_memories WHERE tenant = $1 AND project = $2 AND namespace = $3`, scope.Tenant, scope.Project, scope.Namespace).Scan(&canonicalAfter); err != nil {
		t.Fatal(err)
	}
	if canonicalBefore != canonicalAfter {
		t.Fatalf("canonical memory count changed from %d to %d", canonicalBefore, canonicalAfter)
	}
}
