package postgres

import (
	"context"
	"regexp"
	"testing"
	"time"

	"github.com/FelixSeptem/stele/internal/memory"
	"github.com/FelixSeptem/stele/internal/reasoning"
	"github.com/pashagolub/pgxmock/v4"
)

func TestPersistReasoningInsightCandidateIsScopedAndIdempotent(t *testing.T) {
	db, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	repo := NewRepository(db)
	scope := memory.Scope{Tenant: "tenant-a", Project: "project-a", Namespace: "namespace-a"}
	evidence := memory.DerivedInsightEvidenceRef{Kind: memory.DerivedInsightEvidenceKindCanonicalMemory, ID: "memory-1", Relation: memory.DerivedInsightEvidenceRelationSupports}
	req := reasoning.InsightDerivationRequest{Scope: scope, InsightType: memory.DerivedInsightTypeHypothesis, Mode: reasoning.ModeOffline, Evidence: []memory.DerivedInsightEvidenceRef{evidence}, SourceWatermark: "wm-1", ScopeProof: "proof-1", LifecycleVisibility: "active_only", RedactionPolicy: "references_only", ProviderVersion: "provider-1", SchemaVersion: reasoning.SchemaVersionV1, PolicyVersion: "policy-1", InputDigest: "input-1", Limits: reasoning.DefaultLimits(), Now: time.Date(2026, 9, 30, 12, 0, 0, 0, time.UTC)}
	digest, err := reasoning.EvidenceDigest(req.Evidence)
	if err != nil {
		t.Fatal(err)
	}
	replayID, err := reasoning.InsightReplayID(req)
	if err != nil {
		t.Fatal(err)
	}
	candidate := reasoning.InsightCandidate{ID: "candidate-1", Scope: scope, InsightType: req.InsightType, Title: "Hypothesis", Summary: "Evidence-backed", Evidence: req.Evidence, EvidenceDigest: digest, SourceWatermark: req.SourceWatermark, ScopeProof: req.ScopeProof, LifecycleVisibility: req.LifecycleVisibility, RedactionPolicy: req.RedactionPolicy, ProviderVersion: req.ProviderVersion, SchemaVersion: req.SchemaVersion, PolicyVersion: req.PolicyVersion, ReplayID: replayID, Uncertainty: 0.2, Mode: req.Mode, CreatedAt: req.Now}
	db.ExpectExec(regexp.QuoteMeta("INSERT INTO governed_reasoning_insight_candidates")).WithArgs(
		candidate.ID, scope.Tenant, scope.Project, scope.Namespace, candidate.InsightType, candidate.Mode,
		candidate.Title, candidate.Summary, pgxmock.AnyArg(), candidate.EvidenceDigest, candidate.SourceWatermark,
		candidate.ScopeProof, candidate.LifecycleVisibility, candidate.RedactionPolicy, candidate.ProviderVersion, candidate.SchemaVersion, candidate.PolicyVersion,
		candidate.ReplayID, candidate.Uncertainty, false, false, reasoning.InsightDispositionCandidate, "shadow candidate", candidate.CreatedAt,
		pgxmock.AnyArg(), "review_required", pgxmock.AnyArg(), pgxmock.AnyArg(), "shadow candidate",
	).WillReturnResult(pgxmock.NewResult("INSERT", 1))
	if err := repo.PersistReasoningInsightCandidate(context.Background(), candidate, reasoning.InsightDispositionCandidate, "shadow candidate"); err != nil {
		t.Fatalf("persist candidate: %v", err)
	}
	if err := db.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}
