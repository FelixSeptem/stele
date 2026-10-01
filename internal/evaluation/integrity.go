package evaluation

import (
	"crypto/sha256"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/FelixSeptem/stele/internal/memory"
)

type Verdict string

const (
	VerdictPassed   Verdict = "passed"
	VerdictDegraded Verdict = "degraded"
	VerdictSkipped  Verdict = "skipped"
	VerdictRejected Verdict = "rejected"
)

type FindingCategory string

const (
	FindingExpectedRecall      FindingCategory = "expected-recall"
	FindingMissing             FindingCategory = "missing"
	FindingAltered             FindingCategory = "altered"
	FindingUnexpectedDuplicate FindingCategory = "unexpected-duplicate"
	FindingMisplaced           FindingCategory = "misplaced"
	FindingForeignScope        FindingCategory = "foreign-scope"
	FindingHiddenLifecycle     FindingCategory = "hidden-lifecycle"
	FindingStaleWatermark      FindingCategory = "stale-watermark"
	FindingNondeterministic    FindingCategory = "nondeterministic"
	FindingRollbackIncomplete  FindingCategory = "rollback-incomplete"
)

var hardFindingCategories = map[FindingCategory]struct{}{
	FindingMissing: {}, FindingAltered: {}, FindingUnexpectedDuplicate: {}, FindingMisplaced: {},
	FindingForeignScope: {}, FindingHiddenLifecycle: {}, FindingStaleWatermark: {},
	FindingNondeterministic: {}, FindingRollbackIncomplete: {},
}

type IntegrityInput struct {
	ID            string
	Scope         memory.Scope
	Identity      CompatibilityIdentity
	Action        string
	ActionSuccess bool
	Findings      map[FindingCategory]int
	CreatedAt     time.Time
}

type IntegrityReport struct {
	ID               string
	Scope            memory.Scope
	Identity         CompatibilityIdentity
	Action           string
	ActionSuccess    bool
	IntegritySuccess bool
	Verdict          Verdict
	Findings         map[FindingCategory]int
	Fingerprint      string
	Authoritative    bool
	CreatedAt        time.Time
}

type RedactedIntegrityReport struct {
	ScopeHash        string                  `json:"scope_hash"`
	Identity         CompatibilityIdentity   `json:"identity"`
	Action           string                  `json:"action"`
	ActionSuccess    bool                    `json:"action_success"`
	IntegritySuccess bool                    `json:"integrity_success"`
	Verdict          Verdict                 `json:"verdict"`
	Findings         map[FindingCategory]int `json:"findings"`
	CreatedAt        time.Time               `json:"created_at"`
}

func EvaluateIntegrity(input IntegrityInput) (IntegrityReport, error) {
	if strings.TrimSpace(input.ID) == "" || !logicalIdentityPattern.MatchString(strings.TrimSpace(input.ID)) {
		return IntegrityReport{}, fmt.Errorf("integrity report id is invalid")
	}
	if err := input.Scope.Validate(); err != nil {
		return IntegrityReport{}, fmt.Errorf("scope is required: %w", err)
	}
	if err := input.Identity.Validate(); err != nil {
		return IntegrityReport{}, err
	}
	action := normalizeAction(input.Action)
	if action == "unknown" {
		return IntegrityReport{}, fmt.Errorf("integrity action is invalid")
	}
	findings, err := normalizeFindings(input.Findings)
	if err != nil {
		return IntegrityReport{}, err
	}
	report := IntegrityReport{
		ID: input.ID, Scope: input.Scope.Normalized(), Identity: input.Identity, Action: action,
		ActionSuccess: input.ActionSuccess, Findings: findings, Authoritative: false, CreatedAt: input.CreatedAt.UTC(),
	}
	if report.CreatedAt.IsZero() {
		report.CreatedAt = time.Now().UTC()
	}
	report.IntegritySuccess = !report.hasAnyHardFailure()
	if !report.IntegritySuccess {
		report.Verdict = VerdictRejected
	} else if !report.ActionSuccess {
		report.Verdict = VerdictDegraded
	} else {
		report.Verdict = VerdictPassed
	}
	report.Fingerprint = integrityFingerprint(report)
	return report, nil
}

func ReplayIntegrity(previous IntegrityReport, input IntegrityInput) (IntegrityReport, error) {
	if err := RequireCompatibleIdentity(previous.Identity, input.Identity); err != nil {
		return IntegrityReport{}, err
	}
	if err := AuthorizeReportScope(input.Scope, hashScope(previous.Scope)); err != nil {
		return IntegrityReport{}, err
	}
	replayed, err := EvaluateIntegrity(input)
	if err != nil {
		return IntegrityReport{}, err
	}
	if replayed.Fingerprint != previous.Fingerprint {
		return IntegrityReport{}, fmt.Errorf("integrity replay is nondeterministic")
	}
	replayed.Authoritative = false
	return replayed, nil
}

func (r IntegrityReport) Redacted() RedactedIntegrityReport {
	findings := make(map[FindingCategory]int, len(r.Findings))
	for category, count := range r.Findings {
		findings[category] = count
	}
	return RedactedIntegrityReport{ScopeHash: hashScope(r.Scope), Identity: r.Identity, Action: r.Action,
		ActionSuccess: r.ActionSuccess, IntegritySuccess: r.IntegritySuccess, Verdict: r.Verdict,
		Findings: findings, CreatedAt: r.CreatedAt}
}

func (r IntegrityReport) HasHardFailure(category FindingCategory) bool {
	_, hard := hardFindingCategories[category]
	return hard && r.Findings[category] > 0
}

func (r IntegrityReport) hasAnyHardFailure() bool {
	for category := range hardFindingCategories {
		if r.Findings[category] > 0 {
			return true
		}
	}
	return false
}

func normalizeAction(value string) string {
	return normalizeCategory(value, "consolidation", "merge", "reclassification", "reflection", "projection", "unknown")
}

func normalizeFindings(input map[FindingCategory]int) (map[FindingCategory]int, error) {
	result := make(map[FindingCategory]int)
	for category, count := range input {
		if count < 0 {
			return nil, fmt.Errorf("finding count must not be negative")
		}
		if count == 0 {
			continue
		}
		normalized := FindingCategory(normalizeCategory(string(category),
			string(FindingExpectedRecall), string(FindingMissing), string(FindingAltered), string(FindingUnexpectedDuplicate),
			string(FindingMisplaced), string(FindingForeignScope), string(FindingHiddenLifecycle), string(FindingStaleWatermark),
			string(FindingNondeterministic), string(FindingRollbackIncomplete), "unknown"))
		result[normalized] += count
	}
	return result, nil
}

func integrityFingerprint(report IntegrityReport) string {
	keys := make([]string, 0, len(report.Findings))
	for category := range report.Findings {
		keys = append(keys, string(category))
	}
	sort.Strings(keys)
	var builder strings.Builder
	builder.WriteString(hashScope(report.Scope))
	builder.WriteString("\x00")
	builder.WriteString(report.Identity.FixtureVersion)
	builder.WriteString("\x00")
	builder.WriteString(report.Identity.PolicyVersion)
	builder.WriteString("\x00")
	builder.WriteString(report.Identity.Strategy)
	builder.WriteString("\x00")
	builder.WriteString(report.Identity.Renderer)
	builder.WriteString("\x00")
	builder.WriteString(report.Identity.Provider)
	builder.WriteString("\x00")
	builder.WriteString(report.Identity.SourceWatermark)
	builder.WriteString("\x00")
	builder.WriteString(report.Action)
	builder.WriteString("\x00")
	builder.WriteString(fmt.Sprintf("%t", report.ActionSuccess))
	for _, key := range keys {
		builder.WriteString("\x00")
		builder.WriteString(key)
		builder.WriteString("=")
		builder.WriteString(fmt.Sprintf("%d", report.Findings[FindingCategory(key)]))
	}
	sum := sha256.Sum256([]byte(builder.String()))
	return fmt.Sprintf("integrity:%x", sum[:])
}
