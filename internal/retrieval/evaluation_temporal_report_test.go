package retrieval

import (
	"context"
	"encoding/json"
	"reflect"
	"testing"
	"time"

	"github.com/FelixSeptem/stele/internal/memory"
)

// Section 7.2 extends a replay report with the fact-valid policy that was
// applied, the selection it used, and the coverage it measured. The contract
// under test is that a report is reproducible under a fixed clock and that
// nothing it publishes can carry an instant, alias, scope, or identifier.

func temporalCoverageMetadata() EvaluationRankingMetadata {
	return EvaluationRankingMetadata{
		FixtureVersion:              "retrieval-temporal-evaluation-fixture-v1",
		RepresentationVersion:       "canonical-v1",
		RankingVersion:              "ranking-v1",
		CompatibleEmbeddingRevision: "embedding-v1",
		PolicyVersion:               "policy-v1",
		TemporalPolicyVersion:       "bi-temporal-policy-v1",
		TemporalCoverageVersion:     "temporal-coverage-v1",
	}
}

func temporalCoverageCase(kind EvaluationTemporalCaseKind, mode memory.TemporalSelectionMode) EvaluationReplayCase {
	return EvaluationReplayCase{
		CaseID:                     "temporal-" + string(kind),
		Category:                   "temporal",
		Scope:                      memory.Scope{Tenant: "eval", Project: "bi-temporal", Namespace: string(kind)},
		ExpectedEvidenceGroups:     [][]string{{"fact"}},
		CandidatePoolSize:          1,
		TemporalKind:               kind,
		TemporalMode:               mode,
		TemporalSelectorKind:       "interval",
		TemporalSelectedVersions:   1,
		TemporalHiddenAliasCount:   1,
		TemporalStaleVersions:      0,
		TemporalAmbiguousVersions:  0,
		TemporalProvenanceMismatch: 0,
		TemporalHiddenVersionLeak:  0,
		TemporalFallbackCategory:   "",
	}
}

// TestEvaluationReportCarriesBoundedTemporalCoverage proves a temporal replay
// publishes the policy it applied and the scenarios it measured as counts and
// bounded categories only.
func TestEvaluationReportCarriesBoundedTemporalCoverage(t *testing.T) {
	run := EvaluationReplay{
		Metadata: temporalCoverageMetadata(),
		Cases: []EvaluationReplayCase{
			temporalCoverageCase(EvaluationTemporalCaseCurrentValid, memory.TemporalSelectionCurrent),
			func() EvaluationReplayCase {
				item := temporalCoverageCase(EvaluationTemporalCaseAsOf, memory.TemporalSelectionAsOf)
				item.TemporalConflictDisposition = string(memory.TemporalConflictResolved)
				item.TemporalFallbackCategory = "stale_similarity_excluded"
				return item
			}(),
		},
	}
	report, err := CalculateEvaluationMetrics(run)
	if err != nil {
		t.Fatalf("CalculateEvaluationMetrics() error = %v", err)
	}
	coverage := report.TemporalCoverage
	if coverage == nil {
		t.Fatal("TemporalCoverage = nil, want the temporal coverage to be reported")
	}
	if coverage.PolicyVersion != "bi-temporal-policy-v1" || coverage.CoverageVersion != "temporal-coverage-v1" {
		t.Fatalf("coverage identity = %q/%q, want the declared temporal policy", coverage.PolicyVersion, coverage.CoverageVersion)
	}
	if coverage.Cases != 2 {
		t.Fatalf("coverage cases = %d, want 2", coverage.Cases)
	}
	if coverage.KindCounts[string(EvaluationTemporalCaseCurrentValid)] != 1 || coverage.KindCounts[string(EvaluationTemporalCaseAsOf)] != 1 {
		t.Fatalf("kind counts = %v, want one of each replayed kind", coverage.KindCounts)
	}
	if coverage.ModeCounts[string(memory.TemporalSelectionCurrent)] != 1 || coverage.ModeCounts[string(memory.TemporalSelectionAsOf)] != 1 {
		t.Fatalf("mode counts = %v, want one of each replayed mode", coverage.ModeCounts)
	}
	if coverage.SelectedVersions != 2 {
		t.Fatalf("selected versions = %d, want 2", coverage.SelectedVersions)
	}
	if coverage.FallbackCounts["stale_similarity_excluded"] != 1 {
		t.Fatalf("fallback counts = %v, want the bounded fallback category", coverage.FallbackCounts)
	}
	if coverage.DispositionCounts[string(memory.TemporalConflictResolved)] != 1 {
		t.Fatalf("disposition counts = %v, want the resolved conflict", coverage.DispositionCounts)
	}
	if report.Cases[1].TemporalKind != EvaluationTemporalCaseAsOf || report.Cases[1].TemporalMode != memory.TemporalSelectionAsOf {
		t.Fatalf("case temporal metadata = %+v, want the as_of selection", report.Cases[1])
	}
}

// TestTemporalSafetyFailuresOverrideQuality proves every temporal safety signal
// is promoted into the common hard-failure channel before a release policy can
// consider ranking quality. Counts remain aggregate-only: the report exposes no
// source, scope, version, query, or provider payload to explain the failure.
func TestTemporalSafetyFailuresOverrideQuality(t *testing.T) {
	caseRun := temporalCoverageCase(EvaluationTemporalCaseStaleSimilarity, memory.TemporalSelectionCurrent)
	caseRun.CandidatePoolSize = QueryAnalysisHardMaxAggregateCandidates + 1
	caseRun.Candidates = []EvaluationReplayCandidate{{
		Alias: "foreign-candidate",
		Scope: memory.Scope{Tenant: "foreign", Project: "bi-temporal", Namespace: "stale-similarity"},
		State: memory.MemoryStateActive,
	}}
	caseRun.TemporalStaleVersions = 1
	caseRun.TemporalAmbiguousVersions = 1
	caseRun.TemporalProvenanceMismatch = 1
	caseRun.TemporalHiddenVersionLeak = 1

	report, err := CalculateEvaluationMetrics(EvaluationReplay{
		Metadata: temporalCoverageMetadata(),
		Cases:    []EvaluationReplayCase{caseRun},
	})
	if err != nil {
		t.Fatalf("CalculateEvaluationMetrics() error = %v", err)
	}

	want := map[EvaluationSafetyFailureCategory]int{
		EvaluationSafetyFailureCrossScope:                    1,
		EvaluationSafetyFailureStaleFactWin:                  1,
		EvaluationSafetyFailureValidityAmbiguity:             1,
		EvaluationSafetyFailureProvenanceMismatch:            1,
		EvaluationSafetyFailureHiddenVersionLeakage:          1,
		EvaluationSafetyFailureCategory("resource_overflow"): 1,
	}
	got := make(map[EvaluationSafetyFailureCategory]int, len(report.SafetyFailures))
	for _, failure := range report.SafetyFailures {
		got[failure.Category] = failure.Count
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("SafetyFailures = %#v, want %#v", got, want)
	}

	policy := EvaluationReleasePolicy{
		Version:          "policy-v1",
		ProtectedCutoffs: []int{1},
		MaxP95LatencyMS:  1,
	}
	baseline := report
	baseline.SafetyFailures = nil
	decision, err := EvaluateReleasePolicy(policy, baseline, report)
	if err != nil {
		t.Fatalf("EvaluateReleasePolicy() error = %v", err)
	}
	if decision.Eligible || !containsString(decision.HardFailures, evaluationDecisionSafetyFailure) {
		t.Fatalf("release decision = %+v, want the temporal failures to block release", decision)
	}
}

// TestEvaluationReportTemporalMetadataIsDeterministicUnderFixedClock proves a
// repeated replay with the same clock produces byte-identical output. Without
// this a report could differ run to run and a release decision would be a coin
// flip.
func TestEvaluationReportTemporalMetadataIsDeterministicUnderFixedClock(t *testing.T) {
	run := EvaluationReplay{
		Metadata: temporalCoverageMetadata(),
		Cases: []EvaluationReplayCase{
			temporalCoverageCase(EvaluationTemporalCaseRetroactiveCorrection, memory.TemporalSelectionAsOf),
			temporalCoverageCase(EvaluationTemporalCaseTemporalIsolation, memory.TemporalSelectionDuring),
			temporalCoverageCase(EvaluationTemporalCaseLegacyCompatible, memory.TemporalSelectionCurrent),
		},
	}
	first, err := CalculateEvaluationMetrics(run)
	if err != nil {
		t.Fatalf("CalculateEvaluationMetrics() error = %v", err)
	}
	second, err := CalculateEvaluationMetrics(run)
	if err != nil {
		t.Fatalf("CalculateEvaluationMetrics() error = %v", err)
	}
	one, err := MarshalEvaluationReport(first)
	if err != nil {
		t.Fatalf("MarshalEvaluationReport() error = %v", err)
	}
	two, err := MarshalEvaluationReport(second)
	if err != nil {
		t.Fatalf("MarshalEvaluationReport() error = %v", err)
	}
	if string(one) != string(two) {
		t.Fatalf("repeated replay is not deterministic:\n%s\n%s", one, two)
	}

	var decoded map[string]any
	if err := json.Unmarshal(one, &decoded); err != nil {
		t.Fatalf("json.Unmarshal() error = %v", err)
	}
	if _, present := decoded["temporal_coverage"]; !present {
		t.Fatalf("report = %s, want a temporal_coverage member", one)
	}
}

// TestEvaluationReportOmitsTemporalCoverageForPreTemporalFixtures proves the
// extension is additive: a report without temporal cases keeps its exact prior
// shape, so existing comparisons and stored reports stay comparable.
func TestEvaluationReportOmitsTemporalCoverageForPreTemporalFixtures(t *testing.T) {
	run := EvaluationReplay{
		Metadata: temporalCoverageMetadata(),
		Cases: []EvaluationReplayCase{{
			CaseID:                 "single-fact",
			Category:               "single-fact",
			Scope:                  memory.Scope{Tenant: "eval", Project: "baseline", Namespace: "single-fact"},
			ExpectedEvidenceGroups: [][]string{{"fact"}},
			CandidatePoolSize:      1,
		}},
	}
	report, err := CalculateEvaluationMetrics(run)
	if err != nil {
		t.Fatalf("CalculateEvaluationMetrics() error = %v", err)
	}
	if report.TemporalCoverage != nil {
		t.Fatalf("TemporalCoverage = %+v, want it omitted without temporal cases", report.TemporalCoverage)
	}
	raw, err := MarshalEvaluationReport(report)
	if err != nil {
		t.Fatalf("MarshalEvaluationReport() error = %v", err)
	}
	// The per-case and coverage members are what must stay absent; the metadata
	// policy pair is declared by the caller and is not a per-case claim. Match
	// on JSON keys so the metadata version strings cannot cause a false hit.
	var decoded struct {
		TemporalCoverage *json.RawMessage `json:"temporal_coverage"`
		Cases            []map[string]any `json:"cases"`
	}
	if err := json.Unmarshal(raw, &decoded); err != nil {
		t.Fatalf("json.Unmarshal() error = %v", err)
	}
	if decoded.TemporalCoverage != nil {
		t.Fatalf("report = %s, want no coverage member", raw)
	}
	for _, item := range decoded.Cases {
		if _, present := item["temporal_kind"]; present {
			t.Fatalf("report = %s, want no per-case temporal kind", raw)
		}
	}
}

// TestEvaluationTemporalCoverageRejectsUnsafeOrInconsistentReports proves the
// new report surface bites: each mutation below must be refused rather than
// published.
func TestEvaluationTemporalCoverageRejectsUnsafeOrInconsistentReports(t *testing.T) {
	base := EvaluationReport{
		Metadata: temporalCoverageMetadata(),
		Cases: []EvaluationCaseReport{{
			CaseID:                      "temporal-current",
			Category:                    "temporal",
			TemporalKind:                EvaluationTemporalCaseCurrentValid,
			TemporalMode:                memory.TemporalSelectionCurrent,
			TemporalSelectedVersions:    1,
			TemporalHiddenAliasCount:    1,
			TemporalConflictDisposition: string(memory.TemporalConflictNone),
		}},
		TemporalCoverage: &EvaluationTemporalCoverage{
			PolicyVersion:     "bi-temporal-policy-v1",
			CoverageVersion:   "temporal-coverage-v1",
			Cases:             1,
			KindCounts:        map[string]int{string(EvaluationTemporalCaseCurrentValid): 1},
			ModeCounts:        map[string]int{string(memory.TemporalSelectionCurrent): 1},
			SelectedVersions:  1,
			DispositionCounts: map[string]int{string(memory.TemporalConflictNone): 1},
		},
	}
	if err := base.validateSafeOutput(); err != nil {
		t.Fatalf("baseline validateSafeOutput() error = %v", err)
	}

	mutations := []struct {
		name   string
		mutate func(*EvaluationReport)
	}{
		{
			name:   "case declares a temporal kind without coverage",
			mutate: func(r *EvaluationReport) { r.TemporalCoverage = nil },
		},
		{
			name:   "coverage disagrees with the declared cases",
			mutate: func(r *EvaluationReport) { r.TemporalCoverage.Cases = 2 },
		},
		{
			name:   "temporal kind is unknown",
			mutate: func(r *EvaluationReport) { r.Cases[0].TemporalKind = EvaluationTemporalCaseKind("made_up") },
		},
		{
			name:   "temporal mode is unknown",
			mutate: func(r *EvaluationReport) { r.Cases[0].TemporalMode = memory.TemporalSelectionMode("made_up") },
		},
		{
			name: "temporal conflict disposition is untyped",
			mutate: func(r *EvaluationReport) {
				r.Cases[0].TemporalConflictDisposition = "postgres://operator:secret@db/internal"
			},
		},
		{
			name:   "temporal selector kind carries an attacker-controlled payload",
			mutate: func(r *EvaluationReport) { r.Cases[0].TemporalSelectorKind = "postgres://operator:secret@db/internal" },
		},
		{
			name:   "temporal count is negative",
			mutate: func(r *EvaluationReport) { r.Cases[0].TemporalSelectedVersions = -1 },
		},
		{
			name: "coverage kind count is zero",
			mutate: func(r *EvaluationReport) {
				r.TemporalCoverage.KindCounts = map[string]int{string(EvaluationTemporalCaseCurrentValid): 0}
			},
		},
		{
			name: "coverage kind is unsupported",
			mutate: func(r *EvaluationReport) {
				r.TemporalCoverage.KindCounts = map[string]int{"made_up": 1}
			},
		},
		{
			name: "coverage fallback category is unsafe",
			mutate: func(r *EvaluationReport) {
				r.TemporalCoverage.FallbackCounts = map[string]int{"postgres://operator:secret@db": 1}
			},
		},
		{
			name: "coverage disposition is untyped",
			mutate: func(r *EvaluationReport) {
				r.TemporalCoverage.DispositionCounts = map[string]int{"made_up": 1}
			},
		},
		{
			name:   "coverage case count is zero",
			mutate: func(r *EvaluationReport) { r.TemporalCoverage.Cases = 0 },
		},
	}
	for _, mutation := range mutations {
		t.Run(mutation.name, func(t *testing.T) {
			report := base
			coverage := *base.TemporalCoverage
			coverage.KindCounts = map[string]int{string(EvaluationTemporalCaseCurrentValid): 1}
			coverage.ModeCounts = map[string]int{string(memory.TemporalSelectionCurrent): 1}
			coverage.DispositionCounts = map[string]int{string(memory.TemporalConflictNone): 1}
			coverage.FallbackCounts = nil
			report.TemporalCoverage = &coverage
			report.Cases = append([]EvaluationCaseReport(nil), base.Cases...)
			mutation.mutate(&report)
			if err := report.validateSafeOutput(); err == nil {
				t.Fatalf("validateSafeOutput() error = nil, want the mutated report to be refused")
			}
		})
	}
}

// TestEvaluationMetadataRequiresTemporalPolicyAndCoverageTogether proves a
// report cannot claim a fact-valid policy without declaring what it measured.
func TestEvaluationMetadataRequiresTemporalPolicyAndCoverageTogether(t *testing.T) {
	policyOnly := temporalCoverageMetadata()
	policyOnly.TemporalCoverageVersion = ""
	if err := policyOnly.Validate(); err == nil {
		t.Fatal("Validate() error = nil, want a temporal policy without coverage to be refused")
	}
	coverageOnly := temporalCoverageMetadata()
	coverageOnly.TemporalPolicyVersion = ""
	if err := coverageOnly.Validate(); err == nil {
		t.Fatal("Validate() error = nil, want temporal coverage without a policy to be refused")
	}
	if err := temporalCoverageMetadata().Validate(); err != nil {
		t.Fatalf("Validate() error = %v, want the paired temporal metadata to be accepted", err)
	}
}

// TestEvaluationReplayWithTemporalSelectionReachesSearcher proves the replay
// path actually applies the fixture's selector instead of replaying a current
// search while reporting a historical one.
func TestEvaluationReplayWithTemporalSelectionReachesSearcher(t *testing.T) {
	fixture := loadTemporalFixture(t)
	seed := EvaluationFixtureSeed{FixtureVersion: fixture.Version}
	for _, item := range fixture.Cases {
		for _, source := range item.Sources {
			state := memory.MemoryStateActive
			// The withheld version is seeded as suppressed so the case exercises
			// the selection rather than a lifecycle rejection.
			if item.Temporal != nil {
				for _, hidden := range item.Temporal.HiddenAliases {
					if hidden == source.Alias {
						state = memory.MemoryStateSuppressed
					}
				}
			}
			seed.Aliases = append(seed.Aliases, EvaluationSeededAlias{
				CaseID: item.ID, Alias: source.Alias, Scope: item.Scope,
				MemoryID: "mem-" + item.ID + "-" + source.Alias, State: state,
			})
		}
	}
	searcher := &recordingTemporalSearcher{}
	runner := NewEvaluationRunner(searcher)

	run, err := runner.Replay(context.Background(), fixture, seed, func() EvaluationRankingMetadata {
		metadata := temporalCoverageMetadata()
		metadata.FixtureVersion = fixture.Version
		return metadata
	}())
	if err != nil {
		t.Fatalf("Replay() error = %v", err)
	}

	// The last replayed case is the isolation interval case, so the recorded
	// input must carry its interval rather than a current selection.
	if searcher.input.TemporalConstraint.Mode != memory.TemporalSelectionDuring {
		t.Fatalf("replayed temporal mode = %q, want valid_during", searcher.input.TemporalConstraint.Mode)
	}
	if searcher.input.TemporalConstraint.ValidFrom == nil || searcher.input.TemporalConstraint.ValidTo == nil {
		t.Fatalf("replayed constraint = %+v, want both interval bounds", searcher.input.TemporalConstraint)
	}
	if !searcher.input.TemporalConstraint.ValidFrom.Equal(time.Date(2025, 7, 1, 0, 0, 0, 0, time.UTC)) ||
		!searcher.input.TemporalConstraint.ValidTo.Equal(time.Date(2025, 10, 1, 0, 0, 0, 0, time.UTC)) {
		t.Fatalf("replayed interval = %v..%v, want the fixture window",
			searcher.input.TemporalConstraint.ValidFrom, searcher.input.TemporalConstraint.ValidTo)
	}

	// Every case must report the temporal metadata it replayed, and the aggregate
	// must agree with the per-case declarations.
	byKind := make(map[EvaluationTemporalCaseKind]int)
	for _, item := range run.Cases {
		if item.TemporalKind == "" {
			t.Fatalf("case %q replayed without a temporal kind", item.CaseID)
		}
		byKind[item.TemporalKind]++
		if item.TemporalMode == "" || item.TemporalSelectorKind == "" {
			t.Fatalf("case %q replayed without a temporal mode or selector kind: %+v", item.CaseID, item)
		}
	}
	report, err := CalculateEvaluationMetrics(run)
	if err != nil {
		t.Fatalf("CalculateEvaluationMetrics() error = %v", err)
	}
	if report.TemporalCoverage == nil {
		t.Fatal("TemporalCoverage = nil, want coverage from a temporal replay")
	}
	if report.TemporalCoverage.Cases != len(fixture.Cases) {
		t.Fatalf("coverage cases = %d, want %d", report.TemporalCoverage.Cases, len(fixture.Cases))
	}
	for kind, count := range byKind {
		if report.TemporalCoverage.KindCounts[string(kind)] != count {
			t.Fatalf("kind %q covered %d times, want %d", kind, report.TemporalCoverage.KindCounts[string(kind)], count)
		}
	}
}

type recordingTemporalSearcher struct {
	input SearchInput
}

func (s *recordingTemporalSearcher) Search(_ context.Context, input SearchInput) (SearchResult, error) {
	s.input = input
	return SearchResult{}, nil
}

func timePointer(value time.Time) *time.Time { return &value }
