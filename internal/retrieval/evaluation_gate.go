package retrieval

import (
	"net/url"
	"os"
	"strings"
)

const RetrievalEvaluationDSNSkip = "SKIP_RETRIEVAL_EVALUATION_DSN_REQUIRED"

// OwnedEvaluationDSN reads only the explicitly harness-owned evaluation DSN.
// Ambient runtime credentials are deliberately ignored.
func OwnedEvaluationDSN() (string, string) {
	dsn := strings.TrimSpace(os.Getenv("STELE_TEST_RETRIEVAL_EVALUATION_DSN"))
	if dsn == "" {
		return "", RetrievalEvaluationDSNSkip
	}
	if runtime := os.Getenv("STELE_POSTGRES_DSN"); runtime != "" && runtime == dsn {
		return "", "STELE_TEST_RETRIEVAL_EVALUATION_DSN_MUST_NOT_REUSE_RUNTIME_DSN"
	}
	u, err := url.Parse(dsn)
	if err != nil || (u.Scheme != "postgres" && u.Scheme != "postgresql") || u.Host == "" {
		return "", "STELE_TEST_RETRIEVAL_EVALUATION_DSN_INVALID"
	}
	return dsn, ""
}

type RetrievalEvaluationGate struct {
	RealStackRequired           bool
	Phase64Compatible           bool
	Phase64Passed               bool
	BaselineCandidateCompatible bool
}

func (g RetrievalEvaluationGate) ActiveEligible() bool {
	if _, reason := OwnedEvaluationDSN(); reason != "" {
		return false
	}
	return g.RealStackRequired && g.Phase64Compatible && g.Phase64Passed && g.BaselineCandidateCompatible
}

func (g RetrievalEvaluationGate) SkipReason() string {
	if _, reason := OwnedEvaluationDSN(); reason != "" {
		return reason
	}
	if !g.RealStackRequired || !g.Phase64Compatible || !g.Phase64Passed || !g.BaselineCandidateCompatible {
		return "RETRIEVAL_EVALUATION_GATE_NOT_PASSED"
	}
	return ""
}
