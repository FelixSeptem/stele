package retrieval

import (
	"reflect"
	"testing"
	"time"

	"github.com/FelixSeptem/stele/internal/memory"
)

func TestBuildRetrievalPlanClassifiesQueryFamilies(t *testing.T) {
	tests := []struct {
		name               string
		hints              []QueryAnalysisHint
		signals            []QueryAnalysisSignal
		embeddingAvailable bool
		want               RetrievalQueryFamily
	}{
		{name: "exact lookup", hints: []QueryAnalysisHint{{Kind: QueryAnalysisHintIntent, Disposition: QueryAnalysisHintPresent, Value: "lookup"}}, want: RetrievalQueryFamilyExactLookup},
		{name: "semantic", embeddingAvailable: true, want: RetrievalQueryFamilySemantic},
		{name: "temporal", hints: []QueryAnalysisHint{{Kind: QueryAnalysisHintTemporal, Disposition: QueryAnalysisHintPresent, Value: "recent"}}, want: RetrievalQueryFamilyTemporal},
		{name: "entity relation", hints: []QueryAnalysisHint{{Kind: QueryAnalysisHintEntity, Disposition: QueryAnalysisHintPresent, Value: "stele"}}, want: RetrievalQueryFamilyEntityRelation},
		{name: "multi hop", signals: []QueryAnalysisSignal{{Kind: QueryAnalysisSignalSubquery, Text: "first"}, {Kind: QueryAnalysisSignalSubquery, Text: "second"}}, want: RetrievalQueryFamilyMultiHop},
		{name: "procedural", hints: []QueryAnalysisHint{{Kind: QueryAnalysisHintMemoryClass, Disposition: QueryAnalysisHintPresent, Value: string(memory.MemoryClassProcedural)}}, want: RetrievalQueryFamilyProcedural},
		{name: "general fallback", want: RetrievalQueryFamilyGeneral},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			input := plannerInputForTest(test.hints, test.signals, test.embeddingAvailable)
			plan, err := BuildRetrievalPlan(input)
			if err != nil {
				t.Fatalf("BuildRetrievalPlan() error = %v", err)
			}
			if plan.Family != test.want {
				t.Fatalf("BuildRetrievalPlan().Family = %q, want %q", plan.Family, test.want)
			}
			if err := plan.Validate(input.Policy.HardLimits); err != nil {
				t.Fatalf("plan.Validate() error = %v", err)
			}
		})
	}
}

func TestBuildRetrievalPlanUsesDeterministicPrecedenceAndIdentity(t *testing.T) {
	input := plannerInputForTest(
		[]QueryAnalysisHint{
			{Kind: QueryAnalysisHintEntity, Disposition: QueryAnalysisHintPresent, Value: "stele"},
			{Kind: QueryAnalysisHintTemporal, Disposition: QueryAnalysisHintPresent, Value: "recent"},
			{Kind: QueryAnalysisHintMemoryClass, Disposition: QueryAnalysisHintPresent, Value: string(memory.MemoryClassProcedural)},
		},
		[]QueryAnalysisSignal{{Kind: QueryAnalysisSignalSubquery, Text: "first"}},
		true,
	)

	first, err := BuildRetrievalPlan(input)
	if err != nil {
		t.Fatalf("BuildRetrievalPlan(first) error = %v", err)
	}
	second, err := BuildRetrievalPlan(input)
	if err != nil {
		t.Fatalf("BuildRetrievalPlan(second) error = %v", err)
	}
	if first.Family != RetrievalQueryFamilyProcedural {
		t.Fatalf("precedence family = %q, want %q", first.Family, RetrievalQueryFamilyProcedural)
	}
	if !reflect.DeepEqual(first, second) {
		t.Fatalf("repeated plans differ:\nfirst=%+v\nsecond=%+v", first, second)
	}
	if first.Identity.String() != "retrieval-planner-v1:retrieval-plan-policy-v1:procedural" {
		t.Fatalf("plan identity = %q", first.Identity.String())
	}
}

func TestBuildRetrievalPlanPreservesConfiguredContextPriority(t *testing.T) {
	input := plannerInputForTest(
		[]QueryAnalysisHint{{Kind: QueryAnalysisHintMemoryClass, Disposition: QueryAnalysisHintPresent, Value: string(memory.MemoryClassProcedural)}},
		nil,
		false,
	)

	plan, err := BuildRetrievalPlan(input)
	if err != nil {
		t.Fatalf("BuildRetrievalPlan() error = %v", err)
	}
	if len(plan.ContextPriorities) == 0 || plan.ContextPriorities[0] != memory.MemoryClassProcedural {
		t.Fatalf("context priorities = %v, want procedural first", plan.ContextPriorities)
	}
}

func TestClassifyRetrievalPlanComplexityIsBoundedAndDeterministic(t *testing.T) {
	tests := []struct {
		name    string
		signals []QueryAnalysisSignal
		want    RetrievalPlanComplexityCategory
	}{
		{name: "original query only", want: RetrievalPlanComplexitySimple},
		{name: "one derived term", signals: []QueryAnalysisSignal{{Kind: QueryAnalysisSignalTerm, Text: "one"}}, want: RetrievalPlanComplexitySimple},
		{name: "two derived terms", signals: []QueryAnalysisSignal{{Kind: QueryAnalysisSignalTerm, Text: "one"}, {Kind: QueryAnalysisSignalTerm, Text: "two"}}, want: RetrievalPlanComplexityModerate},
		{name: "one subquery", signals: []QueryAnalysisSignal{{Kind: QueryAnalysisSignalSubquery, Text: "one"}}, want: RetrievalPlanComplexityModerate},
		{name: "two subqueries", signals: []QueryAnalysisSignal{{Kind: QueryAnalysisSignalSubquery, Text: "one"}, {Kind: QueryAnalysisSignalSubquery, Text: "two"}}, want: RetrievalPlanComplexityComplex},
		{name: "four derived signals", signals: []QueryAnalysisSignal{{Kind: QueryAnalysisSignalTerm, Text: "a"}, {Kind: QueryAnalysisSignalTerm, Text: "b"}, {Kind: QueryAnalysisSignalTerm, Text: "c"}, {Kind: QueryAnalysisSignalTerm, Text: "d"}}, want: RetrievalPlanComplexityComplex},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			input := plannerInputForTest(nil, test.signals, false)
			plan, err := BuildRetrievalPlan(input)
			if err != nil {
				t.Fatalf("BuildRetrievalPlan() error = %v", err)
			}
			if plan.ComplexityCategory != test.want {
				t.Fatalf("ComplexityCategory = %q, want %q", plan.ComplexityCategory, test.want)
			}
			repeated, err := BuildRetrievalPlan(input)
			if err != nil {
				t.Fatalf("BuildRetrievalPlan(repeat) error = %v", err)
			}
			if !reflect.DeepEqual(plan, repeated) {
				t.Fatal("repeated complexity classification is not deterministic")
			}
		})
	}
}

func plannerPolicyWithComplexity(category RetrievalPlanComplexityCategory, allocation map[FusionChannel]int) RetrievalPlanPolicy {
	policy := DefaultRetrievalPlanPolicy()
	template := policy.Templates[RetrievalQueryFamilyGeneral]
	template.ComplexityCandidates = map[RetrievalPlanComplexityCategory]map[FusionChannel]int{category: allocation}
	minimum := 0
	for _, count := range allocation {
		if count <= 0 {
			continue
		}
		if minimum == 0 || count < minimum {
			minimum = count
		}
	}
	if minimum > 0 && template.Fusion.PerChannelCandidate > minimum {
		template.Fusion.PerChannelCandidate = minimum
	}
	policy.Templates[RetrievalQueryFamilyGeneral] = template
	return policy
}

func TestBuildRetrievalPlanResharesEnvelopeForModerateComplexity(t *testing.T) {
	policy := plannerPolicyWithComplexity(RetrievalPlanComplexityModerate, map[FusionChannel]int{
		FusionChannelLexical: 20, FusionChannelSemantic: 80, FusionChannelRelation: 50, FusionChannelChunk: 50,
	})
	if err := policy.Validate(); err != nil {
		t.Fatalf("policy.Validate() error = %v", err)
	}

	moderateInput := plannerInputForTest(nil, []QueryAnalysisSignal{{Kind: QueryAnalysisSignalSubquery, Text: "second"}}, false)
	moderateInput.Policy = policy
	moderate, err := BuildRetrievalPlan(moderateInput)
	if err != nil {
		t.Fatalf("BuildRetrievalPlan(moderate) error = %v", err)
	}
	if moderate.Family != RetrievalQueryFamilyGeneral || moderate.ComplexityCategory != RetrievalPlanComplexityModerate {
		t.Fatalf("moderate family/complexity = %q/%q", moderate.Family, moderate.ComplexityCategory)
	}
	if moderate.ChannelCandidates[FusionChannelLexical] != 20 || moderate.ChannelCandidates[FusionChannelSemantic] != 80 {
		t.Fatalf("moderate channel allocation = %v", moderate.ChannelCandidates)
	}
	if moderate.TotalCandidates != 200 {
		t.Fatalf("moderate total candidates = %d, want the unchanged envelope of 200", moderate.TotalCandidates)
	}
	if err := moderate.Validate(policy.HardLimits); err != nil {
		t.Fatalf("moderate plan.Validate() error = %v", err)
	}

	simpleInput := plannerInputForTest(nil, nil, false)
	simpleInput.Policy = policy
	simple, err := BuildRetrievalPlan(simpleInput)
	if err != nil {
		t.Fatalf("BuildRetrievalPlan(simple) error = %v", err)
	}
	if simple.ComplexityCategory != RetrievalPlanComplexitySimple {
		t.Fatalf("simple complexity = %q", simple.ComplexityCategory)
	}
	if simple.ChannelCandidates[FusionChannelLexical] != 50 || simple.ChannelCandidates[FusionChannelSemantic] != 50 {
		t.Fatalf("simple channel allocation = %v, want the declared baseline allocation", simple.ChannelCandidates)
	}
}

func TestBuildRetrievalPlanKeepsBaselineAllocationWithoutComplexityPolicy(t *testing.T) {
	policy := DefaultRetrievalPlanPolicy()
	input := plannerInputForTest(nil, []QueryAnalysisSignal{{Kind: QueryAnalysisSignalSubquery, Text: "second"}}, false)
	input.Policy = policy
	plan, err := BuildRetrievalPlan(input)
	if err != nil {
		t.Fatalf("BuildRetrievalPlan() error = %v", err)
	}
	if plan.ComplexityCategory != RetrievalPlanComplexityModerate {
		t.Fatalf("ComplexityCategory = %q, want moderate", plan.ComplexityCategory)
	}
	for _, channel := range plan.Channels {
		if plan.ChannelCandidates[channel] != 50 {
			t.Fatalf("channel %q allocation = %d, want the unchanged 50 baseline", channel, plan.ChannelCandidates[channel])
		}
	}
}

func TestRetrievalPlanPolicyRejectsInvalidComplexityAllocation(t *testing.T) {
	tests := []struct {
		name       string
		category   RetrievalPlanComplexityCategory
		allocation map[FusionChannel]int
	}{
		{
			name:     "envelope is enlarged",
			category: RetrievalPlanComplexityComplex,
			allocation: map[FusionChannel]int{
				FusionChannelLexical: 20, FusionChannelSemantic: 20, FusionChannelRelation: 20, FusionChannelChunk: 20,
			},
		},
		{
			name:     "declared channel is omitted",
			category: RetrievalPlanComplexityModerate,
			allocation: map[FusionChannel]int{
				FusionChannelLexical: 50, FusionChannelSemantic: 50, FusionChannelRelation: 100,
			},
		},
		{
			name:     "channel exceeds per-channel hard limit",
			category: RetrievalPlanComplexityComplex,
			allocation: map[FusionChannel]int{
				FusionChannelLexical: 110, FusionChannelSemantic: 30, FusionChannelRelation: 30, FusionChannelChunk: 30,
			},
		},
		{
			name:     "unknown category",
			category: RetrievalPlanComplexityCategory("extreme"),
			allocation: map[FusionChannel]int{
				FusionChannelLexical: 50, FusionChannelSemantic: 50, FusionChannelRelation: 50, FusionChannelChunk: 50,
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			policy := plannerPolicyWithComplexity(test.category, test.allocation)
			if err := policy.Validate(); err == nil {
				t.Fatal("policy.Validate() error = nil, want a rejected complexity allocation")
			}
		})
	}
}

func TestBuildRetrievalPlanRejectsUnknownVersionsAndIncompleteTemplates(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*RetrievalPlanInput)
	}{
		{name: "unknown planner", mutate: func(input *RetrievalPlanInput) { input.Policy.PlannerVersion = "unknown" }},
		{name: "unknown policy", mutate: func(input *RetrievalPlanInput) { input.Policy.Version = "unknown" }},
		{name: "missing template", mutate: func(input *RetrievalPlanInput) { delete(input.Policy.Templates, RetrievalQueryFamilyGeneral) }},
		{name: "too many candidates", mutate: func(input *RetrievalPlanInput) {
			template := input.Policy.Templates[RetrievalQueryFamilyGeneral]
			template.TotalCandidates = input.Policy.HardLimits.MaxCandidates + 1
			input.Policy.Templates[RetrievalQueryFamilyGeneral] = template
		}},
		{name: "too many passes", mutate: func(input *RetrievalPlanInput) {
			template := input.Policy.Templates[RetrievalQueryFamilyGeneral]
			template.MaxPasses = 3
			input.Policy.Templates[RetrievalQueryFamilyGeneral] = template
		}},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			input := plannerInputForTest(nil, nil, false)
			test.mutate(&input)
			if _, err := BuildRetrievalPlan(input); err == nil {
				t.Fatal("BuildRetrievalPlan() error = nil, want validation error")
			}
		})
	}
}

func TestBuildRetrievalPlanRejectsInvalidFollowUpChannels(t *testing.T) {
	tests := []struct {
		name     string
		channels []FusionChannel
	}{
		{name: "unknown", channels: []FusionChannel{"unknown"}},
		{name: "undeclared", channels: []FusionChannel{FusionChannelSemantic}},
		{name: "duplicate", channels: []FusionChannel{FusionChannelLexical, FusionChannelLexical}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			input := plannerInputForTest(nil, nil, false)
			template := input.Policy.Templates[RetrievalQueryFamilyGeneral]
			template.Channels = []FusionChannel{FusionChannelLexical}
			template.ChannelCandidates = map[FusionChannel]int{FusionChannelLexical: 2}
			template.TotalCandidates = 2
			template.Fusion.PerChannelCandidate = 2
			template.Fusion.TotalCandidates = 2
			template.MaxPasses = 2
			template.FollowUp = RetrievalPlanFollowUpRule{Enabled: true, MinimumVisible: 1, CandidateAllocation: 1, Channels: test.channels}
			input.Policy.Templates[RetrievalQueryFamilyGeneral] = template
			if _, err := BuildRetrievalPlan(input); err == nil {
				t.Fatal("BuildRetrievalPlan() error = nil, want invalid follow-up rejected")
			}
		})
	}
	t.Run("zero allocation", func(t *testing.T) {
		input := plannerInputForTest(nil, nil, false)
		template := input.Policy.Templates[RetrievalQueryFamilyGeneral]
		template.Channels = []FusionChannel{FusionChannelLexical}
		template.ChannelCandidates = map[FusionChannel]int{FusionChannelLexical: 0}
		template.TotalCandidates = 1
		template.Fusion.PerChannelCandidate = 1
		template.Fusion.TotalCandidates = 1
		template.MaxPasses = 2
		template.FollowUp = RetrievalPlanFollowUpRule{Enabled: true, MinimumVisible: 1, CandidateAllocation: 1, Channels: []FusionChannel{FusionChannelLexical}}
		input.Policy.Templates[RetrievalQueryFamilyGeneral] = template
		if _, err := BuildRetrievalPlan(input); err == nil {
			t.Fatal("BuildRetrievalPlan() error = nil, want zero allocation rejected")
		}
	})
}

func TestBuildRetrievalPlanRejectsFusionOutsidePlanEnvelope(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*RetrievalPlanTemplate)
	}{
		{name: "fusion total exceeds plan total", mutate: func(template *RetrievalPlanTemplate) { template.Fusion.TotalCandidates = template.TotalCandidates + 1 }},
		{name: "fusion channel limit exceeds allocation", mutate: func(template *RetrievalPlanTemplate) {
			template.Fusion.PerChannelCandidate = template.ChannelCandidates[FusionChannelSemantic] + 1
		}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			input := plannerInputForTest(nil, nil, false)
			template := input.Policy.Templates[RetrievalQueryFamilyGeneral]
			template.Channels = []FusionChannel{FusionChannelLexical, FusionChannelSemantic}
			template.ChannelCandidates = map[FusionChannel]int{FusionChannelLexical: 3, FusionChannelSemantic: 2}
			template.TotalCandidates = 5
			template.Fusion.PerChannelCandidate = 2
			template.Fusion.TotalCandidates = 5
			test.mutate(&template)
			input.Policy.Templates[RetrievalQueryFamilyGeneral] = template
			if _, err := BuildRetrievalPlan(input); err == nil {
				t.Fatal("BuildRetrievalPlan() error = nil, want fusion envelope rejected")
			}
		})
	}
}

func TestBuildRetrievalPlanRejectsChannelAllocationAboveTotalCandidates(t *testing.T) {
	input := plannerInputForTest(nil, nil, false)
	template := input.Policy.Templates[RetrievalQueryFamilyGeneral]
	template.Channels = []FusionChannel{FusionChannelLexical, FusionChannelSemantic}
	template.ChannelCandidates = map[FusionChannel]int{FusionChannelLexical: 3, FusionChannelSemantic: 3}
	template.TotalCandidates = 5
	template.Fusion.PerChannelCandidate = 3
	template.Fusion.TotalCandidates = 5
	input.Policy.Templates[RetrievalQueryFamilyGeneral] = template

	if _, err := BuildRetrievalPlan(input); err == nil {
		t.Fatal("BuildRetrievalPlan() error = nil, want channel allocation sum above total rejected")
	}
}

func TestBuildRetrievalPlanCarriesFallbackAllocationInStableIdentity(t *testing.T) {
	input := plannerInputForTest(nil, nil, true)
	template := input.Policy.Templates[RetrievalQueryFamilySemantic]
	template.Channels = []FusionChannel{FusionChannelSemantic}
	template.ChannelCandidates = map[FusionChannel]int{FusionChannelSemantic: 4}
	template.FallbackChannelCandidates = map[FusionChannel]int{FusionChannelLexical: 1}
	template.TotalCandidates = 4
	template.Fusion.PerChannelCandidate = 4
	template.Fusion.TotalCandidates = 4
	input.Policy.Templates[RetrievalQueryFamilySemantic] = template

	plan, err := BuildRetrievalPlan(input)
	if err != nil {
		t.Fatalf("BuildRetrievalPlan() error = %v", err)
	}
	if got := plan.Identity.String(); got != "retrieval-planner-v1:retrieval-plan-policy-v1:semantic:fallback=lexical:1" {
		t.Fatalf("plan identity = %q", got)
	}
	if !reflect.DeepEqual(plan.FallbackChannelCandidates, map[FusionChannel]int{FusionChannelLexical: 1}) {
		t.Fatalf("fallback allocations = %v", plan.FallbackChannelCandidates)
	}

	plan.FallbackChannelCandidates[FusionChannelLexical] = 2
	if err := plan.Validate(input.Policy.HardLimits); err == nil {
		t.Fatal("plan.Validate() error = nil after identity-bound fallback allocation changed")
	}
}

func TestBuildRetrievalPlanRejectsInvalidFallbackAllocation(t *testing.T) {
	tests := []struct {
		name     string
		fallback map[FusionChannel]int
		reranker int
	}{
		{name: "overlaps planned channel", fallback: map[FusionChannel]int{FusionChannelSemantic: 1}},
		{name: "unknown channel", fallback: map[FusionChannel]int{"unknown": 1}},
		{name: "zero allocation", fallback: map[FusionChannel]int{FusionChannelLexical: 0}},
		{name: "exhausts planned capacity", fallback: map[FusionChannel]int{FusionChannelLexical: 3}, reranker: 1},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			input := plannerInputForTest(nil, nil, true)
			template := input.Policy.Templates[RetrievalQueryFamilySemantic]
			template.Channels = []FusionChannel{FusionChannelSemantic}
			template.ChannelCandidates = map[FusionChannel]int{FusionChannelSemantic: 4}
			template.FallbackChannelCandidates = test.fallback
			template.TotalCandidates = 4
			template.Fusion.PerChannelCandidate = 4
			template.Fusion.TotalCandidates = 4
			template.RerankerEligible = test.reranker > 0
			template.RerankerHeadroom = test.reranker
			input.Policy.Templates[RetrievalQueryFamilySemantic] = template
			if _, err := BuildRetrievalPlan(input); err == nil {
				t.Fatal("BuildRetrievalPlan() error = nil, want invalid fallback allocation rejected")
			}
		})
	}
}

func plannerInputForTest(hints []QueryAnalysisHint, signals []QueryAnalysisSignal, embeddingAvailable bool) RetrievalPlanInput {
	analysisSignals := []QueryAnalysisSignal{{Kind: QueryAnalysisSignalOriginal, Text: "test query", Mandatory: true}}
	analysisSignals = append(analysisSignals, signals...)
	return RetrievalPlanInput{
		AcceptedQuery: "test query",
		Analysis: QueryAnalysisResult{
			Identity:    QueryAnalysisIdentity{PolicyVersion: QueryAnalysisPolicyVersionV1, LimitsVersion: QueryAnalysisLimitsVersionV1},
			Disposition: QueryAnalysisDispositionComplete,
			Hints:       append([]QueryAnalysisHint(nil), hints...),
			Signals:     analysisSignals,
		},
		EmbeddingAvailable: embeddingAvailable,
		Policy:             DefaultRetrievalPlanPolicy(),
		Now:                time.Date(2026, 9, 16, 0, 0, 0, 0, time.UTC),
	}
}

func FuzzBuildRetrievalPlanBoundsAndDeterminism(f *testing.F) {
	f.Add("query", uint8(0), true)
	f.Add("another query", uint8(6), false)
	f.Fuzz(func(t *testing.T, query string, familyIndex uint8, embeddingAvailable bool) {
		if query == "" {
			query = "fallback"
		}
		input := plannerInputForTest(nil, nil, embeddingAvailable)
		input.AcceptedQuery = query
		input.Analysis.Signals[0].Text = query
		families := append([]RetrievalQueryFamily(nil), retrievalQueryFamilies...)
		selected := families[int(familyIndex)%len(families)]
		template := input.Policy.Templates[selected]
		template.TotalCandidates = 1 + int(familyIndex)%input.Policy.HardLimits.MaxCandidates
		remaining := template.TotalCandidates
		for _, channel := range template.Channels {
			allocated := remaining
			if allocated > input.Policy.HardLimits.MaxCandidatesPerChannel {
				allocated = input.Policy.HardLimits.MaxCandidatesPerChannel
			}
			template.ChannelCandidates[channel] = allocated
			remaining -= allocated
			if remaining < 1 {
				remaining = 1
			}
		}
		input.Policy.Templates[selected] = template
		before := append([]FusionChannel(nil), template.Channels...)

		first, firstErr := BuildRetrievalPlan(input)
		second, secondErr := BuildRetrievalPlan(input)
		if (firstErr == nil) != (secondErr == nil) {
			t.Fatalf("non-deterministic errors: first=%v second=%v", firstErr, secondErr)
		}
		if firstErr != nil {
			return
		}
		if !reflect.DeepEqual(first, second) {
			t.Fatalf("non-deterministic plans: first=%+v second=%+v", first, second)
		}
		if !reflect.DeepEqual(before, template.Channels) {
			t.Fatalf("input channels mutated: before=%v after=%v", before, template.Channels)
		}
		if first.TotalCandidates > input.Policy.HardLimits.MaxCandidates || first.MaxPasses > 2 || first.LatencyBudget > input.Policy.HardLimits.MaxLatency || first.ContextItems > input.Policy.HardLimits.MaxContextItems {
			t.Fatalf("plan exceeds hard limits: %+v", first)
		}
	})
}

// boundedRetrievalAllocation splits totalCandidates across channels so that
// every channel keeps at least one and at most perChannelCap candidates.
func boundedRetrievalAllocation(seed []byte, total, perChannelCap, channels int) []int {
	counts := make([]int, channels)
	remaining := total
	for index := 0; index < channels; index++ {
		slotsLeft := channels - index
		maxHere := remaining - (slotsLeft - 1)
		if maxHere > perChannelCap {
			maxHere = perChannelCap
		}
		minHere := remaining - (slotsLeft-1)*perChannelCap
		if minHere < 1 {
			minHere = 1
		}
		if maxHere < minHere {
			maxHere = minHere
		}
		value := minHere
		if maxHere > minHere {
			value = minHere + int(seed[index%len(seed)])%(maxHere-minHere+1)
		}
		counts[index] = value
		remaining -= value
	}
	return counts
}

func FuzzRetrievalPlanComplexityAllocationStaysBounded(f *testing.F) {
	f.Add([]byte{3, 7, 11, 13}, uint8(1))
	f.Add([]byte{0, 0, 0, 0}, uint8(0))
	f.Fuzz(func(t *testing.T, seed []byte, signalCount uint8) {
		if len(seed) == 0 {
			seed = []byte{1}
		}
		policy := DefaultRetrievalPlanPolicy()
		template := policy.Templates[RetrievalQueryFamilyGeneral]
		counts := boundedRetrievalAllocation(seed, template.TotalCandidates, policy.HardLimits.MaxCandidatesPerChannel, len(template.Channels))
		allocation := make(map[FusionChannel]int, len(template.Channels))
		minimum := counts[0]
		for index, channel := range template.Channels {
			allocation[channel] = counts[index]
			if counts[index] < minimum {
				minimum = counts[index]
			}
		}
		if template.Fusion.PerChannelCandidate > minimum {
			template.Fusion.PerChannelCandidate = minimum
		}
		template.ComplexityCandidates = map[RetrievalPlanComplexityCategory]map[FusionChannel]int{RetrievalPlanComplexityModerate: allocation}
		policy.Templates[RetrievalQueryFamilyGeneral] = template
		if err := policy.Validate(); err != nil {
			t.Fatalf("policy.Validate() error = %v", err)
		}

		signals := make([]QueryAnalysisSignal, 0, 4)
		for index := 0; index < int(signalCount)%4+1; index++ {
			signals = append(signals, QueryAnalysisSignal{Kind: QueryAnalysisSignalTerm, Text: "term"})
		}
		input := plannerInputForTest(nil, signals, false)
		input.Policy = policy

		plan, err := BuildRetrievalPlan(input)
		if err != nil {
			t.Fatalf("BuildRetrievalPlan() error = %v", err)
		}
		if plan.TotalCandidates != template.TotalCandidates {
			t.Fatalf("TotalCandidates = %d, want the unchanged envelope %d", plan.TotalCandidates, template.TotalCandidates)
		}
		allocated := 0
		for channel, count := range plan.ChannelCandidates {
			if count <= 0 || count > policy.HardLimits.MaxCandidatesPerChannel {
				t.Fatalf("channel %q allocation = %d outside hard bounds", channel, count)
			}
			allocated += count
		}
		if allocated != plan.TotalCandidates {
			t.Fatalf("channel allocations sum to %d, want %d", allocated, plan.TotalCandidates)
		}
		if err := plan.Validate(policy.HardLimits); err != nil {
			t.Fatalf("plan.Validate() error = %v", err)
		}

		repeated, err := BuildRetrievalPlan(input)
		if err != nil {
			t.Fatalf("BuildRetrievalPlan(repeat) error = %v", err)
		}
		if !reflect.DeepEqual(plan, repeated) {
			t.Fatal("complexity allocation is not deterministic")
		}
	})
}
