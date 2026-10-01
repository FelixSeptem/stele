package retrieval

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/FelixSeptem/stele/internal/evaluation"
	"github.com/FelixSeptem/stele/internal/memory"
)

const (
	ReleaseEvidenceSkipDSN                  = RetrievalEvaluationDSNSkip
	ReleaseEvidenceSkipPrerequisite         = "RETRIEVAL_RELEASE_EVIDENCE_PREREQUISITE_REQUIRED"
	ReleaseEvidenceIncompatible             = "RETRIEVAL_RELEASE_EVIDENCE_INCOMPATIBLE"
	ReleaseEvidenceSafetyFailure            = "RETRIEVAL_RELEASE_EVIDENCE_SAFETY_FAILURE"
	ReleaseEvidenceProgressiveFailure       = "RETRIEVAL_RELEASE_EVIDENCE_PROGRESSIVE_FAILURE"
	ReleaseEvidenceParentFirstFailure       = "RETRIEVAL_RELEASE_EVIDENCE_PARENT_FIRST_FAILURE"
	ReleaseEvidenceRollbackFailure          = "RETRIEVAL_RELEASE_EVIDENCE_ROLLBACK_FAILURE"
	ReleaseEvidenceQualityFailure           = "RETRIEVAL_RELEASE_EVIDENCE_QUALITY_FAILURE"
	ReleaseEvidenceTemporalEvidenceRequired = "RETRIEVAL_RELEASE_EVIDENCE_TEMPORAL_REQUIRED"
	ReleaseEvidenceStaleCategory            = "RETRIEVAL_RELEASE_EVIDENCE_STALE"
	ReleaseEvidenceSemanticHitRequired      = "RETRIEVAL_RELEASE_EVIDENCE_SEMANTIC_HIT_REQUIRED"
	ReleaseEvidenceTrajectoryRequired       = "RETRIEVAL_RELEASE_EVIDENCE_TRAJECTORY_REQUIRED"
	ReleaseEvidenceResourceFailure          = "RETRIEVAL_RELEASE_EVIDENCE_RESOURCE_FAILURE"
)

type ReleaseEvidenceVerdict string

const (
	ReleaseEvidencePassed   ReleaseEvidenceVerdict = "passed"
	ReleaseEvidenceSkipped  ReleaseEvidenceVerdict = "skipped"
	ReleaseEvidenceDegraded ReleaseEvidenceVerdict = "degraded"
	ReleaseEvidenceRejected ReleaseEvidenceVerdict = "rejected"
)

type ReleaseEvidenceFreshness string

const (
	ReleaseEvidenceFresh   ReleaseEvidenceFreshness = "fresh"
	ReleaseEvidenceStale   ReleaseEvidenceFreshness = "stale"
	ReleaseEvidenceUnknown ReleaseEvidenceFreshness = "unknown"
)

type ReleaseEvidencePrerequisites struct {
	EvaluationDSN        string
	RuntimeDSN           string
	PostgreSQLReady      bool
	PGVectorReady        bool
	FixtureCompatible    bool
	ProjectionFresh      bool
	RollbackTested       bool
	SemanticHitProven    bool
	TrajectoryCompatible bool
	ResourceWithinBounds bool
}

func (p ReleaseEvidencePrerequisites) Validate() error {
	dsn := strings.TrimSpace(p.EvaluationDSN)
	if dsn == "" {
		return fmt.Errorf("%s", ReleaseEvidenceSkipDSN)
	}
	if strings.TrimSpace(p.RuntimeDSN) != "" && dsn == strings.TrimSpace(p.RuntimeDSN) {
		return fmt.Errorf("evaluation DSN must not reuse runtime DSN")
	}
	u, err := url.Parse(dsn)
	if err != nil || (u.Scheme != "postgres" && u.Scheme != "postgresql") || u.Host == "" {
		return fmt.Errorf("invalid evaluation DSN")
	}
	if !p.PostgreSQLReady || !p.PGVectorReady || !p.FixtureCompatible || !p.ProjectionFresh || !p.RollbackTested {
		return fmt.Errorf("%s", ReleaseEvidenceSkipPrerequisite)
	}
	return nil
}

type ReleaseEvidenceInput struct {
	Scope                memory.Scope
	ProviderProfile      string
	Policy               EvaluationReleasePolicy
	Baseline             EvaluationReport
	Candidate            EvaluationReport
	Progressive          ProgressiveContextEvaluationReport
	ParentFirst          ParentFirstEvaluationReport
	Prerequisites        ReleaseEvidencePrerequisites
	Integrity            *evaluation.IntegrityReport
	EvaluatedAt          time.Time
	SourceWatermark      string
	EvidenceExpiresAt    time.Time
	DeterministicReplay  bool
	SemanticHitProven    bool
	TrajectoryCompatible bool
	ResourceWithinBounds bool
}

// ReleaseEvidenceRunRequest is the explicit, bounded input accepted by the
// release-evidence runner. DSNs are consumed for validation only and are never
// copied into the resulting report.
type ReleaseEvidenceRunRequest struct {
	Scope                memory.Scope
	EvaluationDSN        string
	RuntimeDSN           string
	ProviderProfile      string
	Policy               EvaluationReleasePolicy
	Baseline             EvaluationReport
	Candidate            EvaluationReport
	Progressive          ProgressiveContextEvaluationReport
	ParentFirst          ParentFirstEvaluationReport
	PostgreSQLReady      bool
	PGVectorReady        bool
	FixtureCompatible    bool
	ProjectionFresh      bool
	RollbackTested       bool
	Integrity            *evaluation.IntegrityReport
	EvaluatedAt          time.Time
	SourceWatermark      string
	EvidenceExpiresAt    time.Time
	DeterministicReplay  bool
	SemanticHitProven    bool
	TrajectoryCompatible bool
	ResourceWithinBounds bool
}

// RunOwnedReleaseEvidence evaluates one exact scope using explicitly owned
// evidence. The context is reserved for future real-stack adapters; this
// function performs no canonical writes and never falls back to RuntimeDSN.
func RunOwnedReleaseEvidence(_ context.Context, req ReleaseEvidenceRunRequest) (ReleaseEvidenceReport, error) {
	return EvaluateReleaseEvidence(ReleaseEvidenceInput{
		Scope: req.Scope, ProviderProfile: req.ProviderProfile, Policy: req.Policy,
		Baseline: req.Baseline, Candidate: req.Candidate, Progressive: req.Progressive,
		ParentFirst: req.ParentFirst, EvaluatedAt: req.EvaluatedAt,
		Integrity:       req.Integrity,
		SourceWatermark: req.SourceWatermark, EvidenceExpiresAt: req.EvidenceExpiresAt,
		DeterministicReplay: req.DeterministicReplay,
		SemanticHitProven:   req.SemanticHitProven, TrajectoryCompatible: req.TrajectoryCompatible,
		ResourceWithinBounds: req.ResourceWithinBounds,
		Prerequisites: ReleaseEvidencePrerequisites{EvaluationDSN: req.EvaluationDSN, RuntimeDSN: req.RuntimeDSN,
			PostgreSQLReady: req.PostgreSQLReady, PGVectorReady: req.PGVectorReady,
			FixtureCompatible: req.FixtureCompatible, ProjectionFresh: req.ProjectionFresh,
			RollbackTested: req.RollbackTested, SemanticHitProven: req.SemanticHitProven,
			TrajectoryCompatible: req.TrajectoryCompatible, ResourceWithinBounds: req.ResourceWithinBounds},
	})
}

type ReleaseEvidenceReport struct {
	RunIdentity         string                   `json:"run_identity"`
	ProviderProfile     string                   `json:"provider_profile"`
	PolicyVersion       string                   `json:"policy_version"`
	ScopeHash           string                   `json:"scope_hash"`
	SourceWatermarkHash string                   `json:"source_watermark_hash,omitempty"`
	EvidenceFreshness   ReleaseEvidenceFreshness `json:"evidence_freshness"`
	EvidenceExpiresAt   time.Time                `json:"evidence_expires_at,omitempty"`
	DeterministicReplay bool                     `json:"deterministic_replay"`
	RollbackTested      bool                     `json:"rollback_tested"`
	Verdict             ReleaseEvidenceVerdict   `json:"verdict"`
	ReleaseEligible     bool                     `json:"release_eligible"`
	RealStack           bool                     `json:"real_stack"`
	FailureCategories   []string                 `json:"failure_categories,omitempty"`
	ProgressiveLevels   int                      `json:"progressive_levels"`
	ProgressiveEligible int                      `json:"progressive_eligible"`
	ParentFirstEligible bool                     `json:"parent_first_eligible"`
	QualityEligible     bool                     `json:"quality_eligible"`
	ProgressiveFailures []string                 `json:"progressive_failures,omitempty"`
	ParentFirstFailures []string                 `json:"parent_first_failures,omitempty"`
	IntegrityEligible   bool                     `json:"integrity_eligible"`
	IntegrityFailures   []string                 `json:"integrity_failures,omitempty"`
	GeneratedAt         time.Time                `json:"generated_at"`
}

// MarshalReleaseEvidenceReport is the redacted report boundary. The report
// schema intentionally contains hashes, identities, categories and aggregates
// only; DSNs, scope values, queries, content and provider payloads cannot be
// represented.
func MarshalReleaseEvidenceReport(report ReleaseEvidenceReport) ([]byte, error) {
	if strings.TrimSpace(report.ProviderProfile) == "" || !evaluationSafeIdentity(report.ProviderProfile) {
		return nil, fmt.Errorf("provider profile is invalid")
	}
	if report.ScopeHash != "" && (!strings.HasPrefix(report.ScopeHash, "scope:") || len(report.ScopeHash) != len("scope:")+64) {
		return nil, fmt.Errorf("scope hash is invalid")
	}
	if report.RunIdentity != "" && (!strings.HasPrefix(report.RunIdentity, "run:") || len(report.RunIdentity) != len("run:")+64) {
		return nil, fmt.Errorf("run identity is invalid")
	}
	if report.SourceWatermarkHash != "" && (!strings.HasPrefix(report.SourceWatermarkHash, "watermark:") || len(report.SourceWatermarkHash) != len("watermark:")+64) {
		return nil, fmt.Errorf("source watermark hash is invalid")
	}
	if report.EvidenceFreshness != "" && report.EvidenceFreshness != ReleaseEvidenceFresh && report.EvidenceFreshness != ReleaseEvidenceStale && report.EvidenceFreshness != ReleaseEvidenceUnknown {
		return nil, fmt.Errorf("evidence freshness is invalid")
	}
	if report.ProgressiveLevels < 0 || report.ProgressiveEligible < 0 || report.ProgressiveEligible > report.ProgressiveLevels {
		return nil, fmt.Errorf("progressive counts are invalid")
	}
	return json.Marshal(report)
}

// RenderReleaseEvidenceSummary returns a bounded human-readable rendering
// suitable for operator logs and CI annotations.
func RenderReleaseEvidenceSummary(report ReleaseEvidenceReport) string {
	status := "not eligible"
	if report.ReleaseEligible {
		status = "release eligible"
	}
	return fmt.Sprintf("retrieval release evidence: %s (run=%s, verdict=%s, freshness=%s, replay=%t, rollback=%t, progressive=%d/%d, parent_first=%t, real_stack=%t)", status, report.RunIdentity, report.Verdict, report.EvidenceFreshness, report.DeterministicReplay, report.RollbackTested, report.ProgressiveEligible, report.ProgressiveLevels, report.ParentFirstEligible, report.RealStack)
}

func EvaluateReleaseEvidence(in ReleaseEvidenceInput) (ReleaseEvidenceReport, error) {
	if err := in.Scope.Validate(); err != nil {
		return ReleaseEvidenceReport{}, err
	}
	if strings.TrimSpace(in.ProviderProfile) == "" || len(in.ProviderProfile) > 128 {
		return ReleaseEvidenceReport{}, fmt.Errorf("provider profile is invalid")
	}
	if err := in.Policy.Validate(); err != nil {
		return ReleaseEvidenceReport{}, err
	}
	if strings.TrimSpace(in.Progressive.BaselineIdentity) == "" {
		return ReleaseEvidenceReport{}, fmt.Errorf("progressive baseline identity is required")
	}
	if in.ParentFirst.Mode != ParentFirstModeShadow {
		return ReleaseEvidenceReport{}, fmt.Errorf("parent-first release evidence must be shadow mode")
	}
	if strings.TrimSpace(in.ParentFirst.StrategyIdentity) == "" || !evaluationSafeIdentity(in.ParentFirst.StrategyIdentity) {
		return ReleaseEvidenceReport{}, fmt.Errorf("parent-first strategy identity is invalid")
	}
	now := in.EvaluatedAt.UTC()
	if now.IsZero() {
		now = time.Now().UTC()
	}
	freshness := ReleaseEvidenceFresh
	if !in.EvidenceExpiresAt.IsZero() && !now.Before(in.EvidenceExpiresAt.UTC()) {
		freshness = ReleaseEvidenceStale
	}
	r := ReleaseEvidenceReport{ProviderProfile: in.ProviderProfile, PolicyVersion: in.Policy.Version, Verdict: ReleaseEvidencePassed, RealStack: true, GeneratedAt: now, ProgressiveLevels: len(in.Progressive.Levels), ParentFirstEligible: in.ParentFirst.Eligible, EvidenceFreshness: freshness, EvidenceExpiresAt: in.EvidenceExpiresAt.UTC(), DeterministicReplay: in.DeterministicReplay || in.Candidate.DeterministicReplay, RollbackTested: in.Prerequisites.RollbackTested}
	r.ScopeHash = scopeHash(in.Scope)
	r.SourceWatermarkHash = sourceWatermarkHash(in.SourceWatermark)
	r.RunIdentity = releaseEvidenceIdentity(in, r.ScopeHash, r.SourceWatermarkHash, now)
	if err := in.Prerequisites.Validate(); err != nil {
		r.ReleaseEligible = false
		r.Verdict = ReleaseEvidenceSkipped
		r.RealStack = false
		if err.Error() == ReleaseEvidenceSkipDSN || err.Error() == ReleaseEvidenceSkipPrerequisite {
			r.FailureCategories = []string{err.Error()}
		} else {
			r.FailureCategories = []string{ReleaseEvidenceIncompatible}
		}
		return r, nil
	}
	if freshness == ReleaseEvidenceStale {
		r.ReleaseEligible = false
		r.Verdict = ReleaseEvidenceRejected
		r.FailureCategories = []string{ReleaseEvidenceStaleCategory}
		return r, nil
	}
	if in.Policy.RequireFreshEvidence && strings.TrimSpace(in.SourceWatermark) == "" {
		r.EvidenceFreshness = ReleaseEvidenceUnknown
		r.ReleaseEligible = false
		r.Verdict = ReleaseEvidenceRejected
		r.FailureCategories = appendUniqueCategory(r.FailureCategories, ReleaseEvidenceStaleCategory)
	}
	if in.Policy.RequireSemanticHit && !in.Prerequisites.SemanticHitProven {
		r.ReleaseEligible = false
		r.Verdict = ReleaseEvidenceRejected
		r.FailureCategories = appendUniqueCategory(r.FailureCategories, ReleaseEvidenceSemanticHitRequired)
	}
	if in.Policy.RequireTrajectoryIntegrity && !in.Prerequisites.TrajectoryCompatible {
		r.ReleaseEligible = false
		r.Verdict = ReleaseEvidenceRejected
		r.FailureCategories = appendUniqueCategory(r.FailureCategories, ReleaseEvidenceTrajectoryRequired)
	}
	if in.Policy.RequireResourceBudget && !in.Prerequisites.ResourceWithinBounds {
		r.ReleaseEligible = false
		r.Verdict = ReleaseEvidenceRejected
		r.FailureCategories = appendUniqueCategory(r.FailureCategories, ReleaseEvidenceResourceFailure)
	}
	if r.Verdict != ReleaseEvidencePassed {
		return r, nil
	}
	if temporalReleaseEvidenceRequired(in.Candidate) && !hasCompatibleTemporalReleaseEvidence(in.Candidate) {
		r.ReleaseEligible = false
		r.Verdict = ReleaseEvidenceRejected
		r.FailureCategories = []string{ReleaseEvidenceTemporalEvidenceRequired}
		return r, nil
	}
	decision, err := EvaluateReleasePolicy(in.Policy, in.Baseline, in.Candidate)
	if err != nil {
		r.Verdict = ReleaseEvidenceRejected
		r.FailureCategories = []string{ReleaseEvidenceIncompatible}
		return r, nil
	}
	r.QualityEligible = decision.Eligible
	if !decision.Eligible {
		r.Verdict = ReleaseEvidenceRejected
		r.FailureCategories = append(r.FailureCategories, decision.HardFailures...)
	}
	for _, level := range in.Progressive.Levels {
		if !evaluationSafeIdentity(level.Identity) {
			return ReleaseEvidenceReport{}, fmt.Errorf("progressive level identity is invalid")
		}
		if level.Eligible {
			r.ProgressiveEligible++
		} else {
			r.FailureCategories = appendUniqueCategory(r.FailureCategories, ReleaseEvidenceProgressiveFailure)
			r.ProgressiveFailures = append(r.ProgressiveFailures, string(level.Identity))
		}
	}
	if len(in.Progressive.Levels) == 0 || r.ProgressiveEligible != len(in.Progressive.Levels) {
		r.Verdict = ReleaseEvidenceRejected
		r.ReleaseEligible = false
	}
	if !in.ParentFirst.Eligible {
		r.Verdict = ReleaseEvidenceRejected
		r.ReleaseEligible = false
		r.FailureCategories = appendUniqueCategory(r.FailureCategories, ReleaseEvidenceParentFirstFailure)
		r.ParentFirstFailures = append(r.ParentFirstFailures, in.ParentFirst.StrategyIdentity)
	}
	if r.Verdict == ReleaseEvidencePassed {
		r.ReleaseEligible = true
	}
	r = ApplyIntegrityEvidence(r, in.Integrity)
	return r, nil
}

// ApplyIntegrityEvidence overlays the independent information-integrity gate
// onto a release report. Integrity can only block eligibility; it can never
// turn a skipped or rejected report into a pass.
func ApplyIntegrityEvidence(report ReleaseEvidenceReport, integrity *evaluation.IntegrityReport) ReleaseEvidenceReport {
	if integrity == nil {
		report.IntegrityEligible = true
		return report
	}
	report.IntegrityEligible = integrity.IntegritySuccess && integrity.Verdict == evaluation.VerdictPassed
	if !report.IntegrityEligible {
		report.ReleaseEligible = false
		if report.Verdict == ReleaseEvidencePassed {
			report.Verdict = ReleaseEvidenceRejected
		}
		report.IntegrityFailures = append(report.IntegrityFailures, "information_integrity_failure")
		report.FailureCategories = appendUniqueCategory(report.FailureCategories, "RETRIEVAL_RELEASE_EVIDENCE_INTEGRITY_FAILURE")
	}
	return report
}

func temporalReleaseEvidenceRequired(report EvaluationReport) bool {
	return report.Metadata.TemporalPolicyVersion != "" || report.Metadata.TemporalCoverageVersion != "" || report.TemporalCoverage != nil
}

func hasCompatibleTemporalReleaseEvidence(report EvaluationReport) bool {
	coverage := report.TemporalCoverage
	if coverage == nil || report.Metadata.TemporalPolicyVersion == "" || report.Metadata.TemporalCoverageVersion == "" ||
		coverage.PolicyVersion != report.Metadata.TemporalPolicyVersion || coverage.CoverageVersion != report.Metadata.TemporalCoverageVersion || coverage.Cases == 0 {
		return false
	}
	return report.validateSafeOutput() == nil
}

func appendUniqueCategory(categories []string, category string) []string {
	for _, existing := range categories {
		if existing == category {
			return categories
		}
	}
	return append(categories, category)
}

func scopeHash(scope memory.Scope) string {
	return fmt.Sprintf("scope:%x", stableHash(scope.Tenant+"\x00"+scope.Project+"\x00"+scope.Namespace))
}

func sourceWatermarkHash(watermark string) string {
	if strings.TrimSpace(watermark) == "" {
		return ""
	}
	return fmt.Sprintf("watermark:%x", stableHash(strings.TrimSpace(watermark)))
}

func releaseEvidenceIdentity(in ReleaseEvidenceInput, scopeHash, watermarkHash string, evaluatedAt time.Time) string {
	parts := []string{
		scopeHash, watermarkHash, in.ProviderProfile, in.Policy.Version,
		in.Baseline.Metadata.FixtureVersion, in.Baseline.Metadata.RepresentationVersion,
		in.Baseline.Metadata.RankingVersion, in.Baseline.Metadata.FusionStrategy,
		in.Baseline.Metadata.CompatibleEmbeddingRevision, in.Baseline.Metadata.AnalysisVersion,
		in.Candidate.Metadata.FixtureVersion, in.Candidate.Metadata.RepresentationVersion,
		in.Candidate.Metadata.RankingVersion, in.Candidate.Metadata.FusionStrategy,
		in.Candidate.Metadata.CompatibleEmbeddingRevision, in.Candidate.Metadata.AnalysisVersion,
		in.Progressive.BaselineIdentity, in.ParentFirst.StrategyIdentity,
		evaluatedAt.UTC().Format(time.RFC3339Nano),
	}
	return fmt.Sprintf("run:%x", stableHash(strings.Join(parts, "\x00")))
}

func stableHash(value string) [32]byte {
	return sha256.Sum256([]byte(value))
}
