package reasoning

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/FelixSeptem/stele/internal/memory"
)

// InsightCandidate is the provider-neutral, non-authoritative envelope used
// by offline and shadow reasoning. It is deliberately separate from Candidate:
// ordinary provider candidates continue to reject reserved types, while this
// envelope can carry them to an independently governed activation policy.
type InsightCandidate struct {
	ID                  string                             `json:"id"`
	Scope               memory.Scope                       `json:"scope"`
	InsightType         memory.DerivedInsightType          `json:"insight_type"`
	Title               string                             `json:"title"`
	Summary             string                             `json:"summary"`
	Evidence            []memory.DerivedInsightEvidenceRef `json:"evidence"`
	EvidenceDigest      string                             `json:"evidence_digest"`
	SourceWatermark     string                             `json:"source_watermark"`
	ScopeProof          string                             `json:"scope_proof"`
	LifecycleVisibility string                             `json:"lifecycle_visibility"`
	RedactionPolicy     string                             `json:"redaction_policy"`
	ProviderVersion     string                             `json:"provider_version"`
	SchemaVersion       string                             `json:"schema_version"`
	PolicyVersion       string                             `json:"policy_version"`
	ReplayID            string                             `json:"replay_id"`
	Uncertainty         float64                            `json:"uncertainty"`
	Mode                Mode                               `json:"mode"`
	DirectActivation    bool                               `json:"direct_activation,omitempty"`
	CanonicalMutation   bool                               `json:"canonical_mutation,omitempty"`
	CreatedAt           time.Time                          `json:"created_at"`
}

type InsightDisposition string

const (
	InsightDispositionCandidate     InsightDisposition = "candidate"
	InsightDispositionRejected      InsightDisposition = "rejected"
	InsightDispositionQuarantined   InsightDisposition = "quarantined"
	InsightDispositionWouldActivate InsightDisposition = "would_activate"
	InsightDispositionStale         InsightDisposition = "stale"
	InsightDispositionFallback      InsightDisposition = "fallback"
)

type InsightDerivationRequest struct {
	Scope               memory.Scope
	InsightType         memory.DerivedInsightType
	Mode                Mode
	Evidence            []memory.DerivedInsightEvidenceRef
	SourceWatermark     string
	ScopeProof          string
	LifecycleVisibility string
	RedactionPolicy     string
	ProviderVersion     string
	SchemaVersion       string
	PolicyVersion       string
	InputDigest         string
	Limits              Limits
	Now                 time.Time
}

type InsightDerivationResult struct {
	Disposition   InsightDisposition `json:"disposition"`
	Candidate     InsightCandidate   `json:"candidate"`
	ReplayID      string             `json:"replay_id"`
	Reason        string             `json:"reason,omitempty"`
	Authoritative bool               `json:"authoritative"`
}

type InsightProvider interface {
	DeriveInsight(context.Context, InsightDerivationRequest) (InsightCandidate, error)
}

func (c InsightCandidate) Validate(limits Limits) error {
	if err := limits.Validate(); err != nil {
		return err
	}
	if err := c.Scope.Validate(); err != nil {
		return fmt.Errorf("candidate scope: %w", err)
	}
	if !isReservedInsightType(c.InsightType) {
		return fmt.Errorf("reasoning insight type %q is not reserved", c.InsightType)
	}
	if c.Mode != ModeOffline && c.Mode != ModeShadow {
		return fmt.Errorf("reasoning insight mode must be offline or shadow")
	}
	for _, field := range []struct {
		name  string
		value string
		max   int
	}{
		{"candidate id", c.ID, 256}, {"title", c.Title, 512}, {"summary", c.Summary, limits.MaxOutputBytes},
		{"evidence digest", c.EvidenceDigest, 128}, {"source watermark", c.SourceWatermark, 256},
		{"scope proof", c.ScopeProof, 256}, {"lifecycle visibility", c.LifecycleVisibility, 64}, {"redaction policy", c.RedactionPolicy, 64}, {"provider version", c.ProviderVersion, 256},
		{"schema version", c.SchemaVersion, 128}, {"policy version", c.PolicyVersion, 256}, {"replay id", c.ReplayID, 128},
	} {
		if !bounded(field.value, field.max) {
			return fmt.Errorf("reasoning %s is invalid", field.name)
		}
	}
	if c.DirectActivation || c.CanonicalMutation {
		return fmt.Errorf("reasoning candidate requests an unsafe mutation")
	}
	if c.LifecycleVisibility != "active_only" || c.RedactionPolicy != "references_only" {
		return fmt.Errorf("reasoning candidate evidence policy is invalid")
	}
	if c.Uncertainty < 0 || c.Uncertainty > 1 {
		return fmt.Errorf("reasoning uncertainty must be between 0 and 1")
	}
	if c.CreatedAt.IsZero() {
		return fmt.Errorf("reasoning candidate created at is required")
	}
	if len(c.Evidence) == 0 || len(c.Evidence) > limits.MaxEvidence {
		return fmt.Errorf("reasoning candidate evidence is invalid")
	}
	for _, evidence := range c.Evidence {
		if err := evidence.Validate(); err != nil {
			return fmt.Errorf("reasoning candidate evidence: %w", err)
		}
		if foreign, ok := evidence.Metadata["tenant"].(string); ok && strings.TrimSpace(foreign) != "" && foreign != c.Scope.Tenant {
			return fmt.Errorf("reasoning candidate evidence is outside scope")
		}
	}
	digest, err := EvidenceDigest(c.Evidence)
	if err != nil {
		return err
	}
	if digest != c.EvidenceDigest {
		return fmt.Errorf("reasoning evidence digest does not match citations")
	}
	return nil
}

func (r InsightDerivationRequest) Validate(now time.Time) error {
	if err := r.Limits.Validate(); err != nil {
		return err
	}
	if err := r.Scope.Validate(); err != nil {
		return fmt.Errorf("scope: %w", err)
	}
	if !isReservedInsightType(r.InsightType) {
		return fmt.Errorf("reasoning insight type %q is not reserved", r.InsightType)
	}
	if r.Mode != ModeOffline && r.Mode != ModeShadow {
		return fmt.Errorf("reasoning insight mode must be offline or shadow")
	}
	if !bounded(r.SourceWatermark, 256) || !bounded(r.ScopeProof, 256) || !bounded(r.LifecycleVisibility, 64) || !bounded(r.RedactionPolicy, 64) || !bounded(r.ProviderVersion, 256) || !bounded(r.SchemaVersion, 128) || !bounded(r.PolicyVersion, 256) || !bounded(r.InputDigest, 128) {
		return fmt.Errorf("reasoning derivation metadata is invalid")
	}
	if r.LifecycleVisibility != "active_only" || r.RedactionPolicy != "references_only" {
		return fmt.Errorf("reasoning evidence policy is invalid")
	}
	if len(r.Evidence) == 0 || len(r.Evidence) > r.Limits.MaxEvidence {
		return fmt.Errorf("reasoning derivation evidence is invalid")
	}
	for _, evidence := range r.Evidence {
		if err := evidence.Validate(); err != nil {
			return err
		}
	}
	if now.IsZero() {
		now = time.Now().UTC()
	}
	if !r.Now.IsZero() && r.Now.After(now.Add(time.Minute)) {
		return fmt.Errorf("reasoning derivation timestamp is in the future")
	}
	return nil
}

func EvidenceDigest(evidence []memory.DerivedInsightEvidenceRef) (string, error) {
	canonical := append([]memory.DerivedInsightEvidenceRef(nil), evidence...)
	sort.Slice(canonical, func(i, j int) bool {
		left := string(canonical[i].Kind) + "\x00" + canonical[i].ID + "\x00" + string(canonical[i].Relation)
		right := string(canonical[j].Kind) + "\x00" + canonical[j].ID + "\x00" + string(canonical[j].Relation)
		return left < right
	})
	b, err := json.Marshal(canonical)
	if err != nil {
		return "", fmt.Errorf("marshal reasoning evidence: %w", err)
	}
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:]), nil
}

func InsightReplayID(request InsightDerivationRequest) (string, error) {
	evidenceDigest, err := EvidenceDigest(request.Evidence)
	if err != nil {
		return "", err
	}
	seed := struct {
		Scope               memory.Scope              `json:"scope"`
		Type                memory.DerivedInsightType `json:"type"`
		Mode                Mode                      `json:"mode"`
		EvidenceDigest      string                    `json:"evidence_digest"`
		SourceWatermark     string                    `json:"source_watermark"`
		ScopeProof          string                    `json:"scope_proof"`
		LifecycleVisibility string                    `json:"lifecycle_visibility"`
		RedactionPolicy     string                    `json:"redaction_policy"`
		ProviderVersion     string                    `json:"provider_version"`
		SchemaVersion       string                    `json:"schema_version"`
		PolicyVersion       string                    `json:"policy_version"`
		InputDigest         string                    `json:"input_digest"`
	}{request.Scope.Normalized(), request.InsightType, request.Mode, evidenceDigest, request.SourceWatermark, request.ScopeProof, request.LifecycleVisibility, request.RedactionPolicy, request.ProviderVersion, request.SchemaVersion, request.PolicyVersion, request.InputDigest}
	b, err := json.Marshal(seed)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(b)
	return "reasoning:" + hex.EncodeToString(sum[:16]), nil
}

func ValidateInsightCandidate(request InsightDerivationRequest, candidate InsightCandidate) error {
	if err := request.Validate(request.Now); err != nil {
		return err
	}
	if err := candidate.Validate(request.Limits); err != nil {
		return err
	}
	if candidate.Scope.Normalized() != request.Scope.Normalized() {
		return fmt.Errorf("reasoning candidate scope does not match request")
	}
	if candidate.InsightType != request.InsightType || candidate.Mode != request.Mode {
		return fmt.Errorf("reasoning candidate type or mode does not match request")
	}
	if candidate.SourceWatermark != request.SourceWatermark || candidate.ScopeProof != request.ScopeProof || candidate.LifecycleVisibility != request.LifecycleVisibility || candidate.RedactionPolicy != request.RedactionPolicy || candidate.ProviderVersion != request.ProviderVersion || candidate.SchemaVersion != request.SchemaVersion || candidate.PolicyVersion != request.PolicyVersion {
		return fmt.Errorf("reasoning candidate compatibility metadata does not match request")
	}
	allowed := make(map[string]struct{}, len(request.Evidence))
	for _, evidence := range request.Evidence {
		allowed[evidenceKey(evidence)] = struct{}{}
	}
	for _, evidence := range candidate.Evidence {
		if _, ok := allowed[evidenceKey(evidence)]; !ok {
			return fmt.Errorf("reasoning candidate evidence is not an authorized subset")
		}
	}
	replayID, err := InsightReplayID(request)
	if err != nil {
		return err
	}
	if candidate.ReplayID != replayID {
		return fmt.Errorf("reasoning candidate replay identity does not match request")
	}
	return nil
}

func EvaluateInsightCandidate(ctx context.Context, provider InsightProvider, request InsightDerivationRequest) InsightDerivationResult {
	result := InsightDerivationResult{Disposition: InsightDispositionFallback, Authoritative: false}
	replayID, err := InsightReplayID(request)
	if err != nil {
		result.Reason = err.Error()
		return result
	}
	result.ReplayID = replayID
	if err := request.Validate(request.Now); err != nil {
		result.Disposition = InsightDispositionRejected
		result.Reason = err.Error()
		return result
	}
	if provider == nil {
		result.Reason = "reasoning provider is unavailable"
		return result
	}
	candidate, err := provider.DeriveInsight(ctx, request)
	if err != nil {
		result.Reason = "reasoning provider failed"
		return result
	}
	if err := ValidateInsightCandidate(request, candidate); err != nil {
		result.Disposition = InsightDispositionQuarantined
		result.Reason = err.Error()
		return result
	}
	result.Candidate = candidate
	result.Disposition = InsightDispositionCandidate
	if request.Mode == ModeShadow {
		result.Disposition = InsightDispositionWouldActivate
	}
	return result
}

func evidenceKey(e memory.DerivedInsightEvidenceRef) string {
	return string(e.Kind) + "\x00" + e.ID + "\x00" + string(e.Relation)
}

func isReservedInsightType(t memory.DerivedInsightType) bool {
	switch t {
	case memory.DerivedInsightTypeHypothesis, memory.DerivedInsightTypeGoal, memory.DerivedInsightTypeContradiction, memory.DerivedInsightTypeCausalLink:
		return true
	default:
		return false
	}
}
