package retrieval

import (
	"testing"
	"time"

	"github.com/FelixSeptem/stele/internal/memory"
)

func TestEvaluateParentFirstShadowUsesBoundedExactScopeExpansionAndComparesBaseline(t *testing.T) {
	scope := memory.Scope{Tenant: "tenant", Project: "project", Namespace: "namespace"}
	parent := parentFirstTestCandidate(scope, "parent-a", 0.9)
	children := []ParentFirstChild{
		{ParentID: "parent-a", AdjacentDistance: 0, Evidence: progressiveTestEvidence(scope, "child-a", "memory-a", "alpha")},
		{ParentID: "parent-a", AdjacentDistance: 1, Evidence: progressiveTestEvidence(scope, "child-b", "memory-b", "beta")},
		{ParentID: "parent-a", AdjacentDistance: 2, Evidence: progressiveTestEvidence(scope, "child-c", "memory-c", "gamma")},
	}
	report, err := EvaluateParentFirst(ParentFirstEvaluationInput{Scope: scope, StrategyIdentity: "parent-first-v1", Mode: ParentFirstModeShadow, Parents: []ParentFirstCandidate{parent}, Children: children, Limits: ParentFirstExpansionLimits{MaxParents: 1, MaxChildrenPerParent: 2, MaxAdjacentDistance: 1, MaxCandidates: 3}, Baseline: ParentFirstMetrics{ProtectedRecall: .5, MultiHopCoverage: .5, DuplicateRate: .5, P95LatencyMS: 20, CandidateCount: 2}, Candidate: ParentFirstMetrics{ProtectedRecall: .8, MultiHopCoverage: .7, DuplicateRate: .1, P95LatencyMS: 15, CandidateCount: 3}, Gates: ParentFirstGates{MaxRecallRegression: 0, MaxMultiHopRegression: 0, MaxDuplicateRate: .5, MaxP95LatencyMS: 50}})
	if err != nil {
		t.Fatal(err)
	}
	if !report.Eligible || report.Mode != ParentFirstModeShadow || len(report.Expanded) != 2 {
		t.Fatalf("report = %+v", report)
	}
	if report.Diagnostics.ParentsConsidered != 1 || report.Diagnostics.ChildrenIncluded != 2 || report.Diagnostics.OmittedByDistance != 1 {
		t.Fatalf("diagnostics = %+v", report.Diagnostics)
	}
	if report.Deltas.ProtectedRecall != .3 || report.Deltas.MultiHopCoverage != .2 {
		t.Fatalf("deltas = %+v", report.Deltas)
	}
}

func TestEvaluateParentFirstRejectsForeignAndHiddenExpansion(t *testing.T) {
	scope := memory.Scope{Tenant: "tenant", Project: "project", Namespace: "namespace"}
	foreign := memory.Scope{Tenant: "foreign", Project: "project", Namespace: "namespace"}
	children := []ParentFirstChild{
		{ParentID: "parent-a", Evidence: progressiveTestEvidence(foreign, "foreign", "memory-f", "foreign")},
		{ParentID: "parent-a", Evidence: progressiveTestEvidence(scope, "hidden", "memory-h", "hidden")},
	}
	children[1].Evidence.Memory.State = memory.MemoryStateForgotten
	report, err := EvaluateParentFirst(ParentFirstEvaluationInput{Scope: scope, StrategyIdentity: "parent-first-v1", Mode: ParentFirstModeShadow, Parents: []ParentFirstCandidate{parentFirstTestCandidate(scope, "parent-a", 1)}, Children: children, Limits: ParentFirstExpansionLimits{MaxParents: 1, MaxChildrenPerParent: 2, MaxAdjacentDistance: 1, MaxCandidates: 3}, Baseline: ParentFirstMetrics{}, Candidate: ParentFirstMetrics{}, Gates: ParentFirstGates{MaxDuplicateRate: 1, MaxP95LatencyMS: 100}})
	if err != nil {
		t.Fatal(err)
	}
	if report.Eligible || report.Diagnostics.IsolationFailures != 1 || report.Diagnostics.LifecycleFailures != 1 {
		t.Fatalf("report = %+v", report)
	}
	if !containsParentFirstFailure(report.HardFailures, ParentFirstFailureIsolation) || !containsParentFirstFailure(report.HardFailures, ParentFirstFailureLifecycle) {
		t.Fatalf("hard failures = %v", report.HardFailures)
	}
}

func TestEvaluateParentFirstDisableAndRollbackSelectFlatWithoutRewritingSources(t *testing.T) {
	scope := memory.Scope{Tenant: "tenant", Project: "project", Namespace: "namespace"}
	parent := parentFirstTestCandidate(scope, "parent-a", 1)
	child := ParentFirstChild{ParentID: "parent-a", Evidence: progressiveTestEvidence(scope, "child", "memory-a", "alpha")}
	for _, mode := range []ParentFirstMode{ParentFirstModeDisabled, ParentFirstModeRollback} {
		report, err := EvaluateParentFirst(ParentFirstEvaluationInput{Scope: scope, StrategyIdentity: "parent-first-v1", Mode: mode, Parents: []ParentFirstCandidate{parent}, Children: []ParentFirstChild{child}, Limits: ParentFirstExpansionLimits{MaxParents: 1, MaxChildrenPerParent: 1, MaxAdjacentDistance: 1, MaxCandidates: 2}, Gates: ParentFirstGates{MaxDuplicateRate: 1, MaxP95LatencyMS: 100}})
		if err != nil {
			t.Fatal(err)
		}
		if report.Eligible || report.SelectedStrategy != ParentFirstFlatStrategy || len(report.Expanded) != 0 || report.Diagnostics.Disabled != 1 {
			t.Fatalf("mode %q report = %+v", mode, report)
		}
		if child.Evidence.Memory.State != memory.MemoryStateActive || child.Evidence.Memory.Content != "alpha" {
			t.Fatal("evaluation mutated canonical source")
		}
	}
}

func TestEvaluateParentFirstQualityOrBudgetRegressionFailsClosed(t *testing.T) {
	scope := memory.Scope{Tenant: "tenant", Project: "project", Namespace: "namespace"}
	report, err := EvaluateParentFirst(ParentFirstEvaluationInput{Scope: scope, StrategyIdentity: "parent-first-v1", Mode: ParentFirstModeShadow, Parents: []ParentFirstCandidate{parentFirstTestCandidate(scope, "parent-a", 1)}, Limits: ParentFirstExpansionLimits{MaxParents: 1, MaxChildrenPerParent: 1, MaxAdjacentDistance: 1, MaxCandidates: 2}, Baseline: ParentFirstMetrics{ProtectedRecall: .9, MultiHopCoverage: .8}, Candidate: ParentFirstMetrics{ProtectedRecall: .5, MultiHopCoverage: .4, DuplicateRate: .8, P95LatencyMS: 200, CandidateCount: 3}, Gates: ParentFirstGates{MaxRecallRegression: .1, MaxMultiHopRegression: .1, MaxDuplicateRate: .5, MaxP95LatencyMS: 100}})
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []ParentFirstFailureReason{ParentFirstFailureRecall, ParentFirstFailureMultiHop, ParentFirstFailureDuplicate, ParentFirstFailureLatency, ParentFirstFailureCandidateBudget} {
		if !containsParentFirstFailure(report.HardFailures, want) {
			t.Fatalf("failures = %v, missing %q", report.HardFailures, want)
		}
	}
}

func parentFirstTestCandidate(scope memory.Scope, id string, score float64) ParentFirstCandidate {
	return ParentFirstCandidate{ID: id, Scope: scope, LifecycleState: memory.MemoryStateActive, Score: score, ProjectionStatus: memory.ContextProjectionStatusActive, UpdatedAt: time.Date(2026, 9, 13, 10, 0, 0, 0, time.UTC)}
}
