package retrieval

import (
	"math"
	"net/url"
	"os"
	"strings"
	"time"
)

const (
	RetrievalEvaluationDSNSkip                       = "SKIP_RETRIEVAL_EVALUATION_DSN_REQUIRED"
	RetrievalEvaluationPhase64                       = "phase_6_4"
	RetrievalEvaluationPhase64EvidenceRequired       = "RETRIEVAL_EVALUATION_PHASE_6_4_EVIDENCE_REQUIRED"
	RetrievalEvaluationPhase64Skipped                = "RETRIEVAL_EVALUATION_PHASE_6_4_SKIPPED"
	RetrievalEvaluationPhase64Failed                 = "RETRIEVAL_EVALUATION_PHASE_6_4_FAILED"
	RetrievalEvaluationPhase64Incompatible           = "RETRIEVAL_EVALUATION_PHASE_6_4_INCOMPATIBLE"
	RetrievalEvaluationPhase64Stale                  = "RETRIEVAL_EVALUATION_PHASE_6_4_STALE"
	RetrievalEvaluationPhase64NotRealStack           = "RETRIEVAL_EVALUATION_PHASE_6_4_NOT_REAL_STACK"
	RetrievalEvaluationBaselineCandidateIncompatible = "RETRIEVAL_EVALUATION_BASELINE_CANDIDATE_INCOMPATIBLE"
)

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

type RetrievalEvaluationEvidenceStatus string

const (
	RetrievalEvaluationEvidencePassed  RetrievalEvaluationEvidenceStatus = "passed"
	RetrievalEvaluationEvidenceFailed  RetrievalEvaluationEvidenceStatus = "failed"
	RetrievalEvaluationEvidenceSkipped RetrievalEvaluationEvidenceStatus = "skipped"
)

// RetrievalEvaluationPrerequisiteMetrics carries the measured Phase 6.4
// prerequisite and the thresholds applied to that measurement. It contains no
// queries, candidates, identifiers, credentials, or connection details.
type RetrievalEvaluationPrerequisiteMetrics struct {
	DuplicateRate        float64 `json:"duplicate_rate"`
	MaxDuplicateRate     float64 `json:"max_duplicate_rate"`
	ProtectedCoverage    float64 `json:"protected_coverage"`
	MinProtectedCoverage float64 `json:"min_protected_coverage"`
	CandidatePoolSize    int     `json:"candidate_pool_size"`
	MaxCandidatePoolSize int     `json:"max_candidate_pool_size"`
	P95LatencyMS         float64 `json:"p95_latency_ms"`
	MaxP95LatencyMS      float64 `json:"max_p95_latency_ms"`
}

func (m RetrievalEvaluationPrerequisiteMetrics) passed() bool {
	if !boundedRate(m.DuplicateRate) || !boundedRate(m.MaxDuplicateRate) || m.DuplicateRate > m.MaxDuplicateRate {
		return false
	}
	if !boundedRate(m.ProtectedCoverage) || !boundedRate(m.MinProtectedCoverage) || m.ProtectedCoverage < m.MinProtectedCoverage {
		return false
	}
	if m.CandidatePoolSize < 0 || m.MaxCandidatePoolSize <= 0 || m.CandidatePoolSize > m.MaxCandidatePoolSize {
		return false
	}
	return !math.IsNaN(m.P95LatencyMS) && !math.IsInf(m.P95LatencyMS, 0) && m.P95LatencyMS >= 0 &&
		!math.IsNaN(m.MaxP95LatencyMS) && !math.IsInf(m.MaxP95LatencyMS, 0) && m.MaxP95LatencyMS > 0 &&
		m.P95LatencyMS <= m.MaxP95LatencyMS
}

func boundedRate(value float64) bool {
	return !math.IsNaN(value) && !math.IsInf(value, 0) && value >= 0 && value <= 1
}

// RetrievalEvaluationPrerequisiteEvidence is the bounded, redacted evidence
// required before query analysis can advance to active decomposition.
type RetrievalEvaluationPrerequisiteEvidence struct {
	Stage       string                                 `json:"stage"`
	Status      RetrievalEvaluationEvidenceStatus      `json:"status"`
	RealStack   bool                                   `json:"real_stack"`
	Metadata    EvaluationRankingMetadata              `json:"metadata"`
	GeneratedAt time.Time                              `json:"generated_at"`
	ExpiresAt   time.Time                              `json:"expires_at"`
	Metrics     RetrievalEvaluationPrerequisiteMetrics `json:"metrics"`
}

type RetrievalEvaluationGate struct {
	Now                         time.Time
	CandidateMetadata           EvaluationRankingMetadata
	Phase64Evidence             *RetrievalEvaluationPrerequisiteEvidence
	BaselineCandidateCompatible bool
}

func (g RetrievalEvaluationGate) ActiveEligible() bool {
	return g.SkipReason() == ""
}

func (g RetrievalEvaluationGate) SkipReason() string {
	if _, reason := OwnedEvaluationDSN(); reason != "" {
		return reason
	}
	evidence := g.Phase64Evidence
	if evidence == nil {
		return RetrievalEvaluationPhase64EvidenceRequired
	}
	switch evidence.Status {
	case RetrievalEvaluationEvidenceSkipped:
		return RetrievalEvaluationPhase64Skipped
	case RetrievalEvaluationEvidenceFailed:
		return RetrievalEvaluationPhase64Failed
	case RetrievalEvaluationEvidencePassed:
	default:
		return RetrievalEvaluationPhase64Failed
	}
	if !evidence.RealStack {
		return RetrievalEvaluationPhase64NotRealStack
	}
	if evidence.Stage != RetrievalEvaluationPhase64 || evidence.Metadata.Validate() != nil || g.CandidateMetadata.Validate() != nil || !retrievalEvaluationCoreMetadataCompatible(evidence.Metadata, g.CandidateMetadata) {
		return RetrievalEvaluationPhase64Incompatible
	}
	now := g.Now
	if now.IsZero() {
		now = time.Now().UTC()
	}
	if evidence.GeneratedAt.IsZero() || evidence.ExpiresAt.IsZero() || evidence.GeneratedAt.After(now) || !evidence.ExpiresAt.After(now) || !evidence.ExpiresAt.After(evidence.GeneratedAt) {
		return RetrievalEvaluationPhase64Stale
	}
	if !evidence.Metrics.passed() {
		return RetrievalEvaluationPhase64Failed
	}
	if !g.BaselineCandidateCompatible {
		return RetrievalEvaluationBaselineCandidateIncompatible
	}
	return ""
}

func retrievalEvaluationCoreMetadataCompatible(prerequisite, candidate EvaluationRankingMetadata) bool {
	return strings.TrimSpace(prerequisite.FusionStrategy) != "" &&
		strings.TrimSpace(candidate.FusionStrategy) != "" &&
		prerequisite.FixtureVersion == candidate.FixtureVersion &&
		prerequisite.RepresentationVersion == candidate.RepresentationVersion &&
		prerequisite.RankingVersion == candidate.RankingVersion &&
		prerequisite.FusionStrategy == candidate.FusionStrategy &&
		prerequisite.CompatibleEmbeddingRevision == candidate.CompatibleEmbeddingRevision &&
		prerequisite.PolicyVersion == candidate.PolicyVersion
}
