package assurance

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/FelixSeptem/stele/internal/memory"
)

type IntentConformancePhase string

const (
	IntentConformancePhasePrerequisite     IntentConformancePhase = "prerequisite"
	IntentConformancePhaseSubmission       IntentConformancePhase = "submission"
	IntentConformancePhaseReplay           IntentConformancePhase = "replay"
	IntentConformancePhaseScopeIsolation   IntentConformancePhase = "scope_isolation"
	IntentConformancePhaseInspection       IntentConformancePhase = "inspection"
	IntentConformancePhaseQueueRecovery    IntentConformancePhase = "queue_recovery"
	IntentConformancePhaseWorkerRestart    IntentConformancePhase = "worker_restart"
	IntentConformancePhaseSchedulerRestart IntentConformancePhase = "scheduler_restart"
	IntentConformancePhaseRollback         IntentConformancePhase = "rollback"
	IntentConformancePhaseCleanup          IntentConformancePhase = "cleanup"
)

func (p IntentConformancePhase) valid() bool {
	switch p {
	case IntentConformancePhasePrerequisite, IntentConformancePhaseSubmission, IntentConformancePhaseReplay,
		IntentConformancePhaseScopeIsolation, IntentConformancePhaseInspection, IntentConformancePhaseQueueRecovery,
		IntentConformancePhaseWorkerRestart, IntentConformancePhaseSchedulerRestart, IntentConformancePhaseRollback,
		IntentConformancePhaseCleanup:
		return true
	default:
		return false
	}
}

type IntentConformanceResult string

const (
	IntentConformanceResultPass     IntentConformanceResult = "pass"
	IntentConformanceResultSkip     IntentConformanceResult = "skip"
	IntentConformanceResultDegraded IntentConformanceResult = "degraded"
	IntentConformanceResultFail     IntentConformanceResult = "fail"
)

func (r IntentConformanceResult) valid() bool {
	return r == IntentConformanceResultPass || r == IntentConformanceResultSkip || r == IntentConformanceResultDegraded || r == IntentConformanceResultFail
}

type IntentConformanceCategory string

const (
	IntentConformanceCategoryAccepted         IntentConformanceCategory = "accepted"
	IntentConformanceCategoryReplay           IntentConformanceCategory = "replay"
	IntentConformanceCategoryConflict         IntentConformanceCategory = "conflict"
	IntentConformanceCategoryScopeDenied      IntentConformanceCategory = "scope_denied"
	IntentConformanceCategoryValidation       IntentConformanceCategory = "validation"
	IntentConformanceCategoryQueueRecovery    IntentConformanceCategory = "queue_recovery"
	IntentConformanceCategoryWorkerRestart    IntentConformanceCategory = "worker_restart"
	IntentConformanceCategorySchedulerRestart IntentConformanceCategory = "scheduler_restart"
	IntentConformanceCategoryPolicyDisabled   IntentConformanceCategory = "policy_disabled"
	IntentConformanceCategoryRollback         IntentConformanceCategory = "rollback"
	IntentConformanceCategoryRedaction        IntentConformanceCategory = "redaction"
	IntentConformanceCategoryCleanup          IntentConformanceCategory = "cleanup"
	IntentConformanceCategoryPrerequisite     IntentConformanceCategory = "prerequisite"
)

func (c IntentConformanceCategory) valid() bool {
	switch c {
	case IntentConformanceCategoryAccepted, IntentConformanceCategoryReplay, IntentConformanceCategoryConflict,
		IntentConformanceCategoryScopeDenied, IntentConformanceCategoryValidation, IntentConformanceCategoryQueueRecovery,
		IntentConformanceCategoryWorkerRestart, IntentConformanceCategorySchedulerRestart, IntentConformanceCategoryPolicyDisabled,
		IntentConformanceCategoryRollback, IntentConformanceCategoryRedaction, IntentConformanceCategoryCleanup,
		IntentConformanceCategoryPrerequisite:
		return true
	default:
		return false
	}
}

type IntentConformancePhaseResult struct {
	Phase          IntentConformancePhase
	Result         IntentConformanceResult
	Category       IntentConformanceCategory
	DurationBucket string
}

type IntentConformanceInput struct {
	RunID           string
	Scope           memory.Scope
	SchemaVersion   string
	ProviderVersion string
	StartedAt       time.Time
	FinishedAt      time.Time
	Phases          []IntentConformancePhaseResult
}

type IntentConformanceReport struct {
	RunID           string
	Scope           memory.Scope
	SchemaVersion   string
	ProviderVersion string
	Result          IntentConformanceResult
	Consumable      bool
	Phases          []IntentConformancePhaseResult
	Fingerprint     string
	StartedAt       time.Time
	FinishedAt      time.Time
}

type RedactedIntentConformanceReport struct {
	RunHash         string                         `json:"run_hash"`
	ScopeHash       string                         `json:"scope_hash"`
	SchemaVersion   string                         `json:"schema_version"`
	ProviderVersion string                         `json:"provider_version"`
	Result          IntentConformanceResult        `json:"result"`
	Consumable      bool                           `json:"consumable"`
	PhaseResults    []IntentConformancePhaseResult `json:"phases"`
	Fingerprint     string                         `json:"fingerprint"`
	StartedAt       time.Time                      `json:"started_at"`
	FinishedAt      time.Time                      `json:"finished_at"`
}

func (r RedactedIntentConformanceReport) Validate() error {
	if !strings.HasPrefix(r.RunHash, "run:") || !intentConformanceHashPattern.MatchString(strings.TrimPrefix(r.RunHash, "run:")) {
		return fmt.Errorf("redacted conformance run hash is invalid")
	}
	if !strings.HasPrefix(r.ScopeHash, "scope:") || !intentConformanceHashPattern.MatchString(strings.TrimPrefix(r.ScopeHash, "scope:")) {
		return fmt.Errorf("redacted conformance scope hash is invalid")
	}
	if !validIntentConformanceVersion(r.SchemaVersion) || !validIntentConformanceVersion(r.ProviderVersion) {
		return fmt.Errorf("redacted conformance compatibility identity is invalid")
	}
	if !r.Result.valid() || len(r.PhaseResults) == 0 || len(r.PhaseResults) > 32 || r.StartedAt.IsZero() || r.FinishedAt.Before(r.StartedAt) {
		return fmt.Errorf("redacted conformance envelope is invalid")
	}
	if !strings.HasPrefix(r.Fingerprint, "intent-conformance:") || !intentConformanceHashPattern.MatchString(strings.TrimPrefix(r.Fingerprint, "intent-conformance:")) {
		return fmt.Errorf("redacted conformance fingerprint is invalid")
	}
	for _, phase := range r.PhaseResults {
		if !phase.Phase.valid() || !phase.Result.valid() || !phase.Category.valid() || !validDurationBucket(phase.DurationBucket) {
			return fmt.Errorf("redacted conformance phase is invalid")
		}
	}
	return nil
}

var intentConformanceRunIDPattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._:-]{0,127}$`)
var intentConformanceVersionPattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._+:/-]{0,127}$`)
var intentConformanceHashPattern = regexp.MustCompile(`^[0-9a-f]{64}$`)

func (i IntentConformanceInput) Validate() error {
	if !intentConformanceRunIDPattern.MatchString(strings.TrimSpace(i.RunID)) {
		return fmt.Errorf("intent conformance run id is invalid")
	}
	if err := i.Scope.Validate(); err != nil {
		return fmt.Errorf("intent conformance scope: %w", err)
	}
	if !validIntentConformanceVersion(i.SchemaVersion) || !validIntentConformanceVersion(i.ProviderVersion) {
		return fmt.Errorf("intent conformance compatibility identity is invalid")
	}
	if i.StartedAt.IsZero() || len(i.Phases) == 0 || len(i.Phases) > 32 {
		return fmt.Errorf("intent conformance timing or phases are invalid")
	}
	if !i.FinishedAt.IsZero() && i.FinishedAt.Before(i.StartedAt) {
		return fmt.Errorf("intent conformance finished time precedes start")
	}
	for _, phase := range i.Phases {
		if !phase.Phase.valid() || !phase.Result.valid() || !phase.Category.valid() || !validDurationBucket(phase.DurationBucket) {
			return fmt.Errorf("intent conformance phase is invalid")
		}
	}
	return nil
}

func validIntentConformanceVersion(value string) bool {
	value = strings.TrimSpace(value)
	return intentConformanceVersionPattern.MatchString(value) && !strings.Contains(value, "://") && !strings.ContainsAny(value, "@?=#")
}

func EvaluateIntentConformance(input IntentConformanceInput) (IntentConformanceReport, error) {
	if err := input.Validate(); err != nil {
		return IntentConformanceReport{}, err
	}
	phases := append([]IntentConformancePhaseResult(nil), input.Phases...)
	result := IntentConformanceResultPass
	consumable := true
	for _, phase := range phases {
		switch phase.Result {
		case IntentConformanceResultFail:
			result, consumable = IntentConformanceResultFail, false
		case IntentConformanceResultDegraded:
			if result != IntentConformanceResultFail {
				result = IntentConformanceResultDegraded
			}
			consumable = false
		case IntentConformanceResultSkip:
			if result == IntentConformanceResultPass {
				result = IntentConformanceResultSkip
			}
			consumable = false
		}
	}
	finished := input.FinishedAt.UTC()
	if finished.IsZero() {
		finished = input.StartedAt.UTC()
	}
	report := IntentConformanceReport{RunID: strings.TrimSpace(input.RunID), Scope: input.Scope.Normalized(), SchemaVersion: strings.TrimSpace(input.SchemaVersion), ProviderVersion: strings.TrimSpace(input.ProviderVersion), Result: result, Consumable: consumable, Phases: phases, StartedAt: input.StartedAt.UTC(), FinishedAt: finished}
	report.Fingerprint = intentConformanceFingerprint(report)
	return report, nil
}

func ReplayIntentConformance(previous IntentConformanceReport, input IntentConformanceInput) (IntentConformanceReport, error) {
	if previous.Fingerprint == "" {
		return IntentConformanceReport{}, fmt.Errorf("prior intent conformance fingerprint is required")
	}
	if previous.Scope.Normalized() != input.Scope.Normalized() || previous.SchemaVersion != input.SchemaVersion || previous.ProviderVersion != input.ProviderVersion {
		return IntentConformanceReport{}, fmt.Errorf("intent conformance evidence is incompatible")
	}
	replayed, err := EvaluateIntentConformance(input)
	if err != nil {
		return IntentConformanceReport{}, err
	}
	if replayed.Fingerprint != previous.Fingerprint {
		return IntentConformanceReport{}, fmt.Errorf("intent conformance replay is nondeterministic")
	}
	return replayed, nil
}

func (r IntentConformanceReport) Redacted() RedactedIntentConformanceReport {
	phases := append([]IntentConformancePhaseResult(nil), r.Phases...)
	redacted := RedactedIntentConformanceReport{RunHash: hashIntentConformanceString(r.RunID), ScopeHash: hashIntentConformanceScope(r.Scope), SchemaVersion: r.SchemaVersion, ProviderVersion: r.ProviderVersion, Result: r.Result, Consumable: r.Consumable, PhaseResults: phases, Fingerprint: r.Fingerprint, StartedAt: r.StartedAt, FinishedAt: r.FinishedAt}
	return redacted
}

func intentConformanceFingerprint(report IntentConformanceReport) string {
	type fingerprintInput struct {
		Scope, Schema, Provider string
		Result                  IntentConformanceResult
		Consumable              bool
		Phases                  []IntentConformancePhaseResult
	}
	phases := append([]IntentConformancePhaseResult(nil), report.Phases...)
	sort.Slice(phases, func(a, b int) bool { return phases[a].Phase < phases[b].Phase })
	payload, _ := json.Marshal(fingerprintInput{Scope: hashIntentConformanceScope(report.Scope), Schema: report.SchemaVersion, Provider: report.ProviderVersion, Result: report.Result, Consumable: report.Consumable, Phases: phases})
	return "intent-conformance:" + hashIntentConformanceBytes(payload)
}

func hashIntentConformanceScope(scope memory.Scope) string {
	payload, _ := json.Marshal(scope.Normalized())
	return "scope:" + hashIntentConformanceBytes(payload)
}

func hashIntentConformanceString(value string) string {
	return "run:" + hashIntentConformanceBytes([]byte(value))
}

func hashIntentConformanceBytes(value []byte) string {
	sum := sha256.Sum256(value)
	return hex.EncodeToString(sum[:])
}

func validDurationBucket(value string) bool {
	switch value {
	case "", "lt_1s", "1s_10s", "gt_10s", "unknown":
		return true
	default:
		return false
	}
}
