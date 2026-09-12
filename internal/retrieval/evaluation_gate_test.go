package retrieval

import "testing"

func TestOwnedEvaluationDSNNeverFallsBackToRuntime(t *testing.T) {
	t.Setenv("STELE_TEST_RETRIEVAL_EVALUATION_DSN", "")
	t.Setenv("STELE_POSTGRES_DSN", "postgres://runtime")
	if _, reason := OwnedEvaluationDSN(); reason != RetrievalEvaluationDSNSkip {
		t.Fatalf("reason=%q", reason)
	}
}

func TestOwnedEvaluationGateRequiresRealStackAndPrerequisite(t *testing.T) {
	t.Setenv("STELE_TEST_RETRIEVAL_EVALUATION_DSN", "postgres://owned")
	g := RetrievalEvaluationGate{RealStackRequired: true, Phase64Compatible: true, Phase64Passed: false, BaselineCandidateCompatible: true}
	if g.ActiveEligible() {
		t.Fatal("gate unexpectedly eligible")
	}
	if g.SkipReason() == "" {
		t.Fatal("missing skip reason")
	}
}

func TestOwnedEvaluationGateNeverEligibleWithoutExplicitDSN(t *testing.T) {
	t.Setenv("STELE_TEST_RETRIEVAL_EVALUATION_DSN", "")
	g := RetrievalEvaluationGate{RealStackRequired: true, Phase64Compatible: true, Phase64Passed: true, BaselineCandidateCompatible: true}
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
