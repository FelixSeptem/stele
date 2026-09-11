package memory

import (
	"strings"
	"testing"
	"time"
)

func TestRankingRolloutActivationGate(t *testing.T) {
	gate := RankingRolloutActivationGate{
		DryRunSucceeded:         true,
		EvidenceThresholdStatus: RankingRolloutThresholdStatusSatisfied,
		BlockersPresent:         false,
		AttributionRecorded:     true,
	}

	if !gate.CanActivate() {
		t.Fatal("CanActivate() = false, want true")
	}

	gate.BlockersPresent = true
	if gate.CanActivate() {
		t.Fatal("CanActivate() = true, want false when blockers are present")
	}
}

func TestRankingRolloutPolicyValidate(t *testing.T) {
	policy := RankingRolloutPolicy{
		ID:              "policy_1",
		Scope:           Scope{Tenant: "tenant-a", Project: "project-a", Namespace: "namespace-a"},
		Status:          RankingRolloutPolicyStatusDraft,
		Mode:            RankingRolloutModeDryRun,
		Surfaces:        []RankingRolloutSurface{RankingRolloutSurfaceSearch},
		SignalSources:   []RankingRolloutSignalSource{RankingRolloutSignalSourceTaskEvaluations},
		ThresholdStatus: RankingRolloutThresholdStatusSatisfied,
		EvidenceMinimum: 2,
		Actor:           "operator-a",
		Reason:          "enable dry-run",
		CreatedAt:       time.Date(2026, 7, 12, 8, 20, 0, 0, time.UTC),
		UpdatedAt:       time.Date(2026, 7, 12, 8, 20, 0, 0, time.UTC),
	}

	if err := policy.Validate(); err != nil {
		t.Fatalf("Validate() error = %v", err)
	}

	policy.Mode = RankingRolloutMode("free_form")
	if err := policy.Validate(); err == nil {
		t.Fatal("Validate() error = nil, want invalid rollout mode")
	}
}

func TestRankingRolloutPolicyValidateAcceptsExplicitFusionStrategy(t *testing.T) {
	policy := validRankingRolloutPolicyForTest()
	policy.FusionStrategy = "rrf"
	policy.FusionVersion = "rrf-v1"
	policy.FusionRankConstant = 60
	policy.FusionPerChannelCandidate = 50
	policy.FusionTotalCandidates = 200
	policy.FusionChannelWeights = map[string]float64{"lexical": 1, "semantic": 1}
	if err := policy.Validate(); err != nil {
		t.Fatalf("Validate() error = %v, want explicit fusion strategy accepted", err)
	}
}

func TestRankingRolloutPolicyValidateRejectsInvalidFusionStrategy(t *testing.T) {
	policy := validRankingRolloutPolicyForTest()
	policy.FusionStrategy = "rrf"
	policy.FusionVersion = ""
	if err := policy.Validate(); err == nil {
		t.Fatal("Validate() error = nil, want missing fusion version")
	}
}

func validRankingRolloutPolicyForTest() RankingRolloutPolicy {
	now := time.Date(2026, 9, 7, 12, 0, 0, 0, time.UTC)
	return RankingRolloutPolicy{
		ID: "policy-1", Scope: Scope{Tenant: "tenant-a", Project: "project-a", Namespace: "namespace-a"},
		Status: RankingRolloutPolicyStatusDraft, Mode: RankingRolloutModeDryRun,
		Surfaces:        []RankingRolloutSurface{RankingRolloutSurfaceSearch},
		SignalSources:   []RankingRolloutSignalSource{RankingRolloutSignalSourceUsefulnessFeedback},
		ThresholdStatus: RankingRolloutThresholdStatusSatisfied, Actor: "operator", Reason: "test",
		CreatedAt: now, UpdatedAt: now,
	}
}

func TestRecordRankingRolloutDryRunInputValidateAcceptsComparisonFields(t *testing.T) {
	input := RecordRankingRolloutDryRunInput{
		PolicyID:            "policy_1",
		Scope:               Scope{Tenant: "tenant-a", Project: "project-a", Namespace: "namespace-a"},
		Surface:             RankingRolloutSurfaceContext,
		SignalSource:        RankingRolloutSignalSourceTaskEvaluations,
		ThresholdStatus:     RankingRolloutThresholdStatusSatisfied,
		BaselineRank:        1,
		AdjustedRank:        2,
		ChangedSubjectIDs:   []string{"mem_1"},
		ReasonCodes:         []RankingRolloutImpactReasonCode{RankingRolloutImpactReasonCodeSubjectBoosted},
		SignalCategories:    []string{"task_evaluations"},
		EvidenceCount:       3,
		HiddenEvidenceCount: 1,
		CreatedAt:           time.Date(2026, 7, 12, 8, 30, 0, 0, time.UTC),
	}

	if err := input.Validate(); err != nil {
		t.Fatalf("Validate() error = %v", err)
	}
}

func validQueryAnalysisRolloutPolicyForTest(status RankingRolloutPolicyStatus) RankingRolloutPolicy {
	policy := validRankingRolloutPolicyForTest()
	policy.Status = status
	switch status {
	case RankingRolloutPolicyStatusDiagnosticsOnly:
		policy.Mode = RankingRolloutModeDiagnosticsOnly
	case RankingRolloutPolicyStatusDryRun:
		policy.Mode = RankingRolloutModeDryRun
	case RankingRolloutPolicyStatusActiveForScope:
		policy.Mode = RankingRolloutModeActiveForScope
	}
	policy.QueryAnalysis = &QueryAnalysisRolloutPolicy{
		SchemaVersion: "query-analysis-rollout-v1",
		PolicyVersion: "query-analysis-v1",
		LimitsVersion: "query-analysis-limits-v1",
		MaxQueryBytes: 4096, MaxHints: 4, MaxSignals: 8, MaxSubqueries: 4,
		MaxTermBytes: 256, MaxSubqueryBytes: 1024, MaxAnalysisWork: 7,
		MaxCandidatesPerSignal: 50, MaxAggregateCandidates: 200,
		MaxElapsed: 250 * time.Millisecond,
		ExpiresAt:  time.Date(2026, 9, 8, 12, 0, 0, 0, time.UTC),
	}
	return policy
}

func TestResolveQueryAnalysisRolloutFailClosedAcrossLifecycle(t *testing.T) {
	now := time.Date(2026, 9, 7, 12, 0, 0, 0, time.UTC)
	scope := Scope{Tenant: "tenant-a", Project: "project-a", Namespace: "namespace-a"}
	tests := []struct {
		name       string
		policy     *RankingRolloutPolicy
		wantStage  QueryAnalysisRolloutStage
		wantEffect bool
	}{
		{name: "missing", wantStage: QueryAnalysisRolloutStageOriginalOnly},
		{name: "diagnostics only", policy: ptrRankingRolloutPolicy(validQueryAnalysisRolloutPolicyForTest(RankingRolloutPolicyStatusDiagnosticsOnly)), wantStage: QueryAnalysisRolloutStageDiagnosticsOnly},
		{name: "shadow", policy: ptrRankingRolloutPolicy(validQueryAnalysisRolloutPolicyForTest(RankingRolloutPolicyStatusDryRun)), wantStage: QueryAnalysisRolloutStageShadow},
		{name: "active", policy: ptrRankingRolloutPolicy(validQueryAnalysisRolloutPolicyForTest(RankingRolloutPolicyStatusActiveForScope)), wantStage: QueryAnalysisRolloutStageActive, wantEffect: true},
		{name: "disabled", policy: ptrRankingRolloutPolicy(validQueryAnalysisRolloutPolicyForTest(RankingRolloutPolicyStatusDisabled)), wantStage: QueryAnalysisRolloutStageOriginalOnly},
		{name: "rollback", policy: ptrRankingRolloutPolicy(validQueryAnalysisRolloutPolicyForTest(RankingRolloutPolicyStatusRolledBack)), wantStage: QueryAnalysisRolloutStageOriginalOnly},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ResolveQueryAnalysisRollout(tt.policy, ResolveQueryAnalysisRolloutInput{Scope: scope, Surface: RankingRolloutSurfaceSearch, Now: now})
			if result.Stage != tt.wantStage || result.DerivedSignalsAffectResults != tt.wantEffect {
				t.Fatalf("ResolveQueryAnalysisRollout() = %+v, want stage %q effect %t", result, tt.wantStage, tt.wantEffect)
			}
			if !tt.wantEffect && !result.OriginalOnly {
				t.Fatalf("ResolveQueryAnalysisRollout() = %+v, want original-only", result)
			}
		})
	}
}

func TestResolveQueryAnalysisRolloutRejectsExpiredMalformedAndForeignPolicy(t *testing.T) {
	now := time.Date(2026, 9, 7, 12, 0, 0, 0, time.UTC)
	scope := Scope{Tenant: "tenant-a", Project: "project-a", Namespace: "namespace-a"}
	tests := []struct {
		name   string
		mutate func(*RankingRolloutPolicy)
	}{
		{name: "expired", mutate: func(p *RankingRolloutPolicy) { p.QueryAnalysis.ExpiresAt = now.Add(-time.Nanosecond) }},
		{name: "unknown version", mutate: func(p *RankingRolloutPolicy) { p.QueryAnalysis.SchemaVersion = "query-analysis-rollout-v2" }},
		{name: "unbounded", mutate: func(p *RankingRolloutPolicy) { p.QueryAnalysis.MaxSignals = 17 }},
		{name: "foreign tenant", mutate: func(p *RankingRolloutPolicy) { p.Scope.Tenant = "tenant-b" }},
		{name: "foreign project", mutate: func(p *RankingRolloutPolicy) { p.Scope.Project = "project-b" }},
		{name: "foreign namespace", mutate: func(p *RankingRolloutPolicy) { p.Scope.Namespace = "namespace-b" }},
		{name: "foreign session", mutate: func(p *RankingRolloutPolicy) { p.QueryAnalysisSelector.SessionID = "session-b" }},
		{name: "foreign user", mutate: func(p *RankingRolloutPolicy) { p.QueryAnalysisSelector.UserID = "user-b" }},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			policy := validQueryAnalysisRolloutPolicyForTest(RankingRolloutPolicyStatusActiveForScope)
			policy.QueryAnalysisSelector = QueryAnalysisRolloutSelector{SessionID: "session-a", UserID: "user-a"}
			tt.mutate(&policy)
			result := ResolveQueryAnalysisRollout(&policy, ResolveQueryAnalysisRolloutInput{Scope: scope, Surface: RankingRolloutSurfaceSearch, SessionID: "session-a", UserID: "user-a", Now: now})
			if !result.OriginalOnly || result.DerivedSignalsAffectResults {
				t.Fatalf("ResolveQueryAnalysisRollout() = %+v, want fail-closed original-only", result)
			}
		})
	}
}

func TestQueryAnalysisRolloutPolicyValidationRejectsPartialAndUnknownBundles(t *testing.T) {
	policy := validQueryAnalysisRolloutPolicyForTest(RankingRolloutPolicyStatusDryRun)
	policy.QueryAnalysis.PolicyVersion = ""
	if err := policy.Validate(); err == nil || !strings.Contains(err.Error(), "query-analysis") {
		t.Fatalf("Validate() error = %v, want query-analysis bundle rejection", err)
	}
	policy = validQueryAnalysisRolloutPolicyForTest(RankingRolloutPolicyStatusDryRun)
	policy.QueryAnalysis = nil
	policy.QueryAnalysisSelector.SessionID = "session-a"
	if err := policy.Validate(); err == nil {
		t.Fatal("Validate() error = nil, want selector without payload rejection")
	}
}

func TestResolveQueryAnalysisRolloutRequiresStatusModePair(t *testing.T) {
	now := time.Date(2026, 9, 7, 12, 0, 0, 0, time.UTC)
	scope := Scope{Tenant: "tenant-a", Project: "project-a", Namespace: "namespace-a"}
	for _, tc := range []struct {
		status RankingRolloutPolicyStatus
		mode   RankingRolloutMode
	}{
		{RankingRolloutPolicyStatusActiveForScope, RankingRolloutModeDryRun},
		{RankingRolloutPolicyStatusDryRun, RankingRolloutModeActiveForScope},
		{RankingRolloutPolicyStatusDiagnosticsOnly, RankingRolloutModeActiveForScope},
		{RankingRolloutPolicyStatusActiveForScope, RankingRolloutMode("unknown")},
	} {
		policy := validQueryAnalysisRolloutPolicyForTest(tc.status)
		policy.Mode = tc.mode
		resolved := ResolveQueryAnalysisRollout(&policy, ResolveQueryAnalysisRolloutInput{Scope: scope, Surface: RankingRolloutSurfaceSearch, Now: now})
		if !resolved.OriginalOnly || resolved.DerivedSignalsAffectResults {
			t.Fatalf("status=%q mode=%q resolved=%+v, want original-only", tc.status, tc.mode, resolved)
		}
	}
}

func ptrRankingRolloutPolicy(policy RankingRolloutPolicy) *RankingRolloutPolicy { return &policy }
