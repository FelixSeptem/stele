package retrieval

import (
	"strings"
	"testing"
	"time"

	"github.com/FelixSeptem/stele/internal/memory"
)

func TestRepresentationEvaluationKeepsSearchAndBaselineShadowOnly(t *testing.T) {
	scope := memory.Scope{Tenant: "tenant", Project: "benchmark-representations", Namespace: "ev3"}
	input := validRepresentationEvaluationInput(scope)

	report, err := EvaluateRepresentation(input)
	if err != nil {
		t.Fatal(err)
	}
	if report.Verdict != RepresentationEvaluationShadow || report.SelectedStrategy != "flat-fusion-v1" || report.ActivationEligible || !report.SourceSearchRetained || !report.Comparable {
		t.Fatalf("shadow comparison must retain the baseline and search: %+v", report)
	}
	if report.CriticalFactRetention != 1 || report.SourceCoverage != 1 || report.CitationCoverage != 1 || report.MaintenanceCoverage != 1 || report.MaintenanceCostTotal != 3 {
		t.Fatalf("unexpected aggregate metrics: %+v", report)
	}

	first, err := MarshalRepresentationEvaluationReport(report)
	if err != nil {
		t.Fatal(err)
	}
	second, err := MarshalRepresentationEvaluationReport(report)
	if err != nil {
		t.Fatal(err)
	}
	if string(first) != string(second) {
		t.Fatalf("report serialization is not deterministic:\n%s\n%s", first, second)
	}
	for _, forbidden := range []string{"critical-source-alias", "question text", "tenant", "benchmark-representations", "ev3", "memory-id", "raw-score", "per-stage-cost"} {
		if strings.Contains(string(first), forbidden) {
			t.Fatalf("report leaked %q: %s", forbidden, first)
		}
	}
}

func TestRepresentationEvaluationRequiresMeasuredBaseline(t *testing.T) {
	in := validRepresentationEvaluationInput(memory.Scope{Tenant: "t", Project: "p", Namespace: "n"})
	in.Baseline = nil
	r, err := EvaluateRepresentation(in)
	if err != nil {
		t.Fatal(err)
	}
	if r.Comparable || r.Verdict != RepresentationEvaluationNonPass || !containsRepresentationFailure(r.Failures, RepresentationFailureMissingBaseline) || r.BaselineMetrics != nil || r.ComparisonDelta != nil {
		t.Fatal("a baseline strategy label cannot substitute for baseline measurements")
	}
	if _, err := MarshalRepresentationEvaluationReport(r); err != nil {
		t.Fatal(err)
	}
}

func TestRepresentationEvaluationTaskOutcomeChangesIdentity(t *testing.T) {
	in := validRepresentationEvaluationInput(memory.Scope{Tenant: "t", Project: "p", Namespace: "n"})
	passed, err := EvaluateRepresentation(in)
	if err != nil {
		t.Fatal(err)
	}
	failed := false
	in.TaskObservation = &failed
	report, err := EvaluateRepresentation(in)
	if err != nil {
		t.Fatal(err)
	}
	if passed.RunIdentity == report.RunIdentity {
		t.Fatal("observed task failure was erased from report identity")
	}
}

func TestRepresentationComparisonPersistsSignedMeasuredDeltas(t *testing.T) {
	in := validRepresentationEvaluationInput(memory.Scope{Tenant: "t", Project: "p", Namespace: "n"})
	failed, repeated, cost := false, 2, int64(3)
	in.TaskObservation, in.RepeatedErrorCount, in.MaintenanceCost = &failed, &repeated, &cost
	in.Baseline.TaskObserved, in.Baseline.TaskSucceeded = true, true
	in.Baseline.RepeatedErrorObserved, in.Baseline.RepeatedErrorCount = true, 1
	in.Baseline.MaintenanceObserved, in.Baseline.MaintenanceCostTotal = true, 8
	report, err := EvaluateRepresentation(in)
	if err != nil {
		t.Fatal(err)
	}
	d := report.ComparisonDelta
	if report.BaselineMetrics == nil || d == nil || d.CharacterCount != -20 || d.TokenCount != -5 || d.LatencyMS != -3 ||
		d.TaskSucceeded == nil || *d.TaskSucceeded != -1 || d.RepeatedErrorCount == nil || *d.RepeatedErrorCount != 1 || d.MaintenanceCostTotal == nil || *d.MaintenanceCostTotal != -5 {
		t.Fatalf("measured deltas missing or unsigned: %+v", d)
	}
	if !report.TaskObserved || report.TaskSucceeded || !report.RepeatedErrorObserved || report.RepeatedErrorCount != 2 {
		t.Fatalf("observations erased: %+v", report)
	}
	if _, err := MarshalRepresentationEvaluationReport(report); err != nil {
		t.Fatal(err)
	}
	in.Baseline.CharacterCount++
	other, err := EvaluateRepresentation(in)
	if err != nil {
		t.Fatal(err)
	}
	if report.RunIdentity == other.RunIdentity {
		t.Fatal("baseline metrics absent from run identity")
	}
	if report.BaselineMetrics.CharacterCount != 120 {
		t.Fatal("report baseline aliases mutable input")
	}
}

func TestRepresentationComparisonMissingExternalMetricsRemainAbsent(t *testing.T) {
	in := validRepresentationEvaluationInput(memory.Scope{Tenant: "t", Project: "p", Namespace: "n"})
	in.TaskObservation, in.MaintenanceCost, in.RepeatedErrorCount = nil, nil, nil
	report, err := EvaluateRepresentation(in)
	if err != nil {
		t.Fatal(err)
	}
	if report.TaskObserved || report.RepeatedErrorObserved || report.MaintenanceObserved || report.ComparisonDelta.TaskSucceeded != nil || report.ComparisonDelta.RepeatedErrorCount != nil || report.ComparisonDelta.MaintenanceCostTotal != nil {
		t.Fatalf("absent observations became measured zeros: %+v", report)
	}
}

func TestRepresentationComparisonRejectsUnsafeMeasuredBaseline(t *testing.T) {
	for _, test := range []struct {
		name   string
		mutate func(*RepresentationComparisonMetrics)
		reason RepresentationEvaluationFailure
	}{
		{"critical loss", func(m *RepresentationComparisonMetrics) { m.CriticalFactRetention = 0.5 }, RepresentationFailureCriticalFactLoss},
		{"source loss", func(m *RepresentationComparisonMetrics) { m.SourceCoverage = 0.5 }, RepresentationFailureBrokenReference},
		{"citation loss", func(m *RepresentationComparisonMetrics) { m.CitationCoverage = 0.5 }, RepresentationFailureBrokenReference},
		{"broken reference", func(m *RepresentationComparisonMetrics) { m.BrokenReferenceCount = 1 }, RepresentationFailureBrokenReference},
		{"stale evidence", func(m *RepresentationComparisonMetrics) { m.StaleOrNoisyCount = 1 }, RepresentationFailureFreshness},
		{"candidate budget", func(m *RepresentationComparisonMetrics) { m.CandidateCount = 5 }, RepresentationFailureBudget},
		{"character budget", func(m *RepresentationComparisonMetrics) { m.CharacterCount = 257 }, RepresentationFailureBudget},
		{"token budget", func(m *RepresentationComparisonMetrics) { m.TokenCount = 65 }, RepresentationFailureBudget},
		{"latency budget", func(m *RepresentationComparisonMetrics) { m.LatencyMS = 101 }, RepresentationFailureLatency},
	} {
		t.Run(test.name, func(t *testing.T) {
			in := validRepresentationEvaluationInput(memory.Scope{Tenant: "t", Project: "p", Namespace: "n"})
			test.mutate(in.Baseline)
			report, err := EvaluateRepresentation(in)
			if err != nil {
				t.Fatal(err)
			}
			if report.Comparable || report.Verdict != RepresentationEvaluationNonPass || !containsRepresentationFailure(report.Failures, test.reason) {
				t.Fatalf("unsafe baseline masked by safe candidate: %+v", report)
			}
			if _, err := MarshalRepresentationEvaluationReport(report); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestRepresentationSerializerRejectsUnsafeBaselineInShadowReport(t *testing.T) {
	report, err := EvaluateRepresentation(validRepresentationEvaluationInput(memory.Scope{Tenant: "t", Project: "p", Namespace: "n"}))
	if err != nil {
		t.Fatal(err)
	}
	report.BaselineMetrics.StaleOrNoisyCount = 1
	delta := representationComparisonDelta(representationReportMetrics(report), *report.BaselineMetrics)
	report.ComparisonDelta = &delta
	report.RunIdentity = representationRunIdentity(report, report.EvaluatedAt)
	if _, err := MarshalRepresentationEvaluationReport(report); err == nil {
		t.Fatal("internally hashed shadow report with unsafe baseline accepted")
	}
}

func TestRepresentationRedactionRejectsFractionalObservationCoverage(t *testing.T) {
	in := validRepresentationEvaluationInput(memory.Scope{Tenant: "t", Project: "p", Namespace: "n"})
	in.TaskObservation, in.MaintenanceCost = nil, nil
	report, err := EvaluateRepresentation(in)
	if err != nil {
		t.Fatal(err)
	}
	report.TaskObservationCoverage = 0.5
	report.RunIdentity = representationRunIdentity(report, report.EvaluatedAt)
	if _, err := MarshalRepresentationEvaluationReport(report); err == nil {
		t.Fatal("fractional task coverage accepted for an unobserved task")
	}
}

func TestRepresentationEvaluationFailsClosedForUnsafeOrIncompatibleInput(t *testing.T) {
	scope := memory.Scope{Tenant: "tenant", Project: "benchmark-representations", Namespace: "ev3"}
	tests := []struct {
		name   string
		mutate func(*RepresentationEvaluationInput)
		reason RepresentationEvaluationFailure
	}{
		{name: "missing source search", mutate: func(in *RepresentationEvaluationInput) { in.SourceSearchRetained = false }, reason: RepresentationFailureSourceSearchMissing},
		{name: "lost fact", mutate: func(in *RepresentationEvaluationInput) { in.CriticalFactsReturned = 0 }, reason: RepresentationFailureCriticalFactLoss},
		{name: "foreign evidence", mutate: func(in *RepresentationEvaluationInput) {
			in.SafetyFailures = []RepresentationEvaluationFailure{RepresentationFailureScope}
		}, reason: RepresentationFailureScope},
		{name: "hidden lifecycle evidence", mutate: func(in *RepresentationEvaluationInput) {
			in.SafetyFailures = []RepresentationEvaluationFailure{RepresentationFailureLifecycle}
		}, reason: RepresentationFailureLifecycle},
		{name: "stale evidence", mutate: func(in *RepresentationEvaluationInput) {
			in.SafetyFailures = []RepresentationEvaluationFailure{RepresentationFailureFreshness}
		}, reason: RepresentationFailureFreshness},
		{name: "over budget", mutate: func(in *RepresentationEvaluationInput) { in.CharacterCount = in.Policy.Limits.MaxCharacters + 1 }, reason: RepresentationFailureBudget},
		{name: "nondeterministic", mutate: func(in *RepresentationEvaluationInput) { in.DeterministicReplay = false }, reason: RepresentationFailureNondeterministic},
		{name: "unsupported candidate", mutate: func(in *RepresentationEvaluationInput) {
			in.Policy.Candidate = RepresentationCandidateExactExcerptWithSearch
			in.Family = RepresentationTaskFamilyCrossDocument
		}, reason: RepresentationFailureIncompatible},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			in := validRepresentationEvaluationInput(scope)
			test.mutate(&in)
			report, err := EvaluateRepresentation(in)
			if err != nil {
				t.Fatal(err)
			}
			if report.Verdict != RepresentationEvaluationNonPass || !report.RollbackRequired || report.ActivationEligible || !containsRepresentationFailure(report.Failures, test.reason) {
				t.Fatalf("report = %+v, want non-pass with %q", report, test.reason)
			}
		})
	}
}

func TestRepresentationEvaluationRejectsInvalidIdentityAndMissingLocks(t *testing.T) {
	scope := memory.Scope{Tenant: "tenant", Project: "benchmark-representations", Namespace: "ev3"}
	for _, mutate := range []func(*RepresentationEvaluationInput){
		func(in *RepresentationEvaluationInput) { in.FixtureVersion = "" },
		func(in *RepresentationEvaluationInput) { in.SourceChecksum = "bad" },
		func(in *RepresentationEvaluationInput) { in.QRELChecksum = "" },
		func(in *RepresentationEvaluationInput) { in.SourceWatermark = "source watermark with spaces" },
		func(in *RepresentationEvaluationInput) { in.Policy.Baseline = "not-flat" },
	} {
		in := validRepresentationEvaluationInput(scope)
		mutate(&in)
		if _, err := EvaluateRepresentation(in); err == nil {
			t.Fatal("EvaluateRepresentation() error = nil, want validation error")
		}
	}
}

func TestRepresentationSelectionAdapterUsesExistingDerivedPathsWithoutPublicActivation(t *testing.T) {
	scope := memory.Scope{Tenant: "tenant", Project: "benchmark-representations", Namespace: "ev3"}
	now := time.Date(2026, 11, 10, 12, 0, 0, 0, time.UTC)
	for _, test := range []struct {
		name      string
		family    RepresentationTaskFamily
		candidate RepresentationCandidate
		input     RepresentationPathAdapterInput
	}{
		{
			name: "exact excerpts", family: RepresentationTaskFamilyExactFacts, candidate: RepresentationCandidateExactExcerptWithSearch,
			input: validRepresentationPathAdapterInput(scope, now),
		},
		{
			name: "topic projection", family: RepresentationTaskFamilyCrossDocument, candidate: RepresentationCandidateTopicProjectionWithSearch,
			input: validRepresentationPathAdapterInput(scope, now),
		},
		{
			name: "hierarchical", family: RepresentationTaskFamilyProceduralLesson, candidate: RepresentationCandidateHierarchicalWithSearch,
			input: validRepresentationPathAdapterInput(scope, now),
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			test.input.Family = test.family
			test.input.Candidate = test.candidate
			observation, err := ObserveTaskAppropriateRepresentation(test.input)
			if err != nil {
				t.Fatal(err)
			}
			if !observation.SourceSearchRetained || !observation.DeterministicReplay || observation.CriticalFactsReturned < 1 || len(observation.SafetyFailures) != 0 {
				t.Fatalf("adapter observation = %+v", observation)
			}
		})
	}
}

func TestRepresentationSelectionAdapterFailsClosedForForeignSourceSearch(t *testing.T) {
	scope := memory.Scope{Tenant: "tenant", Project: "benchmark-representations", Namespace: "ev3"}
	foreign := scope
	foreign.Tenant = "foreign"
	observation, err := ObserveTaskAppropriateRepresentation(RepresentationPathAdapterInput{
		Scope: scope, Family: RepresentationTaskFamilyExactFacts, Candidate: RepresentationCandidateExactExcerptWithSearch,
		Limits:        RepresentationEvaluationLimits{MaxCandidates: 4, MaxCharacters: 256, MaxTokens: 64, MaxLatencyMS: 100},
		SourceSearch:  RepresentationSourceSearchObservation{Scope: foreign, Active: true},
		ExactEvidence: []ProgressiveContextEvidence{{Alias: "fixture-alias", Memory: memory.CanonicalMemory{Scope: scope, State: memory.MemoryStateActive, Content: "exact fact"}}}, ExpectedEvidence: 1,
	})
	if err != nil {
		t.Fatal(err)
	}
	if observation.SourceSearchRetained || !containsRepresentationFailure(observation.SafetyFailures, RepresentationFailureScope) {
		t.Fatalf("foreign source search observation = %+v", observation)
	}
}

func TestRepresentationSelectionEmptyHierarchyCannotInventCriticalFacts(t *testing.T) {
	scope := memory.Scope{Tenant: "t", Project: "p", Namespace: "n"}
	observation, err := ObserveTaskAppropriateRepresentation(RepresentationPathAdapterInput{
		Scope: scope, Family: RepresentationTaskFamilyProceduralLesson, Candidate: RepresentationCandidateHierarchicalWithSearch,
		Limits:       RepresentationEvaluationLimits{MaxCandidates: 4, MaxCharacters: 256, MaxTokens: 64, MaxLatencyMS: 100},
		SourceSearch: RepresentationSourceSearchObservation{Scope: scope, Active: true, ReturnedSources: 1, ReturnedCitations: 1},
		ParentFirst:  ParentFirstEvaluationInput{Scope: scope, StrategyIdentity: "parent-first-v1", Mode: ParentFirstModeShadow}, ExpectedEvidence: 1,
	})
	if err != nil {
		t.Fatal(err)
	}
	if observation.CriticalFactsReturned != 0 {
		t.Fatal("empty hierarchy invented critical fact retention")
	}
}

func TestRepresentationSelectionUsesCombinedRepresentationAndSearchBudget(t *testing.T) {
	scope := memory.Scope{Tenant: "t", Project: "p", Namespace: "n"}
	observation, err := ObserveTaskAppropriateRepresentation(RepresentationPathAdapterInput{
		Scope: scope, Family: RepresentationTaskFamilyExactFacts, Candidate: RepresentationCandidateExactExcerptWithSearch,
		Limits:        RepresentationEvaluationLimits{MaxCandidates: 4, MaxCharacters: 20, MaxTokens: 64, MaxLatencyMS: 100},
		SourceSearch:  RepresentationSourceSearchObservation{Scope: scope, Active: true, ReturnedSources: 1, ReturnedCitations: 1, CharacterCount: 16},
		ExactEvidence: []ProgressiveContextEvidence{{Alias: "source", Memory: memory.CanonicalMemory{Scope: scope, State: memory.MemoryStateActive, Content: "exact fact"}, Citations: []Citation{{MemoryID: "source"}}}}, ExpectedEvidence: 1,
	})
	if err != nil {
		t.Fatal(err)
	}
	if observation.CharacterCount != 26 {
		t.Fatalf("combined characters=%d, want 26", observation.CharacterCount)
	}
}

func TestRepresentationMetricsLeaveMissingExternalObservationsInconclusive(t *testing.T) {
	scope := memory.Scope{Tenant: "tenant", Project: "benchmark-representations", Namespace: "ev3"}
	in := validRepresentationEvaluationInput(scope)
	in.TaskObservation = nil
	in.MaintenanceCost = nil
	in.StaleOrNoisyEvidence = 2
	in.BrokenReferences = 1
	report, err := EvaluateRepresentation(in)
	if err != nil {
		t.Fatal(err)
	}
	if report.TaskObservationCoverage != 0 || report.MaintenanceCoverage != 0 || report.MaintenanceCostTotal != 0 || report.StaleOrNoisyCount != 2 || report.BrokenReferenceCount != 1 {
		t.Fatalf("missing observations must stay inconclusive: %+v", report)
	}
	if report.ActivationEligible || report.Verdict != RepresentationEvaluationNonPass || !containsRepresentationFailure(report.Failures, RepresentationFailureBrokenReference) {
		t.Fatalf("broken reference must remain a hard non-pass: %+v", report)
	}
}

func TestRepresentationRedactionRejectsTamperedOrUnboundedReports(t *testing.T) {
	scope := memory.Scope{Tenant: "t", Project: "p", Namespace: "n"}
	for _, test := range []struct {
		name   string
		mutate func(*RepresentationEvaluationReport)
	}{
		{"changed metrics under same run", func(r *RepresentationEvaluationReport) { r.LatencyMS++ }},
		{"changed timestamp under same run", func(r *RepresentationEvaluationReport) { r.EvaluatedAt = r.EvaluatedAt.Add(time.Minute) }},
		{"negative count", func(r *RepresentationEvaluationReport) { r.CandidateCount = -1 }},
		{"invalid mode", func(r *RepresentationEvaluationReport) { r.Mode = "live" }},
		{"nonhex scope hash", func(r *RepresentationEvaluationReport) { r.ScopeHash = "scope:" + strings.Repeat("z", 64) }},
		{"inconsistent verdict", func(r *RepresentationEvaluationReport) { r.RollbackRequired = true }},
	} {
		t.Run(test.name, func(t *testing.T) {
			r, err := EvaluateRepresentation(validRepresentationEvaluationInput(scope))
			if err != nil {
				t.Fatal(err)
			}
			test.mutate(&r)
			if _, err := MarshalRepresentationEvaluationReport(r); err == nil {
				t.Fatal("tampered report accepted")
			}
		})
	}
}

func TestRepresentationSafetyMissingCoverageCannotPass(t *testing.T) {
	for _, mutate := range []func(*RepresentationEvaluationInput){
		func(in *RepresentationEvaluationInput) { in.ReturnedSources = 0 },
		func(in *RepresentationEvaluationInput) { in.ReturnedCitations = 0 },
		func(in *RepresentationEvaluationInput) { in.StaleOrNoisyEvidence = 1 },
	} {
		in := validRepresentationEvaluationInput(memory.Scope{Tenant: "t", Project: "p", Namespace: "n"})
		mutate(&in)
		r, err := EvaluateRepresentation(in)
		if err != nil {
			t.Fatal(err)
		}
		if r.Verdict != RepresentationEvaluationNonPass || !r.RollbackRequired {
			t.Fatal("incomplete or stale evidence passed")
		}
	}
}

func validRepresentationEvaluationInput(scope memory.Scope) RepresentationEvaluationInput {
	observed := true
	maintenance := int64(3)
	return RepresentationEvaluationInput{
		Scope: scope,
		Policy: RepresentationEvaluationPolicy{
			Version: "representation-evaluation-v1", Baseline: "flat-fusion-v1",
			Candidate: RepresentationCandidateTopicProjectionWithSearch, Mode: RepresentationEvaluationModeShadow,
			Limits: RepresentationEvaluationLimits{MaxCandidates: 4, MaxCharacters: 256, MaxTokens: 64, MaxLatencyMS: 100},
		},
		Family:                RepresentationTaskFamilyCrossDocument,
		FixtureVersion:        "representation-fixture-v1",
		SourceChecksum:        strings.Repeat("a", 64),
		EvidenceDigest:        strings.Repeat("c", 64),
		QRELChecksum:          strings.Repeat("b", 64),
		SourceWatermark:       "watermark-v1",
		RendererVersion:       "renderer-v1",
		EvaluatedAt:           time.Date(2026, 11, 10, 12, 0, 0, 0, time.UTC),
		CriticalFactsExpected: 1,
		CriticalFactsReturned: 1,
		ExpectedSources:       1,
		ReturnedSources:       1,
		ExpectedCitations:     1,
		ReturnedCitations:     1,
		CandidateCount:        2,
		CharacterCount:        100,
		TokenCount:            25,
		LatencyMS:             12,
		SourceSearchRetained:  true,
		DeterministicReplay:   true,
		TaskObservation:       &observed,
		MaintenanceCost:       &maintenance,
		Baseline:              &RepresentationComparisonMetrics{CriticalFactRetention: 1, SourceCoverage: 1, CitationCoverage: 1, CandidateCount: 3, CharacterCount: 120, TokenCount: 30, LatencyMS: 15},
	}
}

func containsRepresentationFailure(values []RepresentationEvaluationFailure, want RepresentationEvaluationFailure) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}

func validRepresentationPathAdapterInput(scope memory.Scope, now time.Time) RepresentationPathAdapterInput {
	evidence := progressiveTestEvidence(scope, "item-a", "memory-a", "two words")
	evidence.Memory.SourceVersion = 1
	evidence.Memory.ModifiedAt = now.Add(-time.Minute)
	projection := progressiveTestProjection(scope, now)
	projection.Items[0].Citation.SourceVersion = 1
	return RepresentationPathAdapterInput{
		Scope: scope, Family: RepresentationTaskFamilyExactFacts, Candidate: RepresentationCandidateExactExcerptWithSearch,
		EvaluatedAt: now, Limits: RepresentationEvaluationLimits{MaxCandidates: 4, MaxCharacters: 256, MaxTokens: 64, MaxLatencyMS: 100},
		SourceSearch:  RepresentationSourceSearchObservation{Scope: scope, Active: true, ReturnedSources: 1, ReturnedCitations: 1, CandidateCount: 1, CharacterCount: 16, TokenCount: 4, LatencyMS: 2},
		ExactEvidence: []ProgressiveContextEvidence{evidence}, SourceEvidence: []ProgressiveContextEvidence{evidence},
		CriticalFacts: []RepresentationCriticalFact{{Alias: evidence.Alias, Value: "two words"}}, ExpectedEvidence: 1,
		TopicProjection: &projection, ExpectedWatermarkHash: projection.SourceWatermarkHash(), ExpectedPolicyVersion: projection.PolicyVersion, ExpectedRendererVersion: projection.RendererVersion,
		ParentFirst: ParentFirstEvaluationInput{Scope: scope, StrategyIdentity: "parent-first-v1", Mode: ParentFirstModeShadow,
			Parents:  []ParentFirstCandidate{{ID: projection.ID, Scope: scope, LifecycleState: memory.MemoryStateActive, ProjectionStatus: memory.ContextProjectionStatusActive, UpdatedAt: now}},
			Children: []ParentFirstChild{{ParentID: projection.ID, Evidence: evidence}},
			Limits:   ParentFirstExpansionLimits{MaxParents: 1, MaxChildrenPerParent: 1, MaxCandidates: 4, MaxAdjacentDistance: 1},
			Baseline: ParentFirstMetrics{ProtectedRecall: 1, MultiHopCoverage: 1}, Candidate: ParentFirstMetrics{ProtectedRecall: 1, MultiHopCoverage: 1}},
	}
}

func TestRepresentationAdapterChecksCriticalValuesAndCanonicalReferences(t *testing.T) {
	scope := memory.Scope{Tenant: "t", Project: "p", Namespace: "n"}
	now := time.Date(2026, 11, 10, 12, 0, 0, 0, time.UTC)
	for _, test := range []struct {
		name   string
		mutate func(*RepresentationPathAdapterInput)
		reason RepresentationEvaluationFailure
	}{
		{"changed value with same count", func(in *RepresentationPathAdapterInput) { in.ExactEvidence[0].Memory.Content = "wrong fact" }, RepresentationFailureCriticalFactLoss},
		{"wrong source alias", func(in *RepresentationPathAdapterInput) { in.ExactEvidence[0].Alias = "invented" }, RepresentationFailureBrokenReference},
		{"unresolvable citation", func(in *RepresentationPathAdapterInput) {
			in.ExactEvidence[0].Citations = []Citation{{MemoryID: "made-up"}}
		}, RepresentationFailureBrokenReference},
		{"stale canonical version", func(in *RepresentationPathAdapterInput) { in.ExactEvidence[0].Memory.SourceVersion++ }, RepresentationFailureFreshness},
		{"projection changed value", func(in *RepresentationPathAdapterInput) {
			in.Family = RepresentationTaskFamilyCrossDocument
			in.Candidate = RepresentationCandidateTopicProjectionWithSearch
			in.TopicProjection.Items[0].Text = "wrong fact"
		}, RepresentationFailureCriticalFactLoss},
		{"projection unresolved version", func(in *RepresentationPathAdapterInput) {
			in.Family = RepresentationTaskFamilyCrossDocument
			in.Candidate = RepresentationCandidateTopicProjectionWithSearch
			in.TopicProjection.Items[0].Citation.SourceVersion++
		}, RepresentationFailureFreshness},
		{"projection watermark mismatch", func(in *RepresentationPathAdapterInput) {
			in.Family = RepresentationTaskFamilyCrossDocument
			in.Candidate = RepresentationCandidateTopicProjectionWithSearch
			in.ExpectedWatermarkHash = "wrong"
		}, RepresentationFailureFreshness},
		{"projection missing policy", func(in *RepresentationPathAdapterInput) {
			in.Family = RepresentationTaskFamilyCrossDocument
			in.Candidate = RepresentationCandidateTopicProjectionWithSearch
			in.ExpectedPolicyVersion = ""
		}, RepresentationFailureFreshness},
		{"projection renderer mismatch", func(in *RepresentationPathAdapterInput) {
			in.Family = RepresentationTaskFamilyCrossDocument
			in.Candidate = RepresentationCandidateTopicProjectionWithSearch
			in.ExpectedRendererVersion = "wrong"
		}, RepresentationFailureFreshness},
		{"hierarchy stale parent", func(in *RepresentationPathAdapterInput) {
			in.Family = RepresentationTaskFamilyProceduralLesson
			in.Candidate = RepresentationCandidateHierarchicalWithSearch
			in.ParentFirst.Parents[0].UpdatedAt = now.Add(-25 * time.Hour)
		}, RepresentationFailureFreshness},
		{"hierarchy unrelated parent", func(in *RepresentationPathAdapterInput) {
			in.Family = RepresentationTaskFamilyProceduralLesson
			in.Candidate = RepresentationCandidateHierarchicalWithSearch
			in.ParentFirst.Parents[0].ID = "other"
			in.ParentFirst.Children[0].ParentID = "other"
		}, RepresentationFailureBrokenReference},
	} {
		t.Run(test.name, func(t *testing.T) {
			in := validRepresentationPathAdapterInput(scope, now)
			test.mutate(&in)
			got, err := ObserveTaskAppropriateRepresentation(in)
			if err != nil {
				t.Fatal(err)
			}
			if !containsRepresentationFailure(got.SafetyFailures, test.reason) {
				t.Fatalf("observation = %+v, want %s", got, test.reason)
			}
			if test.reason == RepresentationFailureCriticalFactLoss && got.CriticalFactsReturned != 0 {
				t.Fatal("same evidence count concealed critical value loss")
			}
		})
	}
}

func TestRepresentationAdapterResolvesRawCitationsAgainstCanonicalReferences(t *testing.T) {
	scope := memory.Scope{Tenant: "t", Project: "p", Namespace: "n"}
	now := time.Date(2026, 11, 10, 12, 0, 0, 0, time.UTC)
	for _, candidate := range []RepresentationCandidate{RepresentationCandidateExactExcerptWithSearch, RepresentationCandidateTopicProjectionWithSearch, RepresentationCandidateHierarchicalWithSearch} {
		for _, rawID := range []string{"", "raw-event-a", "invented-event"} {
			t.Run(string(candidate)+"/"+rawID, func(t *testing.T) {
				in := validRepresentationPathAdapterInput(scope, now)
				in.Candidate = candidate
				if candidate == RepresentationCandidateTopicProjectionWithSearch {
					in.Family = RepresentationTaskFamilyCrossDocument
				}
				if candidate == RepresentationCandidateHierarchicalWithSearch {
					in.Family = RepresentationTaskFamilyProceduralLesson
				}
				in.SourceEvidence[0].Citations = []Citation{{MemoryID: "memory-a", RawEventID: "raw-event-a"}}
				in.ExactEvidence[0].Citations = []Citation{{MemoryID: "memory-a", RawEventID: rawID}}
				in.ParentFirst.Children[0].Evidence.Citations = []Citation{{MemoryID: "memory-a", RawEventID: rawID}}
				in.TopicProjection.Items[0].Citation.RawEventID = rawID
				got, err := ObserveTaskAppropriateRepresentation(in)
				if err != nil {
					t.Fatal(err)
				}
				if rawID == "invented-event" {
					if !containsRepresentationFailure(got.SafetyFailures, RepresentationFailureBrokenReference) {
						t.Fatalf("unresolved raw citation accepted: %+v", got)
					}
				} else if len(got.SafetyFailures) != 0 || got.CriticalFactsReturned != 1 {
					t.Fatalf("canonical citation rejected: %+v", got)
				}
			})
		}
	}
}

func TestRepresentationAdapterRejectsInventedUnprotectedText(t *testing.T) {
	scope := memory.Scope{Tenant: "t", Project: "p", Namespace: "n"}
	now := time.Date(2026, 11, 10, 12, 0, 0, 0, time.UTC)
	for _, candidate := range []RepresentationCandidate{RepresentationCandidateExactExcerptWithSearch, RepresentationCandidateTopicProjectionWithSearch, RepresentationCandidateHierarchicalWithSearch} {
		t.Run(string(candidate), func(t *testing.T) {
			in := validRepresentationPathAdapterInput(scope, now)
			in.Candidate = candidate
			if candidate == RepresentationCandidateTopicProjectionWithSearch {
				in.Family = RepresentationTaskFamilyCrossDocument
			}
			if candidate == RepresentationCandidateHierarchicalWithSearch {
				in.Family = RepresentationTaskFamilyProceduralLesson
			}
			in.ExactEvidence[0].Memory.Content = "two words invented instructions"
			in.ParentFirst.Children[0].Evidence.Memory.Content = "two words invented instructions"
			in.TopicProjection.Items[0].Text = "two words invented instructions"
			got, err := ObserveTaskAppropriateRepresentation(in)
			if err != nil {
				t.Fatal(err)
			}
			if !containsRepresentationFailure(got.SafetyFailures, RepresentationFailureBrokenReference) || got.CriticalFactsReturned != 0 {
				t.Fatalf("invented text accepted because critical fact survived: %+v", got)
			}
		})
	}
}

func TestRepresentationAdapterAcceptsLegacyOptionalSourceLifecycle(t *testing.T) {
	scope := memory.Scope{Tenant: "t", Project: "p", Namespace: "n"}
	now := time.Date(2026, 11, 10, 12, 0, 0, 0, time.UTC)
	for _, candidate := range []RepresentationCandidate{RepresentationCandidateTopicProjectionWithSearch, RepresentationCandidateHierarchicalWithSearch} {
		for _, state := range []memory.MemoryState{"", memory.MemoryStateActive, memory.MemoryStateSuppressed, memory.MemoryStateForgotten, memory.MemoryStateDeleted} {
			t.Run(string(candidate)+"/"+string(state), func(t *testing.T) {
				in := validRepresentationPathAdapterInput(scope, now)
				in.Candidate, in.Family = candidate, RepresentationTaskFamilyCrossDocument
				if candidate == RepresentationCandidateHierarchicalWithSearch {
					in.Family = RepresentationTaskFamilyProceduralLesson
				}
				in.TopicProjection.Items[0].Source.LifecycleState = state
				got, err := ObserveTaskAppropriateRepresentation(in)
				if err != nil {
					t.Fatal(err)
				}
				if state == "" || state == memory.MemoryStateActive {
					if len(got.SafetyFailures) != 0 || got.CriticalFactsReturned != 1 {
						t.Fatalf("active canonical evidence rejected for optional source lifecycle: %+v", got)
					}
				} else if !containsRepresentationFailure(got.SafetyFailures, RepresentationFailureLifecycle) {
					t.Fatalf("hidden source lifecycle accepted: %+v", got)
				}
			})
		}
	}
}

func TestRepresentationAdapterRejectsEvidenceOutsideCurrentValidity(t *testing.T) {
	scope := memory.Scope{Tenant: "t", Project: "p", Namespace: "n"}
	now := time.Date(2026, 11, 10, 12, 0, 0, 0, time.UTC)
	for _, target := range []string{"exact", "reference", "projection", "hierarchy child", "hierarchy projection"} {
		for _, future := range []bool{true, false} {
			t.Run(target+"/"+map[bool]string{true: "future", false: "expired"}[future], func(t *testing.T) {
				in := validRepresentationPathAdapterInput(scope, now)
				validity := memory.TemporalValidity{TemporalFactID: "fact", IngestedAt: now.Add(-time.Hour), ValidFrom: now.Add(-time.Hour), ValiditySource: memory.TemporalValiditySourceExplicit}
				if future {
					validity.ValidFrom = now.Add(time.Hour)
				} else {
					expiry := now
					validity.ValidTo = &expiry
				}
				switch target {
				case "exact":
					in.ExactEvidence[0].Memory.TemporalValidity = validity
				case "reference":
					in.SourceEvidence[0].Memory.TemporalValidity = validity
				case "projection":
					in.Family, in.Candidate = RepresentationTaskFamilyCrossDocument, RepresentationCandidateTopicProjectionWithSearch
					in.TopicProjection.Items[0].TemporalValidity = validity
				case "hierarchy child":
					in.Family, in.Candidate = RepresentationTaskFamilyProceduralLesson, RepresentationCandidateHierarchicalWithSearch
					in.ParentFirst.Children[0].Evidence.Memory.TemporalValidity = validity
				case "hierarchy projection":
					in.Family, in.Candidate = RepresentationTaskFamilyProceduralLesson, RepresentationCandidateHierarchicalWithSearch
					in.TopicProjection.Items[0].TemporalValidity = validity
				}
				got, err := ObserveTaskAppropriateRepresentation(in)
				if err != nil {
					t.Fatal(err)
				}
				if got.CriticalFactsReturned != 0 || !containsRepresentationFailure(got.SafetyFailures, RepresentationFailureFreshness) {
					t.Fatalf("out-of-window evidence retained: %+v", got)
				}
			})
		}
	}
}

func TestRepresentationRealComparisonRequiresEvidenceDigest(t *testing.T) {
	in := validRepresentationEvaluationInput(memory.Scope{Tenant: "t", Project: "p", Namespace: "n"})
	in.RealStack = true
	in.EvidenceDigest = ""
	report, err := EvaluateRepresentation(in)
	if err != nil {
		t.Fatal(err)
	}
	if report.Comparable || report.Verdict != RepresentationEvaluationNonPass {
		t.Fatal("real comparison accepted without payload digest")
	}
}

func TestRepresentationEvidenceDigestBindsPayloadIdentity(t *testing.T) {
	in := validRepresentationEvaluationInput(memory.Scope{Tenant: "t", Project: "p", Namespace: "n"})
	first, err := EvaluateRepresentation(in)
	if err != nil {
		t.Fatal(err)
	}
	in.EvidenceDigest = strings.Repeat("d", 64)
	second, err := EvaluateRepresentation(in)
	if err != nil {
		t.Fatal(err)
	}
	if first.RunIdentity == second.RunIdentity || second.EvidenceDigest != in.EvidenceDigest {
		t.Fatal("evidence digest erased from report identity")
	}
}
