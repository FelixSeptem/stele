package insights

import (
	"context"
	"testing"
	"time"

	"github.com/FelixSeptem/stele/internal/memory"
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
