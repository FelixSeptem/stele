package insights

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
	"github.com/FelixSeptem/stele/internal/reasoning"
)

const ReservedInsightActivationPolicyVersion = "reserved-insight-activation-v1"

type ReservedInsightActivationPolicy struct {
	Scope                   memory.Scope
	Version                 string
	Owner                   string
	Enabled                 bool
	EnabledTypes            map[memory.DerivedInsightType]bool
	ProviderContractVersion string
	SchemaVersion           string
	MinEvidence             int
	MinConfidence           float64
	MaxEvidence             int
	MaxCandidateBytes       int
	ExpiresAt               time.Time
	SourceWatermark         string
	RolledBack              bool
}

func DefaultReservedInsightActivationPolicy(scope memory.Scope) ReservedInsightActivationPolicy {
	return ReservedInsightActivationPolicy{
		Scope:             scope.Normalized(),
		Version:           ReservedInsightActivationPolicyVersion,
		Owner:             "operator",
		EnabledTypes:      map[memory.DerivedInsightType]bool{},
		MinEvidence:       1,
		MaxEvidence:       32,
		MaxCandidateBytes: 16 * 1024,
	}
}

func (p ReservedInsightActivationPolicy) ValidateAt(now time.Time) error {
	if err := p.Scope.Validate(); err != nil {
		return err
	}
	switch {
	case strings.TrimSpace(p.Version) == "":
		return fmt.Errorf("activation policy version is required")
	case strings.TrimSpace(p.Owner) == "":
		return fmt.Errorf("activation policy owner is required")
	case p.MinEvidence <= 0:
		return fmt.Errorf("activation policy minimum evidence must be greater than zero")
	case p.MaxEvidence <= 0:
		return fmt.Errorf("activation policy maximum evidence must be greater than zero")
	case p.MaxEvidence < p.MinEvidence:
		return fmt.Errorf("activation policy maximum evidence must be greater than or equal to minimum evidence")
	case p.MaxCandidateBytes <= 0:
		return fmt.Errorf("activation policy maximum candidate bytes must be greater than zero")
	case p.MinConfidence < 0 || p.MinConfidence > 1:
		return fmt.Errorf("activation policy minimum confidence must be between 0 and 1")
	case p.RolledBack:
		return fmt.Errorf("activation policy is rolled back")
	}
	if !p.Enabled {
		return nil
	}
	if p.ExpiresAt.IsZero() {
		return fmt.Errorf("activation policy expiry is required")
	}
	if !now.IsZero() && !p.ExpiresAt.After(now) {
		return fmt.Errorf("activation policy is expired")
	}
	for insightType, enabled := range p.EnabledTypes {
		if !isReservedInsightType(insightType) {
			return fmt.Errorf("activation policy type %q is not reserved", insightType)
		}
		if enabled && insightType != memory.DerivedInsightTypeHypothesis {
			return fmt.Errorf("activation policy type %q is not enabled by the initial policy", insightType)
		}
	}
	return nil
}

func (p ReservedInsightActivationPolicy) MatchesScope(scope memory.Scope) bool {
	return p.Scope.Normalized() == scope.Normalized()
}

func (p ReservedInsightActivationPolicy) Allows(insightType memory.DerivedInsightType) bool {
	return p.Enabled && !p.RolledBack && isReservedInsightType(insightType) && p.EnabledTypes[insightType]
}

func (p ReservedInsightActivationPolicy) Stopped() ReservedInsightActivationPolicy {
	p.Enabled = false
	return p
}

func (p ReservedInsightActivationPolicy) RolledBackCopy() ReservedInsightActivationPolicy {
	p.Enabled = false
	p.RolledBack = true
	return p
}

type ActivationDisposition string

const (
	ActivationDispositionActivated     ActivationDisposition = "activated"
	ActivationDispositionWouldActivate ActivationDisposition = "would_activate"
	ActivationDispositionRejected      ActivationDisposition = "rejected"
	ActivationDispositionQuarantined   ActivationDisposition = "quarantined"
	ActivationDispositionTypeDisabled  ActivationDisposition = "type_disabled"
	ActivationDispositionStale         ActivationDisposition = "stale"
	ActivationDispositionDuplicate     ActivationDisposition = "duplicate"
)

type ActivationInput struct {
	Policy               ReservedInsightActivationPolicy
	Candidate            memory.DerivedInsight
	AuthorizedEvidence   []memory.DerivedInsightEvidenceRef
	CandidateFingerprint string
	IdempotencyKey       string
	SourceWatermark      string
	PolicyVersion        string
	ExistingFingerprints map[string]struct{}
	Shadow               bool
	Now                  time.Time
}

type ActivationResult struct {
	Disposition          ActivationDisposition
	Insight              memory.DerivedInsight
	CandidateFingerprint string
	Reason               string
	Err                  error
}

type ActivationDecisionRecord struct {
	ID                   string
	Scope                memory.Scope
	PolicyVersion        string
	CandidateFingerprint string
	IdempotencyKey       string
	InsightType          memory.DerivedInsightType
	Disposition          ActivationDisposition
	InsightID            string
	Reason               string
	SourceWatermark      string
	CreatedAt            time.Time
	Metadata             map[string]any
}

func (r ActivationDecisionRecord) Validate() error {
	if strings.TrimSpace(r.ID) == "" {
		return fmt.Errorf("activation decision id is required")
	}
	if err := r.Scope.Validate(); err != nil {
		return err
	}
	if strings.TrimSpace(r.PolicyVersion) == "" {
		return fmt.Errorf("activation decision policy version is required")
	}
	if strings.TrimSpace(r.CandidateFingerprint) == "" {
		return fmt.Errorf("activation decision candidate fingerprint is required")
	}
	if !r.InsightType.Valid() {
		return fmt.Errorf("activation decision insight type %q is invalid", r.InsightType)
	}
	if !validActivationDisposition(r.Disposition) {
		return fmt.Errorf("activation decision disposition %q is invalid", r.Disposition)
	}
	if strings.TrimSpace(r.Reason) == "" {
		return fmt.Errorf("activation decision reason is required")
	}
	if r.CreatedAt.IsZero() {
		return fmt.Errorf("activation decision created at is required")
	}
	return nil
}

type ActivationDecisionEvidence struct {
	DecisionID string
	Scope      memory.Scope
	Kind       memory.DerivedInsightEvidenceKind
	ID         string
	Relation   memory.DerivedInsightEvidenceRelation
	ObservedAt time.Time
}

func (e ActivationDecisionEvidence) Validate() error {
	if strings.TrimSpace(e.DecisionID) == "" {
		return fmt.Errorf("activation decision evidence decision id is required")
	}
	if err := e.Scope.Validate(); err != nil {
		return err
	}
	if !e.Kind.Valid() || strings.TrimSpace(e.ID) == "" || !e.Relation.Valid() {
		return fmt.Errorf("activation decision evidence reference is invalid")
	}
	return nil
}

type ReservedInsightActivationStore interface {
	UpsertDerivedInsight(ctx context.Context, insight memory.DerivedInsight) (memory.DerivedInsight, error)
}

type ActivationDecisionStore interface {
	CreateActivationDecision(ctx context.Context, record ActivationDecisionRecord, evidence []ActivationDecisionEvidence) error
}

// AdmitReasoningCandidate converts the non-authoritative provider envelope
// into the ordinary candidate lifecycle. It never activates directly: the
// existing policy, evidence subset, compatibility, confidence, idempotency,
// and shadow checks still run through AdmitReservedInsight.
func AdmitReasoningCandidate(candidate reasoning.InsightCandidate, policy ReservedInsightActivationPolicy, authorizedEvidence []memory.DerivedInsightEvidenceRef, input ActivationInput) ActivationResult {
	if err := candidate.Validate(reasoning.DefaultLimits()); err != nil {
		return ActivationResult{Disposition: ActivationDispositionQuarantined, CandidateFingerprint: candidate.ReplayID, Reason: err.Error(), Err: err}
	}
	if input.CandidateFingerprint == "" {
		input.CandidateFingerprint = candidate.ReplayID
	}
	if input.PolicyVersion == "" {
		input.PolicyVersion = candidate.PolicyVersion
	}
	if input.SourceWatermark == "" {
		input.SourceWatermark = candidate.SourceWatermark
	}
	input.Policy = policy
	input.AuthorizedEvidence = authorizedEvidence
	input.Candidate = memory.DerivedInsight{
		ID:         candidate.ID,
		Scope:      candidate.Scope,
		Type:       candidate.InsightType,
		State:      memory.DerivedInsightStateCandidate,
		Title:      candidate.Title,
		Summary:    candidate.Summary,
		Confidence: memory.DerivedInsightConfidence{Score: 1 - candidate.Uncertainty, Method: "reasoning_uncertainty"},
		Derivation: memory.DerivedInsightDerivation{
			Source:      "reasoning_provider",
			Fingerprint: candidate.ReplayID,
			DerivedAt:   candidate.CreatedAt,
			Metadata: map[string]any{
				"provider_version":     candidate.ProviderVersion,
				"schema_version":       candidate.SchemaVersion,
				"policy_version":       candidate.PolicyVersion,
				"source_watermark":     candidate.SourceWatermark,
				"scope_proof":          candidate.ScopeProof,
				"lifecycle_visibility": candidate.LifecycleVisibility,
				"redaction_policy":     candidate.RedactionPolicy,
				"reasoning_mode":       string(candidate.Mode),
				"reasoning_candidate":  true,
			},
		},
		Evidence:  candidate.Evidence,
		CreatedAt: candidate.CreatedAt,
		UpdatedAt: candidate.CreatedAt,
	}
	return AdmitReservedInsight(input)
}

type ReservedInsightActivationService struct {
	Store ReservedInsightActivationStore
}

func (s ReservedInsightActivationService) Apply(ctx context.Context, input ActivationInput) (result ActivationResult, err error) {
	if s.Store == nil {
		return ActivationResult{}, fmt.Errorf("reserved insight activation store is required")
	}
	result = AdmitReservedInsight(input)
	persistedInsight := false
	defer func() {
		decisionStore, ok := s.Store.(ActivationDecisionStore)
		if !ok || result.Disposition == "" || (result.Disposition == ActivationDispositionActivated && !persistedInsight) {
			return
		}
		record := activationDecisionRecord(input, result)
		evidence := activationDecisionEvidence(input, record.ID)
		if persistErr := decisionStore.CreateActivationDecision(ctx, record, evidence); persistErr != nil && err == nil {
			err = fmt.Errorf("persist activation decision: %w", persistErr)
		}
	}()
	if result.Disposition != ActivationDispositionActivated {
		if result.Err != nil {
			return result, result.Err
		}
		return result, nil
	}
	stored, err := s.Store.UpsertDerivedInsight(ctx, result.Insight)
	if err != nil {
		return ActivationResult{}, fmt.Errorf("persist admitted reserved insight: %w", err)
	}
	result.Insight = stored
	persistedInsight = true
	return result, nil
}

// ApplyReasoningCandidate performs the explicit, authorized handoff for a
// normalized reasoning candidate. The candidate is converted and validated
// before the ordinary activation service persists anything.
func (s ReservedInsightActivationService) ApplyReasoningCandidate(ctx context.Context, candidate reasoning.InsightCandidate, policy ReservedInsightActivationPolicy, authorizedEvidence []memory.DerivedInsightEvidenceRef, now time.Time) (ActivationResult, error) {
	preflight := AdmitReasoningCandidate(candidate, policy, authorizedEvidence, ActivationInput{Policy: policy, Shadow: true, Now: now})
	if preflight.Disposition != ActivationDispositionWouldActivate {
		if preflight.Err != nil {
			return preflight, preflight.Err
		}
		return preflight, nil
	}
	return s.Apply(ctx, ActivationInput{
		Policy:               policy,
		Candidate:            preflight.Insight,
		AuthorizedEvidence:   authorizedEvidence,
		CandidateFingerprint: candidate.ReplayID,
		IdempotencyKey:       candidate.ReplayID,
		SourceWatermark:      candidate.SourceWatermark,
		PolicyVersion:        candidate.PolicyVersion,
		Now:                  now,
	})
}

func activationDecisionRecord(input ActivationInput, result ActivationResult) ActivationDecisionRecord {
	createdAt := input.Now.UTC()
	if createdAt.IsZero() {
		createdAt = time.Now().UTC()
	}
	return ActivationDecisionRecord{
		ID:                   activationDecisionID(result.CandidateFingerprint),
		Scope:                input.Candidate.Scope,
		PolicyVersion:        input.Policy.Version,
		CandidateFingerprint: result.CandidateFingerprint,
		IdempotencyKey:       input.IdempotencyKey,
		InsightType:          input.Candidate.Type,
		Disposition:          result.Disposition,
		InsightID:            result.Insight.ID,
		Reason:               result.Reason,
		SourceWatermark:      input.SourceWatermark,
		CreatedAt:            createdAt,
	}
}

func activationDecisionEvidence(input ActivationInput, decisionID string) []ActivationDecisionEvidence {
	evidence := make([]ActivationDecisionEvidence, 0, len(input.Candidate.Evidence))
	for _, item := range input.Candidate.Evidence {
		evidence = append(evidence, ActivationDecisionEvidence{DecisionID: decisionID, Scope: input.Candidate.Scope, Kind: item.Kind, ID: item.ID, Relation: item.Relation, ObservedAt: item.ObservedAt})
	}
	return evidence
}

func AdmitReservedInsight(input ActivationInput) ActivationResult {
	result := ActivationResult{CandidateFingerprint: input.CandidateFingerprint}
	now := input.Now.UTC()
	if now.IsZero() {
		now = time.Now().UTC()
	}
	if err := input.Policy.ValidateAt(now); err != nil {
		result.Disposition = ActivationDispositionStale
		result.Reason = err.Error()
		result.Err = err
		return result
	}
	if !input.Policy.MatchesScope(input.Candidate.Scope) {
		return rejectActivation(result, "candidate scope does not match activation policy")
	}
	if input.PolicyVersion != "" && input.PolicyVersion != input.Policy.Version {
		return rejectActivation(result, "activation policy version does not match request")
	}
	if !isReservedInsightType(input.Candidate.Type) {
		return rejectActivation(result, "candidate type is not reserved")
	}
	if !input.Policy.Allows(input.Candidate.Type) {
		result.Disposition = ActivationDispositionTypeDisabled
		result.Reason = "reserved insight type is disabled by policy"
		return result
	}
	if input.Candidate.State != memory.DerivedInsightStateCandidate {
		return rejectActivation(result, "candidate must be in candidate lifecycle state")
	}
	if err := input.Candidate.Validate(); err != nil {
		return rejectActivation(result, err.Error())
	}
	if len(input.Candidate.Evidence) < input.Policy.MinEvidence {
		return rejectActivation(result, "candidate does not meet minimum evidence")
	}
	if len(input.Candidate.Evidence) > input.Policy.MaxEvidence {
		return rejectActivation(result, "candidate evidence exceeds activation policy budget")
	}
	encodedCandidate, err := json.Marshal(input.Candidate)
	if err != nil {
		return rejectActivation(result, "candidate cannot be normalized")
	}
	if len(encodedCandidate) > input.Policy.MaxCandidateBytes {
		return rejectActivation(result, "candidate exceeds activation policy byte budget")
	}
	if input.Candidate.Confidence.Score < input.Policy.MinConfidence {
		return rejectActivation(result, "candidate does not meet minimum confidence")
	}
	if input.Policy.SourceWatermark != "" && input.Policy.SourceWatermark != input.SourceWatermark {
		result.Disposition = ActivationDispositionStale
		result.Reason = "source watermark does not match activation policy"
		return result
	}
	if err := validateActivationCompatibility(input); err != nil {
		return rejectActivation(result, err.Error())
	}
	if err := evidenceSubset(input.Candidate.Evidence, input.AuthorizedEvidence); err != nil {
		return rejectActivation(result, err.Error())
	}
	if result.CandidateFingerprint == "" {
		result.CandidateFingerprint = candidateFingerprint(input.Candidate, input.Policy)
	}
	if _, exists := input.ExistingFingerprints[result.CandidateFingerprint]; exists {
		result.Disposition = ActivationDispositionDuplicate
		result.Reason = "candidate fingerprint already has an activation decision"
		return result
	}
	result.Insight = input.Candidate
	result.Insight.State = memory.DerivedInsightStateActive
	if result.Insight.Derivation.Metadata == nil {
		result.Insight.Derivation.Metadata = map[string]any{}
	}
	result.Insight.Derivation.Metadata["activation_policy_version"] = input.Policy.Version
	result.Insight.Derivation.Metadata["activation_decision"] = string(ActivationDispositionActivated)
	result.Insight.Derivation.Metadata["activation_source_watermark"] = input.SourceWatermark
	result.Insight.UpdatedAt = now
	if input.Shadow {
		result.Disposition = ActivationDispositionWouldActivate
		result.Insight.State = memory.DerivedInsightStateCandidate
		result.Reason = "shadow evaluation is non-authoritative"
		return result
	}
	if err := result.Insight.Validate(); err != nil {
		return rejectActivation(result, err.Error())
	}
	result.Disposition = ActivationDispositionActivated
	result.Reason = "candidate admitted by activation policy"
	return result
}

func rejectActivation(result ActivationResult, reason string) ActivationResult {
	result.Disposition = ActivationDispositionRejected
	result.Reason = reason
	result.Err = fmt.Errorf("reserved insight activation rejected: %s", reason)
	return result
}

func validateActivationCompatibility(input ActivationInput) error {
	metadata := input.Candidate.Derivation.Metadata
	if expected := input.Policy.ProviderContractVersion; expected != "" && metadataString(metadata, "provider_contract_version") != expected {
		return fmt.Errorf("provider contract version is incompatible with activation policy")
	}
	if expected := input.Policy.SchemaVersion; expected != "" && metadataString(metadata, "schema_version") != expected {
		return fmt.Errorf("schema version is incompatible with activation policy")
	}
	return nil
}

func evidenceSubset(candidate, authorized []memory.DerivedInsightEvidenceRef) error {
	allowed := make(map[string]struct{}, len(authorized))
	for _, item := range authorized {
		allowed[string(item.Kind)+"\x00"+item.ID] = struct{}{}
	}
	for _, item := range candidate {
		if tenant, ok := item.Metadata["tenant"].(string); ok && tenant != "" {
			return fmt.Errorf("candidate evidence contains foreign scope metadata")
		}
		if _, ok := allowed[string(item.Kind)+"\x00"+item.ID]; !ok {
			return fmt.Errorf("candidate evidence is not an authorized subset")
		}
	}
	return nil
}

func metadataString(metadata map[string]any, key string) string {
	if metadata == nil {
		return ""
	}
	value, _ := metadata[key].(string)
	return strings.TrimSpace(value)
}

func candidateFingerprint(candidate memory.DerivedInsight, policy ReservedInsightActivationPolicy) string {
	ids := make([]string, 0, len(candidate.Evidence))
	for _, item := range candidate.Evidence {
		ids = append(ids, string(item.Kind)+"\x00"+item.ID)
	}
	sort.Strings(ids)
	seed := strings.Join([]string{candidate.Scope.Tenant, candidate.Scope.Project, candidate.Scope.Namespace, string(candidate.Type), candidate.Derivation.Fingerprint, policy.Version, strings.Join(ids, "\x00")}, "\x00")
	sum := sha256.Sum256([]byte(seed))
	return "activation:" + hex.EncodeToString(sum[:16])
}

func isReservedInsightType(insightType memory.DerivedInsightType) bool {
	switch insightType {
	case memory.DerivedInsightTypeHypothesis, memory.DerivedInsightTypeGoal, memory.DerivedInsightTypeContradiction, memory.DerivedInsightTypeCausalLink:
		return true
	default:
		return false
	}
}

func validActivationDisposition(disposition ActivationDisposition) bool {
	switch disposition {
	case ActivationDispositionActivated, ActivationDispositionWouldActivate, ActivationDispositionRejected, ActivationDispositionQuarantined, ActivationDispositionTypeDisabled, ActivationDispositionStale, ActivationDispositionDuplicate:
		return true
	default:
		return false
	}
}

func activationDecisionID(fingerprint string) string {
	sum := sha256.Sum256([]byte("activation-decision:" + fingerprint))
	return "activation_decision_" + hex.EncodeToString(sum[:16])
}
