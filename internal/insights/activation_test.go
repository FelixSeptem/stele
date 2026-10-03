package insights

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/FelixSeptem/stele/internal/memory"
	"github.com/FelixSeptem/stele/internal/reasoning"
)

type activationStoreStub struct {
	stored    []memory.DerivedInsight
	decisions []ActivationDecisionRecord
	evidence  []ActivationDecisionEvidence
}

func (s *activationStoreStub) UpsertDerivedInsight(_ context.Context, insight memory.DerivedInsight) (memory.DerivedInsight, error) {
	s.stored = append(s.stored, insight)
	return insight, nil
}

func (s *activationStoreStub) CreateActivationDecision(_ context.Context, record ActivationDecisionRecord, evidence []ActivationDecisionEvidence) error {
	s.decisions = append(s.decisions, record)
	s.evidence = append(s.evidence, evidence...)
	return nil
}

func (s *activationStoreStub) FindActivationDecision(_ context.Context, scope memory.Scope, candidateFingerprint string) (ActivationDecisionRecord, error) {
	for _, decision := range s.decisions {
		if decision.Scope.Normalized() == scope.Normalized() && decision.CandidateFingerprint == candidateFingerprint {
			return decision, nil
		}
	}
	return ActivationDecisionRecord{}, fmt.Errorf("activation decision not found")
}

func TestReservedInsightActivationPolicyDefaultsDisabledAndEnablesHypothesisOnly(t *testing.T) {
	scope := memory.Scope{Tenant: "tenant-a", Project: "project-a", Namespace: "namespace-a"}
	policy := DefaultReservedInsightActivationPolicy(scope)
	if policy.Enabled {
		t.Fatal("default policy enabled, want disabled")
	}
	if policy.Allows(memory.DerivedInsightTypeHypothesis) {
		t.Fatal("default policy allows hypothesis, want disabled")
	}

	policy.Enabled = true
	policy.EnabledTypes[memory.DerivedInsightTypeHypothesis] = true
	policy.ExpiresAt = time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC)
	if err := policy.ValidateAt(time.Date(2026, 9, 30, 0, 0, 0, 0, time.UTC)); err != nil {
		t.Fatalf("ValidateAt() error = %v", err)
	}
	if !policy.Allows(memory.DerivedInsightTypeHypothesis) {
		t.Fatal("enabled policy does not allow hypothesis")
	}
	for _, insightType := range []memory.DerivedInsightType{
		memory.DerivedInsightTypeGoal,
		memory.DerivedInsightTypeContradiction,
		memory.DerivedInsightTypeCausalLink,
	} {
		if policy.Allows(insightType) {
			t.Fatalf("policy allows %s, want disabled", insightType)
		}
	}
}

func TestReservedInsightActivationPolicyRejectsScopeExpiryAndRollback(t *testing.T) {
	scope := memory.Scope{Tenant: "tenant-a", Project: "project-a", Namespace: "namespace-a"}
	policy := DefaultReservedInsightActivationPolicy(scope)
	policy.Enabled = true
	policy.EnabledTypes[memory.DerivedInsightTypeHypothesis] = true
	policy.ExpiresAt = time.Date(2026, 9, 30, 0, 0, 0, 0, time.UTC)
	policy.RolledBack = true

	if err := policy.ValidateAt(time.Date(2026, 9, 29, 0, 0, 0, 0, time.UTC)); err == nil {
		t.Fatal("ValidateAt() error = nil for rolled back policy")
	}
	if policy.MatchesScope(memory.Scope{Tenant: "tenant-b", Project: scope.Project, Namespace: scope.Namespace}) {
		t.Fatal("MatchesScope() = true for foreign scope")
	}
	policy.RolledBack = false
	if err := policy.ValidateAt(policy.ExpiresAt); err == nil {
		t.Fatal("ValidateAt() error = nil at policy expiry")
	}
}

func TestReservedInsightAdmissionRejectsForeignEvidenceAndDisabledType(t *testing.T) {
	scope := memory.Scope{Tenant: "tenant-a", Project: "project-a", Namespace: "namespace-a"}
	policy := DefaultReservedInsightActivationPolicy(scope)
	policy.Enabled = true
	policy.EnabledTypes[memory.DerivedInsightTypeHypothesis] = true
	policy.ExpiresAt = time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC)
	policy.MinEvidence = 1

	candidate := testHypothesisCandidate(scope)
	candidate.Evidence = append(candidate.Evidence, memory.DerivedInsightEvidenceRef{
		Kind:     memory.DerivedInsightEvidenceKindCanonicalMemory,
		ID:       "foreign-memory",
		Relation: memory.DerivedInsightEvidenceRelationSupports,
		Metadata: map[string]any{"tenant": "tenant-b"},
	})
	result := AdmitReservedInsight(ActivationInput{
		Policy:             policy,
		Candidate:          candidate,
		AuthorizedEvidence: candidate.Evidence[:1],
		Now:                time.Date(2026, 9, 29, 0, 0, 0, 0, time.UTC),
	})
	if result.Disposition != ActivationDispositionRejected {
		t.Fatalf("disposition = %s, want rejected", result.Disposition)
	}

	candidate.Type = memory.DerivedInsightTypeGoal
	result = AdmitReservedInsight(ActivationInput{
		Policy:             policy,
		Candidate:          candidate,
		AuthorizedEvidence: candidate.Evidence[:1],
		Now:                time.Date(2026, 9, 29, 0, 0, 0, 0, time.UTC),
	})
	if result.Disposition != ActivationDispositionTypeDisabled {
		t.Fatalf("disposition = %s, want type_disabled", result.Disposition)
	}
}

func TestReservedInsightAdmissionRejectsProviderPreActivatedCandidate(t *testing.T) {
	scope := memory.Scope{Tenant: "tenant-a", Project: "project-a", Namespace: "namespace-a"}
	policy := DefaultReservedInsightActivationPolicy(scope)
	policy.Enabled = true
	policy.EnabledTypes[memory.DerivedInsightTypeHypothesis] = true
	policy.ExpiresAt = time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC)
	candidate := testHypothesisCandidate(scope)
	candidate.State = memory.DerivedInsightStateActive

	result := AdmitReservedInsight(ActivationInput{Policy: policy, Candidate: candidate, AuthorizedEvidence: candidate.Evidence, Now: time.Date(2026, 9, 29, 0, 0, 0, 0, time.UTC)})
	if result.Disposition != ActivationDispositionRejected {
		t.Fatalf("disposition = %s, want rejected", result.Disposition)
	}
	if result.Insight.ID != "" {
		t.Fatalf("rejected result returned insight %+v", result.Insight)
	}
}

func TestReservedInsightAdmissionActivatesHypothesisWithoutCanonicalMutation(t *testing.T) {
	scope := memory.Scope{Tenant: "tenant-a", Project: "project-a", Namespace: "namespace-a"}
	policy := DefaultReservedInsightActivationPolicy(scope)
	policy.Enabled = true
	policy.EnabledTypes[memory.DerivedInsightTypeHypothesis] = true
	policy.ExpiresAt = time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC)
	policy.MinEvidence = 1
	candidate := testHypothesisCandidate(scope)

	result := AdmitReservedInsight(ActivationInput{
		Policy:             policy,
		Candidate:          candidate,
		AuthorizedEvidence: candidate.Evidence,
		Now:                time.Date(2026, 9, 29, 0, 0, 0, 0, time.UTC),
	})
	if result.Disposition != ActivationDispositionActivated {
		t.Fatalf("disposition = %s, want activated", result.Disposition)
	}
	if result.Insight.State != memory.DerivedInsightStateActive {
		t.Fatalf("insight state = %s, want active", result.Insight.State)
	}
	if result.Insight.Type != memory.DerivedInsightTypeHypothesis {
		t.Fatalf("insight type = %s, want hypothesis", result.Insight.Type)
	}
	if result.Insight.Derivation.Metadata["activation_policy_version"] != policy.Version {
		t.Fatalf("activation metadata = %+v, want policy version", result.Insight.Derivation.Metadata)
	}
}

func TestReservedInsightAdmissionRejectsOverBudgetAndDuplicateCandidates(t *testing.T) {
	scope := memory.Scope{Tenant: "tenant-a", Project: "project-a", Namespace: "namespace-a"}
	policy := DefaultReservedInsightActivationPolicy(scope)
	policy.Enabled = true
	policy.EnabledTypes[memory.DerivedInsightTypeHypothesis] = true
	policy.ExpiresAt = time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC)
	policy.MaxEvidence = 1
	candidate := testHypothesisCandidate(scope)
	candidate.Evidence = append(candidate.Evidence, candidate.Evidence[0])

	result := AdmitReservedInsight(ActivationInput{
		Policy:             policy,
		Candidate:          candidate,
		AuthorizedEvidence: candidate.Evidence,
		Now:                time.Date(2026, 9, 29, 0, 0, 0, 0, time.UTC),
	})
	if result.Disposition != ActivationDispositionRejected {
		t.Fatalf("over-budget disposition = %s, want rejected", result.Disposition)
	}

	candidate = testHypothesisCandidate(scope)
	fingerprint := candidateFingerprint(candidate, policy)
	result = AdmitReservedInsight(ActivationInput{
		Policy:               policy,
		Candidate:            candidate,
		AuthorizedEvidence:   candidate.Evidence,
		ExistingFingerprints: map[string]struct{}{fingerprint: {}},
		Now:                  time.Date(2026, 9, 29, 0, 0, 0, 0, time.UTC),
	})
	if result.Disposition != ActivationDispositionDuplicate {
		t.Fatalf("duplicate disposition = %s, want duplicate", result.Disposition)
	}
}

func TestReservedInsightActivationServicePersistsOnlyAdmittedDerivedInsight(t *testing.T) {
	scope := memory.Scope{Tenant: "tenant-a", Project: "project-a", Namespace: "namespace-a"}
	policy := DefaultReservedInsightActivationPolicy(scope)
	policy.Enabled = true
	policy.EnabledTypes[memory.DerivedInsightTypeHypothesis] = true
	policy.ExpiresAt = time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC)
	store := &activationStoreStub{}
	service := ReservedInsightActivationService{Store: store}
	candidate := testHypothesisCandidate(scope)

	result, err := service.Apply(context.Background(), ActivationInput{
		Policy:             policy,
		Candidate:          candidate,
		AuthorizedEvidence: candidate.Evidence,
		Now:                time.Date(2026, 9, 29, 0, 0, 0, 0, time.UTC),
	})
	if err != nil {
		t.Fatalf("Apply() error = %v", err)
	}
	if result.Disposition != ActivationDispositionActivated {
		t.Fatalf("disposition = %s, want activated", result.Disposition)
	}
	if len(store.stored) != 1 || store.stored[0].State != memory.DerivedInsightStateActive {
		t.Fatalf("stored = %+v, want one active derived insight", store.stored)
	}
	if len(store.decisions) != 1 || store.decisions[0].Disposition != ActivationDispositionActivated {
		t.Fatalf("decisions = %+v, want one activated decision", store.decisions)
	}
	if len(store.evidence) != len(candidate.Evidence) {
		t.Fatalf("decision evidence = %+v, want %d links", store.evidence, len(candidate.Evidence))
	}
}

func TestReservedInsightActivationServiceRecordsRejectedDecisionWithoutInsightMutation(t *testing.T) {
	scope := memory.Scope{Tenant: "tenant-a", Project: "project-a", Namespace: "namespace-a"}
	policy := DefaultReservedInsightActivationPolicy(scope)
	store := &activationStoreStub{}
	service := ReservedInsightActivationService{Store: store}
	candidate := testHypothesisCandidate(scope)
	result, err := service.Apply(context.Background(), ActivationInput{
		Policy:             policy,
		Candidate:          candidate,
		AuthorizedEvidence: candidate.Evidence,
		Now:                time.Date(2026, 9, 29, 0, 0, 0, 0, time.UTC),
	})
	if err != nil {
		t.Fatalf("Apply() error = %v, want nil for bounded disabled disposition", err)
	}
	if result.Disposition != ActivationDispositionTypeDisabled {
		t.Fatalf("disposition = %s, want type_disabled", result.Disposition)
	}
	if len(store.stored) != 0 || len(store.decisions) != 1 {
		t.Fatalf("stored=%d decisions=%d, want no insight and one decision", len(store.stored), len(store.decisions))
	}
}

func TestReservedInsightActivationPolicyStopAndRollbackDisableNewAdmissions(t *testing.T) {
	scope := memory.Scope{Tenant: "tenant-a", Project: "project-a", Namespace: "namespace-a"}
	policy := DefaultReservedInsightActivationPolicy(scope)
	policy.Enabled = true
	policy.EnabledTypes[memory.DerivedInsightTypeHypothesis] = true
	policy.ExpiresAt = time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC)

	stopped := policy.Stopped()
	if stopped.Enabled || stopped.RolledBack {
		t.Fatalf("stopped policy = %+v, want disabled and not rolled back", stopped)
	}
	rolledBack := policy.RolledBackCopy()
	if rolledBack.Enabled || !rolledBack.RolledBack {
		t.Fatalf("rolled back policy = %+v, want disabled and rolled back", rolledBack)
	}
	if policy.Enabled == false || policy.RolledBack {
		t.Fatal("stop/rollback mutated original policy")
	}
}

func TestGoalActivationPolicyRequiresReviewConfiguration(t *testing.T) {
	scope := memory.Scope{Tenant: "tenant-a", Project: "project-a", Namespace: "namespace-a"}
	now := time.Date(2026, 9, 29, 0, 0, 0, 0, time.UTC)
	policy := DefaultReservedInsightActivationPolicy(scope)
	policy.Enabled = true
	policy.EnabledTypes[memory.DerivedInsightTypeGoal] = true
	policy.ExpiresAt = now.Add(24 * time.Hour)
	if err := policy.ValidateAt(now); err == nil {
		t.Fatal("goal policy without review configuration accepted")
	}
}

func TestGoalActivationRemainsReviewOnlyByDefault(t *testing.T) {
	scope := memory.Scope{Tenant: "tenant-a", Project: "project-a", Namespace: "namespace-a"}
	now := time.Date(2026, 9, 29, 0, 0, 0, 0, time.UTC)
	policy := DefaultReservedInsightActivationPolicy(scope)
	policy.Enabled = true
	policy.EnabledTypes[memory.DerivedInsightTypeGoal] = true
	policy.GoalRequireReview = true
	policy.GoalAllowedStates[reasoning.GoalStateProposed] = true
	policy.ExpiresAt = now.Add(24 * time.Hour)
	candidate := testHypothesisCandidate(scope)
	candidate.ID = "goal-1"
	candidate.Type = memory.DerivedInsightTypeGoal
	candidate.Title = "Bounded goal"
	candidate.Summary = "Goal remains review-only"
	candidate.Derivation.Metadata["goal_state"] = string(reasoning.GoalStateProposed)
	candidate.Derivation.Metadata["goal_review_state"] = string(reasoning.GoalReviewRequired)
	result := AdmitReservedInsight(ActivationInput{Policy: policy, Candidate: candidate, AuthorizedEvidence: candidate.Evidence, Now: now})
	if result.Disposition != ActivationDispositionReviewRequired {
		t.Fatalf("goal disposition = %s, want review_required (%s)", result.Disposition, result.Reason)
	}
}

func TestReservedInsightAdmissionRequiresContradictionOverlapAndReview(t *testing.T) {
	scope := memory.Scope{Tenant: "tenant-a", Project: "project-a", Namespace: "namespace-a"}
	policy := DefaultReservedInsightActivationPolicy(scope)
	policy.Enabled = true
	policy.EnabledTypes[memory.DerivedInsightTypeContradiction] = true
	policy.ExpiresAt = time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC)
	policy.ContradictionRequireReview = true
	candidate := testHypothesisCandidate(scope)
	candidate.Type = memory.DerivedInsightTypeContradiction
	candidate.ID = "insight_contradiction_1"
	candidate.Title = "Contradictory facts"
	candidate.Summary = "Two facts overlap"
	candidate.Evidence = append(candidate.Evidence, memory.DerivedInsightEvidenceRef{Kind: memory.DerivedInsightEvidenceKindCanonicalMemory, ID: "memory-2", Relation: memory.DerivedInsightEvidenceRelationSupports})
	candidate.Derivation.Metadata = map[string]any{
		"contradiction_temporal_disposition": "contradiction",
		"contradiction_review_state":         "confirmed",
	}
	result := AdmitReservedInsight(ActivationInput{Policy: policy, Candidate: candidate, AuthorizedEvidence: candidate.Evidence, Now: time.Date(2026, 9, 29, 0, 0, 0, 0, time.UTC)})
	if result.Disposition != ActivationDispositionActivated {
		t.Fatalf("disposition = %s, want activated: %s", result.Disposition, result.Reason)
	}

	candidate.Derivation.Metadata["contradiction_review_state"] = "review_required"
	result = AdmitReservedInsight(ActivationInput{Policy: policy, Candidate: candidate, AuthorizedEvidence: candidate.Evidence, Now: time.Date(2026, 9, 29, 0, 0, 0, 0, time.UTC)})
	if result.Disposition != ActivationDispositionRejected {
		t.Fatalf("review-required disposition = %s, want rejected", result.Disposition)
	}
}

func testHypothesisCandidate(scope memory.Scope) memory.DerivedInsight {
	now := time.Date(2026, 9, 29, 0, 0, 0, 0, time.UTC)
	return memory.DerivedInsight{
		ID:         "insight_hypothesis_1",
		Scope:      scope,
		Type:       memory.DerivedInsightTypeHypothesis,
		State:      memory.DerivedInsightStateCandidate,
		Title:      "Provider may be unavailable during restart",
		Summary:    "Repeated restart failures suggest a provider availability hypothesis.",
		Confidence: memory.DerivedInsightConfidence{Score: 0.8, Method: "bounded_reasoning"},
		Derivation: memory.DerivedInsightDerivation{Source: "reasoning_provider", Fingerprint: "candidate:hypothesis:1", DerivedAt: now, Metadata: map[string]any{"provider_contract_version": "reasoning-v1"}},
		Evidence: []memory.DerivedInsightEvidenceRef{{
			Kind:     memory.DerivedInsightEvidenceKindJobExecution,
			ID:       "job-1",
			Relation: memory.DerivedInsightEvidenceRelationSupports,
		}},
		CreatedAt: now,
		UpdatedAt: now,
	}
}

func TestAdmitReasoningCandidateUsesExistingPolicyAndKeepsShadowNonAuthoritative(t *testing.T) {
	scope := memory.Scope{Tenant: "tenant-a", Project: "project-a", Namespace: "namespace-a"}
	now := time.Date(2026, 9, 29, 0, 0, 0, 0, time.UTC)
	evidence := memory.DerivedInsightEvidenceRef{Kind: memory.DerivedInsightEvidenceKindJobExecution, ID: "job-1", Relation: memory.DerivedInsightEvidenceRelationSupports}
	request := reasoning.InsightDerivationRequest{Scope: scope, InsightType: memory.DerivedInsightTypeHypothesis, Mode: reasoning.ModeShadow, Evidence: []memory.DerivedInsightEvidenceRef{evidence}, SourceWatermark: "wm-1", ScopeProof: "proof-1", LifecycleVisibility: "active_only", RedactionPolicy: "references_only", ProviderVersion: "provider-1", SchemaVersion: reasoning.SchemaVersionV1, PolicyVersion: ReservedInsightActivationPolicyVersion, InputDigest: "input-1", Limits: reasoning.DefaultLimits(), Now: now}
	digest, err := reasoning.EvidenceDigest(request.Evidence)
	if err != nil {
		t.Fatal(err)
	}
	replayID, err := reasoning.InsightReplayID(request)
	if err != nil {
		t.Fatal(err)
	}
	candidate := reasoning.InsightCandidate{ID: "reasoning-candidate-1", Scope: scope, InsightType: request.InsightType, Title: "Bounded hypothesis", Summary: "Evidence-backed hypothesis", Evidence: request.Evidence, EvidenceDigest: digest, SourceWatermark: request.SourceWatermark, ScopeProof: request.ScopeProof, LifecycleVisibility: request.LifecycleVisibility, RedactionPolicy: request.RedactionPolicy, ProviderVersion: request.ProviderVersion, SchemaVersion: request.SchemaVersion, PolicyVersion: request.PolicyVersion, ReplayID: replayID, Uncertainty: 0.2, Mode: request.Mode, CreatedAt: now}
	policy := DefaultReservedInsightActivationPolicy(scope)
	policy.Enabled = true
	policy.EnabledTypes[memory.DerivedInsightTypeHypothesis] = true
	policy.ExpiresAt = now.Add(24 * time.Hour)
	result := AdmitReasoningCandidate(candidate, policy, request.Evidence, ActivationInput{Policy: policy, Shadow: true, Now: now})
	if result.Disposition != ActivationDispositionWouldActivate {
		t.Fatalf("shadow reasoning disposition = %s, want would_activate (%v)", result.Disposition, result.Reason)
	}
	if result.Insight.State != memory.DerivedInsightStateCandidate {
		t.Fatalf("shadow reasoning state = %s, want candidate", result.Insight.State)
	}
}

func TestReasoningActivationStopsAfterPolicyRollback(t *testing.T) {
	scope := memory.Scope{Tenant: "tenant-a", Project: "project-a", Namespace: "namespace-a"}
	now := time.Date(2026, 9, 29, 0, 0, 0, 0, time.UTC)
	policy := DefaultReservedInsightActivationPolicy(scope)
	policy.Enabled = true
	policy.EnabledTypes[memory.DerivedInsightTypeHypothesis] = true
	policy.ExpiresAt = now.Add(24 * time.Hour)
	rolledBack := policy.RolledBackCopy()
	result := AdmitReservedInsight(ActivationInput{Policy: rolledBack, Candidate: testHypothesisCandidate(scope), Now: now})
	if result.Disposition != ActivationDispositionStale {
		t.Fatalf("rolled back reasoning policy disposition = %s, want stale", result.Disposition)
	}
}

func TestApplyReasoningCandidatePersistsOnlyAfterPolicyAdmission(t *testing.T) {
	scope := memory.Scope{Tenant: "tenant-a", Project: "project-a", Namespace: "namespace-a"}
	now := time.Date(2026, 9, 29, 0, 0, 0, 0, time.UTC)
	evidence := memory.DerivedInsightEvidenceRef{Kind: memory.DerivedInsightEvidenceKindJobExecution, ID: "job-1", Relation: memory.DerivedInsightEvidenceRelationSupports}
	request := reasoning.InsightDerivationRequest{Scope: scope, InsightType: memory.DerivedInsightTypeHypothesis, Mode: reasoning.ModeOffline, Evidence: []memory.DerivedInsightEvidenceRef{evidence}, SourceWatermark: "wm-1", ScopeProof: "proof-1", LifecycleVisibility: "active_only", RedactionPolicy: "references_only", ProviderVersion: "provider-1", SchemaVersion: reasoning.SchemaVersionV1, PolicyVersion: ReservedInsightActivationPolicyVersion, InputDigest: "input-1", Limits: reasoning.DefaultLimits(), Now: now}
	digest, _ := reasoning.EvidenceDigest(request.Evidence)
	replayID, _ := reasoning.InsightReplayID(request)
	candidate := reasoning.InsightCandidate{ID: "reasoning-candidate-apply", Scope: scope, InsightType: request.InsightType, Title: "Bounded hypothesis", Summary: "Evidence-backed hypothesis", Evidence: request.Evidence, EvidenceDigest: digest, SourceWatermark: request.SourceWatermark, ScopeProof: request.ScopeProof, LifecycleVisibility: request.LifecycleVisibility, RedactionPolicy: request.RedactionPolicy, ProviderVersion: request.ProviderVersion, SchemaVersion: request.SchemaVersion, PolicyVersion: request.PolicyVersion, ReplayID: replayID, Uncertainty: 0.2, Mode: request.Mode, CreatedAt: now}
	policy := DefaultReservedInsightActivationPolicy(scope)
	policy.Enabled = true
	policy.EnabledTypes[memory.DerivedInsightTypeHypothesis] = true
	policy.ExpiresAt = now.Add(24 * time.Hour)
	store := &activationStoreStub{}
	result, err := (ReservedInsightActivationService{Store: store}).ApplyReasoningCandidate(context.Background(), candidate, policy, request.Evidence, now)
	if err != nil || result.Disposition != ActivationDispositionActivated {
		t.Fatalf("apply reasoning candidate result=%+v err=%v", result, err)
	}
	if len(store.stored) != 1 || len(store.decisions) != 1 || store.stored[0].State != memory.DerivedInsightStateActive {
		t.Fatalf("stored=%d decisions=%d insight=%+v", len(store.stored), len(store.decisions), store.stored)
	}
}

func TestApplyReasoningCandidateRetryIsIdempotentAtAdmissionBoundary(t *testing.T) {
	scope := memory.Scope{Tenant: "tenant-a", Project: "project-a", Namespace: "namespace-a"}
	now := time.Date(2026, 9, 29, 0, 0, 0, 0, time.UTC)
	evidence := memory.DerivedInsightEvidenceRef{Kind: memory.DerivedInsightEvidenceKindJobExecution, ID: "job-1", Relation: memory.DerivedInsightEvidenceRelationSupports}
	request := reasoning.InsightDerivationRequest{Scope: scope, InsightType: memory.DerivedInsightTypeHypothesis, Mode: reasoning.ModeOffline, Evidence: []memory.DerivedInsightEvidenceRef{evidence}, SourceWatermark: "wm-1", ScopeProof: "proof-1", LifecycleVisibility: "active_only", RedactionPolicy: "references_only", ProviderVersion: "provider-1", SchemaVersion: reasoning.SchemaVersionV1, PolicyVersion: ReservedInsightActivationPolicyVersion, InputDigest: "input-1", Limits: reasoning.DefaultLimits(), Now: now}
	digest, _ := reasoning.EvidenceDigest(request.Evidence)
	replayID, _ := reasoning.InsightReplayID(request)
	candidate := reasoning.InsightCandidate{ID: "reasoning-candidate-retry", Scope: scope, InsightType: request.InsightType, Title: "Bounded hypothesis", Summary: "Evidence-backed hypothesis", Evidence: request.Evidence, EvidenceDigest: digest, SourceWatermark: request.SourceWatermark, ScopeProof: request.ScopeProof, LifecycleVisibility: request.LifecycleVisibility, RedactionPolicy: request.RedactionPolicy, ProviderVersion: request.ProviderVersion, SchemaVersion: request.SchemaVersion, PolicyVersion: request.PolicyVersion, ReplayID: replayID, Uncertainty: 0.2, Mode: request.Mode, CreatedAt: now}
	policy := DefaultReservedInsightActivationPolicy(scope)
	policy.Enabled = true
	policy.EnabledTypes[memory.DerivedInsightTypeHypothesis] = true
	policy.ExpiresAt = now.Add(24 * time.Hour)
	store := &activationStoreStub{}
	service := ReservedInsightActivationService{Store: store}
	first, err := service.ApplyReasoningCandidate(context.Background(), candidate, policy, request.Evidence, now)
	if err != nil || first.Disposition != ActivationDispositionActivated {
		t.Fatalf("first apply result=%+v err=%v", first, err)
	}
	second, err := service.ApplyReasoningCandidate(context.Background(), candidate, policy, request.Evidence, now)
	if err != nil || second.Disposition != ActivationDispositionDuplicate {
		t.Fatalf("retry result=%+v err=%v", second, err)
	}
	if len(store.stored) != 1 || store.stored[0].ID != candidate.ID {
		t.Fatalf("retry changed insight identity: %+v", store.stored)
	}
}
