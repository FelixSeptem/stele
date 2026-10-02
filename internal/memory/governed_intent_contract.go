package memory

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"
)

const (
	MemoryIntentMaxContentBytes    = 64 * 1024
	MemoryIntentMaxProvenanceBytes = 16 * 1024
	MemoryIntentMaxEvidence        = 16
	MemoryIntentMaxEvidenceRef     = 256
)

type MemoryIntentEvidence struct {
	Scope   Scope  `json:"scope"`
	Kind    string `json:"kind"`
	ID      string `json:"id"`
	Version int64  `json:"version,omitempty"`
}

func (e MemoryIntentEvidence) Validate(scope Scope) error {
	if err := e.Scope.Validate(); err != nil {
		return fmt.Errorf("evidence scope: %w", err)
	}
	if e.Scope.Normalized() != scope.Normalized() {
		return fmt.Errorf("evidence scope does not match intent scope")
	}
	if strings.TrimSpace(e.Kind) == "" || len(e.Kind) > MemoryIntentMaxEvidenceRef || strings.TrimSpace(e.ID) == "" || len(e.ID) > MemoryIntentMaxEvidenceRef {
		return fmt.Errorf("evidence kind and id are required and bounded")
	}
	if e.Version < 0 {
		return fmt.Errorf("evidence version cannot be negative")
	}
	return nil
}

type MemoryIntentDiagnosticCategory string

const (
	MemoryIntentDiagnosticAccepted           MemoryIntentDiagnosticCategory = "accepted"
	MemoryIntentDiagnosticPending            MemoryIntentDiagnosticCategory = "pending"
	MemoryIntentDiagnosticRejected           MemoryIntentDiagnosticCategory = "rejected"
	MemoryIntentDiagnosticSuppressed         MemoryIntentDiagnosticCategory = "suppressed"
	MemoryIntentDiagnosticFailed             MemoryIntentDiagnosticCategory = "failed"
	MemoryIntentDiagnosticReplayed           MemoryIntentDiagnosticCategory = "replayed"
	MemoryIntentDiagnosticScopeDenied        MemoryIntentDiagnosticCategory = "scope_denied"
	MemoryIntentDiagnosticTargetStale        MemoryIntentDiagnosticCategory = "target_stale"
	MemoryIntentDiagnosticEvidenceIncomplete MemoryIntentDiagnosticCategory = "evidence_incomplete"
	MemoryIntentDiagnosticPolicyDisabled     MemoryIntentDiagnosticCategory = "policy_disabled"
	MemoryIntentDiagnosticRetryExhausted     MemoryIntentDiagnosticCategory = "retry_exhausted"
	MemoryIntentDiagnosticRolledBack         MemoryIntentDiagnosticCategory = "rolled_back"
)

func (c MemoryIntentDiagnosticCategory) Valid() bool {
	switch c {
	case MemoryIntentDiagnosticAccepted, MemoryIntentDiagnosticPending, MemoryIntentDiagnosticRejected, MemoryIntentDiagnosticSuppressed,
		MemoryIntentDiagnosticFailed, MemoryIntentDiagnosticReplayed, MemoryIntentDiagnosticScopeDenied, MemoryIntentDiagnosticTargetStale,
		MemoryIntentDiagnosticEvidenceIncomplete, MemoryIntentDiagnosticPolicyDisabled, MemoryIntentDiagnosticRetryExhausted, MemoryIntentDiagnosticRolledBack:
		return true
	default:
		return false
	}
}

type MemoryIntentTransition struct {
	ID                 string
	IntentID           string
	Scope              Scope
	Sequence           int64
	From               MemoryIntentStatus
	To                 MemoryIntentStatus
	Actor              string
	Reason             string
	DiagnosticCategory MemoryIntentDiagnosticCategory
	WorkReference      string
	OutcomeReference   string
	OccurredAt         time.Time
}

type MemoryIntentHistory struct {
	Intent      MemoryIntentRecord
	Transitions []MemoryIntentTransition
}

type MemoryIntentListInput struct {
	Scope  Scope
	Limit  int
	Cursor time.Time
}

type MemoryIntentPolicyDecision struct {
	Enabled       bool
	PolicyVersion string
	Disposition   MemoryIntentStatus
	Category      MemoryIntentDiagnosticCategory
}

type MemoryIntentResumePolicy struct {
	Scope         Scope
	PolicyVersion string
	Enabled       bool
}

func (p MemoryIntentResumePolicy) Allows(record MemoryIntentRecord) bool {
	return p.Enabled && p.Scope.Normalized() == record.Scope.Normalized() && strings.TrimSpace(p.PolicyVersion) != "" &&
		(record.Status == MemoryIntentStatusPending || record.Status == MemoryIntentStatusAccepted)
}

// MemoryIntentPolicyGate is evaluated before persistence. Implementations
// must bind decisions to the exact input scope and return only bounded state.
type MemoryIntentPolicyGate interface {
	EvaluateMemoryIntent(context.Context, MemoryIntentInput) (MemoryIntentPolicyDecision, error)
}

func (i MemoryIntentListInput) Validate() error {
	if err := i.Scope.Validate(); err != nil {
		return err
	}
	if i.Limit <= 0 || i.Limit > 200 {
		return fmt.Errorf("intent list limit must be between 1 and 200")
	}
	return nil
}

func (t MemoryIntentTransition) Validate() error {
	if strings.TrimSpace(t.IntentID) == "" || !t.Scope.Valid() || t.Sequence <= 0 || strings.TrimSpace(t.Actor) == "" || strings.TrimSpace(t.Reason) == "" || t.OccurredAt.IsZero() {
		return fmt.Errorf("intent transition identity, attribution, and timestamp are required")
	}
	if !t.To.Valid() || (t.From != "" && !t.From.Valid()) || !t.DiagnosticCategory.Valid() {
		return fmt.Errorf("intent transition state or diagnostic category is invalid")
	}
	if len(t.WorkReference) > MemoryIntentMaxEvidenceRef || len(t.OutcomeReference) > MemoryIntentMaxEvidenceRef {
		return fmt.Errorf("intent transition references are too large")
	}
	return nil
}

// Valid reports whether all scope components are present without exposing a
// second scope validation API to callers constructing transition records.
func (s Scope) Valid() bool { return s.Validate() == nil }

func (i MemoryIntentInput) ValidateGoverned() error {
	if err := i.Validate(); err != nil {
		return err
	}
	if len(i.Content) > MemoryIntentMaxContentBytes {
		return fmt.Errorf("intent content exceeds %d bytes", MemoryIntentMaxContentBytes)
	}
	if encoded, err := json.Marshal(i.Provenance); err != nil || len(encoded) > MemoryIntentMaxProvenanceBytes {
		return fmt.Errorf("intent provenance exceeds %d bytes", MemoryIntentMaxProvenanceBytes)
	}
	if len(i.Evidence) > MemoryIntentMaxEvidence {
		return fmt.Errorf("intent evidence exceeds %d references", MemoryIntentMaxEvidence)
	}
	for _, evidence := range i.Evidence {
		if err := evidence.Validate(i.Scope); err != nil {
			return err
		}
	}
	if i.Type == MemoryIntentContradiction && len(i.Evidence) < 2 {
		return fmt.Errorf("contradiction intent requires at least two evidence references")
	}
	if i.Type == MemoryIntentFeedback && strings.TrimSpace(i.TargetInsightID) == "" {
		return fmt.Errorf("feedback target insight id is required")
	}
	return nil
}

func (i MemoryIntentInput) CanonicalFingerprint() (string, error) {
	if err := i.ValidateGoverned(); err != nil {
		return "", err
	}
	path, _ := NormalizeMemoryPath(i.MemoryPath)
	evidence := append([]MemoryIntentEvidence(nil), i.Evidence...)
	sort.Slice(evidence, func(a, b int) bool {
		if evidence[a].Kind != evidence[b].Kind {
			return evidence[a].Kind < evidence[b].Kind
		}
		if evidence[a].ID != evidence[b].ID {
			return evidence[a].ID < evidence[b].ID
		}
		return evidence[a].Version < evidence[b].Version
	})
	payload := struct {
		Scope          Scope                  `json:"scope"`
		Path           string                 `json:"memory_path"`
		Type           MemoryIntentType       `json:"type"`
		Target         string                 `json:"target_memory_id,omitempty"`
		Version        int64                  `json:"target_version,omitempty"`
		Insight        string                 `json:"target_insight_id,omitempty"`
		Content        string                 `json:"content,omitempty"`
		Actor          string                 `json:"actor"`
		Reason         string                 `json:"reason"`
		Provenance     map[string]any         `json:"provenance,omitempty"`
		Evidence       []MemoryIntentEvidence `json:"evidence,omitempty"`
		RequestID      string                 `json:"request_id"`
		OperationID    string                 `json:"operation_id"`
		IdempotencyKey string                 `json:"idempotency_key"`
	}{i.Scope.Normalized(), path, i.Type, strings.TrimSpace(i.TargetMemoryID), i.TargetVersion, strings.TrimSpace(i.TargetInsightID), i.Content, strings.TrimSpace(i.Actor), strings.TrimSpace(i.Reason), i.Provenance, evidence, strings.TrimSpace(i.RequestID), strings.TrimSpace(i.OperationID), strings.TrimSpace(i.IdempotencyKey)}
	b, err := json.Marshal(payload)
	if err != nil {
		return "", fmt.Errorf("marshal intent fingerprint: %w", err)
	}
	digest := sha256.Sum256(b)
	return hex.EncodeToString(digest[:]), nil
}
