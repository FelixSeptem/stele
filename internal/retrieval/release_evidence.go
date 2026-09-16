package retrieval

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/FelixSeptem/stele/internal/memory"
)

const (
	ReleaseEvidenceSkipDSN            = RetrievalEvaluationDSNSkip
	ReleaseEvidenceSkipPrerequisite   = "RETRIEVAL_RELEASE_EVIDENCE_PREREQUISITE_REQUIRED"
	ReleaseEvidenceIncompatible       = "RETRIEVAL_RELEASE_EVIDENCE_INCOMPATIBLE"
	ReleaseEvidenceSafetyFailure      = "RETRIEVAL_RELEASE_EVIDENCE_SAFETY_FAILURE"
	ReleaseEvidenceProgressiveFailure = "RETRIEVAL_RELEASE_EVIDENCE_PROGRESSIVE_FAILURE"
	ReleaseEvidenceParentFirstFailure = "RETRIEVAL_RELEASE_EVIDENCE_PARENT_FIRST_FAILURE"
	ReleaseEvidenceRollbackFailure    = "RETRIEVAL_RELEASE_EVIDENCE_ROLLBACK_FAILURE"
	ReleaseEvidenceQualityFailure     = "RETRIEVAL_RELEASE_EVIDENCE_QUALITY_FAILURE"
)

type ReleaseEvidenceVerdict string

const (
	ReleaseEvidencePassed   ReleaseEvidenceVerdict = "passed"
	ReleaseEvidenceSkipped  ReleaseEvidenceVerdict = "skipped"
	ReleaseEvidenceDegraded ReleaseEvidenceVerdict = "degraded"
	ReleaseEvidenceRejected ReleaseEvidenceVerdict = "rejected"
)

type ReleaseEvidencePrerequisites struct {
	EvaluationDSN     string
	RuntimeDSN        string
	PostgreSQLReady   bool
	PGVectorReady     bool
	FixtureCompatible bool
	ProjectionFresh   bool
	RollbackTested    bool
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
	Scope           memory.Scope
	ProviderProfile string
	Policy          EvaluationReleasePolicy
	Baseline        EvaluationReport
	Candidate       EvaluationReport
	Progressive     ProgressiveContextEvaluationReport
	ParentFirst     ParentFirstEvaluationReport
	Prerequisites   ReleaseEvidencePrerequisites
	EvaluatedAt     time.Time
}

// ReleaseEvidenceRunRequest is the explicit, bounded input accepted by the
// release-evidence runner. DSNs are consumed for validation only and are never
// copied into the resulting report.
type ReleaseEvidenceRunRequest struct {
	Scope             memory.Scope
	EvaluationDSN     string
	RuntimeDSN        string
	ProviderProfile   string
	Policy            EvaluationReleasePolicy
	Baseline          EvaluationReport
	Candidate         EvaluationReport
	Progressive       ProgressiveContextEvaluationReport
	ParentFirst       ParentFirstEvaluationReport
	PostgreSQLReady   bool
	PGVectorReady     bool
	FixtureCompatible bool
	ProjectionFresh   bool
	RollbackTested    bool
	EvaluatedAt       time.Time
}

// RunOwnedReleaseEvidence evaluates one exact scope using explicitly owned
// evidence. The context is reserved for future real-stack adapters; this
// function performs no canonical writes and never falls back to RuntimeDSN.
func RunOwnedReleaseEvidence(_ context.Context, req ReleaseEvidenceRunRequest) (ReleaseEvidenceReport, error) {
	return EvaluateReleaseEvidence(ReleaseEvidenceInput{
		Scope: req.Scope, ProviderProfile: req.ProviderProfile, Policy: req.Policy,
		Baseline: req.Baseline, Candidate: req.Candidate, Progressive: req.Progressive,
		ParentFirst: req.ParentFirst, EvaluatedAt: req.EvaluatedAt,
		Prerequisites: ReleaseEvidencePrerequisites{EvaluationDSN: req.EvaluationDSN, RuntimeDSN: req.RuntimeDSN,
			PostgreSQLReady: req.PostgreSQLReady, PGVectorReady: req.PGVectorReady,
			FixtureCompatible: req.FixtureCompatible, ProjectionFresh: req.ProjectionFresh,
			RollbackTested: req.RollbackTested},
	})
}

type ReleaseEvidenceReport struct {
	ProviderProfile     string                 `json:"provider_profile"`
	PolicyVersion       string                 `json:"policy_version"`
	ScopeHash           string                 `json:"scope_hash"`
	Verdict             ReleaseEvidenceVerdict `json:"verdict"`
	ReleaseEligible     bool                   `json:"release_eligible"`
	RealStack           bool                   `json:"real_stack"`
	FailureCategories   []string               `json:"failure_categories,omitempty"`
	ProgressiveLevels   int                    `json:"progressive_levels"`
	ProgressiveEligible int                    `json:"progressive_eligible"`
	ParentFirstEligible bool                   `json:"parent_first_eligible"`
	QualityEligible     bool                   `json:"quality_eligible"`
	ProgressiveFailures []string               `json:"progressive_failures,omitempty"`
	ParentFirstFailures []string               `json:"parent_first_failures,omitempty"`
	GeneratedAt         time.Time              `json:"generated_at"`
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
	return fmt.Sprintf("retrieval release evidence: %s (verdict=%s, progressive=%d/%d, parent_first=%t, real_stack=%t)", status, report.Verdict, report.ProgressiveEligible, report.ProgressiveLevels, report.ParentFirstEligible, report.RealStack)
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
	r := ReleaseEvidenceReport{ProviderProfile: in.ProviderProfile, PolicyVersion: in.Policy.Version, Verdict: ReleaseEvidencePassed, RealStack: true, GeneratedAt: now, ProgressiveLevels: len(in.Progressive.Levels), ParentFirstEligible: in.ParentFirst.Eligible}
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
	r.ScopeHash = scopeHash(in.Scope)
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
	return r, nil
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

func stableHash(value string) [32]byte {
	return sha256.Sum256([]byte(value))
}
