package retrieval

import (
	"testing"
	"time"
)

func TestOwnedEvaluationDSNNeverFallsBackToRuntime(t *testing.T) {
	t.Setenv("STELE_TEST_RETRIEVAL_EVALUATION_DSN", "")
	t.Setenv("STELE_POSTGRES_DSN", "postgres://runtime")
	if _, reason := OwnedEvaluationDSN(); reason != RetrievalEvaluationDSNSkip {
		t.Fatalf("reason=%q", reason)
	}
}

func TestOwnedEvaluationGateNeverEligibleWithoutExplicitDSN(t *testing.T) {
	t.Setenv("STELE_TEST_RETRIEVAL_EVALUATION_DSN", "")
	g := validRetrievalEvaluationGate(time.Date(2026, 9, 11, 12, 0, 0, 0, time.UTC))
	if g.ActiveEligible() {
		t.Fatal("gate unexpectedly eligible without owned evaluation DSN")
	}
}

func TestOwnedEvaluationDSNRejectsMalformedValue(t *testing.T) {
	t.Setenv("STELE_TEST_RETRIEVAL_EVALUATION_DSN", "not-a-dsn")
	if _, reason := OwnedEvaluationDSN(); reason != "STELE_TEST_RETRIEVAL_EVALUATION_DSN_INVALID" {
		t.Fatalf("reason=%q", reason)
	}
}

func TestRetrievalEvaluationGateAcceptsCompatibleFreshPhase64Evidence(t *testing.T) {
	t.Setenv("STELE_TEST_RETRIEVAL_EVALUATION_DSN", "postgres://owned/evaluation")
	gate := validRetrievalEvaluationGate(time.Date(2026, 9, 11, 12, 0, 0, 0, time.UTC))
	if !gate.ActiveEligible() || gate.SkipReason() != "" {
		t.Fatalf("gate eligible=%v reason=%q, want eligible", gate.ActiveEligible(), gate.SkipReason())
	}
}

func TestRetrievalEvaluationGateRejectsAbsentSkippedIncompatibleStaleOrFailingPhase64Evidence(t *testing.T) {
	t.Setenv("STELE_TEST_RETRIEVAL_EVALUATION_DSN", "postgres://owned/evaluation")
	now := time.Date(2026, 9, 11, 12, 0, 0, 0, time.UTC)
	tests := []struct {
		name   string
		mutate func(*RetrievalEvaluationGate)
		reason string
	}{
		{name: "absent", mutate: func(g *RetrievalEvaluationGate) { g.Phase64Evidence = nil }, reason: RetrievalEvaluationPhase64EvidenceRequired},
		{name: "skipped", mutate: func(g *RetrievalEvaluationGate) { g.Phase64Evidence.Status = RetrievalEvaluationEvidenceSkipped }, reason: RetrievalEvaluationPhase64Skipped},
		{name: "incompatible", mutate: func(g *RetrievalEvaluationGate) { g.Phase64Evidence.Metadata.RankingVersion = "ranking-v2" }, reason: RetrievalEvaluationPhase64Incompatible},
		{name: "missing fusion identity", mutate: func(g *RetrievalEvaluationGate) { g.Phase64Evidence.Metadata.FusionStrategy = "" }, reason: RetrievalEvaluationPhase64Incompatible},
		{name: "stale", mutate: func(g *RetrievalEvaluationGate) { g.Phase64Evidence.ExpiresAt = now.Add(-time.Nanosecond) }, reason: RetrievalEvaluationPhase64Stale},
		{name: "explicit failure", mutate: func(g *RetrievalEvaluationGate) { g.Phase64Evidence.Status = RetrievalEvaluationEvidenceFailed }, reason: RetrievalEvaluationPhase64Failed},
		{name: "duplicate rate", mutate: func(g *RetrievalEvaluationGate) { g.Phase64Evidence.Metrics.DuplicateRate = 0.2 }, reason: RetrievalEvaluationPhase64Failed},
		{name: "protected coverage", mutate: func(g *RetrievalEvaluationGate) { g.Phase64Evidence.Metrics.ProtectedCoverage = 0.9 }, reason: RetrievalEvaluationPhase64Failed},
		{name: "candidate budget", mutate: func(g *RetrievalEvaluationGate) { g.Phase64Evidence.Metrics.CandidatePoolSize = 201 }, reason: RetrievalEvaluationPhase64Failed},
		{name: "latency", mutate: func(g *RetrievalEvaluationGate) { g.Phase64Evidence.Metrics.P95LatencyMS = 251 }, reason: RetrievalEvaluationPhase64Failed},
		{name: "synthetic", mutate: func(g *RetrievalEvaluationGate) { g.Phase64Evidence.RealStack = false }, reason: RetrievalEvaluationPhase64NotRealStack},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			gate := validRetrievalEvaluationGate(now)
			test.mutate(&gate)
			if gate.ActiveEligible() {
				t.Fatal("gate unexpectedly eligible")
			}
			if reason := gate.SkipReason(); reason != test.reason {
				t.Fatalf("reason=%q, want %q", reason, test.reason)
			}
		})
	}
}

func validRetrievalEvaluationGate(now time.Time) RetrievalEvaluationGate {
	metadata := EvaluationRankingMetadata{
		FixtureVersion:              "retrieval-fixture-v1",
		RepresentationVersion:       "retrieval-representation-v1",
		RankingVersion:              "ranking-v1",
		FusionStrategy:              "rrf:rrf-v1",
		CompatibleEmbeddingRevision: "embedding-v1",
		PolicyVersion:               "retrieval-release-v1",
	}
	return RetrievalEvaluationGate{
		Now:                         now,
		CandidateMetadata:           metadata,
		BaselineCandidateCompatible: true,
		Phase64Evidence: &RetrievalEvaluationPrerequisiteEvidence{
			Stage:       RetrievalEvaluationPhase64,
			Status:      RetrievalEvaluationEvidencePassed,
			RealStack:   true,
			Metadata:    metadata,
			GeneratedAt: now.Add(-time.Hour),
			ExpiresAt:   now.Add(time.Hour),
			Metrics: RetrievalEvaluationPrerequisiteMetrics{
				DuplicateRate:        0.05,
				MaxDuplicateRate:     0.1,
				ProtectedCoverage:    1,
				MinProtectedCoverage: 1,
				CandidatePoolSize:    200,
				MaxCandidatePoolSize: 200,
				P95LatencyMS:         250,
				MaxP95LatencyMS:      250,
			},
		},
	}
}
