package retrieval

import (
	"fmt"
	"strings"
	"sync"
	"time"
)

// ReconciliationReason is a bounded, redacted explanation for the current
// activation eligibility verdict. It is intentionally independent from raw
// evaluation failures so it is safe to expose in operator diagnostics.
type ReconciliationReason string

const (
	ReconciliationReasonEligible               ReconciliationReason = "eligible"
	ReconciliationReasonScopeMismatch          ReconciliationReason = "scope_mismatch"
	ReconciliationReasonHandoffIncomplete      ReconciliationReason = "handoff_incomplete"
	ReconciliationReasonPolicyMismatch         ReconciliationReason = "policy_mismatch"
	ReconciliationReasonFixtureMismatch        ReconciliationReason = "fixture_mismatch"
	ReconciliationReasonRepresentationMismatch ReconciliationReason = "representation_mismatch"
	ReconciliationReasonWatermarkMismatch      ReconciliationReason = "watermark_mismatch"
	ReconciliationReasonFreshnessExpired       ReconciliationReason = "freshness_expired"
	ReconciliationReasonAttestationMissing     ReconciliationReason = "attestation_missing"
	ReconciliationReasonAttestationMismatch    ReconciliationReason = "attestation_mismatch"
	ReconciliationReasonRollbackRequired       ReconciliationReason = "rollback_required"
)

type ReleaseEvidenceReconciliationInput struct {
	PolicyID                    string
	ExpectedScopeHash           string
	ExpectedPolicyVersion       string
	ExpectedFixtureVersion      string
	ExpectedStrategyIdentity    string
	ExpectedSourceWatermarkHash string
	Report                      ReleaseEvidenceReport
	Now                         time.Time
}

type ReleaseEvidenceReconciliationResult struct {
	PolicyID               string
	Eligible               bool
	Reason                 ReconciliationReason
	ObservedAt             time.Time
	HandoffIdentity        string
	ScopeHash              string
	PolicyVersion          string
	FixtureVersion         string
	RepresentationIdentity string
	SourceWatermarkHash    string
}

type ReconciliationRecord struct {
	HandoffIdentity string
	ReplayKey       string
	Result          ReleaseEvidenceReconciliationResult
}

type ReconciliationInspection struct {
	PolicyID        string                 `json:"policy_id"`
	HandoffIdentity string                 `json:"handoff_identity"`
	Eligible        bool                   `json:"eligible"`
	Reason          ReconciliationReason   `json:"reason"`
	ObservedAt      time.Time              `json:"observed_at"`
}

type ReconciliationTrigger struct {
	Actor  string `json:"actor"`
	Reason string `json:"reason"`
}

func (t ReconciliationTrigger) Validate() error {
	if strings.TrimSpace(t.Actor) == "" || strings.TrimSpace(t.Reason) == "" {
		return fmt.Errorf("actor and reason are required")
	}
	return nil
}

// ReconciliationLedger models the transactional boundary used by the
// PostgreSQL projection: history is append-only, replay keys are idempotent,
// and a revoked handoff cannot be restored in place.
type ReconciliationLedger struct {
	mu      sync.RWMutex
	history []ReconciliationRecord
	current map[string]ReconciliationRecord
	seen    map[string]struct{}
}

func NewReconciliationLedger() *ReconciliationLedger {
	return &ReconciliationLedger{current: make(map[string]ReconciliationRecord), seen: make(map[string]struct{})}
}

func (l *ReconciliationLedger) Apply(handoffIdentity, replayKey string, result ReleaseEvidenceReconciliationResult) error {
	if l == nil {
		return fmt.Errorf("reconciliation ledger is required")
	}
	if strings.TrimSpace(handoffIdentity) == "" || strings.TrimSpace(replayKey) == "" {
		return fmt.Errorf("handoff and replay identities are required")
	}
	if err := result.Validate(); err != nil {
		return err
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	if _, exists := l.seen[replayKey]; exists {
		return nil
	}
	if previous, exists := l.current[handoffIdentity]; exists && !previous.Result.Eligible && result.Eligible {
		return fmt.Errorf("ineligible handoff requires a new evidence handoff")
	}
	record := ReconciliationRecord{HandoffIdentity: handoffIdentity, ReplayKey: replayKey, Result: result}
	l.seen[replayKey] = struct{}{}
	l.history = append(l.history, record)
	l.current[handoffIdentity] = record
	return nil
}

func (l *ReconciliationLedger) Current(handoffIdentity string) (ReconciliationRecord, bool) {
	if l == nil {
		return ReconciliationRecord{}, false
	}
	l.mu.RLock()
	defer l.mu.RUnlock()
	record, ok := l.current[handoffIdentity]
	return record, ok
}

func (l *ReconciliationLedger) History() []ReconciliationRecord {
	if l == nil {
		return nil
	}
	l.mu.RLock()
	defer l.mu.RUnlock()
	result := make([]ReconciliationRecord, len(l.history))
	copy(result, l.history)
	return result
}

func (r ReleaseEvidenceReconciliationResult) Validate() error {
	if r.Reason == "" {
		return fmt.Errorf("reconciliation reason is required")
	}
	if r.ObservedAt.IsZero() {
		return fmt.Errorf("reconciliation observed time is required")
	}
	return nil
}

// ReconcileReleaseEvidence evaluates only immutable, redacted handoff data and
// current compatibility identities. The order is deterministic and the first
// failed gate wins, so a replay produces the same bounded reason.
func ReconcileReleaseEvidence(input ReleaseEvidenceReconciliationInput) ReleaseEvidenceReconciliationResult {
	now := input.Now.UTC()
	if now.IsZero() {
		now = time.Now().UTC()
	}
	report := input.Report
	result := ReleaseEvidenceReconciliationResult{PolicyID: input.PolicyID, ObservedAt: now, HandoffIdentity: report.RunIdentity, ScopeHash: report.ScopeHash, PolicyVersion: report.PolicyVersion, FixtureVersion: report.FixtureVersion, RepresentationIdentity: report.ProviderProfile, SourceWatermarkHash: report.SourceWatermarkHash, Reason: ReconciliationReasonHandoffIncomplete}
	switch {
	case strings.TrimSpace(input.ExpectedScopeHash) == "" || report.ScopeHash != input.ExpectedScopeHash:
		result.Reason = ReconciliationReasonScopeMismatch
	case report.RunIdentity == "" || report.PolicyVersion == "" || report.FixtureVersion == "" || report.SourceWatermarkHash == "":
		result.Reason = ReconciliationReasonHandoffIncomplete
	case report.PolicyVersion != input.ExpectedPolicyVersion:
		result.Reason = ReconciliationReasonPolicyMismatch
	case report.FixtureVersion != input.ExpectedFixtureVersion:
		result.Reason = ReconciliationReasonFixtureMismatch
	case input.ExpectedStrategyIdentity != "" && report.ProviderProfile != input.ExpectedStrategyIdentity:
		result.Reason = ReconciliationReasonRepresentationMismatch
	case report.SourceWatermarkHash != input.ExpectedSourceWatermarkHash:
		result.Reason = ReconciliationReasonWatermarkMismatch
	case report.EvidenceExpiresAt.IsZero() || !report.EvidenceExpiresAt.After(now):
		result.Reason = ReconciliationReasonFreshnessExpired
	case report.Attestation == nil:
		result.Reason = ReconciliationReasonAttestationMissing
	case report.Attestation.RunIdentity != report.RunIdentity || report.Attestation.ScopeHash != report.ScopeHash || report.Attestation.PolicyVersion != report.PolicyVersion || report.Attestation.FixtureVersion != report.FixtureVersion || report.Attestation.SourceWatermarkHash != report.SourceWatermarkHash:
		result.Reason = ReconciliationReasonAttestationMismatch
	case report.Attestation.RollbackVerdict != ReleaseEvidenceRollbackPassed:
		result.Reason = ReconciliationReasonRollbackRequired
	case report.Attestation.ExpiresAt.IsZero() || !report.Attestation.ExpiresAt.After(now) || report.Attestation.Freshness != ReleaseEvidenceFresh:
		result.Reason = ReconciliationReasonFreshnessExpired
	case !report.ReleaseEligible || report.Verdict != ReleaseEvidencePassed || !report.OperationalOutcome.Consumable || report.OperationalOutcome.State != ReleaseEvidenceRunCompleted || report.OperationalOutcome.Cleanup != ReleaseEvidenceCleanupComplete:
		result.Reason = ReconciliationReasonHandoffIncomplete
	default:
		result.Eligible = true
		result.Reason = ReconciliationReasonEligible
	}
	return result
}
