package retrieval

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/FelixSeptem/stele/internal/memory"
)

func TestBuildProgressiveExperimentReportIsDeterministicAndShadowOnly(t *testing.T) {
	scope := memory.Scope{Tenant: "tenant", Project: "project", Namespace: "namespace"}
	now := time.Date(2026, 10, 2, 10, 0, 0, 0, time.UTC)
	projection := progressiveTestProjection(scope, now)
	input := ProgressiveExperimentInput{
		Policy: ProgressiveExperimentPolicy{
			LevelPolicyVersion: "levels-v1", RendererVersion: "renderer-v1",
			BaselineIdentity: "flat-fusion-v1", StrategyIdentity: "parent-first-v1",
			Mode: ProgressiveExperimentModeShadow, MaxAge: time.Hour,
			Limits: ProgressiveExperimentLimits{MaxCandidates: 4, MaxTokens: 32, MaxCharacters: 256},
		},
		Scope: scope, EvaluatedAt: now,
		Levels:      []ProgressiveContextLevelInput{{Identity: "l0-v1", Kind: ProgressiveContextLevelShortRetrieval, Projection: &projection, ExpectedWatermarkHash: projection.SourceWatermarkHash(), TokenBudget: 32, CharacterBudget: 256, ExpectedEvidence: 1}},
		ParentFirst: ParentFirstEvaluationInput{Scope: scope, StrategyIdentity: "parent-first-v1", Mode: ParentFirstModeShadow, Limits: ParentFirstExpansionLimits{MaxParents: 1, MaxChildrenPerParent: 1, MaxCandidates: 4, MaxAdjacentDistance: 1}, Baseline: ParentFirstMetrics{ProtectedRecall: 1, MultiHopCoverage: 1}, Candidate: ParentFirstMetrics{ProtectedRecall: 1, MultiHopCoverage: 1}},
	}
	first, err := ReplayProgressiveExperiment(input)
	if err != nil {
		t.Fatal(err)
	}
	second, err := BuildProgressiveExperimentReport(input)
	if err != nil {
		t.Fatal(err)
	}
	if first.RunIdentity == "" || first.RunIdentity != second.RunIdentity || first.PolicyIdentity != second.PolicyIdentity {
		t.Fatalf("identities are not deterministic: first=%+v second=%+v", first, second)
	}
	if first.SelectedStrategy != ParentFirstFlatStrategy || first.Eligible {
		t.Fatalf("shadow experiment must preserve baseline: %+v", first)
	}
	payload, err := json.Marshal(first)
	if err != nil {
		t.Fatal(err)
	}
	for _, forbidden := range []string{"memory-a", "two words", "tenant", "project", "namespace"} {
		if strings.Contains(string(payload), forbidden) {
			t.Fatalf("report leaked %q: %s", forbidden, payload)
		}
	}
}

func TestBuildProgressiveExperimentReportFailsClosedOnSafetyFailure(t *testing.T) {
	scope := memory.Scope{Tenant: "tenant", Project: "project", Namespace: "namespace"}
	now := time.Date(2026, 10, 2, 10, 0, 0, 0, time.UTC)
	projection := progressiveTestProjection(scope, now.Add(-2*time.Hour))
	report, err := BuildProgressiveExperimentReport(ProgressiveExperimentInput{
		Policy: ProgressiveExperimentPolicy{LevelPolicyVersion: "levels-v1", RendererVersion: "renderer-v1", BaselineIdentity: "flat-fusion-v1", StrategyIdentity: "parent-first-v1", Mode: ProgressiveExperimentModeOffline, MaxAge: time.Hour},
		Scope:  scope, EvaluatedAt: now,
		Levels: []ProgressiveContextLevelInput{{Identity: "l0-v1", Kind: ProgressiveContextLevelShortRetrieval, Projection: &projection, ExpectedWatermarkHash: projection.SourceWatermarkHash(), TokenBudget: 32, CharacterBudget: 256}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if report.Eligible || report.SelectedStrategy != ParentFirstFlatStrategy || report.Verdict != ProgressiveExperimentNonPass {
		t.Fatalf("unsafe experiment must fail closed: %+v", report)
	}
}

func TestValidateProgressiveExperimentEvidenceRejectsUnsafeOrForeignReports(t *testing.T) {
	scope := memory.Scope{Tenant: "tenant", Project: "project", Namespace: "namespace"}
	report := ProgressiveExperimentReport{ScopeHash: scopeHash(scope), SelectedStrategy: ParentFirstFlatStrategy, Verdict: ProgressiveExperimentShadow}
	if err := ValidateProgressiveExperimentEvidence(scope, &report); err != nil {
		t.Fatalf("valid shadow report rejected: %v", err)
	}
	foreign := scope
	foreign.Tenant = "foreign"
	if err := ValidateProgressiveExperimentEvidence(foreign, &report); err == nil {
		t.Fatal("foreign experiment report accepted")
	}
	report.RollbackRequired = true
	if err := ValidateProgressiveExperimentEvidence(scope, &report); err == nil {
		t.Fatal("rollback-required experiment report accepted")
	}
}
