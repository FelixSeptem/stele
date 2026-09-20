package retrieval

import (
	"testing"
	"time"

	"github.com/FelixSeptem/stele/internal/memory"
)

// Section 6 covers the retrieval planner and the public API. The planner's job
// here is narrow: carry a validated temporal constraint through RQ1's existing
// plan contract without disturbing the original query, the exact scope, the
// lifecycle rules, or the request envelope.

// TestBuildRetrievalPlanCarriesValidDuringConstraint proves the interval
// selector reaches the plan intact, including its half-open end bound.
func TestBuildRetrievalPlanCarriesValidDuringConstraint(t *testing.T) {
	from := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	to := time.Date(2025, 4, 1, 0, 0, 0, 0, time.UTC)
	input := plannerInputForTest(nil, nil, true)
	input.TemporalConstraint = memory.TemporalConstraint{Mode: memory.TemporalSelectionDuring, ValidFrom: &from, ValidTo: &to}

	plan, err := BuildRetrievalPlan(input)
	if err != nil {
		t.Fatalf("BuildRetrievalPlan() error = %v", err)
	}
	if plan.TemporalConstraint.Mode != memory.TemporalSelectionDuring {
		t.Fatalf("mode = %q, want valid_during", plan.TemporalConstraint.Mode)
	}
	if plan.TemporalConstraint.ValidFrom == nil || !plan.TemporalConstraint.ValidFrom.Equal(from) {
		t.Fatalf("ValidFrom = %v, want %v", plan.TemporalConstraint.ValidFrom, from)
	}
	if plan.TemporalConstraint.ValidTo == nil || !plan.TemporalConstraint.ValidTo.Equal(to) {
		t.Fatalf("ValidTo = %v, want %v", plan.TemporalConstraint.ValidTo, to)
	}
	// The retrieval envelope is untouched by a temporal selector: it narrows
	// when a version is eligible, never which channels run or how many
	// candidates they may return.
	baseline, err := BuildRetrievalPlan(plannerInputForTest(nil, nil, true))
	if err != nil {
		t.Fatalf("BuildRetrievalPlan() error = %v", err)
	}
	if len(plan.Channels) != len(baseline.Channels) || plan.TotalCandidates != baseline.TotalCandidates || plan.Fallback != baseline.Fallback {
		t.Fatalf("plan envelope changed: channels=%v total=%d fallback=%q, want %v/%d/%q",
			plan.Channels, plan.TotalCandidates, plan.Fallback, baseline.Channels, baseline.TotalCandidates, baseline.Fallback)
	}
	for i := range baseline.Channels {
		if plan.Channels[i] != baseline.Channels[i] {
			t.Fatalf("channel %d = %q, want %q", i, plan.Channels[i], baseline.Channels[i])
		}
	}
}

// TestBuildRetrievalPlanCombinesTemporalIdentityAndComplexityAllocation guards
// the shared construction path: a temporal constraint must not discard the
// bounded allocation selected for its query complexity, nor vice versa.
func TestBuildRetrievalPlanCombinesTemporalIdentityAndComplexityAllocation(t *testing.T) {
	asOf := time.Date(2025, 6, 1, 0, 0, 0, 0, time.UTC)
	input := plannerInputForTest(
		[]QueryAnalysisHint{{Kind: QueryAnalysisHintTemporal, Disposition: QueryAnalysisHintPresent, Value: "2025-06-01"}},
		[]QueryAnalysisSignal{{Kind: QueryAnalysisSignalSubquery, Text: "follow-up"}},
		false,
	)
	template := input.Policy.Templates[RetrievalQueryFamilyTemporal]
	template.ComplexityCandidates = map[RetrievalPlanComplexityCategory]map[FusionChannel]int{
		RetrievalPlanComplexityModerate: {
			FusionChannelLexical: 20, FusionChannelSemantic: 80,
			FusionChannelRelation: 50, FusionChannelChunk: 50,
		},
	}
	template.Fusion.PerChannelCandidate = 20
	input.Policy.Templates[RetrievalQueryFamilyTemporal] = template
	input.TemporalConstraint = memory.TemporalConstraint{Mode: memory.TemporalSelectionAsOf, AsOf: &asOf}

	plan, err := BuildRetrievalPlan(input)
	if err != nil {
		t.Fatalf("BuildRetrievalPlan() error = %v", err)
	}
	if plan.ComplexityCategory != RetrievalPlanComplexityModerate {
		t.Fatalf("ComplexityCategory = %q, want %q", plan.ComplexityCategory, RetrievalPlanComplexityModerate)
	}
	if plan.Identity.TemporalMode != memory.TemporalSelectionAsOf {
		t.Fatalf("Identity.TemporalMode = %q, want %q", plan.Identity.TemporalMode, memory.TemporalSelectionAsOf)
	}
	if plan.ChannelCandidates[FusionChannelLexical] != 20 || plan.ChannelCandidates[FusionChannelSemantic] != 80 {
		t.Fatalf("ChannelCandidates = %v, want the moderate temporal allocation", plan.ChannelCandidates)
	}
	if err := plan.Validate(input.Policy.HardLimits); err != nil {
		t.Fatalf("plan.Validate() error = %v", err)
	}
}

// TestBuildRetrievalPlanIdentityIsDeterministicAndNamesHistoricalMode proves the
// plan identity is reproducible and distinguishes a historical selection from a
// current one, so a replay cannot report one selection while running another.
func TestBuildRetrievalPlanIdentityIsDeterministicAndNamesHistoricalMode(t *testing.T) {
	asOf := time.Date(2025, 6, 1, 0, 0, 0, 0, time.UTC)
	historical := plannerInputForTest(nil, nil, true)
	historical.TemporalConstraint = memory.TemporalConstraint{Mode: memory.TemporalSelectionAsOf, AsOf: &asOf}

	first, err := BuildRetrievalPlan(historical)
	if err != nil {
		t.Fatalf("BuildRetrievalPlan() error = %v", err)
	}
	second, err := BuildRetrievalPlan(historical)
	if err != nil {
		t.Fatalf("BuildRetrievalPlan() error = %v", err)
	}
	if first.Identity.String() != second.Identity.String() {
		t.Fatalf("plan identity = %q then %q, want deterministic", first.Identity.String(), second.Identity.String())
	}
	if !containsTemporalMode(first.Identity.String(), string(memory.TemporalSelectionAsOf)) {
		t.Fatalf("plan identity = %q, want it to name the as_of selection", first.Identity.String())
	}

	// An ordinary current plan keeps its exact pre-temporal identity string, so
	// existing fixtures, comparisons, and replays are unaffected. The mirror
	// policy (no embeddings available) exercises the general family, which is
	// the shape RQ1 fixtures were recorded against.
	baseline, err := BuildRetrievalPlan(plannerInputForTest(nil, nil, false))
	if err != nil {
		t.Fatalf("BuildRetrievalPlan() error = %v", err)
	}
	if baseline.Identity.String() != "retrieval-planner-v1:retrieval-plan-policy-v1:general" {
		t.Fatalf("ordinary plan identity = %q, want the unchanged pre-temporal identity", baseline.Identity.String())
	}
	if baseline.Identity.TemporalMode != memory.TemporalSelectionCurrent {
		t.Fatalf("ordinary TemporalMode = %q, want current", baseline.Identity.TemporalMode)
	}

	// The same holds when embeddings are available: the family legitimately
	// becomes semantic, but the temporal segment still does not appear.
	semantic, err := BuildRetrievalPlan(plannerInputForTest(nil, nil, true))
	if err != nil {
		t.Fatalf("BuildRetrievalPlan() error = %v", err)
	}
	if semantic.Identity.String() != "retrieval-planner-v1:retrieval-plan-policy-v1:semantic" {
		t.Fatalf("semantic plan identity = %q, want the unchanged pre-temporal identity", semantic.Identity.String())
	}
	if semantic.Identity.TemporalMode != memory.TemporalSelectionCurrent {
		t.Fatalf("semantic TemporalMode = %q, want current", semantic.Identity.TemporalMode)
	}
	if semantic.Identity.String() == first.Identity.String() {
		t.Fatalf("historical identity %q must differ from the ordinary identity", first.Identity.String())
	}
}

// TestBuildRetrievalPlanIdentityNamesExplicitCurrentModeSeparately proves the
// defaulted current selection and an explicitly requested current selection are
// the same plan. A caller that spells out "current" must not silently get a
// differently-identified plan, or replays would diverge for no semantic reason.
func TestBuildRetrievalPlanIdentityNamesExplicitCurrentModeSeparately(t *testing.T) {
	implicit, err := BuildRetrievalPlan(plannerInputForTest(nil, nil, false))
	if err != nil {
		t.Fatalf("BuildRetrievalPlan() error = %v", err)
	}
	explicitInput := plannerInputForTest(nil, nil, false)
	explicitInput.TemporalConstraint = memory.TemporalConstraint{Mode: memory.TemporalSelectionCurrent}
	explicit, err := BuildRetrievalPlan(explicitInput)
	if err != nil {
		t.Fatalf("BuildRetrievalPlan() error = %v", err)
	}
	if explicit.Identity.String() != implicit.Identity.String() {
		t.Fatalf("explicit current identity = %q, implicit current identity = %q, want identical",
			explicit.Identity.String(), implicit.Identity.String())
	}
	if explicit.TemporalConstraint.Mode != memory.TemporalSelectionCurrent {
		t.Fatalf("explicit current mode = %q, want current", explicit.TemporalConstraint.Mode)
	}
	if explicit.TemporalConstraint.AsOf != nil || explicit.TemporalConstraint.ValidFrom != nil || explicit.TemporalConstraint.ValidTo != nil {
		t.Fatalf("explicit current constraint carries selectors: %+v", explicit.TemporalConstraint)
	}
	// A current plan must carry no historical selector at all, otherwise it
	// could be replayed as a historical request it never authorised.
	if explicit.Identity.TemporalMode != memory.TemporalSelectionCurrent {
		t.Fatalf("explicit current TemporalMode = %q, want current", explicit.Identity.TemporalMode)
	}
}

// TestBuildRetrievalPlanRejectsCurrentConstraintCarryingSelectors proves the
// missing-selector fallback stays strict: current is the only selectorless
// mode, and any stray selector is refused rather than ignored.
func TestBuildRetrievalPlanRejectsCurrentConstraintCarryingSelectors(t *testing.T) {
	asOf := time.Date(2025, 6, 1, 0, 0, 0, 0, time.UTC)
	input := plannerInputForTest(nil, nil, false)
	input.TemporalConstraint = memory.TemporalConstraint{Mode: memory.TemporalSelectionCurrent, AsOf: &asOf}
	if _, err := BuildRetrievalPlan(input); err == nil {
		t.Fatal("BuildRetrievalPlan() error = nil, want a current constraint carrying a selector to be rejected")
	}
	from := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	to := time.Date(2025, 4, 1, 0, 0, 0, 0, time.UTC)
	input = plannerInputForTest(nil, nil, false)
	input.TemporalConstraint = memory.TemporalConstraint{Mode: memory.TemporalSelectionCurrent, ValidFrom: &from, ValidTo: &to}
	if _, err := BuildRetrievalPlan(input); err == nil {
		t.Fatal("BuildRetrievalPlan() error = nil, want a current constraint carrying an interval to be rejected")
	}
}

// TestBuildRetrievalPlanRejectsPlanWhoseIdentityDisagreesWithItsConstraint proves
// the identity is not decoration: a plan claiming one selection while carrying
// another fails validation instead of being replayed as something it is not.
func TestBuildRetrievalPlanRejectsPlanWhoseIdentityDisagreesWithItsConstraint(t *testing.T) {
	plan, err := BuildRetrievalPlan(plannerInputForTest(nil, nil, true))
	if err != nil {
		t.Fatalf("BuildRetrievalPlan() error = %v", err)
	}
	plan.Identity.TemporalMode = memory.TemporalSelectionAsOf
	if err := plan.Validate(DefaultRetrievalPlanHardLimits()); err == nil {
		t.Fatal("Validate() error = nil, want the mismatched temporal identity to be rejected")
	}
}

// TestBuildRetrievalPlanRejectsHistoricalPlanBeyondHardBounds proves 6.2's
// baseline rule: a plan that would exceed the hard bounds is refused rather than
// quietly executed, so the caller falls back to the approved baseline.
func TestBuildRetrievalPlanRejectsHistoricalPlanBeyondHardBounds(t *testing.T) {
	asOf := time.Date(2025, 6, 1, 0, 0, 0, 0, time.UTC)
	input := plannerInputForTest(nil, nil, true)
	input.TemporalConstraint = memory.TemporalConstraint{Mode: memory.TemporalSelectionAsOf, AsOf: &asOf}
	plan, err := BuildRetrievalPlan(input)
	if err != nil {
		t.Fatalf("BuildRetrievalPlan() error = %v", err)
	}

	limits := DefaultRetrievalPlanHardLimits()
	limits.MaxCandidatesPerChannel = 0
	if err := plan.Validate(limits); err == nil {
		t.Fatal("Validate() error = nil, want a plan beyond the hard bounds to be rejected")
	}
	// The fallback disposition stays the approved baseline, so a rejected
	// historical plan degrades to ordinary retrieval instead of a partial or
	// widened search.
	if plan.Fallback != RetrievalPlanFallbackBaseline {
		t.Fatalf("Fallback = %q, want %q", plan.Fallback, RetrievalPlanFallbackBaseline)
	}
}

// containsTemporalMode reports whether an identity string names a temporal
// selection. It is a suffix/segment check so the test does not depend on the
// exact ordering of the identity's other segments.
func containsTemporalMode(identity string, mode string) bool {
	return len(identity) >= len(mode) && identity[len(identity)-len(mode):] == mode ||
		len(identity) > len(mode) && identity[:len(mode)] == mode ||
		segmentPrefix(identity, mode)
}

func segmentPrefix(identity string, mode string) bool {
	for i := 0; i+len(mode) <= len(identity); i++ {
		if identity[i:i+len(mode)] == mode {
			return true
		}
	}
	return false
}
